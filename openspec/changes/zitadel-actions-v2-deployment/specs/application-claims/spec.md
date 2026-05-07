## MODIFIED Requirements

### Requirement: Implementation — Zitadel Actions v2 Integration

The data plane MUST be implemented as a **Zitadel Actions v2** target, invoked during the `function/preaccesstoken` and `function/preuserinfo` triggers, returning the v2 response envelope.

Actions v2 is **not** a JavaScript runtime. Zitadel POSTs the function trigger payload (`user`, `user_grants`, `org`, `userinfo`, ...) to the configured HTTP target, and the target MUST respond with a typed envelope containing `append_claims` and (optionally) `append_log_claims` / `set_user_metadata`. Any earlier spec language referring to "`SetCustomClaims`", a "v2 `claims` namespace", or an in-repo JavaScript script was factually incorrect and has been removed.

* **append_claims contract**: the target MUST emit each claim as a `{key, value}` object inside the `append_claims` array. Merging semantics are Zitadel's; MkAuth MUST NOT assume behavior beyond what Zitadel documents.
* **Cache-backed source**: claim values are looked up in Redis using `mapping:<userID>:<projectID>` keys populated by the cache compiler; Redis miss and malformed cache data are handled by the project's configured degraded posture.
* **Multi-project resolution**: when the trigger payload's `user_grants` names more than one project, the response MUST emit namespaced claim keys `mkauth.<projectID>.<claim>` so claims from different projects cannot collide in the issued token. When exactly one project is present, flat keys MUST be used so single-project applications receive the unprefixed payload they expect.
* **Per-project degraded posture preserved**: degraded responses MUST be decided per project. A cache miss on project A MUST NOT suppress claims from project B in the same response.
* **Availability Rule**: the integration MUST define explicit behavior for cache miss, backend timeout, malformed cache data, and unavailable downstream dependencies.
* **Safe Failure Rule**: failure behavior MUST be either `fail_closed` (empty `append_claims` for that project) or `minimal_safe` (configured minimal claim set for that project), depending on the application's documented security posture.
* **Performance Goal**: the path SHOULD be low-latency; reliability and deterministic failure behavior take precedence over a fixed micro-latency target.
* **Compatibility Boundary**: Zitadel Actions v2 MUST remain the only supported source-of-truth-facing claim integration model. Zitadel Actions v1 MUST NOT coexist with the v2 target on the same trigger.

#### Scenario: Cache miss during claim injection
- **WHEN** the Actions v2 target request triggers a cache lookup and no compiled entry exists for that project
- **THEN** MkAuth MUST return the documented safe fallback envelope for that application's configured posture
- **AND** other projects in the same multi-grant response MUST NOT be suppressed by this project's degradation
- **AND** the outcome MUST be observable via the `[DATA PLANE]` log line.

#### Scenario: Unsupported degraded behavior blocked
- **WHEN** an application has no documented failure posture for claim injection
- **THEN** the configuration MUST be rejected as incomplete.

#### Scenario: Request signature verification
- **WHEN** the backend receives a request at `/api/action/inject` and `ZITADEL_ACTION_SIGNING_KEY` is set
- **THEN** the request MUST carry a `ZITADEL-Signature: t=<unix>,v1=<hex>[,v1=<hex>...]` header
- **AND** the backend MUST verify at least one `v1=` value is HMAC-SHA256(`<unix_decimal>.<body>`) under the configured signing key
- **AND** the backend MUST reject requests whose timestamp is outside a 300-second tolerance window
- **AND** requests that fail verification MUST receive `401 INVALID_SIGNATURE` without leaking which verification step failed.

#### Scenario: Multiple `v1=` signatures during key rotation
- **WHEN** Zitadel sends a `ZITADEL-Signature` header with more than one `v1=<hex>` entry
- **THEN** the backend MUST accept the request if any `v1=` entry matches the expected HMAC
- **AND** the backend MUST NOT require all entries to match (entries may be rotated keys).

### Requirement: Actions v2 Target Deployment

The Zitadel Actions v2 target configuration and deployment assets MUST be maintained in the MkAuth repository.

* **Target manifest**: `zitadel/actions/targets.json` MUST declare the target using the `restCall` type (NOT `restWebhook` or `restAsync`). Webhook targets only inspect the HTTP status code; call targets parse the response body and merge it into the issued token — which is required here because the claim envelope lives in the response body. The manifest layout MUST match the stable Zitadel v2 Target proto (`proto/zitadel/action/v2/target.proto`): `name`, `endpoint`, `timeout`, and `payloadType` live at the top level of the target; `interruptOnError` lives inside the `restCall` submessage. `payloadType` SHOULD be set explicitly to `PAYLOAD_TYPE_JSON`. The execution bindings MUST cover `function.name = "preaccesstoken"` and `function.name = "preuserinfo"`.
* **Registration script**: `zitadel/actions/register.sh` MUST apply the manifest idempotently against the stable Zitadel v2 Actions REST API (`POST /v2/actions/targets` to create, `POST /v2/actions/targets/{id}` to update, `POST /v2/actions/targets/search` to look up by name using the `target_name_filter.target_name` field from `proto/zitadel/action/v2/query.proto`, `PUT /v2/actions/executions` to bind/unbind), capture the one-time signing key returned at target creation, and support a `--remove` path that unbinds executions (via `PUT /v2/actions/executions` with `targets: []`) without destroying the target. The `--remove` path MUST be idempotent — Zitadel returns HTTP 404 (`COMMAND-74aaqj8fv9` "Execution condition is invalid") from the unbind PUT when no execution row matches the condition (already-removed, partially-applied, or never-bound state), and the script MUST treat that response as success rather than aborting cleanup.
* **Full-teardown path**: `register.sh` MUST also support a `--purge` mode that runs the full `--remove` unbind sequence and then deletes each manifest target via `DELETE /v2/actions/targets/{id}` and removes the local `.action-signing-key.<name>{,.previous,.rotated_at}` and `.action-env.fragment` files. The DELETE step MUST run after the unbind loop (Zitadel refuses to delete a target still referenced by an execution) and MUST tolerate HTTP 404 so re-running `--purge` against an already-clean instance is idempotent. The mode MUST surface an operator follow-up reminder to clear the relevant signing-key env vars from `.env` and restart the backend before any subsequent `register.sh`, because the next registration will mint fresh signing keys that will not match stale env values.
* **SetExecution payload**: binding MUST send `targets` as an array of target-ID strings, never as an array of wrapper objects. Unbinding MUST use the same `PUT` endpoint with an empty `targets` array — there is no dedicated DELETE for an execution binding.
* **Deployment guide**: the repository MUST contain an operator-facing DEPLOY.md describing prerequisites, step-by-step registration, signing-key injection, end-to-end validation, and rollback.
* **Signing-key handling**: the captured signing key MUST be excluded from version control and stored with at least mode `0600`. In-place rotation MUST be performed via `POST /v2/actions/targets/{id}` with `{"expirationSigningKey":"0s"}`, capturing the new key from the response.
* **Failure-mode validation**: a repository-resident smoke test MUST exercise the target endpoint and assert that the response conforms to the v2 envelope shape (`append_claims` array, each entry is `{key, value}`), optionally signing the test request when the key is available.

#### Scenario: Target configuration is version-controlled
- **WHEN** MkAuth is deployed against a Zitadel instance
- **THEN** `zitadel/actions/targets.json`, `zitadel/actions/register.sh`, the operator DEPLOY.md, and the smoke-test script MUST be present in the repository
- **AND** the target and executions MUST be creatable by running a single operator command against a live Zitadel.

#### Scenario: Rolling back the target does not break token issuance
- **WHEN** the operator runs `register.sh --remove`
- **THEN** token issuance MUST continue with stock Zitadel claims (no user-facing outage)
- **AND** the `restCall.interruptOnError: false` posture MUST be preserved so MkAuth outages are never user-visible through the token path.

#### Scenario: Removing already-absent executions succeeds
- **WHEN** the operator runs `register.sh --remove` against an instance where one or more manifest executions were never bound or have already been unbound
- **THEN** the script MUST exit 0 and report each manifest execution as removed
- **AND** Zitadel's HTTP 404 (`COMMAND-74aaqj8fv9` "Execution condition is invalid") on the unbind PUT MUST be treated as the desired post-state, not as a failure.

#### Scenario: Purging the deployment performs full teardown
- **WHEN** the operator runs `register.sh --purge`
- **THEN** the script MUST first unbind every manifest execution (the same sequence as `--remove`)
- **AND** then call `DELETE /v2/actions/targets/{id}` for every manifest target whose ID was resolved
- **AND** delete the local `.action-signing-key.<name>{,.previous,.rotated_at}` and `.action-env.fragment` files
- **AND** print an operator-facing follow-up requiring `ZITADEL_ACTION_SIGNING_KEY` / `ZITADEL_EVENT_SIGNING_KEY` to be cleared from `.env` and the backend restarted before any subsequent `register.sh` invocation
- **AND** treat HTTP 404 from the DELETE call as success so re-running `--purge` is idempotent.

### Requirement: Strict Request Validation at the MkAuth Boundary

The MkAuth boundary MUST treat the Zitadel-owned v2 payload schema as evolvable while still validating the fields MkAuth depends on.

* **Lenient field acceptance**: the `/api/action/inject` endpoint MUST use a lenient JSON decoder that silently ignores unknown top-level fields (so future Zitadel payload additions do not break verification).
* **Required-field enforcement**: the endpoint MUST reject any request where `user.id` is empty with a `400 VALIDATION_FAILED` response carrying a `user.id` detail key.
* **Trailing-token guard**: the endpoint MUST reject bodies containing more than one JSON value.
* **Boundary preservation**: the lenient decoder MUST be scoped to this endpoint; all other MkAuth mutation endpoints MUST continue to use the strict decoder with `DisallowUnknownFields`.

#### Scenario: Unknown Zitadel fields accepted
- **WHEN** Zitadel sends a v2 payload containing fields MkAuth does not model (e.g. `org`, `user_metadata`, `userinfo`)
- **THEN** the handler MUST accept the request and act on the fields it does model
- **AND** MUST NOT return `400` solely because of unknown fields.

#### Scenario: Missing `user.id` rejected
- **WHEN** a v2 payload arrives with `user.id` absent or empty
- **THEN** the handler MUST return `400 VALIDATION_FAILED` with a `user.id` detail key
- **AND** MUST NOT attempt a cache lookup.
