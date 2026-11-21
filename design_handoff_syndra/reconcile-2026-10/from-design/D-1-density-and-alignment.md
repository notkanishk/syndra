# D-1 · Density, alignment and motion: the system, and the build spec

**Asked by the owner, 18 Nov 2026, looking at staging at 100%:** "why is the button on a separate line. the whole page looks zoomed in with poor use of space and clunky. the design boards are okay, but in the actual page it feel too zoomed in" · "please look for misaligned buttons due to dynamic text or spacing" · and, later the same day, "build elegant enriching posh user experience with immersive consistent cohesive motion as well."

**Status:** decided. Every taste call below was first built on a default marked recommended, pending owner; the owner answered on 18 Nov 2026 ("Yes, do recommended" for Q1–Q6 and Q8; Q7 flipped, page transitions wanted). The answers are in §9.

**Boards:** `design/d1-density-1.dc.html` to `-4` (3060 wide; one source, the parts differ in one constant; about 6,200 · 6,100 · 6,800 · 6,700 tall by estimate, each under 8,000). Every frame is drawn at 100%: 1440 or 1800 wide, never scaled. For the canvas's "App screens" page.

| Part | What it shows |
|---|---|
| 1 | The TrueNAS page in one 1440 by 900 window, today and proposed, and in daylight · the three steps of "how much tighter" · the density tokens |
| 2 | Maintenance and Name · Waiting on you · Accounts: every action in its seat |
| 3 | The 1800 window: the page frame, and Home |
| 4 | Motion: four state changes as stills, today and proposed · the motion tokens |

Not checked, and said so: the boards have not been rendered (the canvas rule), so their heights are estimates and their layout is checked only mechanically (tags balance, the script runs for all 27 frames, no hex, every hole a plain lookup). The standards figures in §3 and §7 are quoted from memory and were not re-fetched: **UNVERIFIED** against the live documents. One sample row uses the words "home folder limit" for a field; the real words come from `fieldNoun` and were not read: **UNVERIFIED**.

---

## 1 · Why it feels zoomed in

Measured from the code, and from the six staging screenshots.

| | Today | Where |
|---|---|---|
| Display type saturates at 1080px | page 40, region 32, card 22, Notice 19, empty state 22, over a 14.5 body and 13 buttons | `globals.css:754-791`: each `clamp` reaches its top at about 1080px of window, so every desktop gets the largest size |
| A region title has no line-height of its own | 32px on the body's 1.55: a 50px line | `globals.css:762-767` |
| Six body sizes within 2.5px | 13 (262 uses), 13.5 (204), 14 (151), 12.5 (145), 14.5 (95), 15 (61) | counted across `ui/src` |
| The page has no width | at 1800, 1,496px of content; a row's label and its button are a screen apart | `AppShell.tsx:97` |
| A sentence's measure is not what it says | `max-w-[72ch]` holds about 93 characters, because `ch` is the width of "0". Region ledes are 86ch, Roles 92ch | `PageHeader.tsx:92`, `Region.tsx:49`, `roles/page.tsx:214`; counted on the lede in screenshot `-0` |
| Rows are tall | 14px above and below a 34px button; sixteen list rows on fourteen pages keep a 60px floor at every width | `Card.tsx:125`; the sixteen are in §5 |
| Fields are taller than buttons | 49px field, 34px button | `Input.tsx:23` |
| Empty states | 32px above and below one sentence, and inset 24 inside a card inset 20 | `states/index.tsx:119` |
| An outline button and an accent button differ by 2px | 34 and 32: height comes from padding plus border | `Button.tsx:50-62` |

The earlier boards looked right because the canvas showed 1440 frames at half size. These are drawn at 100%.

## 2 · The proposal in one paragraph

Page 28, region 22, card 17, body 14, small 13, fine 12.5. The page is a 1280 frame, centred; a sentence is 60ch. A row is 10px above and below with 30px controls; a card header 12; an empty state 20. Every action sits in a right-hand seat on its row's first line, never in a sentence and never on a row of its own, unless the row is a form. Structure, words, colours, the phone and both themes are unchanged: one new column heading ("Status") is the only new word.

## 3 · The density system, as tokens

All in `globals.css`. The type steps go in `@theme` under Tailwind 4's `--text-*` namespace, so `text-body`, `text-small` and `text-fine` exist as utilities (with `--text-body--line-height` and so on); the `type-*` utilities at `globals.css:746-791` read the display steps. Each token is read by the shared part named, so no screen sets its own.

**Type**

| Token | Value | Today | Read by |
|---|---|---|---|
| `--text-display` | 34px / 1.1 (28 below tablet) | 42 | `type-greeting`: Home's greeting, nothing else |
| `--text-page` | 28px / 1.15 (26 below tablet) | 40 | `type-page-title` |
| `--text-heading` | 22px / 1.2 | 32; a dialog 26 | `type-section-title` (a region, Home's group headings), `type-dialog-title` |
| `--text-title` | 17px / 1.3, 600 | 22; Notice 19; outcome 22 | `type-card-title`, `type-empty-title`, Notice's and ActionOutcome's title |
| `--text-figure` | 24px, 600 | 24 | StatCard and the usage figure. Unchanged (owner, 17 Nov: "I like big number") |
| `--text-body` | 14px / 1.5 | 15, 14.5, 14 | sentences, a row's first line |
| `--text-small` | 13px / 1.45 | 13.5, 13 | second lines, notes, every control's label |
| `--text-fine` | 12.5px / 1.4 | 12.5, 12 | labels, captions, times. The floor (A-09) |

Why these: Apple's text styles step 34 · 28 · 22 · 17 (Large Title, Title 1, Title 2, Headline); Material 3 has headline-medium 28, title-large 22, title-medium 16, body-medium 14; Carbon's productive set has heading-04 at 28 and body-01 at 14, and keeps the larger "expressive" sizes for editorial pages. Each step is about 1.27 times the one below. The display face keeps one loud moment, the greeting. The clamps go: a phone already had 26 / 22 / 19, so the display roles step once at tablet (`globals.css:422` already holds a phone block) instead of growing with the window.

**Controls** (pointer, from the desktop breakpoint; `--tap` 44 is untouched below it)

| Token | Value | Today | Read by |
|---|---|---|---|
| `--control-sm` | 30px | 34 outline, 32 accent | `PILL.sm`, the dense field, Chip, the read line's chips |
| `--control-md` | 36px | 37 outline, 35 accent | `PILL.md`: a page header's and a dialog footer's button |

Height is set, not built from padding and border, so the variants match. 30px clears WCAG 2.5.8's 24px minimum; Carbon's small control is 32, Material's dense button 32.

**The page**

| Token | Value | Today | Read by |
|---|---|---|---|
| `--page-max` | 1280px | none | the page frame |
| `--page-x` | `max(26px, calc((100cqw - var(--page-max)) / 2))` | 26 | main's side padding, the top bar's row, the section band's inset: three readers, one frame |
| `--page-top` | 24px | 30 | main |
| `--bar` | 56px | 66 | TopBar, from tablet up |
| `--measure` | 60ch | 46 to 92ch, 840px | every sentence: lede, caption, Notice, guidance, a row's detail line |
| `--field-max` | 400px | the card's width | a text field on a page |

The rule: **sentences 60ch, the page 1280, a field 400.** 60ch of Figtree is about 78 characters (Bringhurst: 45 to 75; WCAG 1.4.8: at most 80). The frame is centred between the rail and the window's edge, and the top bar's row takes the same frame so the breadcrumb stays over the title. At 1440 the content is 1,136px and nothing is felt; the cap starts at a 1,584px window. A wider table shows no more: its columns are fixed seats and one that flexes, so extra width becomes distance between a name and its action. Material 3 says the same for windows past 1200: add a pane or cap the content. Insights' map may opt out (`data-wide` on the page root) if its lanes need the room; nothing else does.

**Rhythm**

| Token | Value | Today | Read by |
|---|---|---|---|
| `--region-gap` | 40px | 32 | region to region (and SectionPill's `gap`) |
| `--region-head-gap` | 12px | 14 | a region's heading to its first card |
| `--card-x` | 20px | 20; EmptyState 24 | the one inset of a card: header, rows, columns, caption, empty state |
| `--card-head-y` | 12px | 16 | CardHeader |
| `--row-y` | 10px | 14; SettingRow 12 | CardRow, SettingRow, the list pages' rows. No 60px floor above tablet |
| `--row-gap` | 16px | 18; SettingRow 20 | between a row's seats, and its column headings |
| `--empty-y` | 20px | 32 | EmptyState |
| `--dot-indent` | 14px | 14, 15, 16, 18 | a line under a status dot: the dot (6) and its gap (8) |

Kept: 28 under a page header (owner, 5 Nov), 16 between cards, every radius. A row with a button is 50px (Carbon's default table row is 48); a row of text is 41. With a smaller title the region boundary is carried by space, so that one gap grows.

**Seats**

| Token | Value | Today | For |
|---|---|---|---|
| `--seat-label` | 170px | 170 | a setting's label |
| `--seat-number` | 110px | 110; a date 66 | a number or a date, right-aligned (G16) |
| `--seat-action-narrow` | 140px | 140, 150 | one short verb, or one link |
| `--seat-action` | 200px | 196, 200, 240, 260 | one or two verbs. The default |
| `--seat-action-wide` | 300px | 380 | three verbs |

## 4 · The row, and the action seat

One anatomy for every row in a card. It extends the rule RULES already has for setting rows (label 170 · value · way on in the 200px right seat).

```
[ lead · body (the one seat that flexes) · status · number · ACTION ]   line 1
[ a sentence about this row, at --measure, from the text's own edge ]   line 2, optional
```

1. **The action seat is the row's last, right-aligned, on its first line.** A button or an ArrowLink. Never inline in a sentence. Never a row of its own, unless the row is a form.
2. **The seat's width belongs to the table, not the row**: the most verbs any row of that table can ever show, from the three tokens. It is a `min-width`, so a long label grows it and the right edge never moves. An empty seat keeps its place. A seat never changes width with the data.
3. **A sentence that explains the row is its detail line**, under the first line, at `--measure`. So a long sentence can no longer push a button down or squeeze it.
4. **One line, centred; more than one, top-aligned**, with each seat's first line centred on the control (`padding-block: calc((var(--control-sm) - 1lh) / 2)`), so a one-line row and a three-line row put their buttons at the same height.
5. **A line under a status dot starts at the word**: `--dot-indent`, one number. StandingLine's next step goes through StatusSeat's own grid, so it cannot drift.
6. **A form row**: the field (dense, at most `--field-max`) and its button on one line; the hint under the field; a disabled button's reason under the button (G31).
7. **A card's explanation is a caption** under its title, at `--measure`. CardHeader's right-hand `note` is for a short fact (All read, a time): today a 150-character note sits right when it fits and drops left when it does not.
8. **A button names its verb, not its row.** The row already says the name.
9. Below tablet the seat wraps under the row and stays right-aligned (the existing rule).
10. **A table's columns are one list, and each states its size** (added 19 Nov 2026, from the layout audit). The headings and every row are drawn from one `Columns` list (`Card.tsx`), so a heading cannot sit anywhere but over its cells. Each column is either a fixed seat from §3 (a number, a status, a label) or a share held to a minimum (`min-w-[N] flex-1`). Flex breaks a line on those sizes and never on the words, so the headings and the rows break at the same widths. A row whose words fit keeps its seat on its first line. Where the row can't hold its columns, the seat drops under it, right-aligned, then the last column. Nothing is ever clipped. A heading sits over its column's box, as part 2 frame 10 draws Person over the avatar and Status over the dot. A right-aligned heading's sort arrow hangs before the word, so the word ends where the figures end. A heading for what the rows' detail line says is the headings' own detail line, under them and from the text's edge (Drift's "Why"), never a cell of its own. A seat holding a disabled button's reason is exactly the table's seat width, and its verbs and the reason wrap inside it.

Not rows, and not changed: a **Notice** and an **EmptyState** end on their action, because a message is read top to bottom and ends on its step. A **link inside a lede, a caption or a meta line** is prose. The **Read again** pill belongs to the sentence it refreshes. **Dialogs** are A-22's.

## 5 · Every misalignment found (`page: element / fix`)

**A button or link whose place depends on a sentence**

| # | Where | Fix |
|---|---|---|
| 1 | TrueNAS · Syndra's reach: "Open Maintenance" at the end of the sentence, and a different label in Basic (`TargetOverview.tsx:841-851`) | the row's seat (narrow), on the line that says Answering; the sentence is the detail line |
| 2 | TrueNAS · Check for differences: "Check now" beside a sentence that runs to three lines (`TargetOverview.tsx:562-610`) | seat; sentence as body, top-aligned |
| 3 | TrueNAS · accounts no longer there: sentence and one or two buttons share a wrapping line (`TargetOverview.tsx:640+`) | seat (default); sentence as detail |
| 4 | TrueNAS · a decided finding: note then "Finish again" wrapping under it (`MergeFindings.tsx:194-220`) | seat |
| 5 | A person · mapped role: "Show the rule" after a sentence (`PersonAccess.tsx:1057-1061`) | seat |
| 6 | A person · unexplained roles: "Decide on Drift" after a run of chips (`PersonAccess.tsx:718-731`) | seat (link) |
| 7 | Change history card: ArrowLink after the consequence (`operations/cascades/page.tsx:123-125`) | seat (link) |
| 8 | The usage card's end row: ArrowLink after the step (`UsageCard.tsx:126-129`) | seat (link) |
| 9 | An app · "In {where}." then a link (`ShapeCard.tsx:128-133`) | seat (link) |
| 10 | Home · the "No default bundle is set" row and Systems not checked (`Home.tsx:801-807`, `:1122-1135`) | seat (link) |
| 11 | RetryWait: text then a stop button that wraps under it (`ReadFreshness.tsx:402-406`) | seat (narrow) |

**A button on a row of its own**

| # | Where | Fix |
|---|---|---|
| 12 | TrueNAS · Waiting on a decision: "Decide" in a third row under a divider (`MergeFindings.tsx:255-261`) | the finding's own first line; the sentence (`:139`) becomes the detail line |
| 13 | TrueNAS · Name: "Save name" and its reason each take a row under a full-width field (`TargetName.tsx:67-98`) | form row: dense field at 400, Save name sm on its line |
| 14 | TrueNAS · what is kept: buttons under a 140px field (`StatsRetention.tsx:68-85`) | form row |
| 15 | Drift: Adopt, Revoke, Owned elsewhere, Show evidence under the explanation (`UnexplainedAccess.tsx:997-1031`) | seat (wide) on the first line; explanation as detail. Show evidence stays the row's own fold control (G3) |
| 16 | A bundle · newer version of an included bundle: button wraps under the text (`BundlePage.tsx:1114-1131`) | seat |

**An action column with no fixed seat, or one that moves**

| # | Where | Fix |
|---|---|---|
| 17 | TrueNAS · Maintenance: buttons lead the row in a 150px seat, left-aligned, two widths (`TargetOverview.tsx:1768-1787`) | sentence first, verb in the seat |
| 18 | TrueNAS · Accounts Syndra manages: the seat is 260, or 380 when any row has a limit verb, so Since moves with the data (`PeopleOnTarget.tsx:55`) | the table's own seat, decided by the system's setup and never by the rows read: default (200) where the system has no home folder limits, wide (300) where it has |
| 19 | TrueNAS · Not created by Syndra: a button, or a sentence in its place (`TargetOverview.tsx:1413-1430`) | seat (default); the sentence is the detail line |
| 20 | Mappings · a mapping row: three controls, one of which can turn into a sentence and shift the others (`MappingManagement.tsx:261-306`, `ConvergeEntitlements.tsx:253`) | seat (wide); the sentence is the detail line |
| 21 | Mappings · a version row, "Roll back to this" (`MappingManagement.tsx:551-565`); TrueNAS · "Say what happened" (`UnconfirmedChanges.tsx:209`); Publish (`VersionBand.tsx:109`) | seat |
| 22 | A person · a role row: status of any width, then one or two buttons (`PersonAccess.tsx:905-1010`) | the status keeps a fixed seat, as wide as the longest status the table can say; actions in the default seat |
| 23 | A role · its people: three labels of three widths (`projects/[id]/roles/[key]/page.tsx:324-336`) | seat (wide) |
| 24 | A bundle · its roles, Keep / Remove (`BundlePage.tsx:512-528`); Automatic rules, "Retire rule" with no column heading (`policies/page.tsx:117-149`) | seat; CardColumns draws the empty heading cell |
| 25 | Holds due: "Lift the hold on {label}", so every button is a different width (`HoldsDueForReview.tsx:224`) | "Lift the hold"; `aria-label` keeps the name. **A wording change the layout forces** |
| 26 | Expiring access: a 300px minimum, two label pairs of different widths (`expiring-access/page.tsx:651-697`) | seat (wide) |
| 27 | Home · Extend, Mark seen, Send it now, Open {system}: last in the row, no seat (`Home.tsx:743`, `:929-953`, `:997`, `:1172-1176`) | seat (narrow) |
| 28 | A service account · What it holds (`ServiceAccountPage.tsx:231-249`); a member's role, "Ask to keep it" or a sentence (`MemberAccess.tsx:479-486`); a member's requests, Withdraw (`RequestsScreen.tsx:706`) | seat |
| 29 | Seven hand-made seats at seven widths: 140 (`roles-that-wait/page.tsx:238`, `pending/page.tsx:405`), 150 (`BundleVersions.tsx:446`), 196 (`RequestsScreen.tsx:401`, `Home.tsx:648`, `PersonRequests.tsx:144`), 200 (`SettingRow.tsx:18`, `PersonSystems.tsx:127`, `ServiceAccountPage.tsx:177`), 240 (`WithdrawnAccess.tsx:456`, `AccountKind.tsx:35`), 260 / 380 (`PeopleOnTarget.tsx:55`) | the three tokens |

**Lines and insets that do not meet**

| # | Where | Fix |
|---|---|---|
| 30 | The line under a status dot starts at 14, 15, 16 or 18px: `ActionOutcome.tsx:123-127` and `RetryStatus.tsx:32` (14), `NextStep.tsx:51` and `MemberAccess.tsx:508` (15), `StandingLine.tsx:15` and `ShapeChooser.tsx:187` (16), a reading (`TargetOverview.tsx:1302`, 18). On Accounts, "Nothing for you to do." is 2px right of "Ready to use" | `--dot-indent` |
| 31 | EmptyState is inset 24 under a card title inset 20 (`states/index.tsx:119`) | `--card-x` |
| 32 | A long CardHeader `note` sits right when it fits and left when it wraps: Waiting on a decision (`MergeFindings.tsx:96-105`), Not created by Syndra (`TargetOverview.tsx:1359`), Check for differences (`:560`) | `caption` |
| 33 | The read line's freshness is inset 2px, 20px or nothing by page (`applications/page.tsx:69` and `RequestsScreen.tsx:270` 2px, `policies/page.tsx:88` 20px, the lists under a FilterBar none) | one inset: none under a FilterBar, `--card-x` when it belongs to the cards below (rule 9b) |
| 34 | The read line's chips are 14px text beside 12.5px freshness (`Badge.tsx:59`, `SectionNav.tsx:83`) | chips at `--text-small`, `--control-sm` |
| 35 | The section pill changes width as you scroll: an arrow on every section but the current one (`SectionNav.tsx:181-185`) | every section keeps its arrow seat (§7, M7) |
| 36 | Home's request row needs 682px of fixed seats inside a 556px column at 1440, so it always wraps (`Home.tsx:636-678`) | Home's layout (below) |

**Density, page by page**

| # | Where | Fix |
|---|---|---|
| 37 | Sixteen rows keep the 60px floor above tablet: `users/page.tsx:632`, `projects/page.tsx:162`, `roles/page.tsx:287`, `applications/page.tsx:118`, `policies/page.tsx:117`, `audit/page.tsx:196`, `governance/pending/page.tsx:480`, `review/expiring-access/page.tsx:592`, `projects/[id]/roles/[key]/page.tsx:209` and `:270`, `zitadel/projects/page.tsx:201`, `zitadel/users/page.tsx:345`, `RequestsScreen.tsx:351` and `:653`, `WithdrawnAccess.tsx:422`, `PersonRequests.tsx:128` | one exported row class from `Card.tsx` (the `line` CardRow already has) |
| 38 | Home: two pinned columns, so a short group leaves its column empty; group headings the same size as card titles (`Home.tsx:264-317`, `:479`) | groups stack; blocks two across when each can be 600 wide, else one; headings `type-section-title` |
| 39 | TrueNAS · Accounts: status and next step stacked under the username; 98px a row | a Status column; 62px |
| 40 | Raw display sizes outside the scale: `MyStorage.tsx:114` and `AnswerPanel.tsx:99` (24), `ActionOutcome.tsx:173` (22), `Notice.tsx:101` and `AccessSource.tsx:327` (19), `Makerspace.tsx:224` (20) and `:299` (18), `BundleVersions.tsx:104` (17) | 24 → `--text-figure` where it is a figure, else `--text-heading`; 22 and 19 → `--text-title`; 20, 18, 17 → `--text-title` |
| 41 | `/storage`, which the owner likes: one card, labels and dividers inside it, one display line (24), a 460px field, 66ch text | its character is the model for this pass and it changes least: the headline joins `--text-heading` (22), the text `--measure`, the field `--field-max` |

## 6 · Build spec: density and alignment

Five passes. Each is one PR and leaves the product consistent.

**Pass 1 · tokens and shared parts** (every page tightens at once; no screen is edited)

| File | Change |
|---|---|
| `ui/src/app/globals.css:32` (`@theme`) | add every token of §3. `:422` phone block: `--text-display: 28px; --text-page: 26px` |
| `globals.css:746-791` | the six `type-*` utilities read the tokens; `type-section-title` gains its line-height; the clamps go |
| `globals.css` (components layer) | `measure` (`max-width: var(--measure)`), `row-seat` (`margin-left:auto; display:flex; flex:none; justify-content:flex-end; gap:8px; min-width:var(--seat-action)`, with `data-seat="narrow|wide"`), `row-detail` (`flex-basis:100%; max-width:var(--measure)`, `text-small`, muted), `row-top` (rule 4) |
| `components/shell/AppShell.tsx:82` | the column becomes a size container (`container-type: inline-size`) so `--page-x` can use `cqw` |
| `AppShell.tsx:97` | `tablet:px-(--page-x) tablet:pt-(--page-top)`; a page root may set `data-wide` to opt out |
| `components/shell/TopBar.tsx:39` | `tablet:h-(--bar) tablet:px-(--page-x)` |
| `components/ui/SectionNav.tsx:149` | the band's `tablet:-inset-x-[26px]` → `-inset-x-(--page-x)` |
| `components/ui/PageHeader.tsx:90-92` | meta and lede: `text-body`; lede `measure` |
| `components/targets/Region.tsx:40-51` | gaps from the tokens; lede `measure`, `text-body`. Export `REGION_GAP` |
| `components/targets/TargetOverview.tsx:162-163, 248` | `gap-(--region-gap)`; `SectionPill gap={REGION_GAP}` |
| `components/ui/Card.tsx:65` | CardHeader: `px-(--card-x) py-(--card-head-y)`; new `caption?: ReactNode` under the title; `note` documented as a short fact |
| `Card.tsx:125` | CardRow: `gap-x-(--row-gap) gap-y-1 px-(--card-x) py-(--row-y)`; new `action`, `seat`, `detail`, `top` props that draw `[data-action-seat].row-seat` and `[data-row-detail].row-detail`. Export the `line` as `ROW` |
| `Card.tsx:204` | CardColumns: tokens; `seat` prop draws the empty heading cell at the seat's width |
| `components/targets/SettingRow.tsx:14-18` | tokens; the seat is `row-seat` |
| `components/states/index.tsx:119-125` | EmptyState: `px-(--card-x) py-(--empty-y)`; guidance `measure text-body` |
| `components/ui/Notice.tsx:94-105` | `py-3.5`; title `--text-title`; body and next `text-body measure` |
| `components/ui/Button.tsx:50-53` | `PILL`: `desktop:h-(--control-sm)` / `-md`, `text-small`, no `desktop:py-*` |
| `components/ui/Input.tsx:12` | `DENSE_FIELD`: `desktop:h-(--control-sm) text-body` |
| `components/ui/Badge.tsx:59` | Chip: `text-small desktop:h-(--control-sm)` |
| `StatusSeat.tsx:15`, `NextStep.tsx:51`, `StandingLine.tsx:15`, `ActionOutcome.tsx:123-127`, `RetryStatus.tsx:32`, `MemberAccess.tsx:508`, `ShapeChooser.tsx:187` | `pl-(--dot-indent)`; StandingLine passes its next step into StatusSeat's grid |
| the sixteen rows of §5 #37 | `ROW` |

**Pass 2 · seats** (§5 #1 to #29, #32): each row takes `action` / `detail`; each long `note` becomes `caption`. Start with `components/targets`, then Home, then the rest. `PeopleOnTarget.tsx:55` loses its data-driven width. The Name card (`TargetName.tsx:67-98`) becomes the form row; so does `StatsRetention.tsx:68-85`. Accounts gains the Status column (`PeopleOnTarget.tsx:106-202`).

**Pass 3 · Home** (`Home.tsx:262-340`, `:466-488`): the grid becomes a stack of the two groups at `--region-gap`; each group's blocks are `grid-template-columns: repeat(auto-fit, minmax(min(100%, 600px), 1fr))`, gap 16; headings `type-section-title`. Decided (Q6).

**Pass 4 · three text sizes**: a mechanical codemod. `text-[15px]`, `text-[14.5px]`, `text-[14px]` → `text-body`; `text-[13.5px]`, `text-[13px]` → `text-small`; `text-[12.5px]`, `text-[12px]` → `text-fine`; `body` in `globals.css:631` → 14px / 1.5. The dialog field (`Input.tsx:23`, 15px) and avatar initials are left alone. Decided (Q3).

**Pass 5 · resync the design system** (`ui/.design-sync`, BRIDGE steps 1 to 3), after the owner's OK, so the canvas mounts the new parts and these boards lose their override block.

**Tests that would catch a regression**

1. `design-system.test.ts`: every token of §3 exists with its value; each `type-*` utility reads its token; no `clamp(` in a type role.
2. **No button in prose** (DOM, over the existing page and card fixtures): no `button`, and no `a` carrying the pill classes, has a `p` ancestor; inside a CardRow or SettingRow every `button` and every ArrowLink has a `[data-action-seat]`, `form` or `[role="dialog"]` ancestor.
3. **No button in prose** (source guard): no `{" "}` followed by `<Button` or `<ArrowLink` in `ui/src/components` and `ui/src/app`, with an allowlist for ledes and captions.
4. CardRow unit: with `action`, the seat is the last child before `detail`, carries `data-action-seat` and the seat's width class; with `detail`, it follows the seat; `top` adds `row-top`.
5. **Seats belong to the table**: PeopleOnTarget rendered with and without a limit verb draws the same seat class; a source guard bans `w-[NNNpx]` beside `justify-end` outside `Card.tsx` and `SettingRow.tsx`.
6. Source guards: `min-h-[60px]` only with `tablet:min-h-0`; no `pl-[14px]`, `pl-[15px]` (use the token); no `max-w-[NNch]` outside the greeting's balance widths; after pass 4, no `text-[NNpx]` in components.
7. CardHeader: a string `note` longer than 48 characters fails in test, pointing at `caption`.
8. Button: an accent and an outline button of one size have the same height class.
9. The copy guard (`plain-language.test.ts`) stays green: the only new strings are "Status" and "Lift the hold".

## 7 · Motion

**What the audit found.** Syndra has one motion system already (FINAL §4, A-05): tint 120, press 180, settle 260, arrive 420 over 8px with a 40ms stagger capped at five, flash 900, breathe 2400, glide 520; a 10px travel cap; light leads shape by 40ms; closing fades first and is quicker; a still form for every movement. Read against the code on 18 Nov it is built and consistent: of the 24 Oct audit's findings, all are fixed but the sign-in door's own loops and literals, the header theme toggle, two lane-map items, and a few timing constants still typed in components. Every keyframe's distance matches the spec. There is no route transition, no shake, no count that ticks. So D-1 proposes **no new system**: five tokens that name numbers already in use, and the fixes below, each in a shared part.

**Tokens** (`globals.css`, beside the existing ones; JS mirrors in `lib/motion/still.ts`, held by `design-system.test.ts`)

| Token | Value | Replaces |
|---|---|---|
| `--duration-tint` `-press` `-settle` `-arrive` `-flash` `-breathe` `-glide`, the six `--ease-*`, `--arrive-step`, `--arrive-cap`, `--travel-cap` | kept | |
| `--duration-fold` | 200ms | the calc at `globals.css:1153`, whose comment says "No token was granted for it" |
| `--light-lead` | 40ms | the literal at `globals.css:893, 916, 919, 1081`, and `--arrive-step` borrowed for a scrim's delay |
| `--travel-arrive` | 8px | the literal in `arrive` and `settle-in` (`:210`, `:260`) |
| `--travel-lift` | 6px | the literal in `lift` (`:322`) |
| `--travel-nudge` | 4px | the literal in `popover-in`, `sink-out`, `flash-value` (`:298`, `:246`, `:375`) |

Material 3 puts a simple change at 100 to 200ms, a panel at 250 to 300 and a full arrival at 400 to 450, with exits shorter than entrances and a decelerating curve for things that enter: tint, settle and arrive, `--duration-fold`, and `--ease-settle`. Apple's guidance is that motion explains a change and is never the point. WCAG 2.3.3 asks that motion from interaction can be turned off, and 2.2.2 that anything looping can be stopped: every movement has a still form, and breathe rests under reduced motion.

**One table for every interaction.** "Built" means the code does this today.

| Interaction | What moves | Tokens | Still form | Owned by | State |
|---|---|---|---|---|---|
| Page change | the page you leave fades out; the page you reach arrives as the next row says; the shell is still | tint | swap | `PageChange` in AppShell | **§7a** (owner, 18 Nov): today nothing travels; builds with the Next 16 slice |
| A page's first paint | header tints in; blocks arrive, 40ms apart, cap 5; only when navigated to | tint; arrive, `--travel-arrive`, step | appears | `page-head`, ArriveList | built; **M1**: regions join the stagger |
| Loading to content | skeleton only after 200ms; it fades, rows arrive | tint, arrive | swap | ListStates, ReadGate | built |
| Hover, press, focus | colour only; controls press 3%; rows darken; the ring appears at once, fades on tint | tint, press | press survives | `control`, `hover-tint` | built; **M10** two lane-map items |
| A link on hover | ends on accent, underline tints in | tint | colour | `link-hover` | built |
| Basic ↔ Advanced | the switch glides; Advanced parts open by Unfold, close fade first | settle, fold, tint | appears | AdvancedOnly, Unfold | built |
| Fold, show more | opens on settle, contents 120ms behind; closing fades first; only new rows arrive | settle, fold, tint, arrive | appears | Unfold, FoldChevron, PaginationRow | built |
| A row arriving after the first read | tints in | tint | appears | `useLeavingRows` (`data-fresh`) | built; **M5** two tables lack it |
| A row leaving | fades, then its gap closes; the gap waits while the pointer or focus is in the list | tint, settle | gone | `LeavingRow` | built; **M5** |
| A count changing | crossfades in a seat sized to the wider number; never ticks, never travels | tint | swap | CountChip, CardHeader, the chips | **M2**: only the rail does it today |
| A status changing in place | words crossfade; the dot recolours; blooms only for the reader's own success | tint, flash | swap, no bloom | Crossfade, StatusSeat | built |
| A value that landed while you were elsewhere | its row washes once, colour only | flash | colour only | `useFlashOnChange` | built in the rail; **M4** the check row |
| A Notice arriving after first paint | its space opens, it tints in 120ms behind; no rise | settle, tint | appears | Notice (`UnfoldOnArrival`) | built |
| A Notice leaving | fades, then its space folds | tint, fold | gone | Notice | **M3**: callers unmount it at once |
| A queue you emptied | the all-clear's space opens; it rises 6px; its lime dot blooms once | settle, `--travel-lift`, flash | appears, no bloom | EmptyState (`resolved`, `lift`, `bloom`) | **M3**: the props exist and are unused on TrueNAS |
| An outcome under a form | its space opens; success rises 6px and blooms; failure tints in, still | settle, `--travel-lift`, flash, tint | appears | ActionOutcome | **M6**: no space opens, so the page jumps |
| A form opening under a row | the row's panel unfolds; the seat's word crossfades (Decide, Cancel) | settle, tint | appears | CardRow `disclosure` | **M6**: findings swap bare |
| Check now, Read again | the button breathes; the old words stay until the new are read, then crossfade; a reader's own read blooms its freshness dot | breathe, tint, flash | swap | Button, ReadFreshness, Crossfade | Read again built; **M4** Check now |
| Jump to a section | glide, then the landing heading flashes once; a folded section opens on the way | glide, flash | jump, flash stays | `glideTo` | built; **M8**: the flash covers the whole region |
| The section pill | drops in 10px once the read line has scrolled away; leaves on tint; its fill glides to the section you are in | settle, `--travel-cap`, tint | appears; the fill jumps | SectionPill, GlideIndicator | built; **M7**: the fill swaps, and the labels shift |
| Tabs, Segmented, FilterPills | the fill or underline glides, one seamless element placed by transform and sized by its own width and height (CRAFT 1.7's indicator exception); labels tint, and each label holds its chosen width (`label-steady`), so no option moves | settle, tint | jump | GlideIndicator, SteadyLabel | built (7z, 20 Nov 2026) |
| A dialog | scrim tints; 40ms later the sheet rises 8px from 98%; closing sinks 4px and fades, then the scrim | tint, `--light-lead`, settle, `--travel-arrive`, `--travel-nudge` | appears with its scrim | Modal | built |
| A phone sheet | from its edge on settle; follows the finger; settles back short of a third | settle | appears | Modal, TouchNav | built; **M9**: settle-back is on tint's curve |
| A dialog's result | success rises 6px and blooms; failure still | settle, flash, tint | appears | ActionOutcome | built |
| A popover, a tooltip | rises 4px on settle; closes on tint | settle, `--travel-nudge`, tint | appears | AnchoredPanel, Term | built |
| Role picker, Copied, typing a name to confirm, daylight | S15, S18, S16, S14 | as FINAL §4 | as FINAL §4 | their parts | built |
| Breathe | an amber Notice's dot, Insights' node while computing, a Button's pending dot | breathe | rests | | built |

**The fixes**

| # | Fix | Where |
|---|---|---|
| M1 | **Regions arrive one after another.** `page-head` staggers only its own siblings, and the TrueNAS page's regions sit in one wrapper, so three regions arrive as one slab at 200ms. The wrapper passes the stagger on (a `data-arrive-through` the `page-head` rule reads), so regions land at 200, 240, 280ms; cap 5 | `globals.css:1270`, `TargetOverview.tsx:162, 248` |
| M2 | **A count crossfades.** `Crossfade` keyed on the number, gated the way the rail's badge is (changed between two real reads, never on the first) | `Badge.tsx:192` CountChip, `Card.tsx:79-88`, `SectionNav.tsx:86, 180`; the model is `Sidebar.tsx:144` |
| M3 | **Resolving the last item, in order**: the Notice fades (120), its space folds (200) while the count crossfades, then the all-clear opens, rises and blooms. Notice gains `open?: boolean` and stays mounted through its own close; the region's count is read from the rows still drawn, not from the read; the two empty states are `resolved`, with `lift`, and `bloom` when the reader's own act emptied the list | `Notice.tsx:121`; `TargetOverview.tsx:201, 269-276, 284`; `MergeFindings.tsx:94-96, 121` |
| M4 | **Check now lands**: the sentence in a `Crossfade`; the row flashes once when the result changed (`useFlashOnChange`); new result rows arrive. No progress bar and no new words | `TargetOverview.tsx:563-590, 612-640` |
| M5 | **Rows fade and fold in every table**: No longer needed and Not created by Syndra map bare rows beside two tables that use `LeavingRow` | `DormantAccounts.tsx:177, 200`; `TargetOverview.tsx:1413-1460` |
| M6 | **Nothing appears by pushing the page**: ActionOutcome's block form mounts in `UnfoldOnArrival`, as Notice does; a finding's Decide form is the row's `disclosure`, and Decide / Cancel crossfade in the seat. That one change covers the save flows that swap bare today (`TargetName.tsx:93-106`, `StatsRetention.tsx:89-113`, `HomeLimit.tsx:194, 250`, `HomeLocation.tsx:105-180`, `HomeMapping.tsx:43, 68`) | `ActionOutcome.tsx:167`; `MergeFindings.tsx:150-250` |
| M7 | **The pill holds its width; its fill glides**: every section keeps an arrow seat, empty for the current one; the fill is the shared GlideIndicator | `SectionNav.tsx:156-186` |
| M8 | **The landing flash is the heading's**: glideTo is handed the whole region; flash its heading row | `SectionNav.tsx:43-45`, `lib/motion/glide.ts:62`, `Region.tsx:42` |
| M9 | A sheet's settle-back moves on tint's curve; make it settle's, as its drag release is | `Modal.tsx:513, 517` (against `:449`) |
| M10 | Two of the lane map's items tint but do not press | `LaneMap.tsx:213, 237` |
| M11 | The new tokens replace their literals; `motion-press` and `motion-settle` have no uses and go | `globals.css:848, 883, 893, 916, 919, 1081, 1153` and the keyframes |
| M12 | **One check for stillness**: Unfold decides by its own `animates()`; it asks `stillMotion()` first, so tests, the server and the reader's setting agree. The loose timing constants move into `still.ts` | `lib/motion/unfold.tsx`; `ReadFreshness.tsx:33, 35`; `states/index.tsx:61`; `lib/motion/arrive.tsx:9-11` |
| M13 | A jump to a rule uses `scrollIntoView`; it becomes `glideTo` | `policies/page.tsx:417` |
| M14 | The sign-in door keeps its two loops and its own timings, written into FINAL §4 as an exemption, with its literals named `--door-*`. Decided (Q8) | `globals.css:1807, 1880, 1889, 1961, 1999, 2003`; `components/login/ceremony.ts:35-36` |

**Constraints kept.** No decorative motion: every row above answers "what changed". Nothing travels more than 10px. No count ticks. A page change is one fade and the arrival already built (§7a); nothing slides. Reduced motion is honoured in JS through `stillMotion()` and in CSS through the one block at `globals.css:671`.

**Motion tests**

1. `design-system.test.ts`: the five new tokens exist; no `40ms` literal and no fold calc remain in `globals.css`; `LIGHT_LEAD_MS` and a new `FOLD_MS` mirror the CSS; `motion-press` and `motion-settle` are gone.
2. Notice: with `open` turned false it is still in the document, marked closing, until tint plus fold have passed (fake timers); under `stillMotion()` it is gone at once.
3. CountChip and CardHeader: a number that changes between two reads renders both layers of a Crossfade for one tint; the first read renders one.
4. The TrueNAS page: when the last finding leaves, the region count changes only after its row has gone; the empty state carries `resolved` and `lift`, and `bloom` only when the reader's act emptied the list.
5. SectionPill: every segment renders an arrow seat; the current one's is hidden from sight and from assistive tech; one GlideIndicator is present.
6. ReconcileControl: a new result changes the Crossfade's id; the row has `flash` only when the result differs from the last read.
7. The regions wrapper carries the stagger attribute, and ArriveList's Back and view-switch memory still stop a replay.
8. `glideTo` on a region flashes its heading, not the section.

## 7a · Page to page (owner, 18 Nov 2026)

Q7 flipped: "a cohesive flow between pages". FINAL §4's "No route transitions" (`FINAL.md:226, 256`) becomes this.

**The decision.** The page you leave fades out; the page you reach arrives exactly as it does today. Leaving is the only new motion. It needs no new token and adds no time, because it plays under the new header's tint.

| Part | What it does | Tokens |
|---|---|---|
| Rail, top bar, phone tab bar, the column's frame | still, always | |
| The page you leave (the main column's content) | fades out where it was scrolled to; no travel | `--duration-tint` 120ms, `--ease-tint` |
| The page you reach | no page-level motion. M1 as built: header tints in over 0 to 120ms; blocks arrive from 120ms, 40ms apart, cap 5, each 420ms over `--travel-arrive` 8px on `--ease-arrive` | unchanged |

The old page is gone at 120ms, as the first block starts to rise (closing fades first; light leads shape). The last block settles at 700ms, as today. The incoming snapshot is `animation: none`, so no page animates twice: M1 is the arrival. Nothing slides: a sliding column breaks the 10px cap, and the rail is a set of places, not a stack.

| Case | What happens | What holds it |
|---|---|---|
| The path changes: a link, the rail, Go to, a redirect | leave, then M1 | the boundary is keyed on `usePathname()` |
| Back, Forward, reload | still (S11) | React: "animations won't run for the back button"; `returnedToThisPage()` stills M1 (`arrive.tsx:29`) |
| Same path: a hash jump, filter, sort or search (`users/page.tsx:131`) | no page motion; `glideTo` and lists keep theirs | key unchanged; `default="none"` |
| Basic ↔ Advanced | no page motion; Unfold as built | `setView` is a plain `setState` (`ui-view.tsx:75`), which never starts a view transition; key unchanged |
| A slow page | the old page stays, live, until the new one can paint; then leave, header, skeleton after 200ms as built | React holds the transition; no `loading.tsx` exists; reads are client-side |
| Phone | the same; a nav sheet closes on its own settle | it sits outside the column |
| Reduced motion | still: the pages swap | the block at `globals.css:777` must name the `::view-transition-*` pseudo-elements (its `*` misses them; React leaves this to us) |
| No API in the browser | still, never broken | Next: "Without browser support, your application works normally; the transitions do not animate." |

**Mechanism.** React's `<ViewTransition>`, as Next documents for the App Router ("route navigations are transitions, so `<ViewTransition>` animations activate automatically during navigation"), keyed as both docs show (`<ViewTransition key={url}>`). Not `document.startViewTransition` by hand: React says "you should never do that yourself", and every link and `router.push` would need wrapping. Not CSS alone: CSS cannot hold a page that has unmounted. Sources: [Next guide](https://nextjs.org/docs/app/guides/view-transitions) · [React reference](https://react.dev/reference/react/ViewTransition) · [React Labs](https://react.dev/blog/2025/04/23/react-labs-view-transitions-activity-and-more) · [Next 15 flag](https://nextjs.org/docs/15/app/api-reference/config/next-config-js/viewTransition) · [MDN](https://developer.mozilla.org/en-US/docs/Web/API/Document/startViewTransition) ("Since October 2025, this feature works across the latest devices and browser versions").

**When: with the Next 16 slice (`openspec/NEXT.md:42`), not on 15.5.** On 15.5.29 it is `unstable_ViewTransition` behind `experimental.viewTransition`, which Next 15's page calls "not recommended for production use", and the flag moves all of Syndra onto React's experimental build (`next/dist/lib/needs-experimental-react.js:11-12`). The current guide says "View transitions work in the App Router with no configuration" and "React 19.3 includes the `<ViewTransition>` component". **UNVERIFIED:** which 16.x ships React 19.3; the slice picks that or later.

**Files**

- `ui/src/lib/motion/page.tsx` (new, about ten lines): `PageChange` renders `<ViewTransition key={pathname} default="none" exit="page-leave" enter="page-arrive">`; exported from `index.ts`.
- `AppShell.tsx:93-101`: mount it once. Try it around `#app-scroll` first: a captured element escapes its ancestors' clips, and the scroller's own clip keeps a scrolled old page off the top bar. If remounting the scroller costs Back its scroll position, wrap `{children}` (`:100`) and raise the top bar's and tab bar's groups by `z-index`. **UNVERIFIED** until spiked, as is `exit` and `enter` firing from a keyed boundary in a layout (Next's guide puts the wrapper in each page).
- `globals.css`, beside `page-head`: `::view-transition-old(.page-leave)` plays `fade-out` (`:325`) on tint; `::view-transition-new(.page-arrive)` is `animation: none`; `::view-transition` takes no pointer events, so the rail answers during the fade; all three join the reduced-motion block.
- `FINAL.md:226, 256` and the canvas copy. No change to `nav.ts`, `arrive.tsx`, `PageHeader` or any page.

**Tests and guards**

1. `design-system.test.ts`: the leave rule uses tint's two tokens and no literal; the arrive rule is `animation: none`; the reduced-motion block names the pseudo-elements; no new motion token.
2. Guard: `ViewTransition`, `startViewTransition` and `view-transition-name` appear only in `lib/motion/page.tsx` and `globals.css`; `next.config` has no `experimental.viewTransition`.
3. `PageChange`: a new path remounts its child; a new query or hash on the same path keeps the instance.
4. Basic ↔ Advanced: with `document.startViewTransition` stubbed and motion opted in (`__motionInTests`), `setView` never calls it and the page instance is kept.
5. Without `document.startViewTransition` (jsdom's default), a path change renders the new page and throws nothing.
6. By eye, both themes, 1440 and 390, Chrome and Safari: a scrolled page leaves without touching the top bar or tab bar; Back, a filter and the view switch are still; reduced motion swaps.

The two questions still the owner's (when; a fade alone or a fade that sinks) are in §9, Q7.

## 8 · What was kept, and what was set aside

Kept as decided: 28 under a page header; 16 between cards; the stat figure at 24; the 12.5px floor; every phone layout and the 44px touch floor; the rail at 252; both themes, with no colour token touched; structure that never moves with data; Basic and Advanced in their seats; FINAL §4's motion rules, unchanged but for the page change (§7a).

Set aside: a density switch for the reader (one density, chosen well, is the calmer product); a narrower rail; a third column on Home; a second pane beside the People list (a real gain at 1800, and its own project); a progress bar for Check now (a check has no steps Syndra could honestly count).

## 9 · The owner's questions

Each had a default the build spec used; the owner answered all eight on 18 Nov 2026.

1. **How much tighter?** A (32 / 24 / 18), B (28 / 22 / 17) or C (24 / 20 / 16): part 1, frames A to C. **Decided (owner, 18 Nov 2026): B.**
2. **A wide window:** a 1280 frame centred between the rail and the edge (part 3, frame 13), or left against the rail. **Decided (owner, 18 Nov 2026): centred.** Settled as shipped (owner, 21 Nov 2026).
3. **Three body sizes:** fold today's six (15, 14.5, 14, 13.5, 13, 12.5) into three (14, 13, 12.5) in a mechanical second pass. **Decided (owner, 18 Nov 2026): yes.** The eight-step scale is settled as shipped (owner, 21 Nov 2026).
4. **Name:** one more row of the settings card the TrueNAS page opens with ("Change name", like "Choose folder"), removing the Name region (part 2, frame 6); or its own region, tightened (frame 5). **Decided (owner, 18 Nov 2026): the row.** Built: Name is the first row of the settings card in every state, Change name opens the one rename dialog, and the Name region is gone.
5. **"Nothing for you to do." in a table:** on every healthy row (part 2, frame 10), or silence for normal, with a step written only where there is one (frame 11; G17 and G12 against the next-step rule). **Decided (owner, 18 Nov 2026): silence.** Built for table rows only: a healthy row with nothing to do says only its state; every other row, outcome, notice and empty state still ends on a step or "Nothing for you to do." (copy guide, `next-step.test.tsx`).
6. **Home:** the two groups stacked, blocks side by side where each has 600px (part 3, frames 15 and 16), or the two pinned columns kept. **Decided (owner, 18 Nov 2026): stacked.**
7. **Page change:** **decided, owner 18 Nov 2026: pages flow.** The page you leave fades on tint and the page you reach arrives as M1; the spec is §7a. Two questions remain. (a) When: with the Next 16 slice, or now on 15.5 on React's experimental build. **Recommended, pending owner: with Next 16.** (b) The leaving page: a fade alone, or a fade that also sinks 4px (`--travel-nudge`, as a dialog's card does). **Recommended, pending owner: the fade alone**; movement belongs to what arrives.
8. **The sign-in door:** keep its own loops and timings as a written exemption. **Decided (owner, 18 Nov 2026): exempt.** FINAL §4's exemption is no longer pending.

**Answered in the build, 19 Nov 2026 (the layout audit, PR #219):** Home's "Checked, nothing waiting" links are now one 44px line each until desktop, so the block is taller on a phone and a tablet than A-24 frame 2p draws, where the lines are 24px apart. Each link had been lifted by -10px either side to keep that 24px pitch, but its 44px target then overlapped the link above it and the Claims line, so the targets were not separate. The floor followed is the product's 44px (`--tap`, until desktop): WCAG 2.5.5 Target Size (Enhanced, AAA) and Apple's Human Interface Guidelines. WCAG 2.5.8 Target Size (Minimum, AA) asks for only 24px, which the product exceeds everywhere. From desktop the links are text height, as drawn. If the owner prefers the drawn 24px pitch on touch, the alternative is a 24px target with spacing (2.5.8 met, 2.5.5 not).
