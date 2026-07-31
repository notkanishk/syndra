-- 000019_ia_surfaces
--
-- Two columns the Basic/Advanced information architecture needs and the schema
-- could not answer:
--
--   1. pending_zitadel_propagations.cascade_id — the writes produced by ONE
--      triggering event share an id. Pending changes groups by it ("both writes
--      share c_8841 because R-014 chained into R-022; they confirm together or
--      not at all") and Change history renders one entry per cascade rather than
--      one per write. Without it the queue can only be sorted by timestamp, and
--      a half-applied cascade — the thing that creates unexplained access — is
--      indistinguishable from two unrelated writes that happened to land close
--      together.
--
--   2. drift_items.upstream_actor / upstream_created_at / last_seen_at — the
--      evidence a triage row has to carry: WHO created this upstream and WHEN,
--      so the row can say "Created in the identity provider on 21 Jul by
--      svc-badge-sync" instead of only "found 9 days ago". Nullable: the
--      reconciliation sweep compares grant sets and genuinely does not know the
--      actor, and an invented one would be worse than an absent one.

ALTER TABLE pending_zitadel_propagations
    ADD COLUMN IF NOT EXISTS cascade_id UUID;

-- Partial: only cascade rows carry one, and the grouping queries always filter
-- on IS NOT NULL.
CREATE INDEX IF NOT EXISTS idx_pending_zitadel_propagations_cascade
    ON pending_zitadel_propagations(cascade_id, created_at)
    WHERE cascade_id IS NOT NULL;

ALTER TABLE drift_items
    ADD COLUMN IF NOT EXISTS upstream_actor      TEXT,
    ADD COLUMN IF NOT EXISTS upstream_created_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS last_seen_at        TIMESTAMPTZ;
