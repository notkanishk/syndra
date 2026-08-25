-- A term is stored trimmed, and the database is what says so (§26).
--
-- 000036's sibling defect. Three Go paths now normalise before writing — the
-- allowance create, the mapping create, the mapping edit — and that leaves two
-- gaps a convention cannot close:
--
--   * rows written BEFORE that fix keep their padding, and the symptom is
--     precisely a carve-out that appears nowhere. `group=lab_makers ` is
--     compared by exact bytes against `lab_makers` in the resolver's
--     suppression and in the holder list's intersection, so it suppresses
--     nothing and renders nowhere while reading as an accepted suspension.
--   * a rollback restores from `target_mapping_version_entries` with an
--     INSERT … SELECT that passes through no Go at all, so a version published
--     before the fix puts its padded values straight back.
--
-- So the invariant moves into the schema. A CHECK is the difference between a
-- rule three call sites remember and a rule a statement cannot break: the
-- rollback fails loudly rather than silently restoring an inert value, and so
-- does any writer added later that skips the helper. Same move as
-- `withdraws_only`'s CHECK.

-- Backfill first, or the constraints below refuse the table they are added to.
-- No uniqueness includes `value` on either table — target_role_mappings is
-- unique on (target, project_id, role_key, field) and allowances only on its id
-- — so trimming cannot collide two rows into one.
UPDATE allowances
   SET field = btrim(field), value = btrim(value)
 WHERE field <> btrim(field) OR value <> btrim(value);

UPDATE target_role_mappings
   SET field = btrim(field), value = btrim(value)
 WHERE field <> btrim(field) OR value <> btrim(value);

ALTER TABLE allowances DROP CONSTRAINT IF EXISTS allowances_term_is_canonical;
ALTER TABLE allowances
    ADD CONSTRAINT allowances_term_is_canonical
    CHECK (field = btrim(field) AND value = btrim(value));

ALTER TABLE target_role_mappings DROP CONSTRAINT IF EXISTS target_role_mappings_term_is_canonical;
ALTER TABLE target_role_mappings
    ADD CONSTRAINT target_role_mappings_term_is_canonical
    CHECK (field = btrim(field) AND value = btrim(value));

-- Deliberately NOT on target_mapping_version_entries. A published version is a
-- historical record — the table carries a trigger refusing every mutation for
-- that reason — and a record of what somebody published is allowed to contain
-- what they published. The rollback is where it gets canonicalised, by btrim in
-- the restoring SELECT, and the CHECK above is what fails if that is ever
-- dropped.
--
-- Trimming the history instead would need the immutability trigger suspended,
-- which is a larger thing to give up than this is worth: it would make "the
-- version you are rolling back to" editable to close a gap the destination
-- table already closes.
