> **Status:** Wave 1 delta — OIDC dashboard renders Title/Team/Location | [< Index](../../../../INDEX.md)

# Requirement: Operator Dashboard Identity Rendering (delta)

## ADDED Requirements

### OIDC sessions populate Title/Team/Location identically to demo
The OIDC callback MUST fetch `/api/v1/me/profile` with the freshly-issued access token immediately after token exchange and embed `title`, `team`, `location`, and `status` into the session cookie. The member dashboard's Identity card MUST render these fields for every authenticated session, regardless of session type.

### Endpoint shape
`GET /api/v1/me/profile` MUST be gated by `withUserAuth`, MUST resolve the requester's user ID from the bearer-token actor, and MUST return the same `models.UserProfile` shape returned by `directory.Default.FindUser` — including the Zitadel metadata overlay.

### Robustness
A failed metadata fetch MUST NOT block session creation; affected fields render as empty strings until the operator updates Zitadel metadata.

(Audit refs: C2, D5)
