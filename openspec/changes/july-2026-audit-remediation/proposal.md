# July 2026 Audit Remediation

## Why

The July 2026 full audit (`AUDIT.md`, July addendum) found two critical defects and a cluster of important ones sharing a single root cause: **authorization was enforced only in the Next.js proxy, not at the backend trust boundary**. Any authenticated member calling the backend directly could grant themselves any role (SC1), read org-wide grants/requests/audit data (SC3), and impersonate other users on request creation (SC8). Independently, the UI session cookie was unsigned and therefore forgeable (SC4), Zitadel webhook redeliveries were not deduplicated (SC5), and an expired session mid-SPA left the user stuck on error toasts (SC9). The same audit's over-engineering pass identified ~330 removable lines.

## What Changes

Security/correctness (backend):

* `POST /users/{id}/grants` and `POST /requests/{id}/decision` move from `withUserAuth` to `withOperatorAuth` — matching the access-governance spec, which frames both as operator actions (SC1).
* New `withSelfOrOperatorAuth` middleware gates per-user reads (`GET /users/{id}/grants`, `/bundles`, `/access`): subject must match `{id}` or carry the operator role (SC3).
* `GET /audit` becomes operator-only; `GET /requests` filters to the caller's own requests for non-operators (SC3).
* `POST /requests` binds `requester_id` to the authenticated principal for non-operators (SC8).
* Shared `isOperator(r)` helper backs all three; `withOperatorAuth` refactored onto it (still reads the parsed principal from context — C4 contract preserved).
* Webhook idempotency keys off the stable `aggregateID:eventType:sequence` tuple (carried as `WebhookPayload.DedupKey`) instead of the per-delivery `ZITADEL-Signature` header, so Zitadel redeliveries dedupe instead of emitting duplicate provisioning intents (SC5).

Security/correctness (ui):

* The session cookie is now HMAC-SHA256 signed (`payload.signature`); tampered, forged, or legacy-unsigned cookies are rejected in both `lib/session.ts` (Node) and `middleware.ts` (Edge, Web Crypto). Secret: `SESSION_SECRET`, falling back to `SYNDRA_API_KEY` (SC4). Existing sessions are invalidated once — users re-login.
* `request<T>` redirects to `/login` on a 401 so an expired session mid-SPA recovers instead of surfacing endless error toasts (SC9).

Over-engineering cuts (OE1–OE6, OE8, OE9, OE12; ~300 lines):

* Delegating closures in `services/deps.go`, `zitadel/deps.go`, `expiry/deps.go`, `drift/deps.go` collapsed to direct function references (tests still swap the vars).
* `expiry.Scheduler` and `drift.Scheduler` merged into one shared `services/periodic.Runner` (immediate run → ticker → panic recovery → Done-joins-in-flight-run); `expiry.Sweep` exported with the batch clamp.
* ui: dead `ResourceName` component deleted; Modal/Drawer share one `useDialogFocusTrap` hook; the four Name components drop the redundant force-render hack (the resolver context already re-renders consumers); dead exports removed (`getClientApiBase`, `getServerApiBase`, `fetchCatalog`, `formatProjectName`, `toastInfo`, `toastPromise`); no-op resolver `prefetch` removed.
* backend: dead `AssignUserToRole` deleted; `dedupProjectIDs` folded into `dedupeNonEmpty`.

Over-engineering cuts, 2026-07-29 re-audit (OE16–OE20, OE22; ~67 lines):

* Five dead `internal/db` repository helpers deleted: `ResolveDriftItem`, `CreateMappingRule`, `AddRoleToBundle`, `InsertExclusion`, `RemoveBundleFromUser`. All are refactor leftovers — when each write path grew an outbox/ledger trace the helper came back as `*AndEnqueue` and the traceless original was never removed. Beyond the dead weight, each was a mutation path that writes to Zitadel-adjacent state without the trace the drift-detection invariant depends on, so a future caller reaching for the shorter name would have silently created undetectable drift. Confirmed unreachable from every liveness root by OpenLore `verify_claim` plus a repo-wide grep.
* Empty `backend/pkg/` directory removed.
* OE21 (`sortedKeys` → `slices.Sorted(maps.Keys(…))`) declined: the stdlib form returns `nil` where the helper returns `[]`, and `zitadelByPair[k]` is allocated before its `RoleKeys` loop, so a zero-role Zitadel grant would emit `"role_keys": null` instead of `[]`.

Sync cuts pulled forward (OE10, OE11; ~14 lines):

* `ProvisioningIntent` drops 9 decode-only fields it never reads (the decoder ignores the rest of the payload); `GetShadowCredentialHash` returns just the hash and `ShadowCredentialHash.Algorithm` is gone — the sole caller already discarded it. Both are data-shape-only changes with no LDAP behavior, which is why they were taken despite the standing sync deferral.

## Explicitly Deferred

* Remaining `sync/` findings (SC2 LDAP `ErrorNetwork` classification — top priority when sync work resumes; SC6 same-UID ordering; SC7 drain context; OE7/OE13/OE14) — LDAP integration excluded from this remediation by decision. OE10/OE11 were pulled forward on 2026-07-29 (see above).
* OE15 (replace golang-migrate) — kept: the dirty-state guard on mid-migration failure is worth one dependency.

## Impact

* Backend: `handlers/router.go`, `handlers/access.go`, `handlers/webhook.go`, `handlers/webhook_translate.go`, `services/{deps,periodic,expiry,drift}`, `zitadel/{deps,orchestrator}.go`, `cmd/api/main.go`, `db/{drift,rules,bundles,exclusions}.go`; `backend/pkg/` removed.
* Sync: `internal/backend/{types,client}.go`, `internal/worker/worker.go` (+ their tests).
* UI: `lib/session.ts`, `lib/api-client.ts`, `middleware.ts`, `components/names/*`, `components/ui/{Modal,Drawer}.tsx`, `lib/{api,format,toast}.ts`, `lib/queries/useNameResolver.tsx`, `components/grants/GrantsClient.tsx`.
* No database schema changes. One behavioral contract change clients must know: member tokens now get 403 on grant/decision routes and cross-user reads (the UI proxy already enforced this, so the shipped UI is unaffected).
* `.env.example` documents optional `SESSION_SECRET`.
