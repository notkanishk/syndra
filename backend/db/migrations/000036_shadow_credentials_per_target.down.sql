-- Collapsing back to one row per person. A member enrolled on two targets has
-- two rows and only one can survive; the newest is kept, which is the one whose
-- metadata the member's page would have been rendering anyway.
DELETE FROM shadow_credentials a
 USING shadow_credentials b
 WHERE a.user_id = b.user_id
   AND (a.updated_at, a.id) < (b.updated_at, b.id);

DROP INDEX IF EXISTS shadow_credentials_user_target_key;
ALTER TABLE shadow_credentials
    ADD CONSTRAINT shadow_credentials_user_id_key UNIQUE (user_id);
ALTER TABLE shadow_credentials DROP COLUMN IF EXISTS target;
