> **Status:** Production environment delta — deployment surface | [< Index](../../../../INDEX.md)

# Requirement: Operational Readiness (delta)

## ADDED Requirements

### Requirement: A deployment MUST NOT start with a credential it did not choose

`docker-compose.yml` MUST NOT supply a default value for `POSTGRES_PASSWORD`. It
MUST be declared required (`${POSTGRES_PASSWORD:?...}`) so that Compose refuses
to start when it is absent, and `DB_DSN` MUST derive its default from that same
variable rather than repeating a literal.

A literal default meant every deployment that did not override it shared one
credential, and the `DB_DSN` default silently agreed with that credential, so
nothing anywhere reported a problem. A missing value MUST be a startup error, not
an inherited secret.

#### Scenario: Compose refuses to start without an explicit password

- **WHEN** `docker compose config` runs with no `POSTGRES_PASSWORD` in the environment or `.env`
- **THEN** it MUST exit non-zero with an error naming `POSTGRES_PASSWORD`
- **AND** no service definition MUST be emitted

#### Scenario: The credential has exactly one definition

- **WHEN** `POSTGRES_PASSWORD` is set and `DB_DSN` is not
- **THEN** the backend's `DB_DSN` MUST resolve to a DSN containing that password
- **AND** changing `POSTGRES_PASSWORD` alone MUST change the resolved `DB_DSN`

### Requirement: Session cookie integrity MUST be separable from the backend API key

The `ui` service MUST receive `SESSION_SECRET`. The UI falls back to
`SYNDRA_API_KEY` when it is unset, which coupled two unrelated rotations: changing
the backend API key invalidated every operator's session cookie with no warning
and no obvious cause. Production `.env` generation MUST always write a distinct
`SESSION_SECRET`.

#### Scenario: Rotating the API key does not log operators out

- **WHEN** a deployment has distinct `SESSION_SECRET` and `SYNDRA_API_KEY` values
- **AND** `SYNDRA_API_KEY` is rotated and the backend restarted
- **THEN** existing session cookies MUST remain valid

### Requirement: A service that cannot run MUST NOT be started by default

The `sync` service MUST be gated behind a Compose profile. Without a reachable
LLDAP it crash-loops, and a permanently restarting container in `docker ps`
trains an operator to read `Restarting` as normal — which is how a real failure
goes unnoticed.

#### Scenario: sync is absent from the default service set

- **WHEN** `docker compose config --services` runs with no profile
- **THEN** `sync` MUST NOT appear
- **AND** `docker compose --profile sync config --services` MUST include it

### Requirement: Production secret generation MUST refuse to overwrite

`scripts/gen-prod-env.sh` MUST exit non-zero without writing when `.env` already
exists. A regenerated `POSTGRES_PASSWORD` no longer matches an
already-initialized Postgres volume, so an accidental re-run would lock the
backend out of its own data while reporting success. Generated files MUST be mode
600.

#### Scenario: Re-running the generator is inert

- **WHEN** `.env` exists and `scripts/gen-prod-env.sh` runs
- **THEN** it MUST exit non-zero
- **AND** the existing `.env` MUST be byte-identical afterwards

### Requirement: A deploy MUST verify that Zitadel's configured target URL reaches the deployment

The deploy pipeline MUST POST to `${SYNDRA_EXTERNAL_URL}/api/action/inject` and
fail when the request cannot connect. Reaching the handler and being rejected is
the passing condition — the assertion is about routing, not authorization.

`SYNDRA_EXTERNAL_URL` is the address *Zitadel* uses, so it is never exercised by
local health checks. On the first deployment it drifted to a decommissioned IP
and every Action posted into the void for months, undetectable from any Syndra
log because the failure occurred entirely on the caller's side.

#### Scenario: A deploy fails when the public path does not route

- **WHEN** the configured `SYNDRA_EXTERNAL_URL` does not resolve to the running deployment
- **THEN** the deploy job MUST fail
- **AND** the failure message MUST distinguish "unreachable" from "handler errored"

#### Scenario: An unsigned request is a passing result

- **WHEN** the target URL routes correctly and signing keys are configured
- **THEN** an unsigned POST MUST return a 4xx status
- **AND** the deploy job MUST treat that as success

### Requirement: Migration-coherence guards MUST read the effective schema

A guard asserting that code-written enum values are permitted by a CHECK
constraint MUST read every `*.up.sql` migration, not the single migration that
introduced the constraint. Constraints move: the MkAuth → Syndra rename relocated
the `drift_type` values into `000025`. A guard pinned to `000016` would assert
what the schema used to be and pass while the running database disagreed with the
code — the precise failure the guard exists to catch.

#### Scenario: A constraint relocated by a later migration is still seen

- **WHEN** a CHECK constraint is dropped and re-added with new values in a later migration
- **AND** the Go layer writes one of the new values
- **THEN** `TestDriftMigrationEnumsMatchCode` MUST pass
- **AND** it MUST fail if the Go layer writes a value no migration permits

### Requirement: Every browser-facing absolute URL MUST be derived from the forwarded origin

Behind a reverse proxy the UI never observes the URL the browser requested.
Next's standalone server builds `request.url` from the address it listens on, so
every request arrives as `http://localhost:3000/...`. Any redirect target,
OIDC `redirect_uri`, or cookie `secure` decision MUST therefore be derived from
`x-forwarded-host` / `x-forwarded-proto`, falling back to `Host`, and MUST be
built as a fresh origin rather than by mutating the incoming URL.

Mutating it is not equivalent: the WHATWG `host` setter replaces the port only
when the assigned value carries one, so a bare hostname assigned over a
`:3000` request URL silently keeps the port.

This logic MUST live in exactly one module. It previously existed as three
byte-identical copies plus two variants that omitted the headers entirely, and
fixing the copy named by an error message left the other five unfixed.

#### Scenario: A proxied redirect does not carry the container's port

- **WHEN** a request arrives with `request.url` of `http://localhost:3000/users` and `x-forwarded-host: syndra.example.org`, `x-forwarded-proto: https`
- **THEN** any redirect issued MUST target `https://syndra.example.org/...`
- **AND** MUST NOT contain the listening port

#### Scenario: A forwarded port is preserved

- **WHEN** `x-forwarded-host` is `syndra.internal:8443`
- **THEN** the resolved origin MUST be `https://syndra.internal:8443`

#### Scenario: Session cookies are Secure behind a TLS-terminating proxy

- **WHEN** TLS terminates at the proxy and the request reaching the app is plain HTTP
- **AND** `x-forwarded-proto` is `https`
- **THEN** the session cookie MUST be set with `secure: true`
- **AND** the app MUST NOT decide this from the protocol of the request it received
