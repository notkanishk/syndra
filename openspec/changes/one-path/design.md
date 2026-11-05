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
