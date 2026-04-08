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

## Implementation: Actions v2 Event Triggers
Monitoring for new user accounts MUST utilize the **Zitadel Actions v2** event mechanism.
- **Post-Registration Flow**: The "Welcome Bundle" assignment MUST be integrated into the v2 `post_user_registration` flow or similar event-level hook to ensure immediate propagation upon account creation.
