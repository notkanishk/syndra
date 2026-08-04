> **Status:** In Progress | [< Index](../../INDEX.md) · [Proposal](proposal.md) · [Design](design.md)

# Tasks — Production Environment & Rename to Syndra

## 1. Compose contract

- [x] 1.1 `POSTGRES_PASSWORD` required with no default; `DB_DSN` derives from it
- [x] 1.2 `SESSION_SECRET` passed to the `ui` service
- [x] 1.3 `sync` moved behind the `sync` compose profile
- [x] 1.4 Verify `docker compose config` fails without `POSTGRES_PASSWORD`
- [x] 1.5 Verify `--profile sync` is the only way `sync` appears in `config --services`

## 2. Secrets

- [x] 2.1 `scripts/gen-prod-env.sh` — random secrets, mode 600, refuses overwrite
- [x] 2.2 `.env.example` documents `POSTGRES_PASSWORD` and points production at the generator
- [x] 2.3 Confirm `zitadel-machine-key.json` is gitignored and in no commit

## 3. Deploy pipeline

- [x] 3.1 `.github/workflows/deploy-prod.yml` targeting the `syndra-prod` runner
- [x] 3.2 Local smoke test (`/healthz`, UI root)
- [x] 3.3 Reachability assertion against `SYNDRA_EXTERNAL_URL`
- [x] 3.4 Prune builder cache, not only images
- [ ] 3.5 Install and start the runner service on `198.51.100.12` *(blocked: needs a registration token)*
- [ ] 3.6 Confirm a push to `main` deploys end to end

## 4. Production host

- [x] 4.1 `runner` user, `docker` group, `/opt/syndra` ownership
- [x] 4.2 Read-only deploy key generated, `github.com` host key pinned
- [ ] 4.3 Add the deploy key to the repository *(blocked: GitHub UI)*
- [ ] 4.4 Clone to `/opt/syndra` *(blocked on 4.3 and on the repository rename)*
- [ ] 4.5 Run `gen-prod-env.sh`, fill `ZITADEL_CLIENT_ID` / `ZITADEL_AUDIENCE` *(blocked on 4.4)*
- [x] 4.6 Copy `zitadel-machine-key.json` to the host at mode 600, owned by `runner`
- [ ] 4.7 `docker compose up -d --build`; confirm `[DIRECTORY] Source=zitadel`

## 5. Zitadel

- [x] 5.1 Project `Syndra`, role `admin`, granted to the operator
- [x] 5.2 Application `syndra`, PKCE, redirect URI `https://syndra.example.org/auth/callback`
- [ ] 5.3 Token Settings: Auth Token Type → **JWT**, "Add user roles to the access token" → **on**
- [ ] 5.4 Project setting: **Assert Roles on Authentication** → on
- [x] 5.5 Service user `syndra-service` with JSON key; org + instance permissions assigned
- [ ] 5.6 `make zitadel-actions-register`; capture both signing keys into `.env`
- [ ] 5.7 `make zitadel-actions-verify` and `zitadel-actions-verify-events`
- [ ] 5.8 Set `ZITADEL_M2M_USER_ID` from the key file's `userId`

## 6. Caddy

- [ ] 6.1 Site block on `198.51.100.15` splitting `/api/action/*` and `/api/webhooks/*` to `:8080`, rest to `:3000`
- [ ] 6.2 Confirm certificate issues and the OIDC redirect URI resolves via `x-forwarded-*`

## 7. Rename

- [x] 7.1 Bulk rename across tracked files, four casings, binaries excluded
- [x] 7.2 Go module paths (`syndra`, `syndra-sync`) and all imports
- [x] 7.3 `git mv` for `syndra-token`, `syndra-core-architecture`, `design_handoff_syndra_ia`
- [x] 7.4 Migration `000025` for the stored `drift_type` value, with a matching down
- [x] 7.5 `TestDriftMigrationEnumsMatchCode` reads every up-migration, not `000016` alone
- [x] 7.6 Verify no dangling references to renamed paths
- [ ] 7.7 Rename the GitHub repository; update `origin`
- [ ] 7.8 Rename the local working directory *(deferred: moves the session's cwd)*

## 8. Verification

- [x] 8.1 `backend`: build, vet, test
- [x] 8.2 `sync`: build, vet, test
- [x] 8.3 `ui`: 349 tests, lint, production build
- [ ] 8.4 Post-deploy: log in via Zitadel and confirm the `admin` role resolves
- [x] 8.5 Codebase-memory refresh after the rename — full re-index (6547 nodes, 14163 edges); the rename moved every Go module path, so an incremental pass would have left the graph pointing at symbols under the old module

## 9. Follow-ups (separate changes)

- [ ] 9.1 Second Zitadel instance for development
- [ ] 9.2 Rebuild `198.51.100.16` from `DEPLOY.md`
- [ ] 9.3 Fix `scripts/deploy-lxc.sh` (stale IP, `rm -rf` destroys `.env` and the machine key)
