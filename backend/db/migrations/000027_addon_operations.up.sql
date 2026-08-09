-- Add-on platform, task 2.11 — the operation plane's record table.
--
-- Two planes reach a target. Entitlements are level-triggered desired state and
-- ride the outbox: durable, retried, retained. Operations are one-shot events —
-- set a password, rotate a credential, purge an account — and they MUST NOT.
-- An outbox row is a durable instruction that a drain will re-attempt, and
-- re-attempting an operation means retaining its secret to re-send it. The
-- moment a password is durable it is a vault, and the vault is the thing this
-- design set out not to build.
--
-- So the operation leaves a record instead of an instruction: who asked, on
-- whom, which operation, and how it ended — committed before the call so a
-- crash mid-dispatch leaves evidence, and never carrying anything that could be
-- replayed.
--
-- There is deliberately NO column for parameter values, and no free-text or
-- JSON column at all. Not "a column we agree not to write secrets into" — one
-- that cannot hold them, because the agreement is what fails. A `failure_detail`
-- or `response_body` column is precisely where a future maintainer would put an
-- add-on's error payload, and an add-on's error payload is the most likely place
-- for a submitted password to be echoed back. `status` is the outcome at the
-- granularity an operator can act on; anything finer belongs in a log line, not
-- in a row that lives beside a subject's identity forever.

-- The five statuses are the four dispatch outcomes plus the pre-dispatch one.
--
--   dispatching   — the record is committed and the call has not yet answered.
--                   Non-terminal. A row left here is a crash between dispatch
--                   and the terminal write, and is the honest state for one:
--                   the add-on may or may not have acted.
--   succeeded     — the add-on applied it and said so.
--   rejected      — the add-on refused it and did not act.
--   unreached     — the call never arrived. Nothing happened on the target.
--   indeterminate — sent, answer lost. May have applied.
--
-- `unreached` and `rejected` are separate because they are different sentences
-- to a member: "nothing happened, try again" versus "the target refused this".
-- `indeterminate` and `dispatching` are separate because one is a recorded
-- answer and the other is the absence of one, and merging them would lose which
-- of the two a row is.
CREATE TABLE IF NOT EXISTS addon_operations (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    target     TEXT NOT NULL REFERENCES targets(target),
    -- The manifest's operation name. Not constrained by a CHECK, for the same
    -- reason `targets` is a table rather than one: an add-on version offering a
    -- further operation must not require a schema migration. Backend policy is
    -- what bounds this, and policy is Go.
    operation  TEXT NOT NULL,
    actor_id   VARCHAR(255) NOT NULL,
    subject_id VARCHAR(255) NOT NULL,
    status     TEXT NOT NULL DEFAULT 'dispatching'
        CHECK (status IN ('dispatching', 'succeeded', 'rejected', 'unreached', 'indeterminate')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    settled_at  TIMESTAMPTZ,
    -- Settled exactly when terminal, in both directions. A terminal row with no
    -- settlement time cannot be aged, and a `dispatching` row carrying one is a
    -- row that was settled and then reopened, which nothing may do.
    CONSTRAINT addon_operations_settled_check CHECK (
        (status = 'dispatching') = (settled_at IS NULL)
    )
);

-- The unresolved surface reads exactly this predicate: rows that are either
-- awaiting an answer or recorded as having lost one. Partial, because those are
-- the rare rows and the whole table is the common case.
CREATE INDEX IF NOT EXISTS idx_addon_operations_unresolved
    ON addon_operations (created_at)
    WHERE status IN ('dispatching', 'indeterminate');

-- A member's own operation history, and the per-subject rate limit that will
-- read the same key (task 2.49).
CREATE INDEX IF NOT EXISTS idx_addon_operations_subject
    ON addon_operations (subject_id, created_at DESC);
