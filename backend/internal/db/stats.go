package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Statistics an add-on reports (feature stats, migration 000093, design §9.6):
// the stored meaning of each metric, its daily points and its gaps. Every day
// here is a makerspace day, travelling as YYYY-MM-DD text ($n::date).

// Retention (decision 13, ruling O4), the one definition the prune, a late
// point at ingestion and every per-person read share, today being $1 and t the
// target's row: a per-person point is kept stats_subject_retention_days (400
// when unset; the column's CHECK is the ceiling) counting today, so N days are
// today − (N − 1) through today, as "the last 30 days" are; a total five
// years, and a gap as long as the totals it explains.
const (
	statsSubjectDays     = `COALESCE(t.stats_subject_retention_days, 400)`
	statsSubjectKeptFrom = `($1::date - (` + statsSubjectDays + ` - 1))`
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
// NamedSubjects is who the collector found a dimension value names, as
// subject ids (migration 000094): shown to admins, left out of the view of a
// member it names anyone else to.
type StatsPoint struct {
	Subject       string
	Dims          map[string]string
	Day           string
	Value         float64
	NamedSubjects []string
}

// StatsGap is one unobserved window [Start, End) of a metric's series.
// NamedSubjects: who a dimension value or the reason names.
type StatsGap struct {
	Dims          map[string]string
	Start, End    string
	Reason        string
	NamedSubjects []string
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
	subjects, dims, days, values, named := make([]string, len(points)), make([]string, len(points)), make([]string, len(points)), make([]float64, len(points)), make([]string, len(points))
	for i, p := range points {
		raw, err := json.Marshal(nonNilDims(p.Dims))
		if err != nil {
			return 0, err
		}
		// A list per row travels as JSON: unnest flattens a text[][].
		who, err := json.Marshal(nonNil(p.NamedSubjects))
		if err != nil {
			return 0, err
		}
		subjects[i], dims[i], days[i], values[i], named[i] = p.Subject, string(raw), p.Day, p.Value, string(who)
	}
	tag, err := c.tx.Exec(ctx, `
		INSERT INTO addon_metric_points (target_id, metric, subject, dims, bucket_start, value, observed_at, named_subjects)
		SELECT $2, $3, x.s, x.d::jsonb, x.b, x.v, $8, ARRAY(SELECT jsonb_array_elements_text(x.n::jsonb))
		  FROM unnest($4::text[], $5::text[], $6::date[], $7::numeric[], $9::text[]) AS x(s, d, b, v, n)
		  JOIN targets t ON t.target = $2
		 WHERE x.b >= `+statsKeptFrom("x.s")+`
		ON CONFLICT (target_id, metric, subject, dims, bucket_start)
		DO UPDATE SET value = EXCLUDED.value, observed_at = EXCLUDED.observed_at, named_subjects = EXCLUDED.named_subjects`,
		today, c.target, c.metric, subjects, dims, days, values, observedAt, named)
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
			INSERT INTO addon_metric_gaps (target_id, metric, dims, gap_start, gap_end, reason, named_subjects)
			SELECT $2, $3, $4::jsonb, $5::date, $6::date, $7, $8
			 WHERE $6::date > `+statsTotalKeptFrom+`
			ON CONFLICT (target_id, metric, dims, gap_start)
			DO UPDATE SET gap_end = EXCLUDED.gap_end, reason = EXCLUDED.reason, named_subjects = EXCLUDED.named_subjects`,
			today, c.target, c.metric, string(raw), g.Start, g.End, g.Reason, nonNil(g.NamedSubjects))
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

// nonNil is a list never NULL: the column is NOT NULL, and nobody named is {}.
func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func nonNilDims(d map[string]string) map[string]string {
	if d == nil {
		return map[string]string{}
	}
	return d
}

// PruneStats deletes what retention no longer keeps on `today` (a makerspace
// day): per-person points past their target's retention, totals and gaps
// past five years. Run at start-up and every 24 hours after (so a shortened
// retention takes effect within a day); returns how many points and gaps went.
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

// StatsKeptFrom is the first day (YYYY-MM-DD) whose per-person points a
// target still keeps on `today`: the prune's own line, so a read can tell a
// day retention deleted from a day nothing was counted.
func StatsKeptFrom(ctx context.Context, today, target string) (string, error) {
	var day string
	err := querier(ctx).QueryRow(ctx, `
		SELECT to_char(`+statsSubjectKeptFrom+`, 'YYYY-MM-DD') FROM targets t WHERE t.target = $2`,
		today, target).Scan(&day)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("no target %q", target)
	}
	return day, err
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
// month (YYYY-MM). NamedSubjects: whom the point's dimension values, or any
// point's of the month, were found at collection to name, sorted, once each.
type StatsValue struct {
	Metric        string
	Dims          map[string]string
	When          string
	Value         float64
	NamedSubjects []string
}

// StatsDays is one subject's points ("" the totals) on a target from a day on.
func StatsDays(ctx context.Context, target, subject, from string) ([]StatsValue, error) {
	return statsValues(ctx, `
		SELECT metric, dims, to_char(bucket_start, 'YYYY-MM-DD'), value, jsonb_build_array(named_subjects)
		  FROM addon_metric_points
		 WHERE bucket_start >= $1::date AND target_id = $2 AND subject = $3
		 ORDER BY metric, bucket_start`, from, target, subject)
}

// StatsMonths is one subject's points summed by calendar month from a day on:
// a temporal sum, meaningful for a counter alone (the caller's to choose).
func StatsMonths(ctx context.Context, target, subject, from string) ([]StatsValue, error) {
	return statsValues(ctx, `
		SELECT metric, dims, to_char(bucket_start, 'YYYY-MM') AS month, SUM(value), jsonb_agg(named_subjects)
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
		// Each read gives its rows' lists as one JSON list of lists (a
		// month's are its days'; jsonb, as array_agg cannot stack lists of
		// different lengths): one set here.
		var named [][]string
		if err := rows.Scan(&v.Metric, &v.Dims, &v.When, &v.Value, &named); err != nil {
			return nil, err
		}
		v.NamedSubjects = slices.Compact(slices.Sorted(slices.Values(slices.Concat(named...))))
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
		SELECT metric, dims, to_char(gap_start, 'YYYY-MM-DD'), to_char(gap_end, 'YYYY-MM-DD'), reason, named_subjects
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
		if err := rows.Scan(&g.Metric, &g.Dims, &g.Start, &g.End, &g.Reason, &g.NamedSubjects); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// StatsPersonValue is one person's value of a per-person series in each
// window: a counter's sum of their days, a gauge's latest value. Last30d is
// nil when they have no point in the 30-day window; Last12m never is, as a
// person is listed for having a point in the 12-month one.
type StatsPersonValue struct {
	Subject string
	Last30d *float64
	Last12m float64
}

// StatsDistribution is one per-person series (an admin's read, the owner's
// ruling of 14 Nov 2026): how it spreads over the members with a value in the
// 30-day window, counted with its median and 90th percentile, and every person
// with a value in the 12-month window, with each window's value. From and
// From12m are the days each window starts: the window's own, or its target's
// first kept day when retention is shorter.
type StatsDistribution struct {
	Target, Metric string
	Dims           map[string]string
	From, From12m  string
	Members        int
	P50, P90       float64
	People         []StatsPersonValue
}

// StatsDistributions is every per-person series, every target, over the days
// from `from30` and from `from12m` that its target still keeps on `today` (the
// prune's own line, so a day it has yet to delete is never counted). One query
// is the one definition of "a person's value": the spread is over the same
// per-person values the list names.
func StatsDistributions(ctx context.Context, today, from30, from12m string) ([]StatsDistribution, error) {
	start30 := `GREATEST($2::date, ` + statsSubjectKeptFrom + `)`
	start12 := `GREATEST($3::date, ` + statsSubjectKeptFrom + `)`
	// A person's value, over the points `since` keeps: a counter's sum, a gauge's latest.
	value := func(since string) string {
		in := ` FILTER (WHERE p.bucket_start >= ` + since + `)`
		return `CASE WHEN m.descriptor->>'instrument' = 'counter' THEN SUM(p.value)` + in + `
			            ELSE (array_agg(p.value ORDER BY p.bucket_start DESC)` + in + `)[1] END`
	}
	rows, err := querier(ctx).Query(ctx, `
		WITH per AS (
			SELECT p.target_id, p.metric, p.dims, p.subject, `+start30+` AS start30, `+start12+` AS start12,
			       `+value(start30)+` AS v30, `+value(start12)+` AS v12
			  FROM addon_metric_points p
			  JOIN addon_metrics m ON m.target_id = p.target_id AND m.metric = p.metric
			  JOIN targets t ON t.target = p.target_id
			 WHERE p.bucket_start >= `+start12+` AND p.subject <> ''
			 GROUP BY p.target_id, p.metric, p.dims, p.subject, m.descriptor->>'instrument', t.stats_subject_retention_days),
		spread AS (
			-- An ordered-set aggregate ignores NULL input: the spread is over
			-- the people with a value in the 30-day window, as before.
			SELECT target_id, metric, dims, count(v30) AS members,
			       COALESCE(percentile_cont(0.5) WITHIN GROUP (ORDER BY v30), 0) AS p50,
			       COALESCE(percentile_cont(0.9) WITHIN GROUP (ORDER BY v30), 0) AS p90
			  FROM per
			 GROUP BY target_id, metric, dims)
		SELECT per.target_id, per.metric, per.dims, to_char(per.start30, 'YYYY-MM-DD'), to_char(per.start12, 'YYYY-MM-DD'),
		       s.members, s.p50, s.p90, per.subject, per.v30, per.v12
		  FROM per
		  JOIN spread s ON s.target_id = per.target_id AND s.metric = per.metric AND s.dims = per.dims
		 ORDER BY per.target_id, per.metric, per.dims, per.subject`, today, from30, from12m)
	if err != nil {
		return nil, fmt.Errorf("read distributions: %w", err)
	}
	defer rows.Close()
	var out []StatsDistribution
	at := map[string]int{}
	for rows.Next() {
		var d StatsDistribution
		var p StatsPersonValue
		if err := rows.Scan(&d.Target, &d.Metric, &d.Dims, &d.From, &d.From12m, &d.Members, &d.P50, &d.P90, &p.Subject, &p.Last30d, &p.Last12m); err != nil {
			return nil, err
		}
		raw, _ := json.Marshal(d.Dims)
		key := d.Target + "\x00" + d.Metric + "\x00" + string(raw)
		i, seen := at[key]
		if !seen {
			i, at[key] = len(out), len(out)
			out = append(out, d)
		}
		out[i].People = append(out[i].People, p)
	}
	return out, rows.Err()
}
