# Shutdown grace period — task ledger

> The add-on gives its own shutdown twenty seconds so an in-flight mutation
> settles. Docker gives it ten, then `SIGKILL`. Nothing makes the two agree, and
> a truncated drain is indistinguishable from a clean stop from outside.

## 1. The fix

- [x] 1.1 `stop_grace_period: 30s` on `truenas-addon`, set above the add-on's own `shutdownTimeout` so the process always reaches its own deadline first. The container timeout is the outer bound and must never be the binding one — if it binds, the add-on's drain logic is decoration
- [x] 1.2 The add-on's shutdown budget becomes a **named constant** (`shutdownTimeout`, `main.go`) rather than a literal inside `main`. A guard cannot read a magic number reliably, and the point of this change is that the two values are checked against each other
- [x] 1.3 Applied to every add-on service as they arrive, not `truenas-addon` alone. `TestEveryAddOnServiceSetsAStopGracePeriod` enumerates every `*-addon` service in the manifest and requires the setting on each, so an add-on added without one fails at once rather than inheriting the truncation silently. What it asserts of a *future* add-on is presence, not sufficiency — that add-on's own budget is a constant in its own module, which this package cannot read, so its module carries the comparison and this one carries the floor

## 2. The guard

- [x] 2.1 A source guard reading the add-on's constant and the `stop_grace_period` from `docker-compose.yml`, failing if the compose value is not strictly greater. Same shape as `config_env_test.go`, which was written for this exact class — a value the add-on depends on that only the deployment manifest supplies
- [x] 2.2 The failure message names both numbers and which file each came from. A guard that says only "mismatch" sends the reader to the wrong one of two files
- [x] 2.3 Mutation-verified in both directions: `stop_grace_period: 5s` fails naming 20s against 5s and both files; deleting the line entirely fails naming the absence and Docker's 10s default. Restored and green after each

## 3. Verification

- [x] 3.1 `go test ./... && go vet ./...` in `addons/truenas`
- [x] 3.2 `docker compose config` parses and shows `stop_grace_period: 30s` on the service
- [x] 3.2a **The shipped binary's shutdown path, run for real.** Everything above asserts the numbers, and numbers are satisfied by a drain that never runs. `shutdown_binary_test.go` builds the add-on, starts it, sends SIGTERM, and asserts it enters the drain, completes it, and exits cleanly **inside the budget Compose grants** — read from `stop_grace_period` rather than hardcoded, so the assertion tracks the deployment. Mutation-verified by making the process exit before the drain. This does NOT close 3.3: what still needs hardware is a mutation actually in flight against a target settling rather than being abandoned, and no local harness can produce one
- [ ] 3.3 Observed once against the live deployment: stop the add-on with a mutation in flight and confirm the settle completes and the terminal status is written. This is the only step that proves the original defect is gone rather than merely reconfigured — everything above asserts the numbers, not the behaviour

## Deliberately not done

- **No change to the shutdown logic itself.** It is correct; it was never given
  the time it asks for. Rewriting it would be fixing the half that works.
- **No general audit of stop timeouts across the stack.** The datastores and the
  UI have no equivalent settle to protect, and widening this change to them
  would delay a one-line fix to a shipped defect.
