# ActionOutcome · placement="seat" (proposed variant)

## Purpose
Report an action's result inside the column the action was taken from, without changing the row's shape. `placement="inline"` renders a tone pill ("Applied") plus a sentence as a full-width flex item; inside a table row that shifts every column and wraps, and it labels a decline "Applied".

## Props
- `outcome`: existing shape (`kind`, `message`, `detail?`).
- `placement="seat"`: new value.
- `word`: the decision in the row's own vocabulary ("Approved", "Declined", "You withdrew this"). Required for seat.
- `className`.

## Renders
Two lines, left-aligned, in the seat the buttons occupied (Requests: 196px):
1. `StatusDot` + `word`, 13.5px/600.
2. Optional, 12.5px, indented under the word: where the result went, in the verdict vocabulary (2b).

## States
- applied, sent not confirmed: healthy dot · "Sent · not yet seen in Zitadel" (faint).
- queued for the operator: warn dot · "Waiting for you to send →" (violet link to Pending changes). Never the applied tone (#96, Locked).
- applied, nothing external: neutral dot · "Nothing was sent" for a decline.
- refused / failed: danger dot · the refusal in one line; failed keeps the request-id copy row beneath.

## Behaviour (unchanged, Locked #96)
The row keeps its seat until the next read; no toast; the result never travels; `role="status"` (`alert` for failed/refused).

## Used by
Requests (operator and member), and every row action that reports in place: People, a person, Drift, Expiring access.

## Block form with a word (A-04 §11, 17 Oct)
The dialog result uses the same `word` instead of the "Applied" pill: dot + word in --font-display 22px/600, one sentence in ink 15px, one faint 13px line. Tone: healthy done now, warn queued (adds Waiting for you to send →), neutral asked / not sent, danger failed. Footer: Close only. Board: route-asking (Results).
