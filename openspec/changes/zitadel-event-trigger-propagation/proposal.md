## Why

The `zitadel-actions-v2-deployment` change wired Actions v2 for **claim injection only** (`function.preaccesstoken`, `function.preuserinfo`). The companion event-driven path — `/api/webhooks/zitadel` and the `live-webhook-listener` change — has handler code, HMAC verification, idempotency, and full downstream dispatch (cache invalidation, mapping-rule cascade, LLDAP provisioning intents, welcome-bundle onboarding), but **no Zitadel-side producer**. As a result, lifecycle changes that originate outside the Syndra UI (operator click in the Zitadel console, future Google Workspace deprovisioning per design.md L106) never propagate.

The legacy verifier on `/api/webhooks/zitadel` also speaks a different HMAC dialect (`X-Zitadel-Signature`, `HMAC(secret, ts + "\n" + body)`) than what Zitadel actually sends for Actions v2 (`ZITADEL-Signature: t=<unix>,v1=<hex>` over `<unix>.<body>` per `zitadel-go@main/pkg/actions/signing.go`). Two HMAC schemes for one Zitadel boundary is a maintenance burden with no producer behind the legacy one.

## What Changes

* Adds a second Zitadel Actions v2 target `syndra-event-listener` (type `restAsync`, endpoint `/api/webhooks/zitadel`) bound to `condition.event` triggers for `user.human.added`, `user.deactivated`, `user.locked`, `user.grant.added`, `user.grant.changed`, `user.grant.removed`. Deactivation and lock are user-aggregate events, not human-aggregate (Zitadel registers them as `userEventTypePrefix + "deactivated"` / `+ "locked"` in `internal/repository/user/`); using `user.human.deactivated` / `user.human.locked` triggers `COMMAND-74aaqj8fv9` "Execution condition is invalid" at SetExecution time.
* Reshapes `zitadel/actions/targets.json` from a single-target schema to a multi-target schema (`targets[]`, `executions[]` with explicit `target` reference). Extends `register.sh` and `rotate.sh` to iterate targets, capture per-target signing keys to `.action-signing-key.<target>` files, and bind executions to the correct target ID.
* Retires the legacy `ZITADEL_WEBHOOK_SECRET` env var and the bespoke `verifyWebhookSignature` / `verifyWebhookFreshness` helpers. Replaces them on `/api/webhooks/zitadel` with the existing `withZitadelActionSignature` middleware keyed off a new `ZITADEL_EVENT_SIGNING_KEY` env var.
* Adds a payload translator (`backend/internal/handlers/webhook_translate.go`) that decodes Zitadel's `ContextInfoEvent` wire format (`zitadel/zitadel:internal/repository/execution/queue.go` — flat top-level `aggregateID`, `aggregateType`, `event_type`, `event_payload`, `userID`) into the existing internal `WebhookPayload`. Detection is by shape (`aggregateID` field present → Zitadel; no `aggregateID` → internal-shape strict decode). Existing internal-shape callers (operator curl, contracts tests) continue to work.
* Extends `WebhookPayload` with `role_keys []string` to carry multi-role grant events without splitting one HTTP delivery into N database rows. `processGrantAdded` and `processGrantRemoved` iterate roles; idempotency stays per-delivery.
* Adds a self-mutation loop guard: events whose top-level `userID` (the editor — Zitadel's `ContextInfoEvent` carries the editor in this field, not the subject) matches `ZITADEL_M2M_USER_ID` (the backend's own service-user ID) are dropped at the translator with a log line. Without this, every backend-initiated grant mutation would echo back through Actions v2 and re-trigger orchestration.

## Capabilities

### New Capabilities
* `lifecycle-event-propagation`: Zitadel Actions v2 event-trigger executions fan out to the `/api/webhooks/zitadel` listener, bringing out-of-band Zitadel mutations into Syndra's orchestration plane.

### Modified Capabilities
* `webhook-invalidation`: Producer-side gap closed. The endpoint is no longer producer-less; it is the documented Actions v2 event sink.
* `application-claims`: Living spec gains a parallel "Event Triggers" deployment subsection alongside the existing function-trigger one.

## Impact

* Deployment: one `make zitadel-actions-register` now creates/updates **two** Zitadel targets and captures **two** signing keys.
* Env: adds `ZITADEL_EVENT_SIGNING_KEY` and `ZITADEL_M2M_USER_ID`. Removes `ZITADEL_WEBHOOK_SECRET`.
* Code: net positive ~250 LOC across `webhook.go`, `webhook_translate.go`, `router.go`. Net negative ~50 LOC from removing the legacy verifier.
* Tests: ~15 new tests (translator per event mapping, signature middleware on webhook route, self-loop guard, role_keys plural dispatch, multi-target register dry-run).
* Backward compatibility: internal-shape webhook callers unchanged. Legacy env var removed cleanly (no producer existed in practice).
