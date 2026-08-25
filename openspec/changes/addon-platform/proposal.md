# Add-on Platform

## Why

Phase 4 shipped the LLDAP bridge and stalled: password propagation was never proven, and the sync service still sits behind an opt-in Compose profile. The premise was wrong. TrueNAS SCALE and UniFi Access both expose first-class management APIs, so routing them through an LDAP directory adds a system, loses fidelity, and blocks on a compatibility question direct APIs make irrelevant.

Add-ons also avoid duplication. Syndra already owns outbox-before-mutation, drift sweep, cascade, expiry, lineage, and versioned policy — target-agnostic in all but naming. Extending them over a `target` dimension beats teaching a second service to repeat them.

## What Changes

- **BREAKING** Remove `sync/`, the provisioning-intent pipeline, `go-ldap/v3`, and the group-flattening convention. Phase 4's LLDAP Integration item is abandoned, not deferred.
- **BREAKING** The vault stops storing Argon2id hashes — TrueNAS accepts plaintext only. Credentials are forwarded, never persisted; only existence and rotation metadata remain. **Every enrolled member must re-enrol.**
- **BREAKING** Rename and reshape the propagation outbox, and add a target dimension to it and the drift tables, backed by a `targets` registry with state rather than a CHECK. Target-specific intent moves behind versioned desired-state snapshots applied in order per subject.
- **BREAKING** Plan-then-apply becomes a backend guarantee on every path, Zitadel included: rehearsals persist under a plan id and applies cite it, instead of the apply recomputing the plan from a re-submitted request. Covers the four bulk and drift-triage surfaces.
- Add an add-on registry and wire contract. Add-ons are separate mutually-authenticated containers on the internal network, declaring an entitlement schema and operation set via a manifest that can only narrow what backend policy permits.
- Ship the TrueNAS SCALE add-on: member credential and mount instructions; account lifecycle (create on role grant, reversible disable, rotation, purge); observability (SMB activity, NAS health, drift).
- Add Syndra allowances — explicit per-user overlays beside role-derived access. Subtractive ones must carry an expiry or a review date.
- **BREAKING** Narrow the operator-only drain rule to grants: revocations gain a background drain, and every apply surface states which rule applied. Add a queued-revoke surface with age escalation and a dormant-account housekeeping view.

## Capabilities

### New Capabilities
- `addon-platform`: target-adapter contract, registry, manifest schema, per-target propagation and drift, fail-open queueing
- `truenas-addon`: TrueNAS SCALE behaviour — identity translation, entitlement application, operations, safety guards

### Removed Capabilities
- `ldap-sync`: the LLDAP bridge and its shadow-hash vault, removed entirely
- `provisioning`: the LLDAP intent pipeline, replaced by target-dimensioned propagation

### Modified Capabilities
- `access-governance`: plan-then-apply as a backend guarantee; allowances as a third access band; drain rule narrowed to grants
- `user-management`: member credential and mount surfaces; lifecycle from role grants; dormant-account housekeeping
- `operational-readiness`: add-on health, reachability, and lifecycle state reporting

## Impact

**Roadmap phase:** Phase 4, redefined.

**Control Plane:** grows a target dimension; policy, audit, and mutation authority stay in the backend. **Data Plane:** unchanged. **Bridge Plane:** replaced by per-target add-on containers.

**Code:** `services/{propagation,drift,expiry}`, `handlers/{bulk,drift,drift_rehearsal,requests_bulk,vault,intents}`, migrations, `docker-compose.yml`, `ui/src/lib/nav.ts`. Deletes `sync/` and the intent pipeline.
