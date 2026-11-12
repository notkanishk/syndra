# Tasks

## 1. Record (backend)
- [x] 1.1 Migration 000070: `about`, `attempt` + `retry_of` (one resend, unique), `resolution` / `resolved_by` / `resolved_at` / `superseded_by`, an open-by-target index. Guard tests keep the column list closed.
- [x] 1.2 `ListUnresolvedAddonOperations(target)` newest first, claimed rows only, passwords excluded; `CountUnresolvedAddonOperations` per target for the indicators (`unconfirmed_operations`).
- [x] 1.3 `ResolveAddonOperation`: its own path apart from dispatch; answer in `resolution`, audit row (`.answered` / `.read`) in the same transaction; never `succeeded`.
- [x] 1.4 `SettleUnclaimedAddonOperations`: never claimed → `unreached`, by Syndra.
- [x] 1.5 `about` written only for adopt, release, purge and setting.put (`addonop.aboutOperations`); dropped for password operations.

## 2. Operator (A-09 §3–4, A-13 §5)
- [x] 2.1 `GET /targets/{target}/unconfirmed` (settles unsent records and readable `setting.put` first) and `POST /targets/{target}/unconfirmed/{id}/answer` `{happened}` (strict JSON, a person's name required). The answer path takes `/answer` because `/targets/{target}/unconfirmed/{id}` collides with `/targets/mappings/{id}/rehearse-edit` in the mux.
- [x] 2.2 System page: unconfirmed rows lead Waiting on you; the region's count and chip include them; the lede gains "or a change {system} never answered".
- [x] 2.3 "Say what happened" dialog: how to tell per kind, two ChoiceRows, nothing chosen first, the before-line, the result in the sheet; the row leaves on the read after Close.
- [x] 2.4 Home: "Not confirmed" block, one row per system, counted in "N things need you" (Advanced, beside the merge findings, as the system pages are).
- [ ] 2.5 `account.adopt` / `account.release` settled by Syndra from a read. `ready-for-human`: confirmed not possible from what Syndra reads. Both write only the add-on's binding, Syndra's own binding row is written only on a confirmed success, and the inventory read lists accounts, not the add-on's bindings. They stay on Waiting on you for a person until the add-on reports its bindings. *Since addon-contract 6.3r (13 Nov 2026):* on an add-on declaring `calls`, one it recorded is settled from GET /calls together with Syndra's binding row (`db.ResolveBindingCall`); the rest still stay for a person.
- [ ] 2.6 `password.rotate` resent by Syndra with a newly generated value until confirmed, and shown in Unfinished revocations while unconfirmed. `needs-info`: the retry schedule is A-13's open question, and today an indeterminate rotate does not show in Unfinished revocations (that list reads queued convergences only). Excluded from Waiting on you as decided.

## 3. Member (A-13 §4)
- [x] 3.1 The page resends once, at once, with the value it still holds, on a lost reply (network failure, a proxy 502–504, or `indeterminate`); never on a clear answer. The button stays pending across both sends; a slow line at 4 s (`SLOW_ANSWER_MS`).
- [x] 3.2 `"Changed"` only on a confirmed `set`; the field clears only then. Unanswered twice: the amber line, the form open with what was typed, "Last changed" withheld.
- [x] 3.3 `resend: true` on `POST /me/targets/{target}/credential`: the server finds the first send itself (`ResendableAddonOperation`, 5-minute window) and links it; the resend is not rate-limited. A send whose outcome could not be written answers `indeterminate`, never "Nothing was changed".
- [x] 3.4 A confirmed `password.set` supersedes every earlier unconfirmed one (`SupersedeAddonOperations`).
- [x] 3.5 `GET /me/targets` carries `password_change: {state: "unconfirmed", at}`; the row's line until the next confirmed change, and the offline form of it.
- [x] 3.6 The add-on accepts the same password again under a new call id while the first is unresolved (test only; no guard change needed).

## 4. Record, as read (A-13 "In the record")
- [x] 4.1 Audit rows of a send carry their operation (`operation`: status, attempt, retry, supersedes, resolution), joined on read; the audit row is never rewritten.
- [x] 4.2 Activity: one row per change, the resend folded in, a faint sends line ("Sent again automatically at …"). Audit keeps both rows; the resend's own row says so. An operator's answer reads "Said it didn't happen".

## Not run
- The SQL has not been run against Postgres (no live-DB run on this branch). Run the live harness before merging.
