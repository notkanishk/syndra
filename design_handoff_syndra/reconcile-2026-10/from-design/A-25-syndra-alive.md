# A-25 · Syndra, alive

**Asked by the owner, 19 Nov 2026:** "syndra should have a soul, syndra should have character, syndra should feel alive and a joy to interact with."

**Status:** revised twice on 20 Nov 2026, each time after the owner reviewed the live prototype. §0 records what he decided. Nothing was published by me: the lead republishes. Moment 0 has since been built in the worktree from this answer (§3); nothing else here is built.

**The prototype:** `design/a25-alive-prototype.html`. One file, 726 lines, no scripts from elsewhere (the two fonts are the only request). It is Syndra's own shell (a rail, a column, a bar, one scroller) at 1440, a phone's bar at 390, both themes, and a Moving / Still switch that shows every still form. The ways a button may answer your hand are behind a switch in moment 2. The published copy differs from it only in its title.

**Checked, and how.** The prototype was run in headless Chrome, at 1440 by 900 and 390 by 844, in both themes, and measured, not looked at. Each light was photographed on and off; the difference is that light alone, read one pixel inside and outside every box that could end it, and under every word it could fall behind. Every number below marked *measured* comes from that. I still have not seen it with eyes, so strengths are set by number against things Syndra already draws. Mechanically: tags balance, the CSS parses with no warnings (the project's own `lightningcss`), the script passes a syntax check and raises no error when run, every custom property it reads is defined, and there is no sideways scroll at 390.

**Read for this:** RULES, FINAL §4, CRAFT and SOURCES, the copy guide §2, A-18 §4, A-24 §3, §4, §7, §8 and what 7w built from it, the craft research (motion, space, psychology, delight), and the light as it is in code today. Every `file:line` is the worktree of `feat/addon-contract-a24-w4` at `7c487295`, read again this round: it has moved twice since the first draft, and every line cited here was found again.

---

## 0 · What the owner decided, 20 Nov 2026

**First review** (of the first draft):

| Question (first draft) | His word | Where it stands |
|---|---|---|
| 1 · The character, and the six rules | "rest recommended seem good" | **Accepted.** Rule 1 is tightened by the audit (§2). |
| 2 · The glide, boundless | the same | **Accepted,** and built in the worktree since (§3). |
| 3 · The member's room | the same | **Accepted.** |
| 4 · Light under your hand | "the light around the pointer is a bit too much. not as elegant and posh." | Revised, then settled below. |
| 5 · The lamp breathes | "rest recommended seem good" | **Accepted:** ten seconds a breath, for operators; a member's lamp stays still until its reading is proven. |
| 6 · The rail | "the menu lighting will possibily cause accessibility issues." | Revised, then settled below. |
| 7 · The trail's light | "rest recommended seem good" | **Accepted.** |
| 8 · Go to | the same | **Accepted as a trial:** the breath on staging for a week; if it is noticed on the fiftieth open, only the resting light stays. |
| 9 · The ember | the same | **Accepted,** last. |
| 10 · The older lights | the same | **Accepted,** as one small pass after the rest. |
| Every moment | "be careful of the bounding boxes clipping the glow - takes away from the immersion." | **Audited and fixed in the prototype;** the real app's risks are listed with `file:line` in §7. |

**Second review** (of the revision):

| Question (round 2) | His word | Where it stands |
|---|---|---|
| R1 · Light under your hand | "i really like the edge. can you see if doing it on button edges or having something similar interaction with buttons is also possible?" | **Settled: the edge.** Buttons: three ways drawn, one recommended, a build-ready spec (§4, moment 2). The choice of way is his (§10 Q1). |
| R2 · Your place in the rail | "do glide with light" | **Settled: the pill glides, with a light under it.** Built to be right, not argued: a build-ready spec (§4, moment 4). |
| R3 · The room's light stays while the page scrolls | not answered | **Open** (§10 Q3). The worktree already builds it this way (`c2c594c1`), and its code calls it "the owner's R3" (`RoomLight.tsx:10`). It is not his word yet. |
| R4 · The hour's light comes in through the window, and from behind the rail | not answered | **Open** (§10 Q4). |

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
5. **Light is violet.** Lime only as a dot's own glow, amber only on the lamp while Zitadel isn't answering. Never red. And it never carries a fact alone: the words beside it say the same thing. (Moment 4's first draft broke this: a light was the only thing marking where you are. As settled, the pill, the word and the dot mark it, and the light only keeps them company.)
6. **Every light has a still form,** drawn: under reduced motion it rests where it would have ended. Under forced colours it is gone. It never takes a line of text under 4.5:1 (`--hour-weight`, 1 in the dark room and 0.75 on paper, is the existing lever).

**What changed in rule 1.** The first draft allowed "a card, the rail, the window" to cut any light, and trusted a mask of 96px or more. The audit showed three gaps: a mask is itself a box (moment 2's pool), the window was cutting a small round glow flat (the lamp), and a scroller under a hairline is not a real object at all (moment 0 in the real shell).

## 3 · Moment 0: the glide you chose, with its edges gone

**Built in the worktree from this answer** (`c2c594c1`, refined in `7c487295`; I have not seen it on staging, and nobody has measured it there):

| | Before (`3f6afac2`) | Now, in the worktree |
|---|---|---|
| The layer | inside the one scroller, so it ended at the top bar's hairline | the column's own: `RoomLight` (`components/shell/RoomLight.tsx:15-18`) carries it up to the shell's column (`AppShell.tsx:96-97`, `relative isolate`), behind `TopBar` (`:98`) and `#app-scroll` (`:111`). `hour-light` is `inset: 0 0 auto`, 480px tall (`globals.css:1127-1147`). |
| The falloff | one straight ramp to `transparent 70%` | six eased stops, `--pool-stops` (`globals.css:1118-1125`) |
| The scrollbar's seat | cut the light a scrollbar's width short of the window | the scroller's track is transparent (`AppShell.tsx:111`) |
| When the page scrolls | the light left with the head | the light stays: it is the room's, not the page's (**open, §10 Q3**) |
| Path, size, timing, curve, strength, once-a-sign-in | as approved | as approved: `door-cross` and `door-settle` (`globals.css:375-382`) over `--door-open` (1800ms, `:2196`), Home only (`Home.tsx:396-400`, `app/page.tsx:25-26`) |

Measured in the prototype, which has the same shell: the light is the same strength each side of the bar's hairline (evening: 38 and 39 of 55 levels in the dark room, 19 and 19 of 28 on paper), so the hairline does not end it. Faint ink at each pool's strongest point is 5.30 to 5.77:1 in the dark room and 4.74 to 5.01:1 on paper, so a line that scrolls under the light stays above 4.5:1. (The build found one more ink that did not: `--warn-text` was 4.34:1 there and is now 4.83:1, `globals.css:715-716`.)

- **Frequency:** Rare, vivid. Once a sign-in (`firstTime`, `openedAlready`).
- **Still form:** the hour's pool, at rest.
- **What still ends it, on purpose:** the window and the rail (§7.2, and §10 Q4).
- **Left to do:** measure it on staging the way §7.4 says, and give My access the same light (moment 1).

---

## 4 · The moments, ranked by life added per unit of distraction

| # | Moment | Class | The gesture | Cost | Decided |
|---|---|---|---|---|---|
| 1 | The member's room | Rare, then still | The hour's light and the door's glide on My access | S | accepted |
| 2 | Light under your hand | Constant | The hairline of the thing your hand is on, lit where your hand is nearest: cards, and buttons | M | **settled: the edge.** Buttons: three ways, one recommended |
| 3 | The lamp breathes | Ambient | The mark's glow breathes while the reading is live | S | accepted |
| 4 | Your place in the rail | Frequent | The pill is carried to where you go, with a faint light under it | M | **settled: glide with light** |
| 5 | The trail lights | Occasional | A light rests on the step your change has reached | M | accepted |
| 6 | Go to is lit | Constant, by key | One breath of light behind Go to | S | accepted, as a week's trial |
| 7 | The ember | At rest | A clear queue's lime dot holds a low glow | S | accepted, last |

**Build order,** one per release (CRAFT §8.8), each judged on staging before the next, and each measured against its row in §7.3 before it ships:

1. **The boundless glide** (built in the worktree; to be measured on staging), **with the rail's two repairs.** The repairs (moment 4) are faults in what is live today, so they do not wait for a light.
2. The member's room.
3. Light under your hand: the edge, on every card frame and on `Button` in one change, because they share one rule and one listener. On every route (the owner, 20 Nov, on staging: "only home page shows the effect").
4. The lamp breathes.
5. Your place in the rail: the glide, with its light. After the lamp, because the rail's light is the lamp's, falling on your place.
6. The trail lights.
7. Go to is lit (one week on staging, then the owner's word).
8. The ember.
9. The older lights (`flash`, the lit card), as one small pass.

### 1 · The member's room

- **What.** My access gets Home's light: the hour's pool behind "Evening, Cody. Here's what you can use.", and the door's light crossing it once after a sign-in.
- **Why it gives soul.** A-24 §3.3: for most people who ever see Syndra, this page is Syndra. Today it has no light at all: `hour-light` is rendered in one place, `Home.tsx:396-400`, and a member lands on `MemberAccess` (`app/page.tsx:24-28`) with the door never passed to it. The thing you love is seen only by operators.
- **Motion.** Exactly moment 0: `door-cross` and `door-settle` over `--door-open` (1800ms) on `--ease-tint`, the pool at the hour's own `--hour-reach` times `--hour-weight`. No new token, no new keyframe.
- **Frequency and the 50th time.** Rare. Once a sign-in on this device; every later visit is the still pool. A member who signs in daily sees 1.8s of light a day, and it holds nothing.
- **Still form.** The hour's pool, at rest.
- **Never.** On a failed read of Zitadel (the page then shows its Notice and no rows; a warm light over that would be the wrong tone). Under forced colours.
- **Lit only after a good read** (owner, 20 Nov: "the glide light plays out while the page is loading and abruptly stops when it switches to account not found. not elegant."). Nothing is lit while the read is out, on an unread read or on a failed one, so nothing is ever cut off mid-animation. When the read lands read, the light arrives by settle: the pool fades in on `fade` over `--duration-settle` on `--ease-settle`; if the door is still due and the read landed within `--door-open` (1800ms, the crossing's own length) of the page's first render, the door crosses instead. A read already in hand at the first render shows the pool at once, as on Home. A pool that goes and comes back settles in; the door crosses once a sign-in, never again.
- **Cost.** S. `MemberAccess.tsx:272-275` hands its hour and the door to the shell's layer, as Home does (§3); `app/page.tsx` passes `door` to both pages. The title's second sentence is `text-faint`, which clears the pool's strongest (measured above).
- **Dial.** Quiet: the still pool only. **Warm, accepted:** pool and glide.

### 2 · Light under your hand (settled 20 Nov: the edge)

**The owner, first:** "the light around the pointer is a bit too much. not as elegant and posh." **Then, of the revision:** "i really like the edge. can you see if doing it on button edges or having something similar interaction with buttons is also possible?"

**What was wrong with the first draft.** A 240px pool at 14% on the whole card, a 90px sheen on every button, a press at 24 to 34%, and a 34px glow at 42% outside the focus ring. Four lights for one hand. All four are cut. (Round 2 also drew "the row" and "nothing follows"; neither was chosen.)

**What the best do on hover, and what I take from each.**

| Who | What they do | Source | What I take |
|---|---|---|---|
| Emil Kowalski | "A hover effect is nice, but if used multiple times a day, it would likely benefit the most from having no animation at all." | https://emilkowal.ski/ui/you-dont-need-animations (read) | Nothing moves and nothing replays. The answer to a hand is a state, the same every time. |
| Material 3 | One flat layer of the content's colour over the whole control: hover 8%, focus 12%, pressed 12%, dragged 16%. | material-web's own token file, `tokens/versions/v0_192/_md-sys-state.scss` in https://github.com/material-components/material-web (read; the m3.material.io page would not render) | The common hover is 8%, and a press is four points over it. The built hover and press stay; the light is not a second fill. |
| Apple, iPadOS and visionOS | The highlight "materializes a Liquid Glass shape between the pointer and the control it's moving over". On visionOS the system "highlights it, providing visual feedback that helps them confirm the item is one they want." | https://developer.apple.com/design/human-interface-guidelines/pointing-devices and `/eyes` (read, from each page's own data file) | The light takes the shape of the control. Its job is to confirm which thing. It is never a blob of its own. |
| Josh Comeau | "I like to make the enter animation quick and snappy, while the exit animation can be a bit more relaxed and lethargic." His example: 125ms in, 450ms out. | https://www.joshwcomeau.com/animation/css-transitions/ (read) | In on tint (120ms), out on settle (260ms): tokens Syndra has. |
| Atlassian | A list item's hover in 50ms; interactions 50 to 150ms. | https://atlassian.design/foundations/motion (research 1) | The in is inside the band. |
| NN/g, data tables | One aid for keeping your place across a row: a hover highlight or zebra, not both. | https://www.nngroup.com/articles/data-tables/ (research 2) | The row's tint is already that aid. The light is not a second one: it is on the edge, not the row. |
| Linear | The redesign set out "to reduce visual noise", "limiting how much chrome (blue in our case)" went into its colours. | https://linear.app/now/how-we-redesigned-the-linear-ui (read) | Colour is spent in lines and words, not in fields. |
| Microsoft, Fluent "Reveal" | A light that followed the pointer across borders and fills, shipped in 2017, later dropped from the default control styles. | **UNVERIFIED:** its documentation page is gone (404); from memory only | A caution, not a proof. What I keep is its quietest part, the border, and only on the thing your hand is on: never on its neighbours. |
| Research 6, §5.4 | "one glow used only on the focus ring and the primary action" | the researcher's own generalisation, marked UNVERIFIED there | No glow anywhere. Not outside the ring either. |

**The principle.** Posh is found, not shown: you notice it the third time, never the first. The light is the hairline of the thing your hand is on, lit where your hand is nearest. Nothing glows around the pointer, and no light falls behind a word.

#### The card (settled)

| | |
|---|---|
| Where | The card's own hairline: one pixel, on its inner edge, lit where it is nearest the pointer. Nothing in the fill. |
| Colour and strength | `--accent` at 50%, on the six eased stops (`--pool-stops`, `globals.css:1118-1125`), radius 320px along the edge. |
| *Measured* against the card | 1.7 to 1.8:1 beside your hand, 1.3:1 from the middle of a row, in the dark room; 1.8 to 1.9:1 and 1.4:1 on paper. Syndra's own strong hairline is 1.5:1 and 1.4:1. Inside the card, behind every word: nothing. |
| In, out | Opacity. In `--duration-tint` (120ms), out `--duration-settle` (260ms), both on `--ease-tint`. |
| The press, the row's tint | As built (`hover-tint`, `globals.css:2050-2065`). The light does not answer a press. |
| Keyboard | A focused row lights the card's hairline beside it (radius 90px, at the row's own height), at once. The ring is the crisp mark, as built (`globals.css:813-815`). |
| Still form | The same: colour only. Gone under forced colours. Not on touch. |

#### Buttons, the edge's sibling: three ways

In the prototype behind a switch (`html[data-btn]`), beside the card, at true size, in both themes. *Measured* numbers are the dark room's, then paper's.

| | **A · Where your hand is** (recommended) | B · The whole edge | C · As built |
|---|---|---|---|
| Outline | Its hairline is lit where your hand is nearest: 1.4:1 against its ground at rest, **2.2:1 under your hand, 1.7:1 at the far end** of a long button, which is just the built hover (2.1:1 and 1.5:1 on paper). | The whole hairline turns violet: 2.6 to 2.9:1 all round. | The fill tints; the hairline reads 1.7:1 (1.5:1). |
| Ghost | It has no hairline. Its edge is drawn only by the light: nothing at rest, **1.6:1 under your hand**, 1.3:1 at the far end. | Nothing: a ghost with a whole border is an outline button. As built. | The fill tints. |
| Filled (accent) | It has no hairline either, so the light is one pixel inside its edge, in the button's own ink: **1.6:1 against its face** (1.75:1 on paper). Its outline against the page does not change (4.05:1 hovered, with or without the light; 5.24:1 on paper). | A lit top edge, one pixel inside: a bevel. | It brightens. |
| Behind the word | **Nothing** (*measured:* 0 inside every button, 0 outside it). No label's contrast moves: each is the same number in A, B and C. | Nothing. | n/a |
| The built hover, press, focus ring | Unchanged. | Unchanged. | Unchanged. |
| The 50th time | A glint that follows your hand along a long button, and on a short one simply a lit edge. | A state: the same every time, and louder (the whole edge at 2.6:1 or more). | Nothing to tire of. |
| What it costs | On a short button every part of the edge is near, so it reads as lit all round. One pseudo-element per button. | It is a state and not a light: a button does not share the card's gesture. A ghost and a field get nothing. | The card answers your hand, and the buttons inside it do not. |

**Recommended: A, for outline, ghost and filled alike.** It is the card's own gesture at a button's size, by the same utility and the same listener, so it costs no second mechanism. It is also quieter than B where it matters: under your hand it is 2.2:1, and it falls back to the built hover by the far end, where B is 2.6:1 or more all round.

**Should the filled button take it? Yes.** Last round I left it as built, as "already the brightest thing". Drawn one pixel inside its edge and in its own ink, the light does not make the button brighter or change its outline against the page; it only shows where your hand is on it, which is the whole idea. Its built brightening stays as the answer you cannot miss. If the owner would rather the primary button stay plain, it is one line to leave out (§10 Q1).

**A field does not take it.** An `<input>` cannot carry the pseudo-element without a wrapper, and it already has its own answer: its border tints on focus (`Input.tsx:43`, `focus:border-accent-line`). It stays as built.

#### The spec, for the build

**One utility** (the prototype's `.rim`; a name for code to choose), beside `hover-tint` in `globals.css`:

- A `::after` on the wearer: `position: absolute`, `padding: 1px`, `border-radius` the wearer's own, `pointer-events: none`, masked to its own one-pixel ring (`mask: linear-gradient(#000 0 0) content-box exclude, linear-gradient(#000 0 0)`, with the `-webkit-` pair). The mask gives the light a shape; the light ends by its own falloff along the ring (rule 1).
- Its background is the one pool: `radial-gradient(circle <radius> at var(--px) var(--py), <the six stops>)`. `--pool-stops` exists (`globals.css:1118-1125`) but is tied to `--accent` and to two classes; it needs a colour of its own (`--pool-c`) to serve the filled button.
- `opacity: 0`; `1` on `:hover` inside `@media (hover: hover)`, and on `:focus-visible` (whole: a 600px circle at the centre). Transition: opacity, `--duration-tint` in, `--duration-settle` out, `--ease-tint`.
- `display: none` under forced colours.

| Wearer | Where the ring sits | Colour | Strength | Radius |
|---|---|---|---|---|
| `Card` (`Card.tsx:31`) | `inset: 0`, the inner edge. The card is `overflow-hidden`, which clips at the padding edge: a ring over the border would be cut away whole. | `--accent` | 50% | 320px |
| Button `outline` (`Button.tsx:66`, has a border) | `inset: -1px`, on its own border | `--accent` | 46% | 84px |
| Button `ghost` (`Button.tsx:67`, no border) | `inset: 0`, its outermost pixel | `--accent` | 46% | 84px |
| Button `accent` (`Button.tsx:65`, no border) | `inset: 1px`, one pixel inside its edge | `--accent-ink` | 34% | 84px |

(The prototype's buttons all carry a transparent one-pixel border, so its insets read one less; the pixels lit are these.)

**Never worn by:** `danger` and `dangerConfirm` (`Button.tsx:68-69`), a disabled button (`DISABLED`, `Button.tsx:74`), anything inside a consent's body, a Notice, a stale card, NotOpenYet, a table's column headings, the rail's rows, a field. Segmented is not a `Button` and does not wear it.

**Unchanged:** every built hover (`Button.tsx:65-67`), the press (`press-scale`, `globals.css:1036-1040`, worn at `Button.tsx:202`: 3%, answered on the frame), the focus ring (`globals.css:813-815`). `Button` is already `relative` (`Button.tsx:202`), so the pseudo-element needs no new box.

**One listener, for cards and buttons.** A host in the shell, beside the others (`AppShell.tsx:124-127`): one passive `pointermove` on the window, gated to one write a frame, ignoring touch pointers. From the event's target it walks up through the wearers (a button, then its card) and writes `--px` and `--py` on each, in that wearer's own box. The prototype's is 14 lines. A second listener for buttons would be a fault.

**Where a box could cut it.** Nowhere by construction: the light is the wearer's own edge. Two things to hold: on `Card` the ring is inside the overflow (above); and a button's ring is `border-radius: inherit`, so a label that wraps and grows its pill (`PILL`, `Button.tsx:55-58`) keeps its ring on its edge.

**For code to verify.** Frame cost of repainting a card-sized masked gradient on People at 200 rows: **UNVERIFIED** (buttons are small and cheap). The contrast test gains one line: no light behind a label.

- **Frequency.** Constant, which allows colour only. Nothing in the page moves.
- **Cost.** M: one utility, one listener, one class each on `Card` and three `Button` variants.
- **As built** (20 Nov, after the owner's two staging rounds: "these dont show the effect. and only home page shows the effect", then "this element is not responding to the hover. please look into other such ui elements as well"). Worn by every `card` and `panel` frame (so a frame cannot be drawn without it: `Card`, the stat tiles, the person page's strips, Insights' panels, the calm line on Home), by the lane map (`rim rim-card`, on its own ground), and by `Button`'s accent, outline and ghost; on every route, by one block in `globals.css` and one listener. Excluded: a stale card, a first-day card, a placeholder (a frame whose content is still a skeleton), an error (`ErrorBoundary`), a Notice, Not open yet, a consent's body on a page, anything in a dialog or popover, danger, dangerConfirm, disabled and pending buttons, fields, the rail and the column headings. Home's all-clear panel keeps it: it is a card. The layout check's `edge` rule finds, in a real browser, anything drawn like a card that does not wear it.

### 3 · The lamp breathes

- **What.** While Syndra's reading of the room is live, the mark's glow breathes: out and back once every ten seconds. Amber and quicker while Zitadel isn't answering (built). Still when there is no reading.
- **Why it gives soul.** It is the most literal answer to "alive", on every page, and it costs no attention. A lamp that is kept breathes; one that is not is a logo. It also keeps the rule honest: breathe means "still happening", and what is still happening is the watch.
- **Motion.** A new glow behind the mark: a pool of `--mark-dot` at 36%, **0.85 mark-widths out** (the first draft said 1.3), eased to nothing. Opacity 0.28 to 1 and scale 0.76 to 1, over `calc(var(--door-breath) / 2)` (5s) on `--ease-breathe`, alternating: one whole breath per `--door-breath` (10s, `globals.css:2192`). The dot, the ring and the arc never move. Waiting: the same glow in `--warn` on `--duration-breathe` (2400ms), beside the built amber dot (`globals.css:1870`).
- **Why 0.85.** *Measured* at 1.3: the rail's top cut the glow at 16% of its strength and its side at 7%, in both themes: a round glow with a flat top, in the one corner that is always on screen. On a phone's bar the window's side cut it at 9%. At 0.85 it is spent at every edge, at 1440 and at 390.
- **Frequency and the 50th time.** Ambient. In the corner of the eye a 22px mark changing over ten seconds is under notice; looked at, it is plainly alive. A hidden tab does not run it.
- **Still form.** The glow at rest (opacity 0.72). No breath.
- **Never.** Lime, dim or "sad": the rail's reading cannot prove those (7w-3 review, state (6)). Never the only place a fact is said: the Notice still says Zitadel isn't answering.
- **Cost.** S for an operator: `SyndraMark` gains `room="watch"` (`SyndraMark.tsx:31-70`) when the rail holds a real reading (`Sidebar.tsx:54`), one pseudo-element, one keyframe. A member's lamp stays still until its reading is proven (accepted).
- **The rule it changes.** FINAL §4: "Breathe … the only loop in the product." This is a second loop, on a healthy state. The owner accepted it at ten seconds a breath.

### 4 · Your place in the rail (settled 20 Nov: glide with light)

**The owner, first:** "the menu lighting will possibily cause accessibility issues." **Then, of the three ways drawn:** "do glide with light". So the light is back, as company and not as the mark. This section is the spec for building it right.

**What carries the place** (never the light). *Measured* on the prototype's copy of the rail, which uses the real tokens; the real rail is `Sidebar.tsx:150-172` and the tab bar `TouchNav.tsx:158-176`.

| Criterion | It asks | How it is met |
|---|---|---|
| 1.4.1 Use of colour (A) | "Color is not used as the only visual means of conveying information" | Weight (600 against 400), the pill's shape and the dot say it too. |
| 1.4.11 Non-text contrast (AA) | What shows a state, "such as whether a component is selected", is 3:1 against what is beside it | The dot, once it takes the tab bar's token (repair 2): 4.92:1 in the dark room, 5.75:1 on paper. Today it is 3.41:1 and 3.87:1. The pill's fill alone is 1.22:1 and 1.17:1, and the light is less: neither is relied on. |
| 1.4.3 Contrast (AA) | 4.5:1 for text | **The word at the pool's centre: 4.72:1 in the dark room, 5.51:1 on paper** (4.92:1 and 5.75:1 with no pool). A muted word on a lit row beside it: 8.5:1 and 8.2:1. |
| 2.3.3 Animation from interactions (AAA); reduced motion | "Motion animation triggered by interaction can be disabled" | Under reduced motion nothing travels: the pill and its light are there at once. |
| Forced colours | The state survives the system's colours | Repair 1: the place takes the system's selected colours. The pill, the dot and the light are all gone there. |
| Low vision, and people who cannot see a soft gradient | Something crisp | A bold violet word, and a dot at 4.9:1 or better. Nothing soft is relied on. |

**The spec, for the build.**

| | |
|---|---|
| **The pill** | As today: `--accent-soft` (`globals.css:628, 706`), the row's own radius (`rounded-nav` in the rail, 13px in the tab bar), under a `font-semibold text-accent-text` word (`Sidebar.tsx:162`) and the dot. It becomes one element that moves; the current row no longer holds the fill itself. |
| **Its movement** | The built indicator's: measured by `useGlideIndicator` (`lib/motion/glide-indicator.tsx:28`, whose selector already matches `aria-current="page"`, `:19`), moved by `indicator-glide` (`globals.css:1060-1064`): `transform` only, `--duration-settle` (260ms), `--ease-settle`. The same movement as Segmented, Tabs and the section band (`Select.tsx:116,158`, `Tabs.tsx:59`, `SectionNav.tsx:187`). |
| **One thing to add to the part** | `GlideIndicator kind="fill"` draws round caps (`glide-indicator.tsx:86-91`), which would turn the rail's 12px corners into a full pill. The rail's rows share one width, so nothing has to stretch: it needs a kind that is one box of the row's own radius. |
| **Before the first measurement** | The current row keeps its own fill, as the part already requires ("nothing is ever unmarked", `glide-indicator.tsx:16-17`). |
| **The light** | One pool under the pill: `--accent` at `calc(6% * var(--hour-weight))`, `closest-side`, the six eased stops. It is the rail's own layer, under everything in the rail (the brand, the rows, the account): the rail's root (`Sidebar.tsx:69`) becomes its stacking context. |
| **Its size** | Sized by the rail, not by the pill. As wide as the rail (252px, `Sidebar.tsx:69`), so `closest-side` spends it at both sides. 176px tall, and never taller than twice the distance from the row's centre to the nearer end of the rail, so it is spent before the rail's top and bottom. In the tab bar (`TouchNav.tsx:89-93`): as tall as the bar, 160px wide, and never wider than twice the distance to the bar's nearer side. |
| **Its movement** | With the pill: the same `transform`, duration and curve, set in the same frame. Its size, where it has to change, changes at once. |
| **Its strength, *measured*** | Strongest 12 levels of 255 in the dark room, 7 on paper. The lit rail beside the pill is 1.02 to 1.04:1 against the rail around it. In the dark room it stays under a card's own brightness (0.0071 to 0.0076 against 0.0077, relative luminance); on paper it can only make the rail dimmer. It is spent at every edge of the rail, the rail piece and the tab bar. |
| **Nothing travels** | On a first paint; on Back; to a place reached by a key (Go to, Enter on a row); while the door's light is crossing (rule 2: it is a first paint). The build needs to know a place was reached by the pointer: only a pointer press on one of the rail's own links, just before the path changes, lets the pill travel. Everything else places it at once. |
| **Reduced motion** | The pill and its light at once (`stillMotion()`, `glide-indicator.tsx:77`). |
| **Forced colours** | The light and the moving pill are not drawn; the current row takes `Highlight` and `HighlightText` (repair 1). |
| **Hover** | The other rows' hover stays `--hover`. The rail's rows never wear the edge's light. |
| **The tab bar** | The same, across: the pill (`TouchNav.tsx:162`) glides between tabs with its pool. The phone's navigation sheet (`TouchNav.tsx:583`) opens fresh each time, so nothing glides in it: it only takes repair 1. |

**Where a box could cut the light, and what holds it.**

| The box | `file:line` | The risk | The guard |
|---|---|---|---|
| The rail's scroller | `Sidebar.tsx:69` (`overflow-y-auto`, `pt-5`, `pb-[22px]`) | A pool taller than the room above the first row or below the last is cut by the rail's top or bottom. The first row's centre is about 86px down. | The height rule above. The pool is a child of what scrolls, so on a short window it leaves with its row. |
| The rail's sides | `Sidebar.tsx:69` (`w-[252px]`, `border-r`) | A pool wider than the rail is cut by it; one narrower than the rail has to be spent by its own width. | As wide as the rail, `closest-side`. |
| The pill | the moving element | None: the pill is a thing in front of the light, with a ground of its own. The pool is not clipped to it. | The pool is a separate, larger layer under it. |
| The tab bar | `TouchNav.tsx:93` (`pt-2`, `pb-[env(safe-area-inset-bottom)]`), at the window's foot (`AppShell.tsx:71`, `overflow-clip`) | A pool taller than the bar spills onto the page above it; with a safe-area inset the tab is not at the bar's middle. | The size rule, measured from the tab's own centre. |
| A row inside a folded group | `Sidebar.tsx:159` (nested rows) | A current place that is folded away has no box to measure. **UNVERIFIED** what the rail marks today in that case. | No box, no pill, no light. Whatever marks the group today stays. |

**The 50th time, said once.** CRAFT §12 says "Nothing fired dozens of times a day takes longer than 150ms", and a page change is that. This is 260ms, the same exception Tabs and the section band already are (S10). The owner chose it with that in view.

**Two repairs, which ship first** (they are faults in what is live, not character; §4, build order):

1. **Forced colours.** The current place takes the system's own selected colours (`forced-color-adjust: none`, `Highlight` and `HighlightText`), in the rail (`Sidebar.tsx:162`), the tab bar (`TouchNav.tsx:162`) and the phone's sheet (`TouchNav.tsx:583`). Today a background colour is forced to the system's (its alpha is kept, so a 15% fill stays invisible), the dot's with it, and every link takes the system's link colour, so only the weight is left: reasoned from CSS forced colours and **UNVERIFIED** on the real rail (`globals.css` has no forced-colours rule for it; its four belong to the lights, `1184, 1197, 1232, 1333`). In the prototype under emulated forced colours the place's computed background became the system's Highlight and its word HighlightText (*measured*), and the prototype draws that form in the system's own colours.
2. **The rail's dot takes the tab bar's token.** `bg-accent` (`Sidebar.tsx:170`) becomes `bg-accent-text`, which `TouchNav.tsx:167` already uses. *Measured:* 3.41:1 becomes 4.92:1 in the dark room and 3.87:1 becomes 5.75:1 on paper.

**The contrast test gains:** the word on the pill over the pool's centre (4.72:1 in the dark room is 0.22 above the floor, so it must be held, and measured again if `--accent-soft` or the pool's 6% moves), and the dot on the pill.

- **Frequency.** Frequent: one settle, the class's ceiling.
- **Still form.** The pill and its light, there at once.
- **Never.** Light as the only mark. Two lit places. A light on the rail's other rows.
- **Cost.** M: the indicator's new kind, one layer in the rail and one in the tab bar, the pointer signal.
- **Linear's lesson, kept in view.** Linear made its sidebar "a few notches dimmer" so the main content takes precedence (https://linear.app/now/behind-the-latest-design-refresh, read; CRAFT §5.4). That is why the pool is 6% and not the 20% of the first draft, and why it is held under a card's brightness.

### 5 · The trail lights

- **What.** Under a result, the change's trail has a soft light resting on the step the change has reached: the first open one. When Syndra reads the next step as true, the light moves down to it, and the dot fills 40ms behind. When the last step is read back, the light goes out. The change is home.
- **Why it gives soul.** It is the keeper walking your change to the door with the lamp, and it is information: where it is now, and that she is still looking. The trail already breathes truth ("each one settles only on what was observed", `ChangeTrail.tsx:41`); the light makes the waiting felt, not just listed.
- **Motion.** A pool 280 by 96px, the accent at 26% (times `--hour-weight`), inside the outcome's own block and spent within 20px of the trail's left. Travel: `translate` to the next step over `--duration-flash` (900ms) on `--ease-tint`. The dot's fill follows by `--light-lead` (40ms), the built rule. While a step is open the pool breathes (`--duration-breathe`, the licensed loop, and exactly its meaning). Leaving: opacity to 0 over `--duration-flash` on `--ease-flash`. Light is not page content, so it may travel the 36px between steps; nothing with ink in it moves. *Measured:* spent at every edge of its block; faint ink at its strongest 4.97:1 in the dark room, 4.94:1 on paper.
- **Frequency and the 50th time.** Occasional, and only by your own act. When Zitadel answers at once (the common case) the light is seen for under a second and goes: it shrinks itself.
- **Still form.** The light on the open step, no travel and no breath; gone when the change is home.
- **Never.** On a refusal or a failure: those have no trail (`changeTrailFor`, `ChangeTrail.tsx:34-35`). **On a removal's trail** (`RemovalDialog.tsx:446`, act `remove`): it takes access away, and grave is no light (§1). This was found by tracing where the trail is drawn; the first draft missed it. **On a give that waits for someone to press Send** (`sent` false, "Not sent to Zitadel yet."): Syndra is not moving it, and a breathing light would claim work that isn't happening. The light comes only once Syndra has sent the give (decided in the build review, 20 Nov 2026). Never ahead of a read (CRAFT §2.6): not on the route drawn before the answer (`PendingTrail`) either. And when the trail stops looking (`TRAIL_WATCH_MS`, six minutes, `ChangeTrail.tsx:19`) the light goes out with it.
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
- **Why it gives soul.** An all-clear is the room at rest, and a room at rest still has a pilot light. It also redraws the one built light that has an edge: the bloom is a flat disc (`globals.css:448-457`).
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
- **Later, for consistency, not a moment (accepted, Q10):** `flash` is a rectangle of light (`globals.css:477`) and `lit-card`'s pool is cut by its card's top (`globals.css:1207-1224`). Both take the edgeless falloff in one small pass once the rest are judged.

## 7 · No box cuts a light: the audit

**The owner:** "be careful of the bounding boxes clipping the glow - takes away from the immersion."

**The test.** A light is cut when it is there one pixel inside an edge and gone one pixel outside it, at 6% of its own strongest or more. A light that passes behind a thing with its own ground is not cut; it is behind something. The measuring scripts are kept outside the repo, in `~/.claude/jobs/146ae399/tmp/a25-rev/` (`measure_rest.mjs` for the lights that did not change in round 3, `measure3.mjs` for the buttons and the rail).

### 7.1 What was cutting light in the prototype, and what was done

| Moment | The box | Found | Done |
|---|---|---|---|
| 2 | The pool's layer was its card, and a mask 44 to 56px inside the card hid that | By construction, not measured (it is replaced): the mask was the box. On a four-row card the pool could only ever be a band. | The pool is gone. The light is the hairline itself. |
| 2 | The glow outside the focus ring, 34px at 42% | Not cut in the prototype, whose card had no overflow. The real card has (`Card.tsx:31`), 20px from a row's button. | Cut. The prototype's card now carries the real card's overflow. |
| 3 | The rail's top and side | *Measured* at 1.3 mark-widths: 16% of its strength at the top, 7% at the side, both themes. On a phone's bar, 9% at the window's side. | 0.85 mark-widths. *Measured:* spent at every edge. |
| 3 | The large lamp, against the column's edge | About 5%, by arithmetic | Given room. *Measured:* spent. |
| 4 | The rail piece and the tab bar | *Measured* on the pool before it was sized by its rail: 13% at the rail piece's top, 20% at the tab bar's side, and light outside both | The pool is sized by the rail (moment 4). *Measured,* round 3: spent at every edge of the rail, the rail piece and the tab bar, on the first place and the last, in both themes. |
| 2 | A button's edge (round 3) | *Measured:* nothing outside the button and nothing behind its word, for outline, ghost and filled, in both themes | Nothing to fix: the light is the button's own edge. |
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

Worktree `feat/addon-contract-a24-w4` at `7c487295`. Nothing in this table was measured in the app: it is read from the code, and each row is **UNVERIFIED** until its change is measured on staging.

| Moment | The box | `file:line` | The risk | The guard |
|---|---|---|---|---|
| 0, 1 | The one scroller | `AppShell.tsx:111` (`#app-scroll`, `overflow-y-auto`) | Any light drawn inside it ends at its top, which is the bar's hairline; the bar has no ground (`TopBar.tsx:39`). | **Guarded in the worktree:** `RoomLight` carries the light to the column (`RoomLight.tsx:15-18`, `AppShell.tsx:96-97`). A page never draws it inside the scroller. |
| 0, 1 | The scrollbar's seat | `AppShell.tsx:111` (`[scrollbar-gutter:stable]`) | Where scrollbars are not overlaid, a light inside the scroller stops a scrollbar's width short of the window. | **Guarded in the worktree:** the light is the column's, and the track is transparent (`[scrollbar-color:var(--faint)_transparent]`). |
| 0 | The falloff | `globals.css:1118-1125` | A straight ramp has an end the eye reads as a rim. | **Guarded in the worktree:** the six eased stops. |
| 0 | The layer's own height | `globals.css:1127-1147` (`height: 480px`) | A pool or the door's crossing that reaches past 480px is cut by its own layer. The worktree says every pool is spent inside it; not measured by me. | Measure it on staging. |
| 0, 1 | The window | `AppShell.tsx:71` (`#app-root`, `overflow-clip`) | Meant (§7.2). | None. §10 Q4. |

| 1 | My access's head | `MemberAccess.tsx:273` | The same scroller. It has no light yet. | It hands its hour to `RoomLight`, as Home does (`Home.tsx:396-400`). |
| 2 | The card | `Card.tsx:31` (`card overflow-hidden`) | Clips at the padding edge: a rim drawn over the border is cut away whole, and any glow outside a focus ring is cut 20px from a row's button. | The rim at `inset: 0`. No glow outside a ring. |
| 2 | A button | `Button.tsx:65-67, 202` | None: the light is the button's own edge, and a button has no overflow. | The ring's radius is the button's own. |
| 3 | The rail | `Sidebar.tsx:69` (`overflow-y-auto`, `pt-5`, `px-3`); the mark at `:71`, 22px (`SyndraMark.tsx:32`) | The glow's centre is about 31px from the rail's top. At 1.3 widths that edge cuts it. | 0.85 widths: 30px from its centre. |
| 3 | The bar's mark on a phone | `TopBar.tsx:43` (16px, inside `px-4`) | Its centre is 24px from the window's side. | 0.85 widths: 22px. |
| 4 | The rail, the tab bar | `Sidebar.tsx:69`, `TouchNav.tsx:93` | A pool taller than the room it has is cut by the rail's top or bottom; one taller than the tab bar spills onto the page. | Sized by the rail: the table in moment 4. |
| 5 | A dialog's panel | `Modal.tsx:392` (`overflow-y-auto`); the trail is in it at `GrantDirectAccess.tsx:257` (`mx-6`) | The trail's only homes are dialogs. A pool reaching more than 24px left of the trail, or below the last step into the footer, is cut by the panel's scroller. | Spent within 20px of the trail's left, and inside the trail's own height. The footer is not checked. |
| 5 | A removal's trail | `RemovalDialog.tsx:446` | Not a clip, a trace: this trail belongs to taking access away. | No light when the act is `remove`. |
| 5 | The trail itself | `CheckTrail.tsx:31` | None: it has no overflow. | None. |
| 6 | Go to's scrim | `Modal.tsx:371` (`fixed inset-0`) | The scrim is the window. A pool taller than twice its centre's height is cut at the window's top. | Sized by the window. |
| 7 | The card | `Card.tsx:31`; the dot at `states/index.tsx:139`, inside `px-6` (`:133`) | The ember reaches 20px from the dot's centre, which is about 27px inside the card. Clear by 7px. | Keep the ember at 20px or under. |
| Later | The lit card | `globals.css:1207-1230` | Its pool is centred on the card's top edge: cut at full strength by its own card, by design. | Q10's pass. |

### 7.4 How the build guards it

A jsdom test cannot see a clipped light. Each light's change measures itself on staging the way this audit did (the light on, the light off, the difference one pixel inside and outside each box in its row of §7.3) and records the numbers in its PR. A light that is cut does not ship.

---

## 8 · The checklist (CRAFT §12), run on the revised prototype

- **The 50th time.** Each moment has a class and sits under its ceiling. Moment 2 is colour on one pixel. Three sit at their ceiling and are flagged: the lamp's loop (accepted), Go to's breath (on trial), and the rail's glide, which at 260ms crosses the 150ms line as Tabs do (the owner's choice).
- **Truth.** The trail's light moves only on a read, and goes out with the watch. The lamp breathes only on a real reading. No warm line is written that a read does not back. A light never carries a fact alone: in the rail the pill, the word and the dot carry it.
- **Never on grave.** No light on a consent, a danger button, a refusal, a failure, a Notice or a removal's trail. The prototype draws a danger button beside the others: it takes none.
- **Space and type.** Three sizes on the page (34, 22, 14) and the fine 12.5. Names A to Z. Figures tabular. No box except the outcome's own block and the cards.
- **One pattern per job.** One utility lights every edge (a card's, a button's); one listener feeds it. The rail's pill moves by the movement Tabs already have.
- **Both rooms, both hands.** Dark and light by the tokens; the Still switch draws every still form; the forced-colours form of the rail is drawn; the keyboard gets the same light as the pointer (a focused row lights the hairline beside it, a focused button its own).
- **No box cuts a light.** *Measured:* at 1440 and 390, in both themes, every light is spent at every box, except the hour's at the window and the rail (§7.2).
- **Contrast.** *Measured:* no line of text under a light is below 4.5:1. The lowest are the word on the rail's pill over its pool in the dark room, 4.72:1, and faint ink under the morning pool on paper, 4.74:1. No light falls behind a button's label at all.
- **Character.** Each screen has one thing only Syndra would have: the lamp, and its light.
- **Not checked.** Anything by eye. Frame cost. The real app (§7.3). Real forced colours on Windows.

## 9 · For code to verify (not the owner's)

1. Every row of §7.3, measured on staging; first, the room's light that is already built.
2. The edge's frame cost on People at 200 rows.
3. `--pool-stops` takes a colour of its own, so the filled button's light can be its ink.
4. The indicator's new kind: one box of the row's own radius.
5. How the rail knows a place was reached by the pointer, so that only that travels.
6. What the rail marks today when the current place is inside a folded group.
7. Forced colours on the real rail, tab bar and sheet, on Windows: that only the weight is left today, and that the repair holds.
8. The contrast test gains: the word on the pill over the pool's centre, the rail's dot on the pill, faint ink at each pool's strongest, and no light behind a button's label.
9. Whether a member's shell holds a read that can honestly drive the lamp.
10. The rail's polling interval, so "live" has a number (`Sidebar.tsx:140`: "The rail is polled").

---

## 10 · The owner's questions, this round

1. **How a button answers your hand.** Where your hand is; the whole edge; or as built. And whether the filled button takes it. **Recommended: where your hand is, on outline, ghost and filled alike.** If you would rather the primary button stay plain, it is left out and nothing else changes.
2. **The rail's light is at 6%,** a little under the 8% you saw, and much wider: as wide as the rail and four rows tall, so it is seen as a soft fall of light and not as a brighter pill. At 6% the lit rail stays under a card's own brightness. **Recommended: 6%.** If it is too faint to find, 8% is the next step, and the rail is then a shade brighter than a card.
3. **The room's light stays while the page scrolls** (open from last round; the worktree already builds it so). To pass behind the top bar, the light has to belong to the column and not to the page, so it no longer leaves with the greeting when Home is scrolled. **Recommended: it stays.**
4. **The hour's light still comes in through the window, and from behind the rail** (open from last round). At its strongest at the window's top in the afternoon and evening, and at 82 to 90% beside the rail in the morning and the quiet hours (§7.2). That is the picture you approved. If it is one of the cuts you saw, the pools move wholly inside the page, which is a different picture: a pool that floats, not a light that comes in. **Recommended: keep it.**
