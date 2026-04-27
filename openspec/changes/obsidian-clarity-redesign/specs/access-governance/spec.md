> **Status:** Proposed | [< Index](../../../../INDEX.md)

## MODIFIED Requirements

### Requirement: Audit log entries MUST display resolved actor, target, and resource names

The `/audit` table MUST replace raw UUID renders for `actor_id`, `target_id`, and `resource_id` with resolved names via `<UserName/>` and `<ResourceName kind/>`. Raw UUIDs MAY remain available behind a hover/title affordance or the `?debug=ids` flag, but MUST NOT be the primary rendered text.

#### Scenario: Actor column resolves to user name
- **WHEN** an audit log row renders for action `bundle.created` (or any action with a user actor)
- **THEN** the actor cell MUST display `<UserName id={log.actor_id}/>`
- **AND** the raw UUID MUST be accessible only via the row's "Show ID" affordance or `?debug=ids`

#### Scenario: Target column resolves per kind
- **WHEN** an audit log row carries `target_kind` (user / project / role / bundle)
- **THEN** the target cell MUST render `<ResourceName kind={log.target_kind} id={log.target_id} />`
- **AND** when `target_id == "-"` (system-wide actions) the cell MUST render the literal `—` (em dash) without resolution

#### Scenario: System actor "system" renders as System
- **WHEN** `log.actor_id == "system"` (background scheduler, webhook ingest, etc.)
- **THEN** the actor cell MUST render the literal "System" with a subtle muted style — not a name lookup

### Requirement: Audit actor filter MUST display resolved labels

The actor filter `<select>` (currently `audit/page.tsx:96, 98`) MUST replace its raw UUID labels with resolved user names via the new `<Select/>` primitive (or a custom combobox). The emitted filter value MUST remain UUID-typed so backend filtering continues to work.

#### Scenario: Filter dropdown labels are names
- **WHEN** the audit filter row renders
- **THEN** the actor filter options MUST render `<UserName id={actorId}/>` as their label
- **AND** the option `value` attribute MUST remain the raw UUID for backend compatibility
- **AND** "System" actors MUST appear as a single non-UUID option

### Requirement: Governance watchlist row format MUST resolve all entity references

The expiring-access watchlist (currently `audit/page.tsx:310`) MUST format rows as `<UserName/> → <ProjectName/>:<RoleName/>` (with the resolved name on each side of the arrow). Cleanup hints with embedded UIDs MUST resolve them similarly.

#### Scenario: Expiring grant row
- **WHEN** an expiring grant row renders in the watchlist
- **THEN** the row MUST display `<UserName id={grant.user_id}/> → <ProjectName id={grant.project_id}/>:<RoleName projectId={grant.project_id} roleKey={grant.role_key}/>`
- **AND** the urgency tone MUST continue to follow the `describeExpiry` semantics from `dashboard-ux-elevation` (critical/warning/neutral)
- **AND** for active escalations (≤ 24h to expiry) a `<Pulse variant="warn"/>` MUST overlay the row's status chip

#### Scenario: Pending request row
- **WHEN** a pending access request row renders in the watchlist
- **THEN** the row MUST display the requester via `<UserName/>` and the requested role via `<RoleName/>`

## ADDED Requirements

### Requirement: Audit detail Drawer MUST surface full payloads

Each audit log row MUST be expandable into a `<Drawer/>` showing the complete event payload (request body, response status, attribution context) via `<JsonView/>`. This replaces the current "log row is the whole story" pattern and gives operators forensic depth without leaving the audit page.

#### Scenario: Click row opens Drawer with payload
- **WHEN** an admin clicks an audit log row's "Detail" affordance
- **THEN** a `<Drawer/>` MUST open from the right side
- **AND** the Drawer MUST render the row's full event payload via `<JsonView/>`
- **AND** the Drawer MUST follow the same focus-trap, Esc, and click-outside semantics as `<Modal/>`

### Requirement: Watchlist cards MUST adopt glass-card surface

The governance watchlist cards (current 3-card summary at the top of `/audit`) MUST adopt the `glass-card` surface treatment with `bg-blob-hero` atmospheric depth, replacing the current solid-bg cards.

#### Scenario: Glass-card watchlist
- **WHEN** the audit page renders
- **THEN** the watchlist summary cards MUST use the `<Card variant="glass"/>` surface
- **AND** the page MUST sit over a `bg-blob-hero` atmospheric layer
