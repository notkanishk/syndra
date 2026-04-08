## Orchestrated Sync Flow
The Sync Service MUST operate strictly as a downstream worker for the MkAuth Backend.

### Internal Hand-off Pattern
- **Event Entry**: The MkAuth Backend receives Zitadel webhooks, parses the event, and determines if an LLDAP change is required.
- **Provisioning Intent**: If sync is required, the Backend MUST send a push-notification (Intent) to the Sync Service.
- **Contract**: The Intent MUST contain the target `UID`, the `Action` (Add/Remove), the `LLDAP_Group`, and optional sensitive payloads (e.g., Shadow Password Hash).

### Connectivity & Isolation
- **Private Access ONLY**: The Sync Service MUST be configured to only accept incoming traffic from the MkAuth Backend IP/Service Name.
- **No Public Interface**: All Zitadel webhook verification logic (HMAC/Cert validation) MUST reside in the Backend, shielding the Sync Service from external traffic.
The Sync Service MUST utilize standard Go idioms to ensure high reliability and operational simplicity.

### Concurrent Event Loop
The service MUST implement a concurrent sync loop using **Go Channels**.
- **Buffered Channels**: High-volume webhooks from Zitadel MUST be pushed into a buffered channel to prevent blocking the HTTP listener.
- **Worker Pool**: A pool of goroutines MUST consume from this channel to execute LDAP mutations in parallel, strictly maintaining order per UID to avoid race conditions on group membership.

### LLDAP Client Integration
The service MUST utilize **`go-ldap/ldap/v3`** for all directory operations.
- **Persistent Connections**: The service MUST maintain a pool of long-lived LDAPS connections with automatic re-bind logic on disconnect.
- **Transactional Safety**: LDAP updates MUST be performed using `ModifyRequest` to ensure atomic attribute updates (e.g., membership additions/removals).

## Security Boundaries
- **Credentials**: The Sync Service MUST use a dedicated LLDAP service account with permissions limited specifically to the User and Group OUs it manages.
- **Data Perimeter**: Shadow passwords MUST NEVER be stored in plain text. During the "Transit-to-LDAP" phase, passwords MUST be handled in memory as sensitive bytes and zeroed out immediately after the LDAP `PasswordModify` request is processed.
