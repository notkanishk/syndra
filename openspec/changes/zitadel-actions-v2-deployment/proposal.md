## Why

`design.md §Data Plane` and `application-claims/spec.md §Compatibility Boundary` designate **Zitadel Actions v2** as the sole source-of-truth-facing claim-injection path for MkAuth. Phases 1–4 built the receiving end: `POST /api/action/inject`, per-project degraded posture (`fail_closed` | `minimal_safe`), a 50 ms Redis timeout budget, and handler-level tests. What never existed was the Zitadel side of the contract — no target, no execution binding, no signature verification. Until that gap closes, every grant, bundle, mapping rule, webhook, and sync intent emits into a cache that nothing consumes.

While planning the deployment we surfaced a factual error baked into the existing spec and handler:

* **Actions v2 is not a JavaScript runtime.** That was v1. v2 is a webhook target with a strictly defined response envelope (`append_claims`, `append_log_claims`, `set_user_metadata`) and an HMAC-SHA256 request signature.
* The existing handler shaped its input as `{user_id, project_id}` and its output as `{customClaims: {...}}` — neither matches the real v2 wire contract. The `application-claims` spec repeats that shape (`SetCustomClaims`, "v2 `claims` namespace"), which are v1 APIs.

This change ships the real production claim-injection path and corrects the spec language so future reviewers aren't misled.

## What Changes

**Backend reshape** (`backend/internal/handlers/action.go`)

* Replace `ActionRequest`/`ActionResponse` with `ActionV2Request`/`ActionV2Response` matching the documented v2 contract.
* Accept Zitadel's large, evolving function payload via a new `decodeJSONLenient` helper scoped to this endpoint. All other endpoints continue to use `decodeJSONStrict`.
* Multi-grant project resolution: for a single `projectId` in `user_grants`, emit flat claim keys (preserves the "Printing Portal only gets Printing roles" scenario); for multiple projects, emit `mkauth.<projectID>.<key>` namespaced keys so claims cannot collide across project scopes.
* Preserve `fail_closed` / `minimal_safe` per-project degraded semantics, wrapped in the v2 envelope. Per-project degradation MUST NOT block sibling projects in a multi-grant response.

**HMAC signature middleware** (new: `backend/internal/handlers/zitadel_action_auth.go`)

* Re-implements `github.com/zitadel/zitadel-go/v3/pkg/actions` signing in stdlib. Verifies the `ZITADEL-Signature` header (format `t=<unix>,v1=<hex>[,v1=<hex>...]`) using HMAC-SHA256 over `<ts>.<body>`, with 300 s tolerance matching Zitadel's default.
* Accepts multiple `v1=` pairs so signing-key rotation works without an outage.
* Dev-mode pass-through when `ZITADEL_ACTION_SIGNING_KEY` is unset, mirroring the existing `withUserAuth` dev fallback.
* Wired into `router.go:110` — `/api/action/inject` is now wrapped `withCORS(withZitadelActionSignature("ZITADEL_ACTION_SIGNING_KEY", ...))`.

**Deployment artifacts** (new: `zitadel/actions/`)

* `targets.json` — declarative target + execution config (endpoint, 3 s timeout, `preaccesstoken` + `preuserinfo` bindings, `interruptOnError: false`).
* `register.sh` — idempotent installer that reads `targets.json`, upserts the target, captures the one-time `signingKey`, and binds/unbinds executions. `--remove` deletes bindings.
* `README.md`, `SIGNING_KEY.md` — operator docs and credential-handling notes.
* `.gitignore` excludes `.action-signing-key`.
* `Makefile` gains `zitadel-actions-register`, `zitadel-actions-remove`, `zitadel-actions-verify`.
* `scripts/smoke-test-action-v2.sh` — wire-level round trip that asserts the v2 envelope shape, optionally signing the request when `ZITADEL_ACTION_SIGNING_KEY` is set.
* `.env.example` + `docker-compose.yml` expose the new `ZITADEL_ACTION_SIGNING_KEY` env var.

**Tests** (zero new runners, zero new deps)

* `backend/internal/handlers/action_test.go` — 12 scenario tests on the reshaped handler: cache-miss fail_closed / minimal_safe, malformed cache, DB outage default, cache-hit single-project flat keys, multi-project namespaced keys, multi-project partial degrade, empty grants, duplicate-grant dedup, missing `user.id`, invalid JSON, unknown fields accepted by lenient decoder.
* `backend/internal/handlers/action_v2_contract_test.go` — verbatim canonical Zitadel v2 function payload round-trip.
* `backend/internal/handlers/zitadel_action_auth_test.go` — 7 middleware scenarios: valid signature, body rewind for downstream decoder, invalid signature rejected, missing header rejected, stale timestamp rejected, dev-mode pass-through, key-rotation (multiple `v1=` sigs).

## Capabilities

### Modified Capabilities

* **`application-claims`** — flip from Partial to Integrated. Status header moves from "Partial (Actions v2 script not in repo, deferred P5)" to "Complete". Section §Implementation is rewritten to replace v1 wording (`SetCustomClaims`, "v2 `claims` namespace") with the real v2 envelope (`append_claims[]`, `append_log_claims[]`). The §Actions v2 Script Deployment scenario text is adjusted from "script source" language to "target configuration + HMAC-verified endpoint + deployment script" to reflect what v2 actually deploys. The trailing "Deferred to Phase 5" paragraph is removed.

## Impact

* **New files:** `backend/internal/handlers/zitadel_action_auth.go` + `_test.go`, `backend/internal/handlers/action_v2_contract_test.go`, `zitadel/actions/{README.md,targets.json,register.sh,SIGNING_KEY.md,.gitignore}`, `scripts/smoke-test-action-v2.sh`, `openspec/changes/zitadel-actions-v2-deployment/*`.
* **Modified files:** `backend/internal/handlers/{contracts.go,action.go,action_test.go,contracts_test.go,router.go}`, `Makefile`, `.env.example`, `docker-compose.yml`, `openspec/INDEX.md`, `openspec/changes/mkauth-core-architecture/{ROADMAP.md,specs/application-claims/spec.md,specs/feature-coverage.md}`.
* **Dependencies:** zero new Go modules. zero new npm packages. `register.sh` needs `curl` + `jq` on the operator host (already assumed).
* **Runtime:** production backend needs `ZITADEL_ACTION_SIGNING_KEY` set after the first `register.sh` run. Until set, the middleware runs in dev pass-through (unchanged behavior).
* **Risk:** correcting the spec's v1-era wording is a documentation-facing change, not a behavioral change. The backend reshape is a breaking wire-format change, but `/api/action/inject` has never been called by a live Zitadel (no target existed), so there is no upstream consumer to migrate.
* **Phase advance:** Phase 5 > Operations > "Actions v2 Deployment" box ticks. `feature-coverage.md` row for `application-claims` flips Partial → Integrated.
