# sync

The bridge plane. A standalone Go worker that reflects identity state into
LLDAP, for makerspace equipment that speaks LDAP and nothing else — Samba,
UniFi, door controllers.

```bash
go run ./cmd/sync
go test ./... && go vet ./...
```

**It is opt-in.** In `docker-compose.yml` this service sits behind the `sync`
profile and does not start by default:

```bash
docker compose --profile sync up -d
```

That gate exists because without a reachable LLDAP the worker crash-loops, and a
permanently restarting container teaches operators to read `Restarting` as
normal — which is how a real failure goes unnoticed.

## Layout

| Path | Responsibility |
|---|---|
| `internal/worker/` | Polling loop, retry with exponential backoff |
| `internal/backend/` | Client for the Syndra backend — fetches provisioning intents |
| `internal/ldap/` | LLDAP operations via `go-ldap/v3` |
| `internal/config/` | Environment configuration. The canonical definition of every `SYNC_*` and `LLDAP_*` variable |

## Design constraints

**No exposed ports.** The worker reacts only to verified intents pulled from the
backend. It is not reachable from outside the Docker network and does not
listen.

**It never talks to Zitadel.** Zitadel webhooks are received by the *backend*,
which validates them against policy and emits an intent. Keeping this worker
downstream of that decision is what keeps LDAP provisioning auditable.

**Password handling is isolated by design.** Some equipment authenticates only
by password, so Syndra maintains shadow credentials as an infrastructure bridge.
That path is deliberately separate from the OIDC identity flow, independently
auditable, and scoped to infrastructure access alone. Do not merge it into the
general identity model.

**LLDAP may live anywhere reachable.** It is not part of this compose stack in
production — the worker only needs `LLDAP_URL` to resolve.

## Status

Partial. Reconciliation and compensating revocations are deferred; password
compatibility is unresolved. See
[`openspec/NEXT.md`](../openspec/NEXT.md) for what that means concretely.
