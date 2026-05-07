# MkAuth Codebase Audit

> **🆕 Latest findings: [May 2026 Addendum](#addendum--may-2026-codebase-audit)** — read this first for current state. The sections below are an inventory snapshot from April 2026 (still useful as a reference map but predate Phase 5).

## Quick Navigation

**If you only have time for one thing → [TL;DR](#tldr)**

### Latest audit (May 2026 — current)
- [TL;DR](#tldr) — one-screen summary
- [Aim Alignment](#aim-alignment) — does the code match the makerspace mission?
- [Strengths Worth Preserving](#strengths-worth-preserving) — patterns to copy
- [Overengineering & Bloat](#findings-overengineering--bloat) — what to trim
- [Spec ↔ Implementation Drift](#findings-spec--implementation-drift) — where docs lie about code
- [Bugs & Correctness](#findings-bugs--correctness-concerns) — what's actually broken
- [Recommendations (Prioritized)](#recommendations-prioritized) — ordered action list
- [Inventory Delta Since April](#inventory-delta-since-april) — what's new (routes, env vars, modules, schema)

### Reference inventory (April 2026 — still valid as a map)
- [Project Overview](#project-overview) · [Architecture Map](#architecture-map)
- [API Endpoints (April inventory: 35 documented, header mislabeled "49"; today: 59)](#api-endpoints-49-routes) · [Database Schema](#database-schema-8-migrations)
- [Authentication Architecture](#authentication-architecture) · [Security Posture](#security-posture)
- [Test Coverage](#test-coverage) · [Dependency Inventory](#dependency-inventory)
- [Key Design Decisions](#key-design-decisions) · [Environment Variables](#environment-variables)
- [Local Development](#local-development-without-docker) · [Error Response Contract](#error-response-contract)
- [Redis Key Schema](#redis-key-schema) · [Middleware Stack](#middleware-stack)

> **Reading the inventory below:** route counts, test counts, env-var lists, and migration counts have grown since April — the May addendum has the deltas. Architectural shape, security model, and design decisions are unchanged.

---

# Inventory Snapshot — April 2026

Full audit of the MkAuth IAM orchestration platform covering architecture, security, type safety, testing, and operational readiness.

---

## Project Overview

MkAuth is an identity access management orchestration layer built on top of Zitadel. It provides role bundles, mapping rules, claim compilation, access request governance, and a topology visualizer for downstream applications.

**Stack**: Go 1.25+ backend (stdlib `net/http`) | Next.js 15 + React 19 frontend (Bun runtime) | PostgreSQL 15 | Redis 7

---

## Architecture Map

### Backend (`/backend`)

```
cmd/api/main.go                    — Entry point, startup, graceful shutdown
internal/
  auth/jwt.go                      — RS256 JWT validation with JWKS (1-hour cache)
  cache/
    compiler.go                    — Fixed-point role resolution + Redis caching
    deps.go                        — Injectable function vars for testability
  db/
    postgres.go                    — pgxpool connection + auto-migrations
    redis.go                       — go-redis/v9 client
    repositories.go                — All SQL queries (parameterized, no injection risk)
    validation.go                  — DFS cycle detection for mapping rules
  handlers/
    router.go                      — Route registration, CORS, auth, security headers, body limits
    deps.go                        — Injectable function vars for all handlers
    contracts.go                   — Strict JSON decoder (DisallowUnknownFields + trailing token check)
    adminctx.go                    — Admin user ID context propagation for audit
    bundles.go                     — Bundle CRUD handlers
    rules.go                       — Mapping rule handlers
    access.go                      — Direct grants, access requests, governance
    views.go                       — Catalog, user list, application views, simulation
    webhook.go                     — Zitadel webhook intake (6 event types, HMAC verified, dedup)
    webhook_events.go              — Operator endpoint for webhook event history
    roles.go                       — Role creation (with clone) and global catalog
    intents.go                     — Provisioning intent handlers (operator view + sync service API)
    action.go                      — Data plane claim injection (50ms Redis timeout)
    health.go                      — /healthz endpoint (Postgres ping)
  models/models.go                 — All domain types (Role, CatalogRole, bundles, grants, topology, etc.)
  services/
    onboarding.go                  — Backend-owned welcome bundle assignment
    roles.go                       — Role creation service (clone resolution, Zitadel propagation, catalog)
    lldap.go                       — LLDAP group name flattening ({project}_{role} convention)
    provisioning.go                — Provisioning intent emission (webhook → LLDAP sync bridge)
    views.go                       — Governance summary, user access views, topology
    deps.go                        — Injectable function vars for services
  seed/                            — Demo data seeding
  demo/                            — Static demo catalog (users, projects, apps)
  zitadel/
    client.go                      — M2M management client (direct HTTP, JWT profile auth)
    token.go                       — Token lifecycle (RS256 assertion, caching, thread-safe refresh)
    keyfile.go                     — Service account key loader (PKCS1/PKCS8 PEM)
    orchestrator.go                — Role writeback orchestration (mapping rules, grants)
    deps.go                        — Injectable function vars for testability
db/migrations/                     — 9 sequential SQL migrations
```

### Frontend (`/ui`)

```
src/
  app/
    page.tsx                       — Dashboard (admin overview / user portal)
    bundles/page.tsx               — Bundle management (client component)
    policies/page.tsx              — Mapping rules (client component)
    users/page.tsx                 — User management + access views
    projects/page.tsx              — Project summaries
    applications/page.tsx          — Application views + claim simulation
    audit/page.tsx                 — Audit log + governance summary
    graph/page.tsx                 — Topology visualizer (SVG)
    requests/page.tsx              — Access request management
    auth/
      login/route.ts               — Demo login POST handler
      zitadel/route.ts             — PKCE flow initiation
      callback/route.ts            — OIDC callback + session creation
      logout/route.ts              — Session destruction
    api/proxy/[...path]/route.ts   — Client-side API proxy to backend
  lib/
    api.ts                         — Server-side typed fetchers (generic <T>)
    session.ts                     — Cookie-based session (demo | oidc union)
    oidc.ts                        — PKCE crypto, token exchange, claim parsing
    types.ts                       — Shared API response types
  components/
    Sidebar.tsx                    — Navigation sidebar
    CreateRuleForm.tsx             — Mapping rule creation form
    requests/                      — Admin and user request views
    ui/                            — Badge, Card primitives
  middleware.ts                    — Route protection (admin-only paths)
```

---

## API Endpoints (49 routes)

### Control Plane (all require auth via `withUserAuth`)

| Method | Path | Handler | Purpose |
|--------|------|---------|---------|
| GET | `/api/v1/bundles` | `handleGetBundles` | List all bundles |
| POST | `/api/v1/bundles` | `handleCreateBundle` | Create bundle |
| GET | `/api/v1/bundles/{id}/roles` | `handleGetBundleRoles` | Bundle's role mappings |
| GET | `/api/v1/bundles/{id}/impact` | `handleGetBundleImpact` | Bundle impact analysis |
| POST | `/api/v1/bundles/{id}/roles` | `handleAddRoleToBundle` | Map role to bundle |
| GET | `/api/v1/catalog` | `handleGetCatalog` | Full resource catalog |
| GET | `/api/v1/users` | `handleGetUsers` | User list with summaries |
| GET | `/api/v1/users/{id}/grants` | `handleGetUserDirectGrants` | User's direct role grants |
| POST | `/api/v1/users/{id}/grants` | `handleUpsertUserDirectGrant` | Upsert direct grant |
| GET | `/api/v1/users/{id}/bundles` | `handleGetUserBundles` | User's bundle assignments |
| POST | `/api/v1/users/{id}/bundles` | `handleAssignBundleToUser` | Assign bundle to user |
| GET | `/api/v1/users/{id}/access` | `handleGetUserAccess` | Full access view |
| GET | `/api/v1/applications` | `handleGetApplications` | Application list |
| GET | `/api/v1/applications/{id}/simulate` | `handleSimulateApplication` | Simulate claims for user |
| GET | `/api/v1/projects` | `handleGetProjects` | Project summaries |
| GET | `/api/v1/topology` | `handleGetTopology` | Role graph for visualizer |
| GET | `/api/v1/rules/mapping` | `handleGetMappingRules` | List mapping rules |
| POST | `/api/v1/rules/mapping` | `handleCreateMappingRule` | Create rule (with cycle check) |
| PUT | `/api/v1/rules/mapping/{id}` | `handleUpdateMappingRule` | Update rule (version increment) |
| GET | `/api/v1/audit` | `handleGetAuditLogs` | Audit log entries |
| GET | `/api/v1/requests` | `handleGetAccessRequests` | Access requests |
| POST | `/api/v1/requests` | `handleCreateAccessRequest` | Submit access request |
| POST | `/api/v1/requests/{id}/decision` | `handleResolveAccessRequest` | Approve/reject request |
| GET | `/api/v1/governance/summary` | `handleGetGovernanceSummary` | Pending + expiring items |
| POST | `/api/v1/roles` | `handleCreateRole` | Create role (with optional clone) |
| GET | `/api/v1/roles` | `handleGetGlobalRoleCatalog` | Global role catalog |
| GET | `/api/v1/onboarding/triggers` | `handleGetOnboardingTriggers` | Onboarding trigger log |
| GET | `/api/v1/webhook/events` | `handleGetWebhookEvents` | Webhook event history |
| GET | `/api/v1/intents` | `handleGetProvisioningIntents` | Provisioning intent history |

### Sync Service API (API-key auth)

| Method | Path | Handler | Purpose |
|--------|------|---------|---------|
| POST | `/api/v1/intents/claim` | `handleClaimIntents` | Atomic claim pending intents (FOR UPDATE SKIP LOCKED) |
| POST | `/api/v1/intents/{id}/complete` | `handleCompleteIntent` | Mark intent completed |
| POST | `/api/v1/intents/{id}/fail` | `handleFailIntent` | Record intent failure |

### Data Plane (own auth mechanisms)

| Method | Path | Auth | Purpose |
|--------|------|------|---------|
| POST | `/api/webhooks/zitadel` | HMAC-SHA256 | Zitadel event intake |
| POST | `/api/action/inject` | Redis timeout | Claim injection for Zitadel Actions |

### Infrastructure

| Method | Path | Auth | Purpose |
|--------|------|------|---------|
| GET | `/healthz` | None | Postgres health check |

---

## Database Schema (8 migrations)

### Tables

| Table | Purpose | Key Constraints |
|-------|---------|-----------------|
| `bundles` | Role groupings | UUID PK, unique name, non-blank check |
| `bundle_roles` | Bundle-to-role mappings | Composite unique (bundle, project, role), cascade delete |
| `user_bundle_assignments` | User-to-bundle assignments | Composite unique (user, bundle) |
| `mapping_rules` | IF source THEN add target | Composite unique on logic tuple, no self-edges, version > 0 |
| `claim_profiles` | JWT claim formatting per project | Format enum (array/csv/space_delimited) |
| `direct_role_grants` | Time-based role assignments | Expiry-after-create check, non-blank fields |
| `access_requests` | Self-service permission requests | Status enum (pending/approved/rejected), positive duration |
| `audit_logs` | Who did what when | Actor, target, action, resource_id, timestamp |
| `claim_failure_mode` | Per-project degraded behavior | Enum (fail_closed/minimal_safe) |
| `onboarding_triggers` | Idempotent welcome bundle log | Unique idempotency key |
| `webhook_events` | Webhook event persistence/dedup | Unique idempotency key, status enum |
| `roles` | MkAuth-managed role metadata | Unique (project, role_key), clone provenance |
| `provisioning_intents` | LLDAP sync intent queue | Unique idempotency key, four-state status machine |

### Migrations

1. `000001_init_schema` — Core tables (bundles, roles, rules, claims, audit)
2. `000002_user_bundles` — User-bundle assignments
3. `000003_access_workflows` — Direct grants + access requests
4. `000004_contract_hardening` — Non-blank checks, enum constraints, duration validation, no self-edges
5. `000005_security_boundary` — Claim failure modes + onboarding triggers
6. `000006_governance_integrity` — Bundle name checks, expiry ordering
7. `000007_webhook_events` — Webhook event persistence and deduplication
8. `000008_roles` — MkAuth-managed role metadata with clone provenance
9. `000009_provisioning_intents` — LLDAP sync intent queue with four-state status machine

---

## Authentication Architecture

### Three-Layer Auth Model

```
Zitadel (IdP) ──PKCE──> Next.js Frontend ──Bearer JWT──> Go Backend
                              │                              │
                         mkauth_session              withUserAuth middleware
                         (httpOnly cookie)           (RS256 JWKS validation)
```

**Frontend (OIDC mode)**:
1. `/auth/zitadel` initiates PKCE flow (S256 code challenge)
2. `/auth/callback` exchanges authorization code for tokens
3. Session cookie stores access token, user info, expiry (discriminated union: `demo | oidc`)
4. Middleware enforces admin-only paths, validates session expiry
5. SSR and proxy forward `Authorization: Bearer <token>` to backend

**Backend (production mode)**:
1. `withUserAuth` extracts Bearer token from Authorization header
2. Validates RS256 signature via Zitadel JWKS endpoint (cached 1 hour)
3. Checks issuer (`https://{domain}`), audience, and expiry
4. Extracts subject (admin user ID) into request context for audit

**Backend (local-dev mode)**:
1. When `ZITADEL_DOMAIN` is unset, falls back to shared API key
2. Constant-time comparison via `crypto/subtle`
3. No admin user ID in context (anonymous audit)

**Data Plane (separate auth)**:
- Webhooks: HMAC-SHA256 over `(timestamp + "\n" + body)`, 5-minute freshness
- Action injection: Redis timeout (50ms), no shared secrets

### Session Cookie

- Name: `mkauth_session`
- Encoding: base64url JSON
- Flags: `httpOnly: true`, `sameSite: lax`, `secure: true` (OIDC mode)
- Payload types: `DemoSessionCookie` (userId + role) or `OidcSessionCookie` (accessToken + userId + role + name + email + expiresAt)
- Legacy compatibility: cookies without `type` field treated as demo

---

## Security Posture

### Strengths

| Area | Implementation |
|------|---------------|
| SQL injection | Zero risk — 100% parameterized queries via pgx (`$1`, `$2`) |
| Input validation | `DisallowUnknownFields()` + trailing token check on all mutations |
| JWT validation | RS256 + JWKS, issuer/audience/expiry checks, key rotation support |
| Webhook integrity | HMAC-SHA256 with timestamp in signed input, freshness window |
| Session security | HttpOnly + SameSite cookies, expiry validation before use |
| PKCE | S256 challenge, state parameter, 300-second TTL on challenge cookies |
| Audit trail | Every mutation logged with actor, target, action, resource |
| Idempotency | DB unique constraints + `ON CONFLICT DO NOTHING` on assignments |
| Cycle detection | DFS on mapping rule graph, validated before insert |
| Degraded modes | Explicit `fail_closed` / `minimal_safe` per project, no implicit fallback |
| CORS | Configurable origin (env-based), credentials enabled |
| Security headers | `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy` |
| Timing attacks | API key comparison uses `crypto/subtle.ConstantTimeCompare` |
| Body limits | 1 MB `MaxBytesReader` on all endpoints |

### Known Gaps (Future Work)

| Area | Status | Notes |
|------|--------|-------|
| Rate limiting | Not implemented | Needs design decision: in-process vs Redis-backed |
| CSP headers | Not on API | Frontend concern — handle in Next.js or reverse proxy |
| HSTS | Not on API | Requires TLS termination at reverse proxy level |
| OpenAPI spec | Not generated | API is internal to Next.js proxy; no external consumers |
| Structured logging | Using `log.Printf` | slog migration would touch every file for marginal gain |
| Pagination | Not implemented | No endpoint currently returns enough data to warrant it |

---

## Test Coverage

### Backend (Go)

| Package | Test File | Tests | Coverage Area |
|---------|-----------|-------|---------------|
| `internal/auth` | `jwt_test.go` | ~8 | Valid token, expired, wrong audience/issuer, tampered signature, malformed JWT, unknown kid |
| `internal/db` | `validation_test.go` | 3 | Cycle detection: disconnected graph, direct cycle, indirect cycle |
| `internal/handlers` | `bundles_test.go` | ~13 | Empty name, whitespace, unknown fields, idempotent assignment, audit attribution |
| `internal/handlers` | `rules_test.go` | ~8 | Missing fields, unknown fields, self-edge, cycle detection via handler, version increment |
| `internal/handlers` | `access_flow_test.go` | ~8 | Expiry math, zero-duration nil pointer, cache rebuild, access request persistence, idempotency 409 |
| `internal/handlers` | `webhook_test.go` | ~16 | HMAC signature validation, timestamp freshness, event dispatch, dedup, provisioning intent emission |
| `internal/handlers` | `intents_test.go` | ~8 | Intent operator view, pending poll, acknowledge/complete/fail transitions |
| `internal/handlers` | `action_test.go` | ~5 | Cache miss fail_closed, cache miss minimal_safe, malformed cache data, DB outage defaults |
| `internal/handlers` | `contracts_test.go` | ~4 | Strict decoding, trailing tokens, unknown fields |
| `internal/cache` | `compiler_test.go` | 5 | Empty grants, direct grants, transitive rules, bundle roles, fixed-point termination |
| `internal/services` | `onboarding_test.go` | ~6 | Trigger insertion, bundle assignment, audit logging, idempotency |
| `internal/services` | `lldap_test.go` | 5 | Group flattening: basic, lowercase, mixed case, underscores, multiple spaces |
| `internal/services` | `provisioning_test.go` | 5 | Intent emission: success, duplicate, unknown project, DB failure, audit failure |
| `internal/services` | `views_test.go` | ~9 | Nil-safe collections, pending count, unused bundle hints, source vs derived roles |

**Total: 170 backend tests, all passing.**

### Frontend (Vitest)

| File | Tests | Coverage Area |
|------|-------|---------------|
| `lib/__tests__/session.test.ts` | 6 | Demo user catalog, lookup, encode roundtrip, OIDC payload encoding |

**Total: 6 frontend tests, all passing.**

### Testing Patterns

- **Injectable deps**: Module-level function vars in `deps.go` files, swapped in tests via `t.Cleanup()`
- **Table-driven tests**: Used in validation and JWT test suites
- **Mock capture**: Closures capture function call arguments for assertion
- **Reset pattern**: `resetXxxDeps(t)` saves and restores all injectable vars

---

## Dependency Inventory

### Backend (Go)

| Module | Version | Purpose |
|--------|---------|---------|
| `github.com/golang-jwt/jwt/v5` | v5.3.1 | JWT parsing and validation |
| `github.com/jackc/pgx/v5` | v5.9.1 | PostgreSQL driver with connection pooling |
| `github.com/redis/go-redis/v9` | v9.18.0 | Redis client |
| `github.com/golang-migrate/migrate/v4` | v4.19.1 | Database migrations |

No web framework — uses stdlib `net/http` with Go 1.22+ routing patterns.

### Frontend (npm/Bun)

| Package | Version | Purpose |
|---------|---------|---------|
| `next` | ^15 | React framework |
| `react` / `react-dom` | ^19 | UI library |
| `tailwindcss` | ^4 | Styling |
| `typescript` | ^5 | Type checking |
| `vitest` | ^4.1.4 | Test runner |

No external OAuth/OIDC library — PKCE implemented natively in `oidc.ts`.

---

## Key Design Decisions

1. **No web framework**: Stdlib `net/http` with Go 1.22+ method routing. Keeps the dependency surface minimal for a security-sensitive system.

2. **Injectable deps over interfaces**: Function vars in `deps.go` files instead of interface-based DI. Simpler, no container, same testability. Established pattern across handlers, services, and cache packages.

3. **Discriminated union sessions**: `demo | oidc` type field in the session cookie. Demo mode works without any external IdP. Legacy cookies (no type field) gracefully degrade to demo.

4. **Claim compilation at write time**: Roles are compiled into Redis when grants change, not at token-issuance time. The data plane reads pre-computed claims with a 50ms timeout. This keeps Zitadel Actions latency-safe.

5. **Explicit degraded modes**: `fail_closed` (empty claims) or `minimal_safe` (configured safe set) per project. No implicit fallback or retry — the operator explicitly chooses the failure behavior.

6. **Backend-owned mutations**: Webhooks are intake-only. All business mutations (bundle assignment, grant creation, audit logging) happen in the Go backend, not in Zitadel-hosted logic.

7. **Cycle detection before insert**: Mapping rules are validated via DFS before database insertion. The fixed-point resolver in the cache compiler is bounded by rule count, so even if a cycle somehow entered the DB, compilation would terminate.

---

## Environment Variables

| Variable | Used By | Default | Purpose |
|----------|---------|---------|---------|
| `DB_DSN` | Backend | (required) | PostgreSQL connection string |
| `REDIS_URL` | Backend | (required) | Redis host:port |
| `PORT` | Backend | `8080` | HTTP listen port |
| `MKAUTH_API_KEY` | Both | (required in dev) | Shared secret for local-dev auth |
| `MKAUTH_SEED_DEMO` | Backend | `false` | Seed demo data on startup |
| `CORS_ORIGIN` | Backend | `http://localhost:3000` | Allowed CORS origin |
| `MIGRATION_PATH` | Backend | `file:///app/db/migrations` | Path to migration files |
| `ZITADEL_DOMAIN` | Both | (unset = demo mode) | Zitadel instance domain |
| `ZITADEL_AUDIENCE` | Backend | — | JWT audience for validation |
| `ZITADEL_CLIENT_ID` | Frontend | — | PKCE client ID |
| `ZITADEL_ADMIN_ROLE_KEY` | Frontend | — | Role key for admin detection |
| `ZITADEL_WEBHOOK_SECRET` | Backend | — | Webhook HMAC signing key |
| `NEXT_PUBLIC_API_URL` | Frontend | — | Client-side backend URL |
| `BACKEND_URL` | Frontend | `http://backend:8080` | SSR backend URL |

---

## Local Development (Without Docker)

Prerequisites: Go 1.25+, Bun 1.x, PostgreSQL 15+, Redis 7+

```bash
# 1. Create database
createdb mkauthdb
psql mkauthdb -c "CREATE USER mkauth WITH PASSWORD 'mkauth_secure_password';"
psql mkauthdb -c "GRANT ALL ON DATABASE mkauthdb TO mkauth;"

# 2. Configure environment
cp .env.example .env
# Edit .env if needed (defaults work with local Postgres/Redis)

# 3. Run
make dev          # Starts backend (:8080) and frontend (:3000) in parallel

# 4. Verify
curl localhost:8080/healthz     # {"status":"ok"}
make test                        # All backend + frontend tests
make lint                        # go vet + eslint
```

---

## Error Response Contract

All API errors follow this shape:

```json
{
  "error": "ERROR_CODE",
  "message": "Human-readable description",
  "details": { "field": "specific error" }
}
```

Error codes: `VALIDATION_FAILED`, `UNAUTHORIZED`, `DB_ERROR`, `WEBHOOK_UNAUTHORIZED`, `WEBHOOK_STALE`, `ORCHESTRATOR_FAULT`, `SERVER_ERROR`, `VIEW_ERROR`, `ALREADY_RESOLVED`.

---

## Redis Key Schema

| Pattern | TTL | Content |
|---------|-----|---------|
| `mapping:{userId}:{projectId}` | 24h | Compiled claims JSON (`roles`, `user_id`, `project_id`, `compiled_at`, `source`) |

---

## Middleware Stack

```
Request → withMaxBody (1MB) → withSecurityHeaders → ServeMux routing
                                                       │
                                     ┌─────────────────┼──────────────────┐
                                     │                 │                  │
                              withCORS +        withCORS +          withCORS only
                              withUserAuth      withAPIKeyAuth     (data plane)
                              (control plane)   (sync service)
                                     │                 │                  │
                              JWT/API key        API key only      HMAC/Redis
                              validation         validation        verification
                                     │                 │                  │
                                  Handler           Handler           Handler
```

---
---

# Addendum — May 2026 Codebase Audit

> The sections above (Project Overview through Middleware Stack) are the April 2026 inventory snapshot. They remain accurate as a reference map but predate the Phase 5 changes (live Zitadel directory, grant expiration scheduler, dashboard UX elevation, event-trigger propagation, Obsidian Clarity redesign, vault/shadow-credential surface). This addendum re-baselines against the current repo and answers: **Is MkAuth elegant, or has it grown into something larger than its purpose?**
>
> **Aim** per `CLAUDE.md` and `mkauth-core-architecture/design.md`: ease role-based control of digital and physical resources for an *academic makerspace* — one operator, a few hundred members, deployed as Docker Compose inside a single Proxmox LXC. Zitadel is the source of truth, MkAuth is the policy/orchestration layer.

## TL;DR

**Healthy:** the architecture is right-sized for the aim. Three planes (control / data / bridge), idempotent intake, pre-compiled claims, isolated sync worker — none of these are oversized for a single-LXC IAM bridge. The contract hardening, JWT/HMAC discipline, and audit trail are genuinely production-grade for the few-hundred-user scale.

**Drifting:** Phase 5 added six concurrent change-streams (`grant-expiration-scheduler`, `live-zitadel-data-source`, `live-directory-identity-completeness`, `dashboard-ux-elevation`, `obsidian-clarity-redesign`, `zitadel-event-trigger-propagation`). Each is fine on its own; together they have left:

- Two coexisting CSS design systems in `ui/` (in-progress migration, but unfinished)
- A 797-line `services/views.go` that re-derives the same user-role map four ways per request
- A reconciliation/discovery surface (`/api/v1/zitadel/*`) sized for SaaS scale that re-introduces the divergent-source-of-truth problem MkAuth was designed to prevent
- An undocumented sync-service env surface that breaks the "1-command install" promise
- One destructive scratchpad (`backend/cmd/test/main.go`) and one in-flight UI redesign that hasn't finished

**Bug-shaped concerns (must-fix before more features land):** dev-mode action signature bypass in production config, OIDC member dashboard rendering blank metadata, vault self-attribution in dev mode, missing-welcome-bundle silent fallback. None are CVE-class but each is an operator-trust failure.

---

## Aim Alignment

| Aim line (design.md) | Reality |
|---|---|
| "Zitadel is the absolute source of truth" | Held in webhook + claim paths. **Violated** by `discovery.go:218-282` operator endpoints that mutate Zitadel grants directly, bypassing MkAuth's grant table. |
| "MkAuth Backend is the single mutation authority" | True for control-plane bundle/grant/rule mutations. Live-Directory Identity Completeness adds direct read paths through Zitadel that are correct, but the parallel write paths in the `/zitadel/*` discovery routes contradict the doctrine. |
| "Admin-console first, member portal second" | Holds. Member surface is small and well-bounded (`api/proxy/[...path]/route.ts:11-25` whitelists exact member paths). |
| "Backend is intake-only for webhooks; mutations are orchestrated server-side" | Holds end-to-end since `zitadel-event-trigger-propagation` shipped. |
| "Sync service is a private worker — no exposed ports" | Holds. Compose has no `ports:` block on sync; `Dockerfile` has no `EXPOSE`. |
| "1-command install/update via `update.sh`" | **Broken.** `LLDAP_*`, `SYNC_*`, `MKAUTH_EXTERNAL_URL`, `ZITADEL_M2M_TOKEN` are all required at runtime but absent from `.env.example`. `scripts/smoke-test-lxc.sh` only works in dev mode. |
| "Linear/Stripe aesthetic, dark/light modes, vibrant accent colors, ⌘K palette" | Theme toggle landed; ⌘K **not implemented**; dark mode is uniformly correct only on Material-token surfaces, broken on legacy-palette surfaces. |
| "Few-hundred users, single LXC" | Most of the system. **Mismatch:** `reconciliationSafetyCap = 10_000`, paginated grant fetchers up to `limit=1000`, `useNameResolver` rAF-batched 409-line resolver, `EXPIRY_SCHEDULER_*` framing for "N>1 backend replicas" — all designed for scale that does not exist here. |

Verdict: the *architecture* matches the aim. Several *implementations* of features quietly assume a SaaS-shaped audience.

---

## Strengths Worth Preserving

These are the patterns to copy when adding the next feature:

1. **Webhook intake pipeline** (`backend/internal/handlers/webhook.go` + `webhook_translate.go` + `webhook_translate_enrich.go`) — single responsibility per file, centralized self-mutation guard, distinguishes Zitadel-origin (200 on enrichment-miss) from internal-shape callers (strict 400). Cleanest pipeline in the repo.
2. **DB-enforced idempotency** — `ON CONFLICT DO NOTHING … RETURNING id` returning `(id, inserted, err)` everywhere (`InsertOnboardingTrigger`, `InsertWebhookEvent`, `InsertProvisioningIntent`). Callers cannot accidentally double-process.
3. **Expiry sweep `processUser`** (`services/expiry/sweep.go:62-122`) — re-validates `expires_at` inside the DELETE, drives every side-effect off `RETURNING`, audits before cascading. Comments document the race they solved. Model for any future scheduled job.
4. **Per-project degraded modes** (`handlers/action.go:73-105`, `repositories.go:GetClaimFailureMode`) — explicit `fail_closed` / `minimal_safe` instead of implicit retry/fallback. Single-project flat keys, multi-project namespaced. Crisp.
5. **Auth posture in the UI** (`lib/oidc.ts`, `auth/callback/route.ts`, `auth/zitadel/route.ts`) — PKCE S256 + state, PKCE cookie scoped to `/auth/callback` with 5-min TTL, OIDC session capped at 12h with 30s skew. Demo cookies actively rejected when `ZITADEL_DOMAIN` is set in *both* `lib/session.ts:217` and `middleware.ts`. No client bundle ever sees `accessToken`.
6. **Sync service shape** (`sync/`) — ~230 LOC of worker logic plus thin LDAP wrapper. No business logic, no policy, no exposed ports. `UIDLocker` is small and correct: per-UID serialization with `O(active)` cleanup. Exactly what a Bridge-Plane executor should be.
7. **`AdminDashboard.tsx`** (`ui/src/components/dashboard/AdminDashboard.tsx`) — tight scope (stat grid + recent activity + intents pulse), all UIDs go through `<UserName/>`, all states (skeleton, empty, populated) handled. Canonical pattern for new admin views.
8. **Idempotent `register.sh`** (`zitadel/actions/register.sh`) — search-by-name → upsert → bind-by-condition; `--purge` correctly orders unbind-then-DELETE so Zitadel doesn't refuse target deletion.
9. **`paginatedResponse` envelope** (`handlers/discovery.go:37-58`) — small, consistent across all Zitadel discovery handlers. Handler code reads identically whether listing users, projects, roles, or grants.

---

## Findings: Overengineering & Bloat

Severity: HIGH = remove or fix soon; MED = costs reading time and onboarding; LOW = harmless rough edge.

### Backend

| # | Severity | Where | Finding |
|---|---|---|---|
| B1 | **HIGH** | `backend/cmd/test/main.go` | Destructive dev scratchpad. `db.PG.Exec(ctx, "DELETE FROM mapping_rules")` runs on every `go run ./cmd/test`. Not referenced from Makefile, Dockerfile, or any script. **Delete the file.** |
| B2 | **HIGH** | `backend/internal/handlers/reconciliation.go:29` + `:113-138` | `reconciliationSafetyCap = 10_000` and the paginated `fetchAllZitadelGrants` loop are sized for a tenant that doesn't exist. A makerspace with ~200 members will plateau under 2k grants. The `OnlyInZitadel` bucket also lists every mapping-rule-derived grant ("expected") on every call, drowning the diff's signal. |
| B3 | **HIGH** | `backend/internal/services/views.go` (797 lines) | `ListUsers` (42-85), `ListApplications` (163-199), `ListProjects` (253-342), and `Topology` (430-623) each call `collectUserRoles(ctx, user.ID)` inside per-user loops. Each call fans out to `directGrants + bundles + bundleRoles + activeRules`. For N users this is O(N × bundles × rules + N²) DB+directory hits per render. Compute one `(user→roles)` map once per request and feed it to all four. The 797-line file is a direct symptom; the refactor likely halves it. |
| B4 | **MED** | `backend/internal/handlers/discovery.go:218-282` + `zitadel/client.go:336-373` | A complete CRUD for Zitadel grants and roles is exposed (POST/PUT/DELETE on `/api/v1/zitadel/users/{id}/grants[/{grantId}]`). Spec says "MkAuth Backend is the single mutation authority"; this endpoint family is the operator's escape hatch back to direct-Zitadel mutation, manufacturing the divergence reconciliation later detects. Either remove it, or amend the doctrine. |
| B5 | **MED** | `backend/internal/db/repositories.go` (1303 lines) | God-file: bundles, mapping rules, audit, direct grants, access requests, claim profiles, onboarding, webhook events, grant index, role management, provisioning intents, shadow vault — all in one file. Splitting into `repositories/{bundles,grants,rules,webhooks,vault,intents,roles}.go` would let `git blame` and code review work per-domain. Functions themselves are good; the seam is wrong. |
| B6 | **MED** | `webhook.go:77-79` + `webhook.go:104-109` | Two silent defaults: missing `event_type` becomes `grant_added`; Zitadel-shape with missing `source_project` returns 200 with "no dispatch." The combination can mask broken triggers (a malformed event passes both filters and disappears silently). |
| B7 | **LOW** | `handlers/vault.go:145-147` | `isComplexityError` does string-prefix sniffing on `err.Error()[:20] == "password complexity:"`, guarded by `len > 20`. The guard prevents a panic, but the contract is still brittle: an error whose message is exactly the 20-byte literal `"password complexity:"` (with no trailing detail) silently fails the match. A typed sentinel is one line. |
| B8 | **LOW** | `webhook.go` + `zitadel_grant_lookup.go` | `grantLookupMaxPages = 100` for a 200-user org is theatrical. Drop to 10. |

### UI

| # | Severity | Where | Finding |
|---|---|---|---|
| U1 | **HIGH** | `ui/src/lib/queries/useNameResolver.tsx` (409 lines) | Tick-batched UID→name resolver with rAF flushing, dual `resolved`/`attempted` state, React Query coalescing, tri-state `ResolveResult`, plus a footgun warning about an "infinite rAF/setState loop" if invariants slip. For ~few-hundred-user scale, fetch the catalog once at provider mount into a `Map` and resolve synchronously. The complex path is for thousands of unresolved UIDs across views — workload that doesn't exist. |
| U2 | **HIGH** | Multiple `ui/src/**/*.tsx` | **Two coexisting CSS design systems.** Most pages use Material-style tokens (`bg-surface-container-low`, `text-on-surface-variant`, `border-outline-variant`) introduced by `obsidian-clarity-redesign` Stage 1. But `Sidebar.tsx`, `ThemeToggle.tsx`, `RequestAccessButton.tsx`, `ErrorBoundary.tsx`, parts of `app/zitadel/page.tsx` still use the older `bg-surface`/`text-foreground`/`text-muted`/`bg-primary` palette. Dark mode is half-broken because of this. *Note:* this is an in-progress migration (`obsidian-clarity-redesign` is not in `archive/`), not negligence — but it needs a finish date and a tasks-list cleanup. |
| U3 | **MED** | `ui/src/components/SidebarNav.tsx:43` + `useGovernanceSummary` | Sidebar fetches `/governance/summary` directly with raw `fetch()` to populate badge counts; `AdminDashboard` fetches the same endpoint via React Query a second time. Same data, two cache stores, no dedup. |
| U4 | **MED** | `ui/src/lib/api.ts` + `ui/src/lib/types.ts` | Mostly dead in OIDC mode. `fetchBundles`, `fetchMappingRules`, `fetchProjects`, `fetchAudit` are exported but only `fetchApplications` and `fetchSystemMode` are called from pages. All other surfaces moved to `lib/queries/*` via `api-client.ts`. Same for `lib/types.ts` — duplicated by per-resource shapes inside each `useX.ts`. Delete or consolidate. |
| U5 | **MED** | `ui/src/app/zitadel/page.tsx` (947 lines) | Largest file in the UI. Three bespoke fetch helpers (`apiGet`/`apiSend`/`apiGetDiagnostic`) where the rest of the codebase uses `request<T>` from `api-client.ts`. The "diagnostic" variant (preserve non-2xx body) is legitimate but should be a flag on `request<T>`, not a one-off. Splits cleanly into 4 section components. |
| U6 | **LOW** | `ui/src/lib/session.ts:206` | `nameToAvatar(payload.name)` when `name` is missing returns "ZI" or initials of a UUID. Use `userId`/`email` as a fallback before the avatar helper. |

### Sync / Zitadel / Scripts

| # | Severity | Where | Finding |
|---|---|---|---|
| S1 | **MED** | `register.sh:77-91`, `rotate.sh:63-77`, `smoke-test-action-v2.sh:28-43`, `smoke-test-event-listener.sh:28-43` | Identical 12-line `_ENV_FILE` loader duplicated four times. Extract to `scripts/lib/load-env.sh` and `source` it. |
| S2 | **MED** | `register.sh:154-189` + `rotate.sh:131-163` | Identical `zitadel_api()` helper duplicated, including the 401/403 PERMISSIONS.md hint block. Same fix: shared lib. |
| S3 | **LOW** | `zitadel/actions/{README,EVENTS,PERMISSIONS,SIGNING_KEY}.md` | 605 lines of documentation for one Action target group. `PERMISSIONS.md` and `SIGNING_KEY.md` content already echoes inline in the `zitadel_api()` 401 hint and the `rotate.sh` output. Fold into one `README.md`. |
| S4 | **LOW** | `sync/internal/config/config.go:42-43` | `RetryAttempts: 3` and `RetryBackoff: 1s` are env-shaped fields without env wiring — operator can't tune. Either expose as `SYNC_RETRY_*` env or remove the fields and inline the constants. |

---

## Findings: Spec ↔ Implementation Drift

Items where the spec, the design doc, or `feature-coverage.md` does not match what the code actually does.

| # | Severity | Spec / claim | Reality |
|---|---|---|---|
| D1 | **HIGH** | `feature-coverage.md` "Welcome bundle — convention-based" | Spec implies a `LIKE '%welcome%'` lookup. Reality is **worse**: `repositories.go:607-628` falls back to "first bundle by `created_at ASC`" if no name matches. In a fresh deployment with one bundle, *any* bundle becomes the welcome bundle. Either error explicitly when no welcome bundle is configured, or document this fallback prominently in the spec. |
| D2 | **HIGH** | UI design.md §5: "vibrant dark/light modes" + "command palette ⌘K" | Theme toggle landed (Material tokens only); ⌘K command palette **not implemented anywhere** (no `cmdk`, no `Cmd+K` listener, no `CommandPalette` component). Either ship it or remove the bullet. |
| D3 | **MED** | "Backend is the single mutation authority" (`design.md` + `CLAUDE.md`) | Contradicted by `discovery.go:218-282` which exposes direct Zitadel grant CRUD as operator-only routes (`/api/v1/zitadel/users/{id}/grants` + PUT/DELETE on individual grant IDs). Either remove these (force all mutations through bundle/rule paths) or amend the doctrine. |
| D4 | **MED** | `feature-coverage.md` "Versioned policies — Partial; version column exists, no rollback" | Confirmed accurate. `mapping_rules.version` is incremented by `UpdateMappingRule` (`repositories.go:145-156`); no history table, no snapshot, no rollback. The spec is honest, but flag this in the roadmap as either "ship rollback" or "drop the versioning conceit and rely on audit_logs for replay." |
| D5 | **MED** | OIDC member portal | `lib/session.ts:202-204` hardcodes `title: ""`, `team: ""`, `location: ""` for OIDC sessions. Member dashboard at `app/page.tsx:55-56` renders `{session.title} • {session.team} • {session.location}` → in production this displays " •  • " (three spaces). User-management spec says these come from Zitadel metadata and are populated. **Visible regression for any non-admin user signing in via OIDC.** |
| D6 | **MED** | "Live-Directory Identity Completeness" claim that `Title`/`Team`/`Location` overlay from Zitadel metadata | Title and Team are rendered on `users/page.tsx`. **Location is not rendered anywhere** despite being in `lib/types.ts:UserProfile`, the demo profile, and the spec. Drop it or render it. |
| D7 | **MED** | "1-command install/update via `update.sh`" | Broken: `LLDAP_BIND_DN`, `LLDAP_BIND_PASSWORD`, all `SYNC_*`, `MKAUTH_EXTERNAL_URL`, `ZITADEL_M2M_TOKEN` are required at runtime but **absent from `.env.example`**. An operator running the documented flow hits cryptic errors with no template to reference. |
| D8 | **LOW** | `feature-coverage.md` "Webhook listener: 6 event types" | Mostly accurate, but `webhook.go:77-79` defaults missing `event_type` to `grant_added` — convenience that contradicts the strict 6-type list. |
| D9 | **LOW** | `.env.example:27-30` framing for `EXPIRY_SCHEDULER_*` | Warns operator to "set to false on extra replicas when running N > 1 backend instances." Single-LXC deployment doesn't have multi-replica; nothing in compose, `install.sh`, or the architecture supports horizontal scale. Misleading framing. |
| D10 | **LOW** | `obsidian-clarity-redesign/tasks.md` checkmarks | All Stage-1 tasks marked `[x]` complete, but the migration of *legacy palette* surfaces (Sidebar, ThemeToggle, ErrorBoundary, RequestAccessButton, parts of `zitadel/page.tsx`) is incomplete. Either move those tasks to a Stage-2 list or stop marking the change "in progress" while leaving its visual contract half-met. |

---

## Findings: Bugs & Correctness Concerns

Severity: HIGH = ship-blocking; MED = trust-eroding but contained; LOW = brittleness, not harm.

| # | Severity | Where | Issue |
|---|---|---|---|
| C1 | **HIGH** | `backend/internal/handlers/zitadel_action_auth.go:50-57` | `withZitadelActionSignature(envVar, …)` falls through *unverified* when the named env var is empty. Justification: dev-mode parity with `withUserAuth`. **Risk in production:** with `ZITADEL_DOMAIN` set but `ZITADEL_EVENT_SIGNING_KEY` or `ZITADEL_ACTION_SIGNING_KEY` accidentally unset (config drift, secrets-mount typo, deploy script regression), `/api/webhooks/zitadel` and `/api/action/inject` accept anonymous POSTs with full effect. **Fix:** when `ZITADEL_DOMAIN` is set, require the matching signing key — refuse to start (or 503 the route) instead of silently disabling verification. |
| C2 | **HIGH** | `ui/src/app/page.tsx:55-56` + `ui/src/lib/session.ts:202-204` | OIDC member dashboard renders blank metadata: " •  • ". Either fetch profile from `/api/v1/users/{self}` during the OIDC callback to populate the cookie, or drop the line for OIDC sessions. |
| C3 | **MED** | `backend/internal/handlers/vault.go:13-30` (`enforceSelfOnly` dev mode) | In dev (API-key auth) the function uses the `{uid}` from the path as the actor. Anyone holding the API key can write a shadow credential for any user — the audit trail then "self-attributes" the action to the victim. Production (Zitadel JWT) is correct. Risk window: local-dev + leaked API key. The audit lie is the real damage. **Fix:** in dev mode, refuse to mutate or require an explicit `?actor=` query param logged separately. |
| C4 | **MED** | `backend/internal/handlers/router.go:189-211` (`withOperatorAuth`) | The inner closure re-extracts the bearer token and calls `auth.HasProjectRole(rawToken, …)`, which decodes the payload **without re-verifying the signature**. This works only because `withUserAuth` runs first. Any future refactor that adds another wrapper between them, or that switches to claim-from-context, silently breaks role enforcement. **Fix:** lift parsed claims into request context in `withUserAuth`; have `withOperatorAuth` read from context. |
| C5 | **MED** | `backend/internal/db/repositories.go:496-523` (`GetClaimFailureMode`) | A real DB error returns `("fail_closed", nil, err)`. `degradedResponse` (`action.go:182-186`) logs and returns empty `append_claims` without distinguishing "no profile configured" from "DB is on fire." A `minimal_safe`-mode project silently downgrades to fail-closed during a transient DB blip. **Fix:** cache last-known mode per project in Redis (already next to the claims) so transient DB failure rides on the cached value. |
| C6 | **MED** | `backend/internal/directory/zitadel.go` (`Users()` overlay) | Cache is set only on success, but `applyUserMetadataOverlay` modifies `out` in place. If the metadata fan-out returns partial results, the cached entry is half-overlaid forever (until 30s TTL). **Fix:** skip cache when overlay had per-user errors, or make the overlay idempotent on retry. |
| C7 | **MED** | `sync/internal/ldap/client.go:170` | `groupOfNames` placeholder uses `member: [""]`. Empty-string DN is technically invalid LDAP; some servers (OpenLDAP strict mode) reject it. LLDAP's Rust impl tolerates it today but this is fragile. Use the bind DN as the placeholder, or switch the schema to a model LLDAP supports natively. |
| C8 | **MED** | `sync/internal/worker/worker.go:101` (LDAP context propagation) | Worker calls `lp.AddUserToGroup(ctx, …)` but `ldap/client.go:185` ignores the context (`_ context.Context`). On graceful shutdown the worker blocks until TCP timeout, instead of cancelling. Plumb context through `withConn` or attach a deadline. |
| C9 | **MED** | `scripts/smoke-test-lxc.sh:11-13` | Hits `GET /api/v1/bundles` with `MKAUTH_API_KEY`. `router.go` wraps the route in `withUserAuth`, which falls through to API-key auth **only when `ZITADEL_DOMAIN` is unset**. On a real OIDC LXC the smoke test 401s. Probe `/healthz` (no auth) instead. |
| C10 | **LOW** | `sync/internal/worker/worker.go:190-194` (shadow-password zeroing) | `defer zero(hashBytes)` after `SetUserPassword` is theatre — the original `hash` string returned by `bc.GetShadowCredentialHash` is GC-heap immutable; the only zeroable buffer is the `[]byte` copy. Either accept the limitation (drop the defer) or document it. |
| C11 | **LOW** | `webhook.go:77-79` + `:104-109` | Two silent defaults can chain: malformed event with empty `event_type` and missing `source_project` returns 200 "no dispatch" instead of 400. Functionally fine; obscures observability of broken triggers. |

---

## Test Quality Spot-Check

Sampled `webhook_test.go`, `reconciliation_test.go`, `action_test.go`:

- `webhook_test.go` — solid behavioral coverage. Each event type asserts the right dispatch fired AND wrong ones didn't (e.g. `TestWebhook_UserDeactivated` explicitly verifies enforce/revoke NOT called). Real edges: signature 401, dedup short-circuit, role_keys plural-vs-singular precedence. Not coverage padding.
- `reconciliation_test.go` — drives diff core through the handler with stub injection. Covers `only_in_mkauth`, `only_in_zitadel`, drift, error pass-through, pagination iteration. Correct shape.
- `action_test.go` — exercises `fail_closed` / `minimal_safe` with malformed cache JSON. Good coverage. **Gap:** when `dbGetClaimFailureMode` itself returns a DB error (correctness #C5), there's no test.

UI test surface is broader than the April audit captured: 16 spec files, ~73 `it()` cases spanning sessions (`lib/__tests__/session.test.ts`), formatting (`lib/__tests__/format.test.ts`), name resolver (`lib/queries/__tests__/useNameResolver.test.tsx`), modal/confirm-modal a11y, page views (`app/{audit,bundles,grants,operations,policies,requests,users,page}/__tests__/page.test.tsx`), and component flows (`components/bundles/__tests__/{AddRolesToBundlePicker,CreateBundleModal}.test.tsx`, `components/roles/__tests__/CreateRoleModal.test.tsx`). The residual gap is narrow but security-critical: **`middleware.ts` (admin redirect, stale demo cookie clearing) and `api/proxy/[...path]/route.ts` (member self-scoping, allowlist) have no dedicated tests** — these enforce the admin/user split and would benefit from explicit coverage.

---

## Recommendations (Prioritized)

### Ship-blockers (act now)

1. **C1** — Make production refuse missing signing keys. When `ZITADEL_DOMAIN != ""`, fail-fast (or 503) on routes that depend on `ZITADEL_EVENT_SIGNING_KEY` / `ZITADEL_ACTION_SIGNING_KEY` if either is empty. The current "log a warning and pass through" is a production timebomb.
2. **C2** — Fix OIDC member dashboard metadata. Either fetch from `/api/v1/users/{self}` during callback into the cookie, or drop the title/team/location line for OIDC sessions.
3. **D1** — Decide what `GetWelcomeBundle` does with no match. "First bundle by created_at" is silent action at a distance. Either error explicitly or document loudly.
4. **C3** — Vault dev-mode self-attribution. Refuse self-only writes in dev-mode, or require an explicit actor.
5. **B1** — Delete `backend/cmd/test/main.go`. Destructive scratchpad with no caller.

### High-leverage cleanups

6. **B3** — Refactor `services/views.go`. Compute `(user → roles)` once per request; hand the same map to `ListUsers`, `ListApplications`, `ListProjects`, `Topology`. Likely halves the file.
7. **U2 + D10** — Finish the obsidian-clarity-redesign palette migration. Move the 5 outstanding surfaces (Sidebar, ThemeToggle, ErrorBoundary, RequestAccessButton, parts of `zitadel/page.tsx`) to Material tokens. Delete the legacy palette from globals.css after.
8. **U1** — Trim `useNameResolver`. Replace the rAF-batched 409 LOC with a single full-catalog fetch on provider mount. Keep only if a benchmark justifies the complex path.
9. **D7** — Document the sync-service env surface. Add an `--- Sync Service / LLDAP ---` block to `.env.example` with `LLDAP_*`, `SYNC_*`, `MKAUTH_EXTERNAL_URL`, `ZITADEL_M2M_TOKEN`.
10. **B4 + D3** — Resolve the "single mutation authority" contradiction. Either remove the operator-side Zitadel grant CRUD (`discovery.go:218-282`) or amend the design doc.

### Quality-of-life

11. **B5** — Split `repositories.go` into `repositories/{bundles,grants,rules,webhooks,vault,intents,roles,onboarding}.go`. Functions are good; the seam is wrong.
12. **U4** — Delete dead code in `lib/api.ts` and `lib/types.ts`. Single source of truth in `lib/queries/*`.
13. **U5** — Split `app/zitadel/page.tsx` into `components/zitadel/{Health,Rotation,Projects,Users,AllGrants}.tsx`. Replace bespoke fetch helpers with `request<T>`.
14. **S1 + S2** — Extract `scripts/lib/load-env.sh` + `scripts/lib/zitadel-api.sh`. Removes ~120 lines of duplicate bash across four scripts.
15. **B2** — Drop reconciliation safety cap to ~2000. Surface mapping-rule-derived grants as a separate "expected" bucket so the diff actually highlights drift.
16. **C9** — Fix `smoke-test-lxc.sh` to probe `/healthz`.
17. **U7** — Test `middleware.ts` and `api/proxy/[...path]/route.ts`. These enforce the admin/user split; an untested admin redirect is a future regression waiting.
18. **D2** — Decide on ⌘K. Either ship `cmdk` or strike the line from the spec.
19. **C5** — Cache last-known `claim_failure_mode` per project in Redis so DB blips don't silently downgrade `minimal_safe` projects.
20. **C4** — Lift parsed JWT claims into request context. Stop re-extracting the bearer token in `withOperatorAuth`.

### Spec-side housekeeping

21. **D8** — remove the `event_type` default in `webhook.go` or document it in the lifecycle spec.
22. **D9** — drop the "N > 1 replicas" framing in `.env.example`.
23. **S3** — fold `PERMISSIONS.md` and `SIGNING_KEY.md` into `zitadel/actions/README.md`. 4 docs → 2.
24. **S4** — wire `SYNC_RETRY_*` env or inline the constants.

---

## What This Audit Does *Not* Find

- **No SQL injection risk.** 100% parameterized queries via pgx (`$1`, `$2`).
- **No major security regressions** since the April audit. PKCE, RS256 JWT validation, HMAC freshness, body-size limits, security headers, constant-time API key comparison — all still in place and enforced.
- **No abandoned features.** Every page in `ui/src/app/` is reachable from the sidebar; every backend route has a caller.
- **No serious test rot.** ~190 backend tests, real behavioral assertions, not coverage padding.
- **No deployment-shape mismatch.** Docker Compose is genuinely minimal; sync service is correctly isolated; backend is single-replica by design.

---

## Inventory Delta Since April

The April inventory above is mostly still correct. New since then:

### Backend additions

- `internal/directory/` — live-vs-demo source seam (files: `directory.go`, `demo.go`, `zitadel.go`)
- `internal/services/expiry/` — grant expiration scheduler (`scheduler.go`, `sweep.go`, `deps.go`)
- `internal/services/vault.go` + `internal/handlers/vault.go` — Shadow Password Vault
- `internal/handlers/discovery.go`, `lookup.go`, `reconciliation.go`, `system.go`, `audit.go`, `bundles.go`, `profile.go`, `rotation_status.go`
- `internal/handlers/webhook_translate.go`, `webhook_translate_enrich.go` (event-trigger propagation)
- `internal/handlers/zitadel_action_auth.go`, `zitadel_grant_lookup.go`
- `internal/zitadel/applications.go`, `user_metadata.go`
- `cmd/mkauth-token/` — used by `register.sh` / `rotate.sh` to mint M2M tokens
- `cmd/test/` — **destructive scratchpad, not used by anything; recommend deletion**
- Migrations 9, 10, 11 (provisioning intents, shadow vault, Zitadel grants index)

### New backend routes (since April inventory)

> Math: the April **inventory table** above lists 35 distinct routes (29 control plane + 3 sync API + 2 data plane + 1 infrastructure). The April header label "49 routes" was a miscount in the original document — verified by re-counting that table. The current router (`backend/internal/handlers/router.go`) registers 59 `mux.HandleFunc` entries, so the **true delta is +24** routes added since the April inventory. The list below contains exactly those 24 routes; pre-existing routes (such as `GET /api/v1/bundles/{id}/impact`, already in the April control-plane table) are not duplicated here.

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/api/v1/system/mode` | user | Live/Demo/Degraded indicator |
| POST | `/api/v1/lookup` | user | Batch UID→name resolver |
| POST | `/api/v1/rules/mapping/validate` | user | Cycle-check a proposed rule before save |
| PUT | `/api/v1/users/{uid}/shadow-credential` | user | Set shadow credential (self) |
| DELETE | `/api/v1/users/{uid}/shadow-credential` | user | Clear shadow credential (self) |
| GET | `/api/v1/users/{uid}/shadow-credential/status` | user | Credential status |
| GET | `/api/v1/users/{uid}/shadow-credential/audit` | user | Per-user credential audit |
| GET | `/api/v1/shadow-credentials/{uid}/hash` | api-key | Sync-service hash retrieval |
| GET | `/api/v1/users/{uid}/profile` | api-key | Sync-service profile fetch |
| GET | `/api/v1/zitadel/health` | operator | M2M end-to-end smoke |
| GET | `/api/v1/zitadel/action-rotation-status` | operator | Signing-key age vs threshold |
| GET | `/api/v1/zitadel/users` | operator | List Zitadel users (paginated) |
| GET | `/api/v1/zitadel/users/{id}` | operator | Single user detail |
| GET | `/api/v1/zitadel/projects` | operator | List Zitadel projects |
| GET | `/api/v1/zitadel/projects/{id}/roles` | operator | Project roles |
| POST | `/api/v1/zitadel/projects/{id}/roles` | operator | Create project role |
| PUT | `/api/v1/zitadel/projects/{id}/roles/{key}` | operator | Update project role |
| DELETE | `/api/v1/zitadel/projects/{id}/roles/{key}` | operator | Delete project role |
| GET | `/api/v1/zitadel/grants` | operator | All grants (paginated) |
| GET | `/api/v1/zitadel/users/{id}/grants` | operator | User-scoped grants |
| POST | `/api/v1/zitadel/users/{id}/grants` | operator | Direct Zitadel grant (**doctrine bypass — see B4/D3**) |
| PUT | `/api/v1/zitadel/users/{id}/grants/{grantId}` | operator | Direct Zitadel grant update (**doctrine bypass**) |
| DELETE | `/api/v1/zitadel/users/{id}/grants/{grantId}` | operator | Direct Zitadel grant remove (**doctrine bypass**) |
| GET | `/api/v1/reconciliation/grants` | operator | MkAuth-vs-Zitadel grant diff (read-only) |

Total live routes: **59** in `backend/internal/handlers/router.go` (counted from `mux.HandleFunc` registrations on May 2026).

### Database schema additions

| Migration | Tables | Purpose |
|---|---|---|
| 000009 | `provisioning_intents` | LLDAP sync intent queue (4-state) |
| 000010 | `shadow_credentials`, `shadow_credential_audit` | Argon2id Samba/LLDAP credentials |
| 000011 | `zitadel_grants_index` | Local cache of Zitadel grant aggregates (enriches grant.changed/removed events) |

### New environment variables (delta from April)

| Variable | Used By | Default | Purpose |
|---|---|---|---|
| `ZITADEL_MACHINE_KEY_PATH` | Backend | (unset = local-policy-only) | Path to service-account JSON key |
| `ZITADEL_M2M_USER_ID` | Backend | — | User ID of the M2M account; webhook self-mutation guard |
| `ZITADEL_EVENT_SIGNING_KEY` | Backend | — | HMAC key for `/api/webhooks/zitadel` |
| `ZITADEL_ACTION_SIGNING_KEY` | Backend | — | HMAC key for `/api/action/inject` |
| `ZITADEL_EVENT_SIGNING_KEY_ROTATED_AT` | Backend | — | RFC3339 timestamp; consumed by Rotation Status panel |
| `ZITADEL_ADMIN_ROLE_KEY` | Backend | `admin` | Role key checked by `withOperatorAuth` |
| `EXPIRY_SCHEDULER_ENABLED` | Backend | `true` | Toggle the expiry sweep goroutine |
| `EXPIRY_SCHEDULER_INTERVAL` | Backend | `5m` | Sweep interval |
| `EXPIRY_SCHEDULER_BATCH_SIZE` | Backend | `500` | Max grants per sweep |
| `MKAUTH_EXTERNAL_URL` | Scripts | — | Public URL for Zitadel Action target registration (**missing from `.env.example`**) |
| `ZITADEL_M2M_TOKEN` | Scripts | — | Pre-minted M2M token for register.sh (**missing from `.env.example`**) |
| `LLDAP_URL` | Sync | — | LLDAP host:port (**missing from `.env.example`**) |
| `LLDAP_BIND_DN` | Sync | — | LLDAP bind DN (**missing, required**) |
| `LLDAP_BIND_PASSWORD` | Sync | — | LLDAP bind password (**missing, required**) |
| `LLDAP_USER_BASE_DN` | Sync | — | User search base (**missing**) |
| `LLDAP_GROUP_BASE_DN` | Sync | — | Group search base (**missing**) |
| `SYNC_POLL_INTERVAL` | Sync | `15s` | Backend poll cadence (**missing**) |
| `SYNC_BATCH_SIZE` | Sync | — | Intents claimed per poll (**missing**) |
| `SYNC_BACKEND_URL` | Sync | — | Backend internal URL (**missing**) |

### Module sizes (May)

| Module | LOC | Was (April) | Delta |
|---|---|---|---|
| `backend/` (Go src, no caches) | 19,241 | ~12k | +7k |
| `ui/` (TS/TSX, src only) | 14,042 | ~6k | +8k (Obsidian Clarity + dashboard UX) |
| `sync/` (Go src) | 1,487 | ~1.2k | +0.3k |

### OpenSpec changes (May 2026)

Complete: 18 changes across phases 1–5. `archive/` is empty (changes still browseable in-tree).

In progress: `obsidian-clarity-redesign` (UI palette migration, Stage 1 done, Stage 2+ outstanding).

Not started: Phase 5 partial-failure rollback, reconciliation auto-correct, welcome-bundle config UI, service-to-bundle automation, rate limiting, observability beyond `[DATA PLANE]`/`[SCHEDULER]` logs, CI/CD. Phase 6 (Google Workspace account poller) untouched.

### Test coverage (May)

| Layer | Files | Approx tests |
|---|---|---|
| Backend (Go) | ~30 `_test.go` files | ~190 (was 170) |
| Sync (Go) | ~5 `_test.go` files | ~32 |
| UI (Vitest) | 16 spec files | ~73 `it()` cases (session + format + name resolver + Modal/ConfirmModal a11y + 8 page views + bundle/role component flows) |

UI security gaps: `middleware.ts` and `api/proxy/[...path]/route.ts` have **no dedicated tests** despite enforcing the admin/user split.

---

## How to Use This Addendum

1. **Operator (you):** Treat the "Ship-blockers" list as a sprint; the rest as Phase 5 cleanup interleavable with the open ROADMAP items.
2. **Future Claude session:** Read `mkauth-core-architecture/specs/feature-coverage.md` first for ground truth, then this addendum for known drift. Findings are dated May 2026; re-validate with `git log` before acting on any specific line citation.
3. **Spec authors:** When updating `feature-coverage.md`, cross-reference the **Spec Drift** table here. Three claims (D1 welcome bundle, D2 ⌘K, D7 1-command install) currently overstate reality.

The architecture is right-sized. Most cleanup is finishing migrations, deleting one scratchpad, and refusing to silently fall through three production-shaped middlewares. Nothing in the findings is structural — they are all *resolvable without re-architecting*.
