-- When each person last used each project (Insights "Recent use" and "Held but
-- not used"). One row per person, project and kind, upserted with the later of
-- the two times: no raw event log, so nothing here grows without bound beyond
-- people x projects x kinds, and a daily prune drops what has gone quiet.
--
-- project_id '' means "signed in, app unknown": the sign-in was seen but its
-- application did not resolve to a project. It is kept so "receiving" is honest,
-- and it never counts as use of any particular project.
CREATE TABLE usage_last_seen (
    user_id    VARCHAR(255) NOT NULL,
    project_id VARCHAR(255) NOT NULL DEFAULT '',
    kind       TEXT NOT NULL CHECK (kind IN ('sign_in', 'badge')),
    last_at    TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (user_id, project_id, kind)
);
CREATE INDEX usage_last_seen_last_at_idx ON usage_last_seen (last_at);
