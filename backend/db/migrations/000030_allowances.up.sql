-- 000030_allowances.up.sql
-- Add-on platform, group 8: the second layer of the access model (change
-- `addon-platform`, design §6).
--
-- Layer 1 is the Zitadel role, mapped to a target entitlement by 000029. Layer
-- 2 is this: an explicit per-user overlay, never inferred, so that "why does
-- this person have access to X" answers with exactly one of — the role gives
-- it, a rule derived it, or somebody granted it, with actor and time.
--
-- Phase 1 ships the SUBTRACTIVE half only. `direction` exists anyway because
-- schema generality is the cheap part and migrating to add a column later is
-- not; what defers is the code — the additive resolver arm, additive authoring
-- and additive lineage land with quotas, when a second consumer makes the
-- abstraction real rather than anticipated.
CREATE TABLE IF NOT EXISTS allowances (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    subject_id VARCHAR(255) NOT NULL,
    target     TEXT NOT NULL REFERENCES targets(target),
    field      VARCHAR(255) NOT NULL,
    value      TEXT NOT NULL,
    -- `deny` removes an entitlement the role layer would otherwise confer.
    -- `allow` is the additive arm, accepted by the schema and refused by the
    -- code until phase 2.
    direction  TEXT NOT NULL CHECK (direction IN ('allow', 'deny')),

    -- Never a bare absence. An allowance answers "who decided this, and why",
    -- and a carve-out with no author is the thing this layer exists to replace.
    actor_id   VARCHAR(255) NOT NULL,
    reason     TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    expires_at  TIMESTAMPTZ,
    review_date TIMESTAMPTZ,
    -- Set when the allowance stops applying, so history survives the lapse. An
    -- allowance is a decision somebody took; deleting it on expiry erases the
    -- only record that the suspension ever happened.
    lifted_at   TIMESTAMPTZ,
    lifted_by   VARCHAR(255),

    -- A subtractive allowance MUST be bounded in time, by an expiry or by a
    -- review date (design §6).
    --
    -- A deny is normally a time-boxed suspension and expires on its own. Some
    -- are genuinely indefinite — an open incident, a safety ban with no agreed
    -- end — and those may omit the expiry only by carrying a review date, which
    -- surfaces in governance when it passes. What is forbidden is neither: an
    -- open-ended carve-out nobody is ever prompted to revisit is how a
    -- temporary measure becomes permanent by inattention.
    --
    -- The CHECK is on `deny` alone. An additive allowance is somebody being
    -- given something, which does not rot the same way.
    CONSTRAINT allowances_denial_is_bounded CHECK (
        direction <> 'deny' OR expires_at IS NOT NULL OR review_date IS NOT NULL
    ),
    -- Lifting is one act with two facts, and half of it is a row that cannot be
    -- rendered.
    CONSTRAINT allowances_lifted_pair CHECK (
        (lifted_at IS NULL AND lifted_by IS NULL) OR
        (lifted_at IS NOT NULL AND lifted_by IS NOT NULL)
    )
);

-- The resolver's read: everything in force for one subject on one target.
-- Partial, because a lifted allowance is history and the resolver never asks
-- for it.
CREATE INDEX IF NOT EXISTS idx_allowances_in_force
    ON allowances (subject_id, target) WHERE lifted_at IS NULL;

-- Governance's read: denials whose review date has passed and which nobody has
-- decided about. An indefinite suspension stays in force until it is decided —
-- surfacing is a prompt, never a lapse.
CREATE INDEX IF NOT EXISTS idx_allowances_review_due
    ON allowances (review_date)
    WHERE lifted_at IS NULL AND direction = 'deny' AND review_date IS NOT NULL;

-- The expiry sweep's read.
CREATE INDEX IF NOT EXISTS idx_allowances_expiring
    ON allowances (expires_at) WHERE lifted_at IS NULL AND expires_at IS NOT NULL;
