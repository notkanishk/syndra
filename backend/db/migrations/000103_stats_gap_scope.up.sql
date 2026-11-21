-- 000103_stats_gap_scope (addon-contract PR 246; the owner's ruling of
-- 21 Nov 2026, "Only that home affected")
--
-- A statistics gap may cover the metric's totals alone (scope 'total': one
-- part of the app not read leaves the whole partial while every person's own
-- figures stand) or one person's own points alone (subject: what of theirs
-- was not counted). Neither, as every gap stored so far: every series at its
-- dimensions. Never both. A gap's key takes both, so one day can hold the
-- totals' gap and a person's beside a share's own. A 000102 backend still
-- running beside it fails its gap upsert (its ON CONFLICT names the old key):
-- that collection rolls back whole and is read again on the next run.
ALTER TABLE addon_metric_gaps ADD COLUMN IF NOT EXISTS scope TEXT NOT NULL DEFAULT ''
    CONSTRAINT addon_metric_gaps_scope_check CHECK (scope IN ('', 'total'));
ALTER TABLE addon_metric_gaps ADD COLUMN IF NOT EXISTS subject TEXT NOT NULL DEFAULT '';
ALTER TABLE addon_metric_gaps DROP CONSTRAINT IF EXISTS addon_metric_gaps_scope_subject_check;
ALTER TABLE addon_metric_gaps ADD CONSTRAINT addon_metric_gaps_scope_subject_check CHECK (scope = '' OR subject = '');

ALTER TABLE addon_metric_gaps DROP CONSTRAINT addon_metric_gaps_pkey;
ALTER TABLE addon_metric_gaps ADD PRIMARY KEY (target_id, metric, dims, scope, subject, gap_start);
