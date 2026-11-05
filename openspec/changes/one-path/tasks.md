# One standard path per operation: tasks

## W1d · Retries (branch `fix/one-retry-core`)

- [x] 1.1 `internal/extcall`: Policy, Backoff (jittered, capped), RetryableStatus, NoAnswer, Do with per-try timeout, read-back and request-id logging
- [x] 1.2 Zitadel client on `extcall.Do`; reads retried, writes resent only after 429; 401 refresh once per call on any try
- [x] 1.3 Read-backs on `AddUserGrant` and `AddProjectRole`
- [x] 1.4 `classifyZitadelError` uses `extcall.RetryableStatus` (truth table unchanged)
- [x] 1.5 Migration 000080 `propagation_outbox.not_before`; `RequeuePropagation` sets it; both batch claims respect it
- [x] 1.6 One start-up wait schedule in `cmd/api/main.go`
- [x] 1.7 Tests: backoff spacing and cap, retryable vs terminal, read-back on ambiguous write, outbox respects `not_before`, max attempts to failed (live), down migration (text and live, in a rolled-back transaction)
- [ ] 1.8 Show "next try at" (`not_before`) on the pending worklist. The models and UI are outside W1d's files #ready-for-agent
- [ ] 1.9 Move the add-on transport onto `extcall` (W1e) #ready-for-agent
- [ ] 1.10 Owner: after deploy, watch `[ZITADEL] ... req=` lines during the next Zitadel blip #ready-for-human
