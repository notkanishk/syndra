> **Status:** Wave 2 · Part 2 delta — Backend coherence (C5) | [< Index](../../../../INDEX.md)

# Requirement: Application Claims (delta)

## ADDED Requirements

### Requirement: Claim failure mode MUST survive transient database outages via a per-project Redis read-through cache

The data plane MUST NOT silently default to `fail_closed` when a transient PostgreSQL fault prevents `db.GetClaimFailureMode(ctx, projectID)` from returning the configured mode. The MkAuth backend MUST maintain a per-project read-through cache in Redis at key `claim_mode:<projectID>` (a sibling of the existing `mapping:<userID>:<projectID>` payload key) so the last-known mode survives transient database faults. On a transient DB error, the cached value MUST be returned in preference to silently degrading to `fail_closed` — `fail_closed` remains the safe default only when both the cache and the DB row are unavailable.

#### Scenario: Cache miss, DB succeeds — DB result returned and cached

- **WHEN** the data plane calls `claimFailureModeRead(ctx, projectID)` and Redis returns no value at key `claim_mode:<projectID>`
- **AND** `db.GetClaimFailureMode(ctx, projectID)` returns `("minimal_safe", {"reason":"degraded"}, nil)`
- **THEN** the helper MUST return `("minimal_safe", {"reason":"degraded"}, nil)`
- **AND** the helper MUST write the JSON-encoded `{"mode":"minimal_safe","minimal_safe_claims":{"reason":"degraded"}}` to Redis at key `claim_mode:<projectID>` with TTL `300` seconds (or the value of `CLAIM_MODE_CACHE_TTL_SECONDS` if set)
- **AND** a subsequent call within the TTL window MUST NOT invoke `db.GetClaimFailureMode`

#### Scenario: Cache hit — DB is bypassed

- **WHEN** Redis returns a valid JSON payload at key `claim_mode:<projectID>`
- **THEN** the helper MUST return the parsed mode and claims without calling `db.GetClaimFailureMode`
- **AND** the Redis fetch MUST complete within the `redisTimeout` (50 ms) budget shared with the rest of the data plane

#### Scenario: Cache hit, DB fails — cached value returned (the core C5 behaviour)

- **WHEN** the data plane calls `claimFailureModeRead(ctx, projectID)` and Redis returns a valid cached payload `{"mode":"minimal_safe","minimal_safe_claims":{"reason":"degraded"}}`
- **AND** `db.GetClaimFailureMode(ctx, projectID)` is not invoked because the cache hit short-circuits the call
- **THEN** the helper MUST return `("minimal_safe", {"reason":"degraded"}, nil)`

#### Scenario: Cache miss, DB fails — cached fallback exhausted, fail_closed returned

- **WHEN** Redis returns no value at `claim_mode:<projectID>`
- **AND** `db.GetClaimFailureMode(ctx, projectID)` returns a non-nil error
- **THEN** the helper MUST return `("fail_closed", nil, nil)`
- **AND** the helper MUST log the DB error at WARNING level so operators can observe the underlying fault

#### Scenario: Cache entry expires via TTL — next call falls through to DB and refreshes

- **WHEN** the TTL on key `claim_mode:<projectID>` has elapsed so Redis returns no value at the next read
- **AND** `db.GetClaimFailureMode(ctx, projectID)` returns `("minimal_safe", {"new":"value"}, nil)`
- **THEN** the helper MUST return `("minimal_safe", {"new":"value"}, nil)`
- **AND** the helper MUST write the fresh payload back to Redis under the same key with a fresh TTL, so the next caller within the new TTL window short-circuits on cache hit
- **AND** the cache MUST NOT pin stale values: a TTL-expired entry is indistinguishable from a never-cached entry, and the read path always re-consults the DB before re-caching

(Audit ref: C5)
