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
