# Requirement: Operational Readiness

Cross-cutting operational requirements for production-grade deployment. All items in this spec are deferred to Phase 5.

## Rate Limiting
The system MUST protect high-risk endpoints from abuse and overload.

### Scenario: Webhook flood protection
- **WHEN** the webhook endpoint receives requests exceeding a configurable rate threshold
- **THEN** the system MUST throttle or reject excess requests with appropriate HTTP status codes
- **AND** legitimate events arriving within the rate limit MUST NOT be silently dropped

### Scenario: Shadow password brute force protection
- **WHEN** the shadow credential set endpoint receives repeated requests for the same user within a short window
- **THEN** the system MUST rate-limit subsequent attempts
- **AND** the rate limit MUST be per-user, not global

### Scenario: Action injection abuse protection
- **WHEN** the data plane endpoint receives sustained high-volume requests outside of normal Zitadel Actions v2 invocation patterns
- **THEN** the system SHOULD surface the anomaly to operators

> **Status:** Deferred to Phase 5. Requires design decision: in-process token bucket vs Redis-backed distributed rate limiting.

## Observability
The system MUST expose operational metrics and support alerting beyond structured logs.

### Scenario: Operator monitors system health
- **WHEN** an operator needs to assess MkAuth's operational state
- **THEN** the system MUST expose metrics for: active grant count, webhook processing latency, sync service intent lag, cache hit/miss rates, and error rates by category

### Scenario: Degraded state triggers alert
- **WHEN** the data plane enters a sustained degraded state (repeated cache misses, Redis timeouts)
- **THEN** the system MUST surface this through a monitorable metric or alert channel

### Scenario: Sync service lag is visible
- **WHEN** provisioning intents remain in pending state beyond the configured poll interval
- **THEN** the operator MUST be able to detect the lag through metrics without reading logs

> **Status:** Deferred to Phase 5. Currently structured logs with `[DATA PLANE]`, `[WEBHOOK]`, `[AUTH]`, `[ONBOARDING]` prefixes only.

## Disaster Recovery
The system MUST define recovery procedures for common failure scenarios.

### Scenario: Database loss recovery
- **WHEN** the Postgres database is lost or corrupted
- **THEN** documented backup and restore procedures MUST exist
- **AND** the system MUST be recoverable to a known-good state within a documented RTO

### Scenario: MkAuth downtime impact
- **WHEN** MkAuth is offline
- **THEN** Zitadel MUST continue issuing tokens with the last-compiled cached claims
- **AND** the recovery procedure for re-syncing stale cache state MUST be documented

### Scenario: Sync service crash recovery
- **WHEN** the sync service crashes and restarts
- **THEN** previously claimed (acknowledged) but incomplete intents MUST be recoverable
- **AND** the system MUST NOT leave LLDAP in a silently inconsistent state

> **Status:** Deferred to Phase 5. No backup/restore documentation or runbook exists.

## CI/CD Pipeline
The system MUST support automated quality gates for code changes.

### Scenario: Code change validation
- **WHEN** a code change is proposed
- **THEN** automated backend tests (`go test ./...`), frontend tests (`bun run test`), linting, and container build verification MUST run before merge

### Scenario: Migration safety
- **WHEN** a database migration is added
- **THEN** automated validation MUST verify both up and down migrations execute cleanly against the current schema

> **Status:** Deferred to Phase 5. Currently manual verification only (`make test`, `make lint`).
