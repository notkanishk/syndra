package db

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	pgx5 "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"

	"syndra/internal/clock"
)

// Statistics against a real database (migration 000093, addon-contract 6.7):
// retention, its one setting, and what insights may say.

const statsTarget = "stats_live"

// statsLive is a target with one counter declared and nothing collected.
func statsLive(t *testing.T) context.Context {
	t.Helper()
	ctx := liveDB(t)
	clear := func() {
		for _, q := range []string{
			`DELETE FROM addon_metric_points WHERE target_id = $1`, `DELETE FROM addon_metric_gaps WHERE target_id = $1`,
			`DELETE FROM addon_metrics WHERE target_id = $1`, `DELETE FROM audit_logs WHERE resource_id = $1`,
		} {
			if _, err := PG.Exec(ctx, q, statsTarget); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := PG.Exec(ctx, `INSERT INTO targets (target) VALUES ($1) ON CONFLICT (target) DO UPDATE SET stats_subject_retention_days = NULL`, statsTarget); err != nil {
		t.Fatal(err)
	}
	clear()
	t.Cleanup(clear)
	if err := SaveStatsDescriptors(ctx, statsTarget, map[string]json.RawMessage{
		"storage.sessions":   json.RawMessage(`{"name":"storage.sessions","instrument":"counter","unit":"count","label":"Storage sessions","per_subject":true}`),
		"storage.home_bytes": json.RawMessage(`{"name":"storage.home_bytes","instrument":"gauge","unit":"bytes","label":"Home folder used","per_subject":true}`),
	}); err != nil {
		t.Fatal(err)
	}
	return ctx
}

// put stores points straight, as a collection would have.
func put(t *testing.T, ctx context.Context, metric, today string, points ...StatsPoint) {
	t.Helper()
	err := CollectStats(ctx, statsTarget, metric, func(c *StatsTx) error {
		_, err := c.UpsertPoints(ctx, today, time.Now(), points)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func pointDays(t *testing.T, ctx context.Context, subject string) map[string]bool {
	t.Helper()
	rows, err := PG.Query(ctx, `SELECT to_char(bucket_start, 'YYYY-MM-DD') FROM addon_metric_points WHERE target_id = $1 AND subject = $2`, statsTarget, subject)
	if err != nil {
		t.Fatal(err)
	}
	days, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, d := range days {
		out[d] = true
	}
	return out
}

// Per-person points go after 400 days, or the target's shorter setting;
// totals after five years whatever the setting; gaps with the totals.
func TestLiveRetentionKeepsTotalsLongerThanPeople(t *testing.T) {
	ctx := statsLive(t)
	// Stored a year ago, with retention as it was then.
	then := clock.AddDate(clock.Day(clock.Now()), -6, 0, 0)
	today := clock.Day(clock.Now())
	day := func(n int) string { return clock.AddDate(today, 0, 0, -n) }
	year5 := clock.AddDate(today, -5, 0, 0)
	put(t, ctx, "storage.sessions", then,
		StatsPoint{Subject: "sub-a", Day: day(399), Value: 1}, StatsPoint{Subject: "sub-a", Day: day(400), Value: 1},
		StatsPoint{Subject: "sub-a", Day: day(29), Value: 1}, StatsPoint{Subject: "sub-a", Day: day(30), Value: 1},
		StatsPoint{Day: day(401), Value: 2}, StatsPoint{Day: year5, Value: 2}, StatsPoint{Day: clock.AddDate(year5, 0, 0, -1), Value: 2})
	err := CollectStats(ctx, statsTarget, "storage.sessions", func(c *StatsTx) error {
		_, err := c.UpsertGaps(ctx, then, []StatsGap{
			{Start: clock.AddDate(year5, 0, 0, -2), End: year5, Reason: "The NAS was off"},
			{Start: clock.AddDate(year5, 0, 0, -1), End: clock.AddDate(year5, 0, 0, 1), Reason: "The NAS was off"},
		})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, _, err := PruneStats(ctx, today); err != nil {
		t.Fatal(err)
	}
	// N days counting today: 400 are today − 399 through today.
	if got := pointDays(t, ctx, "sub-a"); !got[day(399)] || got[day(400)] {
		t.Errorf("per-person days after the prune %v: want %s kept and %s gone", got, day(399), day(400))
	}
	if got := pointDays(t, ctx, ""); !got[day(401)] || !got[year5] || got[clock.AddDate(year5, 0, 0, -1)] {
		t.Errorf("totals after the prune %v: want 401 days and five years kept, the day before gone", got)
	}
	var gaps []string
	rows, _ := PG.Query(ctx, `SELECT to_char(gap_end, 'YYYY-MM-DD') FROM addon_metric_gaps WHERE target_id = $1`, statsTarget)
	gaps, _ = pgx.CollectRows(rows, pgx.RowTo[string])
	if len(gaps) != 1 || gaps[0] != clock.AddDate(year5, 0, 0, 1) {
		t.Errorf("gaps after the prune end %v: want only the one ending after five years ago", gaps)
	}

	thirty := 30
	if err := SetStatsRetention(ctx, statsTarget, &thirty, "op-1"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := PruneStats(ctx, today); err != nil {
		t.Fatal(err)
	}
	if got := pointDays(t, ctx, "sub-a"); !got[day(29)] || got[day(30)] || got[day(399)] {
		t.Errorf("per-person days with a 30-day setting %v: want %s kept, %s gone", got, day(29), day(30))
	}
	if got := pointDays(t, ctx, ""); !got[day(401)] {
		t.Errorf("a per-person setting shortened the totals: %v", got)
	}
}

// The first day a read says retention keeps is the prune's and ingestion's
// own line: kept on it, gone (and refused late) the day before; 400 days
// counting today unless shortened (today − 399).
func TestLiveKeptFromIsThePrunesLine(t *testing.T) {
	ctx := statsLive(t)
	today := clock.Day(clock.Now())
	if from, err := StatsKeptFrom(ctx, today, statsTarget); err != nil || from != clock.AddDate(today, 0, 0, -399) {
		t.Errorf("unset: kept from %q (%v), want 399 days back (400 counting today)", from, err)
	}
	ninety := 90
	if err := SetStatsRetention(ctx, statsTarget, &ninety, "op-1"); err != nil {
		t.Fatal(err)
	}
	from, err := StatsKeptFrom(ctx, today, statsTarget)
	if err != nil || from != clock.AddDate(today, 0, 0, -89) {
		t.Fatalf("90 days: kept from %q (%v), want 89 days back (90 counting today)", from, err)
	}
	before := clock.AddDate(from, 0, 0, -1)
	err = CollectStats(ctx, statsTarget, "storage.sessions", func(c *StatsTx) error {
		n, err := c.UpsertPoints(ctx, today, time.Now(), []StatsPoint{{Subject: "sub-a", Day: from, Value: 1}, {Subject: "sub-a", Day: before, Value: 1}})
		if n != 1 {
			t.Errorf("ingestion kept %d points, want the one on %s", n, from)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PG.Exec(ctx, `INSERT INTO addon_metric_points (target_id, metric, subject, dims, bucket_start, value, observed_at)
		VALUES ($1, 'storage.sessions', 'sub-a', '{}', $2::date, 1, now())`, statsTarget, before); err != nil {
		t.Fatal(err) // stored before the setting was shortened
	}
	if _, _, err := PruneStats(ctx, today); err != nil {
		t.Fatal(err)
	}
	if got := pointDays(t, ctx, "sub-a"); !got[from] || got[before] {
		t.Errorf("after the prune %v: want %s kept and %s gone", got, from, before)
	}
	if _, err := StatsKeptFrom(ctx, today, "no_such_target"); err == nil {
		t.Error("an unknown target has a kept-from day")
	}
}

// The setting is 1 to 400 days or none, by the schema's CHECK (its one
// place): it can shorten and never lengthen. Each change is audited.
func TestLiveRetentionSettingCannotLengthen(t *testing.T) {
	ctx := statsLive(t)
	if days, set, err := StatsRetention(ctx, statsTarget); err != nil || days != 400 || set {
		t.Fatalf("unset reads %d %t %v, want the 400-day ceiling", days, set, err)
	}
	for _, bad := range []int{401, 0, -1, 1000} {
		if err := SetStatsRetention(ctx, statsTarget, &bad, "op-1"); !errors.Is(err, ErrStatsRetentionInvalid) {
			t.Errorf("%d days: %v, want ErrStatsRetentionInvalid", bad, err)
		}
	}
	for _, ok := range []int{1, 400, 90} {
		if err := SetStatsRetention(ctx, statsTarget, &ok, "op-1"); err != nil {
			t.Errorf("%d days: %v", ok, err)
		}
	}
	if days, set, _ := StatsRetention(ctx, statsTarget); days != 90 || !set {
		t.Errorf("after setting 90 reads %d %t", days, set)
	}
	if err := SetStatsRetention(ctx, statsTarget, nil, "op-1"); err != nil {
		t.Fatal(err)
	}
	if days, set, _ := StatsRetention(ctx, statsTarget); days != 400 || set {
		t.Errorf("after clearing reads %d %t", days, set)
	}
	var n int
	if err := PG.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE action = $1 AND resource_id = $2`, StatsRetentionAudit, statsTarget).Scan(&n); err != nil || n != 4 {
		t.Errorf("%d audit rows (%v), want one per accepted change", n, err)
	}
}

// Insights spread a per-person series over members and name nobody: a
// counter is each member's sum over the window, a gauge each member's latest
// value, and the totals are not a member.
func TestLiveDistributionsNameNobody(t *testing.T) {
	ctx := statsLive(t)
	today := clock.Day(clock.Now())
	d1, d2 := clock.AddDate(today, 0, 0, -2), clock.AddDate(today, 0, 0, -1)
	put(t, ctx, "storage.sessions", today,
		StatsPoint{Subject: "sub-cena", Day: d1, Value: 1}, StatsPoint{Subject: "sub-cena", Day: d2, Value: 2},
		StatsPoint{Subject: "sub-batista", Day: d2, Value: 10}, StatsPoint{Day: d2, Value: 50})
	put(t, ctx, "storage.home_bytes", today,
		StatsPoint{Subject: "sub-cena", Day: d1, Value: 900}, StatsPoint{Subject: "sub-cena", Day: d2, Value: 100})
	rows, err := StatsDistributions(ctx, today, clock.AddDate(today, 0, 0, -29))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]StatsDistribution{}
	for _, r := range rows {
		if r.Target == statsTarget {
			got[r.Metric] = r
		}
	}
	if s := got["storage.sessions"]; s.Members != 2 || s.P50 != 6.5 || s.P90 != 9.3 {
		t.Errorf("sessions spread %+v: want 2 members (sums 3 and 10), median 6.5", s)
	}
	if g := got["storage.home_bytes"]; g.Members != 1 || g.P50 != 100 {
		t.Errorf("home bytes spread %+v: want the latest value, 100", g)
	}
	raw, _ := json.Marshal(rows)
	if strings.Contains(string(raw), "sub-cena") || strings.Contains(string(raw), "sub-batista") {
		t.Errorf("insights name a member: %s", raw)
	}
}

// A target kept for fewer days than the window spreads only the days it
// keeps, before the prune has run as after, and says from which day: a
// member's 20-day-old sessions, still stored, are not in the spread.
func TestLiveDistributionsKeepToRetention(t *testing.T) {
	ctx := statsLive(t)
	today := clock.Day(clock.Now())
	put(t, ctx, "storage.sessions", today,
		StatsPoint{Subject: "sub-cena", Day: clock.AddDate(today, 0, 0, -20), Value: 100}, StatsPoint{Subject: "sub-cena", Day: clock.AddDate(today, 0, 0, -9), Value: 2},
		StatsPoint{Subject: "sub-batista", Day: clock.AddDate(today, 0, 0, -20), Value: 50})
	ten := 10
	if err := SetStatsRetention(ctx, statsTarget, &ten, "op-1"); err != nil {
		t.Fatal(err)
	}
	from30 := clock.AddDate(today, 0, 0, -29)
	spread := func() (StatsDistribution, bool) {
		rows, err := StatsDistributions(ctx, today, from30)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range rows {
			if r.Target == statsTarget && r.Metric == "storage.sessions" {
				return r, true
			}
		}
		return StatsDistribution{}, false
	}
	kept, _ := StatsKeptFrom(ctx, today, statsTarget)
	if s, ok := spread(); !ok || s.Members != 1 || s.P50 != 2 || s.From != kept || kept != clock.AddDate(today, 0, 0, -9) {
		t.Errorf("10 days kept, before the prune: %+v (found %t), want only cena's 2 from %s", s, ok, kept)
	}
	if _, _, err := PruneStats(ctx, today); err != nil {
		t.Fatal(err)
	}
	if s, ok := spread(); !ok || s.Members != 1 || s.P50 != 2 || s.From != kept {
		t.Errorf("10 days kept, after the prune: %+v, want the same spread", s)
	}
	if err := SetStatsRetention(ctx, statsTarget, nil, "op-1"); err != nil {
		t.Fatal(err)
	}
	if s, _ := spread(); s.From != from30 {
		t.Errorf("400 days kept: the spread starts %s, want the window's %s", s.From, from30)
	}
}

// 000093 runs down and up on the real engine: down removes exactly its
// tables and column, up brings them back. Left where it began.
func TestLiveMigration93RunsDownAndUp(t *testing.T) {
	ctx := liveDB(t)
	var version int64
	var dirty bool
	if err := PG.QueryRow(ctx, `SELECT version, dirty FROM schema_migrations`).Scan(&version, &dirty); err != nil {
		t.Fatal(err)
	}
	if version < 93 || dirty {
		t.Skipf("the test database is at %d (dirty %t), before this branch's 93", version, dirty)
	}
	config, err := pgxpool.ParseConfig(os.Getenv("SYNDRA_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	driver, err := pgx5.WithInstance(stdlib.OpenDB(*config.ConnConfig), &pgx5.Config{})
	if err != nil {
		t.Fatal(err)
	}
	m, err := migrate.NewWithDatabaseInstance("file://../../db/migrations", "postgres", driver)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if v, _, _ := m.Version(); v != uint(version) {
			_ = m.Migrate(uint(version))
		}
	})
	present := func() (tables int, column bool) {
		if err := PG.QueryRow(ctx, `SELECT count(*) FROM information_schema.tables WHERE table_name IN ('addon_metrics', 'addon_metric_points', 'addon_metric_gaps')`).Scan(&tables); err != nil {
			t.Fatal(err)
		}
		if err := PG.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'targets' AND column_name = 'stats_subject_retention_days')`).Scan(&column); err != nil {
			t.Fatal(err)
		}
		return tables, column
	}
	if err := m.Migrate(92); err != nil {
		t.Fatalf("000093 down: %v", err)
	}
	if tables, column := present(); tables != 0 || column {
		t.Fatalf("after down: %d tables, column %t", tables, column)
	}
	if err := m.Migrate(93); err != nil {
		t.Fatalf("000093 up: %v", err)
	}
	if tables, column := present(); tables != 3 || !column {
		t.Fatalf("after up: %d tables, column %t", tables, column)
	}
}

// 000093 only adds: no DROP or type change in its up migration, and its down
// drops exactly what it added.
func TestMigration93IsExpandOnly(t *testing.T) {
	up, down := readMigrationPair(t, "000093_addon_stats")
	if strings.Contains(strings.ToUpper(up), "DROP ") || strings.Contains(strings.ToUpper(up), " TYPE ") {
		t.Error("000093 up must be additive")
	}
	for _, want := range []string{
		"DROP COLUMN IF EXISTS stats_subject_retention_days", "DROP TABLE IF EXISTS addon_metric_gaps",
		"DROP TABLE IF EXISTS addon_metric_points", "DROP TABLE IF EXISTS addon_metrics",
	} {
		if !strings.Contains(down, want) {
			t.Errorf("000093 down must contain %q", want)
		}
	}
	if !strings.Contains(up, "CHECK (stats_subject_retention_days BETWEEN 1 AND 400)") {
		t.Error("the retention ceiling is the column's CHECK")
	}
}
