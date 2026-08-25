# BOARD-SPEC — `Syndra Mobile.dc.html`

Condensed build spec extracted from the mobile design board (63 phone figures at
390 × 844, one tablet figure at 744px, plus M00/M26/M27 reference panels).
Everything below is what is literally in the file's inline styles and copy.
Copy is quoted verbatim and is normative.

Read alongside `MOBILE.md`. Where the board and `MOBILE.md` disagree, the
disagreement is flagged with **[CONTRADICTION]** and collected in
[§ Contradictions](#contradictions-with-mobilemd).

---

## Recurring values (stated once — do not repeat per figure)

### Board chrome (not product UI — do not build)

| Thing | Value |
| --- | --- |
| Figure wrapper | `width:390px`, `flex-direction:column`, `gap:14px` (M12 uses 298px/gap 12px; M19 uses 340px/gap 13px; M26a uses 744px) |
| Phone frame | `border-radius:30px`, `border:1px solid rgba(255,255,255,.1)`, `background:#080906`, `overflow:hidden`, column flex. `min-height` varies 300–700px — a board artifact, **not** a screen height |
| Status bar | `height:34px`, `padding:0 20px`, JetBrains Mono `11px`, `rgba(243,245,239,.34)`, left `"9:41"`, right `"390 × 844"` (or `"Advanced"` in M01b/M18a) |
| Figure label pill | `11.5px/700`, `letter-spacing:.08em`, uppercase, `padding:4px 10px`, `radius:999px`. Violet `rgba(155,123,255,.22)` + `rgba(155,123,255,.4)` border; lime `rgba(163,230,53,.16)/.4`; amber `rgba(245,165,36,.18)/.45`; red `rgba(255,92,77,.18)/.45` |
| Section eyebrow | `12.5px/600`, `.12em`, uppercase, `#9b7bff` |
| Section h2 | Bricolage Grotesque `46px/600`, `line-height:1.02`, `-.02em` |
| figcaption | `13.5px`, `line-height:1.5`, `rgba(243,245,239,.5)` (M12/M19: `13px`) |
| Section container | `gap:34px` column; figure row `gap:30px` (M12/M19 `gap:24px`) |

### Colour tokens seen in the board

| Token | Hex / rgba | Meaning |
| --- | --- | --- |
| App background | `#080906` | shell |
| Sign-in background | `#0a0b08` | **only** M28 |
| Card surface | `#141612` | every card |
| Inset / field surface | `#0b0c0a` | search fields, copy-row groups, code |
| Sheet surface | `#191a22` | content sheets (M05c, M09, M14b, M15b, M21b, M22a, M23a, M24a) |
| Nav-sheet / bar surface | `#101210` | tab bar, nav sheet (M01b, M18a), bottom bars |
| Action-bar surface | `rgba(16,18,16,.94)` | pinned bottom action bars |
| Sheet footer surface | `rgba(11,12,10,.6)` (M22a `.66`) | in-sheet pinned footer |
| Text primary | `#f3f5ef` | |
| Text secondary | `rgba(243,245,239,.82)` / `.7` / `.62` | |
| Text muted | `rgba(243,245,239,.5)` / `.42` / `.38` / `.34` | |
| Accent (violet) | `#7f5af0` fill · `#9b7bff` link/dot · `#c9b6ff` on-accent text · `#6f4ae0` badge fill · `rgba(155,123,255,.13)` selected-row tint · `rgba(155,123,255,.07)` selected-list tint | |
| Accent, light theme | `#5b3fd6` (M13b only) | |
| Lime | `#a3e635` | healthy — **dot or word only, never a fill, never a button** |
| Amber | `#f5a524`, tint `rgba(245,165,36,.06)`–`.18`, border `rgba(245,165,36,.28)`–`.4` | deadline or broken assumption |
| Red | `#ff5c4d` state, `#ff8d82` text, border `rgba(255,92,77,.3)`–`.45`, tint `rgba(255,92,77,.07)`/`.14` | destructive only; solid fill only after a rung-3 gesture |
| Hairline | `rgba(255,255,255,.05)` row divider · `rgba(255,255,255,.06)` sheet divider · `rgba(255,255,255,.07)` card/section border |

### Layout primitives

| Primitive | Value |
| --- | --- |
| Page gutter | `16px` (M19 `15px`, M12 `14px`, M28 `22px`) |
| Sheet gutter | **`18px`** (nav sheet `14px`) — *not in `MOBILE.md`'s table* |
| Card | `background:#141612`, `border:1px solid rgba(255,255,255,.07)`, `border-radius:18px`, `padding:16px` (or `15px`, or `14px 16px` for one-line cards) |
| Small card | same, `border-radius:16px` |
| Blocked / disabled card | `border:1px dashed rgba(255,255,255,.14)`, title at `rgba(243,245,239,.5)`–`.6`, reason as body text in place |
| Sheet | `position:absolute;left:0;right:0;bottom:0`, `background:#191a22`, `border-top:1px solid rgba(255,255,255,.1)`, `border-radius:24px 24px 0 0`, `padding:12px 18px 24px`, `box-shadow:0 -30px 70px -30px rgba(0,0,0,.95)`, column `gap:13px` |
| Backdrop | `position:absolute;inset:0;background:rgba(8,9,6,.72)` — `.74`/`.76`/`.78`/`.80` where the page should recede further; the page behind is also dimmed with `opacity:.16`–`.26` |
| Grabber | `38 × 4px`, `radius:999px`, `rgba(255,255,255,.16)`, `margin:2px auto 2px` (nav sheet `2px auto 10px`) |
| List row (A2) | `min-height:60px`, `padding:11px 16px`, `display:flex;align-items:center;gap:12px`, `border-bottom:1px solid rgba(255,255,255,.05)`, trailing `›` at `13px`/`rgba(243,245,239,.42)` |
| Disclosed row body | `padding:4px 16px 16px`, column `gap:9px` |
| Disclosed row header | same row + `background:rgba(255,255,255,.035)`, chevron `transform:rotate(90deg)` |
| Group header (sticky) | `padding:11px 16px 7px`, `11px`, `.1em`, uppercase, `rgba(243,245,239,.38)`, `background:rgba(255,255,255,.02)`; amber variant `color:#f5a524; background:rgba(245,165,36,.06)` |
| Copy row | `min-height:52px`, `padding:11px 14px`, mono `13px`, `word-break:break-all`, trailing `"Copy"` at `12px`/`rgba(243,245,239,.42)`; confirmed state `background:rgba(163,230,53,.07)` + `"Copied"` at `12px` `#a3e635` 600 |
| Back-line | `‹` at `15px`/`rgba(243,245,239,.42)` + parent label `13.5px`/`rgba(243,245,239,.5)`, `gap:10px` |
| Page header block | `padding:8px 16px 12px`, `border-bottom:1px solid rgba(255,255,255,.07)`, column `gap:9px` (`10px`/`11px` when a strip or segment follows) |
| Page title | Bricolage `26px/600`, `-.02em` (`24px` where a back-line + id ride above; `22px` on two-object titles) |
| Freshness strip | row, `min-height:44px`, `justify-content:space-between`; dot `7px` + sentence at `13.5px`; Refresh pill `min-height:44px`, `radius:999px`, `border:1px solid rgba(255,255,255,.09)`, `padding:0 16px`, `13px` |
| Pinned action bar | `border-top:1px solid rgba(255,255,255,.07)`, `background:rgba(16,18,16,.94)`, `padding:12px 16px 24px`, `gap:10px`; secondary `min-height:50px` `radius:999px` `border:1px solid rgba(255,255,255,.12)` `14.5px/600`; primary `flex:1` `min-height:50px` `background:#7f5af0` `#f7f4ff` `14.5–15px/600` |
| Tab bar | `border-top:1px solid rgba(255,255,255,.07)`, `background:#101210`, `padding:8px 6px 22px`, `grid repeat(4,1fr)`, `gap:2px`; item `min-height:52px`, `radius:13px`, column centred, `gap:5px`, `7px` dot + `11.5px` label; active `background:rgba(155,123,255,.13)`, dot `#c9b6ff`, label `#c9b6ff` 600; inactive dot `rgba(243,245,239,.34)`, label `rgba(243,245,239,.5)`; badge `position:absolute;top:4px;right:16px`, `min-width:17px;height:17px`, `padding:0 5px`, `radius:999px`, `background:#6f4ae0`, `10.5px/700` |
| Checkbox | `22 × 22px`, `radius:7px`; on `border:1.5px solid #7f5af0; background:#7f5af0`, glyph `"✓"` `13px/700` `#f7f4ff`; off `border:1.5px solid rgba(255,255,255,.22)`; unselectable `border:1.5px dashed rgba(255,255,255,.18)` |
| Status dot | `7px` (list/inline), `8px` (provider cards), `9px` (timeline), `6px` (agreement ticks); "not yet" state = `border:1px solid rgba(243,245,239,.3)`, no fill; "not supported" = `border:1px dashed rgba(243,245,239,.5)` |
| Key–value pair | row `justify-content:space-between`, `font-size:13.5px`, key `rgba(243,245,239,.42)`, value `rgba(243,245,239,.82)`; date/id values mono `12.5px` |
| Inline link | `#9b7bff`, `13.5px/600`; when tappable it is wrapped to `min-height:44px;display:flex;align-items:center` |
| Search field | `flex:1`, `min-height:44px`, `radius:14px`, `border:1px solid rgba(255,255,255,.09)`, `background:#0b0c0a`, `padding:0 14px`, `14.5px`, placeholder `rgba(243,245,239,.38)`; focused border `rgba(155,123,255,.5)`, caret `1.5px × 19px` `#9b7bff` |
| Segmented control | wrapper `padding:4px`, `background:rgba(255,255,255,.045)`, `radius:999px`, `gap:6–8px`; segment `flex:1`, `min-height:38px`, `radius:999px`, `13.5px`; selected `background:#7f5af0`, `#f7f4ff`, 600 |
| Filter chip | `min-height:38px`, `radius:999px`, `padding:0 15px`, `13.5px`; on `background:#7f5af0` `#f7f4ff` 600; off `border:1px solid rgba(255,255,255,.1)`, `rgba(243,245,239,.7)` |
| Tab strip (in-page) | `padding:4–12px 16px 0`, `gap:20–22px`, `border-bottom:1px solid rgba(255,255,255,.07)`; tab `padding-bottom:11px`, `14px`; active `600` `#c9b6ff` + `border-bottom:2px solid #7f5af0`; counts inline in the label (`"Holds 1"`, `"Accounts 41"`) |
| Progress bar | `height:5px`, `radius:999px`, track `rgba(255,255,255,.08)`, fill `#7f5af0` (or `#ff5c4d` for a revocation drain); counter line mono `12px` |
| Keyboard mock | `height:172px` (M05b) / `186px` (M21b), `background:#1c1d19`, `border-top:1px solid rgba(255,255,255,.08)`, three rows of `height:30–32px` `radius:6px` keys, `gap:5px` |

### Motion (M00 — the whole vocabulary; four movements, nothing else)

| Movement | Timing | Rule |
| --- | --- | --- |
| `press` | **90ms** | `transform:scale(.97)` and back on every tap target. Fires on touch-**down**, not release. The only feedback a finger gets. |
| `settle` | **260ms** | Row disclosure, list reflow, content arriving in place. Rows below are **pushed, never overlapped**. |
| `rise` | **300ms** | **New on mobile.** A sheet rises 10px from the edge it will live on. The only surface that travels; dialogs, menus and popovers all become this. |
| `drain` | per item | Inline progress that reports as it goes, never in a modal. Failures accumulate in the line rather than interrupting. |

Reduced motion: each movement has a **still form, not a shorter one**. `press`
becomes a one-frame tint; `settle` and `rise` cut to final position; `drain`
keeps its counting text and drops the bar's transition.

**Forbidden, verbatim:** "Swipe actions. Pull-to-refresh. Long-press menus.
Parallax. Spring overshoot. Toasts. Skeleton shimmer that outlives the request.
Every one of them hides a decision behind a gesture with no label, and the
product's whole argument is that consequences are visible before you act."

**Targets and reach, verbatim:** "44px minimum, 50px for anything destructive,
12px between a destructive control and a benign one with the benign one nearer
the screen edge. Primary actions sit at the bottom inside the safe area. The top
of the screen is for orientation, not for action."

**Colour, verbatim:** "One violet per screen region. Lime is never a fill and
never a button. Amber is a deadline or a broken assumption. Red is destructive
only, and solid only once a rung-3 gesture is satisfied. Touch adds no colours."

Board header facts: Density `A2 default · A1 short lists`; Navigation
`B3 · bar for four, sheet for eight`; Ladder `Three rungs, intact`;
Breakpoints `720 · 1080`.

---

## M01 · Navigation

Section: "Item count decides the placement" — four or fewer destinations become
a tab bar; more than four become the rail arriving as a sheet on `rise`.

### M01a — "Basic · four tabs"
Shows the four rail items as four tabs, in rail order.
Structure: status bar → content `padding:10px 16px`, `gap:12px` → title row
(`"Today"` 26px + view pill) → one card → tab bar.
- View pill: `min-height:34px`, `padding:0 12px`, `radius:999px`,
  `border:1px solid rgba(255,255,255,.09)`, `13px`, `rgba(243,245,239,.62)`;
  caret `"▾"` at `10px`, `rgba(243,245,239,.32)`. **[CONTRADICTION: 34px < 44px]**
- Card: radius 18, padding 16. Eyebrow `"Requests waiting"` (11px/.1em/uppercase/.38);
  value `36px` Bricolage 600, `line-height:1.1`.
- Tab bar: `"Today"` (active) · `"People"` · `"Requests"` (badge `4`) · `"Roles"`.
Copy: `"Today"`, `"Basic"`, `"Requests waiting"`, `"4"`, `"People"`, `"Requests"`, `"Roles"`.
Caption: "The four rail items become the four tabs, in rail order. No *More*, because there is nothing more. The view pill sits in the header, never in the bar."

### M01b — "Advanced · rail as sheet"
Shows the desktop rail as a bottom sheet with a grabber, opened from the view pill.
Structure: status bar (right reads `"Advanced"`) → dimmed page (`opacity:.26`) →
backdrop `rgba(8,9,6,.72)` → sheet.
- Sheet: `#101210`, `border-top:1px solid rgba(255,255,255,.09)`,
  `radius:24px 24px 0 0`, `padding:12px 14px 24px`, `gap:3px`.
- View switch on top: segmented, `gap:6px`, `padding:4px`,
  `background:rgba(255,255,255,.045)`, segments `min-height:38px`.
- Rows: `min-height:44px`, `radius:12px`, `padding:0 14px`, `15px`,
  `rgba(243,245,239,.82)`; active `background:rgba(155,123,255,.13)`, `#c9b6ff`, 600.
- Divider after item four: `height:1px`, `rgba(255,255,255,.06)`, `margin:7px 8px`.
- Badges: violet `#6f4ae0` `min-width:20px;height:20px;padding:0 6px`, `11.5px/700`;
  amber `rgba(245,165,36,.18)` / `#f5a524`.
Copy: `"Basic"`, `"Advanced"`, `"Today"`, `"People"`, `"Requests"` `4`, `"Roles"`, `"Review"` `3`, `"System"`, `"Automation"`, `"History"`.
Caption: "Eight items, one divider, the view switch on top — the desktop rail with a grabber. Opened from the view pill; dismissed by the handle, a backdrop tap, or picking a destination."

### M01c — "Member · two tabs"
Shows the member shell: two inset tabs.
Structure: status bar → title `"My access"` → two cards (radius 18, padding 15, `gap:6px`) → tab bar `grid repeat(2,1fr)`, **`padding:8px 46px 22px`**, `gap:6px`.
Copy: `"My access"`, `"Laser cutter"` / `"You can use this. Induction done 11 Feb."`, `"Studio shares"` / `"Two folders. Password set 4 days ago."`, tabs `"My access"`, `"Storage"`.
Caption: "Two tabs, inset so they fall under the thumb rather than at the corners. A member never sees a view switch — there is nothing to switch to."

---

## M02 · Header and freshness

Section: "One line the operator cannot scroll past." No pull-to-refresh.

### M02a — "Fresh · action open"
Structure: status bar → header (`padding:8px 16px 14px`, `gap:10px`,
bottom border): back-line `‹ "System"` → title `"fileserver-01"` → freshness
strip (`min-height:44px`) → body `padding:14px 16px`, `gap:12px` → one card.
- Fresh dot `7px` `#a3e635`; sentence `13.5px` `rgba(243,245,239,.62)`.
- Refresh: outline pill, `min-height:44px`, `padding:0 16px`, `13px`.
Copy: `"System"`, `"fileserver-01"`, `"Read 40 seconds ago"`, `"Refresh"`, `"Adopt 3 accounts"`, `"Binds identity. Needs a read under 10 minutes old."`
Caption: "Lime dot, plain sentence, named control. The strip sits below the title and above the content, inside the sticky header region."

### M02b — "Stale · adoption blocked"
Same skeleton; the strip goes amber and Refresh takes the accent fill.
- Stale: dot + text both `#f5a524`. Refresh becomes `background:#7f5af0`,
  `#f7f4ff` 600, `padding:0 18px`, still `min-height:44px`.
- Blocked card: `border:1px dashed rgba(255,255,255,.14)`, title dimmed to `.5`,
  reason in place. Second card normal.
Copy: `"Read 11 minutes ago"`, `"Refresh"`, `"Adopt 3 accounts"`, `"Refresh first. Adoption binds an account to a person, and this read is 11 minutes old."`, `"Apply queued writes"`, `"Still fine — this queues rather than binds."`
Caption: "Refresh takes the accent when the read blocks something. The blocked card keeps its dashed border and states its reason in place — never a tooltip, which touch has no way to open."

### M02c — "Tap to copy"
The copy-row specimen. Structure: status bar → body `padding:12px 16px`,
`gap:12px` → title `"Connection"` (22px) → copy-row group
(`background:#0b0c0a`, `border:1px solid rgba(255,255,255,.07)`,
`radius:16px`, `overflow:hidden`) → footnote.
- Rows `min-height:52px`, `padding:11px 14px`, mono `13px`
  `rgba(243,245,239,.82)`, `word-break:break-all`, divider `.05`.
- Confirmed row: `background:rgba(163,230,53,.07)`, trailing `"Copied"` `#a3e635` 600.
Copy: `"Connection"`, `"smb://fileserver-01/studio"` + `"Copy"`, `"req_8f31c0a4"` + `"Copied"`, `"aditi.rao"` + `"Copy"`, `"The row confirms itself for 900ms, then returns. No toast — a toast covers the value you just copied."` (footnote `12.5px`, `.32`).
Caption: "Every id, endpoint, request id and connection string is a 52px row with a Copy affordance. Long strings wrap rather than truncate: an operator reading a path aloud needs all of it."

---

## M03 · Source capsule (signature component)

Component is `dc-import name="Source"` with `kind = direct | bundle | mapping`,
optional `detail` (the bundle name rides inside the capsule).
Hint sizes: direct `94 × 24`, bundle `106 × 24` (with detail `180`/`190 × 24`),
mapping `110 × 24`. Capsules **never shrink below their desktop size** and never
truncate; the resource name wraps instead.

### M03a — "Three kinds at 390px"
Structure: status bar → body `padding:16px`, `gap:16px` → three capsules stacked
`align-items:flex-start`, `gap:12px` → `1px` divider `rgba(255,255,255,.06)` → note.
Copy (note): "Capsules never shrink below their desktop size — they are where the eye lands when scanning forty rows. If a row is too narrow, the resource name wraps; the capsule does not truncate."
Caption: "Same component, same order, same vocabulary as §03. The bundle name rides inside the capsule so a row's second line carries permission and source together."

### M03b — "Popover becomes disclosure"
The desktop hover popover as an in-row disclosure. **One row open at a time, on `settle`.**
Structure: card (radius 18, `overflow:hidden`) → row header
(`min-height:60px`, `padding:12px 16px`, `background:rgba(255,255,255,.035)`;
left column `gap:6px`: name `15.5px/600`, then permission `13px` `.5` + capsule;
right chevron `width:20px`, rotated 90°) → body `padding:4px 16px 16px`, `gap:9px`
of key–value pairs, then a `padding-top:9px; border-top:1px solid rgba(255,255,255,.05)`
closing sentence at `13px`/`.5`.
Copy: `"3D printers"`, `"operate"`, `"Comes from"` → `"Workshop basics"` (link), `"Assigned by"` → `"K. Rao"`, `"Assigned"` → `"02 Jan 2026"` (mono 12.5px), `"To remove this, change the bundle or unassign it. It cannot be removed from here."`
Caption: "One row open at a time, on *settle*. The sentence about where a bundle grant can be removed moves from the popover into the disclosure — otherwise touch loses it entirely."

---

## M04 · Today (landing)

Section: "Today, one block per screen-height." Blocks stack in desktop order and
keep their own actions.

### M04a — "Basic · two blocks"
Structure: status bar → content `padding:10px 16px 16px`, `gap:14px` → title row
(`"Today"` + Basic pill) → block card → block card → tab bar.
- Block card: radius 18, `overflow:hidden`. Head `padding:15px 16px 12px`,
  `align-items:baseline`, eyebrow 11px + count Bricolage `22px/600` (amber when the
  count is a deadline). Item rows `min-height:60px`, `padding:11px 16px`,
  `border-top:1px solid rgba(255,255,255,.05)`, primary `14.5px/600`,
  secondary `13px`/`.5`, trailing `›`. Footer link row **`min-height:48px`**,
  `padding:0 16px`, link `13.5px/600`. **[CONTRADICTION: 48px footer row]**
Copy: `"Today"`, `"Basic"`, `"Requests waiting"` `4`, `"Aditi Rao"` / `"Laser cutter · 2 days"`, `"Meera Nair"` / `"CNC router · 1 day"`, `"All 4 requests"`, `"Expiring this week"` `11`, `"Studio Access"` / `"6 people · Fri 15 Aug"`, `"All 11"`.
Caption: "Each block shows two rows and a link to the rest, so the first screen-height carries work rather than two large numbers. The count moves beside its label instead of below it."

### M04b — "Advanced · five blocks"
Structure: status bar → content `gap:12px` → title row (Advanced pill:
`border:1px solid rgba(155,123,255,.35)`, `background:rgba(155,123,255,.1)`,
`#c9b6ff`) → five one-line cards → **"Go to" bar** replacing the tab bar.
- One-line card: **`radius:16px`**, `padding:14px 16px`, row space-between;
  title `14.5px/600`, subtitle `13px`/`.5`, count Bricolage `22px/600`.
  **[CONTRADICTION: 16px vs M04a's 18px for the same block]**
- Go to bar: `border-top`, `background:#101210`, `padding:12px 16px 24px`,
  space-between; left label `13px`/`.5`; control `min-height:44px`,
  `padding:0 16px`, `radius:999px`, `background:rgba(255,255,255,.06)`,
  `13.5px/600`, with three `3px` dots.
Copy: `"Today"`, `"Advanced"`, `"Requests waiting"` / `"Oldest 2 days"` / `4`, `"Expiring this week"` / `"Across 3 roles"` / `11`, `"Withdrawn, still present"` / `"2 targets"` / `3`, `"Queued writes"` / `"None failed"` / `7`, `"Providers answering"` / `"All 4 responded"` / lime dot, `"Advanced · 8 destinations"`, `"Go to"`.
Caption: "Advanced trades the tab bar for a single *Go to* bar that raises M01b. Blocks compress to one line with the subtitle carrying the fact that makes the count actionable."

**No equivalent in the app:** the "Go to" bar (and the whole tab-bar/nav-sheet
split) — `ui/src/lib/nav.ts` has one rail, no bar, no sheet.

---

## M05 · People

Section: "Search takes the whole screen."

### M05a — "Index"
Structure: status bar → header (`padding:10px 16px 12px`, `gap:11px`, bottom
border): title `"People"` → search row (`gap:8px`: field `flex:1` +
Filter button) → grouped list.
- Filter button: `min-height:44px`, `radius:14px`,
  `border:1px solid rgba(155,123,255,.35)`, `background:rgba(155,123,255,.1)`,
  `padding:0 14px`, `13.5px/600`, `#c9b6ff`, label carries the count.
- Group header: `padding:11px 16px 7px`, `background:rgba(255,255,255,.02)`. Sticky.
- Person row: **`min-height:64px`**, `padding:12px 16px`; name `15.5px/600`,
  fact `13px`/`.5`; optional amber `7px` dot before the chevron.
  **[CONTRADICTION: 64px vs 60px]**
Copy: `"People"`, `"Search 184 people"`, `"Filter 2"`, `"Studio · 3"`, `"Aditi Rao"` / `"4 roles · 1 expiring Fri"`, `"Meera Nair"` / `"2 roles"`, `"Devan Suresh"` / `"6 roles · 1 hold"`, `"Fabrication · 41"`, `"Priya Menon"` / `"3 roles"`.
Caption: "Grouped by project as on desktop, headers sticky. Two lines per person: the name, then the fact an operator opens the page for. The amber dot repeats what the second line says — colour never carries it alone."

### M05b — "Search overlay"
Full-screen replacement: no page title, keyboard up.
Structure: status bar → search bar row (`padding:8px 16px 12px`, `gap:12px`:
focused field + `"Cancel"` `14px`/`.62`) → results grouped **by kind** → keyboard.
- Result group header `padding:10px 16px 6px` (no tint).
- Result row `min-height:60px`, `padding:11px 16px`.
- Match highlight: `background:rgba(155,123,255,.24)`, `radius:3px` —
  **highlighted, not bolded.**
Copy: `"rao"`, `"Cancel"`, `"People · 2"`, `"Rao, Aditi"` / `"Studio · 4 roles"`, `"K. Rao"` / `"Operator · Fabrication"`, `"Accounts on targets · 1"`, `"aditi.rao"` (mono 13.5px) / `"fileserver-01 · adopted"`.
Caption: "Results group by what they are, because an operator searching a name is sometimes after an account on a target. Matches are highlighted, not bolded — bold already means the primary line."

### M05c — "Filter sheet"
Structure: dimmed page (`.22`) → backdrop `rgba(8,9,6,.74)` → sheet
(`#191a22`, `padding:12px 18px 24px`, `gap:13px`): grabber → title row
(`"Filter"` 19px Bricolage 600 + `"Clear 2"` `13.5px` `#9b7bff`) → two chip
groups (eyebrow + wrapping chips, `gap:8px`) → apply button.
- Apply: `height:50px`, `radius:999px`, `background:#7f5af0`, `15px/600`,
  `margin-top:2px`. Label carries the result count.
Copy: `"Filter"`, `"Clear 2"`, `"Project"`: `"Studio"` (on), `"Fabrication"`, `"Electronics"`; `"State"`: `"Expiring soon"` (on), `"Has a hold"`, `"Dormant"`; `"Show 3 people"`.
Caption: "The apply button carries the result count, so the operator knows the outcome before committing. Filters apply on *Show*, not on tap — a list reflowing under a moving thumb loses the operator's place."

---

## M06 · One person's access (E3)

Section: "A1 here, because every row is a decision" — under ~8 rows the
labelled-line card wins over the A2 row.

### M06a — "Access · granted"
Structure: status bar → header (back-line `"People · Studio"`, title
`"Aditi Rao"`, meta `12.5px`/`.5`) → body `padding:12px 16px 16px`, `gap:12px`:
section header row (eyebrow + action link) → grant cards → second section header
→ automatic card.
- Grant card: radius 18, padding 16, `gap:12px`. Head column `gap:7px`:
  resource `16px/600`; permission `13.5px`/`.62` + capsule. `1px` divider
  `rgba(255,255,255,.05)`. Key–value pairs `gap:8px`. Then the remove button.
- Remove button: `height:50px`, `radius:999px`,
  `border:1px solid rgba(255,92,77,.4)`, `background:transparent`, `#ff8d82`,
  `14.5px/600`.
- Non-removable cards carry a `13px`/`.5` sentence instead of a button.
Copy: `"People · Studio"`, `"Aditi Rao"`, `"Studio · member since Jan 2026"`, `"Granted · 2"`, `"Give access"`, `"Laser cutter"` / `"operate"` / direct, `"Granted by"` → `"K. Rao"`, `"Until"` → `"30 May 2026"` (mono, amber), `"Remove this grant"`, `"3D printers"` / `"operate"` / bundle `"Workshop basics"`, `"Comes from a bundle. Change the bundle or unassign it — it cannot be removed here."`, `"Automatic · 1"`, `"Studio Access"` / `"enter"` / mapping, `"Follows the Studio membership rule. Ends when membership does."`
Caption: "Granted above automatic, with the section counts as headers. Only the direct grant carries a remove button; the other two explain in place why they have none — the same sentences the desktop popovers carry."

### M06b — "Person · withheld and cards"
Structure: status bar → header (`padding:8px 16px 0`) → **tab strip**
(`padding:12px 16px 0`, `gap:22px`) → body `padding:14px 16px 16px`, `gap:12px`:
withheld card first, then normal cards.
- Withheld card: `border:1px solid rgba(245,165,36,.28)`; label row
  (`7px` amber dot + `"Withheld"` 11px/.1em/uppercase `#f5a524`); resource
  `16px/600`; explanation `13.5px/1.55` `.7`; key–value; then a `min-height:44px`
  link row.
Copy: `"People · Studio"`, `"Devan Suresh"`, tabs `"Access"` / `"Holds 1"` / `"Cards 2"` / `"History"`, `"Withheld"`, `"CNC router"`, `"Fabrication Access still grants this. A hold placed by K. Rao on 09 Aug takes it away until the refresher induction is done."`, `"Hold ends"` → `"When lifted by hand"`, `"Open the hold"`, `"Laser cutter"` / `"operate"` / bundle `"Fabrication Access"`, `"Studio shares"` / `"read, write"` / mapping, `"On fileserver-01. Account adopted 14 Feb."`
Caption: "Tabs replace the desktop's side panels — four fit at 390px without scrolling, with counts inline. A withheld row leads its section: it is the one thing on the page that contradicts what a role says."

---

## M07 · Role members (E2)

Section: "Forty rows, so A2."

### M07a — "Role · members"
Structure: status bar → header (back-line `"Roles · Fabrication"`, title,
answer sentence `13.5px`/`.62`) → list with the first row disclosed → "N more" row.
- Disclosed header row `min-height:60px` + `background:rgba(255,255,255,.035)`,
  chevron rotated. Body `padding:4px 16px 14px`, `gap:9px`, key–value pairs,
  then a `min-height:44px` link inside a `padding-top:4px` row.
- Collapsed member row `min-height:60px`; name `15px/600` then the capsule as
  the whole second line.
- Overflow row: `min-height:52px`, `padding:0 16px`, `13.5px`/`.42`.
Copy: `"Roles · Fabrication"`, `"Laser cutter · operate"`, `"23 people can currently use this"`, `"Aditi Rao"`, `"Granted by"` → `"K. Rao · 14 Feb"`, `"Until"` → `"30 May 2026"`, `"Open Aditi's access"`, `"Devan Suresh"`, `"Priya Menon"`, `"Meera Nair"`, `"19 more"`.
Caption: "The name is the primary line and the capsule is the whole second line — at this width the source is the only other thing worth a row. The header sentence answers the question in words before the list does."

### M07b — "Expiring · from Today"
Structure: status bar → header (back-line `"Today"`, title, sentence) →
date-grouped list (imminent group header amber) → pinned action bar (`Select` +
`Extend all 11`).
Copy: `"Today"`, `"Expiring this week"`, `"11 grants across 3 roles"`, `"Friday 15 Aug · 6"`, `"Aditi Rao"` / `"Studio Access · enter"`, `"Meera Nair"` / `"Studio Access · enter"`, `"Sunday 17 Aug · 5"`, `"Priya Menon"` / `"Laser cutter · operate"`, `"Select"`, `"Extend all 11"`.
Caption: "Grouped by date, nearest first, with the imminent group in amber. *Extend all* is a bulk change, so it opens a rung-2 sheet; *Select* switches the list into the explicit selection mode from M29."
> The caption references **M29**, which is not on this board. Selection mode is drawn in M25b.

---

## M08 · Roles and apps (E1)

### M08a — "Roles index"
Structure: status bar → header: title `"Roles"` + **three-way segmented control**
(`gap:8px`, `padding:4px`) → grouped list of `min-height:64px` rows.
**[CONTRADICTION: 64px rows; 38px segments]**
Copy: `"Roles"`, segments `"Roles 14"` (on) / `"Bundles 4"` / `"Apps 3"`; `"Fabrication · 6"`, `"Laser cutter · operate"` / `"23 people · 1 mapping"`, `"CNC router · operate"` / `"11 people · 1 hold"`, `"Studio · 5"`, `"Studio Access · enter"` / `"38 people · 2 mappings"`, `"Studio shares · read, write"` / `"38 people · fileserver-01"`.
Caption: "Roles, bundles and apps are a three-way segment rather than three destinations — they belong to one question. Second lines carry population and reach, the two facts that make a role worth opening."

### M08b — "App · token"
Structure: status bar → header (back-line `"Roles · Apps"`, title
`"booking-web"`, lime dot + sentence) → body `padding:14px 16px`, `gap:12px`:
Receives card (mono `13px` list, `gap:9px`) → Token lifetime card (key–value +
prose) → link row `min-height:44px`.
Copy: `"Roles · Apps"`, `"booking-web"`, `"Last token issued 2 minutes ago"`, `"Receives"`: `"laser.operate"`, `"studio.enter"`, `"cnc.operate"`; `"Token lifetime"`, `"Expires after"` → `"15 minutes"`, `"A change to a role reaches this app when the next token is issued — up to 15 minutes."`, `"Debug what this app sees"`.
Caption: "An app's page answers what it receives and when a change reaches it. The lifetime sentence is the one operators actually need, so it is prose rather than a field."

---

## M09 · Giving access (E4 + E5)

Section: "The consequence, not the form." The sheet grows to fit the cascade;
the confirm button stays pinned.

### M09a — "Pick what to give"
Sheet stops **96px** short of the top (`top:96px`), so the person stays visible.
Structure: dimmed page (`.2`) → backdrop `.74` → sheet, which is itself a
three-part column:
1. Head `padding:12px 18px 12px`, `gap:12px`, bottom border `.06`: grabber →
   sheet title `19px` Bricolage 600 → search field.
2. Scroll body `flex:1; overflow:hidden`: eyebrow group headers
   `padding:11px 18px 6px`; picker rows **`min-height:56px`**, `padding:10px 18px`,
   `gap:12px`, checkbox + two-line label.
3. Footer: `border-top:1px solid rgba(255,255,255,.08)`,
   `padding:12px 18px 24px`, `background:rgba(11,12,10,.6)`, `gap:10px`:
   expansion sentence `13px`/`.62` → `height:50px` accent button.
Copy: `"Give access to Aditi Rao"`, `"Search roles and bundles"`, `"Bundles"`, `"Workshop basics"` / `"4 roles"` (checked), `"Fabrication Access"` / `"6 roles · 1 mapping"`, `"Roles"`, `"CNC router · operate"` / `"Fabrication"`, `"1 bundle selected · expands to 4 roles"`, `"Preview what changes"`.
Caption: "The sheet stops 96px short of the top, so the person you are acting on stays visible behind it. The footer names the expansion before the operator commits to a preview."

### M09b — "Cascade preview"
Sheet `top:60px`, backdrop `.78`, page `.16`.
Structure: head (grabber, title, subtitle) → grouped preview rows → footer with
`Back` + `Give access`.
- Preview row `min-height:52px`, `padding:9px 18px`, label `14.5px` + capsule.
- Rule-caused group header is **amber**: `color:#f5a524`,
  `background:rgba(245,165,36,.06)`.
- Footer: `Back` `min-height:50px` outline `padding:0 18px`; primary `flex:1`
  `height:50px`.
Copy: `"This gives Aditi 6 things"`, `"Four from the bundle, two more because a rule follows one of them."`, `"From the bundle · 4"`, `"3D printers · operate"`, `"Hand tools · operate"`, `"2 more"`, `"Because a rule follows · 2"`, `"Studio shares · read, write"`, `"An account on fileserver-01"`, `"Back"`, `"Give access"`.
Caption: "Rule-caused grants are a separate, amber-headed group — the operator did not ask for them. The title counts the total in words, because six is the number they will be held to."

---

## M10 · Token debug (E6)

### M10a — "Gap explained"
Explanation first, evidence second (inverted from desktop).
Structure: status bar → header (back-line `"booking-web"`, title
`"What this app sees"`, subject line `"For Aditi Rao"`) → body `padding:14px 16px`,
`gap:12px`: verdict card (amber border `rgba(245,165,36,.3)`) → "Syndra holds"
list card → "Last token" list card.
- Verdict card: dot row (`7px` amber + `"One role missing"` 11px uppercase amber),
  then a `14.5px/1.55` sentence with the claim inline in mono `13.5px`.
- List rows **`min-height:46px`**, `padding:8px 16px`, `border-top:.05`, mono `13px`.
  Missing row `background:rgba(245,165,36,.07)`, text `#f5a524`, trailing
  `"not in token"` `12px`. **[CONTRADICTION: 46px rows carrying a `Copy` affordance]**
Copy: `"booking-web"`, `"What this app sees"`, `"For Aditi Rao"`, `"One role missing"`, `"cnc.operate was granted 4 minutes ago. The app's token was issued 11 minutes ago and lasts 15, so it will appear within 4 minutes."`, `"Syndra holds · 3"`, `"laser.operate"`, `"studio.enter"`, `"cnc.operate"` + `"not in token"`, `"Last token · 2"`, `"tok_4c9e2f10"` + `"Copy"`, `"Issued"` → `"09:30:14"`.
Caption: "The explanation comes first and the evidence follows — inverted from desktop, where the two lists sit side by side and the sentence sits beneath. On a phone the operator wants the answer before the proof."

### M10b — "Agreed"
Healthy case collapses two lists into one.
Structure: same header; verdict card with neutral border and a lime dot; one
`"Both sides · 2"` card whose rows carry a trailing `6px` lime dot; closing note.
Copy: `"For Meera Nair"`, `"Token matches"`, `"The app is seeing everything Syndra holds. If a member reports otherwise, the problem is inside the app."`, `"Both sides · 2"`, `"studio.enter"`, `"studio.shares.read"`, `"Lime is a dot and a word here, never a fill. Nothing on this screen needs doing."`
Caption: "The healthy case collapses the two lists into one, because there is no difference to show. It ends by telling the operator where to look next — the screen's job is to end the conversation."

---

## M11 · Requests (E7)

Section: "Rung 1, under the thumb." Approving is reversible — no dialog.

### M11a — "Operator · one request"
Structure: status bar → header (back-line `"Requests · 4"`, title, asker line) →
body `gap:12px`: "Why they asked" card → "Checks" card → link row → **pinned
action bar** (`Decline` outline `padding:0 20px`, `15px/600`; `Approve` accent
`flex:1` with `box-shadow:0 10px 26px -14px rgba(127,90,240,.85)`).
- Checks: `7px` dot + `14px` sentence per line; the neutral check uses
  `border:1px solid rgba(243,245,239,.4)` and no fill.
Copy: `"Requests · 4"`, `"Laser cutter · operate"`, `"Asked by Aditi Rao, 2 days ago"`, `"Why they asked"`, `"Cutting acrylic for the exhibition build. Induction with Kabir on 11 Feb."`, `"Checks"`, `"Induction recorded 11 Feb"`, `"No hold on this person"`, `"No prior access to this role"`, `"Open Aditi's access"`, `"Decline"`, `"Approve"`.
Caption: "Checks are dots and words, with the neutral one carrying an outline rather than a colour. Decline sits away from the thumb's resting arc; approve is the wide target."

### M11b — "Member · asking"
Structure: status bar → header (back-line `"My access"`, title) → body `gap:12px`:
state card (violet `7px` dot + `"Waiting"` eyebrow `#c9b6ff`) → "What you wrote"
card → dashed disabled card.
Copy: `"My access"`, `"Laser cutter"`, `"Waiting"`, `"You asked 2 days ago"`, `"Kabir Rao looks after this machine and usually answers within three working days."`, `"What you wrote"`, `"Cutting acrylic for the exhibition build. Induction with Kabir on 11 Feb."`, `"Ask again"`, `"Available once a request is 5 days old. This one is 2 days old."`
Caption: "The member sees who is deciding and roughly when. The disabled nudge states the rule and the current count rather than greying out silently — the same disabled-reason rule the operator side obeys."

---

## M12 · List states (four, no exceptions)

Figures are drawn at **298px** with `radius:26px`, status bar `height:30px`,
`10.5px` mono, page padding `14px` — board scaling, not product values.

### M12a — "Loading"
Title placeholder `height:24px; width:56%; radius:7px; rgba(255,255,255,.07)`,
then three rows at `min-height:60px` (the real row height) with two bars each
(`height:12px`/`10px`, `radius:5px`, `.07`/`.045`), `gap:8px`, `gap:1px` between rows.
**Static — no shimmer.**
Caption: "Three placeholder rows at the real row height. Static — no shimmer, which on a phone reads as activity that isn't happening."

### M12b — "Empty"
Title `"Requests"` (21px Bricolage 600) → centred block `gap:9px`,
`padding:14px 2px`: `"Nothing waiting"` `14.5px/600`; `"When a member asks for access to something you look after, it appears here."` `13.5px/1.55` `.62`.
Caption: "Says what would appear and who causes it. No illustration, no button to nowhere."

### M12c — "Error"
Title `"People"` → error card (`radius:16px`, `border:1px solid rgba(255,92,77,.3)`,
`padding:14px`, `gap:9px`): headline `"Could not load people"` `14.5px/600` `#ff8d82`;
endpoint `"GET /api/v1/users → 502"` mono `12px/1.5` `.62` `word-break:break-all`;
retry control `min-height:44px`, `radius:999px`, outline, `14px/600`, `"Try again"`.
Caption: "Names the endpoint and the status, as a copy row. Operators forward this to whoever runs the backend."
> **[CONTRADICTION]** The caption calls the endpoint a copy row; the drawing gives it no `min-height`, no `Copy` affordance, and `12px` type.

### M12d — "Degraded"
Frame border goes `rgba(245,165,36,.4)` **for the full frame**.
Structure: status bar → banner pinned directly under it
(`background:rgba(245,165,36,.12)`, top+bottom borders `rgba(245,165,36,.3)`,
`padding:11px 14px`, `gap:5px`): `"Demo data"` `13px/700` `#f5a524`; body
`12.5px/1.5` `.82` → dimmed list (`opacity:.55`, rows `min-height:56px`).
Title dimmed to `.62`. Every action inert. Banner **cannot be dismissed**.
Copy: `"Demo data"`, `"The backend could not reach its database. Nothing on this screen is real and no action will take effect."`, `"People"`, `"Sample Person"` / `"2 roles"`, `"Sample Person"` / `"1 role"`.
Caption: "The banner is pinned under the status bar and cannot be dismissed. Rows are dimmed and every action on the screen is inert — the amber border runs the full frame so the state is legible at arm's length."

---

## M13 · Access map (S5)

Section: a graph at 390px is unreadable, so it becomes centre / in-edges / out-edges.
**Tapping any row re-centres the map on it. No pinch-zoom, no panning.**

### M13a — "One node, both directions" (dark)
Structure: status bar → header (back-line `"System · Access map"`, title `24px`,
degree sentence) → body `padding:14px 16px`, `gap:14px`:
`"Points at it"` eyebrow + card (`radius:16px`) of `min-height:52px` rows
(`padding:9px 14px`, label `14.5px` + kind word `12.5px`/`.42`) →
**centre marker** (row `gap:12px`, `padding:2px 4px`: hairline
`rgba(155,123,255,.3)` — `10px` dot `#7f5af0` with
`box-shadow:0 0 14px 2px rgba(155,123,255,.5)` — hairline) →
`"It reaches"` eyebrow + card.
Copy: `"System · Access map"`, `"Studio Access · enter"`, `"4 things point at it · it reaches 3"`, `"Points at it"`: `"Workshop basics"`/`"bundle"`, `"Studio membership"`/`"rule"`, `"2 direct grants"`/`"people"`; `"It reaches"`: `"Studio door"`/`"unifi"`, `"fileserver-01 · studio"`/`"truenas"`, `"38 people"`/`"members"`.
Caption: "The centre is the title, so the graph's one node needs no drawing. Tapping any row re-centres the map on it — the same navigation the desktop graph offers, without pinch-zoom."

### M13b — "Light · same geometry"
The board's **only light-theme figure**. Identical geometry and spacing.
Light palette: frame `border:1px solid rgba(0,0,0,.12)`, page `#f7f7f4`,
card `#fff` with `border:1px solid rgba(20,22,18,.1)`, row divider
`rgba(20,22,18,.07)`, header border `rgba(20,22,18,.1)`, text `#141612`,
secondary `rgba(20,22,18,.62)`, muted `rgba(20,22,18,.45)`/`.42`/`.55`,
accent **`#5b3fd6`**, connector `rgba(91,63,214,.28)`, centre dot `#5b3fd6`
**with the glow removed**.
Caption: "Dark is home; light is daylight, for a phone held outdoors. Same geometry, same spacing, accent darkened to `#5b3fd6` and the dot's glow dropped — a bloom on white reads as a smudge."

---

## M14 · Drift and reconcile (S6 + S7)

### M14a — "Drift list"
Structure: status bar → header (back-line `"Review"`, title `"Drift · 4"`,
freshness strip) → target-grouped list → pinned action bar.
- Drift row is **not** a fixed-height row: `padding:12px 16px`, `gap:8px`,
  column. Line 1: account `15px/600` + kind word `12px` (`#f5a524` for a real
  difference, `.5` for "unknown"). Line 2: sentence `13.5px/1.5` `.62` with both
  sides in `<strong style="color:#f3f5ef;font-weight:600">`.
Copy: `"Review"`, `"Drift · 4"`, `"Read 2 minutes ago"`, `"Refresh"`, `"fileserver-01 · 3"`, `"aditi.rao"` + `"extra on target"` / `"Syndra intends read. The target reports read, write."`, `"devan.suresh"` + `"missing on target"` / `"Syndra intends read, write. The target has no account."`, `"legacy.build"` + `"unknown to Syndra"` / `"Nobody in Syndra claims this account. Adopt it or leave it alone."`, `"Select"`, `"Rehearse fixing 3"`.
Caption: "Each row states the difference as a sentence with both sides bolded, so no column headers are needed. The kind of drift is a word at the end of the first line, not a coloured badge."

### M14b — "Reconcile · draining"
Sheet `top:120px`, backdrop `.78`, page `.18`.
Structure: head (grabber, `"Applying 3 fixes"` 19px, progress bar at `66%`,
mono `12px` counter) → per-item rows `min-height:58px`, `padding:10px 18px`,
`gap:11px` (state dot + two lines) → footer with a single outline control.
- Done row: lime dot. Failed row: `background:rgba(255,92,77,.07)`, `#ff5c4d`
  dot, second line `#ff8d82` `12.5px/1.45`. Waiting row: hollow dot
  (`border:1px solid rgba(243,245,239,.3)`), text dimmed.
Copy: `"Drift · 4"` (behind), `"Applying 3 fixes"`, `"2 of 3 · 1 failed"`, `"aditi.rao"` / `"write removed"`, `"devan.suresh"` / `"Target refused: quota exhausted on pool studio"`, `"legacy.build"` / `"waiting"`, `"Stop after this one"`.
Caption: "Drains inline and reports what happened, per row. The failure states the target's own words and stays in the list rather than interrupting; the operator can stop the run without dismissing anything."

---

## M15 · Bundles (S1 + S2b)

### M15a — "Bundle · contents"
Structure: status bar → header (back-line `"Roles · Bundles"`, title,
population line) → list of `min-height:60px` rows each ending in a **named text
control** `"Remove"` `13px` `#ff8d82` (never an icon; the control is 44px tall)
→ pinned bar with one outline `"Add a role"` (`min-height:50px`).
Copy: `"Roles · Bundles"`, `"Workshop basics"`, `"Held by 12 people · 4 roles"`, `"3D printers · operate"` / `"Fabrication"` / `"Remove"`, `"Hand tools · operate"` / `"Fabrication"` / `"Remove"`, `"Studio Access · enter"` / `"Studio · reaches 2 targets"` / `"Remove"`, `"Laser cutter · operate"` / `"Fabrication"` / `"Remove"`, `"Add a role"`.
Caption: "Remove is a text control at the row's end, not an icon — 44px tall and named. The header's population count is what makes every removal on this page a rung-2 action."

### M15b — "Rung 2 sheet"
Structure: dimmed page (`.2`) → backdrop `.76` → sheet (`padding:12px 18px 24px`,
`gap:13px`): grabber → title `19px` → consequence paragraph `14px/1.55` `.66` →
two acknowledgement rows → gated button.
- Acknowledgement row is a `<label>`: `min-height:44px`, `padding:11px 12px`,
  `radius:14px`, `background:rgba(255,255,255,.035)`, `gap:12px`,
  `align-items:flex-start`; `22px` checkbox `margin-top:1px`; text `14px/1.45` `.82`.
  **The whole row is the tap target, not the box.**
- Gated button: `height:50px`, `radius:999px`,
  `border:1px solid rgba(255,92,77,.4)`, `background:rgba(255,92,77,.14)`,
  `color:rgba(255,141,130,.55)`, `15px/600`. Red stays outline+tint until both
  boxes are ticked; only then does the fill go solid.
- **The button states its own gate in its label.**
Copy: `"Remove Studio Access from this bundle"`, `"Twelve people hold this bundle. Two of them keep Studio Access from a direct grant. Ten lose it, along with the two targets it reaches."`, `"Ten people lose Studio Access today"`, `"Their door access and file shares go with it"`, `"Remove · 1 of 2 acknowledged"`.
Caption: "The whole row is the tap target, not the box. Red stays an outline and a tint until both boxes are ticked; only then does the fill go solid — the same rule as desktop, on a bigger target."

---

## M16 · Causal chain (S2 + S3 + S4)

Every screen in the thread carries a back-line naming the origin **and the
cascade id as a link** (`csc_2f81b0`, mono `12.5px`, `#9b7bff`), separated by a
`13px` `·` at `rgba(243,245,239,.3)`.

### M16a — "Rule"
Structure: status bar → header (back-line `"Automation · Rules"`, title `24px`,
lime dot + state line) → body `gap:12px`: When/Then card (one card, one `1px`
divider — "because they are one sentence") → stats card → pinned action bar
(`Pause` destructive outline `border:1px solid rgba(255,92,77,.4)` `#ff8d82`,
away from the thumb; `Edit the rule` wider benign outline `flex:1`).
Copy: `"Automation · Rules"`, `"Studio membership"`, `"Active · last fired 4 minutes ago"`, `"When"` / `"Somebody is given Studio Access · enter"`, `"Then"` / `"Give them Studio shares · read, write and create an account on fileserver-01"`, `"Fired this month"` → `"14 times"`, `"Queued now"` → `"2 writes"`, `"Pause"`, `"Edit the rule"`.
Caption: "When and Then are one card with a divider, because they are one sentence. Pause is destructive and sits away from the thumb; editing is the wider, benign target."

### M16b — "Pending · back-line"
Structure: status bar → header (back-line + cascade id, title `"Queued writes · 2"`)
→ write rows (`padding:12px 16px`, `gap:8px`: violet `7px` dot + label `15px/600`;
detail line indented `padding-left:17px`) → footnote + link row `min-height:44px`.
Copy: `"Studio membership"` · `"csc_2f81b0"`, `"Queued writes · 2"`, `"Create account · aditi.rao"` / `"On fileserver-01. Queued 4 minutes ago, next attempt in 2."`, `"Grant read, write · studio"` / `"Waits for the account above."`, `"Writes queue rather than fail — the target is reachable but slow. Nothing here needs a decision."`, `"What this rule has done · history"`.
Caption: "The back-line names the rule and carries the cascade id as a link, so the thread survives four taps deep. Dependent writes say what they are waiting for instead of showing a spinner."

### M16c — "History · one cascade"
A timeline spine. Each entry: row `gap:14px`; left `width:9px` column with a
`9px` dot and a `1px` `rgba(255,255,255,.12)` connector; right column
`padding-bottom:18px`, headline `14.5px/600`, detail `13px`/`.5` `margin-top:3px`.
- Dot vocabulary: **filled `#7f5af0` = an operator act; filled `#a3e635` = done;
  ringed `1.5px solid rgba(155,123,255,.7)` = a rule firing; hollow
  `1.5px solid rgba(243,245,239,.3)` = not yet** (last entry drops the connector).
Copy: `"Queued writes"` · `"csc_2f81b0"`, `"What happened"`, `"K. Rao gave Studio Access"` / `"to Aditi Rao · 09:26"`, `"Studio membership fired"` / `"queued 2 writes · 09:26"`, `"Account created"` / `"aditi.rao on fileserver-01 · 09:28"`, `"Grant read, write"` / `"still queued"`.
Caption: "The chain becomes a spine with one dot per event: filled for done, ringed for a rule firing, hollow for not yet. The operator can read cause and effect top to bottom without a table."

---

## M17 · Forensic floor (S8–S11)

### M17a — "History · everything"
Structure: status bar → header: title `"History"` + row (`gap:8px`) of search
field + **time-window control** (`min-height:44px`, `radius:14px`, outline,
`padding:0 14px`, `13.5px`, label `"7 days"`) → date-grouped list of
`min-height:64px` rows (`padding:11px 16px`, column `gap:4px`; sentence
`14.5px/600`, consequence `13px`/`.5`). **[CONTRADICTION: 64px]**
Copy: `"History"`, `"Search a person or id"`, `"7 days"`, `"Today"`, `"K. Rao gave Studio Access to Aditi Rao"` / `"09:26 · caused 2 writes"`, `"Hold placed on Devan Suresh"` / `"08:52 · CNC router · by K. Rao"`, `"3 accounts adopted on fileserver-01"` / `"08:14 · by M. Iyer"`, `"Yesterday"`, `"Card 04 revoked · Priya Menon"` / `"17:40 · reported lost"`.
Caption: "One sentence per event, in the product's own words, with the consequence on the second line. A time window replaces the desktop filter bar; everything else is search."

### M17b — "Provider health"
Structure: status bar → header (back-line `"System"`, title `"Providers"`,
freshness strip) → body `gap:11px` of provider cards (`radius:16px`,
`padding:14px 16px`, `8px` dot + two lines + `›`).
Degraded card: `border:1px solid rgba(245,165,36,.3)`, amber dot, second line
`#f5a524`. **No gauges, no sparklines — a measured round trip stated as a word.**
Copy: `"System"`, `"Providers"`, `"Read 30 seconds ago"`, `"Refresh"`, `"fileserver-01"` / `"truenas · answered in 180ms"`, `"fileserver-02"` / `"truenas · answered in 240ms"`, `"studio-door"` / `"unifi · slow, 4.2s"`, `"directory"` / `"ldap · answered in 90ms"`.
Caption: "A dot, the kind, and the measured latency. Nothing is a gauge or a sparkline: the operator wants to know which one to stop trusting, and that is a word."

---

## M18 · Add-on platform navigation

Section: the add-on platform adds a destination to Review, to System, and to the
member's tabs, and removes nothing from Basic. **The tab bar's four-item limit is
why Storage is a member tab and not an operator one.**

### M18a — "Advanced sheet · with add-ons"
Same nav sheet as M01b (`#101210`, `padding:12px 14px 24px`) but `gap:2px` and
**a second level**: add-on destinations indent under their parent rather than
becoming new top-level rows. Only the current section is expanded.
- Top-level rows **`min-height:42px`**, `14.5px`.
- Indented child rows **`min-height:40px`**, `padding:0 14px 0 28px`, `14px`, `.7`.
  **[CONTRADICTION: 42px and 40px are below the 44px minimum, and disagree with
  M01b's 44px/15px for the same component]**
Copy: `"Today"`, `"People"`, `"Requests"` `4`, `"Roles"`, `"Review"` (active), `"Drift"` `4`, `"Withdrawn access"` `3`, `"Dormant accounts"`, `"System"`, `"Targets"`, `"Automation"`, `"History"`.
Caption: "Add-on destinations indent under their parent rather than becoming new top-level rows — the sheet can afford a second level where a tab bar cannot. Only the current section is expanded."

### M18b — "Member · three tabs"
Tab bar becomes `grid repeat(3,1fr)`, `padding:8px 20px 22px`, `gap:4px`.
Copy: `"My access"`, `"Laser cutter"` / `"You can use this."`, `"Studio door"` / `"Card 07 opens it. Tap in at the reader."`, `"CNC router"` / `"Withheld until your refresher induction."` (amber card border `rgba(245,165,36,.28)`, text `#f5a524`), tabs `"My access"` / `"Storage"` / `"Requests"`.
Caption: "TrueNAS earns a tab because there is something to look at and a password to set. Unifi does not — a door is a fact on the access list, not a place. Withheld access reads as a sentence about the member, never as a badge."

---

## M19 · Member storage

Figures drawn at **340px** with `radius:28px`, status bar `height:32px`/`10.5px`,
page padding `15px`, card radius `16px`, `gap:11px` — board scaling.
Section: "A two-state page lies to the member in the middle."

### M19a — "Ready"
Structure: status bar → header (title `"Storage"` `24px` + count line `13px`) →
two folder cards + a password card.
- Folder card: lime `7px` dot + `"Ready"` eyebrow (`rgba(243,245,239,.55)`),
  folder name `15px/600`, permission sentence `13px`/`.62`, then a **connection
  copy row**: `min-height:48px`, `radius:12px`, `background:#0b0c0a`,
  `border:1px solid rgba(255,255,255,.07)`, `padding:0 12px`, mono `12px`,
  trailing `"Copy"` `11.5px`. **[CONTRADICTION: 48px copy row, mono 12px, `Copy` at 11.5px]**
- Password card: eyebrow + prose + `min-height:48px` outline button.
Copy: `"Storage"`, `"Two folders on fileserver-01"`, `"Ready"`, `"studio"`, `"You can read and write here"`, `"smb://fileserver-01/studio"`, `"Copy"`, `"exhibition-2026"`, `"You can read here"`, `"Your password"`, `"Set 4 days ago. Only you can change it."`, `"Set a new password"`.
Caption: "Permissions in the member's words — 'read and write', not `rw`. The path is a copy row directly under the folder it belongs to."

### M19b — "On its way"
The middle state — **violet, not amber; this is normal, not a deadline.**
Card border `rgba(155,123,255,.3)`, violet dot, eyebrow `#c9b6ff`.
Second card is the dashed disabled pattern.
Copy: `"One folder being set up"`, `"Being set up"`, `"studio"`, `"You have been given access. Your account on the file server is still being created — this usually takes a few minutes. Nothing for you to do."`, `"Set a password"`, `"Available once your account exists."`
Caption: "Violet, not amber — this is normal, not a deadline. 'Nothing for you to do' is the sentence that stops the support message being written."

### M19c — "Withheld"
Card border `rgba(245,165,36,.3)`, amber dot + `"Withheld"` eyebrow `#f5a524`,
explanation `13.5px/1.55` `.75`, then a `min-height:48px` outline action.
Copy: `"One folder on hold"`, `"Withheld"`, `"studio"`, `"Your role includes this folder, but access is being withheld until your refresher induction. Kabir Rao placed the hold on 9 August."`, `"Message Kabir Rao"`, `"The member sees 'withheld'. The operator sees the same object as a 'hold'. One record, two words, chosen for who is reading."`
Caption: "Names the person and the date, and offers the one action that resolves it. A withheld state without a named human is a dead end."

---

## M20 · Target (Advanced › System › target)

Four tabs in a fixed order — is it answering, whose accounts, what it can do,
what state it is in — with the freshness strip **above** them because it governs
all four. Tabs scroll horizontally (`white-space:nowrap`, `gap:20px`).

### M20a — "Target · overview"
Structure: status bar → header `padding:8px 16px 0` (back-line, title, freshness
strip) → tab strip `padding:4px 16px 0` → body `gap:12px`: Answering card
(key–values) → "What it can do" card (dot + sentence lines) → footnote.
- Unsupported capability: dot is `border:1px dashed rgba(243,245,239,.5)`,
  **dashed rather than red — absent is not broken.**
Copy: `"System · Targets"`, `"fileserver-01"`, `"Read 40 seconds ago"`, `"Refresh"`, tabs `"Health"` / `"Accounts 41"` / `"Can do"` / `"Drift 3"`, `"Answering"`, `"Round trip"` → `"180ms"`, `"Version"` → `"TrueNAS 24.10"`, `"Credential"` → `"Rotates in 18 days"`, `"What it can do"`, `"Create and delete accounts"`, `"Set share permissions"`, `"Close open sessions · not supported"`, `"The unsupported capability is stated here so it is never a surprise inside a revocation dialog."`
Caption: "Four tabs scroll horizontally with counts inline; the freshness strip sits above them because it governs every tab. Capabilities are dots and words, with the missing one dashed rather than red — absent is not broken."

### M20b — "Accounts · adoption"
Same header + tab strip (Accounts active) → grouped list, **unclaimed above
claimed** → pinned action bar.
- Account row `min-height:60px`; identifier mono `13.5px`; unclaimed rows end in
  a named text control `"Adopt"` `13px` `#9b7bff` 600; claimed rows end in `›`.
Copy: `"Unclaimed · 3"`, `"legacy.build"` / `"Last login 8 months ago"` / `"Adopt"`, `"r.krishnan"` / `"Last login 3 days ago"` / `"Adopt"`, `"Claimed · 38"`, `"aditi.rao"` / `"Aditi Rao · adopted 14 Feb"`, `"meera.nair"` / `"Meera Nair · adopted 02 Jan"`, `"Select"`, `"Adopt 3 accounts"`.
Caption: "Unclaimed sits above claimed, because that is the work. Adoption binds identity, so this whole screen is unavailable when the read passes ten minutes — see M02b."

---

## M21 · Withdrawn access (Review)

Two populations, never one count.

### M21a — "Withdrawn · two groups"
Structure: status bar → header (back-line `"Review"`, title, sentence) → two
groups, **each header carrying its own explanatory subtitle**
(`padding:12px 16px 8px`, `gap:4px`: eyebrow + `12.5px` subtitle) → rows.
- Group 1 header tint `rgba(255,255,255,.02)` (neutral — expected behaviour).
- Group 2 header tint `rgba(245,165,36,.07)`, eyebrow `#f5a524`.
- Group 1 rows `min-height:64px`, two lines. **[CONTRADICTION: 64px]**
- Group 2 entry is a block (`padding:12px 16px`, `gap:8px`) ending in a
  `min-height:44px` link.
Copy: `"Review"`, `"Withdrawn access"`, `"Three revocations Syndra could not complete on the target"`, `"Will end by itself · 2"` / `"Open sessions. They end when the person reconnects."`, `"Priya Menon · studio"` / `"Session open since 08:10 · fileserver-01"`, `"Devan Suresh · exhibition-2026"` / `"Session open since 09:02 · fileserver-01"`, `"Will not end by itself · 1"` / `"The target refused the revocation. This needs a decision."`, `"r.krishnan · studio"`, `"Target refused: account is the owner of 41 files. Reassign ownership or delete the account."`, `"Open on fileserver-01"`.
Caption: "Each group's header explains itself in a subtitle, so the distinction survives without the desktop's two panels. Only the second group carries amber — the first is expected behaviour."

### M21b — "Revoke on a target · rung 3"
The type-to-confirm sheet. **Docked to the top of the keyboard**
(`bottom:186px`), so it is the only sheet not anchored to the screen bottom:
`border:1px solid rgba(255,255,255,.1)` on all sides with `border-bottom:none`,
`radius:24px 24px 0 0`, `padding:12px 18px 18px`, `gap:12px`.
Structure: grabber → title `19px` → consequence paragraph `13.5px/1.55` `.7` →
labelled input (`12.5px` label with the target string inline in mono `13px`
`#f3f5ef`, `margin-bottom:7px`; field `height:50px`, `radius:14px`,
`border:1px solid rgba(155,123,255,.5)`, `background:#0b0c0a`, `padding:0 14px`,
mono `15px`, caret `1.5px × 20px` `#9b7bff`) → gated destructive button
(`height:50px`, `background:rgba(255,92,77,.3)`, `color:rgba(247,244,255,.6)`).
Below: keyboard `height:186px`, `#1c1d19`.
Copy: `"Take away studio from r.krishnan"`, `"This target cannot close an open session. If they are connected now, they keep working until they reconnect — Syndra cannot stop that."`, `"Type r.krishnan to confirm"`, field content `"r.krishna"` (mid-typing), `"Take it away"`.
Caption: "The sheet docks to the top of the keyboard so the sentence, the field and the button are visible together — the button never hides behind the keys. The name is monospace and selectable but not tap-copyable; typing it is the gesture."

---

## M22 · Plan, then apply

### M22a — "Rehearsal · read the plan"
**Not a bottom sheet.** A full-height surface: `position:absolute;left:0;right:0;
top:34px;bottom:0`, `background:#191a22`, **no border-radius, no grabber**, with a
`"Close"` text control instead. **[CONTRADICTION: the "everything becomes a sheet"
rule has an undocumented fourth form]**
Structure:
1. Head `padding:14px 18px 12px`, `gap:9px`, bottom border: row (`"Rehearsal"`
   `13.5px`/`.5` + `"Close"` `14px`/`.62`) → title `21px` Bricolage 600 → subtitle.
2. Body `flex:1; overflow:hidden`: four groups, each eyebrow header
   `padding:11px 18px 6px`, entries `padding:10px 18px`, `gap:5px`
   (`14.5px/600` name + `13px/1.5` `.62` sentence, changed values in `<strong>`
   `#f3f5ef`). The "Cannot be done" header is amber-tinted.
3. Sticky footer `border-top`, `padding:12px 18px 24px`,
   `background:rgba(11,12,10,.66)`, `gap:10px`: counts row (`13px`/`.62`
   + plan age with a `6px` lime dot at `12.5px`/`.5`) → `height:50px` accent button.
- **A plan older than five minutes replaces `Apply` with `Rehearse again`.**
Copy: `"Rehearsal"`, `"Close"`, `"Fixing 3 differences"`, `"Nothing has happened yet. This is what will happen if you apply."`, `"Will change · 2"`, `"aditi.rao"` / `"Remove write on studio"`, `"devan.suresh"` / `"Create account, then grant read, write"`, `"Will be left alone · 1"`, `"legacy.build"` / `"Unknown to Syndra. Adopting it is a separate decision."`, `"Cannot be done · 1"`, `"r.krishnan"` / `"Owns 41 files. The target will refuse until ownership moves."`, `"2 changes · 1 skipped · 1 will fail"`, `"planned 30s ago"`, `"Apply this plan"`.
Caption: "Full-height, four honest groups, and a sticky footer that never leaves the counts and the plan's age off-screen. A plan older than five minutes replaces *Apply* with *Rehearse again*."

### M22b — "Mappings · versions"
Structure: status bar → header (back-line `"fileserver-01 · Mappings"`, title
`22px`, blast-radius sentence) → body `gap:12px`: live-version card
(`border:1px solid rgba(155,123,255,.28)`; header row `"Live · v4"` `#c9b6ff`
eyebrow + mono `12px` date; statement `14.5px/1.55`; author `13px`/`.5`) →
earlier-versions card (`radius:18px`, `overflow:hidden`; rows `min-height:56px`,
`padding:9px 16px`, `border-top:.05`) → footnote → pinned action bar
(`Remove` **destructive outline** `border:1px solid rgba(255,92,77,.4)` `#ff8d82`
`padding:0 18px`; `Edit the mapping` benign `flex:1` outline).
Copy: `"fileserver-01 · Mappings"`, `"Studio shares → studio"`, `"38 people are affected by changing this"`, `"Live · v4"`, `"since 02 Aug"`, `"Grants read, write on the studio dataset"`, `"Changed by M. Iyer"`, `"Earlier versions"`, `"v3 · read only"` / `"14 Jun – 02 Aug · K. Rao"`, `"v2 · read only, no delete"` / `"02 Jan – 14 Jun · K. Rao"`, `"Versions are a record, not a rollback button. Restoring one is an edit like any other and goes through rehearsal."`, `"Remove"`, `"Edit the mapping"`.
Caption: "The live version is the card; history is a list under it. *Remove* is rung 3 — it opens the type-to-confirm sheet from M21b — and sits away from the thumb, 12px clear of the benign action."
> The bar's `gap` is `10px`, not the 12px the caption and `MOBILE.md` require.

---

## M23 · Holds

One record, three faces: operator says **hold**, member says **withheld**.

### M23a — "Operator · placing a hold"
Rung 1 (reversible) — but the reason field is required and **labelled with who
reads it**.
Structure: dimmed page (`.2`) → backdrop `.76` → sheet (`padding:12px 18px 24px`,
`gap:13px`): grabber → title `19px` → explanation `13.5px/1.55` `.7` → reason
field group (eyebrow + textarea `min-height:76px`, `radius:14px`,
`border:1px solid rgba(155,123,255,.4)`, `background:#0b0c0a`,
`padding:11px 13px`, `14px/1.5`, caret `1.5px × 18px`) → "Ends" segmented pair
(two options `flex:1`, **`min-height:42px`**, `radius:12px`;
selected `background:#7f5af0`) → `height:50px` accent button.
**[CONTRADICTION: 42px segments]**
Copy: `"Hold CNC router for Devan Suresh"`, `"Fabrication Access still grants this. A hold takes it away without changing the role."`, `"Reason · Devan will read this"`, field content `"Until your refresher induction"`, `"Ends"`, `"When lifted"` / `"On a date"`, `"Place the hold"`.
Caption: "Placing a hold is reversible, so it is rung 1 — but the reason field is required and labelled with who reads it. That label is the whole design: it is why holds get written in plain words."

### M23b — "Review · all holds"
Structure: status bar → header (back-line `"Review"`, title `"Holds · 3"`,
definition sentence) → body `padding:13px 16px`, `gap:12px`: hold cards.
- Hold card: head row `align-items:flex-start`, `justify-content:space-between`:
  left name `15.5px/600` + role `13px`/`.5` `margin-top:2px`; right **age** mono
  `12px`, `flex:none` — `rgba(243,245,239,.42)` normally, `#f5a524` past 60 days.
- Quoted reason `13.5px/1.5` `.7`, verbatim with curly quotes and an em-dashed
  attributor.
- Action: `min-height:44px` outline pill, full width via `flex:1`.
- Stale-hold sentence `12.5px/1.5` `#f5a524`.
Copy: `"Review"`, `"Holds · 3"`, `"Access a role grants that somebody is withholding"`, `"Devan Suresh"` / `"CNC router · operate"` / `"4 days"`, `"“Until your refresher induction” — K. Rao"`, `"Lift the hold"`, `"Priya Menon"` / `"Studio shares · read, write"` / `"61 days"`, `"“Pending fee settlement” — Accounts"`, `"Older than 60 days. A hold this old is usually a decision nobody made."`
Caption: "Age is the column that matters, so it sits top-right and turns amber past 60 days with a sentence saying why. The reason is quoted verbatim — it is what the member was told."

---

## M24 · Door cards

Section: "The one flow that is better on a phone." Enrolment waits for a
physical tap and says so in the present tense.

### M24a — "Enrol · waiting for the reader"
Structure: dimmed page (`.18`) → backdrop `.78` → sheet
(`padding:12px 18px 26px`, `gap:15px`): grabber → title `19px` → centred waiting
block (`padding:18px 0 8px`, `gap:14px`) → `min-height:50px` outline `"Cancel"`.
- Reader target: `78 × 78px` ring `border:1px solid rgba(155,123,255,.35)`,
  radius 999; inner `44 × 44px` `background:rgba(155,123,255,.14)` +
  `border:1px solid rgba(155,123,255,.5)`; outer halo `inset:-9px`,
  `border:1px solid rgba(155,123,255,.16)`. **The rings are static — they mark a
  target, they do not pulse.**
- Instruction `15.5px/600` centred; sub-line `13.5px/1.5` `.62`, `max-width:280px`.
Copy: `"Enrol a card for Aditi Rao"`, `"Hold the card against the studio reader"`, `"Waiting for a tap. This stays open for two minutes, then stops on its own."`, `"Cancel"`.
Caption: "The waiting state names the reader and states its own timeout, so an operator holding a card knows how long they have. The rings are static — they mark a target, they do not pretend to pulse."

### M24b — "Cards · lost card drains"
Structure: status bar → header (back-line `"Priya Menon"`, title `"Cards · 2"`)
→ body `gap:12px`: revoking card + active card.
- Revoking card: `border:1px solid rgba(255,92,77,.3)`; head row (name
  `15.5px/600`, uid mono `12.5px` `.5` `margin-top:3px`, right status word
  `"revoking"` `12px` `#ff8d82`); `height:5px` progress at `50%` filled
  `#ff5c4d`; per-door lines `gap:7px` (`7px` dot + `13.5px` sentence: lime =
  done, `#ff5c4d` + `#ff8d82` text = failed, hollow = waiting); closing sentence
  `12.5px/1.5` `.5`.
- **Red here is a state, not a button.**
Copy: `"Priya Menon"`, `"Cards · 2"`, `"Card 04"`, `"3f 9a c1 02"`, `"revoking"`, `"studio-door · revoked"`, `"workshop-door · did not answer, will retry"`, `"gate · waiting"`, `"The card is refused everywhere Syndra has reached. Doors that have not answered still accept it."`, `"Card 07"`, `"7c 21 e4 88"`, `"active"`, `"Enrolled 02 Jan · opens 3 doors"`.
Caption: "Revocation drains inline, per door, and says plainly which doors still accept the card — the sentence an operator repeats to the person who lost it. Red here is a state, not a button."

---

## M25 · Dormant accounts (bulk selection)

Section: "Bulk selection needs a mode, not a gesture." Long-press is forbidden;
a named `Select` control switches modes.

### M25a — "Reading"
Structure: status bar → header (`gap:10px`): control row (back-line left,
`"Select"` right at `min-height:44px`, `padding:0 4px`, `14px/600` `#9b7bff`) →
title → definition sentence → list, **no checkboxes at all**.
- Row **`min-height:66px`**, `padding:12px 16px`, column `gap:4px`; identifier
  mono `13.5px`; context `13px`/`.5`. **[CONTRADICTION: 66px]**
- Unselectable row: `border-left:2px dashed rgba(255,255,255,.16)`, identifier
  dimmed to `.55`, reason on the second line at `13px/1.45` `.62`. No icon, no tooltip.
Copy: `"Review"`, `"Select"`, `"Dormant accounts · 12"`, `"Accounts on targets that nothing in Syndra points at any more"`, `"legacy.build"` / `"fileserver-01 · last login 8 months ago"`, `"t.varma"` / `"fileserver-01 · membership ended 14 Mar"`, `"svc.backup"` / `"Cannot be removed — it owns the nightly snapshot task"`, `"r.krishnan"` / `"fileserver-02 · last login 3 days ago"`.
Caption: "Reading mode has no checkboxes at all, so the list stays a list. The unselectable row carries a dashed left edge and its reason on the second line — no icon, no tooltip."

### M25b — "Selecting"
The header's left control becomes select-all and the title becomes the count.
Structure: header control row (`"Select all 11"` left, `"Done"` right, both
`min-height:44px`) → title `"2 selected"` → rows with checkboxes → pinned bar
with an exclusion note + destructive action.
- Selected row `background:rgba(155,123,255,.07)`.
- Unselectable row keeps its dashed left edge and gets a **dashed** checkbox.
- Action bar: column `gap:9px`; note `12.5px`/`.5`; button `min-height:50px`,
  `border:1px solid rgba(255,92,77,.4)`, `background:rgba(255,92,77,.14)`,
  `#ff8d82`, `14.5px/600`. **The action names rehearsal, not removal.**
Copy: `"Select all 11"`, `"Done"`, `"2 selected"`, `"legacy.build"` / `"last login 8 months ago"`, `"t.varma"` / `"membership ended 14 Mar"`, `"svc.backup"` / `"Owns the nightly snapshot task"`, `"r.krishnan"` / `"last login 3 days ago"`, `"One row cannot be selected. Its reason is on the row."`, `"Rehearse removing 2"`.
Caption: "The title becomes the count, the header's left control becomes select-all, and the action bar names the next step — rehearsal, not removal. Bulk work never skips the plan."

---

## M26 · The middle breakpoint (744px)

### M26a — "744px · collapsed rail, three columns"
The only tablet figure. Frame `radius:26px`, row layout.
- Rail: `flex:none; width:64px`, `background:#101210`,
  `border-right:1px solid rgba(255,255,255,.07)`, `padding:16px 0`, centred,
  `gap:6px`. Mark `22 × 22px` (a `16px` arch, `radius:16px 16px 0 0`,
  `border:1.5px solid rgba(206,188,255,.8)`, no bottom), `margin-bottom:12px`.
  Items `44 × 44px`, `radius:12px`, a `7px` dot each; active
  `background:rgba(155,123,255,.14)`, dot `#c9b6ff`. Badge `min-width:16px;
  height:16px`, `10px/700` at `top:7px;right:7px`; amber presence dot `6px` at
  `top:8px;right:9px`. Divider `28 × 1px`, `margin:6px 0`. **Labels dropped.**
- Content header `padding:16px 22px 14px`, `align-items:flex-end`: title `26px`
  + meta `13.5px`; action pill **`min-height:40px`**, `padding:0 16px`,
  `background:#7f5af0`, `14px/600`. **[CONTRADICTION: 40px on a device
  `MOBILE.md` says keeps 44px targets]**
- Table: card `radius:18px`; header row `min-height:40px`, `padding:0 18px`,
  `gap:16px`, eyebrow type, columns `flex:1` / `width:150px` / `width:110px`;
  body rows `min-height:58px`, `padding:10px 18px`.
- **Three columns is the honest maximum: resource, source, until.**
Copy: `"Aditi Rao"`, `"Studio · 4 roles"`, `"Give access"`, `"Resource"` / `"Source"` / `"Until"`, `"Laser cutter"` / `"operate"` / direct / `"30 May 2026"` (amber mono), `"3D printers"` / `"operate"` / bundle / `"No end date"`, `"Studio Access"` / `"enter"` / mapping / `"No end date"`.
Caption: "The rail keeps its badges and its divider at 64px, labels dropped — the eight destinations are already learned by anyone in Advanced. Three columns is the honest maximum: resource, source, until. Granted-by moves into the row's detail."

Accompanying breakpoint panels (verbatim rules):
- **Under 720px · phone:** "Single column, 16px gutters" · "Tab bar for four or fewer destinations, sheet for more" · "Dialogs, menus and popovers all become sheets" · "A2 rows by default, A1 where a list is short and decisive" · "Primary action pinned to the bottom safe area"
- **720–1080px · tablet:** "Rail returns collapsed to 64px, icons and badges only" · "Tables regain up to three columns; the rest disclose" · "Sheets become centred dialogs again" · "Bottom bar removed; header keeps the view pill" · "Touch targets stay at 44px — this is still a touch device"
- **Above 1080px · desktop:** "The board as drawn in the desktop handoff, unchanged" · "252px rail with labels, full tables, hover popovers" · "Nothing in this document overrides it"

---

## M27 · Build reference (no figures — the board's own summary cards)

**Layout:** "Gutter 16px · card radius 18px · sheet radius 24px top" ·
"Section gap 12px · row min-height 60px · disclosed row pads to 16px" ·
"Status-bar inset respected; bottom bar padded to the home indicator" ·
"Sticky: page title, freshness strip, group headers, action bar"

**Type:** "Page title 26px Bricolage 600 · section title 19–22px" ·
"Row primary 15–15.5px 600 · row secondary 13px" ·
"Body 14px/1.55 · never below 12.5px anywhere" ·
"Monospace 13px for ids, paths and permissions"

**Targets:** "44px minimum · 50px destructive · 52px copy rows" ·
"Whole row is the hit area, never just the checkbox or chevron" ·
"12px between destructive and benign, benign nearer the edge" ·
"Focus ring `#c9b6ff` 2px, 4px offset, `press` timing"

**Carried over unchanged:** every route, screen name and label; the three-rung
ladder and its thresholds (rung 3 still means typing the name); read-freshness
rules (adoption blocks at ten minutes, applying does not); source vocabulary
(Direct, Via bundle, Automatic — dot before word); hold/withheld as one record
with two words; colour meanings; every disabled control states its reason as
text, in place.

**Open, before build (the board's own list):**
1. "The dormant-account listing endpoint from §29 is still unbuilt; M25 assumes it returns the reason a row cannot be removed."
2. "Card enrolment needs a reader-event stream for M24a's waiting state. Polling would work but the two-minute timeout must come from the server, not the client."
3. "TrueNAS first, Unifi Access later — M18b's third member tab appears only once TrueNAS ships."
4. "Provider latency in M17b is a measured round trip, not an uptime percentage. Confirm the backend reports it per read."

**Reading order:** M00 → M28 → M01, M02, M32 → the rest in any order.

---

## M28 · Sign-in on touch

One action: sign in through Zitadel. No email field, no password, no reset.
Three states told through the arch itself.
Page background is **`#0a0b08`**, not the shell's `#080906`. Page gutter `22px`.

Shared geometry across all three figures:
- Ambient wash: `bottom:0; height:300px; linear-gradient(to top, rgba(127,90,240,.1), transparent)`.
- Pool: `bottom:150px; width:420px; height:200px;
  radial-gradient(closest-side, rgba(127,90,240,.5), transparent 74%); filter:blur(40px)`.
- Kicker at `top:62px`: `"Makerspace Syndra"`, `13px/400`, `letter-spacing:.18em`,
  uppercase, `rgba(243,245,239,.4)`.
- Stage `296 × 270px`, `margin-top:-36px` (430:392 ratio kept).
- Arch: `border:1.5px solid rgba(155,123,255,.5)`, `border-bottom:none`,
  `border-radius:131px 131px 0 0`, masked
  `linear-gradient(to bottom,#000 6%,rgba(0,0,0,.5) 54%,transparent 95%)`;
  inner wash `linear-gradient(to top,rgba(127,90,240,.15),transparent 62%)`.
- Orb: `40 × 40px` at `top:38px`, centred. Base ring
  `1.5px solid rgba(155,123,255,.22)` with a radial mask; **lit ring**
  `1.5px solid rgba(206,188,255,.92)` at **opacity 1**, mask
  `linear-gradient(0deg,#000 4%,transparent 80%)` — lit from below, where the
  button is, and **permanent, because touch has no pointer to release it to**;
  bloom `inset:-16px`, `radial-gradient(closest-side,rgba(170,142,255,.3),transparent 76%)`
  at **.9**; core `14px` `#9b7bff` with `box-shadow:0 0 34px 6px rgba(155,123,255,.55)`.
- Wordmark `"Syndra"` Bricolage `40px/600`, `line-height:1`, `-.03em`, at `bottom:52px`.
- Button: `transform:translate(-50%, calc(50% + 10px))` — straddles the arch
  baseline; `background:#7f5af0`, `#f7f4ff`, `radius:14px`, `padding:16px 30px`,
  `15.5px/600`, `white-space:nowrap`, shadow
  `0 26px 60px -18px rgba(127,90,240,.85), 0 8px 24px -8px rgba(127,90,240,.6), 0 0 70px 10px rgba(127,90,240,.18)`.
- Base text block at `bottom:30px`, `gap:10px`, centred.

### M28a — "idle"
Copy: `"Makerspace Syndra"`, `"Syndra"`, `"Sign in with Zitadel →"`, `"Syn — the goddess who keeps the door, and the defence called upon at trial."` (`17px/1.6`, `.55`, `max-width:560px`), `"Built in the lab it runs."` · `"Powered by Zitadel"` (`12.5px`, `.05em`, `.3`, separated by a `3px` dot).
Caption: "Arch scaled to 296 × 270 — the 430:392 ratio kept, the corner radius scaled with it to 131px. Wordmark 40px per §7. The button keeps its auto width and still straddles the arch baseline; at 232px it fits 390 with room, so it does not need to drop below."

### M28b — "opening · the door opens"
The arch retracts to **half height** (clipped to `135px`) and fades to
`opacity:.12` — "retracting alone would leave the uprights with cut ends".
Wash goes to **0, not a low value**. Pool brightens:
`rgba(150,116,255,.72)` + `filter:blur(40px) brightness(1.75)`; ambient `.14`.
Orb bloom to opacity **1**. Button becomes `background:rgba(127,90,240,.34)`,
`color:rgba(247,244,255,.62)`, shadow reduced.
Copy: `"Opening…"`, `"Handing you to Zitadel."` (`16px/600` `#c9b6ff`), `"You'll come back here signed in."` (`14.5px/1.5` `.5`).
Caption: "The arch retracts to half height and fades to nothing — retracting alone would leave the uprights with cut ends. The wash goes to 0, not a low value, or it keeps holding the silhouette. The redirect is issued at the start; the animation is cover, not a gate."

### M28c — "unreachable · the door stays shut"
The arch's **mask is removed**, so the stroke renders as a complete closed line —
the door is shut, told by geometry rather than a banner. Border goes
`rgba(245,165,36,.5)`. Violet pool drops to `opacity:.15`; an amber pool
(`rgba(245,165,36,.42)`) takes its place; ambient wash `rgba(245,165,36,.07)`.
Orb dims to `opacity:.35` (and loses its bloom layer). Button unchanged from idle.
Copy: `"Zitadel didn't answer."` (`16px/600` `#f5a524`), `"Nothing was signed in. Try again in a minute, or find a steward in the space."` (`14.5px/1.5` `.5`).
Caption: "The arch's mask is *removed*, so the stroke renders as a complete closed line — the door is shut, told by the geometry rather than a banner. Border goes amber, the violet pool drops to .15 and an amber pool takes its place, the orb dims to .35. Retrying returns to idle and replays the entrance."

**Height, not width:** "The composition does not compress. Below ~610px of
viewport height the button's overhang collides with the Syn line, so a short
window scrolls." · "the stage keeps `min-height: max(100vh, 800px)` and scrolls" ·
"The base text and both messages share `bottom: 74px` on desktop and bottom 30
here, so a state change never shifts the layout."

**Reduced motion:** "Skips the entrance and the ambience. The ring keeps its
static mask, which on touch it already has. Both state changes still happen —
instantly if necessary, because the arch is what carries the meaning."

---

## M32 · Back, history and deep links

Per-tab stacks; a cascade is one thread that crosses them.

### M32a — "Following a cascade"
Structure: status bar → header: back-line + `·` + cascade id link, title `24px`,
then a **third line** explaining the model (`13px/1.5`, `.5`) → two explanation
cards.
- The third line "appears once, on entry into a cross-tab thread" — it is the
  only place the board explains a navigation model on-screen.
Copy: `"Queued writes"` · `"csc_2f81b0"`, `"fileserver-01"`, `"You are in System, following a thread that began in Automation."`, `"Back goes to"` / `"Queued writes, in Automation — where you came from, not the last thing you looked at in System."`, `"Tapping the System tab"` / `"Leaves the thread and returns System to where it was. The cascade is not lost — its id is still a link in History."`
Caption: "The back-line names the origin and carries the cascade id, so a thread four taps deep still says what it is. The third line is the only place the board explains a navigation model in words on the screen itself — it appears once, on entry into a cross-tab thread."

### M32b — "Arriving cold"
Deep link with no history. Back reads `"Today"`; the parent is reachable as a
**named link in the header beside the cascade id**, not as back.
Copy: `"Today"`, `"Create account · aditi.rao"`, `"Part of Studio membership"` · `"csc_2f81b0"`, `"Queued"`, `"On fileserver-01. Queued 4 minutes ago, next attempt in 2."`, `"Back reads Today because there is nothing behind this screen. It does not reconstruct a chain the operator never walked — an invented history is worse than an honest exit."`
Caption: "The parent is reachable, just not as *back*: it sits in the header as a named link beside the cascade id. Somebody arriving from a chat message can go up the chain deliberately or leave to Today."

**Stack diagram** (`Four stacks, one thread`): four columns (`Today`, `People`,
`Automation`, `System`); entries `min-height:34px`, `radius:9px`,
`background:rgba(255,255,255,.045)`, `12.5px`. The violet run
(`rgba(127,90,240,.2)` + `border:1px solid rgba(155,123,255,.4)`, `#c9b6ff`)
lives in Automation even though its second screen belongs to System; a `1px × 12px`
`rgba(155,123,255,.5)` connector marks the cross. System's own stack keeps
`fileserver-02` (drawn dashed = remembered but not current).

**Rules for the build (verbatim):**
- "Tapping the active tab returns that tab to its root; it never reloads."
- "A sheet is a level of history — back closes it before leaving the screen."
- "Sign-out clears every stack. Sessions last weeks, so this is rare and deliberate."
- "Every screen is addressable, and every deep link lands on the screen itself, never on a parent with the child scrolled into view."
- "The system back gesture and the header's `‹` do the same thing, always."

---

## Contradictions with `MOBILE.md`

Ordered by how much they will hurt at build time.

### 1. The 12.5px type floor is violated everywhere, including by `MOBILE.md` itself
`MOBILE.md` says "**Never below 12.5px anywhere**", and M27's own card repeats it.
But its own typography table lists an **11px** uppercase eyebrow as a role — and
the board draws that eyebrow on almost every screen. Below-floor sizes actually
drawn:

| Size | Where |
| --- | --- |
| 10px | M01a/M04a/M04b view-pill caret `"▾"`; M26a rail badge |
| 10.5px | M01a/M04a tab-bar badge; M12/M19 status-bar time |
| 11px | every uppercase eyebrow / group header, on ~40 figures |
| 11.5px | tab-bar labels (M01a, M01c, M04a, M18b); nav-sheet badges |
| 12px | M02c/M10a `"Copy"`; M10a `"not in token"`; M14a drift-kind words; M14b drain counter; M16b/M22b/M23b mono meta; M24b `"revoking"`/`"active"`; M12c endpoint string |

**Resolve before build:** either the floor means "body and row copy", in which
case say so and give the eyebrow/badge/meta scale explicit exemptions, or the
board needs a pass. As written, no figure on the board complies.

### 2. Row min-height 60px is one of at least eight values
`MOBILE.md`: "Row min-height 60px · copy row 52px".

| Drawn | Figures |
| --- | --- |
| 46px | M10a token/claim rows (**and these carry `Copy`** — see §3) |
| 48px | M04a "All N" footer link rows; M19a/M19c connection + action rows |
| 52px | M07a overflow row; M09b preview rows; M13a/M13b map rows |
| 56px | M09a picker rows; M12d degraded rows; M22b earlier-version rows |
| 58px | M14b drain rows; M26a table rows |
| 60px | the documented value — M03b, M04a, M07a, M15a, M16b, M20b, M05b |
| 64px | M05a people rows; M08a role rows; M17a history rows; M21a session rows |
| 66px | M25a/M25b dormant rows |

The pattern is legible (two-line rows with a fact go to 64–66, picker/preview
rows go to 52–56) but it is **not written down anywhere**. Either document the
A2/A2-tall/A1 ladder or normalise.

### 3. Copy rows are drawn below 52px — including the exact cases the spec names
`MOBILE.md` line 190: "Every monospace id, endpoint, request id and connection
string becomes a **52px copy row**."
- **M10a**: `tok_4c9e2f10` with a `"Copy"` affordance at **`min-height:46px`**.
- **M19a**: `smb://fileserver-01/studio` with `"Copy"` at **`min-height:48px`**,
  mono at **12px** and the affordance at **11.5px**.
- **M12c**: `GET /api/v1/users → 502` — the caption calls it "a copy row" but
  the drawing has **no min-height, no `Copy` affordance, and 12px type**.

M02c is the only figure that gets copy rows right (52px / mono 13px / `Copy` 12px).

### 4. Nav-sheet rows fall below the 44px target minimum, and the same component is drawn two ways
- **M01b** nav sheet: rows `min-height:44px`, `font-size:15px`. ✔
- **M18a** nav sheet: top-level rows **`min-height:42px`** at `14.5px`; indented
  add-on children **`min-height:40px`** at `14px`. ✘

Same component, same sheet, two geometries — and 40/42px are under the stated
floor. Also under 44px: **M23a**'s "Ends" segmented options (`42px`), every
segmented control and filter chip (`38px` — M01b, M05c, M08a), and the header
**view pill (`34px`)**, which is a primary control on M01a/M04a/M04b.

### 5. Card radius 18 vs 16 has no rule, and the same block is drawn both ways
`MOBILE.md` allows "18px · small card 16px" but never defines "small".
**M04a** draws Today's blocks at **18px**; **M04b** draws the same blocks, same
content, at **16px**. Elsewhere 16px is used for: M02c copy group, M12c error
card, M13a/M13b map groups, M17b provider cards, all of M19, M22b's version list.
18px is used for everything else. Pick a rule (e.g. "18 for a content card, 16
for a card that is really a list container") and write it down.

### 6. The sheet gutter is 18px and is not in the token table
Every content sheet uses `padding: 12px 18px 24px` (M05c, M09a/b, M14b, M15b,
M21b, M22a, M23a, M24a); the nav sheet uses `12px 14px 24px` (which *is*
documented at MOBILE.md:148). The layout table only lists the 16px page gutter,
so a builder reading the table alone will get every sheet wrong by 2px.
M19 also uses a 15px page gutter and M12 a 14px one (board scaling, but worth
naming as such).

### 7. "Dialogs, menus and popovers all become sheets" has an undocumented fourth form
**M22a** is a full-height surface (`top:34px; bottom:0`) with **no border-radius
and no grabber**, dismissed by a `"Close"` text control. It is neither a sheet
(no 24px radius, no grabber, no backdrop) nor a page. **M21b** is a second
variant: a sheet docked at `bottom:186px` above the keyboard, with a border on
all sides. Both need naming as first-class surface types.

### 8. Smaller inconsistencies
- **M22b**'s destructive/benign action bar uses `gap:10px`, not the required 12px
  separation (its own caption says "12px clear of the benign action"). Every other
  two-button bar on the board is also `gap:10px` — M07b, M09b, M11a, M14a, M16a, M20b.
- **M26a** (tablet) draws its primary action at **`min-height:40px`** on a
  breakpoint whose own panel says "Touch targets stay at 44px".
- **M07b**'s caption points at **M29**, a section that does not exist on this board.
- Figure frame radii vary (30 / 28 / 26px) and figure `min-height` values
  (300–700px) are board artifacts — do not read them as screen values.

---

## States with no equivalent in the current app

Checked against `ui/src` on this branch. Not exhaustive — a component may exist
under a different name — but these are the ones with nothing obvious behind them.

**Whole features with no code at all**
- **M24 (door cards)** — no card, enrolment, reader, or Unifi surface exists
  anywhere in `ui/src`. M24a additionally needs a reader-event stream and a
  **server-supplied** two-minute timeout (M27 flags this).
- **M13b (light theme)** — the board's only light figure; the app has one theme.
- **M10 as drawn** — `components/apps/TokenPreview.tsx` / `AppTokenScreen.tsx`
  exist, but the "gap explained first, evidence second" verdict card and the
  `"not in token"` per-claim marker are new.

**Navigation primitives that do not exist**
- No tab bar, no bottom sheet, no nav sheet, no "Go to" bar — `ui/src/lib/nav.ts`
  is a single rail, and grep finds no `TabBar`/`Sheet` component. M01a/M01b/M01c,
  M04b, M18a, M18b all depend on primitives that must be built first.
- Per-tab navigation stacks and the cross-tab cascade thread (**M32a/M32b**) —
  there is no stack model to attach this to.
- The **view pill** as a header control (M01a, M04a, M04b) and the sheet-borne
  Basic/Advanced switch (M01b).

**Controls that do not exist on existing screens**
- **M25a/M25b selection mode** — `DormantAccounts.tsx` exists, but the named
  `Select` → checkbox-mode → `Select all N` → count-title flow, and the
  per-row "cannot be selected, here is why" payload, do not. M27 confirms the
  listing endpoint is unbuilt.
- **M05b full-screen search overlay** and **M05c filter sheet with a
  result-counting apply button** — no equivalent on `/users`.
- **M02** freshness strip with the accent-Refresh escalation and the
  ten-minute adoption gate as a *visual* state.
- **M14b / M24b inline drain** with per-item outcomes and `Stop after this one`
  — `RehearsalDialog`/`PlanReview` cover rehearsal, not the drain.
- **M23a hold sheet** with the `"Reason · Devan will read this"` label and the
  When-lifted/On-a-date pair.
- **M03b one-row-at-a-time disclosure** carrying the desktop popover's sentence.
- **M12d degraded banner** as a full-frame, non-dismissible state.

**Label drift between the board and `nav.ts`**
The board writes `"Today"`, `"Storage"`, `"Drift"`, `"Dormant accounts"`,
`"Withdrawn access"`, `"History"`; `nav.ts` has `"Home"`, `"Network storage"`,
`"Unexplained access"` (no dormant destination — it lives inside targets),
`"Withdrawn access"` ✔, `"Change history"`. M27 says "every route, every screen
name, every label" is carried over unchanged — so one of the two is wrong and it
needs settling before either is built against.
