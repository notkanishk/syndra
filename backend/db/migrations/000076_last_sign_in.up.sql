-- 000076_last_sign_in
--
-- "Since you were last here" (A-20 P18, owner approved 5 Nov 2026): the same
-- row that holds a person's first sign-in now holds two more times, the
-- current sign-in and the one before it. Each sign-in moves current into
-- previous and writes now() into current, so previous is "when you were last
-- here" for the sign-in that is happening. The first sign-in has no previous.
--
-- Backfill: current starts as the person's latest recorded act in Syndra (an
-- audit-log entry or a request, the same sources 000075 used for the first),
-- never earlier than their first sign-in. Previous stays empty until their
-- next sign-in moves current into it.
ALTER TABLE person_first_sign_in
    ADD COLUMN IF NOT EXISTS last_sign_in_at     TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS previous_sign_in_at TIMESTAMPTZ;

UPDATE person_first_sign_in p
SET last_sign_in_at = GREATEST(p.first_sign_in_at, COALESCE((
    SELECT MAX(at) FROM (
        SELECT created_at AS at FROM audit_logs WHERE actor_zitadel_user_id = p.user_id
        UNION ALL
        SELECT created_at FROM access_requests WHERE requester_user_id = p.user_id
    ) seen
), p.first_sign_in_at))
WHERE last_sign_in_at IS NULL;

ALTER TABLE person_first_sign_in
    ALTER COLUMN last_sign_in_at SET NOT NULL,
    ADD CONSTRAINT person_sign_in_order CHECK (
        last_sign_in_at >= first_sign_in_at
        AND (previous_sign_in_at IS NULL
             OR (previous_sign_in_at >= first_sign_in_at AND previous_sign_in_at <= last_sign_in_at))
    );
