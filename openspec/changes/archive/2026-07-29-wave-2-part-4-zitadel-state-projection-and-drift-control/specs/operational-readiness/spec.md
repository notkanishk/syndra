> **Status:** Wave 2 · Part 4 delta — Drift vs Pending urgency tiers (parent design §5) | [< Index](../../../../INDEX.md) | [Feature Coverage](../../../mkauth-core-architecture/specs/feature-coverage.md)

# Requirement: Operational Readiness (delta)

## ADDED Requirements

### Requirement: The UI MUST distinguish Pending Propagation from Drift with calibrated, non-overlapping urgency

Two operator surfaces — **Pending Propagation** (operator-initiated, buffered, no risk) and **Drift** (out-of-band, possibly an incident) — MUST be visually distinct at every dimension, with drift louder than pending at each. Drift breaks out of layouts (sticky undismissible banner, dedicated nav, optional audio); pending lives inside layouts (nested nav, dismissible callout, inline tags, no audio). Both MUST use Material tokens from the obsidian-clarity palette.

#### Scenario: Pending Propagation uses amber, dismissible, in-layout treatments

- **WHEN** the pending-propagation count is greater than zero
- **THEN** the sidebar MUST show a nested `Pending [N]` item under Operations (filled badge, `bg-tertiary`/`text-on-tertiary`) and the parent Operations item MUST show a small amber dot
- **AND** the dashboard MUST show a per-session-dismissible callout with the count, last-queued time, current Zitadel reachability, and a `Resume now` button
- **AND** the count badge MUST pulse exactly once when the count increases, with no repeated pulse and no audio

#### Scenario: Drift uses red, undismissible, breaks-out-of-layout treatments

- **WHEN** the drift count is greater than zero
- **THEN** the sidebar MUST show a dedicated top-level `⚠ Drift` item above `/governance/*` with a persistent red dot
- **AND** every admin page MUST show a sticky, undismissible top banner `⚠ N drift items detected — out-of-band changes need triage [Review →]`
- **AND** the dashboard MUST show a full-width undismissible callout above the stat grid with a top-3 preview and `Triage all →`

#### Scenario: Motion and audio respect user preferences

- **WHEN** drift surfaces animate (sidebar pulse, banner slide-in, count-up, new-row highlight)
- **THEN** all motion MUST be suppressed when `prefers-reduced-motion: reduce` is set
- **AND** the optional drift chime MUST be gated by a `localStorage` toggle (default on) reachable from the avatar-menu popover, with a one-time tooltip explaining the cue on first play

### Requirement: Reconciliation MUST be right-sized for the single-LXC makerspace audience and schedulable

The reconciliation sweep MUST page Zitadel grants with a safety cap of 2 000 (down from 10 000), MUST run on a configurable interval (`DRIFT_RECONCILIATION_INTERVAL_HOURS`, default 6) via a scheduler mirroring the grant-expiry scheduler, and MUST also be triggerable on demand by the operator.

#### Scenario: Cap and on-demand trigger

- **WHEN** the reconciliation sweep runs against an org whose grant count is within 2 000
- **THEN** the sweep MUST complete without setting the truncated flag
- **AND** the operator MUST be able to trigger the sweep on demand via a `[Reconcile now]` action on `/governance/drift`, independent of the scheduled interval
