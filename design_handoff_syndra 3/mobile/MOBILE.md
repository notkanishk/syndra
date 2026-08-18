# Handoff: Syndra — mobile and touch

## Overview

`Syndra` is the access-management tool for a single academic makerspace. This
document covers **the touch form of the whole application** — every route,
across three surfaces (member, basic operator, advanced operator).

It is not a separate product and not a separate codebase. Same routes, same
component names, same copy; two breakpoints reflow them. Sections here are
numbered against the main handoff — `M05` is the mobile form of `§05` — so you
can read one beside the other and see only what changed.

**Read `../README.md` first.** It is the normative spec for behaviour, copy,
colour meanings, the acknowledgement ladder and read-freshness rules. Nothing in
this document overrides it; this document says only what touch changes. Where
this bundle and the codebase disagree, `../BUILD-NOTES.md` wins.

## About the design files

The files in this bundle are **design references created in HTML**. They are
prototypes that show the intended look and behaviour. They are not production
code to lift wholesale.

The task is to **recreate this design in the target codebase's existing
environment** — React, Vue, Svelte, a server-rendered template, whatever the
project already uses — following its established patterns, component library and
conventions. If no environment exists yet, choose the most appropriate one for
the project and implement the design there.

`Syndra Mobile.dc.html` is a **board**, not an app: ~60 phone figures laid out
side by side on one canvas, each with a caption saying what it is and why. Open
it in a browser with `support.js` beside it. Every figure is a static 390px-wide
frame — the phone bezels, status bars and keyboard blocks are drawing, not
components to build.

## Fidelity

**High fidelity.** Colours, typography, spacing and target sizes below are final
and exact. The figures are drawn at 390 × 844 (iPhone 12–16 logical size) and one
figure at 744px (M26a). Recreate the UI pixel-perfectly using the codebase's own
libraries and patterns.

Motion is specified by name and duration; the four names come from
`../README.md`'s motion table, with one addition (`rise`).

---

## Breakpoints

Three states. Not four — three is enough to specify and few enough to test.

| Range | Name | Behaviour |
| --- | --- | --- |
| `< 720px` | phone | Single column, 16px gutters. Tab bar for four or fewer destinations, sheet for more. Dialogs, menus and popovers all become sheets. A2 rows by default, A1 where a list is short and decisive. Primary action pinned to the bottom safe area. |
| `720–1080px` | tablet | Rail returns collapsed to 64px, icons and badges only, labels dropped. Tables regain **up to three** columns; the rest disclose. Sheets become centred dialogs again. Bottom bar removed; header keeps the view pill. Touch targets stay 44px — this is still a touch device. |
| `> 1080px` | desktop | The board as drawn in `../README.md`, unchanged. 252px rail with labels, full tables, hover popovers. |

The middle range is where a floor operator with a tablet lives, and may be the
real mobile user. It is drawn once (M26a) because it introduces nothing new — it
is the desktop layout with fewer columns.

---

## Motion and interaction

Cohesion on touch is mostly motion discipline. Desktop has five timings
(`tint`, `press`, `settle`, `arrive`, `flash`); touch uses `press` and `settle`
unchanged, adds `rise`, and re-scopes `arrive` to `drain`. **If a transition is
not one of the four below, it is a bug.**

| Name | Duration | Easing | What it is |
| --- | --- | --- | --- |
| `press` | 90ms | `cubic-bezier(.22,.61,.36,1)` | Scale to `.97` and back, on **every** tap target. Fires on touch-down, not release. The only feedback a finger gets, since there is no hover to inherit. |
| `settle` | 260ms | `cubic-bezier(.16,1,.3,1)` | A row disclosing, a list reflowing, content arriving in place. Rows below are **pushed, never overlapped** — the operator keeps their place in the list. |
| `rise` | 300ms | `cubic-bezier(.16,1,.3,1)` | New on touch. A sheet rises **10px** from the edge it will live on. It is the only surface that travels. |
| `drain` | per item | — | Progress that reports as it goes, inline, never in a modal. Failures accumulate in the line rather than interrupting; the operator reads the outcome when it stops. |

`tint` is dropped: there is no hover on touch. `flash` is kept as authored for
values that change while the page is open.

### Forbidden

Swipe actions. Pull-to-refresh. Long-press menus. Parallax. Spring overshoot.
Toasts. Skeleton shimmer that outlives the request.

Every one of them hides a decision behind a gesture with no label, and the
product's whole argument is that consequences are visible before you act.
Swipe-to-revoke in particular is a destructive action with no stated reason, no
visible label and no rung on the ladder.

### Colour

Unchanged from `../README.md`. One violet per screen region. Lime is never a fill
and never a button. Amber is a deadline or a broken assumption. Red is
destructive only, and solid only once a rung-3 gesture is satisfied. **Touch adds
no colours** — a phone screen has less room to spend them in, not more.

### Targets and reach

- **44px** minimum, **50px** for anything destructive, **52px** for copy rows.
- The **whole row** is the hit area — never just the checkbox or the chevron.
- **12px** between a destructive control and a benign one, with the benign one
  nearer the screen edge. A destructive control is never adjacent to a benign one
  along the thumb arc; it sits on its own line or is separated.
- Primary actions sit at the **bottom**, inside the safe area. The top of the
  screen is for orientation, not for action.
- Focus ring `#c9b6ff` 2px, 4px offset, `press` timing.

### Reduced motion

`prefers-reduced-motion: reduce` gives each movement a **still form, not a
shorter one**:

- `press` → a one-frame tint.
- `settle` and `rise` → cut straight to the final position.
- `drain` → keeps its counting text, drops the bar's transition. The text was
  always the message.

---

## Navigation

**Item count decides the placement.** Four or fewer destinations become a tab
bar; more than four become the rail arriving as a sheet on `rise`. Order, labels
and badges are the desktop rail's, unchanged — Advanced still *appends* to Basic,
and nothing sits above the first four.

| Surface | Destinations | Shape |
| --- | --- | --- |
| Member | 2 (3 with the TrueNAS add-on) | Tab bar, **inset** so tabs fall under the thumb rather than at the corners. No view switch — there is nothing to switch to. |
| Basic operator | 4 — Today, People, Requests, Roles | Tab bar, in rail order. **No "More" tab.** View pill in the header. |
| Advanced operator | 8 — the four above plus Review, System, Automation, History | The rail, unchanged, arriving from the bottom edge with a grabber. View switch at the top of the sheet. |

There is deliberately **no "More" tab**. A fifth `More` slot would quietly
reorder the rail's own rule into "four things and a drawer of leftovers".

**Bottom bar spec.** Background `#101210`, top border
`1px solid rgba(255,255,255,.07)`, padding `8px 6px` plus the home-indicator
inset. Each item: min-height 52px, radius 13px, a 7px dot above an 11.5px label,
gap 5px. Active item: background `rgba(155,123,255,.13)`, dot `#c9b6ff`, label
`#c9b6ff` 600. Inactive: dot `rgba(243,245,239,.34)`, label
`rgba(243,245,239,.5)`. Badge: min-width 17px, height 17px, radius 999,
background `#6f4ae0`, 10.5px/700 `#f7f4ff`, top 4 right 16.

**Nav sheet spec.** Background `#101210`, radius `24px 24px 0 0`, top border
`1px solid rgba(255,255,255,.09)`, shadow `0 -30px 70px -30px rgba(0,0,0,.95)`,
padding `12px 14px 24px`. Grabber 38 × 4px, radius 999,
`rgba(255,255,255,.16)`, centred with 10px below. View switch is a segmented
pill: container `rgba(255,255,255,.045)` radius 999 padding 4, active segment
`#7f5af0` / `#f7f4ff` 600. Rows: min-height 44px, radius 12px, padding `0 14px`,
15px. Divider between Basic's four and Advanced's four:
`1px rgba(255,255,255,.06)`, margin `7px 8px`. Add-on destinations **indent** to
`padding-left: 28px` at 14px under their parent — the sheet can afford a second
level where a tab bar cannot. Only the current section is expanded.

Dismiss on: the grabber, a backdrop tap, or picking a destination. Backdrop
`rgba(8,9,6,.72)`.

Advanced replaces the tab bar with a single **Go to** bar: min-height 44px,
radius 999, `rgba(255,255,255,.06)`, 13.5px/600, with a three-dot glyph.

---

## Header and freshness

The freshness strip becomes **a single line under the page title**, with refresh
as a 44px target on the right. Where the read's age gates an action the strip is
**sticky** and stays while the list scrolls.

- Live: 7px dot `#a3e635`, `Read 40 seconds ago` at 13.5px
  `rgba(243,245,239,.62)`. Refresh is an outline pill,
  `1px solid rgba(255,255,255,.09)`, 13px, padding `0 16px`.
- Stale **and blocking**: dot and text `#f5a524`; refresh takes the accent —
  `background: #7f5af0`, `#f7f4ff` 600, padding `0 18px`.

A blocked action keeps its **dashed** border (`1px dashed rgba(255,255,255,.14)`)
and states its reason in place, as body text inside the card. Never a tooltip —
touch has no way to open one.

**No pull-to-refresh, anywhere.** Refresh is a named control that says when the
data was read. A hidden gesture that silently re-reads a target undermines the
one pattern every add-on screen depends on.

Sticky regions, in order: page title, freshness strip, list group headers,
bottom action bar.

### Tap to copy

Every monospace id, endpoint, request id and connection string becomes a **52px
row** with a `Copy` affordance at 12px `rgba(243,245,239,.42)`. On tap the row
itself confirms — background `rgba(163,230,53,.07)`, the affordance becomes
`Copied` in `#a3e635` 600 — for **900ms**, then returns.

**No toast.** A toast on a phone covers the value you just copied. Long strings
`word-break: break-all` and wrap rather than truncate: an operator reading a path
aloud needs all of it.

---

## Density: how tables become touch

Two row forms. Both keep the source capsule; neither hides a column behind a
gesture.

**A2 — line, then disclosure. The default.** Primary line 15–15.5px/600,
secondary line carrying permission and the source capsule. Row min-height 60px.
Tapping discloses the remaining fields as label/value pairs at 13.5px, plus any
row-level action. **One row open at a time**, on `settle`; rows below are pushed.
The disclosed body pads `4px 16px 16px`. Use for any list that can exceed about
eight rows.

**A1 — stacked card.** Every column becomes a labelled line inside a card
(`#141612`, `1px solid rgba(255,255,255,.07)`, radius 18px, padding 16px). Use
where a list is short and every row is a **decision**, not a scan: one person's
access (M06), holds review (M23), door cards (M24).

A horizontally scrolling table was considered and **rejected**: it hides half the
columns behind a gesture a finger does not discover, and `../README.md`'s rule
that a disabled action states its reason **in the row** cannot hold if the row
runs off-screen.

### The source capsule on touch

Three kinds, fixed vocabulary, dot before word — unchanged from `§03`
(`Direct`, `Via bundle <name>`, `Automatic`). Capsules **never shrink below their
desktop size**; if a row is too narrow the resource name wraps, the capsule does
not truncate.

What changes is the explanation. Desktop hovers a popover; touch has no hover, so
**the popover's content moves into the row's disclosure** — including the
sentence about where a bundle grant can be removed. Without that move, touch
loses it entirely.

---

## Screens

Every route in `../README.md` reflows. Sections below are the ones where touch
changes something worth drawing; a route not listed reflows by the rules above
with no decisions left open.

| § | Screen | What touch changes |
| --- | --- | --- |
| M01 | Navigation, all three surfaces | Count-driven tab bar / sheet, above. |
| M02 | Header, freshness, copy rows | Single-line strip, sticky, named refresh. |
| M03 | Source capsule | Popover → disclosure. |
| M04 | Today (landing) | Blocks stack in desktop order and **keep their own actions**. Basic: two blocks, each showing two rows and a link to the rest, count beside the label rather than below it. Advanced: five blocks compressed to one line each with a subtitle carrying the fact that makes the count actionable, and a *Go to* bar in place of tabs. Collapsing blocks into bare counts would make it the summary dashboard `§04` refuses to be. |
| M05 | People | **Search is a full-screen overlay** — tapping search replaces the page, keyboard up, results under the field. Results group by *what they are* (People / Accounts on targets), matches highlighted `rgba(155,123,255,.24)` not bolded. Filters live in a sheet with a count on the trigger; the apply button carries the **result count** (`Show 3 people`) and filters apply on *Show*, not on tap. Group headers sticky. |
| M06 | One person (E3) | A1 cards. Granted above automatic, section counts as headers. Only a direct grant carries a remove button; the other two explain in place why they have none. Person tabs (Access / Holds *n* / Cards *n* / History) replace desktop side panels — four fit at 390px. A withheld row **leads** its section. |
| M07 | Who can use this (E2) | A2 rows: name is the primary line, the capsule is the whole second line. Header sentence answers the question in words before the list does. Expiring list groups by date, nearest first, imminent group in amber. |
| M08 | Projects, roles, apps (E1) | Roles / Bundles / Apps become a **three-way segment**, not three destinations — they answer one question. Second lines carry population and reach. An app's page keeps the token-lifetime sentence as prose. |
| M09 | Giving access (E4/E5) | Two steps in place, not a wizard. Picker sheet stops **96px short of the top** so the person you are acting on stays visible. Footer names the bundle expansion before the operator commits to a preview. Preview: rule-caused grants are a separate, amber-headed group — the operator did not ask for them. |
| M10 | Token debug (E6) | **Explanation first, evidence second** — inverted from desktop, where the two lists sit side by side. The healthy case collapses the two lists into one and ends by saying where to look next. |
| M11 | A request, both sides (E7) | Rung 1: pinned action bar, no dialog. Checks are dots and words, the neutral one an outline rather than a colour. Decline sits away from the thumb arc; Approve is the wide target. Member side names who is deciding and roughly when; the disabled nudge states the rule and the current count. |
| M12 | List states | Four, no exceptions. **Loading**: three placeholder rows at the real row height, **static** — no shimmer, which on a phone reads as activity that isn't happening. **Empty**: says what would appear and who causes it; no illustration, no button to nowhere. **Error**: names endpoint and status as a copy row. **Degraded**: amber banner pinned under the status bar, **not dismissible**, rows dimmed to .55, every action inert, amber border around the whole frame so the state is legible at arm's length. |
| M13 | Access map (S5) | A graph with labels is unreadable at 390px, so it becomes what it actually is: **a centre, what points at it, what it reaches**. The centre is the page title, so the one node needs no drawing. Tapping any row re-centres. No pinch-zoom. Drawn in light as well as dark (accent darkens to `#5b3fd6`, glow dropped — a bloom on white reads as a smudge). |
| M14 | Drift and reconcile (S6/S7) | Two columns become **two lines**: the difference stated as a sentence with both sides bolded, so no column headers are needed. The kind of drift is a word at the end of the first line, not a coloured badge. Reconciling `drain`s inline per row; a failure states the target's own words and stays in the list rather than interrupting. The operator can stop the run without dismissing anything. |
| M15 | Bundles (S1/S2b) | `Remove` is a **named text control** at the row's end, 44px tall, never an icon. Rung-2 sheet: the whole row is the tap target, not the box, and the button **states its own gate** in its label (`Remove · 1 of 2 acknowledged`) because on a phone the unchecked box may be scrolled out of view when the operator reaches the button. |
| M16 | The causal chain (S2/S3/S4) | Three columns become **one thread**. Every screen carries a back-line naming where you came from plus the cascade id as a link, so the thread survives four taps deep. Dependent writes say what they are waiting for instead of showing a spinner. History becomes a spine: one dot per event — filled for done, ringed for a rule firing, hollow for not yet. |
| M17 | Forensic floor (S8–S11) | One sentence per event in the product's own words, consequence on the second line. A time window replaces the desktop filter bar; everything else is search. Provider health is a dot, the kind, and the measured latency — no gauges, no sparklines. |
| M18 | Add-on platform navigation | Adds one destination to Review, one to System, one member tab; **removes nothing from Basic**. Member's two tabs become three. TrueNAS earns a tab (something to look at, a password to set); Unifi does not — a door is a fact on the access list, not a place. |
| M19 | Member › storage | **Three** states, and the middle one is ordinary. A two-state page lies to the member. `Ready` / `Being set up` (violet, not amber — this is normal, not a deadline; ends with "Nothing for you to do") / `Withheld` (names the person and the date, offers the one action that resolves it). Permissions in the member's words — "read and write", not `rw`. |
| M20 | A target (System) | Four tabs — Health, Accounts *n*, Can do, Drift *n* — scrolling horizontally with counts inline, freshness strip **above** them because it governs all four. Capabilities are dots and words; a missing one is **dashed, not red** — absent is not broken. The unsupported capability is stated here so it is never a surprise inside a revocation dialog. Unclaimed accounts sit above claimed. |
| M21 | Withdrawn access (Review) | **Two populations, never one count.** "Will end by itself" (open sessions, neutral) and "Will not end by itself" (target refused, amber). Each group header explains itself in a subtitle. Only the second carries amber — the first is expected behaviour. |
| M22 | Plan, then apply | Rehearsal sheet goes **full-height** with a sticky footer carrying the plan's counts *and its age*. Four honest groups: will change / will be left alone / cannot be done. A plan older than five minutes replaces *Apply* with *Rehearse again*. Mapping versions are a record, not a rollback button — restoring one goes through rehearsal like any edit. |
| M23 | Holds | One object, three faces: operator sees a *hold*, member sees *withheld*, review sees a list. **The word must never slip.** Placing a hold is rung 1, but the reason field is required and labelled with who reads it — that label is why holds get written in plain words. In review, **age** is the column that matters: top-right, amber past 60 days with a sentence saying why. Reasons are quoted verbatim; they are what the member was told. |
| M24 | Door cards | The one flow that is **better** on a phone — enrolment happens next to the person, at the reader. The waiting state names the reader and states its own timeout; rings are static, they mark a target rather than pretending to pulse. Lost-card revocation `drain`s per door and says plainly which doors still accept the card. Red here is a state, not a button. |
| M25 | Dormant accounts | The one bulk screen. **Long-press to select is invisible**, so a named `Select` control in the header turns rows into checkboxes and raises a count bar; the title becomes the count and the header's left control becomes select-all. Unselectable rows keep a dashed left edge and their stated reason. The action bar names the next step — **rehearsal, not removal**. |
| M26 | The middle breakpoint | 744px, drawn once. |
| M28 | Sign-in | See below. |
| M32 | Back, history, deep links | See below. |

---

## Sign-in on touch (M28)

The spec is `../login/LOGIN.md` and it is unchanged in kind: **one action**, sign
in through Zitadel. No email field, no password, no reset, no rate limit —
Zitadel owns all of that, so Syndra has no credentials to check and cannot refuse
them. Three states: `idle | opening | unreachable`.

Two things change on touch.

**1. Scale (§7).** The composition does not compress; it scales.

| Element | Desktop | Phone |
| --- | --- | --- |
| Arch box | 430 × 392, radius `190px 190px 0 0` | **296 × 270, radius `131px 131px 0 0`** — the 430:392 ratio kept, the radius scaled with it |
| Group offset | `margin-top: -52px` | `margin-top: -36px` |
| Wordmark | 58px | **40px** |
| Orb | 46px at `top: 54` | **40px at `top: 38`**, dot 14px |
| Wordmark → button gap | 34px | 23px (`bottom: 52px` on the wordmark) |
| Button | auto width (~232px), `translateY(calc(50% + 14px))` | unchanged width; `translateY(calc(50% + 10px))` |
| Base text | `bottom: 44`, messages `bottom: 74` | `bottom: 30`, messages share the same slot |

The button keeps its auto width and **still straddles the arch baseline** — at
~232px it fits 390px with room, so §7's permission to drop it below the arch is
not needed. Page background is `#0a0b08`, **not** the app shell's `#080906`.

Everything else is `LOGIN.md` verbatim: the arch mask
(`linear-gradient(to bottom,#000 6%,rgba(0,0,0,.5) 54%,transparent 95%)`), the
wash, the four orb layers, the eyebrow `Makerspace Syndra`, the Syn line, the
credit row, the three-layer button shadow, and all entrance timings and easings.

**2. The lit ring has no pointer to track.** The answer is already in the spec:
§5's keyboard fallback becomes the permanent touch state. The lit ring holds
`linear-gradient(0deg, #000 4%, transparent 80%)` — lit from below, where the
button is — at **opacity 1**, bloom **.9**. There is no pointer to release it
back to, so it never releases. The orb is lit by the thing you are about to
press.

Do **not** substitute device tilt or touch-tracking. Both are the kind of
gesture-driven ambience the forbidden list rules out everywhere else in the
product, and the arch's fade is a static mask by design.

State visuals, unchanged from `LOGIN.md` §2–§3:

- `opening` — arch clip height halves and the stroke fades to 0 (retracting alone
  leaves the uprights with cut ends); wash to **0**, not a low value, or it keeps
  holding the silhouette; button to `rgba(127,90,240,.34)` /
  `rgba(247,244,255,.62)` with the label `Opening…`; orb bloom to 1; pool
  brightness up; `Handing you to Zitadel.` / `You'll come back here signed in.`
  The redirect is issued at the **start** — the animation is cover, not a gate.
- `unreachable` — the arch's mask is **removed** so the stroke renders as a
  complete closed line: the door is shut, told by geometry rather than a banner.
  Border to `rgba(245,165,36,.5)` via the CSS transition, violet pool to .15,
  amber pool to 1, orb to .35; `Zitadel didn't answer.` /
  `Nothing was signed in. Try again in a minute, or find a steward in the space.`
  Retrying returns to `idle` and replays the entrance.

Sessions are **Zitadel sessions on personal phones and last weeks**, so most
operators meet this screen once. That rarity is what earns it the ceremony.

Read `LOGIN.md`'s *Reset* and *Gotchas* sections before writing the animation
code; both traps apply identically here.

---

## Back, history and deep links (M32)

**Per-tab stacks, with one thread that crosses them.**

Each tab keeps its own place, so switching tabs never loses work. The exception is
a cascade: following `csc_2f81b0` from Automation into System is **one
investigation, not two**, and back must retrace it. The thread lives in the tab
it started in, even when its later screens belong to another tab — System's own
stack is untouched.

On entering a cross-tab thread the screen carries one extra line naming the
situation (`You are in System, following a thread that began in Automation.`).
It appears **once**, on entry — it is the only place the product explains a
navigation model on screen.

**Arriving cold** on a deep link — from a chat message, with no history behind
it — back reads **`Today`**. It does not reconstruct a chain the operator never
walked; an invented history is worse than an honest exit. The parent is still
reachable, just not as *back*: it sits in the header as a named link beside the
cascade id.

Rules for the build:

- Tapping the **active** tab returns that tab to its root; it never reloads.
- A **sheet is a level of history** — back closes it before leaving the screen.
- **Sign-out clears every stack.** Sessions last weeks, so this is rare and
  deliberate.
- Every screen is addressable, and every deep link lands **on the screen
  itself** — never on a parent with the child scrolled into view.
- The system back gesture and the header's `‹` do the same thing, always.

---

## State management

Everything in `../README.md` still applies. Touch adds:

| State | Type | Notes |
| --- | --- | --- |
| `navShape` | `'tabs' \| 'sheet'` | Derived from the destination count, not stored. Four or fewer → tabs. |
| `navSheetOpen` | boolean | Advanced only. |
| `openRowId` | id \| null | A2 disclosure. **One at a time** per list. |
| `searchOverlayOpen` | boolean | M05 and any index over twenty rows. |
| `filterSheetOpen` + `draftFilters` | boolean + filter object | Filters are **drafted** in the sheet and committed on *Show*, so the live list does not reflow under a moving thumb. |
| `selectionMode` + `selectedIds` | boolean + Set | M25 only. Entering the mode is an explicit named control. |
| `copiedFieldId` | id \| null | Cleared after 900ms. Replaces any toast queue. |
| `tabStacks` | record of route stacks, one per tab | Per-tab history. |
| `activeThread` | `{ cascadeId, originTab, originLabel } \| null` | The cross-tab thread. Cleared by tapping a tab. |

No new endpoints are required by anything in this document. Two existing gaps are
listed under *Open items*.

---

## Design tokens

Only the touch-specific values. Colour, the full type scale and the semantic
roles are in `../README.md`.

### Layout

| Token | Value |
| --- | --- |
| Gutter | 16px (22px on the sign-in screen) |
| Card radius | 18px · small card 16px |
| Sheet radius | `24px 24px 0 0` |
| Row min-height | 60px · copy row 52px · disclosed row pads `4px 16px 16px` |
| Section gap | 12px |
| Sheet backdrop | `rgba(8,9,6,.72)` — `.74`–`.80` where the page behind should recede further |
| Sheet shadow | `0 -30px 70px -30px rgba(0,0,0,.95)` |
| Grabber | 38 × 4px, radius 999, `rgba(255,255,255,.16)` |
| Safe areas | Status-bar inset respected; bottom bars and sheets pad to the home indicator |

### Typography

| Role | Size / weight |
| --- | --- |
| Page title | 26px Bricolage Grotesque 600, `-.02em` |
| Section / sheet title | 19–22px Bricolage 600 |
| Row primary | 15–15.5px 600 |
| Row secondary | 13px |
| Body | 14px / 1.55 |
| Label (uppercase eyebrow) | 11px, `.1em`, `rgba(243,245,239,.38)` |
| Monospace | 13px for ids, paths, permissions |
| Floor | **Never below 12.5px anywhere** |

### Targets

44px minimum · 50px destructive · 52px copy rows · 12px destructive-to-benign
separation · focus ring `#c9b6ff` 2px at 4px offset.

---

## Accessibility

- Everything in `../README.md`'s accessibility section still holds.
- Colour never carries meaning alone: every dot is paired with a word on the same
  or the next line. The amber dot in a list row repeats what the second line says.
- Disabled controls state their reason **as text, in place** — never as a tooltip
  or a title attribute, which touch cannot open.
- The whole row is the accessible target, so the row is the interactive element,
  not a nested chevron.
- Reduced motion is honoured per *Motion*, above; state changes still happen,
  instantly if necessary, because they carry meaning.
- Type scales with the platform's dynamic type. Rows grow; the 44px floor is a
  floor, not a fixed height.

---

## Open items

1. **The dormant-account listing endpoint (`§29`) is still unbuilt.** M25 assumes
   it returns, per row, the reason a row cannot be removed — not just a boolean.
2. **Card enrolment needs a reader-event stream** for M24a's waiting state.
   Polling works, but the two-minute timeout must come from the server, not the
   client, or two operators enrolling at once will disagree about what expired.
3. **TrueNAS first, Unifi Access later.** M18b's third member tab appears only
   once TrueNAS ships.
4. **Provider latency (M17b) is a measured round trip**, not an uptime
   percentage. Confirm the backend reports it per read.

---

## Files

| Path | What it is |
| --- | --- |
| `Syndra Mobile.dc.html` | **Start here.** The board: M00–M28 and M32, ~60 phone figures at 390px, one tablet figure at 744px, live motion specimens for the four movements. Needs `support.js` beside it. |
| `Source.dc.html` | The Access source capsule, implemented. Used by the board's figures. |
| `support.js` | Runtime for the `.dc.html` files. **Not part of the application.** |
| `../README.md` | The normative spec for behaviour, copy and colour. Read first. |
| `../BUILD-NOTES.md` | Overrides both READMEs where the codebase and the design disagree. |
| `../login/LOGIN.md` | Full spec for `/login`. M28 is its touch form and does not restate it. |
| `../login/login-reference.html` | Standalone working build of the login screen. |

### Suggested reading order

1. *Motion and interaction* — the four movements are the whole cohesion argument.
2. *Breakpoints*, then *Navigation* and *Header and freshness* — every screen
   inherits these three.
3. *Density* — it decides the shape of two thirds of the screens.
4. M28 and M32 — the first screen anyone sees, and the model behind every `‹`.
5. The *Screens* table, against `../README.md`'s matching `§` section.
