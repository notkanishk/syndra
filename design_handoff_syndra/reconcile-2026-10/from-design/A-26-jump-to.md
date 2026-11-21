# A-26 · Go to, and ask (N25, "Jump to anything", brought up to today)

**Asked by the owner, 20 Nov 2026:** revive the benched N25 and bring it up to date as something he can try. Of its scopes, in his words: "if i select people mode itll only search people".

**Status:** round 4, 21 Nov 2026. The owner tried round 3's five ways and said: "i love D and A. can you combine them?" **§0 is round 4: the chosen one,** one prototype with no options. §0b is round 3 (five ways; D and A were taken, B, C and E were not). §0a records what he decided after round 1. §1 to §8 are round 2 as it was, kept for its reasoning; its wells, chips and sections were rejected. Where they differ, the lower number is the newer. Not published by me: the lead publishes. Nothing here is built beyond what Go to already is.

**The prototype for round 4:** `design/n25-jump-to-prototype.html` (§0; it replaces round 2's file of that name). **Round 3's, kept as the record:** `design/n25-jump-to-options-prototype.html` (§0b). The copy to publish differs only in its title.

**Checked, and how (rounds 2 and 3; round 4's own check is in §0).** Run in headless Chrome at 1440 by 900 and 390 by 844, in both themes, and measured, not looked at. Every key path was driven and read back. Each light was photographed on and off against the boxes that could end it. The ground under every kind of text (82 readings in the dark room, 84 on paper) was read with the words taken away, over this page, over a white page and over a black one. Each motion was sampled frame by frame. Numbers marked *measured* come from that. I have not seen it with eyes. Mechanically: tags balance, the CSS parses with no warnings, the script raises no error.

**Read for this:** the 14 Oct board (`route-additions`, as data), `benched-additions.md`, `verdicts.md:322-333`, A-18, A-19, A-24 §8, A-25, CRAFT (§0.2, §9, §12), and the code. Every `file:line` is the main checkout, `feat/addon-contract` at `d121f973`.

---

## 0 · Round 4, 21 Nov 2026: the chosen one

**The owner on round 3:** "i love D and A. can you combine them?" Then he answered what was open. His words are final.

| | He decided |
|---|---|
| 1 · At rest | **D's way.** The lamp, the field, and one quiet line: the last place (Lately). `/` then `↵` goes back. |
| 2 · Entering a scope | **"Lamp lights, bar answers."** The lamp takes the scope's colour, and its light washes once through the bar from the lamp outward, fading over about a second. The bar presses in very slightly, once, as Arc's does. One gesture, starting at the lamp. |
| 3 · The scope's marker | **A small coloured pill, Arc's way:** filled in the scope's colour, with a soft glow of the same colour, beside the lamp. Not the bare coloured word. |
| 4 · Accepted | The lamp takes the scope's colour. "What needs you" only after `↓`. One list, best match first, when looking everywhere; A to Z inside a scope. On a phone it floats at the top with no scope buttons; a scope is entered by tapping the offer or typing `@` `#` `/`. |
| 5 · From A, kept | The quiet "Tab  only people" offer at the bar's right before Tab. The list grows beneath only once you type. |

**The prototype:** `design/n25-jump-to-prototype.html`. One file, 569 lines, no scripts from elsewhere (the two fonts are the only request). No options: the only switches are Light and Still, small, at the bottom left. Every path is typeable: a name, a kind's word then Tab (or a click on the offer), `@` `#` `/`, Backspace giving the typing back, arrows, Enter, a question, `⌘↵`.

### What it is

- **At rest:** the pane, pinned 96px from the top as Go to is today (`Modal.tsx:78`, `tablet:pt-24`). The lamp (32px) is the field's first glyph. One line beneath: the last place, with "Lately" at its right, already chosen. Nothing else.
- **Typing:** the list grows beneath, on the same glass, best match first, each row's kind as one word at its right. No headings, no counts, no fill on the chosen row: a point of the lamp's light sits beside it and its name comes up to ink.
- **A kind's word** ("peo", "role", "pag") puts **"Tab  only people"** at the bar's right, quiet. Tab or a click takes it.
- **In a scope:** the lamp is the scope's colour; the pill sits beside it ("People", 26px, filled, white words, a soft light of its own colour behind it); the placeholder becomes "A person"; the list is that kind, A to Z. Backspace on an empty field leaves and gives "peo" back. Esc, or a click on the pill, leaves and clears.
- **A question** is answered in place, the lamp opening once as the answer lands (D's). `↵` or `⌘↵` follows it on the map.
- **`↓` on an empty field** brings the rest of Lately, and then "what needs you" as the last line, only when something is waiting.

### The gesture, as drawn (question 2)

All of it is the lamp's light, and all of it starts at the lamp, on one key.

| When | What | How |
|---|---|---|
| 0 to 120ms | The lamp takes the scope's colour. Everything it lights (its halo, the pane's rim, the light behind) turns with it, because they share one value. | `--light`, a registered colour, on `--duration-tint`. |
| 0 to 100, back by 240ms | The bar presses in 2% and returns: 13px on the 660px bar, 7px on a phone. About the bar's own centre, so the field does not drift when a list is open beneath. | `scale(.98)`, ease-out in, `--ease-settle` out. Arc's is 5%; that was tried in round 3 at 3.5% and is too much travel on a bar this wide. |
| 0 to 420ms | Its light runs out along the bar, in the glass and along the rim. | The beam grows from a fifth of its length; the rim's light runs from 60px to the bar's width. `--ease-flash`. |
| 120 to 300ms | At its brightest. | |
| 300 to 1020ms | Gone, easing out. | `--duration-tint` + `--duration-flash`: no new duration. |
| 0 to about a second | The lamp's halo flares once (D's, 900ms) and the light behind swells from 60% to 85% and back (1020ms). | |

**The beam and A-25's grammar.** It is one soft light centred on the lamp: a half circle behind it and a long half ellipse ahead, on the same six eased stops as every other light. It is sized to be spent inside the bar: 6% of its strength is left at the bar's top, foot, left and right edge. *Measured* at its fullest against the bar at rest, of 255: 2 to 5 at the edges, 17 to 19 at mid-bar, 26 to 30 beside the pill. No mask and no box ends it. The rim's light is a hairline by its mask (a shape), and ends by its own fall.

**Its strength is set by the words, not by taste.** The placeholder sits in the beam. At 26% the placeholder holds 4.56:1 at the beam's brightest frame in the worst case (paper, People), so that is its strength. The vivid part is where no words are: the rim, the pill, the lamp.

**Less motion:** the lamp simply takes the colour, at once. No press, no beam, no rim, no flare, no breath on opening. *Measured:* 0 animations after Tab, the colour already there. **Less transparency:** the pane is solid `--surface-2`; nothing else changes.

### Each moment's class (CRAFT §0.2), and the dial

| Moment | Class | Dial | Under its ceiling? |
|---|---|---|---|
| The lamp's breath as Go to opens (D's) | Constant, light only | Quiet | A-25 moment 6. The panel is whole on the first frame. |
| Entering a scope: tint, press, beam, rim, flare | Occasional, by your own act | Vivid, once per scope | The light, yes. **The press moves the panel on a key: it bends "Nothing a key triggers animates".** The owner chose it. 2%, 240ms, once. |
| The lamp opens on an answer (D's) | Occasional | Warm | Yes. Once a question. |
| The chosen row's point of light | Constant | Quiet | It does not animate: it is placed. |

**Rules bent, and by whose word.** A key animates the panel (his, question 2). Light is not violet in a scope (A-25 rule 5; his, question 4), and this round the light behind the pane turns with the lamp too, which is mine: one lamp, one colour. One list, best match first, and not the built groups' order (`lib/go-to.ts:19`; his). It floats at the top on a phone, where the built one is a sheet (his).

### Build spec, as deltas from Go to as built

1. **Shape.** `Modal` stays `still pinned` (`GoTo.tsx:52`). The panel becomes one glass surface: `--surface-2` at `--glass` (78% dark, 84% paper) over `blur(36px) saturate(170%)`, radius 28px, one inset hairline. No well, no head, no keys line, no preview.
2. **Field.** 68px (62 on a phone); the lamp 32px (28) first; Bricolage 500 at 23px (19). `--ax`, the left axis, is the lamp's right edge plus 14px: what you type and every row start on it. The field's box never changes (*measured:* identical in 12 states, both widths).
3. **Rest.** One row: the first of Lately. `↓` shows five and "Needs you" last. Typing replaces it.
4. **List.** Flat, best match first, seven at most, kind at the right; inside a scope, all of that kind, A to Z. Up to 8.5 rows, then it scrolls inside itself and softens at the end that has more.
5. **The offer.** While the typed word is two or more letters and starts a kind's name: "Tab  only people" at the field's right, `--muted`, 13px. On a phone it is the kind's own words in its colour, 44px tall, tapped. Tab is taken only while it is offered; otherwise it is the browser's.
6. **Scope.** `@` `#` `/` as the first character, Tab, or the offer. The pill: 26px, radius full, `--kind-*-solid` under `--accent-ink`, a light of `--kind-*` behind it at 50% (times `--hour-weight`), spent inside the field. It is a button: its label says where you are looking and that pressing it looks everywhere again.
7. **The lamp's colour.** `--light`, registered as a colour, on the dialog; the mark's dot, arc, base and glow, its halo, the rim and the light behind all read it.
8. **The gesture.** As the table above. One function, called only when a scope is entered; never on leaving one.
9. **Proposed tokens.** `--glass`; `--kind-people`, `--kind-roles`, `--kind-pages`, each with `-text` and `-solid` (the accent turned on its own wheel, as §4); accent words on glass at `color-mix(in srgb, var(--accent-text) 78%, var(--ink))`. No new duration or curve.

### Measured (headless Chrome, 1440 by 900 and 390 by 844, both rooms; and looked at)

- **Keys, in all four views:** one row at rest (Becky Lynch); `↓` brings six, "Pending changes" last; Esc folds them; "b" lists best first and `↓` moves; "peo" offers "only people"; Tab (a tap on the offer, on the phone) makes the scope and empties the field; all eight people, A to Z; "be" narrows; Backspace on the empty field gives "peo" back; `@` `#` `/` make People, Roles, Pages; Tab with nothing offered moves the focus on; "who has op in laser" answers "3 people have Operator in Laser cutting."; `⌘↵` opens Insights with it; `/` then `↵` returns to the last place; opening a place makes it the one line. No script error.
- **Steady:** the field's box is 390, 96, 660, 68 in all 12 states at 1440, and 8, 12, 374, 62 at 390.
- **Contrast, from the pixels under each word,** over Syndra's own page: lowest at rest **5.17:1** (the placeholder, paper; the pill's words 5.18:1). At the gesture's brightest, sampled at 120, 240 and 400ms for all three scopes: lowest **4.56:1** (the placeholder, paper, People). Solid pane: 5.46:1.
- **Nothing cut:** no line overflows without an ellipsis; eight rows show whole; the light behind is not cut by the window; the beam is spent inside the bar (above); the offer is 44px on a phone.
- **Less asked for:** as above, both measured.
- **Looked at:** every state at both widths in both rooms, and the gesture frozen at 40, 100, 120, 240, 400 and 700ms. Two things were changed for the eye: the first beam was short and barely seen, so it was lengthened and the rim's wave added; the lamp turned too slowly to lead its own light (260ms), so it turns in 120.
- **CRAFT §12:** four sizes of type (23, 19, 14, 13). No box inside the pane. One gesture, once per scope. Its still form is the Still switch.

**Not checked.** The cost of a 36px `backdrop-filter` on the lab's slowest machine. A registered colour property in Safari before 16.4 and Firefox before 128: there the lamp's colour snaps and nothing else is lost. A phone's keyboard over the list (the prototype measures the window, not what the keyboard leaves). Over a page that cannot happen (all white under the dark room): not run again this round; round 3's D held 4.87:1. Whether Insights' reads may be made from the shell for every operator view. The Arc numbers remain **UNVERIFIED** as Arc's own (§0b). Code lines re-read at `d121f973`: `GoTo.tsx:52`, `lib/go-to.ts:19`, `Modal.tsx:78`, `globals.css:266-277`.

### The owner's questions, round 4 (all three answered 21 Nov 2026: as recommended, so the prototype stands as drawn)

**After round 4 (owner, 21 Nov 2026): "the roles modes on go to uses the same color as syndra, please change. i love the other two colors for people and pages".** Roles now take the aqua beyond Pages: the accent turned by -100 (`--kind-roles*: oklch(from var(--accent…) … calc(h - 100))`), so no kind takes Syndra's violet. Measured: white on the Roles pill 4.62:1 dark, 5.96:1 light; Roles words on the pane 6.83:1 dark, 4.64:1 light. People and Pages unchanged.

**And (owner, 21 Nov 2026): "add the pointer hover edge lighting and other hover effects as well to go to".** Go to takes A-25 moment 2's edge as the same rule, not a second one: the pane wears `rim` (the card's kind, 50% and 320px) and the dialog is opted into the one `HandHost` (`data-hand`), so the pane's edge lights under the hand in the lamp's colour (`--pool-c: var(--light)`, the scope's in a scope) and rests calm with the pointer still or away. The lamp-lit hairline moved to the pane's `::before`, since `::after` is the hand's. The scope pill answers as `Button` does (`rim rim-filled`, option A); rows take the list's `hover-tint`, and hover never moves the chosen row (the chosen row's look wins); the offer has a quiet hover. Nothing moves under the hand. Touch: none. Less motion and forced colours: as every edge (`rim`), so no light under forced colours.

1. **The press is 2%.** Arc's is 5%; round 3's A was 3.5%. **Recommended: 2%.** It is felt and not watched. Say if you want it deeper.
2. **The light behind the pane turns with the lamp** in a scope, so one lamp throws one colour. **Recommended: yes.** The other way keeps it violet (A-25 rule 5) and the lamp's own halo disagrees with it.
3. **The beam is soft on purpose:** the placeholder must stay readable under it. A brighter beam needs the placeholder to wait out the flash. **Recommended: keep it soft;** the rim, the pill and the lamp carry the colour.

## 0b · Round 3, 21 Nov 2026: five ways, by taking away (D and A taken into round 4; B, C and E not chosen)

**The owner on round 2:** "this looks so ugly. please make it more elegant. this is too cluttered. find more inspiration, give me more options. find a video or something of the arc browser site search animation and ux. elegant enriching posh ux". Then: "you can also look into winui3", and "also look at apple spotlight in macos tahoe liquid glass".

**The prototype:** `design/n25-jump-to-options-prototype.html`. One file, 690 lines, no scripts from elsewhere. Five ways behind five small letters at the bottom left (A to E), with Light and Still beside them. Every way is fully typeable: a name, `@` `#` `/`, a kind's word then Tab, Backspace out, arrows, Enter, a question, `⌘↵`. It opens on D, the one I recommend. `?opt=a` (to `e`) opens on another.

**Still his, and kept in all five:** the name Go to; glass; modes you enter by typing, as Arc does; the six sentences Insights reads; `⌘↵` follows the answer on the map. The mark is the light in A, C, D and E. B has no mark, in one line: the quiet one is only words, and the room's lamp is already in the rail.

### What was taken away, in every way

| Round 2 had | Round 3 |
|---|---|
| A 60px head holding the orb alone | Gone. The mark is the field's first glyph, or absent. |
| Four wells (field, list, preview, keys) inside the glass | **One surface.** The glass is thick (78% in the dark room, 84% on paper, over a 36px blur; C's acrylic 88%) and the words sit on it. Thin glass was what forced a well under every word. |
| Lately, Needs you and Ask, all before a key is pressed | At rest: nothing (A, E), or the one place you were last (B, C, D). `↓` on an empty field brings the rest. Ask's sentences appear when you type "who". |
| Three coloured chips in the keys line, always | Gone. A mode is offered at the field's end while you type its word, as Arc does. Only B keeps a keys line, as one line of plain words. |
| A chip with a key cap, a word and an × | One word. Backspace, Esc or a click leaves. |
| A preview pane, always there | Gone. B keeps it as a peek, on `→`. |
| Group headings in tracked capitals, counts, matched letters in bold | Quiet headings in B and C; none in A, D and E, where the kind is one word at the row's right. No counts. No bold (it also moved the letters beside it). |
| "Read as", with a pill for every name | One plain line, "You're asking who has Laser cutting · Operator." In D the lamp answers and the sentence itself names what she read. |
| A box round the answer, and round "can't" | No box. "Can't" is one sentence and the four sentences she can read, each a row you can press Enter on. |
| The prototype's own bar of five switches | Five letters in a corner. |

### The five ways

| | What it is | Its one moment | At rest | A mode is | Dial |
|---|---|---|---|---|---|
| **A · Arc bar** | One floating bar a fifth of the way down. The list grows beneath it only once you type, best match first, its kind at the right. | Entering a mode: the bar presses in 3.5% and returns (240ms), and its light washes once in the kind's colour (120ms up, 900ms away). Then it is calm. | The bar. Nothing else. | A small solid pill in the kind's colour, with a glow. | Vivid, once per mode |
| **B · Quiet palette** | Field, one hairline, one grouped list, one faint line of keys. The built sheet on a phone. | None that moves. `→` peeks at a person or a role before you go. | The last place, what needs you, the keys. | A plain word and "›", as Linear's breadcrumb. | Quiet |
| **C · Fluent** | An Acrylic flyout from the top of the window, with grain, lifted by its shadow and not by a border. No scrim: the page stays lit. | The chosen row's 3px accent pill glides to the next row (120ms). It arrives the Fluent way: 250ms, from the button when you clicked it. | The last place, what needs you. | A small tinted token. | Warm |
| **D · Lamp-led** (recommended) | The lamp is the field's first glyph, and the only thing that moves. What you type and every row beneath start on one axis beside it. The chosen row has no fill: the lamp puts a point of light beside it and its words come up to ink. | The lamp: one breath as Go to opens, one opening when an answer lands, one flare in the kind's colour when a mode is entered. | The lamp, the field, the last place. | **The lamp itself takes the kind's colour,** with the kind's word beside it. Nothing is added to the field. | Warm |
| **E · Spotlight** | After macOS Tahoe: a pill of liquid glass, mid-screen. Its rim is a lens: a 4px ring that shows the page sharper and brighter than the glass does. | One hairline of light on the rim follows your pointer. Colour only. | The pill, and the three kinds beside it as round glass buttons. They go when you type. | Its button, moved into the pill with its word. | Warm |

Spotlight is **its own way (E)** and not folded into another: its two ideas (the lens rim, the kinds as buttons) would have hidden inside D, and he asked to see it. Its rim is A-25's built edge ("light under your hand") in another dress, so if he likes it, it joins D at no new rule.

### What I took, and from where

| From | What they do | Source | Taken into |
|---|---|---|---|
| **Arc**, site search | Type a site, Tab: the field is scoped. A quiet "Tab to search" before; a small pill in the site's colour with a glow after. Backspace on an empty field leaves and gives the typing back. | Nielsen, https://blog.jim-nielsen.com/2023/site-search-in-arc-browser/ · https://suther.land/arc-browser-adds-support-for-site-search/ | Every way: the offer at the field's end, Tab, Backspace giving "peo" back. A's pill. |
| Arc, the motion | The whole bar springs toward 0.95 for 0.1s and returns; the site's colour, blurred wide, fades over 1.2s. One gesture, on one surface, and nothing else moves. | **UNVERIFIED as Arc's own:** measured by other projects against Arc (https://github.com/VictorKildahl/cra/pull/151, https://github.com/MartinPTielemans/ilo/issues/1397). No official video was found; the nearest is a review, https://m.youtube.com/watch?v=I1MRsFRESbY at 8:43. | A, whole. Its 1.2s is Syndra's own flash (900ms) after a tint (120ms): no new duration. And the rule for all five: **one moment each.** |
| **macOS Tahoe**, Spotlight | A single pill of glass mid-screen; results grow beneath only as you type. The scopes are small glyph buttons beside the field. | https://www.macrumors.com/2025/06/18/macos-tahoe-26-hands-on/ · https://www.macstories.net/news/macos-tahoe-the-macstories-overview/ | E's shape and its three buttons. "Nothing until you type", in A and E. Its `⌘1` to `⌘4` are not taken: in a browser those keys are the browser's tabs (**UNVERIFIED** by test). |
| **Liquid Glass** | It is defined by lensing at its edges, not by blur and a border; a highlight on the rim answers movement; the tint adapts so text stays legible. Reviewers found it weaker on light grounds. | https://developer.apple.com/design/human-interface-guidelines/materials · https://developer.apple.com/videos/play/wwdc2025/219/ · https://www.macrumors.com/2025/08/01/macos-tahoe-review | E's ring and its hairline. The thick tint, in all five. Paper is the thicker of the two (84%), and measured. **Honest cost:** the ring is a second `backdrop-filter`, 4px wide. Real refraction needs an SVG displacement filter that only Chromium runs; not drawn. |
| **WinUI 3 / Fluent** | "Use background acrylic for transient UI elements." It falls back to a solid colour "when the user turns off Transparency effects". "The acrylic recipe: background, blur, exclusion blend, color/tint overlay, noise". Durations 250, 167 and 83ms; "Fast Out, Slow In", `cubic-bezier(0, 0, 0, 1)`, "for objects or UI entering the scene". | https://learn.microsoft.com/en-us/windows/apps/design/style/acrylic · https://learn.microsoft.com/en-us/windows/apps/design/motion/timing-and-easing (both read 21 Nov) | C: grain, shadow for border, no scrim, the 250ms entrance on that curve with an 83ms fade. The solid fallback, in all five. The leading 3px pill on the chosen row is from memory of WinUI's list: **UNVERIFIED**. C's entrance is *not* a true connected animation ("animating the transition of an element between two different views"); it starts from the button and that is all. |
| Raycast, Linear, older Spotlight | One list and one line of keys; a scope as a quiet breadcrumb. | as §4 | B. |

### Recommended then: D, Lamp-led (the owner took D and A together; see §0)

1. **It takes the most away.** Entering a mode adds nothing to the field: the lamp that was already there changes colour. No pill, no token, no fill on the chosen row.
2. **It could only be Syndra's** (CRAFT §12, Character). A is Arc's bar, C is Windows' flyout, E is Apple's pill. D is the keeper lifting her lamp, which is A-25's whole idea.
3. **It keeps what he loved in Arc:** a colour, a glow, one animation on entering a mode. The lamp does all three.
4. **It bends one rule, not two or three** (below), and it is the only one whose words hold 4.5:1 even over a page that cannot happen.
5. `/` then `↵` still takes you back to where you were.

**Second: A,** if he wants Arc itself. It is the most thrilling for a week and the one most likely to tire: the bar itself moves under a key.

### Measured (headless Chrome, 1440 by 900 and 390 by 844, both rooms, all five ways; not seen with eyes beyond spot checks of the pictures)

- **Keys, in all 20 views:** rows at rest (0, 2, 2, 1, 0 for A to E); `↓` brings Lately; "peo" offers "only people"; Tab makes the mode and empties the field; people are all eight, A to Z; Backspace gives "peo" back; `@` `#` `/`; Tab with nothing offered moves the focus on; arrows; Enter; "who has op in laser" answers "3 people have Operator in Laser cutting."; `⌘↵` opens Insights with it; `/` then `↵` returns to the last place (B, C, D). No script error.
- **Nothing moves under the hand:** the field's box is the same in every one of 12 states (13 in B), in every view.
- **Contrast, read from the pixels under each word** (words taken away, the ground sampled, worst pixel kept), over Syndra's own page: lowest **4.62:1** (C, dark room, the read's time on the chosen answer: C has no scrim). A, B and E: 4.98:1. **D: 5.19:1.** With the pane solid: 5.07:1.
- **Over a page that cannot happen** (all white under the dark room, all black under paper): D holds 4.87:1 and 4.74:1. A, B and E fall to 4.31:1 (a faint word on a chosen row). C falls to 3.32:1: with no scrim, it leans on the page being Syndra's. Round 2's wells gave the same number over any page; round 3 gives that up for one surface, and says so.
- **Nothing cut:** no line overflows without an ellipsis; the list softens into the glass at whichever end has more; no light is cut by the window (measured); the lamp's halo and the pill's glow are spent inside the pane by their sizes (computed: under 6% of their strongest at its edge; not sampled); the kinds' buttons are 44px.
- **Less asked for:** under reduced transparency the pane is solid `--surface-2` and nothing else changes; under reduced motion nothing runs (0 animations after entering a mode) and every light rests where it would have ended.
- Mechanically: tags balance; the page raises no error in 20 driven views.
- **CRAFT §12, run on each way.** Four sizes of type at most (the field, the answer, 14, 13). No box anywhere. One thing each that moves, and it plays once per act. The still form is the Still switch. What breaks a line of it is listed under "Rules bent".

### Each moment's class (CRAFT §0.2)

| Moment | Class | Under its ceiling? |
|---|---|---|
| A: press and wash on entering a mode | Occasional, by your own act | The wash, yes. **The press moves the panel on a key: it bends "Nothing a key triggers animates".** Still: neither. |
| C: the 250ms entrance | Constant | **No.** Go to appears at once today (`GoTo.tsx:52`, `still`). It is Fluent drawn honestly; Still removes it. |
| C: the pill's glide, 120ms | Constant | Yes (under 150ms), `transform` only. |
| D: the lamp's breath as it opens | Constant, light only | A-25 moment 6, already accepted as a trial. The panel is whole on the first frame. |
| D: the lamp opens on an answer; flares on a mode | Occasional | Yes. Once a question; once a mode. |
| E: the rim's light follows the pointer | Constant | Yes: colour only. |

**Rules bent, by way.** A: a key animates the panel; light is not violet in a mode (A-25 rule 5). C: Go to no longer appears at once; no scrim, which a pinned `Modal` draws (`Modal.tsx:78`). D: rule 5 only, and further than round 2 did: the mark itself takes the kind's colour. E: three buttons at rest. A, D and E list best match first, not in the built groups' order (`lib/go-to.ts:19`). A, C, D and E float at the top on a phone; B keeps the built sheet.

**Proposed tokens, new this round:** `--glass` (78% and 84%); accent words on glass a step nearer ink, `color-mix(in srgb, var(--accent-text) 78%, var(--ink))`, which is what holds the link at 4.5:1 on a chosen row. The kinds' colours are round 2's (§4), plus a solid of each for A's pill.

**Not checked.** The cost of a 36px `backdrop-filter` on the lab's slowest machine. E's second filter on Safari. Whether `⌘1` can be had in a browser. Whether Insights' reads may be made from the shell for every operator view (as round 2: it decides who gets Ask). The code lines cited in §1 to §7 are round 2's; of them I re-read `GoTo.tsx:52`, `lib/go-to.ts:19` and `Modal.tsx:78` at `d121f973`.

### The owner's questions, round 3 (all six answered; see §0)

1. **Which way.** **Recommended: D.** A if you want Arc itself.
2. **In a mode, the lamp itself takes the kind's colour** (D). Round 2 kept the mark violet and coloured only its halo. **Recommended: it takes it.** It is what lets D do without a chip.
3. **At rest: one line, or nothing.** D shows the place you were last; A and E show nothing, and lose `/` `↵` (it becomes `/` `↓` `↵`). **Recommended: the one line.**
4. **Where "what needs you" is said.** At rest as a second row (B, C), or only after `↓` (A, D, E). **Recommended: after `↓`.** Home and the rail already say it; Go to stays one line.
5. **One list, best match first** (A, D, E), or the built groups (B, C). **Recommended: one list while looking everywhere.** In a mode it is A to Z either way.
6. **A phone.** Float at the top, clear of the keyboard, with a mode entered by tapping the offer or typing `@` `#` `/`; or keep the built sheet (B); or borrow E's three buttons for a thumb. **Recommended: float at the top, no buttons.** Modes are the 10%.

## 0a · What the owner decided after round 1, 20 Nov 2026

His words, after trying round 1: "i like glass, above, chip. yes to rest. i love how arc browser used to handle their equivalent of chips with the colors and glow and animations. this version still makes me ask a little more of syndra soul".

| Round 1's question | His word | Where it stands |
|---|---|---|
| 1 · The look | "glass" | **Glass,** over my Edge-lit. Made to hold with wells (§2). |
| 2 · The mark | "above" | **The door's orb, above the field,** and the source of the pane's light (§3). |
| 3 · Modes | "chip", as Arc did it, "with the colors and glow and animations" | **A chip,** in its kind's own colour, with a light of its own, made from what you typed (§4). |
| 4 · What it can be asked | "yes to rest" | **The six sentences Insights reads today.** The other four are their own slice. |
| 5 · The name | the same | **Go to.** |
| 6 · `⌘↵` | the same | **It follows the answer on the map.** |
| And | "a little more of syndra soul" | Three moments, drawn, each with a switch (§5). |

> **§1 to §8 are round 2, and were not chosen.** The owner rejected its look ("so ugly", "too cluttered"): the wells, the three chips in the keys line, the 60px head, the sections shown before a key is pressed and the preview pane are all gone. Read these for the reasoning and the code citations only; §0 is what to build.

## 1 · What N25 was, and what has changed

On 14 Oct he picked **3b, "Ask"**: one field, the mark, results grouped by kind, a preview for people and roles, sentences answered from Insights with "Read as" and "Follow it on the map →", `@` `#` `/` as scopes. Its look was left open between Glass, Halo and Edge-lit.

| Since 14 Oct | What it means |
|---|---|
| **Go to is built** (A-19): `GoTo.tsx`, a `still pinned` Modal (`:52`), on `/` and `⌘K` (`GoToHost.tsx:38-47`); it appears and leaves at once; Esc clears, then closes (`GoTo.tsx:47-48, 213`). **Lately is built** (`lib/go-to.ts:60-74, 113-118`). | N25 is Go to, grown. Its name stays Go to (`GoTo.tsx:21-22`). |
| **Insights reads questions already,** with no language model: `suggest()` (`insights/question.ts:145-190`), `sentence()` (`:99-114`), `toParams()` (`:23`), `useAnswer` (`insights/useAnswer.ts:28`). | Ask needs no parser of its own. |
| **A-25, the language of light:** no box cuts a glow; the edge (built, `globals.css:2161-2222`); the lamp; moment 6's pool behind Go to, on hold in favour of this. | Glass is drawn to those rules. Moment 6 comes off hold as part of it. |
| **Standing taste:** nothing moves under the hand; nothing a key opens animates the panel itself. | *Measured:* the panel is in one place at one size on every frame of every motion here. |

## 2 · Glass, made to hold

**What I found in round 1, and why it was the wrong reading of glass.** A plain translucent pane with words on it needs 86% opacity to keep a faint word at 4.5:1 over any page, and at 86% it is not glass. But 14 Oct's 4a had already answered that: the pane is glass, and **every word sits in a well**.

| Part | What it is | Radius |
|---|---|---|
| **The pane** | `--surface-2` at 50% in the dark room, 46% on paper, over `blur(28px) saturate(150%)`. No word is on it. It shows as the frame (12px), the gutters (10px), and the head above the field (60px), where the orb hangs. | 30 |
| **A well** | Opaque `--surface-1`, a `--line` edge. Four of them: the field; the list; the preview; the keys. | 22 |
| **A row** | The chosen row's tint, the answer, the sentences it can read: each inside the list's well. | 16 |
| **The edge** | The pane's own hairline, lit from the orb at its top and falling off along the edge: the built edge, at rest. | n/a |
| **The light from the top** | Moment 6's pool, behind the pane, centred on the orb, sized by the window. It shows round the pane and through its glass. Up in 120ms, to rest over 900ms; the panel never waits. | n/a |
| **Where a pane cannot blur,** or less transparency is asked for | The pane is opaque `--surface-2`. Nothing else changes. | n/a |

*Measured:*

- **Every word, over any page.** The lowest reading is 4.74:1 in the dark room (the link on the chosen answer) and 5.16:1 on paper (a read's time). **The numbers are the same over this page, over a white page and over a black one,** because no word is on the glass.
- **It is glass.** The pane's head reads (72, 71, 78) over a white page and (19, 18, 25) over a black one in the dark room; (217, 210, 239) and (133, 126, 154) on paper. The page comes through.
- **Rule 1.** The light behind: 0 at the window's edges at 1440, 1 of 255 at 390. The edge: 1.79:1 against the unlit hairline at the orb, nothing outside the pane, and at most 1 of 255 inside it.
- **Nothing moves.** The panel is at 96px and 602px tall on every frame, through every state.

14 Oct lit the pane from inside, from its top edge, with glows on the answer and outside the pane. Those are gone: a light inside a clipped pane is cut by the pane (rule 1). The light is behind the pane now, and the glass is what lets it in.

## 3 · The orb

The door's mark, 44px, above the field, in the glass. It is the lamp the pane's light comes from: the edge is lit from it, and the pool behind is centred on it.

| When | What its halo does | Tokens | *Measured* |
|---|---|---|---|
| Go to opens | Breathes once: up, then to rest. The panel, the caret and the list are whole on the first frame. | opacity 0 to 1 in `--duration-tint`, to 0.72 over `--duration-flash`, `--ease-tint`; scale 0.76 to 1 | fullest at 130ms, at rest 0.72; the panel in one place on every frame |
| A key is typed | Lifts a little, and settles. Opacity only: its size does not change. | to 1 in `--duration-tint`, back over `--duration-settle` | 0.72 to 0.99 at 160ms, back by 344ms; size 1 throughout |
| An answer has landed | Opens once, when the typing has rested a settle on a question it could answer. Once a question, however it was typed. | opacity to 1 and scale to 1.18, back over `--duration-flash`, `--ease-flash` | a 33-letter question typed over 2.1s: one opening, 210ms after the last key |
| A question is being read (the build; the prototype's answers are instant) | Breathes, the licensed loop: still happening. | `--duration-breathe` | not drawn |
| In a mode | Its halo, and the edge, take the mode's colour. The mark itself stays violet. | §4 | n/a |
| Still, and forced colours | The halo at rest; under forced colours, none. | n/a | nothing running, 0.72 |

Its halo reaches 18px (0.42 of its width), so it is spent before the pane's top and before the field's well (*measured:* 0 at both). It is never sad, never amber here, and never the only place a thing is said: the answer is in words, and so is "can't".

## 4 · The chip

**What others do, and what I take.**

| Who | What they do | Source | What I take |
|---|---|---|---|
| Arc | Typing a site's name and pressing Tab turned the typing into a pill in that site's own colour, with a glow, and Backspace gave the typing back. | **UNVERIFIED:** Arc's help pages refused the request (403); this is the owner's description and my memory | The typing *becomes* the chip, from where it was. A colour and a light of its own. Tab takes it. Backspace gives the typing back. |
| Linear | "Narrow your search by @-mentioning teams, users, status, and other properties." "This will automatically create and apply a filter to your search." | https://linear.app/docs/search (read) | `@` is a key people already know for this. Typing it is choosing the mode; nothing else is asked. |
| Raycast | "Aliases appear as a small badge next to the command name in Root Search results."; "typing the alias followed by a space focuses the first argument field automatically." | https://manual.raycast.com/command-aliases-and-hotkeys (read) | The key is shown on the thing it belongs to. The offer wears `Tab`; each mode's small chip wears its `@` `#` `/`. |
| Things | "the moment you hit a key, the results show up instantly."; "it automatically detects when you're typing a tag and lets you pull up an app-wide filter in milliseconds." | https://culturedcode.com/things/features/ (read) | Nothing waits for the chip. And typing a kind's own word ("peo") is noticed, and its mode offered. |
| W3C, the combobox pattern | Tab is not a combobox's key for accepting anything: "The combobox is in the page Tab sequence." | https://www.w3.org/WAI/ARIA/apg/patterns/combobox/ (read) | Tab takes a mode **only while one is offered**, the offer says so in words, and it is never the only way. Otherwise Tab is the browser's (*measured:* it moves the focus on). |

**The design.**

| | |
|---|---|
| **How one is made** | Type `@` `#` or `/` first: the key becomes the chip and is not kept. Or type a kind's own word ("peo", "roles", "page"): the field's end offers `Tab` "only people", and Tab (or a click) turns what you typed into the chip. Or click a mode's small chip (below). |
| **What it is** | At the field's start: its key, its kind in words ("People"), and ×. Colour never says it alone. |
| **Its colour** | Its kind's own (below): a fill at 16% over the well, a hairline at 46%, its word in the kind's text colour. *Measured,* the word on the chip: 4.88:1, 4.94:1 and 5.40:1 in the dark room (People, Roles, Pages); 5.52:1, 5.48:1 and 5.38:1 on paper. |
| **Its light** | A small pool of its colour behind it (42% × `--hour-weight`), under its fill and never under its word, spent inside the field's well (*measured:* 0 at the well's edges and outside it). |
| **The panel's light follows it** | While a mode is on, the edge and the orb's halo take that colour, and so does the caret. The mark stays violet. |
| **How it arrives** | From where the typing was, at the typing's own size, to its own: `transform` and `opacity` only, over `--duration-settle` on `--ease-settle`; its light opens over `--duration-flash` behind it. *Measured:* on the first frame the field is already empty and in its new place and the list is already that kind's; the chip is whole by 280ms; `transform` and `opacity` are all that animate. |
| **How it goes** | Backspace in an empty field, the first Esc, or ×. The field is the field again on that frame, and after Tab the typing is back ("peo"). The chip dissolves where it stood over `--duration-tint` (*measured:* gone by 145ms). |
| **In a mode** | Only that kind is searched; with nothing typed it lists that kind, A to Z, with a count ("People 3 of 8"). A sentence is only a name that matches nothing: no Ask. |
| **Where the modes are found** | Three small chips, each in its colour and wearing its key, at the keys line's end. A phone has no keys line and no handy `@`, so there they sit in one row under the field. |
| **Still** | The chip is there, or not. Nothing travels. |
| **Read aloud** | "Looking only among people. Backspace looks everywhere again.", once, when it is made. |

**The colours: proposed tokens, with no new hue written down.** Each is the accent turned on its own wheel, at the accent's own lightness and strength, so both rooms follow by themselves.

| Token | What it is | Why that way |
|---|---|---|
| `--kind-roles`, `--kind-roles-text` | `--accent`, `--accent-text` | Roles are what Syndra is about. They keep violet. |
| `--kind-people`, `--kind-people-text` | `oklch(from var(--accent) l c calc(h + 44))`, and the same of `--accent-text` | Violet's warm neighbour. |
| `--kind-pages`, `--kind-pages-text` | `oklch(from var(--accent) l c calc(h - 46))`, and the same of `--accent-text` (a step darker on paper: `calc(l - .05)`, to hold 4.5:1) | Its cool neighbour. |
| Never | lime, amber, red | Those say a state. A kind's colour stays on violet's side of the wheel. |
| Where a browser cannot turn a colour | every kind is the accent | The word and the key still say the kind. |

Bundles and Projects have no mode today, so they have no colour. I would not add two more: five neighbours of violet between cyan and pink are hard to tell apart, and the fifth lands beside red.

## 5 · More of her

A-18 says where her character is: "in *when* she speaks and *how exactly*. There are no extra words." CRAFT §9.2: her warmth is exactness at the right moment. So nothing here is a greeting, a joke or a flourish, and nothing delays a key. Three moments, each behind a switch.

| | **1 · She reads it back** | **2 · She says what needs you** | **3 · The lamp answers** |
|---|---|---|---|
| What | "Read as  Who has [Laser cutting · Operator]" becomes "You're asking who has [Laser cutting · Operator]." When she cannot: `Nobody and nothing here is called "night shift".`, or "That isn't a question Syndra can answer.", then "She answers only what she can read exactly, and she never guesses." | With nothing typed, under Lately: "Needs you", one row: "Pending changes · 3 changes are waiting for you to send". Only when something is waiting. | §3: a breath as it opens, a lift with each key, one opening when an answer lands. |
| Why it is her | A keeper says back what she heard before she answers, and says exactly which name she does not know. It is the truth rule, spoken. | A-18: "She speaks up only when something has changed for you." When nothing waits, she says nothing. | The lamp is where she is. It shows she has heard, without a word. |
| Truth | The sentence is `sentence()`'s own parts in a different order of words. | The same read as the rail's own counts (`Sidebar.tsx:33`). | It opens only on an answer that was read. |
| The 50th time | Words, at no cost. They are the same length as the plain ones. | One row, only when true. It sits under Lately, so `/` then Enter is still the place you were last. | The lift is the part that could tire: it is on every key. It is opacity only, 120ms up. |
| Cost | Copy only. | S: a row, from a read the shell already makes. | S: three small animations of one element. |

**Recommended: keep all three.** If one has to go, it is the lift on each key, and the breath on opening and the opening on an answer stay.

**Also drawn, and part of the chip:** the pane's light taking the mode's colour (§4). It is the most Arc of anything here, and it bends A-25's rule 5 ("Light is violet"), so it is the owner's (§8 Q2).

## 6 · The panel, part by part

| Part | What it is |
|---|---|
| The pane | Pinned 96px from the top, 760px wide, 602px tall, always. |
| The head | Glass, 60px, the orb. |
| The field | A well. The display face at 23px. The chip at its start; what Tab offers at its end. |
| The list | A well, scrolling inside itself. The built groups in the built order (`lib/go-to.ts:19`), four a kind, the letters that matched in bold, the kind at the right, `↵` on the chosen row. |
| The preview | A well, 270px, always there. A person: what they have, through what, until when. A role: who has it. Anything else: its one line. Always the time of the read, and what Enter will do. |
| Ask | A sentence she can read is the first thing in the list: how she read it, one sentence of answer, the names, the time of the read, "Follow it on the map →". Enter on it, or `⌘↵` from anywhere, opens Insights with it asked. |
| A question she cannot read | Two states in one seat: one of her own openings being typed ("Syndra answers these, and only these."), or no sentence fits. The same four sentences under both. |
| Empty | Lately (five), Needs you (when something does), Ask (three questions; Enter asks one). |
| The keys | A well. `↑` `↓` move. `↵` open, or "follow it on the map", or "ask it". `⌘` `↵` while there is an answer. `Esc` clear, then close. The three modes at its end, or `⌫` "everything again" while one is on. |
| A phone | The built sheet, full height from the foot: the head and the orb (36px), the field, the modes in a row, the list. No preview, no keys line. |

## 7 · The build

One slice, in this order. Each step is useful alone.

1. **Modal's glass.** `Modal.tsx:392` draws the panel (`bg-surface-2`, `tablet:rounded-[22px]`, `overflow-y-auto`). A `tone="glass"` for this one caller: the pane's fill and `backdrop-filter`, radius 30, padding 12, gap 10, and it stops scrolling (its wells scroll). Opaque where `backdrop-filter` is not supported and under `prefers-reduced-transparency`. `GoTo.tsx:52` becomes `size="lg"`, still `still pinned`.
2. **Wells.** One utility: `--surface-1`, a `--line` edge, radius 22. Three radii as tokens (30, 22, 16). `GoTo.tsx:156-217` becomes head, field, body, keys; the sticky field row (`:160`) and `field-row` (`globals.css:1494`) are not needed here, because the list scrolls in its own well.
3. **The orb.** `SyndraMark` at 44 (`SyndraMark.tsx:31-32`) in the head, with a halo of its own (0.42 widths). Its three answers hang off what `GoToPanel` already knows: mount, `onChange` (`GoTo.tsx:148-151`), and the answer's arrival.
4. **The edge and the light.** The built edge (`globals.css:2175-2222`) lights on hover, under `[data-hand]`, which is `main` only (`AppShell.tsx:118`). The pane wears it at rest: lit while open, its pool fixed at the top's middle, a frame's own strength and reach (`globals.css:2183-2186`), its colour the mode's. Moment 6's pool goes on the scrim (`Modal.tsx:371`), centred on the orb and sized by the window, as A-25 specifies.
5. **Modes.** `goTo(source, query)` (`lib/go-to.ts:125`) takes a scope. In `onKeyDown` (`GoTo.tsx:118-129`): Tab only while a mode is offered; Backspace in an empty field. In `onChange`: a leading `@` `#` `/`. `/` still opens Go to from outside a field (`GoToHost.tsx:41`), so the two never meet.
6. **The chip.** The kind tokens beside the accent's (`globals.css:638-643, 716-721`), inside `@supports (color: oklch(from red l c h))`; relative colours are new to this stylesheet. The chip is `FilterChip`'s coloured sibling (`FilterChip.tsx:46`). Its arrival is two short animations of `transform` and `opacity`; its going is a copy that fades where it stood.
7. **The list, the preview, Ask.** As round 1: four a kind (`go-to.ts:110` lists all today), bold letters, `↵`, counts; the preview's reads only for a row that has been chosen for a moment; `suggest()` on what is typed when no mode is on, the Insights model loaded on the first question.
8. **Her words, and what needs you.** The copy of §5, checked against the copy guide. The row from `useIndicators` (`Sidebar.tsx:33`), the rail's own read.

**Where a box could cut a light.**

| The box | `file:line` | The risk | The guard |
|---|---|---|---|
| The pane | `Modal.tsx:392` | It clips at its padding edge: a ring over its border is cut away whole; a halo wider than the head is cut by the pane's top. | The ring at `inset: 0` (`globals.css:2175-2180`). The halo at 0.42 widths: 18px, in a 60px head. |
| The field's well | the new well | The chip's light is 13px above and below a 32px chip, in a 60px well. | Sized by the well; *measured* 0 at its edges. |
| The scrim | `Modal.tsx:371` (`fixed inset-0`) | A pool taller than twice its centre's height is cut by the window's top. | Sized by the window. |

**Guards, as tests.** `GoTo.test.tsx` and `go-to.test.ts` gain: a mode searches one kind and nothing else; a sentence in a mode asks nothing; Tab is not taken when nothing is offered; Backspace gives back what Tab took; the pane's box is the same in every state; every text node of the panel is inside a well; an answer names its read's time and never a number when the read failed; a member sees no modes and no Ask (`GoTo.tsx:91-93`). The contrast test gains each kind's word on its chip in both rooms (the lowest is People in the dark room, 4.88:1) and the link on the chosen answer (4.74:1).

**Two rules this bends, both by his word.** CRAFT §12, "Nothing a key triggers animates": the chip's arrival is 260ms of a small thing, and the field is ready on the first frame. A-25 rule 5, "Light is violet": a mode's chip, and the pane's light while it is on, are violet's neighbours.

**Not checked.** Whether Insights' reads may be made from the shell for every operator view: **UNVERIFIED**, and it decides who gets Ask. The cost of `backdrop-filter` at 28px on the slowest machine in the lab. Anything by eye.

## 8 · The owner's questions

1. **More of her: which to keep.** She reads it back; she says what needs you; the lamp answers. **Recommended: all three.** If one goes, the lift on each key.
2. **The pane's light takes the mode's colour,** on the edge and the orb's halo, or only the chip does. It changes A-25's "Light is violet" to "violet, and in Go to a mode's own neighbour of it". **Recommended: it takes it. The mark itself stays violet.**
3. **Three colours, or five.** People, Roles and Pages have modes and colours. Bundles and Projects have neither. **Recommended: three.**
4. **Tab.** It takes a mode only while one is offered at the field's end, and is otherwise the browser's. **Recommended: yes.**
5. **A phone.** With no keys line and no handy `@`, the three modes sit in one row under the field. **Recommended: yes; it is the only way a thumb can choose one.**
6. **How much glass.** The pane is 50% in the dark room and 46% on paper. Nothing that is read depends on it, so it is purely taste. **Recommended: as drawn; say if you want more of the page through it.**
