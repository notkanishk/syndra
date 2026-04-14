# MkAuth Codebase Audit — April 2026

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
