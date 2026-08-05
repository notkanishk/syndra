# Tasks — violet, healthy, and the motion vocabulary

## Track 1 — Tokens

- [x] VM-01 Point the dark theme's `--accent`/`--accent-text`/`--accent-ink` at the violet ramp; recolour `--accent-soft`/`--accent-line` to match
- [x] VM-02 Add `--violet-600` to the ramp and `--accent-dense` to both themes — the AA fix for small labels on accent
- [x] VM-03 Add `--lime-400`/`--lime-700` to the ramp and `--healthy` to both themes, with no `-soft`/`-ink`/`-line` sibling
- [x] VM-04 Add `--focus-ring` per theme; move `:focus-visible` to it at 2px with a 4px offset on `press` timing
- [x] VM-05 Move `--avatar-from`/`--avatar-to` onto the violet axis, keeping the two tonal steps the olive pair held
- [x] VM-06 Add the six motion durations, three easings, and the stagger step and cap
- [x] VM-07 Rewrite the header comment: five roles, and why accent and healthy keep each other honest

## Track 2 — Motion

- [x] VM-08 `motion-tint` / `motion-press` / `press-scale` / `motion-settle` / `settle-in` / `settle-scrim` / `arrive-list` / `flash` / `flash-value` / `breathe` utilities
- [x] VM-09 Replace all 47 `transition-colors` / `duration-150` across 33 files with `motion-tint`
- [x] VM-10 Retire `panelIn`, `rowIn` and `shimmer`; `skeleton-bar` moves to `breathe`
- [x] VM-11 Buttons take `motion-press press-scale`; `active:brightness` drops, since the scale now carries the press
- [x] VM-12 `Modal` scrim fades and the card rises 40ms behind it; the source popover takes `settle-in`
- [x] VM-13 `ListStates` wraps its children in `.arrive-list` via a `display: contents` wrapper
- [x] VM-14 `useFlashOnChange` + `FLASH_MS`; wired to the rail's polled counts, row and value
- [x] VM-15 Degraded banner's mark takes `breathe`; the button spinner retires into it
- [x] VM-16 Reduced motion collapses `animation-delay` and `transition-delay` as well as durations

## Track 3 — Healthy

- [x] VM-17 Today's "Nothing needs you." dot moves from accent to healthy
- [x] VM-18 Makerspace health cells: values stay in `--ink`, a healthy dot marks the calm ones
- [x] VM-19 The provider-reachable panel drops its accent tint for a neutral panel and a healthy dot
- [x] VM-20 `EmptyState` gains `resolved`; set on the five states where empty means resolved, and pointedly not on "Nobody holds this role yet"

## Track 4 — Accent contrast

- [x] VM-21 Move every text-bearing accent fill to `bg-accent-dense` — Button, Badge, Card, Sidebar badges, ViewSwitch, Select, ChimeToggle, and the two preset pickers
- [x] VM-22 Leave fills that carry no text on `bg-accent` — dots, bars, checkboxes, the graph's project node

## Track 5 — The mark

- [x] VM-23 `.syndra-mark` in globals.css: base ring, conic-masked arc, lit dot, scaling off `--mark-size`
- [x] VM-24 `SyndraMark` component — three spans, no colour of its own
- [x] VM-25 Replace the rail's lime "m" square
- [x] VM-26 `app/icon.svg` — filled bands and a gradient fill rather than gradient strokes, with an OS-theme block
- [ ] VM-27 A raster apple-touch-icon. Deliberately not shipped: SVG covers every desktop browser and Android Chrome, and this is a home-screen icon rather than a tab

## Track 6 — Repo media

- [x] VM-28 `docs/assets/banner.png`; uncomment the README slot
- [x] VM-29 Repo description set via `gh repo edit` — live on github.com
- [ ] VM-30 **Operator-gated.** Social preview. GitHub exposes no REST endpoint for it, so `gh` cannot set it. File is staged at `docs/assets/social-preview.png`; upload at `github.com/notkanishk/syndra/settings` → Social preview

## Track 7 — Spec

- [x] VM-31 `proposal.md`, `design.md`, `tasks.md`
- [x] VM-32 `specs/operational-readiness/spec.md` — six ADDED requirements covering colour roles, AA on filled accent, the motion vocabulary, the loop licence, the changed-value mark, and reduced motion
- [x] VM-33 INDEX row; NEXT.md's "App-wide violet" item struck through

## Track 8 — Verification

- [x] VM-34 `design-system` canary extended: the new palette hexes, the motion vocabulary ban, the single-loop licence, `FLASH_MS` ↔ `--duration-flash`, no label on the bright accent, no healthy fill token, reduced motion collapsing delays
- [x] VM-35 Mutation-tested all six new guards — each fails when its invariant is broken, and passes when restored
- [x] VM-36 `useFlashOnChange` tests — superseded by VM-46. The original "restarts rather than ending early" case asserted the class stayed on for the full window, which was true *because* it had never left
- [x] VM-37 Live check: every token value matches the handoff exactly; mark is 22px with a 35% dot, 1.5px base ring, mask opaque to 88%, conic arc at .62
- [x] VM-38 Live check: `arrive` measures 0/40/80/120/160/200/200/200/200ms at 420ms on `ease-settle`, wrapper is `display: contents`
- [x] VM-39 Both themes checked in the browser
- [x] VM-40 `bun run test && bun run lint && bun run build`

## Track 9 — Review defects

- [x] VM-41 `settle-scrim` moves off the `arrive` keyframes onto an opacity-only `fade`. The scrim is the card's parent, so its 8px translate compounded into an 18px dialog rise and dragged a `fixed inset-0` element off its own bottom edge
- [x] VM-42 `useFlashOnChange` takes `ready`; the gate considers the previous render too, since the placeholder-to-real commit carries a value change and a readiness change at once
- [x] VM-43 `Sidebar` threads `settled={!isPlaceholderData}` — this is what stopped every nonzero badge flashing on page load
- [x] VM-44 The drift chime had the identical bug and louder consequences: an audible alert on every page load. Gated on the same real-reading check
- [x] VM-45 `useFlashOnChange` releases the class for one frame so a second change inside 900ms replays instead of silently extending the first
- [x] VM-46 Rewrote the flash suite. The old "restarts rather than ending early" case asserted the class stayed on for the full window — which was true, and was the bug
- [x] VM-47 `design-system` guard: resolve the scrim's animation name and assert its keyframes carry no transform
- [x] VM-48 `useIndicators` chime tests — silent on first payload, sounds on a rise between real readings, silent when steady or falling
- [x] VM-49 Mutation-tested all six fixes; dropped `betweenRealReadings` from the chime after mutation showed it dead — the early return already keeps the placeholder from seeding `previousDrift`
- [x] VM-50 Measured in Chrome: scrim animation `fade`, never translates, card travel exactly 10px, bottom gap 0; flash restart `currentTime` 508ms without the release vs 8ms with it; zero flashes and zero chimes across a real placeholder-to-real load
