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

---

# One pipeline per fact

Stated by the owner after the counts were found disagreeing:

> always have singular source of truth. 2 elements using two process and sources
> to display same or similar data is ridiculous. specially something so core and
> sensitive as successful role assignment. keep one pipeline for sending and ack
> from zitadel. similarly for other processes, if one pipe is made, use that same
> pipe. morph the data or the pipe, but do not have multiple pipes that lead to
> multiple different contradictory states. someones wellbeing can depend on what
> they can accidentally get or cannot get access to.

## Three facts, and they are not the same fact

Collapsing these would be as wrong as duplicating them.

| Fact | Answered by | Pipe |
|---|---|---|
| What was **decided** | Syndra's own records | `collectUserRoles` / `accessSnapshot` |
| What is **true** | Zitadel | the live grant read |
| What is **owed** | the outbox | ledger → outbox → drain → ack |

The rule is one pipeline **per fact**, and — the part that was actually missing —
**no surface joins two of them itself.**

That join is where the contradiction lived. The person page fetched intent and
reality and joined them in the component, per PROJECT, so a role missing from a
grant that existed still read "Granted". Two components joining differently is
two pipes wearing one name.

## What was removed

`GetEffectiveUserCounts` was a second implementation of the decided fact: a
UNION over the ledger and the bundle tables, counting `DISTINCT user_id`. It
could not agree with `collectUserRoles` and did not, in the field — a role listed
with "4 holders" whose own page said "0 people hold this role".

It was not a bug in the SQL. The SQL could not see what the other pipe can: it
credited ledger rows for people the directory no longer returns, and it resolved
no mapping rules at all, which its own comment admitted. Two implementations of
one derived fact cannot be kept in step by care.

Deleted, not deprecated. Deletion is the only proof that one is left.
`services.RoleHolderCounts` is now the single entry point, and
`TestOnlyOneThingCountsRoleHolders` fails if a second appears in SQL. The guard
judges per QUERY rather than per file, because counting the people **assigned a
bundle** is a different and safe fact — that table IS the record, so a direct
count of it cannot disagree with anything.

`grantIndexHasRole` had already become dead code when the drain stopped treating
the cache as evidence. It is the pipe that caused the failure this document
opens with.

## The cost, stated plainly

`/roles` now builds an access snapshot — one directory listing, plus a
`collectUserRoles` per user, memoised for the request — where it previously ran
one cheap query. At this deployment's documented ~200-user scale that is the
right trade against a count that can be wrong.

Past that scale, the answer is to cache the snapshot across requests, **not** to
reintroduce a second way to count. The guard exists to make that the path of
least resistance.

## What is still two pipes

Reality is read two ways: a live per-user grant list, and a capped whole-org
pull for the reconciliation diff. They answer the same question at different
scopes and neither can be folded into the other without paying the other's cost
on every render. They do not currently contradict each other on screen. Written
down here so the next person meets it as a known and bounded thing rather than
as a surprise.

## Project role counts: the same defect, one fact lower

`/projects` counted roles **with a holder** (`ListProjects`, built by walking
every user's `accessSnapshot` and collecting the role keys that turned up); a
project's own detail page counted roles **that exist** (`GlobalRoleCatalog`,
filtered by project). Both rendered under the word "roles". A project with
three roles nobody had been granted yet read "0 people · 0 roles — nothing here
can be granted" on the list and "3 roles" on its own page — the false sentence
was the harm, since three grantable roles is the opposite of nothing.

Fixed by making `listProjectsFromSnapshot` build the same union
`GlobalRoleCatalog` does — directory-declared roles (already fetched for the
page, free), Syndra-local roles (`svcDbGetAllLocalRoles`, one bulk read), and
roles referenced by a bundle or rule but declared nowhere else
(`svcDbGetAllReferencedRoleKeys`, one bulk read) — instead of re-deriving
"roles" from who currently holds them. Two bulk queries added, not one query
per project; no fan-out. `ProjectSummary.ActiveRoleKeys` (roles-in-use) is
gone, replaced by `ProjectSummary.RoleKeys` (roles that exist) — the second
number was dropped rather than kept alongside the first, since nothing reads it
under its own name today.

`ListProjects`'s holder-derived count and `GlobalRoleCatalog`'s catalog-derived
count are now the same query result read two ways, not two implementations of
"roles" that happened to agree until they didn't.

Role-holder counts (per role, "N people hold this") and the dashboard tiles are
still open — this closes the project-role-count instance of the pattern, not
every instance of it.
