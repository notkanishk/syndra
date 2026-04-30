# Design: Zitadel Event-Trigger Propagation

> **Status:** In Progress | [Proposal](proposal.md) | [Tasks](tasks.md)

## Context

`zitadel-actions-v2-deployment` shipped function-trigger Actions v2 for claim injection. The webhook listener at `/api/webhooks/zitadel` (from `live-webhook-listener`) was producer-less — its HMAC scheme and payload contract were MkAuth-internal, never reachable from Zitadel without a translator. This change closes that loop by registering a second Actions v2 target with `condition.event` executions and converging the two webhook authentication paths onto the canonical Zitadel-Signature scheme already used by `/api/action/inject`.

## Decisions

### D1. Two targets, not one

Function triggers (claim injection) need `restCall` so Zitadel parses the response body. Event triggers need `restAsync` so Zitadel does not block the actor's request on MkAuth latency. Mixing both on one target is impossible — `target_type` is a oneof. Conclusion: two distinct targets, each with its own signing key, bound to disjoint trigger sets.

### D2. Reuse `withZitadelActionSignature`, retire the legacy verifier

`backend/internal/handlers/zitadel_action_auth.go:withZitadelActionSignature(secretEnvVar string, next http.HandlerFunc) http.HandlerFunc` already takes the env-var name as a parameter (it was designed for reuse). Mounting it on `/api/webhooks/zitadel` with a different env var is a one-line router change. The legacy `verifyWebhookSignature` (header `X-Zitadel-Signature`, hash input `ts + "\n" + body`) does not match what Zitadel actually emits and has no producer; carrying it alongside the canonical Actions v2 verifier would be dead code with no upside, so it is removed.

### D3. Translator keyed off shape, not Content-Type

Zitadel's event payload has top-level `aggregate` (with `id`, `type`, `resourceOwner`) and `event` fields. MkAuth's internal `WebhookPayload` has `event_type` at the top level. The two are unambiguously distinguishable. `HandleZitadelWebhook` peeks at the parsed JSON, picks the path, and produces a `WebhookPayload` in either case. No Content-Type sniffing.

### D4. `role_keys[]` plural in WebhookPayload, not fan-out in translator

A Zitadel `user.grant.added` event carries one or more role keys per delivery. Two ways to handle this:
- Fan-out at translator time: one HTTP delivery → N internal events → N `webhook_events` rows. Breaks the "one delivery = one row" idempotency invariant; ops debugging becomes harder.
- Plural at the payload layer: add a new `WebhookPayload.RoleKeys []string` field alongside the existing `RoleKey string`. Translator populates `RoleKeys`. Processors iterate. One delivery = one row. Idempotency key is still the Zitadel-Signature header.

Plural wins. Singular `RoleKey` is preserved for back-compat with operator curl callers and existing tests.

### D5. Self-mutation loop guard at the translator boundary

When the backend calls Zitadel's Management API (e.g. `RemoveUserGrant` from `RevokeMappingRules`), Zitadel emits the corresponding event, Actions v2 POSTs it back to `/api/webhooks/zitadel`, and the orchestrator would re-process the very mutation it just made. Two cheap defenses:
- Editor check: drop events where `aggregate.editorUserId == ZITADEL_M2M_USER_ID`.
- Idempotency safety net: existing `webhook_events.idempotency_key` dedup.

Editor check is the primary defense; idempotency is the backup. The env var is the service-user ID Zitadel returns when MkAuth authenticates with the M2M JWT-profile flow — captured manually on first deploy. Unset env var disables the guard with a startup log warning (acceptable for local-dev).

### D6. Welcome-bundle assignment requires no new code

`processUserCreated` (`backend/internal/handlers/webhook.go:210`) already calls `webhookTriggerOnboarding` → `services.TriggerOnboarding` (`backend/internal/services/onboarding.go:19`) which calls `GetWelcomeBundle` and `AssignBundleToUser`. Idempotency at both layers (`webhook_events` and `onboarding_triggers.idempotency_key`). Once Zitadel's `user.human.added` event reaches the endpoint with a valid signature, the existing pipeline runs unchanged.

### D7. Single `make zitadel-actions-register` for both targets

Operator UX: one make target, one signing-key capture flow, one rotation procedure. Achieved by making `register.sh` and `rotate.sh` iterate over the multi-target manifest. Each target gets its own `.action-signing-key.<name>` capture file. The env-fragment writer outputs both `ZITADEL_ACTION_SIGNING_KEY=...` and `ZITADEL_EVENT_SIGNING_KEY=...` lines. Operators apply with `cat zitadel/actions/.action-env.fragment >> .env`, same as today.

### D8. Event identifiers verified against the running Zitadel instance

Zitadel's exact event-type strings for grants (`user.user.grant.added` vs `user.grant.added` — the prefix has shifted across versions) are version-dependent. The plan cites the modern instance-event names but the implementation MUST verify against the target Zitadel via dev-mode pass-through (signing key unset, accept any payload, log) before flipping signature verification on. Mitigates protocol drift with empirical evidence rather than docs guess.

## Rejected alternatives

- **Keep both HMAC schemes (legacy + Actions v2).** No producer for the legacy scheme exists; carrying both is dead code with no upside.
- **Single target with both function and event triggers.** Zitadel's `target_type` oneof forbids this; even if it didn't, `restCall` blocks on MkAuth latency, which is unacceptable for token issuance and irrelevant for events.
- **Fan-out role_keys at translator time** (D4 alternative).
- **Inline event-name regex matching against `event.eventType` strings.** Brittle. Explicit map in the translator is testable and auditable.

## Risks

1. **Zitadel event-name drift.** Mitigated by D8 (empirical verification) and by lenient JSON decoding in the translator. Translator's event map is the only place to update on a Zitadel-side rename.
2. **`editorUserId` field path.** Documented Zitadel response wraps it under `aggregate` in some versions and `editor` in others. Translator accepts either via dual-path lookup; a real captured payload pins the correct path.
3. **At-least-once delivery.** Zitadel retries failed targets. Existing idempotency on the Zitadel-Signature header dedups; `webhook_events.idempotency_key` enforces.
4. **Out-of-order events.** Don't block — process each independently; cache reconciles on next claim issuance. Same posture as today.
5. **Long-running grant.changed processing.** With `restAsync`, Zitadel does not block on us, so latency is not user-visible. Backend timeouts on downstream calls cap blast radius.

## Ship-blocking verifications

* Run `register.sh` against staging Zitadel; capture both signing keys; verify both targets exist in `GET /v2/actions/targets/search`.
* Smoke-test `/api/webhooks/zitadel` with a captured real Zitadel event payload (one per event type) and assert `webhook_events` row + downstream effect.
* Run `processUserCreated` end-to-end in staging: trigger `user.human.added` from Zitadel console → assert `onboarding_triggers` row exists with welcome-bundle ID + audit log entry.
* Self-loop guard: invoke a backend mutation that hits Zitadel; tail logs for the dropped echo event.
