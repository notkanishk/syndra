-- Back to one gap per (dims, start): the scoped ones go, as a 000102 backend
-- would read a totals' or a person's gap as one over every series.
DELETE FROM addon_metric_gaps WHERE scope <> '' OR subject <> '';
ALTER TABLE addon_metric_gaps DROP CONSTRAINT addon_metric_gaps_pkey;
ALTER TABLE addon_metric_gaps ADD PRIMARY KEY (target_id, metric, dims, gap_start);
ALTER TABLE addon_metric_gaps DROP CONSTRAINT IF EXISTS addon_metric_gaps_scope_subject_check;
ALTER TABLE addon_metric_gaps DROP COLUMN IF EXISTS subject;
ALTER TABLE addon_metric_gaps DROP COLUMN IF EXISTS scope;
