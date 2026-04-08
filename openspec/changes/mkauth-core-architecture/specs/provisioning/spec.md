# Requirement: Identity Provisioning Engine

The MkAuth Provisioning Engine (Sync Service) is responsible for the physical translation of identity state between the MkAuth Control Plane and external systems.

## Event-Driven Synchronization
The engine MUST prioritize rapid updates to ensure security parity between Zitadel and downstream hardware.

### Scenario: Real-time role revocation
- **WHEN** a webhook is received indicating a user has been removed from a "Samba" role
- **THEN** the Provisioning Engine MUST immediately invalidate that user's LLDAP group membership.

## Fault Tolerance & Retries
The provisioning engine MUST maintain consistency even during transient network failures between the Sync Service and LLDAP.

### Scenario: Recovering from LLDAP downtime
- **GIVEN** the LLDAP server is offline
- **WHEN** MkAuth attempts to sync a new password
- **THEN** the request MUST be queued with an exponential backoff retry strategy.
- **AND** the MkAuth dashboard MUST surface a "Sync Degraded" warning to administrators.

## Security Boundaries
- **Credentials**: The Sync Service MUST use a dedicated LLDAP service account with permissions limited specifically to the User and Group OUs it manages.
- **Data Perimeter**: Shadow passwords MUST NEVER be stored in plain text in any MkAuth database or cache; they must only exist as salted hashes or be transmitted via secure channels during the rotation event.
