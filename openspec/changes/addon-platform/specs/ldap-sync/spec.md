## REMOVED Requirements

### Requirement: Group Flattening & Translation

**Reason**: The LLDAP bridge is removed entirely. TrueNAS SCALE exposes a management API that takes group membership directly, so there is no directory to flatten `{project}_{role}` names into and no translation layer to maintain.

**Migration**: Role-to-target mappings replace the flattening convention. An operator binds a Zitadel role to a pre-existing TrueNAS group by name in `target_role_mappings`; the add-on resolves that name to a gid at runtime. Existing flattened LLDAP group names have no equivalent and are not carried over.

#### Scenario: No flattened group names are produced

- **WHEN** a subject's entitlements are resolved for a target
- **THEN** the resolved set MUST contain values drawn from role-to-target mappings
- **AND** MUST NOT contain names synthesised from a project and role pair

### Requirement: Shadow Password Management

**Reason**: The vault stored Argon2id hashes on the assumption that a directory would accept pre-hashed credentials. TrueNAS accepts plaintext only and exposes no hash-write path, so a stored hash can never be propagated anywhere.

**Migration**: Members set their own credential, which is forwarded to the target and never persisted. The vault retains existence and rotation metadata so drift reporting still works. **Every enrolled member must set a new credential after cutover** — the stored hashes cannot be converted, and there is no automated path.

#### Scenario: No credential hash is stored

- **WHEN** a member sets their infrastructure credential
- **THEN** the backend MUST record existence and rotation metadata only
- **AND** MUST NOT store any hash or derivation of the credential

### Requirement: Sync Policy

**Reason**: Sync policy described what a polling LLDAP worker reflected and when. The worker is deleted; targets are converged from resolved desired state through the outbox and the add-on contract.

**Migration**: The entitlement plane replaces it. What reaches a target is the resolved entitlement set; when it reaches the target is governed by the propagation drain rules in `access-governance`.

#### Scenario: Convergence replaces reflection

- **WHEN** a subject's entitlements change
- **THEN** the backend MUST record a desired-state snapshot and an outbox row
- **AND** no polling worker MUST be involved in applying it

### Requirement: LLDAP Reconciliation Loop

**Reason**: Never implemented, and the system it would have reconciled against is removed.

**Migration**: Per-target drift sweep and triage, which existed for Zitadel and now carries a target dimension.

#### Scenario: Reconciliation is per target

- **WHEN** the drift sweep runs
- **THEN** it MUST reconcile each registered target against expected state
- **AND** no LLDAP-specific reconciliation path MUST remain
