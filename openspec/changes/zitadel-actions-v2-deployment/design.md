# Design: Zitadel Actions v2 Deployment

> **Status:** Complete | [< Proposal](proposal.md) | [Tasks](tasks.md) | [IMPLEMENTATION](IMPLEMENTATION.md) | [DEPLOY](DEPLOY.md)

## Context

The Syndra data plane was designed around Zitadel **Actions v2** as the sole
claim-shaping boundary, but the Zitadel side was never wired. Implementation
planning surfaced that the existing `HandleActionInject` was built against a
**v1** mental model (embedded JS, `SetCustomClaims`, `{user_id, project_id}` →
`{customClaims}`), which does not match the real v2 protocol.

This change lands the real v2 integration and corrects the spec language.

## Decisions

### D1. v2 is a webhook target, not a script runtime

Zitadel Actions v1 embedded a JavaScript runtime; v2 does not. Instead Zitadel
POSTs a function-trigger payload to a URL of the operator's choice and expects
a response envelope with typed arrays (`append_claims`, `append_log_claims`,
`set_user_metadata`). The "script source in repo" requirement therefore
becomes:

* the `HandleActionInject` HTTP handler (backend),
* the HMAC signature middleware (backend),
* `zitadel/actions/targets.json` — declarative target + execution config,
* `zitadel/actions/register.sh` — idempotent installer against the v2 Actions API.

No `.js` file ships in this repo because none runs in Zitadel.

### D2. Use `restCall` target type (NOT `restWebhook`)

Zitadel v2 Target has three mutually-exclusive type submessages inside the
`target_type` oneof (per `proto/zitadel/action/v2/target.proto`): `rest_webhook`, `rest_call`, `rest_async`. Their
semantics differ:

* **`restWebhook`** — Zitadel POSTs the payload and only inspects the HTTP
  status code. The response body is ignored.
* **`restCall`** — Zitadel POSTs the payload and parses the response body,
  merging its contents back into the token/userinfo pipeline.
* **`restAsync`** — fire-and-forget; no waiting, no response processing.

Claim injection requires `restCall`: Syndra returns the
`{append_claims:[...]}` envelope in the response body and that body IS the
payload Zitadel merges into the issued token. A webhook target would make the
deployment a functional no-op — the HTTP 200 would come back but Zitadel
would drop the envelope on the floor.

This was the single most important correctness review finding. Targets.json
must use:

```json
{
  "name": "syndra-claim-injector",
  "endpoint": "...",
  "timeout": "3s",
  "restCall": { "interruptOnError": false }
}
```

with `name`, `endpoint`, `timeout`, and `payloadType` at the top level of
the Target (per `proto/zitadel/action/v2/target.proto`), and
`interruptOnError` inside the `restCall` submessage.

### D3. Single target, per-project degraded posture handled server-side

`application-claims/spec.md` requires per-project failure posture (`fail_closed`
vs `minimal_safe`). Zitadel v2 targets configure `restCall.interruptOnError`
**per target, not per project** — meaning one choice applies to every project
the target serves.

Chosen workaround: a single target with `interruptOnError: false`. Syndra's
backend is authoritative; the handler reads `claim_failure_mode` from the DB
per `projectId` and emits either empty `append_claims` (fail_closed) or the
configured minimal claim set (minimal_safe). The target-level setting becomes
a transport-layer safety net, not the posture decision.

Alternative rejected: one target per project. Would scale O(n) in target count
and duplicate HMAC-key management; per-project posture would become
configuration not runtime logic.

### D4. Multi-grant project resolution via namespacing

A `preaccesstoken` payload's `user_grants` can contain zero, one, or many
projects. The spec's scenario "Printing Portal only gets Printing roles"
assumes single-project scope, but nothing in the v2 payload schema guarantees
audience-filtered grants.

Resolution:

* **0 projects:** emit empty `append_claims`. No DB lookup. Observable via log.
* **1 project:** flat claim keys (no namespace). Preserves the single-project
  spec scenario literally.
* **2+ projects:** namespaced keys `syndra.<projectID>.<key>`. Deterministic,
  collision-free, machine-parseable by consumers.

Degraded paths are resolved per-project. A cache miss on project A does not
suppress claims from project B — each project's response is independent and
merged only at the envelope level.

### D5. HMAC verification in stdlib, not via the zitadel-go package

Zitadel-go ships `pkg/actions/signing.go` implementing `ValidateRequestPayload`.
Importing it would add the `github.com/zitadel/zitadel-go/v3` dependency to
the handlers package. We re-implement the algorithm in stdlib
(`crypto/hmac`, `crypto/sha256`, `encoding/hex`) to:

* Keep the `handlers` package self-contained and test-fast.
* Match the existing stdlib-only webhook HMAC pattern in `webhook.go`.
* Isolate any future upstream signature-format changes to a single file.

Algorithm (verified against `zitadel-go@main/pkg/actions/signing.go`):

```
header:  ZITADEL-Signature: t=<unix>,v1=<hex>[,v1=<hex>...]
mac:     HMAC-SHA256(secret, <unix_decimal_string> + "." + <raw_body>)
tolerance: 300s (Zitadel DefaultTolerance)
multiple v1= entries: accepted (key rotation) — any match passes
```

### D6. Lenient JSON decoding scoped to this endpoint only

The v2 payload is Zitadel-owned and will extend over time. `decodeJSONStrict`
(with `DisallowUnknownFields`) would reject every Zitadel-side schema addition
as a 400. Instead, we add `decodeJSONLenient` alongside the strict variant,
used **exclusively** by `HandleActionInject`. Other mutation endpoints keep
strict decoding — the spec's "unsupported format blocked" scenarios still
apply to Syndra-owned payloads.

### D7. Dev-mode pass-through matches existing security-boundary philosophy

`withUserAuth` falls through to `withAPIKeyAuth` when `ZITADEL_DOMAIN` is
unset. The new `withZitadelActionSignature` middleware follows the same
philosophy: when `ZITADEL_ACTION_SIGNING_KEY` is empty, requests pass through
with a warning log. This keeps local `go run ./cmd/api` workable without
spinning up a real Zitadel instance or fake-signing every smoke-test payload.
Never relied on in any environment that accepts real Zitadel traffic.

### D8. `interruptOnError: false` keeps Syndra outages off the user path

If Syndra is unreachable or returns 500, Zitadel with
`interruptOnError: false` issues the token with stock Zitadel claims only.
Users see a graceful degradation; custom claims simply disappear until Syndra
returns. Because Syndra already handles fail_closed vs minimal_safe
server-side, this settings choice is purely a transport-layer safety net —
rollback via `register.sh --remove` uses the same property.

## Rejected alternatives

* **Importing `zitadel-go/v3/pkg/actions.ValidateRequestPayload` directly.**
  Rejected per D4.
* **Requiring `project_id` in the request body** (as the v1-shaped handler
  did). The v2 payload doesn't include a scalar project ID; grants are the
  only signal, and multi-grant responses need a coherent answer. Rejected.
* **Per-project targets.** Rejected per D2.
* **Emitting `set_user_metadata` alongside `append_claims`.** No spec
  requirement; base64 encoding adds a payload footgun. Deferred.

## Ship-blocking verifications performed

* `github.com/zitadel/zitadel-go/main/pkg/actions/signing.go` confirmed via
  raw fetch on 2026-04-23:
  * Header name: `ZITADEL-Signature` (constant `SigningHeader`).
  * Hash input: `<unix_ts>.<body>` (via `fmt.Fprintf(mac, "%d", t.Unix())` +
    `mac.Write([]byte("."))` + `mac.Write(payload)`).
  * Encoding: lowercase hex.
  * Tolerance: 300 s.
  * Multiple v1= pairs allowed.
* Decision recorded here so any future Zitadel protocol change is caught
  against this reference.
* `proto/zitadel/action/v2/target.proto`, `execution.proto`, and `query.proto`
  confirmed via raw fetch on 2026-04-24 after a review pass flagged that my
  first correction pass was citing the older `v2beta/` directory. The wire
  API lives at the stable `/v2/actions/*` path; the Zitadel repo keeps both
  `proto/zitadel/action/v2/` (current) and `proto/zitadel/action/v2beta/`
  (predecessor) side by side. All message shapes verified below match between
  v2 stable and v2beta — the correction is to cite the stable source of
  truth:
  * Target fields `name`, `endpoint`, `timeout`, `signing_key` live at the
    top level. `interrupt_on_error` lives inside the `rest_webhook` /
    `rest_call` submessage.
  * Actions v2 API base path is `/v2/actions/*`, not `/resources/v3beta/actions/*`.
  * `CreateTarget`: `POST /v2/actions/targets`. `UpdateTarget`:
    `POST /v2/actions/targets/{id}` (NOT `PATCH`). `DeleteTarget`:
    `DELETE /v2/actions/targets/{id}`.
  * `ListTargets` (search by name): `POST /v2/actions/targets/search` with
    body `{filters:[{target_name_filter:{target_name:"...", method:"TEXT_FILTER_METHOD_EQUALS"}}], pagination:{...}}`.
    No leading underscore in the path segment. The inner field is
    `target_name` (NOT `name`) per `TargetNameFilter` in
    `proto/zitadel/action/v2/query.proto`, which references
    `zitadel.filter.v2.TextFilterMethod` for the method enum. Sending `name`
    silently matches nothing, which breaks idempotent reruns — the script
    would fall into the create path and produce duplicate targets.
  * `SetExecution`: `PUT /v2/actions/executions` with body
    `{condition:{function:{name:"preaccesstoken"}}, targets:["<id>", ...]}`.
    `targets` is `array<string>` (target IDs), NOT `[{target:"<id>"}]`.
  * Removing a binding: same `PUT` endpoint with `targets: []` — no DELETE on
    `/executions`.
  * Function trigger names are lowercase bare strings (`preaccesstoken`,
    `preuserinfo`), per the testing-function walkthrough. The capitalized
    variants in the concepts page (`PreAccessToken`, `PreUserinfo`) are
    Zitadel's internal enum names.
  * Top-level Target additionally carries `payload_type` — setting
    `"payloadType": "PAYLOAD_TYPE_JSON"` explicitly (rather than relying on
    the default) immunizes the manifest against a future default change.
* Key rotation without recreating the target: `POST /v2/actions/targets/{id}`
  with body `{"expirationSigningKey":"0s"}` returns a new `signingKey`.

## Known risks carried into implementation

1. **Zitadel protocol drift** — if Zitadel changes the header name, timestamp
   field, or signature composition, verification breaks silently. Mitigation:
   HMAC constants are centralized in `zitadel_action_auth.go`; a future
   version check can be added if Zitadel adds one.
2. **Payload size** — existing `withMaxBody` caps at 1 MB. Users with very
   many `user_grants` could approach it. Monitor; raise to 4 MB only when
   observed.
3. **v1 Action collision** — on self-hosted Zitadel, an operator could still
   create a v1 Action with the same trigger and double-inject. Mitigation:
   `DEPLOY.md` warns operators. Detection would require querying Zitadel's
   v1 Actions list, which is not in scope here.
4. **Signing-key rotation semantics** — Zitadel has no documented key-rotation
   API at target level (as of 2026-04-23). Documented in `SIGNING_KEY.md` as
   "recreate target → new key" with a brief verification gap during which
   `interruptOnError: false` keeps users unblocked.
