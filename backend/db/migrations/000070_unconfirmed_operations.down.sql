-- Reverse of 000070. The answers and the resend links go with their columns;
-- the records themselves, and their dispatch status, stay.
DROP INDEX IF EXISTS idx_addon_operations_open_by_target;
DROP INDEX IF EXISTS idx_addon_operations_one_resend;
ALTER TABLE addon_operations
    DROP CONSTRAINT IF EXISTS addon_operations_resolution_check,
    DROP CONSTRAINT IF EXISTS addon_operations_attempt_check,
    DROP COLUMN IF EXISTS superseded_by,
    DROP COLUMN IF EXISTS resolved_at,
    DROP COLUMN IF EXISTS resolved_by,
    DROP COLUMN IF EXISTS resolution,
    DROP COLUMN IF EXISTS retry_of,
    DROP COLUMN IF EXISTS attempt,
    DROP COLUMN IF EXISTS about;
