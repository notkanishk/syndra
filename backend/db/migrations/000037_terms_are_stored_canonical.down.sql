-- Dropping the constraints leaves the values trimmed, because the backfill is
-- not reversible and putting the padding back would be inventing it. What is
-- lost is the enforcement: writers that skip NormaliseTerm can store an inert
-- term again.
ALTER TABLE target_role_mappings DROP CONSTRAINT IF EXISTS target_role_mappings_term_is_canonical;
ALTER TABLE allowances DROP CONSTRAINT IF EXISTS allowances_term_is_canonical;
