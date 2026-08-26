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
