# PROPOSED — CancelQueuedStep (replaces the CancelQueuedConfirm placeholder)

Status: proposed 17 Oct 2026 (A-04 §13.4, §14). Board: `flow-cancel-queued`. Behaviour is code's, unchanged. This is only the drawing.

## A step, not a sheet
It renders inside the host Modal (Give a role, Remove, a bundle's dialogs) in place of the form. A row action with no sheet of its own (Approve) opens a Modal sm that holds only the step. Sheets never stack.

## Anatomy
- ModalHeader `chip`: the host's name ("Give Rohan Mehta a role").
- **Title** names the kind and count: "This cancels a removal that's waiting" / "This cancels 3 grants that are waiting".
- **Lede**: what it means for the person today: "It hasn't reached Zitadel, so Rohan still has Member today."
- **List**: a `--line-strong` box with one group per who-queued-it, when and why.
  - Each change: **Giving** / **Removing** {system · role} to/from {person}.
  - Then "Queued by {name} · {when}" in muted.
  - Then the reason as a quote in ink, or "{First name} gave no reason." in faint.
  - Up to 5 changes, then "Showing 5 of N · Show all N".
- **New since you looked** (a longer refusal): one line with an accent dot above the list. The new groups come first, `--accent-soft`, with an accent-text label, and flash once.
- **Footer**: the note "Nothing changes until you choose." Then **Keep it** (outline, first, initial focus, Escape), then the confirm, which names what goes ahead in the host's tone: "Cancel it and give Member" (accent), "Cancel all 3 and remove" (dangerConfirm).

## After
- **Go ahead**: the host's result (ActionOutcome block with a word), plus a faint line: "The removal Sam queued 2h ago is cancelled, so nothing goes to Zitadel. Change history records both."
- **Keep**: back to the host's form as it was. The footer note becomes "Nothing changed. {Name}'s {removal} is still waiting." with Waiting for you to send →.

## Motion
Ask: tint → settle → arrive. Keep: the reverse. Longer: settle, then flash on the new group. Confirm: press → the button's spinner → Moment 1.
