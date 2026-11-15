## ADR-007: `account_id` is a key within a read, not an identity

**Date:** 2026-11-15
**Status:** Accepted for the rule and the wire (design 7t, D1-D6). Five points pend the owner: Q1 `account_id` stays the uid, Q2 legacy policy B, Q4 `subject` and `conflicts` core in contract 2, Q5 only `unbound` resolves a conflict finding, Q6 the drain raises the finding too. Change: `openspec/changes/addon-contract` (slices 7r, 7t).

### Context

An add-on reports each account with an `account_id`. Syndra took it for an identity: the backend matched a `/subjects` row to a person by `account_id` or username against its own mirror (`target_account_bindings`), and the add-on found an account for a binding by uid. On TrueNAS that is false. The app's uid is not non-reassignable outside the range the add-on hands out: a freed number comes back to a new account.

That is not hypothetical. `addons/truenas/identity.go:7-12` records the incident: three new accounts were handed the numbers of departed members and so their home folders. A later review found the same defect in four more places, each with its own lookup rule (`boundAccount`, `recordID`, name only, the backend's `account_id`). A second rule is exactly what the incident cost.

RFC 7643 §3.1 names the distinction. `id` is "a stable, non-reassignable identifier" issued by the service provider. `externalId` is the client's own identifier for the resource. Here the add-on is the service provider and Syndra the client. The subject plays `externalId`. TrueNAS's uid cannot play `id`.

### Decision

- **D1, one module.** `addons/truenas/identity_resolve.go` owns which account a binding names, for every add-on caller, live (by subject) and over a read (by index). `boundAccount` is its only rule. **D2:** `recordID` is deleted; every account operation, `storage.status`, `activity.get`, adoption's repeat check and the plan call the module; a conflict is a 409 everywhere and never a write to the holder.
- **D3, the add-on says whose account is whose.** `/subjects` rows carry `subject`, and the reply carries `conflicts[]` with a `reason`, the holder, and the add-on's own `detail` and `next`. The backend stops matching rows against its mirror. The binding table the add-on keeps is the only link from person to account; the backend's copy only feeds the outbox, Home and the UI.
- **D4, `account_id` stays the uid, with a narrowed meaning.** It is unique within one read and survives a rename. Syndra MUST NOT conclude from two reads carrying the same `account_id` that they describe the same account, and MUST NOT attribute an account to a subject by it. An add-on whose app can hand an id out again MUST keep the recorded name beside the number and report a name-gone, number-held binding as a conflict. (The alternative, the record key, would rewrite every stored `account_id`, the stats tokens and every number the UI shows, for nothing `subject` does not already give.)
- **D5, one finding.** A conflict is a `target_merge_findings` row, outcome `binding_conflict`, keyed `(target, subject, "")`. The sweep raises it from `conflicts`, the drain raises the same row from a blocked apply, and the sweep closes it when the add-on stops reporting it. It resolves by `unbound` alone (`account.release`, the add-on's own next step); adopting the holder stays in Not created by Syndra. It adds no table. Zero new tables, one place to decide.
- **D6, legacy policy B.** An unmarked binding outside the range whose name is gone and whose number another name holds halts as a conflict instead of creating a duplicate. The cutover pre-check is a must.

### Consequences

- A hand rename of an adopted account is a conflict with the right next step, not a duplicate account, and a reused number is never attributed to the wrong person (stats, standing, drift, homes).
- Contract 2 grows `subject` and `conflicts` as core, so an add-on older than the backend reads as `addon_outdated` and halts rather than reading every binding as deleted. The add-on ships with or before the backend, from one commit.
- A conformance add-on can no longer lean on `account_id` as an identity; the guide says so.
- The backend mirror is not an identity either (7t-6, migration 000098). Its unique indexes on `(target, username)`, `(target, account_uid)` and `(target, account_id)` are gone, the name's too: TrueNAS reuses a freed name as it reuses a number. Only `(target, subject_id)` stays unique. On a write the add-on's answer is authority: `db.RecordTargetBinding` clears another subject's row on the same account (the same id where both carry one, the name only where either has none) in the same transaction, logged and audited (`binding.<target>.stale_cleared`), never a conflict. `target_binding_conflicts`, which only that refusal wrote, is retired: two live claims on one account are the add-on's to report (`claimed_twice`, the D5 finding). The table stays for one release for a rollback and is dropped with `account_uid` (task 1.7b).
