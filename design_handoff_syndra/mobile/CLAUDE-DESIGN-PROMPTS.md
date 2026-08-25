# Claude Design prompt pack — Syndra on touch

Everything the mobile board does **not** already answer, written as prompts you
can paste into Claude Design one at a time.

## How to use this

1. Paste **Prompt 0** first, on its own, at the start of a Claude Design
   conversation. It is the design system. Every later prompt assumes it is in
   context.
2. Then paste prompts in order. Block A before anything else — every screen in
   Block B inherits from it, and drawing B first means drawing it twice.
3. If you start a fresh Claude Design conversation, paste Prompt 0 again.

## Decisions already taken — stated inside the prompts, not open

| Decision | Ruling |
| --- | --- |
| Board vs codebase conflicts | **The codebase wins.** `ui/src/lib/nav.ts` is the structure authority; the board's 4/8 nav model is wrong about this deployment, and the prompts restate the real tree. |
| Toasts | **Removed everywhere, desktop included.** One confirmation vocabulary. `sonner`, `lib/toast.ts` and `lib/drain-toast.ts` come out. |
| Clipboard | **Honest fallback designed.** `navigator.clipboard` is undefined on http and this deploys on a LAN over http. Both states drawn. |
| Bulk on touch | **All five selection surfaces**, using M25's named `Select` control. No operator loses a capability by picking up a phone. |
| Light theme | **Derived from tokens, not drawn.** Same handling as desktop. One application at a different width, not a second product. |
| Tablet | **Eight figures at 744px**, only where the three-column rule genuinely runs out. |
| Platform behaviour | Session expiry, offline, PWA and landscape are **all in scope**, with prompts of their own (Block D). |

## What the board already covers — do not re-commission

M00 motion · M01 nav shapes · M02 header, freshness, copy row · M03 source
capsule · M04 Today · M05 people index, search overlay, filter sheet · M06 person
access · M07 role members, expiring list · M08 roles index, app token · M09 give
access + cascade preview · M10 token debug · M11 request, both sides · M12 four
list states · M13 access map incl. light · M14 drift list + reconcile drain ·
M15 bundle contents + rung-2 sheet · M16 rule, pending, cascade history · M17
audit, provider health · M18 add-on nav · M19 member storage, three states · M20
target overview + accounts · M21 withdrawn access + revoke rung 3 · M22
rehearsal + mapping versions · M23 holds, both faces · M24 door cards · M25
dormant accounts · M26a tablet · M28 sign-in · M32 back and deep links.

Block B commissions what is **missing from that list but present in the code**.

---

# PROMPT 0 — the design system (paste first, always)

You are extending an existing, finished design system for a product called
Syndra. Do not invent a new one. Do not restyle what I describe. Your job is to
draw screens a developer will build from tokens that already exist.

**Product.** Syndra is the access-management tool for one academic makerspace. It
sits in front of Zitadel (the identity provider) and in front of "add-ons" —
separate services such as a TrueNAS file server and a UniFi Access door system.
Three audiences share one application: a **member** who wants in, a **basic
operator** who assigns access, and an **advanced operator** who maintains the
machinery. The same URLs serve all three.

**The product's central argument:** consequences are visible before you act, and
nothing hides behind a gesture. Every rule below follows from that.

## Canvas

390 × 844 phone figures (iPhone 12–16 logical size), 16px gutters, portrait. Draw
a plain status bar; no device bezel photography, no prototype wiring. Lay figures
side by side with a one-line caption under each saying what it is and why it is
drawn that way.

## Colour — five semantic roles, no decoration

- Ground `#080906` · canvas `#0b0c0a` · rail and sheets `#101210` · cards
  `#141612` · hairlines `rgba(255,255,255,.07)`.
- Text: primary `#f3f5ef` · muted `rgba(243,245,239,.62)` · faint
  `rgba(243,245,239,.42)` · label `rgba(243,245,239,.34)`.
- **Violet** `#7f5af0` fill, `#9b7bff` text, tint `rgba(155,123,255,.13)`,
  hairline `rgba(155,123,255,.28)`. One violet per screen region — the accent
  marks the one thing this region is for.
- **Lime** `#a3e635` means healthy. It is **never a fill and never a button**;
  there is no such thing as a healthy button.
- **Amber** `#f5a524` means a deadline or a broken assumption. Not "warning" in
  general.
- **Red** `#ff5c4d` is destructive only, and appears as a solid fill only once a
  type-the-name gate has been satisfied.

Colour never carries meaning alone. Every dot is paired with a word on the same
line or the next one.

The application is themed from CSS custom properties and both themes are authored
in full, so **do not design a light variant** — draw dark, and use these roles
consistently enough that the light theme falls out of the token swap.

## Type

Bricolage Grotesque 600 for titles — page title 26px at `-.02em`, section and
sheet title 19–22px. Figtree for body, 14px/1.55. JetBrains Mono 13px for ids,
paths and permissions. Row primary 15–15.5px/600, row secondary 13px, uppercase
eyebrow 11px at `.1em`. **Nothing below 12.5px, anywhere.** Type scales with the
platform's dynamic type: rows grow, and 44px is a floor, not a height.

## Geometry

Card radius 18px (small card 16px) · sheet radius `24px 24px 0 0` · row
min-height 60px · copy row 52px · disclosed row body pads `4px 16px 16px` ·
section gap 12px · sheet backdrop `rgba(8,9,6,.72)` · sheet shadow
`0 -30px 70px -30px rgba(0,0,0,.95)` · grabber 38 × 4px, radius 999,
`rgba(255,255,255,.16)`.

## Targets and reach

44px minimum · 50px for anything destructive · 52px for copy rows · **12px
between a destructive control and a benign one, with the benign one nearer the
screen edge**. A destructive control is never adjacent to a benign one along the
thumb arc. The whole row is the hit area, never just a checkbox or a chevron.
Primary actions sit at the bottom, inside the safe area — the top of the screen
is for orientation, not for action. Focus ring `#c9b6ff` 2px at 4px offset.

## Motion — four movements, and nothing else

| Name | Duration | Easing | What it is |
| --- | --- | --- | --- |
| `press` | 90ms | `cubic-bezier(.22,.61,.36,1)` | Scale to `.97` and back, on every tap target, on touch-down not release. |
| `settle` | 260ms | `cubic-bezier(.16,1,.3,1)` | A row disclosing, a list reflowing. Rows below are **pushed, never overlapped**. |
| `rise` | 300ms | `cubic-bezier(.16,1,.3,1)` | A sheet arriving 10px from the edge it will live on. The only surface that travels. |
| `drain` | per item | — | Inline progress that reports as it goes. Failures accumulate in the line rather than interrupting. |

Under `prefers-reduced-motion` each movement gets a **still form, not a shorter
one**: `press` becomes a one-frame tint, `settle` and `rise` cut to their final
position, `drain` keeps its counting text and drops the bar's transition.

## Forbidden, on every screen

Swipe actions · pull-to-refresh · long-press menus · parallax · spring overshoot ·
**toasts** · skeleton shimmer · **tooltips and `title` attributes** ·
horizontally scrolling tables · illustrations · charts, gauges and sparklines ·
hover states of any kind.

Each of those hides a decision behind a gesture with no label. Swipe-to-revoke in
particular is a destructive action with no stated reason and no confirmation rung.

## Rules that outrank aesthetics

1. **A disabled control states its reason as body text, in place.** Never a
   tooltip, never a bare grey-out. "Available once your account exists." is the
   shape.
2. **Structure never moves in response to data.** A section with nothing in it
   keeps its seat and shows a hollow zero.
3. **Queued is not succeeded.** Everything Syndra sends to an add-on is queued and
   dispatched later. Never render queued work as done; queued is amber, not
   accent.
4. **Every id, path, endpoint and connection string is a 52px tap-to-copy row.**
   It confirms in place for 900ms — background `rgba(163,230,53,.07)`, the
   affordance becoming "Copied" in `#a3e635` 600 — then returns. Long strings wrap
   with `word-break: break-all` rather than truncating: an operator reading a path
   aloud needs all of it.
5. **Every list has exactly four states.** Loading: three placeholder rows at the
   real row height, static, no shimmer. Empty: says what would appear and who
   causes it — no illustration, no button to nowhere. Error: names the endpoint
   and status as a copy row and always ends "Nothing was changed." Degraded: amber
   banner pinned under the status bar, not dismissible, rows dimmed to .55, every
   action inert, amber border around the whole frame.
6. **Rows come in two forms.** **A2** is the default — a primary line, a secondary
   line carrying the access source, and a tap that discloses the remaining fields
   as label/value pairs plus any row-level action; one row open at a time. **A1**
   is a stacked card, used where a list is short and every row is a decision
   rather than a scan.
7. **The access source capsule** has three fixed forms, dot before word:
   `Direct`, `Via bundle <name>`, `Automatic`. It never truncates and never
   shrinks below its desktop size; if the row is narrow, the resource name wraps
   instead. On touch its explanation moves into the row's disclosure, because
   there is no hover.
8. **Freshness.** A single line under the page title: a dot, "Read 40 seconds
   ago", and a named 44px Refresh control. Where the read's age gates an action
   the strip is sticky, turns amber, and Refresh takes the accent fill. There is
   **no pull-to-refresh anywhere** — refresh is a named control that says when the
   data was read.
9. **Confirmation is inline, never a toast.** The result of an action appears
   where the action was taken — in the row, in the sheet's footer, or as a result
   step in the sheet that ran it.
10. **Three confirmation rungs**, set by what cannot be undone, never by how
    important something feels. Rung 1: the plan states its numbers and the button
    names them. Rung 2: a ticked sentence carrying the quantity — "I understand
    this removes 34 accounts." Rung 3: type a name to arm a red button.

## Voice

Plain sentences in the product's own words. Say what happened and what it means,
not what the API returned. A member reads "read and write", not `rw`. A hold is a
"hold" to the operator and "withheld" to the member — one record, two words,
chosen for who is reading. Never invent a number; if a count is unknown, the
screen says so.

Acknowledge you have this, then wait for the first screen prompt.

---

# BLOCK A — cross-cutting. Draw these first; every screen inherits them.

---

## A1 — Navigation for the real tree

The mobile board drew a nav model this deployment does not have. Here is the real
one, from the file that is the single source of navigation structure. Design the
touch form of **this** tree.

**Basic operator — 4 top-level entries, but one of them is a group:**

1. Home `/`
2. People `/users`
3. **Access** — a group, not a link, containing Projects `/projects`, Roles
   `/roles`, Apps `/applications`
4. Requests `/requests` — carries a count badge, accent

**Advanced operator — Basic, unchanged and in the same order, then appended:**

5. Bundles `/bundles`
6. **Automation** — Automatic rules, Pending changes *(badge, accent)*, Change
   history, Access map, Settings
7. **Review** — Unexplained access *(badge, red)*, Withdrawn access *(badge,
   red)*, Expiring access *(badge, amber)*, Holds due *(badge, amber)*, Audit
8. **System** — Identity provider, **one row per registered add-on** (TrueNAS,
   UniFi Access — injected from deployment configuration, so the row is present
   whether or not the add-on answers, and carries **no badge**), Event activity

**Member — 3 rows, always, and no view switch:** My access `/`, Requests
`/requests`, Network storage `/storage`. The storage row is **ungated on
purpose** — a member without storage access is asking whether they can get it,
and a missing row does not answer that.

Advanced **appends** to Basic. It never reorders, never renames, and never
inserts anything above Basic's four. That invariant must survive whatever shape
you choose.

**Draw:**

1. **Member — 3 destinations.** Tab bar, inset so the tabs fall under the thumb
   rather than at the corners. No view switch: there is nothing to switch to.
2. **Basic — 4 top-level entries, one of which opens 3 children.** Solve the
   group. Options worth considering: a tab that opens a small sheet of its three
   children; a tab that lands on the first child and offers the other two as a
   segmented control in the page header. Draw the one you would ship and say in
   the caption why. There is **no "More" tab** — a fifth "More" slot would
   quietly turn the rail's rule into "four things and a drawer of leftovers".
3. **Advanced — 4 leaves, 3 groups and per-target rows.** The bottom bar becomes
   a single **Go to** bar (min-height 44px, radius 999, `rgba(255,255,255,.06)`,
   13.5px/600, three-dot glyph) which rises the full nav sheet. Sheet: background
   `#101210`, radius `24px 24px 0 0`, top border `1px solid
   rgba(255,255,255,.09)`, padding `12px 14px 24px`, grabber above. Rows
   min-height 44px, radius 12px, padding `0 14px`, 15px. Only the current section
   is expanded; add-on rows indent to `padding-left: 28px` at 14px under System.
   A `1px rgba(255,255,255,.06)` divider separates Basic's four from Advanced's
   appendix, margin `7px 8px`. Dismiss on grabber, backdrop tap, or picking a
   destination.
4. **Badges.** There are **six** counted indicators and four tab slots. Draw what
   an Advanced operator sees when three of the six are non-zero and the
   destinations carrying them live inside the nav sheet: how the Go-to bar
   reports that something needs attention without becoming a seventh number
   nobody can act on. Badge style: min-width 17px, height 17px, radius 999,
   `#6f4ae0`, 10.5px/700 `#f7f4ff`. Red and amber badges keep their own colour.
5. **The view switch.** Operators toggle Basic to Advanced and back. It is a
   **two-state pill, never a dropdown**, both labels always legible. Switching
   **never navigates and never re-sorts** — it reveals in place. Draw it in the
   nav sheet for Advanced, and in the header for Basic, which has no sheet to put
   it in. Members never see it.
6. **Account, sign-out and the makerspace identity.** Nothing in either handoff
   says where these live on touch, and the desktop top bar carries them. Design
   it. Sign-out clears every navigation stack, which makes it deliberate rather
   than incidental — it should not sit one mis-tap from a destination.
7. **Breadcrumbs.** The desktop derives a breadcrumb from this same tree, so a
   role page reads Access › Roles. On a phone the header carries a back
   affordance instead. Show what happens to the group name — whether Access
   survives as an eyebrow above the page title or is dropped, and why.

**Also solve: reveal-in-place on touch.** A Basic operator on a person's access
page sees, on a role that came from an automatic rule, a control reading "This
came from an automatic rule — Open automation details →". Tapping it switches the
view to Advanced, stays on the same URL, and scrolls the newly revealed panel
into view. On desktop the revealed panel sits beside what you were reading. At
390px it is below the fold. Draw before and after: how the operator knows the
view changed, where they land, and how they get back to Basic without losing
their place.

Draw the nav's own loading state too. Badge counts come from a live read, and
structure never moves in response to data — so a badge whose count has not
arrived yet is neither an empty space nor a zero.

---

## A2 — Confirmation without toasts

Toasts are being removed from this product entirely, desktop included. Today
every mutation confirms through a corner toast, and there are around forty
mutating actions. This prompt designs the vocabulary that replaces them. It is
the most load-bearing prompt in the pack: get it right and half the screen
prompts draw themselves.

**The five outcomes an action can have.** Every one needs a form:

1. **Applied** — it happened, upstream, now. Reversible.
2. **Queued** — Syndra recorded the decision; the add-on has not been told yet.
   The response literally reports `succeeded: 0` so a client cannot default it
   into success. **This is the common case for anything touching an add-on** and
   it must never read as done.
3. **Queued and drained** — recorded, and the dispatch ran on the way out. The
   response says which of the two happened, so the confirmation reads "Marked
   lost and dispatched" or "Marked lost — queued, the drain did not run", never a
   fixed string that guesses.
4. **Refused** — the backend declined: the plan went stale, the cohort was too
   large, a confirmation was missing, the target has no manifest. Always
   accompanied by "Nothing was changed."
5. **Partially failed** — a drain that reported per item, some of which failed.
   Failures accumulate in the line rather than interrupting, and the operator
   reads the outcome when it stops.

**Draw all five**, in the three places an action can start from:

- **From a row in a list** — lifting a hold from the holds list. The row reports
  its own outcome, in place, and the list does not reflow under the thumb.
- **From a sheet with a footer action** — placing a hold. The sheet does not
  close on success; it becomes its own result, with one control to dismiss.
- **From a rehearse-then-apply plan** — the dominant pattern, used by nine
  surfaces. The plan sheet has four steps: compose, scope, review, result. Draw
  the **result** step for a plan where 12 rows applied, 3 were queued, 1 was
  refused and 2 needed no change. Rows group by effect and each effect has a
  fixed word: **Apply · Applied · No change · Refused · Failed · Queued**. Queued
  is amber, never accent. Rows needing nothing are **counted, not hidden**.

**Also draw:**

- **An error with an id.** Failures carry a `request_id` an operator quotes to
  whoever runs the deployment. It is a copy row inside the error, not a footnote.
- **A pending action.** No spinners anywhere. The product's one licensed loop is
  a slowly breathing dot on the control that is working; the control keeps its
  label and is not replaced by the dot.
- **A drain reporting per item**, mid-flight, one failure already in the list and
  the run still going, plus the control that stops the run without dismissing
  anything.

The question the design must answer: with no toast, **where does an operator look
to learn what happened?** The answer has to be the same place every time, on
every screen, or the vocabulary has not replaced the toast — it has scattered it.

---

## A3 — The copy row, and what happens when the clipboard does not exist

Every id, path, endpoint, connection string and command in this product is a copy
row. This deployment is reached over **http on a LAN**, where
`navigator.clipboard` is undefined — so on the network most members actually use,
the copy affordance silently does nothing.

Design the row so it never lies.

**Draw:**

1. **Copy available, at rest.** 52px row, JetBrains Mono 13px value, the word
   `Copy` at 12px `rgba(243,245,239,.42)` on the right.
2. **Just copied.** Background `rgba(163,230,53,.07)`, affordance becomes
   `Copied` in `#a3e635` 600, for 900ms, then back. No toast — a toast on a phone
   covers the value you just copied.
3. **Clipboard unavailable.** The row knows before it is tapped, so it never
   offers something it cannot do. Draw the affordance it carries instead — the
   value selected for manual copy on tap, plus a line saying why, in the
   product's voice. It must not read as an error: the value is fine, the browser
   is the limitation.
4. **A long value.** An SMB path or connection string that wraps to three lines.
   It wraps, it does not truncate, and the row grows.
5. **A copy row inside a degraded page**, where every action is inert. Copy is a
   read, not a mutation — decide whether it stays live and say why in the caption.
6. **A multi-line command block** — the kind an operator pastes into a shell,
   which appears inside the degraded banner when the deployment still carries
   demo data. Same rules, more lines.

---

## A4 — Selection mode, on five surfaces

Five lists support bulk work: **People** (grant, remove or extend for up to 500
at once), **Unexplained access** (attribute or mark-external in bulk),
**Requests** (bulk approve or decline), **Expiring access** (bulk acknowledge),
and **Automatic rules** (bulk confirmation-mode change).

On desktop this is built from shift-click ranges and pointer-drag painting.
Neither exists on touch — a drag is a scroll. The dormant-accounts screen already
solves it with a **named `Select` control**, and that pattern now extends to all
five. Long-press to select is forbidden: it is invisible.

**Draw, using the People list as the worked example:**

1. **At rest** — the list with a named `Select` control in the header. Nothing
   about the rows suggests they are selectable until it is tapped, and the header
   is where the capability is announced.
2. **Selection on, nothing chosen** — rows carry checkboxes at a real touch size
   (the desktop ones are 16px; the floor is 44px and the whole row is the
   target). The page title becomes the count. The header's left control becomes
   select-all. Draw what select-all says when the list is filtered — it must be
   unambiguous whether it means these 12 or all 340.
3. **Nine selected, count bar raised** — the bar names the next step. For a
   destructive or wide-reaching action the next step is **rehearsal, not the
   action**: "Rehearse removal for 9 people", never "Remove 9 people".
4. **An unselectable row** — keeps a dashed left edge and states its reason on
   the row, in words. Never a silent grey-out.
5. **The ceiling** — the backend refuses cohorts over 500. Draw the state where
   an operator has selected all of a 640-row list: what the count bar says, and
   which action it offers instead of the one it cannot run.
6. **Leaving selection mode** — the named control that exits it, and what happens
   to the selection. On desktop, Escape clears it and a bare `a` selects
   everything; on touch both need a visible home, and a bare-letter shortcut that
   selects a whole cohort must not survive contact with a mobile keyboard.

Then draw **the same count bar on the Unexplained access queue**, where the
action is destructive and red, to prove the pattern holds when the next step is
not benign.

---

## A5 — The confirmation ladder on a phone, with a keyboard in the way

Three rungs, set by what cannot be undone. Rung 3 is a phone problem: the field
you type into, the sentence explaining what you are typing, and the button it
arms cannot all be visible at 390 × 844 with a keyboard up.

**Rung 2 — a ticked sentence carrying the quantity.** Draw the sheet for the
dormant-account sweep: "I understand this removes 34 accounts." The **whole row
is the tap target, not the box**, and the button **states its own gate in its own
label** — `Remove · 1 of 2 acknowledged` — because on a phone the unticked box
may be scrolled out of view by the time the thumb reaches the button. This sweep
also requires a password, so draw the sheet with **both** the ticked sentence and
the password field, and show how a password field and a keyboard coexist with a
footer button that must stay visible.

**Rung 3 — type the name.** Four places use it, and each asks for a different
thing:

| Where | What must be typed | Extra |
| --- | --- | --- |
| Taking away a person's access on a target | the person's name | mandatory free-text reason |
| Resolving a binding conflict | the conflicting account's username | — |
| Resolving a log-integrity finding | the target's name | — |
| Adopting an unmanaged account | the account's name | copy reads "There is no undo." |

Draw **two**: the take-away (name, reason, red button, with the reason field
above the confirm field so keyboard order matches reading order) and the adopt
(name only, with the no-undo sentence). Show:

1. The sheet at rest, keyboard down, the full consequence readable.
2. Keyboard up, mid-type, **not yet matching** — the button unarmed and saying
   what it is waiting for, the consequence sentence still visible above the
   field. If something must scroll away, it is not the sentence.
3. Matching — the button armed, solid red `#ff5c4d`, 50px tall, with 12px and a
   line break separating it from Cancel, and Cancel nearer the screen edge.
4. The result, in place, per A2.

Matching is trimmed and case-insensitive, and an empty expectation never arms the
button. The backend refuses these regardless of what the UI sends, so the gate is
about the operator understanding, not about security.

---

## A6 — The freshness strip as a real component

The desktop has no shared freshness component; each add-on screen invents one. On
touch it becomes one component used on every screen that reads live state, so it
is worth drawing properly.

Four classifications, always with an age: **live** (read under a minute ago),
**ageing**, **stale** (over ten minutes), and **provisional** — a read that
answered, but from a target that could not confirm it.

Some actions are gated on freshness: adopting an account cannot proceed on a
stale read, because the account may no longer exist. That is the case the sticky
amber strip exists for.

**Draw:**

1. **Live** — 7px `#a3e635` dot, "Read 40 seconds ago" at 13.5px muted, Refresh
   as a 44px outline pill, `1px solid rgba(255,255,255,.09)`, 13px, padding
   `0 16px`.
2. **Ageing** — same shape, older age. Nothing is blocked; the strip is not
   amber. Say in the caption what distinguishes this from stale, so a developer
   does not collapse the two.
3. **Stale and blocking** — dot and text `#f5a524`, Refresh takes the accent fill
   `#7f5af0` with `#f7f4ff` 600 and padding `0 18px`. The strip is **sticky**: it
   stays while the list scrolls. Below it the gated action carries a **dashed**
   border `1px dashed rgba(255,255,255,.14)` and states its reason as body text
   inside the card — "This account was read 14 minutes ago. Refresh before
   adopting it."
4. **Provisional** — the read answered but the target could not confirm. This is
   neither staleness nor an error, and the word it carries must not be borrowed
   from either.
5. **Refreshing** — the breathing dot on the Refresh control, the previous age
   still legible. The old value stays until the new one lands: a strip that
   blanks while it refetches tells the operator less than it did before they
   tapped it.

Show the strip **above** a set of tabs on a screen where it governs all of them,
and directly under the page title where it governs one list.

---

## A7 — Dialogs become sheets: the full system

Desktop has one modal primitive in three widths (420 / 520 / 760px),
focus-trapped, Escape to close, backdrop to close, both suppressed while an
action is in flight. On a phone every one of them becomes a sheet, and the sheet
needs rules the desktop modal never needed.

**Draw:**

1. **The three sizes on a phone.** What 420, 520 and 760 become. The 760 is a
   rehearsal plan and should probably be full-height; the 420 is a rename field
   and should not be. Draw all three so the difference is visible.
2. **A full-height sheet with a sticky footer** — the rehearsal plan, whose
   footer carries the plan's counts **and its age**, and whose body scrolls
   independently. A plan older than five minutes replaces **Apply** with
   **Rehearse again**; draw that swap.
3. **A sheet with an internally scrolling picker** — the add-roles-to-a-bundle
   list, with a search field, sticky group headers and a scrolling body, inside a
   sheet that itself does not scroll.
4. **A sheet raised from a sheet.** This happens for real: a filter sheet opened
   from a search overlay, and a rung-2 confirmation raised from a plan. Decide
   whether the second sheet replaces, stacks, or pushes the first — draw the one
   you would ship, and say what the back gesture does at each depth. A sheet is a
   level of history: back closes it before leaving the screen.
5. **A busy sheet.** While an action is in flight the sheet cannot be dismissed —
   not by the grabber, not by the backdrop, not by back. Draw how it says so; a
   sheet that silently ignores a dismissal reads as a frozen app.
6. **A sheet whose content is shorter than the keyboard** — a single field with a
   footer button, keyboard up. The footer must not float over the field, and the
   sheet must not grow taller than the space above the keyboard.
7. **The picker sheet that stops short.** When acting on a person, the sheet
   stops 96px short of the top so the person stays visible behind it. Draw it,
   and say which sheets get this treatment and which go full-height.

---

# BLOCK B — screens the code has and the board does not

Each of these exists in the shipped application and has no touch figure. Draw
each screen's ordinary state plus the states named in the prompt. Every list gets
its four states from Prompt 0 without being asked again.

---

## B1 — Projects index, and one project

A **project** is a boundary that owns roles. It is not an app, and the difference
matters: an app receives a token, a project owns the roles a token can carry.

**Projects index.** One row per project. Each row carries the project's name, how
many roles it owns, and how many apps it serves — apps-served is a **column, not
a value**, meaning it is a fact about the project rather than a link to
somewhere. A2 rows.

**One project.** Its roles, with **descriptions shown in full and never
truncated** — a role description that has to be hovered to be read is a role
nobody can safely grant. Below the roles, the apps this project serves.

Draw:

1. Projects index, eight rows, one with zero roles (the hollow zero — the row
   stays, the count reads 0, and the row says what that means).
2. One project, four roles, one with a three-line description.
3. A role row disclosed, showing how many people currently hold it and the way
   into the role's own page.

---

## B2 — An app's token shape: editor and preview

The desktop screen is two equal halves — an editor on the left, a live preview of
the resulting token on the right, each with a hard 420px floor. That is 840px of
side-by-side on a 390px screen, so it has to become something else without
becoming two screens.

**What it does.** An app receives a token. This screen decides the token's
*shape*: which claims it carries, under which names. A project sets a default
shape; an app may override it. Editing the shape changes what every future token
for that app looks like — there is no record of the previous shape, so the
preview is the only rehearsal an operator gets.

Draw:

1. **Editor and preview on one column.** The board's rule for the token debug
   screen is explanation first, evidence second. Decide whether that inverts here
   too, and say why in the caption. The preview is JSON — it is monospace,
   it wraps, and it is a copy row.
2. **Mid-edit, preview stale.** The operator has changed a claim name and the
   preview has not recomputed. The preview must say it is behind rather than
   showing a shape that is not the one being edited.
3. **App overriding its project's default** — the state that says clearly what is
   inherited and what this app has changed, with the control that drops the
   override and returns to the project's shape.
4. **Saved.** Per A2, inline, and honest that the change takes effect on the next
   token rather than now.

---

## B3 — Automatic rules: the editor, validation and deletion

A rule says: when a person holds *this*, give them *that*. Rules fire on their
own, which is what makes them worth the ceremony.

The desktop editor is four dropdowns and a segmented control in one 520px dialog.
Every dropdown is a long list. On a phone, four dropdowns in a sheet is four
sheets deep unless you solve it.

Draw:

1. **The rule editor as a sheet** — the four choices (source project, source
   role, target project, target role) plus the rule's confirmation mode. Solve
   how a long option list is chosen on touch: a sheet per field, an inline
   expanding picker, or a stepped sheet. Draw the one you would ship.
2. **A rule stated back in a sentence** before it is saved — this is the moment
   the operator can tell whether they built the rule they meant. It reads as
   prose, not as four fields.
3. **Validation refused it.** A rule that would conflict with an existing one, or
   name a role that no longer exists. The refusal states which of the two, in the
   sheet, with the offending field marked and reachable.
4. **Deleting a rule.** What a rule stops causing when it is gone, stated before
   the confirm — a rule's deletion does not withdraw the access it already caused,
   and the copy has to say so or an operator will assume the opposite.
5. **Bulk confirmation-mode change** across selected rules, using the A4 count
   bar: which rules queue for review, which fire unattended.

---

## B4 — Automation settings

Two settings, one page, and no touch figure anywhere. It sits inside Automation
as a nav destination.

1. **The global confirmation default** — whether a newly created rule queues for
   review or fires unattended. Changing it does not change existing rules, and
   the copy must say so.
2. **The drift chime** — a 120ms tone the application plays when the count of
   unexplained access findings goes up. Default on. It is honoured only when the
   operator has not asked for reduced motion.

Draw the page, both controls, and:

- The state where the chime is on but the device is silenced or the browser has
  not been interacted with, so the sound cannot play. A toggle that says "on"
  while nothing can be heard is a lie the page is in a position to catch.
- What a settings page looks like with exactly two settings on it — it should not
  pad itself out to look busier than it is.

---

## B5 — Bundles: the whole machinery

The board draws a bundle's contents and the rung-2 sheet for removing a role from
one. Everything else about bundles is undrawn, and this is the screen where
editing reaches the most people at once: **changing what a bundle contains
changes access for everyone holding it.**

Draw:

1. **Creating a bundle** — a sheet with a name and nothing else. The lightest
   sheet in the product; it should look like it.
2. **Renaming**, and **deleting**. Deletion states how many people hold it and
   what happens to them, before the confirm.
3. **Adding roles to a bundle** — the picker with search, sticky group headers by
   project, and a scrolling body inside a sheet. Show a search with two matches
   across three groups, matches highlighted `rgba(155,123,255,.24)` rather than
   bolded.
4. **The welcome bundle.** One bundle is the one every new member receives.
   Setting it is a single control, and the screen must make clear that this is a
   property of the deployment rather than of the bundle.
5. **Versions.** A bundle has a published history. Draw the version list as the
   spine the board uses for cascade history: one dot per version, the current one
   filled. **A version is a record, not a rollback button** — restoring one goes
   through rehearsal like any other edit, and the design must not offer a
   one-tap revert.
6. **Publishing a new version** — the rehearsal, whose plan says how many people
   this reaches, then the result per A2.
7. **Moving holders** from one version to another — same rehearsal shape, and the
   plan is grouped by what each holder gains and loses.

---

## B6 — Unexplained access: triage, revoke, and bulk

This is the highest-stakes screen in the product: access that exists upstream
which Syndra cannot explain. The board draws the list and the reconcile drain.
The three decisions an operator makes about a row are undrawn.

An operator has exactly three answers to a drift row:

- **Attribute it** — this access is explained by something Syndra knows about
  after all; record the explanation.
- **Mark it external** — this access is legitimate and not Syndra's to manage;
  stop reporting it.
- **Revoke it** — this access should not exist. **This is the one place in the
  product where a solid red fill appears.**

Draw:

1. **The triage sheet** for one row, offering all three, with the consequence of
   each stated in a sentence. The three are not equally weighted and must not be
   drawn as three equal buttons.
2. **The revoke confirmation** — solid red, and the copy that earns it: what is
   about to be taken away, from whom, and what it will not undo.
3. **A row Syndra cannot classify** — the sweep that found it compares grant sets
   rather than reading events, so it cannot name who did it. "Unknown actor" is
   the honest rendering, and the row must say why it is unknown rather than
   leaving a blank field.
4. **Bulk attribute and bulk mark-external**, using the A4 count bar into a
   rehearsal. Bulk revoke is deliberately not offered; say so on the screen where
   an operator would look for it.
5. **The queue with a filter applied**, where the per-person count still reads
   over the whole queue — "2 more items for this person" is a fact about the
   person, not about the current filter.

---

## B7 — Expiring access: the three decisions

Grants inside 30 days of lapsing. The board draws the list grouped by date. The
decisions are undrawn, and they are what the screen is for.

Three answers to an expiring grant:

- **Extend it** — push the date out. Presets first, picker second.
- **Let it lapse** — acknowledge that doing nothing is the right answer, with a
  note saying why. The row then leaves the queue, but the grant still expires on
  its own date.
- **Nothing yet** — leave it, and it comes back tomorrow.

An acknowledgement **reopens if the grant changes**: if somebody later moves the
expiry date, the row returns to the queue, because the thing that was
acknowledged is no longer the thing on the row.

Draw:

1. **"Let this lapse?"** as a sheet, with the note field and a clear statement of
   what happens on the date and to whom.
2. **An acknowledged row**, still visible in its group rather than hidden, with
   its note quoted and the control that clears the acknowledgement.
3. **A reopened row** — acknowledged, then the date moved. It must say that is
   what happened, in words, or the operator will read it as a bug.
4. **Bulk acknowledge** via the A4 count bar.
5. The list at its worst: eleven rows expiring inside seven days, the imminent
   group in amber, the rest neutral.

---

## B8 — Requests: asking, withdrawing, and deciding in bulk

The board draws one request from both sides. Three states around it are undrawn.

1. **A member asking.** The form asks **in verbs, not role keys** — "Use the
   laser cutter", not `laser.operate`. Two choices (what, and which project it
   belongs to) plus a free-text reason. Draw it with the keyboard up, and show
   what the member sees when the thing they want is not in the list.
2. **A member withdrawing** a request they no longer need, and the state of the
   request afterwards — withdrawn is not declined, and the copy must not let the
   two collapse.
3. **An operator deciding in bulk** — the A4 count bar into a rehearsal, where a
   mixed selection contains requests that can be approved and requests that
   cannot. The plan groups them, and the button names only what it will actually
   do.
4. **A declined request from the member's side**, carrying the operator's reason
   verbatim. The reason is what the member was told; it is quoted, never
   paraphrased or truncated.

---

## B9 — Everything you can do to one person

The board draws a person's access and its withheld rows. The write surfaces on
that page are undrawn. All of them start from a person and end in a plan.

Draw:

1. **Grant direct access** — pick a project, pick a role, optionally set an
   expiry. The sheet stops 96px short of the top so the person stays visible.
2. **Set expiry** — presets before the picker, because "end of term" is what
   people actually mean. Draw the presets, and the fall-through to a date picker
   for the case they do not cover. Operator-only; a member never sees this.
3. **Manage bundles** — assign and unassign, with the expansion named before the
   operator commits to a preview (the board's M09 shows the shape).
4. **The four removal cases**, which are genuinely four different sheets:
   - removing a **direct grant** — straightforward, states what is left;
   - removing a **bundle** from a person — states which roles go and which stay
     because something else still supplies them;
   - a role that **cannot be removed here** because a bundle supplies it — the
     sheet's job is to say where it *can* be removed;
   - a role that is **automatic** — caused by a rule, removable only by changing
     the rule, which reaches everyone.
   Each states the **residual outcome**: what this person will still hold when it
   is done.
5. **A person with a blocked control** — the endpoint behind one removal does not
   exist yet. Dashed border, reason stated in the row, no tooltip.

---

## B10 — A target's remaining panels

The board draws the target overview and its accounts tab. The real page carries
eight panels and is the densest screen in the product. These are undrawn:

1. **Lifecycle control** — putting a target into drain or quiesce. This stops
   Syndra sending it work. Draw the control, the state while draining (with work
   still in flight), and the state where it is quiesced. This is the one control
   whose effect is measured in what *stops* happening, so it must say what is
   still outstanding.
2. **Merge findings** — two records that appear to describe the same account.
   Resolving one requires a reason. Draw the finding stated as a sentence with
   both sides named, and the resolution sheet.
3. **Binding conflicts** — one Syndra person bound to an account that another
   claim disputes. Rung 3, type the account's username.
4. **Log-integrity findings** — the target's log no longer continues from where
   Syndra last read it. Resolving re-baselines the anchor and is rung 3, type the
   target's name. The copy must say plainly what re-baselining gives up.
5. **Adopting an unmanaged account** — rung 3, "There is no undo.", gated on a
   fresh read per A6.
6. **Reconcile now** — the manual read that re-compares Syndra against the
   target, with the drain reporting per item.
7. **Creating a mapping** — the rule that says which Syndra role produces which
   entitlement on this target. The board draws mapping *versions*; creating one is
   undrawn.

Draw each as a sheet raised from the target page, and draw the target page's own
tab bar carrying eight panels' worth of content — with counts inline, the
freshness strip above the tabs because it governs all of them, and horizontal
scrolling of the tabs if they do not fit.

---

## B11 — The identity provider consoles

Four routes, all Advanced, all undrawn on touch, and all of them **write directly
to Zitadel** rather than going through Syndra's own queue. That is the fact the
design has to carry: these screens bypass every safety net the rest of the
product provides. There is no rehearsal, no cascade preview, no ledger entry —
the change lands upstream immediately.

The four: **provider health** (drawn already as M17b), **upstream users**,
**upstream projects**, **upstream grants**.

Users and projects are both a list beside a detail pane on desktop, with hard
floors around 680px. On a phone they become list-then-detail, which the product
already does elsewhere — the interesting part is not the layout, it is the
warning.

Draw:

1. **Upstream users**, list and detail, with server-side paging (these lists are
   as long as the directory).
2. **Upstream grants**, list with paging, each row naming the person, the project
   and the role.
3. **Assigning and updating a grant** — a sheet, and the sentence that says this
   writes to Zitadel now, with no plan and no undo. Decide what rung this
   deserves and say why. My view is that direct upstream writes deserve at least
   rung 2 even though the desktop asks for nothing; argue me out of it in the
   caption if you disagree.
4. **Creating and removing an upstream project role**, same treatment.
5. **The console when Zitadel is unreachable** — this is the one screen where the
   degraded state is not a degradation of Syndra but of the thing the screen is
   about. Say so differently from the ordinary degraded banner.

---

## B12 — The forensic floor: audit, events, and a raw payload

Two long, time-ordered lists that operators reach when something has already gone
wrong. The board draws their shape; the mechanics are undrawn.

**Audit** is cursor-paginated with an explicit "Load more" — not infinite scroll,
because an audit list that grows under a thumb loses the operator's place. It
filters by actor (a debounced search) and by window (7 days / 30 days / all).
Every line names a human or a named machine.

**Event activity** merges webhook events and onboarding triggers into one stream
and polls every five seconds. Each row drills to its raw payload.

Draw:

1. **Audit, mid-list, with "Load more"** at the end and the count of what has
   been loaded so far. Show what the control says while it is fetching, and what
   it says when there is nothing more.
2. **The time window as a touch control** — a sheet, not a dropdown, and the
   result count in the apply button per the board's filter rule.
3. **A trace id** on an audit row: truncated on the line, full inside the
   disclosure as a copy row. On desktop it is a hover tooltip; hover does not
   exist here, so it moves into the row.
4. **Event activity, live** — rows arriving every few seconds while the operator
   reads. New rows must not push what is being read out from under the thumb.
   Decide how arriving rows announce themselves and draw it.
5. **A raw payload** — JSON, monospace, wrapped, a copy row, in a full-height
   sheet. It is evidence, so nothing about it is prettified or elided.

---

## B13 — Member storage: setting a password, and connecting

The board draws the member's three storage states. Two things inside the ready
state are undrawn, and both are the moment the feature actually gets used.

1. **Setting a storage password.** The member types it on the same phone they are
   reading on. It is sent once and never stored by Syndra in a form that can be
   read back. Draw: the field with the keyboard up, the rules the password must
   meet stated **before** typing rather than as errors after, the in-flight state,
   and the confirmation per A2 which says when it will work rather than claiming
   it works now.
   Also draw **re-setting** one that already exists — the copy differs, because
   everybody who enrolled before a recent change has to set a new one, and that
   population needs to be told why without being alarmed.
2. **Connection instructions.** Three platforms — macOS, Windows, Linux — each
   with a copyable connection string and the steps to use it. Only reachable
   resources appear. Draw the platform choice as a segment, the instructions as
   numbered steps in the member's language, and the connection string as a copy
   row with the A3 fallback state, since this is exactly where a member on http
   will tap Copy and get nothing.

Connection instructions appear **only in the ready state** — they are absent when
there is nothing to connect to, rather than present and disabled.

---

## B14 — The member's own landing

A member's landing **is** their access. There is no separate home, because a
landing in front of the only room is an empty room.

The board draws a member's storage and a member's request. The landing itself,
with everything folded in, is undrawn. It carries:

- **What they can use**, grouped by project, with the source of each in a
  sentence rather than a chip — "You have this because you are in Fabrication",
  not `Via bundle`.
- **Doors**, nested under the role that opens them. A door is a fact about a
  role, not a destination of its own.
- **Their access card**, as one read-only line: whether they have one, and
  whether it works.
- **Anything withheld**, which **leads** the list rather than sitting inside it,
  names the person who placed the hold and the date, and offers the one action
  that resolves it.

Draw the landing in three shapes: a member with a lot of access, a member with
none at all (who needs to be told how to get some, by name), and a member with
one thing withheld.

---

## B15 — Today, with the real block counts

The board drew Basic with two blocks and Advanced with five. The application has
**two in Basic and six in Advanced**, and the sixth is not decoration.

**Basic — 2 blocks:** requests waiting on a decision, and access expiring soon.

**Advanced — the same two, then four appended:** queued writes not yet dispatched,
unexplained access, targets that have never been vouched for, and merge findings.

Every block is **something you can finish**. No counts you cannot act on, no
charts. Each block on Advanced compresses to one line with a subtitle carrying
the fact that makes the count actionable — "14 queued · oldest 3 days" rather
than "14".

Draw:

1. **Basic Today**, both blocks, each showing two rows and a link to the rest,
   the count beside the label rather than below it.
2. **Advanced Today**, all six blocks, one line each, with the Go-to bar in place
   of tabs.
3. **Advanced Today with everything at zero** — six hollow zeros, which is a good
   day and should read as one without becoming a celebration screen. Structure
   does not move: all six blocks keep their seats.
4. **One block mid-action**: the queued-writes block carries a "Resume now"
   control that runs the drain from the landing page. Draw it draining, inline,
   per A2.

---

## B16 — When the application itself fails

Four surfaces, none drawn on touch, all of them what an operator sees on a bad
day.

1. **A render error** — the application caught its own failure and kept the shell
   alive. Draw the recovery card: what it says, what it offers, and how the
   operator gets back to somewhere that works. It must not offer to "try again"
   if trying again does the same thing.
2. **403** — reached a route this session may not use. The member allowlist is
   narrow, so this is reachable by typing a URL or following an old link. Say
   which of the two, if it can be known.
3. **404** — the thing existed and no longer does, or never did. Two different
   sentences.
4. **The degraded banner, both variants**: the identity provider is unreachable,
   and the deployment still carries demo data. The second embeds a command an
   operator runs to clear it — a multi-line copy block per A3, inside a banner
   that is pinned under the status bar and not dismissible.

Draw each at 390px, and draw the degraded banner **over a working list** so the
dimming rule (rows at .55, every action inert, amber frame border) is visible in
context rather than in isolation.

---

## B17 — The access map's controls

The board draws the access map as a centre with what points at it and what it
reaches. Its controls are undrawn, and without them the screen cannot be used.

1. **Node search** — finding the thing to centre on. Full-screen overlay per the
   people-search rule, results grouped by what they are (people, roles, projects,
   apps).
2. **Depth** — a segmented control: one hop, two hops, everything. Draw all three
   states of the map, so the difference between them is legible at 390px.
3. **Re-centring** — tapping any row makes it the new centre. Draw the transition
   and where "back" goes afterwards, because a map you can walk into needs a way
   back out that is not the browser's.
4. **A node with more neighbours than fit** — 40 things point at this role. The
   list is capped and says so, with the way to see the rest.

No pinch-zoom, no pan, no graph drawing. The centre is the page title, so the one
node needs no drawing.

---

## B18 — The token simulator, before it has an answer

The board draws the simulator's two outcomes — a gap explained, and everything
agreeing. It does not draw the screen an operator meets first, which is the one
where they choose what to simulate.

The simulator answers "my app isn't seeing the roles it expects" by computing the
token a given app would issue for a given person, right now.

Draw:

1. **Empty, waiting for input** — pick an app, pick a person. Say what the screen
   will tell them once both are chosen, so the empty state is not a dead end.
2. **One chosen, one missing** — the run control disabled, its reason stated in
   place.
3. **Running** — the breathing dot, both choices still visible and still
   changeable.
4. **The person has no access at all through this app** — a real and confusing
   result that is neither an error nor a gap, and needs its own sentence.

---

# BLOCK C — the tablet range

## C1 — 744px, eight screens where the rule runs out

The middle range is 720–1080px, and the rule for it is: the rail returns
collapsed to 64px (icons and badges, no labels), tables regain **up to three**
columns and the rest disclose, sheets become centred dialogs again, the bottom
bar is removed, the header keeps the view pill, and touch targets stay 44px
because this is still a touch device.

That rule settles most screens. It does not settle these eight, and each needs
one figure at **744 × 1133**:

1. **A target's page** — eight panels, four gates, the densest screen in the
   product. Which three columns survive on the accounts table, and whether the
   panels become two columns or stay stacked.
2. **Unexplained access** — the row is a comparison between two sides. Three
   columns can show the difference or hide it, depending which three.
3. **A rehearsal plan** — now a centred dialog rather than a sheet, with a plan
   long enough to scroll. Where the sticky footer and its counts go.
4. **The access map** — the desktop draws three columns of nodes; the phone draws
   a centre and two directions. 744px is exactly between the two.
5. **Dormant accounts in selection mode** — a count bar plus a collapsed rail
   plus a three-column table, all wanting the same edges.
6. **A person's access** — the desktop puts lineage in side panels, the phone
   puts it in tabs. At 744 both are possible, and only one is right.
7. **The people index** — the highest-traffic screen, where three columns is the
   difference between scanning and reading.
8. **Bundles** — a bundle's contents beside its versions, or one after the other.

For each, the caption states which three columns were kept and what became of the
rest. That sentence is the deliverable as much as the figure is.

---

# BLOCK D — platform behaviour nothing has specified yet

None of these exist in the code or in either handoff. They are the difference
between a website that works on a phone and an application somebody keeps on
their phone.

---

## D1 — Coming back after days away

Sessions are Zitadel sessions on personal phones and last **weeks**. A phone gets
backgrounded for days and reopened on a bus. Today the application has no
expiry-warning UI at all: the session simply stops and the next tap goes to the
sign-in screen.

Draw:

1. **Returning to a still-valid session** after two days. The data on screen is
   two days old. Per the freshness rule, the age is stated rather than silently
   refreshed — and the operator learns the age before they act on it, not after.
2. **Returning to an expired session** on a read-only screen — a member opening
   their access. Where they land, and whether they come back to this screen after
   signing in or to the landing.
3. **Expiring mid-action** — an operator taps Apply on a plan and the session has
   gone. Nothing was changed, the plan is not lost, and the screen says both.
   This is the state that decides whether an operator trusts the application after
   a bad week.
4. **The one thing a session cannot survive**: sign-out clears every navigation
   stack. Draw the confirmation, because on a phone sign-out is a mis-tap and on
   desktop it is a decision.

---

## D2 — Offline, and coming back

Makerspace wifi drops. This is **not** the product's existing "degraded" state,
which means the API answered badly — this means the network is gone and nothing
answered at all. The two must not share a banner.

Draw:

1. **Offline, on a screen already loaded.** What stays readable (everything that
   already arrived), what goes inert (every action), and how the banner says so
   without claiming Syndra is broken.
2. **Offline, on a cold open.** No cached read, nothing to show. The screen says
   what it would show and why it cannot, per the empty-state rule.
3. **A mutation attempted while offline** — refused before it is sent, with
   "Nothing was changed." and the action still armed for when the network
   returns. Do not design a queue: this product's whole argument is that Syndra
   decides and records, and a client-side queue would be a second, invisible
   ledger.
4. **Reconnected.** The banner leaves, the freshness strip goes stale-and-amber,
   and the refresh control takes the accent — because the read on screen is now
   old by definition.

---

## D3 — Installed on a home screen

Members are students. If this is a URL they will lose it; if it is an icon they
will not.

Draw:

1. **The app icon**, from the existing mark — the arch and orb of the sign-in
   screen — at 512, 192 and 180px, on both a light and a dark home-screen
   background, and as a maskable icon with its safe zone marked.
2. **The splash screen** shown while the application starts in standalone mode.
   It is the sign-in composition at rest, not a logo on a plain field, and it
   must not animate — a splash that starts an animation the app then interrupts
   reads as a stutter.
3. **Standalone chrome** — with no browser bar, the status bar sits directly on
   the application. Draw the header with the correct top inset and the status-bar
   tint, and the bottom bar with the home-indicator inset.
4. **The install prompt.** Where a member is told this can be installed, and when.
   Not a banner on first load — somewhere it can be found on purpose. Draw where
   it lives and what it says.

---

## D4 — Landscape

The board is portrait-only. A phone in landscape is 844 × 390, which is 390px of
height minus the keyboard.

Draw:

1. **A list in landscape.** Whether the bottom bar survives, moves, or is
   replaced. It cannot simply stay: it costs a fifth of the viewport.
2. **A sheet in landscape** — a full-height sheet is now 390px tall. Decide
   whether sheets become centred dialogs at this height, like the tablet range
   does, and say why.
3. **Rung 3 in landscape, keyboard up** — the hardest case in the product: a
   consequence sentence, a text field and an armed red button in about 150px of
   remaining height. If it does not fit, the design's answer is a rule, not a
   squeeze.
4. **The sign-in screen in landscape.** The arch is a fixed-ratio composition
   that scales rather than compresses. At 390px of height it has to do something
   deliberate.

---

# What to bring back

For each prompt, the useful return is: the figures, the captions, and any place
Claude Design pushed back on a constraint. A pushback is worth more than a
figure — the constraints in Prompt 0 came from a desktop product, and three of
them (no toasts, no tooltips, no horizontal scroll) are the ones a touch design
is most likely to find expensive.

Paste the returned figures back here and they become the input to an OpenSpec
change proposal under `openspec/changes/`, not a licence to skip one.

---

# Found while reading the code — not design questions

These need a decision or a fix in the repository, and no amount of drawing will
settle them.

1. ~~**A member cannot reach their own storage page.**~~ **Fixed** (`e5a95ec`,
   `257fb31`, recorded as BIA-41/BIA-42): `middleware.ts` reads `memberMayVisit`
   instead of keeping its own copy, and `MEMBER_ROUTES` is now derived from
   `MEMBER_NAV` rather than restating it, so the two cannot drift again.
   Original finding, kept for the record: `lib/nav.ts` lists
   `/storage` as a member route and `nav.test.ts` asserts it; `middleware.ts`
   does not list it in `MEMBER_ALLOWED_PATHS`, so a member tapping their own
   "Network storage" row is redirected to `/`. The rail offers a route the
   middleware refuses. This is a live bug on the surface the mobile work is
   meant to serve, and it will look like a mobile bug the moment mobile ships.
2. **The login arch already has two different scales.** `globals.css` carries a
   `max-width: 480px` block scaling the arch to 320 × 292 with radius 141;
   `mobile/MOBILE.md` M28 specifies 296 × 270 with radius 131 and a different
   inset. One of them is wrong and the CSS is the one that ships.
3. **The person page has three tabs in code and four in the mobile handoff.**
   The handoff's fourth (Cards) belongs to a phase that has not shipped. The
   design should draw what exists plus a stated seat for what is coming, rather
   than drawing four and leaving a developer to guess.
4. **Two endpoints the drawings assume do not exist.** The dormant-account
   listing (`GET /api/v1/targets/{target}/accounts/dormant`) and per-grant
   removal (`DELETE /api/v1/users/{id}/grants/{grantId}`). The board draws
   controls waiting on both, dashed and reason-stated, which is the right
   rendering — but the mobile build cannot close those screens until the
   endpoints land.
5. **`sonner` comes out of `package.json`** when A2 lands, along with
   `lib/toast.ts` and `lib/drain-toast.ts`. `lib/drain-outcome.ts` stays — it
   renders the outcome, and only its presentation changes.
6. **Five `title` attributes carry information available nowhere else**, one of
   them the only explanation for a failed Zitadel read. Hover does not exist on
   touch, and the product's own rule already forbids them. They move into row
   disclosures as part of this work.
