# A-24 · The craft: where Syndra stands, and where it could be a pleasure

**Asked by the owner, 18 Nov 2026:** "learn from the best in the world … understand what interactions, however small, have subconsciously big effects … it wants the user to come back just because the experience is so good … give me rich options and reasons. do great things." And, later the same day: "i dont want syndra to be a boring corporate admin tool. i want it to feel delightful and a pleasure to use and experience. i want it to be inspiring."

**Status:** proposed, not published, local only. Nothing here is built, and nothing changes a standing rule without the owner's answer in §12.

**The retained knowledge:** `design_handoff_syndra/craft/CRAFT.md` (the playbook, with a checklist for every board) and `craft/SOURCES.md` (165 pages, each marked read, secondary or unverified). This answer measures Syndra against that playbook.

**Boards:** `design/a24-the-craft-1.dc.html` to `-3` (2060 wide; one source, the parts differ in one constant; about 6,900 · 6,800 · 5,700 tall by estimate, each under 8,000). Every frame is at 100%: 1440, or 390 for a phone. For the canvas's "Flows and explorations" page.

| Part | What it shows |
|---|---|
| 1 | Arriving: Home's head with the hour's light and the term line, in both themes and on a phone · the first arrival of a session as three key frames · a member's first time with something |
| 2 | Finishing a queue at three settings, and in time · the change's trail · a long wait · empty states with and without a plate |
| 3 | A person's page: the line that knows, and access over time · a year of days · Go to three ways · the small things |

**Not checked, and said so.** The boards have not been rendered (the canvas rule): heights are estimates, and they are checked only mechanically (tags balance, the script runs for all 25 frames, no hex, every hole a plain lookup). Which copy of the design system the canvas holds decides whether the frames show D-1's density: **UNVERIFIED**. Whether Figtree and Bricolage carry tabular figures: **UNVERIFIED**. Other open facts are listed in §13 for code.

---

## 1 · Where Syndra stands

Read against `CRAFT.md`, Syndra is already what the best sources call a productive tool. That is worth saying plainly, because it decides where the effort goes.

**Built, and at the level of the best**

| The craft says | Syndra does | Where |
|---|---|---|
| Budget motion; stay in the 70 to 400ms band; exits quicker | Six movements on one curve; tint 120, press 180, fold 200, settle 260, arrive 420 | `globals.css:258-297` |
| Nothing under a second | Skeleton only after 200ms; it fades before rows arrive | `lib/motion/still.ts:27` |
| Never optimistic about access | "Not sent to Zitadel yet", "Sent, not seen back yet", "In Zitadel" | `lib/standing.ts:34-50` |
| The gist, then the evidence; the reason beside the surprise | Plan folds by how a change travels, a summary line, a because-note | `lib/plan-groups.ts:31-35, 154, 244-247` |
| What you reviewed is what runs | "Nothing was applied. This is a fresh preview." | `RehearsalDialog.tsx:412-415` |
| Design the ending | A word in the act's own vocabulary, one sentence, a next step; it lifts and blooms once | `ActionOutcome.tsx:155-186` |
| Unknown is never zero | "A striped day was not counted. Neither it nor a day not counted yet is a zero." | `usage/copy.ts:42-47` |
| A greeting that is true | "Evening, Kanishk. Three things need you." from the page's own reads; "It will keep until morning." | `Home.tsx:367-375, 441-451`; `lib/greeting.ts` |
| Nothing jumps | Back restores scroll, paging and folds; a gap waits while the pointer is in the list | `lib/place.ts`; `Home.tsx:519-560` |
| A message has one place | Nothing is a toast, nothing times out | RULES G26, G27 |

**The gaps.** Small, and of two kinds.

*Things a person feels and cannot name:*
- A page arrives every time it is opened, however often (`lib/motion/arrive.tsx:29`). The hundredth visit to People plays the same 700ms as the first.
- Go to is opened by a key and rises like any dialog (`GoTo.tsx:38`, a Modal).
- Figures are not tabular as a rule: the class is in 12 files and absent from the stat figure (`MetricBlock.tsx:76`).
- Past ten seconds a wait says nothing new, except on one card (`MyStorage.tsx:594`).
- Focus does not return to the row you opened. `scrollbar-gutter`, `caret-color` and `overscroll-behavior` appear nowhere in `ui/src`.
- Reduced motion collapses every duration, tint included (`globals.css:777`).
- Usage figures and Trends' counts are plain text with no way in (`MetricBlock.tsx:70-77`, `TrendsTab.tsx:46`).

*Things that would make it Syndra's own:*
- The one authored moment is the sign-in door. Inside, every screen is equally calm, so no moment is remembered.
- A person's page opens on a sentence that is the same for every person (`PersonAccess.tsx:70-71`), where My access opens on a line that knows (`member/headerLine.ts:62-128`).
- The end of a queue looks like any empty card. Home's all-clear does not even pass `lift` or `bloom` (`Home.tsx:441`).
- A change's journey to Zitadel is said in one word per row. Nothing shows the route.

So the plan has two halves. **Finish the foundation** with a dozen small things nobody will notice and everybody will feel (§11). **Then spend expression in a few rare places**, each drawn at three settings (§2 to §10).

**The dial.** Quiet is the product as it is, or one honest step from it. Warm adds one thing that is true and kind. Vivid is the bold version, allowed only where a moment is rare. The guardrails hold at every setting (`CRAFT.md` §0.4): nothing expressive on a consent, a failure or a loss of access; everything shrinks with repetition; every movement has a still form; nothing asks anyone to come back; every warm sentence comes from the same read as the numbers beside it.

Cost is **S** (a day or less), **M** (a few days), **L** (a slice of its own).

---

## 2 · The first second of every page

**Built:** the shell is still; the header tints in; blocks arrive 420ms over 8px, 40ms apart, capped at five; never on Back, a filter or a sort (`arrive.tsx`). D-1 §7a adds a 120ms fade of the page you leave. A person's name is on screen before their page has loaded (`PersonAccess.tsx:122-145`).

| Setting | What | Why | Cost | Risk | Tokens |
|---|---|---|---|---|---|
| **Quiet** · recommended | **A place arrives once.** The first visit to a path in a session arrives as built. Any later visit: the header tints, the content is there. A visit made by key (Go to, Enter on a row) never arrives | CRAFT §0.2, §1.3: an arrival that informs the first time is latency the thirtieth | S: one map of paths in `arrive.tsx`, beside `returned` | A page that reads slowly looks abrupt on a second visit. The skeleton rule already covers it | none new |
| **Warm** | The same, and the row you opened keeps its tint for one flash on your return, so the eye lands where it left (`data-row-current` exists, `lib/keys.ts:55-60`), with focus restored to it | CRAFT §2.8: the pointer's target survives | S | none | none new |
| **Vivid** | **The name carries you.** The row's avatar and name travel to the page's title (260ms, settle's curve) while the list fades; Back is still, as now | CRAFT §1.11: the thing you opened is the thing you are in. Research 6 ranks it the most premium single interaction a console can have | M, with the Next 16 slice that §7a already waits for; `view-transition-name` on one element per row | It breaks the 10px travel cap, for one named case. Duplicate names skip the transition. Opening people is frequent, so it must never delay the page | changes FINAL §4: one written exception to `--travel-cap` |

**Recommendation: Quiet and Warm now. Vivid as a spike inside the Next 16 slice, judged on staging at the hundredth open, not the first.** If it is still welcome then, it stays.

---

## 3 · Arriving: Home, the door, and a member's own page

**Built:** Home's head is the best-written screen in the product: greeting, a sentence that counts blocks, the date line, the claims line, "Checked, nothing waiting:" (`Home.tsx:360-397`). The mark's dot blooms once after sign-in (`SyndraMark.tsx`). My access has its one line that knows.

### 3.1 Home (part 1, frames 1 to 4)

| Setting | What | Why | Cost | Risk | Tokens |
|---|---|---|---|---|---|
| **Quiet** | As built, with §2's arrival rule | The words are already right | none | Stays undistinguished | none |
| **Warm** · recommended | **The room knows the hour.** A still pool of violet light behind the head, whose place says the time of day: low and left in the morning, overhead in the afternoon, a lamp beside the words in quiet hours. And **the term line**: a hairline for the term, the days gone filled, today a violet point, an amber mark on each day someone's direct access ends in the next 14 | CRAFT §9.3: ambient information is read before any word. Light is already Syndra's material. The term line is a number beside its history (§6.4) | M: one class, three custom properties set from the configured zone, one small part | The light is colour used for atmosphere, which `globals.css:10` forbids. The term line repeats the Ending soon block in another form, so it carries no words of its own | new `--hour-x`, `--hour-y`, `--hour-reach`; one written exception to "colour is never decorative" |
| **Vivid** | Warm, and **the mark says the room's state** in the rail at rest: violet when something needs you, lime when nothing does, amber and breathing when Zitadel isn't answering, dim in quiet hours | CRAFT §9.8: a mark, not a mascot. Lime as a dot is exactly what `--healthy` allows. It is the periphery doing a headline's work | S: `--mark-dot` already exists | A second place that says what Home's headline says. It is a dot, not words, and it is on every page where the headline is not | none new |

### 3.2 The first arrival of a session (part 1, frames 5 to 7)

Once, straight after sign-in. Never again until the next sign-in.

| Setting | What | Why | Cost | Risk | Tokens |
|---|---|---|---|---|---|
| **Quiet** | As built: the mark's dot blooms once | Already there | none | Easy to miss | none |
| **Vivid** · recommended, built last | **The door's light comes with you.** At 0ms the head holds only the doorway's arch of light. By 260ms it has opened into the hour's pool and the greeting has tinted in over it. By 700ms the lines are in and the blocks arrive as today | CRAFT §9.6: a payoff only for a rare, real event. §1.11: where it came from is where it lives. It costs no time: a page takes 700ms now | M, after 3.1 | A second of ceremony for someone who signs in to fix one thing. It never holds input, and reduced motion skips to the last frame | one keyframe, `door-open`, played once |

### 3.3 A member's own page (part 1, frames 8 and 8p)

For most people who ever see Syndra, this page is Syndra.

| Setting | What | Why | Cost | Risk | Tokens |
|---|---|---|---|---|---|
| **Quiet** | As built: "From today, you can use Laser cutting." under the greeting, and the jump to its card | True and placed well | none | The card itself looks like every other | none |
| **Vivid** · recommended | **The card is lit, once.** On the first visit after access starts, that project's card carries the door's light on its top edge, a violet hairline, and "From today" as its note. It stays in its alphabetical place. On the next visit it is a card like the others | CRAFT §9.7: the first sight of something new is the moment that makes a member feel the space is theirs | S: the ladder already knows the case (`headerLine.ts`) | None to trust. Lighting is decoration, so it shares 3.1's exception | the same light |
| **Warm, for storage** | One sentence under the meter on Network storage, from a ladder: plenty of room, getting full with what is left, or full with the next step | Research 6 §10.2: how much is left is a human question | S | The owner likes this page as it is; "plenty" is a judgement and needs a threshold | none |

**Recommendation: Warm Home with the mark's states; the lit card; then the first arrival.** The storage sentence only if the owner wants it (Q18).

---

## 4 · Finishing: a task, a queue, a day, a term

**Built:** every act ends on ActionOutcome. An emptied Home block shows its outcome, then folds (Motion 5b). Pending changes has "Everything has reached Zitadel." (S13). `EmptyState` has `lift` and `bloom`, and most callers do not pass them.

The research is blunt: people remember the peak and the end. One act's ending is designed. The ending of a job is not.

### 4.1 A queue you emptied (part 2, frames 9 to 13)

| Setting | What | Why | Cost | Risk | Tokens |
|---|---|---|---|---|---|
| **Quiet** | As built: the lime dot and two sentences | Correct | none | The end of an hour's work looks like an empty list | none |
| **Warm** · recommended everywhere | `lift` and `bloom` whenever the reader's own act emptied it (D-1 M3, on every queue), and **the ledger line**: "You decided 4 requests today. All 4 are in Zitadel." with a link to Change history | CRAFT §9.5. The tool carries the memory of what you did, and closes the loop for you | M: one read (today's acts by this operator, and how many are held) | "Are in Zitadel" must be read back, never assumed: when it cannot be, the line says how many were sent. G17 says silence for normal; this line appears only at the end of a queue, never beside a working list | none new |
| **Vivid** · recommended on Home's all-clear only | **The list closes.** The sentence takes the heading size in the display face; a lime hairline draws along the card's top edge over one flash and rests; a small plate sits in the right seat | CRAFT §9.6. Once a day at most, and only by your own hand | M: one keyframe, one part | `EmptyState`'s comment says no illustration (§7) | a hairline-draw keyframe on flash's duration; the plate |

### 4.2 A day and a term

- **The day.** Home already says "It will keep until morning." in quiet hours. With 4.1's ledger line, Home's all-clear is the day's ending. Nothing else is needed.
- **A term's first and last day.** The date line already says them (A-18). **Warm:** nothing more. **Vivid, later:** *the term in review*, a view on Insights that opens on a term's last day: who joined, what was given and what ended, the busiest week, in sentences and one band. It is the peak-end rule at the scale of a term, and it is a record, not a reward. Cost L. The owner's (Q19).
- **Refused:** confetti, a "celebrate" button, a rotating pool of jokes. The research offers all three. The copy guide says the keeper never jokes, and a tool that guards a door should not throw a party. If the owner wants them, they are Q17.

---

## 5 · Preview, apply, settled

**Built, and strong:** the lede "Nothing changes until you press Apply."; folds "Straight away", "Will wait for you to send", "Left as they are"; a because-note; "Apply to 12 people"; the forecast breathing while both buttons hold; the result "Given to 12 people" with one sentence per part; a stale plan re-previews and stops.

This arc changes access, so it stays exact at every setting. The one thing missing is a picture of the journey.

| Setting | What | Why | Cost | Risk | Tokens |
|---|---|---|---|---|---|
| **Quiet** | As built | Nothing is wrong with it | none | The route is known only to people who know Syndra | none |
| **Warm** · recommended | **The change's trail** (part 2, frame 14). Three steps in Syndra's existing words, drawn by `CheckTrail`: Recorded in Syndra · Sent to Zitadel · In Zitadel. A step is a ring until it is observed, then a dot. It sits under ActionOutcome in the sheet's result and, until a role is held, as the row's detail line. A role that reaches a connected system adds that system's own word as a fourth step | CRAFT §2.4, §2.6: real progress, never ahead of the truth. It teaches the model once, and afterwards is read in a glance | M: `CheckTrail` needs a label (its screen-reader words say "Check" and "passes"); the result needs each step's state, which `lib/standing.ts` already names | A second saying of the row's status. It is the same fact with its neighbours shown; ActionOutcome keeps the only link | none new |
| **Vivid** | The trail's steps fill in turn as each is observed, on tint, and the last blooms when it is your own act on screen | The picture and the truth are the same thing here | S, on top of Warm | None beyond Warm: nothing fills that has not been read | none new |

**Also, where the code can prove it:** a revoke's before-line says what stays ("Her Zitadel account and her files on TrueNAS stay."). CRAFT §8.4. It must be traced through every surface first, so it is listed in §13, not drawn.

**Recommendation: the trail, with its steps filling as observed.**

---

## 6 · Waiting and freshness

**Built:** nothing under 200ms; ReadFreshness with its dot and "Read again"; the old words stay until the new are read (S17); retries say when and which try ("Syndra sends it again at 14:05 (try 2 of 6)").

| Setting | What | Why | Cost | Risk | Tokens |
|---|---|---|---|---|---|
| **Quiet** · recommended | **A long wait says so once** (part 2, frame 15). After 10 seconds the sentence beside a pending button changes, on tint: "Still reading TrueNAS." and, where true, "You can leave this page. The result will be here when you come back." | CRAFT §2.1: past ten seconds a person starts to doubt. A check has no steps to count, so one changed sentence is the honest signal | S: one shared hook; `SLOW_ANSWER_MS` exists for one card | The second line must be true for each caller (§13) | none new |
| **Warm** | The same, and a skeleton's rows match the real row's height after D-1 | CRAFT §2.3 | S | none | none |
| **Vivid** | none offered. A wait is not a moment to decorate | | | | |

---

## 7 · Empty and all-clear

**Built:** `EmptyState`, with `resolved` for a queue that is clear; the keeper's lines on Drift and Unfinished revocations; "Nobody matches that." with Clear the filters. Its own comment: "No illustration, no 'you're all caught up!'".

| Setting | What | Why | Cost | Risk | Tokens |
|---|---|---|---|---|---|
| **Quiet** | As built | It is honest and it tells the four kinds of empty apart | none | none | none |
| **Warm** · recommended now | `lift` and `bloom` on every queue the reader empties (§4.1) | Earned, and already in the vocabulary | S | none | none |
| **Vivid** | **Plates** (part 2, frames 16 and 17). Three drawings from the door, in hairlines with one lit point: a shut door with a lime point for a queue that is clear; an open door with violet light for a list that is only beginning; and nothing at all for a search that missed or a read that failed | CRAFT §7.3: tone follows the cause. A plate where the state was earned or is a beginning; none where it is a miss | M: one part, three drawings in CSS, both themes | It reverses a written stance of the part. A weak drawing is worse than none, so it must be judged on staging, not on a board | a `Plate` part |

**Recommendation: Warm now. Plates only if the owner likes them at true size (Q9).**

---

## 8 · Search and jump

**Built:** Go to on `/` and Mod+K, groups A to Z, only Pages when nothing is typed, "nothing is pinned and nothing is remembered" (`GoTo.tsx:18-19`); arrow keys move between rows in every list; `?` shows the keys.

| Setting | What | Why | Cost | Risk | Tokens |
|---|---|---|---|---|---|
| **Quiet** · recommended now | **Go to appears at once**: no rise, no scrim fade, and it leaves at once. **Escape clears what is typed, then closes.** The same two-stage Escape in every list's search | CRAFT §0.2: a thing opened by a key, dozens of times a day, must feel like no time passed | S: a `still` form of Modal for this one caller | none | none new |
| **Warm** | **Lately.** With nothing typed, the last five places you opened, most recent first, above Pages. Kept on this device only | Recognition over recall; most jumps are back to where you just were | S | It reverses a deliberate choice made for the alphabetical rule. A list ordered by time is one the rule already allows | none |
| **Vivid** | **It answers, and it acts.** A person's name brings one observed line (what they hold, what ends soon) and the acts you could take: Give a role, Manage bundles. Enter on an act opens its own dialog, which still previews and asks | Stripe's and Raycast's lesson: the shortest road from a name to the act | L | An act reachable from anywhere must carry every consent it has today. Nothing is done from Go to itself | none |

**Recommendation: Quiet now; Warm on the owner's word (Q13); Vivid as its own slice.**

---

## 9 · The person page

**Built:** the header paints at once from the row you clicked; tabs; one card per project, folded state kept; a source chip and popover on every role; "held twice" said; standing words per row.

| Setting | What | Why | Cost | Risk | Tokens |
|---|---|---|---|---|---|
| **Quiet** | As built | Complete | none | Opens on a sentence nobody reads after the first week | none |
| **Warm** · recommended | **The line that knows** (part 3, frame 18). Under the name, in place of the lede: what she holds, read from Zitadel, and the one thing worth knowing, from the same ladder as My access in an operator's words. "Holds 2 roles across 2 projects. Operator in Laser cutting ends in 12 days." Also: where a role is held twice, the popover says which source decides ("Remove the bundle and she still holds this.") | CRAFT §6.1, §6.3, and the 50th-time test on a sentence. The page joins the guard's LEDELESS list, as an app's page did (A-15) | M | A ladder must never guess: when Zitadel can't be read it says so, as My access does | none new |
| **Vivid** | **Access over time** (part 3, frame 19). One card: a line per role from the day it was given to the day it ended, on one axis with the terms marked and today in violet. Ink is held today, grey has ended, dashed amber is the time left before an end | CRAFT §6.4, §9.10. It is the picture an operator builds from the Activity tab, drawn once | M to L: each role's given and ended dates for the year | The dates are Syndra's records, so the caption says so, and "held today" is the observed part. In Advanced first | none new; the band's grammar from A-23 |

**Recommendation: Warm. Vivid in Advanced first (Q11).**

---

## 10 · Statistics

**Built and settled (A-23):** one band per row on one axis, a figure in the stat face, days not counted hatched, one fold for what wasn't counted.

| Setting | What | Why | Cost | Risk | Tokens |
|---|---|---|---|---|---|
| **Quiet** | A-23 as settled, with every figure tabular (§11) | The owner chose it a day ago | S | none | none |
| **Warm** · recommended | **A number opens its list.** A figure that counts things you can open is a name link: usage figures to Insights' Usage, "held but not used" to People narrowed. And the band's last counted day carries full ink, so the figure is tied to its place | CRAFT §6.7, §6.4 | S to M | A link needs a list that exists; where it does not, the figure stays plain | none |
| **Vivid** | **A year of days** (part 3, frames 20 and 21). On Insights' Usage only: 53 weeks by 7 days, five steps of violet, a striped cell for a day not counted, the read day said in a sentence, and three findings in the stat face | CRAFT §9.10: weekends, the winter break and the summer show without a caption. It is a picture of the makerspace's rhythm | L: a daily series for twelve months, where the card reads thirty days | Colour is the weakest way to read an exact value, so every cell reads out in words. The record began on 6 Nov: for a year it would be mostly empty. The owner declined a 12-month switch on the usage card (A-23 O1); this is a different surface, and still his to accept | five `color-mix` steps of `--accent`, as chart tokens |

**Recommendation: Warm now. A year of days once counting has run a term (Q12).**

---

## 11 · The small things

Twelve. None is a feature, and all fit today's tokens. Each removes a flicker, a shift or a dead end. Part 3, frame 23 draws three.

| # | The thing | Where | Cost |
|---|---|---|---|
| 1 | **Every figure is tabular.** One rule, read by Time, CountChip, the number seat, StatCard and the stat face | `globals.css`; today 18 uses in 12 files | S |
| 2 | **Only what changed, changes.** A number that goes from 47 to 46 crossfades its last digit and holds the 4 still. Nothing ticks or rolls | `lib/motion/crossfade.tsx` | S |
| 3 | **The page never shifts sideways.** `scrollbar-gutter: stable` on the scroller | `AppShell.tsx:93` | S |
| 4 | **Back gives you your row.** Focus returns to the row you opened | `lib/place.ts`, `lib/keys.ts:55-60` | S |
| 5 | **Escape in two stages** in every search: clear, then close | `FilterBar`, `GoTo.tsx` | S |
| 6 | **A sheet never drags the page.** `overscroll-behavior: contain` on dialogs and sheets | `Modal.tsx` | S |
| 7 | **The caret is Syndra's.** `caret-color: var(--accent)` in every field | `globals.css` | S |
| 8 | **Go to at once** (§8) | `GoTo.tsx:38` | S |
| 9 | **A place arrives once** (§2) | `lib/motion/arrive.tsx` | S |
| 10 | **Reduced motion keeps colour.** Tint survives the reduced block, so a status crossfades instead of snapping; travel and scale still go | `globals.css:777` | S |
| 11 | **A second tooltip is instant.** Once one term is open, its neighbour opens with no delay | `Term`, `WaitsMark` | S |
| 12 | **A number opens its list** (§10) | `MetricBlock.tsx:70-77`, `TrendsTab.tsx:46` | S to M |

**Recommendation: all twelve, as one pass, before anything expressive.** Polish on a foundation with a loose edge is theatre.

---

## 12 · The owner's questions

Each has my recommended answer. The first thirteen are drawn on the boards.

1. **The dial's default.** Quiet, warm or vivid as Syndra's resting character. **Recommended: warm, with vivid in four named places** (the first arrival of a session, a member's first time, Home's all-clear, a year of days).
2. **The light inside.** A still pool of violet behind Home's head that says the hour, as one written exception to "colour is never decorative". **Recommended: yes.**
3. **The term line** on Home. **Recommended: yes.**
4. **The first arrival of a session:** the door's light opens into Home, once per sign-in. **Recommended: yes, built last and judged on staging.**
5. **The mark says the room's state** in the rail. **Recommended: yes.**
6. **The lit card** on a member's first visit after access starts. **Recommended: yes.**
7. **The ending of a queue:** quiet, warm (the ledger line), or vivid (the list closes). **Recommended: warm everywhere, vivid on Home's all-clear only.**
8. **The change's trail** in every result. **Recommended: yes.**
9. **Plates** in empty states, against the part's own "no illustration". **Recommended: the shut door and the open door only, and only after you have seen them at true size.**
10. **The line that knows** on a person's page, replacing its lede. **Recommended: yes.**
11. **Access over time** on a person's page. **Recommended: yes, in Advanced first.** **Built** (owner, 19 Nov 2026: "i dont see the access over time chart anywhere", which settles it as yes; addon-contract 7w-5): `GET /users/{id}/access-history`, operators, and the card on the person page in Advanced (`components/people/AccessOverTime.tsx`).
12. **A year of days** on Insights. **Recommended: yes, once counting has run a term.**
13. **Go to remembers your last five places**, on this device. **Recommended: yes.**
14. **A place arrives once per session, and never by key.** This tightens S11 in FINAL §4. **Recommended: yes.**
15. **Reduced motion keeps tint.** **Recommended: yes.**
16. **The name carries you** from a row to its page's title: one exception to the 10px cap. **Recommended: spike it inside the Next 16 slice, and keep it only if it is still welcome on the hundredth open.**
17. **Humour, a "celebrate" action, a pool of rotating lines.** The research offers them; the copy guide's keeper never jokes. **Recommended: no. Warmth stays exactness at the right moment.**
18. **One sentence under the storage meter** ("Plenty of room."). **Recommended: yes, if you want any change to a page you like; otherwise leave it.**
19. **The term in review**, on a term's last day. **Recommended: later, as its own slice.**
20. **The makerspace's own accent or name on the door.** Research 6 suggests a space accent; `globals.css` says violet carries both rooms. **Recommended: no accent choice. The space's name on the door, if Makerspace settings is to hold one, is a separate question.**

---

## 13 · For code to verify (not the owner's)

- Whether Figtree and Bricolage, as loaded by `next/font` (`app/layout.tsx:19-40`), carry tabular figures. If Bricolage does not, the stat face keeps proportional figures and only its column aligns.
  **Answered (code, 18 Nov 2026, wave 1):** both do. Read from the font files themselves (`ui/src/app/fonts/*.woff2`, through the fontkit that ships in `next`): Figtree and Bricolage both list `tnum` and `pnum`, and their default figures are proportional (Figtree "0/1/4/7" are 638/410/616/530 units; with `tnum` every one is 620. Bricolage 594/310/607/498, and 580 with `tnum`). JetBrains Mono has no `tnum` and needs none: every glyph is 600. So one rule, `[data-figure]`, makes every figure tabular, the stat face included, and prose keeps its proportional figures.
- Whether a check's result (Check now) is kept when the page is left. If not, §6's second sentence is cut for that caller.
  **Answered (code, 18 Nov 2026, wave 2):** it is not. Check now is a mutation (`useReconcileTarget`, `POST /targets/{t}/reconcile`) held by the card; its answer lives only in that component, no GET returns a last check, and the handler stores none (it audits `target.checked` with whether it failed or halted, no counts). On return the card says "Syndra checks by itself every six hours." So Check now says only "Still reading {system}." after ten seconds, never "You can leave this page". No other long wait in wave 2 can truthfully say it either: the member's password set (`MyStorage`) already asks to keep the page open. The second sentence is built nowhere.
- Whether Escape already clears a FilterBar's search before anything else. **UNVERIFIED.**
  **Answered (code, 18 Nov 2026, wave 1):** it did not. FilterBar's search was a plain `Input` with no key handling, so Escape did nothing in it, or went straight on to what held it (on People it left Select). Wave 1 gives every search field `type="search"` and `Input` the two stages: the first Escape clears, and only an empty field lets Escape go on.
- What a revoke leaves in place on each connected system, traced through every modal, plan row, outcome and audit word, before any "what stays" line is written (§5).
  **Answered (code, 18 Nov 2026, wave 2), and no "what stays" line is written yet.** Revoking a direct grant queues a revoke (`drain.go` reads the live grant, then `RemoveUserGrant`, or `UpdateUserGrant` with the roles left): only the project grant changes, the person's Zitadel user stays, and a role a bundle or rule still gives is kept (`retained_roles`). On TrueNAS, losing the last mapped role converges the account to locked (`enabled` false), the SMB flag off and managed group membership replaced; built-in groups are untouched (`addons/truenas/README.md`). The home folder and its files stay ("A home is never removed", `home.go`), and so does a home quota nobody gives any more. Nothing is deleted: only the separate, elevated `account.purge` removes an account. A revoke on a connected system's own page (`revocation.go`) holds `enabled`, rotates the storage password, and leaves roles as they are; open SMB sessions last until they reconnect. Today neither revoke's confirm says that the account, the home folder and the files stay. The line ("Their Zitadel account and their files on {system} stay.") is true on every path above, but it is not added here: it changes a consent, the owner's to see first, and it needs the kind's own words per system. Found on the way and fixed in the wave 2 review round: the target revoke's result said "Revoked" while the lock was only queued (now On its way), and a direct revoke Zitadel accepted said "the access is gone" (now that Zitadel took it, not yet read back).
- Whether today's acts by one operator, and how many of them are held, can be read in one call (§4.1).
  **Answered (code, 18 Nov 2026, wave 2): no.** `GET /audit` filters by `user_id` as actor *or* target, pages by a keyset cursor and caps at 200; it has no `since`, no actor-only filter and no action filter, and no row says whether its change is held. `confirmed_at` (observed in Zitadel) is on the outbox row only, and no read returns it. So the ledger line ("You decided 4 requests today. All 4 are in Zitadel.") is **not built**. The smallest read that would make it true, proposed and not built: `GET /me/today`, read-only, operator-gated, answering `{decided, sent, in_zitadel, since}` for the signed-in operator from `audit_logs` (actor = them, created_at at or after the start of the makerspace's day in its configured zone, the decision actions) joined to each act's outbox rows (`status` applied for sent, `confirmed_at` for in Zitadel). Until it exists, every queue ends warm without it.
- Whether `<ViewTransition>` can name one element per row without duplicate names when a person appears twice on a page (§2, vivid).
- What the sign-in door says after sign-out. Not read. A closing line there would be the same idea as §4's day.

## 14 · What would change, in one table

| | Inside today's tokens and parts | Would add or change |
|---|---|---|
| §11, all twelve | yes | one tabular rule; a still form of Modal for Go to |
| §2 quiet, warm | yes | S11's wording in FINAL §4 |
| §2 vivid | no | one exception to `--travel-cap` |
| §3 the light, the lit card | no | `--hour-x`, `--hour-y`, `--hour-reach`; one exception to "colour is never decorative" |
| §3 the term line, the mark | yes | one small part; `--mark-dot` exists |
| §3 first arrival | no | one keyframe, `door-open` |
| §4 ledger line, §5 trail, §6 wait, §9 the line | yes | a label on `CheckTrail`; two reads |
| §4 vivid, §7 plates | no | a hairline keyframe; a `Plate` part; `EmptyState`'s stance |
| §8 Lately | yes | Go to's "nothing is remembered" |
| §9 access over time | yes | the band's grammar, reused |
| §10 a year of days | no | five chart steps of `--accent`; a daily series |

## 15 · Ranked by effect on the person

1. The twelve small things (§11): felt every minute, by everyone.
2. A place arrives once, and Go to at once (§2, §8): the tool stops making you wait for it.
3. The change's trail (§5): the model of the whole product, in three dots.
4. The line that knows on a person's page (§9): the first line earns its place every visit.
5. The room knows the hour, and the term line (§3.1): Home becomes a place.
6. The end of a queue, with its ledger line (§4.1): the job has an ending.
7. A member's first time, lit (§3.3): the page most people see, at its best moment.
8. The first arrival of a session (§3.2): the one second of ceremony.
9. A long wait says so once (§6).
10. A number opens its list, then a year of days (§10).

## 16 · How the boards read

Each part opens with three cards: the dial, what holds at every setting, and that part's own stance. Every frame's caption names its setting. The recommended frame in a group is edged in violet. Motion is drawn as key frames, each with its time and the tokens it uses. People are ring names; the operator is Kanishk Singh. The year of days is sample data, and says so on the card.
