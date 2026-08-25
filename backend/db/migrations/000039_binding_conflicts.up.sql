-- Two records disagreeing about who owns an account (§29).
--
-- The accounting half made a refused binding settle terminally instead of
-- `applied`, which stopped the lie. It did not give the finding anywhere to
-- live: the row lands among ordinary drain failures, where "the target refused
-- this call" and "two of your records disagree about who owns an account" read
-- the same to anybody scanning, and want completely different actions.
--
-- A drain report is not a surface. An operator who was not watching that pass
-- never sees it, and retention eventually prunes the row — so the finding is
-- persisted the way the log anchor's is, and for the same reason: the evidence
-- must outlive the moment it was produced.
--
-- What it records is the disagreement, not a verdict. Syndra does not know
-- which subject the account belongs to — that is the whole content of the
-- finding — so both are named and an operator decides.
CREATE TABLE IF NOT EXISTS target_binding_conflicts (
    id           UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    target       TEXT         NOT NULL REFERENCES targets(target),
    -- The account the add-on wrote to.
    username     TEXT         NOT NULL CHECK (btrim(username) <> ''),
    account_uid  BIGINT,
    -- The subject whose convergence landed on it, and the subject Syndra's own
    -- binding attributes it to. Named apart rather than as "expected/actual":
    -- neither is authoritative, which is why this needs a person.
    converged_subject_id VARCHAR(255) NOT NULL CHECK (btrim(converged_subject_id) <> ''),
    bound_subject_id     VARCHAR(255) NOT NULL CHECK (btrim(bound_subject_id) <> ''),
    -- The outbox row that produced it, so the finding traces back to the change
    -- that caused it rather than appearing out of nowhere.
    outbox_id    UUID         NOT NULL,
    detected_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    -- Resolution is a decision with an owner, like every other irreversible one
    -- here. NULL while it stands.
    resolved_at  TIMESTAMPTZ,
    resolved_by  VARCHAR(255),
    resolution   TEXT,
    -- A resolution is who decided and what they decided, together or not at
    -- all: a resolved row with no actor is a finding that closed itself.
    CONSTRAINT binding_conflict_resolution_is_attributed
        CHECK ((resolved_at IS NULL AND resolved_by IS NULL AND resolution IS NULL)
            OR (resolved_at IS NOT NULL AND btrim(resolved_by) <> '' AND btrim(resolution) <> '')),
    -- The two subjects must differ, or this is not a conflict. A row saying a
    -- subject conflicts with themselves would be a bug in the detector rendered
    -- as a finding about a person.
    CONSTRAINT binding_conflict_is_between_two_subjects
        CHECK (converged_subject_id <> bound_subject_id)
);

-- One standing finding per account. A drain that re-drives the same row must
-- not stack duplicates on the same disagreement, and a NEW conflict on the same
-- account after one was resolved is a separate event that deserves its own row.
CREATE UNIQUE INDEX IF NOT EXISTS idx_binding_conflict_open
    ON target_binding_conflicts (target, username)
    WHERE resolved_at IS NULL;

-- The surface's read: what is still standing on this target.
CREATE INDEX IF NOT EXISTS idx_binding_conflict_by_target
    ON target_binding_conflicts (target, detected_at DESC)
    WHERE resolved_at IS NULL;
