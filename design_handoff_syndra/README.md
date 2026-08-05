# Handoff: Syndra

## Overview

**Syndra** is an access-management tool for a single academic makerspace. It sits
on top of [Zitadel](https://zitadel.com) as the identity provider and answers one
question for the person running the shop: *who may enter, why, and until when.*

It is a personal, open-source project, not a commercial product. There is one
makerspace, a handful of operators, and one maintainer. Design decisions
throughout favour **being honest about what the software knows** over looking
capable.

The name comes from **Syn**, the Norse goddess who keeps the door of Frigg's hall,
bars it against those who should not enter, and is invoked as a defence at
trials. Those are the product's two halves: the access list, and the record of
why it looks the way it does. The tagline is *"Syn keeps the door. Syndra keeps
the list."*

This bundle covers the **whole application**: 30 screens across three audiences,
plus the login screen and the repo banner images.

---

## About the design files

The files in `design/` are **design references created in HTML**. They are
prototypes that show intended look and behaviour. They are **not production code
to lift wholesale**.

The task is to **recreate these designs in the target codebase's existing
environment** — React, Vue, Svelte, a server-rendered template, whatever the
project already uses — following its established patterns, component library and
conventions. If no frontend exists yet, choose the most appropriate framework for
the project and implement the designs there.

### How to read the design files

`design/Syndra IA.dc.html` is the master document. Open it in a browser (it needs
`support.js` beside it, which is included). It is a single wide canvas divided
into numbered sections, §01 through §19. Each section contains one or more
`<figure>` elements; **every `<figcaption>` names the screen id and its API
endpoint**, e.g.:

> E3 · `GET /api/v1/users/{id}/access` …

Those captions are the authoritative screen-to-endpoint mapping. Read them.

The captions also carry the *reasoning* behind each decision. When you have to
make a judgement call the README does not cover, the caption for that figure is
usually the answer.

`design/Sidebar.dc.html` and `design/Source.dc.html` are two real components,
implemented rather than mocked, and used repeatedly throughout the board. Their
logic classes are the specification for their behaviour.

---

## Fidelity

**Fidelity is not uniform across the board.** The header chips in
`Syndra IA.dc.html` declare the split, and it matters — building a mid-fidelity
mock pixel-perfectly wastes your time and bakes in numbers nobody chose.

| Surface | Screens | Fidelity | How to treat it |
| --- | --- | --- | --- |
| **Basic** | E1–E7, Today, People, the four list states (§04–§12) | **High** | Colours, typography, spacing, radii, easings and durations are final and exact. Recreate pixel-perfectly. |
| **Unexplained access** | S6 + revoke dialog + reconciliation (§14, §15) | **High** | Same. The highest-stakes screen in the product, designed to the pixel. |
| **Member** | My access, member Requests (§10) | **High** | Same. |
| **Advanced** | S1–S5, S7–S11 (§15–§18) — Expiring access, Bundles, Confirmation policy, Automatic rules, Pending changes, Change history, Audit, Identity provider, Intents, Event activity | **Mid** | Layout and information architecture are settled and should be followed. Treat spacing and type sizes as **directional**: apply the token set in [Design Tokens](#design-tokens) rather than measuring the mock. |
| **Light theme** | §13 | **Directional** | Proves the palette inverts. Not a spec for every screen. |
| **Motion** | §19 | **High** | Durations and easings are exact. |

The Advanced surface is 10 of the 30 screens. Its *structure* — what is on each
screen, in what order, linked to what — is decided. Its *measurements* are not.
Where the mock and the token table disagree on Advanced screens, **the token table
wins**.

Two further exceptions, marked as such in the board:

- **S10 (Intents)** is parked — designed but not to be built yet.
- **Bulk revoke** is designed in full but shipped disabled, so the copy can be
  reviewed. The button carries its own reason for being disabled.

---

## The three audiences

The same application serves three people, and the navigation is the mechanism.

| View | Who | Destinations |
| --- | --- | --- |
| **Basic** | An operator assigning access | Today, People, Access (Projects / Roles / Apps), Requests |
| **Advanced** | The maintainer changing the machinery | All of Basic, then Bundles, Automation, Review, System appended |
| **Member** | Someone who just wants in | My access, Requests |

**Advanced appends to Basic.** Same items, same order, nothing renamed, nothing
inserted above. Switching views never navigates and never re-sorts — it reveals
in place. A Basic user who switches to Advanced stays on the row they were
reading, now expanded with lineage and ids.

The switch is a **two-state pill**, not a dropdown, sitting top-right beside the
account. Both labels stay legible so the current state is unmistakable. Members
do not see it.

See §01 and §02.

---

## Screen inventory

30 screens. `E` = everyday (Basic), `S` = system (Advanced). Section numbers refer
to `Syndra IA.dc.html`.

### Basic surface

**All high fidelity — exact values.**

| § | Screen | Route / endpoint | Purpose |
| --- | --- | --- | --- |
| 04 | **Today** (E-Today) | `GET /api/v1/governance/summary` | Landing. Every block is something you can finish — no counts you cannot act on, no charts. |
| 05 | **People index** | `/users` | Search. Highest-traffic screen in the product. |
| 05 | **Person access** (E3) | `GET /api/v1/users/{id}/access` | One person's access, grouped by project, granted above automatic, every row carrying its source. |
| 06 | **Role members** (E2) | `/projects/{id}/roles/{key}` | "Who can currently use the laser cutter?" Every row carries its source. |
| 07 | **Projects index** (E1) | `/projects` | A project is a boundary that owns roles. Apps-served is a column, not a value. |
| 07 | **Project detail** (E1) | project → roles | Role descriptions shown in full, never truncated to a tooltip. |
| 07 | **Roles index** (E2 index) | `/roles` | Project column is first and never collapses — the same role key means different things in different projects. |
| 07 | **Apps index** | `/applications` | An app is a thing that receives a token. Not the same as a project. |
| 08 | **Assign bundle** (E4) | `POST /api/v1/users/{id}/bundles` | The one place a bundle is assigned or unassigned. Preview is the point, not a footnote. |
| 08 | **Set expiry** (E5) | — | Presets before the picker, because "end of term" is what people mean. Operator-only. |
| 09 | **Token simulator** (E6) | `GET /api/v1/applications/{id}/simulate` | "My app isn't seeing the roles it expects." The most technical thing in Basic; it earns its place. |
| 10 | **Requests, operator** (E7) | `GET /api/v1/requests` | One request expanded at a time, with a decision panel so nobody has to leave. |
| 10 | **Requests, member** (E7) | `POST /api/v1/requests` | Asks in verbs, not role keys. |
| 10 | **My access** (member) | same route as E3, scoped to self | Access source becomes a sentence rather than a chip. |
| 11 | **Four list states** | every list view | Loading, empty, error, degraded. No exceptions. |
| 12 | **Access map** (S5) | `GET /api/v1/topology` | Pick one node; the map draws its neighbourhood. Never everything at once. |

### Advanced surface

**Mid-fidelity except the three drift screens in §14–§15** (Unexplained access,
its revoke dialog, and Reconciliation), which are high-fidelity. For the mid ones,
follow the structure and take measurements from the token table.

| § | Screen | Route / endpoint | Fidelity | Purpose |
| --- | --- | --- | --- | --- |
| 14 | **Unexplained access** (S6) | `GET /api/v1/governance/drift` | High | Highest stakes in the product. Access that exists upstream which Syndra cannot explain. |
| 14 | **Revoke dialog** | `DELETE …` | High | The only place a solid red fill appears. |
| 15 | **Reconciliation** | (relocated from a retired route) | High | Drift in the other direction. Agreeing rows stay visible at reduced contrast. |
| 15 | **Expiring access** (S7) | `/review/expiring-access` | Mid | Its own destination, not an audit tab. Time-boxed work, one action. |
| 16 | **Bundles** (S1) | `GET/POST /api/v1/bundles` | Mid | Editing what a bundle *contains* changes access for everyone holding it. |
| 16 | **Confirmation policy** (S2b) | `GET/PUT /api/v1/config/confirmation` | Mid | Which rules queue for review and which fire unattended. |
| 17 | **Automatic rules** (S2) | `/rules/mapping` | Mid | A rule fires → a write queues → the write is recorded. Read left to right. |
| 17 | **Pending changes** (S3) | `GET /api/v1/propagations` | Mid | The queue rules fill. |
| 17 | **Change history** (S4) | `GET /api/v1/propagations/cascades` | Mid | The record of what they did. Threaded by cascade id. |
| 18 | **Audit** (S8) | `GET /api/v1/audit` | Mid | Every line names a human or a named machine. |
| 18 | **Identity provider** (S9) | `/zitadel/{health,projects,users,grants}` | Mid | Provider health and raw upstream state. |
| 18 | **Intents** (S10) | `GET /api/v1/intents` | Parked | **Parked.** Designed, not to be built yet. |
| 18 | **Event activity** (S11) | `GET /api/v1/webhook/events` + `/onboarding/triggers` | Mid | Merged into one time-ordered stream, each row drillable to its raw payload. |

### Not in this table

- **Login** — fully specified separately in `login/LOGIN.md`, with a working
  standalone reference in `login/login-reference.html`. It is the only
  unauthenticated route.
- **Settings** — a nav destination inside Automation; not separately drawn.

---

## The causal chain

Three screens tell one story and must link to each other, left to right:

```
Automatic rules (S2)  →  Pending changes (S3)  →  Change history (S4)
     a rule fires          a write queues          the write is recorded
```

Every write carries a **cascade id** (e.g. `c_8841`). Two writes caused by the
same rule firing share one cascade id. That id is the join key across S2, S3, S4,
S8 and S11, and it must be a link everywhere it appears. This is the single most
important piece of data plumbing in the product — it is what makes "why does this
person have this access?" answerable.

See §17 and §18.

---

## Signature component: Access source

Every row that grants access states **where the access came from**. Three kinds,
fixed vocabulary, **always in this order**:

1. **Direct** — someone granted it to this person
2. **Via bundle** — it came with a bundle they hold
3. **Automatic** — a rule produced it

The dot carries the meaning before the word is read. The order is fixed so the
eye learns one sequence and reads it everywhere.

Expanding a source row answers *who, when, and until when* — and hosts the only
removal that belongs to that specific source. A grant that came from a rule
offers no removal; it offers the two real ways forward (change the rule, or
adopt the grant).

Implemented in `design/Source.dc.html`. See §03.

---

## Interactions & behaviour

### Motion system

Six tokens cover the entire product. §19 contains live, hoverable specimens of
every one.

| Token | Duration | Easing | Used for |
| --- | --- | --- | --- |
| `tint` | 120ms | `cubic-bezier(.4,0,.6,1)` | Colour and opacity only — row hover, link hover, nav hover. **No transform.** |
| `press` | 180ms | `cubic-bezier(.22,.61,.36,1)` | Buttons, tabs, toggles, focus rings. Anything answering a click. |
| `settle` | 260ms | `cubic-bezier(.16,1,.3,1)` | Disclosures, drawers, dialogs — anything changing page height. |
| `arrive` | 420ms | `cubic-bezier(.16,1,.3,1)` | First paint of a screen or list. Staggered 40ms per row, **capped at six rows**. |
| `flash` | 900ms | `ease-out`, once | A value changed while you were on the page. Never on load. |
| `breathe` | 2.4s | `ease-in-out`, alternate, ∞ | Degraded and loading only. |

Three rules govern all of it:

1. **Direction carries meaning.** Things the system produced *rise*. Things you
   opened *scale from where you clicked*. Nothing slides sideways. Nothing moves
   more than 10px.
2. **Only one thing loops.** A repeating animation means *this is still
   happening* — degraded mode breathes, skeletons shimmer. Nothing healthy and
   no decoration ever loops.
3. **Never animate a number.** Counts, dates and ids are read, not watched. What
   animates is the *row around them*, once, when the value arrived while the
   operator was looking elsewhere.

Specific behaviours worth calling out:

- **Row hover is background only** — no lift, no shadow, no border change. A
  forty-row table where each row lifts is a table that flickers as you read it.
- **Button press is a 3% scale-down**, not a translate, so the button stays under
  the finger. Destructive buttons behave identically — muscle memory must not
  depend on what a button does.
- **Tab underlines grow from the left**, they do not travel between tabs. A
  travelling underline implies the tabs are a sequence; they are not.
- **Dialogs**: scrim fades (260ms), card rises 10px from 97% scale, 40ms behind
  the scrim. It never zooms from screen centre — the destination is where the
  dialog will live.
- **Disclosures animate height and opacity together**, so a payload never appears
  before it has room. Rows below are pushed, never overlapped.

### What deliberately does not move

Route transitions (the sidebar and header persist, so crossfading content implies
a bigger change than happened). Table sorting and filtering (animated reordering
makes it impossible to follow one row with the eye). Counts and timestamps.
Anything inside a dialog after it has opened. And the healthy state, which earns
its calm by being the only thing on screen holding perfectly still.

### Reduced motion

`prefers-reduced-motion: reduce` collapses every duration to `.001ms` and stops
every loop. **No state is conveyed by movement alone**, so nothing is lost — the
degraded row still turns amber, the row still tints, the dialog still appears.

### The four list states

Every list view implements all four. No exceptions. See §11.

| State | Treatment |
| --- | --- |
| **Loading** | Row-shaped skeletons at the *real row height*, so nothing jumps when data lands. Ids resolve asynchronously. |
| **Empty** | Names what is absent and offers the next move. Never a shrug illustration, never "you're all caught up!" |
| **Error** | Names the failed thing, confirms nothing changed, carries a request id the operator can paste into a message. |
| **Degraded** | The backend has fallen back to demo data. **Every number on the page is invented and the page says so.** The one moment a semantic colour fills a whole field — amber, because the data is wrong rather than dangerous. |

### Disabled controls

A disabled action **always states its reason in the row**, not only on hover.
Hover does not exist on touch, and a disabled button with no explanation is a
dead end. See §04.

---

## State management

### Global

```ts
type View = 'basic' | 'advanced' | 'member'
```

Persisted per user. Changing it re-renders in place; it never navigates, never
re-sorts, never scrolls.

The member view is not a preference — it is derived from the account's role. An
operator can switch between Basic and Advanced; a member sees only Member.

### Per list view

```ts
type ListState = 'loading' | 'ready' | 'empty' | 'error' | 'degraded'
```

`degraded` is a **server-declared** state, not an inference from an empty
response. The API tells the client it is serving demo data; the client must not
guess.

### Data fetching

Endpoints are named per screen in the inventory above and in each figure's
caption. Notes:

- **Cache age is user-visible.** "cache 4m old" appears in the Today header and
  the provider health row. Whatever caching layer you use must expose its age.
- **Cascade ids join across five screens.** Fetching a cascade must be possible
  from any of S2, S3, S4, S8, S11.
- **`GET /api/v1/applications/{id}/simulate`** is a read-only dry run. It must
  never write.

---

## Design tokens
<a id="design-tokens"></a>

### Colour — five semantic roles, no decoration

| Token | Dark (home) | Light | Meaning |
| --- | --- | --- | --- |
| **Accent / Violet 500** | `#7f5af0` | fill `#7f5af0`, text `#5b3fd6` | The primary action and the current selection. One per screen region. **Never means "good" or "safe".** |
| **Violet 400** | `#9b7bff` | — | Accent text on dark, where 500 is too low-contrast. Links. |
| **Violet 300** | `#c9b6ff` | — | Active nav label, focus outline. |
| **Healthy / Lime** | `#a3e635` | `#4d7c0f` | *Nothing needed here.* Provider answered, queue empty, cache fresh. **Never a button, never a fill behind text.** A dot, a word, or a hairline. |
| **Deadline / Amber** | `#f5a524` | — | Expiry, rotation due, a broken assumption. The degraded field. |
| **Destructive / Red** | `#ff5c4d` | — | Destructive actions only. **Solid fill appears exactly once in the product**: the confirming button inside a revoke dialog. |

### Surfaces

| Token | Value | Use |
| --- | --- | --- |
| Page | `#080906` | App background |
| Login page | `#0a0b08` | Slightly lifted; login only |
| Rail | `#101210` | Sidebar |
| Card | `#141612` | Panels, cards, table containers |
| Card (nested) | `#0b0c0a` | Specimen frames, code blocks |
| Hairline | `rgba(255,255,255,.07)` | Borders |
| Divider | `rgba(255,255,255,.05)` | Row separators |
| Row hover | `rgba(255,255,255,.035)` | Table and nav hover |

### Text

| Token | Value | Use |
| --- | --- | --- |
| Primary | `#f3f5ef` | Body, headings |
| On accent | `#f7f4ff` | Label on a violet fill |
| 82% | `rgba(243,245,239,.82)` | Table cell content |
| 62% | `rgba(243,245,239,.62)` | Section intros |
| 50% | `rgba(243,245,239,.5)` | Captions |
| 42% | `rgba(243,245,239,.42)` | Eyebrows, labels, ids |
| 32% | `rgba(243,245,239,.32)` | Footnotes |

### Typography

Two families. Google Fonts, or self-host if the codebase already self-hosts.

| Role | Family | Size / weight | Tracking |
| --- | --- | --- | --- |
| Page title | Bricolage Grotesque 600 | 48 / 1.02 | −.02em |
| Section title | Bricolage Grotesque 600 | 34 / 1.05 | −.02em |
| Card title | Bricolage Grotesque 600 | 19–20 | 0 |
| Wordmark | Bricolage Grotesque 600 | 18 (rail), 46–62 (login/banner) | −.01 to −.035em |
| Eyebrow | Figtree 600 | 12.5, uppercase | .12em |
| Body | Figtree 400 | 17 / 1.55 | 0 |
| Table cell | Figtree 400 | 14.5 | 0 |
| Caption | Figtree 400 | 13.5 / 1.5 | 0 |
| Badge | Figtree 600 | 11.5 | 0 |
| Ids, endpoints, timestamps | JetBrains Mono 400 | 12.5–13 | 0 |

**Ids, endpoints, counts in monospace; everything else in Figtree.** Monospace is
a signal that a value is a machine identifier the operator may need to copy.

### Geometry

| Token | Value |
| --- | --- |
| Card radius | 20–22px |
| Inner card radius | 16px |
| Row / nav radius | 12–13px |
| Pill / badge radius | 999px |
| Button radius | 999px (pills), 14px (login) |
| Canvas width | 1600px fixed (see note below) |
| Sidebar width | 252px |
| Section gap | 120px |
| Figure gap | 38px |

### Shadows

```
card:         0 20px 44px -28px rgba(0,0,0,.9)
dialog:       0 30px 70px -30px rgba(0,0,0,.95)
button rest:  0 10px 26px -14px rgba(127,90,240,.85)
button hover: 0 16px 34px -14px rgba(127,90,240,.95)
button press: 0 6px 16px -12px rgba(127,90,240,.7)
```

---

## The mark

A **contained orb**: a faint base ring under a brighter arc, with a lit dot at the
centre. It is a miniature of the login's arch-and-orb.

- Base ring `rgba(155,123,255,.22)`, 1.5px
- Lit arc `rgba(206,188,255,.92)` at 50–62% opacity, falling off symmetrically
  from the apex
- Dot `#9b7bff` at 35% of the box, with `box-shadow: 0 0 14px 2px rgba(155,123,255,.6)`
- Rail size 22px; favicon 16px

On light grounds the values darken: base `rgba(91,63,214,.18)`, arc
`rgba(76,48,196,.85)`, dot `#5b3fd6`. The near-white arc is invisible on paper.

Two implementation notes, both learned the hard way:

- The ring's radial fade mask must keep its opaque stop at **88% or beyond**. At
  62% on a 22px box the mask erases the 1.5px border entirely, because the border
  occupies 93.5–100% of the mask radius.
- For **rasterised output** (favicons, the banner), draw rings and arches as
  *filled* SVG bands, not gradient strokes. Gradient strokes flatten to the first
  stop's colour when rasterised and the fade disappears. CSS `mask-image` does not
  rasterise at all.

---

## Voice

Syndra never says "I". It is a tool in a workshop, not a service — it does not
greet you and has no opinions about your day. Warmth comes from **admitting what
the software does not know** and **naming the consequence** of an action.

| Situation | Not this | This |
| --- | --- | --- |
| Role with no members | "Nobody holds this role yet." | "Nobody holds this role yet. Nothing is checking it, and nothing will until someone does." |
| Empty triage queue | "No unexplained access." | "Everything upstream has an explanation. Checked 4 minutes ago — this can change without anyone doing anything." |
| Revoke confirmation | "This action cannot be undone." | "She loses badge entry to the laser bay at the next cache compile. If an integration created this grant, it will be back tomorrow." |
| Degraded mode | "Running on demo data." | "Syndra can't reach the provider, so every number on this page is invented. Don't act on it." |
| Unbuilt feature | "Not connected yet." | "Not connected yet. This is a plan, not a feature — nothing here is wired to a door." |
| Error | "Something went wrong." | "Couldn't load role members. Nothing was changed. Try again, or send someone this: `req_9c14e`." |

Consequences are stated in **physical terms**. "She loses badge entry to the laser
bay", not "grant deleted".

Naming rules:

- Full name: **Makerspace Syndra**
- In-app, everywhere: **Syndra**
- Never: `SYNDRA`, `Syndra™`, "the Syndra platform", or any first-person voice

---

## Design decisions that are deliberate

Do not "improve" these without asking. Each was argued for.

1. **No bulk revoke.** Every revoke removes real access from a real machine, so
   each one is read on its own. The dialog is designed and reachable so its copy
   can be reviewed, but the button ships disabled and says why.
2. **Unexplained access shows both outcomes on the row.** Revoke *and* adopt,
   ranked by risk, so the operator is not forced to guess which is safer.
3. **Reconciliation names drift directions differently.** Access upstream that
   Syndra did not make is not the same problem as access Syndra made that is
   missing upstream. They get different words.
4. **Agreeing rows stay visible at reduced contrast** rather than being filtered
   out. "Nothing wrong here" is information.
5. **The access map draws one neighbourhood, never the whole graph.** The old
   graph was unreadable because it drew everything at once.
6. **Expiring access is its own destination, not an audit tab.** Audit is a record
   you consult; this is time-boxed work you do.
7. **Only the error row is tinted in the event log.** A log where half the lines
   are coloured is a log nobody reads.
8. **Role descriptions are shown, never truncated to a tooltip.** "Can cut
   unsupervised" versus "may enter and watch" is the whole difference.
9. **The project column in the roles index never collapses.** The same role key in
   two projects is two different things.
10. **Presets before the date picker.** "End of term" is what people actually
    mean.

---

## Known constraint: canvas width

The design board is authored at a **fixed 1600px** width because flex children
resolved at `max-content` when it was fluid, causing wrapping bugs in the
specimen tables. This is a property of the *design document*, not of the
application. The real app should be responsive; the board simply is not, and
should not be read as prescribing a fixed-width app.

The login screen has one real minimum: `min-height: max(100vh, 800px)`. Below
~610px of viewport height the sign-in button's overhang collides with the Syn
line. See `login/LOGIN.md`.

---

## Accessibility

- Contrast: `#f7f4ff` on `#7f5af0` is **4.18:1** — passes AA as large text
  (≥18.5px, or ≥14px bold) but **fails for small text**. The 11.5px count badges
  on violet are below AA. Darken the fill to `#6f4ae0` (5.2:1) for any small text
  on accent.
- Every disabled control states its reason as text, not as a tooltip.
- Degraded and error states are conveyed by text and colour, never colour alone.
- Reduced motion is honoured throughout; no state depends on movement.
- Focus rings use `press` timing and `#c9b6ff` at 2px with 4px offset.
- Ids and endpoints are selectable text, never images — operators copy them.

---

## Assets

**No images, no icon fonts, no SVG illustrations anywhere in the application.**
Everything is CSS and inline SVG: borders, radii, gradients, masks, and one
`feTurbulence` noise tile on the login screen.

The only binary assets are the two repo banner PNGs in `banner/`, which are
exports of `design/Syndra Banner.dc.html`.

---

## Files

| Path | What it is |
| --- | --- |
| `design/Syndra IA.dc.html` | **Start here.** The master board: §01–§19, all 30 screens, live motion specimens. Needs `support.js` beside it. |
| `design/Sidebar.dc.html` | The sidebar component, implemented. Its logic class is the spec for all three views' nav trees. |
| `design/Source.dc.html` | The Access source component, implemented. |
| `design/support.js` | Runtime for the `.dc.html` files. Not part of the application. |
| `design/Syndra Banner.dc.html` | Source for the two repo images. |
| `login/LOGIN.md` | Full spec for `/login` — layout, three states, animation timings, four rasterisation gotchas. |
| `login/login-reference.html` | Standalone, dependency-free working build of the login screen. Opens directly in a browser. Delete the demo controls in the top-right. |
| `banner/syndra-banner-1280x400.png` | README header image (2× export). |
| `banner/syndra-social-1280x640.png` | GitHub social preview (2× export). |
| `banner/BANNER.md` | Markup snippet and re-export instructions. |

### Suggested reading order

1. This README, through "The three audiences".
2. `design/Syndra IA.dc.html` §01–§03 — navigation, the view switch, Access source.
3. §04 Today, then §05 People. Those two are 80% of the product's traffic.
4. §11 the four list states, and §19 motion. Both apply to everything else.
5. `login/LOGIN.md`.
6. The rest of the sections as you build them.
