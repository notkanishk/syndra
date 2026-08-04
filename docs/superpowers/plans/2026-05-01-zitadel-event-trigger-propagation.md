# Zitadel Event-Trigger Propagation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Zitadel push lifecycle events (user added/deactivated/locked, grant added/changed/removed) to Syndra via Actions v2 so welcome-bundle assignment, mapping-rule cascade, cache invalidation, and LLDAP provisioning fire automatically when state changes outside the Syndra UI — and bundle the event-target setup into a single `make zitadel-actions-register` invocation alongside the existing claim-injector target.

**Architecture:** Add a second Zitadel Actions v2 target named `syndra-event-listener` (type `restAsync`, fire-and-forget) bound to `condition.event` triggers, alongside the existing `syndra-claim-injector` (type `restCall`, function triggers). Reshape `zitadel/actions/targets.json` from a single-target manifest to a multi-target one and extend `register.sh`/`rotate.sh` to iterate. On the backend, retire the legacy `ZITADEL_WEBHOOK_SECRET` HMAC scheme on `/api/webhooks/zitadel` and reuse the existing `withZitadelActionSignature` middleware with a new env var `ZITADEL_EVENT_SIGNING_KEY` (the second target produces its own signing key). Add a payload translator (`webhook_translate.go`) that maps Zitadel-native event JSON into the existing internal `WebhookPayload`, extend `WebhookPayload` with `role_keys[]` to carry multi-role grants, and add a self-mutation loop guard via `editorUserId` (skip events whose editor is Syndra's own M2M user). The welcome-bundle path is **already wired** end-to-end (`processUserCreated` → `webhookTriggerOnboarding` → `services.onboarding.TriggerOnboarding` → `GetWelcomeBundle` → `AssignBundleToUser`) — it requires no new code.

**Tech Stack:** Go 1.22 stdlib (handlers, HMAC), Bash + jq + curl (deployment scripts), Zitadel Actions v2 REST API at `/v2/actions/*` (verified 2026-04-24 against `proto/zitadel/action/v2/{target,execution,query}.proto`), PostgreSQL (`webhook_events` idempotency, `onboarding_triggers`).

---

## Phase 0 — Pre-flight verification

### Task 0.1: Snapshot current state

**Files:** none (read-only).

- [ ] **Step 1: Confirm backend tests pass on main**

Run: `cd backend && go test ./... && go vet ./...`
Expected: PASS, no warnings. Capture the test count for later comparison.

- [ ] **Step 2: Confirm bash scripts are clean**

Run: `bash -n zitadel/actions/register.sh && bash -n zitadel/actions/rotate.sh && bash -n scripts/smoke-test-action-v2.sh`
Expected: no output, exit 0.

- [ ] **Step 3: Note the route registration to be modified**

Run: `grep -n "webhooks/zitadel\|action/inject" backend/internal/handlers/router.go`
Expected: three matches at lines 130-132 — the unprotected webhook route, and the action route already wrapped with `withZitadelActionSignature("ZITADEL_ACTION_SIGNING_KEY", ...)`.

(No commit — this is a verification gate.)

---

## Phase 1 — OpenSpec scaffolding

Per `CLAUDE.md` mandatory workflow: spec scaffolding lands first so backend changes have a documented home.

### Task 1.1: Create the change directory + proposal.md

**Files:**
- Create: `openspec/changes/zitadel-event-trigger-propagation/proposal.md`

- [ ] **Step 1: Create directory**

```bash
mkdir -p openspec/changes/zitadel-event-trigger-propagation/specs/lifecycle-event-propagation
```

- [ ] **Step 2: Write proposal.md**

```markdown
## Why

The `zitadel-actions-v2-deployment` change wired Actions v2 for **claim injection only** (`function.preaccesstoken`, `function.preuserinfo`). The companion event-driven path — `/api/webhooks/zitadel` and the `live-webhook-listener` change — has handler code, HMAC verification, idempotency, and full downstream dispatch (cache invalidation, mapping-rule cascade, LLDAP provisioning intents, welcome-bundle onboarding), but **no Zitadel-side producer**. As a result, lifecycle changes that originate outside the Syndra UI (operator click in the Zitadel console, future Google Workspace deprovisioning per design.md L106) never propagate.

The legacy verifier on `/api/webhooks/zitadel` also speaks a different HMAC dialect (`X-Zitadel-Signature`, `HMAC(secret, ts + "\n" + body)`) than what Zitadel actually sends for Actions v2 (`ZITADEL-Signature: t=<unix>,v1=<hex>` over `<unix>.<body>` per `zitadel-go@main/pkg/actions/signing.go`). Two HMAC schemes for one Zitadel boundary is a maintenance burden with no producer behind the legacy one.

## What Changes

* Adds a second Zitadel Actions v2 target `syndra-event-listener` (type `restAsync`, endpoint `/api/webhooks/zitadel`) bound to `condition.event` triggers for `user.human.added`, `user.human.deactivated`, `user.human.locked`, `user.grant.added`, `user.grant.changed`, `user.grant.removed`.
* Reshapes `zitadel/actions/targets.json` from a single-target schema to a multi-target schema (`targets[]`, `executions[]` with explicit `target` reference). Extends `register.sh` and `rotate.sh` to iterate targets, capture per-target signing keys to `.action-signing-key.<target>` files, and bind executions to the correct target ID.
* Retires the legacy `ZITADEL_WEBHOOK_SECRET` env var and the bespoke `verifyWebhookSignature` / `verifyWebhookFreshness` helpers. Replaces them on `/api/webhooks/zitadel` with the existing `withZitadelActionSignature` middleware keyed off a new `ZITADEL_EVENT_SIGNING_KEY` env var.
* Adds a payload translator (`backend/internal/handlers/webhook_translate.go`) that maps Zitadel-native event JSON into the existing internal `WebhookPayload`. Detection is by shape (`aggregate` field present → Zitadel; `event_type` field present → internal). Existing internal-shape callers (operator curl, contracts tests) continue to work.
* Extends `WebhookPayload` with `role_keys []string` to carry multi-role grant events without splitting one HTTP delivery into N database rows. `processGrantAdded` and `processGrantRemoved` iterate roles; idempotency stays per-delivery.
* Adds a self-mutation loop guard: events whose `editorUserId` matches `ZITADEL_M2M_USER_ID` (the backend's own service-user ID) are dropped at the translator with a log line. Without this, every backend-initiated grant mutation would echo back through Actions v2 and re-trigger orchestration.

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
```

- [ ] **Step 3: Commit**

```bash
git add openspec/changes/zitadel-event-trigger-propagation/proposal.md
git commit -m "openspec: propose zitadel-event-trigger-propagation"
```

### Task 1.2: Write design.md

**Files:**
- Create: `openspec/changes/zitadel-event-trigger-propagation/design.md`

- [ ] **Step 1: Write design.md**

```markdown
# Design: Zitadel Event-Trigger Propagation

> **Status:** In Progress | [Proposal](proposal.md) | [Tasks](tasks.md)

## Context

`zitadel-actions-v2-deployment` shipped function-trigger Actions v2 for claim injection. The webhook listener at `/api/webhooks/zitadel` (from `live-webhook-listener`) was producer-less — its HMAC scheme and payload contract were Syndra-internal, never reachable from Zitadel without a translator. This change closes that loop by registering a second Actions v2 target with `condition.event` executions and converging the two webhook authentication paths onto the canonical Zitadel-Signature scheme already used by `/api/action/inject`.

## Decisions

### D1. Two targets, not one

Function triggers (claim injection) need `restCall` so Zitadel parses the response body. Event triggers need `restAsync` so Zitadel does not block the actor's request on Syndra latency. Mixing both on one target is impossible — `target_type` is a oneof. Conclusion: two distinct targets, each with its own signing key, bound to disjoint trigger sets.

### D2. Reuse `withZitadelActionSignature`, retire the legacy verifier

`backend/internal/handlers/zitadel_action_auth.go:withZitadelActionSignature(secretEnvVar string, next http.HandlerFunc)` already takes the env-var name as a parameter (it was designed for reuse). Mounting it on `/api/webhooks/zitadel` with a different env var is a one-line router change. The legacy `verifyWebhookSignature` (header `X-Zitadel-Signature`, hash input `ts + "\n" + body`) does not match what Zitadel actually emits and has no producer; per CLAUDE.md ("delete completely if certain unused"), it is removed.

### D3. Translator keyed off shape, not Content-Type

Zitadel's event payload has top-level `aggregate` (with `id`, `type`, `resourceOwner`) and `event` fields. Syndra's internal `WebhookPayload` has `event_type` at the top level. The two are unambiguously distinguishable. `HandleZitadelWebhook` peeks at the parsed JSON, picks the path, and produces a `WebhookPayload` in either case. No Content-Type sniffing.

### D4. `role_keys[]` plural in WebhookPayload, not fan-out in translator

A Zitadel `user.grant.added` event carries one or more role keys per delivery. Two ways to handle this:
- Fan-out at translator time: one HTTP delivery → N internal events → N `webhook_events` rows. Breaks the "one delivery = one row" idempotency invariant; ops debugging becomes harder.
- Plural at the payload layer: `WebhookPayload.RoleKeys []string` alongside the existing `RoleKey string`. Translator populates `RoleKeys`. Processors iterate. One delivery = one row. Idempotency key is still the Zitadel-Signature header.

Plural wins. Singular `RoleKey` is preserved for back-compat with operator curl callers and existing tests.

### D5. Self-mutation loop guard at the translator boundary

When the backend calls Zitadel's Management API (e.g. `RemoveUserGrant` from `RevokeMappingRules`), Zitadel emits the corresponding event, Actions v2 POSTs it back to `/api/webhooks/zitadel`, and the orchestrator would re-process the very mutation it just made. Two cheap defenses:
- Editor check: drop events where `aggregate.editorUserId == ZITADEL_M2M_USER_ID`.
- Idempotency safety net: existing `webhook_events.idempotency_key` dedup.

Editor check is the primary defense; idempotency is the backup. The env var is the service-user ID Zitadel returns when Syndra authenticates with the M2M JWT-profile flow — captured manually on first deploy. Unset env var disables the guard with a startup log warning (acceptable for local-dev).

### D6. Welcome-bundle assignment requires no new code

`processUserCreated` (webhook.go:210) already calls `webhookTriggerOnboarding` → `services.onboarding.TriggerOnboarding` (onboarding.go:19) which calls `GetWelcomeBundle` and `AssignBundleToUser`. Idempotency at both layers (`webhook_events` and `onboarding_triggers.idempotency_key`). Once Zitadel's `user.human.added` event reaches the endpoint with a valid signature, the existing pipeline runs unchanged.

### D7. Single `make zitadel-actions-register` for both targets

Operator UX: one make target, one signing-key capture flow, one rotation procedure. Achieved by making `register.sh` and `rotate.sh` iterate over the multi-target manifest. Each target gets its own `.action-signing-key.<name>` capture file. The env-fragment writer outputs both `ZITADEL_ACTION_SIGNING_KEY=...` and `ZITADEL_EVENT_SIGNING_KEY=...` lines. Operators apply with `cat zitadel/actions/.action-env.fragment >> .env`, same as today.

### D8. Event identifiers verified against the running Zitadel instance

Zitadel's exact event-type strings for grants (`user.user.grant.added` vs `user.grant.added` — the prefix has shifted across versions) are version-dependent. The plan cites the modern instance-event names but the implementation MUST verify against the target Zitadel via dev-mode pass-through (signing key unset, accept any payload, log) before flipping signature verification on. Mitigates protocol drift with empirical evidence rather than docs guess.

## Rejected alternatives

- **Keep both HMAC schemes (legacy + Actions v2).** No producer for the legacy scheme exists; carrying both is dead code with no upside.
- **Single target with both function and event triggers.** Zitadel's `target_type` oneof forbids this; even if it didn't, `restCall` blocks on Syndra latency, which is unacceptable for token issuance and irrelevant for events.
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
```

- [ ] **Step 2: Commit**

```bash
git add openspec/changes/zitadel-event-trigger-propagation/design.md
git commit -m "openspec: design zitadel-event-trigger-propagation"
```

### Task 1.3: Write tasks.md

**Files:**
- Create: `openspec/changes/zitadel-event-trigger-propagation/tasks.md`

- [ ] **Step 1: Write tasks.md (mirrors this plan; OpenSpec convention)**

```markdown
# Tasks

## 1. Backend: data model + auth
- [ ] 1.1 Add `RoleKeys []string` to `WebhookPayload`; update validation
- [ ] 1.2 Iterate `RoleKeys` in `processGrantAdded` / `processGrantRemoved`
- [ ] 1.3 Switch `/api/webhooks/zitadel` route to `withZitadelActionSignature("ZITADEL_EVENT_SIGNING_KEY", ...)`
- [ ] 1.4 Remove `verifyWebhookSignature`, `verifyWebhookFreshness`, inline calls in `HandleZitadelWebhook`
- [ ] 1.5 Remove `ZITADEL_WEBHOOK_SECRET` from `.env.example` and `docker-compose.yml`

## 2. Backend: payload translator
- [ ] 2.1 Define `ZitadelEventPayload` lenient struct in `webhook_translate.go`
- [ ] 2.2 Map `user.human.added` → `user_created`
- [ ] 2.3 Map `user.human.deactivated` → `user_deactivated`
- [ ] 2.4 Map `user.human.locked` → `user_locked`
- [ ] 2.5 Map `user.grant.added` → `grant_added` with `role_keys[]`
- [ ] 2.6 Map `user.grant.changed` → `grant_changed` with `role_keys[]`
- [ ] 2.7 Map `user.grant.removed` → `grant_removed` with `role_keys[]`
- [ ] 2.8 Unknown-event passthrough (200 + log, no dispatch)
- [ ] 2.9 Wire shape detection into `HandleZitadelWebhook`

## 3. Backend: self-mutation loop guard
- [ ] 3.1 Add `ZITADEL_M2M_USER_ID` env reader
- [ ] 3.2 Drop events at translator when `editorUserId` matches; log + 200

## 4. Deployment: multi-target manifest
- [ ] 4.1 Reshape `targets.json` to `targets[]` + `executions[]` with named `target` reference
- [ ] 4.2 Add `syndra-event-listener` target (`restAsync`, 5s timeout)
- [ ] 4.3 Add 6 `condition.event` executions for the lifecycle event names

## 5. Deployment: register.sh + rotate.sh
- [ ] 5.1 Iterate `targets[]` in `register.sh`; per-target signing-key capture
- [ ] 5.2 Map `executions[].target` (name) → captured target IDs; bind in one pass
- [ ] 5.3 Update `--remove` to unbind every execution and warn about per-target retention
- [ ] 5.4 Update `rotate.sh` to accept a `--target NAME` flag (default both)
- [ ] 5.5 Update `.action-env.fragment` writer to emit both env-var lines

## 6. Ops + docs
- [ ] 6.1 Add `zitadel/actions/EVENTS.md` (operator runbook)
- [ ] 6.2 Update `.env.example` (add `ZITADEL_EVENT_SIGNING_KEY`, `ZITADEL_M2M_USER_ID`; remove `ZITADEL_WEBHOOK_SECRET`)
- [ ] 6.3 Update `docker-compose.yml` (same env-var changes)
- [ ] 6.4 Update `SIGNING_KEY.md` and `DEPLOY.md` for the dual-target flow
- [ ] 6.5 Add `scripts/smoke-test-event-listener.sh`

## 7. OpenSpec finalization
- [ ] 7.1 MODIFIED delta on `application-claims/spec.md` (event-trigger subsection)
- [ ] 7.2 New capability spec `lifecycle-event-propagation/spec.md`
- [ ] 7.3 Update `feature-coverage.md` (`webhook-invalidation` row producer column)
- [ ] 7.4 Update `INDEX.md` change log
- [ ] 7.5 Write `IMPLEMENTATION.md`
- [ ] 7.6 Refresh codebase-memory graph via `mcp__codebase-memory-mcp__detect_changes`
```

- [ ] **Step 2: Commit**

```bash
git add openspec/changes/zitadel-event-trigger-propagation/tasks.md
git commit -m "openspec: tasks for zitadel-event-trigger-propagation"
```

### Task 1.4: Capability spec — lifecycle-event-propagation

**Files:**
- Create: `openspec/changes/zitadel-event-trigger-propagation/specs/lifecycle-event-propagation/spec.md`

- [ ] **Step 1: Write the capability spec**

```markdown
# Capability: lifecycle-event-propagation

> **Status:** ADDED in zitadel-event-trigger-propagation
> **Origin:** zitadel-event-trigger-propagation
> **See also:** application-claims (claim integration), webhook-invalidation (downstream dispatch)

## Purpose

Lifecycle changes that originate in Zitadel (operator console, SCIM, or future external IdP triggers) MUST reach Syndra's orchestration plane within the Actions v2 retry window so welcome-bundle assignment, mapping-rule cascade, cache invalidation, and LLDAP provisioning intents fire automatically.

## Requirements

### Producer
- The Zitadel Actions v2 deployment MUST register a target named `syndra-event-listener` of type `restAsync` with `payloadType = PAYLOAD_TYPE_JSON` pointing at `${SYNDRA_EXTERNAL_URL}/api/webhooks/zitadel`.
- The deployment MUST bind executions for at minimum: `user.human.added`, `user.human.deactivated`, `user.human.locked`, `user.grant.added`, `user.grant.changed`, `user.grant.removed`.
- The deployment MUST be applicable in one operator command. `make zitadel-actions-register` MUST register both the `syndra-claim-injector` (function triggers) and `syndra-event-listener` (event triggers) targets in a single invocation and capture both signing keys.

### Consumer
- `/api/webhooks/zitadel` MUST verify request signatures via the canonical Zitadel `ZITADEL-Signature` header scheme using `ZITADEL_EVENT_SIGNING_KEY`. The legacy `X-Zitadel-Signature` / `ZITADEL_WEBHOOK_SECRET` scheme MUST be removed.
- The handler MUST accept Zitadel-native event payloads (top-level `aggregate` field) and translate them to the internal `WebhookPayload` shape. Existing internal-shape callers (top-level `event_type`) MUST continue to work for back-compat.
- Multi-role grant events MUST surface every affected role; idempotency MUST remain per HTTP delivery (one delivery = one `webhook_events` row).
- Events whose `editorUserId` matches `ZITADEL_M2M_USER_ID` MUST be dropped before dispatch with a log line. Loop suppression is REQUIRED, not OPTIONAL.

## Scenarios

### Scenario: Default bundle on first user creation
- **GIVEN** a welcome bundle is configured (`bundles.is_welcome = true` row exists)
- **AND** Zitadel's `syndra-event-listener` target is registered and reachable
- **WHEN** an operator creates a human user in Zitadel via the console
- **THEN** `/api/webhooks/zitadel` MUST receive a `user.human.added` event with a valid signature
- **AND** `processUserCreated` MUST insert an `onboarding_triggers` row with status=`completed`
- **AND** `direct_role_grants` MUST contain rows for every role in the welcome bundle
- **AND** an audit row attributed to `system:onboarding` MUST exist

### Scenario: Out-of-band grant added in Zitadel console
- **WHEN** an operator adds a project grant to a user via the Zitadel console
- **THEN** `processGrantAdded` MUST run mapping-rule enforcement for every role in the grant
- **AND** the user's compiled cache MUST be rebuilt (or invalidated)

### Scenario: Self-mutation loop suppression
- **GIVEN** `ZITADEL_M2M_USER_ID` is set to the backend's service-user ID
- **WHEN** the backend calls `RemoveUserGrant` via Management API
- **AND** Zitadel emits the corresponding `user.grant.removed` event back to the listener
- **THEN** the translator MUST detect `editorUserId == ZITADEL_M2M_USER_ID` and short-circuit with `200 OK` + a `[WEBHOOK] dropped self-mutation` log line
- **AND** no `webhook_events` row MUST be inserted

### Scenario: Unknown event type
- **WHEN** Zitadel POSTs an event with an unmapped `event` field (e.g. `user.password.changed`)
- **THEN** the translator MUST return `200 OK` with no dispatch and a `[WEBHOOK] unknown event` log line
- **AND** no `webhook_events` row MUST be inserted

### Scenario: Bad signature
- **GIVEN** `ZITADEL_EVENT_SIGNING_KEY` is set
- **WHEN** a request arrives at `/api/webhooks/zitadel` with a missing or invalid `ZITADEL-Signature` header
- **THEN** the handler MUST respond `401 INVALID_SIGNATURE` and MUST NOT decode the body
```

- [ ] **Step 2: Commit**

```bash
git add openspec/changes/zitadel-event-trigger-propagation/specs/
git commit -m "openspec: lifecycle-event-propagation capability spec"
```

### Task 1.5: Update INDEX.md

**Files:**
- Modify: `openspec/INDEX.md`

- [ ] **Step 1: Add row to Change Log table (alphabetic by phase)**

Find the line in the Change Log table (around line 56) for `[Zitadel Actions v2 Deployment]` and add immediately AFTER it:

```markdown
| [Zitadel Event-Trigger Propagation](changes/zitadel-event-trigger-propagation/) | 5 | In Progress | [proposal](changes/zitadel-event-trigger-propagation/proposal.md) / [design](changes/zitadel-event-trigger-propagation/design.md) / [tasks](changes/zitadel-event-trigger-propagation/tasks.md) |
```

Find the Roadmap row for Phase 5 (around line 71) and append the change name to its `Changes` cell: `... + zitadel-event-trigger-propagation`.

Add to Capability Specs table (around line 13) above the architecture reference:

```markdown
| Lifecycle Event Propagation | Pending — see change | [spec](changes/zitadel-event-trigger-propagation/specs/lifecycle-event-propagation/spec.md) | zitadel-event-trigger-propagation |
```

- [ ] **Step 2: Commit**

```bash
git add openspec/INDEX.md
git commit -m "openspec: index zitadel-event-trigger-propagation"
```

---

## Phase 2 — Backend: data model

### Task 2.1: Add `RoleKeys []string` to `WebhookPayload` (TDD)

**Files:**
- Modify: `backend/internal/handlers/webhook.go:18-25` (struct), `:79-99` (validation), `:157-199` (processors)
- Test: `backend/internal/handlers/webhook_test.go` (new test below)

- [ ] **Step 1: Write the failing test**

Append to `backend/internal/handlers/webhook_test.go`:

```go
func TestHandleZitadelWebhook_RoleKeysPlural_DispatchesEachRole(t *testing.T) {
	t.Setenv("ZITADEL_WEBHOOK_SECRET", "")
	t.Setenv("ZITADEL_EVENT_SIGNING_KEY", "")

	// Capture the roles that processGrantAdded sees via the injected enforcer.
	var seen []string
	prev := webhookEnforceMappingRules
	webhookEnforceMappingRules = func(ctx context.Context, userID, project, role string) error {
		seen = append(seen, role)
		return nil
	}
	t.Cleanup(func() { webhookEnforceMappingRules = prev })

	// Stub out persistence/cache/provisioning so this is a pure dispatch test.
	dbInsertWebhookEventStub(t)
	cacheRebuildStub(t)
	webhookEmitProvisioningIntentStub(t)

	body := []byte(`{
		"event_type": "grant_added",
		"user_id": "u1",
		"source_project": "p1",
		"role_keys": ["alpha", "beta"],
		"project_ids": ["p1"]
	}`)
	rr := postWebhook(t, body)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	if !slices.Equal(seen, []string{"alpha", "beta"}) {
		t.Fatalf("expected roles [alpha beta], got %v", seen)
	}
}
```

(The `*Stub` helpers — write thin local helpers that override `dbInsertWebhookEvent`, `cacheRebuildUser`, and `webhookEmitProvisioningIntent` to no-ops if they don't already exist in `webhook_test.go`. The injected-deps pattern is established at `deps.go`.)

- [ ] **Step 2: Run the test and verify it fails**

Run: `cd backend && go test ./internal/handlers -run TestHandleZitadelWebhook_RoleKeysPlural_DispatchesEachRole -v`
Expected: FAIL with `unknown field "role_keys"` (strict decoder rejects) or with `seen` containing only one role.

- [ ] **Step 3: Add `RoleKeys` to the struct**

Edit `backend/internal/handlers/webhook.go:19-25`:

```go
// WebhookPayload represents our interpretation of a Zitadel event payload.
type WebhookPayload struct {
	EventType     string   `json:"event_type"`      // grant_added, grant_removed, grant_changed, user_deactivated, user_locked, user_created
	UserID        string   `json:"user_id"`
	SourceProject string   `json:"source_project"`
	RoleKey       string   `json:"role_key"`        // back-compat singular; prefer RoleKeys for new callers
	RoleKeys      []string `json:"role_keys"`       // multi-role grants from Zitadel event-trigger payloads
	ProjectIDs    []string `json:"project_ids"`     // all projects the user touches
}
```

- [ ] **Step 4: Update validation in `HandleZitadelWebhook`**

Replace `webhook.go:90-99` (the `isGrantEvent` block) with:

```go
isGrantEvent := event.EventType == "grant_added" || event.EventType == "grant_removed" || event.EventType == "grant_changed"
if !trimmedNonEmpty(event.UserID) || !trimmedNonEmpty(event.SourceProject) {
	jsonValidationErrorResponse(w, "user_id and source_project are required", map[string]string{
		"user_id":        "required",
		"source_project": "required",
	})
	return
}
if isGrantEvent {
	// At least one of role_key (singular) or role_keys (plural) MUST be set.
	if !trimmedNonEmpty(event.RoleKey) && len(event.RoleKeys) == 0 {
		jsonValidationErrorResponse(w, "role_key or role_keys is required for grant events", map[string]string{
			"role_key":  "required (or use role_keys array)",
			"role_keys": "required (or use role_key string)",
		})
		return
	}
	// Normalize: if RoleKeys is empty, populate it from singular RoleKey for downstream uniformity.
	if len(event.RoleKeys) == 0 && trimmedNonEmpty(event.RoleKey) {
		event.RoleKeys = []string{event.RoleKey}
	}
	// Inverse: if RoleKey is empty but RoleKeys is set, populate the singular for log/idempotency parity.
	if !trimmedNonEmpty(event.RoleKey) && len(event.RoleKeys) > 0 {
		event.RoleKey = event.RoleKeys[0]
	}
}
```

- [ ] **Step 5: Iterate `RoleKeys` in `processGrantAdded`**

Replace `webhook.go:157-182` with:

```go
func processGrantAdded(ctx context.Context, event WebhookPayload, eventID string) error {
	if len(event.ProjectIDs) > 0 {
		cacheRebuildUser(ctx, event.UserID, event.ProjectIDs)
	} else {
		_ = cacheInvalidateUser(ctx, event.UserID)
	}

	for _, role := range event.RoleKeys {
		if err := webhookEnforceMappingRules(ctx, event.UserID, event.SourceProject, role); err != nil {
			return fmt.Errorf("orchestrator failure for role=%s: %v", role, err)
		}

		// Legacy back-compat: role_key=new_user on grant_added also triggers onboarding.
		if role == "new_user" {
			idempotencyKey := fmt.Sprintf("webhook:%s:%s", event.UserID, event.SourceProject)
			if err := webhookTriggerOnboarding(ctx, event.UserID, "webhook", idempotencyKey); err != nil {
				log.Printf("[WEBHOOK] Onboarding trigger failed for user=%s: %v", event.UserID, err)
			}
		}

		if err := webhookEmitProvisioningIntent(ctx, event.UserID, "add", event.SourceProject, role, eventID); err != nil {
			log.Printf("[WEBHOOK] Provisioning intent emission failed: %v", err)
		}
	}
	return nil
}
```

- [ ] **Step 6: Iterate `RoleKeys` in `processGrantRemoved`**

Replace `webhook.go:187-199`:

```go
func processGrantRemoved(ctx context.Context, event WebhookPayload, eventID string) error {
	_ = cacheInvalidateUser(ctx, event.UserID)
	for _, role := range event.RoleKeys {
		if err := webhookRevokeMappingRules(ctx, event.UserID, event.SourceProject, role); err != nil {
			return fmt.Errorf("revocation failure for role=%s: %v", role, err)
		}
		if err := webhookEmitProvisioningIntent(ctx, event.UserID, "remove", event.SourceProject, role, eventID); err != nil {
			log.Printf("[WEBHOOK] Provisioning intent emission failed: %v", err)
		}
	}
	return nil
}
```

- [ ] **Step 7: Run the test and verify it passes; run the full handlers package**

Run: `cd backend && go test ./internal/handlers -run TestHandleZitadelWebhook_RoleKeysPlural_DispatchesEachRole -v`
Expected: PASS.

Run: `cd backend && go test ./internal/handlers -v`
Expected: all existing tests still pass (singular `role_key` callers unchanged).

- [ ] **Step 8: Commit**

```bash
git add backend/internal/handlers/webhook.go backend/internal/handlers/webhook_test.go
git commit -m "feat(webhook): add role_keys[] for multi-role grant events"
```

---

## Phase 3 — Backend: route + middleware

### Task 3.1: Switch `/api/webhooks/zitadel` to `withZitadelActionSignature` (TDD)

**Files:**
- Modify: `backend/internal/handlers/router.go:130`
- Modify: `backend/internal/handlers/webhook.go` (remove inline signature/freshness calls)
- Test: `backend/internal/handlers/webhook_test.go`

- [ ] **Step 1: Write the failing test**

Add to `webhook_test.go`:

```go
func TestWebhookRoute_RejectsInvalidActionsV2Signature(t *testing.T) {
	t.Setenv("ZITADEL_EVENT_SIGNING_KEY", "test-key")

	body := []byte(`{"event_type":"user_created","user_id":"u1","source_project":"p1"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/zitadel", bytes.NewReader(body))
	req.Header.Set("ZITADEL-Signature", "t=0,v1=deadbeef")
	rr := httptest.NewRecorder()

	mux := http.NewServeMux()
	registerRoutes(mux) // helper that mounts production routes; add if missing
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 from middleware, got %d", rr.Code)
	}
}
```

(If `registerRoutes` does not exist as a test helper, expose `RegisterRoutes(mux *http.ServeMux)` from `router.go` — this is a reasonable refactor since the production `main` already calls `mux.HandleFunc` directly. If you don't want to refactor router.go yet, instead call `withZitadelActionSignature("ZITADEL_EVENT_SIGNING_KEY", HandleZitadelWebhook)(rr, req)` directly from the test.)

- [ ] **Step 2: Run test and verify it fails**

Run: `cd backend && go test ./internal/handlers -run TestWebhookRoute_RejectsInvalidActionsV2Signature -v`
Expected: FAIL — current route is unprotected, returns 200/400/401 with the legacy code path, not the canonical INVALID_SIGNATURE response.

- [ ] **Step 3: Update route registration**

Edit `backend/internal/handlers/router.go:130`:

```go
mux.HandleFunc("POST /api/webhooks/zitadel",
	withCORS(withZitadelActionSignature("ZITADEL_EVENT_SIGNING_KEY", HandleZitadelWebhook)))
```

- [ ] **Step 4: Remove inline signature/freshness calls from `HandleZitadelWebhook`**

Edit `backend/internal/handlers/webhook.go:53-66` — delete the legacy auth/freshness block:

```go
// REMOVE THESE LINES:
// tsHeader := r.Header.Get("X-Zitadel-Timestamp")
// if err := verifyWebhookSignature(body, tsHeader, r.Header.Get("X-Zitadel-Signature")); err != nil { ... }
// if err := verifyWebhookFreshness(tsHeader); err != nil { ... }
```

The middleware now handles signature + freshness (300s tolerance via `verifyZitadelActionSignature`). The handler keeps body parsing, idempotency-key derivation (now off `ZITADEL-Signature` header instead of `X-Zitadel-Signature`), and dispatch.

Update idempotency-key fallback at `webhook.go:108-111`:

```go
// Use the Zitadel-Signature header as idempotency key when available — unique per
// (timestamp, body) per Zitadel signing semantics. Fallback for internal-shape
// callers (operator curl, contracts tests) uses payload+timestamp.
idempotencyKey := r.Header.Get("ZITADEL-Signature")
if idempotencyKey == "" {
	idempotencyKey = fmt.Sprintf("%s:%s:%s:%s",
		event.EventType, event.UserID, event.SourceProject, event.RoleKey)
}
```

- [ ] **Step 5: Run the test**

Run: `cd backend && go test ./internal/handlers -run TestWebhookRoute_RejectsInvalidActionsV2Signature -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/handlers/router.go backend/internal/handlers/webhook.go backend/internal/handlers/webhook_test.go
git commit -m "feat(webhook): switch /api/webhooks/zitadel to Actions v2 signature scheme"
```

### Task 3.2: Retire legacy `ZITADEL_WEBHOOK_SECRET`

**Files:**
- Modify: `backend/internal/handlers/webhook.go` (delete `verifyWebhookSignature`, `verifyWebhookFreshness`, unused imports)
- Modify: `backend/internal/handlers/webhook_test.go` (delete legacy verifier tests)
- Modify: `.env.example`
- Modify: `docker-compose.yml`

- [ ] **Step 1: Delete the legacy helpers**

Delete `backend/internal/handlers/webhook.go:218-267` (both `verifyWebhookSignature` and `verifyWebhookFreshness` functions). Remove now-unused imports:

```diff
 import (
 	"bytes"
 	"context"
-	"crypto/hmac"
-	"crypto/sha256"
-	"encoding/hex"
 	"fmt"
 	"io"
 	"log"
 	"net/http"
-	"os"
-	"strconv"
-	"time"
 )
```

(Verify which imports are still referenced — `os`, `time`, `strconv` may be used elsewhere in the file. Run `goimports -w backend/internal/handlers/webhook.go` to auto-resolve.)

- [ ] **Step 2: Delete the legacy verifier tests**

Delete the 9 tests in `backend/internal/handlers/webhook_test.go` that target `verifyWebhookSignature` / `verifyWebhookFreshness`:

```
TestVerifyWebhookSignature_ValidSignature
TestVerifyWebhookSignature_InvalidSignature
TestVerifyWebhookSignature_FreshTimestampWithStaleBodySignature
TestVerifyWebhookSignature_MissingSignatureHeader
TestVerifyWebhookSignature_MissingTimestampHeader
TestVerifyWebhookSignature_NoSecretLocalDev
TestVerifyWebhookFreshness_FreshTimestamp
TestVerifyWebhookFreshness_StaleTimestamp
TestVerifyWebhookFreshness_MissingTimestamp
TestVerifyWebhookFreshness_NoSecretLocalDev
TestHandleZitadelWebhook_RejectsInvalidSignature
TestHandleZitadelWebhook_RejectsStaleTimestamp
```

These are now redundant — `withZitadelActionSignature` is already covered by `zitadel_action_auth_test.go`.

Also delete the unused `signWebhook` helper if present.

- [ ] **Step 3: Run tests**

Run: `cd backend && go test ./internal/handlers -v && go vet ./internal/handlers`
Expected: PASS, no warnings, no orphaned references.

- [ ] **Step 4: Remove `ZITADEL_WEBHOOK_SECRET` from `.env.example`**

Edit `.env.example:41` — delete the line:
```diff
-# ZITADEL_WEBHOOK_SECRET=your-webhook-secret
```

(The new `ZITADEL_EVENT_SIGNING_KEY` block lands in Task 6.2; keep this commit focused on legacy removal.)

- [ ] **Step 5: Remove from `docker-compose.yml`**

Edit `docker-compose.yml:47` — delete:
```diff
-      - ZITADEL_WEBHOOK_SECRET=${ZITADEL_WEBHOOK_SECRET:-}
```

- [ ] **Step 6: Commit**

```bash
git add backend/internal/handlers/webhook.go backend/internal/handlers/webhook_test.go .env.example docker-compose.yml
git commit -m "refactor: retire legacy ZITADEL_WEBHOOK_SECRET HMAC scheme"
```

---

## Phase 4 — Backend: payload translator

### Task 4.1: Define `ZitadelEventPayload` skeleton + shape detection (TDD)

**Files:**
- Create: `backend/internal/handlers/webhook_translate.go`
- Create: `backend/internal/handlers/webhook_translate_test.go`

- [ ] **Step 1: Write the failing test**

Create `backend/internal/handlers/webhook_translate_test.go`:

```go
package handlers

import (
	"testing"
)

func TestTranslateZitadelEvent_DetectsZitadelShape(t *testing.T) {
	zitadelBody := []byte(`{
		"aggregate": {"id": "user-123", "type": "user", "resourceOwner": "org-1"},
		"event": "user.human.added",
		"editorUserId": "editor-9",
		"payload": {}
	}`)

	got, ok, err := translateZitadelEvent(zitadelBody)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected zitadel shape detected")
	}
	if got.EventType != "user_created" {
		t.Errorf("expected user_created, got %q", got.EventType)
	}
	if got.UserID != "user-123" {
		t.Errorf("expected user_id=user-123, got %q", got.UserID)
	}
}

func TestTranslateZitadelEvent_PassesInternalShape(t *testing.T) {
	internal := []byte(`{"event_type":"user_created","user_id":"u1","source_project":"p1"}`)
	_, ok, err := translateZitadelEvent(internal)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatalf("expected ok=false for internal shape (no aggregate field)")
	}
}
```

- [ ] **Step 2: Run test and verify it fails**

Run: `cd backend && go test ./internal/handlers -run TestTranslateZitadelEvent -v`
Expected: FAIL with `undefined: translateZitadelEvent`.

- [ ] **Step 3: Implement skeleton**

Create `backend/internal/handlers/webhook_translate.go`:

```go
package handlers

import (
	"encoding/json"
	"log"
	"os"
)

// zitadelEventPayload is a lenient struct mirroring the Zitadel Actions v2
// event-trigger payload. Field paths verified empirically — capture a real
// payload via dev-mode pass-through (ZITADEL_EVENT_SIGNING_KEY unset) before
// flipping signature verification on. Unknown fields are ignored to immunize
// the translator against future Zitadel additions.
type zitadelEventPayload struct {
	Aggregate struct {
		ID            string `json:"id"`
		Type          string `json:"type"`
		ResourceOwner string `json:"resourceOwner"`
	} `json:"aggregate"`
	Event        string          `json:"event"`        // e.g. "user.human.added"
	EditorUserID string          `json:"editorUserId"` // who triggered the change
	Payload      json.RawMessage `json:"payload"`      // event-specific body
}

// userGrantPayload covers user.grant.* events.
type userGrantPayload struct {
	UserID    string   `json:"userId"`
	ProjectID string   `json:"projectId"`
	RoleKeys  []string `json:"roleKeys"`
}

// translateZitadelEvent inspects a request body. If it has a top-level
// "aggregate" object (the Zitadel-shape signal), it translates to a
// WebhookPayload and returns ok=true. Otherwise returns ok=false to let the
// caller fall back to internal-shape strict decoding.
//
// Self-mutation loop guard: when ZITADEL_M2M_USER_ID is set and matches
// payload.editorUserId, returns (zero, true, errSelfMutation) — caller MUST
// short-circuit with 200 OK and no dispatch.
func translateZitadelEvent(body []byte) (WebhookPayload, bool, error) {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(body, &probe); err != nil {
		return WebhookPayload{}, false, nil // not JSON we can read; let strict path 400
	}
	if _, hasAgg := probe["aggregate"]; !hasAgg {
		return WebhookPayload{}, false, nil
	}

	var ev zitadelEventPayload
	if err := json.Unmarshal(body, &ev); err != nil {
		return WebhookPayload{}, true, err
	}

	// Self-mutation guard. Empty M2M user ID disables the guard (local-dev).
	if m2mID := os.Getenv("ZITADEL_M2M_USER_ID"); m2mID != "" && ev.EditorUserID == m2mID {
		log.Printf("[WEBHOOK] dropped self-mutation event=%s editor=%s", ev.Event, ev.EditorUserID)
		return WebhookPayload{}, true, errSelfMutation
	}

	mapped := translateEventName(ev)
	return mapped, true, nil
}

var errSelfMutation = sentinelError("zitadel event triggered by Syndra's own M2M user — dropped")

type sentinelError string

func (e sentinelError) Error() string { return string(e) }

// translateEventName dispatches per-event mapping. Unknown events return a
// zero-value WebhookPayload with EventType="" — the caller MUST treat this as
// "200 OK no-op" (matches the unknown-event passthrough scenario).
func translateEventName(ev zitadelEventPayload) WebhookPayload {
	base := WebhookPayload{UserID: ev.Aggregate.ID}
	switch ev.Event {
	case "user.human.added", "user.human.selfregistered":
		base.EventType = "user_created"
	case "user.human.deactivated":
		base.EventType = "user_deactivated"
	case "user.human.locked":
		base.EventType = "user_locked"
	case "user.grant.added", "user.user.grant.added":
		return mapGrantEvent("grant_added", ev)
	case "user.grant.changed", "user.user.grant.changed":
		return mapGrantEvent("grant_changed", ev)
	case "user.grant.removed", "user.user.grant.removed":
		return mapGrantEvent("grant_removed", ev)
	default:
		log.Printf("[WEBHOOK] unknown event=%s aggregate=%s — ignoring", ev.Event, ev.Aggregate.ID)
		return WebhookPayload{} // EventType empty signals no-op
	}
	return base
}

func mapGrantEvent(eventType string, ev zitadelEventPayload) WebhookPayload {
	var grant userGrantPayload
	_ = json.Unmarshal(ev.Payload, &grant) // tolerate missing fields; downstream validates
	out := WebhookPayload{
		EventType:     eventType,
		UserID:        firstNonEmpty(grant.UserID, ev.Aggregate.ID),
		SourceProject: grant.ProjectID,
		RoleKeys:      grant.RoleKeys,
	}
	if len(out.RoleKeys) > 0 {
		out.RoleKey = out.RoleKeys[0]
	}
	out.ProjectIDs = []string{grant.ProjectID}
	return out
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
```

- [ ] **Step 4: Run tests**

Run: `cd backend && go test ./internal/handlers -run TestTranslateZitadelEvent -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/handlers/webhook_translate.go backend/internal/handlers/webhook_translate_test.go
git commit -m "feat(webhook): add Zitadel event payload translator skeleton"
```

### Task 4.2: Test + verify all event mappings

**Files:**
- Modify: `backend/internal/handlers/webhook_translate_test.go`

- [ ] **Step 1: Write a table-driven test for all six event types**

Append to `webhook_translate_test.go`:

```go
func TestTranslateEventName_AllMappings(t *testing.T) {
	cases := []struct {
		name      string
		event     string
		aggID     string
		payload   string
		wantType  string
		wantUser  string
		wantRoles []string
	}{
		{"user added", "user.human.added", "u1", `{}`, "user_created", "u1", nil},
		{"self registered", "user.human.selfregistered", "u2", `{}`, "user_created", "u2", nil},
		{"deactivated", "user.human.deactivated", "u3", `{}`, "user_deactivated", "u3", nil},
		{"locked", "user.human.locked", "u4", `{}`, "user_locked", "u4", nil},
		{"grant added", "user.grant.added", "g1", `{"userId":"u5","projectId":"p1","roleKeys":["alpha","beta"]}`, "grant_added", "u5", []string{"alpha", "beta"}},
		{"grant changed", "user.grant.changed", "g2", `{"userId":"u6","projectId":"p2","roleKeys":["gamma"]}`, "grant_changed", "u6", []string{"gamma"}},
		{"grant removed", "user.grant.removed", "g3", `{"userId":"u7","projectId":"p3","roleKeys":["delta"]}`, "grant_removed", "u7", []string{"delta"}},
		{"unknown", "user.password.changed", "u8", `{}`, "", "u8", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := []byte(`{"aggregate":{"id":"` + tc.aggID + `"},"event":"` + tc.event + `","payload":` + tc.payload + `}`)
			got, ok, err := translateZitadelEvent(body)
			if err != nil || !ok {
				t.Fatalf("translate failed: ok=%v err=%v", ok, err)
			}
			if got.EventType != tc.wantType {
				t.Errorf("event_type: want %q, got %q", tc.wantType, got.EventType)
			}
			if got.UserID != tc.wantUser {
				t.Errorf("user_id: want %q, got %q", tc.wantUser, got.UserID)
			}
			if !slices.Equal(got.RoleKeys, tc.wantRoles) {
				t.Errorf("role_keys: want %v, got %v", tc.wantRoles, got.RoleKeys)
			}
		})
	}
}

func TestTranslateZitadelEvent_SelfMutationGuard(t *testing.T) {
	t.Setenv("ZITADEL_M2M_USER_ID", "service-user-99")
	body := []byte(`{"aggregate":{"id":"u1"},"event":"user.grant.added","editorUserId":"service-user-99","payload":{}}`)
	_, ok, err := translateZitadelEvent(body)
	if !ok {
		t.Fatal("expected ok=true (zitadel shape detected)")
	}
	if err != errSelfMutation {
		t.Fatalf("expected errSelfMutation, got %v", err)
	}
}
```

- [ ] **Step 2: Run tests**

Run: `cd backend && go test ./internal/handlers -run TestTranslateEventName_AllMappings -v && go test ./internal/handlers -run TestTranslateZitadelEvent_SelfMutationGuard -v`
Expected: all subtests PASS.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/handlers/webhook_translate_test.go
git commit -m "test(webhook): cover all Zitadel event mappings + self-loop guard"
```

### Task 4.3: Wire translator into `HandleZitadelWebhook` (TDD)

**Files:**
- Modify: `backend/internal/handlers/webhook.go` (around the body-decode block, lines ~67-77)
- Modify: `backend/internal/handlers/webhook_test.go`

- [ ] **Step 1: Write failing integration test**

Add to `webhook_test.go`:

```go
func TestHandleZitadelWebhook_TranslatesZitadelShape(t *testing.T) {
	t.Setenv("ZITADEL_EVENT_SIGNING_KEY", "")
	t.Setenv("ZITADEL_M2M_USER_ID", "")

	// Stub deps as in TestHandleZitadelWebhook_RoleKeysPlural_DispatchesEachRole
	dbInsertWebhookEventStub(t)
	cacheRebuildStub(t)
	webhookEmitProvisioningIntentStub(t)

	var seenRoles []string
	prev := webhookEnforceMappingRules
	webhookEnforceMappingRules = func(ctx context.Context, userID, project, role string) error {
		seenRoles = append(seenRoles, role)
		return nil
	}
	t.Cleanup(func() { webhookEnforceMappingRules = prev })

	body := []byte(`{
		"aggregate": {"id":"agg-1","type":"user","resourceOwner":"org-1"},
		"event": "user.grant.added",
		"editorUserId": "human-operator-1",
		"payload": {"userId":"u1","projectId":"p1","roleKeys":["alpha","beta"]}
	}`)
	rr := postWebhook(t, body)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	if !slices.Equal(seenRoles, []string{"alpha", "beta"}) {
		t.Fatalf("expected [alpha beta], got %v", seenRoles)
	}
}

func TestHandleZitadelWebhook_DropsSelfMutation(t *testing.T) {
	t.Setenv("ZITADEL_EVENT_SIGNING_KEY", "")
	t.Setenv("ZITADEL_M2M_USER_ID", "service-user-99")

	called := false
	prev := webhookEnforceMappingRules
	webhookEnforceMappingRules = func(ctx context.Context, _, _, _ string) error {
		called = true
		return nil
	}
	t.Cleanup(func() { webhookEnforceMappingRules = prev })

	body := []byte(`{"aggregate":{"id":"u1"},"event":"user.grant.added","editorUserId":"service-user-99","payload":{"userId":"u1","projectId":"p1","roleKeys":["x"]}}`)
	rr := postWebhook(t, body)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 short-circuit, got %d", rr.Code)
	}
	if called {
		t.Fatal("orchestrator MUST NOT be called on self-mutation event")
	}
}
```

- [ ] **Step 2: Run tests and verify failure**

Run: `cd backend && go test ./internal/handlers -run "TestHandleZitadelWebhook_TranslatesZitadelShape|TestHandleZitadelWebhook_DropsSelfMutation" -v`
Expected: FAIL — `event_type` validation rejects (Zitadel-shape body has no top-level event_type).

- [ ] **Step 3: Wire translator into the handler**

Edit `backend/internal/handlers/webhook.go` — replace the body-decode block (currently around lines 60-77) with:

```go
// Try Zitadel-shape translation first; fall back to internal strict decode.
var event WebhookPayload
translated, isZitadel, err := translateZitadelEvent(body)
if err == errSelfMutation {
	jsonResponse(w, http.StatusOK, map[string]string{"message": "self-mutation event dropped"})
	return
}
if err != nil {
	jsonValidationErrorResponse(w, "Invalid Zitadel event payload", map[string]string{"body": err.Error()})
	return
}
if isZitadel {
	if translated.EventType == "" {
		// Unknown / unsupported Zitadel event — no-op success.
		jsonResponse(w, http.StatusOK, map[string]string{"message": "event acknowledged, no dispatch"})
		return
	}
	event = translated
} else {
	if err := decodeJSONStrict(bytes.NewReader(body), &event); err != nil {
		jsonValidationErrorResponse(w, "Invalid webhook payload", map[string]string{"body": err.Error()})
		return
	}
}

// Default event_type for backward compat (internal-shape callers only).
if event.EventType == "" {
	event.EventType = "grant_added"
}
```

- [ ] **Step 4: Run tests**

Run: `cd backend && go test ./internal/handlers -run "TestHandleZitadelWebhook_TranslatesZitadelShape|TestHandleZitadelWebhook_DropsSelfMutation" -v`
Expected: PASS.

Run full handlers package: `cd backend && go test ./internal/handlers -v`
Expected: every existing test still passes.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/handlers/webhook.go backend/internal/handlers/webhook_test.go
git commit -m "feat(webhook): translate Zitadel event payloads into internal dispatch"
```

---

## Phase 5 — Deployment: multi-target manifest + register.sh

### Task 5.1: Reshape `targets.json`

**Files:**
- Modify: `zitadel/actions/targets.json`

- [ ] **Step 1: Replace the manifest with multi-target schema**

Overwrite `zitadel/actions/targets.json`:

```json
{
  "_comment": "Multi-target manifest applied by register.sh. Each target is registered by name (idempotent upsert via /v2/actions/targets/search). Each execution binds by referencing the target's name (resolved to ID at registration time). Layouts match proto/zitadel/action/v2/{target,execution,query}.proto.",
  "targets": [
    {
      "name": "syndra-claim-injector",
      "endpoint": "${SYNDRA_EXTERNAL_URL}/api/action/inject",
      "timeout": "3s",
      "payloadType": "PAYLOAD_TYPE_JSON",
      "restCall": {
        "interruptOnError": false,
        "_note": "interruptOnError:false keeps token issuance unblocked if Syndra is unreachable. Per-project fail_closed/minimal_safe is decided server-side."
      },
      "_signing_key_env": "ZITADEL_ACTION_SIGNING_KEY"
    },
    {
      "name": "syndra-event-listener",
      "endpoint": "${SYNDRA_EXTERNAL_URL}/api/webhooks/zitadel",
      "timeout": "5s",
      "payloadType": "PAYLOAD_TYPE_JSON",
      "restAsync": {
        "_note": "Fire-and-forget. Zitadel does not parse the response body; failures retry per Zitadel's at-least-once delivery semantics. Syndra's idempotency-key dedup catches replays."
      },
      "_signing_key_env": "ZITADEL_EVENT_SIGNING_KEY"
    }
  ],
  "executions": [
    { "target": "syndra-claim-injector", "condition": { "function": { "name": "preaccesstoken" } } },
    { "target": "syndra-claim-injector", "condition": { "function": { "name": "preuserinfo" } } },
    { "target": "syndra-event-listener", "condition": { "event":    { "event": "user.human.added" } } },
    { "target": "syndra-event-listener", "condition": { "event":    { "event": "user.human.selfregistered" } } },
    { "target": "syndra-event-listener", "condition": { "event":    { "event": "user.human.deactivated" } } },
    { "target": "syndra-event-listener", "condition": { "event":    { "event": "user.human.locked" } } },
    { "target": "syndra-event-listener", "condition": { "event":    { "event": "user.grant.added" } } },
    { "target": "syndra-event-listener", "condition": { "event":    { "event": "user.grant.changed" } } },
    { "target": "syndra-event-listener", "condition": { "event":    { "event": "user.grant.removed" } } }
  ]
}
```

(The exact event-name strings — `user.human.*` vs `user.human.added` vs `user.user.grant.added` — depend on the Zitadel version. Verify against the staging instance during Phase 7. The translator already accepts both `user.grant.*` and `user.user.grant.*` to soften this.)

- [ ] **Step 2: Validate JSON**

Run: `jq -e . zitadel/actions/targets.json >/dev/null && echo OK`
Expected: `OK`.

- [ ] **Step 3: Commit**

```bash
git add zitadel/actions/targets.json
git commit -m "feat(zitadel): multi-target manifest with event-listener target"
```

### Task 5.2: Extend `register.sh` to iterate targets[]

**Files:**
- Modify: `zitadel/actions/register.sh`

- [ ] **Step 1: Replace the single-target block with the multi-target loop**

The current `register.sh` has a single-target block (around lines 130-200 in the file you read in Phase 0). Replace the section starting at `TARGET_NAME="$(echo "$RENDERED_MANIFEST" | jq -r '.target.name')"` through the end of the file with:

```bash
# Map: target name -> registered/looked-up target ID. Populated as we walk
# .targets[]; consumed when binding executions.
declare -A TARGET_IDS

# Process each target in the manifest.
TARGET_COUNT="$(echo "$RENDERED_MANIFEST" | jq '.targets | length')"
for ((i = 0; i < TARGET_COUNT; i++)); do
  T="$(echo "$RENDERED_MANIFEST" | jq -c ".targets[$i]")"
  TARGET_NAME="$(echo "$T" | jq -r '.name')"
  SIGNING_KEY_ENV="$(echo "$T" | jq -r '._signing_key_env // empty')"
  TARGET_BODY="$(echo "$T" | jq 'del(._signing_key_env)')"
  SIGNING_KEY_FILE="${SCRIPT_DIR}/.action-signing-key.${TARGET_NAME}"

  echo "Searching for target name=${TARGET_NAME}..." >&2
  SEARCH_BODY="$(jq -n --arg n "$TARGET_NAME" '{
    filters: [{ target_name_filter: { target_name: $n, method: "TEXT_FILTER_METHOD_EQUALS" } }],
    pagination: { limit: 1 }
  }')"
  LIST_RESP="$(zitadel_api POST /targets/search "$SEARCH_BODY")" || exit 5
  EXISTING_ID="$(echo "$LIST_RESP" | jq -r '.targets[0].id // .result[0].id // empty' 2>/dev/null || true)"

  if [[ "${1:-}" == "--remove" ]]; then
    # Remove path: just record IDs so the unbind loop below can reach them.
    [[ -n "$EXISTING_ID" ]] && TARGET_IDS[$TARGET_NAME]="$EXISTING_ID"
    continue
  fi

  if [[ -n "$EXISTING_ID" ]]; then
    echo "Updating target id=${EXISTING_ID} name=${TARGET_NAME}..." >&2
    zitadel_api POST "/targets/${EXISTING_ID}" "$TARGET_BODY" >/dev/null || exit 7
    TARGET_IDS[$TARGET_NAME]="$EXISTING_ID"
    if [[ ! -s "$SIGNING_KEY_FILE" ]]; then
      echo "warning: target ${TARGET_NAME} exists but ${SIGNING_KEY_FILE} is missing." >&2
      echo "         The signing key is only returned at target-creation time." >&2
      echo "         Rotate via: make zitadel-actions-rotate-key TARGET=${TARGET_NAME}" >&2
    fi
  else
    echo "Creating target name=${TARGET_NAME}..." >&2
    CREATE_RESP="$(zitadel_api POST /targets "$TARGET_BODY")" || exit 8
    TARGET_ID="$(echo "$CREATE_RESP" | jq -r '.id')"
    SIGNING_KEY="$(echo "$CREATE_RESP" | jq -r '.signingKey // empty')"
    if [[ -z "$TARGET_ID" || "$TARGET_ID" == "null" ]]; then
      echo "error: CreateTarget did not return an id for ${TARGET_NAME}. Response was:" >&2
      echo "$CREATE_RESP" >&2
      exit 3
    fi
    if [[ -z "$SIGNING_KEY" ]]; then
      echo "error: target ${TARGET_NAME} created but no signingKey in response — aborting." >&2
      echo "$CREATE_RESP" >&2
      exit 4
    fi
    umask 077
    printf '%s\n' "$SIGNING_KEY" > "$SIGNING_KEY_FILE"
    echo "Signing key for ${TARGET_NAME} written to ${SIGNING_KEY_FILE} (mode 0600)." >&2
    TARGET_IDS[$TARGET_NAME]="$TARGET_ID"

    # Append to env-fragment file so operators can apply both keys with one
    # `cat .action-env.fragment >> .env`.
    FRAGMENT_FILE="${SCRIPT_DIR}/.action-env.fragment"
    if [[ -n "$SIGNING_KEY_ENV" ]]; then
      umask 077
      printf '%s=%s\n' "$SIGNING_KEY_ENV" "$SIGNING_KEY" >> "$FRAGMENT_FILE"
      printf '%s=%s\n' "${SIGNING_KEY_ENV}_ROTATED_AT" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$FRAGMENT_FILE"
      echo "  → appended ${SIGNING_KEY_ENV} to ${FRAGMENT_FILE}" >&2
    fi
  fi
done

# --- Bind executions to the right target IDs ---
echo "Binding executions..." >&2
EXEC_COUNT="$(echo "$RENDERED_MANIFEST" | jq '.executions | length')"
for ((i = 0; i < EXEC_COUNT; i++)); do
  EXEC="$(echo "$RENDERED_MANIFEST" | jq -c ".executions[$i]")"
  TARGET_NAME="$(echo "$EXEC" | jq -r '.target')"
  COND="$(echo "$EXEC" | jq -c '.condition')"
  TARGET_ID="${TARGET_IDS[$TARGET_NAME]:-}"
  if [[ -z "$TARGET_ID" ]]; then
    echo "error: execution references unknown target=${TARGET_NAME}" >&2
    exit 9
  fi

  if [[ "${1:-}" == "--remove" ]]; then
    BIND_BODY="$(jq -n --argjson c "$COND" '{ condition: $c, targets: [] }')"
  else
    BIND_BODY="$(jq -n --argjson c "$COND" --arg tid "$TARGET_ID" '{ condition: $c, targets: [$tid] }')"
  fi
  zitadel_api PUT /executions "$BIND_BODY" >/dev/null || exit 10
done

echo "Done." >&2
echo "Targets: $(echo "${!TARGET_IDS[@]}" | tr ' ' ',')" >&2
if [[ "${1:-}" != "--remove" ]] && [[ -f "${SCRIPT_DIR}/.action-env.fragment" ]]; then
  echo "" >&2
  echo "Apply captured signing keys to .env with:" >&2
  echo "  cat zitadel/actions/.action-env.fragment >> .env" >&2
  echo "Then restart the backend so it picks up the new env vars." >&2
fi
```

Note: `declare -A` requires Bash 4+. Add at the top of the script (after `set -euo pipefail`):

```bash
if (( BASH_VERSINFO[0] < 4 )); then
  echo "error: register.sh requires bash 4+ (associative arrays)" >&2
  echo "  macOS default is bash 3; install via 'brew install bash' and rerun." >&2
  exit 1
fi
```

- [ ] **Step 2: Update the rendered-manifest jq pipeline**

Find the `RENDERED_MANIFEST="$(jq …` block and ensure it walks BOTH targets[] entries (the existing walk already does — `walk(if type == "string" …)` — so no change needed). But verify by running:

```bash
ZITADEL_DOMAIN=example.com SYNDRA_EXTERNAL_URL=https://x.example.com \
  bash -c '_RENDERED_MANIFEST="$(jq --arg url "https://x.example.com" "
    walk(if type == \"string\" and test(\"\\\\\\\$\\\\{SYNDRA_EXTERNAL_URL\\\\}\")
         then sub(\"\\\\\\\$\\\\{SYNDRA_EXTERNAL_URL\\\\}\"; \$url) else . end)
    | walk(if type == \"object\" then with_entries(select(.key | startswith(\"_\") | not)) else . end)
  " zitadel/actions/targets.json)"; echo "$_RENDERED_MANIFEST" | jq ".targets[].endpoint"'
```

Expected output:
```
"https://x.example.com/api/action/inject"
"https://x.example.com/api/webhooks/zitadel"
```

- [ ] **Step 3: Syntax-check**

Run: `bash -n zitadel/actions/register.sh`
Expected: clean.

- [ ] **Step 4: Commit**

```bash
git add zitadel/actions/register.sh
git commit -m "feat(zitadel): register.sh handles multi-target manifest"
```

### Task 5.3: Update `rotate.sh` for per-target rotation

**Files:**
- Modify: `zitadel/actions/rotate.sh`

- [ ] **Step 1: Add `--target NAME` flag handling**

Open `zitadel/actions/rotate.sh`. The current script rotates the single target. Modify to accept an optional first arg `--target <NAME>` (default `syndra-claim-injector` for back-compat). When unset and multiple targets exist, iterate over all.

Add near the top, after env-loading and before the target lookup:

```bash
TARGET_FILTER=""
case "${1:-}" in
  --target) TARGET_FILTER="${2:?--target requires a name argument}"; shift 2 ;;
esac
```

Replace the single-target lookup with:

```bash
RENDERED_MANIFEST="$(jq --arg url "$SYNDRA_EXTERNAL_URL" '
  walk(if type == "string" and test("\\$\\{SYNDRA_EXTERNAL_URL\\}")
       then sub("\\$\\{SYNDRA_EXTERNAL_URL\\}"; $url) else . end)
  | walk(if type == "object" then with_entries(select(.key | startswith("_") | not)) else . end)
' "$MANIFEST")"

# When .targets[] exists (multi-target manifest) iterate; else fall back to legacy .target.
if echo "$RENDERED_MANIFEST" | jq -e '.targets' >/dev/null; then
  COUNT="$(echo "$RENDERED_MANIFEST" | jq '.targets | length')"
  for ((i = 0; i < COUNT; i++)); do
    T="$(echo "$RENDERED_MANIFEST" | jq -c ".targets[$i]")"
    NAME="$(echo "$T" | jq -r '.name')"
    if [[ -n "$TARGET_FILTER" && "$NAME" != "$TARGET_FILTER" ]]; then continue; fi
    rotate_target "$NAME" "$(echo "$T" | jq -r '._signing_key_env // empty')"
  done
else
  NAME="$(echo "$RENDERED_MANIFEST" | jq -r '.target.name')"
  rotate_target "$NAME" "ZITADEL_ACTION_SIGNING_KEY"
fi
```

Extract the rotation body into a function `rotate_target()` that takes name + env-var name and writes to per-target files (`.action-signing-key.<name>` + `.action-env.fragment`). The existing rotation logic (search by name → POST `/targets/{id}` with `expirationSigningKey:0s` → capture new `signingKey`) stays the same; just parameterize.

- [ ] **Step 2: Syntax-check**

Run: `bash -n zitadel/actions/rotate.sh`
Expected: clean.

- [ ] **Step 3: Update `.gitignore`**

Edit `zitadel/actions/.gitignore` (already excludes `.action-signing-key`):

```
.action-signing-key
.action-signing-key.*
.action-signing-key.previous
.action-signing-key.previous.*
.action-env.fragment
```

- [ ] **Step 4: Commit**

```bash
git add zitadel/actions/rotate.sh zitadel/actions/.gitignore
git commit -m "feat(zitadel): rotate.sh per-target rotation with --target flag"
```

### Task 5.4: Update `Makefile` targets

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Inspect existing targets**

Run: `grep -n "zitadel-actions" Makefile`
Expected: see `zitadel-actions-register`, `zitadel-actions-remove`, `zitadel-actions-verify`, `zitadel-actions-rotate-key`.

- [ ] **Step 2: Add a TARGET param for rotate**

Modify the `zitadel-actions-rotate-key` recipe to accept an optional `TARGET=`:

```make
.PHONY: zitadel-actions-rotate-key
zitadel-actions-rotate-key:
ifdef TARGET
	bash zitadel/actions/rotate.sh --target "$(TARGET)"
else
	bash zitadel/actions/rotate.sh
endif
```

- [ ] **Step 3: Add a verify target for the event listener**

Append:

```make
.PHONY: zitadel-actions-verify-events
zitadel-actions-verify-events:
	bash scripts/smoke-test-event-listener.sh
```

(The smoke-test script lands in Task 6.3.)

- [ ] **Step 4: Commit**

```bash
git add Makefile
git commit -m "build: parameterize rotate-key make target; add verify-events"
```

---

## Phase 6 — Ops + docs

### Task 6.1: Add `zitadel/actions/EVENTS.md` (durable operator runbook)

**Files:**
- Create: `zitadel/actions/EVENTS.md`

- [ ] **Step 1: Write the runbook**

```markdown
# Zitadel Event-Listener Target

The `syndra-event-listener` Action target receives lifecycle events from Zitadel and POSTs them to Syndra's `/api/webhooks/zitadel` endpoint, driving:

- Welcome-bundle assignment on user creation (`user.human.added` → `processUserCreated` → `TriggerOnboarding` → `AssignBundleToUser`).
- Mapping-rule cascade on grant additions (`user.grant.added` → `EnforceMappingRules`).
- Mapping-rule revocation on grant removals (`user.grant.removed` → `RevokeMappingRules`).
- Cache invalidation on user deactivation/lock (`user.human.deactivated`, `user.human.locked`).

## Subscribed events

| Zitadel event | Syndra `event_type` | Downstream effect |
|---|---|---|
| `user.human.added` | `user_created` | Onboarding trigger → welcome bundle |
| `user.human.selfregistered` | `user_created` | Same |
| `user.human.deactivated` | `user_deactivated` | Cache invalidation |
| `user.human.locked` | `user_locked` | Cache invalidation |
| `user.grant.added` | `grant_added` (per role) | Cache rebuild + mapping rules + LLDAP intent |
| `user.grant.changed` | `grant_changed` (per role) | Same |
| `user.grant.removed` | `grant_removed` (per role) | Cache invalidation + revoke + LLDAP intent |

Unknown events are acknowledged with 200 OK and logged but not dispatched.

## Self-mutation guard

When Syndra's backend mutates Zitadel via Management API (e.g. `RemoveUserGrant`), Zitadel emits the corresponding event back to the listener. Without filtering, you get an infinite loop. The translator drops events whose `editorUserId` matches `ZITADEL_M2M_USER_ID`.

To find the M2M service-user ID, after the first successful Management API call check the Zitadel event log: any event with the resource you mutated will show the editor user ID. Set it in `.env`:

```
ZITADEL_M2M_USER_ID=<the-id>
```

Unset disables the guard (acceptable in local-dev; required in any environment that accepts real Zitadel traffic).

## Signing key

Captured at target creation by `register.sh` and written to `zitadel/actions/.action-signing-key.syndra-event-listener` (mode 0600). Apply to backend env via the env-fragment writer:

```bash
cat zitadel/actions/.action-env.fragment >> .env
docker compose up -d backend
```

Rotation:

```bash
make zitadel-actions-rotate-key TARGET=syndra-event-listener
```

Same rotation flow as the claim injector — see `SIGNING_KEY.md`.

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| Welcome bundle never assigned on new user | Listener target not registered, or signing key mismatch | `make zitadel-actions-register`; check `webhook_events` table for failed rows |
| Loop: same user grant mutated repeatedly | `ZITADEL_M2M_USER_ID` unset or wrong | Set the env var to the backend's M2M service-user ID |
| `401 INVALID_SIGNATURE` in backend logs | Stale or wrong `ZITADEL_EVENT_SIGNING_KEY` | Rotate via `make zitadel-actions-rotate-key TARGET=syndra-event-listener` |
| Unknown-event log spam | Zitadel emitting events Syndra doesn't subscribe to (unlikely if executions are pinned) | Check executions list in Zitadel console; remove unwanted bindings |
```

- [ ] **Step 2: Update `zitadel/actions/README.md` Contents table**

Add a row for `EVENTS.md`:

```markdown
| `EVENTS.md` | Event-listener target reference: subscribed events, self-mutation guard, troubleshooting. |
```

- [ ] **Step 3: Commit**

```bash
git add zitadel/actions/EVENTS.md zitadel/actions/README.md
git commit -m "docs(zitadel): EVENTS.md operator runbook for event-listener target"
```

### Task 6.2: Update `.env.example` and `docker-compose.yml`

**Files:**
- Modify: `.env.example`
- Modify: `docker-compose.yml`

- [ ] **Step 1: Add new env-var blocks to `.env.example`**

After the existing `ZITADEL_ACTION_SIGNING_KEY_ROTATION_THRESHOLD_DAYS` block (around line 97), append:

```
# --- Zitadel Actions v2 event-listener signing key (uncomment when Action target is live) ---
# Returned ONCE by Zitadel when the syndra-event-listener Action target is created
# (see zitadel/actions/register.sh). Captured to .action-signing-key.syndra-event-listener
# and the .action-env.fragment apply file.
# When unset, the /api/webhooks/zitadel endpoint runs in dev mode (signature verification
# disabled, warning logged). When set, every event POST MUST carry a valid ZITADEL-Signature
# header or receive 401.
# ZITADEL_EVENT_SIGNING_KEY=the-hex-key-zitadel-returned-at-event-target-creation
# ZITADEL_EVENT_SIGNING_KEY_ROTATED_AT=2026-04-24T12:34:56Z
# ZITADEL_EVENT_SIGNING_KEY_ROTATION_THRESHOLD_DAYS=90

# --- Self-mutation loop guard ---
# Zitadel service-user ID that Syndra uses for M2M Management API calls. Events whose
# editorUserId matches are dropped at the webhook listener (otherwise backend-initiated
# grant mutations echo through Actions v2 and re-trigger orchestration). Find via the
# Zitadel event log after the first M2M call. Unset disables the guard (dev-mode only).
# ZITADEL_M2M_USER_ID=
```

- [ ] **Step 2: Add to `docker-compose.yml`**

After line 57 (`ZITADEL_ACTION_SIGNING_KEY_ROTATION_THRESHOLD_DAYS=...`):

```yaml
      - ZITADEL_EVENT_SIGNING_KEY=${ZITADEL_EVENT_SIGNING_KEY:-}
      - ZITADEL_EVENT_SIGNING_KEY_ROTATED_AT=${ZITADEL_EVENT_SIGNING_KEY_ROTATED_AT:-}
      - ZITADEL_EVENT_SIGNING_KEY_ROTATION_THRESHOLD_DAYS=${ZITADEL_EVENT_SIGNING_KEY_ROTATION_THRESHOLD_DAYS:-}
      - ZITADEL_M2M_USER_ID=${ZITADEL_M2M_USER_ID:-}
```

- [ ] **Step 3: Commit**

```bash
git add .env.example docker-compose.yml
git commit -m "config: add ZITADEL_EVENT_SIGNING_KEY and ZITADEL_M2M_USER_ID env vars"
```

### Task 6.3: Add smoke-test script

**Files:**
- Create: `scripts/smoke-test-event-listener.sh`

- [ ] **Step 1: Write the smoke test**

```bash
#!/usr/bin/env bash
# scripts/smoke-test-event-listener.sh — POST a synthetic Zitadel event to
# /api/webhooks/zitadel with a valid ZITADEL-Signature header and assert 200.
#
# When ZITADEL_EVENT_SIGNING_KEY is set, signs the body. When unset, posts
# unsigned (dev-mode pass-through).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Inline .env loader (same pattern as register.sh)
_ENV_FILE="${REPO_ROOT}/.env"
if [[ -f "$_ENV_FILE" ]]; then
  while IFS= read -r _raw || [[ -n "$_raw" ]]; do
    [[ "$_raw" =~ ^[[:space:]]*($|#) ]] && continue
    [[ "$_raw" =~ ^[[:space:]]*([A-Za-z_][A-Za-z0-9_]*)=(.*)$ ]] || continue
    _k="${BASH_REMATCH[1]}"; _v="${BASH_REMATCH[2]}"
    if [[ "$_v" =~ ^\"(.*)\"$ ]] || [[ "$_v" =~ ^\'(.*)\'$ ]]; then _v="${BASH_REMATCH[1]}"; fi
    [[ -z "${!_k+x}" ]] && export "$_k=$_v"
  done < "$_ENV_FILE"
fi

BACKEND_URL="${BACKEND_URL:-http://localhost:8080}"
NOW="$(date +%s)"
BODY='{"aggregate":{"id":"smoke-user-1","type":"user","resourceOwner":"org-1"},"event":"user.human.added","editorUserId":"smoke-operator","payload":{}}'

if [[ -n "${ZITADEL_EVENT_SIGNING_KEY:-}" ]]; then
  # ZITADEL-Signature: t=<unix>,v1=<hex>; HMAC-SHA256(<unix>.<body>)
  SIG_INPUT="${NOW}.${BODY}"
  SIG="$(printf '%s' "$SIG_INPUT" | openssl dgst -sha256 -hmac "$ZITADEL_EVENT_SIGNING_KEY" -hex | awk '{print $NF}')"
  HEADER="ZITADEL-Signature: t=${NOW},v1=${SIG}"
else
  echo "warning: ZITADEL_EVENT_SIGNING_KEY unset — posting unsigned" >&2
  HEADER="X-Smoke: unsigned"
fi

STATUS="$(curl -sS -o /tmp/smoke-event.out -w '%{http_code}' \
  -X POST "${BACKEND_URL}/api/webhooks/zitadel" \
  -H 'Content-Type: application/json' \
  -H "$HEADER" \
  -d "$BODY")"

cat /tmp/smoke-event.out; echo
if [[ "$STATUS" != "200" ]]; then
  echo "FAIL: expected 200, got $STATUS" >&2
  exit 1
fi
echo "OK"
```

- [ ] **Step 2: Make executable + syntax check**

Run: `chmod +x scripts/smoke-test-event-listener.sh && bash -n scripts/smoke-test-event-listener.sh`
Expected: clean.

- [ ] **Step 3: Commit**

```bash
git add scripts/smoke-test-event-listener.sh
git commit -m "test(zitadel): smoke test for event-listener endpoint"
```

---

## Phase 7 — Verification + finalization

### Task 7.1: Run the full test matrix

**Files:** none (verification only).

- [ ] **Step 1: Backend tests**

Run: `cd backend && go test ./... && go vet ./...`
Expected: all PASS, no warnings. Test count should be ≥ baseline + 8 (translator subtests + integration tests + role_keys test + signature middleware test + self-loop test).

- [ ] **Step 2: Sync tests**

Run: `cd sync && go test ./... && go vet ./...`
Expected: PASS, no warnings (sync should be unaffected by these changes).

- [ ] **Step 3: UI**

Run: `cd ui && bun run lint && bun run test && bun run build`
Expected: PASS (UI should be unaffected; this is a regression check).

- [ ] **Step 4: Bash syntax**

Run: `bash -n zitadel/actions/register.sh zitadel/actions/rotate.sh scripts/smoke-test-action-v2.sh scripts/smoke-test-event-listener.sh`
Expected: no output.

- [ ] **Step 5: Manifest validity**

Run: `jq -e . zitadel/actions/targets.json >/dev/null && echo OK`
Expected: `OK`.

(No commit — verification gate. If any step fails, fix in a new commit before proceeding to 7.2.)

### Task 7.2: Refresh codebase-memory + write IMPLEMENTATION.md + tick OpenSpec tasks

**Files:**
- Create: `openspec/changes/zitadel-event-trigger-propagation/IMPLEMENTATION.md`
- Modify: `openspec/changes/zitadel-event-trigger-propagation/tasks.md` (tick all)
- Modify: `openspec/INDEX.md` (flip Status from "In Progress" to "Complete")
- Modify: `openspec/changes/syndra-core-architecture/specs/feature-coverage.md` (`webhook-invalidation` row producer status)
- Modify: `openspec/changes/syndra-core-architecture/ROADMAP.md` (tick item)

- [ ] **Step 1: Write IMPLEMENTATION.md**

```markdown
# Implementation Record: Zitadel Event-Trigger Propagation

> **Status:** Complete | [Proposal](proposal.md) | [Design](design.md) | [Tasks](tasks.md)

## What landed

### Backend
- `WebhookPayload.RoleKeys []string` added; `processGrantAdded` and `processGrantRemoved` iterate roles per delivery.
- `/api/webhooks/zitadel` route now wraps `withZitadelActionSignature("ZITADEL_EVENT_SIGNING_KEY", ...)`.
- Legacy `verifyWebhookSignature`, `verifyWebhookFreshness`, and `ZITADEL_WEBHOOK_SECRET` removed.
- New `webhook_translate.go`: `translateZitadelEvent(body) (WebhookPayload, ok, err)` + per-event mapping table covering `user.human.{added,selfregistered,deactivated,locked}` and `user.grant.{added,changed,removed}` (with the `user.user.grant.*` legacy-prefix variants).
- Self-mutation loop guard via `ZITADEL_M2M_USER_ID`.

### Deployment
- `zitadel/actions/targets.json` reshaped to `targets[]` + `executions[]` with named `target` references.
- `register.sh` iterates targets[] and binds executions to the right target IDs in one pass; per-target signing-key capture into `.action-signing-key.<name>` files; multi-line `.action-env.fragment` for one-shot env apply.
- `rotate.sh --target NAME` rotates a single target; default rotates all.
- `make zitadel-actions-register` registers BOTH targets in one command.

### Tests
- 8 new tests: 1 `RoleKeys` plural dispatch, 1 route-middleware 401, 1 translator shape detection, 1 internal-shape passthrough, 8 mapping subtests, 1 self-mutation guard, 2 integration (zitadel-shape full roundtrip + drop-self-mutation).
- 12 legacy verifier tests removed (covered by `zitadel_action_auth_test.go`).

### Docs
- `zitadel/actions/EVENTS.md` (operator runbook).
- `.env.example` + `docker-compose.yml` updated.
- `application-claims/spec.md` event-trigger subsection added.
- New `lifecycle-event-propagation/spec.md` capability spec.

## Verification performed

- `cd backend && go test ./... && go vet ./...` — PASS, count up by 8.
- `cd sync && go test ./...` — PASS.
- `cd ui && bun run lint && bun run test` — PASS.
- `bash -n` on all scripts — clean.
- `jq -e .` on `targets.json` — valid.
- (Pending live verification): `make zitadel-actions-register` against staging Zitadel.

## Gaps carried forward

- **Live staging smoke**: deferred until operator has Zitadel staging creds. `make zitadel-actions-verify-events` is ready.
- **Event-name verification**: the Zitadel event identifiers in `targets.json` cover the modern `user.human.*` / `user.grant.*` names. Translator tolerates `user.user.grant.*` (older prefix) as well. If the staging instance uses other names, update both the executions and the translator's switch.
- **Welcome-bundle UX**: bundle `is_welcome` flag is settable via DB but no UI yet. Tracked under `automation-policies` (deferred to P5).
```

- [ ] **Step 2: Tick all tasks in `openspec/changes/zitadel-event-trigger-propagation/tasks.md`**

Edit the file and replace every `- [ ]` with `- [x]`.

- [ ] **Step 3: Update `INDEX.md`**

Find the row `| [Zitadel Event-Trigger Propagation]... | 5 | In Progress |` and change `In Progress` to `Complete`. Append `/ [impl](changes/zitadel-event-trigger-propagation/IMPLEMENTATION.md)` to the links column.

- [ ] **Step 4: Update `feature-coverage.md`**

Find the `Webhook invalidation` row. In the Notes column, append a sentence: `Producer wired via Actions v2 event-trigger executions in zitadel-event-trigger-propagation (2026-05-XX).`

Bump the `Last updated` date at the top of the file.

- [ ] **Step 5: Update ROADMAP.md**

Find Phase 5 > Operations and add `- [x] zitadel-event-trigger-propagation: lifecycle event delivery via Actions v2 event triggers + welcome-bundle on user creation`.

- [ ] **Step 6: Refresh codebase-memory graph**

This step uses the codebase-memory MCP tool, not bash. From within the Claude Code session executing the plan:

```
mcp__codebase-memory-mcp__detect_changes(
  project="<repo-project>-backend",
  base_branch="main"
)
```

Then re-index if changes are detected:

```
mcp__codebase-memory-mcp__index_repository(
  repo_path="<repo>/backend",
  mode="moderate"
)
```

Expected: graph reflects new `webhook_translate.go` symbols (`translateZitadelEvent`, `translateEventName`, `mapGrantEvent`, `errSelfMutation`, `zitadelEventPayload`, `userGrantPayload`).

- [ ] **Step 7: Commit**

```bash
git add openspec/ docs/superpowers/plans/
git commit -m "openspec: archive zitadel-event-trigger-propagation as complete"
```

---

## Self-Review (run after writing)

**Spec coverage**

- [x] Event-target creation in `make zitadel-actions-register` — Phase 5 (Tasks 5.1, 5.2, 5.4).
- [x] Welcome-bundle assignment confirmation — flagged as already wired in Architecture and re-asserted in spec scenario "Default bundle on first user creation".
- [x] Both keys captured in one apply file — Task 5.2 step 1 (`.action-env.fragment` writer).
- [x] Self-mutation loop guard — Tasks 4.1, 4.3 (test + integration).
- [x] Legacy `ZITADEL_WEBHOOK_SECRET` retirement — Task 3.2.
- [x] Multi-role grants — Task 2.1 (`RoleKeys` plural).
- [x] Operator-facing docs — Tasks 6.1, 6.2.
- [x] OpenSpec scaffolding — Phase 1.

**Placeholder scan**

- No `TBD`, `TODO`, `implement later`, `add error handling`, "similar to Task N", "fill in details".
- Two soft references that are intentionally left to verification (not placeholders): (a) the exact Zitadel event-name strings (Task 5.1 + design D8 explicitly call out empirical verification); (b) `dbInsertWebhookEventStub` / `cacheRebuildStub` / `webhookEmitProvisioningIntentStub` test helpers (Task 2.1 step 1) — written assuming the established `deps.go` injectable-pattern. If they don't exist, the implementing engineer creates them following that pattern, which is documented in `live-webhook-listener/proposal.md` §"Injectable Dependencies".

**Type consistency**

- `WebhookPayload.RoleKeys []string` defined in Task 2.1; used identically in 2.1, 4.1 (`mapGrantEvent`), 4.3.
- `translateZitadelEvent(body []byte) (WebhookPayload, bool, error)` — same signature in Tasks 4.1, 4.2, 4.3.
- `errSelfMutation` — defined in 4.1, referenced by name in 4.2 and 4.3.
- `withZitadelActionSignature(secretEnvVar string, next http.HandlerFunc)` — verified against actual definition in `zitadel_action_auth.go:50` before plan write.
- Manifest `_signing_key_env` annotation — defined in 5.1, consumed in 5.2.

---

## Execution Handoff

**Plan complete and saved to `docs/superpowers/plans/2026-05-01-zitadel-event-trigger-propagation.md`. Two execution options:**

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration.

**2. Inline Execution** — Execute tasks in this session using `superpowers:executing-plans`, batch execution with checkpoints.

**Which approach?**
