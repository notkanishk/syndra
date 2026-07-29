# Wave 2 · Part 4 — Zitadel State Projection & Drift Control

**Status:** Proposed
**Source:** [May 2026 audit resolution design](../../../docs/superpowers/specs/2026-05-08-may-2026-audit-resolution-design.md) §2–§5, §7 Theme 2 — audit refs **B2, B4, C6, D3**
**Phase:** 5.5
**Wave:** 2 (Part 4 of 4 — the marquee architectural change; Parts 1, 2, 3 cover frontend palette, backend coherence, and operational polish, all merged or in progress)

## Why

MkAuth's stated doctrine is *"Backend is the single mutation authority — frontend and triggers signal, backend decides."* The audit (B4/D3) found that doctrine contradicted by reality: the `/api/v1/zitadel/users/{id}/grants` CRUD handlers (`discovery.go:218-282`) mutate Zitadel **directly** via `zitadelAddUserGrant` / `zitadelUpdateUserGrant` / `zitadelRemoveUserGrant`, writing nothing to MkAuth's own store. The result is grants that exist in Zitadel with no MkAuth-side reason, expiry, or attribution — exactly the "who has X and why?" gap the whole product exists to close. There is no path today by which a grant made in the Zitadel admin UI (or by any external tool) acquires MkAuth context, and no surface that tells an operator such a grant happened.

This wave resolves the contradiction by making the **two-layer model** real (parent design §2, §4.1):

- **Zitadel is the current-state mirror.** Every grant, whatever its origin, exists as a `user_grant` in Zitadel. This is unchanged — Zitadel remains the absolute source of truth.
- **MkAuth is the intent ledger.** For every grant in Zitadel, MkAuth stores `reason`, `expires_at`, `granted_by`, and now `source` (`direct` | `bundle` | `rule` | `external_backfill` | `lifecycle_cascade`) + `source_ref`. MkAuth-mediated mutations write the ledger *before* the Zitadel API call, inside one transaction. Direct-Zitadel mutations are detected as **drift** and acquire their ledger record through operator **triage** (Attribute / Revoke / Mark external).

The single load-bearing doctrinal sentence (parent design §7.1) replaces the old one in `design.md` and `CLAUDE.md`. Everything in this wave implements that sentence.

This is the largest and last Wave 2 change. It deliberately lands after Part 2 (backend coherence — `services/views.go` and `repositories.go` are now clean substrate for the new projection lookups) and Part 1 (frontend palette — the drift/pending UI is built on Material tokens from day one, never on the legacy palette).

## What changes

The work is decomposed into **three independently-shippable sub-phases** (parent design §6.5), delivered as **one** OpenSpec change with a phased `tasks.md` (parent design §9 Q7). Sub-phases 1+2 ship a working slice (operator point mutations through the buffer, plus drift visibility for direct grants) without sub-phase 3.

### Sub-phase 1 — Outbox & operator-confirmed propagation (B4/D3)

- **New `pending_zitadel_propagations` outbox table** (migration `000015`). Every MkAuth-mediated Zitadel mutation is enqueued here as a row with `op_type` (`add`/`revoke`/`replace`), `idempotency_key` (UUID), and a `status` lifecycle. The table mirrors the existing `provisioning_intents` pattern (idempotency key, claim-and-process, status transitions) rather than inventing new machinery.
- **`direct_role_grants` gains `source` + `source_ref`** (same migration, additive; existing rows backfill to `source='direct'`, `source_ref=NULL`). The full CHECK enum is installed now so sub-phase 3 needs no further `ALTER`.
- **Transactional enqueue.** A new `db.EnqueueDirectGrantPropagation` runs `INSERT direct_role_grants` + `INSERT audit_logs` + `INSERT pending_zitadel_propagations` in one `pgx.Tx`. The grant intent is durably recorded before any Zitadel call.
- **Operator-confirmed drain** (`services/propagation` package, mirroring `services/expiry`). A `Drain(ctx)` loop, triggered by the operator (`POST /api/v1/propagations/drain` — "Resume now"), processes pending rows in `created_at` order: pre-flight `/zitadel/health` check (halt + surface "Zitadel offline" if unreachable), per-row Zitadel call, per-row ACK (`2xx → applied`; `4xx → failed`; `5xx/timeout → back to pending`, attempts++). An **already-exists check** against the webhook-derived grant index marks a row `applied` without an API call (handles drift-attribution overlap and crash recovery where an `in_flight` row replays idempotently).
- **`applied` is terminal success — there is no `confirmed` state.** This is a deliberate, reasoned deviation from parent design §4.3 step 7 (see `design.md` Decision 1). The existing self-mutation guard (`webhook_translate.go` drops events whose editor is `ZITADEL_M2M_USER_ID`) means Zitadel never echoes MkAuth's own grant mutations back over the webhook — so a webhook "return-trip confirmation" cannot fire for MkAuth-originated grants. The synchronous `2xx` from the Management API *is* the confirmation. The webhook path is reserved for **drift** (sub-phase 2), which is exactly what the self-mutation guard leaves surviving: only externally-originated grant events.
- **`/api/v1/zitadel/*` CRUD rewired internally (B4/D3).** `handleAssignZitadelGrant` / `handleUpdateZitadelGrant` / `handleRemoveZitadelGrant` stop calling Zitadel directly and flow through the canonical enqueue-then-drain path. The `/zitadel/*` URLs stay as backward-compatible aliases (operator bookmarks); both they and `/api/v1/users/{id}/grants` go through one code path. Response shape changes from `{status:"granted"}` to `{outbox_id, idempotency_key, status:"pending"|"applied"}`.
- **Pending Propagation UI** (amber, lives *inside* layouts — parent design §5.2): nested `Pending [N]` sidebar item under Operations, a dismissible dashboard callout with reachability + `Resume now`, and an inline `[⏱ Awaiting Zitadel] [Resume]` tag on affected grants. Governance summary gains a `pending_propagation` block.

### Sub-phase 2 — Drift detection & triage (B2, C6)

- **New `drift_items` + `external_grant_exclusions` tables** (migration `000016`).
- **Real-time drift via webhook.** When `webhook_translate_enrich.go` processes a surviving (non-self) `grant_added`/`grant_removed`, the handler matches it against `external_grant_exclusions` then the expected set; an unexplained grant inserts a `drift_items` row (`detection_source='webhook'`).
- **Reconciliation sweep evolved, not rebuilt.** The existing on-demand `reconciliation.go` diff (already computes `OnlyInZitadel`/`OnlyInMkAuth`/`Drift` buckets) is (a) **cap dropped 10 000 → 2 000** (B2), (b) made schedulable on `DRIFT_RECONCILIATION_INTERVAL_HOURS` (default 6) via an `expiry`-style scheduler, (c) taught that mapping-rule-derived grants are `expected_via_rule` rather than `OnlyInZitadel`, and (d) wired to **upsert `drift_items`** for `zitadel_only` and **re-enqueue outbox rows** for `mkauth_only` (missed-webhook replay).
- **C6 overlay-cache fix.** `directory/zitadel.go:Users()` cache-miss-then-drift cases are covered by the reconciliation backstop; the partial-overlay path is documented and reconciliation closes the gap.
- **`/governance/drift` triage queue** (red, *breaks out* of layouts — parent design §5.3): dedicated top-level nav item with persistent red dot, sticky undismissible cross-page banner, full-width undismissible dashboard callout, and a triage page with three per-row actions (**Attribute** / **Revoke** / **Mark external**) plus bulk attribution for the bootstrap case. Optional soft chime with a localStorage kill-switch (parent design §9 Q9 option (b)).

### Sub-phase 3 — Bundle/rule cascade projection

- **`confirmation_mode` on `mapping_rules` + `bundles` + `config_settings`** (migration `000017`, default `auto`).
- **Cascade enqueueing** on the six trigger points (parent design §4.7): add/remove user↔bundle, add/remove role↔bundle, mapping-rule fire, mapping-rule matcher change. Each enqueues outbox rows with `source='bundle'|'rule'` and `source_ref`. An **other-source check** suppresses a revoke when another source still covers the `(user, project, role)`. Auto-mode rules drain immediately; manual-mode and operator point mutations wait for explicit resume. Expiry sweeps and lifecycle cascades are hardcoded-auto (their authoring is the pre-authorization).

### Cross-cutting documentation (final consolidation task)

`CLAUDE.md` and `mkauth-core-architecture/design.md` adopt the new doctrinal sentence (parent design §7.1); `feature-coverage.md` gains a **Drift Control** row and flips the access-governance reconciliation note; `ROADMAP.md` gets a Phase 5.5 closure line; `INDEX.md` gets this change's row; `.env.example` gains `DRIFT_RECONCILIATION_INTERVAL_HOURS` and `OUTBOX_MAX_RETRIES`.

## Out of scope

- **No automatic resumption when Zitadel returns from offline.** The operator must explicitly trigger the drain (parent design §8). Buffered rows persist; they do not self-drain.
- **No drift auto-resolution heuristics.** Every `drift_items` row requires explicit operator action — no "age > 30d → auto-mark-external" (parent design §8).
- **No idempotency-key round-trip through Zitadel grant metadata.** Rejected in favor of synchronous-ACK confirmation + tuple-matched drift (see `design.md` Decision 1). MkAuth does not depend on Zitadel echoing custom metadata it cannot guarantee.
- **No changes to the data-plane claim path, `claim_failure_mode` cache, or `services/views.go` role-resolution semantics.** Part 2 owns those; this wave only *adds* projection lookups beside them.
- **No multi-replica / horizontal-scale outbox coordination.** Single-LXC; the drain runs in one process (parent design §8, Theme 5 D9).
- **Themes 1/3/4/5** — shipped or owned elsewhere; this wave touches only the cross-cutting docs the parent design's §7 table assigns to Theme 2.

(Audit refs: B2, B4, C6, D3)
