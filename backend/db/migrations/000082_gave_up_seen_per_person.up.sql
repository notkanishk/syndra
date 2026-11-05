-- 000082_gave_up_seen_per_person
--
-- "Mark seen" for changes Syndra gave up on was one marker for the whole team
-- (config_settings.retries_gave_up_seen_at). It is now per person: each
-- operator sees a give-up until they themselves mark it seen.
CREATE TABLE IF NOT EXISTS gave_up_seen (
    actor   TEXT        PRIMARY KEY,
    seen_at TIMESTAMPTZ NOT NULL
);

DELETE FROM config_settings WHERE key = 'retries_gave_up_seen_at';
