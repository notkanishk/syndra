# Deploying Syndra to Production

Production runs on a Proxmox LXC reachable at `https://syndra.example.org`
through a shared Caddy box. Deploys happen automatically on every push to
`main`, driven by a self-hosted GitHub Actions runner living inside the LXC.

This document covers a **first-time bring-up** and the **steady-state** deploy
loop. If the stack is already running, you almost certainly want
[Routine deploys](#routine-deploys) or [Operations](#operations).

### Placeholders

This guide is written against placeholders rather than one installation's real
addresses. Substitute your own throughout:

| Placeholder | What it stands for |
|---|---|
| `<APP_HOST>` | The LXC running the Syndra compose stack |
| `<PROXY_HOST>` | The reverse proxy terminating TLS (Caddy, in this guide) |
| `<ZITADEL_HOST>` | The host running your Zitadel instance |
| `<LEGACY_HOST>` | A previous deployment, referenced only in the appendix |
| `syndra.example.org` | The public hostname Syndra is served on |
| `auth.example.org` | Your Zitadel instance domain |

Keep your real values in `DEPLOY.local.md` — it is gitignored for exactly this
purpose.

---

## Topology

```
                    ┌──────────────────────────────────────┐
  browser  ───TLS──▶│ Caddy · <PROXY_HOST>                 │
                    │   syndra.example.org                 │
                    │   auth.example.org   (Zitadel)       │
                    └──────┬───────────────────────┬───────┘
                           │                       │
        /api/action/*      │                       │  everything else
        /api/webhooks/*    │                       │
                           ▼                       ▼
                  ┌────────────────────────────────────────┐
                  │ syndra · <APP_HOST>    (Docker LXC)    │
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
whole Zitadel instance, not to an organization or project. Only one Syndra
deployment can own the `syndra-claim-injector` and `syndra-event-listener`
targets on a given instance — registering a second one silently repoints the
first. A second environment therefore needs a *second Zitadel instance*, not a
second project.

---

## Prerequisites

| Thing | Value |
|---|---|
| LXC template | community-scripts `docker` (Debian 13, unprivileged) |
| Resources | 4 CPU · 6144 MB RAM · 32 GB disk (`bun run build` peaks past 2 GB) |
| Compose dir | `/opt/syndra`, owned by the `runner` user |
| Repo | `github.com/notkanishk/syndra` (a deploy key is still the right call — it scopes the host to read-only on one repo) |
| DNS | `syndra.example.org` → `<PROXY_HOST>` (the Caddy box) |

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
ssh root@<APP_HOST>
adduser --disabled-password --gecos "" runner
usermod -aG docker runner
install -d -o runner -g runner -m 755 /opt/syndra

su - runner -c 'ssh-keygen -t ed25519 -N "" -C "syndra-prod-deploy" -f ~/.ssh/id_ed25519'
su - runner -c 'ssh-keyscan -t ed25519 github.com > ~/.ssh/known_hosts'
cat /home/runner/.ssh/id_ed25519.pub
```

Add that public key to GitHub → repo **Settings → Deploy keys → Add deploy key**.
Leave *Allow write access* **unchecked** — production only ever reads.

### 2. Clone

```bash
su - runner -c 'git clone git@github.com:notkanishk/syndra.git /opt/syndra'
```

`/opt/syndra` is a long-lived clone. The Actions runner checks out into it
directly rather than into its own workspace, because this directory also holds
`.env`, the Zitadel service-account key, and the identity of the named Docker
volumes. Do not delete and re-clone it — that is how you lose the database
password while keeping the database.

### 3. Generate secrets

```bash
su - runner -c 'cd /opt/syndra && \
  EXTERNAL_URL=https://syndra.example.org \
  ZITADEL_DOMAIN=auth.example.org \
  ./scripts/gen-prod-env.sh'
```

Both variables are required; the script exits if either is unset. `EXTERNAL_URL`
is the address *Zitadel* will POST to, so it is the one value here that no local
health check ever exercises — a wrong value fails silently and permanently.

This writes `/opt/syndra/.env` at mode 600 with fresh random values for
`POSTGRES_PASSWORD`, `SYNDRA_API_KEY`, and `SESSION_SECRET`. It refuses to
overwrite an existing `.env`, because a regenerated `POSTGRES_PASSWORD` no
longer matches an already-initialized database volume.

The Zitadel identifiers are intentionally left blank — fill them in step 4.

`.env` is never committed. `.env.example` is the *development* reference and
must not be hand-adapted for production; its secrets are shared literals.

### 4. Zitadel setup

All of this happens in the Zitadel console at `https://auth.example.org`.

**a. Project.** Create a project (e.g. `Syndra Production`). In its settings,
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

Then open the application's **Token Settings**. The defaults do not work:

| Setting | Default | Required | Why |
|---|---|---|---|
| Auth Token Type | Bearer Token | **JWT** | `ui/src/app/auth/callback/route.ts:116` parses the access token as a JWT, and `backend/internal/auth/jwt.go` verifies its RS256 signature against the JWKS endpoint. An opaque bearer token fails both. |
| Add user roles to the access token | off | **on** | `ui/src/lib/oidc.ts:201` reads `urn:zitadel:iam:org:project:roles` out of the *access* token. Without it every operator resolves to `role: "user"`. |
| User roles inside ID Token | off | off | Nothing reads the ID token. |
| User Info inside ID Token | off | off | Same. Display name and email come from `/me/profile`, which `extractSessionFields` is written to fall back to. |

`ZITADEL_AUDIENCE` must equal this application's Client ID — with JWT access
tokens Zitadel puts the client ID in `aud`, and the backend checks it.

Copy the resulting **Client ID** into *both* `ZITADEL_CLIENT_ID` and
`ZITADEL_AUDIENCE` in `.env`. They are the same value; the backend validates
the token audience against it while the UI uses it to start the flow.

**c. Service user (M2M).** Users → Service Users → New. Set Access Token Type to
**JWT**. Then Keys → New → type **JSON** → Download. Zitadel shows the key
material once.

The downloaded file's `userId` field is `ZITADEL_M2M_USER_ID` — no need to go
back to the console for it:

```bash
jq -r .userId zitadel-machine-key.json
```

Rename it to `zitadel-machine-key.json` (what `.env` and `.gitignore` both
expect) and place it on the host:

```bash
# from your workstation
scp zitadel-machine-key.json root@<APP_HOST>:/opt/syndra/
ssh root@<APP_HOST> 'chown runner:runner /opt/syndra/zitadel-machine-key.json && chmod 600 $_'
```

The service user needs permissions at two different scopes:

- **Organization** (Organization → Members): `ORG_OWNER`, or the narrower pair
  `ORG_USER_MANAGER` + `ORG_PROJECT_PERMISSION_EDITOR`. This covers the
  backend's runtime user/grant/role CRUD.
- **Instance** (Default Settings → Administrators): `action.target.read`,
  `action.target.write`, `action.execution.write`. Org roles do **not** cover
  Actions v2 — without these, step 7 fails with HTTP 403. Add
  `action.target.delete` only if you intend to use `make zitadel-actions-purge`.

  Assign these narrowest-first: a custom instance role holding exactly those
  permissions (Default Settings → Roles) if your version supports it; otherwise
  a prebuilt action-scoped role such as `IAM_ACTION_ADMIN`; otherwise
  `IAM_OWNER`, which works everywhere but grants the whole instance.

Only the registration and rotation scripts use the instance permissions —
steady-state traffic is Zitadel calling Syndra, which needs none. You may
revoke them between runs, or issue a second machine key used only by the
scripts, if you want the backend's everyday key to carry no instance authority.

Full permission matrix: the "Service-Account Permissions" section of
`zitadel/actions/README.md`.

**d. Grant yourself access.** Assign the project's `admin` role to your own
user, or you will authenticate successfully and then see nothing.

**e. Verify the IdP.** Google Workspace is the sole IdP and is configured at the
instance level. Confirm it is active for the organization holding this project.

**f. Permit the target address in Zitadel's outgoing-HTTP denylist.** Zitadel
**v4.15.2** added a protected HTTP client and routes Actions v2 target calls
through it. Its `HTTPClient.DenyList` defaults to every RFC1918 range, so target
creation against any private address fails with:

```
Errors.Target.DeniedURL (COMMAND-NcJUKo)
```

This is deliberate SSRF hardening ([CVE-2026-55671]) and there is **no
allowlist** to pair with it ([zitadel#12326]) — the only lever is narrowing the
denylist. Targets created before the upgrade keep working, which is why an
existing deployment can look fine while a new one cannot register at all.

On the Zitadel host, add an `HTTPClient.DenyList` override to
`/opt/zitadel/config.yaml` — the stock list minus the range your deployment
lives in — then restart. There is no config-validation subcommand and the unit
is `Restart=always`, so validate the YAML off-host before writing it, and back
up first:

```yaml
HTTPClient:
  DenyList:
    - "localhost"
    - "0.0.0.0/8"
    - "10.0.0.0/8"
    - "100.64.0.0/10"
    - "127.0.0.0/8"
    - "169.254.0.0/16"
    - "172.16.0.0/12"
    - "198.18.0.0/15"
    - "::/128"
    - "::1/128"
    - "fc00::/7"
    - "fe80::/10"
```

`192.168.0.0/16` is omitted, which is what lets Zitadel POST to
`syndra.example.org` (→ `<PROXY_HOST>`). Loopback, link-local metadata,
`10/8` and `172.16/12` stay denied.

**This permits any 192.168.x.x target**, so Actions v2 target creation is now a
privileged operation — an operator who can create a target can point Zitadel at
anything on that network. If you want it tighter, replace `192.168.0.0/16` with
its complement around the single address Zitadel must reach; generate the CIDR
list programmatically rather than by hand.

Restarting Zitadel is a brief authentication outage for every service behind it.
Ours came back healthy in ~4s. Keep the rollback to hand:

```bash
cp -p /opt/zitadel/config.yaml.bak-<stamp> /opt/zitadel/config.yaml
systemctl restart zitadel
```

[CVE-2026-55671]: https://github.com/advisories/GHSA-29jh-8cfq-rr8x
[zitadel#12326]: https://github.com/zitadel/zitadel/issues/12326

### 5. Caddy

On the Caddy box (`<PROXY_HOST>`), add a site block:

```caddyfile
syndra.example.org {
	handle /api/action/* {
		reverse_proxy <APP_HOST>:8080
	}
	handle /api/webhooks/* {
		reverse_proxy <APP_HOST>:8080
	}
	handle {
		reverse_proxy <APP_HOST>:3000
	}
}
```

Then `caddy reload`. No application change is needed for the reverse proxy:
`ui/src/lib/oidc.ts:73-74` already derives the OIDC redirect URI from
`x-forwarded-host` / `x-forwarded-proto`, which Caddy sets by default.

#### 5a. Reaching a TrueNAS target

The NAS is not deployed by this runbook and Caddy is not on its path by
default. What Syndra needs from it is one property: **`TRUENAS_URL` must be a
`wss://` endpoint whose certificate the add-on can verify**, because TrueNAS
revokes a user-linked API key presented over plaintext, and because the
alternative is `TRUENAS_VERIFY_TLS=false`. Two ways to get there.

**Preferred — a real certificate on the NAS, no proxy.** TrueNAS issues its own
ACME certificate over a DNS-01 challenge (`Credentials > Certificates`, then
`System > General Settings > GUI SSL Certificate`), which works for a name whose
A record is LAN-only as long as the zone is publicly delegated for the
`_acme-challenge` TXT. Point `nas.example.org` straight at the NAS in
local DNS and everything reaches one name: the UI, the add-on, SMB, NFS.

That last part is the argument. **SMB and NFS cannot traverse Caddy** — it
proxies HTTP, and tcp/445 and tcp/2049 are not HTTP. Putting the name on the
proxy therefore splits the NAS across two names for no gain.

**Alternative — front the HTTP side with Caddy.** Reasonable if ACME on the NAS
is not an option. The site block is a plain WebSocket-transparent proxy; the
middleware API is JSON-RPC over one long-lived WebSocket at `/api/current` and
`reverse_proxy` upgrades it natively, with `Host` passed through unchanged.
`tls_insecure_skip_verify` belongs on the upstream leg only, where the NAS
presents its self-signed certificate. **SMB and NFS then need a second name
pointed directly at the NAS**, which is what `TRUENAS_SHARE_HOST` is for.

Either way, two resolvers must agree on the name: the administrator's browser
and the add-on container. The second is the one that gets missed — the container
resolves through the Docker daemon's resolver, not the LXC's `/etc/hosts`:

```bash
docker compose exec truenas-addon getent hosts nas.example.org
```

Nothing back means the add-on cannot reach the NAS by name whatever else is
configured, and the symptom is a target that registers and never answers.

### 6. Build the images

```bash
su - runner -c 'cd /opt/syndra && docker compose up -d --build'
```

Postgres, Redis, and the UI come up. **The backend will not**, and that is
correct:

```
[STARTUP] Production refusing to start: ZITADEL_DOMAIN is set but
ZITADEL_EVENT_SIGNING_KEY, ZITADEL_ACTION_SIGNING_KEY is empty.
```

`backend/cmd/api/main.go:42` refuses to serve with a live Zitadel configured and
no signing keys, because in that state `/api/action/inject` and
`/api/webhooks/zitadel` accept unsigned requests from anyone. The keys do not
exist until step 7 registers the targets, so the backend stays down across steps
6 and 7 by design. Do not work around it by clearing `ZITADEL_DOMAIN`.

### 7. Register Zitadel Actions

> This takes over the instance's Actions targets. Any other Syndra deployment
> pointed at the same Zitadel instance stops receiving claims and events at this
> moment. That is expected — see the note in [Topology](#topology).

**Prerequisites the script does not state.** `register.sh` needs bash 4+
(associative arrays) and, to mint its own M2M token, a Go toolchain. The
production LXC has bash 5 but no Go and no `make`; macOS has Go but ships bash
3.2. Neither host satisfies both, so mint the token where Go lives and run the
script where bash 4+ lives:

The token administers Actions v2 instance-wide, so it never touches disk on
either machine — pipe it straight from the minting host into the script:

```bash
# from a workstation with Go and the service-account key
cd backend && ZITADEL_DOMAIN=auth.example.org \
  ZITADEL_MACHINE_KEY_PATH=/abs/path/zitadel-machine-key.json \
  go run ./cmd/syndra-token \
| ssh root@<APP_HOST> 'su - runner -s /bin/bash -c "cd /opt/syndra && \
    ZITADEL_DOMAIN=auth.example.org \
    SYNDRA_EXTERNAL_URL=<the URL Zitadel will POST to> \
    ZITADEL_M2M_TOKEN=\$(cat) \
    ./zitadel/actions/register.sh"'
```

Do **not** stage it through `/tmp`. A redirect creates the file with the
umask's default mode — world-readable on most systems — and anyone who reads it
before you delete it can administer this Zitadel instance's Actions until the
token expires. `/tmp` is shared, so on a multi-user workstation or a host with
other service accounts that is a real window, not a theoretical one.

If the pipeline is impractical (an interactive `ssh` prompt, say), keep the file
out of shared directories and mode-restricted for its whole life:

```bash
umask 077   # 0600 on creation, not after
install -d -m 700 ~/.syndra && : > ~/.syndra/m2m.token
cd backend && ... go run ./cmd/syndra-token > ~/.syndra/m2m.token
scp ~/.syndra/m2m.token root@<APP_HOST>:/home/runner/.m2m.token
ssh root@<APP_HOST> 'chown runner:runner /home/runner/.m2m.token && chmod 600 $_'
# ... run register.sh ... then, unconditionally:
shred -u ~/.syndra/m2m.token
ssh root@<APP_HOST> 'shred -u /home/runner/.m2m.token'
```

Setting the mode *after* `>` creates the file is too late; the window is between
those two commands.

This creates two targets (`syndra-claim-injector`, `syndra-event-listener`) and
binds nine executions, per `zitadel/actions/targets.json`. Zitadel returns each
target's signing key **once, at creation**. Capture both into `.env`:

```
ZITADEL_ACTION_SIGNING_KEY=<from syndra-claim-injector>
ZITADEL_EVENT_SIGNING_KEY=<from syndra-event-listener>
ZITADEL_ACTION_SIGNING_KEY_ROTATED_AT=<current time, RFC3339 UTC>
```

The script also writes `zitadel/actions/.action-env.fragment`; prefer appending
that over copy-pasting:

```bash
cat zitadel/actions/.action-env.fragment >> .env
docker compose up -d backend
```

The backend has been down since step 6 and starts serving only once these are in
`.env`. It cannot have accepted an unsigned request in the meantime:
`requireProductionSigningKeys` (`backend/cmd/api/main.go`) returns early only
when `ZITADEL_DOMAIN` is unset, so the signature-passthrough it mentions is
reachable in local development alone — never in this flow, and never on a host
configured against a live Zitadel.

Verify:

```bash
su - runner -s /bin/bash -c "cd /opt/syndra && ./scripts/smoke-test-action-v2.sh http://localhost:8080"
su - runner -s /bin/bash -c "cd /opt/syndra && ./scripts/smoke-test-event-listener.sh http://localhost:8080"
```

### 8. Self-mutation loop guard

Take the service user's ID from its detail page in the Zitadel console (step 4c).
If you did not record it, trigger any grant change from the UI and read the
`editorUserId` off the resulting event in the Zitadel event log. Set it and
restart:

```
ZITADEL_M2M_USER_ID=<service user id>
```

Without this, Syndra's own writes echo back through the event listener and
re-trigger orchestration. It is disabled when unset — dev-mode behavior only.

### 9. Install the Actions runner

GitHub → repo **Settings → Actions → Runners → New self-hosted runner** to get a
registration token, then:

```bash
ssh root@<APP_HOST>
su - runner
mkdir -p ~/actions-runner && cd ~/actions-runner
RUNNER_VERSION=2.336.0   # check github.com/actions/runner/releases for current
curl -fsSL -o runner.tar.gz \
  "https://github.com/actions/runner/releases/download/v${RUNNER_VERSION}/actions-runner-linux-x64-${RUNNER_VERSION}.tar.gz"
tar xzf runner.tar.gz
./config.sh --unattended \
  --url https://github.com/notkanishk/syndra \
  --token <REGISTRATION_TOKEN> \
  --name syndra \
  --labels syndra-prod \
  --work _work
exit

cd /home/runner/actions-runner && ./svc.sh install runner && ./svc.sh start
./svc.sh status
```

The label `syndra-prod` is what `.github/workflows/deploy-prod.yml` targets.

> **Security.** Anyone who can push to `main` can execute arbitrary commands in
> production through the workflow file, because the workflow is itself part of
> the pushed code. Enable branch protection on `main` before relying on this.

---

## Routine deploys

Push to `main`. That is the whole procedure.

The workflow (`.github/workflows/deploy-prod.yml`) checks the pushed SHA out
into `/opt/syndra`, runs `docker compose up -d --build --remove-orphans`, prunes
dangling images, and smoke-tests `/healthz` and the UI root. Concurrency is
serialized, so overlapping pushes queue instead of racing.

Manual run of the same path, without a push:

```bash
# GitHub → Actions → deploy-prod → Run workflow   (workflow_dispatch)
```

Deploying by hand on the box, when the runner is down:

```bash
su - runner -c 'cd /opt/syndra && git fetch --prune origin && git checkout --force origin/main && docker compose up -d --build'
```

### Hosts without a git checkout

`/opt/syndra` is a working copy, and `git checkout --force` is what makes the
production path safe: it removes files the new commit deleted. A host synced any
other way does not get that for free.

The dev LXC is such a host, and the failure is real rather than theoretical. It
was synced by extracting a `git archive` over the existing tree, which could add
files and change files and **never remove one** — so
`ui/src/app/system/hardware-sync` survived the commit that deleted it, the
rebuilt image kept serving the route, and `/system/hardware-sync` answered 200
against a tree that no longer contained it. The stale route was the small half;
the large half was that a green rebuild was taken as evidence for a fix it had
never carried.

```bash
scripts/deploy-source.sh root@<HOST> [/root/syndra]
```

It mirrors rather than copies — the tracked directories are removed and
re-extracted — and, **before** it does, reports anything on the target that this
commit does not contain. That report is the load-bearing part: it is what would
have named `hardware-sync`. The check after the mirror only confirms the
script's own work, which is why both exist.

It never touches `.env` or `secrets/`. Host ports live in `.env` as
`BACKEND_HOST_PORT` / `UI_HOST_PORT` because they used to be edited into
`docker-compose.yml`, which is tracked, and the next sync overwrote them.

### Verifying on a host with no directory

A box with no `ZITADEL_DOMAIN` runs `Source=demo`, and the demo directory knows
five identities. Anything that walks the user directory — the role-holder list
is the one to watch — answers only for those five, so a cohort read against a
real Zitadel subject id **comes back empty and reads as "nothing to report"**
rather than as "nobody here". A smoke test written against the wrong ids gets a
green result from a query that examined nobody. Use `sam_student`, `maya_staff`,
`leo_mentor`, `ava_guest` or `dev_admin`, and assert a non-zero cohort before
asserting anything about its contents.

### Rollback

```bash
su - runner
cd /opt/syndra
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
docker compose exec postgres psql -U syndra syndradb
```

All commands run from `/opt/syndra` as the `runner` user.

### Secret rotation

- **`SYNDRA_API_KEY`** — edit `.env`, `docker compose up -d backend ui sync`.
  Because `SESSION_SECRET` is set independently, live sessions survive.
- **`SESSION_SECRET`** — edit `.env`, `docker compose up -d ui`. Every operator
  is logged out.
- **Zitadel signing keys** — `make zitadel-actions-rotate-key`, then apply the
  emitted `.action-env.fragment` and restart the backend. Zitadel does not
  expire these on its own; rotate on incident, policy, or operator handoff.
- **`POSTGRES_PASSWORD`** — requires changing it inside Postgres first (`ALTER
  ROLE syndra WITH PASSWORD ...`) and then in `.env`. Editing only `.env` locks
  the backend out of its own data.

### Add-on targets

Each add-on sits behind a Compose profile and does **not** start by default: it
holds a credential for the system it provisions, and a container nobody asked
for holding one is a container nobody is watching. Fill its block in `.env` —
the base URL and one transport secret — and start it:

```bash
docker compose --profile truenas up -d
```

Registration and callability are separate states. The backend registers a
configured add-on whether or not it answers, so navigation reflects the
deployment rather than the weather; what turns registration into capability is
the first successful manifest read.

#### Bringing up the TrueNAS add-on

Four things have to exist, in this order. Each one's failure looks like the next
one's, so doing them out of order costs an afternoon.

**1 — The NAS-side identity.** Roles in TrueNAS attach to *groups*, never
directly to users, and an API key inherits whatever its linked user's groups
carry. So: a group, a privilege naming the roles, a user in that group, a key on
that user.

- `Credentials > Groups > Add` — a group, e.g. `syndra-addon`. No sudo.
- `Credentials > Groups > Privileges > Add` — name it, put `syndra-addon` in
  **Local Groups**, and in **Roles** select `ACCOUNT_WRITE` and
  `SYSTEM_AUDIT_READ`. Not `FULL_ADMIN`.
- `Credentials > Users > Add` — e.g. `syndra`, primary group `syndra-addon`.
  No home directory, no shell, SSH off. It never logs in interactively.
- `Credentials > Users >` select the row `> Access > View API Keys > Add` —
  set an expiry, and record it. Copy the key **now**; TrueNAS shows it once.

> **`ACCOUNT_WRITE` includes deletion.** `user.delete` requires exactly the role
> `user.create` and `user.update` require, and TrueNAS has no narrower one — see
> [the RBAC reference][truenas-rbac]. The purge path's separate injected key is
> therefore an *audit and blast-radius* separation, not a capability one: it
> keeps deletion out of the long-lived session and makes every delete traceable
> to a credential issued for that one call. It does **not** mean the standing key
> cannot delete an account. Do not write it down as if it did.

> **HTTPS is not optional on this path.** TrueNAS automatically **revokes** a
> user-linked API key presented over plaintext transport. A misconfigured
> `TRUENAS_URL` of `ws://` does not fail with an auth error you can retry — it
> destroys the credential, and the fix is minting a new one.

**2 — Reachability.** Set `TRUENAS_URL=wss://nas.example.org/api/current`
(step 5a) and leave `TRUENAS_VERIFY_TLS=true`. Routing through the proxy is what
earns that: the NAS's own certificate is self-signed and cannot be issued for a
name it does not know it has, so pointing the add-on straight at the LAN address
means turning verification off. Set `TRUENAS_SHARE_HOST` to the name a member
types into a file manager — it feeds the manifest's connection block, and unset,
the member's page silently omits the mount instructions.

`TRUENAS_SHARE_HOST` is **not** the proxy name. Caddy terminates HTTP; SMB is
tcp/445 and does not pass through it, so a member told to mount
`nas.example.org` would be pointed at a host answering on 443 and nothing
else. Name the NAS directly. The two variables describe two different paths to
the same machine, which is exactly why the add-on never derives one from the
other.

**3 — The backend↔add-on channel.** One value, minted per target:

```bash
sudo ./scripts/gen-addon-secret.sh truenas
```

That writes `./secrets/addon/truenas.key` and prints the `.env` lines naming it.
From that one secret **both ends derive both keys** — the Ed25519 key the add-on
serves and the backend pins, and the HMAC key that signs every request. There is
no CA, no certificate to distribute, nothing that expires, and no second mode to
choose between. (Until recently this step minted four files across two
directories and asked you to pick a mode.)

Three things about it are load-bearing:

- **Run it under `sudo`.** The file is `0640 root:65532`: the backend reads it as
  owner, the add-on reads it by group. The add-on runs as uid 65532 against a
  read-only mount and cannot open a root-owned `0600` file, and `0644` would make
  the deployment's most sensitive value world-readable on the host. This is
  *deliberately not* part of step 3's `gen-prod-env.sh`, which runs as the
  unprivileged deploy user — see the note below.
- **Do not hand-pick the value.** An operator-chosen secret is the one input HKDF
  cannot strengthen.
- **Mint it before starting the add-on.** Docker creates a *directory* when a
  bind mount names a host path that does not exist, and the add-on then exits on
  a secret it cannot read. If that happened, `rmdir ./secrets/addon/truenas.key`
  and re-run; the script says so too.

Re-running for a target that already has a secret does nothing and exits 0 —
publication is a `link(2)`, which never clobbers, so a file here is always a
complete secret. Rotation is a separate procedure (below), because replacing the
value under a running pair is what strands an in-flight mutation.

> **Where this sits in the step order.** First-time bring-up step 3 (`su - runner
> … gen-prod-env.sh`) stays unprivileged and mints no add-on secret: the deploy
> user "must not run as root", and setting `root:65532` ownership requires exactly
> that. `ADDON_TARGETS` is also a line in the `.env` that script is generating, so
> there would be nothing to iterate. Minting is this per-target step, run under
> `sudo` when the target is actually being added.

**4 — Start it, in this order.** `ADDON_TARGETS=truenas` on the backend, then:

```bash
docker compose --profile truenas up -d truenas-addon
docker compose up -d backend            # re-reads ADDON_TARGETS
docker compose exec truenas-addon getent hosts nas.example.org
docker compose logs backend | grep '\[ADDON\]'
```

Expect `[ADDON] Registered target=truenas base=https://truenas-addon:8443
auth=derived`. `auth=none` means no secret reached the backend and the target
will not be callable — that is fail-closed, not a warning. Registration alone
proves nothing about the NAS; what does is the first manifest read, visible on
the target's health response.

| Symptom | Cause |
|---|---|
| Add-on exits at `TRUENAS_URL is required` | The profile started without the `.env` block. |
| Add-on exits at `ADDON_SECRET is required` | No secret reached it. A component holding the NAS credential must not answer an unauthenticated caller, so it refuses to start rather than serving one. |
| Add-on exits reading its secret as a directory | It was started before `gen-addon-secret.sh` ran, so Docker created a directory at the mount path. `rmdir ./secrets/addon/truenas.key`, mint, restart. |
| Registers, never answers | Name does not resolve *inside the container*, or the base URL names a host/port that is not `truenas-addon:8443`. |
| `addon truenas is not the one derived from this deployment's secret` | The pin failed, and it names all three causes it cannot tell apart: the two ends hold different bytes, the add-on's `ADDON_TARGET` does not match the `ADDON_TARGETS` entry (the name is the derivation's salt), or something else is answering on that address. Check the *name* before rotating a secret that was never the problem. |
| `no matching signature` | Same three causes, one leg further in: the handshake pinned but the request MAC did not verify. In practice this is a clock more than two minutes out, since the derivation would have failed at the handshake. |
| NAS auth fails right after it worked once | The key was presented over plaintext and TrueNAS revoked it. Mint a new one and fix the scheme. |
| NAS auth fails repeatedly, then stops responding | TrueNAS locks out for ten minutes after 20 failed authentications in 60 seconds. Wait it out; the add-on's own breaker is what keeps a retry loop from renewing it. |

#### Rotating an add-on transport secret

The two ends cannot move atomically. Calls fail to authenticate for a bounded
window rather than proceeding unauthenticated — which is the correct trade, but
it means an in-flight mutation must be allowed to settle first.

```bash
# 1. Drain — in the UI: Targets > TrueNAS > set lifecycle to `draining`, with a
#    reason. Or through the backend, which is the thing that can sign the call:
curl -sS -X POST "$SYNDRA/api/v1/targets/truenas/lifecycle" \
  -H "Authorization: Bearer $OPERATOR_TOKEN" -H 'Content-Type: application/json' \
  -d '{"state":"draining","reason":"transport secret rotation"}'
#    NOT curl to the add-on directly: /lifecycle is authenticated with a signed
#    request over the pinned transport, so only the backend can make it — and
#    over the very transport you are about to replace, which is why this is
#    step 1. Wait for `drained` on the target's health.
#
#    Editing TRUENAS_LIFECYCLE_STATE does nothing here: it is read once at
#    start-up, and applying the edit needs the very recreation the drain exists
#    to precede.

# 2. Replace the value. One file, both ends.
sudo mv ./secrets/addon/truenas.key ./secrets/addon/truenas.key.prev
sudo ./scripts/gen-addon-secret.sh truenas

# 3. Recreate. NOT `restart`: that reuses the environment the container was
#    created with, so both ends would report success while still holding the
#    previous value.
docker compose up -d --force-recreate backend truenas-addon

# 4. Confirm registration and the first manifest read.
docker compose logs backend | grep '\[ADDON\]'
```

Once the secret is replaced you can no longer reach the old container's
lifecycle handler, and the only remaining lever is SIGTERM — which is why the
drain is step 1 and not step 3. Rollback is restoring `.prev` and recreating
again; nothing persistent depends on the secret, so it is clean.

[truenas-rbac]: https://api.truenas.com/v25.10/rbac.html

---

## Troubleshooting

| Symptom | Cause |
|---|---|
| `POSTGRES_PASSWORD is required` on `up` | `.env` missing or not in the compose directory. Run `scripts/gen-prod-env.sh`. |
| Backend logs `Source=demo` / `Source=local` | `ZITADEL_DOMAIN` or the machine key path is wrong; backend fell back to local-policy-only mode. |
| Login loops back to `/login` | Redirect URI mismatch. Zitadel's registered URI must be byte-identical to `https://syndra.example.org/auth/callback`. |
| Logged in but the console is empty | Project role not granted to your user, or **Assert Roles on Authentication** is off. |
| `register.sh` returns 403 | Service user has org roles but not the instance-level `action.*` permissions. |
| `register.sh` returns 400 `Errors.Target.DeniedURL` | Zitadel ≥ v4.15.2 denies RFC1918 target addresses by default. See step 4f. |
| `register.sh`: "requires bash 4+" | macOS ships bash 3.2. Run it on the LXC (bash 5) with a pre-minted `ZITADEL_M2M_TOKEN`. See step 7. |
| Backend restart-loops on `Production refusing to start` | Signing keys are not in `.env` yet. Expected between steps 6 and 7; not expected afterwards. |
| Tokens carry no Syndra claims | Another deployment re-registered the instance's Actions targets and repointed them. Re-run step 7. |
| Webhook endpoint returns 401 | `ZITADEL_EVENT_SIGNING_KEY` in `.env` no longer matches the target's key. Rotate or re-capture. |

---

## Appendix: what the first deployment got wrong

The original host (`syndra-test`, `<LEGACY_HOST>`) grew by hand over several
months. Everything below is a real defect found on it, and each one is the
reason the corresponding decision above looks the way it does. Keep this list
when rebuilding that box.

**`SYNDRA_EXTERNAL_URL` drifted to a decommissioned IP.** The `.env` pointed at
`<LEGACY_HOST_OLD>:8080`; the box had since moved to `.16`. Nothing failed loudly —
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

**Two sets of database volumes.** `syndra_pgdata` and
`makerspace-authority_pgdata` both exist. Compose derives its project name from
the directory name, so renaming the checkout stranded the original volume while
starting a fresh one. Neither is labelled; only the timestamps distinguish them.
*Prod pins the checkout at `/opt/syndra` and never moves it.*

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

**No reproducible deploy path.** `install.sh` and `update.sh` assume a human at
an SSH prompt. A third script, `scripts/deploy-lxc.sh`, was worse: it targeted a
host that had been decommissioned and `rm -rf`d the remote directory before
extracting, which would have taken `.env` and the Zitadel machine key with it.
It has been deleted rather than fixed — the runner in
[Routine deploys](#routine-deploys) supersedes it, and a rsync-and-pray script
pointed at a live identity system is not worth keeping around for nostalgia.
