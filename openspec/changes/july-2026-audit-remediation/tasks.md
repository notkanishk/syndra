# Tasks — July 2026 Audit Remediation

Finding IDs reference `AUDIT.md` → "Addendum — July 2026 Full Audit".

## Security & Correctness

- [x] SC1 — operator-gate `POST /users/{id}/grants` + `POST /requests/{id}/decision`; regression test: member token → 403 through the real router
- [x] SC3 — `withSelfOrOperatorAuth` on per-user reads; `GET /audit` operator-only; `GET /requests` self-filtered for members; tests for cross-user 403 / self 200 / member list filtering
- [x] SC4 — HMAC-sign session cookie (`payload.sig`) in `lib/session.ts` + Edge verification in `middleware.ts`; tests: forged, tampered, and legacy-unsigned cookies rejected
- [x] SC5 — webhook dedup key = `aggregateID:eventType:sequence`; test: redelivery with fresh signature header dedupes, no re-dispatch
- [x] SC8 — bind `requester_id` to principal for non-operators; test: spoofed requester overridden
- [x] SC9 — `request<T>` 401 → redirect to `/login`
- [ ] SC2 — sync: add `ErrorNetwork` to `IsConnectionError` (deferred — sync excluded; TOP PRIORITY on resume)
- [ ] SC6 — sync: per-UID worker routing (deferred)
- [ ] SC7 — sync: fresh bounded context for drain-time backend calls (deferred)

## Over-engineering

- [x] OE1 — deps.go delegating closures → direct refs (services, zitadel, expiry, drift)
- [x] OE2 — delete unused `ResourceName` component + barrel export
- [x] OE3 — merge expiry/drift schedulers into `services/periodic.Runner`; lifecycle tests moved to the shared package; batch clamp moved into `expiry.Sweep` with test
- [x] OE4 — Modal/Drawer share `useDialogFocusTrap`
- [x] OE5 — remove force-render hack from the four Name components; `SHOW_DEBUG_IDS` hoisted (exported from UserName)
- [x] OE6 — delete dead ui exports (`getClientApiBase`, `getServerApiBase`, `fetchCatalog`, `formatProjectName`, `toastInfo`, `toastPromise`)
- [x] OE8 — fold `dedupProjectIDs` into `dedupeNonEmpty`
- [x] OE9 — delete dead `AssignUserToRole` + tests
- [x] OE12 — remove no-op resolver `prefetch` + its single call site
- [x] OE10 — sync: drop 9 decode-only `ProvisioningIntent` fields (pulled forward out of the sync deferral — decode shape only, no LDAP behavior)
- [x] OE11 — sync: `GetShadowCredentialHash` returns hash only; `ShadowCredentialHash.Algorithm` dropped; interface, mock, and both client tests updated
- [ ] OE7 / OE13 / OE14 — sync module cleanups (still deferred with sync)
- [ ] OE15 — golang-migrate swap (deliberately declined: dirty-state guard worth the dep)

## Over-engineering — 2026-07-29 re-audit (OE16–OE22)

- [x] OE16 — delete dead `db.ResolveDriftItem`
- [x] OE17 — delete dead `db.CreateMappingRule` (+ now-unused `fmt` import in `rules.go`)
- [x] OE18 — delete dead `db.AddRoleToBundle`
- [x] OE19 — delete dead `db.InsertExclusion`
- [x] OE20 — delete dead `db.RemoveBundleFromUser`
- [x] OE22 — remove empty `backend/pkg/` directory
- [ ] OE21 — `sortedKeys` → `slices.Sorted(maps.Keys(…))` (declined: nil-vs-`[]` would change `role_keys` JSON on a zero-role Zitadel grant)

## Verification — 2026-07-29 re-audit

- [x] backend: `go build ./... && go vet ./... && go test ./...` — all packages green
- [x] sync: `go vet ./... && go test ./...` — all packages green
- [x] ui: untouched by this pass (OE2/OE6/OE12 already shipped) — no re-run needed
- [x] AUDIT.md: OE10/OE11 ticked, OE16–OE22 addendum added with the OE21 decline rationale

## Verification

- [x] backend: `go build ./... && go vet ./... && go test ./...` — 487 tests green
- [x] ui: `bun run test && bun run lint && bun run build` — 148 tests green, clean build
- [x] AUDIT.md checkboxes updated with deferral note
