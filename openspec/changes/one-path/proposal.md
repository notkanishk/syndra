# One standard path per operation

**Why.** The owner, 5 Nov 2026: "follow same pipe for same operation… core should remain same… consistent and safe behaviour… be elegant." The architecture audit of that day (job `146ae399`, `architecture-audit.md`) found operations that reach the same outcome by two pipes, where one pipe skips the other's guards. This change collects the waves that put each operation on one pipe. Each wave adds its own section here.

## W1a · Zitadel screen pipes (audit items 1 and 2; families 1c, 1d, 5, 11)

**Problem (verified on `main` at `4d668447`).**
- Zitadel › Projects created, renamed and deleted roles by calling Zitadel's Management API straight from `handlers/discovery.go` (`zitadelAddProjectRole`, `zitadelUpdateProjectRole`, `zitadelDeleteProjectRole`). There was no trace, no audit row, no local `roles` row, and no key check. Delete had no guard, and Zitadel takes a deleted role from every holder. Meanwhile `POST /api/v1/roles` (`services.CreateRole`) made roles another way: local row first, audit after with its error discarded, and a rollback on any error, which left an orphan role in Zitadel after a 503 or a timeout that had actually landed.
- Zitadel › People's grant remove (`handleRemoveZitadelGrant`) queued one revoke of every role on the Zitadel grant. It did not check whether a bundle or a rule still gave a role, which `services.DeleteDirectGrant` does. Assign skipped `checkRoleOffered`, the already-held check and `services.Deliver`.

**What.**
- **Roles: one pipe.** `services/roles.go` `writeRole` is now the only way a project role is written, from either screen. `CreateRole`, `UpdateRole` and `DeleteRole` each describe their write, and `writeRole` runs it the same way every time:
  1. writes the trace (Syndra's `roles` row plus an audit row, in one transaction under the access lock);
  2. calls Zitadel;
  3. on any error, reads the role back;
  4. undoes the record only when Zitadel is known not to be in the state asked for: the read-back shows it absent, or Zitadel answered a final refusal (a 4xx other than 408/429). That undo is audited as `role.not_changed`. When the answer was lost (a timeout, a broken connection, 408, 429, 5xx) AND the read-back failed, nobody knows: the record stays, the audit says `role.change_unknown`, and the route answers 502 `OUTCOME_UNKNOWN` with a sentence that says Syndra can't tell and how to check, never "Nothing was changed" (coherence audit C6 and W13, 5 Nov 2026). Before this, a failed read-back always undid, so a delete that may already have taken the role from every holder was reported as not having happened.
- **Delete never strips silently.** It is refused (409 `ROLE_IN_USE`) while Syndra gives the role through a person, a bundle or a rule. It is refused (409 `ROLE_HELD`) while anyone holds it in Zitadel, with the holders' names and their count, until the caller sends that exact count back as `?confirm_holders=N`. It is refused (409 `HOLDERS_UNKNOWN`) when there is no complete, current observation to count from.
- **Grants: the person page's pipe.** The three `/api/v1/zitadel/users/{id}/grants` routes call the same pieces as People › Access:
  - Assign runs `checkRoleOffered` and `dbRequestHolding` on every role before any write, then writes one direct grant per role.
  - Remove sends a role with a direct grant through `services.DeleteDirectGrant`. A role Syndra still gives (`UserExpectsRole`) is kept. Only a role nothing in Syndra gives is revoked directly.
  - Replace is remove-then-give over the difference.
  - All three rebuild the person's cache and deliver through `services.Deliver`. Each answers `roles[]`, every role with its change (given, revoked, kept) and its own status after delivery (applied, waiting, pending, failed, kept), and `status` is the least delivered of them (coherence audit W8). The screen reports each role, never "done" for one that waits or that Zitadel didn't take.
- **The legacy routes stay.** They are now thin callers.

**Not in scope.**
- Rule-target delivery for direct grants (audit 1b) belongs to W3a, the planner.
- `handlers/roles.go`'s error mapping for `POST /api/v1/roles` is unchanged. It is not a W1a file, and an `ErrRoleNotChangedInZitadel` there still answers 500.
- Target mappings and role waits that name a deleted role are not refused. They give nobody anything.
Source: owner, 5 Nov 2026, and the architecture audit of the same day (twelve families of operations that reach one outcome by more than one code path). Each wave gives one family a single core and moves its callers onto it. Sections are per wave agent; each owns a disjoint set of files.

## W1d · Retries, backoff and idempotency (audit family 11, ranked item 7)

**Why.** Every external call carried its own retry rules, and two of them were unsafe:

- The outbox requeued a row with no time gate (`db/propagations.go:663` on main), so a short Zitadel blip could spend the whole retry budget (`OUTBOX_MAX_RETRIES=5`, `services/propagation/deps.go:209`) in back-to-back passes. With the client's own three tries per call, one row could take up to 15 unspaced attempts.
- The Zitadel client resent any method on a 503 (`zitadel/client.go:518`), including a create whose first try had landed. Role create then saw a failure, rolled back its own row (`services/roles.go:80-84`) and left an orphan role in Zitadel.
- A 401 after a 429 failed without refreshing the token (`client.go:509`, refresh on the first try only).
- A lost answer on a read failed at once with no retry (`client.go:500`).
- The start-up wait schedule was written twice (`cmd/api/main.go:171`, `:209`).

**What changes.**

- New `backend/internal/extcall`: one per-try timeout, one classification (retryable: 408, 429, 5xx, timeout, broken connection; every other status is final, and what a 409 means stays the caller's), one jittered capped backoff, a read-back hook for an ambiguous write, and a request id on every log line.
- The Zitadel client calls through it. Reads are retried. A write is resent only after a 429. Any other failure of a write is ambiguous: `AddUserGrant` and `AddProjectRole` read back, and every other write returns the failure unretried for the outbox to handle.
- The outbox gains `not_before` (migration 000080). A requeue sets it from `extcall.Backoff`, and the background claims skip the row until then. A row past its budget still lands in `failed`, which the unfinished-revocations surface already shows.
- *Amended 5 Nov 2026 (owner, coherence audit W1, W2, S9).* "Handed to the outbox, which retries with a backoff" was true only for revocations: an add was retried only when somebody pressed Send after its backoff. Now:
  - **One retry model** (`extcall/retry.go`). Each system's rule is a `Policy`: `OutboxRetry` (one send plus `OUTBOX_MAX_RETRIES`, 1m doubling to 30m) and `AddonResend` (a member's storage password: one resend, by the page). Shared: the states (sent, retrying with next try and try n of N, waiting for you, failed), the backoff (`extcall.Backoff`), the audit mark (`retried_by: automatic`, `try`, `of`), and on screen one set of words (`ui/src/lib/retry.ts`) and one part (`RetryStatus`).
  - **The background drain retries what was already sent.** `ClaimDueRetries` takes a row that was sent once (straight away, or by Send), that Zitadel didn't take (503, no answer, 429), and whose `not_before` has passed — gives included, because the operator consented when it went straight away. Never a row that waits for the operator (`role_waits`, by stamp or by the role being marked; `manual`), and never a withdrawal (the revocation claim owns those). Every automatic try is audited (`propagation.retried`, with try n of N and how it went). The drain ticks every minute (`REVOCATION_DRAIN_INTERVAL`, default 1m) so the named next try is kept on time.
  - **Send is the operator asking now.** `ClaimPendingPropagations` no longer skips rows in backoff; only the background claims do.
  - **Truth in the words.** `Delivery.WaitReason()` returns `halted:retrying` when a send was requeued or errored, and `Drain.Retry` carries the first such row's next try and try number, read from the row. Approve, give, remove, bundle retire and rule retire say "Zitadel didn't answer. Syndra sends it again at 14:05 (try 2 of 6).", never "Sent". `GetPendingPropagations` returns `not_before`, `attempts` and `retry`; each retrying row on Pending changes says when, in IST. Home's Pending changes block counts retrying changes and changes Syndra gave up on this week (`/governance/indicators` `retrying`, `gave_up`).
- One start-up wait schedule in `cmd/api/main.go`.

**Not preserving, by intent:** retry timing, and writes are no longer resent inline on a 503.
Owner, 5 Nov 2026. Source: the architecture audit (family 8 and the wave plan). Each wave names the files it owns.

## W1b. Error envelope (audit item 6)

Problem, verified on main: `jsonErrorResponse` hid `err.Error()` only at exactly 500 (`handlers/router.go:685-700`), so raw Zitadel and add-on text reached the screen at 502 and 503 (`discovery.go:136-432`, `drift.go:417`, `propagations.go:21,46`, `targets.go:513`, `inventory.go:161,181,226`, `entitlements.go:211`, `profile.go:26,69`). The UI proxy put prose in `error` (`route.ts` 403, 401, 400, 413, 502), and passed a gateway page through as text. `merge_findings.go:395` sent `err.Error()` in the `error` slot; `claim_shape.go:203` sent the developer string `save again with acknowledge_breaking` as the message.

Done:
- `handlers/errors.go` holds the envelope, `jsonErrorResponse`, `supportReference`, `jsonValidationErrorResponse`. A 500, and a 502/503/504 whose code is in `rawUpstreamCodes`, logs the raw text with a reference and sends a plain sentence plus `details.request_id`.
- The proxy speaks the same shape (`{error: CODE, message, details?}`); a non-JSON gateway error keeps its status and Retry-After and loses its text.
- `ApiError` never shows a bare string body at 5xx and has a fallback sentence per status. `MergeFindings` and `ShapeChooser` show the reference.
- The two hand-built bodies now embed `ErrorResponse` with typed extras.

Not done (files owned by other waves, so left alone): 5xx call sites that pass an authored sentence are not touched on purpose, because some carry safety ("the change may have applied"). The code list is the ceiling. The upgrade is a `writeUpstream(w, status, code, err)` at each raw call site once W1a/W1e land, and then `rawUpstreamCodes` goes away. `bundles.go:65,339`, `access.go:335`, `claimserve/door.go:338` and `addons/truenas/server.go:416` are still hand-built (UNVERIFIED whether they matter to the screen). `outcome.ts` code map (`CODES`, `isCode`) is not built: the envelope now carries the sentence, so the UI has no code-to-copy table to keep.
Owner, 5 Nov 2026. Source: the architecture audit (families 1-11, ranked items, build order W1a-W3). Each wave gives one operation one path, so a rule, a trace or a guard added later lands in one place.

This file holds the waves that have landed. Tasks and evidence are in [tasks.md](tasks.md).

## W1e/W1f: one add-on pipe (audit family 2, item 11)

**Problems, verified on `main` before the fix.**

- A lifecycle change left no trace: `addons/read.go` `SetLifecycle` wrote no `addon_operations` row, and its handler (`handlers/inventory.go` `handleSetTargetLifecycle`) wrote no audit row.
- The breaker check (`allow`, `credentialFor`, `record`) was hand-copied in 12 places: `entitlement.go` Apply, `transport.go` Call, and ten functions in `read.go`. A new endpoint that forgot it never tripped the breaker.
- `PutSetting` ran its own record pipe and sent no idempotency key. It also kept its own copy of the outcome-to-status map, which `addonop.statusFor` had as well.
- The TrueNAS add-on decided twice: `plan.go` `projected` and the `changes` test, against `apply.go` `converge` building `update`.

**Fix.**

- `backend/internal/addons/pipe.go`: `(*Addon).send` is the one request path (breaker, credential, claim, signed call, record). `operate` adds the row and audit line before the call and settles after. `StatusFor` is the one outcome-to-status map.
- A write is refused unless it carries a trace made before the call. Every write carries an `Idempotency-Key` header.
- Lifecycle now begins an `addon_operations` row (`lifecycle.set`, subject = the target, about = the new state), which writes the audit line `addon.<target>.lifecycle.set.dispatched` in the same transaction. It still skips the breaker, on purpose.
- `addons/truenas`: `decide(current, desired, home) change` is the one decision. `plan` reads it to say what would happen; `converge` and `createAndConverge` read it to do it.

**Behaviour.** The same outcomes for the same inputs, except two additions: a lifecycle flip leaves a row and an audit line, and writes carry the `Idempotency-Key` header. Wording is untouched: plan keeps `describePlannedChange`, apply keeps `describeChange`.

## W2b · One cache refresh (audit family 10, item 12, backend)

Verified on main before changing: `drain.go` refreshed the cache only after an applied row (`refreshClaims`, was `drain.go:516`); a failed row and a row out of retries reached `markFailed` with no refresh (was `drain.go:326-349`). `services/service_accounts.go` (`SaveServiceAccount`, `UnmarkServiceAccount`) had none, though `facts.IsServiceAccount` is compiled from the mark (`cache/compiler.go`). `handleUpdateBundle` had none, though token facts carry bundle names (`compiler.go` `bundleNames`).

Change: `cache.Refresh(ctx, people, projects...)` in `cache/refresh.go` is the one call a write path makes after its commit. Routed through it: the drain (applied, failed, exhausted via `refreshAfterRow`), service-account mark/unmark, and a bundle rename (every holder from `db.GetBundleHoldersByVersion`, on a detached bounded context). A cache failure is logged and never fails the write; the 12-hour warm stays the backstop. Guard: `cache/refresh_guard_test.go` fails if a listed write path stops calling it, or if new code calls `cache.RefreshUser` directly.

Not moved (outside W2b's file list, still on `RefreshUser`/`RebuildUserCache`): `handlers/webhook.go`, `access.go`, `bulk.go`, `role_members.go`, `claim_delivery.go`, `services/expiry/sweep.go`. They are allow-listed in the guard.

## W2c · One invalidation map (audit family 10, item 12, UI)

Change: `ui/src/lib/queries/invalidation.ts` maps each kind of write (`access`, `bundle`, `bundleRename`, `request`, `drift`, `role`, `serviceAccount`, `expiryNote`, `claimShape`, `signingKey`, `target`, `storageCredential`, `terms`) to the keys it makes stale; every mutation hook calls `invalidateAfter(qc, kind, scope)`. Every kind that can move a rail count includes `["governance"]`, which holds `["governance","indicators"]`. Hooks that were short now reach what the audit named: approving a request (people, pending changes), a drain (people), `useAnswerUnconfirmed` (had no invalidation at all). The signing-key pair is now one `signingKey` kind (`app/zitadel/page.tsx`, `SigningKeyGuide.tsx`). `useUpstream.ts` is W1a's and untouched.

Guards (`__tests__/invalidationMap.test.tsx`): every kind is classified and rail kinds hold `["governance"]`; every `useMutation` in `lib/queries` calls `invalidateAfter` unless it is a listed dry run or lookup; no raw `invalidateQueries` in `lib/queries`.

Left: raw `invalidateQueries` in `components/targets/HomeLimit.tsx`, `components/targets/UnconfirmedChanges.tsx` (writes) and `components/states/{DegradedBanner,index}.tsx` (global refetch), outside W2c's list.
