# One standard path per operation

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
