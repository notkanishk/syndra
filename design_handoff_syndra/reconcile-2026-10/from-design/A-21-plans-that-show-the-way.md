# A-21 · Plans that show the way, and single actions that ask before they cancel (draft, 7 Nov 2026)

Answer to the A-21 design brief (planner decisions 7, 9, 11 and 12; `architecture-deepening/tasks.md` 1.2, 1.6, 1.7). Boards: `a21-plan-grouped.dc.html`, `a21-plan-result.dc.html`, `a21-ask-before-cancel.dc.html`, `a21-rule-rows.dc.html` (Proposed, 7 Nov 2026), in `design/`; §8 says how they read. Nothing on the canvas drew a plan before them. Read-only; nothing was published, edited or commented.

**Read for this:** BRIDGE.md, RULES.md (with G1 to G19), FINAL.md, the copy guide (§2 the keeper's voice, §5 what every action carries, the forecast table), `architecture-deepening/design.md` (decisions 7, 9, 11, 12, the frontier of 5 Nov and "Built: slice 1"), `people-bulk-and-dashboard-depth/specs/operational-readiness/selection.md`, A-17 to A-20, `route-automation-settings-v5` (the `pending` and `bundle` frames, the "When given" column), `route-bundles-v2` (the publish compose step and its caption), `proposed/CancelQueuedStep.md`, `proposed/NextStep.md`, `proposed/ActionOutcome-seat.md`; and the code on `main` after PR 116: `RehearsalDialog.tsx`, `PlanReview.tsx`, `GrantDirectAccess.tsx`, `ManageBundles.tsx`, `Holding.tsx` with `request-decisions.ts`, `RemovalDialog.tsx`, `CancelQueuedStep.tsx`, `RuleConsent.tsx`, `lib/retry.ts`, `lib/outcome.ts`, `lib/wait-reason.ts`, `services/planner.go`, `services/bulk.go`, `services/cancels.go`, `handlers/bulk.go`, `handlers/plan_gate.go`, `handlers/cancel_gate.go`.

**In one line each:**
- **Q1.** A plan is three folds in a fixed order, **Straight away · Will wait for you to send · Left as they are**, one row per person inside them. The heading says *which*, the row says *what*. After Apply the same folds cross to the true tense: **Done · Waiting for you to send → · Zitadel didn't answer · Zitadel refused · Left as they were**. One pure function groups, one heading part draws it, every bulk op gets it by passing through `RehearsalDialog`.
- **Q2.** A single action learns from the planner the moment it has enough to plan, so the sheet says before the press that the act cancels a waiting change, the press pushes the existing CancelQueuedStep at once with no round trip, and nothing that cancels nothing gains a step. A stale plan is one Notice inside the sheet and a refreshed note; nothing is written.
- **Q3.** A rule-derived role stays a sentence inside the person's row ("Also gives Trained in Laser Lab, by the automatic rule from Member in Laser Lab."), its delivery counts like any other write when the person's fold is chosen, and the same sentence is what Give, Approve and Manage bundles say before the press. One source of words: the planner.

---

## 1 · Findings

**clash** means the code contradicts a settled rule; **gap** means no rule covers it; **drift** means the docs and the code disagree.

- **No plan is grouped, and no board draws one.** `PlanReview.tsx:94-104` is one scroll box, rows sorted by name, each with a Badge ("Will change", "No change", "Refused"). The only drawing of a grouped review is a caption on `route-bundles-v2` ("Straight away · Waits for you · Unchanged") and `BUILD-LIST.md`'s line for the publish review. **drift** with decision 11.
- **Delivery never reaches the screen.** `services.Write.Delivery` (`straight_away`, `waits_for_you`, `none`) lives inside `Change.Writes`, which is not serialised. `BulkOutcome` carries `effect`, `detail`, `consequence`, `wait_reason`, `retry`, `rule_waiting`; `fingerprint` and `cancel_ids` are `json:"-"`. The UI can learn that a role waits only by reading the sentence "Giving X waits for you to send." in `consequence`, and today it does not. The cancel note under the list is a regex over the words (`PlanReview.tsx:25-39`). **gap**
- **The plan's badge forecasts in the wrong tense for the one thing that matters.** `PLAN_EFFECT_LABEL.apply` is "Will change" for a write that goes straight away and for one that waits alike; "Will wait for you to send" exists for `queued` rows, and no preview ever produces one (`planner.go` emits `apply`, `no_change`, `blocked`). So the plan-row rule ("forecasts in the future tense, amber dot") is in the code and never on screen. **clash**
- **After Apply, the settle overwrites the person's words.** `settleBulkRow` (`handlers/bulk.go:204-220`) replaces `detail` with a generic "Recorded in Syndra. A role it gives always waits for you, so it is in Pending changes until you send it.", so the row that said "Joins Safety. Gains Member in Laser Lab." before Apply says something else after. The per-person reading is lost at the moment it is most wanted. **clash** (G12: a fact said once, in the same place)
- **Rules are counted twice.** A rule-derived role is a sentence in the row before Apply and a separate count after it (`rule_waiting` → "2 more changes from automatic rules wait in Pending changes for you to send."). Decision 7 puts the rule writes in the Change; once they are there, the second count is a second fact. **drift**
- **Single actions ask after the click.** Give, Remove, Manage bundles, Approve and Extend write first and learn from a 409 `WOULD_CANCEL_QUEUED` that the write would cancel something; `useCancelStep` then pushes the step (`CancelQueuedStep.tsx`). The reader presses, waits, and is asked. Decision 9 says the planner answers before the write. **drift**
- **The consent for a cancel is a header.** The go-ahead re-sends with `X-Syndra-Cancel-Ack`, which admits every cancellation at that moment, not the ones the reader saw. Decision 4 made the bulk plan admit only the ids it showed; the single actions still admit all. **gap**
- **The same role-giving sentence has three sources.** The planner (`planner.go:176-234`), `rule_follow.go:165-170` (the `/rule-preview` read behind `useRuleConsent`) and Manage bundles' `useInsightsWhatIf` ("and, through an automatic rule, {role}") each say what a rule adds, in two wordings. **clash** (one pattern per job)
- **"Withdraw" survives on two buttons.** "Withdraw the queued grant" (`RemovalDialog.tsx`, the direct case) and "Withdraw the queued change" (the bundle case). The vocabulary gives *withdraw* to a member taking back a request, and *grant* was retired for *give* (A-17 Q7). A-20 P5 already moved the service-account page off "Withdraw" onto the queue's own verb. **clash**
- **The stale banner is a Notice now** (`RehearsalDialog.tsx:723-745`, `Notice tone="warn" blocking`), as A-17 asked. Its sentences are right; it has no words for a single action, because a single action has no plan yet. **gap**
- **The bulk forecast reads the rows** (`bulkForecast`, `RehearsalDialog.tsx:151-167`): "Will wait for you to send" only if every acting row is `queued`, which no preview row is, so a bundle whose only role waits forecasts "Will be given". The same honesty gap A-20 P8 found on Approve. **clash**

---

## 2 · Q1: a plan grouped by how each change travels

### 2.1 The decision

The reader asks a plan two things: *after I press Apply, is anything still mine to do?* and *what happens to each person?* Today it answers the second only. The grouping answers the first, and it must not cost the second.

So the plan groups **people, not writes**, and it groups them by whether a step is left for them. A person whose roles all go straight away sits under **Straight away**. A person with any role that waits sits under **Will wait for you to send**, and their row names which roles wait and which go now. A person for whom nothing is written sits under **Left as they are**. One row per person, everywhere; nobody appears twice.

Why by person and not by write: a row split across two folds is two half-sentences, and the thing Syndra has promised is "person by person" (the preview lede). Why "any role waits" puts the person under the amber fold: the step left is a fact about the person's access ("He can't supervise yet"), and a person with a step left is a person with a step left.

### 2.2 The folds, before Apply

Fixed order. A fold with nobody in it is not drawn (as a batch on Pending changes). Each is a fold (FoldChevron, open by default), its heading on `--surface-0`, with the long-list rule (10 rows, then "Showing 10 of N · Show 25 more · Show all N · Back to 10").

| Order | Heading | Dot | Count | Faint line under the heading | Who sits here |
|---|---|---|---|---|---|
| 1 | **Straight away** | none | "7 people" | none | every write goes to Zitadel in the same request; also rows that only cancel a waiting change (§2.4) |
| 2 | **Will wait for you to send** | amber StatusDot, no link | "4 people" | "Because Trained in Laser Lab always waits for you." / "Because Trained in Laser Lab and Supervisor in Laser Lab always wait for you." | any write waits (`waits_for_you`), whether or not others go now |
| 3 | **Left as they are** | none | "1 person" | none | `no_change` and `blocked` rows |

- "Straight away" and "Left as they are" carry no dot: silence for normal (G17), and the v5 "When given" column already draws "Straight away" quiet and "Waits for you" marked. The amber dot is the only colour in the box, so the glance is the amber heading and its count.
- The heading is the plan-row forecast the rule asks for ("Will wait for you to send", amber dot, no link). Rows under it no longer carry the badge "Will change": the heading says which, the row says what.
- "Left as they are" replaces the v5 caption's "Unchanged". **Change to a settled word, and why:** the fold holds Refused rows too (departed account, retired bundle, Zitadel's list unreadable), and "Unchanged" would make a refusal read as a non-event. "Left as they are" is the sentence `planNote` already uses ("Syndra leaves those as they are."), now in the heading instead of the footer. Each row in it keeps its own word in the right seat: **No change** or **Refused**, Badge as today.
- The faint cause line under fold 2 is the Pending batch's own sentence ("They wait because Supervisor always waits for you"), shortened. It needs the set of waiting roles across the fold; §6 adds it to the row.

### 2.3 The row

One `PlanRow`, unchanged in shape: name (14.5/600, wraps, A-17), then the planner's words in `--muted` 13px, exactly as `planner.go` writes them, then the cancels line in `--faint`. No badge except in "Left as they are".

How the three kinds of person read:

- **All straight away** (under Straight away): "Joins Safety. Gains Member in Laser Lab and Trained in Laser Lab."
- **Some of each** (under Will wait for you to send): "Joins Safety. Gains Member in Laser Lab and Trained in Laser Lab. Giving Trained in Laser Lab waits for you to send." The reader learns Member goes now and Trained waits from the one row, and the heading already told them a step is left.
- **All waits** (under Will wait for you to send): "Joins Safety. Gains Trained in Laser Lab. Giving Trained in Laser Lab waits for you to send." The last sentence repeats the heading. It stays: a row is read alone on a phone with the heading scrolled past, in Change history, and in the result; one source of words beats a row that is incomplete out of context.
- **Nothing to send** (under Straight away): "Joins Safety. Already holds every role it gives, so nothing is sent to Zitadel."
- **No change** (under Left as they are): "Already in Safety." with the Badge **No change**.
- **Refused** (under Left as they are): "Syndra couldn't read Zitadel's list of roles to check the ones this gives still exist. Try again shortly." with the Badge **Refused**. The words carry their step, as the planner writes them.

### 2.4 Rows that only cancel a queued change

A remove whose give is still waiting in Pending changes writes nothing to Zitadel (`Delivery = none`): it cancels the give, and the person never had the role. It sits under **Straight away** (it is finished when you apply, with nothing left for you), and its words say that nothing is sent.

Planner words, a change to `planner.go` (today the row says "Loses Member in Laser Lab." for a role the person never had):

- Remove: "Leaves Safety. The give of Member in Laser Lab that was waiting is cancelled, so nothing is sent to Zitadel."
- Assign that only cancels a waiting removal: "Joins Safety. The removal of Member in Laser Lab that was waiting is cancelled, so nothing is sent to Zitadel. They keep it."

The cancels sentence that follows every row today ("Cancels a change waiting for you to send: the grant of …") is kept for rows that cancel *and* send; for a cancel-only row the one sentence above is the whole story (G12).

### 2.5 The consent

- **The before-line** (ModalFooter note) always has the same shape and says whether this finishes now or waits, by people. It replaces `planNote` ("No change for N · N refused …"), whose counts the headings now carry.
  - all straight away: "Every change goes to Zitadel when you apply."
  - all wait: "Every change waits for you to send. Nothing reaches Zitadel until you press Send on Pending changes."
  - some of each: "Apply sends 7 people's changes to Zitadel now. 4 people's changes wait for you to send."
  - with a cancel-only row: the sentence above, then "For 1, this only cancels a change that's waiting; nothing is sent." (plural: "For 2, this only cancels changes that are waiting; nothing is sent.")
  - nothing to apply: "Nothing here would change, so there is nothing to apply." (today's `applyBlocked` reason, unchanged)
  - Zitadel down: the `waits` before-line first, as today.
- **The cancels note** under the list stays as it is (`--warn-text`, `role="note"`): "Cancels 2 changes that are waiting for you to send: …. If anyone queues another first, this plan goes stale and nothing is cancelled unseen." It is the one place in the plan that is allowed to be warm in colour, because it is the one thing in the plan that undoes someone's work.
- **The Apply button** carries the number, and when the plan cancels, it carries that too, in the CancelQueuedStep's own shape ("Cancel all 3 and remove"):
  - "Apply to 12 people"
  - "Cancel 2 and apply to 12 people" / "Cancel 1 and apply to 12 people"
  - destructive ops keep `dangerConfirm`; the label is the same.
- **The forecast** while applying (ActionForecast, amber breathing dot) reads from the plan, not from a guess: "Will wait for you to send" if every person sits under fold 2; the op's own word ("Will be given", "Will be removed") otherwise. Fixes the last finding in §1.

### 2.6 After Apply

The same box, the same rows, re-grouped by what happened. Rows are keyed by person; the re-grouping is the re-sort motion (fade out, fade in, 120ms each way; rows never travel). The folds keep their order so nothing swaps places under the reader: what was "Straight away" splits into three outcomes in place.

| Order | Heading | Dot | Faint line under the heading | Row seat |
|---|---|---|---|---|
| 1 | **Done** | healthy StatusDot, blooms once when every written row reached | none | none: silence for normal |
| 2 | **Waiting for you to send →** | amber, violet link to Pending changes (`?role=` when one role waits, else unfiltered) | "Because Trained in Laser Lab always waits for you." | each row: amber dot + "Waiting for you to send →" with `?person=`, as NextStep draws it |
| 3 | **Zitadel didn't answer** | amber, still (no breathe: not a Notice) | "Syndra sends them again at 14:05 (try 2 of 6)." (`retryLine`, "them") | each row: its `RetryStatus` line |
| 4 | **Zitadel refused** | danger, still | none | each row: the refusal and its step, as `refusedDetail` writes them |
| 5 | **Left as they were** | none | none | Badge **No change** / **Refused**, unchanged from the plan |

Fold 1 reads "Done", not "Reached Zitadel", because it also holds the rows that had nothing to send (already held every role; only cancelled a waiting change). "Done" answers the fold's question, nothing of theirs is still yours to do, and the row says whether anything was sent. The copy guide's own word for a confirmed send is Done.

A person with some of each after Apply (Member reached, Trained waiting) sits under fold 2, and their row keeps its plan words, so "Gains Member in Laser Lab and Trained in Laser Lab. Giving Trained in Laser Lab waits for you to send." is still true and still complete. **Change to the settle:** `settleBulkRow` stops replacing `detail`; it sets `effect`, `wait_reason` and `retry` only. The generic `queuedDetail` sentences retire; the fold heading and the row's seat say the state.

**The result block** (ActionOutcome, block form, above the box; the dialog's lede reads "Finished. Here is what happened." as today):

| Case | Word | Sentence | Next step |
|---|---|---|---|
| all reached | "Given to 11 people" / "Revoked from 11 people" | "All 11 are in Zitadel now." | "Nothing for you to do." |
| some wait | "Given to 11 people" | "7 are in Zitadel now. 4 wait for you to send." | "Waiting for you to send →" (NextStep, the fold's link) |
| all wait | "Given to 11 people" | "All 11 wait for you to send. Nothing has reached Zitadel yet." | "Waiting for you to send →" |
| some not answered | "Given to 11 people" | "9 are in Zitadel now. 2 aren't yet; Syndra sends them again on its own." | "Nothing for you to do. Syndra sends them again at 14:05 and says on Home if it stops sending them." |
| some refused | "Given to 9 of 11 people" | "9 are in Zitadel now. Zitadel refused 2; each row says why." | "Follow the step on each row Zitadel refused, below." (today's `REFUSED_ROWS_NEXT`) |
| nobody held it | "Done" | "Nobody's access changed, because nobody holds it yet." | "Nothing for you to do." |

The sentences compose left to right in the fold order, one clause per fold that has people in it, so a plan with all five gives: "7 are in Zitadel now. 2 wait for you to send. 1 isn't yet; Syndra sends it again on its own. Zitadel refused 1; the row says why." The word counts people written (`succeeded + queued`), "of N" only when something was refused or failed. The healthy dot blooms only when every written row reached Zitadel.

`ruleWaitingSentence` ("N more changes from automatic rules wait …") retires: with decision 7, a rule's writes are writes, counted in the person's fold (§4).

### 2.7 States

| State | What shows |
|---|---|
| Opening (no compose) | The lede, an empty box with the skeleton after 200ms. No headings, no counts: nothing has been read. The footer note reads "Waiting for the preview." and the button is disabled with that reason, as today. |
| Scope (over the limit) | Unchanged: the count sentence, the tick, "Preview the change for N people". |
| Plan | §2.2 to §2.5. |
| Plan that cancels | The same, with the cancels note and the "Cancel N and apply …" button. |
| Nothing to apply | Only "Left as they are" is drawn; the footer says "Nothing here would change, so there is nothing to apply."; the button is disabled with the same reason, or `definitionLabel` where a definition still applies. |
| Applying | The button busy; the forecast under the box; the folds still. |
| Stale (any `PLAN_*` code) | The Notice as built (title "Nothing was applied. This is a fresh preview.", the code sentence, "Changed since you last looked: {names}.", next "Read the preview below, then apply."), then the fresh plan in its folds. Unchanged. |
| Result | §2.6, Close only. |
| Failed to preview or apply | ActionOutcome failed, as today. |

### 2.8 Phone (390) and both themes

- The dialog is the bottom sheet (S3). The box keeps its 46vh; fold headings are `position: sticky` at the box's top on `--surface-0`, so the glance survives scrolling inside the box. Folding a long group is how you reach the next; the 10-row cap keeps each short.
- Row: name on the first line, words wrapping beneath; the Badge (Left as they are only) wraps under the name and stays right-aligned (`margin-left:auto`), never under the words.
- Footer: the before-line wraps to two lines at most; the button is full width, its label unchanged ("Cancel 2 and apply to 12 people" fits at 390 in one line at 14px; UNVERIFIED at the widest op name).
- Themes: nothing is new. Tokens only: `warn`, `healthy`, `danger` dots; `--surface-0` heading ground; `--line-strong` box; `--muted` words; `--faint` counts and cause lines; `--warn-text` for the cancels note. Dark has no card shadow. The amber dot on "Will wait for you to send" is the same dot a person's row uses, so a reader who knows one knows the other.

### 2.9 Anatomy and what holds it by construction

- **`groupPlan(plan)`** in `lib/plan-groups.ts`, pure: `(BulkPlan) → PlanGroup[]`, each `{ key, heading, tone, dot, note?, href?, rows }`. One table of headings, `GROUPS`, with a `before` and an `after` word per key (the tense crossing). Every `(effect, delivery)` pair maps to exactly one group; a test holds the table and the map, and fails on any pair with no group.
- **`beforeLine(plan)`**, **`applyLabel(plan)`**, **`resultOutcome(plan)`** in the same file, each a pure function of the same rows. `RehearsalDialog` calls these three and draws nothing of its own.
- **`PlanReview`** keeps its name and its box; inside, one `FoldHeading` per group and `PlanRow` per person, `PaginationRow` after ten. The regex over the cancels words goes: the row carries `cancels` (§6).
- **`FoldHeading`** (`proposed/FoldHeading.md`, new): FoldChevron (28px round) · optional StatusDot or NextStep · the word (14.5/600) · the count in `--faint` · an optional faint line beneath. `role="group"`, labelled "Will wait for you to send, 4 people". Pending changes' hand-made fold rows (inline SVG, `pending/page.tsx`) move onto it, so a batch on Pending and a fold in a plan are one part. UNVERIFIED whether Drift's two directions and Expiring's sections already share a heading part; if they do, that part is `FoldHeading` and gains the dot and note.
- **Every bulk op** reaches the screen through `RehearsalDialog` → `PlanReview`, so an op is grouped the day its rows carry `delivery`. Until then its plan is flat, as today: `groupPlan` draws the folds only when every row has a delivery, and a test fails on a plan where some rows have it and some do not, so no op can half-claim "Straight away".

---

## 3 · Q2: a single action that would cancel a queued change asks first

### 3.1 The decision

The sheet plans as soon as it can (a role is chosen, a request is on screen, a bundle is ticked), with one read: `usePlanConsent(action)` returns the planner's Change for that one person: the row words, what it cancels, its delivery, a refusal if any, and the fingerprint. That one hook replaces `useRuleConsent` (`/rule-preview`), `previewRuleGifts` and Manage bundles' `useInsightsWhatIf` preview, so the sentence "Also gives X, by the automatic rule from Y." has one author.

Then:

1. **Nothing cancelled:** one step, as today. The before-line is the planner's delivery ("{Role} always waits for you, so Liv gets it once you send it." / "Liv can use it as soon as you give it."), and the rule lines follow it.
2. **Something cancelled:** the before-line says so before the press, in one sentence. The press pushes the existing **CancelQueuedStep** at once, already filled from the plan, no round trip. Keep it first, the confirm names what goes ahead ("Cancel it and give Member"). The write carries the cancel ids the step showed.
3. **The 409 stays as the second safety.** If someone queues another change between the plan and the press, the write is refused as stale, and the step shows it with its existing "One more since you looked" group at the top.

**Change to a standing rule, and why.** RULES says "Undoing a change that's still waiting is a pushed step inside the sheet that asked". That stands; what changes is *when*: the step is pushed by the press, not by a refusal, and the sheet foretells it. The alternative, folding the ledger and the renamed confirm into the form so a cancel is one step, was weighed and not taken: a cancel undoes a named colleague's queued work with a reason attached, Keep it must be the first focus (A-17 Q1), Approve has no form to fold it into, and one pattern for four surfaces beats one fewer click on two of them.

### 3.2 The surfaces

**Give a role** (`GrantDirectAccess.tsx`)
- Before-line, in this order: the date line as today; then the delivery line; then, when the plan cancels: "Giving it cancels the removal Randy Orton queued 2 hours ago. Nothing is cancelled until you choose on the next step." (several: "Giving it cancels 2 changes that are waiting. Nothing is cancelled until you choose on the next step."); then the rule lines.
- Button stays "Give role". The press pushes the step. Its confirm: "Cancel it and give Member" (accent). Keep it returns to the form with the kept note as today ("Nothing changed. Randy Orton's removal is still waiting." + Waiting for you to send →).
- Forecast from the plan: "Will wait for you to send" when the role waits, else "Will be given".
- Result: as today, plus the after line from the step: "The removal Randy Orton queued 2 hours ago is cancelled, so nothing goes to Zitadel. Change history records both."

**Revoke direct access** (`RemovalDialog.tsx`, `DirectDialog`)
- When the plan's delivery is `none` (the give never reached Zitadel), the dialog is the cancel, said plainly:
  - Lede: "Given to Liv on 3 Nov by Randy Orton, and still waiting for you to send."
  - Body, neutral dot: "Nothing to take back. It hasn't reached Zitadel, so Liv never had it. This cancels that give and sends nothing."
  - Button: **"Cancel the waiting give"** (accent). **Replaces "Withdraw the queued grant"**: *withdraw* belongs to a member and *grant* was retired for *give* (A-17 Q7, A-20 P5).
  - Result: word **Cancelled** (the forecast table already has "Will be cancelled" → "Cancelled"), sentence "The give of Member in Laser Lab that was waiting is cancelled. Liv never had it, so nothing goes to Zitadel.", next "Nothing for you to do."
- When the plan cancels something else as well as revoking (rare: a re-give of the same role waiting from someone else), the danger note gains the sentence "Revoking it cancels the give Randy Orton queued 2 hours ago." and the press pushes the step; confirm "Cancel it and revoke" (dangerConfirm).
- The bundle case (`BundleDialog`) is the same: **"Cancel the waiting change"** replaces "Withdraw the queued change"; the result word is Cancelled; the sentence "The {bundle} bundle that was waiting is cancelled. Nothing it carries had reached Liv, so nothing goes to Zitadel."

**Approve a request** (`Holding.tsx`, `request-decisions.ts`)
- The row's "If you approve:" line is the plan's words: "If you approve: Also gives Trained in Laser Lab, by the automatic rule from Member in Laser Lab. Approving cancels the removal Randy Orton queued 2 hours ago." (the cancel sentence last, as the planner orders it).
- The hold states stay: "Checking what automatic rules also give." → "Checking what approving does." (the read now covers more than rules), "Held until Syndra can read the automatic rules." → "Held until Syndra can read what approving does.", and the failed line "Syndra couldn't read what approving does, so it can't show what else this would give or cancel. Try again to read it." with Try again.
- Press Approve: the standalone step (Modal sm) opens at once, filled from the plan; confirm "Cancel it and approve". No change to the step.
- Forecast from the plan: "Will wait for you to send" for a role that waits (closes A-20 P8's honesty gap for Approve), else "Will be approved".

**Manage bundles** (`ManageBundles.tsx`, a person's own assign and remove)
- Under each ticked bundle, the planner's row words replace the hand-built "Adding X would give …" list: "Joins Safety. Gains Member in Laser Lab and Trained in Laser Lab. Also gives Supervisor in Laser Lab, by the automatic rule from Trained in Laser Lab. Giving Trained in Laser Lab waits for you to send." A remove: "Leaves Safety. Loses Member in Laser Lab. Keeps Trained in Laser Lab another way." The empty-bundle and unpublished-edits lines stay as they are.
- The before-line is `beforeLine` over the staged changes, the same function as the bulk plan: "Apply sends 2 changes to Zitadel now. 1 waits for you to send." With one change: "This goes to Zitadel when you apply." / "This waits for you to send."
- Cancels are gathered across every staged change into **one step** before the first write: "This cancels 3 changes that are waiting", confirm "Cancel all 3 and apply 2 changes". Today the loop asks once per bundle and Keep stops the rest; one ask is fewer steps and one decision.
- Result as today; the after line lists the cancels as the step writes it.

### 3.3 When the plan went stale

The write carries the fingerprint the sheet planned against; apply re-plans under the lock and refuses on any difference (decision 3), answered as 409 `PLAN_STALE` with the fresh row. Nothing is written. Each surface says it where the act was taken, then shows the fresh plan in its own ordinary seat, so the reader reads the new state in the same words as the old one:

- **In a sheet** (Give, Revoke, Manage bundles): a Notice (warn, blocking) between the header and the form: title **"Nothing was changed. Liv's access moved while this was open."**, next **"Read the note under the form, then press Give role again."** (the button's own label). The form stays as it was; the before-line and the rule lines refresh from the fresh plan; if the fresh plan cancels, the next press pushes the step; if it cancels nothing any more, no step. Particular cases say themselves through the ordinary seats:
  - now already held: the picker row is ticked grey with "Already has it" (the picker's held row, unchanged), the button is disabled with reason "Liv already has Member in Laser Lab.", the before-line is "Nothing to give."
  - the change it would have cancelled was sent meanwhile: the cancel sentence simply leaves the before-line.
  - a new queued change appeared: the before-line names it; the step on press shows it under "New since you looked".
- **In a row** (Approve): the buttons return beside one line, `role="alert"`, danger dot: **"Nothing was changed. Liv's access moved while you looked. Read the line above, then approve again."** The "If you approve:" line refreshes.

No "stale" and no "plan" on screen: the reader never had a plan, they had a sheet, and what moved was a person's access.

### 3.4 Outcome words after

Unchanged where they exist (Given, Removed, Approved, Recorded, Refused by Zitadel, the retry lines). New or changed:

| Act | Word | Sentence | Faint line / next |
|---|---|---|---|
| Give that cancelled a removal | Given | "Liv can use Member in Laser Lab on their own now" (or the waits form) | "The removal Randy Orton queued 2 hours ago is cancelled, so nothing goes to Zitadel. Change history records both." |
| Revoke that was only a cancel | Cancelled | "The give of Member in Laser Lab that was waiting is cancelled. Liv never had it, so nothing goes to Zitadel." | "Nothing for you to do." |
| Remove a bundle that was only a cancel | Cancelled | "The Safety bundle that was waiting is cancelled. Nothing it carried had reached Liv, so nothing goes to Zitadel." | "Nothing for you to do." |
| Approve that cancelled | Approved | as today | the same after line, in the seat's second line |
| Manage bundles with cancels | Recorded | as today | "The 3 changes that were waiting for you to send are cancelled, so nothing goes to Zitadel for them. Change history records them all." |
| Kept (any) | none: back to the form | "Nothing changed. Randy Orton's removal is still waiting." | Waiting for you to send → |

---

## 4 · Q3: rule-derived changes in a plan

### 4.1 Where a rule's role lives

A rule-derived role is a **sentence inside the person's row**, never a row of its own: the plan is person by person, and a rule is a reason a person gains something, like a bundle is. The words stand as the planner writes them:

- give: "Also gives Trained in Laser Lab, by the automatic rule from Member in Laser Lab."
- remove: "Also loses Trained in Laser Lab, which the automatic rule from Member in Laser Lab gave."
- a rule two hops down: "Also gives Supervisor in Laser Lab, by the automatic rule from Trained in Laser Lab." (each hop names the role that set it off, which is the hop before it; "hop" never appears on screen)

A rule has no name and no page of its own; the role that set it off is its name, and it is plain prose in the row (a cause that isn't a thing stays plain; RULES on links).

### 4.2 How it is grouped under Q1

The rule's writes are writes (decision 7). A person's fold is chosen over every write they have, rule-derived or not, so a person whose own roles go straight away but whose rule-given role waits sits under **Will wait for you to send**, and their row reads: "Joins Safety. Gains Member in Laser Lab. Also gives Trained in Laser Lab, by the automatic rule from Member in Laser Lab. Giving Trained in Laser Lab waits for you to send." The last sentence lists the rule-given role like any other; the sentence before it has already said where it came from. The fold's faint cause line names it too ("Because Trained in Laser Lab always waits for you.").

Nothing in the heading counts rules apart (G17). The separate count after Apply (`rule_waiting`, "N more changes from automatic rules …") retires with it.

### 4.3 After Apply, and on Pending changes

With `source='rule'` and the triggering role recorded (task 1.6), the rule's queued rows on Pending changes sit in the rule's own batch, as the v5 `pending` frame already draws: "These 2 changes come from one rule: anyone with Member in Laser Lab also gets Trained in Laser Lab. They wait because Trained in Laser Lab always waits for you." The "Caused by" seat reads "Rule from Member in Laser Lab" instead of "The new rule". A row's "Waiting for you to send →" from the plan's result lands there with `?person=`, and the person's two rows (the bundle's and the rule's) are both in view. Change history records the rule's row with the role that set it off.

### 4.4 Before the press

The same sentence is what Give and Approve show before the press (§3.2), from the same planner, so a reader who approves and then sees the plan for a bulk assign reads one voice. `rule_follow.go`'s copy of the sentence and Manage bundles' "and, through an automatic rule, …" retire.

---

## 5 · Exact words, in one place

New or changed strings. Everything not listed stands as built.

**Fold headings, before Apply:** "Straight away" · "Will wait for you to send" · "Left as they are".
**Fold headings, after Apply:** "Done" · "Waiting for you to send →" · "Zitadel didn't answer" · "Zitadel refused" · "Left as they were".
**Counts:** "1 person" / "N people".
**Cause line:** "Because {role} always waits for you." / "Because {role} and {role} always wait for you." (alphabetical).
**Retry line under a fold:** `retryLine(retry, "Zitadel", "them")`.
**Before-line (bulk and Manage bundles):** "Every change goes to Zitadel when you apply." · "Every change waits for you to send. Nothing reaches Zitadel until you press Send on Pending changes." · "Apply sends {n} people's changes to Zitadel now. {m} people's changes wait for you to send." · "For {k}, this only cancels a change that's waiting; nothing is sent." (plural: "changes that are waiting") · "This goes to Zitadel when you apply." · "This waits for you to send." · "Nothing to give."
**Apply button:** "Apply to {n} people" · "Cancel {k} and apply to {n} people" · "Cancel all {k} and apply {n} changes" (Manage bundles).
**Planner rows, cancel-only:** "Leaves {bundle}. The give of {role} that was waiting is cancelled, so nothing is sent to Zitadel." · "Joins {bundle}. The removal of {role} that was waiting is cancelled, so nothing is sent to Zitadel. They keep it."
**Result sentences:** "All {n} are in Zitadel now." · "{n} are in Zitadel now." · "{m} wait for you to send." · "All {n} wait for you to send. Nothing has reached Zitadel yet." · "{k} isn't in Zitadel yet; Syndra sends it again on its own." / "{k} aren't yet; Syndra sends them again on its own." · "Zitadel refused {r}; each row says why." / "Zitadel refused 1; the row says why." · "Nobody's access changed, because nobody holds it yet."
**Result words:** "Given to {n} people" · "Given to {n} of {t} people" · "Revoked from {n} people" · "Revoked from {n} of {t} people" · "Extended for {n} people" · "Done" · "Cancelled".
**Before the press, a cancel:** "Giving it cancels the removal {name} queued {ago}. Nothing is cancelled until you choose on the next step." · "Giving it cancels {k} changes that are waiting. Nothing is cancelled until you choose on the next step." · "Revoking it cancels the give {name} queued {ago}." · "Approving cancels the removal {name} queued {ago}."
**Approve's hold and failed lines:** "Checking what approving does." · "Held until Syndra can read what approving does." · "Syndra couldn't read what approving does, so it can't show what else this would give or cancel. Try again to read it."
**Cancel-only dialogs:** lede "Given to {first} on {day} by {name}, and still waiting for you to send." · body "Nothing to take back. It hasn't reached Zitadel, so {first} never had it. This cancels that give and sends nothing." · button "Cancel the waiting give" / "Cancel the waiting change" · result "The give of {role} that was waiting is cancelled. {first} never had it, so nothing goes to Zitadel." / "The {bundle} bundle that was waiting is cancelled. Nothing it carried had reached {first}, so nothing goes to Zitadel."
**Stale, in a sheet:** title "Nothing was changed. {First}'s access moved while this was open." · next "Read the note under the form, then press {button} again." · disabled reason "{First} already has {role}."
**Stale, in a row:** "Nothing was changed. {First}'s access moved while you looked. Read the line above, then approve again."
**Retired:** "Will change" (badge) · "No change for N · N refused. Syndra leaves those as they are." · "Every selected person will change." · "N more changes from automatic rules wait in Pending changes for you to send." · "Withdraw the queued grant" · "Withdraw the queued change" · "Adding {bundle} would give" / "Removing {bundle} would revoke" · "and, through an automatic rule, {role}" · the `queuedDetail` sentences that overwrite a row after Apply.

No em dash in any of them; every outcome ends on a step or "Nothing for you to do."; Zitadel is named; "people" counts.

---

## 6 · For code

**Backend**
1. `services/bulk.go` `BulkOutcome`: add `delivery` (`straight_away` | `waits_for_you` | `none`, absent on `no_change` and `blocked`), `waits` (the role words that wait, alphabetical) and `cancels` (the queued changes it cancels, the shape `CancelQueuedStep` reads: kind, place, role, by, queued at, reason). `fingerprint` and `cancel_ids` stay unserialised. Test: every `apply` row has a delivery; `waits` is non-empty iff delivery is `waits_for_you`.
2. `services/planner.go`: the cancel-only words (§2.4). The fingerprint is unchanged (words are not in it).
3. `handlers/bulk.go` `settleBulkRow`: keep `detail`; set `effect`, `wait_reason`, `retry` only. Retire `queuedDetail`. Keep `refusedDetail` as the row's seat line.
4. Single actions through the planner (task 1.2): one preview read per person and action (`GET /users/{id}/plan?op=give&project=&role=`, and the same for remove, approve, assign_bundle, remove_bundle) returning the row plus `fingerprint`; the write carries `fingerprint` and `cancel_ids`; apply re-plans under the lock and answers 409 `PLAN_STALE` with the fresh row on any difference; `cancel_gate.go`'s `WOULD_CANCEL_QUEUED` stays as the second safety; `X-Syndra-Cancel-Ack` retires. `rule_follow.go`'s sentence and the `/rule-preview` read fold into the planner's words.
5. Task 1.6: rule writes with `source='rule'` and the triggering role; `db.Answers` reviewed for what a removal cancels (the task's own note). Pending changes' "Caused by" reads the role.

**UI**
6. `lib/plan-groups.ts`: `GROUPS`, `groupPlan`, `beforeLine`, `applyLabel`, `resultOutcome`; tests: every `(effect, delivery)` lands in one group; headings cross tense on `applied`; a mixed plan (some rows without delivery) is drawn flat; the before-line's four shapes; the result's clauses in fold order; the bloom only when every written row reached.
7. `components/ui/FoldHeading.tsx` per `proposed/FoldHeading.md` (to write): FoldChevron · StatusDot or NextStep · word · count · note; `role="group"` with the label. Pending changes' fold rows move onto it (`pending/page.tsx`).
8. `components/ui/PlanReview.tsx`: folds via `FoldHeading` + `PaginationRow`; sticky headings in the box; names wrap; Badge only in "Left as they are"; the cancels regex goes (reads `cancels`); the cancels note stays.
9. `components/ui/RehearsalDialog.tsx`: footer note from `beforeLine`; label from `applyLabel`; result from `resultOutcome`; `bulkForecast` reads delivery. Stale Notice unchanged.
10. `lib/usePlanConsent.ts`: replaces `RuleConsent.tsx`'s hook, `previewRuleGifts` and Manage bundles' `useInsightsWhatIf` preview; returns `{ row, cancels, delivery, waits, refusal, fingerprint, hold, failed, retry }`.
11. `CancelQueuedStep.tsx` `useCancelStep`: `step.ask(cancels)` pushes from the plan at press; `step.run` keeps the 409 path for the in-between case and the "New since you looked" group; Manage bundles gathers across staged changes into one ask.
12. `GrantDirectAccess.tsx`, `RemovalDialog.tsx`, `ManageBundles.tsx`, `Holding.tsx` + `request-decisions.ts`: the before-lines, the plan-driven step, fingerprint and cancel ids on the write, the stale Notice or line, the Cancelled word and the renamed buttons (§3). Forecasts from the plan's delivery.
13. Copy guide §5: the fold headings table (before and after), the before-line shapes, the stale words; §6: "Cancel the waiting give" beside *withdraw*. RULES: the pushed-step bullet gains "pushed by the press, foretold by the before-line"; the plan-row bullet gains "and the plan's folds are its headings". `plain-language.test.ts` already guards the dash and the exclamation mark; add "Will change" and "waiting to be sent" to its sweep if not already there (the second is).
14. A-17's open items on the same files land with this: `PlanReview` names wrap and sort (already sorted); hand-made pills become Badge; `CancelQueuedStep`'s "Show all N" becomes `PaginationRow`.

Order: 1, 2, 3, 6, 7, 8, 9 (Q1 is complete and visible); then 4, 10, 11, 12 (Q2); 5 (Q3's data; its words already hold).

---

## 7 · Questions for the owner

Each "yes" accepts the recommendation.

1. **Group people, not writes, and put a person with any role that waits under "Will wait for you to send"?** *Recommended: yes. One row per person; the step left is a fact about the person.*
2. **"Left as they are" for the third fold, holding No change and Refused rows, instead of the v5 caption's "Unchanged"?** *Recommended: yes.*
3. **The Apply button names the cancels ("Cancel 2 and apply to 12 people"), as the cancel step's confirm does?** *Recommended: yes.*
4. **Keep the row's plan words after Apply and let the fold and the seat say the state (the settle stops rewriting `detail`)?** *Recommended: yes.*
5. **Single actions: the step is pushed by the press, filled from the plan, and the sheet says before the press that it will cancel; no inline fold of the ledger into the form?** *Recommended: yes. One pattern for all four surfaces.*
6. **One ask for Manage bundles, gathering every staged change's cancels, instead of one per bundle?** *Recommended: yes.*
7. **"Cancel the waiting give" and "Cancel the waiting change" replace the two "Withdraw …" buttons, with the result word Cancelled?** *Recommended: yes.*
8. **A rule-derived role stays a sentence in the person's row and counts in their fold; the separate "N more changes from automatic rules" count retires?** *Recommended: yes.*
9. **The first fold after Apply reads "Done", holding rows that reached Zitadel and rows that had nothing to send?** *Recommended: yes. The row says whether anything was sent.*

---

## 8 · How the boards read

All four sit on the canvas's "App screens" page, right of A-20, 1600 wide. Each mounts the real parts from `window.Syndra` (Modal, ModalHeader, ModalFooter, Button, Badge, StatusDot, Notice, ActionOutcome, Input, Segmented, PageHeader, AppShell); the fold heading, the pushed step's list, the role picker's chosen row and the row's NextStep link are drawn inline and say so (BRIDGE: a part the system lacks is drawn inline until built). Light frames are named `…-light`.

**`a21-plan-grouped`** (1600 × 2900)
1. The plan: Give the Safety bundle to 13 people, dark and light at 1440 (frames `plan`, `plan-light`), with a person with some of each (Jade), a cancel-only row (Alexa), a row that cancels and sends (Tiffany), a row that sends nothing (Zoey), No change (Dev) and Refused (Randy); the cancels note; the before-line and "Cancel 2 and apply to 11 people".
2. The phone sheet at 390 (`plan-phone`).
3. The consent, line by line: a table of the before-line's shapes and the button's labels.

**`a21-plan-result`** (1600 × 3000)
1. The result with every fold (`result`, `result-light`): Done 5, Waiting for you to send 4, Zitadel didn't answer 1, Zitadel refused 1, Left as they were 2; the word "Given to 10 of 11 people"; each waiting row's link.
2. The common result (`result-all`): everything reached or waits, one next step.
3. The phone (`result-phone`).
4. The outcome block, case by case.

**`a21-ask-before-cancel`** (1600 × 3400)
1. Give a role: nothing cancelled (`give-plain`, one step as built); foretold (`give`, `give-light`), with the chosen row's note and the footer's cancel sentence; asked on the press (`give-step`), the pushed step with Keep it first.
2. Stale (`give-stale`, `give-stale-light`): the Notice and the refreshed note.
3. Revoke with delivery none (`remove`, `remove-light`): "Cancel the waiting give"; after (`remove-done`): Cancelled. The step on a phone (`give-step-phone`).

**`a21-rule-rows`** (1600 × 2300)
1. The plan with two rule hops in one row (`rule`, `rule-light`): Roman and Finn stop at the first rule; Alexa's row walks both and ends on the waits sentence.
2. Give a role with the same words in its footer (`give`, `give-light`).
