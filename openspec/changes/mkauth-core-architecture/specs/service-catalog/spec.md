# Requirement: Service Catalog (User Portal)

Standard users MUST have a simplified, high-level view of the ecosystem focused on Applications/Services rather than underlying Zitadel roles.

## Service Discovery
The portal MUST present a gallery of all "Published" applications available in the MkAuth ecosystem.

### Scenario: Browsing helpdesk access
- **GIVEN** a standard user logs into the MkAuth Portal
- **THEN** they MUST see a card for the "Helpdesk System" including its description and icon.
- **AND** the card MUST display the user's current status for that service: `Active`, `Pending`, or `No Access`.

## Requesting a Service
When a user requests a service, the system MUST automatically map that request to the required technical permissions (Bundles/Roles).

### Scenario: Requesting entry to a lab
- **GIVEN** the "Laser Lab" service is associated with the `laser_lab_basic` Bundle
- **WHEN** a user clicks "Request Access" on the Laser Lab card
- **THEN** MkAuth MUST initiate an `AccessRequest` for the `laser_lab_basic` Bundle.
- **AND** the user MUST be prompted for a justification.

## Service Access Abstraction
Users MUST NOT see the "Raw Roles" that make up a service unless they have `admin` privileges.

### Scenario: Viewing my services
- **WHEN** a user views their "Active Services"
- **THEN** the system MUST show the Application name (e.g., `Printing Portal`)
- **AND** hide technical role details (e.g., `zitadel:project:234:member`).

## Infrastructure Credentials
Standard users MUST be able to manage legacy credentials (e.g., Samba Passwords) directly through the portal, but only when they possess the relevant service access.

The portal MUST present these secrets as infrastructure-specific credentials rather than as the user's primary account password.

### Scenario: Managing Samba passwords
- **GIVEN** a user has active access to a service that requires an LLDAP identity (e.g., "Makerspace File Share")
- **WHEN** they visit the MkAuth Portal
- **THEN** a "Infrastructure Credentials" section MUST appear.
- **AND** it MUST include a card for "Samba/LDAP Password" with a "Set Password" action.

### Scenario: Infrastructure credential labeling
- **WHEN** a user opens the Samba password management flow
- **THEN** the UI MUST describe the secret as an infrastructure-specific credential for Samba/LLDAP access
- **AND** it MUST NOT imply that this password changes the user's primary Zitadel or OIDC login

### Scenario: Restricted access to credentials
- **GIVEN** a user has NO access to any LDAP-enabled services
- **WHEN** they visit the MkAuth Portal
- **THEN** the "Infrastructure Credentials" section MUST be completely hidden to prevent confusion.


### Requirement: Service request contract stays aligned
The system MUST keep the documented service-request abstraction aligned with the implemented backend contract and clearly mark any temporary fallback behavior.

#### Scenario: Temporary project-role fallback
- **WHEN** the portal still submits requests through a lower-level project/role contract instead of a dedicated service-mapping contract
- **THEN** the implementation status MUST be documented as partial
- **AND** the follow-up hardening work MUST define the intended service-level contract explicitly
