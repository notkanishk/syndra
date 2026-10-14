## ADDED Requirements

### Requirement: Database query and scanner compatibility coverage

The system MUST maintain live-database regression coverage for production reads
whose correctness depends on the number, order, or database types of selected
columns matching a shared row scanner.

#### Scenario: Role mappings are resolved for an applied target

- **WHEN** a target mapping exists for a project and role held by a subject
- **THEN** the production role-set query MUST return the complete mapping row
- **AND** its shared scanner MUST read the result without a column-count or type mismatch
- **AND** the mapping timestamps MUST be populated from the stored row
