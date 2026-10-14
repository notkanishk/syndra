## ADDED Requirements

### Requirement: A password is accepted only when something will receive it
`POST /me/targets/{t}/credential` SHALL refuse before dispatch with `409 TARGET_PAUSED` when the target's lifecycle is `draining` or `read_only`, and with `503 TARGET_UNREACHABLE` when it is not answering. The member page SHALL render no password field in either state and SHALL NOT promise to apply a password later.

#### Scenario: Paused system
- **WHEN** a member submits a password while the target is draining
- **THEN** the request is refused with `409 TARGET_PAUSED` and the add-on receives nothing

### Requirement: The page claims only what was read
`GET /me/targets` items SHALL carry `checked_at` and `reachable_since` from the health monitor. The page SHALL show "Offline" only for a confirmed outage, SHALL show no status when Syndra can't tell, and SHALL never render usage as 0 while offline.

#### Scenario: Offline usage
- **WHEN** the target is confirmed offline
- **THEN** each share reads "Usage unavailable while offline"

### Requirement: Where a member connects is what their groups open
`GET /me/targets` storage SHALL carry `places`, read by the add-on from the folder ACLs for the groups the account is actually in, home folder first. Each place has both addresses, built once from the connection host. Usage SHALL appear only on places the member can change: 0 when they have saved nothing, absent when it couldn't be read. `places_readable: false` SHALL mean not read, never none.

#### Scenario: A group that opens nothing
- **WHEN** the member's groups are granted nothing beyond traverse
- **THEN** `places` is empty and `places_readable` is true, and the page says no folder is open to them yet

### Requirement: Access given back asks for a new password
When an account has had a password, SMB is wanted and SMB is off, storage SHALL report `needs_new_password`, the account SHALL NOT be reported usable, and the view SHALL carry `restored_at`, the latest end of a deny hold on that target.

#### Scenario: Hold lifted on 12 Oct
- **WHEN** access taken away is given back on 12 Oct
- **THEN** the page asks for a new password and says access was given back on 12 Oct
