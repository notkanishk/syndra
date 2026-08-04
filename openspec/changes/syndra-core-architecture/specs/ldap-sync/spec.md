> **Status:** Partial (reconciliation deferred P5, password compat unresolved) | [< Index](../../../../INDEX.md) | [Feature Coverage](../feature-coverage.md)

# Requirement: LLDAP Sync & Group Mapping

The system MUST provide a reliable synchronization mechanism to reflect Zitadel identity state into an LLDAP server for use by legacy protocols (Samba, UniFi).

## Group Flattening & Translation
Since LLDAP lacks Zitadel's project-based hierarchy, Syndra MUST translate roles into a flattened group namespace.

### Scenario: Mapping a Samba role
- **GIVEN** a Zitadel role `share_admin` in project `Samba`
- **WHEN** the "Sync to LLDAP" toggle is enabled for this role
- **THEN** the Sync Service MUST create/manage a group in LLDAP named `samba_share_admin`.
- **AND** all users with that Zitadel role MUST be members of that LLDAP group.

## Shadow Password Management
To enable Samba authentication for OIDC-based users, Syndra MUST manage a secondary "Shadow Password" specifically for LLDAP.

This secondary credential is a required infrastructure bridge for legacy makerspace systems and MUST NOT be treated as a general-purpose identity credential.

### Scenario: Setting a Samba password
- **WHEN** a user sets a password in the Syndra portal for "Samba Access"
- **THEN** the system MUST enforce complexity requirements:
    - Minimum 12 characters.
    - Diversity of character sets (Upper, Lower, Numeric, Symbol).
    - Entropy check to prevent common patterns.
- **AND** the system MUST use a password propagation mechanism that is proven compatible with the target LLDAP deployment.

### Scenario: Compatibility research before password sync rollout
- **GIVEN** Syndra stores an infrastructure-only shadow credential for LLDAP-backed services
- **WHEN** the team prepares to enable end-to-end password synchronization into a real LLDAP deployment
- **THEN** the team MUST first verify that the target LLDAP server supports the intended password update flow and hash semantics
- **AND** Syndra MUST NOT assume that writing a pre-hashed credential through a plain LDAP attribute update is valid until that compatibility is confirmed
- **AND** password-sync rollout MAY remain paused while the rest of Syndra development continues

### Scenario: Infrastructure-only credential boundary
- **WHEN** Syndra stores, displays, or transmits a Samba/LLDAP password
- **THEN** that credential MUST be handled as an infrastructure-only secret
- **AND** the system MUST NOT present it as the user's primary account password or identity credential

## Sync Policy
- **One-Way Authority**: Zitadel/Syndra remains the absolute source of truth. Manual changes in LLDAP MUST be overwritten during the next sync cycle.
- **Membership Cleanup**: When a user loses a role in Zitadel, they MUST be removed from the corresponding LLDAP group within a configurable sync window (e.g., 5 minutes).

## LLDAP Reconciliation Loop
The system MUST periodically verify that LLDAP group memberships match Syndra's authoritative provisioning state, independent of the event-driven intent pipeline.

### Scenario: Drift detected in LLDAP
- **WHEN** a reconciliation cycle finds LLDAP memberships that do not match the expected state derived from Syndra grants and mapping rules
- **THEN** the system MUST correct LLDAP to match Syndra's authoritative state
- **AND** log the drift for operator review

### Scenario: Missed intent recovery
- **WHEN** the sync service missed intents due to a crash or network partition
- **THEN** the reconciliation loop MUST detect and correct the resulting drift without requiring manual intervention

> **Status:** Deferred to Phase 5. The sync service currently operates on an event-driven intent model with no periodic full-sync.
