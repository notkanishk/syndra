> **Status:** Partial (config UI deferred P5) | [< Index](../../../../INDEX.md) | [Feature Coverage](../feature-coverage.md)

# Requirement: Automation & Welcome Policies

The system MUST support automatic assignment of bundles to users based on system-wide triggers.

## Default "Welcome" Bundle
Admins MUST be able to designate a specific bundle to be automatically assigned to every new user account detected in Zitadel.

### Scenario: Setting a global default bundle
- **GIVEN** a bundle "Basic Access" containing `wiki:member` and `platform:support` roles
- **WHEN** an admin marks this bundle as the "Default for new accounts"
- **THEN** MkAuth MUST monitor for new user creation events in Zitadel.
- **AND** automatically grant the "Basic Access" bundle to those users upon detection.

## Policy State Management
The system MUST track which bundles are assigned as "Default" and ensure only one bundle (or a specific set) acts as a global entry point.

### Scenario: Admin dashboard visibility
- **WHEN** viewing the list of bundles
- **THEN** any bundle marked as a "Welcome" bundle MUST be visually highlighted with a status badge (e.g., `Default`).

## Implementation: Event Intake and Backend-Orchestrated Assignment
Detection of new user accounts MAY originate from Zitadel-compatible event mechanisms, but the MkAuth Backend MUST remain the sole mutation authority for assigning Welcome Bundles.

- **Single Writer Rule:** Welcome-bundle assignment MUST be performed by the MkAuth Backend so audit logging, retries, idempotency, and policy evaluation remain centralized.
- **Trigger Source:** Zitadel Actions v2 or validated backend webhook intake MAY be used to signal that a new user exists.
- **Boundary Rule:** Zitadel-facing hooks and events MUST NOT directly become the business mutation engine for bundle assignment logic.

### Scenario: New user detected through Zitadel-compatible trigger
- **WHEN** Zitadel signals that a new user account was created
- **THEN** MkAuth MAY accept that signal through an Actions v2-compatible event path or a validated backend webhook
- **AND** the MkAuth Backend MUST perform the actual welcome-bundle assignment

### Scenario: Welcome assignment remains backend-owned
- **WHEN** a Welcome Bundle is configured for automatic onboarding
- **THEN** the resulting role or bundle mutation MUST be executed by the MkAuth Backend
- **AND** the audit trail, retry behavior, and idempotency guarantees MUST be anchored to the backend path

### Scenario: Retry-safe onboarding mutation
- **WHEN** MkAuth retries a previously attempted welcome-bundle assignment
- **THEN** the operation MUST be idempotent
- **AND** the system MUST avoid duplicate grants while preserving operator visibility into the retry outcome

## Configuration-Driven Welcome Bundle Selection
The system MUST allow admins to explicitly designate a welcome bundle through a configuration interface rather than relying on name-matching conventions.

### Scenario: Explicit welcome bundle designation
- **WHEN** an admin configures a specific bundle as the welcome default
- **THEN** the system MUST use that explicit configuration for all onboarding assignments
- **AND** MUST NOT fall back to name-matching heuristics

### Scenario: Welcome bundle visible in admin UI
- **WHEN** viewing the list of bundles
- **THEN** the explicitly configured welcome bundle MUST be visually highlighted with a status badge

> **Status:** Deferred to Phase 5. Currently `GetWelcomeBundle` returns the first bundle with "welcome" in its name, or the first bundle overall.
