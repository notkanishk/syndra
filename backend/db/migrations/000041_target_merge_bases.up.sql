-- 000041_target_merge_bases.up.sql
--
-- The third state reconciliation needs and has never had (change
-- `reconciliation-as-merge`).
--
-- Reconciliation compares two things: what Syndra wants and what the target
-- has. Every difference between them is therefore ambiguous — Syndra moved, the
-- target was changed by hand, both moved to the same value, or both moved to
-- different ones — and a two-way diff produces no conflict, only a winner. The
-- winner has always been Syndra, so a hand edit on the NAS is silently reverted
-- and nothing anywhere says it happened.
--
-- This is the merge base: the state the target REPORTED after the last
-- successful apply. Reported, never intended. A base recorded from what Syndra
-- asked for equals the desired state by construction and can never produce a
-- conflict, which is today's behaviour with more machinery — and it is why
-- `apply-reads-back-what-it-wrote` had to land first.
--
-- One row per (target, subject), holding the managed fields only. An unmanaged
-- field is not "unchanged", it is out of scope, and a base claiming authority
-- over something Syndra never set would manufacture findings about values
-- nobody here decided.
CREATE TABLE IF NOT EXISTS target_merge_bases (
    target     TEXT         NOT NULL REFERENCES targets(target),
    subject_id VARCHAR(255) NOT NULL CHECK (btrim(subject_id) <> ''),

    -- field -> the value the target reported. JSONB because the value shape is
    -- the target's, not this schema's: a list of groups, a boolean, and
    -- whatever a later add-on declares.
    base       JSONB        NOT NULL,

    -- When the read that produced it happened. Not `updated_at`: this dates the
    -- OBSERVATION, and an operator asking "what was it before" is asking about
    -- a moment on the target rather than about a row in this database.
    observed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (target, subject_id)
);

-- An empty base is not a base. It would classify every managed field as
-- theirs-only on the first pass — a base that says "the target held nothing"
-- when what happened is that nobody recorded anything.
ALTER TABLE target_merge_bases
    ADD CONSTRAINT target_merge_base_is_not_empty
    CHECK (jsonb_typeof(base) = 'object' AND base <> '{}'::jsonb);
