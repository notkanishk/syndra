## 1. Security hardening

- [x] 1.1 Replace wildcard `Access-Control-Allow-Origin: "*"` with configurable `CORS_ORIGIN` env var
- [x] 1.2 Add `withSecurityHeaders` middleware (`X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`)
- [x] 1.3 Replace `!=` API key comparison with `crypto/subtle.ConstantTimeCompare`
- [x] 1.4 Add `withMaxBody` middleware (1 MB limit) wrapping the mux
- [x] 1.5 Update `NewRouter` return type from `*http.ServeMux` to `http.Handler` to support middleware wrapping

## 2. Local development infrastructure

- [x] 2.1 Create `.env.example` with all backend and frontend env vars documented
- [x] 2.2 Make migration path configurable via `MIGRATION_PATH` env (fallback to Docker default)
- [x] 2.3 Create root `Makefile` with `dev`, `test`, and `lint` targets

## 3. Backend reliability

- [x] 3.1 Add `GET /healthz` endpoint (Postgres ping, no auth)
- [x] 3.2 Implement graceful shutdown with `signal.NotifyContext` and connection cleanup

## 4. Type safety

- [x] 4.1 Change `SERVER_API` to read `BACKEND_URL` env instead of hardcoded Docker hostname
- [x] 4.2 Add generic type parameter to `fetchServerJson<T>` and `fetchWithAuth<T>`
- [x] 4.3 Create `ui/src/lib/types.ts` with shared API response types mirroring Go models
- [x] 4.4 Type all named fetchers with concrete return types (`fetchBundles` -> `Promise<Bundle[]>`, etc.)
- [x] 4.5 Update `page.tsx` to use typed `fetchWithAuth<UserAccessView>` and `fetchWithAuth<AccessRequest[]>`
- [x] 4.6 Update `jsonResponse` parameter from `interface{}` to `any` (Go 1.18+ idiom)

## 5. Test coverage

- [x] 5.1 Create `cache/deps.go` with injectable function vars for compiler dependencies
- [x] 5.2 Refactor `compiler.go` to call through injectable vars
- [x] 5.3 Write 5 compiler tests: empty grants, direct grants, mapping rule transitivity, bundle roles, fixed-point termination
- [x] 5.4 Add Vitest to frontend dev dependencies and create `vitest.config.ts`
- [x] 5.5 Write 6 session tests: demo user catalog, lookup, encode roundtrip, OIDC payload encoding
- [x] 5.6 Verify all backend tests pass (`go test ./...`, `go vet ./...`)
- [x] 5.7 Verify all frontend checks pass (`bun run lint`, `bun run test`, `bun run build`)
