-- 000075_first_sign_in
--
-- A person's first sign-in to Syndra, as one timestamp, written once and never
-- overwritten (A-18 Q9, idea 8: the one-time welcome). The sign-in route writes
-- it with INSERT ... ON CONFLICT DO NOTHING, so a later sign-in changes nothing.
--
-- Backfill: Syndra has kept no record of its own sign-ins, so the people known
-- to have used it are marked as signed in, from the earliest thing they did in
-- it: an audit-log entry they made or a request they filed. Holding a role, or
-- signing in to some other app, is not using Syndra: those people may never
-- have opened it, and they still get the welcome on their first visit.
CREATE TABLE IF NOT EXISTS person_first_sign_in (
    user_id          TEXT        PRIMARY KEY CHECK (btrim(user_id) <> ''),
    first_sign_in_at TIMESTAMPTZ NOT NULL
);

INSERT INTO person_first_sign_in (user_id, first_sign_in_at)
SELECT user_id, MIN(at) FROM (
    SELECT actor_zitadel_user_id AS user_id, created_at AS at FROM audit_logs
    UNION ALL
    SELECT requester_user_id, created_at FROM access_requests
) seen
WHERE user_id IS NOT NULL AND btrim(user_id) <> '' AND user_id <> 'system' AND at IS NOT NULL
GROUP BY user_id
ON CONFLICT (user_id) DO NOTHING;
