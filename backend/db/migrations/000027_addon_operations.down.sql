-- Reverse of 000027.
--
-- Refuses while any operation is unresolved. A `dispatching` or
-- `indeterminate` row is the only surviving evidence that a secret-bearing call
-- may have applied to a target and nobody knows whether it did — the secret was
-- never retained, so the row cannot be reconstructed from anything else. A down
-- migration that dropped it would silently close the one question a human still
-- owes an answer to.
--
-- Settled rows are history and go with the table; that is what a down migration
-- is for. It is the open ones that must stop it.
DO $$
DECLARE
    open_count BIGINT;
BEGIN
    IF to_regclass('public.addon_operations') IS NULL THEN
        RETURN;
    END IF;

    SELECT COUNT(*) INTO open_count
      FROM addon_operations
     WHERE status IN ('dispatching', 'indeterminate');

    IF open_count > 0 THEN
        RAISE EXCEPTION
            'refusing to drop addon_operations: % operation(s) are still unresolved. Each one may have applied to its target and its parameters were never retained, so dropping the table destroys the only record that the question exists. Resolve them first.',
            open_count;
    END IF;
END
$$;

DROP INDEX IF EXISTS idx_addon_operations_subject;
DROP INDEX IF EXISTS idx_addon_operations_unresolved;
DROP TABLE IF EXISTS addon_operations;
