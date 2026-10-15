-- A request the person came to hold another way is fulfilled (15 Oct 2026).
--
-- Given directly, or through a bundle, while a request for the same role was
-- waiting, the request stayed open: the queue still asked for a decision, the
-- person couldn't ask again once the role ended, and the only way to close it
-- was to decline it, which told them "Not approved" for access they had.
--
-- 'fulfilled' is a resolution, not a decision: whoever gave the role is the
-- reviewer (or `system`), and the note says how it came.
ALTER TABLE access_requests DROP CONSTRAINT IF EXISTS ck_access_requests_status_enum;
ALTER TABLE access_requests
    ADD CONSTRAINT ck_access_requests_status_enum
    CHECK (status IN ('pending', 'approved', 'rejected', 'withdrawn', 'fulfilled'));

ALTER TABLE access_requests DROP CONSTRAINT IF EXISTS ck_access_requests_pending_unresolved;
ALTER TABLE access_requests
    ADD CONSTRAINT ck_access_requests_pending_unresolved
    CHECK (
        (status = 'pending' AND reviewer_user_id IS NULL AND resolved_at IS NULL)
        OR (status IN ('approved', 'rejected', 'fulfilled') AND reviewer_user_id IS NOT NULL AND resolved_at IS NOT NULL)
        OR (status = 'withdrawn' AND reviewer_user_id IS NULL AND resolved_at IS NOT NULL)
    );

-- 000054 closed requests for roles already held as declined, which tells the
-- person "Not approved". They weren't refused; they already had it.
UPDATE access_requests
   SET status = 'fulfilled',
       review_note = 'Already had this when it was asked, so there was nothing to decide.'
 WHERE status = 'rejected'
   AND reviewer_user_id = 'system'
   AND review_note = 'They already have this, so there was nothing to approve.';
