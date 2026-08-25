-- 000028_plan_request_binding.down.sql
--
-- Dropping the column removes a citation dimension, which makes every
-- unclaimed plan claimable by a request it was not computed for. That is only
-- safe while nothing is holding one, so the rollback refuses rather than
-- quietly widening what an outstanding approval authorises — the same posture
-- 000026's rollback takes towards a foreign target row.
DO $$
DECLARE
    outstanding BIGINT;
BEGIN
    SELECT COUNT(*) INTO outstanding
      FROM plans
     WHERE applied_at IS NULL
       AND request_fingerprint <> '';

    IF outstanding > 0 THEN
        RAISE EXCEPTION
            'refusing to roll back: % unapplied plan(s) are bound to a request, and dropping the binding would let them be applied against a different one. Let them expire or apply them first.',
            outstanding;
    END IF;
END
$$;

ALTER TABLE plans DROP CONSTRAINT IF EXISTS plans_request_fingerprint_shape_check;
ALTER TABLE plans DROP COLUMN IF EXISTS request_fingerprint;
