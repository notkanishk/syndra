-- 000102_audit_target_action_index (addon-contract 7w-5, access over time)
--
-- A person's access over time (db/access_history.go) reads audit_logs by the
-- person it is about, a handful of action names and a window, and its lateral
-- lookups (the assignment before an unassignment, a move, another ending in
-- between) ask the same three things again for each row. Every other index on
-- the table is partial (000064) or on cascade_id (000023), so without this each
-- of those reads scans the whole log. Expand-only: a backend rolled back across
-- it starts on the newer schema unchanged.
CREATE INDEX IF NOT EXISTS idx_audit_logs_target_action_created
    ON audit_logs (target_zitadel_user_id, action, created_at);
