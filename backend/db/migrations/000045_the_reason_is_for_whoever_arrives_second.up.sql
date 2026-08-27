-- 000045_the_reason_is_for_whoever_arrives_second.up.sql
--
-- A decision on a merge finding requires a reason, and the reason was written
-- nowhere the next reader could find it.
--
-- The API refuses a resolution without one, and for the adopt path it lands on
-- the allowance the decision creates. For every other resolution it was read,
-- validated, and dropped.
--
-- That is exactly backwards, because of who the reason is FOR. A finding takes
-- one decision — the answers are opposites, and keeping Syndra's value and
-- taking the target's cannot both be queued without one releasing an account
-- the other is re-provisioning — so the second operator to open a finding is
-- refused, and told only that somebody decided and what they chose. The one
-- thing that would let them agree or disagree is why. Made mandatory for the
-- person who arrives second, and then not kept for them.
ALTER TABLE target_merge_findings
    ADD COLUMN IF NOT EXISTS decision_reason TEXT;

-- Attributed with the rest of the decision, by the same rule and for the same
-- reason: a choice with no author decided itself, and a choice with no reason
-- cannot be argued with.
ALTER TABLE target_merge_findings
    DROP CONSTRAINT IF EXISTS merge_finding_decision_is_attributed;

ALTER TABLE target_merge_findings
    ADD CONSTRAINT merge_finding_decision_is_attributed
    CHECK ((decided_at IS NULL AND decided_by IS NULL AND decision IS NULL)
        OR (decided_at IS NOT NULL AND btrim(decided_by) <> '' AND decision IS NOT NULL
            AND btrim(coalesce(decision_reason, '')) <> ''));
