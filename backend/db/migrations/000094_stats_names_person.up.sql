-- 000094_stats_names_person (addon-contract 7i, the owner's ruling of 14 Nov
-- 2026: usage is traceable to a person, for admins only)
--
-- Before this, collection dropped a series whose dimension value named a bound
-- person and withheld a gap reason that did. Now both are kept as the add-on
-- sent them, and the same matcher records at ingestion whether the row names
-- a person: names_person. Admins read every row; a member's own usage leaves
-- a flagged series out and replaces a flagged gap's reason (handlers/stats.go),
-- so a member never sees another person's name.
--
-- A point's flag is its dimension values'; a gap's, its dimension values' or
-- its reason's.
--
-- Expand-only: two columns with a default every existing row satisfies (it
-- was stored by a backend that kept nothing naming a person), and an older
-- backend ignores them.
ALTER TABLE addon_metric_points ADD COLUMN IF NOT EXISTS names_person BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE addon_metric_gaps ADD COLUMN IF NOT EXISTS names_person BOOLEAN NOT NULL DEFAULT false;
