# A-25 · Syndra, alive

**Asked by the owner, 19 Nov 2026:** "syndra should have a soul, syndra should have character, syndra should feel alive and a joy to interact with."

**Status:** revised 20 Nov 2026, after the owner reviewed the live prototype. §0 records what he decided. Nothing here is built, and nothing was published by me: the lead republishes.

**The prototype:** `design/a25-alive-prototype.html`. One file, 722 lines, no scripts from elsewhere (the two fonts are the only request). It is Syndra's own shell (a rail, a column, a bar, one scroller) at 1440, a phone's bar at 390, both themes, and a Moving / Still switch that shows every still form. Moments 2 and 4 each have their ways behind a switch. The published copy differs from it only in its title.

**Checked, and how.** This round the prototype was run in headless Chrome, at 1440 by 900 and 390 by 844, in both themes, and measured, not looked at. Each light was photographed on and off; the difference is that light alone, read one pixel inside and outside every box that could end it, and under every line of text it falls behind. Every number below marked *measured* comes from that. I still have not seen it with eyes, so strengths are set by number against things Syndra already draws. Mechanically: tags balance, the CSS parses with no warnings (the project's own `lightningcss`), the script passes a syntax check and raises no error when run, every custom property it reads is defined, and there is no sideways scroll at 390.

**Read for this:** RULES, FINAL §4, CRAFT and SOURCES, the copy guide §2, A-18 §4, A-24 §3, §4, §7, §8 and what 7w built from it, the craft research (motion, space, psychology, delight), and the light as it is in code today. Every `file:line` is the worktree of `feat/addon-contract-a24-w4`, now at `3f6afac2` (one commit on from the first draft; the lines cited then were checked again and hold).

---

## 0 · What the owner decided, 20 Nov 2026

| Question (first draft) | His word | Where it stands |
|---|---|---|
| 1 · The character, and the six rules | "rest recommended seem good" | **Accepted.** Rule 1 is tightened by the audit (§2). |
| 2 · The glide, boundless | the same | **Accepted.** The audit found what the real shell needs for it (§3, §7.3). |
| 3 · The member's room | the same | **Accepted.** |
| 4 · Light under your hand | "the light around the pointer is a bit too much. not as elegant and posh." | **Revised:** §4, moment 2. Three ways, one recommended. |
| 5 · The lamp breathes | "rest recommended seem good" | **Accepted:** ten seconds a breath, for operators; a member's lamp stays still until its reading is proven. |
| 6 · The rail | "the menu lighting will possibily cause accessibility issues." | **Revised:** §4, moment 4. The light is cut; today's pill is kept. |
| 7 · The trail's light | "rest recommended seem good" | **Accepted.** |
| 8 · Go to | the same | **Accepted as a trial:** the breath on staging for a week; if it is noticed on the fiftieth open, only the resting light stays. |
| 9 · The ember | the same | **Accepted,** last. |
| 10 · The older lights | the same | **Accepted,** as one small pass after the rest. |
| Every moment | "be careful of the bounding boxes clipping the glow - takes away from the immersion." | **Audited and fixed in the prototype;** the real app's risks are listed with `file:line` in §7. |

## 1 · The character, in one paragraph

Syndra is the keeper of a lit door. The sign-in screen is an arch of violet light, the mark is a lamp, and the two rooms are a dark room and daylight. So her character is not a mascot, a joke or a flourish. It is this: **she carries a lamp, and the light is her tone of voice.**

- When she is **warm** (you arrive, you finish, something is yours from today), the light moves, once.
- When she is **plain** (everything between), the light rests, or answers your hand.
- When she is **grave** (a consent, a refusal, a failure, anything that takes access away), there is no light at all. Exactness is the whole courtesy there.

That is the same three tones CRAFT §9.2 gives her words. Words and light now follow one rule, which is why a handful of small things can read as one character.

## 2 · The grammar of light

Six rules. They are what make a new light Syndra's and not ornament. A light that breaks one is not built.

1. **Light has no edge, and no box gives it one.** Every pool falls to nothing on an eased curve (six stops, not a straight line to a rim), and it is spent, under 6% of its strongest, before it reaches any box that holds it: an overflow, a scroller, a card it sits on, the end of its own layer. Two things may end a light sooner, because both are real. One is a thing in front of it with a ground of its own: a card, the rail, a dialog's panel. The other is the window, and only the hour's light may come in through it. A hairline is not a ground, so the top bar cannot end a light. A mask may give a light a shape (a hairline, the mark's ring); it never gives it an end.
2. **One light moves at a time.** While the door's light crosses, nothing else travels.
3. **Light never holds anything back.** Words are whole from the first frame. Nothing waits for a light, fades in under one, or is staged around one.
4. **Light that answers you is quick; light on its own errand is unhurried.** An answer (your hand, your click) uses tint or settle and has begun within a frame. A light travelling alone (the door, the trail) uses tint's even curve, `--ease-tint`, over a long duration.
5. **Light is violet.** Lime only as a dot's own glow, amber only on the lamp while Zitadel isn't answering. Never red. And it never carries a fact alone: the words beside it say the same thing. (Moment 4's first draft broke this. A light was the only thing marking where you are.)
6. **Every light has a still form,** drawn: under reduced motion it rests where it would have ended. Under forced colours it is gone. It never takes a line of text under 4.5:1 (`--hour-weight`, 1 in the dark room and 0.75 on paper, is the existing lever).

**What changed in rule 1.** The first draft allowed "a card, the rail, the window" to cut any light, and trusted a mask of 96px or more. The audit showed three gaps: a mask is itself a box (moment 2's pool), the window was cutting a small round glow flat (the lamp), and a scroller under a hairline is not a real object at all (moment 0 in the real shell).

## 3 · Moment 0: the glide you chose, with its edges gone

**As built at `3f6afac2`** (`globals.css:1115-1200`, keyframes `376-383`): a pool crosses from `-10cqw` to `62cqw` over `--door-open` (1800ms, `globals.css:2181`) on `--ease-tint`, while the hour's own pool settles in. Once a sign-in, Home only (`Home.tsx:396`, `app/page.tsx:26`). Since the first draft the builder has widened the layer to the column's edges and up to the top of the scroller, and dropped the linear masks (`globals.css:1116-1136`).

**What still cuts it.** The layer lives inside the page, and the page lives inside the one scroller (`#app-scroll`, `AppShell.tsx:102`). The scroller's top edge is the top bar's hairline. So the light reaches that line and stops, at 29% (afternoon) to 81% (morning) of its strength by arithmetic on `globals.css:1137-1174`, **UNVERIFIED on staging**. The bar above it has no ground of its own (`TopBar.tsx:39`, a bottom hairline only), so nothing is in front of the light there: it simply ends at a line. That is a box.

**The change:**

| | Built | Boundless |
|---|---|---|
| The layer | inside the scroller, up to its top | the column's own (`AppShell.tsx:92`), behind the top bar and the scroller |
| The falloff | one straight ramp to `transparent 70%` (`globals.css:1139, 1186`) | six eased stops to nothing (the prototype's `--pool`) |
| The mask | none, since `3f6afac2` | none |
| When the page scrolls | the light leaves with the head | **the light stays: it is the room's, not the page's** (the owner's, §10 Q3) |
| Path, size, timing, curve, strength, once-a-sign-in | as built | as built |

Measured in the prototype, which now has the app's own shell: the light is the same strength each side of the bar's hairline (evening: 38 and 39 of 55 levels in the dark room, 19 and 19 of 28 on paper), so the hairline no longer ends it. Faint ink at each pool's strongest point is 5.30 to 5.77:1 in the dark room and 4.74 to 5.01:1 on paper, so any line that scrolls under the light stays above 4.5:1.

- **Frequency:** Rare, vivid. Once a sign-in (`firstTime`, `openedAlready`).
- **Still form:** the hour's pool, at rest.
- **Cost:** S to M. The utility's inset and gradient, and a seat for the layer in the shell that Home and My access hand their hour to (they stop drawing it themselves).
- **What still ends it, on purpose:** the window and the rail (§7.2, and §10 Q4).

---

## 4 · The moments, ranked by life added per unit of distraction

| # | Moment | Class | The gesture | Cost | Decided |
|---|---|---|---|---|---|
| 1 | The member's room | Rare, then still | The hour's light and the door's glide on My access | S | accepted |
| 2 | Light under your hand | Constant | The card's hairline, lit where it is nearest your hand | M | **revised:** three ways, the edge recommended |
| 3 | The lamp breathes | Ambient | The mark's glow breathes while the reading is live | S | accepted |
| 4 | Your place in the rail | Frequent | Today's pill, kept. No light | S | **revised:** three ways, today's pill recommended |
| 5 | The trail lights | Occasional | A light rests on the step your change has reached | M | accepted |
| 6 | Go to is lit | Constant, by key | One breath of light behind Go to | S | accepted, as a week's trial |
| 7 | The ember | At rest | A clear queue's lime dot holds a low glow | S | accepted, last |

**Build order,** one per release (CRAFT §8.8), each judged on staging before the next, and each measured against its row in §7.3 before it ships:

1. **The boundless glide, with the rail's two repairs.** The repairs (moment 4) are faults in what is live today, so they go first and do not wait for a light.
2. The member's room.
3. Light under your hand: the edge, on Home and People first.
4. The lamp breathes.
5. The trail lights.
6. Go to is lit (one week on staging, then the owner's word).
7. The ember.
8. The older lights (`flash`, the lit card), as one small pass.

The rail's light has left the order. If the owner picks "it glides" in §10 Q2, it takes the fourth place, as an S.

### 1 · The member's room

- **What.** My access gets Home's light: the hour's pool behind "Evening, Cody. Here's what you can use.", and the door's light crossing it once after a sign-in.
- **Why it gives soul.** A-24 §3.3: for most people who ever see Syndra, this page is Syndra. Today it has no light at all: `hour-light` is rendered in one place, `Home.tsx:396`, and a member lands on `MemberAccess` (`app/page.tsx:24-28`) with the door never passed to it. The thing you love is seen only by operators.
- **Motion.** Exactly moment 0: `door-cross` and `door-settle` over `--door-open` (1800ms) on `--ease-tint`, the pool at the hour's own `--hour-reach` times `--hour-weight`. No new token, no new keyframe.
- **Frequency and the 50th time.** Rare. Once a sign-in on this device; every later visit is the still pool. A member who signs in daily sees 1.8s of light a day, and it holds nothing.
- **Still form.** The hour's pool, at rest.
- **Never.** On a failed read of Zitadel (the page then shows its Notice and no rows; a warm light over that would be the wrong tone). Under forced colours.
- **Cost.** S. `MemberAccess.tsx:272-275` hands its hour and the door to the shell's layer, as Home does (§3); `app/page.tsx` passes `door` to both pages. The title's second sentence is `text-faint`, which clears the pool's strongest (measured above).
- **Dial.** Quiet: the still pool only. **Warm, accepted:** pool and glide.

### 2 · Light under your hand (revised 20 Nov)

**The owner:** "the light around the pointer is a bit too much. not as elegant and posh."

**What was wrong, plainly.** The first draft put a 240px pool at 14% on the whole card, a 90px sheen on every button, a press at 24 to 34%, and a 34px glow at 42% outside the focus ring. That is four lights for one hand, strongest where it mattered least. The card's pool also had to be masked 44 to 56px inside its card to hide its own box, and that mask was the box (§7.1).

**What the best do on hover, and what I take from each.**

| Who | What they do | Source | What I take |
|---|---|---|---|
| Emil Kowalski | "A hover effect is nice, but if used multiple times a day, it would likely benefit the most from having no animation at all." | https://emilkowal.ski/ui/you-dont-need-animations (read this round) | Nothing moves and nothing replays. The answer to a hand is a state, the same every time. |
| Material 3 | One flat layer of the content's colour over the whole control: hover 8%, focus 12%, pressed 12%, dragged 16%. | material-web's own token file, `tokens/versions/v0_192/_md-sys-state.scss` in https://github.com/material-components/material-web (read this round; the m3.material.io page would not render) | The common hover is 8%. Mine was 14 to 24. And a press is four points over a hover, not ten. |
| Apple, iPadOS and visionOS | The highlight "materializes a Liquid Glass shape between the pointer and the control it's moving over". On visionOS the system "highlights it, providing visual feedback that helps them confirm the item is one they want." | https://developer.apple.com/design/human-interface-guidelines/pointing-devices and `/eyes` (read this round, from each page's own data file) | The light takes the shape of the control. Its job is to confirm which thing. It is never a blob of its own. |
| Josh Comeau | "I like to make the enter animation quick and snappy, while the exit animation can be a bit more relaxed and lethargic." His example: 125ms in, 450ms out. | https://www.joshwcomeau.com/animation/css-transitions/ (read this round) | In on tint (120ms), out on settle (260ms): tokens Syndra has. |
| Atlassian | A list item's hover in 50ms; interactions 50 to 150ms. | https://atlassian.design/foundations/motion (research 1) | The in is inside the band. |
| NN/g, data tables | One aid for keeping your place across a row: a hover highlight or zebra, not both. | https://www.nngroup.com/articles/data-tables/ (research 2) | The row's tint is already that aid. A pool on top of it was a second cue for one job. |
| Linear | The redesign set out "to reduce visual noise", "limiting how much chrome (blue in our case)" went into its colours. | https://linear.app/now/how-we-redesigned-the-linear-ui (read this round) | Colour is spent in lines and words, not in fields. |
| Microsoft, Fluent "Reveal" | A light that followed the pointer across borders and fills, shipped in 2017, later dropped from the default control styles. | **UNVERIFIED:** its documentation page is gone (404 this round); from memory only | A caution, not a proof. The largest trial of light under a pointer was withdrawn. What I keep is its quietest part, the border, on one card at a time. |
| Research 6, §5.4 | "one glow used only on the focus ring and the primary action" | the researcher's own generalisation, marked UNVERIFIED there | No glow anywhere else. I go one step further: none outside the ring either. |

**The principle.** Posh is found, not shown: you notice it the third time, never the first. The light takes the shape of the thing your hand is on, and nothing glows around the pointer.

**Three ways,** in the prototype behind a switch (`html[data-hand]`), at real scale:

| | **A · The edge** (recommended) | B · The row | C · Nothing follows |
|---|---|---|---|
| Where it lives | The card's own hairline, one pixel on its inner edge, lit where it is nearest your hand. An outline button's and a field's hairline take the colour whole. | The hovered row's own tint: violet where your hand is, fading along the row. Buttons as in A. | Nowhere. As built. |
| Strength | The accent at 50%, on one pixel. *Measured* against the card: 1.7:1 beside your hand and 1.3:1 from the middle of a row in the dark room; 1.8:1 and 1.4:1 on paper. Syndra's own strong hairline (an outline button's border) is 1.5:1 and 1.4:1. | The accent at 7% (times `--hour-weight`): half the first draft, under Material's 8%. | none |
| Radius, falloff | 320px along the edge, the six eased stops | 300px along the row, the same stops | none |
| The press | As built: the row darkens one step (`hover-tint`, `globals.css:2035-2050`) and a button presses (`press-scale`, `globals.css:1036`). The light does not answer a press. | 11% on that frame: Material's four points. | as built |
| Focus | The ring as built, at once (`globals.css:813-815`). The focused control's hairline is lit, because hover and focus are one state; a focused row lights the card's hairline beside it. **No glow outside the ring.** | The ring. The row's tint, its centre at the row's start. No glow. | The ring. |
| Light behind words | **None.** Text contrast is exactly as built. | Yes. *Measured* at its strongest, pressed: muted ink 6.6:1 and faint ink 4.85:1 in the dark room; 7.2:1 and 4.95:1 on paper. | none |
| Can a box cut it | No. The light is the edge, inside the card's own overflow. | Its top and bottom are the row's, as the tint's are today. | n/a |
| The 50th time | A hairline you notice the third time. | A second cue on a row that already tints. | Nothing to tire of. |
| What it costs | In the middle of a long table no edge is near, so nothing shows. It lives at the sides, where the names and the actions are. | Light behind words again. | Nothing of the lamp in the thing you do most. |

**Recommended: A, the edge.** It is the only one of the three that brings the lamp to the most frequent thing you do and puts nothing behind a word. If it does not survive a day on staging, the honest fallback is C, not B.

- **Frequency.** Constant, which allows colour only. Nothing in the page moves.
- **Still form.** The same. Opacity and colour are what reduced motion keeps (A-24 Q15). Gone under forced colours.
- **Never** (kept, and longer by two). On `danger` and `dangerConfirm` buttons, inside a consent's body, on a Notice, on a stale card or NotOpenYet, on a table's column headings, and **on the rail's rows**. The accent button is left exactly as built (`hover:brightness-110`, `Button.tsx:65`): it is already the brightest thing on the page. And not on touch: a finger has no hover to answer.
- **Cut from the first draft.** The pool on the card, the sheen on buttons, the brighter press, the glow outside the ring, the light that followed a finger.
- **Cost.** M. One utility (a pseudo-element masked to a one-pixel ring) worn by `Card`, one listener writing two custom properties once a frame (the prototype's is 12 lines), and one class on Button's `outline` (`Button.tsx:66`).
- **For code.** `Card` is `overflow-hidden` (`Card.tsx:31`), which clips at the padding edge. A rim drawn over the border (`inset: -1px`) would be cut away whole, so it is drawn at `inset: 0`, on the inner edge, as the prototype does with the real card's overflow in place. Frame cost of repainting a card-sized masked gradient on People at 200 rows: **UNVERIFIED**.

### 3 · The lamp breathes

- **What.** While Syndra's reading of the room is live, the mark's glow breathes: out and back once every ten seconds. Amber and quicker while Zitadel isn't answering (built). Still when there is no reading.
- **Why it gives soul.** It is the most literal answer to "alive", on every page, and it costs no attention. A lamp that is kept breathes; one that is not is a logo. It also keeps the rule honest: breathe means "still happening", and what is still happening is the watch.
- **Motion.** A new glow behind the mark: a pool of `--mark-dot` at 36%, **0.85 mark-widths out** (the first draft said 1.3), eased to nothing. Opacity 0.28 to 1 and scale 0.76 to 1, over `calc(var(--door-breath) / 2)` (5s) on `--ease-breathe`, alternating: one whole breath per `--door-breath` (10s, `globals.css:2177`). The dot, the ring and the arc never move. Waiting: the same glow in `--warn` on `--duration-breathe` (2400ms), beside the built amber dot (`globals.css:1878`).
- **Why 0.85.** *Measured* at 1.3: the rail's top cut the glow at 16% of its strength and its side at 7%, in both themes: a round glow with a flat top, in the one corner that is always on screen. On a phone's bar the window's side cut it at 9%. At 0.85 it is spent at every edge, at 1440 and at 390.
- **Frequency and the 50th time.** Ambient. In the corner of the eye a 22px mark changing over ten seconds is under notice; looked at, it is plainly alive. A hidden tab does not run it.
- **Still form.** The glow at rest (opacity 0.72). No breath.
- **Never.** Lime, dim or "sad": the rail's reading cannot prove those (7w-3 review, state (6)). Never the only place a fact is said: the Notice still says Zitadel isn't answering.
- **Cost.** S for an operator: `SyndraMark` gains `room="watch"` (`SyndraMark.tsx:31-70`) when the rail holds a real reading (`Sidebar.tsx:36`), one pseudo-element, one keyframe. A member's lamp stays still until its reading is proven (accepted).
- **The rule it changes.** FINAL §4: "Breathe … the only loop in the product." This is a second loop, on a healthy state. The owner accepted it at ten seconds a breath.

### 4 · Your place in the rail (revised 20 Nov; it was "The light goes with you")

**The owner:** "the menu lighting will possibily cause accessibility issues." He is right, and rule 5 already said so: light never carries a fact alone. The first draft made a soft pool the thing that says where you are.

**The criteria, and where each stands.** *Measured* on the prototype's copy of the rail, which uses the real tokens; the real rail is `Sidebar.tsx:150-172` and the tab bar `TouchNav.tsx:158-176`.

| Criterion | It asks | Today's pill | A pool of light as the place (first draft) |
|---|---|---|---|
| 1.4.1 Use of colour (A) | "Color is not used as the only visual means of conveying information" | Passes. Weight (600 against 400), the fill's shape and the dot say it too. | Weaker: the fill's shape is gone. |
| 1.4.11 Non-text contrast (AA) | What shows a state, "such as whether a component is selected", is 3:1 against what is beside it | The dot is 3.41:1 in the dark room and 3.87:1 on paper (`bg-accent`, `Sidebar.tsx:170`): a pass, close to the floor. The fill alone is 1.22:1 and 1.17:1 against the rail, so it never says it alone. | A gradient has no edge to measure and is strongest near 1.2:1. It cannot pass. |
| 1.4.3 Contrast (AA) | 4.5:1 for text | The word on the pill is 4.92:1 and 5.75:1. Other rows are 8.69:1 and 8.43:1. | About 4.8:1 and 5.7:1 by arithmetic, UNVERIFIED. |
| 2.3.3 Animation from interactions (AAA); reduced motion | "Motion animation triggered by interaction can be disabled" | Nothing moves. | It travelled for 420ms; lit at once under reduced motion. |
| Forced colours | The state survives the system's colours | **A gap today.** A background colour is forced to the system's (its alpha is kept, so a 15% fill is still invisible), the dot's with it, and every link takes the system's link colour. Only the weight is left. Reasoned from CSS forced colours and **UNVERIFIED** on the real rail; `globals.css` has no forced-colours rule for it (its four belong to the lights: `1178, 1197, 1234, 1335`). | Gone, like every light. Only the weight is left. |
| Low vision, and people who cannot see a soft gradient | Something crisp | A bold violet word, and a dot at 3:1 or better. The soft fill is not relied on. | The pool is exactly the thing that cannot be seen. |

**Linear's lesson.** In its refresh Linear made the sidebar "a few notches dimmer" so that the main content area takes precedence (https://linear.app/now/behind-the-latest-design-refresh, read this round; CRAFT §5.4). A pool at 20% would have made Syndra's rail the most coloured field on the screen, and one that moves on every page change. The rail already has its one light, the lamp at its top.

**Three ways,** in the prototype behind a switch (`html[data-rail]`):

| | **A · Today's pill** (recommended) | B · It glides | C · With a light |
|---|---|---|---|
| What | The filled pill, the bold violet word and the dot, there at once, as it is today. No light, and nothing travels. | The same pill, carried to where you go by `GlideIndicator`, the part behind Segmented, Tabs and the section band (`lib/motion/glide-indicator.tsx`; `Select.tsx:116,158`, `Tabs.tsx:59`, `SectionNav.tsx:187`). Its selector already matches `aria-current="page"` (line 19). Transform only, on settle (260ms), a jump under reduced motion (`globals.css:1055-1064`). | B, and a pool at 8% resting under the pill, spent inside the rail. |
| Accessibility | As the table, with the two repairs below. | The same. The pill is the crisp thing in every frame. | The same. The word stays 4.90:1 and 5.75:1 over the pool (*measured*). |
| The 50th time | Nothing to notice. | CRAFT §12: "Nothing fired dozens of times a day takes longer than 150ms." A page change is that, and this is 260ms of chrome moving while the page itself changes. | The same, and a light that says nothing the pill has not. |
| What it costs | The rail stops being one of the seven. | Moving chrome, in the part of the screen meant to recede; folding groups and nested rows to measure. S to M. | Two marks for one fact, and a brighter rail. |

**Recommended: A. And honestly: keeping today's filled pill, with no light, is the best answer.** I was wrong to replace it. B is the only other one I would defend: it keeps "it goes with you" with a part Syndra already has, and it is crisp. C is drawn so the most a light may do here can be seen. I would not build it.

**Two repairs, whichever is chosen.** They are faults in what is live, not character, so they ship first (§4, build order).

1. **Forced colours.** The current place takes the system's own selected colours (`forced-color-adjust: none`, `Highlight` and `HighlightText`), in the rail (`Sidebar.tsx:162`) and the tab bar (`TouchNav.tsx:162, 583`). In the prototype under emulated forced colours the pill's computed background became the system's Highlight and its word HighlightText (*measured*). The prototype also draws that form in the system's own colours, for people who will never meet it.
2. **The rail's dot takes the tab bar's token.** `bg-accent` (`Sidebar.tsx:170`) becomes `bg-accent-text`, which `TouchNav.tsx:167` already uses. *Measured:* 3.41:1 becomes 4.92:1 in the dark room and 3.87:1 becomes 5.75:1 on paper, and the two navs stop disagreeing. It wants a line in the contrast test.

- **Frequency.** Frequent. A: nothing moves. B: one settle, which is the class's ceiling.
- **Still form.** There at once.
- **Never.** Light as the place. A hover light on the rail's rows (hover stays `--hover`).
- **A note, not a change.** Syndra's rail ground is lighter than its canvas in both themes (`globals.css:604-607, 687-690`). Cards are lighter still, so content wins. Worth a look when a token next moves.

### 5 · The trail lights

- **What.** Under a result, the change's trail has a soft light resting on the step the change has reached: the first open one. When Syndra reads the next step as true, the light moves down to it, and the dot fills 40ms behind. When the last step is read back, the light goes out. The change is home.
- **Why it gives soul.** It is the keeper walking your change to the door with the lamp, and it is information: where it is now, and that she is still looking. The trail already breathes truth ("each one settles only on what was observed", `ChangeTrail.tsx:40-41`); the light makes the waiting felt, not just listed.
- **Motion.** A pool 280 by 96px, the accent at 26% (times `--hour-weight`), inside the outcome's own block and spent within 20px of the trail's left. Travel: `translate` to the next step over `--duration-flash` (900ms) on `--ease-tint`. The dot's fill follows by `--light-lead` (40ms), the built rule. While a step is open the pool breathes (`--duration-breathe`, the licensed loop, and exactly its meaning). Leaving: opacity to 0 over `--duration-flash` on `--ease-flash`. Light is not page content, so it may travel the 36px between steps; nothing with ink in it moves. *Measured:* spent at every edge of its block; faint ink at its strongest 4.97:1 in the dark room, 4.94:1 on paper.
- **Frequency and the 50th time.** Occasional, and only by your own act. When Zitadel answers at once (the common case) the light is seen for under a second and goes: it shrinks itself.
- **Still form.** The light on the open step, no travel and no breath; gone when the change is home.
- **Never.** On a refusal or a failure: those have no trail (`changeTrailFor`, `ChangeTrail.tsx:34-35`). **On a removal's trail** (`RemovalDialog.tsx:446`, act `remove`): it takes access away, and grave is no light (§1). This was found by tracing where the trail is drawn; the first draft missed it. Never ahead of a read (CRAFT §2.6). And when the trail stops looking (`TRAIL_WATCH_MS`, six minutes, `ChangeTrail.tsx:19`) the light goes out with it.
- **Cost.** M. One element in `CheckTrail` (`CheckTrail.tsx:29-46`), placed at the first step with `ring`; shown only by `ChangeTrail` for a give (`GrantDirectAccess.tsx:257`), never by the claims trails (`DebugMode.tsx:361`, `ClaimPreview.tsx:157`), which are checks and not journeys.
- **Dial.** Quiet: as built (the dots fill on tint). **Warm, accepted:** the light.

### 6 · Go to is lit

- **What.** Go to still appears at once, whole, caret ready (A-24 §8, built: `GoTo.tsx:52`, `still`). Behind it, one breath of light comes up and eases to rest, as if she had lifted the lamp to look.
- **Why it gives soul.** Go to is the moment you call her. Today it is the one overlay with no character at all, by design. This gives it a presence that costs no time.
- **Motion.** One pool behind the panel, centred on the field, the accent at 46% times `--hour-weight` over the scrim. **Its size is the window's:** as wide as the window up to 1180px, and no taller than twice its centre's height, so it is spent at the window's top and sides (*measured*, at 1440 and 390). Opacity only: 0 to 1 in `--duration-tint` (120ms), then down to 0.6 over `--duration-flash` (900ms), on `--ease-tint`. The panel, the caret and the typing wait for neither. It leaves with the panel, at once.
- **Frequency and the 50th time.** Constant, and opened by a key, which is where the playbook is strictest. The up is inside the 150ms ceiling. The 900ms easing-down carries no information: it is pure character, and it is the part to cut if it tires. Its fallback is drawn (the Still switch): the light at rest from the first frame.
- **Still form.** The light at rest (0.6), no breath.
- **Never.** On any other dialog. A consent gets a plain scrim.
- **Cost.** S. `Modal.tsx:371`: the scrim gains one pseudo-element when `still`; one keyframe.
- **The rule it touches.** CRAFT §12: "Nothing a key triggers animates." The panel does not. The light does, in opacity only. **Accepted as a trial:** a week on staging; if it is noticed on the fiftieth open, only the resting light stays.

### 7 · The ember

- **What.** The lime dot of a clear queue holds a low, edgeless glow. When your own act cleared the queue, the glow opens once from the dot as the state lifts in.
- **Why it gives soul.** An all-clear is the room at rest, and a room at rest still has a pilot light. It also redraws the one built light that has an edge: the bloom is a flat disc (`globals.css:449-458`).
- **Motion.** A pool of `--healthy` at 44%, 17px out from a 6px dot, at opacity 0.8, still. Earned: scale 0.2 to 1 with opacity 1 to 0.8 over `--duration-flash` (900ms) on `--ease-flash`, beside the built `lift-in` (6px, 260ms). No overshoot. *Measured:* spent at every edge of its card.
- **Frequency and the 50th time.** At rest it is still. The opening is Rare and earned: once, by your own hand, never on arrival or refresh (the built rule for bloom).
- **Still form.** The ember at rest.
- **Never.** Behind words: it is the dot's own light. Not on a search that missed or a read that failed (neither is `resolved`).
- **Cost.** S. `EmptyState`'s `resolved` dot (`components/states/index.tsx:133-139`).
- **Dial.** Quiet: as built. **Warm, accepted, last:** the ember.

---

## 5 · The voice

Syndra speaks as the keeper who knows everyone (the copy guide §2, A-18, accepted 5 Nov). A-25 adds no new register. It adds one sentence to the definition:

> **She is warm the way a good lab technician is warm: she knows your name, the hour, and exactly what you hold. She is never jokey, never excited, never sorry. Her warmth is saying the true thing at the right moment and then stopping; her light does the same.**

Three tones, one voice. Five lines in the guide's style. They illustrate tone only: every fact in a shipped line comes from a read, and each is traced through its surfaces before it is written (nothing says what doesn't happen).

| Tone | Where | Words | Light |
|---|---|---|---|
| Warm | Home, when your own act cleared the last block | "That was the last one. Nothing needs you right now." | the ember opens |
| Warm | A role nobody holds yet | "Nobody holds Operator in 3D printing yet. Give it to the first person →" | the card's edge, near your hand |
| Plain | A change Syndra is sending again | "Sent to Zitadel at 14:31. Syndra looks again at 14:36, the second of five tries. Nothing for you to do." | the trail's light, waiting |
| Grave | The before-line of a revoke | "Bianca Belair loses Operator in 3D printing when you confirm. This finishes now." | none |
| Grave | A revoke that did not go | "Not revoked. Zitadel isn't answering, so Bianca Belair still holds Operator in 3D printing. Try again once it answers." | none |

Already in the product and already right: "Evening, Kanishk. Three things need you."; "It will keep until morning."; "Everyone inside is on the list, and everyone on the list is inside."

## 6 · Left out, so nobody proposes them again

- **A pool of light on a card, under the pointer.** Drawn, reviewed, and too much (the owner, 20 Nov).
- **A glow outside a focus ring.** The ring is the crisp thing. A halo beside it was the loudest light on the page, and `Card` cuts it.
- **Light as the mark of where you are.** A fact carried by a gradient cannot be seen by everyone (moment 4).
- **A light on every page's title as it arrives.** It would spend the door's gesture thirty times a day and make the rare one ordinary.
- **A lamp that turns lime when all is well, or dims at night.** The rail's reading cannot prove either (7w-3). The mark says only what it can prove.
- **Light on a success outcome** beyond the trail. The lift and the bloom are built and enough.
- **Shimmer, a moving gradient, a glow that follows the scroll.** CRAFT §10.
- **A farewell at sign-out.** The door is its own screen and has its own light.
- **Later, for consistency, not a moment (accepted, Q10):** `flash` is a rectangle of light (`globals.css:478`) and `lit-card`'s pool is cut by its card's top (`globals.css:1207-1224`). Both take the edgeless falloff in one small pass once the rest are judged.

## 7 · No box cuts a light: the audit

**The owner:** "be careful of the bounding boxes clipping the glow - takes away from the immersion."

**The test.** A light is cut when it is there one pixel inside an edge and gone one pixel outside it, at 6% of its own strongest or more. A light that passes behind a thing with its own ground is not cut; it is behind something. The measuring script is kept outside the repo, at `~/.claude/jobs/146ae399/tmp/a25-rev/measure.mjs`.

### 7.1 What was cutting light in the prototype, and what was done

| Moment | The box | Found | Done |
|---|---|---|---|
| 2 | The pool's layer was its card, and a mask 44 to 56px inside the card hid that | By construction, not measured (it is replaced): the mask was the box. On a four-row card the pool could only ever be a band. | The pool is gone. The edge's light is the hairline; the row's light is the row. |
| 2 | The glow outside the focus ring, 34px at 42% | Not cut in the prototype, whose card had no overflow. The real card has (`Card.tsx:31`), 20px from a row's button. | Cut. The prototype's card now carries the real card's overflow. |
| 3 | The rail's top and side | *Measured* at 1.3 mark-widths: 16% of its strength at the top, 7% at the side, both themes. On a phone's bar, 9% at the window's side. | 0.85 mark-widths. *Measured:* spent at every edge. |
| 3 | The large lamp, against the column's edge | About 5%, by arithmetic | Given room. *Measured:* spent. |
| 4 | The rail piece and the tab bar | *Measured* on the pool before it was resized: 13% at the rail piece's top, 20% at the tab bar's side, and light outside both | Only C has a light now, sized to its own bar. *Measured:* spent. |
| 6 | The window's top, and at 390 its sides | About 20% at the top and 30% at a phone's sides, by arithmetic | The pool is sized by the window. *Measured:* spent, 1440 and 390. |
| 0 | The prototype scrolled the window; the app never does | Its light passed behind the bar only because the whole page scrolled. In the app the scroller begins under the bar (§3). | The prototype has the app's shell now, and the light is the column's. *Measured:* the same strength each side of the bar's hairline. |
| 5 | The outcome's block | *Measured:* spent at every edge | Moved 16px right, so it is spent inside a dialog's 24px margin too. |
| 7 | The card | *Measured:* spent at every edge | Nothing. |
| 1 | The phone's frame | The pool meets it at its strongest: the frame is that drawing's window. | Kept. |

### 7.2 What still ends a light, on purpose

Only the hour's light, and only at two real things. *Measured,* as a share of each pool's strongest:

| | The window's top | The window's right | The rail |
|---|---|---|---|
| Morning | 90 to 92% | none | 90% |
| Afternoon | 100% | 10 to 13% | 13% |
| Evening | 96 to 100% | 61 to 62% | none |
| Quiet hours | 4% | none | 82 to 84% |
| The door, as it sets out | 42 to 44% | none | 100% |

That is the picture as approved: the first draft's §3 said "above and at the sides only the window and the rail cut it", and each hour's light is placed to come from outside the room (low from the left, overhead, from the right). The rail has a ground of its own, so the light is behind it. It is the owner's to say whether this is one of the cuts he saw (§10 Q4).

### 7.3 Where the real app can cut each light

Worktree `feat/addon-contract-a24-w4` at `3f6afac2`. Nothing in this table was measured in the app: it is read from the code, and each row is **UNVERIFIED** until its change is measured on staging.

| Moment | The box | `file:line` | The risk | The guard |
|---|---|---|---|---|
| 0, 1 | The one scroller | `AppShell.tsx:102` (`#app-scroll`, `overflow-y-auto`) | `hour-light` is drawn inside it (`Home.tsx:396`). The scroller's top is the bar's hairline, and the bar has no ground (`TopBar.tsx:39`). As built (`globals.css:1124, 1135`) the light stops at that line at 29 to 81% of its strength. | The layer leaves the scroller: a child of the column (`AppShell.tsx:92`), behind `TopBar` and `#app-scroll`. Home and My access hand their hour up; they stop drawing it. |
| 0, 1 | The scrollbar's seat | `AppShell.tsx:102` (`[scrollbar-gutter:stable]`) | Where scrollbars are not overlaid, the gutter lies outside the scroller's padding box. A light inside the scroller stops a scrollbar's width short of the window, under the evening pool at about 60%. | The same move. The column has no gutter. |
| 0 | The falloff | `globals.css:1139, 1186` (`transparent 70%`) | A straight ramp has an end the eye reads as a rim. Not a box, but the same loss. | The six eased stops (Q2, accepted). |
| 0, 1 | The window | `AppShell.tsx:68` (`#app-root`, `overflow-clip`) | Meant (§7.2). | None. §10 Q4. |
| 1 | My access's head | `MemberAccess.tsx:273` | The same scroller. | As moment 0. |
| 2 | The card | `Card.tsx:31` (`card overflow-hidden`) | Clips at the padding edge: a rim drawn over the border is cut away whole, and any glow outside a focus ring is cut 20px from a row's button. | The rim at `inset: 0`. No glow outside a ring. |
| 3 | The rail | `Sidebar.tsx:69` (`overflow-y-auto`, `pt-5`, `px-3`); the mark at `:71`, 22px (`SyndraMark.tsx:32`) | The glow's centre is about 31px from the rail's top. At 1.3 widths that edge cuts it. | 0.85 widths: 30px from its centre. |
| 3 | The bar's mark on a phone | `TopBar.tsx:43` (16px, inside `px-4`) | Its centre is 24px from the window's side. | 0.85 widths: 22px. |
| 4 | The rail, the tab bar | `Sidebar.tsx:69`, `TouchNav.tsx:93` | Only if C is chosen: a pool wider than the rail is cut by its scroller, and one taller than the tab bar spills onto the page. | Sized to the row and the bar's own padding. A and B have no light. |
| 5 | A dialog's panel | `Modal.tsx:392` (`overflow-y-auto`); the trail is in it at `GrantDirectAccess.tsx:257` (`mx-6`) | The trail's only homes are dialogs. A pool reaching more than 24px left of the trail, or below the last step into the footer, is cut by the panel's scroller. | Spent within 20px of the trail's left, and inside the trail's own height. The footer is not checked. |
| 5 | A removal's trail | `RemovalDialog.tsx:446` | Not a clip, a trace: this trail belongs to taking access away. | No light when the act is `remove`. |
| 5 | The trail itself | `CheckTrail.tsx:31` | None: it has no overflow. | None. |
| 6 | Go to's scrim | `Modal.tsx:371` (`fixed inset-0`) | The scrim is the window. A pool taller than twice its centre's height is cut at the window's top. | Sized by the window. |
| 7 | The card | `Card.tsx:31`; the dot at `states/index.tsx:139`, inside `px-6` (`:133`) | The ember reaches 20px from the dot's centre, which is about 27px inside the card. Clear by 7px. | Keep the ember at 20px or under. |
| Later | The lit card | `globals.css:1207-1224` | Its pool is centred on the card's top edge: cut at full strength by its own card, by design. | Q10's pass. |

### 7.4 How the build guards it

A jsdom test cannot see a clipped light. Each light's change measures itself on staging the way this audit did (the light on, the light off, the difference one pixel inside and outside each box in its row of §7.3) and records the numbers in its PR. A light that is cut does not ship.

---

## 8 · The checklist (CRAFT §12), run on the revised prototype

- **The 50th time.** Each moment has a class and sits under its ceiling. Moment 2 is now colour on one pixel. Moment 4 moves nothing. Two still sit at their ceiling and are flagged: the lamp's loop (accepted) and Go to's breath (on trial).
- **Truth.** The trail's light moves only on a read, and goes out with the watch. The lamp breathes only on a real reading. No warm line is written that a read does not back. A light never carries a fact alone.
- **Never on grave.** No light on a consent, a danger button, a refusal, a failure, a Notice or a removal's trail.
- **Space and type.** Three sizes on the page (34, 22, 14) and the fine 12.5. Names A to Z. Figures tabular. No box except the outcome's own block and the cards.
- **Both rooms, both hands.** Dark and light by the tokens; the Still switch draws every still form; the forced-colours form of the rail is drawn; the keyboard gets the same light as the pointer (a focused row lights the hairline beside it).
- **No box cuts a light.** *Measured:* at 1440 and 390, in both themes, every light is spent at every box, except the hour's at the window and the rail (§7.2).
- **Contrast.** *Measured:* no line of text under a light is below 4.5:1. The lowest is faint ink under the morning pool on paper, 4.74:1.
- **Character.** Each screen has one thing only Syndra would have: the lamp, and its light.
- **Not checked.** Anything by eye. Frame cost. The real app (§7.3). Real forced colours on Windows.

## 9 · For code to verify (not the owner's)

1. Every row of §7.3, measured on staging.
2. A seat in the shell for the room's light, behind `TopBar` and `#app-scroll`, that Home and My access hand their hour and their door to.
3. The edge's frame cost on People at 200 rows.
4. Forced colours on the real rail and tab bar, on Windows: that only the weight is left today, and that the repair holds.
5. The contrast test gains: the rail's dot on the pill, the word on the pill, and faint ink at each pool's strongest for a line that scrolls under the room's light.
6. Whether a member's shell holds a read that can honestly drive the lamp.
7. The rail's polling interval, so "live" has a number (`Sidebar.tsx:140`: "The rail is polled").

---

## 10 · The owner's questions, this round

1. **Light under your hand.** The edge, the row, or nothing follows. In all three the pool on the card, the sheen, the brighter press and the glow outside the focus ring are cut. **Recommended: the edge.** If a day on staging says no, nothing follows.
2. **Your place in the rail.** Today's pill with no light; the same pill gliding; or the glide with a faint light. The forced-colours form and the stronger dot come with any of them. **Recommended: today's pill, no light. It is the best one.**
3. **The room's light stays while the page scrolls.** To pass behind the top bar, as you accepted, the light has to belong to the column and not to the page, so it no longer leaves with the greeting when Home is scrolled. The other choice is a light that leaves with the page and ends at the bar's hairline. **Recommended: it stays.**
4. **The hour's light still comes in through the window, and from behind the rail.** At its strongest at the window's top in the afternoon and evening, and at 82 to 90% beside the rail in the morning and the quiet hours (§7.2). That is the picture you approved. If it is one of the cuts you saw, the pools move wholly inside the page, which is a different picture: a pool that floats, not a light that comes in. **Recommended: keep it.**
