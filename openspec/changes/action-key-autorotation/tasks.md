# Tasks: Syndra replaces the action signing key itself

Branch `feat/action-key-autorotate`, 5 Nov 2026. Design: [design.md](design.md). ADR-005.

## Backend

- [x] `zitadel.RotateTargetSigningKey`: `POST /v2/actions/targets/{id}` with `{"expirationSigningKey":"0s"}`, the key from the answer, behind `ActionKeyRotator` and `KeyRotator()`. Proof: `TestRotateTargetSigningKey` covers the wire shape, an answer with no key, a 403, and an id with a separator.
- [x] Migration 000077 `action_key_rotations` and `db/action_key_rotations.go`: start, mark replaced, finish, abandon, and state. The advisory lock is shared with the drain as `tryAdvisoryLock`. Proof: `TestActionKeyRotations_Live` covers the lock's exclusivity, an interrupted attempt, abandoned, unverified then verified, and the schema's refusals. It ran green against the dev-box test database on 5 Nov 2026.
- [x] The rotator (`handlers/action_key_rotate.go`): lock, 24h guard, Zitadel answers, the trace before the first write, the key published from the answer, the check, the outcome, the audit row, and `KEY_ROTATION`. Proof:
  - `TestRotate_TraceBeforeTheCall`: the exact event order, and no write without a trace.
  - `TestRotate_SingleFlight`.
  - `TestRotate_RecentlyReplacedRefusedUnlessForced`.
  - `TestRotate_RefusedWhileZitadelIsDown`.
  - `TestRotate_RefusedWriteIsAnHonestFailure`.
- [x] No window where a call can't be verified:
  - `SignatureCheck` takes a `refresh` hook; `Plane.RefreshKeys` and `AwaitNewKeys` add the in-flight mark (`claims:keyring:rotating`).
  - `Keyring.Next` ignores a lagging read of the previous key; `Keyring.Hold` keeps the previous key accepted; `ChangedAt` is persisted.
  - Proof:
    - `TestRotate_NoUnverifiableWindowInEitherProcess`: the backend door, a separate claims process over the same Redis, and the event listener all verify the new key on their first call and the old one through the overlap. `events:keyring` is in Redis, and a stale read doesn't demote the key.
    - `TestRotate_ACallInTheGapWaitsForTheKey`.
    - `TestKeyring_NextAndHold`.
    - `TestSignatureCheck_RefreshesOnceOnAMismatch`: a forged call is refused within 300ms.
    - `TestRotationInFlight_EndsItself`.
- [x] The event listener's key is under the keyring (`events:keyring`, followed every minute with the claims key, and loaded at start), so no restart is ever needed. `.env` keys are bootstrap only.
- [x] The check after a replacement, and its failure. Proof:
  - `TestRotate_ProbeFailureHoldsBothKeysAndDoesNotLoop`: unverified, both keys held for 7 days, the finding raised, and three schedule looks replace nothing.
  - `TestConfirmUnverified_ByTheProbesNotAnotherReplacement`.
- [x] Age is observed (`observedLastReplaced`: Syndra's record, then the keyring's `changed_at`, then `.env`).
- [x] The schedule (`RunActionKeyAutoRotate` every 5 min; lead, window, 20h gap, recent L1 pass, kill switch). Proof:
  - `TestNextAutoAttempt_LeadAndWindow`: covers a 92-day-old key, inside and outside the window, the window's edges, the lead, an unknown age, the gap, a window across midnight, and bad windows.
  - `TestRunAuto_FirstRunOnAnOverdueKey`.
  - `TestAutoConfig_OffWithoutZitadel`.
- [x] Endpoints: `POST /zitadel/action-key/rotate`, `POST /zitadel/action-key/verify`, and the extended status (`?watch=1`, `register_command`, `SYNDRA_SERVER_DIR`). Proof:
  - `TestHandleRotateActionKey_StrictJSONAndResult`: four bad bodies, the result, no key in the answer, audited.
  - `TestActionKeyRoutes_AreOperatorOnly`.
  - `TestHandleVerifyActionKey`.
  - `TestServerCommand`.
  - The router guards: gate, strict decoding, seams.
- [x] Compose: the schedule's envs, `TZ`, and `SYNDRA_SERVER_DIR=${PWD}`. `cmd/api` embeds `time/tzdata`.

## UI

- [x] The SigningKey row's states, Replace now (Modal, Cancel first and focused, "Will be replaced", result), and `FORECAST.replace` with its copy guide §5 row. Proof: `app/zitadel/__tests__/page.test.tsx`.
- [x] The by-hand guide (route-zitadel §2) in `components/system/SigningKeyGuide.tsx`:
  - one command;
  - it moves on by itself when the key changes;
  - it checks the key automatically;
  - "Not seen yet?" appears after 3 minutes;
  - it can be resumed;
  - it covers the setup path.
  - Proof: `SigningKeyGuide.test.tsx`.
- [x] `KEY_ROTATION` copy in Claims delivery. Audit sentences for `action_key.replaced`, `action_key.not_replaced` and `action_key.verified_later`. The proxy refuses a member on the rotate route (`route.test.ts`).

## Docs

- [x] ADR-005 (reverses #71). This change. DEPLOY.md's rotation section, permissions note and troubleshooting. rotate.sh's header (now the manual fallback). The application-claims spec's two sections. NEXT.md.

## Operator-gated

- [ ] Set `TZ` in production's `.env` (for example `TZ=Asia/Kolkata`). Unset, the 03:00–05:00 window is read in UTC. #ready-for-human
- [ ] Confirm the backend's service account still holds `action.target.write`. If it was revoked after registration, the first automatic attempt fails with `KEY_ROTATION:failed` and the dead-man alert, and nothing else changes. #ready-for-human
- [ ] After the first automatic replacement in production, read `/zitadel`. The row should say "Replaced on its own on …", the audit should hold an `action_key.replaced` row, and `sig_mismatch` on `/api/v1/claims/delivery` should stay at zero around the replacement. #ready-for-human
