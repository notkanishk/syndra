# Tasks — plain-language copy

## 1. The audit

- [x] 1.1 Seven independent readings of every user-facing string, one rubric (jargon, unclear, missing explanation, inconsistent vocabulary, accessibility copy, tone) — 729 findings: 132 high, 353 medium, 244 low.
- [x] 1.2 Cross-slice vocabulary reconciled: six names for Zitadel, six verbs for ending access, four words for the preview step, four for the queue, three for staff.
- [x] 1.3 Four naming decisions taken with the owner (see proposal.md).

## 2. The guide

- [x] 2.1 `design.md` — audience, register, sentence shape, page and action anatomy, vocabulary with per-page glosses, member/staff split, error rules, accessibility copy, what the guard checks.

## 3. The rewrite

- [x] 3.1 `PageHeader` gains `lede`; `meta` returns to metadata only.
- [x] 3.2 Every page carries a lede (30 headers); every review queue's lede says what inaction means.
- [x] 3.3 Every string in shell, states, login, error pages, `lib/` rewritten.
- [x] 3.4 Every shared primitive rewritten — RehearsalDialog (Preview → Apply), PlanReview, ActionOutcome, ReadFreshness, SelectionBar, Acknowledge, Withheld, CopyableValue, Modal.
- [x] 3.5 People, Home, Requests, member screens.
- [x] 3.6 Projects, Roles, Apps, Bundles, token format.
- [x] 3.7 Review queues, Pending changes, Audit, Change history, Incoming events, `drain-outcome`, `audit-vocabulary` (missing action keys filled).
- [x] 3.8 Connected systems, target page, mappings, dormant accounts, member storage.
- [x] 3.9 Automatic rules, Access map, Zitadel pages.
- [x] 3.10 Nav labels: Identity provider → Zitadel; Withdrawn access → Unfinished revocations; Event activity → Incoming events. Order, grouping and hrefs unchanged.
- [x] 3.11 Relative times read as words ("4 min ago", "3 hours ago", "2 days ago").

## 4. Untrue sentences found and fixed

- [x] 4.1 `ReadFreshness` rendered "read null" on the stale branch.
- [x] 4.2 Holds due rendered a hold's raw resolver value — the word `true` — as its subject.
- [x] 4.3 Pending changes told the reader to "resume" with no such button; the button now says what it does ("Send N changes") and the lede says nothing sends itself.
- [x] 4.4 Access map offered a Depth control and "Expand to 2 hops" that drew nothing — removed.
- [x] 4.5 Zitadel grants filter promised "person, project or role" and matched ids only — placeholder and empty state now say so.
- [x] 4.6 Rule validation said "immediately on save" for a rule set to wait — respects the mode.
- [x] 4.7 Member request screen showed an expiry date computed from now while expiry is set at approval — says "about N days after it is approved".
- [x] 4.8 Zitadel "Remove grant" fired on one click with no consequence — now a consequence sentence and a rung-2 tick.

## 5. The guard

- [x] 5.1 `plain-language.test.ts` — tokenizer-based scan of JSX text and string literals; fails on never-on-screen words, a `PageHeader` without `lede`, a sentence in `meta`, an exclamation mark, a bare button label, a single-verb `aria-label`, and an `ARGUED` entry whose file is gone.
- [x] 5.2 Mutation-checked: an injected banned word fails it; a removed lede fails it; the clean tree passes.
- [x] 5.3 `one-control-surface` `ARGUED` map updated for the renamed adopt-log button.

## 7. The login door, restored

- [x] 7.1 `LoginDoor.tsx` reverted to its pre-sweep wording by the owner's decision. The door is authored copy with its own voice (Syn, who keeps the door) and is not swept with the rest of the product: "Sign in with Zitadel", "Handing you to Zitadel", "Powered by Zitadel", the Syn line.
- [x] 7.2 `lib/login-error.ts` was NOT part of that revert. Its sentences render only on a failed sign-in, not on the door, and they keep the swept wording ("ask makerspace staff") — a member who cannot get in is the last person who should meet a word for staff that appears nowhere else.
- [x] 7.3 One argued exception, in `ARGUED`: `Operator` in `LoginDoor.tsx` labels a test identity in the development sign-in list, which renders only when `mode === "demo"`. A deployed door runs `oidc`, shows one button and no identity list, so no member or staff member can reach the word.

## 8. A hole the revert exposed

- [x] 8.1 The guard could not see one-word string literals, so every banned word that lives as a bare label — a badge, a filter option, a column header — passed it. `{next === "approved" ? "Approved" : "Denied"}` sat in Requests through the guard's own release, along with two more `Denied` labels beside it.
- [x] 8.2 Found by mutation-checking an exception rather than trusting it: the argued entry for `Operator` passed with the word still in the file, which is only possible if the guard never read it. An exception that is never exercised is a permission granted to nobody, and it says the rule is being kept when it is not.
- [x] 8.3 A capitalised one-word literal is now read as a label. Lowercase single words stay out — that is the shape of a code string (`"admin"`, `"oidc"`, a query key). Three real `Denied` labels fixed; mutation-checked with a planted label.

## 9. A rename that was confidently wrong

- [x] 9.1 `Event activity` was renamed `Zitadel events`. The page merges two streams — Zitadel's webhook events AND onboarding triggers, whose own `source` is `webhook` / `manual` / `system` — so it carries rows Zitadel never sent, and its own source filter offers "Zitadel" as one option of three. The title named a subset of the page as the whole of it, and the lede ("What Zitadel told Syndra") was false for every manual or system trigger.
- [x] 9.2 The empty state one screen below said it correctly the whole time — "when somebody changes access in Zitadel, **or when a new person joins**" — so the page contradicted itself in two places a reader sees together. The old name was merely vague; the new one was vague replaced by wrong, which is worse, because a reader believes it.
- [x] 9.3 Now `Incoming events`: what reached Syndra from outside. The lede names both streams.
- [x] 9.4 The guard cannot catch this class. Every word in "Zitadel events" is permitted vocabulary; what was wrong was the claim, and no word list can see a title that names part of its page. It is the same limit as §6.4 — read the sentence beside what it describes.

## 10. The accuracy sweep

Three read-only agents checked every claim in the UI against the code that
produces it: 25 high-severity findings across pages, timing and conditionals.
Reports in the job scratch directory (`V1-pages.md`, `V2-timing.md`,
`V3-components.md`).

- [x] 10.1 **The invented minute.** "within about a minute" appeared in six
  places and was wrong in all of them, against three different mechanisms. It
  was mine: the first sweep turned the mechanism word *cache compile* into a
  promise, and no such cadence exists. The revocation drain is five minutes and
  can be switched off (`cmd/api/main.go:361-372`), the drift sweep is six
  hours, and grant propagation does not move until somebody presses Send. The
  one immediate path — drift revoke — drains inline (`handlers/drift.go:266`)
  and was hedging when it could have been definite.
- [x] 10.2 **A warning that rotted.** Every `/zitadel` screen warned that
  changes there skip Syndra and leave no record. The handlers moved onto
  `EnqueueDirectGrantPropagation`, which writes ledger, audit and outbox in one
  transaction and returns 202. `DirectWriteWarning` now takes `traced` so each
  surface tells its own truth; project and role edits, which really do go
  straight out, keep the original warning.
- [x] 10.3 **Lifting a hold does not give access back.** `handleLiftAllowance`
  updates one column and returns 200; nothing queues a convergence. The page
  claimed the opposite and reported `applied`.
- [x] 10.4 **A halted pass reported three zeroes.** `halt()` returns the
  struct untouched, so the card rendered unmeasured zeros in the same shape a
  healthy converged system produces. `halted` was missing from the UI's type
  entirely. Added, branched on, and covered by `HaltedReconcile.test.tsx`.
- [x] 10.5 **Counts named the wrong thing.** `assigned_user_count` is
  `direct_role_grants` only, so a role forty people held through a bundle read
  `0` under "Members" — and the rule editor said "saving changes nothing today"
  in front of a Save that would reach all forty. The column is now "Direct" and
  the sentence says what it did not count. Separately, a rule's `holder_count`
  counts holders of its *target* role produced by the rule, while the copy
  called it "the first role" — the wrong role named over a real number.
- [x] 10.6 **A trap defused.** `ShadowCredential` is withdrawn from the member
  view behind a comment saying "its backend are intact; restoring it is
  re-adding the line". The backend is not intact — its `PUT` route was deleted
  and setting a credential moved to `POST /me/targets/{target}/credential`.
  Re-adding the line as written would have shipped a password form that 404s.

## 11. The claims behind the copy — fixed

- [x] 11.1 **`zitadel_reachable` now asks Zitadel.** It was `MgmtClient != nil`
  — a question about configuration wearing the words of a question about the
  network, driving a banner that reads "Zitadel is not answering" and gating
  Send. During a real outage, with the client configured, it stayed green.
  `zitadelAnswering` makes the same cheap limit-1 call the propagation drain
  already used, memoised for 30s so a room full of dashboards costs one
  request — and a *failure* is cached for only 5s, because recovery is the
  moment somebody is waiting on. Tested, including a test through the seam
  itself: every test that calls the probe directly still passes if the seam is
  quietly rewired to the old check, which is how the weak version survived.
- [x] 11.2 **Counts include bundle holders.** `GetEffectiveUserCounts` unions
  direct grants with the bundle join through each person's *pinned* version —
  the same join the per-person path uses. The direct-only query is deleted
  rather than left beside it: two functions answering "who holds this" is how
  they come to disagree. Validated against the production schema read-only.
  Rules are still not counted, and that is deliberate — they chain, resolving
  them is an iterative forward pass (`cache.CompileUserCache`), and a SQL twin
  of a resolver is a second definition of holding. The copy names the gap.
- [x] 11.3 **`ShadowCredential` deleted.** 385 lines calling
  `PUT /users/{uid}/shadow-credential`, which the backend removed; setting a
  credential moved per-target to `POST /me/targets/{target}/credential`, which
  Network storage already implements. It was kept alive by a test that mocked
  the dead hook into passing, which is exactly what made it look maintained.
- [x] 11.5 **The handle prefix comes from the row.** It was hardcoded to `"R"`,
  so every bundle cascade printed a rule's handle underneath a legend
  explaining that `R_` meant a rule. The legend was right; the rows were lying
  to it.

### 11.4 Cascade atomicity — assessed, not built

The copy claiming a cascade is "sent together or not at all" was removed
because it was false. It should stay removed, and the behaviour should not
change to match it.

A cascade dispatches to Zitadel and to add-ons over HTTP. There is no
transaction spanning those, so "all or nothing" could only mean compensating
actions — revoking what had already been applied when a later row failed. For
access management that is strictly worse than a partial apply: it takes access
away from people the operator meant to keep it for, to satisfy a property
nobody asked for. Row-at-a-time with retry, halting on an unreachable target,
is the right shape, and Change history already reports the truth when a
cascade lands partly ("8 went through and 2 failed").

The defect was never the behaviour. It was a sentence describing behaviour
nobody had built.

## 12. The vocabulary pass, finished

- [x] 12.1 Every inline gloss of something the audience already knows is gone —
  nine of them, eight of which expanded "Zitadel" into "the service everyone
  signs in through" in the middle of a sentence. The word carries its own
  definition now.
- [x] 12.2 A definition still belongs where the page's whole subject is that
  thing: the Bundles lede saying what a bundle is, the Access map legend, and
  the Projects lede disambiguating a Zitadel project from what "project" means
  in a makerspace. Those are kept deliberately — the rule was never "explain
  nothing", it was "do not explain the same thing on every screen".
- [x] 12.3 `merge finding`, `baseline` and `log anchor` were the three coinages
  still unused after the pass stopped. They are back on the target screens, each
  marked up once.
- [x] 12.4 **`head_rewritten` was told backwards.** `ClassifyLogHead` returns it
  for two different things (`db/log_anchor.go:75-84`): the same record count
  hashing differently, and records having *grown* while the head stayed put. The
  card said "the number of entries is the same" for both, so entries appended
  that chain onto nothing Syndra verified — the more alarming case, on the one
  screen whose job is noticing tampering — was described as its opposite. The
  sentence now follows the two counts, which are already on screen and cannot
  disagree with themselves. Covered by `LogAnchorVerdict.test.tsx`, all three
  branches, mutation-checked.
- [x] 12.5 A `<Term>` puts its definition in the surrounding paragraph's
  `textContent`, so a sentence containing one is never contiguous — two tests
  matched across it and broke. That is a property of the markup, not a fault,
  and it is written on the primitive so the next person meets it as a note
  rather than a puzzle. The definition is deliberately not `aria-hidden`: it is
  what `aria-describedby` resolves to.

## 6. Open

- [ ] 6.1 A person picker for adopting an account (the field is labelled *Person* with a hint saying where the id is found; a staffer still has to paste an id).
- [ ] 6.2 Zitadel grants filter on resolved names (needs the name resolvers to expose their cache synchronously).
- [ ] 6.3 `KiB/MiB/GiB` vs `GB` — TrueNAS shows GiB; kept GiB so Syndra never disagrees with the storage server. Revisit if members ask.
- [ ] 6.4 The guard cannot check tone, referents or stated consequences. Those are checked by reading a sentence to somebody who has never seen the product.
