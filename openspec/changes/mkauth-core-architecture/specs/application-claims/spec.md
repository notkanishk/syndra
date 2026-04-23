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
- **Multi-project resolution**: when `user_grants` names more than one project, the response MUST emit namespaced claim keys `mkauth.<projectID>.<claim>` so claims across projects cannot collide in the issued token. Single-project responses use flat keys so legacy single-audience applications receive the unprefixed payload.
- **Per-project degraded posture**: degraded responses are decided per project. A cache miss on project A MUST NOT suppress claims from project B in the same response.
- **Availability Rule**: the integration MUST define explicit behavior for cache miss, backend timeout, malformed cache data, and unavailable downstream dependencies.
- **Safe Failure Rule**: failure behavior MUST be either `fail_closed` (empty `append_claims` for that project) or `minimal_safe` (configured minimal claim set), depending on the application's documented security posture.
- **Performance Goal**: the path SHOULD be low-latency; reliability and deterministic failure behavior take precedence over a fixed micro-latency target.
- **Compatibility Boundary**: Zitadel Actions v2 MUST remain the only supported source-of-truth-facing claim integration model. Zitadel Actions v1 MUST NOT coexist with the v2 target on the same trigger.

### Scenario: Cache miss during claim injection
- **WHEN** the Actions v2 target request triggers a cache lookup and no compiled entry exists
- **THEN** MkAuth MUST return the documented safe fallback envelope for that application
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
- **WHEN** MkAuth introduces or changes an internal payload between the UI, backend, or sync service
- **THEN** that change MUST NOT replace, bypass, or redefine the Zitadel Actions v2 compatibility contract
- **AND** the source-of-truth-facing claim path MUST remain anchored to the Actions v2 flow

## Production degraded behavior
The system MUST define a documented production failure posture for every application that depends on the Actions v2-compatible claim path.

### Scenario: Application-specific degraded posture configured
- **WHEN** an application is configured for claim shaping
- **THEN** MkAuth MUST require a documented degraded-mode posture for cache miss, timeout, malformed cache data, or unavailable dependencies
- **AND** the allowed posture MUST be either fail-closed or an explicitly documented minimal safe fallback

### Scenario: Implicit fallback prohibited
- **WHEN** the production claim path encounters a failure condition and no degraded-mode posture was configured
- **THEN** the configuration MUST be treated as incomplete
- **AND** the system MUST reject or block that production claim path until the posture is explicitly defined

## Actions v2 Target Deployment
The Zitadel Actions v2 target configuration and deployment assets MUST be maintained in the MkAuth repository with operator documentation.

### Scenario: Target configuration is version-controlled
- **WHEN** MkAuth is deployed against a Zitadel instance
- **THEN** `zitadel/actions/targets.json`, `zitadel/actions/register.sh`, a DEPLOY guide, and the failure-mode smoke test MUST be available in the repository
- **AND** the target + executions MUST be creatable by running a single operator command
- **AND** the signing key returned at target creation MUST be captured to a 0600-mode file excluded from version control.

### Scenario: Rollback preserves token issuance
- **WHEN** the operator unbinds the executions via `register.sh --remove`
- **THEN** token issuance MUST continue with stock Zitadel claims
- **AND** `restCall.interruptOnError: false` MUST be preserved so MkAuth outages never block users.

## Signing Key Rotation is Policy-Driven, Not Scheduled
Zitadel does not expire the Action target signing key: `CreateTargetRequest` has no expiration field, and `UpdateTargetRequest.expiration_signing_key` (per `proto/zitadel/action/v2/target.proto`) is a rotation trigger, not a TTL — current Zitadel only accepts `"0s"` (immediate hard swap). The first key returned at target creation works indefinitely. Rotation is therefore a MkAuth deployment-policy decision (incident response, compliance cadence, operator handoff), not a Zitadel requirement.

The repository MUST ship an operator-facing one-shot rotate command (`make zitadel-actions-rotate-key` invoking `zitadel/actions/rotate.sh`) that performs the target lookup, `POST /v2/actions/targets/{id}` with `{"expirationSigningKey":"0s"}`, captures the new `signingKey` from the response, and preserves the prior key for audit. MkAuth MUST NOT schedule this command in-process; scheduled rotation, if required by deployment policy, MUST be driven by an external trigger (host cron, CI job, operator runbook). Backend middleware MAY remain single-key until such time as Zitadel supports longer graceful periods on `expiration_signing_key` — at that point dual-key acceptance becomes worth adding to close the verification-gap window that exists today between Zitadel accepting the rotated key and the backend being restarted with the new env value.

### Scenario: Operator rotates the signing key on demand
- **WHEN** an operator needs to rotate the Action signing key (incident response, compliance cadence, handoff)
- **THEN** the repository MUST provide a single command (`make zitadel-actions-rotate-key`) that performs the rotation, captures the new key to a 0600-mode file, and preserves the prior key for audit
- **AND** the operator documentation MUST describe the post-rotate env-var swap + backend restart step and the verification-gap window during that swap
- **AND** MkAuth MUST NOT schedule rotation automatically — the command MUST be invoked by explicit operator or external-trigger action.

## Rotation Status MUST be Observable to Operators
Because Zitadel does not expire the signing key and MkAuth does not schedule rotation, operators need an in-app signal that a key has crossed the deployment's rotation-policy threshold. MkAuth MUST expose `GET /api/v1/zitadel/action-rotation-status` (gated by `withOperatorAuth`) returning `{last_rotated_at, age_days, threshold_days, status, rotate_command}` and MUST render a corresponding panel on `/zitadel` in the admin UI.

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
- **AND** `rotate_command` MUST be `"make zitadel-actions-rotate-key"`.

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
