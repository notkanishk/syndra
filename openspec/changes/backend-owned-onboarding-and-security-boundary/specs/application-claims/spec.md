## MODIFIED Requirements

### Requirement: Production degraded behavior for claim injection
The system MUST define a documented production failure posture for every application that depends on the Actions v2-compatible claim path.

#### Scenario: Application-specific degraded posture configured
- **WHEN** an application is configured for claim shaping
- **THEN** MkAuth MUST require a documented degraded-mode posture for cache miss, timeout, malformed cache data, or unavailable dependencies
- **AND** the allowed posture MUST be either fail-closed or an explicitly documented minimal safe fallback

#### Scenario: Implicit fallback prohibited
- **WHEN** the production claim path encounters a failure condition and no degraded-mode posture was configured
- **THEN** the configuration MUST be treated as incomplete
- **AND** the system MUST reject or block that production claim path until the posture is explicitly defined
