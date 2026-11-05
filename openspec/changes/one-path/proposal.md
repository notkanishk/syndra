# One standard path per operation

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
