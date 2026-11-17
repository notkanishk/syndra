-- 000101_stats_gap_kind (addon-contract 7s; the owner's ruling of 18 Nov 2026,
-- "label each gap")
--
-- Every statistics gap carries the add-on's kind (contract 2, stats.json):
-- began (before the add-on started counting the metric), unread (the app could
-- not be read), dropped (purged, or older than the buffer holds) or unobserved
-- (the app does not record the window). Syndra reads the kind and never parses
-- the reason; a began gap is drawn as before the record, not as a gap.
--
-- Backfill: every gap stored so far came from the TrueNAS add-on or the
-- reference, whose reasons are fixed sentences, so each takes the kind its
-- sentence was written for (addons/truenas/stats.go gapKinds and reasonBegan;
-- addons/reference/stats.go). Any other sentence is unread, the kind that
-- claims least: a window not observed, hatched and listed. The default is the
-- same kind, so a 000100 backend still running beside it stores a gap rather
-- than failing its collection.
ALTER TABLE addon_metric_gaps ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'unread'
    CONSTRAINT addon_metric_gaps_kind_check CHECK (kind IN ('began', 'unread', 'dropped', 'unobserved'));

UPDATE addon_metric_gaps SET kind = CASE
    WHEN reason LIKE 'Not collected yet: counting began on %' THEN 'began'
    WHEN reason IN ('The add-on no longer holds these days', 'The NAS no longer keeps its log of these days') THEN 'dropped'
    WHEN reason = 'Auditing does not record everyone on this share' THEN 'unobserved'
    ELSE 'unread'
  END;

-- A gap the add-on sent is never lost (7s review): one with no reason is kept,
-- as unread, its reason empty (the page says "no reason given"), so its days
-- are never read as counted zeros. The reason's check loosens to allow it; a
-- 000100 backend's writes still satisfy it. 000093 named the check
-- addon_metric_gaps_reason_check (an inline CHECK on reason).
ALTER TABLE addon_metric_gaps DROP CONSTRAINT IF EXISTS addon_metric_gaps_reason_check;
ALTER TABLE addon_metric_gaps ADD CONSTRAINT addon_metric_gaps_reason_check CHECK (char_length(reason) <= 200);
