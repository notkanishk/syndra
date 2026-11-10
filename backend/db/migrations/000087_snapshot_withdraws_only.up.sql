-- 000087_snapshot_withdraws_only
--
-- Whether a desired state can only take access away (EntitlementSet.WithdrawsOnly:
-- the lifecycle switch alone, for a departure or a denied account) is a fact
-- about the state, so it is recorded with it. The outbox row copies it from
-- the snapshot its approval cites, on the system path and the operator's
-- alike: before this, only the system path declared it, and an operator's
-- approved lock for a departed member replaced the merge base with the two
-- fields it observed. Existing snapshots read false: what they were.
ALTER TABLE desired_state_snapshots ADD COLUMN IF NOT EXISTS withdraws_only boolean NOT NULL DEFAULT false;

-- And the fact is held to its meaning: a withdraw-only state is the lifecycle
-- switch alone, switched off. The background runner sends such a row with
-- nobody reviewing it, so a caller that marked a full set withdraw-only would
-- otherwise have groups, home and quota sent unreviewed.
ALTER TABLE desired_state_snapshots DROP CONSTRAINT IF EXISTS desired_state_snapshots_withdraws_only_check;
ALTER TABLE desired_state_snapshots ADD CONSTRAINT desired_state_snapshots_withdraws_only_check
    CHECK (NOT withdraws_only OR (
        (state_json - 'enabled' - 'smb_enabled') = '{}'::jsonb
        AND state_json -> 'enabled' = 'false'::jsonb
        AND state_json -> 'smb_enabled' = 'false'::jsonb));
