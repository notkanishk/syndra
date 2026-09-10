-- The store stops being a cache written by notifications and becomes a record
-- of what Syndra ASKED and what Zitadel ANSWERED.
--
-- `zitadel_grants_index` was maintained by webhooks and by nothing else. The
-- self-mutation guard drops Syndra's own changes, so it could only ever mirror
-- other people's — and it went on naming grants Syndra itself had revoked. It
-- was then read as evidence that a grant already existed, and a call was
-- skipped on its word. Nothing in the schema said it was a cache, so nothing
-- stopped it being read as truth.
--
-- Two things have to be recorded for the store to be readable honestly, and
-- neither could be expressed before.
CREATE TABLE IF NOT EXISTS zitadel_observations (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    -- 'org' covers everybody; 'user' covers one person. Same question, two
    -- scopes — a per-person read is not a different mechanism, it is a
    -- narrower observation, and it is recorded the same way.
    scope        TEXT NOT NULL,
    subject_id   TEXT,
    observed_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- WHETHER THE ANSWER WAS WHOLE. This is the field the product could not do
    -- without: presence can be concluded from any answer, and ABSENCE can only
    -- be concluded from a complete one. A read that hit its cap, or failed
    -- part-way, is real about what it saw and says nothing about what it did
    -- not — and treating it as a full picture is how a capped listing becomes
    -- a claim that somebody has lost access they still hold.
    complete     BOOLEAN NOT NULL,
    grants_seen  INTEGER NOT NULL DEFAULT 0,
    error        TEXT,
    CONSTRAINT ck_zitadel_observations_scope CHECK (scope IN ('org', 'user')),
    CONSTRAINT ck_zitadel_observations_subject
        CHECK ((scope = 'org' AND subject_id IS NULL)
            OR (scope = 'user' AND subject_id IS NOT NULL))
);

-- The reader always wants the most recent covering observation, so the index
-- is on the ordering it is fetched by.
CREATE INDEX IF NOT EXISTS idx_zitadel_observations_recent
    ON zitadel_observations (scope, subject_id, observed_at DESC);

-- When THIS row was last seen, as distinct from when it was written. A row
-- carried forward by an incomplete sweep keeps its own age rather than
-- inheriting the freshness of the pass that failed to cover it.
ALTER TABLE zitadel_grants_index
    ADD COLUMN IF NOT EXISTS observed_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
