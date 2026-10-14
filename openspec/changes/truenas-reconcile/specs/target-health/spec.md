## ADDED Requirements

### Requirement: Offline is confirmed, never inferred from one read
Syndra SHALL probe every active add-on target on a fixed interval and SHALL report a target offline only after 3 consecutive failed probes. `reachable_since` SHALL be the time of the first failure in that run, and `checked_at` the time of the latest probe. Member and operator pages SHALL read the same monitor.

#### Scenario: One slow answer stays green
- **WHEN** one probe fails and the next succeeds
- **THEN** the target is never reported offline and `reachable_since` is cleared

#### Scenario: Three failures confirm it
- **WHEN** three consecutive probes fail, the first at 09:40
- **THEN** the target is reported offline with `reachable_since` 09:40

#### Scenario: After a restart Syndra can't tell
- **WHEN** no probe has completed since the process started
- **THEN** the state is `unknown` and no page claims online or offline
