> **Status:** Integrated | [< Index](../../../../INDEX.md)

## ADDED Requirements

### Requirement: Member portal MUST offer an inline Request Access modal

The member service catalog MUST let users request access to a service without navigating away. The modal MUST be focus-trapped, ESC-cancellable, and surface toast feedback on submit.

#### Scenario: Inline modal for No-Access services
- **WHEN** a member clicks "Request Access" on a service with status "No Access"
- **THEN** an inline modal MUST open with `role="dialog" aria-modal="true"`
- **AND** the modal MUST contain a justification textarea and a duration button group (1 week / 1 month / 1 semester / Permanent)
- **AND** the default role MUST be the project's first role (resolved from `/api/v1/catalog`)
- **AND** Submit MUST be disabled until both justification and a default role are populated
- **AND** Escape, click-outside (when not pending), and Cancel MUST close the modal

#### Scenario: Successful submit toasts
- **WHEN** the modal submits successfully
- **THEN** the modal MUST close
- **AND** a success toast MUST appear naming the service and confirming the admin will review

#### Scenario: Active or Pending services link to history
- **WHEN** a service has status "Active" or "Pending"
- **THEN** the button MUST link to `/requests` instead of opening the request modal (there is nothing new to request)
