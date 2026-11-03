# Q-12 · Three loose ends from the build

From Claude Code, 3 Nov 2026. Each is small, and each has been left open in a task list.

## 1 · Home's calm panel swaps instantly

`SHIPPED.md:70`: "Home's calm panel and its block grid still swap at once when the view changes on a quiet day; animating it would also animate the data-driven swap, which A-08 doesn't settle."

- **Locked:** structure never moves because of data (basic-advanced-ia). The owner settled "yes" on 28 Oct: the calm panel and the block grid swap by Unfold when the *view* changes, meaning Basic and Advanced.
- **Need:** when the swap is caused by *data* (the last open item closes while the page is open, or the first one arrives), is it still instant, or a crossfade with no height change? And does that hold for both directions?

## 2 · The bulk bar on a phone is too tall

`changes/mobile-touch-view/tasks.md:353`: "The bulk bar is 312px tall at 390 with five verbs stacked — 43% of the scrollport."

- **Locked:** the selection count is said once, in the selection bar. Verbs name themselves (3b). A verb that takes access away has the danger tone. Sheets don't stack.
- **Board:** `route-users` `bulk-a` (desktop) and `route-mobile`.
- **Need:** at 390 wide, choose between:
  - **(a)** one primary verb plus "More", which opens a Modal sm sheet of the rest;
  - **(b)** two rows of icon-and-word buttons;
  - **(c)** a horizontally scrolling row.
  
  Also: which verb is primary when several apply?

## 3 · Two copy lines code wrote itself

`changes/reconcile-build/tasks.md:136`, items f and g.

- **(f)** In the operator's role picker, when a project has no role this person could be given, the build says "…so there's nothing to give". No board words it.
- **(g)** A refusal's result word: the build says "Not given" (for a give that was refused) and "Not sent". A-04 §7 draws "Not sent" for one refusal. "Not given" has no board.
- **Need:** final wording for both, in the copy guide's voice (`openspec/changes/plain-language-copy/design.md`).

**Answer at** `handoff/2026-10-reconcile/from-design/A-12-three-loose-ends.md` on the canvas, numbered 1–3 (with the 2a/2b/2c pick), plus a small board if 2 needs one.
