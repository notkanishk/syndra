# Recorded is not delivered

## What went wrong

An operator assigned a bundle, looked at the person's page, and read it as
confirmation the work was done. It was not: the deployment runs that bundle in
manual mode, so three grants were sitting in the outbox waiting for somebody to
confirm Pending changes. Then they removed the bundle again, and Syndra queued
three *revocations* for access that had never been delivered — six operations
netting to nothing, three phantom entries under Unfinished revocations, and a
modal that said "THEY WILL LOSE ... no other source gives it" about a role the
person had never held.

Every screen involved was internally consistent. Each was answering "what has
this person been given" while the operator was asking "what does this person
have", and nothing in the product distinguished the two.

## The rule this change adds

**A revoke exists to undo a delivery.** If the delivery is still in the queue,
cancel it; there is nothing to undo. If it has left, revoke it.

**A recorded grant is not a delivered one, and no surface may render them the
same way.** Syndra's tables say what somebody has been given — that is what an
assignment means — and the outbox says whether it has been carried out.

## Why the queue was the honest source

The outbox holds a row exactly while Syndra still owes the change to a target,
so "is this delivered" needs no new bookkeeping to answer. The alternative —
asking Zitadel — is answered by the drift sweep on a six-hour cadence, which is
far too old to date a grant made a minute ago and would report a fresh, applied
grant as undelivered for the rest of the afternoon.

## What is deliberately not done

**The revoke is dropped only where nothing could have heard about the grant.**
Skipping one that was needed leaves access somebody asked to remove; queueing
one that was not needed costs a no-op call. Those are not equally expensive, so
three conditions must all hold — a `pending` row (never `in_flight`), from the
same source being taken away, with no `applied` delivery of that grant ever. Any
of them failing leaves the previous behaviour untouched.

**The section headings do not move.** A role whose sources are all queued stays
under Granted and gains a marker. Structure never moves in response to data
(`basic-advanced-ia`); a row that jumped between sections as a drain completed
would be the same defect wearing a fix.
