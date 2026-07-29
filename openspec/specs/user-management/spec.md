# user-management Specification

## Purpose
TBD - created by archiving change wave-3-frontend-remainder-and-consolidation. Update Purpose after archive.
## Requirements
### Requirement: Directory identity overlay is limited to Title and Team

The per-user identity overlay merged from the Zitadel user-metadata K/V store MUST populate only `Title` and `Team`. The `Location` attribute — added by `live-directory-identity-completeness` and rendered by Wave 1 — is removed from `UserProfile`, the OIDC session cookie, the session profile shape, all render surfaces, and the demo catalog. No `location` key is read from Zitadel metadata, carried in the session, or displayed.

Rationale: `location` never earned the vertical it occupied (backend profile → directory metadata read → OIDC callback → session cookie → dashboard render). `Title` and `Team` traverse the identical path and remain, demonstrating the overlay mechanism is sound; `location` is excised at the field level.

#### Scenario: User metadata overlay merges only title and team
- **WHEN** `zitadelSource.Users()` merges a user's Zitadel metadata keys into `UserProfile`
- **THEN** `title` populates `Title` and `team` populates `Team`
- **AND** a `location` metadata key is ignored (no `Location` field exists to receive it)

#### Scenario: Session and dashboard carry no location
- **WHEN** an OIDC session is established and the member dashboard identity card renders
- **THEN** the identity card shows `Team` (and `Title` where present) with no `location` segment
- **AND** the OIDC session cookie contains no `location` field

### Requirement: OIDC avatar initials always resolve to a non-empty seed

The OIDC session avatar MUST be derived from the first non-empty of: the user's `name`, the local-part of the user's `email`, or the `userId`. An OIDC payload with an empty `name` MUST still yield non-empty avatar initials rather than a blank badge.

#### Scenario: Blank name falls through to email then userId
- **WHEN** an OIDC session payload has an empty `name`
- **THEN** the avatar initials are computed from the email local-part
- **AND** when the email is also empty, from the `userId`

