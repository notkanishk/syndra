-- 000043_a_decision_is_not_a_settlement.up.sql
--
-- A finding closed when the work was merely QUEUED (change
-- `reconciliation-as-merge`).
--
-- Keeping Syndra's state does not change the target; it enqueues a convergence
-- that the drain applies later, and the apply's read-back is what records the
-- new base. Closing the finding at the moment of the decision therefore claimed
-- a difference was over while it was still there — and the very next sweep,
-- seeing it, raised a SECOND standing finding about the same field. One decision
-- would produce a queue that refills itself every six hours until the drain
-- caught up.
--
-- So a decision and a settlement become two different things. The decision is
-- recorded when a person makes it, and the row stays standing — which is also
-- what keeps the deduplicating index doing its job. The row closes when a pass
-- observes that the two sides agree, and it closes carrying the decision that
-- got them there rather than the anonymous `agreed`.
ALTER TABLE target_merge_findings
    -- What was decided, and by whom, before anything has been applied.
    ADD COLUMN IF NOT EXISTS decision    TEXT,
    ADD COLUMN IF NOT EXISTS decided_by  VARCHAR(255),
    ADD COLUMN IF NOT EXISTS decided_at  TIMESTAMPTZ;

-- A decision is who chose and what they chose, together or not at all — the
-- same rule the resolution columns already carry, for the same reason: a choice
-- with no author is a finding that decided itself.
ALTER TABLE target_merge_findings
    ADD CONSTRAINT merge_finding_decision_is_attributed
    CHECK ((decided_at IS NULL AND decided_by IS NULL AND decision IS NULL)
        OR (decided_at IS NOT NULL AND btrim(decided_by) <> '' AND decision IS NOT NULL));

-- The surface's question: what has somebody decided that the target has not yet
-- caught up with. Partial, because a settled row's decision is already in
-- `resolution`.
CREATE INDEX IF NOT EXISTS idx_merge_finding_decided
    ON target_merge_findings (target, decided_at DESC)
    WHERE resolved_at IS NULL AND decided_at IS NOT NULL;
