# Feature Coverage Matrix (Planned vs Integrated)

Last updated: 2026-04-09

This document compares the **initially planned** MkAuth feature surface (as described in `openspec/changes/mkauth-core-architecture/design.md`) against what is **already integrated in the repo** (backend + UI). It is meant to be a durable “reality check” that future specs/design changes can reference.

Legend:
- **Integrated**: present end-to-end (API/storage + UI where applicable) or otherwise usable in the intended flow.
- **Partial**: present but stubbed, dev-only, or missing a major part of the intended design.
- **Not integrated**: described in the design, but no meaningful repo support found yet.

## Capability-level coverage (per change proposal)

| Capability | Planned (proposal/specs) | Integrated extent | Evidence (non-exhaustive) | Notes / gaps |
| --- | --- | --- | --- | --- |
| `demo-catalog` | Seeded demo users/projects/apps; dashboard renders without live Zitadel; claim metadata visible. | **Integrated** | `backend/internal/demo/*`, `backend/internal/services/views.go`, `backend/internal/seed/demo.go`, `ui/src/app/page.tsx` | Demo-first approach is working; remains separate from real Zitadel sync. |
| `access-governance` | Direct grants (optional expiry), requests + decisions, governance summary, lineage includes direct grants. | **Integrated** | `backend/db/migrations/000003_access_workflows.up.sql`, `backend/internal/handlers/router.go`, `backend/internal/db/repositories.go`, `ui/src/app/requests/page.tsx`, `ui/src/app/audit/page.tsx` | Uses demo identities/IDs; real admin identity and Zitadel enforcement is still Phase 2. |
| `topology-graph` | Graph API (nodes/edges), placeholder nodes for unknown refs, UI inspection + filtering, stable lanes. | **Integrated** | `backend/internal/handlers/router.go`, `backend/internal/services/views.go`, `ui/src/app/graph/page.tsx`, `openspec/changes/mkauth-core-architecture/specs/topology-graph/spec.md` | UI uses a custom SVG lane renderer (not React Flow), but meets the spec’d behaviors. |
| `application-claims`| Multi-project role selection, JWT claim shaping (array, CSV, space-delimited), simulator. | **Integrated** | `backend/internal/services/views.go`, `ui/src/app/applications/page.tsx`, `openspec/changes/mkauth-core-architecture/specs/application-claims/spec.md` | Simulation is fully powered by Redis-cached claims; shaping preserves legacy compatibility. |
| `user-management` | Unified view of all roles across projects, access lineage ("Why?"), bundle vs direct grants. | **Integrated** | `backend/internal/services/views.go`, `ui/src/app/users/*`, `openspec/changes/mkauth-core-architecture/specs/user-management/spec.md` | Lineage is visualized via role reasoning tree in UI; groupings by project context is the default view. |
| `role-management` | Create roles in Zitadel projects, clone/snapshot existing role metadata, global disambiguation labels. | **Not integrated** | `openspec/changes/mkauth-core-architecture/specs/role-management/spec.md` | New spec defined; requires backend implementation for role creation flow. |
| `automation-policies`| Define "Welcome" bundles for new accounts, automatic assignment triggers, global default status. | **Not integrated** | `openspec/changes/mkauth-core-architecture/specs/automation-policies/spec.md` | New spec defined; requires webhook/sync worker implementation. |
| `service-catalog` | Standard user portal; request access to services (apps); auto-mapping to bundles/roles. | **Partial** | `ui/src/app/page.tsx`, `ui/src/app/requests/page.tsx`, `ui/src/components/Sidebar.tsx`, `openspec/changes/mkauth-core-architecture/specs/service-catalog/spec.md` | A member-facing portal and self-service request page now exist behind demo session auth, but service requests still resolve to direct project/role picks rather than app-to-bundle automation. |
| `ldap-sync` | Bridge Zitadel (OIDC) to LLDAP (Hardware); prefix-flattening; shadow password management. | **Not integrated** | `openspec/changes/mkauth-core-architecture/specs/ldap-sync/spec.md` | Defines the translation layer; will use Go for implementation consistency. |
| `provisioning` | Event-driven identity sync engine; fault-tolerant LLDAP writes; secure credential rotation. | **Not integrated** | `openspec/changes/mkauth-core-architecture/specs/provisioning/spec.md` | Implemented as a Go-native concurrent worker using `go-ldap/v3`. |

## Architecture/design feature coverage (planned in design.md)

| Area | Planned feature (design.md) | Integrated extent | Evidence (non-exhaustive) | Notes / gaps |
| --- | --- | --- | --- | --- |
| **Control plane vs data plane** | “Slow & smart” control plane + “fast & dumb” data plane split. | **Integrated** | `backend/internal/handlers/router.go`, `backend/internal/handlers/action.go` | Control-plane calls into local DB + compilers; data-plane endpoint reads Redis and returns claims. |
| **Data plane auth** | Data plane secured via independent mechanism; performance-critical. | **Partial** | `backend/internal/handlers/action.go` | Endpoint is intentionally “wide open locally” per router comment; no production-grade verification mechanism documented yet. |
| **Bearer auth on routes** | Bearer token auth enforced on protected endpoints. | **Integrated** | `backend/internal/handlers/router.go` (`withAuth`), `docker-compose.yml` (`MKAUTH_API_KEY`) | API-key bearer auth is present; not per-user Zitadel admin auth. |
| **Redis cache** | Precompiled, flattened role payloads returned sub-millisecond. | **Integrated** | `backend/internal/cache/compiler.go`, `backend/internal/handlers/action.go`, `docker-compose.yml` (`redis`) | Compilation is present; cache keys are `mapping:<user>:<project>`. |
| **Webhook invalidation** | Webhook listener updates Redis on out-of-band Zitadel changes. | **Partial** | `backend/internal/handlers/webhook.go`, `backend/internal/cache/compiler.go` | Webhook route exists; full real-world Zitadel event validation + live sync appears Phase 2. |
| **Zitadel management API (M2M)** | Backend grants roles via Zitadel Management API using service account token. | **Partial** | `backend/internal/zitadel/client.go`, `backend/internal/zitadel/orchestrator.go` | Explicitly stubbed; operates in “local-policy-only mode” until credentials exist. |
| **Zero-trust backend vs UI** | Backend validates logged-in admin permissions before mutations. | **Partial** | `ui/src/middleware.ts`, `ui/src/app/api/proxy/[...path]/route.ts`, `ui/src/lib/session.ts` | The frontend now enforces demo session auth and hides admin-only routes from member sessions, but the backend still ultimately trusts the shared API key rather than a live per-admin identity token. |
| **Mapping rules (flat IF/THEN)** | Conditional mapping rules; avoid deep inheritance; deterministic propagation. | **Integrated** | `backend/db/migrations/000001_init_schema.up.sql` (`mapping_rules`), `backend/internal/handlers/router.go`, `backend/internal/db/repositories.go` | Implemented as mapping rule edges (source role → target role). |
| **Cycle detection** | Must block circular dependencies on rule creation. | **Integrated** | `backend/internal/db/validation.go` | DFS-based cycle detection on proposed inserts is present. |
| **Versioned policies** | Explicit versioning with rollbacks. | **Partial** | `backend/db/migrations/000001_init_schema.up.sql` (`mapping_rules.version`) | Version column exists, but repo does not show multi-version history, policy snapshots, or rollback primitives. |
| **Partial assignment failure handling** | Roll back if Zitadel API becomes unreachable mid-update. | **Not integrated** | (depends on real Zitadel writes) | Requires real management client + transactional write strategy. |
| **User-centric view** | Show user roles grouped by project/bundle; source vs derived; “why do they have X?” lineage. | **Integrated** | `backend/internal/services/views.go`, `ui/src/app/users/*`, `openspec/changes/mkauth-core-architecture/specs/user-management/spec.md` | The design’s “lineage” goal is implemented using a reasons-tracking model. |
| **Application-centric view** | Claim shaping rules + token simulator to preview JWT payload. | **Integrated** | `backend/internal/handlers/router.go` (`/applications/{id}/simulate`), `ui/src/app/applications/page.tsx`, `openspec/changes/mkauth-core-architecture/specs/application-claims/spec.md` | Sim is powered by cached Redis claims and app claim profiles. |
| **Project view** | Roles in a project + who has them + how bundles/policies use them. | **Integrated** | `backend/internal/services/views.go`, `ui/src/app/projects/page.tsx` | Backed by demo catalog and local DB-derived assignments. |
| **Bundle / policy view** | Bundle definitions + impacted users; policy management UI. | **Integrated** | `ui/src/app/bundles/page.tsx`, `ui/src/app/policies/page.tsx`, `backend/internal/handlers/router.go` | Bundles and mapping rules (“policies”) are both present as first-class views. |
| **Access requests / governance view** | Pending requests, expiring grants, cleanup hints; least-privilege summary. | **Integrated** | `ui/src/app/requests/page.tsx`, `ui/src/components/requests/*`, `ui/src/app/audit/page.tsx`, `backend/internal/services/views.go` | Cleanup hints exist in governance summary; member sessions now see a self-scoped request view while admins retain the approval queue. |
| **Audit logging** | Mandatory tracking of admin actions: who granted what and when. | **Integrated** | `backend/db/migrations/000001_init_schema.up.sql` (`audit_logs`), `backend/internal/db/repositories.go`, `ui/src/app/audit/page.tsx` | Audit stream exists; actor identity is currently demo/static in UI flows. |
| **Temporary roles auto-expire** | Semester/time-bound roles should expire automatically. | **Partial** | `backend/db/migrations/000003_access_workflows.up.sql` (expiry), `backend/internal/services/views.go` (expiring window) | Expiry is stored and surfaced; automatic enforcement/cleanup scheduler is not evidenced. |
| **Topology “God Mode”** | Macro graph view for exploration and debugging. | **Integrated** | `ui/src/app/graph/page.tsx`, `backend/internal/handlers/router.go` | Implemented with lane layout + node inspector. |
| **Command palette** | `⌘K` palette for power users. | **Not integrated** | (no references found in UI) | Could be added later without changing backend contracts. |
| **UI style (Linear/Stripe)** | Clean cards, shadows, whitespace; dark/light with accent colors. | **Partial** | `ui/src/app/*`, `ui/src/components/*` | Visual intent appears present; explicit theme toggle / mode persistence not evidenced. |
| **Separated FE/BE** | Decoupled containers for Frontend (Session) and Backend (Service). | **Integrated** | `docker-compose.yml`, `ui/src/app/api/proxy/[...path]/route.ts`, `design.md` | The UI and backend are already split into separate services, and the UI proxies backend calls over the internal container network. |
| **Deployment workflow** | Docker Compose in LXC, update script pulls + rebuilds stack. | **Integrated** | `docker-compose.yml`, `update.sh`, `install.sh` | `update.sh` performs `git pull origin main` + `docker compose build/up`. |

## Practical takeaways (what this matrix implies)

- The repo already reflects a **“v1 doc baseline”**: seeded demo catalog + governance workflows + topology graph + cache/action simulation.
- For a detailed timeline of future implementation steps, see the **[Development Roadmap](file:///Users/notkanishk/Documents/Mkrspc/Projects/MkAuth/openspec/changes/mkauth-core-architecture/ROADMAP.md)**.
- The main remaining “Phase 2” gap is **real Zitadel integration**: live OIDC sessions, M2M management writes, real webhook validation + syncing, and true per-admin backend authZ.
- Some design bullets are implemented by **equivalent behavior** (e.g., topology UI uses a lane-based SVG renderer rather than React Flow).
