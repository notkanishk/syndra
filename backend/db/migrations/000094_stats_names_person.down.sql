-- 000094_stats_names_person, reversed: back to what a backend at 000093
-- holds, which never kept anything naming a person. A flagged point is one it
-- would have dropped, so it goes. A flagged gap whose dimensions no remaining
-- point holds (jsonb @>) is on a series that named a person, which 000093
-- dropped with its points, so it goes too; any other flagged gap was flagged
-- for its reason and keeps its window (the days are still uncounted) with the
-- sentence 000093 stored in place of that reason. Without this, the older
-- backend would show what it never stored.
DELETE FROM addon_metric_points WHERE names_person;
DELETE FROM addon_metric_gaps g WHERE g.names_person AND g.dims <> '{}'::jsonb
   AND NOT EXISTS (SELECT 1 FROM addon_metric_points p WHERE p.target_id = g.target_id AND p.metric = g.metric AND p.dims @> g.dims);
UPDATE addon_metric_gaps SET reason = 'The add-on''s reason named a person.' WHERE names_person;
ALTER TABLE addon_metric_gaps DROP COLUMN IF EXISTS names_person;
ALTER TABLE addon_metric_points DROP COLUMN IF EXISTS names_person;
