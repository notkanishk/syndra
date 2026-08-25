-- Dropping the table discards standing findings. Each one is two records
-- disagreeing about who owns an account, and the disagreement survives in the
-- data whether or not this table remembers noticing it.
DROP TABLE IF EXISTS target_binding_conflicts;
