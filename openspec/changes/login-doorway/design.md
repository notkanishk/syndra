# Design — Login Doorway

## The composition

A single centred flex column with three light layers behind it and three text blocks around it.
Only the arch group is fixed-width (`430 × 392`, `margin-top: -52px`); everything else is pinned to
the ceiling (eyebrow, `top: 44`) or the floor (Syn line and credit, `bottom: 44`).

The button straddles the arch's baseline at `translateY(calc(50% + 14px))` — 14px below dead centre
so the arch's fade finishes before the button starts. That overhang is why the composition does not
compress: it collides with the Syn line below ~610px of viewport height, so the stage carries
`min-height: max(100vh, 800px)` and a short window scrolls instead.

`.login-message` sits at `bottom: 74px`, the Syn line's own height, so a state change fades one out
and the other in over the same slot rather than moving the layout.

## Decisions

### 1. Two colours are CSS, not animation

The arch's mask (`--door-arch-mask`) and its border colour hang off `[data-scene]` in `globals.css`,
not off a WAAPI keyframe. Both are the values a filling animation strands worst: a finished
`fill: "both"` animation keeps winning over inline styles long after it has dropped out of
`element.getAnimations()`, and clearing an inline `mask-image` deletes it outright when the value
only ever lived in the style attribute. Driving them from an attribute React already owns makes the
reset an attribute change, and lets the authored `transition: border-color .7s ease` fire on its own.

Everything that *is* animated is kept in a `running[]` array and cancelled by reference — the trap
the handoff spent four rounds on, held in one module (`ceremony.ts`) with the reason written above it.

### 2. The arch clip is a percentage

The entrance grows `.login-arch-clip` from `0%` to `100%` and the sign-in retracts it to `50%`.
The reference used `0px → 392px`. A percentage resolves against `.login-group`'s definite height,
so the narrow breakpoint scales the arch by changing one custom property and needs no second set of
keyframes.

### 3. The button is a link

The handoff asks for a real `<button>`, and gets one — in demo mode, where clicking reveals a list
rather than navigating. In OIDC mode the control is an `<a href="/auth/zitadel">`: it satisfies
every stated accessibility requirement (keyboard reachable, visible `:focus-visible` outline), works
with JavaScript disabled, and lets the click through untouched. The handoff is explicit that the
redirect is issued at the *start* of the sequence — not preventing the navigation is the smallest
way to honour that. A modified click (⌘/Ctrl/Shift/Alt) opens a new tab and leaves this page as it was.

### 4. The colours WAAPI needs are read from the stylesheet

`Element.animate()` needs literals, and the project's rule is that colour lives only in
`globals.css`. `openDoor()` reads `--door-fill`, `--door-fill-quiet`, `--door-shadow-rest` and
`--door-shadow-quiet` off the stage with `getComputedStyle` once at creation. No hex in a component,
and both button shadows keep their three layers so they interpolate cleanly.

### 5. Reduced motion is a duration multiplier

`prefers-reduced-motion: reduce` skips the entrance and the cursor-lit ring outright — the ring falls
back to its authored static top-lit mask. The sign-in and failure scenes still run, with every
duration and delay forced to `0`: they carry meaning, so they land as instant state changes rather
than being dropped.

### 6. Contents are gated on the scene, wrappers are not

Both message wrappers stay mounted so their live regions (`role="status"`, `role="alert"`) exist
before the change that fills them, and so the choreography always has a target. Their *contents*
render only in the scene that owns them: opacity is not hidden, and a screen reader would otherwise
read a message the page is not showing — and, in demo mode, tab into five invisible buttons.

### 7. The failure state is in the server HTML

`useState(failure ? "unreachable" : "idle")` runs on the server too, so a visitor arriving at
`/login?error=…` gets `data-scene="unreachable"` and the closed amber arch in the first paint. There
is no frame in which the door is open before it shuts. Retry drops the `?error=` with
`history.replaceState` so a reload does not re-raise a refusal already moved past.

### 8. Always the dark room

The doorway does not follow the theme. Its tokens are `--door-*` rather than `--ground`/`--ink`,
and `html:has(.login-stage)` carries the page colour and `color-scheme: dark` so a light-theme
operator does not get a lilac scrollbar framing the night.

### 9. Ambience is CSS, and opt-in

The breathing pool and the animated grain ship on. They are authored as CSS `@keyframes` rather than
WAAPI: they never need to be cancelled, reset, or coordinated with a scene, so they stay out of
`running[]` entirely, and they animate the two properties the choreography never touches — the
pool's transform and the grain's background position. Nothing can fight over a value, which is why
`ceremony.test.ts` asserts no scene ever animates the pool's transform or touches the grain at all.

They sit inside `@media (prefers-reduced-motion: no-preference)` rather than relying on the global
`reduce` rule to switch them off. Motion nobody asked for is exactly the motion a reduced-motion
visitor must never be *started* on. Measured at 120fps with both loops running, worst frame 9.3ms.

## What was left out

- **The demo controls** in the reference's top-right corner, which the handoff says to delete.
