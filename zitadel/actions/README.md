# Zitadel Actions v2 Artifacts

This directory holds the deployment assets for Syndra's Zitadel Actions v2
claim-injection Action — the production claim-shaping boundary defined in
[application-claims/spec.md](../../openspec/changes/syndra-core-architecture/specs/application-claims/spec.md).

## Why there is no JavaScript file here

Zitadel has two generations of Actions. **v1** embedded JavaScript that ran
inside the Zitadel process; that's what the legacy `SetCustomClaims` / `claims`
namespace APIs referred to. **v2** — the only generation Syndra supports —
does not have an embedded runtime. Instead, Zitadel POSTs the function trigger
payload to an HTTP target of your choice, and your handler returns a response
envelope (`append_claims`, `append_log_claims`, `set_user_metadata`).

So the "script" for Syndra's v2 Action is a combination of:

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
| `EVENTS.md` | Event-listener target reference: subscribed events, self-mutation guard, troubleshooting. |

The one-time signing-key lifecycle and the instance-scoped service-account
permissions the scripts require are documented in the
[Service-Account Permissions](#service-account-permissions) and
[Signing Key Handling](#signing-key-handling) sections below (answers, among
other things, "why does `HTTP 403` happen with `ORG_OWNER`?").

## Quick start

```bash
# 1. Ensure backend M2M creds are configured (standard Phase 3 prereq).
export ZITADEL_DOMAIN=your-instance.zitadel.cloud
export ZITADEL_MACHINE_KEY_PATH=./zitadel-machine-key.json
export SYNDRA_EXTERNAL_URL=https://syndra.internal  # where Zitadel should POST

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

---

## Service-Account Permissions

Canonical operator reference for the Zitadel permissions the Actions v2
scripts (`zitadel/actions/register.sh`, `zitadel/actions/rotate.sh`)
require.

### Why org-level roles don't work

Actions v2 target management lives at the **instance scope** in Zitadel.
The org-level roles recommended for the backend's normal user/grant CRUD
(`ORG_OWNER` or `ORG_USER_MANAGER` + `ORG_PROJECT_PERMISSION_EDITOR`,
per `.env.example`) do NOT cover it — a fresh
`make zitadel-actions-register` run on an org-only service user fails
with `HTTP 403`.

### Minimum permissions the scripts require

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

### Assignment — narrowest first

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

### Duration

The service user only needs these permissions during:

- `make zitadel-actions-register` — one time on install, plus re-runs if
  the endpoint or timeout changes.
- `make zitadel-actions-rotate-key` — on your rotation cadence (incident
  response or compliance policy; Zitadel does not expire the key on its
  own — see [Signing Key Handling → Zitadel does not expire the signing
  key](#zitadel-does-not-expire-the-signing-key)).

In steady state — Zitadel calling Syndra's `/api/action/inject` — no
outbound Actions admin call is made, so the role can be:

- **Kept permanently assigned** (pragmatic, least operator toil).
- **Assigned and revoked per run** (defensive; rotate happens rarely
  enough for this to be tractable).
- **Scoped to a separate M2M key** distinct from the backend's runtime
  key (strictest; isolates blast radius if the backend's everyday key
  leaks).

### The backend's own M2M key

The backend container reads `ZITADEL_MACHINE_KEY_PATH` to mint tokens
for runtime user/grant/role CRUD through `backend/internal/zitadel/client.go`.
That key does **not** need any action permissions — its org-level
assignment per `.env.example` stays correct. If you want strict
separation, use two machine users: one for the backend container, one
for the operator-run scripts (both read `ZITADEL_MACHINE_KEY_PATH`, but
from different paths depending on which context is running).

### If you still get HTTP 403

The `zitadel_api` helper in `register.sh` / `rotate.sh` now surfaces
Zitadel's own error body on every non-2xx response. Re-run the failing
command and read the JSON — the `message` field will name the exact
missing permission (e.g. `permission denied: action.target.write`), so
you can confirm which specific grant to add without guessing.

---

## Signing Key Handling

Zitadel returns each Action target's **signing key exactly once**, in the JSON
response body of the `CreateTarget` call (`POST /v2/actions/targets`). There
is no read API to retrieve it again afterward. Lose it and you must either
recreate the target or rotate the key in place (see below).

### Two targets, two keys

Syndra's deployment registers two Actions v2 targets:

| Target name | Type | Triggers | Backend env var |
|---|---|---|---|
| `syndra-claim-injector` | `restCall` | `function.preaccesstoken`, `function.preuserinfo` | `ZITADEL_ACTION_SIGNING_KEY` |
| `syndra-event-listener` | `restAsync` | `condition.event` (user/grant lifecycle) | `ZITADEL_EVENT_SIGNING_KEY` |

The two keys are independent — rotation, leak-response, and storage all
happen per target. Both follow the same lifecycle described below; substitute
the appropriate target name and env var.

### Zitadel does not expire the signing key

Verified against `proto/zitadel/action/v2/target.proto`: `CreateTargetRequest`
has no expiration field, and `UpdateTargetRequest.expiration_signing_key` is a
rotation *trigger*, not a time-to-live. The current Zitadel implementation
only accepts `"0s"` (immediate hard swap); longer graceful-signing periods
are a stated roadmap item but not yet live.

**Practical consequence:** the first signing key is valid forever unless you
explicitly rotate it. Zitadel will never prompt you to rotate, never issue a
"key expires in N days" warning, and never auto-rotate on its own.

**So why rotate at all?** Because it's a Syndra policy choice, not a Zitadel
requirement:

- **Incident response** — credential suspected leaked, staff off-boarding,
  host compromise, or a strict "rotate after any touch" policy.
- **Compliance** — frameworks like SOC 2 / ISO 27001 often mandate rotating
  shared credentials on a cadence (commonly 90 days).
- **Defense in depth** — cap the blast radius of an undetected leak.

None of these apply to Zitadel; they're operator policy. Syndra ships the
rotate command below but deliberately does **not** run it on a schedule —
rotation frequency is a deployment-level decision, not a runtime one. If/when
Zitadel enables longer graceful periods on `expiration_signing_key`, a
backend dual-key acceptance path becomes worth building; until then,
scheduled automation buys little and adds infrastructure.

### Lifecycle

1. `zitadel/actions/register.sh` creates each target, extracts the `signingKey`
   field from the response, and writes it to
   `zitadel/actions/.action-signing-key.<target-name>` with mode `0600` — one
   file per target (`syndra-claim-injector`, `syndra-event-listener`).
2. The same run appends `ZITADEL_ACTION_SIGNING_KEY=...` and
   `ZITADEL_EVENT_SIGNING_KEY=...` (plus the `_ROTATED_AT` companions) to
   `zitadel/actions/.action-env.fragment`. The operator applies it to backend
   env (via `.env`, systemd drop-in, or secret manager) in one shot and
   restarts the backend.
3. From that point on, every `POST /api/action/inject` request must carry a
   valid `ZITADEL-Signature` header or receive a `401 INVALID_SIGNATURE` —
   and the same goes for `/api/webhooks/zitadel` against the event signing key.

All three scripts (`register.sh`, `rotate.sh`,
`scripts/smoke-test-action-v2.sh`) automatically load `.env` from the
repo root when present, so values filled into `.env` are available to the
scripts and `make` targets without a prior `source`/`set -a` step. Vars
explicitly set in the shell (`ZITADEL_DOMAIN=other.com make …`) always
win over `.env` — the loader only exports keys that aren't already set.

### Rotation (in place, no target recreate)

Use the shipped command:

```bash
make zitadel-actions-rotate-key                            # rotate every target
make zitadel-actions-rotate-key TARGET=syndra-event-listener  # rotate one
```

Under the hood this runs `zitadel/actions/rotate.sh`, which:

1. Resolves an M2M token (same `ZITADEL_M2M_TOKEN` / `ZITADEL_MACHINE_KEY_PATH`
   paths as `register.sh`).
2. Iterates `targets[]` from `targets.json` (or filters to the one named via
   `--target`), looking up each target ID by name via
   `POST /v2/actions/targets/search`.
3. For every target: calls `POST /v2/actions/targets/{id}` with body
   `{"expirationSigningKey":"0s"}`.
4. Backs up the previous `.action-signing-key.<target>` to
   `.action-signing-key.<target>.previous` (mode 0600) and overwrites
   `.action-signing-key.<target>` with the new key.
5. Captures the per-target rotation timestamp to
   `.action-signing-key.<target>.rotated_at` (RFC3339 UTC).
6. Writes a ready-to-append env fragment at `zitadel/actions/.action-env.fragment`
   (mode 0600) containing the appropriate `<env>=…` and `<env>_ROTATED_AT=…`
   lines for each rotated target (`ZITADEL_ACTION_SIGNING_KEY` for the claim
   injector, `ZITADEL_EVENT_SIGNING_KEY` for the event listener). The operator
   applies it with a single redirect — no copy-paste from the terminal — which
   eliminates any risk of line-wrap, unevaluated shell substitution, or the
   outside chance of someone copying from the script source instead of its
   output. **Panel observability scope:** the `/zitadel` Rotation Status panel
   currently only reflects `ZITADEL_ACTION_SIGNING_KEY_ROTATED_AT` (the
   claim-injector key). `ZITADEL_EVENT_SIGNING_KEY_ROTATED_AT` is captured to
   the fragment and forwarded through `docker-compose.yml` for forward
   compatibility, but no backend endpoint reads it yet. Track event-listener
   key age out-of-band (e.g. via the per-target `.rotated_at` file) until the
   panel is extended to report both targets.

Apply the fragment with one of:

```bash
cat zitadel/actions/.action-env.fragment >> .env
# OR, for systemd EnvironmentFile deploys:
sudo install -m 0600 zitadel/actions/.action-env.fragment /etc/syndra/action-env
```

Then restart the backend and delete the fragment (`rm zitadel/actions/.action-env.fragment`).

Raw `curl` equivalent (fallback for deep-dive debugging only):

```bash
curl -fsS -X POST "https://${ZITADEL_DOMAIN}/v2/actions/targets/${TARGET_ID}" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{"expirationSigningKey":"0s"}'
```

The response returns the new `signingKey`. Capture it immediately — it will
not be returned again.

Because Syndra only accepts signatures that match the currently configured
`ZITADEL_ACTION_SIGNING_KEY`, there is a brief window between Zitadel
accepting the new key on outbound Action calls and Syndra being restarted
with the new value. During that window, incoming Action requests fail
signature verification, Syndra returns `401`, and Zitadel proceeds to issue
the token with stock claims (because `restCall.interruptOnError: false`).
Users are never blocked; custom claims simply disappear for the gap. Keep it
under a minute.

`.action-signing-key.<target>.previous` is retained for audit / operator
rollback but is **not read by the backend at runtime** — Syndra trusts a
single env var per target. If you need to roll back, copy the previous value
into the matching env var (`ZITADEL_ACTION_SIGNING_KEY` or
`ZITADEL_EVENT_SIGNING_KEY`) and restart.

### Rotation observability (the Status panel)

The backend exposes `GET /api/v1/zitadel/action-rotation-status`
(gated by `withOperatorAuth`) which reports:

```json
{
  "last_rotated_at": "2026-04-24T12:34:56Z",
  "age_days": 12,
  "threshold_days": 90,
  "status": "ok",
  "rotate_command": "make zitadel-actions-rotate-key"
}
```

Status values (highest precedence first):

- `disabled` — `ZITADEL_ACTION_SIGNING_KEY` is unset. Signature verification
  is off; every inbound Action request is passing unchecked. Rotation age is
  meaningless in this state and the response reports `disabled` regardless
  of `ROTATED_AT`. **This is a production misconfiguration, not a missing
  metric** — fix it before trusting anything the panel says.
- `unknown` — key is installed, but `ZITADEL_ACTION_SIGNING_KEY_ROTATED_AT`
  is unset, malformed, or in the future (clock skew / typo). Future
  timestamps are explicitly not clamped to `ok` age-0 — that would suppress
  warn/stale indefinitely.
- `ok` — age < threshold.
- `warn` — threshold ≤ age < 2× threshold. Schedule a rotation.
- `stale` — age ≥ 2× threshold. Rotate soon.

The response also includes `key_installed: bool` so callers can distinguish
`disabled` from `unknown` programmatically.

The `/zitadel` admin page renders this as a **Rotation Status** card showing
the badge, last-rotated timestamp, age, threshold, and a read-only copyable
`make zitadel-actions-rotate-key` snippet. The panel does **not** have a
"rotate now" button — rotation is a cryptographic mutation whose failure
modes (Zitadel accepts the new key but the backend still serves the old
one) are easier to reason about when the operator runs the command from
their own terminal with full context.

Configure via two env vars on the backend:

- `ZITADEL_ACTION_SIGNING_KEY_ROTATED_AT` — RFC3339 UTC. After each rotation
  `rotate.sh` writes this (alongside the new signing key) to
  `zitadel/actions/.action-env.fragment`; apply it with
  `cat zitadel/actions/.action-env.fragment >> .env` and restart the backend.
  On a fresh install, seed it manually to `date -u +%Y-%m-%dT%H:%M:%SZ`.
  Unset or unparseable → status `unknown`.
- `ZITADEL_ACTION_SIGNING_KEY_ROTATION_THRESHOLD_DAYS` — warn threshold in
  days. Default `90` (common compliance cadence). Non-positive or
  non-numeric values fall back to the default.

### Storage

- **Never** commit `.action-signing-key.*` or `.action-env.fragment` to git.
  `zitadel/actions/.gitignore` excludes the per-target key files (current
  and previous) plus the env fragment.
- Treat all signing-key files as production credentials: bind-mount from host
  secret storage (LXC-bound volume or sops-encrypted file) rather than baking
  into images.
- For the PoC deployment on Proxmox LXC, storing on the host at
  `/etc/syndra/.action-signing-key.<target>` (mode 0600, root:syndra) is
  sufficient.

### Full target deletion (DELETE endpoint)

If you need to retire the target entirely (not just unbind executions):

```bash
curl -fsS -X DELETE "https://${ZITADEL_DOMAIN}/v2/actions/targets/${TARGET_ID}" \
  -H "Authorization: Bearer ${TOKEN}"
```

Zitadel auto-removes the target from every execution binding as part of the
delete. To retain the target and only unbind, use `register.sh --remove`,
which issues `PUT /v2/actions/executions` with an empty `targets` array per
bound condition.
