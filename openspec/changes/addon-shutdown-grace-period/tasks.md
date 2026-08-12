# Shutdown grace period — task ledger

> The add-on gives its own shutdown twenty seconds so an in-flight mutation
> settles. Docker gives it ten, then `SIGKILL`. Nothing makes the two agree, and
> a truncated drain is indistinguishable from a clean stop from outside.

## 1. The fix

- [ ] 1.1 `stop_grace_period` on `truenas-addon`, set above the add-on's own `shutdownTimeout` so the process always reaches its own deadline first. The container timeout is the outer bound and must never be the binding one — if it binds, the add-on's drain logic is decoration
- [ ] 1.2 The add-on's shutdown budget becomes a **named constant** rather than a literal inside `main`. A guard cannot read a magic number reliably, and the point of this change is that the two values are checked against each other
- [ ] 1.3 Applied to every add-on service as they arrive, not `truenas-addon` alone. The next add-on inherits the same shutdown path and would inherit the same truncation silently

## 2. The guard

- [ ] 2.1 A source guard reading the add-on's constant and the `stop_grace_period` from `docker-compose.yml`, failing if the compose value is not strictly greater. Same shape as `config_env_test.go`, which was written for this exact class — a value the add-on depends on that only the deployment manifest supplies
- [ ] 2.2 The failure message names both numbers and which file each came from. A guard that says only "mismatch" sends the reader to the wrong one of two files
- [ ] 2.3 Mutation-verified: lower the compose value below the constant and confirm the guard fails, as `config_env_test.go` was checked

## 3. Verification

- [ ] 3.1 `go test ./... && go vet ./...` in `addons/truenas`
- [ ] 3.2 `docker compose config` parses and shows the value on the service
- [ ] 3.3 Observed once against the live deployment: stop the add-on with a mutation in flight and confirm the settle completes and the terminal status is written. This is the only step that proves the original defect is gone rather than merely reconfigured — everything above asserts the numbers, not the behaviour

## Deliberately not done

- **No change to the shutdown logic itself.** It is correct; it was never given
  the time it asks for. Rewriting it would be fixing the half that works.
- **No general audit of stop timeouts across the stack.** The datastores and the
  UI have no equivalent settle to protect, and widening this change to them
  would delay a one-line fix to a shipped defect.
