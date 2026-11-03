# Changes nobody can confirm, and passwords that settle themselves

## Why
A change sent to a connected system whose answer was lost (`indeterminate`) may or may not have happened, and its parameters were never kept, so nothing can send it again. Until now no surface showed these rows, and a member whose password reply was lost was told to "check" with nothing to check against. Design answered both: A-09 §3–5, revised by A-13 §4–5 (owner, 3 Nov).

## What
- **Operator (A-09 §3–4, A-13 §5).** Each system's page leads Waiting on you with its unconfirmed changes, newest first: what was asked, an amber "Not confirmed", when, who asked, and one verb, "Say what happened". The dialog says how to tell for the kind, offers two choices, and records the answer (nothing is sent). Home's "Not confirmed" block counts them per system. Passwords never appear (`password.set` settles itself; `password.rotate` belongs to its revoke). Records never claimed settle as `unreached`; `setting.put` settles from the add-on's own record of the setting.
- **Member (A-13 §4).** A lost reply is sent again once, at once, with the same password, by the page (the only place the value still is). "Changed" shows only on a confirmed `set`. Unanswered twice: an amber line, and the form stays open with what was typed. Back later: one line in the Storage password row until the next confirmed change, which settles every earlier unconfirmed one.
- **Record.** Migration 000070: `about`, `attempt`, `retry_of` (unique), `resolution`, `resolved_by`, `resolved_at`, `superseded_by`. `status` stays what the dispatch learned; the answer is written beside it. Audit rows join their operation so Activity tells one change with both sends.

## Not changing
- The server never retries a member's password and keeps no copy.
- `SettleAddonOperation` still moves only from `dispatching`; the operator's answer has its own path and never writes `succeeded`.
