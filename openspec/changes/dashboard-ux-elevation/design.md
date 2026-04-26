# Dashboard UX Elevation Design

## Decisions

### 1. Toast over inline message state

Inline `setMessage` strings are easy to miss and bound to a single component. `sonner` is small (~3KB), idiomatic, and centralizes feedback in one bottom-right region with the `<Toaster richColors closeButton />` config. Wrapped in `lib/toast.ts` so callers depend on a stable surface (`toastSuccess`, `toastError`, `toastInfo`) — we can swap libraries later without touching call sites.

### 2. Confirmation modal is reusable, not per-flow

Every destructive path (delete role, revoke grant, reject request, assign-bundle confirmation) reaches for `<ConfirmModal>`. The modal handles focus trap, Esc, click-outside, `aria-modal`, and the busy state. Each call site supplies copy and the async `onConfirm`. This keeps destructive UX consistent across views and lets the audit pass a11y in one place.

### 3. Sidebar active-link splits server/client

`<Sidebar>` stays a server component so it can mount the async `<SystemModeBadge>` and read the session directly. The nav rail is a client child (`<SidebarNav>`) so we can use `usePathname()` for `aria-current="page"` and fetch the governance summary for activity badges. This avoids forcing the entire sidebar into the client bundle just for a `pathname` check.

### 4. Theme toggle uses `data-theme`, not the `dark:` prefix

Tailwind v4 with `@theme` tokens makes a `data-theme` attribute the simplest swap surface: each theme block in `globals.css` redefines the `--background`, `--foreground`, `--primary`, etc. tokens, and every `bg-background`/`text-foreground` utility resolves through the token. No component-level `dark:` annotations needed.

The provider mirrors the system preference into a `data-theme-system` attribute when no user preference is stored, so the OS preference still wins by default. Storing an explicit choice into localStorage opts out of OS updates.

### 5. JsonView is a tiny in-tree tokenizer, not a syntax-highlighting dep

The Token Simulator only needs JSON pretty-printing with mild color hierarchy and optional diff highlighting. A 100-line walker keeps the bundle lean and avoids `react-syntax-highlighter`'s ~30KB. Compare mode passes an opposite-side `compareWith` value through the walker; differing primitives get amber underline.

### 6. Per-role rollups composed client-side

Rather than expand the `/api/v1/projects` response shape, we compose the rollups from `/bundles`, `/bundles/{id}/roles`, and `/rules/mapping` on the projects page. The data is small, already cached server-side, and recomputed cheaply on every render via `useMemo`. Trade-off: per-role member counts would require either a backend endpoint or N user-access fetches, so we deferred that aspect — the bundle-and-rule rollup is the higher-value "where does this role get used?" surface.

### 7. Mapping-rule validation is a separate POST, not piggybacked on create

`POST /api/v1/rules/mapping/validate` lets the form fire on every change without persisting anything; create still does its own check on submit. The response shape is intentionally minimal (`would_cycle`, `self_reference`, `reason`) so the form can react fast without a payload negotiation. Backend short-circuits on partial input so an in-progress form doesn't 400.

### 8. Topology pan/zoom uses CSS transforms, not a library

A single transformed `<div>` with `transform: translate() scale()` is enough — no third-party graph library needed. Pan tracks `mousedown` on the canvas (only when the empty surface is the target so node clicks still register), `mousemove` updates the offset, `mouseup` clears the start. Cmd/Ctrl+wheel adjusts scale (gated by modifier so normal vertical scroll still works). Reset / +/- buttons overlay the canvas in a small floating control. Node `<button>` elements stop mousedown propagation so they don't trigger pan.

### 9. Audit grouping + filtering is client-side

50–200 entries fit comfortably in memory; grouping by day and filtering by category/actor/free-text is `useMemo` over the loaded array. "Load more" bumps the backend `limit` query param up to the existing 200 cap. Server-side filter params would expand the SQL surface for marginal value at this scale; revisit if the audit log grows beyond a few thousand routine entries per day.

### 10. Authorship in the inline request modal

`<RequestAccessButton>` does not pass `requester_id` in the body — the proxy injects `session.id` for non-admin requests (per `live-only-production-ui` design decision). The modal stays scoped to the user portal flow; admin-on-behalf-of stays in the dedicated `AdminRequestsView`.

## Non-decisions / Deferred

- **Per-role member counts** in the project view. Requires either a new backend endpoint or N user-access fetches. Deferred until the backend has a `GetUsersWithRole(projectID, roleKey)` query.
- **Affected-users count on rule validation.** Same constraint — would need a query that walks current grants. The cycle warning is the high-value safety check; the impact preview is polish for a future iteration.
- **Mobile / responsive layout.** Sidebar stays at fixed 256px. Out of scope; a mobile nav redesign is a separate change.
- **Cancel pending request.** Backend has no cancel endpoint today and the audit log story for "user cancelled their own request" needs to be designed before the UI can reasonably surface it.
- **Iconography library** (e.g., lucide-react). Each new component currently uses inline SVGs sparingly — the bundle stays lean. Adding a library is easy if a future iteration demands consistent icon usage across many CTAs.
- **React Testing Library setup.** Vitest runs in `node` env; behavioral component tests would require `happy-dom` plus RTL. Existing tests focus on pure helpers (`session`, `format`) and backend handlers, which is sufficient coverage for the changes shipped.
