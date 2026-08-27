## 1. Guards

- [x] 1.1 `TestEveryRouteCarriesAGate` — every registered route names one of
      five gates, `/healthz` excepted by argument. Gates are matched by name
      rather than by shape: a misspelled wrapper containing the word "Auth"
      would pass a shape test.
- [x] 1.2 `TestEveryMutationDecodesStrictly` — follows one delegation hop,
      because a two-line handler that hands off to a shared body is the normal
      shape here. `HandleActionInject` is excepted with its reason: Zitadel owns
      that payload and may extend it.
- [x] 1.3 `TestNoHandlerBypassesItsOwnSeam` — a handler calling a service
      directly while a seam for that exact function sits in `deps.go`.
- [x] 1.4 `TestTheLogSpeaksOneVocabulary` — a fixed subsystem set, and severity
      never in the tag. Walks `backend/` and `addons/`: one product, one
      operator log, read by one person during one incident.
- [x] 1.5 `TestOnlyTheTracedPathWritesZitadelGrants` — with an allowlist that
      refuses to rot, since an entry naming a file that no longer exists is a
      permission granted to nobody, sitting where the next reader will trust it.

## 2. The drifts the guards found

- [x] 2.1 `[CACHE WARN]` / `[CACHE ERROR]` / `[CACHE]` and `[ZITADEL ERROR]` /
      `[ZITADEL]` — severity in the tag fragments the subsystem's own index.
- [x] 2.2 `drain.go` wrote both `[DRAIN]` and `[PROPAGATION]`, in one file, for
      one job. The add-on had the same drift: `[SERVE]` for a line that is
      `[STARTUP]`'s.
- [x] 2.3 `handleGetUserDirectGrants` called `services.UserDirectGrants`
      directly while `svcUserDirectGrants` sat in `deps.go` — an untestable
      handler and a dead seam that reads as live.
- [x] 2.4 Two seams over `services.ResolveEntitlements`: four call sites went
      through a file-local wrapper and one through `deps.go`. The wrapper now
      delegates to the seam, so one substitution covers all five.
- [x] 2.5 Three `fmt.Errorf(... %v)` wraps in the orchestrator dropped the error
      chain, against 571 sites that use `%w`.

## 3. Verification

- [x] 3.1 `go build ./... && go vet ./... && go test ./...` in `backend/` and
      `addons/truenas`.
- [x] 3.2 Every guard mutation-checked: a route stripped of its gate, a handler
      past its seam, a mutation decoding leniently, a severity tag, a second
      subsystem name, a new direct Zitadel writer, and a rotted allowlist entry.
      Each fails the guard that names it.

## Open

- [ ] 4.1 The untraced rule-propagation path (`openspec/NEXT.md`). Dormant, and
      the fix belongs in its own change: enqueue rather than call.
- [ ] 4.2 Five dependency seams live outside `deps.go`
      (`mapping_plan.go`, `rotation_status.go`, `system.go` ×3). Harmless where
      they are, but the pattern says one place, and 136 of 141 are in it.
