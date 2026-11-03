# Operator Deployment Guide: Zitadel Actions v2

> **Companion to:** [proposal.md](proposal.md) | [design.md](design.md) | [tasks.md](tasks.md) | [IMPLEMENTATION](IMPLEMENTATION.md)

This is the operator-run procedure for registering the Syndra claim-injection
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
  out to `backend/cmd/syndra-token` (Go toolchain required on the host) to
  mint a fresh token via the JWT profile grant.
* `SYNDRA_EXTERNAL_URL` — the public URL Zitadel will POST to. Must be
  reachable from Zitadel's egress. On the Proxmox LXC deploy this is typically
  `https://syndra.<your-domain>` — verify with `curl -I` from the Zitadel host
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
  SYNDRA_EXTERNAL_URL=https://syndra.example.org \
  ZITADEL_MACHINE_KEY_PATH=./zitadel-machine-key.json \
  make zitadel-actions-register
```

On first run, this creates the `syndra-claim-injector` target, binds
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
# or for systemd: sudo install -m 0600 zitadel/actions/.action-env.fragment /etc/syndra/action-env
docker compose up -d backend
rm zitadel/actions/.action-env.fragment
```

If your `.env` already has values for these keys, remove the old lines
before the redirect or use your deploy tool's standard env-overwrite path.

If you deploy via systemd instead of Compose, add the same variable to the
unit's `EnvironmentFile` and `systemctl restart syndra-backend`.

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
or production login), decode it, and confirm Syndra-injected claims are
present. The keys are the project's configured names (e.g. `roles`). A token
carries one project: if the app requests another project's audience scope,
the token carries no Syndra claims and the backend logs `Refusing
cross-project claims` (ADR-002).

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
  app. The current posture uses `false` specifically so Syndra unavailability
  does not block token issuance.
* **Do not** change the target type from `restCall` to `restWebhook`.
  Webhook targets only inspect the HTTP status code; they discard the
  response body — which is precisely where Syndra returns the claim
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
| `/healthz/claims` counts `degraded_no_facts` for every call | Cache compiler has not run / Redis not reachable | Verify `REDIS_URL` and that the warm ran (`warm_age_s`); a miss also queues a recompile of that person. |
| `/healthz/claims` says `SNAPSHOT_NOT_LOADED` or `SNAPSHOT_EMPTY` | The data plane has no config yet: Redis is down, or the backend has not published (first boot of S2, before migrations finish) | Wait a minute (the publish runs at start and every minute); check Redis. Every call meanwhile answers fail closed, counted. |
| Token carries no Syndra claims, log says `Refusing cross-project claims` | The app requests another project's `:aud` scope, so the token spans two projects | Drop the extra audience scope, or give the role in the app's own project with a rule. |
| Token carries no Syndra claims, log says `No project grants` | The app neither asserts roles nor requests role scopes, so Zitadel lists no grants | Turn on "Assert roles on authentication" for the project or app, or request `urn:zitadel:iam:org:project:roles`. |
| `/zitadel` Rotation Status panel shows `unknown` | `ZITADEL_ACTION_SIGNING_KEY_ROTATED_AT` not set on backend, malformed, or in the future | Seed it with `date -u +%Y-%m-%dT%H:%M:%SZ` now, or run `make zitadel-actions-rotate-key` and apply the emitted fragment: `cat zitadel/actions/.action-env.fragment >> .env`. |
| `/zitadel` Rotation Status panel shows `warn` or `stale` | Signing key is older than the configured threshold | Run `make zitadel-actions-rotate-key`; apply the emitted fragment (`cat zitadel/actions/.action-env.fragment >> .env`, or `sudo install -m 0600 …` for systemd); restart the backend; delete the fragment. |

## 3 Nov 2026: the grant field name

Zitadel sends each grant's project as `project_id` (its `query.UserGrant` JSON tags). Its docs show `projectId`. Syndra read only `projectId`, so every call logged `No project grants in request` and answered with no claims. Actions v1 kept filling the tokens, so nothing failed loudly. Syndra now reads both. After deploy, a healthy sign-in logs `Cache hit for key=mapping:…`; a `No project grants` line now really means the token listed no project. (Since S1 a hit is a count, not a line: `outcomes_10m.served` on `/healthz/claims`.)

## 3 Nov 2026: the data plane's own door (claims-delivery-integrity S2)

The backend now serves the claims target on a second listener, `:8090` (`SYNDRA_CLAIMS_ADDR`, host port `CLAIMS_HOST_PORT`), with only `POST /api/action/inject` and `GET /healthz/claims` on it. It starts first, before migrations, and answers from an in-memory snapshot plus one Redis read, never Postgres or Zitadel. The old route on `:8080` keeps working, so nothing breaks before the Caddy edit.

**Owner step, on the Caddy host (`<PROXY_HOST>`):** in the Syndra site block, replace the `/api/action/*` handle with exactly this, then `caddy reload`:

```caddyfile
	handle /api/action/* {
		reverse_proxy <APP_HOST>:8090 {
			lb_try_duration 2s
			lb_try_interval 100ms
		}
	}
```

`lb_try_duration` makes Caddy retry a refused connection for up to 2s, so a restart shorter than that is latency, not a token without claims. Caddy retries only dial failures for a POST, so a call that reached Syndra is never sent twice. Leave `/api/webhooks/*` on `:8080`.

**Check after the edit:** `curl -s -X POST -d '{}' https://<syndra host>/api/action/inject` still answers 401 (the deploy job checks the same), and `curl -s http://<APP_HOST>:8090/healthz/claims` shows `"snapshot":{"loaded":true,…}`.

**Rollback:** point the handle back at `<APP_HOST>:8080`. Both doors serve the same answer.

## 3 Nov 2026: the detectors and the keyring (claims-delivery-integrity S3, S4, S5)

Three things now watch claim delivery from outside the token path, and the claims target's signing key is read from Zitadel instead of only from `.env`:

- **The sign-in correlator** (S3) matches every `oidc_session.added` against the calls Zitadel made for that person and app. No call within 2 minutes either side is `CLAIMS_NOT_CALLED`; a call answered without the person's claims is `CLAIMS_NOT_SERVED`. It needs no setup.
- **The configuration reconciler** (S4) reads the targets, executions, the Actions v1 Complement Token flow and every app's token settings at start and every 10 minutes, and raises `CONFIG_DRIFT` for each check (R1 to R9) that differs. Read-only.
- **The keyring** (S5) reads the claims target's signing key every minute, and at once after a call matches no key Syndra holds. The key Zitadel signed with before a rotation keeps verifying for 15 minutes. `ZITADEL_ACTION_SIGNING_KEY` stays the starting point and the fallback: with the read denied, it alone verifies.

All three surface on System › Zitadel ("Claims delivery") and on Home's claims line, and as id-free reasons on `GET /healthz/claims`.

**Owner step 1 (S4-perm): let Syndra read the Actions setup.** In the Zitadel console, Instance → Managers, add the backend's service account (the user in `ZITADEL_MACHINE_KEY_PATH`'s key) with the role **`IAM_OWNER_VIEWER`**. It is read-only: it grants `action.target.read` and `action.execution.read`. It also lets the account read the claims target's signing key, which Syndra already holds. Until this is done, System › Zitadel shows "couldn't check" for R1 to R3 (`CANNOT_CHECK`), Home counts it as a problem, and the keyring cannot follow a rotation (the env key keeps verifying).

*Check:* within 10 minutes (or after a backend restart), `curl -s http://<APP_HOST>:8090/healthz/claims` no longer lists `CANNOT_CHECK:R1`, and System › Zitadel's Configuration part says the setup matches.

**Owner step 2 (S3-alert): the dead-man's switch.** Create a check that expects a ping every 5 minutes with a grace of 10 (healthchecks.io free tier, or an Uptime Kuma push monitor), alerting your phone or email. Put its ping URL in the server's `.env` as `SYNDRA_DEADMAN_URL=…` and redeploy. Syndra pings it every 5 minutes **only while** `/healthz/claims` is `ok`, with the reasons in the body; any problem, a crashed backend or a stopped LXC is a missed ping, and the service alerts you. Do step 1 first: an open `CANNOT_CHECK` stops the pings.

*Check:* the service shows a ping within 5 minutes of the deploy. Unset, `/healthz/claims` carries the note `NO_EXTERNAL_ALERT` (it changes no status). The URL is never logged.

**Rotating the claims key** is now `make zitadel-actions-rotate-key` alone: Syndra follows within seconds, no restart. Update `ZITADEL_ACTION_SIGNING_KEY` in `.env` at the next deploy (System › Zitadel says so while it is behind). The event listener's key (`ZITADEL_EVENT_SIGNING_KEY`) is still env-only and still needs the restart.

**The proof still to record** (`changes/claims-delivery-integrity/tasks.md`): on the dev box, unbind the `preaccesstoken` execution and confirm `CONFIG_DRIFT:R2` within 10 minutes and `CLAIMS_NOT_CALLED` within 5 minutes of a sign-in, and a missed dead-man ping; rotate the key and confirm zero `sig_mismatch` after the first call.
