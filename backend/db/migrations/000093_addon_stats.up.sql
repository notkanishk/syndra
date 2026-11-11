-- 000093_addon_stats (addon-contract 6.7, design §9.6)
--
-- Syndra's own copy of the statistics add-ons report (feature stats): what a
-- metric means, the daily points, and the windows nobody could observe.
--
-- addon_metrics is the stored meaning of a metric name (ruling B3, the
-- targets.field_roles pattern): written at registration from the accepted
-- manifest, never deleted, so a name never comes back meaning something else
-- and the meaning holds while the add-on is down. It also carries the
-- metric's watermark (the makerspace midnight through which Syndra holds it
-- final), written last in each collection, and what the last collection
-- dropped. Points and gaps reference it: a descriptor before any point is a
-- constraint, not a convention.
--
-- A point is one makerspace day (bucket_start, a date in the one zone; the
-- end is the next day) of one series: subject '' is the total, which includes
-- everyone; dims {} is a metric without dimensions (a primary key column
-- cannot be NULL). jsonb equality ignores key order, so a series is one key
-- however the add-on spelled it. Values are never negative (ruling O2).
--
-- stats_subject_retention_days shortens how long per-person points are kept
-- on one target (ruling O4): NULL is the ceiling, 400 days, and the CHECK is
-- the ceiling's one definition, so the setting can shorten and never
-- lengthen. Totals and gaps are kept five years and have no setting.
--
-- Expand-only: new tables and a nullable column; an older backend ignores all
-- of it.
CREATE TABLE IF NOT EXISTS addon_metrics (
    target_id         TEXT        NOT NULL REFERENCES targets (target),
    metric            TEXT        NOT NULL,
    descriptor        JSONB       NOT NULL,
    watermark         TIMESTAMPTZ,
    last_collected_at TIMESTAMPTZ,
    unbound_accounts  INT         NOT NULL DEFAULT 0 CHECK (unbound_accounts >= 0),
    dropped           JSONB       NOT NULL DEFAULT '{}',
    PRIMARY KEY (target_id, metric)
);

CREATE TABLE IF NOT EXISTS addon_metric_points (
    target_id    TEXT        NOT NULL,
    metric       TEXT        NOT NULL,
    subject      TEXT        NOT NULL DEFAULT '',
    dims         JSONB       NOT NULL DEFAULT '{}',
    bucket_start DATE        NOT NULL,
    value        NUMERIC     NOT NULL CHECK (value >= 0),
    observed_at  TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (target_id, metric, subject, dims, bucket_start),
    FOREIGN KEY (target_id, metric) REFERENCES addon_metrics (target_id, metric)
);

-- The nightly retention deletes by day across every target.
CREATE INDEX IF NOT EXISTS idx_addon_metric_points_bucket_start ON addon_metric_points (bucket_start);

CREATE TABLE IF NOT EXISTS addon_metric_gaps (
    target_id TEXT  NOT NULL,
    metric    TEXT  NOT NULL,
    dims      JSONB NOT NULL DEFAULT '{}',
    gap_start DATE  NOT NULL,
    gap_end   DATE  NOT NULL,
    reason    TEXT  NOT NULL CHECK (char_length(reason) BETWEEN 1 AND 200),
    PRIMARY KEY (target_id, metric, dims, gap_start),
    CHECK (gap_end > gap_start),
    FOREIGN KEY (target_id, metric) REFERENCES addon_metrics (target_id, metric)
);

ALTER TABLE targets ADD COLUMN IF NOT EXISTS stats_subject_retention_days INT
    CONSTRAINT targets_stats_subject_retention_days_check CHECK (stats_subject_retention_days BETWEEN 1 AND 400);
