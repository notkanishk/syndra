> **Status:** Integrated | [< Index](../../../../INDEX.md) | [Feature Coverage](../feature-coverage.md)

# Requirement: Application Claim Selection & Shaping

The system MUST provide a way for downstream applications to define which roles they consume and how those roles are presented in the JWT.

## Claim selection from projects
The system MUST allow an application to be associated with a specific project context, pulling all active roles (source or derived) for that project.

### Scenario: High-precision claim scoping
- **GIVEN** a user has multiple roles across different projects (e.g., Printing, Laser, Door Access)
- **WHEN** the "Printing Portal" application requests claims for this user
- **THEN** it only receives roles relevant to the "Printing" project, ensuring least-privilege for the application.

## Claim shaping for JWT payload
The system MUST allow applications to define a custom claim name and format for their roles.

### Scenario: Shaping roles for a legacy consumer
- **GIVEN** an application that expects roles in a space-delimited string (e.g., "admin operator trainee")
- **WHEN** the application is configured with `FormatType: space_delimited` and `ClaimName: permissions`
- **THEN** the token simulation and data plane response MUST return the roles in that specific format.

## One shape per application (claims-shaping, ADR-003)
A token MUST carry only the shape of the application it is for, picked by the request's `application.client_id`. An application with no shape of its own MUST get its project's default, and a project with none MUST get the built-in `roles` list, never nothing. Every roles claim MUST be sent, as `[]` when the person holds none of the roles the app sees. Contract: `changes/claims-shaping/design.md`.

### Scenario: Two apps on one project
- **GIVEN** NetBird and Inventory on one project, NetBird sending `groups` in its own words and Inventory sending `roles`
- **WHEN** a person signs in to NetBird
- **THEN** the token carries `groups` only, with no Inventory key
- **AND** a client with no shape of its own, or a request with no client id, gets the project's default

### Scenario: A save the app will act on
- **GIVEN** NetBird's shape sends `netbird-staff`
- **WHEN** an operator saves a shape that renames it, without `acknowledge_breaking: true`
- **THEN** the save is refused with 409 `BREAKING_UNACKNOWLEDGED`, naming the change and whom it reaches

## Cross-project claim propagation
The system MUST support "selecting" claims from other projects by utilizing mapping rules to project the desired roles into the application's local context.

### Scenario: Granting door access based on lab certification
- **GIVEN** a mapping rule: `IF project:printing role:calibrator THEN ADD project:doors role:3d_lab_pin`
- **WHEN** the "Door Controller" application (Project: doors) requests claims for a user with the Printing Calibrator role
- **THEN** the JWT MUST include the `3d_lab_pin` role.

## Implementation: Zitadel Actions v2 Integration
The data plane MUST be implemented as a **Zitadel Actions v2** target, invoked during the `function/preaccesstoken` and `function/preuserinfo` triggers, returning the v2 response envelope. Actions v2 is not a JavaScript runtime; Zitadel POSTs the function-trigger payload (`user`, `user_grants`, `org`, `userinfo`, ...) to the configured HTTP target and the target responds with a typed envelope.

- **append_claims envelope**: the target MUST emit each claim as `{key, value}` inside the `append_claims` array. The optional `append_log_claims` list carries diagnostic keys. `set_user_metadata` is reserved for future use.
- **Cache-backed source**: claim values are looked up in Redis using `mapping:<userID>:<projectID>` keys populated by the cache compiler.
- **One project per token**: Zitadel lists only the token's role audience in `user_grants`. When it names exactly one project, the response uses that project's flat, configured keys. When it names more than one, the response MUST carry no Syndra claims and log the request; cross-project delivery is refused, never namespaced. (Owner, 28 Oct 2026; ADR-002. Detail in `basic-advanced-ia/specs/application-claims`.)
- **Degraded posture**: decided by the token's project.
- **Availability Rule**: the integration MUST define explicit behavior for cache miss, backend timeout, malformed cache data, and unavailable downstream dependencies.
- **Safe Failure Rule**: failure behavior MUST be either `fail_closed` (empty `append_claims` for that project) or `minimal_safe` (configured minimal claim set), depending on the application's documented security posture.
- **Performance Goal**: the path SHOULD be low-latency; reliability and deterministic failure behavior take precedence over a fixed micro-latency target.
- **Compatibility Boundary**: Zitadel Actions v2 MUST remain the only supported source-of-truth-facing claim integration model. Zitadel Actions v1 MUST NOT coexist with the v2 target on the same trigger.

### Scenario: Cache miss during claim injection
- **WHEN** the Actions v2 target request triggers a cache lookup and no compiled entry exists
- **THEN** Syndra MUST return the documented safe fallback envelope for that application
- **AND** the outcome MUST be observable for operators via the `[DATA PLANE]` log line.

### Scenario: Unsupported degraded behavior blocked
- **WHEN** an application has no documented failure posture for claim injection
- **THEN** the configuration MUST be rejected as incomplete.

### Scenario: Request signature verification
- **WHEN** the backend receives a request at `/api/action/inject` and `ZITADEL_ACTION_SIGNING_KEY` is set
- **THEN** the request MUST carry a `ZITADEL-Signature: t=<unix>,v1=<hex>[,v1=<hex>...]` header, verified by HMAC-SHA256 over `<unix_decimal>.<body>`
- **AND** the backend MUST enforce a 300-second timestamp tolerance
- **AND** requests that fail verification MUST receive `401 INVALID_SIGNATURE` without leaking which step failed.


## Claim contract hardening
The system MUST treat claim-shaping payloads and action responses as hardened contracts, with strict validation and regression coverage for every supported format.

### Scenario: Unsupported claim format blocked
- **WHEN** an application configuration declares an unknown claim format
- **THEN** the backend MUST reject or fail the configuration deterministically
- **AND** automated tests MUST verify that unsupported formats do not silently degrade into a permissive payload

### Scenario: Internal contract does not replace Actions v2
- **WHEN** Syndra introduces or changes an internal payload between the UI, backend, or sync service
- **THEN** that change MUST NOT replace, bypass, or redefine the Zitadel Actions v2 compatibility contract
- **AND** the source-of-truth-facing claim path MUST remain anchored to the Actions v2 flow

## Production degraded behavior
The system MUST define a documented production failure posture for every application that depends on the Actions v2-compatible claim path.

### Scenario: Application-specific degraded posture configured
- **WHEN** an application is configured for claim shaping
- **THEN** Syndra MUST require a documented degraded-mode posture for cache miss, timeout, malformed cache data, or unavailable dependencies
- **AND** the allowed posture MUST be either fail-closed or an explicitly documented minimal safe fallback

### Scenario: Implicit fallback prohibited
- **WHEN** the production claim path encounters a failure condition and no degraded-mode posture was configured
- **THEN** the configuration MUST be treated as incomplete
- **AND** the system MUST reject or block that production claim path until the posture is explicitly defined

## Actions v2 Target Deployment
The Zitadel Actions v2 target configuration and deployment assets MUST be maintained in the Syndra repository with operator documentation.

### Scenario: Target configuration is version-controlled
- **WHEN** Syndra is deployed against a Zitadel instance
- **THEN** `zitadel/actions/targets.json`, `zitadel/actions/register.sh`, a DEPLOY guide, and the failure-mode smoke test MUST be available in the repository
- **AND** the target + executions MUST be creatable by running a single operator command
- **AND** the signing key returned at target creation MUST be captured to a 0600-mode file excluded from version control.

### Scenario: Rollback preserves token issuance
- **WHEN** the operator unbinds the executions via `register.sh --remove`
- **THEN** token issuance MUST continue with stock Zitadel claims
- **AND** `restCall.interruptOnError: false` MUST be preserved so Syndra outages never block users.

## Syndra Replaces the Signing Keys Itself
*Superseded 5 Nov 2026: this section used to read "Signing Key Rotation is Policy-Driven, Not Scheduled", with Syndra forbidden to schedule rotation. The owner reversed that ("Automatic + button"); see `changes/action-key-autorotation` and ADR-005.*

Zitadel does not expire the Action target signing key: `UpdateTargetRequest.expiration_signing_key` is a rotation trigger, and current Zitadel only accepts `"0s"`, an immediate hard swap. The limit (default 90 days, `ZITADEL_ACTION_SIGNING_KEY_ROTATION_THRESHOLD_DAYS`) is Syndra's policy, and Syndra keeps it.

**The schedule.** Syndra MUST replace the keys of both of its targets on its own. It does so once the claims key's observed age reaches the threshold minus a lead (default 5 days), inside a quiet window (default 03:00–05:00 local), and only when Zitadel answers and the L1 probe passed in the last 5 minutes. It makes at most one automatic attempt per window. `SYNDRA_ACTION_KEY_AUTOROTATE=off` turns it off.

**Each replacement:**
- MUST hold a single-flight lock.
- MUST be refused within 24h of the last one, unless forced from the button.
- MUST write its trace before the Management API call.
- MUST publish the new key to the keyring from Zitadel's answer before anything else.
- MUST keep the previous key accepted for the overlap.
- MUST run the test sign-in, and record the outcome and an audit row.

**When the test sign-in fails:**
- MUST keep both keys accepted.
- MUST raise a `KEY_ROTATION` claims finding.
- MUST NOT retry on its own.

`make zitadel-actions-rotate-key` stays as the manual fallback. Syndra notices its key change through the keyring, so no `.env` edit or restart follows it.

### Scenario: The key is replaced before it is due
- **WHEN** the claims key's age reaches threshold − lead and the quiet window opens with Zitadel answering and L1 passing
- **THEN** Syndra replaces both keys, verifies with them at once in every process, runs the test sign-in, and records `verified` with the next due date
- **AND** no call signed with either the new or the old key is refused during the overlap.

### Scenario: The test sign-in fails after a replacement
- **WHEN** the check after a replacement does not pass
- **THEN** the attempt is recorded `unverified`, both keys stay accepted for 7 days, and `/healthz/claims` is degraded with `KEY_ROTATION:unverified`
- **AND** Syndra does not replace the key again on its own.

## Rotation Status MUST be Observable to Operators
Operators need an in-app signal of how old the key is, when it is due, and what Syndra's schedule will do about it. Syndra MUST expose `GET /api/v1/zitadel/action-rotation-status` (gated by `withOperatorAuth`) and MUST render it on `/zitadel` in the admin UI. The response returns:
- `{last_rotated_at, last_rotated_source, age_days, due_at, threshold_days, status, rotate_command, register_command}`;
- `auto{on, off_reason, lead_days, window, next_attempt_at}`;
- `rotating`;
- `last_outcome`.

*Since action-key-autorotation:*
- **Age is observed.** `last_rotated_at` is Syndra's own record of its last replacement, or the keyring seeing Zitadel's key change, whichever is later. `ZITADEL_ACTION_SIGNING_KEY_ROTATED_AT` below is read only while neither exists.
- **`key_installed`** is true when the keyring or the env holds a key.

The `status` field MUST use this ladder (highest precedence first):

- `disabled` — `ZITADEL_ACTION_SIGNING_KEY` is unset. Signature verification is off; the backend is passing every Action request through unchecked. Rotation age is not meaningful in this state and MUST NOT be surfaced — the response MUST report `disabled` regardless of `ZITADEL_ACTION_SIGNING_KEY_ROTATED_AT`, and MUST omit `last_rotated_at` and `age_days`.
- `unknown` — key is installed, but the rotation timestamp is unset, not RFC3339-parseable, or **in the future** (clock skew or typo). Future-dated timestamps MUST NOT be silently clamped to age 0 — that would suppress `warn`/`stale` indefinitely and defeat the operator-warning purpose of the panel.
- `ok` — age < threshold.
- `warn` — threshold ≤ age < 2× threshold.
- `stale` — age ≥ 2× threshold.

The response MUST also include a `key_installed` boolean so callers can distinguish `disabled` from `unknown` programmatically without parsing status strings.

Configuration is via two env vars:

- `ZITADEL_ACTION_SIGNING_KEY_ROTATED_AT` — RFC3339 UTC. After each rotation, `rotate.sh` writes the new value (alongside the new signing key) to `zitadel/actions/.action-env.fragment` (mode 0600); operators apply it with `cat … >> .env` (or `sudo install -m 0600 …` for systemd `EnvironmentFile` deploys), never by copy-paste. On a fresh install before any rotate, seed the value manually to `date -u +%Y-%m-%dT%H:%M:%SZ`. When unset or unparseable the endpoint reports `status=unknown`.
- `ZITADEL_ACTION_SIGNING_KEY_ROTATION_THRESHOLD_DAYS` — positive integer. Default 90 (common compliance cadence). Non-positive or non-numeric values MUST fall back to the default with a logged warning.

### Scenario: Fresh rotation reports status=ok
- **GIVEN** `ZITADEL_ACTION_SIGNING_KEY_ROTATED_AT` was set to a timestamp within the last `threshold_days`
- **WHEN** an operator requests `GET /api/v1/zitadel/action-rotation-status`
- **THEN** the response `status` MUST be `"ok"`
- **AND** `age_days` and `last_rotated_at` MUST be populated
- **AND** `rotate_command` MUST be `"make zitadel-actions-rotate-key"`, prefixed with `cd <SYNDRA_SERVER_DIR> && ` when the server's checkout is known.

### Scenario: Aged key reports warn or stale
- **GIVEN** the signing key was rotated more than `threshold_days` ago but less than `2 × threshold_days`
- **WHEN** the status endpoint is called
- **THEN** the response `status` MUST be `"warn"`
- **AND** when the age exceeds `2 × threshold_days` the `status` MUST be `"stale"`.

### Scenario: Missing or malformed timestamp reports unknown
- **GIVEN** `ZITADEL_ACTION_SIGNING_KEY` is set AND `ZITADEL_ACTION_SIGNING_KEY_ROTATED_AT` is unset or not valid RFC3339
- **WHEN** the status endpoint is called
- **THEN** the response `status` MUST be `"unknown"`
- **AND** `last_rotated_at` and `age_days` MUST be absent
- **AND** the error MUST NOT surface through a non-200 HTTP status (the UI renders the `unknown` state; callers MUST NOT have to special-case transport failures for expected missing-config states).

### Scenario: Future-dated timestamp reports unknown, not ok
- **GIVEN** `ZITADEL_ACTION_SIGNING_KEY_ROTATED_AT` parses as RFC3339 but is in the future (e.g. clock skew or a year-digit typo)
- **WHEN** the status endpoint is called
- **THEN** the response `status` MUST be `"unknown"`
- **AND** the endpoint MUST NOT silently clamp the age to 0 and report `"ok"` — a future-dated timestamp is a configuration error, not a fresh rotation, and the warn/stale ladder MUST remain reachable once the timestamp is corrected.

### Scenario: Signing key not installed reports disabled
- **GIVEN** `ZITADEL_ACTION_SIGNING_KEY` is unset on the backend (dev-mode pass-through)
- **WHEN** the status endpoint is called
- **THEN** the response `status` MUST be `"disabled"`
- **AND** the response `key_installed` MUST be `false`
- **AND** `last_rotated_at` and `age_days` MUST be absent regardless of whether `ZITADEL_ACTION_SIGNING_KEY_ROTATED_AT` is set
- **AND** the UI MUST render this as a production misconfiguration (destructive badge + explanatory text pointing at the missing env var), not as a benign missing-config state.

### Scenario: UI panel is read-only and cannot trigger rotation
- **WHEN** an operator views the Rotation Status panel on `/zitadel`
- **THEN** the panel MUST display the status badge, age, threshold, last-rotated timestamp, and a copyable shell-command snippet
- **AND** the panel MUST NOT render any control that, when clicked, actually rotates the key
- **AND** the command snippet MUST be the same value the backend returns in `rotate_command` (so the UI does not hard-code a divergent command).
