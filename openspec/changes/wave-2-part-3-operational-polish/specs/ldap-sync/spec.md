> **Status:** Wave 2 · Part 3 delta — Operational polish (C7, C8, S4) | [< Index](../../../../INDEX.md)

# Requirement: LLDAP Sync & Group Mapping (delta)

## ADDED Requirements

### Requirement: New LLDAP groups MUST be bootstrapped with the bind DN as the placeholder member, not an empty string

`groupOfNames` is the structural objectClass Syndra uses for sync-managed LLDAP groups, which requires at least one `member` attribute at creation time. The current `sync/internal/ldap/client.go:EnsureGroup` implementation supplies `member: [""]` — an empty distinguished name — which LLDAP's permissive schema check accepts but a strict OpenLDAP deployment would reject. The sync service MUST instead supply the configured bind DN (`p.cfg.BindDN`) as the placeholder member, because the bind DN is by construction a valid DN known to the directory. The placeholder MUST NOT be the empty string. This preserves the "create with one member; real users join alongside" pattern without depending on a specific LDAP server's tolerance for empty DNs.

#### Scenario: New group is created with the bind DN as placeholder member

- **WHEN** `EnsureGroup(ctx, "samba_share_admin")` is called and no group exists at `cn=samba_share_admin,ou=groups,<base_dn>`
- **AND** the pool was initialised with `BindDN = "uid=admin,ou=people,dc=example,dc=com"`
- **THEN** the AddRequest sent to the LDAP server MUST include `member` attribute with the single value `"uid=admin,ou=people,dc=example,dc=com"`
- **AND** the AddRequest MUST NOT include an empty-string member value

#### Scenario: Subsequent user additions overwrite the bind-DN placeholder

- **GIVEN** a freshly-bootstrapped group whose only `member` is the bind DN
- **WHEN** `AddUserToGroup(ctx, "u-alice", "samba_share_admin")` is called and the operation succeeds
- **AND** the next reconciliation pass observes the group
- **THEN** the group's `member` attribute MUST contain `"uid=u-alice,ou=people,<base_dn>"`
- **AND** the bind-DN placeholder MAY remain or be removed depending on the reconciliation policy — neither outcome breaks correctness, because the bind DN is a real principal in the directory

### Requirement: LLDAP operations MUST honour context cancellation at mutex-boundary checkpoints

`sync/internal/ldap/client.go:withConn` is the choke point through which every LLDAP operation passes — it acquires the pool mutex, executes the caller's function with the held connection, and retries once if the function returns a connection error. The current signature accepts only `fn func(*ldapv3.Conn) error`; queued or not-yet-attempted ops have no way to bail out when the parent context is cancelled. The signature MUST accept `context.Context` as its first parameter and MUST return `ctx.Err()` immediately on cancellation at three checkpoints: before attempting to acquire `p.mu`, immediately after acquiring `p.mu`, and before any reconnect retry.

This requirement deliberately does NOT cover mid-LDAP-call cancellation. The active op holding the mutex is not interrupted; it runs to completion or fails via the underlying connection timeout. Likewise, a goroutine already blocked inside `p.mu.Lock()` continues to wait for the mutex (Go's `sync.Mutex` is not select-able) — cancellation is observed only at the post-acquisition checkpoint, with worst-case latency equal to the in-flight op's remaining duration.

#### Scenario: Cancelled context returns immediately without acquiring the mutex

- **GIVEN** a context `ctx` for which `ctx.Err()` returns `context.Canceled`
- **WHEN** any LLDAP operation that funnels through `withConn(ctx, ...)` is invoked
- **THEN** the operation MUST return `context.Canceled` (or wrap it via `fmt.Errorf("...: %w", ctx.Err())`) without acquiring `p.mu`
- **AND** the operation MUST NOT issue any LDAP request

#### Scenario: Context cancelled mid-queue while another goroutine holds the mutex

- **GIVEN** goroutine A is executing `fn` inside `withConn` with `p.mu` held
- **AND** goroutine B is blocked on `p.mu.Lock()` inside `withConn(ctxB, ...)`
- **WHEN** `ctxB` is cancelled before goroutine A releases the mutex
- **THEN** when goroutine B eventually acquires the mutex, the post-acquisition `ctx.Err()` check MUST return `context.Canceled` and `fn` MUST NOT be invoked

#### Scenario: Reconnect retry honours cancellation

- **GIVEN** the first invocation of `fn` returns a connection error and the pool would normally reconnect and retry
- **WHEN** `ctx` is cancelled between the first failure and the reconnect call
- **THEN** the operation MUST return `ctx.Err()` and MUST NOT attempt to reconnect

### Requirement: Sync-service retry attempts and backoff MUST be configurable via environment variables

`sync/internal/config/config.go` defines `RetryAttempts` (default `3`) and `RetryBackoff` (default `1s`), both consumed by `worker.go:retryTransient`. The values are currently hardcoded constants in the config struct initialiser — they cannot be tuned without recompiling. The sync service MUST read `SYNC_RETRY_ATTEMPTS` as a positive integer and `SYNC_RETRY_BACKOFF` as a positive Go duration from the environment, falling back to the documented defaults (`3` and `1s` respectively) when either is unset or empty. Malformed values — including non-positive ones — MUST surface as a `Load()` error rather than being silently coerced. (`retryTransient` runs `RetryAttempts + 1` attempts: a negative `RetryAttempts` skips its loop so `fn` is never called, and `0` means no retries at all — both violate the positive-integer contract. A non-positive `RetryBackoff` would make the exponential backoff fire immediately.)

#### Scenario: SYNC_RETRY_ATTEMPTS overrides the default

- **GIVEN** the environment has `SYNC_RETRY_ATTEMPTS=5`
- **WHEN** `LoadConfig()` returns a `Config`
- **THEN** the returned config MUST have `RetryAttempts == 5`

#### Scenario: SYNC_RETRY_BACKOFF overrides the default

- **GIVEN** the environment has `SYNC_RETRY_BACKOFF=250ms`
- **WHEN** `LoadConfig()` returns a `Config`
- **THEN** the returned config MUST have `RetryBackoff == 250 * time.Millisecond`

#### Scenario: Absent env vars fall back to documented defaults

- **GIVEN** neither `SYNC_RETRY_ATTEMPTS` nor `SYNC_RETRY_BACKOFF` is set in the environment
- **WHEN** `LoadConfig()` returns a `Config`
- **THEN** the returned config MUST have `RetryAttempts == 3`
- **AND** `RetryBackoff == 1 * time.Second`

#### Scenario: Non-positive retry values are rejected

- **GIVEN** the environment has `SYNC_RETRY_ATTEMPTS=0` (or a negative integer) or `SYNC_RETRY_BACKOFF=0s` (or a negative duration)
- **WHEN** `LoadConfig()` runs
- **THEN** `Load()` MUST return an error rather than a `Config` with the coerced value

(Audit refs: C7, C8, S4)
