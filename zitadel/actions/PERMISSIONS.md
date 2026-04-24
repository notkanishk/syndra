# Service-Account Permissions for Actions v2

Canonical operator reference for the Zitadel permissions the Actions v2
scripts (`zitadel/actions/register.sh`, `zitadel/actions/rotate.sh`)
require. Living under `zitadel/actions/` so the path stays stable across
the OpenSpec archive workflow — do not move this without updating the
pointers in `.env.example`, `DEPLOY.md`, and the `zitadel_api` helper.

## Why org-level roles don't work

Actions v2 target management lives at the **instance scope** in Zitadel.
The org-level roles recommended for the backend's normal user/grant CRUD
(`ORG_OWNER` or `ORG_USER_MANAGER` + `ORG_PROJECT_PERMISSION_EDITOR`,
per `.env.example`) do NOT cover it — a fresh
`make zitadel-actions-register` run on an org-only service user fails
with `HTTP 403`.

## Minimum permissions the scripts require

| Script call | Zitadel permission |
|---|---|
| `POST /v2/actions/targets/search` (register + rotate lookup) | `action.target.read` |
| `POST /v2/actions/targets` (first register) | `action.target.write` |
| `POST /v2/actions/targets/{id}` (re-register, key rotate) | `action.target.write` |
| `PUT /v2/actions/executions` (bind / unbind) | `action.execution.write` |
| `DELETE /v2/actions/targets/{id}` (full removal, rare) | `action.target.delete` |

Drop `action.target.delete` if you won't use the full-removal path —
`register.sh --remove` only unbinds executions, it does not delete the
target.

## Assignment — narrowest first

1. **Custom instance role** (recommended if your Zitadel version supports
   custom roles at **Default Settings → Roles**) — create a role with
   exactly the four permissions above and assign it to the service user
   at **Default Settings → Administrators**. Smallest blast radius.
2. **Prebuilt narrow role** — recent Zitadel versions ship action-scoped
   admin roles (names vary by version; look for `IAM_ACTION_ADMIN` or
   similar in your role catalog). If available, use it.
3. **`IAM_OWNER`** — the fallback. Works on every Zitadel version but
   grants control over everything instance-wide; only use when the
   narrower options aren't available on your version.

## Duration

The service user only needs these permissions during:

- `make zitadel-actions-register` — one time on install, plus re-runs if
  the endpoint or timeout changes.
- `make zitadel-actions-rotate-key` — on your rotation cadence (incident
  response or compliance policy; Zitadel does not expire the key on its
  own — see `SIGNING_KEY.md § Zitadel does not expire the signing key`).

In steady state — Zitadel calling MkAuth's `/api/action/inject` — no
outbound Actions admin call is made, so the role can be:

- **Kept permanently assigned** (pragmatic, least operator toil).
- **Assigned and revoked per run** (defensive; rotate happens rarely
  enough for this to be tractable).
- **Scoped to a separate M2M key** distinct from the backend's runtime
  key (strictest; isolates blast radius if the backend's everyday key
  leaks).

## The backend's own M2M key

The backend container reads `ZITADEL_MACHINE_KEY_PATH` to mint tokens
for runtime user/grant/role CRUD through `backend/internal/zitadel/client.go`.
That key does **not** need any action permissions — its org-level
assignment per `.env.example` stays correct. If you want strict
separation, use two machine users: one for the backend container, one
for the operator-run scripts (both read `ZITADEL_MACHINE_KEY_PATH`, but
from different paths depending on which context is running).

## If you still get HTTP 403

The `zitadel_api` helper in `register.sh` / `rotate.sh` now surfaces
Zitadel's own error body on every non-2xx response. Re-run the failing
command and read the JSON — the `message` field will name the exact
missing permission (e.g. `permission denied: action.target.write`), so
you can confirm which specific grant to add without guessing.
