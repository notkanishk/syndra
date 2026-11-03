-- Sign-ins per app (A-10 Part 2; owner, 3 Nov: "1a with statistics").
--
-- 1. usage_last_seen gains the app. One row per person, project, app and kind,
--    still overwritten with the later time. Existing rows become "app unknown"
--    for their project (app_id ''): they keep saying the project was used, and
--    name no app.
-- 2. sign_in_daily: one number per app per day, naming nobody. Written in the
--    same statement as the last-seen row and only when that row moved strictly
--    later, so a redelivered event is never counted twice. app_id '' is
--    "app unknown". Pruned with usage_last_seen (13 months).
ALTER TABLE usage_last_seen ADD COLUMN app_id VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE usage_last_seen DROP CONSTRAINT usage_last_seen_pkey;
ALTER TABLE usage_last_seen ADD PRIMARY KEY (user_id, project_id, app_id, kind);
CREATE INDEX usage_last_seen_app_idx ON usage_last_seen (app_id, last_at) WHERE app_id <> '';

CREATE TABLE sign_in_daily (
    app_id VARCHAR(255) NOT NULL,
    day    DATE NOT NULL,
    count  INTEGER NOT NULL CHECK (count > 0),
    PRIMARY KEY (app_id, day)
);
