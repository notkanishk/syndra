# Implementation Record: Zitadel Event-Trigger Propagation

> **Status:** Complete | [Proposal](proposal.md) | [Design](design.md) | [Tasks](tasks.md)

## What landed

### Backend (`backend/internal/handlers/`)

- `WebhookPayload.RoleKeys []string` added alongside the existing singular `RoleKey`. Validation accepts either form; when only `RoleKey` is set, processors auto-promote it into a single-element `RoleKeys` slice. `processGrantAdded` and `processGrantRemoved` iterate `RoleKeys` and run mapping-rule enforcement / revocation per role within a single delivery (one HTTP request → one `webhook_events` row regardless of role count).
- `/api/webhooks/zitadel` is now wrapped with `withZitadelActionSignature("ZITADEL_EVENT_SIGNING_KEY", ...)` (the same middleware already used by `/api/action/inject` against the claim-injector key).
- Legacy verifier paths removed: `verifyWebhookSignature`, `verifyWebhookFreshness`, all inline signature/freshness checks in `HandleZitadelWebhook`, and the `ZITADEL_WEBHOOK_SECRET` env var (no producer existed for the legacy `X-Zitadel-Signature` HMAC dialect).
- New `webhook_translate.go` defines a lenient `ZitadelEventPayload` struct and a `translateZitadelEvent` function with explicit per-event mapping. Coverage includes `user.human.{added,selfregistered,deactivated,locked}` and `user.grant.{added,changed,removed}`. Unknown event types short-circuit to a `200 + log` passthrough without dispatch or persistence.
- Self-mutation loop guard via the new `ZITADEL_M2M_USER_ID` env var: events whose `editorUserId` matches the backend's M2M service-user ID are dropped at the translator boundary. Unset env var emits a one-time startup warning and disables the guard (local-dev only).
- Shape detection at `HandleZitadelWebhook` peeks at the parsed JSON: top-level `aggregate` → Zitadel-native path through the translator; top-level `event_type` → internal-shape passthrough (operator curl, contract tests).

### Deployment (`zitadel/actions/`)

- `targets.json` reshaped from a single-target object to a multi-target manifest: `targets[]` (each with `name`, type-specific submessage, and `_signing_key_env` annotation) plus `executions[]` whose entries reference targets by name.
- Two targets defined: `mkauth-claim-injector` (type `restCall`, 3s timeout, function triggers `preaccesstoken`/`preuserinfo`) and `mkauth-event-listener` (type `restAsync`, 5s timeout, condition.event triggers for the lifecycle events listed above).
- `register.sh` iterates `targets[]`, captures each target's signing key into `.action-signing-key.<name>` (mode 0600), maps `executions[].target` (name) → captured target IDs in one pass, and writes a multi-pair `.action-env.fragment` with `ZITADEL_ACTION_SIGNING_KEY=...` + `ZITADEL_EVENT_SIGNING_KEY=...` + their `_ROTATED_AT` companions. `--remove` unbinds every execution; missing-target lookup is gated to the non-remove path so partially-deleted targets do not block cleanup.
- `rotate.sh --target NAME` rotates one target; default rotates every target. Per-target file isolation (`.action-signing-key.<name>{,.previous,.rotated_at}`) lets one signing key be rotated without touching the other.
- `Makefile` recipes: `zitadel-actions-register` registers both targets in one shot; `zitadel-actions-rotate-key TARGET=...` (optional) rotates one or all; `zitadel-actions-verify-events` runs the new smoke test.

### Ops + docs

- `zitadel/actions/EVENTS.md` — operator runbook covering subscribed events, the self-mutation guard, signing-key handling, smoke test, and troubleshooting.
- `zitadel/actions/SIGNING_KEY.md` — rewritten Lifecycle/Rotation/Storage sections to reflect per-target file naming and the multi-pair env fragment. Panel-observability scope explicitly narrowed: `/zitadel` Rotation Status currently reports only the claim-injector key; event-key timestamps are written by `rotate.sh` for forward compatibility but not yet consumed by the backend.
- `.env.example` and `docker-compose.yml` — added `ZITADEL_EVENT_SIGNING_KEY`, `ZITADEL_EVENT_SIGNING_KEY_ROTATED_AT` (forward-compat pass-through), `ZITADEL_M2M_USER_ID`. The legacy `ZITADEL_WEBHOOK_SECRET` is gone from both.
- `scripts/smoke-test-event-listener.sh` — POSTs a synthetic *unmapped* Zitadel event (`user.password.changed`) so the check exercises auth + shape detection + translator unknown-event passthrough without invoking any downstream processor. Safe against staging and production.

### OpenSpec

- New capability spec `lifecycle-event-propagation/spec.md` (this change directory).
- MODIFIED delta on `application-claims/spec.md`: new requirements for multi-target deployment, event-trigger authentication + translation, and self-mutation loop suppression.
- `openspec/INDEX.md` — change row flipped to "Complete" with impl link; capability row promoted from "Pending — see change" to "Integrated".
- `openspec/changes/mkauth-core-architecture/specs/feature-coverage.md` — `Webhook invalidation` row Notes column updated to record the producer wiring; `Last updated` bumped.
- `openspec/changes/mkauth-core-architecture/ROADMAP.md` — Phase 5 Operations gains a ticked "Event-Trigger Propagation" entry alongside the existing Actions v2 Deployment item.

## Verification performed

- `cd backend && go test ./...` — 311 pass in 14 packages.
- `cd backend && go vet ./...` — clean.
- `cd sync && go test ./...` — 32 pass in 5 packages; vet clean.
- `cd ui && bun run lint && bun run test && bun run build` — lint clean, 73 tests pass, build clean.
- `bash -n` on `register.sh`, `rotate.sh`, `smoke-test-action-v2.sh`, `smoke-test-event-listener.sh` — clean.
- `jq -e . zitadel/actions/targets.json` — valid JSON.
- `make -n zitadel-actions-{register,rotate-key,verify-events}` — recipes resolve.

## Gaps carried forward

- **Live staging smoke**: deferred until operator has Zitadel staging credentials. `make zitadel-actions-verify-events` is ready and side-effect-safe (unmapped event payload).
- **Event-name verification**: the executions in `targets.json` cover the modern `user.human.*` / `user.grant.*` names. If the staging instance emits the older `user.user.grant.*` prefix, both the executions and the translator's switch need a mirrored entry. Translator already accepts the legacy prefix via dual mapping; only the registration manifest would need an addition.
- **Event-key panel observability**: `/zitadel` Rotation Status panel currently reflects only `ZITADEL_ACTION_SIGNING_KEY_ROTATED_AT`. Extending the backend status endpoint and UI panel to report both targets is tracked as future work; for now the per-target `.rotated_at` files plus the env-fragment timestamp are the operator-facing source of truth for the event-listener key.
- **Welcome-bundle UX**: the `bundles.is_welcome` flag is settable via DB but no admin UI yet. Already tracked under `automation-policies` (deferred to Phase 5 backlog).
