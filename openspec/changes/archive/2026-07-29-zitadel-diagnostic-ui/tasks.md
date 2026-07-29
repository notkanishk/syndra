## Tasks

### Backend

- [x] Swap `/api/v1/zitadel/health` route from `withAPIKeyAuth` to `withOperatorAuth` in `backend/internal/handlers/router.go`. Update the surrounding comment to reflect the unified auth model.

### UI — Proxy

- [x] Add `DELETE` handler to `ui/src/app/api/proxy/[...path]/route.ts`, mirroring the existing `PUT` handler (same auth flow, same forwarding). Register it in the method-dispatch block at the bottom of the file.

### UI — Diagnostic page

- [x] Create `ui/src/app/zitadel/page.tsx` as a `"use client"` component.
- [x] TypeScript interfaces mirroring backend response shapes: `HealthResponse`, `Paginated<T>`, `ZitadelUser`, `ZitadelProject`, `ProjectRole`, `UserGrant`.
- [x] Section 1: Health — single button triggers `GET /api/proxy/zitadel/health`, renders status badge + latency + raw JSON under a `<details>` block. Uses a diagnostic fetcher that preserves structured non-2xx responses.
- [x] Section 2: Projects & Roles — project dropdown (auto-loaded), selecting a project loads its roles. Inline create (key + display + group), inline edit (display + group), inline delete with confirm.
- [x] Section 3: Users & Grants — user dropdown with email filter, selecting a user loads their grants. Inline assign (project + comma-separated role keys), inline edit (role keys), inline revoke with confirm.
- [x] Section 4: All Grants — one button triggers fetch; render as a compact table.
- [x] Shared helpers colocated in the same file: `apiGet`, `apiGetDiagnostic`, `apiSend(method, body)`, 3-second auto-clearing flash messages.

### UI — Nav

- [x] Add "Operations" section header + "Zitadel Diagnostics" link in the admin branch of `ui/src/components/Sidebar.tsx`. Place after "Governance" so destructive tools sit at the bottom of the nav.

### Verification

- [x] `cd backend && go test ./... && go vet ./...` — backend still green (196 tests, 11 packages).
- [x] `cd ui && bun run lint && bun run build` — frontend compiles (`/zitadel` at 4.16 kB).
- [x] `openspec validate zitadel-diagnostic-ui --strict` — passes with spec deltas.
- [x] Operator smoke-test procedure documented in [IMPLEMENTATION.md](IMPLEMENTATION.md) (post-deploy verification path).

### Docs

- [x] Write `openspec/changes/zitadel-diagnostic-ui/IMPLEMENTATION.md`.
- [x] Add the change to the Change Log table in `openspec/INDEX.md`.
- [x] Add spec deltas under `specs/` for all modified capabilities.
