-- Withdrawn requests become rejected-by-nobody, which the pre-withdrawal
-- constraint cannot express — so they are rejected by 'system' with the note
-- saying what actually happened. Deleting the rows instead would erase asks
-- that were genuinely made, and leaving them 'withdrawn' would fail the
-- constraint this restores.
UPDATE access_requests
SET status = 'rejected',
    reviewer_user_id = 'system',
    review_note = COALESCE(NULLIF(review_note, ''), 'Withdrawn by the requester.')
WHERE status = 'withdrawn';

ALTER TABLE access_requests DROP CONSTRAINT IF EXISTS ck_access_requests_status_enum;
ALTER TABLE access_requests
    ADD CONSTRAINT ck_access_requests_status_enum
    CHECK (status IN ('pending', 'approved', 'rejected'));

ALTER TABLE access_requests DROP CONSTRAINT IF EXISTS ck_access_requests_pending_unresolved;
ALTER TABLE access_requests
    ADD CONSTRAINT ck_access_requests_pending_unresolved
    CHECK (
        (status = 'pending' AND reviewer_user_id IS NULL AND resolved_at IS NULL)
        OR (status IN ('approved', 'rejected') AND reviewer_user_id IS NOT NULL AND resolved_at IS NOT NULL)
    );
