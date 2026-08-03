-- A member can take back their own request.
--
-- Until now the only way out of a mistaken ask was for an operator to reject
-- it, which puts a refusal in the log for something nobody refused, and puts a
-- decision in an operator's queue that the person who filed it had already
-- changed their mind about. requests_bulk.go has been writing the copy for this
-- since it was built — "No such request — it may have been withdrawn."
--
-- 'withdrawn' is a resolution, not a decision. reviewer_user_id stays NULL:
-- nobody reviewed it. The requester is already on the row, so recording them a
-- second time as their own reviewer would be a fact the row already states,
-- written in a column that means something else.
ALTER TABLE access_requests DROP CONSTRAINT IF EXISTS ck_access_requests_status_enum;
ALTER TABLE access_requests
    ADD CONSTRAINT ck_access_requests_status_enum
    CHECK (status IN ('pending', 'approved', 'rejected', 'withdrawn'));

-- Resolution invariant, restated for three outcomes rather than two: pending
-- has neither reviewer nor resolution time; a decision has both; a withdrawal
-- has a time and no reviewer. The constraint is what stops a withdrawal being
-- written as an anonymous approval.
ALTER TABLE access_requests DROP CONSTRAINT IF EXISTS ck_access_requests_pending_unresolved;
ALTER TABLE access_requests
    ADD CONSTRAINT ck_access_requests_pending_unresolved
    CHECK (
        (status = 'pending' AND reviewer_user_id IS NULL AND resolved_at IS NULL)
        OR (status IN ('approved', 'rejected') AND reviewer_user_id IS NOT NULL AND resolved_at IS NOT NULL)
        OR (status = 'withdrawn' AND reviewer_user_id IS NULL AND resolved_at IS NOT NULL)
    );
