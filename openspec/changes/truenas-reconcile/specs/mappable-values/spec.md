## ADDED Requirements

### Requirement: The schema says what is mappable
Every entitlement field that is not a lifecycle field SHALL be mappable. A field MAY carry `label` (how people say it), `single` (one value per person) and `per_person` (an operator may set one person's own value). Syndra SHALL label a field by its name when the add-on gives no label. The frontend SHALL NOT name any field.

#### Scenario: A lifecycle field
- **WHEN** values are asked for `enabled`
- **THEN** the answer is `404 FIELD_NOT_MAPPABLE`, because the resolver computes it from whether someone holds a mapped role

### Requirement: Values are the add-on's last read, labelled with its age
`GET /targets/{t}/mappable-values?field=` SHALL return the add-on's enumeration with `read_at` and `current: true` when the add-on answers, the last successful list with `current: false` when it does not, and `409 VALUES_NEVER_READ` when no list was ever read.

#### Scenario: Target paused
- **WHEN** the add-on cannot be read and a list was read at 09:12
- **THEN** that list is returned with `read_at` 09:12 and `current: false`

#### Scenario: Group deleted between read and write
- **WHEN** a mapping names a group the add-on no longer resolves
- **THEN** the write is refused with `400 VALIDATION_FAILED` and `details.value = "unresolvable"`

### Requirement: An empty group list says what it left out
The group values answer SHALL carry `builtin_count` and the first two built-in group names in read order, and SHALL never list a built-in group.

#### Scenario: A NAS with only built-in groups
- **WHEN** TrueNAS lists 34 groups, all built in
- **THEN** values is empty, `builtin_count` is 34, and `builtin_examples` names the first two

### Requirement: Values may carry how people say them
A values answer MAY carry `labels`, value to label. A home folder's value SHALL be its absolute path, and its label `share › folder`.

#### Scenario: A home folder option
- **WHEN** a share `main` has a dataset folder `home_dir`
- **THEN** its value is the folder's absolute path and its label is `main › home_dir`
