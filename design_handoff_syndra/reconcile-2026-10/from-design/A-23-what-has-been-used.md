# A-23 · What has been used: final, and the build spec for 7s

**Asked by the owner, 15 Nov 2026:** "make the statistics section more elegant, enriching, posh, polished; hide technical in basic … I really like the live member storage page … show the whole page … prioritise highest quality user experience along with motion."

**Settled by the owner, 17 Nov 2026:** "yes to all recommendations … other than big number; yes to recommended even for the extra suggestions. I like big number. Do a refinement run for any out of place or misaligned UI elements." And: "elegant posh elevating enriching user experience."

**Boards (final):** `design/a23-what-has-been-used-part1.dc.html` to `-part6` (1640 wide; one source, the parts differ in one constant; about 5,600 · 4,000 · 4,600 · 4,000 · 3,400 · 6,200 tall by estimate, each under 8,000), with two helpers the frames mount, `a23-usage-card.dc.html` and `a23-band.dc.html`. The canvas's "App screens" page. Parts: 1 the whole TrueNAS page; 2 the states; 3 Network storage, phones, true size; 4 motion; 5 the person page, dark, and who sees her numbers; 6 the person page, light and on a phone. The first board `a23-what-has-been-used.dc.html` is superseded; the option and comparison parts of earlier rounds are retired.

**Status:** settled, not published. Local only. This is the spec slice 7s builds from.

---

## 1 · The decisions

| | Settled | 
|---|---|
| Q1 · days before the record | the window's faint baseline (`--line`); the record's strong baseline starts at a 6px tick. Never hatched, never a zero |
| Q2 · "What wasn't counted" | one fold per card, closed |
| Q3 · the figure's face | **the stat face: Bricolage 24/600, StatCard's figure** |
| Q4 · the meter | drawn once per page, in the storage card; never in the usage card |
| O1 · a 30 days / 12 months switch | no |
| O2 · who used a share | one link per card to Insights › Usage |
| O3 busiest day, O5 export | not now |
| R1 · the home folder's month | under its meter in the storage cards (member and person); the usage card on those pages keeps only the shares |
| R2 · the retention card | Advanced only |

## 2 · How the big number stays calm

The earlier stat-face drawing had three weaknesses: a column of tiles, a card a third taller, and three display figures under the storage card's display headline. The final design solves each without making the number smaller:

- **The row is still a folder row** (`PlaceBlock`, `MyStorage.tsx:532`): the name left, its number right. The band sits between them. The figures form one right-aligned column in the display face, a ledger rather than tiles.
- **One row, one baseline.** Name, the band's baseline and the figure sit on one line; a second line (the 12-month figure, or a level's change) hangs under the figure in faint. A row is one line tall, so the card is shorter than either earlier version.
- **One band column per card.** Every band has the same left and right edge, so the days line up down the card: a day the NAS was off is one hatched stripe through every row, and each section needs only one axis.
- **Fewer figures where the headline is.** With R1 the member's card holds the shares only, and its figures sit in the right column, away from the headline's left edge.
- **Phone.** The row stacks: name, figure, band. This was the stat face's real win, and it is kept.

## 3 · The usage card (`components/usage`)

### 3.1 Anatomy

```
Card  role=group  (no header on the target page: the Region is the title; CardHeader on a person's or member's card)
├ section  px-5 py-[18px], row-divider between sections; pt-1.5 for the first section under a CardHeader
│   STORAGE SESSIONS                                                     ← h3, type-label, only when the metric has several rows
│   ┌ name 200 ──┬ band, the rest ───────────────────────┬ figure 240 ─┐
│   │ makers     │ ▮▮▮▮ ▮▮▮▮▮ ▮▮▮ ▒ ▮▮▮▮ ▮▮▮▮▮ ▮         │ 588 in the last 30 days │   one baseline
│   │            │                                        │ 7,081 in the last 12 months │   13 faint
│   ├────────────┼────────────────────────────────────────┼─────────────┤   hairline, 12px above and below
│   │ gitlab_data│ …                                      │ …           │
│   └────────────┴────────────────────────────────────────┴─────────────┘
│                 17 Oct                           15 Nov                 ← the section's axis, inset to the band column
│   2 accounts Syndra does not manage also used it. They are in the totals.   ← 13 faint, when it applies
├ section  (a metric with one series is one row, its label in the name seat)
│   │ HOME FOLDER USED │ ╱▔▔▔▔ line + area │ 629.0 GiB on 14 Nov │
│   │                  │                    │ Up 17.2 GiB since 17 Oct │
├ › What wasn't counted                                                  ← one per card, closed
└ Nothing for you to do. Who used each share is in Insights.  Open Usage →      Counting began 6 Nov · Last counted 2 minutes ago
```

### 3.2 Sizes and tokens

| Part | Spec |
|---|---|
| Section | `flex flex-col gap-3 px-5 py-[18px]`, `row-divider` except the first; first section under a `CardHeader` `pt-1.5` |
| Section label | `h3.type-label text-label`, on its own line when the metric has several rows; in the row's name seat when it has one |
| Row | `grid grid-cols-[200px_minmax(0,1fr)_240px] gap-x-6 items-start py-3`, `row-divider` between rows, first `pt-0`, last `pb-0` |
| Name seat | `min-h-10 flex items-end`, `text-[15.5px] font-semibold leading-[1.25] [overflow-wrap:anywhere]`; the name is the series' dimension values joined by " · ", capped as today (`usage.ts` `capText`), no label, no quotes |
| Band seat | the band, 40px tall, width measured (ResizeObserver, fallback 320) |
| Figure seat | `flex flex-col items-end gap-[3px] text-right`; first line `min-h-10 flex items-end justify-end`, inside it `inline-flex flex-wrap items-baseline justify-end gap-x-[7px] leading-none`: the value `font-display text-[24px] font-semibold leading-none text-ink` (StatCard's figure, `StatCard.tsx:45`) and its window words `text-[13px] text-muted`; second line `text-[13px] tabular-nums text-faint` |
| Axis | once per section, the rows' last child: `flex justify-between pt-1.5 pl-[224px] pr-[264px] text-[12.5px] tabular-nums text-faint`; left the window's first day, right the readout seat (`Crossfade`, `aria-live="polite"`) |
| Quiet lines | `text-[13px] text-faint max-w-[80ch]` under the rows: `unbound` (totals), `leftOut` (a member's own card) |
| Fold | `row-divider px-5 py-3.5`; `FoldChevron` + "What wasn't counted" `text-[14px] font-semibold`, `Unfold`; lines `pl-[38px] text-[13px] text-muted`, the last sentence `text-faint` |
| End row | `row-divider flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1.5 px-5 py-3.5 text-[13px] text-muted`; left the step and, on a totals card, `ArrowLink`; right `text-faint` |
| Below tablet | the row is one column, `gap-y-1.5`: name, figure (left-aligned, no min height), band; the axis loses its inset |

Band (replaces `Spark`): height 40, `PAD_Y` 6; baseline `--line-strong` 1px from the record's start, `--line` before it, a 6px tick at the start when it is inside the window. A counter is columns: `--accent`, 45% of a day's step wide, 1px radius, a real 0 a 1.5px stub; the first and last columns end at the band's edges (the band's horizontal inset is half a column). A level is a 2px `--accent` line with its area in `--accent-soft`, dots r 2.5 ringed in `--surface-1`; `PAD_X` 3. With a limit, the scale's top is the limit, a faint `--line` there and "{limit} limit" at its right in 12.5 faint. Hatch: 1px every 6px in `--chart-hatch`, full height (3:1 kept, `globals.css:511-514`, `:587`). The read day's column: `--tint-3`. Runs and the never-interpolate rule are unchanged (`Spark.tsx:72-78`).

Tokens used: `--accent`, `--accent-soft`, `--surface-1`, `--line`, `--line-strong`, `--chart-hatch`, `--tint-3`, `--ink`, `--muted`, `--faint`, `--label`, `--divider`; `--warn`, `--warn-text` in the storage cards' meter. No new token.

### 3.3 What each figure says

- Counter: "{v}" + "in the last 30 days" ("in the last {n} days" when the record is shorter, `copy.ts:27`); second line "{v} in the last 12 months", not said when both windows start on one day.
- Level: "{v}" + "on {day}"; second line its change over the record (§6), the latest value against the first counted day.
- No value: the copy row's sentence in the figure seat, `text-[14px] text-muted`.
- Not counted yet (`watermark === null`): one row for the metric, its label in the name seat, an empty band seat, "Not counted yet" in the figure seat; no axis.

### 3.4 Four kinds of day

| The day | Drawn | Readout and screen reader |
|---|---|---|
| Counted | a column, or a point on the line | "{day}: {value}" |
| Inside the record, couldn't count | hatched full height; no column, the line breaks | "{day}: couldn't count" |
| After the last final day | blank, baseline continues | "{day}: not counted yet" |
| Before the record | the window's faint baseline only | "{day}: before counting began" / "{day}: kept for only {n} days on this system" |

The readout seat is the section's: today at rest; on hover, focus, ← and → in any of its bands, that day's words, led by the row's name where the section has several rows ("makers, 9 Nov: couldn't count"). Each band is one focusable `role="img"` with the existing label (`copy.ts:127`); the column `<title>`s stay.

### 3.5 The fold and the end row

- **Fold**, present only when there is something to say (G17): the gap lines as 7r groups them (`gapLines`, `usage.ts:65`), merged across the card's metrics when range and reason are the same; in Advanced only (`AdvancedOnly`), per metric with something dropped, "{metric}: the last count did not keep {…}."; last, "A day with no line was not counted. It is not a zero." The buffer gap is not listed.
- **End row, left:** the step. Totals card: "Nothing for you to do. Who used each share is in Insights." + `ArrowLink` "Open Usage" to Insights › Usage (the pattern Insights' own end row uses, `copy.ts:109-110`). A person's or member's card: "Nothing for you to do." Not counted: `NOT_COUNTED_NEXT`.
- **End row, right, faint:** "Counting began {day}" when the record starts inside the window, then "Last counted {age}" from the newest `collected.at` (totals only, `stats.go:201-203`), joined by " · ". A person's card: "Kept for only {n} days on this system" when `kept_from` is inside the window; no Last counted.

### 3.6 States (part 2)

| State | Drawn | Ends on |
|---|---|---|
| Not counted yet | three one-row metrics, "Not counted yet", empty band seats, no axis | "Nothing for you to do. The numbers appear here once Syndra has counted." |
| Day one | faint window baseline; tick at the first day with one column or dot; today blank | step + link · "Counting began 14 Nov · Last counted 2 minutes ago" |
| Ten days in | tick at 6 Nov; "in the last 10 days"; "Up … since 6 Nov" | same, "Counting began 6 Nov" |
| Full window | no tick, no fold, no note | step + link · "Last counted 2 minutes ago" |
| A gap | hatched in every band it touches; the fold; the unmanaged lines | same |
| Add-on paused (read-only) | unchanged: reads keep working; Syndra's reach says what is paused | same |
| Offline | the page's one Notice; the last days blank; UNVERIFIED whether the add-on fills them in or reports a gap on return | step + link · "Last counted 3 days ago" |
| Waiting to retry | the page's cards recede; this one does not (Syndra's own record) | step + link · "Last counted 2 hours ago" |
| Syndra could not read | the one sentence (`copy.ts:50`), no seats | "Reload this page to try again." |
| No statistics reported; loading | unchanged (`TargetStats.tsx:19-44`) | |

## 4 · The storage cards (R1, Q4)

The Home folder row keeps its meter (`Meter`, max 360) and gains its month directly under it, at the meter's width:

```
18.4 GiB of 20.0 GiB
[ meter ················ nearly full ]            ← as built
Up 0.3 GiB since 17 Oct                           ← 13 faint
[ band 40px: line + area against the limit ]      ← the limit line and "20.0 GiB limit"
17 Oct                             15 Nov         ← its own axis, 12.5 faint
```

- `flex flex-col gap-1 max-w-[360px]`; the band scaled to the observed limit (`observedLimits`); a limit that could not be read: no limit line, the band on its own scale; a limit of 0: as "no limit" today.
- Member (`MyStorage.tsx` `PlaceBlock`, the home place): after the `Meter`, before the addresses.
- Person (`PersonStorage.tsx` Home folder row): after the `Meter`, before the limit lines.
- The usage card on those pages no longer draws "Home folder used" (it would be said twice, G12), and never draws a meter. The TrueNAS totals card keeps Home folder used: it is everyone's, and nothing else on that page says it.
- Data: the per-person gauge series the usage response already carries (`view.usage`), the metric whose `against` names the home limit.

## 5 · The pages

- **TrueNAS** (`TargetOverview.tsx:337-340`): the Region, the totals card, and `StatsRetention` inside `AdvancedOnly` (R2). It is the page's last card, so nothing above it moves.
- **Network storage** (`MemberTargetCard.tsx:38-45`): the storage card, then "What I have used" with the shares.
- **The person page** (`PersonAccess.tsx`; `PersonSystems.tsx:90-93`): the storage card, then "What {person} has used" with the shares. Operators are the admin role (`router.go:509-517`); there is no non-admin operator. The member's own card leaves out a line that named someone else and says so (`stats.go:180`, `copy.ts:31`).
- **Basic and Advanced:** Basic keeps every statistic and the fold. Advanced adds the dropped-values lines inside the fold and the retention card.

## 6 · Copy (guide §15)

Changed or new rows; every other row stands and stays held by `usageCopyGuide.test.ts`.

| When | Says |
|---|---|
| Gaps: under the lines (was "A day left blank …") | A day with no line was not counted. It is not a zero. |
| The fold's heading (was "Couldn't count these days", which stays as the list's screen-reader name) | What wasn't counted |
| A chart's hover, a day before counting began | {day}: before counting began |
| The end row, when the record starts inside the window | Counting began {day} |
| A level's change over the record, up | Up {difference} since {day} |
| The same, down | Down {difference} since {day} |
| The same, within rounding | Unchanged since {day} |
| A level drawn against its limit: the line's word | {limit} limit |
| The readout in a section with several rows | {name}, {day}: {value} |
| A dropped-values line in the card's fold (Advanced) | {metric}: the last count did not keep {…}. |
| The totals card's step | Nothing for you to do. Who used each share is in Insights. |
| Its link | Open Usage |

A row's name is the add-on's value, capped, without the dimension's label or quotes; the label and quotes stay in sentences (the fold). The buffer gap's identification is 7r's (UNVERIFIED: by its reason code or by `start` at the window's start).

## 7 · Motion

| Moment | Movement | Token | Still form |
|---|---|---|---|
| First paint, navigated to | S11: the header tints in, then the page's cards arrive 8px up, staggered 40ms, cap 5. The usage card is one arriving card; rows, bands and figures are drawn complete inside it | tint 120 · arrive 420 `--ease-arrive` · `--arrive-step` 40, cap 5 | everything at once |
| A later read | rows keyed by series hold still; a day that gained a value has its column, or its dot and segment, tint in; Last counted crossfades; nothing arrives, nothing blooms | tint 120 (Crossfade) | words swap at once |
| The viewport changes | the band re-measures and redraws at once | none | same |
| The fold opens | Unfold: height on settle, lines tint in behind, chevron turns 90° on settle | settle 260 `--ease-settle` · tint 120 · `motion-turn` | opens at once |
| The fold closes | fades first, then the gap settles shut | tint 120, then settle 260 | leaves at once |
| Reading a day (hover, focus, ← →) | the column takes `--tint-3`; the section's readout crossfades; colour only | tint 120 (Crossfade; aria-live) | changes at once |
| Basic to Advanced | the dropped lines and the retention card open by Unfold, close fade first (A-08 §3) | settle 260 · tint 120 | appear and leave at once |
| Press on the fold's control | 3% press | press 180 `--ease-press` | press survives |
| The link to Insights, on hover | A-08 2a: its underline tints in | tint 120 | same |
| Never | a column that grows, a line that draws itself, a number that counts up, a bloom for a count Syndra ran, arrive replaying on refresh | | |

Reduced motion is `stillMotion()` (`lib/motion/still.ts:27`) and the global rule (`globals.css:657-709`).

## 8 · Build list for 7s

No backend change. No new design-system part.

| File | Change |
|---|---|
| `ui/src/lib/usage.ts` | `plot`: a fourth cell state `before` (a day before `known.from`, or inside the buffer gap passed as `known.began`); `gapRuns` excludes it; `recordFrom`. Add `change(cells)`: the first and last counted cells and their difference. Names: `seriesName(metric, dims)` = the dimension values joined by " · ", capped |
| `ui/src/components/usage/Spark.tsx` → the band | props `{ plot, instrument, limit?, readIndex, onRead }`; width by `ResizeObserver`; columns for a counter, line plus area for a gauge; the faint pre-record baseline, the tick, the limit line and word; the read column; `tabIndex=0`, ← → move the read day; no axis inside |
| `ui/src/components/usage/MetricBlock.tsx` | the section and row of §3.2; the label on its own line or in the name seat; the figure seat; one axis and one `Crossfade` readout per section, fed by its bands' `onRead`; `unbound` and `leftOut` as faint lines. Remove `Gaps`, `Collected`, the meter and the `limits` prop |
| `ui/src/components/usage/TargetStats.tsx` | one fold per card (merge `gapLines` across metrics by range and reason; dropped lines per metric inside `AdvancedOnly`); the end row with the step, the `ArrowLink` to Insights › Usage, and the began and last-counted note |
| `ui/src/components/usage/UsageCard.tsx` | the same fold and end row without the link; drop `limits`; `ViewUsage` filters out the metric whose `against` is the home limit when the view's kind draws it under its meter, and returns nothing when no metric is left |
| `ui/src/components/usage/copy.ts` | the rows of §6: `GAPS_NOTE`, `FOLD_HEADING`, `dayBefore`, `countingBegan`, `changeUp/Down/None`, `limitWord`, `readoutNamed`, `droppedFor`, `OVERVIEW_NEXT`, `OVERVIEW_NEXT_LINK` |
| `ui/src/components/kinds/storage/MyStorage.tsx` | `PlaceBlock`, the home place: the month under the `Meter` (§4), from `view.usage` |
| `ui/src/components/kinds/storage/PersonStorage.tsx` | the Home folder row: the same block under the `Meter` |
| `ui/src/components/targets/TargetOverview.tsx` | `<AdvancedOnly><StatsRetention …/></AdvancedOnly>` |
| `ui/src/components/usage/InsightsUsage.tsx` | accept the link's landing (the Usage tab; a `#` anchor per system is enough) |
| `openspec/changes/plain-language-copy/design.md` §15, `usageCopyGuide.test.ts` | the rows of §6 |

Tests to add: a row's name, band and figure are one grid row; a one-series metric puts its label in the name seat; day one draws the faint baseline, a tick and one column or dot, and the end row's note; a full window draws no tick, no fold, no note; a gap hatches and breaks columns and line; a counter's edge columns stay inside the band; a level's change line, each of the three cases; the limit scale and word; one axis per section and one fold per card; "Last counted" once per totals card and never on a person's; no meter and no Home folder used in a person's or member's usage card, and the month under the meter in both storage cards; the Insights link on the totals card only; the dropped lines and the retention card only in Advanced; both themes' hatch at 3:1 (`chartTokens.test.ts`); **every row mounts its band** (the board's own regression: a guard nobody sets is a band that never mounts).

## 9 · Open, for code to verify (not the owner's)

- How 7r identifies the buffer gap ("counting began").
- Whether an offline add-on back-fills missed days or reports a gap on return (the Offline state draws them "not counted yet").
- Whether Insights › Usage can be linked per system or only to the tab.
