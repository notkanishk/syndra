# Operator Deployment Guide: Zitadel Actions v2

> **Companion to:** [proposal.md](proposal.md) | [design.md](design.md) | [tasks.md](tasks.md) | [IMPLEMENTATION](IMPLEMENTATION.md)

This is the operator-run procedure for registering the MkAuth claim-injection
Action against a live Zitadel instance. Assumes Phase 3 prerequisites are
already in place (`ZITADEL_DOMAIN`, `ZITADEL_MACHINE_KEY_PATH` or a
pre-minted M2M token, backend reachable from Zitadel).

## Prerequisites

The following values must be available to the scripts — either in the
repo-root `.env` (the scripts auto-load it) or exported in your shell.
Explicit `export` wins over `.env`, so a one-off override like
`ZITADEL_DOMAIN=other.example.com make zitadel-actions-register` works
without editing `.env`.

* `ZITADEL_DOMAIN` — e.g. `auth.example.org`.
* Either `ZITADEL_M2M_TOKEN` (pre-minted Bearer) or `ZITADEL_MACHINE_KEY_PATH`
  (service-user key file). When only the key path is set, the scripts shell
  out to `backend/cmd/mkauth-token` (Go toolchain required on the host) to
  mint a fresh token via the JWT profile grant.
* `MKAUTH_EXTERNAL_URL` — the public URL Zitadel will POST to. Must be
  reachable from Zitadel's egress. On the Proxmox LXC deploy this is typically
  `https://mkauth.<your-domain>` — verify with `curl -I` from the Zitadel host
  before running the installer.
* Local tooling: `curl`, `jq`, `python3` (only for `smoke-test-action-v2.sh`
  signed variant).

### Service-account permissions

Actions v2 target management lives at the **instance scope** in Zitadel.
The org-level roles `.env.example` recommends for the backend's normal
user/grant CRUD (`ORG_OWNER` or `ORG_USER_MANAGER +
ORG_PROJECT_PERMISSION_EDITOR`) do NOT cover it — a fresh
`make zitadel-actions-register` on an org-only service user fails with
`HTTP 403`.

**Canonical reference:** the "Service-Account Permissions" section of
[`zitadel/actions/README.md`](../../../zitadel/actions/README.md#service-account-permissions)
— lives under the durable operator tree, survives the OpenSpec archive
workflow. Covers the per-call permission table, three
narrowest-first assignment paths (custom instance role → prebuilt
action-scoped role → `IAM_OWNER` fallback), duration guidance, and
separate-M2M-key options.

## Step 1 — Register the Action target

Assuming `.env` is populated (see `.env.example` for the keys), this is one
command:

```bash
make zitadel-actions-register
```

Or, with values provided inline instead of via `.env`:

```bash
ZITADEL_DOMAIN=auth.example.org \
  MKAUTH_EXTERNAL_URL=https://auth.example.org \
  ZITADEL_MACHINE_KEY_PATH=./zitadel-machine-key.json \
  make zitadel-actions-register
```

On first run, this creates the `mkauth-claim-injector` target, binds
`preaccesstoken` and `preuserinfo` executions, and writes the one-time
signing key to `zitadel/actions/.action-signing-key` (mode 0600).

Re-running the command is safe — the target is upserted by name.

## Step 2 — Inject the signing key into the backend

On fresh install (no prior rotate has run), seed the two env vars from the
files `register.sh` wrote:

```bash
{
  printf 'ZITADEL_ACTION_SIGNING_KEY=%s\n' "$(cat zitadel/actions/.action-signing-key)"
  printf 'ZITADEL_ACTION_SIGNING_KEY_ROTATED_AT=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
} >> .env
# Optional: override the default 90-day rotation-warning threshold
# echo "ZITADEL_ACTION_SIGNING_KEY_ROTATION_THRESHOLD_DAYS=90" >> .env
docker compose up -d backend
```

The `ROTATED_AT` env var feeds the Rotation Status panel on `/zitadel`
(`GET /api/v1/zitadel/action-rotation-status`). Using `date -u` here on
first install records the current time as the install timestamp so the age
meter starts at 0 instead of `unknown`.

**After every subsequent rotation**, `make zitadel-actions-rotate-key`
writes a ready-to-append env fragment to
`zitadel/actions/.action-env.fragment` (mode 0600) containing both updated
lines. Apply it with one redirect (no copy-paste from the terminal):

```bash
cat zitadel/actions/.action-env.fragment >> .env
# or for systemd: sudo install -m 0600 zitadel/actions/.action-env.fragment /etc/mkauth/action-env
docker compose up -d backend
rm zitadel/actions/.action-env.fragment
```

If your `.env` already has values for these keys, remove the old lines
before the redirect or use your deploy tool's standard env-overwrite path.

If you deploy via systemd instead of Compose, add the same variable to the
unit's `EnvironmentFile` and `systemctl restart mkauth-backend`.

**Verification:** once the backend restarts, `docker compose logs backend |
grep '\[ACTION\]'` should NOT show the `signature verification disabled
(dev mode)` warning. Absence of that line = production mode is on.

## Step 3 — Smoke test

```bash
ZITADEL_ACTION_SIGNING_KEY=$(cat zitadel/actions/.action-signing-key) \
  make zitadel-actions-verify
```

Expected output includes `OK: /api/action/inject returned a well-formed v2
envelope.` and a JSON body with an `append_claims` array.

## Step 4 — End-to-end validation against a real token

Issue an access token via your normal user OIDC flow (`make dev` login path
or production login), decode it, and confirm MkAuth-injected claims are
present. For a single-project grant the claim keys are flat (e.g. `role`);
for multi-project users the keys are namespaced `mkauth.<projectID>.<key>`.

Example (replace `<token>`):

```bash
echo "<token>" | cut -d. -f2 | base64 -d | jq
```

Also tail backend logs:

```bash
docker compose logs backend | grep '\[DATA PLANE\]'
```

`Cache hit for key=mapping:<userId>:<projectId>` = healthy path.
`Cache miss` lines during the first minute after deploy are expected while the
compiler warms Redis.

## Rollback

```bash
make zitadel-actions-remove
```

Unbinds the executions by issuing `PUT /v2/actions/executions` with
`targets: []` per bound condition. The target itself remains (so the
signing key and configuration survive). Token issuance falls back to stock
Zitadel claims. Because the target is configured with
`restCall.interruptOnError: false`, users experience no outage during rollback.

To fully remove, call the DELETE endpoint directly:

```bash
curl -X DELETE "https://${ZITADEL_DOMAIN}/v2/actions/targets/${TARGET_ID}" \
  -H "Authorization: Bearer ${TOKEN}"
```

Zitadel automatically removes the target from every execution binding as
part of the delete.

## Operator warnings

* **Do not** create a v1 Action with a `Pre access token creation` trigger on
  the same target project. On self-hosted Zitadel, both can coexist and both
  will inject claims, which may double-apply or conflict. If you have existing
  v1 Actions, audit and retire them before running `zitadel-actions-register`.
* **Do not** commit `zitadel/actions/.action-signing-key` — the `.gitignore`
  in that directory blocks it by default; do not override.
* **Do not** enable `interruptOnError: true` in `targets.json` without
  confirming per-project `fail_closed` semantics apply to every downstream
  app. The current posture uses `false` specifically so MkAuth unavailability
  does not block token issuance.
* **Do not** change the target type from `restCall` to `restWebhook`.
  Webhook targets only inspect the HTTP status code; they discard the
  response body — which is precisely where MkAuth returns the claim
  envelope. A webhook-typed target makes the deployment a functional no-op.
* **Do** rotate the signing key via `make zitadel-actions-rotate-key` (runs
  `zitadel/actions/rotate.sh`). The script calls
  `POST /v2/actions/targets/{id}` with `{"expirationSigningKey":"0s"}`,
  backs up the previous key to `.action-signing-key.previous`, and writes
  the new key to `.action-signing-key`. The raw curl is available in the
  README's "Signing Key Handling" section as a deep-dive fallback.
* **Do not** put `make zitadel-actions-rotate-key` on a schedule unless your
  compliance framework requires it. Zitadel does not expire the signing
  key (see the README's "Signing Key Handling" section, *Zitadel does not
  expire the signing key*); the
  first key works indefinitely. Rotate on-incident, on policy cadence, or
  on operator handoff — not on a cron tick.

## Troubleshooting

| Symptom | Probable cause | Fix |
|---|---|---|
| `register.sh`/`rotate.sh` exits with `HTTP 403` during `POST /v2/actions/targets*` | Service user lacks instance-scoped Actions permissions (most common when it was set up with only `ORG_OWNER`) | Grant `action.target.read`, `action.target.write`, `action.execution.write` (and optionally `action.target.delete`) at **Default Settings → Administrators**. Full matrix + narrowest-first assignment options in the "Service-Account Permissions" section of [`zitadel/actions/README.md`](../../../zitadel/actions/README.md#service-account-permissions). |
| `401 INVALID_SIGNATURE` on every Zitadel call | `ZITADEL_ACTION_SIGNING_KEY` not set on backend, or set to the wrong value | Set `ZITADEL_ACTION_SIGNING_KEY` to the contents of `zitadel/actions/.action-signing-key` in the backend env, restart. |
| `signature verification disabled (dev mode)` log in prod | `ZITADEL_ACTION_SIGNING_KEY` empty in container env | Same as above. |
| `/zitadel` Rotation Status panel shows `disabled` | `ZITADEL_ACTION_SIGNING_KEY` is unset on the backend (signature verification off) | Same root cause as the two rows above — fix the env var and restart before trusting any rotation age. |
| Custom claims missing from issued tokens | Executions not bound; or backend returning non-200 | Re-run `make zitadel-actions-register`; check backend logs for `[DATA PLANE]` entries. |
| `Cache miss for key=mapping:…` for every call | Cache compiler has not run / Redis not reachable | Verify `REDIS_URL` and that grant/rule changes have been recompiled. |
| Multi-project user sees colliding claim keys | Upstream Zitadel cached an older target response | Target response is always current — check for a stale v1 Action on the same trigger. |
| `/zitadel` Rotation Status panel shows `unknown` | `ZITADEL_ACTION_SIGNING_KEY_ROTATED_AT` not set on backend, malformed, or in the future | Seed it with `date -u +%Y-%m-%dT%H:%M:%SZ` now, or run `make zitadel-actions-rotate-key` and apply the emitted fragment: `cat zitadel/actions/.action-env.fragment >> .env`. |
| `/zitadel` Rotation Status panel shows `warn` or `stale` | Signing key is older than the configured threshold | Run `make zitadel-actions-rotate-key`; apply the emitted fragment (`cat zitadel/actions/.action-env.fragment >> .env`, or `sudo install -m 0600 …` for systemd); restart the backend; delete the fragment. |
