## ADDED Requirements

### Requirement: Built-in groups are never mappable
Built-in groups (`builtin: true`) SHALL be outside the set the TrueNAS add-on manages. It SHALL exclude them from value enumeration, report them unresolvable on the write-path check, omit them from a subject's reported groups, and preserve an account's existing built-in memberships when converging.

#### Scenario: Mapping to an admin group
- **WHEN** a mapping names `builtin_administrators`
- **THEN** the value check reports it unresolvable and the mapping is refused

#### Scenario: Converging an account in builtin_users
- **WHEN** an account in `builtin_users` is converged to its mapped groups
- **THEN** the write carries `builtin_users` through, and the reported groups do not list it

### Requirement: A truncated group read is an error
A group read that returns more rows than `subjectReadCap` SHALL fail rather than act on a partial list.

#### Scenario: Over the cap
- **WHEN** `group.query` returns `subjectReadCap + 1` rows
- **THEN** enumeration and convergence both return an error and change nothing
