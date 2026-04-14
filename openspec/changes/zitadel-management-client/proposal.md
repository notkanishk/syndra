## Why

MkAuth's orchestrator has contained the logic for mapping-rule enforcement and role propagation since Phase 2, but the actual Zitadel Management API client was stubbed — `MgmtClient interface{} = nil`. The system ran in "local-policy-only mode": webhook-driven orchestration triggered the code path but every grant operation was a no-op. This blocked the Live Webhook Listener and any production role lifecycle management.

The frontend OIDC token forwarding, backend JWT validation, and webhook HMAC verification were all completed in earlier Phase 3 work, satisfying the security preconditions documented in the design. The M2M client was the final missing piece before the orchestration layer becomes functional.

## What Changes

* Implements a direct HTTP client for the Zitadel Management API v1 using JWT profile M2M authentication (RS256 assertion signed with a service account private key, token caching with thread-safe refresh).
* Provides four Management API operations: `AddUserGrant`, `RemoveUserGrant`, `ListUserGrants`, `GetUser`.
* Includes retry with exponential backoff on 429/503, automatic token refresh on 401, and structured error extraction from Zitadel API responses.
* Extends the `ZitadelClient` interface from 1 method to 4, and changes `MgmtClient` from untyped `interface{}` to typed `ZitadelClient` — eliminating runtime type assertions in the orchestrator.
* Adds injectable deps (`httpDo`, `timeNow`, `dbGetActiveMappingRules`) for full testability without network calls.
* Graceful degradation preserved: when `ZITADEL_MACHINE_KEY_PATH` is absent, the system continues in local-policy-only mode with no behavioral change.

## Capabilities

### New Capabilities
* `zitadel-m2m-client`: Service account key loading (PKCS1/PKCS8), JWT profile token exchange, and authenticated Management API calls.
* `grant-lifecycle`: AddUserGrant, RemoveUserGrant, ListUserGrants enable full grant CRUD against Zitadel.
* `user-lookup`: GetUser retrieves human user profiles (displayName, email) from Zitadel's nested v1 response format.

### Modified Capabilities
* `orchestration-engine`: EnforceMappingRules and AssignUserToRole now call through the real client when credentials are present.
* `zitadel-integration`: Status upgraded from Partial to Integrated in the feature coverage matrix.

## Impact

* Adds 5 new files under `backend/internal/zitadel/`: `keyfile.go`, `token.go`, `deps.go`, `client_test.go`, `orchestrator_test.go`.
* Replaces `backend/internal/zitadel/client.go` (stub → real implementation).
* Modifies `backend/internal/zitadel/orchestrator.go` (extended interface, simplified type flow, injectable deps).
* Modifies `.env.example` (adds `ZITADEL_MACHINE_KEY_PATH`).
* Updates `AUDIT.md`, `ROADMAP.md`, `feature-coverage.md`, `design.md` to reflect implemented status.
* Zero new `go.mod` dependencies — reuses `golang-jwt/jwt/v5` and stdlib `net/http`.
* Does not change any database schema, API routes, or frontend code.
