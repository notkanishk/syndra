# Signing Key Handling

Zitadel returns the Action target **signing key exactly once**, in the JSON
response body of the `CreateTarget` call (`POST /v2/actions/targets`). There
is no read API to retrieve it again afterward. Lose it and you must either
recreate the target or rotate the key in place (see below).

## Zitadel does not expire the signing key

Verified against `proto/zitadel/action/v2/target.proto`: `CreateTargetRequest`
has no expiration field, and `UpdateTargetRequest.expiration_signing_key` is a
rotation *trigger*, not a time-to-live. The current Zitadel implementation
only accepts `"0s"` (immediate hard swap); longer graceful-signing periods
are a stated roadmap item but not yet live.

**Practical consequence:** the first signing key is valid forever unless you
explicitly rotate it. Zitadel will never prompt you to rotate, never issue a
"key expires in N days" warning, and never auto-rotate on its own.

**So why rotate at all?** Because it's a MkAuth policy choice, not a Zitadel
requirement:

- **Incident response** — credential suspected leaked, staff off-boarding,
  host compromise, or a strict "rotate after any touch" policy.
- **Compliance** — frameworks like SOC 2 / ISO 27001 often mandate rotating
  shared credentials on a cadence (commonly 90 days).
- **Defense in depth** — cap the blast radius of an undetected leak.

None of these apply to Zitadel; they're operator policy. MkAuth ships the
rotate command below but deliberately does **not** run it on a schedule —
rotation frequency is a deployment-level decision, not a runtime one. If/when
Zitadel enables longer graceful periods on `expiration_signing_key`, a
backend dual-key acceptance path becomes worth building; until then,
scheduled automation buys little and adds infrastructure.

## Lifecycle

1. `zitadel/actions/register.sh` creates the target, extracts the `signingKey`
   field from the response, and writes it to `zitadel/actions/.action-signing-key`
   with mode `0600`.
2. The operator copies that value into the backend environment as
   `ZITADEL_ACTION_SIGNING_KEY` (via `.env`, systemd drop-in, or secret
   manager) and restarts the backend.
3. From that point on, every `POST /api/action/inject` request must carry a
   valid `ZITADEL-Signature` header or receive a `401 INVALID_SIGNATURE`.

All three scripts (`register.sh`, `rotate.sh`,
`scripts/smoke-test-action-v2.sh`) automatically load `.env` from the
repo root when present, so values filled into `.env` are available to the
scripts and `make` targets without a prior `source`/`set -a` step. Vars
explicitly set in the shell (`ZITADEL_DOMAIN=other.com make …`) always
win over `.env` — the loader only exports keys that aren't already set.

## Rotation (in place, no target recreate)

Use the shipped command:

```bash
make zitadel-actions-rotate-key
```

Under the hood this runs `zitadel/actions/rotate.sh`, which:

1. Resolves an M2M token (same `ZITADEL_M2M_TOKEN` / `ZITADEL_MACHINE_KEY_PATH`
   paths as `register.sh`).
2. Looks up the target ID by name via `POST /v2/actions/targets/search`.
3. Calls `POST /v2/actions/targets/{id}` with body `{"expirationSigningKey":"0s"}`.
4. Backs up the current `.action-signing-key` to `.action-signing-key.previous`
   (mode 0600) and overwrites `.action-signing-key` with the new key.
5. Captures the rotation timestamp to `.action-signing-key.rotated_at` (RFC3339 UTC).
6. Writes a ready-to-append env fragment at `zitadel/actions/.action-env.fragment`
   (mode 0600) containing both `ZITADEL_ACTION_SIGNING_KEY=…` and
   `ZITADEL_ACTION_SIGNING_KEY_ROTATED_AT=…` lines. The operator applies it
   with a single redirect — no copy-paste from the terminal — which
   eliminates any risk of line-wrap, unevaluated shell substitution, or
   the outside chance of someone copying from the script source instead of
   its output. The timestamp feeds the Rotation Status panel on `/zitadel`.

Apply the fragment with one of:

```bash
cat zitadel/actions/.action-env.fragment >> .env
# OR, for systemd EnvironmentFile deploys:
sudo install -m 0600 zitadel/actions/.action-env.fragment /etc/mkauth/action-env
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

Because MkAuth only accepts signatures that match the currently configured
`ZITADEL_ACTION_SIGNING_KEY`, there is a brief window between Zitadel
accepting the new key on outbound Action calls and MkAuth being restarted
with the new value. During that window, incoming Action requests fail
signature verification, MkAuth returns `401`, and Zitadel proceeds to issue
the token with stock claims (because `restCall.interruptOnError: false`).
Users are never blocked; custom claims simply disappear for the gap. Keep it
under a minute.

`.action-signing-key.previous` is retained for audit / operator rollback but
is **not read by the backend at runtime** — MkAuth trusts a single env var.
If you need to roll back, copy the previous value into `ZITADEL_ACTION_SIGNING_KEY`
and restart.

## Rotation observability (the Status panel)

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

## Storage

- **Never** commit `.action-signing-key` or `.action-signing-key.previous` to
  git. `zitadel/actions/.gitignore` excludes `.action-signing-key`; the
  previous-key file inherits that exclusion via pattern match in
  deployed checkouts (confirm your `.gitignore` covers both).
- Treat both as production credentials: bind-mount from host secret storage
  (LXC-bound volume or sops-encrypted file) rather than baking into images.
- For the PoC deployment on Proxmox LXC, storing on the host at
  `/etc/mkauth/.action-signing-key` (mode 0600, root:mkauth) is sufficient.

## Full target deletion (DELETE endpoint)

If you need to retire the target entirely (not just unbind executions):

```bash
curl -fsS -X DELETE "https://${ZITADEL_DOMAIN}/v2/actions/targets/${TARGET_ID}" \
  -H "Authorization: Bearer ${TOKEN}"
```

Zitadel auto-removes the target from every execution binding as part of the
delete. To retain the target and only unbind, use `register.sh --remove`,
which issues `PUT /v2/actions/executions` with an empty `targets` array per
bound condition.
