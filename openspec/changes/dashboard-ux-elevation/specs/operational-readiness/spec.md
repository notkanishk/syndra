> **Status:** Integrated | [< Index](../../../../INDEX.md)

## ADDED Requirements

### Requirement: All form actions MUST surface toast feedback

The dashboard MUST display toast notifications on the success and failure of every privileged form action so admins are never left guessing whether a submission landed.

#### Scenario: Toast on success
- **WHEN** a create or update action returns a 2xx response
- **THEN** a success toast MUST appear naming the action (e.g., "Bundle created", "Direct grant saved", "Request submitted")

#### Scenario: Toast on failure
- **WHEN** a create or update action returns a non-2xx response
- **THEN** an error toast MUST appear with the backend's error message (or a generic fallback if the body is empty)
- **AND** the form MUST remain open so the admin can correct and retry

### Requirement: Destructive actions MUST require explicit confirmation

The dashboard MUST gate destructive actions (delete, revoke, reject) behind a styled confirmation modal — not native `window.confirm()` — so the affordance matches the rest of the design language and is keyboard-accessible.

#### Scenario: ConfirmModal replaces window.confirm
- **WHEN** an admin triggers a destructive action (delete role, revoke grant, reject request)
- **THEN** a modal MUST open with `role="dialog" aria-modal="true"`, focus trap, Esc cancel, and click-outside cancel
- **AND** the destructive variant MUST use the destructive button styling on the confirm action
- **AND** the action MUST only commit after the admin clicks the modal's Confirm button

### Requirement: Loading states MUST use skeleton placeholders

The dashboard MUST render skeleton placeholders matching the eventual layout while data loads, instead of bare "Loading…" text, so layout shift is avoided and perceived performance improves.

#### Scenario: Skeleton on initial load
- **WHEN** a list view is fetching its primary data
- **THEN** the loading indicator MUST be a stack of pulse-animated skeleton cards (or rows) with representative geometry
- **AND** `aria-busy="true"` MUST be set on the wrapping container

### Requirement: Sidebar MUST highlight the active route and surface activity counts

The sidebar nav MUST mark the current route with `aria-current="page"` and visual highlight, and MUST surface counts of pending requests and expiring grants on the relevant nav items so admins know where attention is needed.

#### Scenario: Active link highlight
- **WHEN** a user navigates to any route present in the sidebar
- **THEN** the matching nav link MUST set `aria-current="page"` and apply the active visual treatment (left border + tinted bg)

#### Scenario: Activity badges from governance summary
- **WHEN** an admin session loads the sidebar
- **THEN** `Access Requests` MUST display a count Badge equal to the number of pending requests (when > 0)
- **AND** `Audit Log` MUST display a count Badge equal to the number of expiring grants (when > 0)
- **AND** the counts MUST be sourced from `/api/proxy/governance/summary` and silently fall back to 0 on fetch failure

### Requirement: Theme MUST be user-controllable

The dashboard MUST expose a manual light/dark theme toggle that persists to localStorage and is honored on subsequent loads.

#### Scenario: Theme toggle in sidebar footer
- **WHEN** the sidebar renders for an authenticated user
- **THEN** a sun/moon icon button MUST be present in the footer Card
- **AND** clicking it MUST flip the `data-theme` attribute on `<html>` between `light` and `dark`
- **AND** the choice MUST persist to localStorage and be re-applied on next load

#### Scenario: Default to system preference when no override is stored
- **WHEN** no theme override is stored in localStorage
- **THEN** the resolved theme MUST follow `prefers-color-scheme`
- **AND** subsequent OS-level changes MUST be reflected in the rendered theme as long as no override is stored

### Requirement: Focus-visible outlines MUST be present globally

The dashboard MUST render visible focus outlines on all interactive controls when navigated by keyboard, using the primary token color, without showing them on mouse focus.

#### Scenario: :focus-visible global rule
- **WHEN** any element receives focus via keyboard navigation
- **THEN** it MUST display a 2px solid outline in the primary color with 2px offset
- **AND** mouse focus MUST NOT trigger the outline (so click affordances stay clean)
