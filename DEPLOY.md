# Deploying MkAuth to Production

Production runs on a Proxmox LXC named `syndra` (`198.51.100.12`), reachable at
`https://syndra.example.org` through the shared Caddy box at `198.51.100.15`.
Deploys happen automatically on every push to `main`, driven by a self-hosted
GitHub Actions runner living inside the LXC.

This document covers a **first-time bring-up** and the **steady-state** deploy
loop. If the stack is already running, you almost certainly want
[Routine deploys](#routine-deploys) or [Operations](#operations).

---

## Topology

```
                    ┌─────────────────────────────────────┐
  browser  ───TLS──▶│ Caddy · 198.51.100.15                 │
                    │  syndra.example.org            │
                    │  auth.example.org  (Zitadel)   │
                    └──────┬──────────────────────┬───────┘
                           │                      │
        /api/action/*      │                      │  everything else
        /api/webhooks/*    │                      │
                           ▼                      ▼
                  ┌────────────────────────────────────────┐
                  │ syndra · 198.51.100.12   (Docker LXC)   │
                  │                                        │
                  │  backend :8080 ◀── ui :3000            │
                  │      │                                 │
                  │  postgres   redis      [sync — opt-in] │
                  └────────────────────────────────────────┘
```

Only two paths need to reach the backend from outside: `/api/action/inject` and
`/api/webhooks/zitadel`, both POSTed to by Zitadel. Everything else is served by
the UI, which talks to the backend over the internal Docker network. The UI owns
exactly one route under `/api` (`/api/proxy/[...path]`), so the path split above
has no collisions.

**Zitadel Actions v2 is instance-scoped.** Targets and executions belong to the
whole Zitadel instance, not to an organization or project. Only one MkAuth
deployment can own the `mkauth-claim-injector` and `mkauth-event-listener`
targets on a given instance — registering a second one silently repoints the
first. A second environment therefore needs a *second Zitadel instance*, not a
second project.

---

## Prerequisites

| Thing | Value |
|---|---|
| LXC template | community-scripts `docker` (Debian 13, unprivileged) |
| Resources | 4 CPU · 6144 MB RAM · 32 GB disk (`bun run build` peaks past 2 GB) |
| Compose dir | `/opt/mkauth`, owned by the `runner` user |
| Repo | `github.com/notkanishk/makerspace-authority` — **private**, needs a deploy key |
| DNS | `syndra.example.org` → `198.51.100.15` (the Caddy box) |

Create the LXC on the Proxmox host with:

```bash
bash -c "$(curl -fsSL https://raw.githubusercontent.com/community-scripts/ProxmoxVE/main/ct/docker.sh)"
```

Choose Advanced and override the defaults (2 CPU / 2 GB / 4 GB) with the values
in the table. Skip the Portainer add-on. Assign a static IP.

---

## First-time bring-up

### 1. Host user and deploy key

Everything runs as an unprivileged `runner` user that is a member of the
`docker` group. The GitHub Actions runner must not run as root.

```bash
ssh root@198.51.100.12
adduser --disabled-password --gecos "" runner
usermod -aG docker runner
install -d -o runner -g runner -m 755 /opt/mkauth

su - runner -c 'ssh-keygen -t ed25519 -N "" -C "syndra-mkauth-deploy" -f ~/.ssh/id_ed25519'
su - runner -c 'ssh-keyscan -t ed25519 github.com > ~/.ssh/known_hosts'
cat /home/runner/.ssh/id_ed25519.pub
```

Add that public key to GitHub → repo **Settings → Deploy keys → Add deploy key**.
Leave *Allow write access* **unchecked** — production only ever reads.

### 2. Clone

```bash
su - runner -c 'git clone git@github.com:notkanishk/makerspace-authority.git /opt/mkauth'
```

`/opt/mkauth` is a long-lived clone. The Actions runner checks out into it
directly rather than into its own workspace, because this directory also holds
`.env`, the Zitadel service-account key, and the identity of the named Docker
volumes. Do not delete and re-clone it — that is how you lose the database
password while keeping the database.

### 3. Generate secrets

```bash
su - runner -c 'cd /opt/mkauth && ./scripts/gen-prod-env.sh'
```

This writes `/opt/mkauth/.env` at mode 600 with fresh random values for
`POSTGRES_PASSWORD`, `MKAUTH_API_KEY`, and `SESSION_SECRET`. It refuses to
overwrite an existing `.env`, because a regenerated `POSTGRES_PASSWORD` no
longer matches an already-initialized database volume.

The Zitadel identifiers are intentionally left blank — fill them in step 4.

`.env` is never committed. `.env.example` is the *development* reference and
must not be hand-adapted for production; its secrets are shared literals.

### 4. Zitadel setup

All of this happens in the Zitadel console at `https://auth.example.org`.

**a. Project.** Create a project (e.g. `MkAuth Production`). In its settings,
enable **Assert Roles on Authentication** — without it Zitadel omits the
`urn:zitadel:iam:org:project:roles` claim, and the UI has no way to tell an
operator from anyone else (`ui/src/lib/oidc.ts:201` reads exactly that claim).

Under **Roles**, create at minimum `admin`. This must match
`ZITADEL_ADMIN_ROLE_KEY` in `.env` (default `admin`). It gates the
`/api/v1/zitadel/*` discovery and management endpoints.

**b. Application.** Inside the project, create an application:

| Field | Value |
|---|---|
| Type | Web |
| Auth method | PKCE |
| Redirect URI | `https://syndra.example.org/auth/callback` |
| Dev mode | off |

Post-logout redirect is not needed — `/auth/logout` clears the local session
cookie and redirects to `/login` without calling Zitadel's end-session endpoint.

Copy the resulting **Client ID** into *both* `ZITADEL_CLIENT_ID` and
`ZITADEL_AUDIENCE` in `.env`. They are the same value; the backend validates
the token audience against it while the UI uses it to start the flow.

**c. Service user (M2M).** Create a machine user, then generate and download a
**JSON key**. Place it on the host:

```bash
# from your workstation
scp zitadel-machine-key.json root@198.51.100.12:/opt/mkauth/
ssh root@198.51.100.12 'chown runner:runner /opt/mkauth/zitadel-machine-key.json && chmod 600 $_'
```

The service user needs permissions at two different scopes:

- **Organization** (Organization → Members): `ORG_OWNER`, or the narrower pair
  `ORG_USER_MANAGER` + `ORG_PROJECT_PERMISSION_EDITOR`. This covers the
  backend's runtime user/grant/role CRUD.
- **Instance** (Default Settings → Administrators): `action.target.read`,
  `action.target.write`, `action.execution.write`. Org roles do **not** cover
  Actions v2 — without these, step 7 fails with HTTP 403. Add
  `action.target.delete` only if you intend to use `make zitadel-actions-purge`.

Full permission matrix: the "Service-Account Permissions" section of
`zitadel/actions/README.md`.

**d. Grant yourself access.** Assign the project's `admin` role to your own
user, or you will authenticate successfully and then see nothing.

**e. Verify the IdP.** Google Workspace is the sole IdP and is configured at the
instance level. Confirm it is active for the organization holding this project.

### 5. Caddy

On the Caddy box (`198.51.100.15`), add a site block:

```caddyfile
syndra.example.org {
	handle /api/action/* {
		reverse_proxy 198.51.100.12:8080
	}
	handle /api/webhooks/* {
		reverse_proxy 198.51.100.12:8080
	}
	handle {
		reverse_proxy 198.51.100.12:3000
	}
}
```

Then `caddy reload`. No application change is needed for the reverse proxy:
`ui/src/lib/oidc.ts:73-74` already derives the OIDC redirect URI from
`x-forwarded-host` / `x-forwarded-proto`, which Caddy sets by default.

### 6. First boot

```bash
su - runner -c 'cd /opt/mkauth && docker compose up -d --build'
curl -fsS http://198.51.100.12:8080/healthz
```

Migrations run automatically on backend start. Confirm live Zitadel mode in the
logs — look for `[DIRECTORY] Source=zitadel`. If it says anything else, the
machine key or `ZITADEL_DOMAIN` is wrong and the backend fell back to
local-policy-only mode.

Then log in at `https://syndra.example.org`.

### 7. Register Zitadel Actions

> This takes over the instance's Actions targets. Any other MkAuth deployment
> pointed at the same Zitadel instance stops receiving claims and events at this
> moment. That is expected — see the note in [Topology](#topology).

```bash
su - runner -c 'cd /opt/mkauth && set -a && . ./.env && set +a && make zitadel-actions-register'
```

This creates two targets (`mkauth-claim-injector`, `mkauth-event-listener`) and
binds nine executions, per `zitadel/actions/targets.json`. Zitadel returns each
target's signing key **once, at creation**. Capture both into `.env`:

```
ZITADEL_ACTION_SIGNING_KEY=<from mkauth-claim-injector>
ZITADEL_EVENT_SIGNING_KEY=<from mkauth-event-listener>
ZITADEL_ACTION_SIGNING_KEY_ROTATED_AT=<current time, RFC3339 UTC>
```

The script also writes `zitadel/actions/.action-env.fragment`; prefer appending
that over copy-pasting:

```bash
cat zitadel/actions/.action-env.fragment >> .env
docker compose up -d backend
```

Until these keys are set, both endpoints accept **unsigned** requests and log a
warning. Do not leave a publicly routable production host in that state.

Verify:

```bash
make zitadel-actions-verify        BACKEND_URL=http://localhost:8080
make zitadel-actions-verify-events BACKEND_URL=http://localhost:8080
```

### 8. Self-mutation loop guard

Trigger any grant change from the UI, then find the M2M service user's ID in the
Zitadel event log (it appears as the `editorUserId` on the resulting event). Set
it and restart:

```
ZITADEL_M2M_USER_ID=<service user id>
```

Without this, MkAuth's own writes echo back through the event listener and
re-trigger orchestration. It is disabled when unset — dev-mode behavior only.

### 9. Install the Actions runner

GitHub → repo **Settings → Actions → Runners → New self-hosted runner** to get a
registration token, then:

```bash
ssh root@198.51.100.12
su - runner
mkdir -p ~/actions-runner && cd ~/actions-runner
RUNNER_VERSION=2.336.0   # check github.com/actions/runner/releases for current
curl -fsSL -o runner.tar.gz \
  "https://github.com/actions/runner/releases/download/v${RUNNER_VERSION}/actions-runner-linux-x64-${RUNNER_VERSION}.tar.gz"
tar xzf runner.tar.gz
./config.sh --unattended \
  --url https://github.com/notkanishk/makerspace-authority \
  --token <REGISTRATION_TOKEN> \
  --name syndra \
  --labels mkauth-prod \
  --work _work
exit

cd /home/runner/actions-runner && ./svc.sh install runner && ./svc.sh start
./svc.sh status
```

The label `mkauth-prod` is what `.github/workflows/deploy-prod.yml` targets.

> **Security.** Anyone who can push to `main` can execute arbitrary commands in
> production through the workflow file, because the workflow is itself part of
> the pushed code. Enable branch protection on `main` before relying on this.

---

## Routine deploys

Push to `main`. That is the whole procedure.

The workflow (`.github/workflows/deploy-prod.yml`) checks the pushed SHA out
into `/opt/mkauth`, runs `docker compose up -d --build --remove-orphans`, prunes
dangling images, and smoke-tests `/healthz` and the UI root. Concurrency is
serialized, so overlapping pushes queue instead of racing.

Manual run of the same path, without a push:

```bash
# GitHub → Actions → deploy-prod → Run workflow   (workflow_dispatch)
```

Deploying by hand on the box, when the runner is down:

```bash
su - runner -c 'cd /opt/mkauth && git fetch --prune origin && git checkout --force origin/main && docker compose up -d --build'
```

### Rollback

```bash
su - runner
cd /opt/mkauth
git log --oneline -10
git checkout --force <good-sha>
docker compose up -d --build
```

Database migrations do not roll back. A rollback across a migration boundary
needs the migration reverted deliberately first.

---

## Operations

```bash
# status and logs
docker compose ps
docker compose logs -f backend
docker compose logs --tail=200 ui

# restart one service after an .env change
docker compose up -d backend

# database shell
docker compose exec postgres psql -U mkauth mkauthdb
```

All commands run from `/opt/mkauth` as the `runner` user.

### Secret rotation

- **`MKAUTH_API_KEY`** — edit `.env`, `docker compose up -d backend ui sync`.
  Because `SESSION_SECRET` is set independently, live sessions survive.
- **`SESSION_SECRET`** — edit `.env`, `docker compose up -d ui`. Every operator
  is logged out.
- **Zitadel signing keys** — `make zitadel-actions-rotate-key`, then apply the
  emitted `.action-env.fragment` and restart the backend. Zitadel does not
  expire these on its own; rotate on incident, policy, or operator handoff.
- **`POSTGRES_PASSWORD`** — requires changing it inside Postgres first (`ALTER
  ROLE mkauth WITH PASSWORD ...`) and then in `.env`. Editing only `.env` locks
  the backend out of its own data.

### The sync service

`sync` sits behind a Compose profile and does **not** start by default. Without
a reachable LLDAP it crash-loops, and a permanently restarting container hides
real failures. Once LLDAP exists, fill the `LLDAP_*` block in `.env` and:

```bash
docker compose --profile sync up -d
```

---

## Troubleshooting

| Symptom | Cause |
|---|---|
| `POSTGRES_PASSWORD is required` on `up` | `.env` missing or not in the compose directory. Run `scripts/gen-prod-env.sh`. |
| Backend logs `Source=demo` / `Source=local` | `ZITADEL_DOMAIN` or the machine key path is wrong; backend fell back to local-policy-only mode. |
| Login loops back to `/login` | Redirect URI mismatch. Zitadel's registered URI must be byte-identical to `https://syndra.example.org/auth/callback`. |
| Logged in but the console is empty | Project role not granted to your user, or **Assert Roles on Authentication** is off. |
| `make zitadel-actions-register` returns 403 | Service user has org roles but not the instance-level `action.*` permissions. |
| Tokens carry no MkAuth claims | Another deployment re-registered the instance's Actions targets and repointed them. Re-run step 7. |
| Webhook endpoint returns 401 | `ZITADEL_EVENT_SIGNING_KEY` in `.env` no longer matches the target's key. Rotate or re-capture. |

---

## Appendix: what the first deployment got wrong

The original host (`mkauth-test`, `198.51.100.16`) grew by hand over several
months. Everything below is a real defect found on it, and each one is the
reason the corresponding decision above looks the way it does. Keep this list
when rebuilding that box.

**`MKAUTH_EXTERNAL_URL` drifted to a decommissioned IP.** The `.env` pointed at
`198.51.100.14:8080`; the box had since moved to `.16`. Nothing failed loudly —
Zitadel kept POSTing to a host that no longer answered, so the claim injector
and event listener were silently dead. `CORS_ORIGIN` and `NEXT_PUBLIC_API_URL`
carried the same stale address. *Prod counters this with the deploy-time check
that the public Action target URL reaches the running deployment, and by using
a hostname instead of an IP.*

**Secrets were world-readable.** `.env` and `zitadel-machine-key.json` were both
mode `644`, owned by root, in `/root`. A service-account private key that any
account on the box can read is a credential, not a secret. *Prod runs as an
unprivileged `runner` user with both files at mode 600.*

**Everything ran as root**, including what a CI runner would drive. *Prod
separates the deploy identity from the host's root account.*

**Two sets of database volumes.** `mkauth_pgdata` and
`makerspace-authority_pgdata` both exist. Compose derives its project name from
the directory name, so renaming the checkout stranded the original volume while
starting a fresh one. Neither is labelled; only the timestamps distinguish them.
*Prod pins the checkout at `/opt/mkauth` and never moves it.*

**Build cache reached 10.5 GB** — a quarter of the disk. `update.sh` prunes
images but not the builder cache, which is the part that actually grows. *Prod's
workflow prunes both, with a 4 GB cache floor so rebuilds stay fast.*

**`ZITADEL_M2M_USER_ID` was never set**, leaving the self-mutation loop guard
disabled for the entire life of the deployment. Backend-initiated grant changes
echoed back through the event listener and re-triggered orchestration.
*Prod treats it as a required bring-up step (step 8) rather than an optional
tuning knob.*

**`ZITADEL_WEBHOOK_SECRET` was still in `.env`** — a dead variable from a
pre-Actions-v2 webhook design, referenced by no code. Dead config is
indistinguishable from live config during an incident.

**`sync` had been crash-looping for months.** Its `Restarting (1)` line was
permanent furniture in `docker ps`, which is exactly how a real failure gets
missed. *Prod gates it behind a compose profile.*

**`postfix` runs on the host** — an unused MTA inherited from the template.
Remove it: `apt purge postfix`.

**No reproducible deploy path.** `install.sh`, `update.sh`, and
`scripts/deploy-lxc.sh` all assume a human at an SSH prompt, and
`deploy-lxc.sh` still targets `198.51.100.14` and `rm -rf`s the remote directory
before extracting — which deletes `.env` and the Zitadel machine key along with
it. Do not point that script at a host you care about until it is fixed.
