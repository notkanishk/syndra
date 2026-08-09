-- 000026_addon_platform_target_dimension.down.sql
-- Reverse of the up, in reverse order.
--
-- This down migration REFUSES rather than reinterprets. Once a second target
-- exists, its rows cannot survive the loss of the column that names it: a
-- drift row for TrueNAS with `target` dropped is a Zitadel drift row that never
-- happened, sitting in an operator's triage queue. Rolling back after a target
-- has been registered is disabling that target's registry row (design §Rollback
-- and §3), not this migration. So the guard fires and the operator decides,
-- rather than the schema quietly telling a lie.
DO $$
DECLARE
    n BIGINT;
BEGIN
    SELECT (SELECT count(*) FROM propagation_outbox WHERE target <> 'zitadel')
         + (SELECT count(*) FROM drift_items WHERE target <> 'zitadel')
         + (SELECT count(*) FROM external_grant_exclusions WHERE target <> 'zitadel')
         + (SELECT count(*) FROM desired_state_snapshots WHERE target <> 'zitadel')
      INTO n;
    IF n > 0 THEN
        RAISE EXCEPTION
            'refusing to drop the target dimension: % row(s) name a target other than zitadel. Disable the target in `targets` instead of rolling this migration back.', n;
    END IF;
END
$$;

-- Reconciliation currency is an observation, rebuilt by the next sweep. It is
-- dropped after the guard above, so a refused rollback leaves it intact.
DROP TABLE IF EXISTS target_reconciliation;

-- 1.7 --------------------------------------------------------------------
ALTER TABLE external_grant_exclusions
    DROP CONSTRAINT IF EXISTS external_grant_exclusions_pkey;
ALTER TABLE external_grant_exclusions
    ADD PRIMARY KEY (user_id, project_id, role_key);
ALTER TABLE external_grant_exclusions
    DROP COLUMN IF EXISTS target;

DROP INDEX IF EXISTS idx_drift_items_pending_unique;
CREATE UNIQUE INDEX idx_drift_items_pending_unique
    ON drift_items (user_id, project_id, drift_type, role_keys)
    WHERE status = 'pending_triage';

ALTER TABLE drift_items DROP CONSTRAINT IF EXISTS drift_items_drift_type_check;
UPDATE drift_items SET drift_type = 'zitadel_only' WHERE drift_type = 'target_only';
ALTER TABLE drift_items
    ADD CONSTRAINT drift_items_drift_type_check
    CHECK (drift_type IN ('zitadel_only', 'syndra_only'));

ALTER TABLE drift_items DROP CONSTRAINT IF EXISTS drift_items_zitadel_shape_check;
ALTER TABLE drift_items
    ALTER COLUMN project_id SET NOT NULL,
    ALTER COLUMN role_keys  SET NOT NULL;
ALTER TABLE drift_items DROP COLUMN IF EXISTS target;

-- 1.5 --------------------------------------------------------------------
DROP INDEX IF EXISTS idx_propagation_outbox_plan_subject;
ALTER TABLE propagation_outbox DROP COLUMN IF EXISTS plan_subject_id;
DROP TABLE IF EXISTS plan_subjects;
DROP TABLE IF EXISTS plans;

-- 1.4 --------------------------------------------------------------------
DROP TRIGGER IF EXISTS desired_state_snapshots_version_monotonic ON desired_state_snapshots;
DROP TRIGGER IF EXISTS desired_state_snapshots_immutable ON desired_state_snapshots;
DROP TABLE IF EXISTS desired_state_snapshots;
DROP FUNCTION IF EXISTS enforce_desired_state_snapshot_version();
DROP FUNCTION IF EXISTS reject_desired_state_snapshot_mutation();

-- 1.3 --------------------------------------------------------------------
DROP INDEX IF EXISTS idx_propagation_outbox_target_status;

ALTER TABLE propagation_outbox DROP CONSTRAINT IF EXISTS propagation_outbox_zitadel_shape_check;
ALTER TABLE propagation_outbox DROP COLUMN IF EXISTS target;

ALTER TABLE propagation_outbox
    ALTER COLUMN project_id SET NOT NULL,
    ALTER COLUMN role_keys  SET NOT NULL;

ALTER TABLE propagation_outbox DROP CONSTRAINT IF EXISTS propagation_outbox_op_type_check;
ALTER TABLE propagation_outbox
    ADD CONSTRAINT pending_zitadel_propagations_op_type_check
    CHECK (op_type IN ('add', 'revoke', 'replace'));

ALTER TABLE propagation_outbox DROP CONSTRAINT IF EXISTS propagation_outbox_status_check;
ALTER TABLE propagation_outbox
    ADD CONSTRAINT pending_zitadel_propagations_status_check
    CHECK (status IN ('pending', 'in_flight', 'applied', 'failed'));

ALTER TABLE propagation_outbox DROP CONSTRAINT IF EXISTS propagation_outbox_source_check;
ALTER TABLE propagation_outbox
    ADD CONSTRAINT pending_zitadel_propagations_source_check
    CHECK (source IN ('direct', 'bundle', 'rule', 'external_backfill', 'lifecycle_cascade'));

-- 1.2 --------------------------------------------------------------------
ALTER INDEX IF EXISTS propagation_outbox_idempotency_key_key
    RENAME TO pending_zitadel_propagations_idempotency_key_key;
ALTER INDEX IF EXISTS propagation_outbox_pkey
    RENAME TO pending_zitadel_propagations_pkey;
ALTER INDEX IF EXISTS idx_propagation_outbox_cascade
    RENAME TO idx_pending_zitadel_propagations_cascade;
ALTER INDEX IF EXISTS idx_propagation_outbox_source
    RENAME TO idx_pending_zitadel_propagations_source;
ALTER INDEX IF EXISTS idx_propagation_outbox_status
    RENAME TO idx_pending_zitadel_propagations_status;

ALTER TABLE IF EXISTS propagation_outbox RENAME TO pending_zitadel_propagations;

-- 1.1 --------------------------------------------------------------------
DROP TABLE IF EXISTS targets;
