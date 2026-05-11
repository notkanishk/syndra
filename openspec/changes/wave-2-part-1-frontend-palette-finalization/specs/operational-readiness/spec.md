> **Status:** Wave 2 · Part 1 delta — Frontend palette finalization | [< Index](../../../../INDEX.md)

# Requirement: Frontend Palette Coherence (delta)

## ADDED Requirements

### Requirement: Frontend MUST resolve chromatic surfaces through the canonical token contract

The frontend MUST resolve every chromatic color, surface, stroke, text, and status utility through the canonical token contract defined in `ui/src/app/globals.css` — the Material 3 baseline (`--color-surface`, `--color-on-surface`, `--color-on-surface-variant`, `--color-surface-container-{lowest,low,medium,high,highest}`, `--color-primary*`, `--color-secondary*`, `--color-tertiary*`, `--color-outline*`, `--color-error*`) plus the semantic status pairs (`--color-success*`, `--color-warning*`, `--color-info*`) and their `on-*` foreground pairs (`--color-on-success`, `--color-on-warning`, `--color-on-info`). The "Legacy compatibility tokens" block introduced in OCR-S1-05 to bridge unmigrated surfaces MUST NOT exist after this change ships. Hardcoded core-Tailwind chromatic utilities (e.g. `bg-red-500`, `text-red-400`, `text-emerald-400`, `text-emerald-500`, `border-sky-500/30`, `dark:text-amber-300`) that previously survived in `app/graph/page.tsx`'s `nodeTone()`, `ui/JsonView.tsx`'s syntax-highlight tones, and `app/zitadel/page.tsx`'s error/success flash messages MUST be migrated to the semantic status tokens so the design system carries no chromatic exceptions. Pure neutral utilities (`text-white`, `text-black`, `text-transparent`) MAY persist as utility colors only when they do NOT pair with a chromatic semantic background; any colored-background-plus-foreground pairing MUST use the `on-*` token paired with that background (e.g. `bg-success text-on-success`, `bg-primary text-on-primary`, `bg-warning text-on-warning`).

#### Scenario: globals.css carries no legacy aliases
- **WHEN** an operator inspects `ui/src/app/globals.css`
- **THEN** the `@theme` block MUST NOT define `--color-foreground`, `--color-surface-hover`, `--color-border`, `--color-primary-hover`, `--color-muted`, `--color-danger`, or `--color-danger-hover`
- **AND** the comment header "Legacy compatibility tokens — kept so existing utilities like `bg-surface, border-border, text-foreground` continue to resolve while pages migrate to the new ladder" MUST be removed
- **AND** the semantic status tokens (`--color-success`, `--color-success-hover`, `--color-warning`, `--color-warning-hover`, `--color-info`, `--color-info-hover`) MUST remain because they have no MD3 baseline equivalent

#### Scenario: No legacy utility class survives in source
- **WHEN** the canary test `ui/src/__tests__/no-legacy-tokens.test.ts` walks `ui/src/` (excluding `__tests__`, `node_modules`, `.next`, build artifacts)
- **THEN** it MUST find zero occurrences of `bg-surfaceHover`, `bg-surface-hover`, `hover:bg-surfaceHover`, `hover:bg-surface-hover`, `border-border`, `text-foreground`, `hover:text-foreground`, `text-muted`, `hover:text-muted`, `bg-primary-hover`, `hover:bg-primary-hover`, `bg-primaryHover`, `hover:bg-primaryHover`, `bg-danger`, `text-danger`, or `bg-danger-hover` as substring matches in any `.ts`, `.tsx`, or `.css` file
- **AND** the test MUST run as part of `bun run test` so CI fails on regression

#### Scenario: Syntax-highlight, graph-node, and zitadel flash tones route through semantic status tokens
- **WHEN** an operator inspects `ui/src/components/ui/JsonView.tsx`, `ui/src/app/graph/page.tsx`, or `ui/src/app/zitadel/page.tsx`
- **THEN** none MUST contain `text-amber-500`, `text-emerald-500`, `text-sky-500`, `text-amber-600`, `text-emerald-600`, `text-sky-600`, `text-red-400`, `text-emerald-400`, `border-emerald-500`, `border-amber-500`, `border-sky-500`, `bg-emerald-500`, `bg-amber-500`, `bg-sky-500`, `dark:text-emerald-300`, `dark:text-amber-300`, or `dark:text-sky-300` as substring matches
- **AND** the corresponding tones MUST render via the `success` / `warning` / `info` / `error` semantic status tokens (e.g. `text-warning`, `border-success/30 bg-success/8 text-success`, `text-error`)
- **AND** the canary test MUST extend the banned-substring list to include these hardcoded core-Tailwind chromatic utilities so the no-exceptions contract survives

#### Scenario: Chromatic-background buttons MUST pair with on-* foreground tokens
- **WHEN** an operator inspects any button or button-shaped surface in `ui/src/components/ui/{Button,SubmitButton}.tsx`, `ui/src/app/zitadel/page.tsx`, or any other in-scope file that combines `bg-primary` / `bg-success` / `bg-warning` / `bg-info` / `bg-error` with a foreground text class
- **THEN** the foreground MUST be the matching `text-on-*` token (`text-on-primary` for `bg-primary`, `text-on-success` for `bg-success`, `text-on-warning` for `bg-warning`, `text-on-info` for `bg-info`, `text-on-error` for `bg-error`)
- **AND** `text-white` MUST NOT appear as the foreground of any chromatic semantic background in any file this change touches
- **AND** `globals.css` MUST declare the `--on-success`, `--on-warning`, and `--on-info` CSS variables (with light- and dark-theme values that meet WCAG AA at 4.5:1 against their paired background) plus the corresponding `--color-on-success`, `--color-on-warning`, `--color-on-info` mappings inside the `@theme` block

### Requirement: Inline modal surfaces MUST compose `<Modal>`

Inline modal surfaces (member-portal Request Access flow, admin Confirm flows) MUST compose the canonical `<Modal>` primitive from OCR-S1-16. Hand-rolled focus-trap, Esc, and click-outside dismiss implementations MUST NOT survive in component code, since they re-implement the contract that `Modal.test.tsx` already proves correct.

#### Scenario: RequestAccessButton uses Modal
- **WHEN** a member clicks "Request Access" on a service-catalog tile
- **THEN** the resulting dialog MUST be rendered through `<Modal>` (the same primitive used by ConfirmModal and the admin create flows)
- **AND** focus MUST trap within the dialog while open
- **AND** Esc MUST dismiss the dialog
- **AND** clicking outside the dialog MUST dismiss it
- **AND** these contracts MUST be covered by the existing `Modal.test.tsx` (no duplicate component-local a11y test required)

(Audit refs: U2, D10)
