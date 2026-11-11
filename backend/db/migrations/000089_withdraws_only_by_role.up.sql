-- 000089_withdraws_only_by_role (addon-contract 2.2)
--
-- A withdraw-only snapshot is still the lifecycle switch alone, switched off,
-- but the core no longer knows the switch's names (the account and channel
-- fields are the target's own, design §3). Only `account` and `channel` fields
-- travel as JSON booleans; every other role is an array. So "the switch alone,
-- off" is: a non-empty object whose every value is JSON false. Strict mode,
-- because in lax mode a string or array compared with a boolean is unknown and
-- the filter never matches, which accepted a row carrying groups. Verified on
-- Postgres 16. No column, no registry read: a CHECK cannot read another table.
--
-- Added without NOT VALID, so every existing row is re-validated; the new form
-- accepts every row the 000087 form did.
ALTER TABLE desired_state_snapshots DROP CONSTRAINT IF EXISTS desired_state_snapshots_withdraws_only_check;
ALTER TABLE desired_state_snapshots ADD CONSTRAINT desired_state_snapshots_withdraws_only_check
    CHECK (NOT withdraws_only OR (state_json <> '{}'::jsonb AND NOT jsonb_path_exists(state_json, 'strict $.* ? (@.type() != "boolean" || @ == true)')));
