# Q-04 · Asking for access, an operator's own ask, and where popovers draw

From Claude Code, 15 Oct 2026. Follows A-03. Nothing here retires an earlier answer. Three things found while testing on production, each fixed in code. The fixes are yours to confirm or redraw.

**Answer in the design project at** `handoff/2026-10-reconcile/from-design/A-04-asking-and-popovers.md`, with boards beside it in `handoff/2026-10-reconcile/design/`.

## 1 · Ask for access offered a member nothing to pick (route-requests `ask`)

**Found (Locked).**
- The board's picker is the system's native Select, and the code drew the same one. No new picker was involved.
- The list was empty because of where it read from. It used the operator's lists of projects and roles, which a member may not read. So a member opened the dialog and saw only "Choose…".
- The menu itself is the browser's own, coloured by the person's system accent. That's why it looked unlike anything in Syndra.

**Built (Locked).**
- The dialog reads the member catalogue, the same list My access shows, so the two can never offer different places.
- "Ask for this →" in the catalogue opens the dialog filled in, as before.
- Every other "Ask for access" link opens the dialog on arrival:
  - the no-access state on My access
  - its header button
  - "Ask for access in Requests →" on Network storage
- Before, the person landed on Requests and had to find the button again.

**Question.**
- "What do you need to use?" is a native Select over every place.
- With a dozen places and several roles each, should it become the role picker you drew for bundles (BUILD-LIST 3)? That would mean one search over place and role, with the catalogue's folds.
- Or does the two-step Select stand, because the catalogue on My access already does the browsing?
- Draw whichever you pick, with the place that has nothing defined (`ask-nothing`).

## 2 · An operator asking for their own access (route-requests, route-me)

**Found (Locked).**
- A-03 §4 gave operators My access, where they ask like anyone. Its "Ask for this →" goes to Requests.
- For an operator, Requests is the queue. The queue never read the link, so they landed on it with the dialog shut and their choice lost.

**Built (Locked).**
- The queue reads the link and opens Ask for access over itself, filled in.
- Sent, the request waits in the queue with everyone's.
- Its row says it's their own before approving: "Your own request. Approving it is allowed…" (A-03 §4, owner: allowed and marked).

**Questions.**
1. **Where an operator follows their own ask.**
   - A member has "Your requests" on Requests. An operator has only the queue, which filters to waiting by default.
   - Once their ask is decided, it leaves that view, and nothing on their own pages says what happened.
   - Should "Your requests" also show for an operator, under the queue? Or should My access's Recent changes carry it, since it already quotes decisions?
2. **Asking from the queue itself.** The queue has no "Ask for access" button, on purpose: it's for deciding. Confirm that an operator asks only from My access.
3. **Self-approval in the dialog.**
   - Should the dialog's footer say, before sending, that the operator will be the one deciding it unless another operator does?
   - Or is the queue row's note enough?

## 3 · Dialogs and popovers drew inside the card they came from (system-wide)

**Found (Locked).** Two shapes of one bug.
- **Dialogs.** Reconcile accounts, opened from a mapping row, drew inside its card: clipped by it and over its rows, with the page behind uncovered. A card that has just arrived on the screen still holds its rise animation, and a fixed-position dialog inside it is fixed to the card, not the window.
- **Popovers.** The source popover ("Granted directly", "Carried by a bundle") sat inside cards that clip their contents. On a table's last rows it lost its bottom half, and near the right edge its side. That covers a person's roles, a role's members, expiring access, and inside the removal dialog.

**Built (Locked).**
- Every dialog, sheet and docked note now draws above the page, never inside what opened it.
- Every popover draws above the page too, at its trigger:
  - under it, nudged back inside the window
  - above it when there's more room up than down
  - right-aligned for the Filters panel, whose button ends its row
  - the access map's search results match the search box's width
- Glossary terms (Term) use the same placement. Their upward flip used to cover the word they define.
- A guard test fails any dialog or popover that draws inside the page again.

**Questions.**
1. **Popover placement in the design system.** The rules above aren't written anywhere in yours. Should the design system get a Popover entry with them, so new boards draw popovers as code places them? In particular: the 7px gap, the 16px window gutter, and the flip.
2. **Width at phone sizes.** The source popover is 380px and shrinks to the window less 32px. On a phone, is that right, or should every popover there be a bottom sheet?

New part for `/design-sync` to bring into the design system: `AnchoredPanel` (the popover's placement), beside `Notice`, `NotOpenYet` and `ConnectionGuide`.

## Also changed, no question

A member's pages no longer ask for three operator reads on every load. The access map's bundles, the systems list and the system mode were each refused, silently, and two were retried. Nothing a member sees changes.
