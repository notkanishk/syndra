# Design: Syndra replaces the action signing key itself

Decision record: [ADR-005](../../../docs/adr/005-syndra-replaces-the-action-signing-key.md). Code: `backend/internal/handlers/action_key_rotate.go`, the keyring in `claim_watch.go` and `zitadel_action_auth.go`, `claimserve` (`Keyring`, `Plane.RefreshKeys`, `AwaitNewKeys`, `SignatureCheck`), `zitadel.RotateTargetSigningKey`, `db/action_key_rotations.go`, migration 000077.

## 1. One replacement, in order

`rotateActionKeys(trigger, actor, force)` runs these steps. A refusal at any of steps 1–3 changes nothing and writes nothing.

1. **Lock.** `pg_try_advisory_lock(771234502)` on its own pooled connection (the drain lock's helper, now shared as `tryAdvisoryLock`). Postgres releases it if the holder's session dies, so a crash never leaves it held. While the lock is held, no other attempt can be running, so any leftover `in_flight` row is marked `abandoned`. A second click, or the schedule during a click, gets `409 ROTATION_IN_FLIGHT`.
2. **24h guard.** If Syndra's record shows a replacement of the claims key in the last 24h, the attempt is refused (`409 RECENTLY_REPLACED`) unless `force` is set. The button sends `force:true`, because its Modal is the confirmation. The schedule never forces.
3. **Zitadel answers.** The attempt reads `/v2/actions/targets/search` for the target ids. If the read fails, the answer is `503 ZITADEL_UNAVAILABLE`. This read is the health check: the only one that matters is that the API about to be written answers. If there is no claims target, the answer is `409 NOT_REGISTERED`.
4. **Trace.** `action_key_rotations` gets a row with `in_flight` and the planned targets, written **before** the first write. If the insert fails, nothing is sent to Zitadel.
5. **Each target, claims first.**
   - Set `claims:keyring:rotating` (see §2).
   - `POST /v2/actions/targets/{id}` with `{"expirationSigningKey":"0s"}`. This is rotate.sh's call, and the new key comes back in the answer.
   - `adoptKey` publishes the new key to the keyring before anything else happens. The row's `replaced` gets the target name next, so a crash after Zitadel answered still says which keys changed.
   - If Zitadel refuses (4xx), stop. Any other error (5xx, transport) may mean Zitadel replaced the key and the answer was lost, so Syndra reads the keys back from Zitadel at once.
6. **Check** (§4), then the outcome:
   - `verified`, `unverified` or `failed` is written to the row.
   - An `audit_logs` row is written (`action_key.replaced` or `action_key.not_replaced`, with trigger, outcome, targets, probe class and error).
   - `KEY_ROTATION` is raised, or settled.

The targets are `claimcheck.ClaimTarget` and `claimcheck.EventTarget`, which `TestManifestMatchesTargetsJSON` keeps equal to `targets.json`. Keys are never logged, never stored in Postgres, and never in an answer.

## 2. No window where a call can't be verified, in either process

Both doors verify the claims call: the claims container (`cmd/claims`, :8090, where Caddy routes) and the backend's fallback (:8080). Only the backend verifies events. All of them read the keyring from Redis.

- **From the answer.** `adoptKey` → `moveKeyring`:
  - For claims, it writes `claims:keyring`, moves `claims:gen`, and polls this process's plane at once.
  - For events, it writes `events:keyring` and swaps this process's atomic copy.
  - The backend therefore verifies with the new key before the rotator takes another step.
- **The other process, without waiting for its poll.** `SignatureCheck` takes a `refresh` hook. When a call matches no key, the door asks again (`Plane.RefreshKeys` → `AwaitNewKeys`) and verifies once more. The new key therefore verifies on the claims container's first call, not after its next 1s poll.
  - The extra look is rate-limited to one Redis read per 50ms across all callers. A forged call is still refused at once (tested under 300ms).
  - The in-flight check is one GET per refused call. A `ponytail:` comment marks this as the place to cache it if floods ever show.
- **The gap before Zitadel's answer.** Zitadel signs with the new key from the moment it commits it, a few milliseconds before its answer reaches Syndra. While `claims:keyring:rotating` holds a future unix time, a door that matches no key polls every 50ms for up to 1s for the keyring to move. 1s is far inside Zitadel's 3s call timeout. The mark's value ends itself, with no TTL needed, so a crash leaves no door waiting.
- **Overlap.**
  - The previous key keeps verifying `PreviousKeyKeep` (15 min), as before.
  - After an unverified check it is held 7 days (`Keyring.Hold`). This is mild: the reason to replace is age, not compromise, and the owner can always replace again.
- **Projection lag.** Zitadel's search reads a projection that can lag its write. A read just after a replacement that still returns the old key would have swapped the keyring back and dropped the new key 15 minutes later. `Keyring.Next` treats a read of the key just moved off (the kept previous one) as stale, never as a rotation back: Zitadel makes every key fresh, so a replaced one cannot return.

### Persistence (restart while Zitadel is unreachable)

Both keyrings live in Redis with no TTL. Redis runs with `--appendonly yes` on the `redisdata` volume, on the internal-only `data` network. This is the existing secret-at-rest handling for the claims keyring since S5, and it is extended to `events:keyring`.
- On start, the backend loads `events:keyring` before asking Zitadel.
- The claims container loads `claims:keyring` with its snapshot.
- The `.env` keys are only the bootstrap: they verify until a keyring is read, and they are the last fallback.

**Choice.** The simplest safe option was to reuse Redis AOF rather than add Postgres storage. Plaintext keys in Postgres would reach every `pg_dump` and backup, Syndra has no envelope key to encrypt them with, and adding one would be a second secret to manage. Redis AOF is plaintext on a local volume, the same exposure as `.env`.

## 3. Age is observed

`observedLastReplaced` picks, in order:
1. Syndra's record: the latest attempt whose `replaced` holds the claims target. The time is `finished_at`, or `started_at` for an attempt that was interrupted.
2. The keyring's `changed_at`, when it is more than a minute later. This is a key change Syndra saw but did not make, such as a manual rotate.sh. The value is stamped by `Keyring.Next` and persisted with the keyring.
3. `ZITADEL_ACTION_SIGNING_KEY_ROTATED_AT`, only while neither of the above exists. A date in the future is no date.

The status reports `last_rotated_source` (`syndra`, `keyring` or `env`) and `due_at` (last replaced + threshold). The threshold is still `ZITADEL_ACTION_SIGNING_KEY_ROTATION_THRESHOLD_DAYS` (default 90).

## 4. The check after a replacement

`verifyRotation` runs these checks in order:
1. This process verifies with each new key (`actionSigningKeys()[0]`, `eventSigningKeys()[0]`). A failed publish shows up here, where L1 cannot see it, because L1 signs with whatever Syndra holds.
2. **L1** at once (`RunProbeL1`): the canary call signed with the key Syndra now holds, through public DNS, Caddy and the claims container. It proves the claims container has the key.
3. **L2** when the canary is set up (`RunProbeL2`): a real Zitadel token. It proves Zitadel signs with the key Syndra holds.

Each probe records itself as its schedule would. The results lead to these outcomes:
- **Pass:** `verified`, and `next_due_at` = replaced + threshold.
- **Fail:** `unverified`.
  - Both keys stay accepted for 7 days.
  - `KEY_ROTATION:unverified` is raised. It is soft, so `/healthz/claims` goes degraded, the dead-man ping stops, and the owner's external alert fires.
  - Nothing replaces the key again on its own. The schedule's next due date is 85 days away, and the 20h gap applies as well.
  - `confirmUnverified` (every schedule look) marks the attempt `verified` and settles the finding once L1, and L2 when set up, have passed since the attempt and the keyring has read Zitadel since. That is observation, never another replacement.
- **Zitadel refused:** `failed`.
  - The old key still verifies.
  - `KEY_ROTATION:failed` is raised.
  - The schedule tries again in a later window, at most once per window.

## 5. The schedule

`RunActionKeyAutoRotate` runs every 5 minutes (`ACTION-KEY`, started with the keyring when a Management client exists).
- `nextAutoAttempt = windowFrom(max(now, last + threshold − lead, lastAuto + 20h))`.
- An unknown age is due.
- The window is `HH:MM-HH:MM` in the container's local time (`TZ`, an IANA zone; the image embeds `time/tzdata`) and may cross midnight.
- It replaces only when the next attempt is now, L1 passed in the last 5 minutes, and the switch is on.

| Env | Default | Meaning |
|---|---|---|
| `SYNDRA_ACTION_KEY_AUTOROTATE` | on | `off` (or false/0/no) is the kill switch. The look still runs, to confirm an unverified attempt. |
| `SYNDRA_ACTION_KEY_LEAD_DAYS` | 5 | Days before the threshold. Must be under the threshold. |
| `SYNDRA_ACTION_KEY_WINDOW` | `03:00-05:00` | The quiet window. |
| `TZ` | unset (UTC) | The zone the window is read in. |
| `SYNDRA_SERVER_DIR` | `${PWD}` from compose | The checkout the by-hand guide's command `cd`s into. |

On staging `ZITADEL_DOMAIN` is empty, so the schedule is off (`off_reason: no_zitadel`).

**The first production run.** The key was 92 days old, so its age is past `threshold − lead`.
- The look runs at start, but L1 has not run yet, so it waits. L1 starts 5s after listen.
- The first look inside the first quiet window after deploy, with L1 passed in the last 5 minutes, replaces both keys, checks them, and records `verified`.
- From then on the age comes from Syndra's record, and the next attempt is 85 days later. `TestRunAuto_FirstRunOnAnOverdueKey` walks this path: L1 failing, kill switch, outside the window, then the replacement, then nothing more.

## 6. Endpoints

- `POST /api/v1/zitadel/action-key/rotate` (operator).
  - Body: `{"force": bool}`, strict.
  - 200: `{outcome, replaced_at, replaced[], probe, next_due_at, error}`.
  - Errors: 409 `ROTATION_IN_FLIGHT`, `RECENTLY_REPLACED` or `NOT_REGISTERED`; 503 `ZITADEL_UNAVAILABLE` or `NOT_CONFIGURED`.
  - It runs on a context detached from the request (2 min), because a Zitadel write cut off halfway is the one outcome nobody can read back.
- `POST /api/v1/zitadel/action-key/verify` (operator). Body `{}`, strict.
  - It reads Zitadel's keys into the keyring and runs the check.
  - It changes no key and writes no record row. It answers `{probe, last_rotated_at, next_due_at}`.
- `GET /api/v1/zitadel/action-rotation-status` gains:
  - `last_rotated_source`, `due_at` and `register_command`;
  - `auto{on, off_reason, lead_days, window, next_attempt_at}`;
  - `rotating`;
  - `last_outcome{trigger, started_at, finished_at, outcome, replaced, probe, error}`.
  - `?watch=1` asks the keyring to read Zitadel now, at most once per 10s whoever asks: the guide's watch.
  - `rotate_command` and `register_command` carry `cd $SYNDRA_SERVER_DIR && …` when that is an absolute, plain path.

Both POST routes use `withOperatorAuth`. The UI proxy passes admins through, and `route.test.ts` proves a member is refused.

## 7. UI (Zitadel page)

- **The row says:**
  - "Replaced on its own on <date>. Next by <date>."
  - "Due on <date>; Syndra replaces it on its own before then."
  - "Replacing…" while one runs.
  - The two failure states.
  - When switched off, it says so.
- **Replace now** opens a Modal: one sentence, Cancel first and focused, then "Replace the key". It shows `FORECAST.replace` ("Will be replaced") while in flight, then the result.
- **Do it by hand** opens the route-zitadel §2 guide (`components/system/SigningKeyGuide.tsx`):
  - **Step 1** is one copy row with the server's command, and "Let Syndra do it now" as the primary act.
  - The guide watches `?watch=1` every 3s and moves on by itself when the key changes.
  - **Step 2** calls `/verify` and becomes the result.
  - A "Not seen yet?" fold appears after 3 minutes with the old explicit steps.
  - It can be resumed through sessionStorage.
  - "How to set it up" is the same guide with `register_command`.
- `ClaimsDelivery` has copy for `KEY_ROTATION`, and the audit vocabulary has sentences for the three actions.

## Open

- `TechnicalDetail` (P11) is not on main yet. The guide has its own fold, and switches to `TechnicalDetail` when that lands.
- The backend's service account must keep `action.target.write`. DEPLOY.md used to say the instance permissions could be revoked between script runs; that is no longer true.
