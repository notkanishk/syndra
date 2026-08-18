-- 000042_target_merge_findings.up.sql
--
-- The differences a reconciliation may not resolve (change
-- `reconciliation-as-merge`).
--
-- With a merge base in place, a pass can finally say WHO changed a value — and
-- two of the answers are not a sweep's to act on. The target moved and Syndra
-- did not; both moved differently; or the account the binding names is gone.
-- Each needs a person, and a state that needs a person has to outlive the pass
-- that found it.
--
-- Left as the return value of a sweep, `theirs_only` is visible to whoever ran
-- that sweep and to nobody else — and it is the most common of the three,
-- because it is what a hand edit on the target looks like. That is the failure
-- this table exists to prevent, and it is the same one `target_binding_conflicts`
-- was written for: a drain report is not a surface, and retention prunes it.
--
-- What it records is a DISAGREEMENT with its history, not a verdict. All three
-- values travel with it, because "what was it before" is the question an
-- operator asks first and, for most targets, one nothing else can answer.
CREATE TABLE IF NOT EXISTS target_merge_findings (
    id         UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    target     TEXT         NOT NULL REFERENCES targets(target),
    subject_id VARCHAR(255) NOT NULL CHECK (btrim(subject_id) <> ''),

    -- The managed field this is about. EMPTY for an account-level finding —
    -- `deleted_upstream` is about the account existing, not about any value —
    -- and empty rather than NULL so the uniqueness index below treats it as one
    -- slot rather than as "no value", which in SQL is never equal to itself.
    field      TEXT         NOT NULL DEFAULT '',

    -- Which of the outcomes this is. Constrained to the three a pass may not
    -- resolve: the other four are not findings, and a row claiming one of them
    -- would be a bug in the classifier rendered as a decision for a human.
    outcome    TEXT         NOT NULL
        CHECK (outcome IN ('theirs_only', 'conflict', 'deleted_upstream')),

    -- The three values, as the classifier saw them. Nullable because
    -- `deleted_upstream` has none: there is no field, and no value to compare.
    base_value   JSONB,
    ours_value   JSONB,
    theirs_value JSONB,

    detected_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Refreshed by every pass that still sees it, so an operator can tell a
    -- finding that is still true from one that has been sitting resolved-by-
    -- circumstance since Tuesday.
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Resolution is a decision with an owner, like every other one here. NULL
    -- while it stands.
    resolved_at  TIMESTAMPTZ,
    resolved_by  VARCHAR(255),
    resolution   TEXT,
    -- What the person chose, in the vocabulary the surface offers. `keep_ours`
    -- applies Syndra's state over the target's; `take_theirs` adopts the
    -- target's value into the desired state through a per-subject decision;
    -- `unbound` is the deleted-upstream answer that stops managing the account.
    -- Constrained, because a free-text resolution is a decision nobody can
    -- count.
    --
    -- `agreed` is the one a person does not choose: the difference stopped
    -- existing, because somebody changed the policy or the target so that the
    -- two sides now match. Closing it is not auto-resolution — nothing was
    -- decided and nothing was written — it is the finding's subject having gone
    -- away, and leaving it open would fill the queue with disagreements that no
    -- longer exist, which is the other way to make a queue unreadable.
    CONSTRAINT merge_finding_resolution_is_known
        CHECK (resolution IS NULL OR resolution IN ('keep_ours', 'take_theirs', 'reprovisioned', 'unbound', 'agreed')),
    -- A resolution is who decided and what they decided, together or not at
    -- all: a resolved row with no actor is a finding that closed itself.
    CONSTRAINT merge_finding_resolution_is_attributed
        CHECK ((resolved_at IS NULL AND resolved_by IS NULL AND resolution IS NULL)
            OR (resolved_at IS NOT NULL AND btrim(resolved_by) <> '' AND resolution IS NOT NULL))
);

-- One standing finding per subject, target and field. A sweep every six hours
-- against one unresolved hand edit must produce one row, not four a day —
-- otherwise the surface reports a single problem as a growing list, which reads
-- as it getting worse.
--
-- A NEW finding on the same field after one was resolved is a separate event
-- and deserves its own row, which is why the index is partial.
CREATE UNIQUE INDEX IF NOT EXISTS idx_merge_finding_open
    ON target_merge_findings (target, subject_id, field)
    WHERE resolved_at IS NULL;

-- The surface's read: what is still standing on this target, newest first.
CREATE INDEX IF NOT EXISTS idx_merge_finding_by_target
    ON target_merge_findings (target, detected_at DESC)
    WHERE resolved_at IS NULL;
