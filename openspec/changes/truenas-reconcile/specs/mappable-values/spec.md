## ADDED Requirements

### Requirement: The add-on declares what is mappable
An add-on manifest SHALL be able to declare `mappable_fields` (name, label, enumerate). Syndra SHALL list mappable fields only from the manifest, falling back to every non-lifecycle field labelled by name when the manifest declares none. The frontend SHALL NOT name any field.

#### Scenario: Older add-on without the declaration
- **WHEN** a manifest has no `mappable_fields`
- **THEN** every non-lifecycle field is mappable and labelled by its name

### Requirement: Values are the add-on's last read, labelled with its age
`GET /targets/{t}/mappable-values?field=` SHALL return the add-on's enumeration with `read_at` and `current: true` when the add-on answers, the last successful list with `current: false` when it does not, and `409 VALUES_NEVER_READ` when no list was ever read.

#### Scenario: Target paused
- **WHEN** the add-on cannot be read and a list was read at 09:12
- **THEN** that list is returned with `read_at` 09:12 and `current: false`

#### Scenario: Group deleted between read and write
- **WHEN** a mapping names a group the add-on no longer resolves
- **THEN** the write is refused with `400 VALIDATION_FAILED` and `details.value = "unresolvable"`
