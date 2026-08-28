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

## 6. Open

- [ ] 6.1 A person picker for adopting an account (the field is labelled *Person* with a hint saying where the id is found; a staffer still has to paste an id).
- [ ] 6.2 Zitadel grants filter on resolved names (needs the name resolvers to expose their cache synchronously).
- [ ] 6.3 `KiB/MiB/GiB` vs `GB` — TrueNAS shows GiB; kept GiB so Syndra never disagrees with the storage server. Revisit if members ask.
- [ ] 6.4 The guard cannot check tone, referents or stated consequences. Those are checked by reading a sentence to somebody who has never seen the product.
