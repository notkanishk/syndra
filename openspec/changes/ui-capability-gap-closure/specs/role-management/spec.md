> **Status:** ui-capability-gap-closure delta — role disambiguation moves from a backend string to a UI rule | [< Index](../../../../INDEX.md)

# Requirement: Role Reference Rendering (delta)

## MODIFIED Requirements

### Requirement: A role MUST be named as a `(project, role)` pair wherever the project is not already established

The identity of a role is `(project_id, role_key)`. `role_key` alone is not an
identity: `admin` in Printing Lab grants nothing in Metal Shop, and a surface
that renders only `admin` has asserted something false — that there is one such
role.

Composition MUST live in exactly two places:

- `<RoleRef projectId roleKey />` — rows and lists. Renders the project's
  resolved name, then the raw `role_key` in monospace. A table is scanned for
  identifiers, so it shows one.
- `roleLabel(projectName, roleKey, roleDisplayName?)` — sentences: toasts,
  dialog ledes, warning banners. Renders the project name, then the role's
  human display name (falling back to the humanized key). Prose is read, so it
  does not show a key.

A surface MUST NOT hand-roll the pair.

The pair MAY be established by structure instead of by repetition, in which
case the role is named alone: a roles index with a Project column, a page
scoped to one project, or role rows grouped under a project card. A surface
that establishes the project structurally MUST NOT also repeat it inline.

#### Scenario: The same key in two projects reads as two roles

- **GIVEN** `admin` exists in both Printing Lab and Metal Shop
- **WHEN** both are rendered in the same list — a request queue, an audit tail, an expiring-access table
- **THEN** the two rows MUST differ in their rendered text

#### Scenario: A write is confirmed by the pair it wrote

- **GIVEN** an operator grants `trained` in Printing Lab from the person page
- **WHEN** the grant succeeds and the dialog closes
- **THEN** the toast MUST name "Printing Lab / Trained operator", not `trained`

The dialog's project select is not sufficient: it is gone by the time the toast
is read.

#### Scenario: Half an identity is never rendered

- **WHEN** either `projectId` or `roleKey` is absent
- **THEN** `<RoleRef>` MUST render an em dash

A bare `admin` with no project is worse than nothing, because it looks like an
answer.

## REMOVED Requirements

### Requirement: `CatalogRole` carries a `display_label` for global disambiguation

**Reason:** relocated to the UI, not dropped as a capability.

`display_label` was `"{ProjectName}: {DisplayName}"`, composed in
`services.GlobalRoleCatalog` and present only on `GET /roles` rows. Grant rows,
audit entries, access requests, propagation rows and drift items carry
`(project_id, role_key)` and no label — so the field named as the
disambiguation mechanism could not disambiguate any surface where two same-key
roles actually collide. Its single consumer rendered it beside a `project_name`
column, printing the project twice on every row.

Superseded by `<RoleRef>` and `roleLabel()`, which resolve any pair through the
name resolver and therefore apply everywhere.
