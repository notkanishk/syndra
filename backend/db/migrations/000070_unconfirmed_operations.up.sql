-- Changes nobody can confirm, and passwords that settle themselves (A-09 §3–5,
-- revised by A-13 §4–5).
--
-- `status` stays what the dispatch learned: `indeterminate` is still "sent, and
-- the answer was lost", forever. What a person, or a later confirmed change,
-- says about it afterwards is a separate fact written beside it, never over it.
-- That is what keeps a late answer from turning `indeterminate` into
-- `succeeded` on no evidence (SettleAddonOperation's rule), while still letting
-- the question close.

ALTER TABLE addon_operations
    -- A non-secret line naming what the change was about: the account name for
    -- adopt, release and purge, the value for setting.put. Never a password:
    -- Go writes it only for those four operations (addonop.aboutOperations),
    -- and a password-carrying operation's record has it NULL by construction.
    ADD COLUMN IF NOT EXISTS about TEXT,
    -- The page's one automatic resend of a member's own password, after the
    -- first send's answer was lost. Attempt 2 names attempt 1. A member's own
    -- later change is a new change, attempt 1, never a retry.
    ADD COLUMN IF NOT EXISTS attempt SMALLINT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS retry_of UUID REFERENCES addon_operations(id),
    -- How the open question was closed, when it was.
    --   happened / not_happened — an operator said so (Say what happened), or
    --                             Syndra read the answer back from the target.
    --   superseded              — a later confirmed change of the same kind,
    --                             for the same person and system, made it moot.
    ADD COLUMN IF NOT EXISTS resolution TEXT,
    -- Who closed it: the operator's id, `system:syndra` for a read, or the
    -- member whose later change superseded it.
    ADD COLUMN IF NOT EXISTS resolved_by VARCHAR(255),
    ADD COLUMN IF NOT EXISTS resolved_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS superseded_by UUID REFERENCES addon_operations(id);

ALTER TABLE addon_operations
    ADD CONSTRAINT addon_operations_attempt_check CHECK (
        attempt IN (1, 2) AND (attempt = 1) = (retry_of IS NULL)
    ),
    ADD CONSTRAINT addon_operations_resolution_check CHECK (
        (resolution IS NULL OR resolution IN ('happened', 'not_happened', 'superseded'))
        AND (resolution IS NULL) = (resolved_at IS NULL)
        AND (resolution IS NULL) = (resolved_by IS NULL)
        AND (COALESCE(resolution, '') = 'superseded') = (superseded_by IS NOT NULL)
    );

-- Once, and only once: a send has at most one automatic resend. The page
-- enforces it too, but a constraint is a rule for the table.
CREATE UNIQUE INDEX IF NOT EXISTS idx_addon_operations_one_resend
    ON addon_operations (retry_of) WHERE retry_of IS NOT NULL;

-- The per-target Waiting on you read and its count: the rows still open.
CREATE INDEX IF NOT EXISTS idx_addon_operations_open_by_target
    ON addon_operations (target, created_at DESC)
    WHERE status IN ('dispatching', 'indeterminate') AND resolution IS NULL;
