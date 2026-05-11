# Wave 2 · Part 1 — Tasks

> Detailed step-by-step (TDD-ordered, with exact code blocks and commands) lives at [`docs/superpowers/plans/2026-05-08-wave-2-part-1-frontend-palette-finalization.md`](../../../docs/superpowers/plans/2026-05-08-wave-2-part-1-frontend-palette-finalization.md). The list below is the OpenSpec progress ledger. Task numbering matches the plan.

- [x] **Task 0** — Commit OpenSpec scaffolding (this directory + spec delta + plan)
- [x] **Task 1** — Add `--on-success` / `--on-warning` / `--on-info` foreground tokens to `ui/src/app/globals.css` (light + dark theme + `@theme` block; additive — no deletions). Unblocks every subsequent chromatic-background migration.
- [x] **Task 2** — Migrate `ui/src/components/ui/Skeleton.tsx`
- [x] **Task 3** — Migrate `ui/src/components/ui/CopyButton.tsx`
- [x] **Task 4** — Migrate `ui/src/components/ui/SubmitButton.tsx` (also fixes `bg-primaryHover` typo; success variant uses `text-on-success`)
- [x] **Task 5** — Migrate `ui/src/components/ui/Button.tsx` (success/warning variants: drop `bg-[var(--success)] text-white` → `bg-success text-on-success`; same shape for warning)
- [x] **Task 6** — Migrate `ui/src/components/ui/JsonView.tsx` (`text-muted` separator + syntax-highlight tones: amber→warning, emerald→success, sky→info)
- [x] **Task 7** — Migrate `ui/src/components/ThemeToggle.tsx`
- [x] **Task 8** — Migrate `ui/src/components/SidebarNav.tsx`
- [x] **Task 9** — Migrate `ui/src/components/Sidebar.tsx` (LXC live dot → `<Pulse static />`)
- [x] **Task 10** — Migrate `ui/src/components/RequestAccessButton.tsx` (palette + `<Modal>` wrap)
- [x] **Task 11** — Migrate `ui/src/components/ErrorBoundary.tsx`
- [x] **Task 12** — Migrate `ui/src/app/page.tsx` (member home — Welcome panel + service catalog)
- [x] **Task 13** — Migrate `ui/src/app/graph/page.tsx` `nodeTone()` (application/bundle/role branches → info/warning/success; project branch already MD3)
- [x] **Task 14** — Migrate `ui/src/app/zitadel/page.tsx` legacy CRUD sections (Rotation, Health, Projects, Users, AllGrants) — extends to `text-red-400` (×6), `text-emerald-400` (×2), and `bg-primary text-white` CTA buttons (×6)
- [x] **Task 15** — Add `ui/src/__tests__/no-legacy-tokens.test.ts` canary + delete legacy compatibility token block from `ui/src/app/globals.css`; canary green
- [x] **Task 16** — INDEX update + obsidian-clarity Stage 5 pointer in `obsidian-clarity-redesign/tasks.md`
- [x] **Task 17** — Verification (`bun run lint && bun run test && bun run build`; `go vet ./... && go test ./...` regression backstop) + `mcp__codebase-memory-mcp__detect_changes` graph refresh + `openspec validate wave-2-part-1-frontend-palette-finalization --strict`. Manual dev-server walk-through deferred to user.
