## Why

Syndra reached a point where the product shape, governance model, and security boundary architecture are all validated — but several cross-cutting concerns had accumulated without dedicated attention: wildcard CORS, missing security headers, timing-vulnerable API key comparison, no request body limits, hardcoded Docker paths blocking local development, untyped frontend API layer, zero frontend tests, and no cache compiler test coverage. These are the kinds of issues that individually feel minor but collectively determine whether a codebase is production-grade or merely functional.

A full audit was conducted to surface and fix these issues in a single pass rather than letting them compound across future feature work.

## What Changes

* Hardens the HTTP security posture: configurable CORS origin, security response headers, constant-time API key comparison, and 1 MB request body limits on all endpoints.
* Establishes local development infrastructure: `.env.example`, configurable migration path, and a root `Makefile` with `dev`, `test`, and `lint` targets — enabling the backend and frontend to run without Docker using local Postgres and Redis.
* Adds backend reliability primitives: a `/healthz` endpoint and graceful shutdown with connection cleanup.
* Strengthens frontend type safety: all server-side API fetchers are now generic-typed with concrete return types, eliminating `Promise<any>` from the API layer. A shared `types.ts` mirrors Go models used by the frontend.
* Extends test coverage to previously untested critical paths: cache compiler (5 tests covering role resolution, transitivity, and bundle inclusion) and frontend session logic (6 tests covering encode/decode, OIDC payloads, and demo user lookup).

## Capabilities

### New Capabilities
* `local-dev-workflow`: Makefile, `.env.example`, and configurable migration path enabling Docker-free local development.

### Modified Capabilities
* `production-security-boundary`: CORS, security headers, constant-time comparison, body limits close remaining HTTP-layer gaps.
* `contract-quality`: Frontend API layer now enforces typed contracts; cache compiler has injectable deps and test coverage.
* `backend-api-testing`: Cache compiler tests added; frontend test infrastructure (Vitest) established with session module coverage.

## Impact

* Affects `backend/internal/handlers/router.go` (CORS, headers, body limits, health route, constant-time compare), `backend/cmd/api/main.go` (graceful shutdown), `backend/internal/db/postgres.go` (migration path), `backend/internal/cache/compiler.go` (injectable deps).
* Affects `ui/src/lib/api.ts` (typed fetchers, env-based backend URL), `ui/src/app/page.tsx` (typed `fetchWithAuth` calls), and new `ui/src/lib/types.ts`.
* Adds Vitest to frontend dev dependencies and creates the first frontend test file.
* Does not change any database schema, API contract, or business logic.
