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
- `dtsPropsFor` inlines app-internal types (SessionUser, BulkPlan, ActionOutcome, ReadState, WithheldItem) that the emitted .d.ts would otherwise reference by a dangling name. Keep them in sync if those types change.
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
- Generic components (Segmented, FilterPills, Tabs) get `dtsPropsFor` with `T` -> `string`; the extractor emits a dangling `T` otherwise.

## Known render warns
- Cells for one-line components (ReadFreshness, ConfirmByTyping, Skeleton, Mono…) are mostly empty space at the default card height - not a defect.
- Skeleton bars are faint on a light-theme Card - token design.
- Term's popover needs hover/click (portalled); cards show the dotted-underline trigger only.
- Withheld member copy says "this one part" for several items - component wording.
- RehearsalDialog's scope step (needs a thrown `ApiError`) and result step (needs a click) are not previewed.

## Findings for the app (not changed by the sync)
- DegradedBanner: the "For the person who runs the Syndra server" type-label reads low-contrast on the amber surface.

## Re-sync
Run from `ui/`: re-stage scripts (`cp -r` from the skill), `(cd .ds-sync && npm i esbuild ts-morph @types/react && PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1 npm i playwright)`, `ln -sfn ../.ds-sync/node_modules .design-sync/node_modules`, run `cfg.buildCmd`, fetch `_ds_sync.json` to `.design-sync/.cache/remote-sync.json`, then
`DS_CHROMIUM_PATH=... node .ds-sync/resync.mjs --config .design-sync/config.json --node-modules ./node_modules --entry ./.design-sync/entry.tsx --out ./ds-bundle --remote .design-sync/.cache/remote-sync.json`.
- Auth: the first session's login could read but not create/write ("subscription required" / "cannot edit this project") until `/design-login` was re-run against the account that owns the project.

## Re-sync risks
- `entry.tsx` enumerates the synced files and `componentSrcMap` enumerates their exports: a new component in `src/components/{ui,shell,names,states}` is NOT picked up until both are extended.
- `dtsPropsFor` hand-inlines app types (SessionUser, BulkPlan, ActionOutcome, ReadState, WithheldItem, the Segmented/FilterPills/Tabs props). If those types change, the uploaded contracts go stale silently - diff them against `src/` on every re-sync.
- `SyndraRoot` duplicates the composition in `src/components/providers.tsx` (minus devtools). If Providers gains a provider, mirror it in entry.tsx.
- `host.css` restates next/font variables and the body surface from `src/app/layout.tsx` / `globals.css`; the fonts in `.design-sync/fonts/` were fetched from Google Fonts (latin subset) once - re-fetch if layout.tsx changes families or weights.
- `overrides/dts.mjs` is a fork of the skill's `lib/dts.mjs` (types root + `@/` paths): diff against the bundled lib on every re-sync and merge upstream changes.
- Seeded fixtures in names/DegradedBanner previews mirror `CatalogResponse` / `SystemMode`; shape changes there break those cards (they fall back to skeletons, not errors).
- Not previewed: RehearsalDialog scope + result steps, Term's open popover.
