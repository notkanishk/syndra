# Zitadel is the truth. Syndra's tables are the intent.

Stated by the owner after a send that reported success and delivered nothing:

> if i press send to zitadel, it should actually send. how is it that every ui
> element but one showed that its not in zitadel. the truth is always zitadel.
> do not do any predictive ui changes. base it all off of zitadel ack of the
> successful execution.

## The rule

**No surface may state that somebody HAS access on the strength of Syndra's own
records.** Those records say what was decided. Whether it is true is a question
only the directory can answer, and the answer has to be asked for.

Three states, never two:

| Zitadel says | The surface says |
|---|---|
| the role is there | the access, plainly |
| the role is not there | it is not there — and why, if the queue explains it |
| it could not be asked | it could not be asked |

The third is the one that gets collapsed, and collapsing it is the expensive
mistake: an outage rendered as an absence sends somebody to re-grant access
that was never missing.

## What this forbids

**Predictive rendering.** A mutation that has been accepted is not a mutation
that has happened. `202 pending` means a row was written; the screen may say a
change is queued, and may not say the person now holds anything.

**A cache standing in for the directory.** The local grant index is a
convenience for the drift sweep, not evidence about the present. It was
consulted to decide whether a grant needed sending at all — the defect this
document exists because of.

**A truth gated behind a view toggle.** The person page fetched Zitadel's grants
only in Advanced, because the answer had been an id to quote in a ticket. It
had become the difference between a true row and a false one, and Basic was
rendering the false one.

## What it costs, and why that is the right trade

One live read per person-page load, and one per outbox row at drain time
instead of a cache lookup. That is the price of the page being right. The
optimisations removed here each saved a call and bought the possibility of
saying something untrue about somebody's access.

## Where it is not yet applied

Role-holder counts, project role counts and the dashboard tiles are still
derived from Syndra's tables alone. They are counts rather than claims about a
named person, which is why they are further down the list — but a count that
disagrees with the directory is the same defect at lower resolution, and this
rule reaches them next.
