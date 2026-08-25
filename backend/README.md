# backend

The control plane. Go API that evaluates policy and is the **single mutation
authority** — the frontend signals intent, this decides.

```bash
go run ./cmd/api        # :8080, runs migrations on startup
go test ./... && go vet ./...
```

Configuration comes from the environment; every variable is documented in
[`.env.example`](../.env.example).

## Layout

| Path | Responsibility |
|---|---|
| `cmd/api/` | Entry point. Wires dependencies, enforces the production signing-key guard |
| `cmd/syndra-token/` | Mints an M2M access token from the Zitadel machine key. Used by the Actions registration scripts, not at runtime |
| `internal/handlers/` | HTTP layer. Strict JSON decoding on every mutation endpoint |
| `internal/services/` | Policy engine — rule evaluation, bundle publishing, cascades, drift triage, entitlement rehearsal |
| `internal/services/merge/` | The three-way merge classifier. A pure function over ours/theirs/base, guarded against ever learning a target's name |
| `internal/addons/` | The target plane's half of the backend: registry, capability manifests, signed transport, operation policy, dispatch |
| `internal/repoguard/` | Tests, not code. Reads the tracked working tree and fails on anything specific to one deployment |
| `internal/claims/` | Token shaping. **The only shaper**, applied on read by both the Actions v2 handler and the simulator |
| `internal/zitadel/` | Management API client. `StatusError` carries the upstream HTTP status |
| `internal/directory/` | Reads users, projects, and roles. Flips between the demo catalog and live Zitadel based on configuration |
| `internal/auth/` | Token validation, role resolution, the internal API key path |
| `internal/cache/` | Redis — precompiled flat roles for the data plane |
| `internal/db/` | Connection, migrations, query layer |
| `internal/models/` | Shared types |
| `internal/demo/`, `internal/seed/` | Local-dev catalog. Bypassed entirely when a live Zitadel is configured |
| `db/migrations/` | golang-migrate, numbered, up and down |

## Things worth knowing before changing anything

**`internal/claims` is the only place token shape is decided.** Both the Actions
v2 handler and the simulator call into it on read. If a preview is computed by
different code than the token it previews, the preview is worthless — that
constraint is why the package exists.

**Mutations leave a trace before the Management API call.** Direct grants write
an intent ledger entry; bundle and rule cascades write to the outbox. This
ordering is deliberate: a Zitadel-side change with no matching trace is how
drift gets *detected* rather than silently absorbed. Do not reorder it for
convenience.

**The production guard is load-bearing.** `requireProductionSigningKeys` in
`cmd/api/main.go` returns early only when `ZITADEL_DOMAIN` is empty. With a live
Zitadel configured and no signing keys, the backend refuses to start rather than
serving unauthenticated Action endpoints.

**Dependencies are injected via `deps.go` files** so handlers and services are
testable without a live database or Zitadel.

**The `db` package has no live-database harness.** Its tests are
migration-coherence guards — they read every `*.up.sql` and assert the schema
permits what the Go layer writes. Pin such a guard to a single migration and it
will assert what the schema *used to be*.
