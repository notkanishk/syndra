# Implementation Record: Zitadel Event-Trigger Propagation

> **Status:** Complete | [Proposal](proposal.md) | [Design](design.md) | [Tasks](tasks.md)

## What landed

### Backend (`backend/internal/handlers/`)

- `WebhookPayload.RoleKeys []string` added alongside the existing singular `RoleKey`. Validation accepts either form; when only `RoleKey` is set, processors auto-promote it into a single-element `RoleKeys` slice. `processGrantAdded` and `processGrantRemoved` iterate `RoleKeys` and run mapping-rule enforcement / revocation per role within a single delivery (one HTTP request → one `webhook_events` row regardless of role count).
- `/api/webhooks/zitadel` is now wrapped with `withZitadelActionSignature("ZITADEL_EVENT_SIGNING_KEY", ...)` (the same middleware already used by `/api/action/inject` against the claim-injector key).
- Legacy verifier paths removed: `verifyWebhookSignature`, `verifyWebhookFreshness`, all inline signature/freshness checks in `HandleZitadelWebhook`, and the `ZITADEL_WEBHOOK_SECRET` env var (no producer existed for the legacy `X-Zitadel-Signature` HMAC dialect).
- New `webhook_translate.go` defines a lenient `ZitadelEventPayload` struct and a `translateZitadelEvent` function with explicit per-event mapping. Coverage includes `user.human.{added,selfregistered}`, `user.{deactivated,locked}`, and `user.grant.{added,changed,removed}` — deactivation and lock are user-aggregate events in Zitadel (`userEventTypePrefix + "deactivated"` / `+ "locked"`), not human-aggregate, so the translator and the manifest both use the bare `user.*` names. Unknown event types short-circuit to a `200 + log` passthrough without dispatch or persistence.
- **Wire-format correction (post-merge fix)**: the original `zitadelEventPayload` struct was built against a guessed shape (`{aggregate:{id,...}, event, payload, editorUserId}`) that does not match Zitadel's actual `ContextInfoEvent` (`zitadel/zitadel:internal/repository/execution/queue.go`). All real Zitadel events 4xx'd at validation; only the (incorrectly-shaped) smoke test passed. The struct now matches Zitadel exactly: top-level flat `aggregateID`, `aggregateType`, `event_type`, `event_payload`, `userID` (the editor). Shape detection probes `aggregateID` (not `aggregate`). Editor location collapsed to a single field. Smoke-test script and unit/handler test fixtures updated to the real shape. See `docs/superpowers/plans/2026-05-07-zitadel-event-listener-wire-format-fix.md`.
- **Grants index (`zitadel_grants_index` table, migration `000011`)**: Zitadel's `user.grant.changed` payload omits `projectId`; `user.grant.removed` omits `roleKeys`. New `enrichGrantPayload` in `webhook_translate_enrich.go` fills both via a local index populated by `grant.added` (and refreshed by `grant.changed`), with a synchronous Zitadel `ListUserGrants` fallback on local-index miss (`zitadel_grant_lookup.go`). Index ops are non-fatal — failures log but never bounce the webhook (4xx back to Zitadel triggers redelivery storms). The handler delete-path runs after `processGrantRemoved` succeeds; upsert runs after `processGrantAdded` for both `grant_added` and `grant_changed`. The translator surfaces `WebhookPayload.GrantID` (= aggregate ID) so the enrichment step can correlate index lookups.
- Self-mutation loop guard via the new `ZITADEL_M2M_USER_ID` env var: events whose top-level `userID` (Zitadel's `ContextInfoEvent` carries the editor in this field, NOT the subject) matches the backend's M2M service-user ID are dropped at the translator boundary. Unset env var emits a one-time startup warning and disables the guard (local-dev only).
- Shape detection at `HandleZitadelWebhook` peeks at the parsed JSON: top-level `aggregateID` → Zitadel-native path through the translator; top-level `event_type` (no `aggregateID`) → internal-shape passthrough (operator curl, contract tests).

### Deployment (`zitadel/actions/`)

- `targets.json` reshaped from a single-target object to a multi-target manifest: `targets[]` (each with `name`, type-specific submessage, and `_signing_key_env` annotation) plus `executions[]` whose entries reference targets by name.
- Two targets defined: `mkauth-claim-injector` (type `restCall`, 3s timeout, function triggers `preaccesstoken`/`preuserinfo`) and `mkauth-event-listener` (type `restAsync`, 5s timeout, condition.event triggers for the lifecycle events listed above).
- `register.sh` iterates `targets[]`, captures each target's signing key into `.action-signing-key.<name>` (mode 0600), maps `executions[].target` (name) → captured target IDs in one pass, and writes a multi-pair `.action-env.fragment` with `ZITADEL_ACTION_SIGNING_KEY=...` + `ZITADEL_EVENT_SIGNING_KEY=...` + their `_ROTATED_AT` companions. `--remove` unbinds every execution; missing-target lookup is gated to the non-remove path so partially-deleted targets do not block cleanup. The unbind PUT in `--remove` mode runs with `ZITADEL_API_TOLERATE_404=1` so HTTP 404 responses (`COMMAND-74aaqj8fv9` "Execution condition is invalid", emitted by Zitadel when no execution row matches the condition) are treated as success — `make zitadel-actions-remove` is therefore safe to run against partially-applied or already-clean instances.
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
- **Event-name verification**: the executions in `targets.json` cover the user-aggregate lifecycle names Zitadel actually registers — `user.human.{added,selfregistered}` (human-specific creation paths) and `user.{deactivated,locked}` plus `user.grant.{added,changed,removed}` (user/usergrant aggregate operations). Zitadel's `ExecutionEventCondition.Existing()` validator (`internal/command/action_v2_execution.go`) rejects unknown names with HTTP 404 + `COMMAND-74aaqj8fv9` "Execution condition is invalid" at SetExecution time, so the manifest names must match the prefix definitions in `internal/repository/user/` and `internal/repository/usergrant/` exactly. If a staging instance emits the legacy `user.user.grant.*` prefix, both the executions and the translator's switch need a mirrored entry; the translator already accepts the legacy prefix via dual mapping, only the registration manifest would need an addition.
- **Event-key panel observability**: `/zitadel` Rotation Status panel currently reflects only `ZITADEL_ACTION_SIGNING_KEY_ROTATED_AT`. Extending the backend status endpoint and UI panel to report both targets is tracked as future work; for now the per-target `.rotated_at` files plus the env-fragment timestamp are the operator-facing source of truth for the event-listener key.
- **Welcome-bundle UX**: the `bundles.is_welcome` flag is settable via DB but no admin UI yet. Already tracked under `automation-policies` (deferred to Phase 5 backlog).
