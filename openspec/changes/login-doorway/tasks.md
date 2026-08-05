# Tasks — login doorway

## Track 1 — Tokens

- [x] LD-01 Brand violet ramp (`--violet-500/400/300`, `--on-violet`) declared once in `globals.css`,
      theme-independent, and spent only by the doorway. **Not adopted app-wide:** both theme blocks
      still carry their own `--accent` hex — dark lime, light violet — and this change does not touch
      them. The ramp exists so that unifying them later is a repoint in two theme blocks rather than
      a sweep through every component. Deferred work recorded in `NEXT.md`.
- [x] LD-02 `--door-*` token set — always-dark page and ink, arch stroke and mask, orb rings, amber,
      both button fills and both three-layer shadows, and the geometry (`arch-w/h/r`, `wordmark`,
      the two insets). Nothing about the doorway is a literal in a `.tsx` file.
- [x] LD-03 Narrow breakpoint (≤480px) scales the arch on its own 430:392 ratio with the radius
      scaled to match, and drops the wordmark to 40px — by overriding four custom properties.

## Track 2 — The screen

- [x] LD-04 `.login-*` component styles: stage, floor, pool, amber pool, grain, eyebrow, arch,
      wash, orb (four layers), wordmark, action, base text, message slot, identity pills.
- [x] LD-05 `line-height: normal` on the stage. The app body sets 1.55 for dense operator rows;
      nothing here is a row, and inheriting it made the button 6px taller than the design and moved
      the wordmark off its mark.
- [x] LD-06 `--door-arch-inset` replaces a percentage padding. Padding percentages resolve against
      the *containing block*, not the element — `padding: 0 10%` on the group measured 10% of the
      stage and crushed the button's label onto two lines.
- [x] LD-07 `.login-base` and `.login-message` carry their own gutter: `left/right: 0` escapes the
      stage's padding, so on a phone the Syn line would touch both edges.

## Track 3 — Choreography

- [x] LD-08 `ceremony.ts` — `openDoor(root)` returns `play/lock/unlock/dispose`. Every animation
      kept in `running[]` and cancelled by reference; authored values written back explicitly.
- [x] LD-09 Arch mask and border colour driven by `[data-scene]` in CSS, never by an animation.
- [x] LD-10 Entrance (~1.9s), sign-in (~1.2s) and refusal, at the handoff's exact durations, delays
      and easings. Arch clip animates in percent so the breakpoint needs no second keyframe set.
- [x] LD-11 `ringLight(dx, dy)` returns an angle and a falloff, not a gradient. The gradient — its
      stops and its colour — is authored on `.login-orb-lit` and reads two custom properties, so
      there is one mask rather than one in CSS and a second assembled in a module. Includes the
      `+180` that puts a gradient's opaque first stop toward the cursor rather than away from it.
- [x] LD-12 Keyboard parity: focus locks the ring lit from below, where the button is; pointer
      moves are ignored while locked. Releasing removes the two properties and falls back to the
      authored top-lit mask — the reference left the ring pointing at a button that no longer had
      focus, and clearing is only safe *because* the gradient lives in the stylesheet.
- [x] LD-13 Reduced motion skips the entrance and the ring, and collapses the two meaningful scenes
      to instant state changes.

## Track 4 — Behaviour

- [x] LD-14 `LoginDoor.tsx` — three scenes, `data-scene` on the stage, label swap 200ms behind the
      press so the button's own dip reads first.
- [x] LD-15 OIDC mode: `<a href="/auth/zitadel">`, navigation not prevented, modified clicks left alone.
- [x] LD-16 Demo mode: the same door opens onto the five development identities, focus moved to the
      first. Each is a real `<form action="/auth/login" method="post">`, unchanged from before.
- [x] LD-17 `loginFailure(code)` — three messages behind one failure picture; the code is never rendered.
- [x] LD-18 Retry returns to `idle`, replays the entrance, and drops `?error=` from the URL.

## Track 5 — Ambience

- [x] LD-25 Breathing pool (`doorBreath`, 10s, alternate) and animated grain (`doorGrain`, 6 offsets
      on `steps(1, end)`, 8fps) turned on, as CSS `@keyframes` rather than WAAPI — they never need
      cancelling and they animate the two properties the choreography never touches.
- [x] LD-26 Both live inside `@media (prefers-reduced-motion: no-preference)`, so a visitor who asked
      for less motion is never started on them rather than being rescued by the global reduce rule.
      Guarded by `design-system.test.ts`, which brace-matches the block and fails if either rule is
      declared twice or lifted out of it.
- [x] LD-27 `ceremony.test.ts` asserts no scene animates the pool's `transform` and no scene touches
      the grain — the collision that would silently freeze the breath.
- [x] LD-28 Measured live: `doorBreath`/`doorGrain` both running, the grain landing on all six
      authored offsets (not interpolating between them), the pool keeping its `-50%` translate, and
      120 frames at a median of 8.3ms with a worst frame of 9.3ms.

## Track 6 — The stranger's first request

- [x] LD-29 `NameResolverProvider` takes an `enabled` gate and `Providers` a `hasSession`, supplied
      from the session the root layout already resolves. The resolver mounts for every route
      including the unauthenticated one, and was firing `GET /api/proxy/{catalog,bundles}` at a
      stranger — four round trips and four console errors, on the one screen somebody sees before
      they decide whether to trust this software. Both default to `true`: a caller who forgets the
      prop gets working name resolution, not silently blank names.
- [x] LD-30 The gate covers all three requests, not two. Gating only the catalog and bundle warm-ups
      left `POST /lookup` escaping: a descendant calling `resolveUser` on an id the catalog does not
      carry queues a miss and fires it. `useMissResolver` now takes the gate and refuses twice —
      `note()` keeps ids out of the queue, and the query keeps a queue filled *before* the gate closed
      (a session expiring under a mounted tree) from draining after it.
- [x] LD-31 Verified live both ways — `/login` issues **zero** proxy requests and logs nothing, and
      signing in as a development identity still fires `catalog` and `bundles` on the next page.
- [x] LD-32 The "no request" test carries a **positive control**. The miss lookup is queued from a
      microtask and fired a render later, so asserting silence straight after render only races the
      queue — which is exactly how the first version of this test passed while the leak was live. The
      control asserts the same wait catches all three requests when enabled; if it ever stops firing,
      the silence beside it stops meaning anything. Mutation-checked: removing both guards reproduces
      the leak and fails the test, and either guard alone holds.

## Track 7 — The name

- [x] LD-33 Syn line rewritten. The old second half — "and the defence called upon at trial" — is
      Norse legal procedure, which gives Syn a biography and breaks the brand board's own voice rule:
      **a doorkeeper, not a character**. The gloss stays because "keeps the door" is literally the
      screen you are looking at; what follows it is now Syndra's job. Reads:
      *"Syn — the goddess who keeps the door. Syndra keeps the list, and the reason for every name
      on it."* True to the model — a grant always carries its source (direct, via bundle, automatic).
- [x] LD-34 `text-wrap: balance` rather than `pretty` on that line. `pretty` only rescues the last
      line from being an orphan; centred over two lines it still leaves one long and one short.
      `balance` evens them, and the break falls on the sentence caesura after "door."
- [x] LD-35 `Syn keeps the door. Syndra keeps the list.` — the branding turn's own title card —
      placed twice and no more. A line used everywhere stops being a line and becomes a slogan, and
      this system's voice is plain. It goes in the README as an epigraph above the functional
      tagline, and on **System › Identity provider**, which is the one screen where the split is
      operationally literal: Zitadel is the door now, Syndra is the list. Folded into that page
      rather than given a nav entry — navigation structure is a contract, not a place for prose.
- [ ] LD-36 **Operator-gated.** GitHub repo description still reads the old blurb. `gh` is not
      installed on this machine; run:
      `gh repo edit notkanishk/syndra --description "Syn keeps the door. Syndra keeps the list. An identity and access orchestration layer for Zitadel."`

## Track 8 — Spec

- [x] LD-23 `specs/operational-readiness/spec.md` delta — five requirements with scenarios: one
      action on the unauthenticated surface, state told through the composition rather than a banner,
      error semantics (never echo the provider's code, never report a refusal as silence), motion as
      cover rather than a gate, and the dev picker gated both on the provider and on being asked for.
      Without it, the doorway's observable contract would be lost when this change is archived.

## Track 9 — Verification

- [x] LD-19 `login-error.test.ts` — the three classes, the unclassified fallback, "nothing was
      signed in" on every variant, and that an injected code never reaches the copy.
- [x] LD-20 `ceremony.test.ts` — the ring's four cardinal angles and its 420px falloff; that a
      scene cancels the previous scene's animations *by reference*; that the authored values come
      back after a scene that moved them; that `dispose()` stops listening; that the focus lock holds.
- [x] LD-21 `LoginDoor.test.tsx` — one action and it is a real link; no message the page is not
      showing; the click reaches the browser; a ⌘-click leaves the page alone; the refused state
      arrives server-rendered; retry clears the URL; the demo identities are unreachable until the
      door opens.
- [x] LD-22 Verified in Chrome against `login-reference.html` at 1240×800 — resting, refused,
      opening and the demo reveal — plus 390×844 and a light-theme visitor. The whole loop was
      walked live: click → `/auth/zitadel` → `?error=misconfigured` → refused → retry → idle.
- [x] LD-24 Ring mask re-verified live after moving the gradient into CSS: the computed
      `mask-image` tracks the pointer (180° → 360° → 90°, falloff 76% → 82% → 81%), focus pins it to
      0°/80%, and blur restores the authored value byte-for-byte.

## Deliberately not done

- **App-wide violet** — the ramp is in place and unspent outside the doorway. Repainting the dark
  room is its own change with its own visual pass.
