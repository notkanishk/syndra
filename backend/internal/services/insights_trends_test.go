package services

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"syndra/internal/db"
	"syndra/internal/directory"
	"syndra/internal/models"
)

var ist = time.FixedZone("IST", 5*3600+1800)

func at(y int, m time.Month, d, h, min int) time.Time { return time.Date(y, m, d, h, min, 0, 0, ist) }

func TestParseTrendPeriod(t *testing.T) {
	for in, want := range map[string]TrendPeriod{"": TrendTerm, "term": TrendTerm, "30d": Trend30Days, "year": TrendYear} {
		if got, err := ParseTrendPeriod(in); err != nil || got != want {
			t.Errorf("%q = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"week", "30D", "Term", " term", "90d", "all", "term,year"} {
		if _, err := ParseTrendPeriod(in); !errors.Is(err, ErrInvalidTrendPeriod) {
			t.Errorf("%q: err = %v, want ErrInvalidTrendPeriod", in, err)
		}
	}
}

func TestWindowFor_TermsAndBoundaries(t *testing.T) {
	for _, tc := range []struct {
		now             time.Time
		from, to, label string
	}{
		{at(2026, 1, 1, 0, 0), "2026-01-01", "2026-05-31", "Spring 2026"},    // first day of Spring
		{at(2026, 3, 15, 12, 0), "2026-01-01", "2026-05-31", "Spring 2026"},  // inside
		{at(2026, 5, 31, 23, 59), "2026-01-01", "2026-05-31", "Spring 2026"}, // last minute of Spring
		{at(2026, 6, 1, 0, 0), "2026-06-01", "2026-07-31", "Summer 2026"},    // first minute of Summer
		{at(2026, 7, 31, 23, 59), "2026-06-01", "2026-07-31", "Summer 2026"},
		{at(2026, 8, 1, 0, 0), "2026-08-01", "2026-12-31", "Monsoon 2026"}, // first minute of Monsoon
		{at(2026, 10, 16, 10, 0), "2026-08-01", "2026-12-31", "Monsoon 2026"},
		{at(2026, 12, 31, 23, 59), "2026-08-01", "2026-12-31", "Monsoon 2026"}, // Monsoon never spills into next year
	} {
		w := windowFor(TrendTerm, tc.now)
		if w.From.Format(dateLayout) != tc.from || w.To.Format(dateLayout) != tc.to || w.Label != tc.label {
			t.Errorf("%s: got %s..%s %q, want %s..%s %q", tc.now.Format(time.RFC3339),
				w.From.Format(dateLayout), w.To.Format(dateLayout), w.Label, tc.from, tc.to, tc.label)
		}
		if w.From.Location() != ist || w.From.Hour() != 0 {
			t.Errorf("%s: window must start at local midnight, got %s", tc.now, w.From)
		}
	}
}

func TestWindowFor_RollingPeriodsEndToday(t *testing.T) {
	now := at(2026, 10, 16, 10, 0)
	w := windowFor(Trend30Days, now)
	if w.From.Format(dateLayout) != "2026-09-17" || w.To.Format(dateLayout) != "2026-10-16" || w.Label != "Last 30 days" {
		t.Errorf("30d = %v", w)
	}
	w = windowFor(TrendYear, now)
	if w.From.Format(dateLayout) != "2025-10-17" || w.To.Format(dateLayout) != "2026-10-16" {
		t.Errorf("year = %v", w)
	}
	// 365 days counted inclusively, across a leap day.
	w = windowFor(TrendYear, at(2028, 3, 1, 9, 0))
	if got := w.To.Sub(w.From).Hours() / 24; got != 364 {
		t.Errorf("year spans %v days apart, want 364 (365 inclusive)", got)
	}
}

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func TestBucketWeeks_MondayStartsAndZeroWeeksStay(t *testing.T) {
	days := []db.TrendDay{
		{Day: day(2026, 10, 2), Action: "bundle.assigned", N: 2},         // Fri, week of 09-28
		{Day: day(2026, 10, 4), Action: "bundle.unassigned", N: 1},       // Sun, still week of 09-28
		{Day: day(2026, 10, 5), Action: "access_request.approved", N: 3}, // Mon starts week of 10-05
		{Day: day(2026, 10, 5), Action: "direct_grant.upserted", N: 4},
		{Day: day(2026, 10, 6), Action: "access_request.self_approved", N: 1},
		{Day: day(2026, 10, 7), Action: "direct_grant.revoked_by_expiry", N: 5},
		{Day: day(2026, 10, 19), Action: "bundle.assigned", N: 7},  // Mon, week of 10-19
		{Day: day(2026, 10, 6), Action: "bundle.created", N: 99},   // not a change of access
		{Day: day(2026, 9, 20), Action: "bundle.assigned", N: 50},  // before the window's first week
		{Day: day(2026, 10, 26), Action: "bundle.assigned", N: 50}, // after the last week
	}
	// from Thursday 10-01 through Tuesday 10-20: weeks 09-28, 10-05, 10-12, 10-19.
	got, totals := bucketWeeks(days, at(2026, 10, 1, 0, 0), at(2026, 10, 20, 0, 0))
	want := []models.TrendWeek{
		{WeekStart: "2026-09-28", ThroughBundle: 2, TakenAway: 1},
		{WeekStart: "2026-10-05", ByRequest: 4, ByHand: 4, TakenAway: 5},
		{WeekStart: "2026-10-12"},
		{WeekStart: "2026-10-19", ThroughBundle: 7},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("weeks =\n%+v\nwant\n%+v", got, want)
	}
	if totals.Given != 2+4+4+7 || totals.TakenAway != 6 {
		t.Fatalf("totals = %+v", totals)
	}
}

func TestBucketWeeks_NoDaysStillGivesTheWeeks(t *testing.T) {
	got, totals := bucketWeeks(nil, at(2026, 10, 12, 0, 0), at(2026, 10, 12, 0, 0))
	if len(got) != 1 || got[0].WeekStart != "2026-10-12" || totals != (models.TrendTotals{}) {
		t.Fatalf("weeks = %+v totals %+v", got, totals)
	}
}

func TestGroupReasons(t *testing.T) {
	got := groupReasons([]string{
		"Not needed", "  not needed ", "NOT NEEDED", "", "   ", "Ask your lead", "ask your lead",
		"Too broad", "Out of scope", "Duplicate", "Wrong project",
	}, 5)
	want := []models.TrendReason{
		{Reason: "Not needed", N: 3}, // first-seen wording, trimmed
		{Reason: "Ask your lead", N: 2},
		{Reason: "No reason given", N: 2},
		{Reason: "Duplicate", N: 1}, // ties break A to Z; Out of scope, Too broad, Wrong project fall off
		{Reason: "Out of scope", N: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("reasons = %+v\nwant %+v", got, want)
	}
	if got := groupReasons(nil, 5); got == nil || len(got) != 0 {
		t.Fatalf("no notes must be an empty list, got %#v", got)
	}
}

func TestMedian(t *testing.T) {
	if _, ok := median(nil); ok {
		t.Error("empty has no median")
	}
	if m, ok := median([]float64{7}); !ok || m != 7 {
		t.Errorf("one = %v %v", m, ok)
	}
	in := []float64{9, 1, 5}
	if m, _ := median(in); m != 5 {
		t.Errorf("odd = %v, want 5", m)
	}
	if !reflect.DeepEqual(in, []float64{9, 1, 5}) {
		t.Errorf("median reordered its input: %v", in)
	}
	if m, _ := median([]float64{4, 1, 3, 2}); m != 2.5 {
		t.Errorf("even = %v, want 2.5", m)
	}
}

func TestMedianOver(t *testing.T) {
	if medianOver(nil, 3600) != nil {
		t.Error("nothing to take a median of is nil, not zero")
	}
	// 5400s = 1.5h; 4000s/3600 = 1.111 -> one decimal.
	if v := medianOver([]float64{5400}, 3600); v == nil || *v != 1.5 {
		t.Errorf("1.5h = %v", v)
	}
	if v := medianOver([]float64{4000}, 3600); v == nil || *v != 1.1 {
		t.Errorf("rounds to one decimal = %v", v)
	}
	if v := medianOver([]float64{0}, 60); v == nil || *v != 0 {
		t.Errorf("a median of zero is zero, not absent: %v", v)
	}
}

// trendWorld stubs every read the report makes.
type trendWorld struct {
	users    []models.UserProfile
	projects []models.ProjectCatalog
	holds    map[string]int
	findings models.InsightsFindings

	days    []db.TrendDay
	reqs    db.RequestTrendRows
	outside []models.TrendOutsideRole
	sends   []float64
	drift   db.DriftTrendRows
	ending  []db.EndingSoonRow
	open    int

	failAt string

	gotFrom, gotTo time.Time
	gotOffset      int
	gotActions     []string
	snapshots      int
	askedHolds     []string
}

func (w *trendWorld) install(t *testing.T, now time.Time) {
	t.Helper()
	oDir, oSnap, oNow := directory.Default, svcInReadSnapshot, trendNow
	oF, oD, oR, oO, oS, oDr, oE, oOpen, oH := trendFindings, trendAuditDays, trendRequests, trendOutside,
		trendSendSeconds, trendDrift, trendEndingSoon, trendDriftOpen, trendRoleHolders
	t.Cleanup(func() {
		directory.Default, svcInReadSnapshot, trendNow = oDir, oSnap, oNow
		trendFindings, trendAuditDays, trendRequests, trendOutside = oF, oD, oR, oO
		trendSendSeconds, trendDrift, trendEndingSoon, trendDriftOpen, trendRoleHolders = oS, oDr, oE, oOpen, oH
	})
	fail := func(at string) error {
		if w.failAt == at {
			return errors.New(at + " failed")
		}
		return nil
	}
	directory.Default = &snapshotFixtureDirectory{users: w.users, projs: w.projects}
	trendNow = func() time.Time { return now }
	svcInReadSnapshot = func(ctx context.Context, fn func(context.Context) error) error {
		w.snapshots++
		return fn(ctx)
	}
	trendFindings = func(context.Context) (models.InsightsFindings, error) { return w.findings, fail("findings") }
	trendRoleHolders = func(_ context.Context, id string) (int, error) {
		w.askedHolds = append(w.askedHolds, id)
		return w.holds[id], nil
	}
	trendAuditDays = func(_ context.Context, from, to time.Time, actions []string, off int) ([]db.TrendDay, error) {
		w.gotFrom, w.gotTo, w.gotOffset, w.gotActions = from, to, off, actions
		return w.days, fail("days")
	}
	trendRequests = func(context.Context, time.Time, time.Time) (db.RequestTrendRows, error) {
		return w.reqs, fail("requests")
	}
	trendOutside = func(_ context.Context, _, _ time.Time, limit int) ([]models.TrendOutsideRole, error) {
		if limit != 5 {
			t.Errorf("outside limit = %d, want 5", limit)
		}
		return w.outside, fail("outside")
	}
	trendSendSeconds = func(context.Context, time.Time, time.Time) ([]float64, error) { return w.sends, fail("sends") }
	trendDrift = func(context.Context, time.Time, time.Time) (db.DriftTrendRows, error) { return w.drift, fail("drift") }
	trendEndingSoon = func(context.Context, time.Time) ([]db.EndingSoonRow, error) { return w.ending, fail("ending") }
	trendDriftOpen = func(context.Context) (int, error) { return w.open, fail("open") }
}

func TestInsightsTrends_AssemblesEveryPart(t *testing.T) {
	now := at(2026, 10, 16, 10, 0) // Monsoon 2026
	w := &trendWorld{
		users: []models.UserProfile{
			{ID: "gone-holding", Status: "inactive"},
			{ID: "gone-holding2", Status: "Deleted"}, // any case
			{ID: "gone-clean", Status: "inactive"},
			{ID: "here", Status: "active"},
		},
		projects: []models.ProjectCatalog{{ID: "p1", Name: "Laser room"}},
		holds:    map[string]int{"gone-holding": 2, "gone-holding2": 1, "here": 9},
		findings: models.InsightsFindings{
			Unused: models.InsightUnused{
				Roles:   []models.RoleGrant{{ProjectID: "p", RoleKey: "a"}, {ProjectID: "p", RoleKey: "b"}},
				Bundles: []models.InsightUnusedBundle{{BundleID: "b1"}},
			},
			Overlaps: []models.InsightOverlap{{}, {}, {}},
		},
		days: []db.TrendDay{
			{Day: day(2026, 10, 12), Action: "bundle.assigned", N: 2},
			{Day: day(2026, 10, 13), Action: "direct_grant.removed", N: 1},
		},
		reqs: db.RequestTrendRows{
			CameIn: 6, Approved: 3, Declined: 2, Withdrawn: 1,
			DecideSeconds: []float64{3600, 7200, 36000},
			DeclineNotes:  []string{"No", "no ", ""},
		},
		outside: []models.TrendOutsideRole{{ProjectID: "p9", RoleKey: "x", N: 4}},
		sends:   []float64{60, 180},
		drift:   db.DriftTrendRows{Found: 4, Cleared: 3, ClearSeconds: []float64{7200, 14400, 21600}},
		ending: []db.EndingSoonRow{
			{ProjectID: "p1", D30: 1, D60: 2, D90: 3},
			{ProjectID: "gone-project", D30: 0, D60: 0, D90: 1},
		},
		open: 7,
	}
	w.install(t, now)

	got, err := InsightsTrends(context.Background(), TrendTerm)
	if err != nil {
		t.Fatal(err)
	}

	if got.Window != (models.TrendWindow{From: "2026-08-01", To: "2026-12-31", Label: "Monsoon 2026"}) {
		t.Errorf("window = %+v", got.Window)
	}
	// The query window is [first day 00:00, day after last 00:00) in local time.
	if !w.gotFrom.Equal(at(2026, 8, 1, 0, 0)) || !w.gotTo.Equal(at(2027, 1, 1, 0, 0)) || w.gotOffset != 5*3600+1800 {
		t.Errorf("queried %s .. %s offset %d", w.gotFrom, w.gotTo, w.gotOffset)
	}
	// Weekly runs through today's week, not on to the end of a term that has not happened.
	if n := len(got.Weekly); got.Weekly[0].WeekStart != "2026-07-27" || got.Weekly[n-1].WeekStart != "2026-10-12" {
		t.Errorf("weekly runs %s..%s", got.Weekly[0].WeekStart, got.Weekly[n-1].WeekStart)
	}
	if got.Totals != (models.TrendTotals{Given: 2, TakenAway: 1}) {
		t.Errorf("totals = %+v", got.Totals)
	}

	r := got.Requests
	if r.CameIn != 6 || r.Approved != 3 || r.Declined != 2 || r.Withdrawn != 1 ||
		r.MedianHoursToDecide == nil || *r.MedianHoursToDecide != 2 ||
		!reflect.DeepEqual(r.DeclinedReasons, []models.TrendReason{{Reason: "No", N: 2}, {Reason: "No reason given", N: 1}}) ||
		!reflect.DeepEqual(r.OutsideBundles, w.outside) {
		t.Errorf("requests = %+v", r)
	}
	if got.Sending.Sample != 2 || got.Sending.MedianMinutesApprovalToSent == nil || *got.Sending.MedianMinutesApprovalToSent != 2 {
		t.Errorf("sending = %+v", got.Sending)
	}
	if got.Drift.Found != 4 || got.Drift.Cleared != 3 || got.Drift.MedianHoursToClear == nil || *got.Drift.MedianHoursToClear != 4 {
		t.Errorf("drift = %+v", got.Drift)
	}
	wantEnding := []models.TrendEndingSoon{
		{ProjectID: "p1", ProjectName: "Laser room", D30: 1, D60: 2, D90: 3},
		{ProjectID: "gone-project", ProjectName: "gone-project", D90: 1}, // unnamed -> its id
	}
	if !reflect.DeepEqual(got.EndingSoon, wantEnding) {
		t.Errorf("ending soon = %+v", got.EndingSoon)
	}
	// Two of the three people who have left still hold something; nobody else was asked.
	if want := (models.TrendUpkeep{PeopleWhoLeftHolding: 2, DriftOpen: 7, RolesNobodyHas: 2, BundlesNothingNew: 1, Overlaps: 3}); got.Upkeep != want {
		t.Errorf("upkeep = %+v, want %+v", got.Upkeep, want)
	}
	if !reflect.DeepEqual(w.askedHolds, []string{"gone-holding", "gone-holding2", "gone-clean"}) {
		t.Errorf("asked what these hold: %v", w.askedHolds)
	}
	if w.snapshots != 1 {
		t.Errorf("snapshots = %d: every database read belongs in one", w.snapshots)
	}
	// Every action the buckets know is asked for, once.
	if want := len(actionsThroughBundle) + len(actionsByRequest) + len(actionsByHand) + len(actionsTakenAway); len(w.gotActions) != want {
		t.Errorf("asked for %d actions, want %d", len(w.gotActions), want)
	}
}

func TestInsightsTrends_NothingHappenedIsNullMediansAndEmptyLists(t *testing.T) {
	w := &trendWorld{}
	w.install(t, at(2026, 3, 2, 9, 0))
	got, err := InsightsTrends(context.Background(), Trend30Days)
	if err != nil {
		t.Fatal(err)
	}
	if got.Requests.MedianHoursToDecide != nil || got.Sending.MedianMinutesApprovalToSent != nil || got.Drift.MedianHoursToClear != nil {
		t.Errorf("no data must be null, got %+v %+v %+v", got.Requests, got.Sending, got.Drift)
	}
	if got.Requests.DeclinedReasons == nil || got.EndingSoon == nil || got.Weekly == nil {
		t.Errorf("lists must be [] not null: %+v", got)
	}
	if got.Sending.Sample != 0 {
		t.Errorf("sample = %d", got.Sending.Sample)
	}
}

func TestInsightsTrends_AnyFailingReadFails(t *testing.T) {
	for _, at0 := range []string{"findings", "days", "requests", "outside", "sends", "drift", "ending", "open"} {
		w := &trendWorld{failAt: at0}
		w.install(t, at(2026, 10, 16, 10, 0))
		if _, err := InsightsTrends(context.Background(), TrendTerm); err == nil {
			t.Errorf("%s failing was swallowed", at0)
		}
	}
}
