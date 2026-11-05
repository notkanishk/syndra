# One standard path per operation

Source: owner, 5 Nov 2026, and the architecture audit of the same day (twelve families of operations that reach one outcome by more than one code path). Each wave gives one family a single core and moves its callers onto it. Sections are per wave agent; each owns a disjoint set of files.

## W1d · Retries, backoff and idempotency (audit family 11, ranked item 7)

**Why.** Every external call carried its own retry rules, and two of them were unsafe:

- The outbox requeued a row with no time gate (`db/propagations.go:663` on main), so a short Zitadel blip could spend the whole retry budget (`OUTBOX_MAX_RETRIES=5`, `services/propagation/deps.go:209`) in back-to-back passes. With the client's own three tries per call, one row could take up to 15 unspaced attempts.
- The Zitadel client resent any method on a 503 (`zitadel/client.go:518`), including a create whose first try had landed. Role create then saw a failure, rolled back its own row (`services/roles.go:80-84`) and left an orphan role in Zitadel.
- A 401 after a 429 failed without refreshing the token (`client.go:509`, refresh on the first try only).
- A lost answer on a read failed at once with no retry (`client.go:500`).
- The start-up wait schedule was written twice (`cmd/api/main.go:171`, `:209`).

**What changes.**

- New `backend/internal/extcall`: one per-try timeout, one classification (retryable: 408, 429, 5xx, timeout, broken connection; every other status is final, and what a 409 means stays the caller's), one jittered capped backoff, a read-back hook for an ambiguous write, and a request id on every log line.
- The Zitadel client calls through it. Reads are retried. A write is resent only after a 429. Any other failure of a write is ambiguous: `AddUserGrant` and `AddProjectRole` read back, and every other write returns the failure unretried for the outbox to handle.
- The outbox gains `not_before` (migration 000080). A requeue sets it from `extcall.Backoff`, and both batch claims skip the row until then. A row past its budget still lands in `failed`, which the unfinished-revocations surface already shows.
- One start-up wait schedule in `cmd/api/main.go`.

**Not preserving, by intent:** retry timing, and writes are no longer resent inline on a 503.
