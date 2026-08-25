-- 000030_allowances.down.sql
--
-- An allowance in force is somebody's access being deliberately withheld.
-- Dropping the table restores that access silently, which is the one direction
-- a rollback must never move access in — so it refuses while any is in force
-- and points at lifting them, which is recorded and re-converges the subject.
--
-- Lifted rows are history and go with the table: the audit log holds the
-- decision independently, and history that only exists here has already served
-- its purpose once the suspension is over.
DO $$
DECLARE
    in_force BIGINT;
BEGIN
    SELECT COUNT(*) INTO in_force FROM allowances WHERE lifted_at IS NULL;
    IF in_force > 0 THEN
        RAISE EXCEPTION
            'refusing to roll back: % allowance(s) are in force, and dropping them would silently restore access somebody decided to withhold. Lift them first, which is recorded and re-converges each subject.',
            in_force;
    END IF;
END
$$;

DROP TABLE IF EXISTS allowances;
