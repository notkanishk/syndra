# Violet, healthy, and the motion vocabulary

## Why

`/login` was built to the brand handoff and left the rest of the app behind it.
The doorway spends `--violet-500/400/300`; every other screen still runs on the
lime accent, so an operator crosses from a violet arch into a lime application
one navigation later. The full-app handoff (`design_handoff_syndra/`) closes
that, and brings two things the app has never had.

**The accent moves from lime to violet, and lime comes back with a job.** In the
handoff lime is `Healthy` — "nothing needed here", the provider answered, the
queue is empty. That is a role the app has been unable to express: today a calm
health cell is simply the absence of colour. The two are read against each
other, and the pairing is what makes either legible — violet is what you can act
on, lime is what needs nothing from you.

This is why the change is not a token swap. Violet **never means "good" or
"safe"**, and lime is **never a button and never a fill behind text**. A
mechanical find-and-replace would paint the "Nothing needs you." dot violet,
which inverts its meaning: the whole statement of that row is that there is
nothing to act on.

**Motion becomes a vocabulary instead of a habit.** The app had
`transition-colors duration-150` in 47 places, two ad-hoc keyframes, and one
spinner. The handoff specifies six roles — `tint`, `press`, `settle`, `arrive`,
`flash`, `breathe` — governed by three rules: direction carries meaning, only
one thing loops, and never on a number. Six named roles are not decoration; they
are the reason a reviewer can see that a row lifting on hover is wrong without
opening a browser.

**The rail mark is still a lime "m" square** — a MkAuth relic that survived the
rename. The handoff replaces it with the contained orb, a miniature of the
login's arch-and-orb, so the rail reads as the same room the door opened onto.
The app has also never had a favicon.

## What changes

- **Accent** — the dark theme's `--accent` family points at the violet ramp
  rather than carrying lime hexes. Light was already violet and is unchanged
  apart from gaining the new tokens.
- **`--accent-dense`** — a new token, and a contrast fix rather than a
  preference. `--accent-ink` on `--accent` is 4.18:1, which fails AA for
  anything below large text, and every filled control in this product carries a
  label at 13.5px or smaller. The rule that falls out is mechanical enough to
  test: **`--accent` fills that carry no text, `--accent-dense` fills that do.**
- **`--healthy`** — one token, deliberately with no `-soft`, `-ink` or `-line`
  sibling. There is no healthy button and no healthy filled field, so the token
  set gives you nothing to build one out of. Applied to the four Today health
  cells, the "Nothing needs you." line, the provider-reachable panel, and the
  five empty states where emptiness means *resolved* rather than *absent*.
- **Motion** — six duration/easing pairs and the utilities that spend them.
  Every `transition-colors` in the app becomes `motion-tint`; buttons gain
  `press` and the 3% scale-down; dialogs and the source popover gain `settle`;
  every list gains `arrive` at the one place they all already pass through; the
  rail's polled counts gain `flash`; degraded and skeletons gain `breathe`, and
  the button spinner is retired into it.
- **The mark** — `SyndraMark`, three spans over `.syndra-mark` in globals.css,
  plus `app/icon.svg`.
- **Repo media** — the handoff's banner in the README, the social preview
  staged, and the repo description set.

## Impact

- Affected specs: `operational-readiness`
- Affected code: `ui/src/app/globals.css`, `ui/src/app/icon.svg`,
  `ui/src/components/shell/SyndraMark.tsx`, `ui/src/lib/useFlashOnChange.ts`,
  `ui/src/components/states/index.tsx`, `ui/src/components/ui/{Button,Badge,Card,Select,Modal}.tsx`,
  `ui/src/components/shell/Sidebar.tsx`, `ui/src/components/today/{Today,Makerspace}.tsx`,
  `ui/src/components/states/DegradedBanner.tsx`, `ui/src/app/zitadel/page.tsx`,
  and the 33 files whose `transition-colors` became `motion-tint`
- `README.md`, `docs/assets/banner.png`, `docs/assets/social-preview.png`
- No backend change. No API change. No route change.
