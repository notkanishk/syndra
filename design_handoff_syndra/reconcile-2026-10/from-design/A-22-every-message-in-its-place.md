# A-22 · Every message in its place (draft, 8 Nov 2026)

Answer to A-22 (owner, 8 Nov 2026: "warnings and messages and button text … logically placed and clear and not confusing. Elegant and simple."). Boards: `a22-dialog-lifecycle.dc.html` and `a22-pending-one-outcome.dc.html` (Proposed, 8 Nov 2026), in `design/`; §7 says how they read. Read-only; nothing was published, edited in `ui/src`, or commented.

**Read for this:** BRIDGE.md, RULES.md (with G1 to G19), FINAL.md, the copy guide (§3 shape of a sentence, §5 what every action carries, §8 errors), A-04 §11 and §14, A-17 G4, A-19 P2, A-21 §2.6 and §3; the parts as built on `main` after PR 126: `Modal.tsx` (ModalHeader, ModalFooter, CANCEL_FIRST), `ActionOutcome.tsx` (block, inline, seat, ActionForecast), `Notice.tsx`, `NextStep.tsx`, `RetryStatus.tsx`, `CancelQueuedStep.tsx`, `RehearsalDialog.tsx`, `PlanReview.tsx`, `Acknowledge.tsx`, `lib/outcome.ts`, `lib/retry.ts`, `lib/plan-groups.ts`, `lib/drain-outcome.ts`; and every dialog, sheet and row seat under `ui/src/components/**` and `ui/src/app/**` named in §3. Four scouts read the surfaces file by file and quoted them with lines; I read the shared parts and the rules myself. The correctness build already under way fixes the four staging findings; this answer gives the rules they fall under and the sweep of everything else.

**In one line each:**
- **The rules.** Twelve, G20 to G31 (§2): a dialog has four places for words and three moments; one message per moment; the title is the act in the button's words; the dismiss says what it keeps; red, amber and quiet follow what happens to the person; a result is ActionOutcome, a dysfunction is Notice, a step left is NextStep, a retry is RetryStatus, and nothing else carries a message.
- **The sweep.** 48 surfaces (§3). Misleading: 11. Confusing: 9. Cluttered: 7. Could be more elegant: 10. Nothing wrong: 11. The misleading ones share four causes: a pre-act body left standing above its result, a fixed success word on a failure, two results about one press, and a title that names an act the sheet is not doing.
- **By construction.** Three changes to shared parts carry most of it (§5): ModalFooter's `cancel` learns `keeps`, ActionOutcome's block takes the sheet's body with it, and one result slot per surface is a type, not a habit.

---

## 1 · Findings

**clash** means the code contradicts a settled rule; **gap** means no rule covers it; **drift** means the docs and the code disagree.

- **A pre-act warning stays mounted above the result.** `RemovalDialog.tsx:350-402` (the danger-note is not guarded on `outcome`), `BundleDialog` `:566-593`, `BundlePage.tsx` DeleteBundleDialog `:782`, `policies/page.tsx` DeleteRuleConfirm and the rule editor, `UnexplainedAccess.tsx:1221-1244`, `HoldDialog.tsx`, `expiring-access/page.tsx:733-742` (Let it lapse's lede and field), `CreateRoleDialog.tsx`, `bundles/page.tsx:248`, `RenameBundleDialog`. A sentence in the future tense ("will lose this role") over a sentence in the past ("Cancelled") is two moments on one screen. **clash** with A-04 §11 ("every dialog ends on its own result") and G12.
- **A fixed success word is shown on a failure.** `CreateRoleDialog.tsx:168` ("Created"), `ProjectClaimDefault.tsx:279` ("Saved"), `BundlePage.tsx:609` ("Default set"), `ManageBundles.tsx:305` ("Recorded"), `zitadel/users/page.tsx` thrown errors ("Failed"/"Refused", the generic labels). **clash** with G4 (the result is the act's own word).
- **Two outcomes speak about one press.** `governance/pending/page.tsx:161-168` (the card) and `:253/518-533` (EverythingSent) are independent; `AddRolesToBundle.tsx:182/191` (a danger-note failure panel beside an "Added" block); `changes/roles-that-wait/page.tsx:272-290` ("Stopped" never cleared under a later danger Notice); `RehearsalDialog.tsx:697/739` (stale Notice left standing under a later failure); `zitadel/users/page.tsx:470/504` (stale Notice above an earlier failed result); `MappingManagement.tsx:598` (a rollback reports in the row and in the dialog). **gap**: no rule said one result per surface.
- **The title names an act the sheet is not doing.** `RemovalDialog.tsx:325` "Revoke direct access to {role}?" over a cancel of an unsent give; `UnexplainedAccess.tsx:1160-1186` "Is this owned by another system?" for Mark as owned elsewhere. **clash** with "the verb is the label, the title and the confirm".
- **Two Cancels.** `RemovalDialog.tsx:447-489`: the dismiss "Cancel" beside the act "Cancel the waiting give"; the same for "Cancel the waiting change" (`:633-708`). **gap**.
- **One failure, four words.** "Not extended" (`PersonAccess.tsx:838`, Home), "Didn't go through" (`expiring-access/page.tsx:701-703`, `UnexplainedAccess.tsx`, Let it lapse), "Couldn't put it on hold" (`HoldDialog.tsx`), "Couldn't lift it" (`HoldsDueForReview.tsx`), "That didn't save." (`ServiceAccountDialogs.tsx`, as a Notice). **clash** with one pattern per job.
- **A result drawn as a Notice; a warning drawn as a Notice.** Results: `ServiceAccountDialogs.tsx` ("That didn't save."), `roles-that-wait/page.tsx:284-290` ("That role still waits."), `SignInLookup.tsx:114` ("That lookup didn't run."), `TargetOverview.tsx` LifecycleControl ("That did not go through."). Pre-act warnings: `targets/RevokeDialog.tsx:118` ("Anyone connected stays connected for now."), the rule editor's "Checked: N things to know". A Notice is for something wrong with what you are looking at (RULES: "every dysfunction is one Notice"); a result and a consequence are not dysfunctions. **clash**.
- **The rule editor is md with two 46vh lists.** `policies/page.tsx:360`, `RolePickerOne.tsx:147`. The second list ("then also give them") begins past 600px on an 800px screen whenever the first list is longer than three rows. Four buttons in its footer (Cancel, Check rule, Save rule, Retire rule). **clash** with G16 and the phone rule (Modal lg for a list that is the subject).
- **Mapping dialogs say Zitadel about TrueNAS.** `AddMappingDialog.tsx`, `MappingManagement.tsx` pass no `system`, so `RehearsalDialog`'s result and `beforeLine`/`resultOutcome` (`plan-groups.ts:216-315`, "Zitadel" hard-coded) report a TrueNAS send as Zitadel. **clash** with copy guide §3.10.
- **Pending changes' word table is wrong at both ends.** `pending/page.tsx:166`: `no_change` wears "Sent" ("Nothing was waiting." under Sent; "Another send is already running." under Sent); a halted pass with "3 sent" is `failed`, so it wears "Not sent" and ActionOutcome appends "Nothing was changed."; `zitadel_offline` reads "Nothing was sent. Nothing was changed." **clash** with G4 and §3.6.
- **"Every change waits" counts people where it means changes** (`plan-groups.ts:216-232`, the owner's finding 3). **clash** with §3.8; the correctness build has it.
- **The sheet-becomes-its-result rule is unevenly built.** It holds in GrantDirectAccess, AskForAccess, the Zitadel › Users dialogs, ServiceAccountDialogs, SayWhatHappened, HomeLimit and HomeLocation (`{done ? <ActionOutcome/> : <form/>}`); it does not in the eleven surfaces of the first finding. **drift** between A-04 §11 and half the code.
- **ResolveFindingDialog and ResolveConflictDialog have no result** (`TargetOverview.tsx:1101-1314`, `onSuccess: onClose`). **clash** with 96b.
- **A seat with no part.** `MyStorage.tsx` PasswordForm draws its own `Result` rows; `ShapeSheets.tsx:233-250` draws its own success block. **clash** with "one part per job".

---

## 2 · The rules (G20 to G31)

Each is grounded in a part that exists, so the rule holds by what the part can and cannot do. Add to RULES.md under "Refinement sweep" as a new heading, "Messages (A-22, 8 Nov 2026)".

- **G20 The title is the act, in the button's words.** A dialog's title is the act's verb and its object as the confirm says them ("Revoke Community in Navidrome?", "Retire Safety?", "Cancel the waiting give", "Mark as owned elsewhere"). A question mark only when the act takes something away; an act that records, asks, previews or cancels something unsent is a statement. When the act changes with the data (a Revoke that finds the give never sent), the title changes with it. `ModalHeader.title` is the only title; the `chip` says where the thing comes from (Direct access, Bundle, Rule, the host of a pushed step).

- **G21 A sheet has four places for words.** Header: chip, title, lede (the fact that is true now, at most two sentences, G12). Body: the thing (a form, a plan, a list) and, when a named person loses something, the *What happens* list (`danger-note`: a bold lead and one sentence per consequence, the alternative named beneath). Footer note: the before-line, one faint sentence saying whether the act finishes now or waits and what it cancels (`ModalFooter.note`, `useWaitsBeforeLine`, `useForetell`). Footer pair. Nothing else in a sheet holds a message: no tinted box, no Notice for a consequence, no caption that repeats the lede.

- **G22 Three moments, one message each.** *Before:* lede, What happens, before-line. *Pressing:* the before-line's seat shows the forecast (`ActionForecast`, "Will be revoked"), both buttons hold, nothing else moves (A-19 P2). *After:* the sheet is its result: one `ActionOutcome` block in the body's place; the lede, the What-happens list, the consent tick and the before-line unmount with the form; the header keeps chip and title; the footer is one outline Close. A failed or refused press is also a result: it replaces the before-line and sits under the form, which stays so the act can be pressed again, with its reason read first. A sheet the reader keeps working in (Manage bundles, Add roles, the rule editor's check) is a *work sheet*: its list stays, the result sits between the list and the footer, and the pair becomes Close plus the act only while something is still ticked.

- **G23 One result per surface.** A surface holds at most one `ActionOutcome` and shows it in one slot; setting a new result replaces the old one, and the press clears the slot before it writes (`setOutcome(null)` first, every time). A Notice that was about the previous attempt (a stale plan) clears on the next result. When one press does several things (sends to two systems, applies to nine people), the result is one word that says the whole ("Sent 7 of 9", "Given to 9 of 11 people") and one sentence per part in a fixed order (A-21 §2.6). A page whose empty state already says what happened draws no card for it (Pending changes, §3.14).

- **G24 Button pairs.** The act names its verb and its object ("Revoke this role", "Cancel the waiting give", "Send 9 changes", "Give Community"); once the object is chosen in the form, the button carries it. The dismiss is the ghost half placed by `ModalFooter.cancel` and says what it keeps: **Keep it** (or **Keep them**, **Keep them open**) when the act takes something away, retires, cancels or declines; **Cancel** only on a form that creates, asks or gives, where nothing exists yet to keep. Two buttons in a pair never share a verb. After the result, one outline **Close**. Never OK, Confirm, Yes, Submit, or a second Cancel. `CancelQueuedStep`'s Keep it becomes this slot (ghost, first), not an outline of its own.

- **G25 Red, amber, quiet.** Colour follows what the act does to a named person, never the name of the button. *Red* (the What-happens list on `danger-soft`, `dangerConfirm`, the danger dot on a failure or refusal): a person loses access or something is gone for good, and it reaches Zitadel or a system. *Amber* (warn text and dot, the I-understand tick on `warn-soft`, a warn Notice): a step is left for the reader or a state must be read before pressing (waits for you, a stale plan, a system not answering, a change over the usual size). *Quiet* (the lede, a faint before-line, a neutral dot): what only records, asks, previews or cancels something that never reached anybody. A Revoke that only cancels an unsent give is quiet and its act is accent (A-21 §3). The rung (`Acknowledge.tsx`) is set by what cannot be undone, as today.

- **G26 How long a message stays.** Nothing times out and nothing is a toast. A sheet's result stays until Close; reopened, the sheet starts clean (A-04 §11). A row's inline or seat result stays until the list is next read, never leaving on the tap. A page's own result (Send) stays until the page is next read or left. A Notice stays as long as its condition. The note after Keep it ("Nothing changed. Randy's removal is still waiting.") stays until the next press.

- **G27 Which part carries which message.** `ActionOutcome` is what *you* did: block in a sheet or at the top of a page's content, inline or seat in a row. `Notice` is what is wrong with what you are looking at: stale, failed to read, offline, needs a decision; one per card, under the header (G7); never a result, never a consequence. `NextStep` is the step left, worn by the thing that changed. `RetryStatus` is a change Syndra sends again. Explanations are the lede, a caption or the before-line. A member's own voice ("Not confirmed yet", "On its way") uses the same parts with its own words (`RetryStatus own`, `sendsItselfWord`).

- **G28 Count what the surface counts.** A sentence counts the noun the rows are: people in a people plan, changes on Pending changes, roles in a role set. "Every" only when every. A whole is said when some did not go ("7 of 9"). Never a count of people where the reader is looking at changes, or the reverse.

- **G29 One word per act, and its "Not".** The result word is the act's own past participle: Given, Revoked, Removed (a part out of a whole), Retired, Cancelled, Saved, Created, Asked, Approved, Declined, Extended by 90 days, Marked, Adopted, Sent, Looked up, Recorded (an act whose whole effect is a record). Its failure is "Not" plus the same word ("Not given", "Not sent", "Not saved", "Not extended", "Not put on hold", "Not lifted"). "Refused by Zitadel" when Syndra recorded it and Zitadel refused (copy guide). "Didn't go through", "Couldn't …", "That didn't save." and the generic "Failed"/"Refused" labels are retired from results. A word is the same on every surface the act appears on.

- **G30 Phone.** The sheet's footer is pinned (`ModalFooter` sticky), so the before-line and the pair are always under the thumb and the What-happens list is read above them. A sheet whose content is a list is `lg` and full height; two open lists never stack: a picker whose choice is made folds to its chosen chip (ChosenChip, G15) with the list reopening on tap, so the second list begins in view. The result is the first thing in the sheet after the title, with Close in reach without scrolling.

- **G31 Consent lives in one line and one rung.** The before-line says whether the act finishes now or waits, and what it cancels; rung 2 (tick) or rung 3 (type the name) sits in the body directly above the footer; a disabled act says its reason under itself (`Button.reason`). Nothing else asks: no second sentence about the same consequence, no "Are you sure".

---

## 3 · The sweep

Order: misleading > confusing > cluttered > could be more elegant. "Now" quotes the code; "Rule" names the G it breaks; "New" is the exact words and placement. Where nothing is wrong, the entry says so and stops.

### 3.1 Revoke direct access · `people/RemovalDialog.tsx` DirectDialog (`:271-495`) — misleading

- **Now:** title `Revoke direct access to ${roleLabel}?` (`:325`) also when the give never left Syndra; dismiss "Cancel" (ghost) beside act "Cancel the waiting give" (accent, `:447-489`); after the press the `danger-note` ("{subject} will lose this role." … "ends once the change reaches Zitadel.", `:365-400`) stays above the "Cancelled" block (`:411`), because nothing guards it on `outcome`. Result word for a real revoke is "Removed" (`:103`), message "Direct access to {role} removed" (`:111-161`), forecast "Will be removed".
- **Rule:** G20, G22, G24, G29.
- **New, cancel-only:** chip *Direct access*; title **Cancel the waiting give**; lede as today ("Given to Raquel Rodriguez on 3 Nov by Kanishk Singh, and still waiting for you to send."); body the one neutral line as today; before-line "Nothing is sent. Change history records the give and its cancel."; pair **Keep it** · **Cancel the waiting give** (accent). After: chip + title, block **Cancelled** (words as today), Close.
- **New, revoke:** title **Revoke {Role} in {Project}?**; lede "Given by hand to Raquel Rodriguez on 3 Nov by Kanishk Singh. In Zitadel now."; What happens as today; before-line as today (`waits` or the drain line); pair **Keep it** · **Revoke this role** (dangerConfirm). Pressing: forecast **Will be revoked** (new `FORECAST.revoke`; Q4). After: block **Revoked**, message "Revoked in Syndra. Revocations send on their own, so it leaves Zitadel within a minute" (the queued/sendsItself case; the other cases keep their sentences with "Revoked" for "Removed"), Close. Failure word **Not revoked**. The What-happens list, lede and before-line unmount with the result. Board: `a22-dialog-lifecycle` §1 and §2.

### 3.2 Remove a bundle · `RemovalDialog.tsx` BundleDialog (`:503-714`) — misleading

- **Now:** the "They will lose" / "They will keep" lists (`:566-593`) stay after the result; dismiss "Cancel" beside "Cancel the waiting change" (`:633-708`); act "Remove bundle".
- **Rule:** G22, G24.
- **New:** pair **Keep it** · **Remove the bundle** (dangerConfirm); cancel-only pair **Keep it** · **Cancel the waiting change**; body unmounts with the result; words as today (Cancelled / Removed / Not removed is right here: a bundle is a part out of a whole).

### 3.3 Retire a bundle · `bundles/BundlePage.tsx` DeleteBundleDialog (`:735-866`) — misleading

- **Now:** the What-happens `danger-note` (`:782`) stays above "Retired"; pair "Cancel" · "Retire and revoke" / "Retire {name}".
- **New:** pair **Keep it** · **Retire and revoke** (or **Retire {name}** with nobody holding it); body unmounts; the footer note "Emptying the bundle instead leaves it assignable and grants nothing." is the alternative beneath (copy guide §5) and stays before the press only. Words as today.

### 3.4 Retire a rule · `app/policies/page.tsx` DeleteRuleConfirm (`:560-679`) — misleading

- **Now:** the danger-note stays after "Retired"; pair "Cancel" · "Retire rule and revoke access".
- **New:** pair **Keep it** · **Retire rule and revoke access**; body unmounts. Words as today.

### 3.5 Pending changes › Send · `app/governance/pending/page.tsx:135-168`, `lib/drain-outcome.ts` — misleading

- **Now:** word `statesNothingChanged ? "Not sent" : kind === "queued" ? "Not all sent" : "Sent"` (`:166`): "Nothing was waiting." and "Another send is already running." both under **Sent**; a halted pass with "3 sent" under **Not sent** with "Nothing was changed." appended; `zitadel_offline` reads "Zitadel is not answering. Nothing was sent. Nothing was changed."; the card (`:161`) and EverythingSent (`:253`, "Everything has reached Zitadel." · "{n} changes sent at {HH:MM}. Nothing is waiting for you.") render independently, so a send whose Zitadel rows all went and whose TrueNAS rows were skipped shows both.
- **Rule:** G23, G28, G29, §3.6.
- **New:** one result slot. When the queue is empty after the send, the EmptyState is the result (as built: count, time, "Nothing is waiting for you.", lime dot blooms) and **no card is drawn**. Otherwise one block: word **Sent {sent} of {total}** (or **Sent {n}** when all of a partial filter went; **Not sent** when none went; **Nothing to send** when nothing was waiting; **Not sent** with "Another send is already running. This one did nothing, so no change was sent twice." for `drain_in_progress`); sentence in fold order, one clause per system: "7 reached Zitadel. TrueNAS didn't answer, so its 2 are still here."; detail "Syndra sends them again on its own; each row below says when." (or the drain-off / unknown sentences as today); next step as today's `STOPPED_NEXT`, retry next, or the Send link. `ActionOutcome` stops appending "Nothing was changed." when the sentence already says nothing was sent (regex `/nothing was (changed|sent)/i`). Home's "Send it now" inline result uses the same words. Board: `a22-pending-one-outcome`.

### 3.6 Drift › Adopt / Revoke / Owned elsewhere · `review/UnexplainedAccess.tsx` ResolutionDialog (`:1108-1343`) — misleading

- **Now:** titles "Adopt this access in Syndra?", "Is this owned by another system?", "Revoke this access?" (`:1160-1186`); body in a `bg-tint-1` box (`:1204`, G19); the revoke `danger-note` ("They will lose this role.", `:1221-1244`) stays above "Revocation recorded"; words "Revocation recorded", "Waits for you", "Didn't go through"; adopt message "Resolved. It drops off this list once you close this." (`:1394-1403`); pair "Cancel" · "Revoke access".
- **Rule:** G19, G20, G22, G24, G29.
- **New:** titles **Adopt this access in Syndra** (statement), **Mark as owned elsewhere**, **Revoke this role?**; the subject line "{user} · {project} / {role}" becomes the lede's first sentence, no box; pairs **Cancel** · **Adopt in Syndra**, **Cancel** · **Mark as owned elsewhere**, **Keep it** · **Revoke this role** (dangerConfirm); bodies unmount with the result. Words: **Adopted** "Syndra owns it now and explains it like any other access. It leaves this list when you close this." (+ the rule-waits detail as today); **Marked** "Another system manages it. It leaves this list when you close this."; revoke keeps its five delivery sentences (copy guide forecast table) with the word **Revoked** for applied, **On its way** / **Not in Zitadel yet** / **Waiting for you to send** as today; failures **Not adopted**, **Not marked**, **Not revoked**.

### 3.7 Rule editor · `app/policies/page.tsx` RuleEditor (`:167-539`) — misleading (second list below the fold) and cluttered

- **Now:** Modal md (`:360`); picker 1 and picker 2 each a 46vh list (`RolePickerOne.tsx:147`), so "then also give them" starts past 600px on an 800px screen; the check's warn Notice ("Checked: N things to know", `:~420`) and the outcome block (`:439`) can both stand; after Save the whole form stays under "Saved"; footer: Cancel · Check rule · Save rule · Retire rule.
- **Rule:** G16/G30, G21, G22, G23, G27.
- **New:** Modal **lg**. Each picker folds to its chosen chip once chosen (ChosenChip with ×; the list reopens on the chip), so with "If someone holds" chosen the second list begins directly under the ⇒ line. Two steps in one sheet, like Preview → Apply: pair **Cancel** · **Check rule** until checked; then the check's findings as the *What happens* list (plain lines, amber dot per line, no Notice: "Saving would give 12 people Trained in Laser Lab." / "Also sets off the rule from Trained in Laser Lab, which would then give them Supervisor in Laser Lab." / "3 would lose …"), the "can't be saved" case stays a danger Notice (the rule itself is wrong), and the pair becomes **Cancel** · **Save rule** with **Check again** as outline sm beside the list. **Retire rule** leaves the editor for the rule's row on Automatic rules (outline danger sm, G16), opening DeleteRuleConfirm as today. After Save: the form unmounts; block **Saved** / **Created** with today's detail; Close. Failure **Not saved**. (Q5.)

### 3.8 Give a role · `people/GrantDirectAccess.tsx` — could be more elegant

- **Now:** form unmounts on success (`:154`), stale Notice exclusive with the result, pair "Cancel" · "Give role", footer note stacks the date line ("The sweep removes it on that date. Two weeks before, it shows under Expiring access, where the only action is to extend.", `:241-278`), the role line, the cancel line and the rule lines.
- **New:** act **Give {Role}** once a role is chosen ("Give a role", disabled with its reason, before); the date line moves to the Ends field's `FieldHint`; the footer note keeps delivery ("Raquel can use it as soon as you give it." / the waits line), the cancel sentence and the rule sentences. Words as today (Given / Not given / Refused by Zitadel).

### 3.9 Manage bundles · `people/ManageBundles.tsx` — misleading (one word)

- **Now:** word "Recorded" fixed, so an error wears it (`:305`); no forecast while pressing; a work sheet (rows stay; Close when nothing is staged, `:213`).
- **New:** word **Recorded** on success, **Not applied** on failure (`outcome.recorded` keeps "Refused by Zitadel"); forecast **Will be applied** (new `FORECAST.apply`); the staged-change notes stay (work sheet, G22); the stale Notice as today.

### 3.10 Bulk plan · `ui/RehearsalDialog.tsx`, `PlanReview.tsx`, `people/BulkDialog.tsx` — confusing

- **Now:** the plan stays after Apply by design (A-21 §2.6) and the outcome sits under it (`:739`); `stalePlan` survives a later non-stale failure (`:697/739`), and a failed rehearse's outcome survives a later preview (`:612` only); the `consequence` paragraph in `text-warn-text` (`:727`) stays after the result; "Every change waits for you to send" counts people (`plan-groups.ts:216-232`); mapping dialogs report "Zitadel" for TrueNAS.
- **New:** `rehearse()` and `apply()` both begin with `setOutcome(null); setStalePlan(null)`; `consequence` is a pre-act line and unmounts on the result step; `beforeLine` and `resultOutcome` take `system` and count changes where the fold counts changes; pairs as today (Back/Cancel · Apply to N people; dismiss while busy with its reason). Nothing else changes: the lede by step, the scope tick and the result word are right.

### 3.11 Extend (row seats) · `people/PersonAccess.tsx:798-843`, `app/review/expiring-access/page.tsx:564-707`, Home — confusing

- **Now:** failure word "Not extended" on the person's page and Home, "Didn't go through" on Expiring access (`:701-703`).
- **New:** **Not extended** everywhere. The seat shape (forecast → Extended by 90 days + the plan's sentence + "Nothing for you to do.") is right.

### 3.12 Let it lapse · `expiring-access/page.tsx` LetItLapseDialog (`:714-817`) — confusing

- **Now:** title "Let it lapse?"; lede and the Why field stay above "Recorded"; failure "Didn't go through".
- **New:** title **Let it lapse** (records only, no question mark); body unmounts; pair **Cancel** · **Let it lapse** (nothing is taken away, so Cancel stands); words **Recorded** / **Not recorded**; the before-line "Extend instead if the access should continue." as today.

### 3.13 Put on hold · `review/HoldDialog.tsx` (`:78-144`) — confusing

- **Now:** form stays mounted above "On hold"; failure "Couldn't put it on hold".
- **New:** body unmounts; word **On hold** / **Not put on hold**; pair **Cancel** · **Put {spoken} on hold** as today.

### 3.14 Holds due · `review/HoldsDueForReview.tsx` (`:165-218`) — confusing

- **Now:** failure word "Couldn't lift it"; the server's sentence is replaced by a generic one (`:201-210`).
- **New:** **Not lifted**, message `sentence(failed.message)` then "Nothing was changed." (as `ActionOutcome` already does for failed), "Try again" stays beside the seat.

### 3.15 Zitadel › Users · `app/zitadel/users/page.tsx` Revoke / Give / Replace (`:422-811`) — confusing

- **Now:** body unmounts on success (right); a thrown error's word is the generic "Failed"/"Refused" (`setResult({…, word: ""})`); the stale Notice (`:470`) and an earlier failed block (`:504`) can both stand; pairs "Cancel" · "Revoke these 2 roles in Zitadel" / "Replace roles in Zitadel" (dangerConfirm).
- **New:** words **Not given** / **Not revoked** / **Not replaced** for thrown errors; a stale refusal clears the result slot; pairs **Keep them** · **Revoke these 2 roles in Zitadel**, **Keep them** · **Replace roles in Zitadel**, **Cancel** · **Give roles in Zitadel**. The AcknowledgeCount tick ("I understand this revokes 2 roles.") and the DirectWriteWarning checkbox are right (rung 2, G31).

### 3.16 New role · `roles/CreateRoleDialog.tsx` — misleading (one word)

- **Now:** word "Created" fixed (`:168`); form stays above the result; "Make it wait for you →" after success (right: a next step).
- **New:** **Created** / **Not created**; body unmounts; the ArrowLink stays under the block.

### 3.17 New bundle · `app/bundles/page.tsx:166-306`; Rename · `BundlePage.tsx:633-731` — confusing

- **Now:** form stays above "Created" / "Saved".
- **New:** body unmounts; words as today.

### 3.18 Add roles to a bundle · `bundles/AddRolesToBundle.tsx:182-206` — misleading

- **Now:** on a partial failure a danger-note panel ("{failure} What is still ticked was not added. Press Add again…") and an "Added" block render side by side; Close and Add both show.
- **New:** one block: **Added 2 of 5** (kind failed, `partial`), message "{failure}", detail "What is still ticked was not added.", next "Press Add {what} again for the rest." A work sheet, so Add stays beside Close while anything is ticked.

### 3.19 Roles that wait · `app/changes/roles-that-wait/page.tsx:241-290, 296-413` — misleading

- **Now:** "Stop waiting" fires on one press (fine: reversible, no sheet needed); its "Stopped" block is never cleared, so it can sit above the later danger Notice "That role still waits."; AddRoles' failure is a Notice "Not all of them were marked."
- **New:** one slot per card: **Stopped** / **Not stopped** as inline ActionOutcome under the row; AddRoles' failure is the block **Marked 2 of 4** (partial) or **Not marked**, never a Notice.

### 3.20 Bundle default · `BundlePage.tsx` EndCards (`:554-631`); Project default shape · `projects/ProjectClaimDefault.tsx:279` — misleading (one word)

- **Now:** fixed words "Default set" and "Saved" on failure; ProjectClaimDefault keeps an old "Saved" above fresh edits.
- **New:** **Default set** / **Not set**; **Saved** / **Not saved**; the slot clears on the first edit after a result.

### 3.21 Service accounts · `system/ServiceAccountDialogs.tsx:48-305` — confusing

- **Now:** form replaced by the result (right); failure as a danger Notice "That didn't save." inside the form.
- **New:** failure is the block **Not marked** / **Not saved** / **Not unmarked** under the form; everything else as today ("This finishes now." is a model before-line).

### 3.22 Sign-in lookup · `people/SignInLookup.tsx:55-140` — confusing

- **Now:** failure as a danger Notice "That lookup didn't run." (`:114`); success **Looked up** (right).
- **New:** **Not looked up** as the block, next "Check the address and look it up again."

### 3.23 TrueNAS revoke · `targets/RevokeDialog.tsx:72-205` — cluttered

- **Now:** pre-act warn Notice "Anyone connected stays connected for now." (`:118`); the result is a second `Modal` with its own header (`:72-94`); "Partly done" is `kind: "applied"` and `outstanding` is a warn paragraph beside the block; pair "Cancel" · "Revoke access for {name}".
- **New:** the session sentence is a line in the What-happens list (quiet), not a Notice; the same sheet becomes its result (header kept, body swapped) with the word **Revoked** or **Partly done** (kind `queued`, `partial`, `outstanding` as its detail); pair **Keep it** · **Revoke access for {name}** (dangerConfirm). The typed name (rung 3) is right.

### 3.24 Resolve a log finding / who owns an account · `targets/TargetOverview.tsx:1101-1314` — misleading (no result)

- **Now:** both close on success with no word; both warn in a `border-danger-line bg-danger-soft` box (the danger-note shape, right); "Accept this log and start over" is accent.
- **New:** block **Resolved** "Syndra takes the log as {name} reports it now." / **Recorded** "{username} is {person}'s account. Fixes for both people wait in Pending changes." then Close; "Accept this log and start over" becomes dangerConfirm (records go missing for good, G25); pairs **Cancel** · … (nothing is kept by not resolving; the finding stays either way).

### 3.25 Assign an account · `TargetOverview.tsx` AdoptPanel (`:1491-1625`) — could be more elegant

- **Now:** the sheet closes and the row shows the result with a "Dismiss this result" button.
- **New:** the sheet becomes its result (**Assigned** / **Not confirmed**, Close); the row result leaves on the next read (G26), no dismiss button.

### 3.26 Member storage password · `storage/MyStorage.tsx:695-813` — could be more elegant

- **Now:** hand-drawn `Result` rows (Changed / refused / didn't reach / may not have changed) and a `RetryStatus own`.
- **New:** `ActionOutcome placement="inline"` with the member's words: **Changed** "{label} has your new password."; **Not changed** "{label} refused the change." / "The change didn't reach {label}."; **Not confirmed yet** with `RetryStatus own` as its detail. Same sentences, one part. (Q6, low priority.)

### 3.27 Save an app's shape · `apps/ShapeSheets.tsx:230-292`; Go back · `apps/ClaimHistory.tsx:147-201` — could be more elegant

- **Now:** success is a hand-drawn block (`:233-250`); failure is ActionOutcome; GoBack's BREAKING refusal silently reveals a checkbox.
- **New:** success is `ActionOutcome` **Saved** with today's sentence and faint line; GoBack keeps the checkbox pattern (a rung appearing is a reading, not a result) with its before-line "Tick the box above to go back." as today.

### 3.28 Home limit · `targets/HomeLimit.tsx:125-283` — confusing (vocabulary)

- **Now:** acts "Keep it" (review) and "End it" (danger) beside dismiss "Cancel".
- **New:** **Keep this limit** and **End this limit** (outline danger in the footer, md), dismiss **Cancel**; "Keep it" is reserved for the dismiss (G24).

### 3.29 Pushed cancel step · `ui/CancelQueuedStep.tsx:370-470` — could be more elegant

- **Now:** "Keep it" is an outline Button of its own, first; the act is "Cancel it and {goAhead}"; title, lede, "Nothing changes until you choose." and the forecast "Will be cancelled" are right.
- **New:** Keep it moves into `ModalFooter.cancel` (ghost, first, focus as today). Nothing else.

### 3.30 Home cards · `home/Home.tsx` — nothing wrong, one word

- Extend inline (Extended / Not extended), Retries (Mark seen / Not marked), Open requests seat (Approved / Declined / Not decided) are right. "Send it now" takes §3.5's words.

### 3.31 Member My access · `member/MemberAccess.tsx`, `RecentChanges.tsx`, `requests/AskForAccess.tsx` — nothing wrong, one dot

- "On its way" with its faint line, "Ask to keep it" → "Asked to keep it", the Recent changes words and sentences are right. **Asked** is `kind: "no_change"` (neutral dot); the ask was recorded, so it is `applied` (lime, blooms once). "Not sent" for an ask already waiting is right.

### 3.32 Approve / Decline on a row · `requests/RequestsScreen.tsx:401-447`, `lib/request-decisions.ts` — nothing wrong

- Forecast → word → sentence → step in the seat; "Not decided" beside the returned buttons on a failure; the stale line "Nothing was changed. Liv's access moved while you looked." with "Read the line above, then approve again." All as the rules say.

### 3.33 Ask for access · `requests/AskForAccess.tsx` — nothing wrong

- Form unmounts on the result; "Asked" / "Not sent"; the before-line says who decides. Right.

### 3.34 Replace the signing key · `app/zitadel/page.tsx:312-369`; the guide · `system/SigningKeyGuide.tsx:178-345` — could be more elegant

- The dialog's words (Replaced, Replaced not confirmed, Partly replaced, Not known, Not replaced) are A-19's and right. The guide draws its own header (step bars, title, subtitle); it should be `ModalHeader` with chip "Step 1 of 2" and the lede as the subtitle, the bars drawn under it. UNVERIFIED whether the bars are worth keeping once the chip says the step.

### 3.35 Say what happened · `targets/UnconfirmedChanges.tsx`; Home folders · `targets/ProjectSetup.tsx`; Service account mark · §3.21 — nothing wrong

- Title is the question the operator must answer; the act names the answer ("Record it as done"); the body is replaced by **Done** / **Not done** with "It leaves Waiting on you."; Close. Model sheets.

### 3.36 Shell and page Notices · `shell/ShellNotices.tsx`, `states/DegradedBanner.tsx`, `OfflineBanner.tsx`, `states/index.tsx`, `app/error.tsx`, `operations/page.tsx`, `system/ClaimsDelivery.tsx`, `app/system/targets/page.tsx`, `applications/page.tsx` — nothing wrong

- Every Notice ends on `next` or `nothingToDo`; offline qualifies degraded; ErrorState carries the reference. Right.

### 3.37 Filters sheet · `ui/FilterPanel.tsx:108-177` — nothing wrong

- "Show 12 people" and "Clear all" are two acts, not a pair with a dismiss; order stays.

### 3.38 Lifecycle, stale accounts, merge findings · `targets/TargetOverview.tsx`, `targets/MergeFindings.tsx` — cluttered

- LifecycleControl's failure is a warn Notice inside a CardRow ("That did not go through."); the stale-account confirm's "Cancel" is outline; MergeFindings' error is a warn span. **New:** `ActionOutcome inline` **Not changed** / **Not decided**; the confirm's dismiss is ghost **Keep tracking it**; AlreadyDecided stays a neutral Notice (something is wrong with what you are looking at).

### 3.39 Selection bar, Automatic dialog, source picker, RetiredBundle, IncludeChangeLine, PublishVersionDialog, MoveHoldersDialog — nothing wrong

- PublishVersionDialog and MoveHoldersDialog inherit §3.10. The "This one isn't yours to remove." sheet with one Close is right. The source picker's lone outline Cancel is right (G1).

---

## 4 · Exact words, in one place

| Act | Forecast | Done | Not done | Where |
|---|---|---|---|---|
| Give a role | Will be given | Given | Not given | GrantDirectAccess, Zitadel › Users |
| Revoke direct access | Will be revoked (new) | Revoked (was Removed) | Not revoked | RemovalDialog Direct, Drift, TrueNAS |
| Remove a bundle from a person | Will be removed | Removed | Not removed | RemovalDialog Bundle |
| Cancel a waiting give or change | Will be cancelled | Cancelled | Not cancelled | RemovalDialog (both) |
| Retire a bundle or rule | (none, write first) | Retired | Not retired | BundlePage, policies |
| Approve / Decline | Will be approved / declined | Approved / Declined | Not decided | Requests, Home |
| Extend | Will be extended by 90 days | Extended by 90 days | Not extended | Person, Expiring, Home |
| Let it lapse | (none) | Recorded | Not recorded | Expiring |
| Put on hold / Lift | (none) / Will be lifted | On hold / Lifted | Not put on hold / Not lifted | HoldDialog, Holds due |
| Adopt / Mark owned elsewhere | Will be adopted / marked | Adopted / Marked | Not adopted / Not marked | Drift |
| Apply bundle changes | Will be applied (new) | Recorded | Not applied | Manage bundles |
| Apply a plan | Will be given / removed / extended | Given to 9 of 11 people … | (the same word; refused rows say why) | RehearsalDialog |
| Send | (button pending) | Sent 7 of 9 · Sent · Nothing to send | Not sent | Pending changes, Home |
| Save / Create (rule, role, bundle, shape, default) | (none) | Saved / Created | Not saved / Not created | editors |
| Mark a service account | (none) | Marked / Unmarked | Not marked / Not unmarked | ServiceAccountDialogs |
| Look up sign-ins | (none) | Looked up | Not looked up | SignInLookup |
| Record what happened | (none) | Done / Not done | Not recorded | UnconfirmedChanges |
| Stop waiting / Mark roles | (none) | Stopped / Marked 2 of 4 | Not stopped / Not marked | Roles that wait |

Refused by Zitadel after Syndra recorded it: **Refused by Zitadel**, everywhere, as the copy guide says. Retired words: Didn't go through, Couldn't …, That didn't save., That did not go through., Revocation recorded, Default set (on failure), Failed, Refused (as bare labels).

Dismiss words: **Keep it** · **Keep them** · **Keep them open** · **Keep tracking it** · **Cancel** (forms only) · **Back** (a step behind) · **Close** (after a result).

---

## 5 · Build list, shared parts first

1. **`ui/Modal.tsx` ModalFooter** — `cancel.label` stays, and a `keeps?: string` convenience is not needed; instead a guard test: a footer whose act label begins with "Cancel" must pass a `cancel.label` other than "Cancel" (`Modal.test.tsx`). `CancelQueuedStep` passes Keep it through `cancel` (ghost, `data-keep` focus kept).
2. **`lib/outcome.ts`** — `FORECAST.revoke` "Will be revoked", `FORECAST.apply` "Will be applied"; `NOTHING_CHANGED` not appended when the message matches `/nothing was (changed|sent)/i`; a `notWord(word)` helper is not needed: each surface passes its two words. Guard: `reaudit-outcomes.test.tsx` gains one case per retired word (G29) so none returns.
3. **`ui/ActionOutcome.tsx`** — no change to the part; a lint-style test (`plain-language.test.ts`) fails on a surface that renders `<ActionOutcome` inside a `Modal` without a sibling condition hiding its form (`{done ? … : …}` or `{!done && …}`). Scope: the eleven sheets in §1 finding 1.
4. **`lib/plan-groups.ts`, `ui/RehearsalDialog.tsx`** — `system` threaded into `beforeLine`/`resultOutcome`/`resultSentence`; counts in the fold's noun; `setOutcome(null); setStalePlan(null)` at the start of `rehearse` and `apply`; `consequence` unmounts on the result step.
5. **`lib/drain-outcome.ts`, `app/governance/pending/page.tsx`, `home/Home.tsx`** — the word table of §3.5; the card not drawn when the queue cleared (`cleared && folded` wins); the Home inline result shares the words.
6. **`people/RemovalDialog.tsx`** — §3.1, §3.2 (title by act, Keep it, unmount, Revoked).
7. **`review/UnexplainedAccess.tsx`** — §3.6.
8. **`app/policies/page.tsx`, `bundles/RolePickerOne.tsx`** — §3.7 (lg, chip fold, two steps, Retire rule to the row).
9. **`bundles/BundlePage.tsx`, `bundles/AddRolesToBundle.tsx`, `app/bundles/page.tsx`, `roles/CreateRoleDialog.tsx`, `projects/ProjectClaimDefault.tsx`** — §3.3, §3.16 to §3.20.
10. **`review/HoldDialog.tsx`, `review/HoldsDueForReview.tsx`, `app/review/expiring-access/page.tsx`, `people/PersonAccess.tsx`** — §3.11 to §3.14.
11. **`app/zitadel/users/page.tsx`** — §3.15.
12. **`app/changes/roles-that-wait/page.tsx`, `system/ServiceAccountDialogs.tsx`, `people/SignInLookup.tsx`** — §3.19, §3.21, §3.22 (results, never Notices).
13. **`targets/RevokeDialog.tsx`, `targets/TargetOverview.tsx`, `targets/HomeLimit.tsx`, `targets/MergeFindings.tsx`** — §3.23 to §3.25, §3.28, §3.38.
14. **`people/GrantDirectAccess.tsx`, `people/ManageBundles.tsx`** — §3.8, §3.9.
15. **`storage/MyStorage.tsx`, `apps/ShapeSheets.tsx`, `system/SigningKeyGuide.tsx`, `requests/AskForAccess.tsx`** — §3.26, §3.27, §3.34, §3.31 (last; elegance only).
16. **RULES.md** — G20 to G31 under a "Messages (A-22)" heading; copy guide §5 gains the dismiss-word rule and the word table of §4; `plain-language.test.ts` gains the retired words.

Items 1 to 5 are the shared parts; 6 to 15 inherit them and are each one sitting. The correctness build's four fixes (the owner's findings 1 to 4) are items 5, 6 and the count in 4.

---

## 6 · Questions for the owner

**Owner, 8 Nov 2026: yes to Q1–Q7.** Later the same day the owner corrected the wording of the cancel sheets (give is only a verb): "Cancel giving Community in Navidrome to Raquel?", act "Cancel giving it", dismiss "Keep it waiting", result **Cancelled** "Community in Navidrome won't be given to Raquel. It never reached Zitadel."; the same for a waiting removal ("Cancel removing … from …?") and a waiting bundle. Those words replace "Cancel the waiting give" and "Cancel the waiting change" wherever §3 quotes them.

1. **The dismiss says what it keeps** (Keep it / Keep them) whenever the act takes something away, retires, cancels or declines, and stays Cancel only on a form that creates, asks or gives? *Recommended: yes.* One rule by what the act does. Keep it everywhere reads oddly beside Give a role; Cancel everywhere is the pair you saw.
2. **A question mark only when the act takes something away** (Revoke …? Retire …? Remove …?), never on an act that records, asks or cancels something unsent? *Recommended: yes.* The question is the pause; a reversible act needs none.
3. **After the result, the lede leaves with the body** and the header keeps chip and title? *Recommended: yes.* The lede was the fact before the press; the result's sentence is the fact after it.
4. **Revoke's own words**: forecast "Will be revoked" and result "Revoked" on a person's direct access (and under Drift), leaving "Removed" for a bundle taken out of a person's set? *Recommended: yes.* The title says Revoke; the result said Removed; the verbs rule gives each its own word. (The copy guide's forecast table changes one row.)
5. **The rule editor**: Modal lg; a picker folds to its chosen chip; Check rule then Save rule as two steps in one sheet; Retire rule moves out of the editor to the rule's row? *Recommended: yes.* Four footer buttons is a toolbar, not a pair; the second list below the fold is the finding you made.
6. **Pending changes**: when every change reached, the empty state is the result and no card is drawn; otherwise one card whose word counts changes and says the whole (Sent 7 of 9)? *Recommended: yes.*
7. **The member's storage password results become ActionOutcome inline** (Changed / Not changed / Not confirmed yet) in the member's words? *Recommended: yes, last.* Same sentences, one part; nothing a member sees changes but the shape.

---

## 7 · How the boards read

- **`a22-dialog-lifecycle.dc.html`** (board ≈ 27 KB; App screens page). §1: Revoke before · pressing · after · after-light, 720×520 at half scale (1440 frames). §2: the cancel of an unsent give: today-on-staging (the bug as seen), before · pressing · after · after-light, then two 390 phone frames (before, after). §3: the button-pairs table with the real Button parts. §4: the tone ladder with the real dots. Questions 1 to 4 at the end.
- **`a22-pending-one-outcome.dc.html`** (board ≈ 17 KB; App screens page). §1: today (two cards about one send), proposed partial (dark, light), proposed all-reached (the empty state is the result), and a 390 phone of the partial. §2: the word, sentence and step, case by case. Questions 6 and 7 at the end.

Both boards mount Modal, ModalHeader, ModalFooter, ActionOutcome, Button, StatusDot, PageHeader, AppShell, SyndraRoot, Card and EmptyState from `window.Syndra`; the What-happens list, the source chip, the forecast seat and the queue rows are drawn inline and say so in the board's style comment. Checked mechanically: tags balance, the script runs for every frame, no hex, every `{{hole}}` is a plain lookup. Not rendered.

---

## UNVERIFIED

- Line numbers for `policies/page.tsx` RuleEditor's check Notice (≈`:420`) and for `HoldDialog.tsx`'s form conditionals: the scouts quoted the words and the shape but not every line.
- Whether the rule editor's second list is below 600px for the live role catalogue: the estimate is from the 46vh class and an 800px viewport, not measured.
- `withRuleWaits` (GrantDirectAccess `:327`) and `useRequestDecisions`'s rule-waits sentences: not read; their words may need the "Not" word rule applied too.
- TrueNAS revoke's result sentences (`detail`, `disclosure`, `outstanding`) come from the backend and were not read.
- Whether `SigningKeyGuide`'s step bars are a design-system part; the recommendation is to keep them under a `ModalHeader`.
- The bundle's exported names are inferred from the boards up to A-21; `ActionForecast` is not mounted from the bundle (drawn inline) in case it is not exported.
