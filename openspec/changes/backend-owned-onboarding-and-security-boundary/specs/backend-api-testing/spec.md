## MODIFIED Requirements

### Requirement: Production-boundary regression coverage
The system MUST maintain regression coverage for backend user-token authorization on privileged actions, webhook authenticity, action-injection degraded behavior, and backend-owned onboarding mutation paths.

#### Scenario: Security-boundary behavior changes
- **WHEN** a privileged orchestration path changes
- **THEN** automated tests MUST verify backend authorization, authenticity enforcement, degraded-mode handling, idempotent retry behavior, and operator-visible failure reporting

---

## Implementation notes (Phase 3)

**JWT authorization tests** — `backend/internal/auth/jwt_test.go`
- Valid RS256 token accepted; subject returned
- Audience as single string and as array (Zitadel sends both)
- Expired token rejected
- Wrong audience rejected
- Wrong issuer rejected
- Tampered signature rejected
- Malformed JWT (two-part, empty, garbage) rejected
- Unknown `kid` rejected

**Webhook authenticity tests** — `backend/internal/handlers/webhook_test.go`
- Valid signature (over `tsHeader + "\n" + body`) accepted
- Invalid signature rejected with `401 WEBHOOK_UNAUTHORIZED`
- Replay attack: fresh timestamp with captured-body signature fails (`TestVerifyWebhookSignature_FreshTimestampWithStaleBodySignature`)
- Missing signature header rejected
- Missing timestamp header rejected at signature check
- Stale timestamp (10 min old) rejected with `400 WEBHOOK_STALE`
- Local-dev mode (no secret) skips both checks
- Handler integration: `HandleZitadelWebhook` returns correct codes for bad signature and stale timestamp
