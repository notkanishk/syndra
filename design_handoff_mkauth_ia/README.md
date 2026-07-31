# Handoff: MkAuth — Basic / Advanced information architecture

## Overview

MkAuth is access management for an academic makerspace. The existing UI is functionally complete but organisationally chaotic — thirteen top-level admin links across six sidebar sections, with overlapping names.

This redesign adds **no features**. It reorganises existing capability into two views of the same product:

- **Basic** — the everyday surface. Student staff and lab managers, in a hurry, in a noisy room. Answers: *who can get into this thing / what can this person get into / why does this person have this / give this person access.*
- **Advanced** — the system surface. The one or two people who own the machine. Answers: *why did the system do that on its own,* and everything underneath it.

The internal preference key is `ui_view` with values `basic | advanced`. **It must not be called `mode`** — `GET /api/v1/system/mode` already exists and reports demo-vs-live backend state.

Members are a third audience with two destinations and no view switch at all.

## About the design files

The files in `design/` are **design references created in HTML**. They are prototypes showing intended look, structure and copy — *not* production code to lift.

Your job is to **recreate these designs in the target codebase** (Next.js App Router + Tailwind + Bun, per the brief) using its established patterns and component layer: the existing `Modal`, `Drawer`, shared focus-trap hook, toast system, and the async name-resolver context that turns ids into display names. Reuse them; do not fork parallel versions.

The HTML uses a small custom runtime (`support.js`) purely so the design could be authored as one streaming document. Ignore it. Open `design/MkAuth IA.dc.html` in a browser to view the board; read the markup for exact values.

`DESIGN-BRIEF.md` is the original product brief. Where this README and the brief disagree on naming, **this README wins** (the brief used "Everyday / System"; the product owner has settled on Basic / Advanced).

## Fidelity

**High fidelity** for the Basic surface, the member surface and S6: final colours, typography, spacing, radii, copy and interaction states. Recreate accurately with the codebase's libraries.

**Mid fidelity** for S1–S4, S2b and S7–S11: layout, hierarchy, controls and copy are specified and correct; you may exercise judgement on dense-table minutiae, provided the semantic colour rules and the stated prohibitions hold.

Two deliberate exceptions, both marked in the design:

1. **Remove direct access** is drawn in full but rendered **disabled** everywhere, because `DELETE /api/v1/users/{id}/grants/{grantId}` does not exist yet. Ship it disabled with its reason visible. Do not substitute the Zitadel-side delete — it removes a different object and the next cache compile puts the access back.
2. **`/login`** is untouched. Nothing in this redesign changes the unauthenticated surface.

## Screen inventory

Every route in the matrix is now drawn. Board section numbers refer to `design/MkAuth IA.dc.html`.

| § | Screen | Route | Fidelity |
|---|---|---|---|
| 01 | Sidebar — Basic / Advanced / member | — | high |
| 02 | View switch + scoped-jump pattern | — | high |
| 03 | Access source component | — | high |
| 04 | Today (Basic, Advanced deltas, empty) | `/` | high |
| 05 | People index | `/users` | high |
| 05 | Person detail (E3) | `/users/{id}` | high |
| 06 | Role → members (E2) + 3 removal dialogs | `/projects/{id}/roles/{key}` | high |
| 07 | Projects index (E1) | `/projects` | high |
| 07 | Project → roles | `/projects/{id}` | high |
| 07 | Roles cross-project index | `/roles` | high |
| 07 | Apps index | `/applications` | high |
| 08 | Manage bundles (E4) + Grant direct access (E5) | modal / drawer | high |
| 09 | App token format + preview (E6) | `/applications/{id}` | high |
| 10 | Requests — operator (E7) | `/requests` | high |
| 10 | Requests — member (E7) | `/requests` | high |
| 10 | My access — member | `/` | high |
| 11 | Empty / loading / error / degraded | every list | high |
| 12 | Access map (S5) | `/graph` | high |
| 13 | Light theme + semantic colour system | — | high |
| 14 | Unexplained access — triage + reconciliation (S6) | `/governance/drift` | high |
| 15 | Expiring access (S7) | `/review/expiring-access` | mid |
| 16 | Bundles (S1) | `/bundles` | mid |
| 16 | Automation settings (S2b) | `/automation/settings` | mid |
| 17 | Automatic rules (S2) | `/policies` | mid |
| 17 | Pending changes (S3) | `/governance/pending` | mid |
| 17 | Change history (S4) | `/operations/cascades` | mid |
| 18 | Audit (S8) | `/audit` | mid |
| 18 | Identity provider (S9) | `/zitadel` | mid |
| 18 | Hardware sync — parked (S10) | *(hardware)* | mid |
| 18 | Event activity (S11) | `/operations` | mid |

---

## Design tokens

### Colour — dark theme (primary)

| Token | Value | Use |
|---|---|---|
| `ground` | `#080906` | Page canvas, outermost |
| `surface-0` | `#0b0c0a` | App shell background inside the frame |
| `surface-1` | `#141612` | Cards, list containers, panels |
| `surface-2` | `#1b1e19` | Dialogs, popovers (elevated) |
| `rail` | `#101210` | Sidebar background |
| `border` | `rgba(255,255,255,.06)` | Row dividers, shell edges |
| `border-strong` | `rgba(255,255,255,.14)` | Secondary button outlines |
| `text` | `#f3f5ef` | Primary |
| `text-muted` | `rgba(243,245,239,.6)` | Body/secondary |
| `text-faint` | `rgba(243,245,239,.42)` | Metadata, timestamps |
| `text-label` | `rgba(243,245,239,.34)` | Uppercase micro-labels |
| `accent` | `#d8f24e` | Primary action, current selection |
| `accent-ink` | `#12140d` | Text on accent fill |
| `accent-soft` | `rgba(216,242,78,.15)` | Accent tint fill |
| `warn` | `#f5a524` | Attention fills and badges |
| `warn-text` | `#f7b955` | Attention text on dark |
| `warn-ink` | `#231703` | Text on warn fill |
| `warn-soft` | `rgba(245,165,36,.08–.12)` | Attention tint |
| `danger` | `#ff5c4d` | Destructive fills and badges |
| `danger-text` | `#ff8d82` | Destructive text/outline on dark |
| `danger-ink` | `#2a0c08` | Text on danger fill |
| `danger-soft` | `rgba(255,92,77,.07–.12)` | Destructive tint |

### Colour — light theme

| Token | Value |
|---|---|
| `ground` | `#f4f1fb` (lilac) |
| `surface` | `#ffffff` |
| `border` | `rgba(37,28,66,.08)` |
| `text` | `#1d1830` |
| `text-muted` | `rgba(29,24,48,.6)` |
| `accent` | `#7a5af8` (fill), `#5b3fd6` (text/links) |
| `accent-soft` | `rgba(122,90,248,.12)` |
| `warn` | `#a86a00` |
| `danger` | `#d5382a` |
| card shadow | `0 2px 10px rgba(60,40,120,.05)` |

Dark is the default; light is a full theme, not a tint. Geometry, spacing and component structure are identical between them — only colour changes.

### Semantic colour rules (non-negotiable)

Four roles. Colour is never decorative.

- **Accent** — the primary action and the current selection. One per screen region. Never means "good" or "safe".
- **Attention (amber)** — a deadline or a broken assumption: expiring access, queued writes, unreachable identity provider, degraded data, an unmet prerequisite, a signing key about to rotate. Nothing is lost yet; something will be.
- **Destructive (red)** — takes access away, or reports that something already failed. **Solid red fill appears only on the confirming button inside a dialog.** In a table row, destructive actions are red *outline* only — a solid red row button is one stray click from an outage on the laser cutter.
- **Neutral** — everything else, which is most of the interface.

Further rules the design holds to:

- **Access source chips encode a kind, not a severity.** `Direct` uses the accent (it is the one source an operator can act on); `Via bundle` and `Automatic` stay neutral. Severity never rides on a source chip, and a source never borrows a severity colour.
- **In a log, colour marks the exception only.** Event activity tints one row (the error); Audit colours one word (`Revoked`). A log where half the lines are coloured is a log nobody reads.
- **Agreeing/quiet rows stay visible at reduced contrast** rather than being filtered out — "nothing wrong with the other four projects" is itself an answer.

### Typography

| Role | Family | Weight | Size / line-height |
|---|---|---|---|
| Board display | Bricolage Grotesque | 600 | 82px / .96, `letter-spacing:-.03em` |
| Section heading | Bricolage Grotesque | 600 | 48px / 1.02, `-.02em` |
| Page title (in-app) | Bricolage Grotesque | 500 | 30–42px / 1.05, `-.02em` |
| Card / group title | Bricolage Grotesque | 600 | 20–24px |
| Dialog title | Bricolage Grotesque | 600 | 26px, `-.01em` |
| Body | Figtree | 400 | 14–15px / 1.55 |
| Row primary | Figtree | 600 | 15px |
| Row secondary | Figtree | 400 | 13.5–14px |
| Button label | Figtree | 600 | 13.5–14.5px |
| Micro-label | Figtree | 600 | 11.5px, `letter-spacing:.10em`, uppercase |
| Table header | Figtree | 600 | 11.5px, `letter-spacing:.09em`, uppercase |
| Code / ids / claims / timestamps | JetBrains Mono | 400–500 | 12.5–13px |

Google Fonts: `Bricolage+Grotesque:opsz,wght@12..96,400;500;600;700`, `Figtree:wght@400;500;600;700`, `JetBrains+Mono:wght@400;500`.

Never render a role key, grant id, cascade id, claim name, token payload or log timestamp in the body face — those are always mono.

### Geometry

| Token | Value |
|---|---|
| App shell radius | 26px |
| Card / panel radius | 20px |
| Inner block radius | 12–16px |
| Nav row radius | 12px |
| Pill (buttons, chips, badges) | 999px |
| Checkbox | 5px radius, 18px |
| Radio | 999px, 18px (5px accent ring when selected) |
| Avatar | 999px — 30/32px row, 34/38px list, 62px header |
| Shell top bar height | 66px |
| Sidebar width | 252px |
| Content padding | 26px horizontal, 30px top, 36px bottom |
| Card padding | 14–20px header, 12–14px rows |
| Gap between cards | 18–22px |

Spacing scale in practice: 4 / 6 / 8 / 10 / 12 / 14 / 16 / 18 / 20 / 22 / 26 / 30 / 36px.

Shadows: dialogs `0 22px 50px rgba(0,0,0,.55)`; popovers `0 18px 40px rgba(0,0,0,.5)`; light-theme cards `0 2px 10px rgba(60,40,120,.05)`.

Icons: Lucide, stroke 1.5. The design deliberately uses geometric primitives (dot, ring, dashed ring) rather than icons for the Access source component.

### Interaction states

Every interactive element needs a themed hover and a pressed state — never browser defaults.

- Buttons: hover lightens the fill / tint by one step; pressed darkens it.
- Nav rows: hover `rgba(243,245,239,.05)`; active is the accent tint pill.
- Focus: `outline: 2px solid var(--accent); outline-offset: 2px` on `:focus-visible`. Never leave the default blue ring.
- Disabled: reduced-alpha version of the semantic colour it would otherwise carry (a blocked destructive button is `rgba(255,92,77,.14)` fill with `rgba(255,141,130,.4)` text) — **plus a visible reason**, not only a `title`.
- Transitions: 120–160ms `ease-out` on hover/tint; 200ms on dialog and popover entry. Nothing longer — this is a tool used in a hurry.

---

## Navigation — the stable contract

**Structure never moves. Only badge values change.** The current sidebar injects a Drift section at the top when the count is non-zero, pushing every other item down under the cursor. That is prohibited. A section with nothing in it renders in place with a hollow `0` badge; it never disappears and nothing appears above it.

Switching Basic → Advanced **appends** sections. It never reorders or renames what was already there.

```
BASIC                        ADVANCED (everything in Basic, plus)
─────────────                ────────────────────────────────────
Today                        Today
People                       People
Access                       Access
  · Projects                   · Projects
  · Roles                      · Roles
  · Apps                       · Apps
Requests            [3]      Requests                       [3]
                             Bundles
                             AUTOMATION
                               · Automatic rules
                               · Pending changes             [2]
                               · Change history
                               · Access map
                               · Settings
                             REVIEW
                               · Unexplained access         [12]  ← danger
                               · Expiring access             [1]  ← attention
                               · Audit
                             SYSTEM
                               · Identity provider
                               · Hardware sync
                               · Event activity

MEMBER
─────────────
My access            ← landing route
Requests            [1]
```

Parent entries with children (Access, Automation, Review, System) are **section labels, not links** — 10.5px uppercase, `letter-spacing:.13em`, `rgba(243,245,239,.36)`. Leaf rows are 12px-radius pills with a 6px status dot at the left; the active row is `rgba(216,242,78,.13)` fill with `#d8f24e` text at weight 600.

Badge tone follows the semantic palette: Requests and Pending changes take the accent, Expiring access takes attention amber, Unexplained access takes danger red.

### Badge data source

The sidebar must **not** consume `GET /api/v1/governance/summary`. Today it fetches the full payload and takes `.length` of the returned arrays — downloading every pending request and expiring grant object on each mount to render two numbers.

Add and poll a compact endpoint:

```
GET /api/v1/governance/indicators
→ { "pending_requests": 3, "expiring_grants": 1, "pending_propagation": 2, "drift": 12 }
```

Sidebar polls indicators frequently and cheaply; Today consumes the full summary and refreshes on view.

### Route → home matrix

Every existing route gets exactly one home. No route appears twice.

| Route | Member | Basic | Advanced | Notes |
|---|---|---|---|---|
| `/` | **My access** | Today | Today | |
| `/users` | — | People | People | |
| `/users/{id}` | *self only* | People › detail | People › detail | Same route, scoped |
| `/projects` | — | Access › Projects | Access › Projects | |
| `/projects/{id}` | — | Access › Projects | Access › Projects | Project → roles |
| *(new)* `/roles` | — | Access › Roles | Access › Roles | Partially backed — see Gaps |
| *(new)* `/projects/{id}/roles/{key}` | — | Access › Roles | Access › Roles | Needs new endpoint |
| `/applications` | — | Access › Apps | Access › Apps | Index |
| `/applications/{id}` | — | Access › Apps | Access › Apps | Token format + preview |
| `/requests` | **Requests** | Requests | Requests | Backend self-filters |
| `/bundles` | — | — *(assign from People)* | Bundles | Split by verb, below |
| `/policies` | — | — | Automation › Automatic rules | Legacy path |
| *(new)* `/automation/settings` | — | — | Automation › Settings | |
| `/governance/pending` | — | — | Automation › Pending changes | |
| `/operations/cascades` | — | — | Automation › Change history | |
| `/graph` | — | — | Automation › Access map | |
| `/governance/drift` | — | — | Review › Unexplained access | Two tabs |
| *(new)* `/review/expiring-access` | — | — | Review › Expiring access | Own route, not an audit tab |
| `/audit` | — | — | Review › Audit | |
| `/grants` | — | *301* | *301* | → `/governance/drift?tab=reconciliation` |
| `/zitadel` | — | — | System › Identity provider | |
| `/operations` | — | — | System › Event activity | |
| *(hardware)* | — | — | System › Hardware sync | Parked |
| `/login` | unauthenticated | | | Unchanged |

Any route with `—` in the Member column is **not rendered and not reachable** for members; the backend 403s the underlying reads regardless.

`/grants` ceases to exist: the *All grants* tab is removed (redundant with People and role membership, which answer the same question with the Access source attached); the *Reconciliation* tab relocates into Review › Unexplained access as a second tab.

**Bundles split by verb, not by view.** Assign/unassign a bundle to a person is Basic, from the person's detail page. Create/edit/delete a bundle and set the default for new members is Advanced. The rule generalises: *acting on one person* is Basic; *changing the machine that acts on everyone* is Advanced.

---

## The view switch

**Placement.** Top-right of the 66px header bar, immediately left of the account chip, separated from the rail. It reads as "what I'm looking at", not as a destination.

**Form.** A two-state segmented pill — never a dropdown, so both destinations stay legible and the current one is unmistakable across a noisy room.

```
container: display:flex; background:rgba(243,245,239,.07); border-radius:999px; padding:4px
active:    padding:6px 16px; border-radius:999px; background:#d8f24e; color:#12140d; 13.5px/600
inactive:  padding:6px 16px; border-radius:999px; color:rgba(243,245,239,.6); 13.5px/400
```

**Behaviour — three rules.**

1. **The URL does not change.** A person's detail page is the same route in both views. Switching reveals additional panels and actions in place; it never navigates. Losing your place is the fastest way to make a view switch feel punitive.
2. **No dead ends.** If something can only be resolved in Advanced, Basic says so plainly and offers a scoped jump — "This came from an automatic rule — **Open automation details →**". Activating it sets `ui_view = advanced`, stays on the same URL, and scrolls the newly-revealed panel into view. It never lands on a bare list.
3. **Basic is not Advanced-minus-features.** It is a smaller job done completely.

**Not rendered for members at all** — not greyed, not present-but-403.

---

## Screens

### Shell (all operator screens)

```
┌─ 252px sidebar ─┬───────────── content ─────────────┐
│ rail #101210    │ top bar 66px, border-bottom       │
│ border-right    │  breadcrumb …  [switch] [account] │
│                 ├───────────────────────────────────┤
│                 │ padding 30px 26px 36px            │
│                 │ vertical stack, gap 18–24px       │
└─────────────────┴───────────────────────────────────┘
```

Breadcrumb: 14.5px, ancestors at `rgba(243,245,239,.45)`, current page `#f3f5ef` weight 600. Account chip: 30px avatar + 13.5px name.

---

### Today — operator landing

**Purpose.** Actionable work only. Not "Dashboard", not "Overview" — both promise a summary and deliver a link farm.

**Greeting.** `Morning, Priya.` in Bricolage Grotesque 42px/500, followed by `Four things need you.` in the same line at `rgba(243,245,239,.4)`. Below: `Thursday 30 July · last checked 11:04` at 14.5px faint.

**Blocks.** Each is a rounded 20px card on `surface-1` with a header row (title 22px/600 + count badge + optional right-side link or note) and one row per item separated by `1px rgba(255,255,255,.05)`.

Basic shows two blocks:

1. **Open requests** `[3]` accent badge — row: 34px avatar · name (170px, 15px/600) · project/role (250px, role key in mono) · quoted reason (flex, faint) · relative time (66px) · `Approve` (accent fill) + `Deny` (outline). Actions are **terminal** — approving here does not open a detail page.
2. **Direct access expiring soon** `[1]` amber badge — same row shape, with the Direct source chip, the literal text `No action — expires 2 Aug`, remaining time in amber, and a single `Extend` outline button. Header note: *"Extending is the only action — doing nothing lets it lapse."*

Advanced appends two more:

3. **Pending changes** `[2]` accent — one summary line plus a `Resume now` button. When `pending_propagation.zitadel_reachable === false` the button is **disabled and the reason is a visible amber strip inside the card**, not a tooltip: *"Disabled — identity provider unreachable since 09:38. Writes stay queued; nothing is lost."*
4. **Unexplained access** `[12]` danger — one summary line plus `Open triage`.

**Empty state.** A 44px accent-tint circle with a 12px accent dot, `Nothing needs you.` at 36px/500, one sentence with a timestamp, and one link out. No illustration, no "you're all caught up!".

`GET /api/v1/governance/summary → GovernanceSummary`

---

### People index — `/users`

Header: `People` 40px/500 + `248 accounts · 4 with access expiring inside 30 days`. Right side: a 260px-min search field and an `Any project ⌄` filter pill.

Table columns: **Person** (250px — 32px avatar, name 15px/600, title 12.5px faint) · **Team** (150px) · **Bundles** (220px, neutral pills) · **Access** (flex, "6 roles across 3 projects") · **Needs attention** (150px, right-aligned).

The last column is the reason this list exists rather than being a plain directory: it carries the one thing about this person that might need you, in the semantic palette — `1 expires in 2 days` amber, `1 open request` accent, `1 unexplained` red, `—` faint when clear. Departed accounts render the whole row at 60% opacity.

**Search matches role keys as well as names and emails** — "who has `trained` in the laser lab" gets typed here before anyone thinks to go to Roles.

Pagination is an explicit `Load next 50`, not infinite scroll.

---

### E3 · Person → access

The highest-traffic screen in the product.

**Header.** 62px avatar with initials · name 40px/500 · metadata row (email · title · team · `u_2f81` in mono) at 14.5px with 3px dot separators · right-aligned `Manage bundles` (outline) and `Grant direct access` (accent fill).

**Tabs.** Pill tabs, not underlines: `Access` (active, `rgba(243,245,239,.09)` fill) / `Requests` / `Activity`.

**Bundles strip.** A single 18px card: micro-label `BUNDLES`, then one neutral pill per bundle, then `Manage bundles →` right-aligned.

> **Bundle chips are not individually removable.** No inline ✕. Chips communicate membership and nothing else. Removal lives behind the explicit `Manage bundles` control, which opens the assign/unassign surface with impact preview. Inline removal invites a misclick that silently strips a dozen roles, and it stops scaling the moment someone holds four bundles.

**Multi-source notice.** When a role is held more than once, an accent-tint card states it plainly above the groups: *"Laser Lab / trained is held twice — through the Lab Tech bundle and through automatic rule R-014. Removing Lab Tech would not remove this role."*

**Role groups.** One 20px card per project. Header: project name 22px/600 + `2 roles · serves Badge Reader, Bookings` at 13.5px faint. Inside, roles are split under two micro-labels in fixed order: **GRANTED** then **AUTOMATIC** — the things a human decided read first.

Row: display name + role key in mono (230px) · Access source chip(s) (flex) · expiry/status · a 30px circular `⋯` overflow that opens the source-specific removal.

**Advisory notes.** `cleanup_hints` render as a quiet `?`-prefixed line at `rgba(243,245,239,.45)` below the groups — **never as an error**.

**In Advanced**, this same page (same URL) additionally reveals: raw grant ids, cascade lineage (rule id, fired timestamp, cascade id, holder count), and hardware sync state — appended in place.

`GET /api/v1/users/{id}/access → UserAccessView`

---

### E2 · Roles index and role → members

**`/roles`** — cross-project index. Columns: Project (180px, first and never collapsed) · Role display name + key · Group · Members (right-aligned). Filters for project and group as pill dropdowns.

> The same key in two projects means two different things. That is why Project is the first column.

**Partial-coverage notice, required.** `GET /api/v1/roles` calls `GetAllLocalRoles`, which returns only roles created through MkAuth. An accent-tint card above the table states: *"Showing MkAuth-managed roles only. Roles created directly in the identity provider aren't listed yet."* with a link to check a project's roles upstream. Silently partial lists are how people conclude a role doesn't exist and create a duplicate.

**`/projects/{id}/roles/{key}`** — role detail. Header: project name (13.5px faint) above role display name (40px/500), then the role key as a mono outline pill, then `Group: Safety-gated · cloned from Metal Shop / trained`. Clone provenance shows whenever present.

Members table header carries source filter pills: `All` / `Direct 3` / `Bundle 8` / `Automatic 7`. Row: avatar · name (210px) · title (170px) · source chip(s) with `+n more` overflow · since/expiry · the source-specific action.

### Source-specific removal — the rule

**There is never a generic "Revoke role."** A person can hold one role through several sources at once, so a generic action is ambiguous at best and destructive at worst. Name the action after the thing being removed, and state the residual outcome.

| Source | Action in the row | Confirmation must say | Backed today |
|---|---|---|---|
| `direct` | **Remove direct access** (red outline, currently disabled) | "They will still retain this role via *Lab Tech*." — or, if this was the only source, "They will lose this role." | **No** |
| `bundle` | **Remove bundle assignment** (red outline) | Which bundle, and every *other* role they'd lose with it | Yes |
| `mapping` | *no removal* — `Open the rule →` in accent | "This is automatic, from *3D Lab / operator*." | n/a |

The residual-outcome sentence is not optional garnish. It is the difference between a safe click and an outage on the laser cutter.

**Three confirmation dialogs** (`surface-2`, 22px radius, `0 22px 50px rgba(0,0,0,.55)`, focus-trapped):

1. **Bundle removal.** Source chip at top, then title, then two explicit lists under micro-labels: **HE WILL LOSE** (red-tinted rows, label in `#ff8d82`) and **HE WILL KEEP** (accent-tinted rows, each annotated with *why* it survives — "still automatic via R-014", "still via Studio Member"). Confirm button is a **solid red fill**.
2. **Direct removal.** Source chip, title, grant provenance line, then a red-tinted callout carrying the residual outcome in bold plus the real-world consequence ("She loses badge entry to the laser bay at the next cache compile"), and a note showing the alternative copy when a second source exists. Confirm button is **disabled** with the blocking endpoint named in body copy beneath it.
3. **Automatic — no removal offered.** Title: *"This one isn't yours to remove."* Shows the rule as `input ⇒ output` with its holder count and the warning that editing affects all of them, then explains the only per-person route: remove the *input* role. Two ways forward: `Open rule R-014 →` (accent tint) and `Go to 3D Lab / operator` (outline). No destructive colour anywhere — nothing is being destroyed.

**Endpoint gap:** role → members does not exist. `POST /api/v1/lookup` is only an id→display-name resolver. Assume a `BundleImpact`-shaped response: role identifier plus users with their access sources.

---

### E1 · Projects, and project → roles

**Index.** Table: Project · People (right) · Roles (right) · Apps served (a row of neutral pills).

**Apps and projects are not 1:1.** Badge Reader reads four projects; Studio Access feeds two apps. "Apps served" is a column of pills, never a single value.

**Detail (`/projects/{id}`).** Header: `Projects` kicker, project name 32px/500, then `31 people · 4 roles · serves Badge Reader and Bookings · proj_laser`. A `Token format` outline button top-right jumps to E6 scoped to this project.

Roles table: Role (display name + key, **plus its description rendered in full** at 13.5px `rgba(243,245,239,.5)`, plus clone provenance when present) · Group (120px) · Members (96px, right-aligned, accent, `14 →` linking into E2).

Descriptions are shown, not truncated to a tooltip — "can cut unsupervised" versus "may enter and watch" is the entire decision an operator is making, and a role key alone never conveys it.

`GET /api/v1/projects → ProjectSummary` · `GET /api/v1/roles`

---

### Apps index — `/applications`

Table: App (170px) · Kind (80px — `OIDC` / `SAML` / `API`) · Reads roles from (flex, project names) · Format (96px, right, mono).

Kind is shown because an OIDC client, a SAML SP and a plain API consumer fail in different ways.

An amber callout surfaces the common failure directly on the index: *"Badge Reader reads four projects in two different formats. That is legal and often deliberate, but it is the single most common cause of 'this app isn't seeing the roles it expects'."* The Format cell for such an app reads `mixed` in amber. Each row opens E6.

---

### E4 · Assign / unassign a bundle

Reached from **Manage bundles** on the person's detail page (and from the bundle itself in Advanced).

Panel: person name (13px faint) above `Manage bundles` (28px/600). A checkbox list of bundles as 14px-radius rows — assigned rows carry an accent fill checkbox and accent border; unassigned rows are neutral outline. The default-for-new-members bundle carries a neutral `Default for new members` pill.

**Below the fold is not acceptable for the preview.** A `rgba(243,245,239,.04)` panel titled `ADDING SHOP STEWARD WOULD GRANT` lists, with a dot per line:

- accent dot — roles gained directly from the bundle
- dashed dot, muted — roles gained by cascade, with the rule named ("and cascade to Metal Shop / trained via R-021")
- grey dot, faint — "3 roles he already holds — no change"

Footer: `Apply 1 change` (accent) + `Cancel`, with `Queues for confirmation` at the right.

Unassigning shows the same preview in reverse and **must distinguish roles that will actually be lost from roles the person retains through another source**.

`POST /api/v1/users/{id}/bundles` · `DELETE /api/v1/users/{id}/bundles/{bundleId}` · `GET /api/v1/bundles/{id}/impact` · `GET /api/v1/bundles/{id}/roles`

---

### E5 · Grant direct access

Project select → role select → expiry. The selected role's field carries an accent border and a helper line beneath it in plain language: *"Safety-gated · 22 people hold it · unlocks the Shopbot and the panel saw."*

**Expiry gets presets before the picker** — `30 days` / `End of term` / `Pick a date` / `Never` as pills, because "end of term" is what people actually mean. The resolved date shows with its distance: `18 Dec 2026 — in 141 days`.

An accent-tint consequence note closes the panel: *"On 18 Dec the sweep removes this automatically. He'll show up under Expiring access two weeks before, where the only action is to extend."*

Operator-only. Members get 403 by design, so **do not render the affordance for them**.

`POST /api/v1/users/{id}/grants` · `GET /api/v1/users/{id}/grants → DirectGrant`

---

### E6 · App token — format and preview

One screen, two equal halves.

**Left — Token format.** Per project: claim name (mono) and a `array` / `csv` segmented pill. Sub-copy warns that changing these changes what every app reading that project receives.

**Right — Preview token.** Person picker chip + `Run`, then the output on `surface-0` in JetBrains Mono 13px / line-height 1.85, claim names in accent, values in near-white, and `//` comments in `rgba(243,245,239,.35)` explaining the interesting silences:

```
// effective_role_keys → claims
"mkauth.laser.roles": ["trained", "maintainer"],
"mkauth.studio.roles": "door,wiki-read",
"mkauth.metal.roles": []

// Metal Shop is empty — he holds no role there.
// maintainer expires 2 Aug and will drop out.
```

An empty array is shown **as an empty array**, with a line saying why. The silence is the answer people are looking for. `Copy payload` and `Show raw roles` beneath.

`GET /api/v1/applications → ApplicationView` · `GET /api/v1/applications/{id}/simulate?user=`

---

### E7 · Requests — operator

Tabs: `Open 3` / `Decided`. **One request is expanded at a time**; the rest are compact rows.

Expanded request: 38px avatar · name 16px/600 · `asked 4 hours ago` · the ask in a sentence (`wants **Laser Lab / operator**`) · the requester's own words quoted at 14.5px `rgba(243,245,239,.6)`.

Beneath it, a `surface-0` decision panel in three columns, so nobody has to leave for the person's page to answer "should they have this":

- **What he already has** — current roles here, and whether he holds any safety-gated role anywhere.
- **Approving would grant** — the direct grant, *plus any cascade it triggers* ("then cascade Studio Access / door via R-022").
- **Prerequisite** — amber when unmet: `Laser safety not recorded` / "He holds no `trained` role here".

Actions: an expiry pill (`Expires End of term · 18 Dec ⌄` — the same presets as E5), `Approve as direct access` (accent fill), `Deny with a reason` (outline), and a note that either way the requester is told, in the operator's words.

> **Approving *is* E5.** It writes a direct grant with an expiry, so the button says so rather than saying "Approve".

### E7 · Requests — member

**Submit form asks in verbs, not role keys**, and maps the answer to `(project, role)` behind the scenes:

- *What do you want to use?* → `The laser cutter — Laser Lab`
- *What do you need to do with it?* → radio options written as capabilities: `Watch during open hours` ("No safety training needed") / `Cut and engrave on my own` ("Needs laser safety — a manager will check", amber)
- *Anything they should know?* → free text

**Your requests** list with three states as pills: `Waiting` (accent tint), `Granted` (neutral), `Declined` (red tint). A decline **always carries the operator's own words** — "Come to a Thursday induction first and I'll approve it the same day. — A. Osei". A bare "Declined" generates a support conversation every time.

Members see only their own — enforced at the backend, and the UI must match rather than filter client-side.

`GET/POST /api/v1/requests` · `POST /api/v1/requests/{id}/decision`

---

### Member · My access

Landing route for members. Two destinations total; no Today, because a work queue for someone with no queue is an empty room.

Greeting: `Hi Tomas.` / `Here's what you can use.` Sub-line: `Two memberships and six roles. One expires soon.`

Roles grouped by project in two side-by-side cards. **Access source becomes a sentence, not a chip** — a member has no vocabulary to attach to Direct / Via bundle / Automatic:

- bundle → "Because you're in **Lab Tech**"
- direct → "Given to you until 2 Aug" (amber)
- mapping → "Comes with door access, automatically"

An amber callout for the expiring role with a single `Request an extension` action.

No jargon anywhere on this screen: no "derived", no `effective_role_keys`, no role keys.

---

### S5 · Access map (Advanced)

Currently the least legible screen despite being the most conceptually valuable. The fix is not a better force layout — it is **not drawing everything at once**.

**Left panel (228px).** Search field, a `SHOW` legend with five node shapes (project = accent rounded square, role = filled circle, bundle = outlined rounded square, rule = dashed circle, app = faint outlined square), a `Depth` segmented control (`1 hop` / `2` / `All`), and a footer note: *"248 nodes, 611 edges in total. Drawing them all is what made this screen useless."*

**Centre.** One focused node answers exactly two questions, left to right:

```
FEEDS IN (240px) → rail (54px) → [ focused node 250px ] → rail (54px) → FEEDS OUT (flex)
```

The focused node is an accent-tinted 20px card with a 1.5px accent border. Incoming and outgoing nodes are `surface-1` cards with a shape swatch, a name and a one-line qualifier ("bundle · 8 holders", "from 3D Lab / operator · 7 holders", "reads mkauth.laser.roles · array"). Rules use a dashed border to match their dashed source chip.

Connector rails are thin gradient lines fading toward the accent at the focused node. **If you implement these with SVG, compute both endpoints' `getBoundingClientRect()` first and derive the path from those coordinates** — do not hand-guess offsets.

A dashed `Expand to 2 hops · 6 more nodes` affordance closes the outgoing column.

`GET /api/v1/topology → TopologyGraph`

> The topology kinds (`project`, `role`, `contains`, `application`, `bundle`, `rule`) are a **separate namespace** from Access source and must not use that component.

---

### S6 · Unexplained access — the highest-stakes screen

Two tabs: `Triage queue 12` (count in red) and `Reconciliation`.

**Bulk bar.** `4 selected` · `Adopt 4 in MkAuth` (accent fill) · `Mark 4 as owned elsewhere` (outline) · and a stated absence: *"Bulk revoke is deliberately not offered."*

> **Bulk adopt and bulk mark-as-external exist; bulk revoke does not.** Adopting is reversible bookkeeping. Revoking removes real access from real machines, and reading twelve consequences at once is not something anyone actually does — so revoke stays one row, one dialog, one decision. This is stated on the screen in a red-tint card, not just in the spec.

**Row.** Checkbox (18px) · **Who** (186px — avatar, name, status like "Alumni · left Mar 2026") · **What they can get into** (250px — project/role plus a risk pill: red `Safety-gated · cuts metal`, amber `Safety-gated`, neutral `Role not in catalogue`) · **Why MkAuth can't explain it** (flex, a full sentence naming the upstream actor and date) · **Found** (96px, red when oldest) · **Resolve** (300px, right-aligned, always in the same order: `Adopt` accent fill / `Revoke` red outline / `Owned elsewhere` outline).

**Ordering is by risk then age, not age alone** — a safety-gated role found yesterday outranks a wiki role found last week. A red 3px left border and the red age are the only ranking signals; **the row layout never changes**.

Actions that don't apply are neutralised rather than hidden: a service-account row shows `Adopt` in muted grey and `Owned elsewhere` pre-filled.

**Expanded row** (`surface-0`, 14px radius, three columns):

- **If you revoke** — the physical consequence ("She loses badge entry to the laser bay at the next cache compile. She holds nothing else in this project.")
- **If you adopt** — what MkAuth records, who becomes granter of record, and that nothing changes upstream.
- **Evidence** — mono `grant_id / created / actor / last_seen`.

**Revoke dialog.** The only place a solid red fill appears. Red-tint outcome callout, a paragraph on what happens both sides (and that the item will reappear tomorrow if an integration re-creates it), and the one piece of context that changes the decision — "Marta left in March 2026 … 2 more items for this person".

**Reconciliation tab.** MkAuth ↔ provider diff. Columns: Project/role · MkAuth (right) · Provider (right) · Difference (sentence) · Action. Two directions of drift, named differently:

- provider has extras → red count, `See in triage →`
- MkAuth has records the provider lacks (a write that never landed) → amber count, `Re-push →`

Agreeing rows stay visible at 55% opacity with `In agreement`.

`GET /api/v1/governance/drift` · `POST …/drift/{id}/{attribute|revoke|mark-external}` · `POST …/drift/bulk-attribute` · `POST …/drift/reconcile` · `GET /api/v1/reconciliation/grants`

---

### S7 · Expiring access

Own route (`/review/expiring-access`), not an `/audit` tab: audit is a record you consult, this is time-boxed work you act on before a deadline. Nesting the second inside the first buries a deadline in a log and makes the sidebar badge point at a tab rather than a destination.

Header sub-line states the mechanism: *"Direct grants inside the next 30 days, soonest first. The sweep removes each one on its date whether or not you visit this page."*

Columns: Who (210px) · What (260px) · Granted (180px, "by P. Raghunathan, 4 Jul") · **State** (flex — `No action — expires 2 Aug`) · Remaining (110px, right) · Action (110px, right).

**There is exactly one action: extend.** Only the soonest row is emphasised (amber left border, amber tint, accent-fill `Extend`); the rest carry outline `Extend` and neutral remaining-time. Amber is a deadline signal, not a decoration for the whole table.

Two explanatory cards close the screen:

- *Why there's no second button* — a "Let it lapse" or "Dismiss" control would submit nothing and change nothing, but would make an operator believe they had recorded a decision. The row's resting state already says what happens.
- *Flagged, not assumed* (dashed border) — if operators later need an acknowledged/deferred state, that is a real new capability with its own column and endpoint. **It must not be faked client-side by hiding rows** — a shared queue that diverges per operator is worse than a noisy one.

Extending is already backed: `POST /api/v1/users/{id}/grants` upserts on `(user, project, role)` and overwrites `expires_at`, so re-submitting with a later date renews in place rather than creating a duplicate.

---

### S1 · Bundles

Three-column layout: bundle list (210px, nav-style rows with holder counts, the default bundle carrying its pill) · roles in the selected bundle · impact.

Each role row has a red-outline `Remove`. Selecting one replaces the impact panel with a red-tint **`REMOVING LASER LAB / TRAINED AFFECTS 8 PEOPLE NOW`** breakdown:

- red dot — **5 lose the role outright** — no other source
- grey dot — 3 keep it via automatic rule R-014
- dashed dot — 2 also lose Studio Access / wiki-read by cascade

with `Remove role from bundle` (solid red), `Cancel`, and `Names of all 8 →`.

> The impact panel occupies the space where a confirmation dialog would have been. The consequence is on screen *before* you commit, not after you click.

A separate row handles **Default for new members** — a segmented control, with the copy stating that exactly one bundle can hold it.

`GET/POST /api/v1/bundles` · `POST /api/v1/bundles/{id}/roles` · `DELETE …/roles/{projectId}/{roleKey}` · `PUT /api/v1/bundles/{id}/welcome` · `GET /api/v1/bundles/{id}/impact`

---

### S2b · Automation settings — `/automation/settings`

One setting: the global default confirmation mode for *newly created* rules. Two radio cards, each described **by its cost rather than its name** — "manual" and "auto" tell an operator nothing about what they're trading:

- **Queue for review** — "Writes wait in Pending changes until someone confirms them. Slower, and nothing reaches a machine without a person seeing it first."
- **Apply immediately** — "Cascades reach the identity provider the moment a rule fires. Fewer clicks, and no chance to catch a bad rule before it lands."

An amber card states the placement rule: this is org-wide policy, which is why it has a page inside Automation. **It must never live in the sidebar footer, an account menu or a preferences drawer** — those read as personal settings, and someone will flip a policy for everyone thinking they changed something about themselves.

`GET/PUT /api/v1/config/confirmation-mode-default`

---

### S2 · Automatic rules

**List.** Rule id (mono, accent when selected) · **If … then** as a sentence (`3D Lab / operator ⇒ **Laser Lab / trained**`, antecedent muted, consequent bold) · Holders · On fire (`Queue` neutral / `Immediate` amber).

**Editor.** Two fields either side of a `⇒`, labelled *If someone holds* and *then also give them* — the rule reads as an English sentence with two fields in it, not a form with a "source" and a "target". Per-rule confirmation mode as a segmented pill.

**Validation, amber, above the actions:**

```
VALIDATED — 2 THINGS TO KNOW
Would grant Laser Lab / trained to 3 more people immediately on save.
Chains into R-022, which would then also give those 3 Studio Access / door.
```

Save is blocked until validation passes, and the note says so. Naming the chain matters — a rule that triggers another rule is the single most surprising thing this system does.

`GET/POST /api/v1/rules/mapping` · `PUT …/{id}` · `POST …/validate` · `POST /api/v1/policies/confirmation-mode`

---

### S3 · Pending changes

Amber banner when unreachable: *"Identity provider unreachable since 09:38. Confirming is disabled, not failing silently. Writes stay queued and in order; nothing is lost and nothing has reached a machine."* `Confirm all 2` is disabled in the header.

Table: Who (150px) · Write (flex, `grant Laser Lab / trained`) · Caused by (120px — rule id in accent + cascade id faint) · Queued (78px, right).

**Grouped by cascade, not flat by timestamp** — a half-applied cascade is the thing that creates unexplained access. A closing line explains that both writes share `c_8841` because R-014 chained into R-022, and that they confirm together or not at all.

`GET /api/v1/propagations` · `POST /api/v1/propagations/drain`

---

### S4 · Change history

One 18px card per cascade, newest first: cascade id (mono, accent for the most recent) · a one-line title of what changed · a state pill (`2 writes waiting` amber / `8 applied` accent / `no writes` neutral) · timestamp · then a sentence of consequence.

Each entry is a sentence about consequence, not a diff — `8 applied`, `2 waiting`, `no writes` is the whole vocabulary.

`GET /api/v1/propagations/cascades → CascadeSummary`

---

### S8 · Audit

Filters: date range, actor, `Export CSV`. Columns: When (110px mono) · Who (130px) · What they did (flex, a full sentence) · Trace (70px, right — cascade id in accent, or `—`).

Every line names a human or a *named* machine (`expiry-sweep`). Destructive verbs take `#ff8d82` on the word only (`**Revoked** unexplained access — …`). The `c_` column is a live link into Change history, which is the only way to see what an entry actually did downstream.

`GET /api/v1/audit`

---

### S9 · Identity provider

**Health is a sentence with a cause and a timestamp — never a green or red dot on its own.** Red card when down: *"Unreachable — 6 consecutive failures. Last successful call 09:38 today. Connection refused on `:8080/oauth/v2/token`. MkAuth is serving its own cache; the degraded banner is showing everywhere."*

Three stat cards: **Action signing key** (`Rotates in 9 days` in amber, last rotated + `kid_`), **Cache age** (`1h 26m`, "Compiled 09:38, not since"), **Projects upstream** (`6 · 611 grants`, "As of last successful read").

**Inspect upstream directly** — three rows (projects/roles, users, grants) each with their path in mono and an `Open` button. All three are **disabled while the provider is unreachable**, with a red dashed strip explaining: they read live, and there is nothing to read.

`GET /api/v1/zitadel/{health,projects,users,grants,action-rotation-status}` and nested variants

---

### S10 · Hardware sync — parked

Dashed 1px border on the whole panel. A dashed 44px circle, `Not connected yet.` at 26px/600, then two paragraphs: what will live here (provisioning intents, per-user shadow credentials, the path for door controllers and machine interlocks that can't speak OIDC), and the honest status (LLDAP isn't built; nothing queued, nothing pending, no hardware reading from MkAuth). One outline action: `Read the integration plan`.

Footer note, which is the design rule: **not a spinner, not an empty table, not "0 intents"** — those all imply the feature works and is merely idle. The dashed border says unbuilt, and the copy says so out loud.

`GET /api/v1/intents` · `GET/PUT/DELETE /api/v1/users/{uid}/shadow-credential*`

---

### S11 · Event activity

**A raw timeline, not a dashboard.** No tiles, no counts, no roll-ups — Today already owns actionable summary, and a second dashboard here would split attention and make it ambiguous which is authoritative.

Filters: source, date. Row: time (64px mono `09:42:07`) · event type (132px, 12.5px/600 — `onboarding.trigger`, `provider.error`, `cache.compile`, `webhook.user.added`, `drift.sweep`, `expiry.sweep`) · what happened (flex, one sentence, cascade ids in accent) · `payload ⌄`.

**Only the error row is tinted**, and only because it is the row someone is scrolling to find. Each row drills to its raw payload; each cascade id links into Change history.

`GET /api/v1/webhook/events` · `GET /api/v1/onboarding/triggers`, merged into one time-ordered stream.

---

## Access source — the signature component

The "why does this person have this" explanation appears in E2, E3, unexplained-access triage and change history. It is the single element that will do most to make this product feel calm. Build it once.

The data model emits exactly **three** kinds from `ExplainUserAccess`, so the vocabulary is fixed and small. **Order is always Direct → Via bundle → Automatic**, everywhere, so a scanning eye learns one sequence.

| `RoleReason.Kind` | Label | Mark | Chip style (dark) | Revocable here |
|---|---|---|---|---|
| `direct` | **Direct** | 8px solid accent dot | `background:rgba(216,242,78,.15); color:#d8f24e` | Yes |
| `bundle` | **Via bundle** | 8px ring, 2px `rgba(243,245,239,.8)` | `background:rgba(243,245,239,.08); color:rgba(243,245,239,.82)` | No — remove the bundle |
| `mapping` | **Automatic** | 8px **dashed** ring | `border:1px dashed rgba(243,245,239,.34); color:rgba(243,245,239,.66)` | No — change the rule |

Shared: `display:inline-flex; align-items:center; gap:6px; padding:5px 11px 5px 9px; border-radius:999px; font-size:12.5px; font-weight:600; white-space:nowrap`.

The dot carries the meaning before the word does: **solid** = a person did it, **ring** = a bundle did it, **dashed** = the system did it on its own. Dashed means nobody clicked it.

**Optional qualifier.** A 12.5px `rgba(243,245,239,.5)` string may follow the chip — "Lab Tech", "3D Lab / operator", "expires 2 Aug".

**Multiples.** A role can carry more than one source. Render the strongest source (fixed order above) as a full chip and collapse the rest into a neutral `+2 more` pill of the same height. Never a wall of chips.

**Expanded form** — popover on the chip (or an inline detail row). Header: the chip plus a plain-language title ("Granted directly"). Body: a `By / When / Expires / Note` grid at 14px with 80px labels. Footer: only the action that belongs to *this* source. Bundle and Automatic popovers link out instead of offering a remove.

Props: `kind: 'direct' | 'bundle' | 'mapping'`, `detail?: string`. See `design/Source.dc.html`.

---

## Four states, every list view

Not three. **Degraded is real and specific.**

| State | Treatment |
|---|---|
| **Loading** | Row-shaped skeletons at the real row height (avatar circle + text bars, descending opacity `.07 → .03`), so nothing jumps when data lands. Because the name resolver is async, **every id-bearing surface needs a resolving state — never flash a raw UUID.** |
| **Empty** | A 22px/600 sentence naming what is absent ("Nobody holds this role yet.") + one line of guidance + one accent link to the next move. No illustration. |
| **Error** | Danger-bordered card. 22px/600 title naming the failed thing ("Couldn't load role members."), a line confirming **nothing was changed**, a `Try again` button and a mono request id (`req_9c40`) the operator can paste into a message. |
| **Degraded** | `GET /api/v1/system/mode` returns `degraded: true` when Zitadel is configured but the backend fell back to demo data — at which point every number on screen is fiction. A **persistent, undismissable amber field banner**: "These numbers are not real." + "The identity provider is configured but unreachable, so MkAuth is serving demo data. Don't grant or revoke anything until this clears." The content behind it is dimmed to 50%. Amber rather than red, because the data is wrong rather than dangerous. |

---

## Interactions & behaviour

- **View switch** — sets `ui_view`, persists per user, re-renders in place. No route change, no scroll reset. A scoped jump additionally scrolls its target panel into view (use `scrollTo` on the container, not `scrollIntoView`).
- **Destructive actions** always open a focus-trapped dialog. Never a bare `confirm()`, never an inline undo-only toast.
- **Terminal row actions** (Approve / Deny on Today) resolve in place with a toast and remove the row. They never navigate.
- **Disabled controls** state their reason in visible copy, not only in a `title`. Hover does not exist on touch and does not survive a screenshot sent to a colleague.
- **Overflow `⋯`** on a role row opens the source-specific menu; with multiple sources it lists one entry per source, each named after its own removal.
- **One expanded item at a time** in Requests (operator) and the S6 triage queue — expanding a second collapses the first.
- **Pagination is explicit** (`Load next 50`), never infinite scroll, on People and the triage queue.

## State management

Per screen: `uiView` (`'basic' | 'advanced'`, persisted), `theme` (`'dark' | 'light'`, persisted), `activeTab` (person detail, S6, Requests), filter selections, bulk selection set (S6), expanded row id, dialog open + payload, optimistic row state for approve/deny/adopt.

Global: indicators poll (sidebar badges), `system/mode` degraded flag (banner), name-resolver cache, toast queue.

Data fetching: sidebar polls `/governance/indicators`; Today fetches `/governance/summary` on view; everything else fetches per route.

## Permissions

Two classes only: **operator** (full) and **member** (self-service). The backend enforces at the trust boundary — cross-user reads return 403 — so **never render an affordance that will fail**. Advanced is operator-only and is not rendered for members at all.

## Accessibility — non-negotiable

Keyboard navigation throughout; dialog focus traps; visible `:focus-visible` rings in the accent; contrast checked in both themes. There is a standing per-route manual checklist — empty / loading / error / dense / ultra-wide / narrow / keyboard / light-theme contrast. Design to pass it.

Amber and red are load-bearing, so never let colour be the *only* signal: the expiring row also says "expires 2 Aug", the degraded banner also says "These numbers are not real", destructive buttons also say "Remove", and the S6 risk pill also says "Safety-gated · cuts metal".

## Gaps to build around

1. **Role → members has no endpoint.** Design the view; engineering adds it. Assume a `BundleImpact`-shaped response.
2. **`/roles` is only partially backed** — `GetAllLocalRoles` returns MkAuth-created roles only. Ship the explicit scope notice until an aggregate endpoint or per-project fan-out over `GET /api/v1/zitadel/projects/{id}/roles` exists.
3. **Removing a direct grant has no endpoint.** `DELETE /api/v1/users/{id}/grants/{grantId}` is needed. Until then the action renders disabled. Do not wire it to the Zitadel-side delete.
4. **No compact indicators endpoint.** Add `GET /api/v1/governance/indicators`.
5. **No paginated expiring-grants endpoint.** S7 reads `expiring_grants` off `governance/summary` for now.
6. **Hardware sync is parked** pending LLDAP — needs the honest "not connected yet" state above, not a spinner.
7. **Apps and projects are not 1:1.** Never design or model as though each project has exactly one app.

## Open decisions for the product owner

- **Bulk revoke in S6** is deliberately absent. If the triage backlog is routinely twelve deep, revisit.
- **Acknowledged/deferred expiry** in S7 is flagged, not assumed. It needs a real state and endpoint if operators ask for it.
- **Legacy URL alignment** (`/policies` under Automation, `/graph` for Access map, `/operations` for Event activity) is a phase-2 cleanup behind redirects. Nothing in this design depends on it.

## Assets

None. No images, no illustrations. Avatars are CSS gradients standing in for real photos. Icons are Lucide at stroke 1.5 — the design deliberately avoids icons where a geometric primitive reads faster (the Access source marks, the S5 node legend).

## Files

| File | What it is |
|---|---|
| `design/MkAuth IA.dc.html` | The full design board — every screen, state and annotation, §01–§18. Open in a browser. |
| `design/Sidebar.dc.html` | The navigation rail as a component, all three audiences (`view`: `basic` / `advanced` / `member`). |
| `design/Source.dc.html` | The Access source chip, all three kinds. |
| `design/support.js` | Authoring runtime for the HTML board. Not part of the design; do not port. |
| `DESIGN-BRIEF.md` | Original product brief — naming contract, endpoint inventory, full route matrix. Uses the older "Everyday / System" labels. |
