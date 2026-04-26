## Why

`live-only-production-ui` closed the demo-leak surface and added empty-state coverage, but the UX audit (recorded in `/Users/notkanishk/.claude/plans/using-the-mandatory-workflow-cuddly-hejlsberg.md`) showed the dashboard was still functionally-but-not-elegantly delivering on the `mkauth-core-architecture/design.md` promises:

- **Access Lineage** rendered Source and Derived columns identically with no inheritance chain.
- **Bundle preview** was missing — admins clicked "Assign" without seeing the role list.
- **Token Simulator** was bare-text JSON, no copy, no compare.
- **Project View** showed totals only, no per-role bundle/rule rollups.
- **Governance** displayed raw expiry dates, no urgency tone.
- **Auto-expiring grants** had a bare number-input picker; no countdown.
- **Mapping rule creation** validated server-side only; no live preview, no cycle preview.
- **Member portal** required navigating away to request access.
- **Audit feed** was a flat 50-entry table with no filters or grouping.
- **Topology graph** had no pan/zoom and no deeplinks to detail pages.

Cross-cutting UX layer was missing: no toast system (silent failures), no confirmation modals on destructive actions (`window.confirm` only), no error boundary, "Loading…" plain text instead of skeletons, no theme toggle, no sidebar active-link highlighting, no semantic accent palette, weak a11y on the audit grid.

## What Changes

### Track D — cross-cutting UX foundations

- `sonner` toast library wired in via `lib/toast.ts` semantic helpers (`toastSuccess`, `toastError`, `toastInfo`). Replaces every inline `setMessage` pattern across users, bundles, policies, requests admin/user.
- `<ErrorBoundary>` (class component) wraps `<main>` so render errors surface a small recovery card instead of blanking the chrome.
- `<SubmitButton>` primitive: built-in spinner, `aria-busy`, semantic variants. Applied to all major forms.
- `<ConfirmModal>` primitive: focus-trapped, `role="dialog" aria-modal="true"`, Esc/click-outside cancel, destructive variant. Replaces `window.confirm()` for: delete role, revoke grant (Zitadel diagnostics), reject access request, bundle-assignment confirmation.
- `<Skeleton>` + `<SkeletonCardList>` primitives applied across users, bundles, projects, applications, policies, audit, topology.
- Sidebar split into a server-side outer (`Sidebar.tsx`) + a client-side `<SidebarNav>` that uses `usePathname()` for active-link highlighting (`aria-current="page"` + bg + left border) and renders activity badges next to **Audit Log** (expiring grants ≤14d) and **Access Requests** (pending count) sourced from `/api/proxy/governance/summary`.
- Theme system: `data-theme="light"|"dark"` attribute on `<html>` driven by `<ThemeProvider>` + `useTheme()` hook with localStorage persistence; `<ThemeToggle>` icon button in the sidebar footer; `globals.css` reorganized so both themes have explicit token sets and prefers-color-scheme falls through to a system marker that still resolves correctly.
- Semantic accent tokens (`--color-success`, `--color-warning`, `--color-danger`, `--color-info`) added to the design tokens; `<Badge>` extends with `success`/`warning`/`info` variants; new `<Button>` primitive carries the same matrix.
- Accessibility pass: visible `:focus-visible` outlines globally; audit grid converted from CSS Grid to semantic `<table>` with `<th scope="col">` and `<caption>`; aria-labels added to icon-only buttons (theme toggle, copy, search, zoom controls); keyboard-accessible focus traps in modals.

### Track C — design-promise gaps per view

- **Access Lineage (User-Centric)**: distinct color treatment for Source (primary tint) vs Derived (emerald tint with ↳ inheritance arrows); each role shows a `formatRoleRef` human label plus a small monospace raw `project:role` tag; reasons surface the inheritance kind via tooltip.
- **Bundle Preview**: each bundle button shows a role-count Badge plus the first 4 role chips; clicking Assign opens a `<ConfirmModal>` listing the exact roles that will be applied.
- **Token Simulator**: `<CopyButton>` for the JSON payload; new `<JsonView>` tokenizer renders syntax-highlighted JSON without a new dependency; new "Compare with" select runs a second simulation in parallel and renders both panels side-by-side with differing values highlighted in amber.
- **Project View — per-role rollups**: each role row in the Role Catalog is expandable. On expand: bundles using the role, mapping rules where the role is target ("inherited from"), rules where it's source ("triggers"). Composed entirely client-side from the existing `/bundles`, `/bundles/{id}/roles`, and `/rules/mapping` endpoints — no backend change.
- **Bundle View — Affected Projects badge**: derives unique project_ids from `bundleRoles` and renders a "Affected projects (N)" header with project-name chips. Contained roles also use the human label.
- **Governance — urgency on expiring grants**: `describeExpiry()` helper returns `{countdown, tone, daysLeft}` with `critical/warning/neutral/expired` tones. The audit page sorts expiring grants soonest-first and renders tone-coded urgency badges + bordered cards. Cleanup-hints sub-state gets explicit empty copy.
- **Auto-expiring roles — friendly picker + countdown**: bare `<input type="number">` replaced with a button group (`1 week / 1 month / 1 semester / Permanent`) + a small custom-days input; live preview line under the picker. Existing grants list shows the countdown ("expires in 3 days") with destructive Badge variant when ≤7 days.
- **Self-service requests — notes, approver, status**: `UserRequestsView` history surfaces reviewer name + decision note when `status !== pending`; rejected status uses the destructive Badge variant. Inline modal (`<RequestAccessButton>`) on the member service catalog opens a focus-trapped form with justification textarea + duration button group, replacing the two-step navigate-to-`/requests` flow.
- **Mapping rules — live preview + cycle warning**: new `POST /api/v1/rules/mapping/validate` endpoint reuses `dbDetectCycleOnInsert` without persisting. `CreateRuleForm` debounces validation calls (250ms quiet) and shows a live "IF X THEN ADD Y" preview, an amber warning on cycles, and a green confirmation when safe. Submit is disabled on cycle/self-reference.
- **Audit timeline — grouping + filters + load more**: rows group under day headers (Today / Yesterday / weekday / absolute date). Three-control filter row: free-text debounced search, action category, actor. "Load more" cursor-bumps the limit up to 200. Filtered-empty state offers a "Clear filters" CTA.
- **Topology Graph — pan/zoom + deeplinks**: canvas wrapped in a CSS-transformed `<div>` with mouse-drag panning and Cmd/Ctrl+wheel zoom (clamped 0.4x–2.5x). Zoom-in / zoom-out / Reset buttons + a help hint overlay the canvas. Inspector now includes a "View details →" deeplink to the relevant detail page.
- **Microcopy**: new `lib/format.ts` helpers (`formatRoleRef`, `formatProjectName`, `humanizeKey`) used everywhere a role reference is displayed so non-technical staff see "3D Lab · Member" with a small monospace `printing:member` tag for power users.

## Capabilities

### Modified Capabilities

- `user-management`: visual lineage chain with inheritance attribution; bundle preview before assignment; friendly expiry picker + countdown indicators.
- `application-claims`: Token Simulator copy/highlight/compare; access-request authorship surfaces in user history.
- `role-management`: per-role rollups in the project view (bundles + rules in/out).
- `automation-policies`: live preview, debounced cycle validation, and impact warning on mapping-rule creation.
- `access-governance`: urgency-tone expiring grants list; audit timeline grouping + filters + cursor pagination.
- `service-catalog`: inline modal Request Access flow with justification + duration picker; reviewer notes in user history.
- `topology-graph`: pan/zoom controls, transform-based viewport, node deeplinks to detail pages.
- `operational-readiness`: toast system, ConfirmModal, ErrorBoundary, theme toggle, sidebar active-link highlighting + activity badges, semantic accent tokens, focus-visible accessibility, semantic audit table.

## Impact

- **Backend additions** (small, scoped):
  - `POST /api/v1/rules/mapping/validate` (uses existing `dbDetectCycleOnInsert`).
- **Frontend additions**: `Skeleton.tsx`, `Button.tsx`, `ConfirmModal.tsx`, `SubmitButton.tsx`, `CopyButton.tsx`, `EmptyState.tsx` (already from `live-only-production-ui`), `JsonView.tsx`, `ErrorBoundary.tsx`, `SystemModeBadge.tsx` (from `live-only-production-ui`), `SidebarNav.tsx`, `ThemeToggle.tsx`, `RequestAccessButton.tsx`, `lib/toast.ts`, `lib/theme.tsx`, `lib/format.ts`, `lib/useDebounce.ts`.
- **Dependency**: `sonner` (~3KB, idiomatic in the React/Next ecosystem).
- **Test coverage**: 12 new tests across `format.test.ts` and the existing `session.test.ts` extension. Backend `go test ./...` continues to pass at 272 tests.
- **No migration required.** Behavior changes only.
- **No breaking contract changes.** All API surface additions are net-new endpoints; existing endpoints retain their schemas.
- **Demo mode preserved.** Every change uses catalog-driven defaults or the existing demo session flow; nothing new branches on `ZITADEL_DOMAIN` in the UI.
