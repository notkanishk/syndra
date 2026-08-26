You are extending an existing, finished design system for a product called
Syndra. Do not invent a new one. Do not restyle what I describe. Your job is to
draw screens a developer will build from tokens that already exist.

Read this whole prompt, acknowledge it in a sentence, and wait. The screen brief
comes next.

## The product

Syndra is the access-management tool for one academic makerspace. It sits in
front of Zitadel (the identity provider) and in front of **add-ons** — separate
services that hold accounts of their own, such as a TrueNAS file server. Three
audiences share one application: a **member** who wants in, a **basic operator**
who assigns access, and an **advanced operator** who maintains the machinery. The
same URLs serve all three.

**The central argument: consequences are visible before you act, and nothing
hides behind a gesture.** Every rule below follows from it.

**The one fact that shapes the add-on screens:** nothing Syndra does reaches a
target directly. Changes are queued, and an operator resumes a drain. So *queued*
is an ordinary state and not a transient one, and rendering it as done is the
failure these screens exist to avoid.

## Canvas

**Desktop, 1440 × N.** A 252px navigation rail on the left with its own
background and one hairline on its right edge; the content column beside it, max
width ~1570px, 24px gutters. Draw the rail only where it clarifies the screen —
otherwise draw the content column alone and say so in the caption.

One 390 × 844 figure per screen where the brief asks for it, and not otherwise.

Lay figures side by side with a caption under each saying what it is **and why it
is drawn that way**. The caption carries the reasoning; it is the part a
developer reads when the drawing does not cover their case.

## Colour — semantic roles, no decoration

Surfaces: ground `#080906` · canvas `#0b0c0a` · rail `#101210` · card `#141612`
· raised `#1b1e19` · hairline `rgba(255,255,255,.06)` · strong hairline
`rgba(255,255,255,.14)` · tints `rgba(243,245,239,.04 / .07 / .09)`.

Text: primary `#f3f5ef` · muted `rgba(243,245,239,.6)` · faint
`rgba(243,245,239,.42)` · label `rgba(243,245,239,.34)`.

- **Violet** — fill `#7f5af0`, dense fill `#6d47e0`, text `#9b7bff`, soft
  `rgba(155,123,255,.15)`, hairline `rgba(155,123,255,.28)`. It marks the one
  thing a region is for. One per region.
- **Lime** `#a3e635` means healthy. **Never a fill and never a button** — there
  is no such thing as a healthy button.
- **Amber** — `#f5a524`, text `#f7b955`, soft `rgba(245,165,36,.12)`. A deadline
  or a broken assumption. Not "warning" in general.
- **Red** — `#ff5c4d`, text `#ff8d82`, soft `rgba(255,92,77,.1)`, hairline
  `rgba(255,92,77,.4)`. Destructive only. Solid red fill appears **only** on the
  confirming button inside a dialog, never in a table row.

**Colour never carries meaning alone.** Every status word is preceded by its tone
dot on the same line.

Both themes are authored in full from these tokens, so **do not design a light
variant** — draw dark, and use the roles consistently enough that light falls out
of the swap. Say what changes only where it is not mechanical.

## Type

Bricolage Grotesque for titles, Figtree for body, JetBrains Mono for identifiers.

| Role | Size |
| --- | --- |
| Page title | 40px / 500 / `-.02em` |
| Section title | 32px / 500 |
| Card title | 22px / 600 |
| Dialog title | 26px / 600 |
| Empty-state title | 22px / 600 |
| Body | 14–15px / 1.55 |
| Row primary | 15.5px / 600 · row secondary 13–13.5px |
| Uppercase label | 12.5px / 600 / `.1em` |
| Mono — ids, paths, usernames, role keys | 12.5px |

**Nothing below 12.5px, anywhere.**

An identifier is typeset by *what it is*, not by where it sits: an account name
inside a sentence is 12.5px mono, and an account name that is a row's title is
13.5px mono. Those are the only two.

## Geometry

Card radius 20px · panel 18px · inner block 14px · pill 999px. Row min-height
60px. Card header pads `16px 20px`; a row pads `14px 20px`. Section gap 18px.
Dialog shadow `0 22px 50px rgba(0,0,0,.55)`.

Every control clears **44px until 1080px wide** and returns to its dense size
above it — a 720–1080px window is a tablet, which is still a thumb.

## Controls — one surface each

- **Button.** `accent` (violet fill) · `outline` (hairline + ink) · `ghost` (no
  border) · `danger` (red outline) · `dangerConfirm` (red fill). Two sizes.
- **Badge.** A pill: neutral, accent, amber, red, and two soft forms that *name*
  something dangerous rather than sounding an alarm.
- **Tabs** — changing what a page shows. **FilterPills** — narrowing a list. Both
  are the button's box without its border, so a row containing a filter, a tab
  and a button lines up.
- **Card** with a header, and rows separated by a hairline.

**The variant rule.** The one action a row or a finding offers is `outline` — a
borderless control in a table reads as text until it is hovered, and hover does
not exist on touch or in a screenshot. Destructive is `danger`. `ghost` is only
the quieter half of a pair: a Cancel, a Dismiss. A dialog's confirm is `accent`
or `dangerConfirm`.

## Motion — three movements

| Name | Duration | What it is |
| --- | --- | --- |
| `tint` | 120ms | Colour and opacity only. Never transform — forty rows that each lift on hover is a table that flickers as you read down it. |
| `press` | 180ms | Anything answering a click. A 3% scale-down, never a translate, so the control stays under the pointer. Destructive buttons behave identically; muscle memory must not depend on what a button does. |
| `settle` | 260ms | A row disclosing, a list reflowing. Rows below are **pushed, never overlapped** — an overlay would cover the row somebody was comparing this one against. |

Pending work shows a **breathing dot**, never a spinner. Under reduced motion
each movement gets a still form, not a shorter one.

## Forbidden

Tooltips and `title` attributes · toasts · skeleton shimmer · illustrations ·
charts, gauges and sparklines · horizontally scrolling tables · swipe actions ·
pull-to-refresh · anything that hides a decision behind a gesture with no label.

## Rules that outrank aesthetics

1. **A disabled control states its reason as body text, in place.** Never a
   tooltip, never a bare grey-out. Hover does not exist on touch and does not
   survive a screenshot sent to a colleague.
2. **Structure never moves in response to data.** A section with nothing in it
   keeps its seat and shows a hollow zero. A panel that appears when its count
   goes non-zero is prohibited — it pushes everything else down under the
   operator's cursor, and it teaches people not to trust the page.
3. **Queued is not succeeded.** Never render queued work as done. The word
   *done* and a tick mark do not appear on these screens at all.
4. **Every list has four states.** Loading: placeholder rows at the real row
   height, static. Empty: names what is absent and who causes it. Error: names
   what failed and ends "Nothing was changed." Degraded: an amber banner, rows
   dimmed, every action inert.
5. **Freshness is one component.** A dot, a sentence carrying an age — "The
   account list was read just now" — an optional clause when the read hit its
   cap, and a `Read again` control that appears **only when the read is stale**.
   Never a second way of saying how old something is, and never a word like
   "recently" where an age belongs.
6. **Three confirmation rungs**, set by what cannot be undone rather than by how
   important something feels. Rung 1: a plain button whose label names its
   numbers. Rung 2: a ticked sentence with the quantity inside it — "I understand
   this changes 34 people." Rung 3: type the name to arm a red button. The
   backend enforces its own confirmation regardless; these rungs are what a
   person meets, not what protects the data.
7. **Say which machine to look at.** Every failure reading names where the fault
   is. "Not answering" sends an operator to the NAS; "backed off" is Syndra
   refusing its own calls and sends them to Syndra. Getting that wrong costs an
   afternoon.

## Voice

Plain sentences in the product's own words. Say what happened and what it means,
not what the API returned. Never invent a number; if a count is unknown, say so.
A hold is a "hold" to the operator and "withheld" to the member — one record, two
words, chosen for who is reading.

Acknowledge you have this, then wait for the brief.
