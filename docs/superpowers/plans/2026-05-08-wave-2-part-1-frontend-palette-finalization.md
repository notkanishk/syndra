# Wave 2 · Part 1 — Frontend Palette Finalization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Migrate every remaining legacy-palette-alias call site in `ui/src/` to canonical Material 3 tokens, then delete the "Legacy compatibility tokens" block from `globals.css`, removing the alias backdoor permanently.

**Architecture:** Pure token-level find-and-replace driven by a published mapping table (see `openspec/changes/wave-2-part-1-frontend-palette-finalization/design.md` §2). Migration runs leaf-first (UI primitives) before consumers (pages and shells), then the alias block is deleted in one commit accompanied by a new canary test (`ui/src/__tests__/no-legacy-tokens.test.ts`) that proves no banned utility name survives. `RequestAccessButton`'s hand-rolled focus-trap dialog folds into the canonical `<Modal>` primitive while the file is being touched anyway.

**Tech Stack:** Next.js 15 (App Router) · Bun · Tailwind v4 with `@theme` tokens · Vitest · React Testing Library · TypeScript strict.

---

## Pre-flight

Run these once before starting. Confirm the tree is clean and the baseline tests pass.

- [ ] **Pre-flight 1: Working tree is clean and on a feature branch**

```bash
cd /Users/notkanishk/Documents/Mkrspc/Projects/Syndra
git status
# Pick the branch prefix per your platform convention — `feat/` for Claude Code,
# `codex/` for Codex, or whatever your team uses — then create the branch:
git checkout -b <prefix>/wave-2-part-1-frontend-palette-finalization
```

Expected: `nothing to commit, working tree clean`. New branch created.

- [ ] **Pre-flight 2: Baseline UI tests are green**

```bash
cd /Users/notkanishk/Documents/Mkrspc/Projects/Syndra/ui
bun run test
```

Expected: all existing test files pass (per OCR-S4-12, baseline is 73/73 across 16 files).

- [ ] **Pre-flight 3: Backend tests are green (regression backstop)**

```bash
cd /Users/notkanishk/Documents/Mkrspc/Projects/Syndra/backend
go vet ./... && go test ./...
```

Expected: vet clean; `ok` for every package; baseline ~288/288.

---

## Task 0: Commit OpenSpec scaffolding

The proposal, design, tasks, and spec delta files have already been created during planning. This task commits them so subsequent migration commits land on a stable scaffold.

**Files:**
- Create: `openspec/changes/wave-2-part-1-frontend-palette-finalization/proposal.md` (already on disk)
- Create: `openspec/changes/wave-2-part-1-frontend-palette-finalization/design.md` (already on disk)
- Create: `openspec/changes/wave-2-part-1-frontend-palette-finalization/tasks.md` (already on disk)
- Create: `openspec/changes/wave-2-part-1-frontend-palette-finalization/specs/operational-readiness/spec.md` (already on disk)
- Create: `docs/superpowers/plans/2026-05-08-wave-2-part-1-frontend-palette-finalization.md` (this file)

- [ ] **Step 1: Confirm files exist**

```bash
ls openspec/changes/wave-2-part-1-frontend-palette-finalization/
ls openspec/changes/wave-2-part-1-frontend-palette-finalization/specs/operational-readiness/
ls docs/superpowers/plans/2026-05-08-wave-2-part-1-frontend-palette-finalization.md
```

Expected: `proposal.md  design.md  tasks.md  specs/`; `spec.md`; the plan file path resolves.

- [ ] **Step 2: Validate the OpenSpec change**

```bash
cd /Users/notkanishk/Documents/Mkrspc/Projects/Syndra
openspec validate wave-2-part-1-frontend-palette-finalization --strict
```

Expected: `Change 'wave-2-part-1-frontend-palette-finalization' is valid`.

If validate fails with a missing-spec-delta error, double-check the spec delta sits under `specs/operational-readiness/spec.md` (matches the capability-spec slug used by `openspec/INDEX.md`).

- [ ] **Step 3: Commit**

```bash
git add openspec/changes/wave-2-part-1-frontend-palette-finalization/ \
        docs/superpowers/plans/2026-05-08-wave-2-part-1-frontend-palette-finalization.md
git commit -m "$(cat <<'EOF'
docs: scaffold wave-2-part-1-frontend-palette-finalization OpenSpec change

Land the proposal, design, tasks, and operational-readiness spec delta for
the audit-resolution Wave 2 palette finalization (Theme 4 palette portion;
audit refs U2 + D10). Detailed step-by-step plan committed under
docs/superpowers/plans/.
EOF
)"
```

---

## Task 1: Add `--on-success` / `--on-warning` / `--on-info` foreground tokens to `globals.css`

This additive token block lands FIRST so every subsequent chromatic-background migration (SubmitButton success variant in Task 4, Button.tsx success/warning in Task 5, the six `bg-primary text-white` CTAs in Task 14) has the matching `text-on-*` utility available. The migration would not type-check at the Tailwind level otherwise — `text-on-success` would resolve to nothing because no `@theme` mapping exists yet.

Closes a real WCAG accessibility bug: pre-migration `text-white` on dark-mode `bg-success` (`#34d399`) / `bg-warning` (`#fbbf24`) / `bg-info` (`#60a5fa`) measures ~1.4–2.0 contrast ratio — well below AA's 4.5:1 floor. The new dark-theme `on-*` values are deeper same-family colors that pass AA against the bright backgrounds.

**Files:**
- Modify: `ui/src/app/globals.css` (light theme block, dark theme block, `@theme` mappings)

- [ ] **Step 1: Add light-theme `--on-success` / `--on-warning` / `--on-info`**

In `ui/src/app/globals.css`, inside the `:root, [data-theme="light"]` block, immediately after the existing `--info-hover: #075985;` declaration (around line 151), add:

```css
    --on-success: #ffffff;
    --on-warning: #ffffff;
    --on-info: #ffffff;
```

(All three land at `#ffffff` because the light-theme `bg-*` values — success `#047857`, warning `#b45309`, info `#0369a1` — are dark enough to take white text at WCAG AA contrast against the standard bg color.)

- [ ] **Step 2: Add dark-theme `--on-success` / `--on-warning` / `--on-info`**

In `ui/src/app/globals.css`, inside the `[data-theme="dark"]` block, immediately after the existing `--info-hover: #93c5fd;` declaration, add:

```css
    --on-success: #003822;  /* deep forest green; AA against #34d399 success */
    --on-warning: #2a1800;  /* deep amber-brown; AA against #fbbf24 warning */
    --on-info: #001a4a;     /* deep navy; AA against #60a5fa info */
```

- [ ] **Step 3: Add `--color-on-success` / `--color-on-warning` / `--color-on-info` to the `@theme` block**

In `ui/src/app/globals.css`, inside the `@theme {` block, locate the existing `--color-success: var(--success);` / `--color-warning: var(--warning);` / `--color-info: var(--info);` mappings inside the legacy-compatibility section. Immediately after `--color-info-hover: var(--info-hover);`, add:

```css
  --color-on-success: var(--on-success);
  --color-on-warning: var(--on-warning);
  --color-on-info: var(--on-info);
```

These mappings expose the `text-on-success`, `bg-on-success`, `border-on-success`, etc. Tailwind utilities for all three statuses. The mappings survive the legacy-block deletion in Task 15 because they are semantic status pairings, not legacy aliases.

- [ ] **Step 4: Verify the new declarations resolve**

```bash
cd /Users/notkanishk/Documents/Mkrspc/Projects/Syndra/ui
grep -nE 'on-success|on-warning|on-info' src/app/globals.css
```

Expected: nine matches — three CSS variable declarations under `:root, [data-theme="light"]`, three under `[data-theme="dark"]`, and three `--color-on-*` mappings inside `@theme`.

- [ ] **Step 5: Build the UI to confirm Tailwind picks up the new utilities**

```bash
bun run build
```

Expected: clean build. Tailwind v4 emits utilities for every `--color-*` mapping in `@theme`, so `text-on-success`, `bg-on-warning`, etc. are now resolvable.

- [ ] **Step 6: Commit**

```bash
git add src/app/globals.css
git commit -m "feat(ui): add --on-success/--on-warning/--on-info foreground tokens

Pairs with the existing semantic status backgrounds. Light theme:
#ffffff (bg-success/warning/info are dark enough for white text at AA).
Dark theme: deeper same-family colors that pass AA against the bright
bg-* values: on-success #003822, on-warning #2a1800, on-info #001a4a.
The matching --color-on-* mappings in @theme expose text-on-success /
bg-on-success / border-on-success utilities. Closes the WCAG AA contrast
failure that text-white on dark-mode bg-success/warning/info shipped at
~1.4-2.0 CR. Unblocks Tasks 4, 5, and 14 of wave-2-part-1."
```

---

## Task 2: Migrate `ui/src/components/ui/Skeleton.tsx`

Smallest leaf. Two legacy classes: `bg-surfaceHover` (camelCase typo — currently no-op in Tailwind v4) and `border-border`.

**Files:**
- Modify: `ui/src/components/ui/Skeleton.tsx:14, 25`

- [ ] **Step 1: Replace the pulse-block background**

Edit `ui/src/components/ui/Skeleton.tsx` line 14:

```tsx
// Before
      className={`animate-pulse rounded-md bg-surfaceHover ${className}`}
// After
      className={`animate-pulse rounded-md bg-surface-container-high ${className}`}
```

- [ ] **Step 2: Replace the card-list border**

Edit `ui/src/components/ui/Skeleton.tsx` line 25:

```tsx
// Before
        <div key={i} className="rounded-xl border border-border p-4 space-y-2">
// After
        <div key={i} className="rounded-xl border border-outline-variant p-4 space-y-2">
```

- [ ] **Step 3: Verify no legacy tokens remain in this file**

```bash
cd /Users/notkanishk/Documents/Mkrspc/Projects/Syndra/ui
grep -nE 'bg-surfaceHover|bg-surface-hover|border-border|text-foreground|text-muted|bg-primary-hover|bg-primaryHover|bg-danger|text-danger' src/components/ui/Skeleton.tsx
```

Expected: empty (exit code 1, no matches).

- [ ] **Step 4: Commit**

```bash
git add src/components/ui/Skeleton.tsx
git commit -m "refactor(ui): migrate Skeleton.tsx to MD3 palette tokens

bg-surfaceHover -> bg-surface-container-high; border-border ->
border-outline-variant. Part of wave-2-part-1-frontend-palette-finalization."
```

---

## Task 3: Migrate `ui/src/components/ui/CopyButton.tsx`

Single line touched (line 33). Four legacy classes: `border-border`, `text-muted`, `hover:text-foreground`, `hover:border-primary/40`.

**Files:**
- Modify: `ui/src/components/ui/CopyButton.tsx:33`

- [ ] **Step 1: Replace the button class string**

Edit `ui/src/components/ui/CopyButton.tsx` line 33:

```tsx
// Before
      className={`inline-flex items-center gap-1.5 rounded-md border border-border px-2 py-1 text-xs text-muted transition-colors hover:text-foreground hover:border-primary/40 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary ${className}`}
// After
      className={`inline-flex items-center gap-1.5 rounded-md border border-outline-variant px-2 py-1 text-xs text-on-surface-variant transition-colors hover:text-on-surface hover:border-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary ${className}`}
```

- [ ] **Step 2: Verify**

```bash
cd /Users/notkanishk/Documents/Mkrspc/Projects/Syndra/ui
grep -nE 'bg-surfaceHover|bg-surface-hover|border-border|text-foreground|text-muted|bg-primary-hover|bg-primaryHover|bg-danger|text-danger' src/components/ui/CopyButton.tsx
```

Expected: empty.

- [ ] **Step 3: Commit**

```bash
git add src/components/ui/CopyButton.tsx
git commit -m "refactor(ui): migrate CopyButton.tsx to MD3 palette tokens

border-border -> border-outline-variant; text-muted ->
text-on-surface-variant; hover:text-foreground -> hover:text-on-surface;
hover:border-primary/40 -> hover:border-primary."
```

---

## Task 4: Migrate `ui/src/components/ui/SubmitButton.tsx` (palette + `bg-primaryHover` typo fix)

Three button variants and one bug fix. The current `bg-primaryHover` (camelCase) silently no-ops in Tailwind v4; fixing it makes primary buttons gain a visible hover background. Per design.md §2.2, danger → `error` tokens, success → `success` tokens (kept), primary → `primary-container` on hover.

**Files:**
- Modify: `ui/src/components/ui/SubmitButton.tsx:31-36, 40-46`

- [ ] **Step 1: Replace the variant class strings**

Edit `ui/src/components/ui/SubmitButton.tsx` lines 31-36:

```tsx
// Before
  const variantClasses =
    variant === "danger"
      ? "bg-red-500 hover:bg-red-600 text-white"
      : variant === "success"
        ? "bg-emerald-500 hover:bg-emerald-600 text-white"
        : "bg-primary hover:bg-primaryHover text-white";
// After
  const variantClasses =
    variant === "danger"
      ? "bg-error text-on-error hover:bg-error/90"
      : variant === "success"
        ? "bg-success text-on-success hover:bg-success-hover"
        : "bg-primary text-on-primary hover:bg-primary-container hover:text-on-primary-container";
```

- [ ] **Step 2: Replace the disabled/spinner color references that leak white**

The spinner at line 51 uses `border-white/40 border-t-white`. The success variant still ships `text-white`, so keep this; for danger/primary it now reads `text-on-error`/`text-on-primary` whose value happens to be near-white in dark theme but a deliberate token in light theme. The spinner remains visible because it inherits `currentColor` via `border-t-white` — but that hardcodes white. For correctness, swap to `currentColor`.

Edit `ui/src/components/ui/SubmitButton.tsx` lines 49-52:

```tsx
// Before
      {isPending && (
        <span
          aria-hidden="true"
          className="h-4 w-4 animate-spin rounded-full border-2 border-white/40 border-t-white"
        />
      )}
// After
      {isPending && (
        <span
          aria-hidden="true"
          className="h-4 w-4 animate-spin rounded-full border-2 border-current/40 border-t-current"
        />
      )}
```

- [ ] **Step 3: Verify**

```bash
cd /Users/notkanishk/Documents/Mkrspc/Projects/Syndra/ui
grep -nE 'bg-surfaceHover|bg-surface-hover|border-border|text-foreground|text-muted|bg-primary-hover|bg-primaryHover|bg-danger|text-danger|bg-red-500|bg-emerald-500' src/components/ui/SubmitButton.tsx
```

Expected: empty.

- [ ] **Step 4: Run the SubmitButton-consuming tests**

`SubmitButton` is consumed by every create/update form modal. The Stage 1 component tests (`ConfirmModal.test.tsx`, `Modal.test.tsx`) and the page tests (`bundles`, `policies`, `requests`, `users`) exercise the submit path indirectly.

```bash
bun run test src/components/ui/__tests__/ConfirmModal.test.tsx \
             src/components/ui/__tests__/Modal.test.tsx \
             src/app/users/__tests__/page.test.tsx \
             src/app/__tests__/page.test.tsx
```

Expected: all green. If the existing snapshot tests assert on hex values resolved through legacy aliases, the tests already factor through @theme — they should remain green because the alias and MD3 token resolve to the same hex.

- [ ] **Step 5: Commit**

```bash
git add src/components/ui/SubmitButton.tsx
git commit -m "refactor(ui): migrate SubmitButton.tsx to MD3 palette tokens; fix bg-primaryHover typo

danger -> bg-error/text-on-error; success -> bg-success/text-on-success
(uses the new --on-success token from Task 1, closing a WCAG contrast
failure in dark mode); primary -> bg-primary/text-on-primary with
hover:bg-primary-container. The previous 'bg-primaryHover' (camelCase)
was a Tailwind no-op leaving primary buttons without a hover background;
the canonical token name fixes it. Spinner border swapped from
hardcoded white to currentColor."
```

---

## Task 5: Migrate `ui/src/components/ui/Button.tsx` (success/warning variants → `on-*` tokens)

`Button` is the shared button primitive consumed across the admin and member apps. The `success` and `warning` variants ship `bg-[var(--success)] text-white` and `bg-[var(--warning)] text-white` (lines 72-73). The `var(...)` indirection is redundant — the `@theme` block already exposes `bg-success` / `bg-warning` utilities that resolve to the same value — and the `text-white` foreground fails WCAG AA against the dark-mode bright backgrounds. The migration drops the `var(...)` indirection and pairs each background with the matching `on-*` token from Task 1.

The `primary` variant of `Button` already uses MD3 tokens (verified via OCR-S1-20) and is unchanged. Same for `secondary`, `ghost`, `outline`, `destructive`, and `link` variants — only `success` and `warning` carry the typed-out `var(...)` + `text-white` pair.

**Files:**
- Modify: `ui/src/components/ui/Button.tsx:72-73`

- [ ] **Step 1: Replace the success variant string**

In `ui/src/components/ui/Button.tsx` line 72:

```tsx
// Before
    success: "bg-[var(--success)] text-white hover:bg-[var(--success-hover)] shadow-sm",
// After
    success: "bg-success text-on-success hover:bg-success-hover shadow-sm",
```

- [ ] **Step 2: Replace the warning variant string**

In `ui/src/components/ui/Button.tsx` line 73:

```tsx
// Before
    warning: "bg-[var(--warning)] text-white hover:bg-[var(--warning-hover)] shadow-sm",
// After
    warning: "bg-warning text-on-warning hover:bg-warning-hover shadow-sm",
```

- [ ] **Step 3: Verify**

```bash
cd /Users/notkanishk/Documents/Mkrspc/Projects/Syndra/ui
grep -nE 'bg-\[var\(--success|bg-\[var\(--warning|text-white' src/components/ui/Button.tsx
```

Expected: empty. The success and warning variants now read straight through the `@theme` utilities; no `var(...)` indirection and no `text-white` survive in this file.

- [ ] **Step 4: Run Button-consuming tests**

`Button` is widely consumed. Run the impacted tests:

```bash
bun run test src/components/ui/__tests__/Modal.test.tsx \
             src/components/ui/__tests__/ConfirmModal.test.tsx \
             src/app/users/__tests__/page.test.tsx \
             src/app/__tests__/page.test.tsx
```

Expected: all green.

- [ ] **Step 5: Commit**

```bash
git add src/components/ui/Button.tsx
git commit -m "refactor(ui): migrate Button.tsx success/warning to on-* tokens

success: bg-[var(--success)] text-white -> bg-success text-on-success;
warning: bg-[var(--warning)] text-white -> bg-warning text-on-warning.
Drops the var() indirection (the @theme bg-success / bg-warning
utilities resolve through the same CSS variables) and pairs each
background with the new --on-success / --on-warning foreground tokens
from Task 1. Closes the WCAG AA contrast failure on dark-mode success
and warning button surfaces. Other variants (primary, secondary, ghost,
outline, destructive, link) already used MD3 tokens and are unchanged."
```

---

## Task 6: Migrate `ui/src/components/ui/JsonView.tsx` (`text-muted` separator + syntax-highlight tones)

JsonView carries two threads:
1. The `: ` key/value separator uses `text-muted` (two occurrences).
2. The JSON syntax-highlight tones (`text-amber-500` for null + diff, `text-emerald-500` for strings, `text-sky-500` for numbers/booleans) migrate to semantic status tokens (`text-warning`, `text-success`, `text-info`). The semantic-status tokens already encode dark-mode variants via `@theme`, so the visual character is preserved and the design system gains a single token vocabulary.

**Files:**
- Modify: `ui/src/components/ui/JsonView.tsx` (lines 46, 54, 67, 113, 115, 116, 121, 131)

- [ ] **Step 1: Replace both `text-muted` occurrences**

Replace every occurrence of `<span className="text-muted">: </span>` with `<span className="text-on-surface-variant">: </span>` in `ui/src/components/ui/JsonView.tsx`. (Two identical occurrences at the inline-primitive and nested-walk render paths.)

- [ ] **Step 2: Replace the null-literal tone**

In `ui/src/components/ui/JsonView.tsx`, replace every occurrence of `<span className="text-amber-500">null</span>` with `<span className="text-warning">null</span>`. (Two occurrences at the top-level `null` branch and the inline-primitive `null` branch.)

- [ ] **Step 3: Replace the string-value tones (diff and non-diff variants)**

In `ui/src/components/ui/JsonView.tsx`, replace every occurrence of `differs ? "text-amber-500 underline decoration-dashed" : "text-emerald-500"` with `differs ? "text-warning underline decoration-dashed" : "text-success"`. (Two occurrences — top-level string branch and inline-primitive string branch.)

- [ ] **Step 4: Replace the number/boolean tones (diff and non-diff variants)**

In `ui/src/components/ui/JsonView.tsx`, replace every occurrence of `differs ? "text-amber-500 underline decoration-dashed" : "text-sky-500"` with `differs ? "text-warning underline decoration-dashed" : "text-info"`. (Two occurrences — top-level number/boolean branch and inline-primitive number/boolean branch.)

- [ ] **Step 5: Verify**

```bash
cd /Users/notkanishk/Documents/Mkrspc/Projects/Syndra/ui
grep -nE 'bg-surfaceHover|bg-surface-hover|border-border|text-foreground|text-muted|bg-primary-hover|bg-primaryHover|bg-danger|text-danger|text-amber-500|text-emerald-500|text-sky-500' src/components/ui/JsonView.tsx
```

Expected: empty.

- [ ] **Step 6: Commit**

```bash
git add src/components/ui/JsonView.tsx
git commit -m "refactor(ui): migrate JsonView.tsx to MD3 palette + semantic status tokens

text-muted -> text-on-surface-variant for the key/value separator.
Syntax-highlight tones move into the design system: amber -> warning
(null + diff), emerald -> success (strings), sky -> info (numbers,
booleans). The semantic-status tokens already encode dark-mode variants
via @theme, so the visual character is preserved with no dark: overrides."
```

---

## Task 7: Migrate `ui/src/components/ThemeToggle.tsx`

Single button with four legacy classes: `border-border`, `text-muted`, `hover:text-foreground`, `hover:border-primary/40`.

**Files:**
- Modify: `ui/src/components/ThemeToggle.tsx:24`

- [ ] **Step 1: Replace the button class string**

Edit `ui/src/components/ThemeToggle.tsx` line 24:

```tsx
// Before
      className="inline-flex h-8 w-8 items-center justify-center rounded-lg border border-border text-muted transition-colors hover:text-foreground hover:border-primary/40 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
// After
      className="inline-flex h-8 w-8 items-center justify-center rounded-lg border border-outline-variant text-on-surface-variant transition-colors hover:text-on-surface hover:border-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
```

- [ ] **Step 2: Verify**

```bash
cd /Users/notkanishk/Documents/Mkrspc/Projects/Syndra/ui
grep -nE 'bg-surfaceHover|bg-surface-hover|border-border|text-foreground|text-muted|bg-primary-hover|bg-primaryHover|bg-danger|text-danger' src/components/ThemeToggle.tsx
```

Expected: empty.

- [ ] **Step 3: Commit**

```bash
git add src/components/ThemeToggle.tsx
git commit -m "refactor(ui): migrate ThemeToggle.tsx to MD3 palette tokens"
```

---

## Task 8: Migrate `ui/src/components/SidebarNav.tsx`

Two surfaces: section eyebrow header (`text-muted`) and the active/inactive nav link states. Active state uses `bg-primary/10 text-primary` today; MD3 equivalent is `bg-primary-container text-on-primary-container`. Inactive hover uses `hover:bg-surfaceHover hover:text-foreground`.

**Files:**
- Modify: `ui/src/components/SidebarNav.tsx:100-114`

- [ ] **Step 1: Replace the section header text**

Edit `ui/src/components/SidebarNav.tsx` line 100:

```tsx
// Before
          <p className="px-3 py-1 text-xs font-semibold text-muted uppercase tracking-wider">
// After
          <p className="px-3 py-1 text-xs font-semibold text-on-surface-variant uppercase tracking-wider">
```

- [ ] **Step 2: Replace the active/inactive nav link classes**

Edit `ui/src/components/SidebarNav.tsx` lines 110-114:

```tsx
// Before
                className={`group flex items-center justify-between rounded-md px-3 py-2 text-sm transition-colors ${
                  active
                    ? "bg-primary/10 text-primary border-l-2 border-primary"
                    : "text-muted hover:text-foreground hover:bg-surfaceHover border-l-2 border-transparent"
                }`}
// After
                className={`group flex items-center justify-between rounded-md px-3 py-2 text-sm transition-colors ${
                  active
                    ? "bg-primary-container text-on-primary-container border-l-2 border-primary"
                    : "text-on-surface-variant hover:text-on-surface hover:bg-surface-container-high border-l-2 border-transparent"
                }`}
```

- [ ] **Step 3: Verify**

```bash
cd /Users/notkanishk/Documents/Mkrspc/Projects/Syndra/ui
grep -nE 'bg-surfaceHover|bg-surface-hover|border-border|text-foreground|text-muted|bg-primary-hover|bg-primaryHover|bg-danger|text-danger|bg-primary/[0-9]+' src/components/SidebarNav.tsx
```

Expected: empty.

- [ ] **Step 4: Commit**

```bash
git add src/components/SidebarNav.tsx
git commit -m "refactor(ui): migrate SidebarNav.tsx to MD3 palette tokens

Active nav state: bg-primary/10 text-primary -> bg-primary-container
text-on-primary-container (MD3 selected state). Inactive hover:
hover:bg-surfaceHover -> hover:bg-surface-container-high; text-muted ->
text-on-surface-variant. Section eyebrow: text-muted ->
text-on-surface-variant."
```

---

## Task 9: Migrate `ui/src/components/Sidebar.tsx` (palette + LXC dot → `<Pulse static />`)

Six legacy classes plus a hardcoded `bg-emerald-500` indicator. Per design.md §2.2, the live indicator routes through the canonical `<Pulse variant="success" static />` primitive (introduced OCR-S1-24).

**Files:**
- Modify: `ui/src/components/Sidebar.tsx:1-5, 11, 13, 22, 25, 28, 34, 39`

- [ ] **Step 1: Add the `Pulse` import**

Edit `ui/src/components/Sidebar.tsx` lines 1-5:

```tsx
// Before
import { Card } from './ui/Card';
import SidebarNav from './SidebarNav';
import SystemModeBadge from './SystemModeBadge';
import ThemeToggle from './ThemeToggle';
import type { SessionUser } from '@/lib/session';
// After
import { Card } from './ui/Card';
import { Pulse } from './ui/Pulse';
import SidebarNav from './SidebarNav';
import SystemModeBadge from './SystemModeBadge';
import ThemeToggle from './ThemeToggle';
import type { SessionUser } from '@/lib/session';
```

- [ ] **Step 2: Replace the shell, header, and footer surfaces**

Edit `ui/src/components/Sidebar.tsx`:

Line 11 (shell):
```tsx
// Before
    <div className="w-64 h-screen bg-surface border-r border-border flex flex-col">
// After
    <div className="w-64 h-screen bg-surface border-r border-outline-variant flex flex-col">
```

Line 13 (heading):
```tsx
// Before
        <h2 className="text-xl font-bold text-foreground tracking-tight">Syndra</h2>
// After
        <h2 className="text-xl font-bold text-on-surface tracking-tight">Syndra</h2>
```

Line 22 (footer card):
```tsx
// Before
        <Card className="!p-4 bg-surfaceHover border-none shadow-none">
// After
        <Card className="!p-4 bg-surface-container-high border-none shadow-none">
```

Lines 25, 28 (footer text):
```tsx
// Before
              <p className="text-xs text-muted">Signed in as</p>
              <div className="mt-2">
                <p className="text-sm font-medium">{session.name}</p>
                <p className="text-xs text-muted">{session.role === "admin" ? "Admin session" : "Member session"}</p>
// After
              <p className="text-xs text-on-surface-variant">Signed in as</p>
              <div className="mt-2">
                <p className="text-sm font-medium">{session.name}</p>
                <p className="text-xs text-on-surface-variant">{session.role === "admin" ? "Admin session" : "Member session"}</p>
```

Line 34 (sign-out button):
```tsx
// Before
            <button type="submit" className="w-full rounded-lg border border-border px-3 py-2 text-sm text-muted transition-colors hover:border-primary/40 hover:text-foreground">
// After
            <button type="submit" className="w-full rounded-lg border border-outline-variant px-3 py-2 text-sm text-on-surface-variant transition-colors hover:border-primary hover:text-on-surface">
```

- [ ] **Step 3: Replace the LXC live dot with `<Pulse static />`**

Edit `ui/src/components/Sidebar.tsx` lines 38-41:

```tsx
// Before
          <div className="flex items-center mt-4">
            <span className="w-2 h-2 rounded-full bg-emerald-500 mr-2"></span>
            <span className="text-sm font-medium">Proxmox LXC</span>
          </div>
// After
          <div className="flex items-center gap-2 mt-4 text-success">
            <Pulse variant="success" static />
            <span className="text-sm font-medium text-on-surface">Proxmox LXC</span>
          </div>
```

(`Pulse` uses `currentColor` for its dot fill via the parent `text-success` class; `pulse-dot-static` from Stage 1 is the underlying utility. The label color resets to `text-on-surface` so the green tone applies only to the dot.)

- [ ] **Step 4: Verify**

```bash
cd /Users/notkanishk/Documents/Mkrspc/Projects/Syndra/ui
grep -nE 'bg-surfaceHover|bg-surface-hover|border-border|text-foreground|text-muted|bg-primary-hover|bg-primaryHover|bg-danger|text-danger|bg-emerald-500|hover:border-primary/' src/components/Sidebar.tsx
```

Expected: empty.

Confirm `Pulse` imports correctly:

```bash
grep -n "Pulse" src/components/Sidebar.tsx src/components/ui/Pulse.tsx
```

Expected: import in Sidebar; component definition in Pulse.

- [ ] **Step 5: Commit**

```bash
git add src/components/Sidebar.tsx
git commit -m "refactor(ui): migrate Sidebar.tsx to MD3 palette tokens

Shell, header, footer card, sign-out button all route through MD3 tokens.
The hardcoded bg-emerald-500 LXC live dot is replaced with <Pulse
variant='success' static />, the project's canonical live-indicator
primitive (OCR-S1-24)."
```

---

## Task 10: Migrate `ui/src/components/RequestAccessButton.tsx` (palette + `<Modal>` wrap)

Largest non-page leaf migration. Two threads:
1. Palette token migration across the modal body (lines 149, 151, 154, 160, 165, 168, 184-185, 200).
2. Replace the hand-rolled focus-trap dialog (lines 38-39, 62-79, 137-150, 197-200) with the canonical `<Modal>` primitive from `ui/Modal.tsx` (OCR-S1-16). Modal already provides focus trap, `aria-modal="true"`, Esc, and click-outside dismiss covered by `Modal.test.tsx`.

The button trigger (line 81-89 for "View Requests" link, line 128-135 for "Request Access" button) keeps its existing styling and route — they are unrelated to the modal.

**Files:**
- Modify: `ui/src/components/RequestAccessButton.tsx` (entire file rewrite is cleanest)

- [ ] **Step 1: Read the current file once more to confirm line numbers**

```bash
wc -l src/components/RequestAccessButton.tsx
```

Expected: ~217 lines.

- [ ] **Step 2: Write the migrated file**

Replace the entire file content with the following MD3 + Modal version (use whichever full-file rewrite affordance your platform provides — `Write` in Claude Code, an `apply_patch` whole-file substitution in Codex, or a plain text-editor save):

```tsx
"use client";

import { useEffect, useState } from "react";

import { Modal } from "@/components/ui/Modal";
import { SubmitButton } from "@/components/ui/SubmitButton";
import { toastError, toastSuccess } from "@/lib/toast";

interface ProjectInfo {
  id: string;
  name: string;
  roles: Array<{ key: string; label: string }>;
}

interface CatalogResponse {
  projects?: ProjectInfo[];
}

interface RequestAccessButtonProps {
  projectId: string;
  serviceName: string;
  /** "No Access" gets the inline modal; everything else routes to /requests. */
  status: "Active" | "Pending" | "No Access";
}

/**
 * Inline "Request Access" affordance for the member service catalog.
 * Wraps the canonical <Modal> primitive (focus trap, aria-modal, Esc, click-outside)
 * with a justification textarea and a friendly duration picker; submits to
 * `/api/proxy/requests` and shows toast feedback. For already-active or pending
 * services, links straight to `/requests` since there's nothing to request.
 */
export default function RequestAccessButton({ projectId, serviceName, status }: RequestAccessButtonProps) {
  const [open, setOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [project, setProject] = useState<ProjectInfo | null>(null);
  const [justification, setJustification] = useState("");
  const [duration, setDuration] = useState<"7" | "30" | "120" | "0">("30");

  useEffect(() => {
    if (!open) return;
    let cancelled = false;
    (async () => {
      try {
        const res = await fetch("/api/proxy/catalog");
        if (!res.ok) return;
        const data: CatalogResponse = await res.json();
        if (cancelled) return;
        const match = (data.projects ?? []).find((p) => p.id === projectId) ?? null;
        setProject(match);
      } catch {
        // Submit will surface the failure if the catalog is genuinely broken.
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [open, projectId]);

  if (status !== "No Access") {
    return (
      <a
        href="/requests"
        className="mt-4 inline-flex rounded-lg bg-primary px-4 py-2 text-sm font-medium text-on-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
      >
        View Requests
      </a>
    );
  }

  const defaultRole = project?.roles?.[0];
  const defaultRoleKey = defaultRole?.key ?? "";
  const defaultRoleLabel = defaultRole?.label ?? "default role";

  const submit = async () => {
    if (!justification.trim() || !defaultRoleKey) {
      toastError(!justification.trim() ? "Add a justification before submitting." : "No default role available for this service.");
      return;
    }
    setSubmitting(true);
    try {
      const res = await fetch("/api/proxy/requests", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          project_id: projectId,
          role_key: defaultRoleKey,
          justification: justification.trim(),
          duration_days: Number.parseInt(duration, 10),
        }),
      });
      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        throw new Error(body.message || "Failed to submit request.");
      }
      toastSuccess("Request submitted", `Your administrator will review the request for ${serviceName}.`);
      setOpen(false);
      setJustification("");
    } catch (err) {
      toastError(err instanceof Error ? err.message : "Failed to submit request.");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        className="mt-4 inline-flex rounded-lg bg-primary px-4 py-2 text-sm font-medium text-on-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
      >
        Request Access
      </button>

      <Modal
        open={open}
        onClose={() => { if (!submitting) setOpen(false); }}
        labelledBy={`request-${projectId}-title`}
      >
        <h2 id={`request-${projectId}-title`} className="text-lg font-semibold text-on-surface">
          Request access to {serviceName}
        </h2>
        <p className="mt-2 text-sm text-on-surface-variant">
          {project
            ? `You'll be requesting the "${defaultRoleLabel}" role. Add a brief justification and pick how long you need it.`
            : "Loading service details…"}
        </p>

        <label className="mt-4 block text-xs font-medium text-on-surface-variant">Why do you need this access?</label>
        <textarea
          value={justification}
          onChange={(e) => setJustification(e.target.value)}
          placeholder="Briefly explain the project or task that needs this access."
          className="mt-2 min-h-[6rem] w-full rounded-lg border border-outline-variant bg-background px-3 py-2 text-sm focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
        />

        <p className="mt-4 text-xs uppercase tracking-[0.18em] text-on-surface-variant">Duration</p>
        <div className="mt-2 flex flex-wrap gap-2">
          {([
            { label: "1 week", value: "7" },
            { label: "1 month", value: "30" },
            { label: "1 semester", value: "120" },
            { label: "Permanent", value: "0" },
          ] as const).map((opt) => {
            const selected = duration === opt.value;
            return (
              <button
                type="button"
                key={opt.value}
                onClick={() => setDuration(opt.value)}
                className={`rounded-full border px-3 py-1 text-xs font-medium transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary ${
                  selected
                    ? "border-primary bg-primary-container text-on-primary-container"
                    : "border-outline-variant text-on-surface-variant hover:text-on-surface hover:border-primary"
                }`}
              >
                {opt.label}
              </button>
            );
          })}
        </div>

        <div className="mt-6 flex justify-end gap-3">
          <button
            type="button"
            onClick={() => setOpen(false)}
            disabled={submitting}
            className="rounded-lg border border-outline-variant px-4 py-2 text-sm font-medium text-on-surface transition-colors hover:bg-surface-container-high disabled:opacity-50"
          >
            Cancel
          </button>
          <SubmitButton
            isPending={submitting}
            pendingLabel="Submitting…"
            disabled={!justification.trim() || !defaultRoleKey}
            label="Submit request"
            onClick={submit}
          />
        </div>
      </Modal>
    </>
  );
}
```

- [ ] **Step 3: Confirm `Modal` exposes the props this file uses**

```bash
grep -n "interface ModalProps\|export function Modal\|export const Modal" src/components/ui/Modal.tsx
sed -n '1,80p' src/components/ui/Modal.tsx
```

Expected: `Modal` accepts an `open` boolean, an `onClose` callback, and a `labelledBy` (or equivalently named) prop pointing at a heading element id.

If the prop names differ (e.g. `isOpen`/`onDismiss`/`ariaLabelledBy`), adjust the `<Modal>` invocation in the rewrite to match. The exact prop shape is whatever `Modal.tsx` already exports; do not modify `Modal.tsx` here.

- [ ] **Step 4: Verify**

```bash
cd /Users/notkanishk/Documents/Mkrspc/Projects/Syndra/ui
grep -nE 'bg-surfaceHover|bg-surface-hover|border-border|text-foreground|text-muted|bg-primary-hover|bg-primaryHover|bg-danger|text-danger|bg-primary/[0-9]+|hover:border-primary/' src/components/RequestAccessButton.tsx
```

Expected: empty.

Run the existing Modal a11y test to confirm the contract is unchanged from this file's perspective:

```bash
bun run test src/components/ui/__tests__/Modal.test.tsx
```

Expected: green.

- [ ] **Step 5: Commit**

```bash
git add src/components/RequestAccessButton.tsx
git commit -m "refactor(ui): migrate RequestAccessButton to MD3 tokens; compose <Modal>

Folds the hand-rolled focus-trap dialog into the canonical <Modal>
primitive (OCR-S1-16). Focus trap, aria-modal, Esc, and click-outside
dismiss are now inherited from Modal.test.tsx coverage. All palette
classes routed through MD3 tokens; bg-primary/10 selected duration pill
becomes bg-primary-container text-on-primary-container."
```

---

## Task 11: Migrate `ui/src/components/ErrorBoundary.tsx`

Five legacy classes plus a hardcoded `border-red-500/40 bg-red-500/5` semantic-error alert. Route through error-container tokens.

**Files:**
- Modify: `ui/src/components/ErrorBoundary.tsx:36-49`

- [ ] **Step 1: Replace the alert wrapper, headings, and button**

Edit `ui/src/components/ErrorBoundary.tsx` lines 36-49:

```tsx
// Before
        <div
          role="alert"
          className="rounded-xl border border-dashed border-red-500/40 bg-red-500/5 p-8 text-center"
        >
          <p className="text-sm font-semibold text-foreground">Something went wrong</p>
          <p className="mx-auto mt-2 max-w-md text-sm text-muted">
            The page hit an unexpected error while rendering. Try again, or refresh if it
            keeps happening.
          </p>
          <button
            onClick={this.reset}
            type="button"
            className="mt-4 inline-flex rounded-lg bg-primary px-4 py-2 text-xs font-semibold uppercase tracking-[0.16em] text-white"
          >
            Try again
          </button>
        </div>
// After
        <div
          role="alert"
          className="rounded-xl border border-dashed border-error/40 bg-error-container/40 p-8 text-center"
        >
          <p className="text-sm font-semibold text-on-error-container">Something went wrong</p>
          <p className="mx-auto mt-2 max-w-md text-sm text-on-surface-variant">
            The page hit an unexpected error while rendering. Try again, or refresh if it
            keeps happening.
          </p>
          <button
            onClick={this.reset}
            type="button"
            className="mt-4 inline-flex rounded-lg bg-primary px-4 py-2 text-xs font-semibold uppercase tracking-[0.16em] text-on-primary"
          >
            Try again
          </button>
        </div>
```

- [ ] **Step 2: Verify**

```bash
cd /Users/notkanishk/Documents/Mkrspc/Projects/Syndra/ui
grep -nE 'bg-surfaceHover|bg-surface-hover|border-border|text-foreground|text-muted|bg-primary-hover|bg-primaryHover|bg-danger|text-danger|bg-red-500|border-red-500' src/components/ErrorBoundary.tsx
```

Expected: empty.

- [ ] **Step 3: Commit**

```bash
git add src/components/ErrorBoundary.tsx
git commit -m "refactor(ui): migrate ErrorBoundary.tsx to MD3 palette tokens

Semantic error alert routed through error/error-container tokens;
heading and button tone use on-error-container/on-primary."
```

---

## Task 12: Migrate `ui/src/app/page.tsx` (member home)

Member-facing welcome panel + service catalog. The admin branch (line 117) delegates to `<AdminDashboard>`, which is already MD3-native. Only the member branch needs migration.

**Files:**
- Modify: `ui/src/app/page.tsx:43-101`

- [ ] **Step 1: Replace the welcome heading**

Edit `ui/src/app/page.tsx` lines 44-45:

```tsx
// Before
          <h1 className="mt-3 text-3xl font-bold text-foreground tracking-tight">Welcome back, {session.name}</h1>
          <p className="mt-2 text-muted">
// After
          <h1 className="mt-3 text-3xl font-bold text-on-surface tracking-tight">Welcome back, {session.name}</h1>
          <p className="mt-2 text-on-surface-variant">
```

- [ ] **Step 2: Replace the three identity/active/pending card subtexts**

Edit `ui/src/app/page.tsx`. Three identical-looking lines:

Line 56:
```tsx
// Before
            <p className="mt-2 text-sm text-muted">{session.team} • {session.location}</p>
// After
            <p className="mt-2 text-sm text-on-surface-variant">{session.team} • {session.location}</p>
```

Line 63:
```tsx
// Before
            <p className="mt-2 text-sm text-muted">Applications currently available to your session.</p>
// After
            <p className="mt-2 text-sm text-on-surface-variant">Applications currently available to your session.</p>
```

Line 70:
```tsx
// Before
            <p className="mt-2 text-sm text-muted">Requests still waiting in the governance queue.</p>
// After
            <p className="mt-2 text-sm text-on-surface-variant">Requests still waiting in the governance queue.</p>
```

- [ ] **Step 3: Replace the service-catalog tile classes**

Edit `ui/src/app/page.tsx` lines 93-101:

```tsx
// Before
                <div key={entry.application.id} className="rounded-2xl border border-border bg-surfaceHover p-5">
                  <div className="flex items-start justify-between gap-3">
                    <div>
                      <p className="text-lg font-semibold text-foreground">{entry.application.name}</p>
                      <p className="mt-1 text-sm text-muted">{entry.application.description}</p>
                    </div>
                    <Badge variant={status === "Active" ? "secondary" : "outline"}>{status}</Badge>
                  </div>
                  <p className="mt-4 text-xs uppercase tracking-[0.22em] text-muted">{entry.application.consumer}</p>
// After
                <div key={entry.application.id} className="rounded-2xl border border-outline-variant bg-surface-container-high p-5">
                  <div className="flex items-start justify-between gap-3">
                    <div>
                      <p className="text-lg font-semibold text-on-surface">{entry.application.name}</p>
                      <p className="mt-1 text-sm text-on-surface-variant">{entry.application.description}</p>
                    </div>
                    <Badge variant={status === "Active" ? "secondary" : "outline"}>{status}</Badge>
                  </div>
                  <p className="mt-4 text-xs uppercase tracking-[0.22em] text-on-surface-variant">{entry.application.consumer}</p>
```

- [ ] **Step 4: Verify**

```bash
cd /Users/notkanishk/Documents/Mkrspc/Projects/Syndra/ui
grep -nE 'bg-surfaceHover|bg-surface-hover|border-border|text-foreground|text-muted|bg-primary-hover|bg-primaryHover|bg-danger|text-danger' src/app/page.tsx
```

Expected: empty.

- [ ] **Step 5: Run the home-page test (if it currently exists for the member branch)**

```bash
bun run test src/app/__tests__/page.test.tsx
```

Expected: green. The existing test file primarily covers the admin hero (per OCR-S2-05). The member branch has no dedicated test today — the migration is verified by the canary test in Task 15.

- [ ] **Step 6: Commit**

```bash
git add src/app/page.tsx
git commit -m "refactor(ui): migrate member home (app/page.tsx) to MD3 palette tokens

Welcome panel headings, identity/active/pending card subtexts, and
service-catalog tiles route through MD3 tokens. Admin branch is
unchanged (already MD3-native via <AdminDashboard>)."
```

---

## Task 13: Migrate `ui/src/app/graph/page.tsx` `nodeTone()`

The topology canvas encodes node kind through `nodeTone()` (`app/graph/page.tsx:39-50`). The `project` branch already uses MD3 tokens (`border-primary-container/40 bg-primary-container/10 text-primary-container`); the other three branches use hardcoded core-Tailwind tones with explicit `dark:` overrides. Migrate them to the semantic status tokens — `application → info`, `bundle → warning`, `role → success` — keeping the same `/30 /8` opacity shape and dropping the now-redundant `dark:text-*-300` overrides (the design tokens already auto-flip via `@theme`).

**Files:**
- Modify: `ui/src/app/graph/page.tsx:42, 44, 48`

- [ ] **Step 1: Replace the three node-tone branches**

In `ui/src/app/graph/page.tsx`:

Line 42 (application):
```tsx
// Before
      return "border-sky-500/30 bg-sky-500/8 text-sky-600 dark:text-sky-300";
// After
      return "border-info/30 bg-info/8 text-info";
```

Line 44 (bundle):
```tsx
// Before
      return "border-amber-500/30 bg-amber-500/8 text-amber-600 dark:text-amber-300";
// After
      return "border-warning/30 bg-warning/8 text-warning";
```

Line 48 (role):
```tsx
// Before
      return "border-emerald-500/30 bg-emerald-500/8 text-emerald-600 dark:text-emerald-300";
// After
      return "border-success/30 bg-success/8 text-success";
```

The `project` branch (line 46) stays unchanged — already MD3.

- [ ] **Step 2: Verify**

```bash
cd /Users/notkanishk/Documents/Mkrspc/Projects/Syndra/ui
grep -nE 'sky-500|amber-500|emerald-500|sky-600|amber-600|emerald-600|sky-300|amber-300|emerald-300' src/app/graph/page.tsx
```

Expected: empty.

- [ ] **Step 3: Commit**

```bash
git add src/app/graph/page.tsx
git commit -m "refactor(ui): migrate graph/page.tsx nodeTone() to semantic status tokens

application -> info, bundle -> warning, role -> success. The project
branch already used MD3 primary-container tokens. The explicit dark:
text-*-300 overrides are dropped because the semantic-status tokens
auto-flip between light and dark themes via @theme."
```

---

## Task 14: Migrate `ui/src/app/zitadel/page.tsx` legacy CRUD sections

Largest migration by line count (~50 occurrences across the Rotation, Health, Projects, Users, AllGrants sections). The Stage-3 `<LiveStatusTile>` (lines 1-60ish) is already MD3-native and is **not** touched. The U5 page split into `components/zitadel/{Health,Rotation,Projects,Users,AllGrants}.tsx` is **out of scope** for this part — palette migration happens in-place; the file split lands in Wave 3.

**Strategy:** Apply seven file-scoped find-and-replace passes to swap each banned token, then sweep the remaining hardcoded red/destructive utilities by hand. (Claude Code workers use `Edit` with `replace_all: true`; Codex workers use `apply_patch` with multiple substitution stanzas; either way the file should end up with zero banned substrings.)

**Files:**
- Modify: `ui/src/app/zitadel/page.tsx` (multiple line ranges, see steps below)

- [ ] **Step 1: Replace `bg-surfaceHover` (6 occurrences)**

In `ui/src/app/zitadel/page.tsx`, replace every occurrence of `bg-surfaceHover` with `bg-surface-container-high`. Expected: 6 replacements at lines 284, 303, 308, 381, 522, 785.

- [ ] **Step 2: Replace `border-border` (16+ occurrences)**

In `ui/src/app/zitadel/page.tsx`, replace every occurrence of `border-border` with `border-outline-variant`. Expected: replacements at all `border border-border` and `border-t border-border` sites in the CRUD sections.

- [ ] **Step 3: Replace `text-foreground` (8 occurrences)**

In `ui/src/app/zitadel/page.tsx`, replace every occurrence of `text-foreground` with `text-on-surface`.

- [ ] **Step 4: Replace `text-muted` (~30 occurrences)**

In `ui/src/app/zitadel/page.tsx`, replace every occurrence of `text-muted` with `text-on-surface-variant`.

- [ ] **Step 5: Replace `hover:text-foreground` artifacts**

Step 3 already replaces `text-foreground` substring; `hover:text-foreground` becomes `hover:text-on-surface`. Confirm with a grep:

```bash
cd /Users/notkanishk/Documents/Mkrspc/Projects/Syndra/ui
grep -n 'hover:text-foreground\|hover:text-on-surface' src/app/zitadel/page.tsx
```

Expected: zero `hover:text-foreground`; non-zero `hover:text-on-surface` if present.

- [ ] **Step 6: Replace destructive Revoke button styling at lines ~554 and ~814**

Two near-identical Revoke buttons:

```tsx
// Before (line ~554)
                        className="rounded-lg border border-red-500/40 px-3 py-1 text-xs text-red-400 hover:bg-red-500/10"
// After
                        className="rounded-lg border border-error/40 px-3 py-1 text-xs text-error hover:bg-error-container/40"
```

```tsx
// Before (line ~814)
                        className="rounded-lg border border-red-500/40 px-3 py-1 text-xs text-red-400 hover:bg-red-500/10"
// After
                        className="rounded-lg border border-error/40 px-3 py-1 text-xs text-error hover:bg-error-container/40"
```

(Both instances are identical, so a single file-scoped replace-every-occurrence pass handles both.)

- [ ] **Step 7: Replace every remaining `text-red-400` with `text-error` (6 sites)**

In `ui/src/app/zitadel/page.tsx`, replace every occurrence of `text-red-400` with `text-error`. Expected: 6 replacements at lines 300 (inline error in Rotation), 376/377 (Health `result.error` + `networkError` flashes), 597 (Projects role-edit ternary), 855 (Users grant-edit ternary), 912 (AllGrants `error` flash).

After this step, the previous Step 6 Revoke replacement was already `text-red-400 → text-error` within the bigger class string, so this pass sweeps any independent occurrences.

- [ ] **Step 8: Replace every `text-emerald-400` with `text-success` (2 sites)**

In `ui/src/app/zitadel/page.tsx`, replace every occurrence of `text-emerald-400` with `text-success`. Expected: 2 replacements at lines 597 and 855, both inside `flash.kind === "ok" ? "text-emerald-400" : "text-red-400"` ternaries. The matching `text-red-400` half is already covered by Step 7.

- [ ] **Step 9: Replace every `text-white` paired with `bg-primary` with `text-on-primary` (6 sites)**

In `ui/src/app/zitadel/page.tsx`, six button class strings combine `bg-primary` with `text-white`. They appear at lines 365 (Health "Run health"), 538 (Projects role-edit Save), 588 (Projects "Add role" submit), 795 (Users grant-edit Save), 846 (Users "Assign grant" submit), 905 (AllGrants "Load more"). Each contains the substring `bg-primary px-` followed by `text-white`.

Replace every occurrence of `text-white` with `text-on-primary` in this file. Expected: 6 replacements. (All `text-white` occurrences in the legacy CRUD sections pair with `bg-primary`; the Stage-3 `<LiveStatusTile>` at the top of the file does not use `text-white`.)

- [ ] **Step 10: Verify**

```bash
cd /Users/notkanishk/Documents/Mkrspc/Projects/Syndra/ui
grep -nE 'bg-surfaceHover|bg-surface-hover|border-border|text-foreground|text-muted|bg-primary-hover|bg-primaryHover|bg-danger|text-danger|text-red-400|text-emerald-400|text-white|border-red-500|hover:bg-red-500' src/app/zitadel/page.tsx
```

Expected: empty.

- [ ] **Step 11: Build the UI to confirm Tailwind resolves every utility**

```bash
bun run build
```

Expected: build completes without `unknown class` warnings. Tailwind v4 errors loudly when an utility cannot resolve to an `@theme` token. If any utility errors, the offending class did not migrate cleanly — return to it.

- [ ] **Step 12: Commit**

```bash
git add src/app/zitadel/page.tsx
git commit -m "refactor(ui): migrate /zitadel CRUD sections to MD3 palette tokens

Rotation, Health, Projects, Users, AllGrants sections route through MD3
tokens. Destructive Revoke buttons and inline error text use error /
error-container tokens. The six text-red-400 sites (inline error + flash
states across Rotation, Health, Projects, Users, AllGrants) route through
text-error. The two text-emerald-400 flash-success ternary branches route
through text-success. The six bg-primary text-white CTA buttons (Health,
Projects role-edit/add, Users grant-edit/assign, AllGrants load-more)
route through text-on-primary. The Stage-3 <LiveStatusTile> at the top
of the file was already MD3-native and is unchanged. Page-split (U5)
stays deferred to Wave 3 Theme 4 remainder."
```

---

## Task 15: Add canary test + delete legacy block from `globals.css`

This is the keystone task. The canary test asserts the end-state contract; deleting the alias block is the last edit that makes it pass.

**Files:**
- Create: `ui/src/__tests__/no-legacy-tokens.test.ts`
- Modify: `ui/src/app/globals.css:54-69` (delete the legacy compatibility tokens block)

- [ ] **Step 1: Write the canary test**

Create `ui/src/__tests__/no-legacy-tokens.test.ts`:

```ts
import { readFileSync, readdirSync, statSync } from "node:fs";
import { join, resolve } from "node:path";
import { describe, expect, it } from "vitest";

const SRC_ROOT = resolve(__dirname, "..");
const GLOBALS_CSS = resolve(SRC_ROOT, "app/globals.css");

const BANNED_UTILITIES = [
  // Legacy palette aliases (OCR-S1-05 compatibility block).
  "bg-surfaceHover",
  "bg-surface-hover",
  "hover:bg-surfaceHover",
  "hover:bg-surface-hover",
  "border-border",
  "text-foreground",
  "hover:text-foreground",
  "text-muted",
  "hover:text-muted",
  "bg-primary-hover",
  "hover:bg-primary-hover",
  "bg-primaryHover",
  "hover:bg-primaryHover",
  "bg-danger",
  "text-danger",
  "bg-danger-hover",
  // Hardcoded core-Tailwind tones that previously survived in JsonView
  // syntax highlighting and graph/page.tsx nodeTone(). Migrated to
  // success / warning / info semantic status tokens.
  "text-amber-500",
  "text-emerald-500",
  "text-sky-500",
  "text-amber-600",
  "text-emerald-600",
  "text-sky-600",
  "border-amber-500",
  "border-emerald-500",
  "border-sky-500",
  "bg-amber-500",
  "bg-emerald-500",
  "bg-sky-500",
  "dark:text-amber-300",
  "dark:text-emerald-300",
  "dark:text-sky-300",
  // Hardcoded zitadel/page.tsx flash + error tones. Migrated to
  // text-error / text-success. text-white is intentionally NOT banned
  // here — it's a neutral utility and may persist outside this change's
  // scope. In every chromatic-background pairing within this change's
  // touched files, text-white has been replaced with the matching
  // text-on-* token.
  "text-red-400",
  "text-emerald-400",
];

const BANNED_CSS_TOKENS = [
  "--color-foreground",
  "--color-surface-hover",
  "--color-border",
  "--color-primary-hover",
  "--color-muted",
  "--color-danger",
  "--color-danger-hover",
];

const SKIP_DIRS = new Set(["__tests__", "node_modules", ".next", "dist"]);
const SKIP_FILES = new Set([
  // The canary test itself enumerates the banned strings; exclude it.
  resolve(__dirname, "no-legacy-tokens.test.ts"),
]);
const INCLUDED_EXTS = new Set([".ts", ".tsx", ".css"]);

function* walk(dir: string): Generator<string> {
  for (const entry of readdirSync(dir)) {
    const path = join(dir, entry);
    const stat = statSync(path);
    if (stat.isDirectory()) {
      if (SKIP_DIRS.has(entry)) continue;
      yield* walk(path);
    } else if (stat.isFile()) {
      if (SKIP_FILES.has(path)) continue;
      const ext = entry.slice(entry.lastIndexOf("."));
      if (INCLUDED_EXTS.has(ext)) yield path;
    }
  }
}

describe("no-legacy-tokens canary", () => {
  it("globals.css has no legacy compatibility token aliases", () => {
    const css = readFileSync(GLOBALS_CSS, "utf8");
    for (const token of BANNED_CSS_TOKENS) {
      expect(css, `globals.css must not declare ${token}`).not.toContain(token);
    }
    expect(
      css,
      "globals.css must not carry the 'Legacy compatibility tokens' header",
    ).not.toMatch(/Legacy compatibility tokens/);
  });

  it("no source file uses a banned palette utility class", () => {
    const offenders: { file: string; matches: string[] }[] = [];
    for (const file of walk(SRC_ROOT)) {
      const content = readFileSync(file, "utf8");
      const found = BANNED_UTILITIES.filter((u) => content.includes(u));
      if (found.length > 0) {
        offenders.push({ file: file.replace(SRC_ROOT, "src"), matches: found });
      }
    }
    expect(
      offenders,
      `Banned palette utilities still in source:\n${offenders
        .map((o) => `  - ${o.file}: ${o.matches.join(", ")}`)
        .join("\n")}`,
    ).toEqual([]);
  });
});
```

- [ ] **Step 2: Run the canary test (should still RED at the globals.css assertion until Step 3)**

```bash
cd /Users/notkanishk/Documents/Mkrspc/Projects/Syndra/ui
bun run test src/__tests__/no-legacy-tokens.test.ts
```

Expected: the **first** test ("globals.css has no legacy compatibility token aliases") FAILS because the legacy block still exists. The **second** test ("no source file uses a banned palette utility class") PASSES because Tasks 2–14 swept the source tree.

If the second test fails, list the offenders and return to the named tasks to migrate the missed lines before proceeding.

- [ ] **Step 3: Delete the legacy compatibility tokens block from `globals.css`**

The pre-edit state at this point is the original "Legacy compatibility tokens" block PLUS the three `--color-on-success` / `--color-on-warning` / `--color-on-info` mappings appended at the end by Task 1 Step 3. The replacement preserves the six existing status mappings AND the three new on-* mappings under one accurate comment, and drops the seven legacy aliases plus the obsolete comment header.

Edit `ui/src/app/globals.css` lines 54-72 (the "Legacy compatibility tokens" block plus the three on-* mappings Task 1 appended).

```css
// Before (lines 54-72 — original legacy block + Task 1's three appended on-* mappings)
  /* Legacy compatibility tokens — kept so existing utilities like
     bg-surface, border-border, text-foreground continue to resolve while
     pages migrate to the new ladder. Each maps to a Clarity equivalent. */
  --color-foreground: var(--on-surface);
  --color-surface-hover: var(--surface-container-high);
  --color-border: var(--outline-variant);
  --color-primary-hover: var(--primary-container);
  --color-muted: var(--on-surface-variant);
  --color-success: var(--success);
  --color-success-hover: var(--success-hover);
  --color-warning: var(--warning);
  --color-warning-hover: var(--warning-hover);
  --color-danger: var(--error);
  --color-danger-hover: var(--error-container);
  --color-info: var(--info);
  --color-info-hover: var(--info-hover);
  --color-on-success: var(--on-success);
  --color-on-warning: var(--on-warning);
  --color-on-info: var(--on-info);
// After (replace those 19 lines with one unified semantic-status block; both bg-* and on-* mappings survive)
  /* Semantic status tokens — no MD3 baseline equivalent; kept for
     <Pulse>, <Badge>, SubmitButton variants, Button success/warning
     variants, and graph/page.tsx + JsonView tones. The on-* foreground
     pair (added in Task 1) survives this deletion because it is a
     semantic status mapping, not a legacy alias. */
  --color-success: var(--success);
  --color-success-hover: var(--success-hover);
  --color-warning: var(--warning);
  --color-warning-hover: var(--warning-hover);
  --color-info: var(--info);
  --color-info-hover: var(--info-hover);
  --color-on-success: var(--on-success);
  --color-on-warning: var(--on-warning);
  --color-on-info: var(--on-info);
```

(Drops the seven legacy aliases — `--color-foreground`, `--color-surface-hover`, `--color-border`, `--color-primary-hover`, `--color-muted`, `--color-danger`, `--color-danger-hover` — and the obsolete comment. Keeps the six status background/hover mappings AND the three on-* foreground mappings under one accurate comment.)

- [ ] **Step 4: Run the canary test — must be all green**

```bash
bun run test src/__tests__/no-legacy-tokens.test.ts
```

Expected: both tests pass.

- [ ] **Step 5: Run the full UI test suite**

```bash
bun run test
```

Expected: all tests green (baseline 73/73 + the 2 new canary tests = 75/75 across 17 files). If any existing test fails, the snapshot or render expectation may have been built against the now-removed alias name; investigate before committing.

- [ ] **Step 6: Build the UI**

```bash
bun run build
```

Expected: build clean. Tailwind v4 will fail loudly on any unresolved utility — a green build proves no banned utility survived in source.

- [ ] **Step 7: Commit**

```bash
git add src/__tests__/no-legacy-tokens.test.ts src/app/globals.css
git commit -m "refactor(ui): delete legacy palette aliases from globals.css; add canary test

The 'Legacy compatibility tokens' block in globals.css served as a
backdoor for unmigrated surfaces. With every call site now MD3-native,
the seven legacy aliases (foreground, surface-hover, border,
primary-hover, muted, danger, danger-hover) are removed. The semantic
status tokens (success, warning, info + hover pairs) stay — they have
no MD3 baseline.

A new canary test enforces the contract going forward: globals.css MUST
NOT redeclare the legacy tokens, and no source file MAY use any of the
banned palette utilities.

Closes audit refs U2 + D10 from the May 2026 audit (wave-2-part-1)."
```

---

## Task 16: Spec deltas wired + INDEX update + obsidian-clarity Stage 5 pointer

The spec delta and proposal/design/tasks were committed in Task 0. This task wires the change into the master index and adds the Stage 5 pointer the design doc asks for.

**Files:**
- Modify: `openspec/INDEX.md`
- Modify: `openspec/changes/obsidian-clarity-redesign/tasks.md`

- [ ] **Step 1: Add the new change to `openspec/INDEX.md` Change Log table**

Insert a new row immediately after the `wave-1-production-trust-hardening` row (around line 51 today):

```markdown
| [Wave 2 · Part 1 — Frontend Palette Finalization](changes/wave-2-part-1-frontend-palette-finalization/) | 5.5 | In progress | [proposal](changes/wave-2-part-1-frontend-palette-finalization/proposal.md) / [design](changes/wave-2-part-1-frontend-palette-finalization/design.md) / [tasks](changes/wave-2-part-1-frontend-palette-finalization/tasks.md) |
```

Also update the Phase 5.5 row in the "Roadmap Phase -> Change Mapping" table to add `wave-2-part-1-frontend-palette-finalization` next to `wave-1-production-trust-hardening`.

- [ ] **Step 2: Append a Stage 5 pointer to `obsidian-clarity-redesign/tasks.md`**

Append after line 143 (the existing OCR-S4-15 line):

```markdown

## Stage 5 — Palette Finalization (handoff)

- [x] OCR-S5-01 Palette finalization handed to [`wave-2-part-1-frontend-palette-finalization`](../wave-2-part-1-frontend-palette-finalization/) (May 2026 audit refs U2 + D10). Sidebar, ThemeToggle, RequestAccessButton, ErrorBoundary, the legacy CRUD parts of `app/zitadel/page.tsx`, plus the long-tail call sites (`app/page.tsx`, `SidebarNav`, `ui/{CopyButton,Skeleton,SubmitButton,JsonView}`) migrate there. The "Legacy compatibility tokens" block in `globals.css` is deleted there. See that change for completion.
```

- [ ] **Step 3: Validate the OpenSpec change is still strict-clean**

```bash
cd /Users/notkanishk/Documents/Mkrspc/Projects/Syndra
openspec validate wave-2-part-1-frontend-palette-finalization --strict
```

Expected: `Change 'wave-2-part-1-frontend-palette-finalization' is valid`.

- [ ] **Step 4: Commit**

```bash
git add openspec/INDEX.md openspec/changes/obsidian-clarity-redesign/tasks.md
git commit -m "docs(openspec): wire wave-2-part-1 into INDEX; add obsidian-clarity Stage 5 pointer

INDEX gains the wave-2-part-1 change-log row under Phase 5.5.
obsidian-clarity-redesign/tasks.md gains a Stage 5 pointer that hands
palette finalization to the new change, per the May 2026 audit's
'move corresponding tasks to a Stage-2 list and check them off here'
instruction."
```

---

## Task 17: Final verification

End-to-end gates: lint, test, build, backend regression, codebase-memory graph refresh, manual dev-server walk-through, OpenSpec validation.

- [ ] **Step 1: UI lint**

```bash
cd /Users/notkanishk/Documents/Mkrspc/Projects/Syndra/ui
bun run lint
```

Expected: no new findings. If any pre-existing warning surfaces, document but do not fix in this change unless caused by the migration.

- [ ] **Step 2: UI test suite (full)**

```bash
bun run test
```

Expected: 75/75 across 17 files (baseline 73/73 + 2 new canary tests).

- [ ] **Step 3: UI production build**

```bash
bun run build
```

Expected: clean build. Bundle sizes for `/`, `/zitadel`, and `/login` may shrink slightly because the legacy alias declarations no longer ship in the CSS bundle.

- [ ] **Step 4: Backend regression backstop**

```bash
cd /Users/notkanishk/Documents/Mkrspc/Projects/Syndra/backend
go vet ./... && go test ./...
```

Expected: clean. This change touches no Go code; the run is a sanity check.

- [ ] **Step 5: Codebase-memory graph refresh**

```bash
# Run via the codebase-memory MCP tools, not bash:
#   mcp__codebase-memory-mcp__detect_changes(project="Users-notkanishk-Documents-Mkrspc-Projects-Syndra")
```

Expected: detect_changes acknowledges the modified frontend files. The migration is structural-token-level, so the graph (which indexes function/class structure, not Tailwind utility usage) should report only the small changes in `RequestAccessButton.tsx` (Modal composition) and `Sidebar.tsx` (new `Pulse` import). No symbol additions/removals elsewhere.

- [ ] **Step 6: OpenSpec validate**

```bash
cd /Users/notkanishk/Documents/Mkrspc/Projects/Syndra
openspec validate wave-2-part-1-frontend-palette-finalization --strict
```

Expected: `Change 'wave-2-part-1-frontend-palette-finalization' is valid`.

- [ ] **Step 7: Manual dev-server walk-through**

Start the dev server:

```bash
cd /Users/notkanishk/Documents/Mkrspc/Projects/Syndra/ui
bun run dev
```

Walk these surfaces in a browser at `http://localhost:3000`:

| Surface | What to check |
|---|---|
| **Sidebar (admin, dark theme)** | Active nav highlight (primary-container background, on-primary-container text), inactive hover state, eyebrow section labels, sign-out button border + hover, LXC live dot pulses steadily green, theme toggle border + hover. |
| **Sidebar (admin, light theme)** | Same as above; visual contrast verified on warm-white surfaces. |
| **Sidebar (member, both themes)** | Reduced section list (`Portal` / `Access`); same hover treatment as admin. |
| **Member home (`/`)** | Welcome heading, three identity/active/pending cards, service-catalog tiles (border + background reset to `bg-surface-container-high`), Request Access button. |
| **Request Access modal** | Click "Request Access" on a "No Access" tile — modal opens via `<Modal>`. Tab cycles through textarea → duration pills → Cancel → Submit. Esc dismisses. Click outside dismisses. Selected duration pill uses `bg-primary-container text-on-primary-container`. |
| **ErrorBoundary** | Force a child component to throw (e.g. temporarily edit a page to throw on render). Confirm the alert renders with `border-error/40 bg-error-container/40`, "Try again" button uses `bg-primary text-on-primary`. Revert the temporary throw. |
| **Zitadel diagnostics (`/zitadel`)** | Rotation Refresh button hover; Health "Run health" button; Projects role list (border + bg-surface-container-high); Users grant list; AllGrants table border. Revoke button (destructive) shows `border-error/40 text-error` with `hover:bg-error-container/40`. Inline error message tone uses `text-error`. |
| **SubmitButton variants** | On any create flow (e.g. `/bundles` Create Bundle modal), confirm primary button now has a visible hover state (was the typo-broken no-op). Danger variant on a destructive flow (if reachable) tones through error. |

If any surface looks visually wrong (colors swapped, contrast lost, hover missing), capture the screenshot, identify the offending class, and add a follow-up commit before proceeding.

Stop the dev server (`Ctrl+C`) when done.

- [ ] **Step 8: Push the branch + open PR**

```bash
git push -u origin <prefix>/wave-2-part-1-frontend-palette-finalization
gh pr create --title "Wave 2 · Part 1 — Frontend Palette Finalization (audit U2 + D10)" --body "$(cat <<'EOF'
## Summary
- Migrates every remaining legacy-palette-alias call site (`Sidebar`, `SidebarNav`, `ThemeToggle`, `RequestAccessButton`, `ErrorBoundary`, `app/page.tsx`, `app/graph/page.tsx`, `app/zitadel/page.tsx` legacy CRUD, `ui/{CopyButton,Skeleton,SubmitButton,JsonView}`) to canonical Material 3 + semantic-status tokens.
- Deletes the "Legacy compatibility tokens" block from `globals.css` and locks the contract via a new `no-legacy-tokens.test.ts` canary that scans `globals.css` and the source tree on every `bun run test`.
- Folds `RequestAccessButton`'s hand-rolled focus-trap dialog into the canonical `<Modal>` primitive.
- Fixes the `bg-primaryHover` (camelCase, Tailwind no-op) typo in `SubmitButton`, restoring a visible hover state for primary buttons project-wide.
- Migrates the holdouts: `JsonView` syntax tones (amber → warning, emerald → success, sky → info) and `graph/page.tsx` `nodeTone()` (application → info, bundle → warning, role → success) — no design-system carve-outs remain.

Implements [May 2026 audit-resolution](docs/superpowers/specs/2026-05-08-may-2026-audit-resolution-design.md) §3 Theme 4 palette portion (audit refs **U2 + D10**). Wave 2 Part 1 of 4.

## Test plan
- [ ] `bun run lint && bun run test && bun run build` clean (baseline +2 canary tests).
- [ ] Backend regression `go vet ./... && go test ./...` clean.
- [ ] Manual walk: sidebar both themes, member home, Request Access modal a11y, ErrorBoundary, /graph topology canvas (node tones), /zitadel CRUD sections including Revoke.
- [ ] `openspec validate wave-2-part-1-frontend-palette-finalization --strict` valid.
EOF
)"
```

(Pause for the user to confirm the PR is desired before pushing.)

---

## Self-Review

After saving this plan, the author cross-checked:

**Spec coverage:** Every line item from the audit's Theme 4 palette portion (U2 + D10 plus the in-scope hardcoded color sweeps, the graph/page.tsx + JsonView syntax-tone migrations, and the new on-* foreground tokens needed to close the WCAG contrast bug) maps to a task: on-* token additions → Task 1; UI primitive migrations → Tasks 2–5; flat-leaf primitive migrations → Tasks 6–11; page-level migrations → Tasks 12–14; "Delete the legacy palette tokens from globals.css" → Task 15; "Move corresponding tasks in obsidian-clarity-redesign/tasks.md to a Stage-2 list and check them off here" → Task 16 Step 2; D10 (= U2) → covered by the same set.

**Placeholder scan:** No "TBD", no "implement later", no "add appropriate error handling", no "similar to Task N", no untyped references. Every code block contains the exact final content.

**Type consistency:** `<Modal>` props in Task 10 are described as `open`/`onClose`/`labelledBy` with a Step 3 verification that confirms the actual prop shape before commit (the rewrite adapts to whatever `Modal.tsx` exports). `<Pulse static />` in Task 9 matches the OCR-S1-24 specification (`pulse-dot-static` utility, `currentColor` fill, variant via parent text color). The `text-on-success` / `text-on-warning` utilities consumed in Tasks 4, 5, and 14 are defined in Task 1; the dependency is acyclic. The canary test in Task 15 uses `vitest`'s `describe`/`it`/`expect` already present in the project's existing tests.

**Plan length:** 18 tasks (Task 0 scaffolding + Tasks 1–17) across 13 file migrations (12 source files + 2 `globals.css` edits) + 1 spec/index update + 1 verification sweep. Each task has 3–12 atomic steps in the 2–5 minute range.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-08-wave-2-part-1-frontend-palette-finalization.md`. Two execution options:

1. **Subagent-Driven (recommended)** — Dispatch a fresh subagent per task, review between tasks, fast iteration. Each migration commit is independently reviewable; the canary test in Task 15 catches any drift.
2. **Inline Execution** — Execute tasks in this session using `superpowers:executing-plans`, with checkpoints after every commit (or batched per major file).

Which approach?
