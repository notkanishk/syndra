> **Status:** Integrated | [< Index](../../../../INDEX.md)

## ADDED Requirements

### Requirement: Mapping-rule creation MUST surface cycles before submission

The mapping-rule creation form MUST validate the candidate rule for cycles in real time so admins see the warning inline rather than after a submit failure.

#### Scenario: Live cycle detection endpoint
- **WHEN** the UI POSTs to `/api/v1/rules/mapping/validate` with a candidate `{source_project, source_role, target_project, target_role}`
- **THEN** the backend MUST reuse `dbDetectCycleOnInsert` to check the candidate without persisting it
- **AND** the response MUST be `{would_cycle, self_reference, reason?}`
- **AND** partial input (any required field missing) MUST return `{would_cycle: false, self_reference: false}` so the form doesn't 400 mid-edit

#### Scenario: Form debounces validation and disables submit on cycles
- **WHEN** an admin changes any select in the create form
- **THEN** the form MUST POST to the validate endpoint after 250ms of quiet
- **AND** while validating, the form MUST display "Checking for cycles…"
- **AND** if the response indicates `would_cycle` or `self_reference`, the form MUST display an amber warning with the reason and disable the Create button
- **AND** if the response indicates safe, the form MUST display a green confirmation

#### Scenario: Live preview of the rule string
- **WHEN** an admin is filling out the create form
- **THEN** the form MUST display a live "IF {source} THEN ADD {target}" preview using the human role labels (via `formatRoleRef`)
- **AND** the preview MUST update as the admin changes selects
