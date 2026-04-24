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

## Rollback

`register.sh --remove` deletes the executions (unbinds triggers). Because the
target is configured with `interruptOnError: false`, token issuance continues
with stock Zitadel claims during and after rollback — no user-facing outage.

## Local development

When `ZITADEL_ACTION_SIGNING_KEY` is unset, the middleware logs a warning and
passes requests through without verification. This matches the dev-mode
fall-through in `withUserAuth` (no `ZITADEL_DOMAIN` set). Never rely on this
in any environment that accepts traffic from a real Zitadel instance.
