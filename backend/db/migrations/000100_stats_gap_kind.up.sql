-- 000100_stats_gap_kind (addon-contract 7s; the owner's ruling of 18 Nov 2026,
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
-- same kind, so a 000099 backend still running beside it stores a gap rather
-- than failing its collection.
ALTER TABLE addon_metric_gaps ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'unread'
    CONSTRAINT addon_metric_gaps_kind_check CHECK (kind IN ('began', 'unread', 'dropped', 'unobserved'));

UPDATE addon_metric_gaps SET kind = CASE
    WHEN reason LIKE 'Not collected yet: counting began on %' THEN 'began'
    WHEN reason IN ('The add-on no longer holds these days', 'The NAS no longer keeps its log of these days') THEN 'dropped'
    WHEN reason = 'Auditing does not record everyone on this share' THEN 'unobserved'
    ELSE 'unread'
  END;
