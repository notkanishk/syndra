> **Status:** basic-advanced-ia delta — navigation contract, indicators, source-specific removal | [< Index](../../../../INDEX.md)

# Requirement: Access Governance (delta)

## ADDED Requirements

### Requirement: Navigation structure MUST NOT move in response to data

The rail MUST render from a static structure; only badge values may change. A section with nothing in it MUST keep its position and render a hollow zero. Nothing MUST ever be inserted above an existing item.

The previous rail injected a Drift section at the top whenever its count went non-zero, pushing every other item down under the operator's cursor mid-click.

Switching Basic → Advanced MUST append sections only. It MUST NOT reorder or rename anything already present.

#### Scenario: A count going non-zero moves nothing

- **GIVEN** the Advanced rail rendered with every indicator at zero
- **WHEN** the same rail renders with pending requests 3, expiring 1, pending changes 2 and drift 12
- **THEN** the sequence of rows MUST be identical

#### Scenario: An empty section keeps its seat

- **WHEN** the drift count is zero
- **THEN** the Unexplained access row MUST still render, showing a hollow `0`

### Requirement: The rail MUST be fed scalars, not payloads

`GET /api/v1/governance/indicators` MUST return `pending_requests`, `expiring_grants`, `pending_propagation`, `drift` and `zitadel_reachable` as scalars. The rail MUST poll this endpoint and MUST NOT consume `GET /api/v1/governance/summary`, which carries every pending request and expiring grant object.

The expiry horizon used by the indicators MUST be the same constant Today uses, so a badge and the page it points at cannot disagree about what "soon" means.

#### Scenario: The badge and the page agree

- **WHEN** the indicators are computed
- **THEN** the expiring-grants count MUST be taken over the same window as the governance summary's expiring grants

#### Scenario: Reachability is only probed when it matters

- **WHEN** the outbox is empty
- **THEN** the Zitadel reachability probe MUST be skipped, because there is nothing to resume and the rail polls frequently

### Requirement: Role removal MUST be named after its source and MUST state the residual outcome

There MUST be no generic "revoke role" control. A person may hold one role through several sources at once, so the action MUST be named after the thing being removed — *Remove direct access*, *Remove bundle assignment* — and where the source is a mapping rule, no removal MUST be offered at all.

Every confirmation MUST state what the person is left holding. Where more than one source exists, the dialog MUST list one entry per source rather than guessing.

#### Scenario: The last source says they lose it

- **GIVEN** a person holds a role only through a direct grant
- **WHEN** the removal dialog opens
- **THEN** it MUST state that they will lose the role

#### Scenario: A surviving source says they keep it

- **GIVEN** the same person also holds the role through the Lab Tech bundle
- **WHEN** the direct removal dialog opens
- **THEN** it MUST state that they will still hold the role, and MUST name the surviving source

#### Scenario: An automatic source offers the rule, not a removal

- **GIVEN** a person holds a role because a mapping rule fired
- **WHEN** the row's action is used
- **THEN** no destructive control MUST be rendered
- **AND** the dialog MUST name the rule's input role as the only per-person route

### Requirement: A disabled control MUST state its reason in visible copy

A blocked action MUST render its reason as text, not only as a `title` attribute, and MUST retain a reduced-alpha form of the semantic colour it would otherwise carry. Hover does not exist on touch and does not survive a screenshot sent to a colleague.

#### Scenario: Resume now is blocked by an unreachable provider

- **GIVEN** `pending_propagation.zitadel_reachable` is false
- **THEN** the Resume now control MUST be disabled
- **AND** the reason MUST be rendered as visible copy stating that writes stay queued and nothing is lost

### Requirement: A control MUST be exactly one interactive element

Something that navigates MUST be a link; something that acts MUST be a button. A button MUST NOT be nested inside a link to borrow its styling — `<a><button/></a>` is invalid HTML and presents a keyboard or screen-reader user with two overlapping controls where the page shows one. Shared appearance MUST be achieved by sharing the class set, not by nesting the elements.

#### Scenario: A button-shaped navigation

- **WHEN** an action navigates rather than mutating — "Open triage", "Request an extension", "Open the rule"
- **THEN** it MUST render a single anchor carrying the button's styling
- **AND** the accessibility tree MUST expose exactly one control for it

### Requirement: Destructive fill MUST appear only on a dialog's confirming button

A destructive action in a row MUST be a red outline. A solid destructive fill MUST appear only on the confirming button inside a focus-trapped dialog. A solid red button in a table row is one stray click from an outage.

#### Scenario: Row versus dialog

- **WHEN** a role-member row offers a removal
- **THEN** the control MUST be an outline
- **AND** the confirming button inside the resulting dialog MUST be the solid fill
