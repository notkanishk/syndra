package db_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"syndra/internal/addons"
	"syndra/internal/clock"
	"syndra/internal/db"
	"syndra/internal/services/stats"
)

// The collector against a real database (addon-contract 6.6): the add-on is a
// scripted Read, everything else is the code that runs.

const collectTarget = "stats_collect_live"

var (
	sessions = addons.Metric{Name: "storage.sessions", Instrument: "counter", Unit: "count", Label: "Storage sessions",
		PerSubject: true, Dimensions: []addons.Dimension{{Name: "share", Label: "Share"}}}
	homeBytes = addons.Metric{Name: "storage.home_bytes", Instrument: "gauge", Unit: "bytes", Label: "Home folder used", PerSubject: true}
	doors     = addons.Metric{Name: "access.unlocks", Instrument: "counter", Unit: "count", Label: "Door unlocks"}
)

const buffer = 90 * 24 * time.Hour

func collectLive(t *testing.T) context.Context {
	t.Helper()
	ctx := db.LiveDB(t)
	clear := func() {
		for _, q := range []string{
			`DELETE FROM addon_metric_points WHERE target_id = $1`, `DELETE FROM addon_metric_gaps WHERE target_id = $1`,
			`DELETE FROM addon_metrics WHERE target_id = $1`, `DELETE FROM target_account_bindings WHERE target = $1`,
		} {
			if _, err := db.PG.Exec(ctx, q, collectTarget); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := db.PG.Exec(ctx, `INSERT INTO targets (target) VALUES ($1) ON CONFLICT DO NOTHING`, collectTarget); err != nil {
		t.Fatal(err)
	}
	clear()
	t.Cleanup(clear)
	stored := map[string]json.RawMessage{}
	for _, m := range []addons.Metric{sessions, homeBytes, doors} {
		raw, _ := json.Marshal(m)
		stored[m.Name] = raw
	}
	if err := db.SaveStatsDescriptors(ctx, collectTarget, stored); err != nil {
		t.Fatal(err)
	}
	account := "1001"
	if err := db.RecordTargetBinding(ctx, db.TargetBinding{Target: collectTarget, SubjectID: "sub-cena", Username: "john_cena", AccountID: &account, BoundBy: "test"}); err != nil {
		t.Fatal(err)
	}
	return ctx
}

// day is n makerspace days before today; mid is a day's first instant.
func day(n int) string { return clock.AddDate(clock.Day(clock.Now()), 0, 0, -n) }

func mid(t *testing.T, d string) time.Time {
	t.Helper()
	m, err := clock.Midnight(d)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func pt(t *testing.T, d, subject string, dims map[string]string, v float64) addons.StatPoint {
	return addons.StatPoint{Start: mid(t, d), End: mid(t, clock.AddDate(d, 0, 0, 1)), Subject: subject, Dimensions: dims, Value: &v}
}

func gp(t *testing.T, from, to string, dims map[string]string, reason string) addons.StatGap {
	return addons.StatGap{Start: mid(t, from), End: mid(t, to), Dimensions: dims, Reason: reason}
}

var share = func(s string) map[string]string { return map[string]string{"share": s} }

// script is an add-on: each collection is answered by the next reply's pages
// (an error for a page that fails), and every call is recorded.
type script struct {
	replies [][]any
	calls   []call
	n       int
}

type call struct {
	metric, cursor string
	since          time.Time
}

func (s *script) read(_ context.Context, metric string, since time.Time, cursor string) (addons.StatsPage, error) {
	s.calls = append(s.calls, call{metric, cursor, since})
	page := 0
	if cursor != "" {
		fmt.Sscanf(cursor, "p%d", &page)
	} else {
		s.n++
	}
	switch p := s.replies[s.n-1][page].(type) {
	case error:
		return addons.StatsPage{}, p
	case addons.StatsPage:
		if page+1 < len(s.replies[s.n-1]) {
			p.NextCursor = fmt.Sprintf("p%d", page+1)
		}
		return p, nil
	}
	panic("a scripted page is a StatsPage or an error")
}

func page(t *testing.T, watermark string, points []addons.StatPoint, gaps ...addons.StatGap) addons.StatsPage {
	return addons.StatsPage{TakenAt: time.Now(), Watermark: mid(t, watermark), Points: points, Gaps: gaps}
}

func collect(t *testing.T, ctx context.Context, m addons.Metric, s *script) map[string]int {
	t.Helper()
	dropped, err := stats.Collect(ctx, collectTarget, m, buffer, s.read)
	if err != nil {
		t.Fatalf("collect %s: %v", m.Name, err)
	}
	return dropped
}

type held struct {
	Points    []string
	Gaps      []string
	Watermark *time.Time
}

// holding is everything Syndra holds of a metric, as comparable text.
func holding(t *testing.T, ctx context.Context, metric string) held {
	t.Helper()
	var h held
	rows, err := db.PG.Query(ctx, `SELECT subject || ' ' || dims::text || ' ' || to_char(bucket_start, 'YYYY-MM-DD') || ' ' || value::text
		FROM addon_metric_points WHERE target_id = $1 AND metric = $2 ORDER BY 1`, collectTarget, metric)
	if err != nil {
		t.Fatal(err)
	}
	if h.Points, err = pgx.CollectRows(rows, pgx.RowTo[string]); err != nil {
		t.Fatal(err)
	}
	rows, err = db.PG.Query(ctx, `SELECT dims::text || ' ' || to_char(gap_start, 'YYYY-MM-DD') || ' ' || to_char(gap_end, 'YYYY-MM-DD') || ' ' || reason
		FROM addon_metric_gaps WHERE target_id = $1 AND metric = $2 ORDER BY 1`, collectTarget, metric)
	if err != nil {
		t.Fatal(err)
	}
	if h.Gaps, err = pgx.CollectRows(rows, pgx.RowTo[string]); err != nil {
		t.Fatal(err)
	}
	if err := db.PG.QueryRow(ctx, `SELECT watermark FROM addon_metrics WHERE target_id = $1 AND metric = $2`, collectTarget, metric).Scan(&h.Watermark); err != nil {
		t.Fatal(err)
	}
	return h
}

func has(rows []string, want string) bool {
	for _, r := range rows {
		if r == want {
			return true
		}
	}
	return false
}

// The same (metric, since) collected twice changes nothing, for points and
// gaps alike: final ones upserted equal, provisional ones replaced equal.
func TestLiveTheSameSinceTwiceChangesNothing(t *testing.T) {
	ctx := collectLive(t)
	reply := func() []any {
		return []any{page(t, day(2), []addons.StatPoint{
			pt(t, day(4), "", share("makers"), 5), pt(t, day(4), "sub-cena", share("makers"), 2),
			pt(t, day(2), "", share("makers"), 3)},
			gp(t, day(6), day(5), share("scratch"), "Auditing does not record everyone on this share"),
			gp(t, day(2), day(0), share("scratch"), "Auditing does not record everyone on this share"))}
	}
	s := &script{replies: [][]any{reply(), reply(), reply()}}
	collect(t, ctx, sessions, s)
	if first := s.calls[0].since; first.Sub(time.Now().Add(-buffer)).Abs() > time.Minute {
		t.Errorf("the first collection asked since %v, want now − the add-on's buffer", first)
	}
	collect(t, ctx, sessions, s)
	before := holding(t, ctx, sessions.Name)
	collect(t, ctx, sessions, s)
	after := holding(t, ctx, sessions.Name)
	if !s.calls[1].since.Equal(s.calls[2].since) || !s.calls[1].since.Equal(mid(t, day(2))) {
		t.Fatalf("since %v then %v, want the stored watermark %s twice", s.calls[1].since, s.calls[2].since, day(2))
	}
	if !reflect.DeepEqual(before, after) {
		t.Errorf("the same (metric, since) twice changed what Syndra holds:\n%+v\n%+v", before, after)
	}
	if len(after.Points) != 3 || len(after.Gaps) != 2 {
		t.Errorf("holding %+v", after)
	}
}

// A provisional value that comes back absent is gone: the reply's provisional
// window replaces Syndra's.
func TestLiveAProvisionalValueAbsentNextTimeIsGone(t *testing.T) {
	ctx := collectLive(t)
	s := &script{replies: [][]any{
		{page(t, day(2), []addons.StatPoint{pt(t, day(3), "", share("makers"), 4), pt(t, day(2), "", share("makers"), 5),
			pt(t, day(1), "", share("makers"), 3), pt(t, day(1), "sub-cena", share("makers"), 3)})},
		{page(t, day(2), []addons.StatPoint{})},
	}}
	collect(t, ctx, sessions, s)
	if h := holding(t, ctx, sessions.Name); len(h.Points) != 4 {
		t.Fatalf("first collection holds %v", h.Points)
	}
	collect(t, ctx, sessions, s)
	h := holding(t, ctx, sessions.Name)
	if len(h.Points) != 1 || !has(h.Points, ` {"share": "makers"} `+day(3)+` 4`) {
		t.Errorf("after the provisional days (the watermark's own included) came back absent Syndra holds %v, want only the final %s", h.Points, day(3))
	}
}

// A day reported late, under a watermark the add-on held back, lands and
// retracts the gap that stood for it.
func TestLiveALateDayLandsAndRetractsItsGap(t *testing.T) {
	ctx := collectLive(t)
	s := &script{replies: [][]any{
		{page(t, day(3), []addons.StatPoint{pt(t, day(4), "", share("makers"), 1)}, gp(t, day(3), day(1), share("makers"), "The NAS was off"))},
		{page(t, day(1), []addons.StatPoint{pt(t, day(3), "", share("makers"), 6), pt(t, day(2), "sub-cena", share("makers"), 2), pt(t, day(2), "", share("makers"), 2)})},
	}}
	collect(t, ctx, sessions, s)
	if h := holding(t, ctx, sessions.Name); len(h.Gaps) != 1 {
		t.Fatalf("first collection holds gaps %v", h.Gaps)
	}
	collect(t, ctx, sessions, s)
	if !s.calls[1].since.Equal(mid(t, day(3))) {
		t.Errorf("the second collection asked since %v, want the held-back %s", s.calls[1].since, day(3))
	}
	h := holding(t, ctx, sessions.Name)
	if len(h.Gaps) != 0 {
		t.Errorf("the late days' gap was not retracted: %v", h.Gaps)
	}
	if len(h.Points) != 4 || !has(h.Points, ` {"share": "makers"} `+day(3)+` 6`) {
		t.Errorf("the late days did not land: %v", h.Points)
	}
	if !h.Watermark.Equal(mid(t, day(1))) {
		t.Errorf("watermark %v, want %s", h.Watermark, day(1))
	}
}

// One dark metric does not re-send another: each metric is asked from its
// own watermark.
func TestLiveOneDarkMetricDoesNotResendAnother(t *testing.T) {
	ctx := collectLive(t)
	lit := &script{replies: [][]any{{page(t, day(1), nil)}, {page(t, day(1), nil)}}}
	dark := &script{replies: [][]any{{page(t, day(30), nil)}, {page(t, day(30), nil)}}}
	collect(t, ctx, sessions, lit)
	collect(t, ctx, homeBytes, dark)
	collect(t, ctx, sessions, lit)
	collect(t, ctx, homeBytes, dark)
	if !lit.calls[1].since.Equal(mid(t, day(1))) || !dark.calls[1].since.Equal(mid(t, day(30))) {
		t.Errorf("since: %s asked %v (want %s), %s asked %v (want %s)",
			sessions.Name, lit.calls[1].since, day(1), homeBytes.Name, dark.calls[1].since, day(30))
	}
}

// A page that fails rolls the whole collection back: the watermark stays and
// so do the provisional rows the reply would have replaced.
func TestLiveAFailedPageKeepsTheWatermark(t *testing.T) {
	ctx := collectLive(t)
	s := &script{replies: [][]any{
		{page(t, day(3), []addons.StatPoint{pt(t, day(2), "", share("makers"), 3)})},
		{page(t, day(1), []addons.StatPoint{pt(t, day(2), "", share("makers"), 9)}), errors.New("the add-on went away")},
	}}
	collect(t, ctx, sessions, s)
	before := holding(t, ctx, sessions.Name)
	if _, err := stats.Collect(ctx, collectTarget, sessions, buffer, s.read); err == nil {
		t.Fatal("a failed second page reported success")
	}
	if after := holding(t, ctx, sessions.Name); !reflect.DeepEqual(before, after) {
		t.Errorf("a failed page changed what Syndra holds:\n%+v\n%+v", before, after)
	}
	if len(s.calls) != 3 || s.calls[2].cursor != "p1" {
		t.Errorf("calls %+v, want the second page asked by its cursor", s.calls)
	}
}

// The stored watermark never moves backwards, is always a makerspace
// midnight (an off-midnight one floored to its day's), and never passes the
// start of today, which is still being counted.
func TestLiveTheWatermarkIsAMidnightThatNeverGoesBack(t *testing.T) {
	ctx := collectLive(t)
	off := page(t, day(1), nil)
	off.Watermark = off.Watermark.Add(5 * time.Hour)
	s := &script{replies: [][]any{{page(t, day(3), nil)}, {page(t, day(5), nil)}, {off}, {page(t, clock.AddDate(day(0), 0, 0, 1), nil)}}}
	for i, want := range []string{day(3), day(3), day(1), day(0)} {
		collect(t, ctx, sessions, s)
		if h := holding(t, ctx, sessions.Name); !h.Watermark.Equal(mid(t, want)) {
			t.Errorf("collection %d: watermark %v, want %s", i+1, h.Watermark, want)
		}
	}
}

// What the add-on may not say is dropped and counted, by reason, and the
// rest is kept: a point that is not one makerspace day, no value or a
// negative one, a subject on a metric not per person or with no binding here,
// an undeclared dimension, and a late point past retention. A dimension naming
// a bound person is not a drop (TestLiveWhatNamesAPersonIsKeptAndFlagged).
func TestLiveEachDropIsCountedByReason(t *testing.T) {
	ctx := collectLive(t)
	utcMidnight := pt(t, day(3), "", share("makers"), 1)
	utcMidnight.Start, utcMidnight.End = utcMidnight.Start.UTC().Truncate(24*time.Hour), utcMidnight.End.UTC().Truncate(24*time.Hour)
	twoDays := pt(t, day(3), "", share("projects"), 1)
	twoDays.End = mid(t, day(1))
	noValue := pt(t, day(3), "", share("tools"), 1)
	noValue.Value = nil
	s := &script{replies: [][]any{{page(t, day(1), []addons.StatPoint{
		pt(t, day(3), "", share("makers"), 7), pt(t, day(3), "sub-cena", share("makers"), 2),
		utcMidnight, twoDays, noValue,
		pt(t, day(3), "", share("lab"), -1),
		pt(t, day(3), "sub-orton", share("makers"), 1),
		pt(t, day(3), "", map[string]string{"printer": "xl-1"}, 1),
		pt(t, day(401), "sub-cena", share("makers"), 1), pt(t, day(401), "", share("makers"), 1),
	}, gp(t, day(3), day(2), share("lab"), ""))}}}
	dropped := collect(t, ctx, sessions, s)
	want := map[string]int{stats.DropNotADay: 2, stats.DropBadValue: 3, stats.DropUnboundSubject: 1,
		stats.DropUndeclaredDimension: 1, stats.DropPastRetention: 1}
	if !reflect.DeepEqual(dropped, want) {
		t.Errorf("dropped %v, want %v", dropped, want)
	}
	var stored map[string]int
	if err := db.PG.QueryRow(ctx, `SELECT dropped FROM addon_metrics WHERE target_id = $1 AND metric = $2`, collectTarget, sessions.Name).Scan(&stored); err != nil || !reflect.DeepEqual(stored, want) {
		t.Errorf("the metric's row says dropped %v (%v)", stored, err)
	}
	h := holding(t, ctx, sessions.Name)
	if len(h.Points) != 3 || !has(h.Points, ` {"share": "makers"} `+day(3)+` 7`) || !has(h.Points, `sub-cena {"share": "makers"} `+day(3)+` 2`) ||
		!has(h.Points, ` {"share": "makers"} `+day(401)+` 1`) || len(h.Gaps) != 0 {
		t.Errorf("kept %v and gaps %v: want the makerspace day's two points and the total past a person's retention", h.Points, h.Gaps)
	}

	totals := &script{replies: [][]any{{page(t, day(1), []addons.StatPoint{pt(t, day(3), "sub-cena", nil, 1), pt(t, day(3), "", nil, 4)})}}}
	if dropped := collect(t, ctx, doors, totals); !reflect.DeepEqual(dropped, map[string]int{stats.DropNotPerSubject: 1}) {
		t.Errorf("a subject on a metric not per person: dropped %v", dropped)
	}
}

// What names a bound person (their subject id, account name or account id,
// as a whole token in any case) is kept as the add-on sent it and flagged
// names_person in the row, at collection: a point by its dimension value, a
// gap by its value or its reason. Nothing else is flagged, and nothing is
// counted as dropped (the owner's ruling of 14 Nov 2026).
func TestLiveWhatNamesAPersonIsKeptAndFlagged(t *testing.T) {
	ctx := collectLive(t)
	s := &script{replies: [][]any{{page(t, day(1), []addons.StatPoint{
		pt(t, day(3), "", share("makers"), 7), pt(t, day(3), "sub-cena", share("home-JOHN_CENA"), 2),
		pt(t, day(3), "", share("1001"), 1), pt(t, day(3), "", share("sub-cena"), 1),
	}, gp(t, day(3), day(2), share("john_cena"), "Off"), gp(t, day(3), day(2), share("makers"), "account 1001 is locked"),
		gp(t, day(3), day(2), share("lab"), "Auditing is off"))}}}
	if dropped := collect(t, ctx, sessions, s); len(dropped) != 0 {
		t.Errorf("dropped %v: naming a person is not a drop", dropped)
	}
	rowsOf := func(q string) []string {
		t.Helper()
		rows, err := db.PG.Query(ctx, q, collectTarget, sessions.Name)
		if err != nil {
			t.Fatal(err)
		}
		out, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	points := rowsOf(`SELECT subject || ' ' || dims::text || ' ' || names_person FROM addon_metric_points WHERE target_id = $1 AND metric = $2 ORDER BY 1`)
	if want := []string{` {"share": "1001"} true`, ` {"share": "makers"} false`, ` {"share": "sub-cena"} true`, `sub-cena {"share": "home-JOHN_CENA"} true`}; !reflect.DeepEqual(points, want) {
		t.Errorf("points %q, want %q", points, want)
	}
	gaps := rowsOf(`SELECT dims::text || ' ' || reason || ' ' || names_person FROM addon_metric_gaps WHERE target_id = $1 AND metric = $2 ORDER BY 1`)
	if want := []string{`{"share": "john_cena"} Off true`, `{"share": "lab"} Auditing is off false`, `{"share": "makers"} account 1001 is locked true`}; !reflect.DeepEqual(gaps, want) {
		t.Errorf("gaps %q, want %q", gaps, want)
	}

	// The reads a page is built from carry the flag: a day, its month, a gap.
	if vals, err := db.StatsDays(ctx, collectTarget, "sub-cena", day(30)); err != nil || len(vals) != 1 || !vals[0].NamesPerson {
		t.Errorf("days %+v (%v): want cena's one point, flagged", vals, err)
	}
	if vals, err := db.StatsMonths(ctx, collectTarget, "sub-cena", day(30)); err != nil || len(vals) != 1 || !vals[0].NamesPerson {
		t.Errorf("months %+v (%v): want cena's one month, flagged", vals, err)
	}
	read, err := db.StatsGaps(ctx, collectTarget, day(30))
	var flagged []string
	for _, g := range read {
		if g.NamesPerson {
			flagged = append(flagged, g.Dims["share"])
		}
	}
	if err != nil || len(read) != 3 || !reflect.DeepEqual(flagged, []string{"john_cena", "makers"}) && !reflect.DeepEqual(flagged, []string{"makers", "john_cena"}) {
		t.Errorf("gaps read %+v (%v): want john_cena's and makers' flagged", read, err)
	}
}

// Syndra keeps constants.series_per_metric dimension sets of a metric: the
// ones it already holds, and new ones until the cap; the rest are dropped and
// counted. People are not series.
func TestLiveTheSeriesCapKeepsFiveHundred(t *testing.T) {
	ctx := collectLive(t)
	var points []addons.StatPoint
	for i := range addons.SeriesPerMetric + 1 {
		points = append(points, pt(t, day(2), "", share(fmt.Sprintf("s%03d", i)), 1))
	}
	points = append(points, pt(t, day(2), "sub-cena", share("s000"), 1))
	s := &script{replies: [][]any{{page(t, day(3), points)}, {page(t, day(3), append([]addons.StatPoint{pt(t, day(2), "", share("s999"), 1)}, points...))}}}
	if dropped := collect(t, ctx, sessions, s); dropped[stats.DropSeries] != 1 {
		t.Errorf("dropped %v, want the 501st series", dropped)
	}
	if h := holding(t, ctx, sessions.Name); len(h.Points) != addons.SeriesPerMetric+1 {
		t.Errorf("holds %d points, want %d series and the person's point", len(h.Points), addons.SeriesPerMetric)
	}
	if dropped := collect(t, ctx, sessions, s); dropped[stats.DropSeries] != 2 {
		t.Errorf("second collection dropped %v: the 500 held are kept, s500 and s999 are not", dropped)
	}
	// s999 came first, while the held series were being replaced: still new.
	if h := holding(t, ctx, sessions.Name); has(h.Points, ` {"share": "s999"} `+day(2)+` 1`) || !has(h.Points, ` {"share": "s499"} `+day(2)+` 1`) {
		t.Errorf("a new series displaced a held one: %d points", len(h.Points))
	}
}

// An add-on that answers the same cursor again is paging for ever: the
// collection stops, keeps nothing and keeps its watermark.
func TestLiveACursorThatRepeatsStopsTheCollection(t *testing.T) {
	ctx := collectLive(t)
	calls := 0
	loop := func(context.Context, string, time.Time, string) (addons.StatsPage, error) {
		calls++
		if calls > 5 {
			t.Fatal("still paging")
		}
		p := page(t, day(1), []addons.StatPoint{pt(t, day(2), "", share("makers"), 1)})
		p.NextCursor = "again"
		return p, nil
	}
	if _, err := stats.Collect(ctx, collectTarget, sessions, buffer, loop); err == nil {
		t.Fatal("a repeating cursor reported success")
	}
	if h := holding(t, ctx, sessions.Name); len(h.Points) != 0 || h.Watermark != nil {
		t.Errorf("a stopped collection kept %+v", h)
	}
}

// A cursor cycle (A, B, A) is paging for ever as surely as a repeat of the
// last cursor: the collection stops at the first cursor it has already
// followed, after three pages, not at metricTimeout.
func TestLiveACursorCycleStopsTheCollection(t *testing.T) {
	ctx := collectLive(t)
	next := map[string]string{"": "A", "A": "B", "B": "A"}
	calls := 0
	cycle := func(_ context.Context, _ string, _ time.Time, cursor string) (addons.StatsPage, error) {
		calls++
		if calls > 5 {
			t.Fatal("still paging")
		}
		p := page(t, day(1), []addons.StatPoint{pt(t, day(2), "", share("makers"), 1)})
		p.NextCursor = next[cursor]
		return p, nil
	}
	if _, err := stats.Collect(ctx, collectTarget, sessions, buffer, cycle); err == nil || calls != 3 {
		t.Fatalf("a cursor cycle: %v after %d pages, want an error after 3", err, calls)
	}
	if h := holding(t, ctx, sessions.Name); len(h.Points) != 0 || h.Watermark != nil {
		t.Errorf("a stopped collection kept %+v", h)
	}
}
