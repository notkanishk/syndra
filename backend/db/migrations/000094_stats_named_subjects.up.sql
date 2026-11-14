-- 000094_stats_named_subjects (addon-contract 7i, the owner's ruling of 14 Nov
-- 2026: usage is traceable to a person, for admins only)
--
-- Before this, collection dropped a series whose dimension value named a bound
-- person and withheld a gap reason that did. Now both are kept as the add-on
-- sent them, and the same matcher records at ingestion whom the row names:
-- named_subjects, the subject ids of everyone in the directory a dimension
-- value (or a gap's reason) names, '{}' for nobody. Admins read every row; a
-- member's own usage leaves out a series that names anyone but them and
-- replaces such a gap's reason (handlers/stats.go), so a member never sees
-- another person's name, and their own name never hides their own line.
--
-- Expand-only: two columns with a default every existing row satisfies (it
-- was stored by a backend that kept nothing naming a person), and an older
-- backend ignores them.
ALTER TABLE addon_metric_points ADD COLUMN IF NOT EXISTS named_subjects TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE addon_metric_gaps ADD COLUMN IF NOT EXISTS named_subjects TEXT[] NOT NULL DEFAULT '{}';
