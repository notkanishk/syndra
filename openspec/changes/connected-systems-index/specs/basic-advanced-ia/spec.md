## ADDED Requirements

### Requirement: The target plane MUST have a seat in the rail on every operator deployment

The Advanced rail's System group MUST contain a `Connected systems` row on
every deployment, whether or not that deployment has registered an add-on. The
row MUST be part of the static navigation tree rather than derived from the
roster, so that the breadcrumb can name it and so that its seat is held
independently of configuration.

The page behind it MUST state, in words, that no system is connected when none
is, and MUST name what would connect one.

An empty roster rendering as an unchanged rail is indistinguishable from a
build that does not carry the add-on platform at all. An operator reading that
silence concludes the feature did not ship, which is a conclusion the product
gave them no way to check. Every other section of this rail already keeps its
seat when it is empty; the target plane is the section where the absence is
most easily misread.

#### Scenario: The deployment has registered no add-on

- **WHEN** `GET /api/v1/targets` returns no target
- **THEN** the System group MUST still render the `Connected systems` row
- **AND** `/system/targets` MUST state that no system is connected
- **AND** it MUST name `ADDON_TARGETS` as what registers one

#### Scenario: The deployment has registered an add-on

- **WHEN** one or more targets are registered
- **THEN** `Connected systems` MUST remain in place, above the per-target rows
- **AND** `Zitadel` (formerly `Identity provider`) MUST remain the first row of the group
- **AND** the index MUST NOT claim a target's own route as its own

### Requirement: The index MUST NOT collapse registration, reachability and transport into one status

Registration is deployment configuration; a capability manifest having been
read is a runtime fact; a transport secret that fails to load is a fault on
Syndra's side. The index MUST report these as distinct states.

An operation count MUST NOT be rendered for a target that has published no
manifest. `0` there is a claim about the target — that it can do nothing —
rather than about Syndra never having asked.

#### Scenario: A registered target has never answered

- **WHEN** a target is registered and not callable
- **THEN** the row MUST say it has no manifest yet
- **AND** MUST NOT render an operation count

#### Scenario: The transport secret fails to load

- **WHEN** a target's transport status is an error
- **THEN** the row MUST report a transport fault
- **AND** MUST NOT report the target as answering

## MODIFIED Requirements

### Requirement: A list cell MUST NOT carry prose wider than its column

A fixed-width column in a table-shaped card MUST carry only values that fit it.
Explanatory prose about a row belongs in a column with room for it — in
practice the name column, which is `flex-1`.

The projects table rendered "No roles yet — nothing here can be granted" inside
a 60px right-aligned count column. It wrapped to six lines, the row grew to
four times the height of its neighbours, and the table read as broken.

The fact is worth stating; a bare `0` beside a member count does read as a
project that is merely quiet. Stating it beside the project's name keeps both
properties: the count column reads as a column of counts, and the reader still
learns that nothing in that project can be granted.

#### Scenario: A project has no roles

- **WHEN** a project's active role list is empty
- **THEN** the count column MUST render the count
- **AND** the sentence explaining that nothing can be granted MUST be rendered
  beside the project's name
- **AND** MUST NOT be a descendant of the count column
