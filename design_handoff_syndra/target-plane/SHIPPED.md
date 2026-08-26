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

## Gaps this work opened or found, in the order they matter

1. **The rollback apply does not cite its plan.** Edit and delete do, and get
   `PLAN_STALE` protection; rollback recomputes its cohort at apply time, so the
   act stays correct and the number an operator approved can differ from the
   number that converges.
2. **`ResolvesValue` fails open and now says so — but only on the rehearsal.**
   The create path validates the same way and its response carries no
   `value_checked`, so a mapping created against an unreachable target says
   nothing about the check not having run.
3. **000045 is unapplied.** `decision_reason` is required by the constraint the
   migration adds, so a deployment running the new backend against the old
   schema refuses every decision. It must go out together.
