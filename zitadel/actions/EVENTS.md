# Zitadel Event-Listener Target

The `mkauth-event-listener` Action target receives lifecycle events from Zitadel
and POSTs them to MkAuth's `/api/webhooks/zitadel` endpoint, driving:

- Welcome-bundle assignment on user creation (`user.human.added` →
  `processUserCreated` → `TriggerOnboarding` → `AssignBundleToUser`).
- Mapping-rule cascade on grant additions (`user.grant.added` →
  `EnforceMappingRules`).
- Mapping-rule revocation on grant removals (`user.grant.removed` →
  `RevokeMappingRules`).
- Cache invalidation on user deactivation/lock (`user.deactivated`,
  `user.locked` — note: deactivation and locking are user-aggregate events,
  not human-aggregate; Zitadel rejects `user.human.deactivated` /
  `user.human.locked` with `COMMAND-74aaqj8fv9` "Execution condition is
  invalid").

The target is type `restAsync` (fire-and-forget) — Zitadel does not block on
MkAuth latency. Companion target `mkauth-claim-injector` (type `restCall`,
function triggers) handles claim shaping; both are registered by a single
`make zitadel-actions-register` invocation.

## Subscribed events

| Zitadel event | MkAuth `event_type` | Downstream effect |
|---|---|---|
| `user.human.added` | `user_created` | Onboarding trigger → welcome bundle |
| `user.human.selfregistered` | `user_created` | Same |
| `user.deactivated` | `user_deactivated` | Cache invalidation |
| `user.locked` | `user_locked` | Cache invalidation |
| `user.grant.added` | `grant_added` (per role) | Cache rebuild + mapping rules + LLDAP intent |
| `user.grant.changed` | `grant_changed` (per role) | Same |
| `user.grant.removed` | `grant_removed` (per role) | Cache invalidation + revoke + LLDAP intent |

Unknown events are acknowledged with `200 OK` and logged but not dispatched.

## Self-mutation guard

When MkAuth's backend mutates Zitadel via the Management API (e.g.
`RemoveUserGrant` from mapping-rule revocation), Zitadel emits the
corresponding event back to the listener. Without filtering, you get an
infinite loop. The translator drops events whose `editorUserId` matches
`ZITADEL_M2M_USER_ID`.

To find the M2M service-user ID, after the first successful Management API
call check the Zitadel event log: any event with the resource you mutated
will show the editor user ID. Set it in `.env`:

```
ZITADEL_M2M_USER_ID=<the-id>
```

Unset disables the guard (acceptable in local-dev; required in any
environment that accepts real Zitadel traffic).

## Signing key

Captured at target creation by `register.sh` and written to
`zitadel/actions/.action-signing-key.mkauth-event-listener` (mode 0600).
Apply to backend env via the env-fragment writer:

```bash
cat zitadel/actions/.action-env.fragment >> .env
docker compose up -d backend
```

The fragment writer emits two pairs of lines — one for each target
(`ZITADEL_ACTION_SIGNING_KEY=...`, `ZITADEL_ACTION_SIGNING_KEY_ROTATED_AT=...`,
`ZITADEL_EVENT_SIGNING_KEY=...`, `ZITADEL_EVENT_SIGNING_KEY_ROTATED_AT=...`) —
so a single redirect updates both halves of the deployment.

Rotation:

```bash
make zitadel-actions-rotate-key TARGET=mkauth-event-listener
```

Same rotation flow as the claim injector — see `SIGNING_KEY.md`.

## Smoke test

```bash
make zitadel-actions-verify-events
```

Wraps `scripts/smoke-test-event-listener.sh`. POSTs a synthetic *unmapped*
event (`user.password.changed`) with a valid `ZITADEL-Signature` header (or
unsigned, in dev mode) and asserts `200 OK`. Using an unmapped event type
exercises authentication + shape detection + translator unknown-event
passthrough **without** invoking any downstream processor — safe to run
against staging and production without mutating onboarding or grant state.
Defaults to `BACKEND_URL=http://localhost:8080`; override with
`BACKEND_URL=https://staging.example.com make zitadel-actions-verify-events`
for remote checks.

## Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| Welcome bundle never assigned on new user | Listener target not registered, or signing key mismatch | `make zitadel-actions-register`; check `webhook_events` table for failed rows |
| Loop: same user grant mutated repeatedly | `ZITADEL_M2M_USER_ID` unset or wrong | Set the env var to the backend's M2M service-user ID |
| `401 INVALID_SIGNATURE` in backend logs | Stale or wrong `ZITADEL_EVENT_SIGNING_KEY` | Rotate via `make zitadel-actions-rotate-key TARGET=mkauth-event-listener` |
| Unknown-event log spam | Zitadel emitting events MkAuth doesn't subscribe to (unlikely if executions are pinned) | Check executions list in Zitadel console; remove unwanted bindings |
