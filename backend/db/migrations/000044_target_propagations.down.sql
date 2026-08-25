-- 000044_target_propagations.down.sql
--
-- Dropping this loses the record that a write landed. Reconciliation falls back
-- to what a complete read has observed, which is weaker in exactly one case and
-- it is the case this table was added for: a grant applied and removed between
-- two sweeps becomes indistinguishable from one that was never written, and is
-- replayed rather than reported.
DROP INDEX IF EXISTS idx_target_propagations_subject;
DROP TABLE IF EXISTS target_propagations;
