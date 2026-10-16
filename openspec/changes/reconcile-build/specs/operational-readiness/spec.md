# Delta: operational-readiness

## MODIFIED Requirements

### Requirement: System mode reports whether Zitadel is answering
`GET /api/v1/system/mode` MUST report the active directory source, whether Zitadel is configured, whether it is answering, and when it last answered a directory read. Syndra MUST NOT show sample data in place of Zitadel (N23).

#### Scenario: Zitadel stops answering after a good read
- **WHEN** Zitadel is configured and a directory read fails after an earlier one succeeded
- **THEN** reads MUST return the last good read, and `/system/mode` MUST return `degraded: true` with `last_read_at` set to when Zitadel last answered
- **AND** the UI MUST show the warn Notice "Zitadel isn't answering. This is what it held at {time}." above the page, not dimmed

#### Scenario: Zitadel has never been read
- **WHEN** Zitadel is configured and no directory read has ever succeeded (including a management key that won't load)
- **THEN** directory reads MUST return `503 DIRECTORY_UNREACHABLE` with no rows, and `/system/mode` MUST return `degraded: true`, `last_read_at: null`
- **AND** lists MUST render no rows, skeleton or empty state

#### Scenario: Demo only without Zitadel
- **WHEN** `ZITADEL_DOMAIN` is unset
- **THEN** the demo directory MAY serve, and `/system/mode` MUST return `directory: "demo"`, `degraded: false`
