package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Statistics an add-on reports (feature stats, migration 000093, design §9.6):
// the stored meaning of each metric, its daily points and its gaps. Every day
// here is a makerspace day, travelling as YYYY-MM-DD text ($n::date).

// Retention (decision 13, ruling O4), the one definition the nightly prune and
// a late point at ingestion both read, today being $1 and t the target's row:
// a per-person point is kept stats_subject_retention_days (400 when unset; the
// column's CHECK is the ceiling), a total five years, and a gap as long as the
// totals it explains.
const (
	statsSubjectDays     = `COALESCE(t.stats_subject_retention_days, 400)`
	statsSubjectKeptFrom = `$1::date - ` + statsSubjectDays
	statsTotalKeptFrom   = `$1::date - INTERVAL '5 years'`
)

// statsKeptFrom is the first day kept for a point whose subject column is s.
func statsKeptFrom(s string) string {
	return `CASE WHEN ` + s + ` = '' THEN ` + statsTotalKeptFrom + ` ELSE ` + statsSubjectKeptFrom + ` END`
}

// StatsDescriptors is every metric's stored descriptor on a target, by name.
func StatsDescriptors(ctx context.Context, target string) (map[string]json.RawMessage, error) {
	rows, err := querier(ctx).Query(ctx, `SELECT metric, descriptor FROM addon_metrics WHERE target_id = $1`, target)
	if err != nil {
		return nil, fmt.Errorf("read metrics of %s: %w", target, err)
	}
	defer rows.Close()
	out := map[string]json.RawMessage{}
	for rows.Next() {
		var name string
		var raw []byte
		if err := rows.Scan(&name, &raw); err != nil {
			return nil, err
		}
		out[name] = raw
	}
	return out, rows.Err()
}

// SaveStatsDescriptors stores descriptors by name. A row is never deleted, so
// a name keeps its meaning after the add-on stops declaring it.
func SaveStatsDescriptors(ctx context.Context, target string, descriptors map[string]json.RawMessage) error {
	return InTx(ctx, func(tx pgx.Tx) error {
		for name, raw := range descriptors {
			if _, err := tx.Exec(ctx, `
				INSERT INTO addon_metrics (target_id, metric, descriptor) VALUES ($1, $2, $3)
				ON CONFLICT (target_id, metric) DO UPDATE SET descriptor = EXCLUDED.descriptor`,
				target, name, string(raw)); err != nil {
				return fmt.Errorf("store metric %s of %s: %w", name, target, err)
			}
		}
		return nil
	})
}

// StatsPoint is one point as stored: Subject "" is the total, Day the bucket.
type StatsPoint struct {
	Subject string
	Dims    map[string]string
	Day     string
	Value   float64
}

// StatsGap is one unobserved window [Start, End) of a metric's series.
type StatsGap struct {
	Dims       map[string]string
	Start, End string
	Reason     string
}

// StatsTx is one metric's collection, inside one transaction (design §9.6):
// the provisional window replaced, final points upserted, the watermark
// written last. Anything that fails rolls all of it back.
type StatsTx struct {
	tx             pgx.Tx
	target, metric string
}

// CollectStats runs fn as one metric's collection.
func CollectStats(ctx context.Context, target, metric string, fn func(*StatsTx) error) error {
	return InTx(ctx, func(tx pgx.Tx) error { return fn(&StatsTx{tx: tx, target: target, metric: metric}) })
}

// Watermark is the metric's stored watermark, nil before its first
// collection, and locks its row: two collections of one metric take turns.
func (c *StatsTx) Watermark(ctx context.Context) (*time.Time, error) {
	var w *time.Time
	err := c.tx.QueryRow(ctx, `SELECT watermark FROM addon_metrics WHERE target_id = $1 AND metric = $2 FOR UPDATE`,
		c.target, c.metric).Scan(&w)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("metric %s of %s has no stored meaning", c.metric, c.target)
	}
	return w, err
}

// DeleteProvisional deletes what is not final: points whose day ends after
// the watermark (the day `from` on) and gaps that end after it. The reply
// replaces them, so a provisional value that is now absent is gone.
func (c *StatsTx) DeleteProvisional(ctx context.Context, from string) error {
	if _, err := c.tx.Exec(ctx, `DELETE FROM addon_metric_points WHERE target_id = $2 AND metric = $3 AND bucket_start >= $1::date`,
		from, c.target, c.metric); err != nil {
		return fmt.Errorf("delete provisional points: %w", err)
	}
	if _, err := c.tx.Exec(ctx, `DELETE FROM addon_metric_gaps WHERE target_id = $2 AND metric = $3 AND gap_end > $1::date`,
		from, c.target, c.metric); err != nil {
		return fmt.Errorf("delete provisional gaps: %w", err)
	}
	return nil
}

// Series is every dimension set the metric holds points for, as stored jsonb
// decoded: the one source of the cardinality cap.
func (c *StatsTx) Series(ctx context.Context) ([]map[string]string, error) {
	rows, err := c.tx.Query(ctx, `SELECT DISTINCT dims FROM addon_metric_points WHERE target_id = $1 AND metric = $2`, c.target, c.metric)
	if err != nil {
		return nil, fmt.Errorf("read series: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowTo[map[string]string])
}

// UpsertPoints stores points by key (equal keys replace, nothing adds) and
// reports how many it kept: a point already past retention on `today` is not
// stored. Keys must be distinct within one call.
func (c *StatsTx) UpsertPoints(ctx context.Context, today string, observedAt time.Time, points []StatsPoint) (int, error) {
	if len(points) == 0 {
		return 0, nil
	}
	subjects, dims, days, values := make([]string, len(points)), make([]string, len(points)), make([]string, len(points)), make([]float64, len(points))
	for i, p := range points {
		raw, err := json.Marshal(nonNilDims(p.Dims))
		if err != nil {
			return 0, err
		}
		subjects[i], dims[i], days[i], values[i] = p.Subject, string(raw), p.Day, p.Value
	}
	tag, err := c.tx.Exec(ctx, `
		INSERT INTO addon_metric_points (target_id, metric, subject, dims, bucket_start, value, observed_at)
		SELECT $2, $3, x.s, x.d::jsonb, x.b, x.v, $8
		  FROM unnest($4::text[], $5::text[], $6::date[], $7::numeric[]) AS x(s, d, b, v)
		  JOIN targets t ON t.target = $2
		 WHERE x.b >= `+statsKeptFrom("x.s")+`
		ON CONFLICT (target_id, metric, subject, dims, bucket_start)
		DO UPDATE SET value = EXCLUDED.value, observed_at = EXCLUDED.observed_at`,
		today, c.target, c.metric, subjects, dims, days, values, observedAt)
	if err != nil {
		return 0, fmt.Errorf("store points: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// UpsertGaps stores gaps by (dims, start), and reports how many it kept: one
// ending before the totals' retention is not stored.
func (c *StatsTx) UpsertGaps(ctx context.Context, today string, gaps []StatsGap) (int, error) {
	kept := 0
	for _, g := range gaps {
		raw, err := json.Marshal(nonNilDims(g.Dims))
		if err != nil {
			return 0, err
		}
		tag, err := c.tx.Exec(ctx, `
			INSERT INTO addon_metric_gaps (target_id, metric, dims, gap_start, gap_end, reason)
			SELECT $2, $3, $4::jsonb, $5::date, $6::date, $7
			 WHERE $6::date > `+statsTotalKeptFrom+`
			ON CONFLICT (target_id, metric, dims, gap_start)
			DO UPDATE SET gap_end = EXCLUDED.gap_end, reason = EXCLUDED.reason`,
			today, c.target, c.metric, string(raw), g.Start, g.End, g.Reason)
		if err != nil {
			return 0, fmt.Errorf("store gap: %w", err)
		}
		kept += int(tag.RowsAffected())
	}
	return kept, nil
}

// Finish writes the watermark, last: never backwards (GREATEST, which a NULL
// first watermark does not hold back), with when this collection ran, the
// unbound accounts it was told of and what it dropped, by reason.
func (c *StatsTx) Finish(ctx context.Context, watermark, at time.Time, unbound int, dropped map[string]int) error {
	raw, err := json.Marshal(dropped)
	if err != nil {
		return err
	}
	_, err = c.tx.Exec(ctx, `
		UPDATE addon_metrics SET watermark = GREATEST(watermark, $3), last_collected_at = $4,
		       unbound_accounts = $5, dropped = $6
		 WHERE target_id = $1 AND metric = $2`,
		c.target, c.metric, watermark, at, unbound, string(raw))
	if err != nil {
		return fmt.Errorf("write watermark: %w", err)
	}
	return nil
}

func nonNilDims(d map[string]string) map[string]string {
	if d == nil {
		return map[string]string{}
	}
	return d
}

// PruneStats deletes what retention no longer keeps on `today` (a makerspace
// day): per-person points past their target's retention, totals and gaps
// past five years. Nightly; returns how many points and gaps went.
func PruneStats(ctx context.Context, today string) (points, gaps int64, err error) {
	tag, err := querier(ctx).Exec(ctx, `
		DELETE FROM addon_metric_points p USING targets t
		 WHERE t.target = p.target_id AND p.bucket_start < `+statsKeptFrom("p.subject"), today)
	if err != nil {
		return 0, 0, fmt.Errorf("prune points: %w", err)
	}
	points = tag.RowsAffected()
	tag, err = querier(ctx).Exec(ctx, `DELETE FROM addon_metric_gaps WHERE gap_end <= `+statsTotalKeptFrom, today)
	if err != nil {
		return points, 0, fmt.Errorf("prune gaps: %w", err)
	}
	return points, tag.RowsAffected(), nil
}

// ErrStatsRetentionInvalid is a retention the schema refuses: not 1 to 400
// days (targets_stats_subject_retention_days_check, the rule's one place).
var ErrStatsRetentionInvalid = errors.New("Per-person statistics are kept between 1 and 400 days.")

// StatsRetentionAudit is the audit action a retention change writes.
const StatsRetentionAudit = "target.stats_retention_set"

// StatsRetention is how many days a target keeps per-person points (the
// setting, else the ceiling) and whether an operator set it.
func StatsRetention(ctx context.Context, target string) (days int, set bool, err error) {
	var setting *int
	err = querier(ctx).QueryRow(ctx, `
		SELECT t.stats_subject_retention_days, `+statsSubjectDays+` FROM targets t WHERE t.target = $1`,
		target).Scan(&setting, &days)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, fmt.Errorf("no target %q", target)
	}
	return days, setting != nil, err
}

// SetStatsRetention sets, or with nil clears, how many days a target keeps
// per-person points, audited in the same transaction.
func SetStatsRetention(ctx context.Context, target string, days *int, actor string) error {
	return InTx(ctx, func(tx pgx.Tx) error {
		var from *int
		err := tx.QueryRow(ctx, `SELECT stats_subject_retention_days FROM targets WHERE target = $1 FOR UPDATE`, target).Scan(&from)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("no target %q", target)
		}
		if err != nil {
			return fmt.Errorf("read retention of %s: %w", target, err)
		}
		_, err = tx.Exec(ctx, `UPDATE targets SET stats_subject_retention_days = $2 WHERE target = $1`, target, days)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23514" && pgErr.ConstraintName == "targets_stats_subject_retention_days_check" {
			return ErrStatsRetentionInvalid
		}
		if err != nil {
			return fmt.Errorf("set retention of %s: %w", target, err)
		}
		return InsertAuditLogDetails(context.WithValue(ctx, txKey, tx), actor, "", StatsRetentionAudit, target,
			map[string]*int{"from": from, "to": days})
	})
}

// StatsMetric is one stored metric with its collection state.
type StatsMetric struct {
	Metric          string
	Descriptor      json.RawMessage
	Watermark       *time.Time
	LastCollectedAt *time.Time
	UnboundAccounts int
	Dropped         map[string]int
}

// StatsMetrics is every metric stored for a target, by name.
func StatsMetrics(ctx context.Context, target string) ([]StatsMetric, error) {
	rows, err := querier(ctx).Query(ctx, `
		SELECT metric, descriptor, watermark, last_collected_at, unbound_accounts, dropped
		  FROM addon_metrics WHERE target_id = $1 ORDER BY metric`, target)
	if err != nil {
		return nil, fmt.Errorf("read metrics of %s: %w", target, err)
	}
	defer rows.Close()
	var out []StatsMetric
	for rows.Next() {
		var m StatsMetric
		var raw []byte
		if err := rows.Scan(&m.Metric, &raw, &m.Watermark, &m.LastCollectedAt, &m.UnboundAccounts, &m.Dropped); err != nil {
			return nil, err
		}
		m.Descriptor = raw
		out = append(out, m)
	}
	return out, rows.Err()
}

// StatsValue is one series' value for a day (YYYY-MM-DD) or, summed, a
// month (YYYY-MM).
type StatsValue struct {
	Metric string
	Dims   map[string]string
	When   string
	Value  float64
}

// StatsDays is one subject's points ("" the totals) on a target from a day on.
func StatsDays(ctx context.Context, target, subject, from string) ([]StatsValue, error) {
	return statsValues(ctx, `
		SELECT metric, dims, to_char(bucket_start, 'YYYY-MM-DD'), value
		  FROM addon_metric_points
		 WHERE bucket_start >= $1::date AND target_id = $2 AND subject = $3
		 ORDER BY metric, bucket_start`, from, target, subject)
}

// StatsMonths is one subject's points summed by calendar month from a day on:
// a temporal sum, meaningful for a counter alone (the caller's to choose).
func StatsMonths(ctx context.Context, target, subject, from string) ([]StatsValue, error) {
	return statsValues(ctx, `
		SELECT metric, dims, to_char(bucket_start, 'YYYY-MM') AS month, SUM(value)
		  FROM addon_metric_points
		 WHERE bucket_start >= $1::date AND target_id = $2 AND subject = $3
		 GROUP BY metric, dims, month
		 ORDER BY metric, month`, from, target, subject)
}

func statsValues(ctx context.Context, q string, args ...any) ([]StatsValue, error) {
	rows, err := querier(ctx).Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("read statistics: %w", err)
	}
	defer rows.Close()
	var out []StatsValue
	for rows.Next() {
		var v StatsValue
		if err := rows.Scan(&v.Metric, &v.Dims, &v.When, &v.Value); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// StatsGapRow is a stored gap of a metric.
type StatsGapRow struct {
	Metric string
	StatsGap
}

// StatsGaps is every gap on a target that ends after a day.
func StatsGaps(ctx context.Context, target, after string) ([]StatsGapRow, error) {
	rows, err := querier(ctx).Query(ctx, `
		SELECT metric, dims, to_char(gap_start, 'YYYY-MM-DD'), to_char(gap_end, 'YYYY-MM-DD'), reason
		  FROM addon_metric_gaps
		 WHERE gap_end > $1::date AND target_id = $2
		 ORDER BY metric, gap_start`, after, target)
	if err != nil {
		return nil, fmt.Errorf("read gaps of %s: %w", target, err)
	}
	defer rows.Close()
	var out []StatsGapRow
	for rows.Next() {
		var g StatsGapRow
		if err := rows.Scan(&g.Metric, &g.Dims, &g.Start, &g.End, &g.Reason); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// StatsDistribution is how one per-person series spreads over members from a
// day on: each member's sum of a counter's days, or a gauge's latest value,
// counted and its median and 90th percentile. Never which member.
type StatsDistribution struct {
	Target, Metric string
	Dims           map[string]string
	Members        int
	P50, P90       float64
}

// StatsDistributions is every per-person series' distribution, every target.
func StatsDistributions(ctx context.Context, from string) ([]StatsDistribution, error) {
	rows, err := querier(ctx).Query(ctx, `
		WITH per AS (
			SELECT p.target_id, p.metric, p.dims, p.subject,
			       CASE WHEN m.descriptor->>'instrument' = 'counter' THEN SUM(p.value)
			            ELSE (array_agg(p.value ORDER BY p.bucket_start DESC))[1] END AS v
			  FROM addon_metric_points p
			  JOIN addon_metrics m ON m.target_id = p.target_id AND m.metric = p.metric
			 WHERE p.bucket_start >= $1::date AND p.subject <> ''
			 GROUP BY p.target_id, p.metric, p.dims, p.subject, m.descriptor->>'instrument')
		SELECT target_id, metric, dims, count(*),
		       percentile_cont(0.5) WITHIN GROUP (ORDER BY v), percentile_cont(0.9) WITHIN GROUP (ORDER BY v)
		  FROM per
		 GROUP BY target_id, metric, dims
		 ORDER BY target_id, metric`, from)
	if err != nil {
		return nil, fmt.Errorf("read distributions: %w", err)
	}
	defer rows.Close()
	var out []StatsDistribution
	for rows.Next() {
		var d StatsDistribution
		if err := rows.Scan(&d.Target, &d.Metric, &d.Dims, &d.Members, &d.P50, &d.P90); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
