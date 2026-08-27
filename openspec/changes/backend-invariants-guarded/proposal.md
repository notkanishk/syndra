# Backend invariants, checked against the source

## Why

The UI sweep of 2026-08-27 found that its rules were real, written down, and
unenforced — and that every one of them had been broken somewhere. The same
question asked of the backend gives a different answer, and the difference is
worth recording: the backend's stated rules were being **followed**.

- 130 routes, every one behind a recognised auth gate
- 78 distinct error codes, not one of them mapping to two HTTP statuses
- no SQL built by string concatenation, anywhere
- no bare `http.Error` bypassing the JSON error vocabulary
- every mutation endpoint decoding strictly, with exactly one argued exception
- `rows.Close()` on every path, `rows.Err()` checked, in both places that
  looked wrong at first glance

What was missing was not compliance. It was **enforcement**: nothing would have
noticed the day one of those stopped being true, and the security-relevant one —
the gate on a route — looks exactly like the 130 safe lines around it.

## What changes

Four source guards, and the handful of drifts they turned up on the way in.

The drifts were all in the same class the UI sweep found: one thing said two
ways. A subsystem writing its log under two tags. Severity baked into a tag so
that grepping the subsystem returned only the lines that went well. Two
dependency seams over one function, so a test that stubbed the package had
stubbed a fifth of it. A handler calling a service directly past the seam that
existed for it.

One finding is larger and is NOT fixed here: the webhook orchestrator writes
Zitadel grants directly, with no ledger row, no outbox row and no audit line.
It is dormant — the path needs a mapping rule to run and production has none —
and fixing it changes a live webhook path, so it is recorded in
`openspec/NEXT.md` and pinned by a guard that lists it by name with its
argument. A second untraced writer cannot now appear the way the first did.

## Impact

No behavioural change to any endpoint. Log tags change for nine lines, which
changes what an operator's `grep` returns — in the direction of returning
everything the subsystem wrote.
