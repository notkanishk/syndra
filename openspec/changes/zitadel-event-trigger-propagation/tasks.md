# Tasks

## 1. Backend: data model + auth
- [x] 1.1 Add `RoleKeys []string` to `WebhookPayload`; update validation
- [x] 1.2 Iterate `RoleKeys` in `processGrantAdded` / `processGrantRemoved`
- [x] 1.3 Switch `/api/webhooks/zitadel` route to `withZitadelActionSignature("ZITADEL_EVENT_SIGNING_KEY", ...)`
- [x] 1.4 Remove `verifyWebhookSignature`, `verifyWebhookFreshness`, inline calls in `HandleZitadelWebhook`
- [x] 1.5 Remove `ZITADEL_WEBHOOK_SECRET` from `.env.example` and `docker-compose.yml`

## 2. Backend: payload translator
- [x] 2.1 Define `ZitadelEventPayload` lenient struct in `webhook_translate.go`
- [x] 2.2 Map `user.human.added` → `user_created`
- [x] 2.3 Map `user.human.deactivated` → `user_deactivated`
- [x] 2.4 Map `user.human.locked` → `user_locked`
- [x] 2.5 Map `user.grant.added` → `grant_added` with `role_keys[]`
- [x] 2.6 Map `user.grant.changed` → `grant_changed` with `role_keys[]`
- [x] 2.7 Map `user.grant.removed` → `grant_removed` with `role_keys[]`
- [x] 2.8 Unknown-event passthrough (200 + log, no dispatch)
- [x] 2.9 Wire shape detection into `HandleZitadelWebhook`

## 3. Backend: self-mutation loop guard
- [x] 3.1 Add `ZITADEL_M2M_USER_ID` env reader
- [x] 3.2 Drop events at translator when `editorUserId` matches; log + 200

## 4. Deployment: multi-target manifest
- [x] 4.1 Reshape `targets.json` to `targets[]` + `executions[]` with named `target` reference
- [x] 4.2 Add `mkauth-event-listener` target (`restAsync`, 5s timeout)
- [x] 4.3 Add 6 `condition.event` executions for the lifecycle event names

## 5. Deployment: register.sh + rotate.sh
- [x] 5.1 Iterate `targets[]` in `register.sh`; per-target signing-key capture
- [x] 5.2 Map `executions[].target` (name) → captured target IDs; bind in one pass
- [x] 5.3 Update `--remove` to unbind every execution and warn about per-target retention
- [x] 5.4 Update `rotate.sh` to accept a `--target NAME` flag (default both)
- [x] 5.5 Update `.action-env.fragment` writer to emit both env-var lines

## 6. Ops + docs
- [x] 6.1 Add `zitadel/actions/EVENTS.md` (operator runbook)
- [x] 6.2 Update `.env.example` (add `ZITADEL_EVENT_SIGNING_KEY`, `ZITADEL_M2M_USER_ID`; remove `ZITADEL_WEBHOOK_SECRET`)
- [x] 6.3 Update `docker-compose.yml` (same env-var changes)
- [x] 6.4 Update `SIGNING_KEY.md` for the dual-target flow (no `DEPLOY.md` in repo)
- [x] 6.5 Add `scripts/smoke-test-event-listener.sh` + restore `make zitadel-actions-verify-events`

## 7. OpenSpec finalization
- [x] 7.1 MODIFIED delta on `application-claims/spec.md` (event-trigger subsection)
- [x] 7.2 New capability spec `lifecycle-event-propagation/spec.md`
- [x] 7.3 Update `feature-coverage.md` (`webhook-invalidation` row producer column)
- [x] 7.4 Update `INDEX.md` change log
- [x] 7.5 Write `IMPLEMENTATION.md`
- [x] 7.6 Refresh codebase-memory graph via `mcp__codebase-memory-mcp__detect_changes`
