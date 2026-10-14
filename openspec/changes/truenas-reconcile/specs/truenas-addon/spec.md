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

### Requirement: What a group opens is read from the folder ACLs
The add-on SHALL read each enabled SMB share's root and first-level folder ACLs, unsimplified, and evaluate them as NFS4 does: entries in order, the first that applies to the reader and names a permission deciding it, inherit-only entries not applying to the folder itself. A place is read access with READ_DATA and change access with WRITE_DATA as well; traverse alone is not a place. A folder behind a root the reader can't traverse is out of reach. Per group, the answer SHALL be what holding the group adds to what every account opens (everyone@ and builtin_users), and the latter SHALL be listed apart.

#### Scenario: The production layout
- **WHEN** `main` lets Community traverse and `waddle_share` grants Community MODIFY
- **THEN** Community opens `main › waddle_share`, can change files

#### Scenario: The NAS can't be read
- **WHEN** a read fails after one succeeded
- **THEN** the last read is served with `current: false`, and with none the answer is `REACH_NEVER_READ`

### Requirement: One home location per system
A role SHALL say only whether it gives a home folder (`home: "true"`) and its default limit; any other `home` value SHALL be refused. Where homes go SHALL be one setting per system, `home_root`, which SHALL only be set to a first-level folder of an enabled SMB share that is a dataset of its own and not hidden, as read from the NAS at the time it is set. Each person's home SHALL be `<home_root>/<account name>`. Changing `home_root` SHALL move nothing. With no `home_root`, no home SHALL be made and the outcome SHALL say why. Of several limits, the highest SHALL apply.

#### Scenario: An application's share
- **WHEN** a share's first-level folders are hidden folders and plain directories
- **THEN** none of them is offered as the home location

#### Scenario: Two roles with different limits
- **WHEN** a person holds two roles that give a home folder, with limits of 10 GiB and 50 GiB
- **THEN** one home is made and its limit is 50 GiB

#### Scenario: The location changes
- **WHEN** `home_root` changes from `main › home_dir` to `main › homes`
- **THEN** existing homes stay where they are and new ones are made in `main › homes`
