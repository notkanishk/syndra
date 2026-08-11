-- One definition of canonical, shared by both languages (§27).
--
-- 000037 put the invariant in the schema and expressed it with `btrim(x)`,
-- which is `btrim(x, ' ')` — ASCII SPACES ONLY. Go's `strings.TrimSpace`
-- strips every Unicode whitespace rune. Two definitions that agree on the
-- common case and diverge on the ones that actually occur.
--
-- Both halves of the divergence bite:
--
--   * the CHECK ADMITS a term Go would have trimmed. `group=lab_makers<TAB>`
--     satisfies it, so the constraint blesses as canonical a value that then
--     misses the exact-byte comparison in the resolver's suppression and in
--     the holder-list intersection — an accepted carve-out matching nothing,
--     with a constraint asserting it is fine.
--   * the rollback's btrim strips spaces only, so a version published before
--     the normalisation fix holding a tab- or NBSP-padded value restores with
--     the padding intact and inert — through the exact path 000037 closed.
--
-- U+00A0 is the realistic vector rather than a contrived one: a group name
-- pasted out of the TrueNAS web UI carries a non-breaking space routinely, and
-- it is invisible in every surface an operator would look at.
--
-- So the rule becomes a function rather than an expression repeated four times
-- in two dialects. The CHECKs, the backfill and the mapping rollback all call
-- it, and Go stays on TrimSpace — the two now describe the same set, and a Go
-- write satisfies the constraint by construction rather than by coincidence.

-- The 25 runes Go's unicode.IsSpace reports:
--   U+0009, U+000A, U+000B, U+000C, U+000D, U+0020, U+0085, U+00A0, U+1680, U+2000, U+2001, U+2002, U+2003, U+2004, U+2005, U+2006, U+2007, U+2008, U+2009, U+200A, U+2028, U+2029, U+202F, U+205F, U+3000
-- IMMUTABLE because a CHECK constraint requires it, and it is: the set is a
-- constant of the Unicode standard, not of this deployment.
CREATE OR REPLACE FUNCTION syndra_canonical_term(t TEXT) RETURNS TEXT
    LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE
    AS $$ SELECT btrim(t, E'\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000') $$;

-- Re-backfill: 000037's pass left everything Go would have trimmed and btrim
-- would not.
UPDATE allowances
   SET field = syndra_canonical_term(field), value = syndra_canonical_term(value)
 WHERE field <> syndra_canonical_term(field) OR value <> syndra_canonical_term(value);

UPDATE target_role_mappings
   SET field = syndra_canonical_term(field), value = syndra_canonical_term(value)
 WHERE field <> syndra_canonical_term(field) OR value <> syndra_canonical_term(value);

ALTER TABLE allowances DROP CONSTRAINT IF EXISTS allowances_term_is_canonical;
ALTER TABLE allowances
    ADD CONSTRAINT allowances_term_is_canonical
    CHECK (field = syndra_canonical_term(field) AND value = syndra_canonical_term(value));

ALTER TABLE target_role_mappings DROP CONSTRAINT IF EXISTS target_role_mappings_term_is_canonical;
ALTER TABLE target_role_mappings
    ADD CONSTRAINT target_role_mappings_term_is_canonical
    CHECK (field = syndra_canonical_term(field) AND value = syndra_canonical_term(value));
