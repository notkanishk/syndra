# Wave 2 · Part 1 — Frontend Palette Finalization

**Status:** Proposed
**Source:** [May 2026 audit resolution design](../../../docs/superpowers/specs/2026-05-08-may-2026-audit-resolution-design.md) §3 Theme 4 (palette portion) — audit refs **U2 + D10**
**Phase:** 5.5
**Wave:** 2 (Part 1 of 4 — palette before drift UI; Parts 2–4 cover Theme 3 backend coherence, Theme 5 operational polish, and Theme 2 drift control)

## Why

The May 2026 audit found that the obsidian-clarity-redesign migration left a permanent backdoor: a "Legacy compatibility tokens" block in `ui/src/app/globals.css` that still maps `bg-surface-hover`, `border-border`, `text-foreground`, `text-muted`, `bg-primary-hover`, and `bg-danger` to MD3 equivalents so unmigrated surfaces keep rendering. Five named files (`Sidebar.tsx`, `ThemeToggle.tsx`, `RequestAccessButton.tsx`, `ErrorBoundary.tsx`, the legacy CRUD parts of `app/zitadel/page.tsx`) and a long tail of secondary call sites (`app/page.tsx`, `SidebarNav.tsx`, `ui/{CopyButton,Skeleton,SubmitButton,JsonView}.tsx`) still resolve through that block. The aliases let drift accumulate silently — every new component built atop them invisibly extends the migration debt.

This blocks Wave 2 Part 4 (drift UI) — the design doc requires the new drift surfaces to mount on Material tokens from day one rather than be built on legacy aliases and rewritten later.

## What changes

- **Sidebar shell + nav:** `Sidebar.tsx`, `SidebarNav.tsx`, `ThemeToggle.tsx` migrate to MD3 tokens (`bg-surface`, `bg-surface-container-high`, `text-on-surface-variant`, `border-outline-variant`, `bg-primary-container/text-on-primary-container` for active nav, `<Pulse variant="success" static />` for the LXC live dot).
- **Member surfaces:** `app/page.tsx` (member home Welcome panel + service catalog cards) and `RequestAccessButton.tsx` (inline access-request modal) migrate to MD3 tokens; the modal additionally wraps `<Modal>` (per OCR-S1-16) so it inherits the focus-trap + Esc + click-outside contract instead of re-implementing them.
- **New foreground tokens:** `globals.css` gains `--on-success`, `--on-warning`, and `--on-info` (light + dark theme values + `@theme` mappings). Light values land at `#ffffff`; dark values are deeper same-family colors (`#003822`, `#2a1800`, `#001a4a`) that meet WCAG AA at 4.5:1 against the corresponding light bg-* tokens. This closes a real accessibility bug: the pre-migration `text-white` on dark-mode `bg-success` / `bg-warning` / `bg-info` (`#34d399` / `#fbbf24` / `#60a5fa`) measures ~1.4–2.0 contrast ratio.
- **UI primitives:** `ErrorBoundary.tsx`, `ui/CopyButton.tsx`, `ui/Skeleton.tsx`, `ui/SubmitButton.tsx`, `ui/Button.tsx`, `ui/JsonView.tsx` migrate. `SubmitButton` additionally fixes the `bg-primaryHover` typo (Tailwind silently no-ops camelCase classes, leaving primary buttons with no hover background) and routes `danger`/`success` variants through `error`/`success` tokens; the success variant uses the new `text-on-success` foreground. `Button.tsx` success and warning variants drop the `var(--success)` indirection in favor of `bg-success text-on-success` / `bg-warning text-on-warning`, also fixing the AA contrast bug. `JsonView` syntax-highlight tones migrate to semantic status tokens — `text-amber-500` (null + diff) → `text-warning`, `text-emerald-500` (strings) → `text-success`, `text-sky-500` (numbers/booleans) → `text-info` — preserving the visual encoding while routing through the design system.
- **Zitadel diagnostics legacy CRUD:** Rotation-status, Health, Projects, Users, AllGrants sections inside `app/zitadel/page.tsx` (everything below the Stage-3 `<LiveStatusTile>`) migrate to MD3 tokens. The sweep extends beyond legacy palette aliases to cover the six `text-red-400` inline-error and flash-state sites (lines 300, 376, 377, 597, 855, 912), the two `text-emerald-400` flash-success sites (lines 597, 855), and the six `bg-primary text-white` CTA buttons (lines 365, 538, 588, 795, 846, 905) — all rerouted to `text-error`, `text-success`, and `text-on-primary` respectively. Page split (U5) is explicitly out of scope and stays for Wave 3.
- **Token block removal:** The "Legacy compatibility tokens" block in `globals.css` is deleted (`--color-foreground`, `--color-surface-hover`, `--color-border`, `--color-primary-hover`, `--color-muted`, `--color-danger`, `--color-danger-hover`). Status tokens (`--color-success`, `--color-warning`, `--color-info` and their `-hover` pairs) stay — they have no MD3 baseline and are semantic, not aliases.
- **Hardcoded core-Tailwind colors** (`bg-emerald-500` LXC dot in `Sidebar`, `border-red-500/40 bg-red-500/5` ErrorBoundary alert, `bg-red-500`/`bg-emerald-500` SubmitButton variants, `text-red-400`/`border-red-500/40`/`hover:bg-red-500/10` zitadel CRUD destructive controls, `JsonView` syntax tones, `graph/page.tsx` `nodeTone()` mappings) route through error/success/warning/info tokens. The `graph/page.tsx` `nodeTone()` function migrates the application/bundle/role branches (`border-sky-500/30 bg-sky-500/8 text-sky-600 dark:text-sky-300` → `border-info/30 bg-info/8 text-info`; amber → warning; emerald → success); the project branch already uses `primary-container` MD3 tokens and is unchanged. The dark-mode `dark:text-*-300` overrides are dropped because the design tokens already auto-flip via `@theme`.
- **Spec + index updates:** `specs/operational-readiness/spec.md` adds a "no legacy palette aliases" requirement; `obsidian-clarity-redesign/tasks.md` gains a Stage 5 pointer line that hands palette finalization to this change; `INDEX.md` adds a new change-log row.
- **Regression test:** A new `ui/src/__tests__/no-legacy-tokens.test.ts` reads `globals.css` and asserts the legacy block is absent, then greps the `src/` tree and asserts no `bg-surfaceHover`, `bg-surface-hover`, `border-border`, `text-foreground`, `text-muted`, `bg-primary-hover`, `bg-primaryHover`, `bg-danger`, or `text-danger` utility class survives. Lands with the migration; runs on every CI build thereafter.

## Out of scope

- Theme 2 (Wave 2 Part 4) drift control architecture, drift UI, outbox propagation flow.
- Theme 3 (Wave 2 Part 2) backend coherence refactors.
- Theme 5 (Wave 2 Part 3) operational polish (sync env, `mapping_rules.version` drop, scripts dedup).
- U5 — the 947-line `app/zitadel/page.tsx` split into `components/zitadel/{Health,Rotation,Projects,Users,AllGrants}.tsx` (Wave 3, Theme 4 remainder). Palette migration here happens in-place.
- U1 — `useNameResolver` rewrite (Wave 3).
- U3 — `useGovernanceSummary` dedup (Wave 3).
- U4 — dead exports in `lib/api.ts` (Wave 3).
- U6 — avatar fallback ordering (Wave 3).
- U7 — `middleware.ts` + proxy route admin/member tests (Wave 3).
- D2 / D6 — ⌘K strike, `UserProfile.location` removal (Wave 3).
- Status semantic tokens (`success`/`warning`/`info` + `-hover` pairs). They are semantic, not legacy aliases.
