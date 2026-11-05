# One standard path per operation: design

## W1d · Retries (`backend/internal/extcall`)

**The rule.** Nothing is sent twice unless sending it twice is harmless.

| Outcome of a try | Read (`Idempotent`) | Write |
|---|---|---|
| 2xx, or any final status (4xx except 408/429) | returned | returned |
| 429 | retried with backoff | retried with backoff (the other side did not process it) |
| 408, 5xx, timeout, broken connection | retried with backoff | **ambiguous**: read-back if the caller gave one, otherwise returned unretried |
| caller's context cancelled | stop | stop |

- **Backoff.** `min(Max, Base·2^n)` with equal jitter, uniform in `[d/2, d]`. Half the wait is guaranteed, so spacing never collapses; half is random, so callers that failed together do not retry together.
- **Read-back.** It is called once, after an ambiguous outcome. "Landed" returns a synthetic 200, since the caller asked for an effect and the effect is there. "Not landed" or a read-back error returns the original failure, and the write is still not resent: a timed-out request may yet be processed.
- **Timeout.** A timeout applies to each try. Its cancel rides on the response body, so a caller can still read the answer after `Do` returns.
- **Logging.** A request id per call goes out as `X-Request-Id` and is logged under the caller's subsystem tag (`[ZITADEL]`), so the log vocabulary does not grow.

**Zitadel client.** `idempotent` covers GET plus the search POSTs (`/_search` in v1, `/search` in v2). The policy is three tries, 100ms doubling to 2s, and 10s per try. A 401 refreshes the token once per call, on any try. The read-backs are:
- `AddUserGrant`: does the user's grant on the project carry every role asked for?
- `AddProjectRole`: does the role key exist?

Both conclude only from a complete read.

**Outbox.** `propagation_outbox.not_before TIMESTAMPTZ NULL`, where NULL means claimable now. `RequeuePropagation` sets it in the same statement that increments `attempts`. The Go schedule (`extcall.Backoff` for 0..15 spent tries) goes in as an array and the row indexes it by its own count, so the count and the wait cannot disagree. The policy is 1m doubling to a 30m cap, roughly 1+2+4+8+16 minutes across the default budget of five. `ClaimPendingPropagations` and `ClaimPendingRevocations` apply `notBeforeDue`. `ClaimPropagationByID` does not: it is an operator's explicit apply of one row, and the budget still bounds it. The drain's existing exhausted branch (`drain.go:334`) is unchanged and lands the row in `failed`.

**The drain's classification** (`classifyZitadelError`) uses `extcall.RetryableStatus` for its transient set and keeps its own rule that a 409 on an add means applied. Its truth table is unchanged.

**Open (other waves).**
- `services/roles.go:80-84` still rolls back when the read-back cannot confirm the role. That is W1a's.
- The add-on transport's own retries are W1e's to move onto `extcall`.
- `PutSetting` has no idempotency key (family 2).
