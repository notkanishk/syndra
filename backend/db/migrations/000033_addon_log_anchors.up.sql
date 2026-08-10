-- 000033_addon_log_anchors.up.sql
-- Add-on platform 2.28: anchoring the add-on's mutation log.
--
-- The mutation log is an append-only, hash-chained record the add-on keeps of
-- every write it performed. It survives loss or tampering of Syndra's own audit
-- tables, which is the whole reason it exists — and that reason is worth exactly
-- as much as the log's resistance to being trimmed by whoever holds the add-on.
--
-- A chain verifies its own contents and cannot notice its own TRUNCATION: cut
-- the first thousand records off and re-chain from there and every remaining
-- link still verifies. What catches that is an outside observer remembering
-- where the head was. So the backend records the head digest and the record
-- count each time it reads them, and refuses to move the anchor when the new
-- pair is not an extension of the old one.
--
-- Two violations, and they are different accusations:
--   * the count went DOWN — records that existed are gone;
--   * the count held and the head MOVED — the same number of records now hash
--     to something else, so the content was rewritten.
--
-- The anchor deliberately does not advance past a violation. Advancing would
-- adopt the tampered state as the new baseline and report every subsequent read
-- as healthy — which is the one thing an anchor must never do, since the whole
-- mechanism is a memory of what was true before.

CREATE TABLE IF NOT EXISTS addon_log_anchors (
    target       TEXT        PRIMARY KEY REFERENCES targets(target),
    -- The chain head as the add-on last reported it, and the number of records
    -- behind it. Both, because either alone is defeatable: a head with no count
    -- cannot see a truncation that re-chained, and a count with no head cannot
    -- see a rewrite that kept the length.
    head         TEXT        NOT NULL,
    records      BIGINT      NOT NULL CHECK (records >= 0),
    anchored_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- What the add-on reported when the anchor refused to move, and when. Kept
    -- ON the row rather than in a log line, because a finding that lives only in
    -- a log is a finding nobody is looking at.
    violation_reason TEXT,
    violation_head   TEXT,
    violation_records BIGINT,
    violation_at     TIMESTAMPTZ,

    -- A closed vocabulary, checked here as well as in Go. This column is read by
    -- an operator surface, and free text on a path that reports tampering is a
    -- channel for whatever the least trusted component chose to send.
    CONSTRAINT addon_log_anchors_reason_check CHECK (
        violation_reason IS NULL OR violation_reason IN ('records_decreased', 'head_rewritten')
    ),
    -- The four violation columns are one fact and move together. A reason with
    -- no timestamp, or a timestamp with no reason, describes a moment that did
    -- not happen.
    CONSTRAINT addon_log_anchors_violation_is_whole CHECK (
        (violation_reason IS NULL AND violation_at IS NULL AND violation_head IS NULL AND violation_records IS NULL)
        OR
        (violation_reason IS NOT NULL AND violation_at IS NOT NULL AND violation_head IS NOT NULL AND violation_records IS NOT NULL)
    )
);
