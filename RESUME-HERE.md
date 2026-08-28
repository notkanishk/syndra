# Paused mid-pass — read this first

Stopped at the user's request, 2026-08-28. Nothing is lost and nothing is broken
in production.

## State of the world

| Where | What | Safe? |
|---|---|---|
| **Production** | `48f2c03` — PR #8, *Incoming events* rename. Serving 200. | Yes. Untouched by any of the below. |
| **`origin/main`** | `48f2c03`. Same as production. | Yes. |
| **local `main`** | `e70922c` — the glossary + `<Term>` + rewritten guard. **1 commit ahead, unpushed.** Suite was green (883/883), lint clean, build compiles. | Yes, but unpushed on purpose: pushing deploys, and nobody was watching. |
| **`wip/vocabulary-restore-paused`** | `5dc4deb` — three agents' partial edits, stopped mid-file. **Suite is NOT green. Do not deploy.** | Quarantined off main. |

To resume: `git checkout wip/vocabulary-restore-paused`. To abandon that pass
and keep only the foundation: `git checkout main` and delete the branch.

## What is finished

1. **The writing guide is rewritten** for the real audience —
   `openspec/changes/plain-language-copy/design.md`. §1 (these are university
   makerspace staff, not laypeople), §2 (plain but not simplified), §6 (the
   glossary mechanism, one name per thing, states, pages).
2. **`ui/src/lib/glossary.ts`** — every term defined once, in three kinds:
   products, standard, and Syndra's own coinages. The coinages carry
   `mustDefine: true`.
3. **`ui/src/components/ui/Term.tsx`** + tests — a button (not a hover span),
   definition stays in the DOM when shut so `aria-describedby` resolves.
4. **The guard is inverted** — `plain-language.test.ts`. The ban list shrank to
   second-names and developer noise; a new positive rule requires a coinage's
   definition to be reachable on the page that uses it. Mutation-checked.

## What is half-done

The second pass — deleting inline glosses, restoring precise vocabulary,
marking up first use — got through roughly a fifth of the tree before it was
stopped. The 19 files on the WIP branch are partially converted. Re-run the
brief at `~/.claude/jobs/b48ed467/tmp/copy-sweep/APPLY2.md`, which is still
accurate, and prefer cheap models for it: it is mechanical.

**Known loose end:** `lib/nav.ts` may be mid-rename of *Unexplained access* →
*Drift*. Check that its test and `crumbsFor` agree before trusting the suite.

## What the accuracy sweep found — this is the important part

Three read-only agents checked every claim in the UI against the code.
**25 high-severity findings.** Full detail in
`~/.claude/jobs/b48ed467/tmp/copy-sweep/V1-pages.md`, `V2-timing.md`,
`V3-components.md`. None of it is fixed yet.

### Mine to answer for

**"within about a minute" is wrong in all six places it appears.** In the first
sweep I translated the mechanism word *cache compile* into a specific promise.
I invented the number. There is no one-minute cadence anywhere in the backend:
the revocation drain is **5 minutes** and disableable (`cmd/api/main.go:361-372`),
the drift sweep is **6 hours**, and grant propagation never moves without an
operator pressing Send.

Also mine: *"They are sent together or not at all"* over a cascade —
`DrainBatch` processes rows one at a time and can break mid-cascade; and
*"The failed ones are still waiting under Pending changes"* — `failed` is
terminal and filtered out of that very list.

### Copy that rotted when the code improved

The whole of `app/zitadel/**` warns that changes there "skip Syndra", leave "no
record", and happen "the moment you press the button". The handlers were
rewritten under it (`backend/internal/handlers/discovery.go:241-258`): they now
write the ledger, the audit row and the outbox in one transaction and return
202 `pending`. The page reports `applied` on a 202.

### Real defects, not copy problems

- **`zitadel_reachable` is `MgmtClient != nil`** (`services/deps.go:53`) — a
  config check wearing the words of a probe. The "Zitadel is not answering"
  banner cannot fire during an actual outage, and Send stays enabled.
- **`GetAssignedUserCounts` counts `direct_role_grants` only**
  (`db/roles.go:175-177`). A role forty people hold through a bundle shows
  **Members: 0**, and the rule editor prints *"Nobody holds the first role yet,
  so saving changes nothing today"* immediately before a save that grants it to
  all forty. False reassurance in front of a write.
- **Lifting a hold does not restore access.** `handleLiftAllowance` is one
  `UPDATE allowances SET lifted_at` and returns; nothing queues a convergence.
  The UI says "lifting gives the access back straight away" and reports
  `applied`.
- **`ShadowCredential`'s set flow has no backend route.** The `PUT` endpoint was
  deliberately deleted (`handlers/vault.go:50-53`); the member-facing screen
  still says the password "will work on the workshop machines within a few
  minutes".
- **Halted reads render as facts.** `TargetOverview` prints
  "0 accounts managed · 0 fixes waiting" from a halted pass, one row above its
  own "Nothing was concluded" warning; `PeopleOnTarget` shows "Nobody yet" over
  an inventory read that returned nil.
- The `R_` handle prefix is hardcoded for every row while the legend says
  `R_` = rule, `b_` = bundle.

### The lesson worth keeping

The guard checks words. Every one of the findings above uses permitted
vocabulary and says something false. A word list cannot see a wrong claim —
only reading the copy against the code that produces it can. That limit is
written into the guide at §10 rather than papered over.

## Suggested order when you come back

1. Decide whether to push local `main` (safe, green) or hold it.
2. Fix the six timing claims — smallest diff, largest untruth.
3. Fix the `zitadel/**` copy that describes a deleted code path.
4. Triage the five real defects; each needs a decision, not a rewrite.
5. Then finish the mechanical vocabulary pass.
