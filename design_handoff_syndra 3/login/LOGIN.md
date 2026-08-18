# Handoff: Syndra — login screen

## Overview

`Syndra` is the access-management tool for a single academic makerspace. It is a
personal, open-source project, not a commercial product. The name comes from
**Syn**, the Norse goddess who keeps the door of Frigg's hall, bars it against
those who should not enter, and is invoked as a defence at trials — which is
what the application does: it decides who gets in, and keeps the record of why.

This bundle covers **one screen: `/login`**. It is the only unauthenticated
route. There is exactly one action on it — sign in through Zitadel. There is no
email field, no password field, no "or continue with", no sign-up, no password
reset. Zitadel is the sole identity provider and owns all of that.

The screen is deliberately ceremonial: an arch (the doorway), an orb suspended in
it, the wordmark, and a single button that sits on the arch's threshold. Its
three states — resting, opening, refused — are told through the arch itself
rather than through banners.

## About the design files

The files in this bundle are **design references created in HTML**. They are
prototypes that show the intended look and behaviour. They are not production
code to lift wholesale.

The task is to **recreate this design in the target codebase's existing
environment** — React, Vue, Svelte, a server-rendered template, whatever the
project already uses — following its established patterns, component library
and conventions. If no environment exists yet, choose the most appropriate one
for the project and implement the design there.

Two things in the reference are worth carrying across nearly verbatim, because
they were arrived at by fixing real bugs:

1. The **animation reset** approach (`running[]` array, see *Interactions*).
2. The **mask geometry** on the arch and the orb rings (see *Gotchas*).

## Fidelity

**High fidelity.** Colours, typography, spacing, easing curves and durations
below are final and exact. Recreate the UI pixel-perfectly using the codebase's
own libraries and patterns.

---

## Screen: `/login`

**Purpose.** An unauthenticated visitor arrives, understands what the
application is, and signs in through Zitadel. Members and staff use the same
account. A visitor who cannot get in should learn that from the page, not from a
console error.

**Canvas.** Designed at 1240 × 800. The composition is centred on both axes and
fills the viewport, but it needs a floor: use `min-height: max(100vh, 800px)`.
Below ~610px of height the button's overhang collides with the Syn line, so a
short window must scroll rather than compress. Nothing here is fixed-width
except the arch group.

### Layout

The stage is a single centred flex column with three absolutely positioned
layers behind it and three absolutely positioned text blocks around it.

```
┌─ stage (100vh, bg #0a0b08, overflow hidden, padding 56px) ────────────┐
│  .floor       abs, bottom 0, height 420          (ambient floor glow) │
│  .pool        abs, bottom 196, 820×290, blur 48  (violet pool)        │
│  .pool-amber  abs, same box, opacity 0           (failure only)       │
│  .grain       abs, inset 0, opacity .4, overlay  (noise)              │
│                                                                       │
│  .eyebrow     abs, top 44, centred      "MAKERSPACE SYNDRA"           │
│                                                                       │
│         ┌── .group  430 × 392, margin-top -52 ──┐   ← in flow,        │
│         │   arch stroke + inner wash            │     centred         │
│         │   orb        abs, top 54, centred     │                     │
│         │   wordmark   "Syndra"                 │                     │
│         │   button     translateY(50% + 14px)   │   ← straddles the   │
│         └───────────────────────────────────────┘     arch baseline   │
│                                                                       │
│  .base        abs, bottom 44, column, gap 12                          │
│                 Syn line (17px, max-width 560, centred)               │
│                 credit row (12.5px, dot-separated)                    │
│                                                                       │
│  .handoff     abs, bottom 74, opacity 0   (sign-in state)             │
│  .error       abs, bottom 74, opacity 0   (failure state)             │
└───────────────────────────────────────────────────────────────────────┘
```

`margin-top: -52px` on the group is intentional. The button overhangs the arch's
baseline by ~35px, which makes the composition read as bottom-heavy when the
group is mathematically centred. The offset balances the gap above the arch
against the gap below the button.

`.handoff` and `.error` sit at the same `bottom: 74px` as the Syn line, so the
layout does not shift when the page changes state — the Syn line fades out and a
message fades in over the same slot.

### Components

#### Eyebrow
- Text: `Makerspace Syndra`
- 13px / 400, `letter-spacing: .18em`, `text-transform: uppercase`
- Colour `rgba(243,245,239,.4)`, centred, `top: 44px`

#### Arch (the doorway)
Two stacked layers inside a 430 × 392 box.

*Stroke* — `.arch`
- `border: 1.5px solid rgba(155,123,255,.5)`, **no bottom border**
- `border-radius: 190px 190px 0 0`
- `mask-image: linear-gradient(to bottom, #000 6%, rgba(0,0,0,.5) 54%, transparent 95%)`
  — the uprights dissolve before reaching the floor. This fade is the single most
  important detail on the screen; the whole direction is "no hard edges".
- `transition: border-color .7s ease` (used by the failure state)
- Wrapped in `.arch-clip` (`overflow: hidden`, height 392) so the entrance can
  reveal it top-down by animating the wrapper's height.

*Inner wash* — `.wash`
- Same box and radius, `background: linear-gradient(to top, rgba(127,90,240,.15), transparent 62%)`
- `mask-image: linear-gradient(to bottom, #000 60%, transparent 99%)`

#### Orb
46 × 46 box at `top: 54px`, four layers:

| Layer | Spec |
| --- | --- |
| Base ring | `1.5px solid rgba(155,123,255,.22)`, mask `radial-gradient(closest-side, #000 88%, rgba(0,0,0,.5) 97%, transparent 100%)` |
| Lit ring | `1.5px solid rgba(206,188,255,.92)`, `opacity .5`, mask `linear-gradient(180deg, #000 4%, transparent 76%)`, `transition: opacity .25s ease-out` |
| Bloom | `inset: -16px`, `radial-gradient(closest-side, rgba(170,142,255,.3), transparent 76%)`, `opacity 0`, `transition: opacity .3s ease-out` |
| Dot | 16px circle `#9b7bff`, `box-shadow: 0 0 34px 6px rgba(155,123,255,.55)` |

The base ring is always visible so the circle never disappears; the lit ring is
the layer that tracks the pointer.

#### Wordmark
- `Syndra`, Bricolage Grotesque 600, 58px / 1, `letter-spacing: -.03em`
- `margin-bottom: 34px`

#### Sign-in button
- Label `Sign in with Zitadel` + `→` (17px), `gap: 10px`
- Background `#7f5af0`, text `#f7f4ff`
- `border-radius: 14px`, `padding: 16px 30px`, 15.5px / 600
- Width is **auto** (sized to the label, ~232px), not the arch width
- `transform: translateY(calc(50% + 14px))` — straddles the arch baseline, 14px
  lower than dead centre so the arch's fade finishes before the button starts
- Three-layer shadow:
  ```
  0 26px 60px -18px rgba(127,90,240,.85),
  0 8px 24px -8px  rgba(127,90,240,.6),
  0 0 70px 10px    rgba(127,90,240,.18)
  ```
- Focus ring: `outline: 2px solid #c9b6ff`, `outline-offset: 4px`

#### Base text
- Syn line: `Syn — the goddess who keeps the door, and the defence called upon at trial.`
  17px / 1.6, `rgba(243,245,239,.55)`, `max-width: 560px`, centred, `text-wrap: pretty`
- Credit row: `Built in the lab it runs.` · `Powered by Zitadel`
  12.5px, `letter-spacing: .05em`, `rgba(243,245,239,.3)`, `gap: 14px`,
  separated by a 3px `rgba(243,245,239,.25)` dot

#### Messages
Both 2 lines, centred, `gap: 7px`, at `bottom: 74px`, `opacity: 0` at rest.

*Handoff (sign-in):*
- Head 16px / 600 `#c9b6ff` — `Handing you to Zitadel.`
- Sub 14.5px `rgba(243,245,239,.5)` — `You'll come back here signed in.`

*Error (provider unreachable):*
- Head 16px / 600 `#f5a524` — `Zitadel didn't answer.`
- Sub 14.5px `rgba(243,245,239,.5)` — `Nothing was signed in. Try again in a minute, or find a steward in the space.`

---

## Interactions & behaviour

Everything is driven by the Web Animations API in the reference. Use whatever the
codebase prefers, but keep the durations, delays and easings.

Default easing: `cubic-bezier(.22,.61,.36,1)`. Where noted, `E` =
`cubic-bezier(.16,1,.3,1)`.

### 1. Entrance — plays on mount, ~1.9s

| Target | From → to | Duration | Delay | Easing |
| --- | --- | --- | --- | --- |
| pool | opacity 0 → 1 | 1500 | 150 | default |
| arch-clip | height 0 → 392px | 950 | 120 | E |
| wash | opacity 0 → 1 | 900 | 560 | default |
| eyebrow | opacity 0 → 1, translateY 6 → 0 | 700 | 300 | default |
| orb | opacity 0 → 1, scale .6 → 1 | 800 | 640 | E |
| wordmark | opacity 0 → 1, translateY 12 → 0 | 750 | 820 | default |
| button | opacity 0 → 1, translateY (50%+26px) → (50%+14px) | 750 | 1000 | default |
| Syn line | opacity 0 → 1, translateY 8 → 0 | 750 | 1180 | default |
| credit | opacity 0 → 1 | 700 | 1360 | default |

The arch draws from the apex downward because the clip wrapper's height grows.

### 2. Sign-in — the door opens, ~1.2s

Fires on button click, then the real redirect to Zitadel happens. The animation
exists to cover the redirect's latency, which is otherwise a frozen page.

| Target | Change | Duration | Delay |
| --- | --- | --- | --- |
| button | scale 1 → .975 → 1 | 320 | 0 |
| button label | text → `Opening…` | — | 200 (timeout) |
| button | background → `rgba(127,90,240,.34)`, colour → `rgba(247,244,255,.62)`, shadow → quiet 3-layer | 700 | 200 |
| arch-clip | height 392 → 196px | 900 | 220 (easing E) |
| arch | opacity 1 → 0 | 950 | 220 |
| wash | opacity 1 → 0 | 780 | 200 |
| pool | filter `blur(48px) brightness(1)` → `brightness(1.75)` | 1000 | 200 |
| orb bloom | opacity 0 → 1 | 800 | 200 |
| Syn line | opacity 1 → 0 | 400 | 0 |
| handoff | opacity 0 → 1, translateY 8 → 0 | 700 | 400 |

The arch retracts *and* fades — retracting alone leaves the uprights with cut
ends. The wash must go to **0**, not a low value: at any non-zero opacity it
keeps holding the arch silhouette after the stroke is gone.

Issue the actual `window.location.assign(zitadelAuthorizeUrl)` at the start of
this sequence, not at the end. The animation is cover, not a gate.

### 3. Provider unreachable — the door stays shut

The inverse of sign-in, and the reason there is no red banner: the arch's fade
is **removed**, so the stroke renders as a complete closed line.

| Target | Change | Duration | Delay |
| --- | --- | --- | --- |
| arch | `mask-image: none` (set directly, not animated) | — | 0 |
| arch | `border-color` → `rgba(245,165,36,.5)` via the CSS transition | 700 | 0 |
| pool | opacity 1 → .15 | 800 | 0 |
| pool-amber | opacity 0 → 1 | 800 | 0 |
| orb | opacity 1 → .35 | 700 | 0 |
| Syn line | opacity 1 → 0 | 400 | 0 |
| error | opacity 0 → 1, translateY 8 → 0 | 700 | 350 |

Trigger on: Zitadel discovery/authorize endpoint unreachable, network failure, or
a returned error. It is a page state, not a toast.

### 4. Cursor-lit ring — continuous

On `pointermove` (passive listener on `window`):

```js
dx  = cursorX - orbCenterX
dy  = cursorY - orbCenterY
deg = atan2(dx, -dy) * 180/PI + 180     // gradient points AWAY from the cursor
near = max(0, 1 - hypot(dx, dy) / 420)  // 0 at 420px, 1 at the centre

litRing.maskImage = `linear-gradient(${deg}deg, #000 4%, transparent ${70 + near*14}%)`
litRing.opacity   = 0.4 + near * 0.6
bloom.opacity     = near * near
```

The `+ 180` is required: in a CSS linear gradient the first stop sits at the side
*opposite* the stated angle, so pointing the gradient away from the cursor puts
the opaque end toward it.

### 5. Keyboard parity

On button `focus`: set `focusLock = true`, force the ring mask to
`linear-gradient(0deg, #000 4%, transparent 80%)` (lit from below, where the
button is), ring opacity 1, bloom .9. On `blur`: release the lock, bloom 0, ring
opacity .5. While locked, `pointermove` skips the ring.

### 6. Optional ambience

- **Breathing pool** — `transform: translateX(-50%) scale(1)` → `scale(1.06)`,
  10000ms, `alternate`, infinite, `ease-in-out`.
- **Animated grain** — 6 `background-position` offsets on `steps(1, end)`,
  750ms, infinite (8fps). Offsets: `0 0`, `-37px 21px`, `52px -14px`,
  `-19px -46px`, `31px 38px`, `-58px 7px`.

Both are off by default in the reference and exposed as toggles for review. Ship
whichever the team likes; neither is load-bearing.

### 7. Responsive behaviour

The composition does not compress. The arch group is a fixed 430 × 392 and the
button overhangs its baseline, while the base text is pinned to the floor — the
two collide below ~610px of viewport height. Give the stage
`min-height: max(100vh, 800px)` so short windows scroll instead. Horizontally
there is nothing to do: the widest element is the 560px Syn line, which wraps.

At phone widths, reduce the wordmark to ~40px and the arch box proportionally
(keep the 430:392 ratio and the 190px corner radius scaled to match), or drop the
arch to a simple lit orb above the wordmark. The arch is the identity; the
overhanging button is not, and can sit below the arch on narrow screens.

### 8. Reduced motion

`prefers-reduced-motion: reduce` skips the entrance, the cursor-lit ring and the
ambience. The ring falls back to its authored static top-lit mask. The sign-in
and failure state changes should still be conveyed — as instant state changes if
necessary, since they carry meaning.

---

## State management

One enum is enough:

```ts
type LoginState = 'idle' | 'opening' | 'unreachable'
```

| State | Entered when | Leaves when |
| --- | --- | --- |
| `idle` | mount | button clicked |
| `opening` | button clicked | page navigates to Zitadel |
| `unreachable` | Zitadel unreachable or returns an error | user retries (back to `idle`, replay the entrance) |

Non-visual state: `focusLock` (boolean), plus the array of running animations.

No data fetching on this screen beyond the Zitadel authorize redirect. If the app
does a provider health check before enabling the button, a failed check should
put the page straight into `unreachable`.

### Reset — read this before writing the animation code

The one thing that broke repeatedly during design. **Keep every `Animation`
object you create in an array and cancel it by reference.** Do not reset by
calling `element.getAnimations().forEach(a => a.cancel())`:

- Finished `fill: 'both'` animations drop out of `getAnimations()` after a short
  while, while their filled values keep winning over inline styles. The reset
  then silently does nothing, and the failure state sticks forever.
- After cancelling, **write the authored values back explicitly**. Clearing an
  inline property (`el.style.maskImage = ''`) deletes it outright when the value
  only ever lived in the style attribute — the arch's fade never comes back.
- Anything a filling animation might strand (the arch's border colour) is better
  set directly with a CSS `transition` than animated. That is why `.arch` carries
  `transition: border-color .7s ease`.

If your framework has a real animation library (Framer Motion, GSAP,
`@react-spring`), use it and let it own teardown — the trap above is specific to
hand-rolled WAAPI.

---

## Design tokens

### Colour

| Token | Hex / value | Use |
| --- | --- | --- |
| Page | `#0a0b08` | login background (the app shell uses `#080906`) |
| Violet 500 | `#7f5af0` | primary button, active nav, focus |
| Violet 400 | `#9b7bff` | accent text on dark, orb dot, links |
| Violet 300 | `#c9b6ff` | handoff message head, focus outline |
| Ring lit | `rgba(206,188,255,.92)` | orb lit ring |
| Ring base | `rgba(155,123,255,.22)` | orb base ring |
| Arch stroke | `rgba(155,123,255,.5)` | arch border |
| Pool | `rgba(127,90,240,.3)` → `.08` → transparent | floor light |
| Amber | `#f5a524` | deadlines, broken assumptions, this page's failure state |
| Amber stroke | `rgba(245,165,36,.5)` | closed arch |
| Red | `#ff5c4d` | destructive only — **not used on this screen** |
| Lime | `#a3e635` | "healthy / nothing needed" only — not used here |
| Text | `#f3f5ef` | primary |
| Text 55 | `rgba(243,245,239,.55)` | Syn line |
| Text 40 | `rgba(243,245,239,.4)` | eyebrow |
| Text 30 | `rgba(243,245,239,.3)` | credit row |
| On violet | `#f7f4ff` | button label |

Violet is the primary accent and appears once per screen as a filled element.
Amber means a deadline or a broken assumption. Red is destructive actions only.

### Typography

| Role | Family | Size / weight | Tracking |
| --- | --- | --- | --- |
| Wordmark | Bricolage Grotesque | 58 / 600, lh 1 | −.03em |
| Eyebrow | Figtree | 13 / 400, uppercase | .18em |
| Syn line | Figtree | 17 / 400, lh 1.6 | 0 |
| Button | Figtree | 15.5 / 600 | 0 |
| Message head | Figtree | 16 / 600 | 0 |
| Message sub | Figtree | 14.5 / 400 | 0 |
| Credit | Figtree | 12.5 / 400 | .05em |

Google Fonts: `Bricolage Grotesque` (600) and `Figtree` (400, 500, 600).
Self-host both if the codebase already self-hosts fonts.

### Geometry

- Arch box 430 × 392, radius `190px 190px 0 0`, stroke 1.5px
- Orb 46, dot 16, bloom inset −16
- Button radius 14, padding 16 / 30
- Pool 820 × 290, `blur(48px)`, `bottom: 196`
- Floor glow height 420
- Stage padding 56, eyebrow `top: 44`, base `bottom: 44`, messages `bottom: 74`
- Group offset `margin-top: -52`

### Shadows

```
button rest:  0 26px 60px -18px rgba(127,90,240,.85),
              0 8px 24px -8px  rgba(127,90,240,.6),
              0 0 70px 10px    rgba(127,90,240,.18)

button quiet: 0 10px 26px -16px rgba(127,90,240,.5),
              0 4px 12px -8px   rgba(127,90,240,.25),
              0 0 34px 4px      rgba(127,90,240,.06)

orb dot:      0 0 34px 6px rgba(155,123,255,.55)
```

Both button shadows have three layers so they interpolate cleanly.

---

## Gotchas

Four things that cost real time during design. All are already handled in
`login-reference.html`.

1. **Data URIs in inline styles.** An SVG noise data URI written into a `style`
   attribute must contain **no semicolons** — not `;utf8,`, not `;base64,`. A
   semicolon terminates the declaration and the URI is truncated to
   `data:image/svg+xml`, which fails silently. Percent-encode `<` as `%3C` and
   `>` as `%3E`. Not an issue in a stylesheet, only inline.

2. **Ring mask geometry.** `radial-gradient(closest-side, #000 62%, transparent 100%)`
   on a 46px box erases a 1.5px border entirely: the border occupies 93.5–100% of
   the mask radius, exactly where alpha is running out. Keep the opaque stop at
   88% or beyond.

3. **Filling animations outrank inline styles.** See *Reset* above.

4. **Fade the wash to zero on sign-in.** Any residual opacity keeps the arch
   silhouette visible after the stroke has gone.

---

## Accessibility

- The button is a real `<button>`, keyboard reachable, with a visible
  `:focus-visible` outline.
- Focus lights the orb ring from below, giving keyboard users the same feedback
  the pointer gives.
- Contrast: button label `#f7f4ff` on `#7f5af0` is **4.18:1** — below AA 4.5 for
  normal text, but the label is 15.5px/600, which qualifies as large text (AA 3:1)
  and passes. Any *small* text placed on `#7f5af0` elsewhere in the app does not:
  darken the fill to `#6f4ae0` (5.2:1) for those. Syn line at
  `rgba(243,245,239,.55)` on `#0a0b08` ≈ 6.9:1. Credit row at `.3` is decorative
  metadata; raise it to `.4` if the team wants AA on it.
- Messages should be announced: `role="status"` for the handoff,
  `role="alert"` for the failure.
- Everything decorative (floor, pool, grain, arch, orb) is
  `aria-hidden`/presentational. The accessible content is the eyebrow, wordmark,
  button, Syn line and credit.
- Reduced motion is honoured, see *Interactions §8*.

## Assets

None. There are no images, no icon fonts and no SVG illustrations. Everything is
CSS: borders, radii, gradients, masks and one inline `feTurbulence` noise tile.
The arrow in the button is the character `→`.

## Files

| File | What it is |
| --- | --- |
| `login-reference.html` | **Start here.** Standalone, dependency-free implementation of the final login screen with all five behaviours. Opens directly in a browser. The demo controls in the top-right are for review only — delete them. |
| `Syndra Brand.dc.html` | The full design board from the branding exploration. Contains the login options that were rejected, four sidebar mark options, four accent themes, four typography settings, the ten orb studies, the five text placements, and a brand reference card (palette, naming rules, voice). Panel `8a` is the final login. Requires `support.js` alongside it. |
| `support.js` | Runtime for the board file. Not needed for `login-reference.html`. |

### Decisions locked in, for context

- Accent theme: violet primary (`#7f5af0`), lime demoted to "healthy" only.
- Login layout: arch centred, text pinned top and bottom (panel `9c`).
- Orb: base ring plus cursor-lit arc (panel `7a`).
- Typography: Bricolage Grotesque + Figtree, unchanged from the existing app.
- Voice: neutral and plain. Syndra never speaks in the first person. Warmth comes
  from naming the consequence and admitting what the software does not know.
- Name in the product: **Makerspace Syndra** in full, **Syndra** everywhere in-app.
