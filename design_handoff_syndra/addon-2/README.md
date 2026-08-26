# Commission 2 — the target plane as it actually is

## The premise, corrected first

`/system/targets/{target}` **was** designed. Twice:

- `design/Syndra IA.dc.html` **§21 · "Four questions about one add-on"** — health,
  the unmanaged inventory, capabilities, maintenance. Four stacked sections.
- `mobile/Syndra Mobile.dc.html` **M20** — the same four questions as four
  horizontally-scrolling tabs (Health · Accounts · Can do · Drift) with the
  freshness strip above them, because it governs all four.

Both boards say the same thing in their captions: *four questions, in that
order, because each answer changes what the next one means.*

**The page now has eleven panels and no organising structure.** In order:

| # | Panel | Board section |
|---|---|---|
| 1 | Health | §21 ✓ |
| 2 | What the target reports (alerts, pools, services) | **none** |
| 3 | What roles reach here | §24, but drawn as a per-mapping detail *screen* |
| 4 | Published versions | §24, same caveat |
| 5 | People with an account here | **none** — §21 drew only the unmanaged half |
| 6 | Accounts nothing explains any more | §29 ✓ |
| 7 | Accounts Syndra did not create | §21 ✓ |
| 8 | What it can do | §21 ✓ |
| 9 | Waiting on a decision | **none** — postdates both boards |
| 10 | Reconciliation | **none** |
| 11 | Maintenance | §21 ✓ |

Plus two findings that render *inside* Health and have no drawing anywhere: a
change record that has been edited, and two of Syndra's own records disagreeing
about who owns an account.

So the honest statement is not "this page was never designed". It is:

> **Four of eleven panels were designed, individually. The composition never
> was, and the four-question spine both boards specify is no longer visible in
> it.**

That is the commission. Not a restyle — an argument about what this page is now
that it holds eleven answers, and whether the four questions survive as
structure or are replaced by something that admits there are more of them.

## What else has no drawing

Everything below is built, deployed and load-bearing, and appears on no board.

| Screen | Route / endpoint | Why it exists |
|---|---|---|
| Connected systems | `/system/targets` · `GET /api/v1/targets` | A deployment with no add-on registered showed nothing about add-ons anywhere, which reads as the platform not having shipped |
| Waiting on a decision | target page · `GET /api/v1/targets/{t}/merge-findings` | Reconciliation as a three-way merge: differences it refuses to resolve on its own |
| The decision form | `POST …/merge-findings/{id}/resolve` | Five resolutions, a mandatory reason, and a policy hint naming who else is affected |
| What the target reports | `GET /api/v1/targets/{t}/system-health` | The NAS's own health — a failing disk shows up here and nowhere else in Syndra |
| People with an account here | target page · `/inventory` | The managed half of the roster, with Hold and Take away |
| The change record was edited | inside Health · `/log-anchor/resolve` | A chain verifies its own contents and cannot notice its own truncation |
| Two records disagree | inside Health · `/binding-conflicts/{id}/resolve` | Neither store is authoritative; it needs a person, not a reconcile |
| Applied history on a drift item | `GET /api/v1/governance/drift/{id}/origin` | A grant Syndra applied and somebody removed by hand is not an independent mystery |
| Waiting on a decision, on Today | `/` | The home block that routes to the above |

## Files

- `CLAUDE-DESIGN-PROMPTS.md` — the prompt pack, one prompt per commission.
- Prompt 0 is **not** repeated here. Use the one in
  `../mobile/CLAUDE-DESIGN-PROMPTS.md`; the design system has not changed, and a
  second copy of it is a second thing to keep in agreement.
