-- 000077_action_key_rotations
--
-- Syndra replaces the Zitadel Actions v2 signing keys itself
-- (action-key-autorotation, ADR-005). One row per attempt, written BEFORE the
-- first Management API call (the trace CLAUDE.md requires of every
-- Syndra-mediated Zitadel mutation) and finished after it.
--
-- targets is what the attempt set out to replace; replaced is what Zitadel
-- did replace, appended the moment each answer arrives, so a crash after
-- Zitadel answered still says which keys changed. outcome:
--   in_flight  : running, or interrupted (the next attempt, holding the lock,
--                marks a leftover one abandoned)
--   verified   : every target replaced and the test sign-in passed
--   unverified : replaced, but the test sign-in did not pass (both keys stay
--                accepted, a claims finding is open, nothing retries)
--   failed     : Zitadel did not replace every target
--   abandoned  : interrupted before it finished
-- No key is ever stored here: the keys live only in Redis (claims:keyring,
-- events:keyring) and Zitadel.
CREATE TABLE IF NOT EXISTS action_key_rotations (
    id          BIGSERIAL   PRIMARY KEY,
    trigger     TEXT        NOT NULL CHECK (trigger IN ('auto', 'button')),
    actor       TEXT        NOT NULL CHECK (btrim(actor) <> ''),
    targets     TEXT[]      NOT NULL CHECK (cardinality(targets) > 0),
    replaced    TEXT[]      NOT NULL DEFAULT '{}',
    started_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ,
    outcome     TEXT        NOT NULL DEFAULT 'in_flight'
                CHECK (outcome IN ('in_flight', 'verified', 'unverified', 'failed', 'abandoned')),
    probe       JSONB,
    error       TEXT,
    CHECK ((outcome = 'in_flight') = (finished_at IS NULL))
);

CREATE INDEX IF NOT EXISTS action_key_rotations_started_idx ON action_key_rotations (started_at DESC);
