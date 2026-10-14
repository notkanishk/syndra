# PROPOSED — Next step (a step left)

Status: proposed 13 Oct 2026, revised (N16, N17). Rule: a step left is said where you took the first one.

## It is the existing status, not a new box
The thing that changed shows, in its own status seat, the status a person's role row already uses:
- Amber StatusDot + violet link "Waiting for you to send →" (to Pending changes, filtered).
- One faint 12.5px line under it, indented 15px: what isn't true yet and why, by name.
  "He can't supervise yet · Supervisor always waits for you"
  "Zitadel didn't answer at 10:42, so Priya can't print yet"
Where it lives: a person's role row (status seat), a request's decision seat, a rule row (under its sentence), a bundle page's read line, a bundle role row.
Removed when the change is sent.

## Before acting
The dialog's ModalFooter note, one sentence, future tense: "Supervisor always waits for you, so Rohan gets it once you send it." / "Rohan can use it as soon as you give it."

## After acting
ActionOutcome inside the dialog: queued "Given in Syndra. Not in Zitadel yet" / applied "Rohan can use the printers now". Then Done.

## Reasons (outbox wait_reason)
role_waits → "{Role} always waits for you" · halted:zitadel_offline → "Zitadel didn't answer at {time}" · halted:drain_in_progress → "Another send was running at {time}".

## Never
A tinted box, a new colour, sending for the operator, navigating away.
