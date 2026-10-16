package services

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"syndra/internal/db"
	"syndra/internal/models"
)

func TestParseAsOfDate(t *testing.T) {
	now := at(2026, 10, 16, 0, 5) // just after local midnight: today is the 16th
	s := &snapStore{}
	s.install(t, &now)
	for _, ok := range []string{"2026-10-16", "2026-10-15", "2020-02-29"} {
		if got, err := ParseAsOfDate(ok); err != nil || got != ok {
			t.Errorf("%q = %q, %v; want ok", ok, got, err)
		}
	}
	for _, bad := range []string{"", "2026-10-17", "2027-01-01", "2026-13-01", "2026-02-30", "2021-02-29",
		"2026-1-5", "16-10-2026", "2026/10/16", " 2026-10-16", "2026-10-16T00:00:00Z", "today"} {
		if _, err := ParseAsOfDate(bad); !errors.Is(err, ErrInvalidAsOfDate) {
			t.Errorf("%q: err = %v, want ErrInvalidAsOfDate", bad, err)
		}
	}
}

func TestParseTimelineRange_Limits(t *testing.T) {
	// 400 days inclusive is the longest: 2026-01-01 + 399 = 2027-02-04.
	if _, _, err := ParseTimelineRange("2026-01-01", "2027-02-04"); err != nil {
		t.Errorf("400 days inclusive: %v", err)
	}
	for name, c := range map[string][2]string{
		"401 days":     {"2026-01-01", "2027-02-05"},
		"reversed":     {"2026-02-02", "2026-02-01"},
		"missing from": {"", "2026-02-01"},
		"missing to":   {"2026-02-01", ""},
		"malformed":    {"2026-2-1", "2026-02-03"},
		"not a date":   {"2026-02-01", "2026-02-30"},
	} {
		if _, _, err := ParseTimelineRange(c[0], c[1]); !errors.Is(err, ErrInvalidTimelineRange) {
			t.Errorf("%s: err = %v, want ErrInvalidTimelineRange", name, err)
		}
	}
	if f, to, err := ParseTimelineRange("2026-02-01", "2026-02-01"); err != nil || f != to {
		t.Errorf("one day: %q %q %v", f, to, err)
	}
}

// snapStore is the daily job's world: a clock and a store of day -> snapshot.
type snapStore struct {
	rows        map[string]db.InsightsSnapshot
	builds      int
	inserts     int
	hasErr      error
	buildErr    error
	insertErr   error
	raceInsert  bool // another writer lands the row between our check and our insert
	onOrBefore  *db.InsightsSnapshot
	first, last *string
	asOfCalls   struct{ from, to time.Time }
	audit       []db.TrendDay
}

func (s *snapStore) install(t *testing.T, now *time.Time) {
	t.Helper()
	oN, oH, oB, oI, oO, oBo, oA := trendNow, snapshotHas, snapshotBuild, snapshotInsert, snapshotOnOrBefore, snapshotBounds, trendAuditDays
	t.Cleanup(func() {
		trendNow, snapshotHas, snapshotBuild, snapshotInsert, snapshotOnOrBefore, snapshotBounds, trendAuditDays = oN, oH, oB, oI, oO, oBo, oA
	})
	s.rows = map[string]db.InsightsSnapshot{}
	trendNow = func() time.Time { return *now }
	snapshotHas = func(_ context.Context, day string) (bool, error) {
		_, ok := s.rows[day]
		return ok, s.hasErr
	}
	snapshotBuild = func(_ context.Context, day string) (db.InsightsSnapshot, error) {
		s.builds++
		return db.InsightsSnapshot{Day: day}, s.buildErr
	}
	snapshotInsert = func(_ context.Context, snap db.InsightsSnapshot) (bool, error) {
		s.inserts++
		if s.insertErr != nil {
			return false, s.insertErr
		}
		if _, exists := s.rows[snap.Day]; exists || s.raceInsert {
			return false, nil // ON CONFLICT (day) DO NOTHING
		}
		s.rows[snap.Day] = snap
		return true, nil
	}
	snapshotOnOrBefore = func(context.Context, string) (*db.InsightsSnapshot, error) { return s.onOrBefore, nil }
	snapshotBounds = func(context.Context) (*string, *string, error) { return s.first, s.last, nil }
	trendAuditDays = func(_ context.Context, from, to time.Time, _ []string, _ int) ([]db.TrendDay, error) {
		s.asOfCalls.from, s.asOfCalls.to = from, to
		return s.audit, nil
	}
}

func TestTakeDailyInsightsSnapshot_BuildsOncePerLocalDay(t *testing.T) {
	now := at(2026, 10, 16, 9, 0)
	s := &snapStore{}
	s.install(t, &now)
	ctx := context.Background()

	for i := 0; i < 3; i++ { // start-up, then two ticks the same day
		if err := TakeDailyInsightsSnapshot(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if s.builds != 1 || s.inserts != 1 || len(s.rows) != 1 {
		t.Fatalf("builds=%d inserts=%d rows=%d, want 1 each", s.builds, s.inserts, len(s.rows))
	}
	if _, ok := s.rows["2026-10-16"]; !ok {
		t.Fatalf("rows = %v, want the local date 2026-10-16", s.rows)
	}

	// Late evening UTC is already the next day in IST: that is the server's date.
	now = time.Date(2026, 10, 16, 19, 0, 0, 0, time.UTC).In(ist) // 00:30 on the 17th
	if err := TakeDailyInsightsSnapshot(ctx); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.rows["2026-10-17"]; !ok || s.builds != 2 {
		t.Fatalf("next local day: rows = %v builds = %d", s.rows, s.builds)
	}
}

func TestTakeDailyInsightsSnapshot_ConflictIsANoOp(t *testing.T) {
	now := at(2026, 10, 16, 9, 0)
	s := &snapStore{raceInsert: true}
	s.install(t, &now)
	if err := TakeDailyInsightsSnapshot(context.Background()); err != nil {
		t.Fatalf("a lost insert race is not an error: %v", err)
	}
	if s.builds != 1 || s.inserts != 1 || len(s.rows) != 0 {
		t.Fatalf("builds=%d inserts=%d rows=%d", s.builds, s.inserts, len(s.rows))
	}
}

func TestTakeDailyInsightsSnapshot_FailuresAreReturnedAndRetried(t *testing.T) {
	now := at(2026, 10, 16, 9, 0)
	ctx := context.Background()
	for name, setup := range map[string]func(*snapStore){
		"check":  func(s *snapStore) { s.hasErr = errors.New("db down") },
		"build":  func(s *snapStore) { s.buildErr = errors.New("zitadel down") },
		"insert": func(s *snapStore) { s.insertErr = errors.New("write failed") },
	} {
		s := &snapStore{}
		s.install(t, &now)
		setup(s)
		if err := TakeDailyInsightsSnapshot(ctx); err == nil {
			t.Fatalf("%s: want the error, so the runner logs it", name)
		}
		if len(s.rows) != 0 {
			t.Fatalf("%s: a failed run left a row", name)
		}
		// The next tick, with the fault gone, takes the day.
		s.hasErr, s.buildErr, s.insertErr = nil, nil, nil
		if err := TakeDailyInsightsSnapshot(ctx); err != nil || len(s.rows) != 1 {
			t.Fatalf("%s: retry: err=%v rows=%d", name, err, len(s.rows))
		}
	}
}

func str(s string) *string { return &s }

func TestInsightsAsOf_NoSnapshotCarriesTheFirstDay(t *testing.T) {
	now := at(2026, 10, 16, 9, 0)
	ctx := context.Background()

	s := &snapStore{first: str("2026-10-10"), last: str("2026-10-15")}
	s.install(t, &now)
	var none *NoSnapshotError
	_, err := InsightsAsOf(ctx, "2026-10-01")
	if !errors.As(err, &none) || none.FirstDay == nil || *none.FirstDay != "2026-10-10" {
		t.Fatalf("err = %v, want NoSnapshotError with first day 2026-10-10", err)
	}

	s.first, s.last = nil, nil // nothing has ever been stored
	_, err = InsightsAsOf(ctx, "2026-10-01")
	if !errors.As(err, &none) || none.FirstDay != nil {
		t.Fatalf("err = %v, want NoSnapshotError with no first day", err)
	}
}

func TestInsightsAsOf_NearestOnOrBeforeAndChangesSince(t *testing.T) {
	now := at(2026, 10, 16, 9, 0)
	s := &snapStore{
		first: str("2026-10-10"), last: str("2026-10-14"),
		onOrBefore: &db.InsightsSnapshot{
			Day: "2026-10-12", TakenAt: at(2026, 10, 12, 3, 0),
			Topology:   models.TopologyGraph{Nodes: []models.TopologyNode{{ID: "n"}}, Edges: []models.TopologyEdge{}},
			RolePeople: map[string]int{"p:r": 4},
		},
		audit: []db.TrendDay{
			{Day: day(2026, 10, 12), Action: "bundle.assigned", N: 2},
			{Day: day(2026, 10, 12), Action: "direct_grant.removed", N: 1},
			{Day: day(2026, 10, 15), Action: "access_request.approved", N: 3},
		},
	}
	s.install(t, &now)
	got, err := InsightsAsOf(context.Background(), "2026-10-13") // asked the 13th; the 12th is the latest on or before
	if err != nil {
		t.Fatal(err)
	}
	if got.Day != "2026-10-12" || got.FirstDay != "2026-10-10" || got.ChangesSince != 6 || got.RolePeople["p:r"] != 4 || len(got.Topology.Nodes) != 1 {
		t.Fatalf("got %+v", got)
	}
	if want := at(2026, 10, 12, 0, 0); !s.asOfCalls.from.Equal(want) {
		t.Errorf("counted from %v, want local midnight %v", s.asOfCalls.from, want)
	}
}

func TestInsightsAsOf_CountsTheSameActionsAsTrends(t *testing.T) {
	want := append(append(append(append([]string{}, actionsThroughBundle...), actionsByRequest...), actionsByHand...), actionsTakenAway...)
	if got := accessChangeActions(); !reflect.DeepEqual(got, want) {
		t.Fatalf("actions = %v, want %v", got, want)
	}
}

func TestInsightsTimeline_FillsEveryDayAndReportsSnapshotDays(t *testing.T) {
	now := at(2026, 10, 16, 9, 0)
	s := &snapStore{
		first: str("2026-10-14"), last: str("2026-10-16"),
		audit: []db.TrendDay{
			{Day: day(2026, 10, 14), Action: "bundle.assigned", N: 2},
			{Day: day(2026, 10, 14), Action: "direct_grant.revoked", N: 1},
			{Day: day(2026, 10, 16), Action: "access_request.approved", N: 5},
		},
	}
	s.install(t, &now)
	got, err := InsightsTimeline(context.Background(), "2026-10-13", "2026-10-16")
	if err != nil {
		t.Fatal(err)
	}
	want := []models.TimelineDay{{Day: "2026-10-13"}, {Day: "2026-10-14", Changes: 3}, {Day: "2026-10-15"}, {Day: "2026-10-16", Changes: 5}}
	if !reflect.DeepEqual(got.Days, want) {
		t.Fatalf("days = %v, want %v", got.Days, want)
	}
	if got.SnapshotDays[0] == nil || *got.SnapshotDays[0] != "2026-10-14" || *got.SnapshotDays[1] != "2026-10-16" {
		t.Fatalf("snapshot days = %v", got.SnapshotDays)
	}
	if !s.asOfCalls.to.Equal(at(2026, 10, 17, 0, 0)) {
		t.Errorf("window ends %v, want the midnight after the last day", s.asOfCalls.to)
	}

	s.first, s.last = nil, nil
	got, _ = InsightsTimeline(context.Background(), "2026-10-13", "2026-10-13")
	if got.SnapshotDays != [2]*string{} || len(got.Days) != 1 {
		t.Fatalf("empty store: %+v", got)
	}
}
