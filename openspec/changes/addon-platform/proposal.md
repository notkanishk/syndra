# Add-on Platform

## Why

Phase 4 shipped the LLDAP bridge and stalled: password propagation was never proven, and the sync service still sits behind an opt-in Compose profile. The premise was wrong. The equipment Syndra needs to reach — TrueNAS SCALE for lab storage, UniFi Access for doors — both expose first-class management APIs. Routing them through an LDAP directory adds a system, loses fidelity, and blocks on a compatibility question that direct APIs make irrelevant.

Replacing the bridge with target add-ons also avoids duplication. Syndra already owns outbox-before-mutation, drift sweep, cascade, expiry, lineage, and versioned policy — target-agnostic in everything but naming. Extending them over a `target` dimension beats teaching a second service to repeat them.

## What Changes

- **BREAKING** Remove `sync/`, `services/lldap.go`, `go-ldap/v3`, and the LLDAP group-flattening convention. Phase 4's LLDAP Integration item is abandoned, not deferred.
- **BREAKING** The shadow-password vault stops storing Argon2id hashes. TrueNAS accepts plaintext only, so member-set passwords are forwarded and never persisted; the vault retains existence and rotation metadata for drift.
- Add a target dimension to `pending_zitadel_propagations`, `direct_role_grants`, and the drift tables, with a registry table rather than a CHECK. One drain loop, one sweep, filtered by target, with versioned desired-state snapshots applied in order per subject.
- **BREAKING** Plan-then-apply becomes a backend guarantee on every path, Zitadel included: the backend issues and holds the plan, and bulk, drift-triage, and reconciliation applies cite a plan identifier instead of returning a plan body.
- Add an add-on registry and wire contract. Add-ons are separate containers on the internal network, mutually authenticated, declaring an entitlement schema and an operation set via a manifest that can only narrow what backend policy already permits.
- Ship the TrueNAS SCALE add-on: member password and mount instructions; account lifecycle (auto-create on role grant, SMB suspend, lock, purge); observability (SMB activity, NAS health, drift).
- Add Syndra allowances — explicit per-user overlays alongside role-derived access. Subtractive allowances are time-boxed suspensions and must carry an expiry.
- Add a queued-revoke surface with age escalation, and a dormant-account housekeeping view with bulk actions.
- Every add-on operation is dry-runnable through the existing `BulkPlan`/`BulkOutcome` rehearse-then-apply pattern.

## Capabilities

### New Capabilities
- `addon-platform`: the target-adapter contract, add-on registry, manifest schema, per-target propagation and drift, fail-open queueing semantics
- `truenas-addon`: TrueNAS SCALE target behavior — identity translation, entitlement application, operations, safety guards

### Modified Capabilities
- `access-governance`: allowances as a third access band beside source and derived; queued-revoke escalation
- `user-management`: member self-service credential and mount surfaces; account lifecycle driven by role grants; dormant-account housekeeping
- `operational-readiness`: add-on health and reachability reporting; dry-run required on every target-affecting operation

## Impact

**Roadmap phase:** Phase 4, redefined. Phases 5 and 6 are unaffected.

**Control Plane:** grows a target dimension. Policy, audit, and mutation authority stay in the backend.

**Data Plane:** unchanged. Actions v2 remains the only claim-integration path.

**Bridge Plane:** replaced. The LLDAP sync worker becomes a set of per-target add-on containers.

**Code:** `backend/internal/services/{propagation,drift,expiry}`, `handlers/{drift_rehearsal,reconciliation,vault}`, migrations, `docker-compose.yml`, `ui/src/lib/nav.ts`. Deletes `sync/`.
