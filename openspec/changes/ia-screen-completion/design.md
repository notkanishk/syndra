# Design — completing the drawn IA

## Decision 1 — one cascade id per enqueue call, not per write

`enqueueCascadeRows` receives the closure diff for one triggering event and inserts one outbox row per write. The cascade id is minted **once per call** and stamped on every row in the batch.

This is correct rather than convenient: a chained rule does not produce a second enqueue call. The cascade service computes the full closure diff — including everything a chain contributes — before it enqueues, so "R-014 fired and R-022 chained off it" arrives here as a single set of params. One id per call is therefore exactly the set of writes one event produced, which is what Pending changes needs to say "they confirm together or not at all" and what Change history needs to render one entry per cascade.

Rows written before `000019` have no id. Both readers fall back to the row's own outbox id, so old rows appear as single-write cascades rather than disappearing from history.

**Rejected:** deriving the grouping from `(source, source_ref, created_at)` buckets. It would collapse two independent firings of the same rule seconds apart into one entry, and the failure mode is silent.

## Decision 2 — drift evidence is nullable, and the UI says so

`upstream_actor` and `upstream_created_at` are populated only from the webhook path, which carries Zitadel's editor and event time. The reconciliation sweep compares grant sets; it can see that a grant exists with nothing explaining it and genuinely cannot see who made it.

Rather than backfilling a plausible actor, both columns stay NULL and the triage row switches sentence: *"Found in the identity provider by the reconciliation sweep, which compares grant lists and can't see who made the change."* An invented actor in a triage queue is worse than an absent one — it is the field an operator would quote in a message to the person named.

`last_seen_at` is stamped on every detection including the deduped no-op, which is why `UpsertDriftItem` became `ON CONFLICT DO UPDATE`. It answers "is this still there?" for a row first found nine days ago. The update clause uses `COALESCE(existing, excluded)` for the evidence pair so a sweep re-detecting what a webhook already attributed cannot erase the actor.

## Decision 3 — risk ranking is three tiers, computed server-side

Triage order is risk then age. Risk is: safety-gated (2), role absent from the catalogue (1), everything else (0). Age breaks ties, oldest first.

Three tiers, not five: anything finer is a judgement the data cannot support, and an operator cannot hold more than three levels in their head while triaging. "Safety-gated" is matched as a case-insensitive substring of the identity provider's own role group rather than an enum, because the group is free text upstream and an exact match would silently downgrade `Safety-gated (metal)` to routine.

The ordering is computed in `services.DriftTriageQueue` and the UI **must not** re-sort. Two sorts disagreeing is how a queue stops being a queue.

The row layout never varies with risk. Only the order changes, plus a left border on the leading row — and only the leading row, because if every safety-gated row were marked the marking would stop meaning "start here".

## Decision 4 — the People index carries one attention signal, not three

The design's Needs-attention column renders a single phrase. Where a person triggers more than one signal the row shows the most serious: unexplained access, then an approaching expiry, then an open request.

A cell holding three coloured phrases is a cell nobody reads, and the column exists precisely so a scanning eye can find the row that needs work. The full picture is one click away on the person's page, which is where it belongs.

The three counts are loaded once per request as three whole-table reads rather than three queries per row. Drift is best-effort: a failure there renders zeros rather than 500-ing the People page over one column.

## Decision 5 — upstream writes are restored, disclosed, and never the default path

The identity-provider console is back in full, including grant assign/update/remove and project-role CRUD. The write half sits behind a collapsed disclosure whose expanded state leads with the three consequences, in order of how easily each is missed:

1. the next cache compile can overwrite the change,
2. the drift sweep will report it as unexplained access created by somebody it cannot name,
3. nobody reading Change history afterwards will find out it happened.

Every write control is `danger`-toned even where it is not destructive, and each dialog repeats the warning next to the button rather than relying on the one at the top of the page. A warning three scroll-lengths above the control it applies to is a warning nobody reads.

This does not weaken the single-mutation-authority boundary: those endpoints already existed and were already operator-gated. What changed is that the escape hatch is visible and labelled instead of only reachable with curl.

## Decision 6 — the access map opens on a root, and search is a shortcut

Focus starts `null`. The centre renders every node grouped by kind — projects, roles, bundles, apps — each group ordered by degree so the nodes worth opening surface first. Picking one enters the focused view; a breadcrumb returns.

This keeps the design's core insight (do not draw everything at once) while removing the assumption it silently made: that you already know the name of what you want. The root is lists, not a force layout, so it stays legible at 248 nodes.

The `SHOW` legend became a filter, and it tracks **hidden** kinds rather than shown ones, so the default empty set shows everything. A legend where clicking one entry silently hides the other three reads as a bug the first time it happens.

## Decision 7 — S10 renders the parked state, and the ledger only when it has rows

Hardware sync is a dashed panel with the copy saying "not connected yet" out loud. The intents table renders **only when it holds rows**. Zero rows would be the "0 intents" the design forbids — a table that says the feature works and simply has nothing to do — while real rows are evidence that nothing has been lost while waiting, which is worth showing.

## Decision 8 — audit sentences are a map, and unknown actions fall through

`describeAction` maps the verbs the backend actually writes (verified against every `InsertAuditLog` call site and every inline `INSERT INTO audit_logs`) to sentences. An unrecognised action renders its raw key.

Falling through to the key is deliberate. A log that invents a description for something it does not recognise is worse than one that admits it does not know, and the raw key is at least greppable against the source.

## Gaps this change does NOT close

- **Apps read one project each.** The design draws an app reading four projects in two formats. Zitadel's model puts an application inside exactly one project, and `app_claim_overrides.application_id` is unique, so the many-to-many the design assumes does not exist in the data. The index instead warns when a *project* is read in more than one format, which is the same failure expressed in the shape the data supports.
- **Audit has no cascade id.** The trace column links into Change history for cascade-producing actions and shows a dash otherwise. A real trace needs `cascade_id` threaded through every audit write, which is a wider change than this one.
- **`GET /audit` still takes only a limit.** Both filters narrow the loaded 200 rather than the query, and the header says so.
