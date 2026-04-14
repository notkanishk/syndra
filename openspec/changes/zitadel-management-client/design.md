## Rationale

The orchestration layer was architecturally complete but inert — it had the logic to propagate roles via mapping rules but no way to actually write grants to Zitadel. Rather than pulling in the `zitadel-go/v3` SDK (which brings gRPC, protobuf, and dozens of transitive dependencies), this implementation uses direct HTTP calls to the stable Management API v1 endpoints. This matches the existing codebase style: `internal/auth/jwt.go` already uses `http.DefaultClient.Do` for JWKS fetching, and the project has zero framework dependencies.

## Technical Specification

### 1. Service Account Key Loading (`keyfile.go`)

**`ServiceAccountKey` struct** mirrors the Zitadel machine user JSON key file:
- `type` (must be `"serviceaccount"`), `keyId`, `key` (PEM-encoded RSA private key), `userId`.

**`LoadServiceAccountKey(path) → (*ServiceAccountKey, *rsa.PrivateKey, error)`**:
- Reads and validates the JSON file.
- Validates `type == "serviceaccount"` and non-empty required fields.
- Decodes PEM block and parses as PKCS1; falls back to PKCS8 (Zitadel may emit either format).
- Returns both the metadata (for JWT assertion claims) and the parsed private key (for signing).

### 2. Token Manager (`token.go`)

M2M authentication via JWT profile grant (RFC 7523):

**JWT assertion construction (`buildAssertion`)**:
- `iss`/`sub`: service account userId.
- `aud`: `https://{domain}` (the Zitadel issuer URL).
- `kid` header: keyId from the key file.
- `exp`: now + 1 hour. Signed with RS256 using `golang-jwt/jwt/v5`.

**Token exchange (`refresh`)**:
- `POST https://{domain}/oauth/v2/token` with `grant_type=urn:ietf:params:oauth:grant-type:jwt-bearer`.
- Scope: `openid urn:zitadel:iam:org:project:id:zitadel:aud`.
- Response body limited to 64 KB.

**Caching and thread safety**:
- `tokenManager` holds cached `accessToken` + `expiresAt` behind `sync.RWMutex`.
- `Token(ctx)` returns cached token if > 5 minutes from expiry; otherwise acquires write lock and refreshes.
- Double-checked locking pattern prevents thundering herd on concurrent refresh.
- `ForceRefresh()` clears cache, used by the HTTP client on 401 responses.

### 3. Extended Interface (`orchestrator.go`)

```go
type ZitadelClient interface {
    AddUserGrant(ctx context.Context, userID, projectID string, roleKeys []string) error
    RemoveUserGrant(ctx context.Context, userID, grantID string) error
    ListUserGrants(ctx context.Context, userID string) ([]UserGrant, error)
    GetUser(ctx context.Context, userID string) (*ZitadelUser, error)
}
```

**Response types**:
- `UserGrant`: `ID`, `UserID`, `ProjectID`, `RoleKeys`.
- `ZitadelUser`: `ID`, `Username`, `DisplayName`, `Email`, `State`.

**`MgmtClient`** changed from `interface{} = nil` to typed `ZitadelClient`. This eliminates the two runtime type assertions (`MgmtClient.(ZitadelClient)`) that existed in `EnforceMappingRules` and `AssignUserToRole`.

### 4. HTTP Management Client (`client.go`)

**`managementClient` struct**: domain, tokenManager, `*http.Client` (10s timeout).

**`doRequest(ctx, method, path, body)`** — centralized transport layer:
- Marshals body as JSON, injects `Authorization: Bearer {token}`.
- On 401: calls `ForceRefresh()` and retries once.
- On 429/503: retries with exponential backoff (100ms → 200ms → 400ms, max 3 attempts).
- On other 4xx/5xx: no retry; extracts `apiError` from response body for diagnostics.
- Response body reads capped at 1 MB (consistent with `withMaxBody` middleware).

**Zitadel Management API v1 endpoint mapping**:

| Method | HTTP | Path | Notes |
|--------|------|------|-------|
| `AddUserGrant` | POST | `/management/v1/users/{userId}/grants` | Body: `{projectId, roleKeys}` |
| `RemoveUserGrant` | DELETE | `/management/v1/users/{userId}/grants/{grantId}` | No body |
| `ListUserGrants` | POST | `/management/v1/users/grants/_search` | Body: `{queries: [{userIdQuery: {userId}}]}` — user ID is a query filter, not a path segment |
| `GetUser` | GET | `/management/v1/users/{userId}` | Response nests human data under `user.human.profile` and `user.human.email` |

**GetUser response parsing**: Zitadel v1 nests human user data as `user.human.profile.displayName` and `user.human.email.email`. The implementation uses an intermediate struct to extract these nested fields into the flat `ZitadelUser` type.

### 5. Injectable Dependencies (`deps.go`)

Following the established pattern from `cache/deps.go` and `handlers/deps.go`:

- `httpDo`: wraps `client.Do(req)` — tests inject a mock transport.
- `timeNow`: wraps `time.Now` — tests control time for token expiry.
- `tokenHTTPClient`: the `*http.Client` used by the token manager — tests inject mock transport.
- `dbGetActiveMappingRules`: wraps `db.GetActiveMappingRules` — orchestrator tests inject mock rules.

### 6. Initialization (`InitClient`)

- Reads `ZITADEL_DOMAIN` and `ZITADEL_MACHINE_KEY_PATH` from environment.
- If either is absent: logs `[ZITADEL] M2M credentials not set; operating in local-policy-only mode.` and returns nil.
- If both present: loads service account key, creates token manager, creates management client, assigns to `MgmtClient`.
- Called from `cmd/api/main.go` during startup (no change to call site — the function signature is unchanged).

## Test Coverage

**22 tests across 2 files**:

`client_test.go` (17 tests):
- Key file: valid load, missing fields, invalid PEM, wrong type, file not found.
- Token manager: cached token, expired refresh, refresh failure, JWT assertion format (validates iss/sub/aud/kid).
- API client: AddUserGrant success + 401 refresh, RemoveUserGrant, ListUserGrants (verifies correct path + query filter body), GetUser (verifies nested response parsing), 429 retry, 400 no-retry.

`orchestrator_test.go` (5 tests):
- Nil client graceful skip, matching rule propagation, grant error continuation (both rules attempted), assign success, assign nil client.

All tests use `resetDeps(t)` with `t.Cleanup` for isolation. RSA keys generated with `crypto/rand`. Mock HTTP transport via injectable `httpDo`. No network calls, no external dependencies.

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| Direct HTTP over `zitadel-go` SDK | SDK brings gRPC + protobuf + ~30 transitive deps; codebase is stdlib-only |
| JWT profile grant over client credentials | Zitadel's M2M auth uses JWT bearer assertion, not client_secret |
| v1 Management API over v2 | v1 endpoints are stable and better documented for grant operations |
| 5-minute refresh margin | Prevents edge-case token expiry during in-flight requests |
| Double-checked locking in token manager | Prevents thundering herd while allowing concurrent reads of cached token |
| `ListUserGrants` uses global search endpoint | Zitadel v1 documents grant search at `/users/grants/_search` with `userIdQuery` filter, not at a user-scoped path |
| Flat `ZitadelUser` type with nested parsing | Consumer code gets clean flat fields; parsing complexity is contained in `GetUser` |

## Verification

```bash
cd backend && go build ./...                        # Compiles
cd backend && go vet ./...                          # Clean
cd backend && go test ./...                         # All tests pass
cd backend && go test ./internal/zitadel/... -v     # 22/22 passing
```

Smoke test (with credentials):
- Set `ZITADEL_DOMAIN` and `ZITADEL_MACHINE_KEY_PATH` → logs `[ZITADEL] Management client initialized`
- Webhook triggers `EnforceMappingRules` → real Zitadel API calls

Smoke test (without credentials):
- Unset both → logs `[ZITADEL] M2M credentials not set; operating in local-policy-only mode.`
- All existing behavior preserved
