# Add-on transport: one secret, derived keys

## Why

The add-on transport asks an operator to run a private certificate authority for
two containers built from one repository, deployed by one Compose file, onto one
host. That cost is paid **per add-on**, and TrueNAS is the first of four — UniFi
Access and the LLDAP replacement are named in the roadmap. Per target today:
a CA, a server certificate, a client certificate, two secret directories, an
expiry nobody tracks, and a rotation ceremony nobody has performed.

**The certificates are not what authenticates the channel, and the branch's own
argument for them is aimed at the wrong target.** `transport.go` rejects a shared
secret because "it authenticates the caller but binds nothing to the request, so
an intercepted call replays verbatim". That is true of a secret you *present*
and false of one you *sign with* — and signing with a shared secret is already
built, tested, and shipped here as signed mode. Authentication never needed a
PKI.

What genuinely needs the TLS is **confidentiality**, and the evidence is in the
contract: `capabilities.go` declares `secret_params: ["password"]` and
`["elevated_key"]`. A member's plaintext TrueNAS password and the elevated purge
credential cross that hop. Signing a body does not hide it.

So the transport is answering two different needs with one expensive mechanism.
Separating them lets the expensive half go: derive the add-on's TLS key from the
same secret that signs the requests, and the certificate stops being something
anyone distributes, renews, or stores.

## What Changes

- **BREAKING** Mutual TLS is removed as a transport mode. Every add-on is
  configured with exactly one value — a per-target shared secret — and both
  transport keys are derived from it. `ADDON_TRUENAS_CLIENT_CERT`,
  `ADDON_TRUENAS_CLIENT_KEY`, `ADDON_TRUENAS_CA_CERT`, `ADDON_CLIENT_CA_FILE`,
  `TLS_CERT_FILE` and `TLS_KEY_FILE` are retired.
- **BREAKING** The add-on derives an Ed25519 keypair via HKDF and self-signs an
  in-memory certificate at boot. Nothing is written to disk and no certificate
  outlives a restart.
- **BREAKING** The backend pins that exact public key in place of CA and
  hostname verification — strictly stronger than what it replaces, and the
  reason an active on-path attacker cannot reach the secret-bearing body.
- Signed requests become the only caller authentication, with the HMAC key
  derived from the same secret under a different `info` string. The MAC
  construction itself is unchanged.
- `addons/contract/transport_derivation.json` becomes a wire-contract artifact,
  asserted from both suites. The derivation is a new place where two separately
  deployed binaries must agree byte for byte, which is the defect class this
  branch keeps finding; the vector is what makes them agree.
- Each add-on moves to its own Compose network, so no add-on shares an L3
  segment with another.
- `scripts/gen-addon-certs.sh` is deleted, along with the certificate-expiry
  surfacing on target health — there is no longer an expiry to surface.
- `requireHTTPS` survives with its reason rewritten. Its current justification
  cites the client certificate and the private CA, neither of which will exist;
  the real reason is that the body carries declared secret parameters. The rule
  stays **unconditional** — registration happens before any add-on is contacted,
  so the check cannot consult a manifest and must not be specified as though it
  could.
- The backend gains the add-on's `secretValue` semantics, so the secret is
  configurable as a value or a `_FILE` path under the same name at both ends.
  The certificate mounts go; **a mount for the secret stays**, because an
  environment value is readable from `docker inspect` and `/proc/1/environ`.
- Duplicate secrets across targets are refused at start-up. The spec has always
  required distinctness; generating fresh values covers first configuration
  only, and nothing reports a duplicate reintroduced by a copied block or a
  rotation.

## Impact

- **Affected specs:** `addon-platform` (transport requirement modified; three
  requirements added)
- **Affected code:** `addons/truenas/{main,transport}.go`,
  `backend/internal/addons/{credentials,registry,transport}.go`,
  `docker-compose.yml`, `.env.example`, `DEPLOY.md`, `addons/contract/`
- **Operator impact:** `sudo ./scripts/gen-addon-secret.sh <target>` per target
  replaces the certificate ceremony, and works the same for a target added later
  — which `gen-prod-env.sh` cannot do, since it refuses to run once `.env`
  exists. It stays a **separate, explicitly privileged step**: the environment
  bootstrap runs as the unprivileged deploy user by design, and setting the
  secret's ownership needs privilege that user must not have.
  Rotation is: `POST /lifecycle` and wait for `drained`, change the value, then
  `docker compose up -d --force-recreate backend truenas-addon` — **recreate,
  not restart.** A restart preserves the environment a container was created
  with, so "change it and restart" would leave both ends on the old secret while
  appearing to have rotated. The drain must be the runtime call and must precede
  the change, because it travels over the transport being rotated.

## Found while specifying this, and moved out of it

`docker-compose.yml` sets `stop_grace_period` nowhere, so Docker's 10-second
default kills the add-on before its own 20-second shutdown drain completes — the
drain that exists so an in-flight mutation settles rather than being abandoned
half-applied. That is a live defect on the current branch, not one this change
introduces.

It is **not in this change**. An earlier draft called it separable in prose while
carrying it as a task and a requirement, which made it part of this change for
any implementation workflow — scope stated one way and encoded another. It now
has its own: [`addon-shutdown-grace-period`](../addon-shutdown-grace-period/).
Nothing here depends on it, because the rotation procedure quiesces through the
runtime lifecycle operation before anything is stopped.
- **Not affected:** the add-on → TrueNAS leg. That is a separate connection with
  separate reasoning (`TRUENAS_URL`, `TRUENAS_VERIFY_TLS`) and this change does
  not touch it.

## Sequencing

This deletes a reviewed authentication path. It should land **after** the first
live TrueNAS bring-up completes on the transport that has already been reviewed
(`NEXT.md` §4a), so that a NAS-side failure and a transport-side failure are
never being diagnosed at the same time.
