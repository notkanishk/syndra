# Tasks

## 1. Cross-cutting UX foundations (Track D)

- [x] 1.1 Install `sonner`. Mount `<Toaster richColors closeButton position="bottom-right" />` in `app/layout.tsx`. Wrap with semantic helpers in `lib/toast.ts`.
- [x] 1.2 Replace every `setMessage(...)` pattern with `toastSuccess`/`toastError` across users, bundles, policies, requests admin, requests user.
- [x] 1.3 New `<ErrorBoundary>` (class component) wrapping `<main>` in the layout.
- [x] 1.4 New `<SubmitButton>` primitive with built-in spinner + `aria-busy`. Apply to all create/update forms.
- [x] 1.5 New `<ConfirmModal>` primitive: focus trap, Esc/click-outside cancel, destructive variant, busy support. Wire into delete role, revoke grant, reject request, bundle assignment.
- [x] 1.6 New `<Skeleton>` + `<SkeletonCardList>` primitives. Replace bare "Loading…" text across users/bundles/projects/applications/policies/audit/graph.
- [x] 1.7 Split sidebar: outer `<Sidebar>` server component + new client `<SidebarNav>` with `usePathname()` for `aria-current="page"` and bg/border highlight. Activity badges from `/api/proxy/governance/summary`.
- [x] 1.8 Theme system: `<ThemeProvider>` + `useTheme()` hook in `lib/theme.tsx` driving `data-theme` attribute on `<html>`; `<ThemeToggle>` icon button mounted in sidebar footer; `globals.css` reorganized with explicit token blocks per theme.
- [x] 1.9 Semantic accent tokens (`--color-success`/`-warning`/`-danger`/`-info`) added to `globals.css`. `<Badge>` extended with `success`/`warning`/`info` variants. New `<Button>` primitive with the same matrix.
- [x] 1.10 A11y pass: global `:focus-visible` outline rules; audit grid converted to semantic `<table>` with `<th scope="col">` + `<caption>`; aria-labels on icon-only buttons (theme toggle, copy, search, zoom).
- [x] 1.11 New `<CopyButton>` and `<JsonView>` primitives for copyable code surfaces.
- [x] 1.12 New `lib/useDebounce.ts` hook applied to users search and audit search.
- [x] 1.13 New `lib/format.ts` with `formatRoleRef`, `formatProjectName`, `humanizeKey`, `describeExpiry`. 12 new vitest cases covering all four helpers.

## 2. Design-promise gaps per view (Track C)

- [x] 2.1 **Access Lineage**: distinct color treatment Source vs Derived; inheritance arrow + tooltip; raw role pair as monospace tag.
- [x] 2.2 **Bundle preview**: role-count Badge + first-4 chips per bundle; ConfirmModal listing exact roles before assignment.
- [x] 2.3 **Token Simulator**: CopyButton, JsonView with key/value tokenization, "Compare with" select with side-by-side diff highlighting.
- [x] 2.4 **Project View**: per-role expandable rows with bundle/rule rollups computed client-side from `/bundles` + `/rules/mapping`.
- [x] 2.5 **Bundle View**: Affected Projects badge derived from bundleRoles; contained roles use human labels.
- [x] 2.6 **Governance urgency**: `describeExpiry` helper; sorted soonest-first; tone-coded urgency badges (critical/warning/neutral); cleanup-hints sub-state empty copy.
- [x] 2.7 **Friendly expiry picker**: button group (1 week / 1 month / 1 semester / Permanent) + custom-days input; live preview line; countdown badge in existing grants list.
- [x] 2.8 **Self-service request notes**: reviewer name + decision note in user request history; rejected status uses destructive Badge variant.
- [x] 2.9 **Inline request modal**: `<RequestAccessButton>` opens a focus-trapped modal on the member service catalog with justification + duration picker; toast feedback.
- [x] 2.10 **Mapping rule live preview**: `POST /api/v1/rules/mapping/validate` backend handler reusing `dbDetectCycleOnInsert`. `CreateRuleForm` debounces validation, shows live "IF X THEN ADD Y" preview, amber cycle warning, green safe confirmation; submit disabled on cycle.
- [x] 2.11 **Audit timeline**: day grouping (Today / Yesterday / weekday / absolute date); filter row (search + action category + actor); "Load more" cursor up to 200; filtered-empty state with Clear filters CTA.
- [x] 2.12 **Topology pan/zoom**: CSS-transformed canvas with mousedown drag pan, Cmd/Ctrl+wheel zoom (clamped 0.4x–2.5x); +/-/Reset overlay buttons + help hint; node buttons stop mousedown propagation. Inspector "View details →" link to detail page.
- [x] 2.13 **Microcopy**: `formatRoleRef` applied across users grant list, bundles preview, lineage display.

## 3. Backend additions

- [x] 3.1 `handlers/rules.go` — `handleValidateMappingRule` returning `{would_cycle, self_reference, reason?}` for partial-input-tolerant cycle checks. Wired in `router.go`.

## 4. OpenSpec deltas

- [x] 4.1 `specs/user-management/spec.md` — Access Lineage chain + bundle preview + friendly expiry picker.
- [x] 4.2 `specs/application-claims/spec.md` — Token Simulator copy/highlight/compare; reviewer notes in request history.
- [x] 4.3 `specs/role-management/spec.md` — per-role bundle/rule rollups in project view.
- [x] 4.4 `specs/automation-policies/spec.md` — live mapping-rule preview + cycle warning + impact endpoint.
- [x] 4.5 `specs/access-governance/spec.md` — urgency-tone expiring grants; audit timeline grouping + filters.
- [x] 4.6 `specs/service-catalog/spec.md` — inline modal Request Access flow.
- [x] 4.7 `specs/topology-graph/spec.md` — pan/zoom + node deeplinks.
- [x] 4.8 `specs/operational-readiness/spec.md` — toast, ConfirmModal, ErrorBoundary, theme toggle, sidebar activity badges, focus-visible.

## 5. Validation

- [x] 5.1 `cd backend && go vet ./... && go test ./...` — 272 tests pass.
- [x] 5.2 `cd ui && bun run test && bun run lint && bun run build` — 22 tests pass, lint clean, build clean.
- [x] 5.3 `openspec validate dashboard-ux-elevation --strict`.
- [ ] 5.4 Codebase memory: `mcp__codebase-memory-mcp__detect_changes` and re-index after merge.
