-- 000096_merge_finding_binding_conflict (addon-contract 7t-3)
--
-- A binding the add-on reports as naming no account, because another account
-- holds its name or number (/subjects conflicts, a blocked apply's conflict),
-- is a standing finding with its own outcome, 'binding_conflict', in the
-- account-level slot (field ''). Before this it read as 'deleted_upstream' and
-- was offered a re-create the add-on then refused. Widened, every earlier
-- outcome kept; the name is the one Postgres gave 000042's inline check.
ALTER TABLE target_merge_findings DROP CONSTRAINT IF EXISTS target_merge_findings_outcome_check;
ALTER TABLE target_merge_findings ADD CONSTRAINT target_merge_findings_outcome_check
    CHECK (outcome IN ('theirs_only', 'conflict', 'deleted_upstream', 'binding_conflict'));
