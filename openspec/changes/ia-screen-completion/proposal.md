## Why

The Basic/Advanced rebuild (`basic-advanced-ia`) landed against a handoff in which only the Basic surface and the access map were drawn. S1–S4, S2b and S6–S11 were "specified in the brief but not yet drawn", and the Advanced screens were built from prose. The handoff has since been revised: every route in the matrix is now drawn, S6 and the People index are marked **high** fidelity, and several screens carry prohibitions the prose version never stated.

Comparing the drawn design against what shipped surfaced three separate problems.

**Screens that contradict a stated prohibition.** Identity provider reported health as a bare red/green dot, which the design forbids in as many words ("never a green or red dot on its own"). Hardware sync rendered an intents table that is empty by construction — the exact "0 intents" the design says implies a working, idle feature. Expiring access painted every row's remaining-time amber, so the deadline signal decorated the whole table instead of marking the row with a deadline. Change history listed one row per write rather than one entry per cascade, which is how a half-applied cascade — the thing that produces unexplained access — becomes indistinguishable from two unrelated writes.

**Capability that exists in the backend and had no UI.** The rebuild deleted the components that used them and never replaced the affordances: role creation (`POST /roles`, including clone-from), removing a role from a bundle (`DELETE /bundles/{id}/roles/…`), bulk drift adoption (`POST /governance/drift/bulk-attribute`), rule editing and per-rule confirmation mode (`PUT /rules/mapping/{id}`, `POST /policies/confirmation-mode`), audit export, event payload inspection, and the whole upstream-inspection surface (`/zitadel/{projects,users,grants}`) that S9 explicitly asks for. Eight endpoints with no caller is not a small design decision; it is a migration that dropped features silently.

**Ids where names belong.** Automatic rules, event activity, hardware sync, the apps index, bundle role rows, the bundle-assignment cascade preview and the app token header all rendered a raw project id in place of a project name. The name resolver existed and was simply not reached for.

Two further things the design assumes and the data could not answer: a drift row cannot say *who* created a grant upstream and *when* (so it could only say "found 9 days ago"), and no write carried an identifier shared with the other writes the same event produced (so Pending changes could only sort by timestamp).

Separately, the access map had no overview at all. It auto-focused the most connected node on load and offered no route back, so the only way to reach a different part of the graph was to already know the name of what you wanted and type it.

Targets **Phase 5** (operator experience). Follows `basic-advanced-ia`.

## What Changes

**Data plane.** Migration `000019` adds `pending_zitadel_propagations.cascade_id` — one id per triggering event, stamped at enqueue — and `drift_items.{upstream_actor, upstream_created_at, last_seen_at}`, populated from the Zitadel event where the webhook knew them and left NULL where the reconciliation sweep genuinely does not. `UpsertDriftItem` becomes an upsert that refreshes `last_seen_at` on re-detection and never overwrites known evidence with an unknown.

**Control plane.** `GET /review/expiring-grants?within_days=` (its own window, so a 30-day review and Today's 14-day queue can differ without either lying), `GET /propagations/cascade-groups` (Change history's unit, including cascades whose writes have not landed), and `POST /governance/drift/bulk-mark-external` (the symmetric second bulk resolution; bulk revoke stays deliberately absent). `GET /governance/drift` returns an enriched triage view — risk group, whether the role is still in the catalogue, holder status, and how many other pending items the same person has — ordered by risk then age. `GET /users` gains the "needs attention" trio plus bundle names and project count, and its search now matches role keys. `ProjectRole.Group` becomes its own field instead of being smuggled through `Description`, which is why every upstream role rendered its group where its description belonged.

**Screens.** Every screen the revised handoff draws is brought to it, and every deleted capability is restored — including the upstream console, whose write half is collapsed behind a disclosure that names all three consequences of bypassing MkAuth before it shows a button.

**Access map.** Opens on a browsable root — every node grouped by kind, ordered by connectedness — with a breadcrumb back to it from any focused node. Search becomes a shortcut rather than the only door.

**Demo data.** The UI carries no fabricated rows; the seeder is env-gated and skipped when a live directory is active. The one real hole was `seed_active: true` alongside a live directory — real people, seeded bundles and rules, and no signal at all. The degraded banner now covers that case with its own copy.

## Impact

- **Affected specs**: access-governance, role-management, user-management, application-claims
- **Affected code**: migration `000019`, `backend/internal/{models,db,services,handlers,directory}`, `ui/src/{app,components,lib}`
- **Behaviour change**: the People index and Review › Expiring access now work to a 30-day window (Today stays at 14). Direct writes to the identity provider are reachable again, behind an explicit disclosure.
