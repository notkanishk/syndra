## Rationale

This change is a horizontal audit — it cuts across security, DX, type safety, and test coverage rather than adding vertical feature work. The design principle is minimal targeted fixes: each change addresses a verified issue without introducing new abstractions, libraries, or patterns beyond what the codebase already uses.

## Technical Specification

### 1. Security Hardening

**CORS (router.go: `withCORS`)**
- Reads `CORS_ORIGIN` env var at handler initialization time (not per-request).
- Defaults to `http://localhost:3000` for local dev.
- Sets `Access-Control-Allow-Credentials: true` so cookie-based OIDC sessions work with a specific origin.
- In Docker, set to the UI's public URL via `docker-compose.yml`.

**Security Headers (router.go: `withSecurityHeaders`)**
- Applied once, wrapping the entire mux — not per-route.
- Headers: `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: strict-origin-when-cross-origin`.
- CSP and HSTS are not added — CSP is a frontend concern (Next.js handles it), and HSTS requires TLS termination which is handled by the reverse proxy.

**Constant-Time API Key Comparison (router.go: `withAPIKeyAuth`)**
- Uses `crypto/subtle.ConstantTimeCompare` instead of `!=` to prevent timing side-channels on the shared API key.
- Only applies in local-dev mode (production uses JWT validation).

**Request Body Limits (router.go: `withMaxBody`)**
- `http.MaxBytesReader(w, r.Body, 1<<20)` — 1 MB limit.
- Applied at the mux level so all routes are protected.
- The webhook handler already had its own `io.LimitReader`; the mux-level limit is belt-and-suspenders there.

### 2. Local Development Infrastructure

**`.env.example`**
- Documents every env var used by backend and frontend with safe local defaults.
- Includes `MIGRATION_PATH=file://db/migrations` for running outside Docker.
- Zitadel vars are commented out with descriptive labels.

**Configurable Migration Path (postgres.go)**
- Reads `MIGRATION_PATH` env, falls back to `file:///app/db/migrations` (Docker default).
- For local dev: `MIGRATION_PATH=file://db/migrations` (relative to working dir).

**Makefile**
- `make dev`: Runs backend and frontend in parallel (`make -j2`).
- `make test`: Runs `go test ./...` and `bun test`.
- `make lint`: Runs `go vet ./...` and `bun run lint`.
- No Docker, build, or deploy targets — those are separate concerns.

### 3. Backend Reliability

**Health Check (`GET /healthz`)**
- No auth, no CORS — designed for load balancers and monitoring.
- Pings Postgres with a 2-second timeout.
- Returns `{"status": "ok"}` (200) or `{"status": "unhealthy", "db": "..."}` (503).
- Does not check Redis — Redis failure is already handled gracefully by the data plane's degraded modes.

**Graceful Shutdown (main.go)**
- `http.Server` with `signal.NotifyContext` listening for SIGINT/SIGTERM.
- `server.Shutdown(ctx)` with a 10-second deadline drains in-flight requests.
- Closes `db.PG` (pgxpool) and `db.Redis` after the server stops.

### 4. Type Safety

**`SERVER_API` Environment Variable (api.ts)**
- Changed from hardcoded `http://backend:8080` to `process.env.BACKEND_URL || "http://backend:8080"`.
- Matches the pattern already used in the proxy route handler.

**Generic Typed Fetchers (api.ts)**
- `fetchServerJson<T>` and `fetchWithAuth<T>` use a generic type parameter defaulting to `unknown`.
- Each named fetcher specifies its return type: `fetchBundles` returns `Promise<Bundle[]>`, etc.
- Both `eslint-disable` comments for `@typescript-eslint/no-explicit-any` are removed.

**Shared Types (types.ts)**
- Mirrors the Go models needed by server-side fetchers.
- Page-level components may still define their own narrower types for client-side use — this is intentional, not drift.

**`interface{}` to `any` (router.go)**
- `jsonResponse` parameter updated to use Go 1.18+ `any` alias. No behavioral change.
- `map[string]interface{}` in models and compiler is left as-is — correct for dynamic JSON claims.

### 5. Test Coverage

**Cache Compiler (compiler_test.go + deps.go)**
- Follows the same injectable-dependency pattern established in `handlers/deps.go`.
- New `cache/deps.go` exposes: `dbGetDirectGrantsForUser`, `dbGetActiveMappingRules`, `dbGetBundlesForUser`, `dbGetRolesForBundle`, `redisSet`, `redisDel`, `redisScanKeys`.
- `compiler.go` refactored to call through these vars instead of direct package references.
- Tests: no grants (empty), direct grants only, transitive mapping rules, bundle role inclusion, fixed-point termination on long chains.

**Frontend Session Tests (session.test.ts + vitest.config.ts)**
- Vitest configured with `@` path alias and Node environment.
- Tests target the pure functions exported from `session.ts` that don't require the Next.js cookie store.
- Coverage: `getDemoUsers` count, `getDemoUser` lookup and miss, `createSessionValue` encode roundtrip, `createOidcSessionValue` payload integrity.

## Quality Principles

- Every change reuses existing patterns (injectable deps, env-based config, middleware wrapping).
- No new dependencies beyond Vitest (which is the standard Vite-ecosystem test runner).
- No new abstractions — `withSecurityHeaders` and `withMaxBody` are 6-line functions.
- No changes to API contracts, database schema, or business logic.

## Explicitly Excluded

| Item | Reason |
|------|--------|
| OpenAPI/Swagger | API is internal to the Next.js proxy; no external consumers exist |
| Structured logging (slog) | Works fine with `log.Printf`; migration touches every file for marginal gain |
| Pagination | No endpoint returns enough data to warrant it in current usage patterns |
| Rate limiting | Requires a design decision (in-process vs Redis-backed); separate change |
| CI/CD pipeline | Orthogonal to code quality; add when deploying |
| OpenTelemetry | No observability infrastructure to send to |

## Verification

```
cd backend && go test ./...   # All passing (5 packages, including new cache tests)
cd backend && go vet ./...    # Clean
cd ui && bun run lint          # No warnings
cd ui && bun run test          # 6/6 passing
cd ui && bun run build         # Successful
```
