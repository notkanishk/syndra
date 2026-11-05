-- 000075_first_sign_in
--
-- A person's first sign-in to Syndra, as one timestamp, written once and never
-- overwritten (A-18 Q9, idea 8: the one-time welcome). The sign-in route writes
-- it with INSERT ... ON CONFLICT DO NOTHING, so a later sign-in changes nothing.
--
-- Backfill: Syndra has kept no record of its own sign-ins, so nobody who has
-- already used it may be welcomed as new. Everyone Syndra holds any record of
-- is marked as having signed in, from the earliest such record: an audit-log
-- entry they made, a request they filed, a sign-in event Zitadel reported, or
-- a grant held (a grant carries its own created_at). People with no record at
-- all are the ones who have never touched Syndra, and still get the welcome.
CREATE TABLE IF NOT EXISTS person_first_sign_in (
    user_id          TEXT        PRIMARY KEY CHECK (btrim(user_id) <> ''),
    first_sign_in_at TIMESTAMPTZ NOT NULL
);

INSERT INTO person_first_sign_in (user_id, first_sign_in_at)
SELECT user_id, MIN(at) FROM (
    SELECT actor_zitadel_user_id AS user_id, created_at AS at FROM audit_logs
    UNION ALL
    SELECT requester_user_id, created_at FROM access_requests
    UNION ALL
    SELECT user_id, last_at FROM usage_last_seen WHERE kind = 'sign_in'
    UNION ALL
    SELECT user_id, created_at FROM zitadel_grants_index
) seen
WHERE user_id IS NOT NULL AND btrim(user_id) <> '' AND user_id <> 'system' AND at IS NOT NULL
GROUP BY user_id
ON CONFLICT (user_id) DO NOTHING;
