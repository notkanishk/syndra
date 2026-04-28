> **Status:** Proposed | [< Index](../../../../INDEX.md)

## MODIFIED Requirements

### Requirement: Audit log entries MUST display resolved actor, target, and resource names

The `/audit` table MUST replace raw UUID renders for `actor_id`, `target_id`, and `resource_id` with resolved names via `<UserName/>` and `<ResourceName kind/>`. Raw UUIDs MAY remain available behind a hover/title affordance or the `?debug=ids` flag, but MUST NOT be the primary rendered text.

#### Scenario: Actor column resolves to user name
- **WHEN** an audit log row renders for action `bundle.created` (or any action with a user actor)
- **THEN** the actor cell MUST display `<UserName id={log.actor_id}/>`
- **AND** the raw UUID MUST be accessible only via the row's "Show ID" affordance or `?debug=ids`

#### Scenario: Target line resolves to a user name
- **WHEN** an audit log row carries a `target_id`
- **THEN** the target line MUST render `<UserName id={log.target_id}/>`
- **AND** when `target_id == "-"` or empty (system-wide actions) the target line MUST be omitted

> **Implementation note.** The current `audit_logs` schema persists `target_zitadel_user_id` (no `target_kind` column), so Stage 2 routes every non-system target through `<UserName/>`. A `<ResourceName kind/>` dispatch becomes meaningful only when the audit shape is widened to carry the resource kind alongside the id; that schema change is deferred and tracked in `feature-coverage`.

#### Scenario: System actor "system" renders as System
- **WHEN** `log.actor_id == "system"` (background scheduler, webhook ingest, etc.)
- **THEN** the actor cell MUST render the literal "System" with a subtle muted style — not a name lookup

### Requirement: Audit actor filter MUST display resolved labels

The actor filter `<select>` MUST replace its raw UUID labels with resolved user names. The emitted filter value MUST remain UUID-typed so backend filtering continues to work.

Native `<option>` elements render their children as plain text and cannot compose React nodes, so the implementation reads the resolver cache directly to build the label string. Until resolution lands the option falls back to a truncated UID prefix; once the lookup batch settles the option text switches to the resolved display name. A future combobox primitive (Stage 4) will replace the native dropdown entirely.

#### Scenario: Filter dropdown labels are names once resolved
- **WHEN** the audit filter row renders and the lookup batch has settled
- **THEN** every actor filter option's label MUST be the resolved display name (no Zitadel UUID matches the option text)
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
