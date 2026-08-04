# Syndra — Project Overview

Syndra is an IAM (Identity Access Management) orchestration platform built on top of Zitadel.

## Purpose
Manages role bundles, mapping rules, claim compilation, access request governance, and topology visualization for downstream applications. Acts as an orchestration layer between Zitadel (identity provider) and downstream services.

## Tech Stack
- **Backend**: Go 1.25+ (stdlib `net/http`, no frameworks)
- **Frontend**: Next.js 15 + React 19 (Bun runtime)
- **Database**: PostgreSQL 15 (pgx/v5 connection pooling)
- **Cache**: Redis 7 (go-redis/v9)
- **Identity**: Zitadel OIDC (PKCE flow, RS256 JWT, M2M Management API)
- **Deployment**: Docker Compose in Proxmox LXC

## Key Architecture
- Separate containers for Frontend, Backend
- Backend is the single mutation authority (audit, retries, idempotency)
- Injectable dependency pattern (module-level function vars in `deps.go` files) for testability
- Discriminated union session cookies (`demo | oidc`)
- Fixed-point iteration for mapping rule resolution in cache compiler
- HMAC-SHA256 webhook verification with timestamp freshness

## Structure
```
backend/
  cmd/api/main.go              — Entry point, graceful shutdown
  internal/
    auth/jwt.go                — RS256 JWT validation via JWKS
    cache/compiler.go          — Fixed-point role resolution + Redis caching
    db/                        — pgxpool, migrations, repositories, validation
    handlers/                  — Route registration, middleware, all API handlers
    models/models.go           — Domain types
    services/                  — Onboarding, governance, views
    zitadel/                   — M2M client, orchestrator, token manager
  db/migrations/               — 6 sequential SQL migrations

ui/
  src/app/                     — Next.js pages and API routes
  src/lib/                     — API client, session, OIDC, types
  src/components/              — Shared React components
```
