# UI Capability Gap Closure

**Status:** In progress
**Source:** [`docs/UI-CAPABILITY-GAPS.md`](../../../docs/UI-CAPABILITY-GAPS.md) — audited 2026-08-02 against `465bdb5`
**Phase:** 5.5
**Depends on:** nothing. Every item is independently landable.

## Why

The August 2026 audit compared all 88 backend routes, every query hook, the
design brief's screen specs, and 27 open changes against what the console
actually renders. It found capability the product has — built, tested, in some
cases specified as its own named capability — that no operator can reach.

The findings split by where the gap lives, and that split decides the work:

- **A — Regressions.** The overhaul (`d8676df`) dropped surfaces that existed.
  `86638b3` restored some. The rest are UI-only errands.
- **B — Built, unreachable.** Backend shipped with tests; no UI calls it.
- **C — Missing lifecycle.** The endpoint does not exist either. These are
  backend + UI, and two of them (`C1`, `C2`) are one-way doors today: a mapping
  rule or a bundle, once created, cannot be removed.
- **D — Planned, never surfaced.** Roadmap items with no surface at all.
- **E — Live deployment.** What the running instance shows that the console
  does not — a crash-looping sync container the UI reports as a static "not
  connected yet".

This change is the backlog and the delta record for closing them. It is
deliberately one change rather than fifteen: the items share a design
vocabulary (the access-source triple, the rehearsal pattern, the name
resolver), and splitting them would multiply spec surface without adding
clarity.

## What changes

Grouped by the audit's buckets; see `tasks.md` for the ticked state.

**Bucket A has landed in full.** Twelve items: the twelve below. **Buckets B and C's landable
items have landed too** — `B1`, `B2`, `C1`, `C2`, `C3`.

**Nothing MkAuth creates is permanent any more, and retiring it is the revoke half of an edit.**
A mapping rule and a bundle could be authored and never removed. Neither needed a deletion
mechanism: every cascade here projects an effective-role closure delta, and a deletion is that
same computation with one edge gone — `CascadeRuleUpdated` with no replacement edge, and
`CascadeBundleRemovedFromUser` run over every holder. Both commit the mutation and its revokes in
one transaction, because an assignment that vanished without its revoke is not a gap, it is drift
with no actor attached. A bundle can also be renamed, which runs no cascade and publishes no
version: a name is what operators call it, not what it grants.

**A member can take back their own ask.** Withdrawing is a resolution, not a decision — resolved,
with no reviewer recorded, because nobody reviewed it. Adding a third terminal state exposed two
latent bugs: the decision guard enumerated the decided statuses instead of testing "not pending",
and both request views rendered "settled and not approved" as a denial.

**The vault has a surface, and it says what it is not.** Four endpoints, Argon2id storage,
twenty-three tests, and nothing that reached them. It belongs to the person rather than the
System page the brief named, because every endpoint is self-only.

**Role identity is composed in one place, and always as a pair.** `admin` in
Printing Lab and `admin` in Metal Shop are two different roles. The product had
two dormant mechanisms for saying so — a `<RoleName>` component with zero call
sites and a `formatRoleRef()` helper called only by its own test — while nine
surfaces hand-rolled the pair and two named a role without its project at all.
The composition now lives in exactly two places: `<RoleRef>` for rows and
`roleLabel()` for prose.

## Capability deltas

- `role-disambiguation` (from `advanced-role-crud`) — **relocated, not
  removed.** It was implemented as a backend-composed `display_label` string on
  catalog rows. A server-composed label can only exist where the server composed
  it, so it could never apply to a grant row, an audit entry, or a request — the
  places where the ambiguity actually bites. The capability now lives in the UI,
  where it applies to every role reference. `CatalogRole.display_label` is
  removed.

**Queues say when they are not finished.** A drain reported two of its five
outcomes, so a pass that requeued eight writes read as an idle queue on HTTP
200. The audit log capped at 200 rows with no cursor, so older history was
unreachable and nothing said so. Both now state their own limits.

**Filters exist where the capability already did.** Event activity can be asked
for the deliberate non-actions the `dropped_enrichment_incomplete` status was
invented to make visible; drift triage can be scoped to a project or to the
detection source that carries no actor. Bulk confirmation-mode apply reaches its
endpoint. Every filtered list distinguishes "empty" from "nothing matches".

**The ask is the member's, and the decider can see it.** Requests were filed as
90 days regardless of what was wanted. A member now chooses in weeks and terms,
and the operator deciding sees what was asked for.

**A member can see what exists before naming it.** The one route in was a form
that asked you to name what you wanted first.

**Two pieces of tedium became one action.** Adding roles to a bundle is a
searchable multi-select across every project with a resumable apply, and a role
can be created from the project it belongs to.

## Out of scope

- `C5` (claim profile versioning) and `C8` (an actor for sweep-found drift) — both
  speculative in the audit's own words. Neither has bitten. Both now carry the
  concrete trigger that would justify them.
- `C9b` (hardware sync state on the person page) — not buildable. There is no
  per-user sync state while the bridge is parked, and it needs the same contract
  `E1` needs.
- LLDAP end-to-end and the Google Workspace poller (`D10`, `D11`) — parked
  tracks with their own changes.

## Closed since first draft

- `C6` — the Trace column was an inference twice over: a bundle/rule id wearing a
  `c_` prefix, linking to an unfiltered page. `enqueueCascadeRows` now owns the
  audit insert it was already minting an id for, so the lineage is structural
  rather than remembered.
- `C7` — settled: an application lives in exactly one project, which is what the
  schema already said. The design diagram was the thing that was wrong.
- `C9a` — Advanced shows Zitadel's own grant id alongside MkAuth's, per project,
  operator-only.
- `C4` — the expiring-access queue can record "seen, letting it lapse", with the
  reopen rule set to **when the grant changes**. The rule is a stored date and a
  comparison rather than an invalidation, so nothing can forget it silently.
