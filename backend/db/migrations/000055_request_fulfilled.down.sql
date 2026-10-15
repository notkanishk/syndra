-- Back to a decline by `system`, the only closed state the older constraint has.
UPDATE access_requests
   SET status = 'rejected',
       reviewer_user_id = COALESCE(reviewer_user_id, 'system')
 WHERE status = 'fulfilled';

ALTER TABLE access_requests DROP CONSTRAINT IF EXISTS ck_access_requests_pending_unresolved;
ALTER TABLE access_requests
    ADD CONSTRAINT ck_access_requests_pending_unresolved
    CHECK (
        (status = 'pending' AND reviewer_user_id IS NULL AND resolved_at IS NULL)
        OR (status IN ('approved', 'rejected') AND reviewer_user_id IS NOT NULL AND resolved_at IS NOT NULL)
        OR (status = 'withdrawn' AND reviewer_user_id IS NULL AND resolved_at IS NOT NULL)
    );

ALTER TABLE access_requests DROP CONSTRAINT IF EXISTS ck_access_requests_status_enum;
ALTER TABLE access_requests
    ADD CONSTRAINT ck_access_requests_status_enum
    CHECK (status IN ('pending', 'approved', 'rejected', 'withdrawn'));
