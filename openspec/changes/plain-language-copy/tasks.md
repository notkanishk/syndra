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
- [x] 3.7 Review queues, Pending changes, Audit, Change history, Zitadel events, `drain-outcome`, `audit-vocabulary` (missing action keys filled).
- [x] 3.8 Connected systems, target page, mappings, dormant accounts, member storage.
- [x] 3.9 Automatic rules, Access map, Zitadel pages.
- [x] 3.10 Nav labels: Identity provider → Zitadel; Withdrawn access → Unfinished revocations; Event activity → Zitadel events. Order, grouping and hrefs unchanged.
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

## 6. Open

- [ ] 6.1 A person picker for adopting an account (the field is labelled *Person* with a hint saying where the id is found; a staffer still has to paste an id).
- [ ] 6.2 Zitadel grants filter on resolved names (needs the name resolvers to expose their cache synchronously).
- [ ] 6.3 `KiB/MiB/GiB` vs `GB` — TrueNAS shows GiB; kept GiB so Syndra never disagrees with the storage server. Revisit if members ask.
- [ ] 6.4 The guard cannot check tone, referents or stated consequences. Those are checked by reading a sentence to somebody who has never seen the product.
