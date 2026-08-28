> **Status:** operator-runbook-surfaces delta — demo residue, supported reset, operator commands | [< Index](../../../../INDEX.md)

# Requirement: Operational Readiness (delta)

## ADDED Requirements

### Requirement: Demo data MUST be reported by what is stored, not by what this process did

`GET /system/mode` MUST report `seed_residue`: the number of stored rows referencing a demo fixture, regardless of which process wrote them or whether seeding is currently enabled.

The demo-data banner MUST key off that count. It MUST NOT key off `seed_active`.

`seed_active` answers "did this process seed?". Turning `SYNDRA_SEED_DEMO` off and restarting changes that answer to false and changes nothing an operator can see — every seeded row is still stored and still served. A banner reading that flag therefore disappears at exactly the moment the operator is checking whether their fix worked, and its disappearance reads as confirmation.

The fixture id sets MUST come from the demo catalog itself, so adding a fixture widens the check rather than silently narrowing it.

A failed count MUST be logged and reported as zero. It MUST NOT fail the mode probe, and MUST NOT be reported as a positive.

#### Scenario: Residue outlives the flag

- **GIVEN** a deployment whose database holds seeded rows
- **AND** `SYNDRA_SEED_DEMO` is false, so nothing seeded this process
- **WHEN** the operator loads any screen
- **THEN** the banner MUST still appear
- **AND** it MUST state how many rows came from the seeder
- **AND** it MUST say the rows are leftovers from an earlier run rather than repeating the instruction that was already followed

#### Scenario: A clean database says nothing

- **GIVEN** no stored row references a demo fixture
- **WHEN** the mode probe answers
- **THEN** `seed_residue` MUST be 0 and no banner MUST render

#### Scenario: A failed count is not a clean database

- **GIVEN** the residue query errors
- **WHEN** the mode probe answers
- **THEN** it MUST return 200 with the rest of the response intact
- **AND** `seed_residue` MUST be 0
- **AND** the failure MUST be logged

### Requirement: Seeding MUST NOT be enabled by a default the operator never wrote

Deployment manifests MUST NOT supply a default value for `SYNDRA_SEED_DEMO`. The backend already resolves it — seed when no live directory client came up, do not when one did — and a manifest-level default overrides that decision for every deployment whose environment file does not mention the variable.

#### Scenario: An unset variable leaves the decision with the backend

- **GIVEN** `SYNDRA_SEED_DEMO` is absent from the environment file
- **AND** a live Zitadel management client initialises successfully
- **WHEN** the backend starts
- **THEN** it MUST NOT seed

### Requirement: There MUST be a supported way back to a known starting state

Two states MUST be reachable, and they are not the same: removing only fixture-derived rows, and truncating every operator-owned table.

Both MUST be dry-run by default, printing per-table counts and deleting nothing. Committing MUST require an explicit flag AND typed confirmation. The deletion MUST run in one transaction — a reset that removes bundle roles and leaves the bundle produces an empty bundle nobody can explain, which is worse than either end state. The derived claim cache MUST be flushed, or deleted grants keep being served until their entries expire.

Neither mode MUST touch the identity provider. Syndra's ledger records what Syndra decided; clearing it revokes nothing upstream, and the next reconciliation sweep reports the surviving grants as unexplained access — which is how they get re-adopted deliberately rather than assumed.

Fixture-only mode MUST name any non-fixture account that loses access because it holds a fixture bundle or a grant on a fixture project. Those rows cascade out with the fixture and do not appear in the per-table counts.

#### Scenario: The default run deletes nothing

- **GIVEN** a database holding fixture rows
- **WHEN** the reset script is run without the apply flag
- **THEN** it MUST print per-table counts
- **AND** it MUST delete nothing
- **AND** it MUST print the command that would commit

#### Scenario: A real account losing access is named before anything is deleted

- **GIVEN** a real user assigned to a bundle created by the seeder
- **WHEN** fixture-only mode is run
- **THEN** it MUST name that account
- **AND** it MUST state that removing the fixture removes their access and that Syndra will not re-grant it

#### Scenario: Confirmation is typed, not defaulted

- **GIVEN** the apply flag is passed
- **WHEN** the confirmation prompt is answered with anything other than the mode name
- **THEN** nothing MUST be deleted

### Requirement: A screen that names a command MUST name what follows it

Where the console tells an operator to run something, it MUST render the command as copyable text alongside the steps that must happen afterwards — an environment change, a restart, a verification.

The command MUST come from the backend where the backend already reports one, so the two cannot drift apart.

These MUST NOT be offered as buttons. Each has a failure mode in which the first half lands and the second does not — a signing key Zitadel accepted and the backend is not verifying against — and a browser that reports success while the system sits in that half-state is worse than no affordance. An operator at a terminal has the exit code and can stop.

Clipboard failure MUST be silent and the command MUST remain selectable text. The app is reached over a LAN address where the clipboard API is unavailable, so a copy error is the expected path, not an exception.

#### Scenario: Rotation states the restart it depends on

- **GIVEN** the Zitadel page with a signing key installed
- **WHEN** the signing-key panel renders
- **THEN** it MUST show the rotate command the backend reported
- **AND** it MUST state the environment variables to update and the restart that must follow
- **AND** it MUST state that claims are stock-only between the rotation and the restart
- **AND** it MUST NOT offer a control that performs the rotation

#### Scenario: No key installed offers registration, not rotation

- **GIVEN** no signing key is installed
- **WHEN** the panel renders
- **THEN** it MUST say that signature verification is passing requests through unchecked
- **AND** it MUST offer the registration command
- **AND** it MUST NOT offer the rotation command, because there is nothing to rotate

#### Scenario: A service that cannot start explains itself

- **GIVEN** a deployment with no LDAP server
- **WHEN** Hardware sync renders
- **THEN** it MUST state that the sync container restarts on a loop, that this is configured behaviour rather than a fault, and that nothing is queued or lost
- **AND** it MUST offer the command to stop it and name what to set when a directory server exists
