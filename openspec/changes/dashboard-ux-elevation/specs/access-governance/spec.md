> **Status:** Integrated | [< Index](../../../../INDEX.md)

## ADDED Requirements

### Requirement: Expiring grants MUST surface urgency at a glance

The Governance Watchlist MUST sort expiring grants soonest-first and render an urgency tone (critical / warning / neutral / expired) so admins can triage the riskiest expirations without reading every date.

#### Scenario: Sort and tone-code expiring grants
- **WHEN** the audit page loads with expiring grants in the response
- **THEN** the list MUST be sorted by `expires_at` ascending (soonest first)
- **AND** grants expiring in ≤7 days MUST render with the destructive Badge variant and a red-tinted card
- **AND** grants expiring in 8–14 days MUST render with the outline Badge variant and an amber-tinted card
- **AND** grants expiring in >14 days MUST render with the secondary Badge variant
- **AND** each card MUST show a human countdown ("expires in 3 days") next to the absolute expiry timestamp

### Requirement: Audit feed MUST group by day and support filters

The Audit feed MUST group entries under day headers and let admins filter by category, actor, and free-text search. The list MUST cursor-bump up to a 200-entry cap.

#### Scenario: Day grouping
- **WHEN** the audit feed renders with non-empty data
- **THEN** rows MUST be partitioned under day headers using "Today" / "Yesterday" / weekday name / absolute date
- **AND** each section MUST show the entry count next to the header

#### Scenario: Filter row
- **WHEN** the audit feed renders
- **THEN** a filter row MUST be present with a debounced free-text search, an action category select (`all` / `approved` / `rejected` / `created` / `updated` / `other`), and an actor select populated from the loaded entries
- **AND** changing any filter MUST update the visible groups in place
- **AND** when filters yield zero results, the empty state MUST surface a "Clear filters" CTA

#### Scenario: Load more
- **WHEN** the loaded count equals the current limit and the limit is below 200
- **THEN** a "Load more" button MUST be visible
- **AND** clicking it MUST raise the limit by 50 (capped at 200) and refetch

### Requirement: Audit log table MUST be semantic

The audit feed MUST render as a `<table>` with `<th scope="col">` and a screen-reader caption so assistive technology can navigate it.

#### Scenario: Semantic table
- **WHEN** the audit feed renders
- **THEN** the table MUST include an `sr-only` `<caption>` describing the section
- **AND** column headers MUST use `<th scope="col">`
- **AND** rows MUST use `<tr>` and `<td>` rather than CSS Grid
