-- Back to the ASCII-space definition. The values stay trimmed — the backfill is
-- not reversible and re-inserting whitespace would be inventing it — so what is
-- lost is only the agreement with Go: a tab-padded term becomes writable again.
ALTER TABLE target_role_mappings DROP CONSTRAINT IF EXISTS target_role_mappings_term_is_canonical;
ALTER TABLE allowances DROP CONSTRAINT IF EXISTS allowances_term_is_canonical;

ALTER TABLE allowances
    ADD CONSTRAINT allowances_term_is_canonical
    CHECK (field = btrim(field) AND value = btrim(value));
ALTER TABLE target_role_mappings
    ADD CONSTRAINT target_role_mappings_term_is_canonical
    CHECK (field = btrim(field) AND value = btrim(value));

DROP FUNCTION IF EXISTS syndra_canonical_term(TEXT);
