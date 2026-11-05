-- 000079_one_time_zone
--
-- One zone (owner, 5 Nov 2026: "get everything in one IST time zone"). Until
-- every container ran with TZ=Asia/Kolkata (5 Nov 2026, PR #102), two stores
-- keyed their rows by the UTC day. An instant is absolute; a day is not: a
-- moment at 23:00 UTC is 04:30 the next morning in India. This re-keys both
-- on the makerspace's day. The zone is named here, never left to the session's
-- TimeZone (the cluster's is UTC). Running it twice changes nothing more.
--
-- 1. insights_snapshots. Every row's taken_at survives, so its day is
--    re-derived exactly from it. A snapshot taken between 18:30 and 24:00 UTC
--    moves to the next day. Where two rows land on one day, the first picture
--    of that day is kept, the rule the table is written by (ON CONFLICT DO
--    NOTHING); the later one goes. Deleting and re-inserting in one statement
--    lets a row move onto a day another row is leaving.
WITH moved AS (
    DELETE FROM insights_snapshots RETURNING day, taken_at, topology, role_people
)
INSERT INTO insights_snapshots (day, taken_at, topology, role_people)
SELECT DISTINCT ON ((taken_at AT TIME ZONE 'Asia/Kolkata')::date)
       (taken_at AT TIME ZONE 'Asia/Kolkata')::date, taken_at, topology, role_people
  FROM moved
 ORDER BY (taken_at AT TIME ZONE 'Asia/Kolkata')::date, taken_at, day;

-- 2. sign_in_daily. One total per app per day; the sign-ins it counted are not
--    kept in Syndra (usage_last_seen holds each person's latest only), so SQL
--    cannot recount it, and moving a UTC day's total onto one IST day would be
--    wrong for every sign-in between 00:00 and 05:30 IST. Zitadel's own log
--    still holds every sign-in event, so the backend recounts each day up to
--    and including `through` from it, on the makerspace's day, once `through`
--    has ended (handlers.RecountSignInDays), then deletes this row. 2026-11-05
--    is the last day any total was written on the UTC day. Asked for only when
--    such a day has a total.
CREATE TABLE IF NOT EXISTS sign_in_daily_recount (
    through DATE PRIMARY KEY
);

INSERT INTO sign_in_daily_recount (through)
SELECT DATE '2026-11-05'
 WHERE EXISTS (SELECT 1 FROM sign_in_daily WHERE day <= DATE '2026-11-05')
ON CONFLICT DO NOTHING;

-- 3. Hold end and review dates (allowances). The hold and revoke dialogs sent
--    a picked day as UTC midnight, so "until 30 Nov" lifted at 05:30 IST on
--    30 Nov. An end date now means the whole of its day in the makerspace's
--    zone, stored as its last second (internal/clock.EndOfDay). A value at
--    exactly midnight UTC, to the microsecond, can only have come from a
--    picked day (an end counted from now never lands there), so its meaning is
--    unambiguous and it is moved to the end of that day. Only holds still in
--    force and dates still ahead: one that already lifted, or a review already
--    due, is history and stays. The arithmetic is on the epoch, which no zone
--    touches; a moved value is no longer midnight UTC, so a second run moves
--    nothing.
--
--    Not moved, because their meaning is not recoverable: a grant's end made
--    as now + N×24h (direct_role_grants, from any "N days" choice) is an
--    instant with no day attached. Those stay; new ones end as the Nth day does.
UPDATE allowances
   SET expires_at = (DATE '1970-01-01' + (extract(epoch FROM expires_at)::bigint / 86400)::int + 1)::timestamp
                    AT TIME ZONE 'Asia/Kolkata' - interval '1 second'
 WHERE lifted_at IS NULL AND expires_at > now()
   AND extract(epoch FROM expires_at) = floor(extract(epoch FROM expires_at) / 86400) * 86400;

UPDATE allowances
   SET review_date = (DATE '1970-01-01' + (extract(epoch FROM review_date)::bigint / 86400)::int + 1)::timestamp
                     AT TIME ZONE 'Asia/Kolkata' - interval '1 second'
 WHERE lifted_at IS NULL AND review_date > now()
   AND extract(epoch FROM review_date) = floor(extract(epoch FROM review_date) / 86400) * 86400;
