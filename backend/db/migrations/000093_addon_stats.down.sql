-- 000093_addon_stats, reversed: exactly what the up migration added. The
-- statistics Syndra collected go with it; the add-ons still hold their own
-- buffers, and a backend at 000093 again collects from each metric's buffer.
ALTER TABLE targets DROP COLUMN IF EXISTS stats_subject_retention_days;
DROP TABLE IF EXISTS addon_metric_gaps;
DROP TABLE IF EXISTS addon_metric_points;
DROP TABLE IF EXISTS addon_metrics;
