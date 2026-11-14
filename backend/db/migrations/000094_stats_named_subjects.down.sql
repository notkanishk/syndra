-- 000094_stats_named_subjects, reversed: a backend at 000093 never kept a point
-- whose dimension named a person, so a point that names one goes (privacy
-- first: this includes a name the older backend did not check, someone not
-- bound on that target). A gap that names one keeps its window, since the
-- days are still uncounted and a deleted gap would read as zero, with the
-- sentence 000093 stored in place of its reason. Without this, the older
-- backend would show a member what names someone else.
DELETE FROM addon_metric_points WHERE named_subjects <> '{}';
UPDATE addon_metric_gaps SET reason = 'The add-on''s reason named a person.' WHERE named_subjects <> '{}';
ALTER TABLE addon_metric_gaps DROP COLUMN IF EXISTS named_subjects;
ALTER TABLE addon_metric_points DROP COLUMN IF EXISTS named_subjects;
