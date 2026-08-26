# Build notes — the target-plane reply, read against the code

Written after reading the repo. **This file overrides `CLAUDE-DESIGN-REPLY.md`
where the two disagree**, because the reply was written without the code in view.
The design reasoning stays in the reply; the boards stay in `design/`.

## What arrived, verified

| Check | Result |
|---|---|
| Figures against `FIGURES.md` | 20 claimed, 20 present — `T1`–`T5` (5 figures) and `S1`–`S15` (15). No mismatch. |
| `support.js` | Byte-identical to `../design/support.js`, referenced as `./support.js`. Boards open standalone. |
| `design/Sidebar.dc.html` | Byte-identical to `../design/Sidebar.dc.html`. **Kept, not deleted** — `T1` and `S1` import it, and a copy two folders up does not satisfy a relative import. |
| External loads | Google Fonts only. |
| Archive layout | Four paths at the root, no wrapper. |

Ids were restamped from `1a–1e` / `2a–2o` to `T1–T5` / `S1–S15`, consistently
across badge, anchor, caption and cross-reference. Section 7 of the reply lists
six things described and never drawn, so nothing in the prose reads as a
handed-over drawing. Both of those are the discipline that was missing last
commission and had to be reconstructed afterwards.

---

## 1 · One drawn promise the backend refuses — fix the copy, not the backend

Open question 8, and it is the only place where building what is drawn would
break something.

The decided-and-waiting row says:

> "deciding again replaces the queued work; it does not stack"

**The backend does the opposite, deliberately.** `db.RecordMergeDecision`:

```sql
UPDATE target_merge_findings
SET decision = $3, decided_by = $2, decided_at = NOW()
WHERE id = $1::uuid AND resolved_at IS NULL AND decision IS NULL
```

A second decision matches no row and returns `ErrMergeFindingDecided` —
*"already decided as X by Y"*. The guard is not incidental; its comment says why:

> Without it this was an unconditional overwrite: a second request could replace
> a decision whose work was already queued, and for `unbound` that meant
> releasing the account on the target while a re-provision sat in the outbox.
>
> One writer wins and the loser is told so. Fail-closed rather than
> last-write-wins, because the two answers here are opposites.

So the row's sentence has to be inverted. It is not "deciding again replaces the
queued work" — it is **"this is already decided, by whom, and as what."** The
drawing's instinct was right that an operator needs to know what a second press
does; the answer is that there is no second press, and the surface should say who
got there first rather than implying the operator can change their mind.

Two consequences for the figures: `S7`'s decided-and-waiting row needs the
already-decided sentence, and it needs a *refused* state, which no figure has.

**Not to be resolved by relaxing the backend.** The two resolutions this guard
sits between are recreate-the-account and stop-managing-it, and taking the second
request silently would make the outcome depend on which HTTP request arrived
last.

---

## 2 · The thirteen open questions, answered where the code answers them

| # | Question | Answer from the code |
|---|---|---|
| 1 | Is one control staying live during an outage acceptable? | **Yes — and the drawn reason is wrong.** Maintenance does not edit Syndra's own record: `addons.SetLifecycle` is `POST {addon}/lifecycle` over the network. It stays live for a better reason — that call is **deliberately exempt from the breaker**, because "letting a refusal here count towards opening the circuit would conflate *we told it to stop* with *it stopped answering*". Keep the control live; it needs a **failure path the drawing does not have**, since the call can be refused. Rule 4 gets a stated exception rather than a silent one. |
| 2 | Do the health snapshot and the account list have separate ages? | **Separate.** `TargetHealth.snapshot_taken_at` and `TargetInventory.read_at` are different fields from different reads. Two strips is the honest drawing. |
| 3 | Where do the log-anchor and binding-conflict findings come from? | **The health read, exactly as drawn.** Both are fields on `TargetHealth` — `log_anchor` and `binding_conflicts[]`. Moving them into region 1 changes nothing about what the page fetches. |
| 4 | Should the census line name the mappings or count them? | Design's call; the data supports either. The reply's own reasoning — that naming makes the line the one thing on the page that changes shape with its data — is the argument, and it is consistent with the rule. Keep the count. |
| 5 | Does the dashed control now carry two meanings? | **It would collide.** In this codebase dashed already means *produced by an automatic rule* — a dashed chip on a role, a dashed edge in the access map (`app/graph/page.tsx`, `app/policies/page.tsx`). It is a provenance idiom, not a disabled one. "Off right now, with a reason" must use the established pattern: reduced-alpha disabled plus the reason as body text in place, per rule 1. The dashes stay with automatic rules. |
| 6 | Is the pool at 94% Syndra's conclusion to draw? | **No — and it does not have to be.** `PoolStatus.warning` arrives verbatim from TrueNAS's own `pool.query` (`addons/truenas/operations.go:874-885`), alongside `healthy` and `status`. Render the target's flag. Computing amber from `free/size` would be Syndra publishing a conclusion the target declined to publish, which is the one thing the two-cards split exists to prevent. |
| 7 | What routes exist behind *apply it again* and *stop wanting it*? | Not answered here — the drift screen's own resolution set needs reading against these two labels before either is built. Stays open. |
| 8 | Does deciding twice replace the queued work or add to it? | **Neither. It is refused.** See §1. |
| 9 | Is the half-finished unbind repaired through the same resolve call? | **Yes, and it is idempotent, exactly as drawn.** The handler recognises a repair as `finding.Decision == req.Resolution && req.Resolution == ResolutionUnbound` and re-runs the release. The row's copy — "pressing it again changes nothing on the target" — is accurate. |
| 10 | Which readings on the index can co-occur? | Genuinely open. The health card already resolves nine readings by rendering all that apply rather than picking one; the index row has space for one. Precedence has to be decided, and transport-unreadable above everything is the only ordering the code implies. |
| 11 | What does a sweep with nothing owed return? | **It distinguishes them.** The reconcile result carries `bound`, `queued`, `current` and `stale`; `current: false` is a sweep that did not read the target. `current: true, queued: 0` is read-and-owed-nothing. The card's copy is safe. |
| 12 | Does a finding always name a person? | **Always.** `SubjectID` is required and validated non-empty when a finding is recorded. The roster row's finding count has somewhere to point for every finding that exists. |
| 13 | Is maintenance-stays-live consistent with what the member sees? | **No, and this is a real gap the drawing found.** `MyStorage` and `useMyStorage` carry no lifecycle field at all, so a member reads nothing when an operator sets read-only. Neither backend nor UI has it. Genuinely new work, and out of scope for these boards. |

---

## 3 · Where the design is better than the brief, and the brief was wrong

Recorded because the brief is in this folder and will be read again.

- **The two findings do not belong in the Health card.** The brief placed them
  there because that is where they render today. The reply's argument is correct
  and the code agrees: neither is a fact about reachability, both stand whether
  or not the add-on answers, and both wait on a person. That they arrive *in* the
  health payload (question 3) is a transport detail, not a reason to render them
  as readings.
- **"Nine of eleven panels have nothing to say" was wrong**, and I wrote it. Six
  still speak during an outage — the reach card, the census, the manifest, the
  maintenance state and both Syndra-side findings — and the last state read is
  content, not absence. `T3` is the figure that depends on that, and it is right.
- **Health and maintenance are one question.** *Not answering* while somebody is
  draining it for a credential rotation is a different fact from *not answering*
  at 04:00, and the built page has them six panels apart. This is the strongest
  structural finding in the reply.

## 3a · The six proposed components, against `components/ui/`

Reply section 6 proposes six additions. Read against the tree before anything is
built, which is the check the last commission skipped.

| # | Proposed | Verdict |
|---|---|---|
| 1 | Three-state block | **New — build it.** Nothing compares three values with the differing pair marked. It replaces prose: `MergeFindings.tsx` currently says *"It was ["makers"] when Syndra last saw it, and is [] now"*, which is the same content as a sentence and loses the comparison at a glance. |
| 2 | Count chip, three forms | **Extend `Badge`, do not add a component.** `Badge` already has `hollow` for the zero form. It needs one more form — the em dash for a failed read — and that is a prop, not a peer. |
| 3 | Freshness strip | **Already exists. Do not build it.** `components/ui/ReadFreshness.tsx` is exactly this: a tone dot, a sentence carrying an age, a truncation clause, and a `Read again` that renders only when the read is stale. The reply says it considered "the amber banner inside §21's unmanaged inventory, which is where this behaviour currently lives" — it did not know the shared component exists, because it could not. **This is this commission's version of the §23 mistake**, caught before it cost anything. |
| 4 | Claimant pair | **New — build it.** The existing dialog shape pairs a recommended action with a quieter one, and every instance has a preferred answer. The reply's argument is right: a preference here would be the design deciding the thing its own copy says it cannot. |
| 5 | Neutral reading dot | **Extend `STATUS_TONE`.** It carries healthy / accent / warn / danger; this adds `neutral` at `--faint`. One line, not a component. |
| 6 | Region index | **New — build it**, touch only. |

So: two genuinely new components, two one-line extensions of existing ones, one
touch-only addition, and one that must not be built at all.

## 3b · Two answers sent back before the mapping zip

Both were flagged by the reply as the hardest open questions of commission 3.
Both are settled by the code, and the first corrects an error in my own brief.

### A rollback does not rehearse. Nothing does it per mapping either.

The brief told Claude Design that "edit, delete and rollback all rehearse before
they land". **Rollback does not.** There is no rehearsal endpoint for it:

```
POST /targets/mappings/{id}/rehearse-edit      ← exists
POST /targets/mappings/{id}/rehearse-delete    ← exists
POST /targets/{target}/mappings/versions/{version}/rollback   ← no rehearsal
```

`handleRollbackMappingVersion` restores the version and calls
`rollbackAndConverge`, returning `queued_convergences` — a convergence queued per
affected holder, unrehearsed and with no cohort acknowledgement.

I did not invent the claim. `useMappings.ts:14` and `MappingManagement.tsx:40`
both assert it in comments, and I repeated them without checking the routes.
**That is pre-existing drift in the repo**, not a design problem: two file
headers describe a rehearsal the API never grew.

So the honest answer to *does a rollback rehearse as one plan or one per
mapping* is **neither, today**. Which of the three it should become is a product
decision, and it is now the more interesting question:

- one plan for the whole version is the only shape that matches what a rollback
  *is* — a set restored together — and it fires the cohort ceremony once;
- one per mapping would fire it up to once per row, for a single act;
- unrehearsed, as now, is the only option that contradicts the screen's own
  argument, since publishing is rehearsed and reverting a publish is not.

The two stale comments should be corrected either way, and are not to be treated
as a specification.

### The member payload can tell draining from read-only, and should

`MyTargetView` — what `GET /api/v1/me/targets` already returns — carries
`reachable: boolean` today. Adding `lifecycle` to it is an extension of a payload
the member already receives, not a new read.

**It must not be a boolean.** Three values exist at the source and are distinct
all the way down: the add-on's own `LIFECYCLE_STATE` is `active | draining |
read_only`, `TargetHealth.lifecycle` carries the same three, and the operator's
maintenance strip renders all three. Collapsing them at the member boundary would
be the only place in the system where the distinction is lost, and it is exactly
the place where it changes what somebody should do: under draining a credential
they set will land once the queue clears, and under read-only it will not.

So **C1 and C2 both stand as drawn.**

## 3c · Commission 3 landed, and M7's number is computable

Zip verified: `M1`–`M7`, `B1`–`B2`, `C1`–`C3` claimed and present, 7 + 2 + 3
figures across three new boards. `support.js` unchanged, ids continue cleanly.

### The 71 exists. The backend already computes it.

M7's open question — can the backend produce distinct people deduplicated across
three role cohorts, or only per-mapping holder counts — is answered by the code
that performs a rollback today:

```go
// One convergence per subject, not per mapping: two restored mappings on
// one role would otherwise queue the same resolved set twice.
seen := map[string]struct{}{}
for _, m := range mappings {
    holders, _ := dbMappingHolders(ctx, m.ProjectID, m.RoleKey)
    for _, id := range holders {
        if _, dup := seen[id]; dup { continue }
        seen[id] = struct{}{}
        ...
        converged++
    }
}
```

`converged` **is** the distinct-people number. The dedup M7 needs is not new work
— it is the shape the rollback path already has, and `queued_convergences` in the
response is that count returned to the caller.

A rehearsal needs it computed *before* the write rather than after, and
everything for that exists: `db.MappingVersionEntry` carries `ProjectID` and
`RoleKey` for the version being restored, so the would-be set is knowable without
mutating anything, and `db.MappingHolders(projectID, roleKey)` is the same lookup
the loop already calls. The extraction is the dedup half of that loop with no
convergence recorded.

**So the fallback is not needed. Draw the 71.**

### But today's count is half the cohort — and that is a defect, not a design note

`RollbackMappingVersion` clears the whole working set before reinserting:

```sql
DELETE FROM target_role_mappings WHERE target = $1
```

and `rollbackAndConverge` then iterates `dbListRoleMappings` **after** the
restore. So the loop reaches holders of roles in the *restored* set only.

A person who holds only a role whose mapping the rollback **deletes** is in no
post-rollback holder list. No convergence is queued for them, and their account
keeps what that mapping granted. `TestARollbackReResolvesEveryoneItReaches`
asserts the dedup but stubs one holder list for every mapping, so it cannot see
the difference between the before-set and the after-set.

Two consequences, and they point the same way:

1. **For M7:** the cohort the ceremony must state is the **union of the before
   and after sets**, not the after set. Someone losing an entitlement is as
   affected as someone gaining one, and is arguably the one who should be
   counted most carefully. Today's `converged` would understate it.
2. **For the backend:** this looks like a real gap. The six-hourly sweep should
   catch it — Syndra's desired set moved and the target's did not, which
   classifies as a fast-forward and applies — so it is a delay of up to six
   hours rather than a silent permanent divergence. **That expectation is
   unverified and needs its own test**; it is stated here as the reason this is
   not urgent, not as a finding.

Neither is a reason to hold the drawing. The union is what M7 should say it
counts, and the backend change is the same loop reading two sets instead of one.

## 4 · Still open, and who owns it

- Questions 7, 10 and 13 above.
- The six items in reply section 7, of which the **mapping screen** is the
  substantial one: panels 3 and 4 are removed from the target page and it does
  not exist yet. Nothing should be removed from the built page until it does.
- The reply adds six components to the design system (its section 6). Each needs
  reading against `components/ui/` before anything is built — the three-state
  block and the count chip in particular look close to things that already exist,
  and the last commission's one real mistake was declaring a screen canonical when
  a better component was already in the tree with five callers.
