> **Status:** Wave 1 delta — startup signing-key gate + vault dev-mode actor requirement | [< Index](../../../../INDEX.md)

# Requirement: Production Security Boundary (delta)

## ADDED Requirements

### Production refuses missing signing keys
When `ZITADEL_DOMAIN` is set, the backend MUST exit with a non-zero status during startup if either `ZITADEL_EVENT_SIGNING_KEY` or `ZITADEL_ACTION_SIGNING_KEY` is empty. The HTTP server MUST NOT bind a port until both keys are present.

### Runtime middleware refuses misconfigured production
`withZitadelActionSignature` MUST return `503 MISCONFIGURED` when the configured signing-key env is empty AND `ZITADEL_DOMAIN` is set. Dev-mode passthrough is allowed only when both are empty.

### Vault mutations require explicit actor in dev mode
`PUT /api/v1/users/{uid}/shadow-credential` and `DELETE /api/v1/users/{uid}/shadow-credential` MUST refuse with `400 MISSING_ACTOR` when no JWT actor is in the request context AND no `?actor=<id>` query parameter is supplied. The audit row MUST record the explicit actor, not the target user. Reads (`/status`, `/audit`) MUST continue to fall back to `{uid}` for the audit field; they MUST NOT require `?actor=`.

(Audit refs: C1, C3)
