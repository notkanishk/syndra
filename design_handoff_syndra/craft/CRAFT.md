# The craft

The designer's playbook for Syndra. Load it before every design, and run the checklist in §12 on every board.

**What this is.** Six research passes (18 Nov 2026) read the people and products whose interaction work is most admired, and the consoles closest to Syndra, and asked of each finding: why does it work on a person, what are the numbers, and does it belong in a makerspace's access console. This file is what was kept. `SOURCES.md` beside it lists every page, and says which were read and which were not.

**How to read an entry.** The principle; **Why** (the mechanism in a person); **Values**; **Seen in**; and **Syndra**: adopt, adapt or avoid, with what the code does today. Syndra's own decisions (`RULES.md`, `FINAL.md`, the copy guide) win wherever they speak. A principle here that would break one is offered to the owner as a question, never built.

**Marks.** UNVERIFIED means no primary page was read for it. Everything else was read on the page named.

---

## 0 · The two aims, and the dial

The owner, 18 Nov 2026: Syndra is not to be "a boring corporate admin tool". It should be "delightful and a pleasure to use", and "inspiring". Delight is a goal of the design, not a garnish on it.

**0.1 Delight has two depths, and the deep one comes first.**
- **Why:** NN/g separates surface delight (an animation, a line of copy, a picture) from deep delight (the thing works, at the right moment, and stays out of the way). Walter's pyramid puts pleasurable at the top of functional, reliable and usable: a higher tier only counts when the lower ones hold. One bad moment outweighs many good ones.
- **Seen in:** https://www.nngroup.com/articles/theory-user-delight/ · https://www.nngroup.com/articles/pillars-user-delight/
- **Syndra: adopt.** Deep delight for an operator is feeling competent and protected; for a member, that the space is theirs and is looking after them. Never polish motion while a row is misaligned.

**0.2 Spend expression in proportion to rarity (the 50th-time test).**
- **Why:** an animation that informs the first time is latency the hundredth. Kowalski: the more often an action is triggered, the less it should animate; Raycast opens with no animation at all. Taylor's Family: frequent actions get subtle details, rare ones get delight. Apple's guidance says the same of frequent interactions. Novelty decays by definition (Comeau).
- **Values:** anything fired dozens of times a day stays under 150ms or does not move (Atlassian). Keyboard-initiated actions do not animate (Kowalski, Freiberg).
- **Seen in:** https://emilkowal.ski/ui/you-dont-need-animations · https://benji.org/family-values · https://rauno.me/craft/interaction-design · https://developer.apple.com/design/human-interface-guidelines/motion · https://atlassian.design/foundations/motion · https://www.joshwcomeau.com/blog/whimsical-animations
- **Syndra: adopt, as the first question of every design.** Give each moment a frequency class before drawing it:

| Class | How often | What it may carry | In Syndra |
|---|---|---|---|
| Constant | many times a minute | colour only, or nothing | hover, row focus, sort, filter, typing, Go to opening |
| Frequent | dozens a day | tint and press; one settle | a dialog, a fold, a status changing, opening a page |
| Occasional | a few a day | lift, bloom, a sentence in the keeper's voice | a result, a refresh you asked for |
| Rare | daily or less | an authored moment | the first arrival of a session, a queue you emptied, a member's first time, a term's first day |

**0.3 The dial: quiet, warm, vivid.**
Every option is drawn at up to three settings, so the owner chooses character with his eyes open.
- **Quiet:** the product as it is, or one honest step from it.
- **Warm:** adds one thing that is true and kind: a sentence, a light, a line that knows.
- **Vivid:** the bold version, drawn at full strength. Allowed only in the Rare class.

**0.4 The guardrails that make delight trustworthy.** These hold at every setting.
1. Never on a consent, a failure, or anything that takes access away. Those get exactness: grave, specific, one next step.
2. It shrinks with repetition: once a session, once a visit, or only when the reader's own act caused it.
3. Reduced motion is a designed form, not a patch (§1.9).
4. No compulsion: no streaks, counts that reward returning, limited-time language or guilt (§10).
5. Every warm sentence is built from the same read as the numbers beside it. Holding is observed; records explain.

---

## 1 · Motion

**1.1 Two modes: productive and expressive.**
- **Why:** it names the decision a designer actually makes: am I helping a task, or marking a moment? Carbon keeps expressive motion for significant moments and uses productive motion wherever people are completing tasks, including rendering data tables.
- **Seen in:** https://carbondesignsystem.com/elements/motion/overview/
- **Syndra: adopt.** Syndra's six movements are its productive set. The expressive set is the Rare class of §0.2, and it is short enough to list on one page.

**1.2 Stay inside the consensus band.**
- **Why:** under about 0.1s an effect binds to its cause; a long flourish on a quick action makes the tool feel slower than it is.
- **Values:** feedback 70 to 150ms; overlays 150 to 250ms; large surfaces 300 to 400ms; nothing in the product above 500ms ("a real drag", NN/g). Carbon: 70, 110, 150, 240, 400, 700. Material 3: 50ms steps from 50 to 600; enter 250 to 400, exit 200 (M3 now maintains springs instead, but the table stands). Kowalski: UI motion under 300ms.
- **Seen in:** https://www.nngroup.com/articles/animation-duration/ · https://carbondesignsystem.com/elements/motion/overview/ · https://m3.material.io/styles/motion/easing-and-duration/tokens-specs · https://emilkowal.ski/ui/great-animations
- **Syndra: adopt; already inside it.** Tint 120, press 180, fold 200, settle 260, arrive 420 (`globals.css:269-280`). Arrive is the one value at the band's edge, and it plays on every page: see §1.3.

**1.3 A place arrives once.**
- **Why:** §0.2. A page opened for the thirtieth time today is not a new place. Text someone may already be reading should not be displaced (NN/g).
- **Values:** first visit to a path in a session: arrive as built (420ms, 8px, 40ms apart, cap 5). A later visit: the header tints (120ms) and the content is there. A visit made by key (Go to, a row's Enter): never arrives.
- **Seen in:** https://www.nngroup.com/articles/animation-usability/ · https://emilkowal.ski/ui/you-dont-need-animations
- **Syndra: adapt; a tightening of S11.** Today a page arrives on every navigation that is not Back (`lib/motion/arrive.tsx:29`). The owner's.

**1.4 Exits are shorter than entrances, and leave by the road they came.**
- **Why:** what leaves needs less attention than what is next. A permanent exit may end at speed; a thing that can come back ends at rest (Material 3). Direction symmetry builds a map of where things live.
- **Values:** exit about half to two thirds of enter.
- **Seen in:** https://m3.material.io/styles/motion/easing-and-duration/applying-easing-and-duration · https://atlassian.design/foundations/motion · https://www.nngroup.com/articles/animation-duration/
- **Syndra: adopt; built** ("closing fades first, then folds, and is quicker"; fold 200 against settle 260).

**1.5 One curve family, front-loaded.**
- **Why:** ease-out shows the response in the first 100ms. The sources differ only on exact curves, which argues for choosing one family and never leaving it.
- **Values:** Carbon productive entrance `(0, 0, 0.38, 0.9)`; Material standard `(0.2, 0, 0, 1)`; Syndra's settle and arrive `(0.16, 1, 0.3, 1)`. Colour and opacity never overshoot (Material's split of spatial from effects).
- **Seen in:** https://emilkowal.ski/ui/good-vs-great-animations · https://m3.material.io/styles/motion/overview/specs
- **Syndra: adopt; built.** Never add a curve to solve one screen.

**1.6 No bounce without momentum.**
- **Why:** bounce reads as physical only when the person put energy in. Apple starts every spring at 100% damping and allows overshoot only for gestures that carry momentum.
- **Seen in:** https://developer.apple.com/videos/play/wwdc2018/803/ · https://motion.dev/docs/react-transitions
- **Syndra: adopt; built** (FINAL §4: never a shake, never a bounce). A phone sheet following the finger is the only spring-like motion.

**1.7 Transitions for state, keyframes for one-shots; transform and opacity only.**
- **Why:** a transition can be retargeted mid-run; a keyframe cannot, so a reversal waits or jumps. Only transform and opacity stay off the main thread, which a console with large tables keeps busy.
- **Seen in:** https://emilkowal.ski/ui/building-a-toast-component · https://web.dev/articles/animations-guide
- **Syndra: adopt.** A row returning mid-fade already reverses (FINAL §4).
- **The one exception: a control's indicator** (`GlideIndicator`, `indicator-glide`). An out-of-flow indicator with no children may transition its own `width` and `height` beside `transform`. A pill scaled whole squashes its round ends, and one drawn in parts to avoid that showed its seams as two lines under a translucent fill (staging, 20 Nov 2026), so it is one element that takes the chosen option's size. Absolutely positioned and childless, it is the only box that re-lays out, never the group. MUI's Tabs indicator does the same (`left` and `width`). Never `left` or `top`: its place is a transform.
  - Seen in: https://github.com/mui/material-ui/blob/master/packages/mui-material/src/Tabs/Tabs.js

**1.8 Never from nothing, and from where it belongs.**
- **Why:** nothing in the world appears from zero size; a popover that grows from its trigger visibly belongs to it.
- **Values:** start at scale 0.93 to 0.98; set `transform-origin` to the trigger.
- **Seen in:** https://emilkowal.ski/ui/7-practical-animation-tips
- **Syndra: adopt; built** (a dialog rises from 98%; a popover rises 4px).

**1.9 Reduced is not none.**
- **Why:** the triggers are distance, mismatched direction and zoom. Opacity, colour and blur are "unlikely to be problematic" (Val Head). Removing every transition also removes useful feedback.
- **Values:** under `reduce`, drop travel and scale; keep colour and opacity at about 100 to 150ms; never set `animation: none` where code waits for `animationend`.
- **Seen in:** https://alistapart.com/article/designing-safer-web-animation-for-motion-sensitivity/ · https://web.dev/articles/prefers-reduced-motion · https://www.smashingmagazine.com/2021/10/respecting-users-motion-preferences/
- **Syndra: adapt.** Today the reduced block collapses every duration to 0.01ms and keeps press and flash's colour (`globals.css:777`). Tint could survive too, so a status still crossfades instead of snapping. The owner's.

**1.10 Motion explains a relationship, or it is noise.**
- **Why:** any movement takes attention before reading does. NN/g reserves it for attention, a change of state and a relationship; motion with no spatial meaning adds delay and nothing else.
- **Seen in:** https://www.nngroup.com/articles/animation-usability/
- **Syndra: adopt; built** ("every row answers: what changed"). The test for any new motion: say what relationship it shows, in one clause.

**1.11 Continuity: the thing you opened is the thing you are in.**
- **Why:** object constancy. Family's rule is that an element which persists should not duplicate or reset; the eye follows one thing through a change and the model survives.
- **Values:** a shared element travels 200 to 250ms; names must be unique; treat it as an enhancement that fails to nothing.
- **Seen in:** https://benji.org/family-values · https://developer.chrome.com/docs/web-platform/view-transitions/same-document · https://developer.mozilla.org/en-US/docs/Web/API/View_Transition_API/Using
- **Syndra: adapt.** The name already arrives with you (`PersonAccess.tsx:122-145`), and D-1 §7a adds the page fade. A name that travels from its row to its title breaks the 10px cap, so it is a vivid option and the owner's.

---

## 2 · Feedback and timing

**2.1 Three limits: 0.1s, 1s, 10s.**
- **Why:** three different states of mind: cause and effect, unbroken thought, held attention.
- **Values:** under 0.1s show only the result; past 1s show that work is happening; past 10s show how far, and offer a way out.
- **Seen in:** https://www.nngroup.com/articles/response-times-3-important-limits/
- **Syndra: adopt.** Past 10s Syndra says nothing new today, except one line on the member's password card after 4s (`MyStorage.tsx:594`).

**2.2 Under a second, show nothing.**
- **Why:** a loader that flashes makes a fast thing feel slow.
- **Values:** nothing under 1s; a looping indicator for 1 to 3s (Primer) or 2 to 10s (NN/g); determinate past 3 to 10s; background the task past 10s. A delay before the loader and a minimum time on screen are common practice, but no primary source gives numbers: UNVERIFIED.
- **Seen in:** https://primer.style/product/ui-patterns/loading/ · https://www.nngroup.com/articles/skeleton-screens/ · https://www.nngroup.com/articles/progress-indicators/
- **Syndra: adopt; built.** The skeleton waits 200ms (`lib/motion/still.ts:27`), and it fades before rows arrive.

**2.3 A skeleton is the real shape.**
- **Why:** it sets the expectation and removes the shift when content lands. A frame-only skeleton says nothing. One announcement for a group, not one per row.
- **Seen in:** https://www.nngroup.com/articles/skeleton-screens/ · https://primer.style/product/ui-patterns/loading/
- **Syndra: adopt.** Check each `RowSkeleton` against its row's height after D-1.

**2.4 Count what is real, or do not count.**
- **Why:** a moving indicator makes a wait bearable, and a false one spends trust. NN/g's example is "Updating address 3 of 50".
- **Seen in:** https://www.nngroup.com/articles/progress-indicators/
- **Syndra: adopt.** A plan result already counts ("Sent 7 of 9"). A check has no steps to count, so it gets a sentence (§2.1), never a bar. No artificial delay, ever.

**2.5 The press is answered on the frame it happens.**
- **Why:** the nearest thing on the web to a haptic; it confirms before the network does.
- **Values:** scale 0.97 on a control; a tint on a dense row, because scaling a row disturbs its neighbours.
- **Seen in:** https://emilkowal.ski/ui/7-practical-animation-tips
- **Syndra: adopt; built** (3% press; rows darken to `--tint-3`).

**2.6 Never optimistic about access.**
- **Why:** optimistic UI removes the wait by claiming success. For access, the claim is the product.
- **Seen in:** https://workos.com/docs/directory-sync/understanding-events (state is reconciled; events are hints)
- **Syndra: avoid for anything that changes access; adopt for neutral acts** (sort, filter, a view choice). Pending is said as it is: "Not sent to Zitadel yet", "Sent, not seen back yet", "In Zitadel" (`lib/standing.ts:34-50`).

**2.7 A tooltip warms up once.**
- **Why:** the first delay filters accidents; after intent is shown, delay is friction.
- **Seen in:** https://emilkowal.ski/ui/7-practical-animation-tips
- **Syndra: adopt** where two terms sit near each other (the WaitsMark column).

**2.8 Position, focus, scroll and pointer survive every change.**
- **Why:** Linear's changelog reads as a list of these fixes: the next checkbox stays under the pointer, a board does not blink, Escape clears and then closes. Quality is felt as nothing jumping.
- **Seen in:** https://linear.app/changelog
- **Syndra: adopt.** Built: Back restores scroll, paging and folds (`lib/place.ts`); a gap waits while the pointer is in the list. Not built: focus returning to the row you opened; a two-stage Escape in search.

---

## 3 · Space and rhythm

**3.1 One scale, used by meaning.**
- **Why:** nearness reads as relation only when distances differ in clear steps.
- **Values:** Carbon: 2, 4, 8, 12, 16, 24, 32, 40, 48, 64. Inside a part 4 to 12; between related things 16 to 24; between sections 40 to 64.
- **Seen in:** https://carbondesignsystem.com/elements/spacing/overview/
- **Syndra: adopt; built by D-1** (row 10, card inset 20, cards 16 apart, regions 40).

**3.2 The gap around a group is larger than the gaps inside it.**
- **Why:** nearness is read before words, and can outweigh colour and shape. It is free.
- **Seen in:** https://www.nngroup.com/articles/gestalt-proximity/ · https://lawsofux.com/law-of-proximity/
- **Syndra: adopt, as a check at 1440 and at 390:** stacking on a phone can part things that sat together.

**3.3 Alignment is the cheapest luxury.**
- **Why:** Linear's designers say their alignment work is felt "after a few minutes" and not seen. A few pixels of misalignment read as noise that nobody can name.
- **Seen in:** https://linear.app/now/how-we-redesigned-the-linear-ui
- **Syndra: adopt; D-1's whole subject** (every action in its seat, one dot indent). It is the one delight with no downside.

**3.4 Space first, then a hairline, then a ground, then a card.**
- **Why:** each step up costs width and flattens the hierarchy. Butterick's table advice: take every border off, then add back only what helps.
- **Seen in:** https://practicaltypography.com/tables.html · https://www.nngroup.com/articles/visual-hierarchy-ux-definition/
- **Syndra: adopt; it is the owner's own rule** (a box means something is wrong). No card inside a card.

**3.5 A sentence has a measure; a page has a width.**
- **Why:** past about 90 characters the eye loses its place on the return.
- **Values:** 45 to 90 characters; line spacing 120 to 145% (Butterick).
- **Seen in:** https://practicaltypography.com/line-length.html · https://practicaltypography.com/line-spacing.html
- **Syndra: adopt; built** (`--measure` 60ch, `--page-max` 1280).

**3.6 Density belongs to the person, and wider windows show more, not bigger.**
- **Why:** density that changes on its own is a surprise reflow. Bloomberg moved to resizable windows so a larger screen shows more rows.
- **Seen in:** https://m3.material.io/foundations/layout/grids-spacing/density · https://www.bloomberg.com/company/stories/how-bloomberg-terminal-ux-designers-conceal-complexity/
- **Syndra: adapt.** D-1 chose one density well over a switch. Keep that; revisit only if operators ask.

---

## 4 · Type and figures

**4.1 Three sizes on any one screen.**
- **Why:** each further level of emphasis dilutes the rest. NN/g: at most three sizes and two large elements.
- **Seen in:** https://www.nngroup.com/articles/visual-hierarchy-ux-definition/
- **Syndra: adopt.** The scale has eight steps (D-1); a screen should show three or four.

**4.2 A productive ladder.**
- **Values:** Carbon productive: 12/16 label, 14/20 body, headings 20/28 and 28/36. Apple's Mac styles: body 13/16, title 22/26, large title 26/32, floor 10 to 11. Material 3: a 1.125 ratio on a 14 base.
- **Seen in:** https://carbondesignsystem.com/elements/typography/type-sets/ · https://developer.apple.com/design/human-interface-guidelines/typography · https://m3.material.io/styles/typography/type-scale-tokens
- **Syndra: adopt; built** (34, 28, 22, 17, 14, 13, 12.5).

**4.3 The display face has one job.**
- **Why:** a large, quiet, well-set line carries more character than any ornament. Linear uses a display cut for headings and the plain face below.
- **Seen in:** https://linear.app/now/how-we-redesigned-the-linear-ui
- **Syndra: adopt; built.** Bricolage is for the greeting, titles, a result's word and a figure. Never inside a table or a form.

**4.4 Every figure is tabular.**
- **Why:** equal-width digits let the eye compare by length down a column, and stop a changing number from shifting its neighbours.
- **Values:** `font-variant-numeric: tabular-nums` on every count, date, time, size and figure; right-align a column of numbers.
- **Seen in:** https://developer.mozilla.org/en-US/docs/Web/CSS/font-variant-numeric (Apple's guidelines do not say this; the earlier attribution was wrong)
- **Syndra: adopt.** Not built as a rule: the class appears in 12 files and is missing on the stat figure (`MetricBlock.tsx:76`). Whether Figtree and Bricolage carry tabular figures is UNVERIFIED.

**4.5 Mono is the voice of an identifier.**
- **Why:** a second face that only ever means "this is a key" is learned once (Geist's label and mono roles).
- **Seen in:** https://vercel.com/geist/typography
- **Syndra: adopt; built** (a role key, a grant id, a claim, a token).

**4.6 Capitals for less than a line, with tracking.**
- **Values:** 5 to 12% letterspacing on capitals (Butterick). Apple opens tracking at small sizes and closes it at large ones.
- **Seen in:** https://practicaltypography.com/summary-of-key-rules.html
- **Syndra: adopt; built** (`type-label`). An eyebrow names a parent thing and nothing else (G6).

---

## 5 · Density

**5.1 A row's height follows its content.**
- **Values:** Carbon: 24, 32, 40, 48, and 64 only for two lines; 16 between columns; the header matches the rows.
- **Seen in:** https://carbondesignsystem.com/components/data-table/style/
- **Syndra: adopt; built** (41 for text, 50 with a button).

**5.2 A table is read two ways.**
- **Why:** along a row from a name to its facts, and down a column to compare. The first column is a readable name; the most important columns come first; one row-tracking aid is enough.
- **Seen in:** https://www.nngroup.com/articles/data-tables/
- **Syndra: adopt; built** (name · description · numbers; hover tint; no zebra).

**5.3 Nothing that someone decides on is cut short.**
- **Syndra's own rule**, and the research agrees: truncate identifiers only, with the whole one tap away.

**5.4 The chrome recedes.**
- **Why:** when the frame is dimmer than the content, the content is the brightest thing on the screen. Linear dimmed its sidebar "a few notches" and softened its dividers.
- **Seen in:** https://linear.app/now/behind-the-latest-design-refresh
- **Syndra: adopt.** Check the rail against the page in both themes whenever a token moves.

---

## 6 · Presentation

**6.1 The gist, then the evidence.**
- **Why:** Terraform ends a plan on one line of counts; Cloudflare's tester says how many people a policy allows and opens to their names. A summary first lets a person stop early.
- **Seen in:** https://developer.hashicorp.com/terraform/cli/commands/plan · https://developers.cloudflare.com/cloudflare-one/policies/access/policy-management/
- **Syndra: adopt; built** (a plan's summary line; Home's headline counts blocks). Wanted: the same first line on a person's page.

**6.2 The reason sits beside the surprise.**
- **Why:** Terraform prints "forces replacement" next to the thing it is about to destroy. The cause of the frightening thing is answered before it is asked.
- **Seen in:** https://developer.hashicorp.com/terraform/cli/commands/plan
- **Syndra: adopt; built** ("Because Trained in Laser Lab always waits for you.", `plan-groups.ts:154`).

**6.3 Why does she have this, as a chain of checks.**
- **Why:** Google's troubleshooter answers per policy layer, flags the one that decides, and shows Unknown where it could not read. An absent row would read as no.
- **Seen in:** https://docs.cloud.google.com/policy-intelligence/docs/troubleshoot-access
- **Syndra: adopt.** The source chip and its popover exist (`AccessSource.tsx:165-190`), and held-twice is said. Missing: which source decides, and the same answer for "why not".

**6.4 A number sits next to its history.**
- **Why:** Tufte: a figure gains meaning from context, and a long history counters recency. Word-sized, no frame, a grey band for normal, the last point tied to the number by colour.
- **Values:** about 5:1 for a month, up to 20:1 for a season; lines at least 1.5px on a screen; one shared scale when rows are compared.
- **Seen in:** https://www.edwardtufte.com/notebook/sparkline-theory-and-practice-edward-tufte/
- **Syndra: adopt; built by A-23** (one band per row, one axis per section).

**6.5 Position before area, area before colour.**
- **Why:** aligned positions are judged most exactly (Cleveland and McGill; read only in summary, and the ranking is common, not settled).
- **Syndra: adopt.** Bars and lines on one baseline for anything compared. A grid of tinted days is for rhythm, never for an exact figure: it always carries a readout.

**6.6 One signal, one meaning.**
- **Why:** a feature that only ever means one thing is found in a glance however many rows there are. Things keeps about 95% of its surface neutral and gives each colour one job (third-party teardown). Bloomberg found thin red and green lines the first thing to fail, and kept amber for what has no meaning.
- **Seen in:** https://www.csc2.ncsu.edu/faculty/healey/PP/ · https://www.bloomberg.com/company/stories/designing-the-terminal-for-color-accessibility/ · https://carbondesignsystem.com/patterns/status-indicator-pattern/
- **Syndra: adopt; built** (violet acts, lime needs nothing, amber waits, red loses). A status is a dot and a word, never colour alone. At most six kinds of status in a view.

**6.7 Every number opens the list behind it.**
- **Why:** a count with no way in is a vanity figure.
- **Syndra: adopt.** Partly built: Trends' Ending soon links; usage figures and "held but not used" do not (`MetricBlock.tsx:70-77`, `TrendsTab.tsx:46`).

---

## 7 · Segmentation and disclosure

**7.1 Two levels, and never a consequence behind one.**
- **Why:** what is on the first level is read as what matters. Three levels lose people. Disclosure hides options; it must not hide risk.
- **Seen in:** https://www.nngroup.com/articles/progressive-disclosure/
- **Syndra: adopt; built** (Basic and Advanced). Never a third level inside Advanced.

**7.2 Headings are the scan targets.**
- **Why:** readers who find good headings read those, then the one section they need ("layer-cake" scanning, the most effective pattern NN/g recorded).
- **Seen in:** https://www.nngroup.com/articles/layer-cake-pattern-scanning/
- **Syndra: adopt; the copy guide's rule** (five words or fewer, naming the thing).

**7.3 An empty space says which kind of empty.**
- **Why:** a blank cannot be told from loading, failure or a filter. Four states: loading, nothing yet, nothing matches, could not read. Tone follows the cause: a first-time blank invites, a finished queue may be glad, a miss is neutral (Atlassian).
- **Seen in:** https://www.nngroup.com/articles/empty-state-interface-design/ · https://carbondesignsystem.com/patterns/empty-states-pattern/ · https://atlassian.design/components/empty-state/usage
- **Syndra: adopt; built** (`EmptyState`, `resolved`; "Nobody matches that." with Clear the filters).

**7.4 Unknown is never zero.**
- **Why:** the worst thing a console can do is look healthy and be wrong. Primer: hide a count that cannot be read; never draw a generic empty state where data should be.
- **Seen in:** https://primer.style/product/ui-patterns/degraded-experiences/
- **Syndra: adopt; built and worded** ("Neither it nor a day not counted yet is a zero.").

**7.5 A message has one place.**
- **Why:** a fixed ladder from passive to blocking gives one decision (how urgent) and one expectation. Status belongs near the thing it describes; an alert is for what is critical (Apple).
- **Seen in:** https://carbondesignsystem.com/patterns/notification-pattern/ · https://developer.apple.com/design/human-interface-guidelines/feedback
- **Syndra: adopt; A-22 is this, and goes further** (nothing is a toast, nothing times out).

---

## 8 · Safety and trust

**8.1 What you reviewed is what runs.**
- **Why:** PlanetScale binds an approval to its content: edit the change and the approval is gone. Terraform separates a speculative plan from a saved one.
- **Seen in:** https://planetscale.com/docs/vitess/schema-changes/deploy-requests
- **Syndra: adopt; built** ("Nothing was applied. This is a fresh preview.", `RehearsalDialog.tsx:412`).

**8.2 Undo before warning.**
- **Why:** a warning seen often is dismissed by reflex, so it causes the regret it was for (Raskin). Undo removes the fear at its root. Apple: do not warn when the loss is the expected result.
- **Seen in:** https://alistapart.com/article/neveruseawarning/ · https://www.nngroup.com/articles/confirmation-dialog/ · https://developer.apple.com/design/human-interface-guidelines/feedback
- **Syndra: adapt.** Nothing has a timed undo. The honest equivalent exists: a change that waits can be cancelled until it is sent. Say so where it is true, and never show an undo that cannot work.

**8.3 A confirmation states the consequence, and names its buttons by outcome.**
- **Why:** "Are you sure?" gives nothing to check. Carbon's sharpest rule: no dialog when the person needs information outside it to decide.
- **Seen in:** https://www.nngroup.com/articles/confirmation-dialog/ · https://carbondesignsystem.com/patterns/dialog-pattern/ · https://shopify.dev/docs/api/app-home/web-components/overlays/modal
- **Syndra: adopt; built** (G20 to G31).

**8.4 Say what stays.**
- **Why:** Vercel's rollback lists what will not change; WorkOS's danger zone says what deletion does not touch. That is the second worry, answered.
- **Seen in:** https://vercel.com/docs/instant-rollback · https://workos.com/docs/admin-portal.md
- **Syndra: adopt where the code can prove it.** A revoke that says what remains must be traced through every surface first.

**8.5 A standing state stays visible.**
- **Why:** a rollback that paused automation and said so only in a toast is forgotten in a week.
- **Syndra: adopt; built** (a hold is a band on the person; nothing is a toast).

**8.6 Dismissal carries its own way back.**
- **Why:** Sentry lets a person silence an issue until it gets worse. Quiet without that is a risk left lying.
- **Seen in:** https://docs.sentry.io/product/issues/states-triage/
- **Syndra: adapt** for Drift, if acknowledging is ever built.

**8.7 A problem comes with its fix.**
- **Why:** 1Password groups problems by cause, names one action for each, and shows only groups that have something in them.
- **Seen in:** https://support.1password.com/watchtower/
- **Syndra: adopt; Home is this** (blocks by cause; an empty block folds away).

**8.8 Change arrives gradually.**
- **Why:** for people in daily, high-stakes work, the cost of change is cognitive. Bloomberg ships a visual change in small steps and never changes look and behaviour at once.
- **Seen in:** https://www.bloomberg.com/company/stories/how-bloomberg-terminal-ux-designers-conceal-complexity/ · https://lawsofux.com/jakobs-law/
- **Syndra: adopt.** One expressive addition per release, judged on staging before the next.

---

## 9 · Delight that respects people

**9.1 A greeting is worth having when it is true.**
- **Why:** recognition is the oldest personal touch, and the gap between good and cringe is whether the line says something real.
- **Seen in:** https://slack.design/articles/thevoiceofthebrand-5principles/
- **Syndra: adopt; built** ("Evening, Kanishk. Three things need you."; "It will keep until morning."). The sentence comes from the same read as the blocks.

**9.2 The voice is constant; the tone moves.**
- **Why:** people read tone before content. Mailchimp keeps one voice and shifts tone with the reader's state; if unsure about humour, keep a straight face.
- **Seen in:** https://styleguide.mailchimp.com/voice-and-tone/
- **Syndra: adapt.** Three tones: warm (arrival, all-clear, a result that went well), plain (everything between), grave (a consent, a refusal, a failure). The keeper never jokes (the copy guide); her warmth is exactness at the right moment.

**9.3 The background can be the data.**
- **Why:** Apple Weather lets you feel the conditions before you read a number: ambient information at no effort (press reports, not an Apple page).
- **Seen in:** https://www.macrumors.com/guide/ios-15-weather-app
- **Syndra: adapt.** Light is Syndra's material: the door is an arch of it, and the themes are a dark room and daylight. A still pool of light whose place says the hour is the Weather idea at a work tool's scale. It never signals alarm and never moves with data.

**9.4 The earned mark.**
- **Why:** a small reward bound to something the person just did closes the loop, and survives repetition when it is short and always the same (Things' checkbox, by a third-party teardown).
- **Values:** under 400ms; the same every time; silent.
- **Seen in:** https://blakecrosley.com/blog/design-study-things (secondary) · https://www.macstories.net/reviews/things-3-beauty-and-delight-in-a-task-manager/
- **Syndra: adopt; built** as the bloom: once, for something that went well, because of you, on screen.

**9.5 Design the ending, and the failure.**
- **Why:** memory keeps the peak and the end, not the average (in Kahneman's experiment, 80% chose the longer trial that ended slightly better). The worst moment is usually the peak, so failure gets the best writing.
- **Seen in:** https://www.nngroup.com/articles/peak-end-rule/ · https://www.nngroup.com/articles/error-message-guidelines/
- **Syndra: adopt.** Built for one act (ActionOutcome). Not yet designed: the end of a whole queue, and the end of a day.

**9.6 A big payoff only for a rare, real event.**
- **Why:** Playdate saves its confetti for a new arrival and builds up to it in proportion. Copy the proportion, not the confetti.
- **Syndra: adapt.** Rare and real here: the first arrival of a session, a queue you emptied, a member's first time with something, a term's first and last day.

**9.7 The first run is the product introducing itself.**
- **Why:** the first minute teaches the interface and sets the tone at once; people start immediately and do not read manuals.
- **Seen in:** https://lawsofux.com/paradox-of-the-active-user/
- **Syndra: adopt.** "Welcome to Syndra." exists for a first visit. A member's first sight of something new is the stronger moment.

**9.8 A mark, not a mascot.**
- **Why:** a small, consistent character gives a product a face at almost no word count. Mailchimp's winks and never speaks. A badly drawn one is worse than none.
- **Seen in:** https://styleguide.mailchimp.com/voice-and-tone/
- **Syndra: adapt.** The mark exists, and its dot already blooms once when you come through the door (`SyndraMark.tsx`). It can say the room's state at rest. It is never sad and never on a consent.

**9.9 Care in the edges.**
- **Why:** care where nobody inspects says the maker cares everywhere: an empty state, a settings page, an email subject, an audit sentence. Saarinen: the spec is the baseline, not the goal.
- **Seen in:** https://www.figma.com/blog/karri-saarinens-10-rules-for-crafting-products-that-stand-out/ · https://linear.app/blog/why-is-quality-so-rare
- **Syndra: adopt.** Every part has its states finished: hover, focus, pressed, disabled with its reason, loading, empty, error, reduced motion, both themes, phone.

**9.10 Data can be beautiful if every mark is true.**
- **Why:** one clear shape per question, direct labels, a caption in plain words.
- **Syndra: adopt.** A-23's band is the model. A chart draws itself in at most once per visit, and never on a refresh.

**9.11 Sound is opt-in or absent.**
- **Why:** sound confirms when eyes are elsewhere, and in a shared room it is the riskiest delight there is.
- **Seen in:** https://notbor.ing/words/the-most-satisfying-checkbox
- **Syndra: avoid by default.** The drift sound is already a choice on the device, in Account.

---

## 10 · Anti-patterns

Each is refused for a reason, so it can be recognised in a new disguise.

| Refused | Why |
|---|---|
| Streaks, badges, points, completion percentages, "come back" nudges | Compulsion. People are enrolled or paid to use Syndra; it has to deserve trust, not win attention (https://www.deceptive.design/) |
| Celebration on an access change, a consent or a failure | Wrong tone for a security event, and it wears out under repetition |
| Confetti, particles, a bounce, a shake | No momentum behind them (§1.6); FINAL §4 forbids them |
| A loop on a healthy state; a moving gradient in the working view | Attention taken and nothing said; battery; vestibular risk. Breathe is the one loop |
| Fake progress, fake delay, a count that ticks for show | Motion that implies work not happening spends trust |
| Stagger past five, or on a table | Forty rows at 40ms is 1.6s of waiting |
| A loader under a second; shimmer everywhere | Reads as flicker |
| Hover that scales or lifts a row | Disturbs its neighbours, and can flicker when it moves from under the pointer |
| A confirm on what can be undone; "Are you sure?" | Habituation: the one that matters stops being read |
| A toast as the only record of anything | Gone before it is read (G26) |
| Unknown shown as zero; an empty state where data should be | Looks healthy and is wrong |
| A list that moves under the pointer on refresh | Mis-clicks; the gap waits instead |
| A card around everything; zebra with rules with hover | Three cues for one job |
| A raw id as a label | The first column is a name |
| Pie or donut for comparison | Angle and area are judged worse than position |
| A third disclosure level; a consequence behind a fold | People get lost; risk must be seen |
| Forced humour; a joke in an error | Insincere in a serious product, and stale by the third reading |
| Scroll-jacking, parallax, large zooms | The motion-sickness triggers |
| Warm copy that the data on the page contradicts | One read, one truth |
| Polish theatre: animating while rows misalign | Surface delight on a weak core |

---

## 11 · How a new idea is judged

1. **Name its frequency class** (§0.2). That sets the ceiling.
2. **Say what relationship or fact it shows**, in one clause. If there is none, it is decoration: allowed only in the Rare class.
3. **Use what exists.** A token, a movement, a part, a sentence already in the copy guide. A new one needs a reason no existing one answers.
4. **Draw its still form** and its phone form beside it.
5. **Trace it through every surface it touches.** Nothing says what does not happen.
6. **Give it a setting on the dial**, and draw the other settings beside it for the owner.

---

## 12 · The checklist, run on every board

**The first glance**
- [ ] Squint: only the title, the primary action and the one thing that is wrong stand out.
- [ ] The first line answers "what is this, and is it working?" for this page, this person, today.
- [ ] Three sizes of type on the screen, four at most.

**The 50th time**
- [ ] Each moving thing has a frequency class, and stays under its ceiling.
- [ ] Nothing a key triggers animates. Nothing fired dozens of times a day takes longer than 150ms.
- [ ] Whatever is expressive here plays once: per session, per visit, or per act of the reader's own.
- [ ] A generic sentence that is the same on every visit has been cut, or replaced by a line that knows.

**Truth**
- [ ] Every count of what someone holds is observed. Records only explain.
- [ ] Unknown is said as unknown, never zero, never empty.
- [ ] Every warm sentence comes from the same read as the numbers beside it.
- [ ] Every outcome ends on a next step, or "Nothing for you to do."
- [ ] Nothing celebrates on a consent, a refusal, a failure or a loss of access.

**Space and type**
- [ ] The gap around each group is larger than the gaps inside it, at 1440 and at 390.
- [ ] Every action is in its seat; every line under a dot starts at the word.
- [ ] Every figure is tabular; a column of numbers is right-aligned.
- [ ] Sentences are within the measure. Nothing decided on is cut short.
- [ ] No box, unless something is wrong.

**Continuity**
- [ ] Nothing jumps under the reader: position, focus, scroll and the pointer's target survive the change.
- [ ] What leaves does so more quickly than it came, and by the same road.
- [ ] A changed number changes only where it changed.

**Both rooms, both hands**
- [ ] Dark and light, where colour carries anything.
- [ ] The still form is drawn. Reduced motion keeps colour and loses travel.
- [ ] The phone: 44px under a thumb, sheets from the edge, the result first after the title.

**Character**
- [ ] One thing on this screen could only be Syndra's: the keeper's sentence, the light, the mark, a figure set well.
- [ ] It would still be welcome on the worst day someone opens this page.
