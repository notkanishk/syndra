## MODIFIED Requirements

### Requirement: Production degraded behavior for claim injection
The system MUST define a documented production failure posture for every application that depends on the Actions v2-compatible claim path.

#### Scenario: Application-specific degraded posture configured
- **WHEN** an application is configured for claim shaping
- **THEN** MkAuth MUST require a documented degraded-mode posture for cache miss, timeout, malformed cache data, or unavailable dependencies
- **AND** the allowed posture MUST be either fail-closed or an explicitly documented minimal safe fallback

#### Scenario: Implicit fallback prohibited
- **WHEN** the production claim path encounters a failure condition and no degraded-mode posture was configured
- **THEN** the configuration MUST be treated as incomplete
- **AND** the system MUST reject or block that production claim path until the posture is explicitly defined

---

## Implementation notes (Phase 3)

- `backend/db/migrations/000005_security_boundary.up.sql` — adds `claim_failure_mode TEXT NOT NULL DEFAULT 'fail_closed' CHECK (... IN ('fail_closed', 'minimal_safe'))` and `minimal_safe_claims JSONB` to `claim_profiles`
- `backend/internal/db/repositories.go` — `GetClaimFailureMode(ctx, projectID)`: returns the configured mode and optional minimal safe claims; `pgx.ErrNoRows` (no profile) returns `fail_closed` as the safe default; real DB faults return an error that is logged and visible
- `backend/internal/handlers/action.go` — `degradedResponse(ctx, projectID)`: called on Redis timeout (50 ms budget), cache miss, or malformed data; reads the configured mode and returns the appropriate payload; all paths are explicit — no implicit fallback exists
- Structured log prefix `[DATA PLANE]` with cache hit/miss/timeout/malformed labels makes degraded outcomes observable to operators
