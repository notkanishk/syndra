# Zitadel Actions v2 Artifacts

This directory holds the deployment assets for MkAuth's Zitadel Actions v2
claim-injection Action — the production claim-shaping boundary defined in
[application-claims/spec.md](../../openspec/changes/mkauth-core-architecture/specs/application-claims/spec.md).

## Why there is no JavaScript file here

Zitadel has two generations of Actions. **v1** embedded JavaScript that ran
inside the Zitadel process; that's what the legacy `SetCustomClaims` / `claims`
namespace APIs referred to. **v2** — the only generation MkAuth supports —
does not have an embedded runtime. Instead, Zitadel POSTs the function trigger
payload to an HTTP target of your choice, and your handler returns a response
envelope (`append_claims`, `append_log_claims`, `set_user_metadata`).

So the "script" for MkAuth's v2 Action is a combination of:

- `backend/internal/handlers/action.go` — the HTTP handler that receives the
  trigger payload and returns the claim envelope.
- `backend/internal/handlers/zitadel_action_auth.go` — HMAC-SHA256 middleware
  that verifies every request against the signing key Zitadel generated at
  target creation.
- `targets.json` (this directory) — declarative description of the target
  and the executions (function triggers) that bind it.
- `register.sh` (this directory) — idempotent script that applies the
  manifest against a live Zitadel instance via the v2 Actions API.

## Contents

| File | Purpose |
|------|---------|
| `targets.json` | Declarative target + execution config. Edit this to change endpoint, timeout, or trigger bindings. |
| `register.sh` | Apply `targets.json` against the configured Zitadel instance. Captures the target signing key on first run. |
| `rotate.sh` | One-shot in-place rotation of the target signing key; writes a ready-to-append env fragment. |
| `SIGNING_KEY.md` | How to handle the one-time signing key Zitadel returns. |
| `EVENTS.md` | Event-listener target reference: subscribed events, self-mutation guard, troubleshooting. |
| `PERMISSIONS.md` | Canonical reference for the instance-scoped Zitadel permissions the scripts require (answers "why does `HTTP 403` happen with ORG_OWNER?"). |

## Quick start

```bash
# 1. Ensure backend M2M creds are configured (standard Phase 3 prereq).
export ZITADEL_DOMAIN=your-instance.zitadel.cloud
export ZITADEL_MACHINE_KEY_PATH=./zitadel-machine-key.json
export MKAUTH_EXTERNAL_URL=https://mkauth.internal  # where Zitadel should POST

# 2. Register the target and capture the signing key.
make zitadel-actions-register

# 3. Inject the captured signing key into backend env and restart.
echo "ZITADEL_ACTION_SIGNING_KEY=$(cat .action-signing-key)" >> .env
docker compose up -d backend

# 4. Smoke-test against a real token issue.
make zitadel-actions-verify
```

## Rollback and teardown

Two non-destructive levels of removal. Pick the smallest one that fits the
situation — `--remove` leaves the door open for a clean re-bind without
re-issuing signing keys; `--purge` is for end-of-life or "I want this gone"
moments where you accept the cost of fresh keys + a backend env-swap.

### `register.sh --remove` — unbind executions only

What it does, top to bottom:

1. **Looks up each manifest target by name** via `POST /v2/actions/targets/search`
   (`target_name_filter.target_name = "<name>"`). Records the resolved IDs
   for use later, but does not create or update any target.
2. **Skips the create/update branch entirely** — targets are left exactly
   as-is (existing endpoint, timeout, signing key all intact in Zitadel).
3. **Walks `executions[]` from the manifest** and, for each execution,
   sends `PUT /v2/actions/executions` with `{condition, targets: []}`.
   Zitadel interprets the empty targets array as "remove all targets from
   this execution" — which, for a single-target binding, deletes the
   execution row. Each call is unbinded *by condition alone*, so it works
   even when the target ID is unknown (partially-deleted state).
4. **Tolerates HTTP 404** on that PUT (`COMMAND-74aaqj8fv9` "Execution
   condition is invalid") — Zitadel returns 404 when no execution row
   matches the condition, which is the desired post-state. So `--remove`
   is idempotent: safe to run twice, safe to run against an instance
   where bindings were never applied.
5. **Skips the env-fragment hint** at the end (no new keys to inject).

What it explicitly **does not** touch:

- `DELETE /v2/actions/targets/{id}` is never called. The targets stay
  registered.
- The local `.action-signing-key.<name>` files are kept on disk.
- The `ZITADEL_ACTION_SIGNING_KEY` / `ZITADEL_EVENT_SIGNING_KEY` env vars
  in `.env` are not touched.
- The backend doesn't need to restart. Because the targets are configured
  with `restCall.interruptOnError: false` (claim injector) and `restAsync`
  (event listener), token issuance and Zitadel's own command path
  continue uninterrupted with stock claims and no listener fan-out — no
  user-facing outage during or after rollback.

When you re-run `register.sh` after a `--remove`, it finds the existing
targets by name, takes the upsert branch (`POST /targets/{id}`), and
re-binds the executions with the *same* target IDs. The backend's
existing signing-key env vars still match because the keys were never
rotated. Net effect: rollback drill, then forward again, with zero
operator key-handling.

### `register.sh --purge` — full teardown

Runs the entire `--remove` flow, then adds a destructive cleanup pass:

6. **Deletes every manifest target** via `DELETE /v2/actions/targets/{id}`.
   Tolerates 404 for the same idempotency reason. Order matters — this
   runs *after* the unbind loop, because Zitadel refuses to delete a
   target that's still referenced by an execution.
7. **Removes the local key files**:
   `.action-signing-key.<name>`, `.action-signing-key.<name>.previous`,
   `.action-signing-key.<name>.rotated_at`, and `.action-env.fragment`.
8. **Prints the operator follow-up** — clear `ZITADEL_ACTION_SIGNING_KEY`
   and `ZITADEL_EVENT_SIGNING_KEY` from `.env` and restart the backend
   before re-registering, otherwise the backend's HMAC verification will
   reject every Zitadel-signed request (the new targets will mint fresh
   keys that don't match the stale env values).

### Side-by-side

| Step | `--remove` | `--purge` |
|------|------------|-----------|
| Resolve target IDs by name | Yes | Yes |
| Create/update targets | No | No |
| Unbind executions (`PUT /executions` w/ `targets:[]`) | Yes | Yes |
| Tolerate 404 on unbind | Yes | Yes |
| `DELETE /v2/actions/targets/{id}` | **No** | **Yes** |
| Delete local `.action-signing-key.*` files | **No** | **Yes** |
| Delete `.action-env.fragment` | **No** | **Yes** |
| Backend env action required after | None | Clear key env vars + restart |
| Reversible without operator key handling? | **Yes** (re-run `register.sh`) | **No** (fresh keys minted) |
| Safe to re-run idempotently | Yes | Yes |
| Use case | Rollback drill, temporary disable, retire bindings without losing key material | End-of-life teardown, leaving the Zitadel instance, rotating to a totally fresh key set |

### Make targets

```bash
make zitadel-actions-remove    # unbind executions; targets + keys retained
make zitadel-actions-purge     # also delete targets + local key files
```

## Local development

When `ZITADEL_ACTION_SIGNING_KEY` is unset, the middleware logs a warning and
passes requests through without verification. This matches the dev-mode
fall-through in `withUserAuth` (no `ZITADEL_DOMAIN` set). Never rely on this
in any environment that accepts traffic from a real Zitadel instance.
