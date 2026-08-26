# What shipped against the boards

Read with `BUILD-NOTES.md`, which is the reply read against the code. This is
the other direction: the boards read against what was built.

Branch `feat/target-plane-redesign`, unmerged.

## Built

| Figure | Where it lives |
|---|---|
| **M1, M2, M5** version band | `components/targets/VersionBand.tsx` — three readings, one shape, and the enumerated unpublished edits |
| **M3** rehearsal + scope step | `components/ui/RehearsalDialog.tsx`, at **rung 2** |
| **M4** the validation pair | the plan's consequence carries `value_checked`; the refusal names near misses |
| **M6** the census handoff | `MappingCensus` on the target page → `/system/targets/{target}/mappings` |
| **M7** the rollback plan | `rehearse-rollback`, cohort over the union of both sets |
| **T1–T4** the four regions | `components/targets/Region.tsx` + `TargetOverview` |
| **T5** the touch form | `RegionIndex` — five rows, always five, each a jump |
| **B1** a decision already taken | `AlreadyDecided`, accent, quoting the other operator's reason |
| **B2** a lifecycle change that did not land | the band's footer strip |
| **C1–C3** the member's pause | `Paused` in `MyStorage` |
| **S1–S3** connected systems | shipped earlier; the neutral reading landed with it |

## Changed on the way, and why

**M3 is rung 2, not rung 3.** The board draws the number typed. The component's
own header argued the opposite before either board existed — copying digits
trains an operator not to look, and rung 3 is reserved for taking access from a
person somebody named. The operator ruled for the code. What did change is that
the step was a rung BELOW what it claimed: a warn panel and a button carrying
the count, where the ceremony has to be an act about the number.

**M3's refusal is 422 `COHORT_ACKNOWLEDGEMENT_REQUIRED`,** not the
"409 · COHORT_LIMIT" in the caption. The surface branches on the code and never
on the status, so the label was never load-bearing.

**M6's people count is not rendered.** The board states how many people hold the
mapped roles. Nothing returns distinct people across roles, so counting it there
is one request per row and then a union — and two mappings on one role would be
added together and overstate it. Left out rather than approximated.

**The band has no "backed off" figure and neither does the build.** Described in
the reply's section 7, in words, because the deployment drawn has no target in
that state. The reading exists in code and has never been seen.

## Not built

- **M1's per-row "TrueNAS recognises this group"** — the check runs at edit
  time, not per row on load. Rendering it per row means one call per mapping on
  every page view.
- **M5's "two accounts already on the NAS are unaffected"** card — needs the
  inventory read on a screen that does not currently take one.
- **§29's populated dormant card and its bulk action** — already shipped, not
  redrawn, and unchanged by this work.

## Four exceptions that made the copy untrue, and are now closed

The mapping flow is this branch's safety story, and it had four holes that each
made an otherwise careful sentence false.

**The rollback rehearsal could not be applied, and the endpoint could be
bypassed.** The rehearsal returned a transient plan with no `plan_id`; the
shared dialog disables Apply without one, correctly — so every rollback that
reached anybody was a dead end on screen, while the endpoint would still change
the mapping set for anyone calling it directly. A ceremony only the UI performs
is a suggestion, and one the UI cannot complete is worse than having none. The
rehearsal issues a real approval now, bound to the target and the version, and
the apply spends it under the same lock the cohort is read under.

**Creating a mapping silently changed access.** Edit and delete cite an approval
and queue a convergence per holder; create wrote the row and stopped. Because
entitlements are DERIVED from mappings, the row alone changed what every holder
was entitled to — and nothing else would have found them, since the periodic
reconciler walks existing bindings and a person never bound to that target is in
no list it reads. Create now takes the same path: rehearsal, citation,
transaction, convergence.

**`DELETE` discarded its decode error.** The tolerance was written for an empty
body — a mapping nobody holds needs no citation — and tolerated far more: a
misspelled key decoded to an empty struct and was acted on as though no plan had
been cited, on the one endpoint in that file that removes access. The empty body
is still tolerated and nothing else is.

**Two publish controls, and they disagreed.** The band and the history panel each
owned a note field and a Publish button; the panel refused a blank note and the
band published with one. One control now, and the backend refuses a blank note
outright — the note is the only record of why a set was right, and its whole
reader is somebody months later deciding whether to roll back to it.

## Gaps this work opened or found

All three that were open are closed.

**The create form exists.** `AddMappingDialog` composes project, role, field and
value, rehearses through the shared dialog, meets the cohort ceremony if the
backend refuses for size, and applies the plan it showed. The field is a text
box prefilled with `group` rather than a list: the add-on's schema is the
authority on which fields exist, and a hard-coded list here would be a second
definition that goes stale the day an add-on declares a third.

**The requests queue has a visible end.** It rendered every row it was given,
where the drift queue and the people list both cap at a page. The cap is on
rendering only — selection stays scoped to the whole pending queue, so the
bar's ceiling message keeps counting what it always counted.

**000045 verified against the live schema.** A throwaway clone of production's
schema (at 44) took the migration cleanly, the widened constraint refused a
decision with no reason and accepted one with it, and the down migration
restored the narrower constraint and dropped the column without disturbing a
row that already carried a decision. The probe was dropped; production is
untouched at `44 | not dirty`.

**It still has to ship with its backend.** The constraint requires a reason, so
the new code against the old schema refuses every decision, and the old code
against the new schema writes decisions the constraint rejects. One deploy.

## Found on the way, not fixed here

- **`BulkOp` never contained the mapping surfaces**, though they have spoken
  that shape since they were rehearsed. Widening it broke an exhaustive switch,
  which is the compiler pointing at what the type had been hiding. Fixed, and
  worth remembering as the shape of that class of bug.
- **`RequestsScreen`'s field hints were inside their labels**, making each
  control's accessible name its title plus a paragraph. Fixed in the new form;
  the pattern may exist elsewhere and has not been swept for.
