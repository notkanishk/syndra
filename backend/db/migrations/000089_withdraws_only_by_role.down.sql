-- Back to the 000087 form, which names TrueNAS's two fields. NOT VALID: a
-- withdraw-only row written under 000089 for a target whose account is not
-- `enabled` is a true statement the old form cannot read, and refusing the
-- rollback over it would strand the deploy. New rows are checked as before.
ALTER TABLE desired_state_snapshots DROP CONSTRAINT IF EXISTS desired_state_snapshots_withdraws_only_check;
ALTER TABLE desired_state_snapshots ADD CONSTRAINT desired_state_snapshots_withdraws_only_check
    CHECK (NOT withdraws_only OR (
        (state_json - 'enabled' - 'smb_enabled') = '{}'::jsonb
        AND state_json -> 'enabled' = 'false'::jsonb
        AND state_json -> 'smb_enabled' = 'false'::jsonb)) NOT VALID;
