# One path — design

## W1a · Zitadel screen pipes

### Roles: `writeRole` (`backend/internal/services/roles.go`)

```
trace tx (access lock) ── local(ctx) + audit(role.created|updated|deleted)
        │ commit; no trace, no write
        ▼
send(ctx, zitadel.MgmtClient)
        │ error?
        ▼
readBackRole (detached ctx, 15s; first page of ListProjectRoles)
        │ landed(found)?  yes → done (log only)
        ▼ no, or the read-back failed
undo tx (access lock) ── undo(ctx) + audit(role.not_changed, {action, error, read_back_error?})
        ▼
ErrRoleNotChangedInZitadel
```

- **Trace.** The `roles` row is Syndra's record of the role, and the audit row is written in the same transaction. A role-only write has no outbox. The call is synchronous, so the transaction commits before the call, the same way key rotation does (`dbStartActionKeyRotation`).
- **Read-back on every error, not only on ones that look ambiguous.** A definite 4xx reads back as unchanged and is undone. A 503 or a timeout that landed reads back as landed and is kept. One rule is simpler than classifying errors, and it is correct for both.
- **Undo when the read-back itself fails.** Syndra cannot tell whether the change landed. It puts its own record back and says so in the `role.not_changed` details (`read_back_error`). The worst case is a Zitadel role with no local row. That case is visible: the catalogue lists it with source `zitadel`, and the next attempt reads it back as landed.
- **No client** (local-policy-only mode). The record is written and audited, and nothing is sent. This is what `CreateRole` already did, now true for all three operations.
- **Cache.** After any change, `directory.Default.InvalidateProject` runs, so the next catalogue read sees it. A role carries no per-person claims of its own: a delete is refused while Syndra gives the role, so no compiled claims cache holds it.

### Delete guard (`roleDeleteGuard`)

It runs inside the trace transaction, so the count and the write sit under one access lock.

| Condition | Answer |
|---|---|
| `RoleMembers(...).Members` non-empty, or a live bundle or rule names the role (`GetRoleUsageCounts`) | `RoleInUseError`, 409 `ROLE_IN_USE`. A confirmation never overrides it. |
| No observation, last observation failed, or it was capped | `RoleHeldError{Unknown}`, 409 `HOLDERS_UNKNOWN` |
| `len(ObservedOnly) != confirm_holders` | `RoleHeldError{Holders}`, 409 `ROLE_HELD` with `details.count` and `details.holders` (names, alphabetical) |
| otherwise | proceeds. The audit row's `holders_lost` lists their ids. |

The UI never counts the holders itself. It shows the refusal's names and count, and a tick (`AcknowledgeCount`) arms "Delete and revoke it for N people", which resends with `confirmHolders: N`.

### Grants: Zitadel › People (`backend/internal/handlers/discovery.go`)

| Route | Pipe |
|---|---|
| `POST …/grants` | `canGiveRoles` (`checkRoleOffered` + `dbRequestHolding`, all roles before any write) → `giveRoles` (one `grantDirect` add per role) |
| `PUT …/grants/{grantId}` | diff against the grant's roles → `canGiveRoles(adds)` → `removeRoles(removes)` → `giveRoles(adds)` |
| `DELETE …/grants/{grantId}` | `removeRoles(all roles on the grant)` |

`removeRoles` handles each role one of three ways:
- **direct grant** → `services.DeleteDirectGrant`, which uses the closure delta before and after;
- **no direct grant, but `UserExpectsRole`** (a bundle or a rule) → kept, reported in `retained_roles`;
- **nothing in Syndra gives it** → one traced `grantDirect` revoke.

`finishZitadelGrantChange` is the person page's tail: `rebuildUserCacheDetachedFn`, then `svcDeliver(ids, apply != "true")`. Remove is sent with `?apply=true`, as People › Access removes. Give and replace are not, as People › Access gives.

The response is `{outbox_ids, given_roles, revoked_roles, retained_roles, status, wait_reason}`.

**Known ceiling** (`ponytail:` in code): `UserExpectsRole` is read outside the access lock. W3a moves it under the lock.

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
