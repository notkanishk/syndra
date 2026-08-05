## ADR-001: Zitadel Event-Listener Wire Format and Grant Enrichment Strategy

**Date:** 2026-05-07
**Status:** Accepted

### Context

The `syndra-event-listener` Action target subscribes to Zitadel lifecycle events
(user.human.added, user.deactivated, user.locked, user.grant.{added,changed,removed}).
The translator must decode the event payload into Syndra's internal `WebhookPayload`
shape before dispatch. Two design problems emerged:

1. **Wire-format guess vs. reality.** The original `zitadelEventPayload` struct
   was modeled on a guessed shape (`{aggregate:{id,...}, event, payload, editorUserId}`).
   That shape never appears on the wire. Zitadel emits its `ContextInfoEvent` from
   `internal/repository/execution/queue.go`: flat top-level fields
   (`aggregateID`, `aggregateType`, `event_type`, `event_payload`, `userID`),
   with mixed snake_case / camelCase that look like typos but are intentional.
   Every real Zitadel event 4xx'd at validation; only the (also-guessed) smoke
   test passed.

2. **Missing fields on grant.changed / grant.removed.** Zitadel's user_grant
   aggregate event_payload is the *delta*, not the full state:
   - `user.grant.added`     → `{userId, projectId, grantId, roleKeys}`
   - `user.grant.changed`   → `{userId, roleKeys}`            (no `projectId`)
   - `user.grant.removed`   → `{userId, projectId, grantId}`  (no `roleKeys`)

   The handler's validation rejects grant events without `source_project` (400),
   so changed/removed events would never reach the processors.

### Decision

**Wire format:** the translator decodes against Zitadel's actual
`ContextInfoEvent`. Shape detection probes the top-level `aggregateID`. A new
`WebhookPayload.GrantID` field surfaces the grant aggregate ID for downstream
correlation.

**Grant enrichment:** a two-layer best-effort lookup populates the missing
fields before validation:

1. **Local cache:** `zitadel_grants_index` table (migration 000011) keyed by
   `grant_id`, populated by successful `grant_added` and `grant_changed`
   processing, deleted on successful `grant_removed`. Repository functions
   `UpsertGrantIndex` / `GetGrantIndex` / `DeleteGrantIndex` live in
   `internal/db/repositories.go`. Sentinel `db.ErrGrantIndexNotFound`
   distinguishes cache miss from query error.
2. **Zitadel API fallback:** on local miss, `listUserGrantsViaZitadel`
   (`internal/handlers/zitadel_grant_lookup.go`) calls the existing
   `zitadelListUserGrants` deps seam to look up the grant by aggregate ID.
3. **Log-and-continue:** when both lookups fail, the payload is left
   unenriched and the handler proceeds. Validation may still reject (400) for
   genuinely incomplete payloads, but a transient enrichment failure must
   never bounce a Zitadel event back as 4xx — that triggers a redelivery
   storm with no clean resolution path.

`enrichGrantPayload` is wired between the translator and the validation block
in `HandleZitadelWebhook`. Index maintenance lives in the dispatch switch
post-success and is non-fatal.

### Alternatives considered

- **Always query Zitadel.** Rejected: every grant event becomes a synchronous
  Management API round-trip, multiplying latency and load on Zitadel for
  what is by definition a high-frequency operation.
- **Denormalize into webhook_events.** Rejected: webhook_events is the
  audit/dedup log; coupling enrichment lookups to its schema would mix
  concerns and add row-scan complexity for a per-grant-aggregate lookup
  that is naturally keyed by grant_id.
- **Reject grant.changed/removed.** Rejected: half-functional listener.

### Consequences

- The local index is best-effort; `provisioning/spec.md`'s reconciliation
  backlog already covers periodic full-sync.
- Pre-existing grants (created before 000011 landed) hit the API fallback
  on first changed/removed event. Per-event API cost is bounded; if
  operationally noisy, a one-shot bootstrap script can populate the index
  from `ListAllGrants`.
- Index maintenance failures are non-fatal — a stale row will be repaired
  on the next event for that grant.
