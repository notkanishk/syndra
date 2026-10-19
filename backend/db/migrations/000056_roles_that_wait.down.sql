ALTER TABLE propagation_outbox DROP COLUMN IF EXISTS wait_reason;
DROP TABLE IF EXISTS role_waits;
