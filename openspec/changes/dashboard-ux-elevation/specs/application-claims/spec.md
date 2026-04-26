> **Status:** Integrated | [< Index](../../../../INDEX.md)

## ADDED Requirements

### Requirement: Token Simulator MUST support Copy and Compare workflows

The `/applications` Token Simulator MUST let admins copy the simulated JWT payload and compare two users' payloads side-by-side so claim shaping can be verified across personas.

#### Scenario: Copy JSON to clipboard
- **WHEN** an admin clicks the "Copy JSON" affordance on the simulation panel
- **THEN** the rendered `custom_claims` JSON MUST be written to the clipboard
- **AND** a brief "Copied!" confirmation MUST appear

#### Scenario: Compare two users
- **WHEN** an admin selects a "Compare with" user in addition to the primary user
- **THEN** the simulator MUST render two simulation panels side-by-side
- **AND** primitive values that differ between the two payloads MUST be visually highlighted (amber underline) so admins can spot the deltas at a glance

### Requirement: User request history MUST surface reviewer notes

When a member views their request history, the UI MUST surface the reviewer's identity and decision note for any request that has been resolved.

#### Scenario: Approved/rejected requests display reviewer attribution
- **WHEN** a member opens `/requests`
- **AND** a request has status "approved" or "rejected"
- **THEN** the request card MUST display "Reviewed by {reviewer_id}: {review_note}" if either field is present
- **AND** rejected requests MUST use the destructive Badge variant on the status pill
