-- C6 (ISC-44) — the audit Trace column stops being an inference.
--
-- pending_zitadel_propagations.cascade_id (migration 000019) already groups the
-- writes ONE triggering event produced. The audit row describing that same
-- event had no way to name it, so the console inferred a trace from
-- audit_logs.resource_id — a bundle or rule id, rendered with a `c_` prefix and
-- linked to an unfiltered change history. Two false statements in one column:
-- the identifier was not what its prefix claimed, and the link did not go to
-- the entry it named.
--
-- NULLable on purpose, and left NULL for every row written before this column
-- existed. There is no honest backfill: matching old audit rows to outbox
-- batches by timestamp proximity would be mostly right, and a lineage link that
-- is mostly right on a record of who may operate a laser cutter is worse than
-- one that is absent. Those rows keep showing their resource id, unlinked and
-- labelled as the object it is.
--
-- No foreign key to pending_zitadel_propagations: outbox rows are drained and
-- eventually pruned, and an audit row must outlive the queue that carried out
-- its consequence. Same reasoning as source_ref (000017) and
-- onboarding_triggers.bundle_id (000021) — history records what caused a
-- change, and the cause having since been swept up is not a reason to forget it.
ALTER TABLE audit_logs
    ADD COLUMN IF NOT EXISTS cascade_id UUID;

-- Partial: only cascade-originated rows carry one, and the only query that
-- filters on it asks for a specific cascade.
CREATE INDEX IF NOT EXISTS idx_audit_logs_cascade_id
    ON audit_logs(cascade_id)
    WHERE cascade_id IS NOT NULL;
