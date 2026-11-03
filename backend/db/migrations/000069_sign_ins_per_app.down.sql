DROP TABLE IF EXISTS sign_in_daily;
DROP INDEX IF EXISTS usage_last_seen_app_idx;
-- Fold app rows back into one row per project, keeping the latest time.
DELETE FROM usage_last_seen u USING usage_last_seen v
 WHERE u.user_id = v.user_id AND u.project_id = v.project_id AND u.kind = v.kind
   AND (u.last_at, u.app_id) < (v.last_at, v.app_id);
ALTER TABLE usage_last_seen DROP CONSTRAINT usage_last_seen_pkey;
ALTER TABLE usage_last_seen DROP COLUMN app_id;
ALTER TABLE usage_last_seen ADD PRIMARY KEY (user_id, project_id, kind);
