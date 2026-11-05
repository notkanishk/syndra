# design-sync notes (ui/)

Package shape, run from `ui/`. Scope: `src/components/{ui,shell,names,states}` (primitives + shell), per owner. Screens are out of scope.

## How the build is wired
- No dist, no Storybook. `.design-sync/entry.tsx` is the bundle entry (`--entry ./.design-sync/entry.tsx`): re-exports of the in-scope files plus `SyndraRoot` (the providers.tsx stack minus devtools, plus Next's `PathnameContext`). `Sidebar` is a default export - re-exported by name.
- `.design-sync/process-shim.ts` is the entry's first import: Next's client modules (`next/link`) read `process.env.__NEXT_*`; without it every card throws `process is not defined`.
- The converter compiles `process.env.NODE_ENV` as `"development"`, so the real `Providers` would render React Query Devtools in every card - that is why `SyndraRoot` composes the providers itself.
- CSS: `buildCmd` compiles `src/app/globals.css` unchanged through the app's own `@tailwindcss/postcss` (`.design-sync/build-css.mjs` -> `.cache/syndra.css` = `cfg.cssEntry`). Owner rule: globals.css ships as-is, incl. the `--door-*` block; no hand-copied token file, no alias tokens. Utilities exist only if the app's sources use them (Tailwind scan).
- `.design-sync/host.css` (appended by build-css.mjs) = what the Next host supplies: next/font's `--font-*` vars, and the app's body surface restated unlayered (the preview harness sets `body{background:#fff}`, which otherwise beats `@layer base` and breaks dark theme).
- Fonts: next/font Google families self-hosted in `.design-sync/fonts/` (latin, variable woff2, OFL) via `cfg.extraFonts`.
- Types: `buildCmd` also runs `tsc -p .design-sync/tsconfig.types.json` -> `.cache/types`; `.design-sync/overrides/dts.mjs` (declared in `libOverrides`) reads those and maps the `@/` alias. Needs `ln -sfn ../.ds-sync/node_modules .design-sync/node_modules` per clone.
- `dtsPropsFor` inlines app-internal types (SessionUser, BulkPlan, ActionOutcome, ReadState, WithheldItem, CancelStep, NavSection, Sort, GlideIndicator's Box) that the emitted .d.ts would otherwise reference by a dangling name. Keep them in sync if those types change.
- Render check / capture: no playwright browser cache; use system Chrome: `DS_CHROMIUM_PATH="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"`.

## Authoring previews
- Import from `"ui"` (maps to `window.Syndra`). Every cell is wrapped in `SyndraRoot` (pathname "/") via `cfg.provider`.
- Layout glue uses inline `style` with `var(--token)` only - never a literal hex, never Tailwind classes the app doesn't already use. Tokens: `--ground --canvas --surface-0/1/2 --rail --line --line-strong --ink --muted --faint --label --accent* --healthy --warn* --danger*`.
- Route-dependent shell parts: nest `<SyndraRoot pathname="/users">`. Operator view: `UiViewProvider isOperator` + a helper that calls `useUiView().setView(...)` in an effect - set the view in EVERY cell, the provider persists it to localStorage and leaks across cells otherwise.
- Shell is `hidden` below the tablet breakpoint (720px): wide parts need `overrides.<Name>: {cardMode: "column", viewport: "1200x1000"}`.
- Data hooks (indicators, targets, name catalog) fetch relative `/api` URLs that 404 in previews. Seed them (next bullet) rather than relying on fallbacks.
- Content: makerspace realism (laser cutter, CNC router, wood shop; Indian student names). Copy follows `openspec/changes/plain-language-copy/design.md`.
- Data-backed parts: pass `data` to a nested `<SyndraRoot data={[{ key: ["name-catalog"], value: catalog }]}>` (fresh client per seeded root). Keys: `["name-catalog"]`, `["name-bundles"]`, `["system","mode"]`, `["governance","indicators"]`, `["targets"]`. Without seeding, UserName/ProjectName/RoleRef stay on skeletons for real ids (the miss-resolver never settles) and DegradedBanner renders nothing.
- `ApiError` is exported from "ui" for request-id states (ErrorState, RehearsalDialog scope step).

## Capture quirks (from preview waves)
- Overlay / full-height cards: the `?story=` wrapper `.ds-single` has `transform`, so it is the containing block for `position:fixed` and has no height - a `fixed inset-0` Modal collapses to 0px. Every dialog, AppShell and TouchNav story renders inside a stage `{ margin: -24, height: "100dvh" }` (cancels the harness's 24px padding). Fixed-px frames inside the padding clip.
- Seeding detail: an id absent from the seeded catalog fires `POST /lookup`, 404s, then settles to "Unknown account" + mono id - that is the honest-miss cell. DegradedBanner: `degraded:true` (+`reason`) vs `seed_residue>0 && directory:"zitadel"` (operator copy varies with `seed_active`).
- Capture clock is frozen months earlier than today: build every time relative to `Date.now()` (Relative, ReadFreshness, LogTime) - never a fixed ISO date.
- Capture renders in the host timezone (IST): pick UTC times that land on plausible local hours.
- Click-to-open state (FilterPanel, AccountSheet): click the trigger from a ref in `useEffect`. A FilterPanel wrapper needs `alignItems: "flex-start"` or the absolute panel lands clipped at the cell bottom.
- OfflineBanner: dispatch `window` `offline` in a cell effect.
- ClipboardUnavailableNote is the ONE preview that deletes `navigator.clipboard` at module scope (its card is its own page). Never do that elsewhere.
- Modal is `fixed inset-0`, centred on the viewport: dialog cards need `cardMode: "single"` and a viewport >= 720px wide (below it every dialog becomes a bottom sheet). Modal's focus trap rings the first footer button - real behaviour.
- RehearsalDialog without `compose` auto-rehearses: `onRehearse={async () => plan}` lands on review; a never-resolving `onApply` holds it there.
- `SelectAllRow` needs `ref={() => {}}` and `checked`. `ViewSwitch` stretches in a block cell - wrap it in `display:flex`. `Badge` has no `healthy` tone.
- AppShell owns its own `UiViewProvider`: pin a view with the setView helper inside its children.
- UNC paths in JSX string attributes: single backslashes in source.
- CommandBlock `tone="onWarn"` needs a solid `var(--warn)` surface behind it.
- Generic components (Segmented, FilterPills, Tabs, SortHead, ReadLine, SectionPill) get `dtsPropsFor` with `T` -> `string`; the extractor emits a dangling `T` otherwise.

## Known render warns
- Cells for one-line components (ReadFreshness, ConfirmByTyping, Skeleton, Mono…) are mostly empty space at the default card height - not a defect.
- Skeleton bars are faint on a light-theme Card - token design.
- Term's popover needs hover/click (portalled); cards show the dotted-underline trigger only.
- Withheld member copy says "this one part" for several items - component wording.
- RehearsalDialog's scope step (needs a thrown `ApiError`) and result step (needs a click) are not previewed.
- `[RENDER_BLANK]` on Modal, ModalHeader, ModalFooter, RehearsalDialog and PushedStep: the fixed overlay collapses the measured PNG; the capture sheets show them drawn. `[GRID_OVERFLOW]` on FilterPanel/FilterField (the open panel escapes its cell). `[RENDER_THIN]` on CopiedSeat (floor card).
- PushedStep's preview refuses one write with `WouldCancelQueued` on mount (exported from "ui" for this); it needs the seeded name catalog.

## Findings for the app (not changed by the sync)
- DegradedBanner: the "For the person who runs the Syndra server" type-label reads low-contrast on the amber surface.

## Re-sync
Run from `ui/`. `.ds-sync/` already holds the staged scripts and `esbuild ts-morph @types/react playwright` - reuse them; only on a fresh clone re-stage (`cp -r` from the skill), `(cd .ds-sync && npm i esbuild ts-morph @types/react && PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1 npm i playwright)` and `ln -sfn ../.ds-sync/node_modules .design-sync/node_modules`. Then run `cfg.buildCmd`, put the last published `_ds_sync.json` at `.design-sync/.cache/remote-sync.json`, and
`DS_CHROMIUM_PATH="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" node .ds-sync/resync.mjs --config .design-sync/config.json --node-modules ./node_modules --entry ./.design-sync/entry.tsx --out ./ds-bundle --remote .design-sync/.cache/remote-sync.json`.
- Where it goes: `ds_merge.py` converts the bundle and Claude Code publishes it to the Syndra Design System artifact, then copies it into the canvas at `project/ds/syndra/` (BRIDGE steps 2 and 3). Nothing here talks to claude.ai/design: the resync only builds locally, and `config.json`'s `projectId` (the retired Claude Design project) is an identity field no script reads over the network.
- Remote file: nothing fetches it any more. Before the build wipes `ds-bundle/`, copy the previous bundle's own sidecar: `cp ds-bundle/_ds_sync.json .design-sync/.cache/remote-sync.json` (only while `ds-bundle/` still holds what was last published - every local run overwrites it, so after a publish refresh the cache copy from the published bundle; the diff then anchors on it, `anchor: ok`). If no previous bundle exists, drop `--remote`: everything is graded as new and uploads.
- 3 Nov 2026: 76 -> 104 components (Notice, NotOpenYet, NextStep, StatusSeat, WaitsMark, ChoiceRow, Checkbox, Radio, Seat, ArrowLink, SortHead, RunLabel, ReadLine, SectionPill, Unfold, Tinted, UnfoldOnArrival, PaginationRow, Unresolved, PushedStep, AnchoredPanel, ArriveList, CopiedSeat, Crossfade, FormPart, GlideIndicator, LeavingRow, ReadGate). Motion/hook helpers ship on the floor card, unpreviewed: ArriveList, CopiedSeat, Crossfade, FormPart, GlideIndicator, LeavingRow, Tinted, UnfoldOnArrival, ReadGate, Seat, RunLabel, SectionPill (shows only once the read line has scrolled away).

## Re-sync risks
- `entry.tsx` enumerates the synced files and `componentSrcMap` enumerates their exports: a new component in `src/components/{ui,shell,names,states}` is NOT picked up until both are extended.
- `dtsPropsFor` hand-inlines app types (SessionUser, BulkPlan, ActionOutcome, ReadState, WithheldItem, CancelStep, NavSection, the Segmented/FilterPills/Tabs/SortHead/ReadLine/SectionPill props). If those types change, the uploaded contracts go stale silently - diff them against `src/` on every re-sync.
- `SyndraRoot` duplicates the composition in `src/components/providers.tsx` (minus devtools). If Providers gains a provider, mirror it in entry.tsx.
- `host.css` restates next/font variables and the body surface from `src/app/layout.tsx` / `globals.css`; the fonts in `.design-sync/fonts/` were fetched from Google Fonts (latin subset) once - re-fetch if layout.tsx changes families or weights.
- `overrides/dts.mjs` is a fork of the skill's `lib/dts.mjs` (types root + `@/` paths): diff against the bundled lib on every re-sync and merge upstream changes.
- Seeded fixtures in names/DegradedBanner previews mirror `CatalogResponse` / `SystemMode`; shape changes there break those cards (they fall back to skeletons, not errors).
- Not previewed: RehearsalDialog scope + result steps, Term's open popover.

- 5 Nov 2026 (A-19 P4): + GoTo, GoToPanel, Kbd. GoTo previews seed `["name-catalog"]` and `["name-bundles"]` through `SyndraRoot data`, pin the view with the `View` helper, and type into the combobox from an effect (the native value setter plus an `input` event) so the grouped results draw; GoToPanel has its own preview (in the sheet, and typed). `nav` is inlined in `dtsPropsFor` (NavEntry). Not yet re-synced or published.

## Fonts on the design canvas (3 Nov 2026)
The canvas renders each artboard as a srcdoc document whose base is `/appifact/<name>.dc.html`, so a linked stylesheet's relative `./*.woff2` resolves to a 404. `fonts/fonts.css` therefore lists the canvas's uploaded font assets first (`url(/_blob/<id>)`), then the relative file, which is what local previews use. The three ids are the canvas's uploads of the same woff2 files; if a font file changes, upload it to the canvas again and replace its id here. `host.css` gives each `--font-*` a system fallback stack, so a font that fails never falls back to Times.

## The container-query copy (3 Nov 2026)
The canvas shows a board's frames inline (`<dc-import … frame=…>` inside a `syndra-frame` container), where `@media` would see the artboard's width. After each resync, run `node .design-sync/build-container-css.mjs` from `ui/`: it writes `ds-bundle/_ds_bundle.container.css`, the same stylesheet with every width `@media` turned into `@container syndra-frame (…)`. Boards link it instead of `_ds_bundle.css`. Publish both.

## Moves and Phone on every card (5 Nov 2026, A-19 P6)
- Every card carries two generated lines: **Moves** (which verb, and its still form) and **Phone** (what changes below tablet). They come from one table, `.design-sync/moves.json`; a card not in it takes the defaults. Run `node .design-sync/moves.mjs` after editing it (or after adding a component to `componentSrcMap`): it writes a marked block at the top of each preview file and the whole table to `.design-sync/MOVES.md`, which the resync reads for the floor cards that have no preview. `--check` exits 1 when either is stale; `src/lib/motion/__tests__/motion.test.tsx` fails the same way.
- The motion code the cards describe lives in `src/lib/motion/` now (`ArriveList`, `Unfold`, `Tinted`, `UnfoldOnArrival`, `LeavingRow`, `Crossfade`, `GlideIndicator`, `useFlashOnChange`, `glideTo`, `stillMotion`), not in `src/components/ui/`. `config.json` and `entry.tsx` point there; the exports are unchanged. `bloom` and `lift` are CSS classes in `globals.css`, so their lines sit on the cards that use them (StatusDot, ActionOutcome, EmptyState).
- The motion tokens (`--ease-*`, `--duration-*`, `--travel-cap`, `--arrive-*`) were already in `globals.css`, which ships as it is, so the bundle's `_ds_bundle.css` carries them. The design-system artifact's own `tokens.css` is what lacks them; that is the publish step, not this repo.
- + KeysSheet (the Keys sheet, `?`). Not yet re-synced or published.
- 5 Nov 2026: AdvancedLink added (entry, config, preview, moves row; `ArrowLink` and `NextStep` render it). In the design system there is no view provider, so it is drawn as the plain link it is there; re-sync adds the card.

## P9, the design system tells the truth (5 Nov 2026)
- `moves.json` is now read from the code, not from the A-19 guess; `src/lib/motion/__tests__` still fails when previews or MOVES.md go stale. Chip and Badge do not move (only ChosenChip). ArriveList rises 8px.
- `SyndraRoot` provides `AppRouterContext` with a router that goes nowhere: AppShell mounts `IntentPrefetchHost`, whose `useRouter()` throws without it (that was every "This page could not render" on AppShell).
- TouchNav is `cardMode: single` at 390px (it is `tablet:hidden`); the render check still reports `[RENDER_BLANK]` because its own page is 900px wide. Benign.
- RoleWordsTable and SignInRow are `cardMode: column` (a table needs the width). SectionPill needs `#app-scroll` and a scrolled container, so its story scrolls its own box.
- `--font-sans` stays (body uses it; published boards read it). `--radius-shell` is gone.
