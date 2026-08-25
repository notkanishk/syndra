-- 000044_target_propagations.up.sql
--
-- The memory that a write LANDED (change `reconciliation-as-merge`).
--
-- Reconciliation can now tell "somebody removed this" from "the write never
-- happened" — but only for a grant a complete sweep had already observed. A
-- grant applied and removed between two sweeps had nothing behind it: the outbox
-- has no `confirmed` state by design and its terminal rows are pruned, and
-- `zitadel_grants_index` is deleted by the very event that removes a grant. So
-- the strongest evidence Syndra ever has — the target ACCEPTED this write, at a
-- known time, for a named person's decision — was thrown away within the
-- retention window.
--
-- This is that evidence, kept. One row per (target, subject, field), overwritten
-- by each later success, so it says "the last time this landed" rather than
-- growing without bound.
--
-- It is NOT a merge base. A base is what the target was seen HOLDING at a read;
-- this is what the target ACCEPTED at a write. They answer different questions
-- and disagree in the case that matters: a value applied at noon and removed at
-- one is accepted-and-not-held, which is exactly the removal a base alone cannot
-- see.
CREATE TABLE IF NOT EXISTS target_propagations (
    target     TEXT         NOT NULL REFERENCES targets(target),
    subject_id VARCHAR(255) NOT NULL CHECK (btrim(subject_id) <> ''),

    -- The thing that was written, in the target's own field vocabulary:
    -- `project/role` for Zitadel, an entitlement field for an add-on.
    field      TEXT         NOT NULL CHECK (btrim(field) <> ''),

    -- When the target accepted it, and which outbox row carried it. The row id
    -- outlives the row: retention prunes the outbox, and this keeps the thread
    -- back to what authorised the write even after that.
    applied_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    outbox_id  UUID,

    -- Who the change was attributed to. A propagation with no author answers two
    -- thirds of "who did what to whom", which is the same gap the mutation log
    -- exists to close.
    actor      VARCHAR(255) NOT NULL DEFAULT '',

    PRIMARY KEY (target, subject_id, field)
);

-- The drift queue's read: everything Syndra has landed for one person on one
-- target, for the rows that ask "was this ever really applied".
CREATE INDEX IF NOT EXISTS idx_target_propagations_subject
    ON target_propagations (target, subject_id);
