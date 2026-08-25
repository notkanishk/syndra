-- 000031_mapping_version_delete_guard.up.sql
-- Add-on platform, audit follow-up: published mapping history is immutable in
-- BOTH directions.
--
-- 000029 guarded UPDATE and stopped there, so a published version could still
-- be DELETEd — and its entries cascade away with it, which erases the set a
-- rollback restores and the record every later publish is compared against.
-- The reason the UPDATE guard exists ("a record that can be edited is a
-- comparison against nothing") is exactly the reason the DELETE guard has to,
-- and deleting the row erases more than editing one field of it.
--
-- `desired_state_snapshots` already guards both, so this also removes a
-- disagreement between two tables that hold the same kind of history.

CREATE OR REPLACE FUNCTION reject_target_mapping_version_mutation() RETURNS trigger
    LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'target_mapping_versions rows are immutable history (attempted %)', TG_OP;
END;
$$;

DROP TRIGGER IF EXISTS target_mapping_versions_immutable ON target_mapping_versions;
CREATE TRIGGER target_mapping_versions_immutable
    BEFORE UPDATE OR DELETE ON target_mapping_versions
    FOR EACH ROW EXECUTE FUNCTION reject_target_mapping_version_mutation();

-- The entries are the version's content. Guarding the parent and leaving the
-- children editable would leave the version number intact over a set somebody
-- rewrote — the same comparison-against-nothing, one table down.
CREATE OR REPLACE FUNCTION reject_target_mapping_version_entry_mutation() RETURNS trigger
    LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'target_mapping_version_entries rows are immutable history (attempted %)', TG_OP;
END;
$$;

DROP TRIGGER IF EXISTS target_mapping_version_entries_immutable ON target_mapping_version_entries;
CREATE TRIGGER target_mapping_version_entries_immutable
    BEFORE UPDATE OR DELETE ON target_mapping_version_entries
    FOR EACH ROW EXECUTE FUNCTION reject_target_mapping_version_entry_mutation();
