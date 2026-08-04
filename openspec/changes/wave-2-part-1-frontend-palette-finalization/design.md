# Wave 2 · Part 1 — Design

This design defers to the [May 2026 meta-spec](../../../docs/superpowers/specs/2026-05-08-may-2026-audit-resolution-design.md) §3 Theme 4 and §6 (sequencing) for cross-cutting structure. The detailed step-by-step execution plan lives at [`docs/superpowers/plans/2026-05-08-wave-2-part-1-frontend-palette-finalization.md`](../../../docs/superpowers/plans/2026-05-08-wave-2-part-1-frontend-palette-finalization.md).

---

## 1. Why this lands first in Wave 2

The four Wave 2 parts share one ordering edge: the drift UI in Part 4 mounts surfaces on **every** admin page (sticky banner) and inside the sidebar (`⚠ Drift` nav item with red count badge). Building those surfaces atop the legacy palette alias block would either:

1. lock the legacy block in for another wave (defeats the audit's "delete legacy palette tokens from `globals.css`" intent), or
2. force a rewrite of the drift UI a wave later (cost the audit warned against in §6.1).

Shipping the palette migration first removes the alias block, gives Part 4 a single-token-vocabulary substrate, and means every drift surface is born MD3-native. This part is parallelizable with Theme 3 (Part 2) and Theme 5 (Part 3), with no shared edits.

---

## 2. Token migration table

`globals.css` defines two parallel ladders today: the **MD3 ladder** (`--color-surface`, `--color-on-surface`, `--color-surface-container-{lowest,low,medium,high,highest}`, `--color-primary*`, `--color-secondary*`, `--color-tertiary*`, `--color-outline*`, `--color-error*`) and a **legacy alias ladder** that shadows MD3 with shorter names. Every utility in the second ladder maps 1:1 to a utility in the first. The migration is a literal find-and-replace per the table below; no visual treatment changes.

### 2.1 Legacy alias deletions (these utility classes disappear from the codebase)

| Legacy utility | MD3 replacement | Maps to (CSS variable) |
|---|---|---|
| `bg-surface-hover`, `hover:bg-surface-hover` | `bg-surface-container-high`, `hover:bg-surface-container-high` | `--surface-container-high` |
| `bg-surfaceHover`, `hover:bg-surfaceHover` (camelCase typo — Tailwind silently no-ops) | `bg-surface-container-high`, `hover:bg-surface-container-high` | `--surface-container-high` |
| `border-border` | `border-outline-variant` | `--outline-variant` |
| `text-foreground` | `text-on-surface` | `--on-surface` |
| `text-muted`, `hover:text-muted` | `text-on-surface-variant`, `hover:text-on-surface-variant` | `--on-surface-variant` |
| `hover:text-foreground` | `hover:text-on-surface` | `--on-surface` |
| `bg-primary-hover`, `hover:bg-primary-hover` | `bg-primary-container`, `hover:bg-primary-container` | `--primary-container` |
| `bg-primaryHover` (camelCase typo, currently no-op in Tailwind) | `bg-primary-container` | `--primary-container` |
| `bg-danger` | `bg-error` | `--error` |
| `text-danger` | `text-error` | `--error` |
| `bg-danger-hover` | `bg-error-container` | `--error-container` |

### 2.2 Hardcoded core-Tailwind color migrations (in-scope files only)

| Hardcoded utility | Used in | MD3 replacement | Rationale |
|---|---|---|---|
| `bg-emerald-500` (small live dot) | `Sidebar.tsx:39` | `<Pulse variant="success" static />` | Replace with the Stage-1 Pulse primitive — already the project's standard for live indicators. |
| `border-red-500/40 bg-red-500/5` | `ErrorBoundary.tsx:38` | `border border-error/40 bg-error-container/40` | Semantic error alert; route through error tokens. |
| `bg-red-500 hover:bg-red-600 text-white` (danger button) | `SubmitButton.tsx:33` | `bg-error hover:bg-error/90 text-on-error` | MD3 destructive button. |
| `bg-primary hover:bg-primaryHover text-white` (primary button) | `SubmitButton.tsx:36` | `bg-primary hover:bg-primary-container text-on-primary` | Fixes the `bg-primaryHover` typo; uses canonical primary container on hover. |
| `border-red-500/40 text-red-400 hover:bg-red-500/10` (revoke button) | `app/zitadel/page.tsx:554, 814` | `border-error/40 text-error hover:bg-error-container/40` | Destructive control on a list row. |
| `bg-primary` (member portal button) | `RequestAccessButton.tsx:85,132` | `bg-primary text-on-primary` | Already MD3; just makes on-primary text token explicit. |
| `bg-primary/10 text-primary` (active nav state) | `SidebarNav.tsx:112` | `bg-primary-container text-on-primary-container` | MD3 equivalent for accented selection state. |
| `border-primary bg-primary/10 text-primary` (selected duration pill) | `RequestAccessButton.tsx:184` | `border-primary bg-primary-container text-on-primary-container` | Same pattern as active nav. |
| `hover:border-primary/40` | multiple files | `hover:border-primary` | Tailwind opacity drops to plain border. |
| `text-amber-500` (null literal + diff highlight) | `JsonView.tsx:46, 54, 113, 115` | `text-warning` | Diff highlight is semantically "needs attention"; null shares the tone. |
| `text-emerald-500` (string values) | `JsonView.tsx:54, 115` | `text-success` | Visual character preserved; success token in dark mode resolves near emerald-500. |
| `text-sky-500` (number + boolean values) | `JsonView.tsx:67, 116` | `text-info` | Info token in dark mode resolves near sky-500. |
| `border-sky-500/30 bg-sky-500/8 text-sky-600 dark:text-sky-300` (application node) | `graph/page.tsx:42` | `border-info/30 bg-info/8 text-info` | Info token already encodes dark-mode variant; drop the explicit `dark:` override. |
| `border-amber-500/30 bg-amber-500/8 text-amber-600 dark:text-amber-300` (bundle node) | `graph/page.tsx:44` | `border-warning/30 bg-warning/8 text-warning` | Same shape; warning token. |
| `border-emerald-500/30 bg-emerald-500/8 text-emerald-600 dark:text-emerald-300` (role node) | `graph/page.tsx:48` | `border-success/30 bg-success/8 text-success` | Same shape; success token. |
| `text-red-400` (inline error + flash error text) | `app/zitadel/page.tsx:300,376,377,597,855,912` | `text-error` | Six sites; semantically all are error messages. |
| `text-emerald-400` (flash success text in success/error ternary) | `app/zitadel/page.tsx:597,855` | `text-success` | Two ternary branches alongside the matching `text-red-400` → `text-error` replacement. |
| `bg-primary … text-white` (primary CTA buttons) | `app/zitadel/page.tsx:365,538,588,795,846,905` | `bg-primary … text-on-primary` | Six sites across Health, role-edit Save, role-add, grant-edit Save, grant-assign, and Users sections. The `text-on-primary` token already exists; this just removes the hardcoded white. |
| `bg-[var(--success)] text-white hover:bg-[var(--success-hover)]` (success button variant) | `ui/Button.tsx:72` | `bg-success text-on-success hover:bg-success-hover` | Drops the `var(...)` indirection (the @theme `bg-success` utility resolves to the same value) and pairs with the new `--on-success` token. |
| `bg-[var(--warning)] text-white hover:bg-[var(--warning-hover)]` (warning button variant) | `ui/Button.tsx:73` | `bg-warning text-on-warning hover:bg-warning-hover` | Same shape; warning token pair. |
| `bg-emerald-500 hover:bg-emerald-600 text-white` (SubmitButton success variant) | `ui/SubmitButton.tsx:35` | `bg-success hover:bg-success-hover text-on-success` | Uses the new `--on-success` pair, replacing the hardcoded white. |

### 2.3 New `on-*` foreground tokens

The migration adds three CSS variables to `globals.css` so chromatic-background buttons can pair with a properly-themed foreground. Light-theme `on-*` values land at `#ffffff` (the bg-* tokens are dark enough to take white text at AA contrast). Dark-theme `on-*` values are deeper same-family colors that pass WCAG AA at 4.5:1 against the corresponding light bg-* values:

```css
/* Light theme additions */
--on-success: #ffffff;
--on-warning: #ffffff;
--on-info:    #ffffff;

/* Dark theme additions */
--on-success: #003822;  /* deep forest green; AA against #34d399 success */
--on-warning: #2a1800;  /* deep amber-brown; AA against #fbbf24 warning */
--on-info:    #001a4a;  /* deep navy; AA against #60a5fa info */
```

And the `@theme` block gains the corresponding `--color-on-success`, `--color-on-warning`, `--color-on-info` mappings so the Tailwind utilities (`text-on-success`, `bg-on-success`, etc.) resolve through the standard token pipeline.

The pre-migration code shipped `text-white` against all three dark-mode chromatic backgrounds (mint `#34d399`, amber `#fbbf24`, sky `#60a5fa`). Each of these pairs measures ~1.4-2.0 contrast ratio — a WCAG AA failure. The new tokens close that bug at the same time as the spec contract.

### 2.4 Tokens that stay (NOT touched)

- `--color-success` / `--color-success-hover` — semantic status; no MD3 equivalent. Used by `<Pulse variant="success" />`, the SubmitButton success variant, the migrated `JsonView` string tone, and the migrated `graph/page.tsx` role-node tone.
- `--color-warning` / `--color-warning-hover` — semantic status. Used by the migrated `JsonView` null/diff tone and the migrated `graph/page.tsx` bundle-node tone.
- `--color-info` / `--color-info-hover` — semantic status. Used by the migrated `JsonView` number/boolean tone and the migrated `graph/page.tsx` application-node tone.

These three semantic-status pairs now serve dual duty: the original status-indicator role plus the syntax-highlight / graph-node visual encoding. The dual use is intentional and documented — the visual character is preserved, the design system gains a single token vocabulary, and the `dark:text-*-300` per-utility overrides are eliminated because the tokens already auto-flip between light and dark themes via the `@theme` block.

---

## 3. File scope and migration order

Files are migrated leaf-first (UI primitives) before consumers (pages and shells). This keeps each commit independently green: a primitive migration cannot break a page that uses the primitive, because both ends of the call already accept MD3 tokens. The two `globals.css` edits straddle the migration — the additive `on-*` token block lands FIRST so every subsequent file may use the new utilities, and the legacy alias deletion lands LAST so the canary fires green on a fully-swept tree.

```
Order   File                                              Lines  Token surfaces touched
─────   ────────────────────────────────────────────────  ─────  ────────────────────────
 1      ui/src/app/globals.css (ADD on-* tokens)             313  Adds --on-success/--on-warning/--on-info to :root, [data-theme="dark"], and @theme; no deletions
 2      ui/src/components/ui/Skeleton.tsx                    36  bg-surfaceHover, border-border
 3      ui/src/components/ui/CopyButton.tsx                   52  border-border, text-muted, hover:text-foreground, hover:border-primary/40
 4      ui/src/components/ui/SubmitButton.tsx                 57  bg-red-500/600, bg-emerald-500/600, bg-primary hover:bg-primaryHover (BUG); success → text-on-success
 5      ui/src/components/ui/Button.tsx                     ~140  success/warning variants: bg-[var(--success)] text-white → bg-success text-on-success; same for warning
 6      ui/src/components/ui/JsonView.tsx                    140  text-muted (×2); text-amber-500, text-emerald-500, text-sky-500 (syntax-highlight tones)
 7      ui/src/components/ThemeToggle.tsx                     50  border-border, text-muted, hover:text-foreground, hover:border-primary/40
 8      ui/src/components/SidebarNav.tsx                     130  text-muted, bg-primary/10, hover:bg-surfaceHover, border-l-2 border-primary
 9      ui/src/components/Sidebar.tsx                         48  bg-surface, border-border, text-foreground, bg-surfaceHover, text-muted, bg-emerald-500
 10     ui/src/components/RequestAccessButton.tsx            217  border-border, bg-surface, text-foreground, text-muted, bg-primary, hover:bg-surfaceHover; +Modal wrap
 11     ui/src/components/ErrorBoundary.tsx                   58  border-red-500/40 bg-red-500/5, text-foreground, text-muted, bg-primary
 12     ui/src/app/page.tsx                                  118  bg-surfaceHover, border-border, text-foreground, text-muted
 13     ui/src/app/graph/page.tsx (nodeTone() only)          ~330  sky-500/30, sky-500/8, sky-600, dark:sky-300 (and amber, emerald shapes)
 14     ui/src/app/zitadel/page.tsx (legacy CRUD sections)   946  bg-surfaceHover, border-border, text-foreground, text-muted, text-red-400 (×6), text-emerald-400 (×2), text-white on bg-primary (×6), border-red-500/40, hover:bg-red-500/10
 15     ui/src/app/globals.css (DELETE legacy block)         313  Removes the OCR-S1-05 "Legacy compatibility tokens" block; ships with canary
```

---

## 4. RequestAccessButton modal contract harmonization

The current `RequestAccessButton` re-implements focus management and Esc/click-outside dismiss inline (`useRef`, `useEffect`, `addEventListener("keydown")`). OCR-S1-16 introduced `<Modal>` as the canonical primitive with focus trap, `aria-modal="true"`, Esc, and click-outside semantics. Stage 3 task OCR-S3-04 wrapped the create-rule form in `<Modal>` for the same reason.

This change folds the same harmonization into the palette migration, since the file is being touched anyway: replace the hand-rolled dialog with `<Modal>` and let the existing Modal a11y test (`Modal.test.tsx` from OCR-S1-16) cover the focus-trap contract instead of duplicating coverage. No prop-shape change for callers; component remains a default export with `(projectId, serviceName, status)` props.

The `<SubmitButton>` import is removed in favor of plain `<Button variant="primary">` because the duration picker handles primary submission and the existing `SubmitButton` is the only file in the codebase that mixes pending-state and palette concerns. (Removing the file is out of scope; only swapping its consumer here.)

Actually, on closer reading, `SubmitButton` is consumed in places this change does not touch (all `Create*Modal.tsx` flows). Per audit norms, primitive substitutions are out of scope unless required. Decision: keep `<SubmitButton>` as the submit affordance inside `RequestAccessButton`'s migrated `<Modal>`. Only the palette tokens inside `SubmitButton` itself change.

---

## 5. Validation strategy

### 5.1 Static canary test

A new `ui/src/__tests__/no-legacy-tokens.test.ts` enforces the contract going forward. The implementation plan ([Task 15](../../../docs/superpowers/plans/2026-05-08-wave-2-part-1-frontend-palette-finalization.md)) carries the canonical `BANNED_UTILITIES` and `BANNED_CSS_TOKENS` arrays — that file is the **single source of truth** for the banned list; this design only describes the categories so the doc does not drift each time the arrays change.

- Reads `ui/src/app/globals.css` and asserts the seven legacy compatibility CSS tokens (`--color-foreground`, `--color-surface-hover`, `--color-border`, `--color-primary-hover`, `--color-muted`, `--color-danger`, `--color-danger-hover`) and the `Legacy compatibility tokens` comment header are absent.
- Walks `ui/src/` (excluding `__tests__`, `node_modules`, `.next`, build artifacts) and asserts no source file contains any banned utility substring. The banned list covers four categories:
  - **Legacy palette aliases** — `bg-surfaceHover`, `bg-surface-hover`, `border-border`, `text-foreground`, `text-muted`, `bg-primary-hover`, `bg-primaryHover`, `bg-danger`, `text-danger`, `bg-danger-hover`, plus the `hover:` and `hover:bg-` variants of the same set.
  - **JsonView hardcoded syntax tones** — `text-amber-500`, `text-emerald-500`, `text-sky-500` (migrated to `text-warning`/`text-success`/`text-info`).
  - **graph/page.tsx hardcoded `nodeTone()` substrings** — `border-{amber,emerald,sky}-500`, `bg-{amber,emerald,sky}-500`, `text-{amber,emerald,sky}-600`, and `dark:text-{amber,emerald,sky}-300` (migrated to the corresponding `border-success/30 bg-success/8 text-success` / `info` / `warning` shape, with the `dark:` overrides dropped because `@theme` auto-flips).
  - **zitadel/page.tsx flash and error tones** — `text-red-400`, `text-emerald-400` (migrated to `text-error` / `text-success`). The accompanying `bg-primary … text-white` button pairs in the same file (six sites) migrate to `bg-primary text-on-primary`; `text-white` is NOT added to the canary because it is a neutral utility legitimately used outside this change's scope, but every chromatic-background pairing in the files this change touches now uses the correct `text-on-*` token. A future audit MAY tighten this further once the codebase eliminates `text-white` from all chromatic pairings.
- The test ships in the same commit as the `globals.css` legacy-block delete (Task 15 in the plan), so the green build proves the global swap.

### 5.2 Component snapshot tests stay valid

The migration is purely token-level. Each migrated file's existing snapshot/render tests in `__tests__/` remain green if the visual output is identical. The MD3 → legacy alias map in `globals.css` resolves to the same hex values today, so post-migration utilities produce identical CSS output. Any snapshot drift is a real visual change and must be investigated before commit.

### 5.3 Manual visual sanity (per `superpowers:verification-before-completion`)

After the full migration lands, a single dev-server walk-through covers:
- Sidebar (light + dark theme, signed in as admin and member) — active nav highlight, hover state, LXC live dot, Sign-out button hover.
- Member home (`/`) — service catalog cards, identity card, Request Access button.
- Request Access modal — opens, focus-trap, Esc dismiss, click-outside dismiss, duration pills, submit pending state.
- ErrorBoundary — force-render a throwing child to verify the alert tone.
- Zitadel diagnostics (`/zitadel`) — Rotation, Health, Projects, Users, AllGrants sections, including destructive Revoke buttons.

The plan documents the exact dev-server commands and the surfaces to inspect.

### 5.4 CI gates

- `cd ui && bun run lint` — no new ESLint findings.
- `cd ui && bun run test` — all existing tests green; new `no-legacy-tokens.test.ts` green.
- `cd ui && bun run build` — production build clean.
- `cd backend && go vet ./... && go test ./...` — unchanged (this part touches no Go code), but run as a regression backstop.

---

## 6. Spec, index, and cross-change updates

| Document | Edit |
|---|---|
| `openspec/changes/wave-2-part-1-frontend-palette-finalization/specs/operational-readiness/spec.md` | New delta: `Requirement: Frontend MUST NOT carry legacy palette aliases` with two scenarios. Already wired by this change directory. |
| `openspec/changes/obsidian-clarity-redesign/tasks.md` | Append a "Stage 5 — palette finalization" pointer block under OCR-S4-15: `[x] OCR-S5-01 Palette finalization handed to wave-2-part-1-frontend-palette-finalization (see that change for completion).` Tracks the audit's "Move corresponding tasks ... to a Stage-2 list and check them off here" instruction without duplicating progress. |
| `openspec/INDEX.md` | New change-log row for `wave-2-part-1-frontend-palette-finalization` under Phase 5.5; status `In progress` while implementing, `Complete` after archive. |
| `openspec/changes/syndra-core-architecture/specs/feature-coverage.md` | Optional: add a one-line note under the Operational Readiness row that the palette migration is finalized. Skip if it would duplicate the existing "Integrated, … theme toggle …" note. |

---

## 7. Risk and rollback

- **Risk:** A migrated utility renders to a slightly different hex value than its alias did, producing a subtle visual change. **Mitigation:** the alias block in `globals.css` literally maps each legacy utility to a single MD3 utility; CSS output before/after is byte-identical. Snapshot tests catch any divergence.
- **Risk:** A migrated file imports from a path the test grep does not see (e.g. minified third-party file, generated `.next/` artifact). **Mitigation:** the canary test scopes to `ui/src/` and explicitly excludes `__tests__`, `node_modules`, `.next`, and dist artifacts.
- **Risk:** SubmitButton typo fix changes the visual hover behavior for primary buttons (which today silently have no hover background due to the camelCase typo). **Mitigation:** this is the desired fix — primary buttons SHOULD have a visible hover state. Verified in the dev-server walk-through.
- **Rollback:** `git revert` of the migration commits. The legacy alias block in `globals.css` is restored verbatim. No data, schema, or backend contract is touched, so revert is a no-op for live operators.
