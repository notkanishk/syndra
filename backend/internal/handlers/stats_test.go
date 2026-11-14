package handlers

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"syndra/internal/addons"
	"syndra/internal/auth"
	"syndra/internal/clock"
	"syndra/internal/db"
	"syndra/internal/models"
	"syndra/internal/services/addonop"
)

// Statistics, read (addon-contract 6.8): a member their own, an admin the
// rest, every day a makerspace day.

// statsStore is Syndra's statistics for one target, by subject ("" totals),
// and every subject a read asked for.
type statsStore struct {
	asked []string
	// retention is the target's per-person days (400 when 0); from, the day
	// each points read started on.
	retention int
	from      []string
}

func stubStats(t *testing.T, s *statsStore) {
	t.Helper()
	prev := []any{dbStatsMetrics, dbStatsDays, dbStatsMonths, dbStatsGaps, dbStatsDistributions, dbStatsRetention, dbStatsKeptFrom}
	t.Cleanup(func() {
		dbStatsMetrics = prev[0].(func(context.Context, string) ([]db.StatsMetric, error))
		dbStatsDays = prev[1].(func(context.Context, string, string, string) ([]db.StatsValue, error))
		dbStatsMonths = prev[2].(func(context.Context, string, string, string) ([]db.StatsValue, error))
		dbStatsGaps = prev[3].(func(context.Context, string, string) ([]db.StatsGapRow, error))
		dbStatsDistributions = prev[4].(func(context.Context, string, string, string) ([]db.StatsDistribution, error))
		dbStatsRetention = prev[5].(func(context.Context, string) (int, bool, error))
		dbStatsKeptFrom = prev[6].(func(context.Context, string, string) (string, error))
	})
	watermark := time.Date(2026, 11, 10, 18, 30, 0, 0, time.UTC)
	dbStatsMetrics = func(_ context.Context, target string) ([]db.StatsMetric, error) {
		if target != "truenas" {
			return nil, nil
		}
		return []db.StatsMetric{
			{Metric: "storage.home_bytes", Descriptor: json.RawMessage(`{"name":"storage.home_bytes","instrument":"gauge","unit":"bytes","label":"Home folder used","per_subject":true,"against":"home_quota"}`), Watermark: &watermark},
			{Metric: "storage.sessions", Descriptor: json.RawMessage(`{"name":"storage.sessions","instrument":"counter","unit":"count","label":"Storage sessions","per_subject":true,"dimensions":[{"name":"share","label":"Share"}]}`), Watermark: &watermark,
				UnboundAccounts: 2, Dropped: map[string]int{"series": 1}},
			{Metric: "storage.total_bytes", Descriptor: json.RawMessage(`{"name":"storage.total_bytes","instrument":"gauge","unit":"bytes","label":"Pool used"}`)},
		}, nil
	}
	// Each subject's own numbers: u1 has used 3 + 4 sessions, u2 90.
	values := map[string]float64{"u1": 3, "u2": 90, "": 500}
	dbStatsDays = func(_ context.Context, _, subject, from string) ([]db.StatsValue, error) {
		s.asked, s.from = append(s.asked, subject), append(s.from, from)
		v := values[subject]
		out := []db.StatsValue{
			{Metric: "storage.home_bytes", Dims: map[string]string{}, When: "2026-11-09", Value: v * 1000},
			{Metric: "storage.home_bytes", Dims: map[string]string{}, When: "2026-11-11", Value: v * 2000},
			{Metric: "storage.sessions", Dims: map[string]string{"share": "makers"}, When: "2026-11-10", Value: v},
			{Metric: "storage.sessions", Dims: map[string]string{"share": "makers"}, When: "2026-11-11", Value: v + 1},
		}
		if subject == "" {
			out = append(out, db.StatsValue{Metric: "storage.total_bytes", Dims: map[string]string{}, When: "2026-11-11", Value: 7})
		}
		return out, nil
	}
	dbStatsMonths = func(_ context.Context, _, subject, from string) ([]db.StatsValue, error) {
		s.asked, s.from = append(s.asked, subject), append(s.from, from)
		v := values[subject]
		return []db.StatsValue{
			{Metric: "storage.home_bytes", Dims: map[string]string{}, When: "2026-11", Value: 1},
			{Metric: "storage.sessions", Dims: map[string]string{"share": "makers"}, When: "2026-10", Value: 10 * v},
			{Metric: "storage.sessions", Dims: map[string]string{"share": "makers"}, When: "2026-11", Value: 2*v + 1},
			{Metric: "storage.sessions", Dims: map[string]string{"share": "projects"}, When: "2026-09", Value: v},
		}, nil
	}
	dbStatsGaps = func(context.Context, string, string) ([]db.StatsGapRow, error) {
		return []db.StatsGapRow{{Metric: "storage.sessions", StatsGap: db.StatsGap{Dims: map[string]string{"share": "scratch"}, Start: "2026-11-08", End: "2026-11-10", Reason: "Auditing is off on this share"}}}, nil
	}
	dbStatsRetention = func(context.Context, string) (int, bool, error) { return 400, false, nil }
	// The db's line (StatsKeptFrom, proved against the prune live): N days
	// counting today, today − (N − 1).
	dbStatsKeptFrom = func(_ context.Context, today, _ string) (string, error) {
		return clock.AddDate(today, 0, 0, 1-cmp.Or(s.retention, 400)), nil
	}
}

// myUsage is the usage block on the one target a page returns.
func myUsage(t *testing.T, rr *httptest.ResponseRecorder) statsUsage {
	t.Helper()
	var body struct {
		Targets []struct {
			Target string      `json:"target"`
			Usage  *statsUsage `json:"usage"`
		} `json:"targets"`
	}
	if rr.Code != http.StatusOK || json.Unmarshal(rr.Body.Bytes(), &body) != nil || len(body.Targets) != 1 || body.Targets[0].Usage == nil {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
	return *body.Targets[0].Usage
}

func sessionsOf(t *testing.T, u statsUsage) statsMetricView {
	t.Helper()
	for _, m := range u.Metrics {
		if m.Name == "storage.sessions" {
			return m
		}
	}
	t.Fatalf("no storage.sessions in %+v", u)
	return statsMetricView{}
}

// A member reads their own points and nobody else's, whatever the query says;
// an admin reads one person's on that person's page.
func TestMyUsageIsTheActorsOwn(t *testing.T) {
	h := &personStorageHarness{storageHarness: storageHarness{entitled: true, bound: true}}
	stubPersonStorage(t, h)
	manifest := addonsAcceptedShape
	t.Cleanup(func() { addonsAcceptedShape = manifest })
	addonsAcceptedShape = func(context.Context, string) (addons.Shape, error) {
		return addons.Shape{Kind: "storage", EntitlementSchema: truenasSchema}, nil
	}
	s := &statsStore{}
	stubStats(t, s)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/targets?subject=u2&id=u2&user=u2", nil)
	req = req.WithContext(withPrincipal(req.Context(), &auth.Principal{Subject: "u1"}))
	rr := httptest.NewRecorder()
	handleMyTargets(rr, req)
	usage := myUsage(t, rr)
	for _, asked := range s.asked {
		if asked != "u1" {
			t.Fatalf("the member's page read %q's points", asked)
		}
	}
	if got := sessionsOf(t, usage).Series[0].Days[0].Value; got != 3 {
		t.Errorf("the member sees %v sessions on their first day, want their own 3", got)
	}
	for _, m := range usage.Metrics {
		if !m.PerSubject || m.Collected != nil {
			t.Errorf("a person's page carries %s (per person %t, collection line %v)", m.Name, m.PerSubject, m.Collected)
		}
	}

	s.asked = nil
	req = httptest.NewRequest(http.MethodGet, "/api/v1/users/u2/targets", nil)
	req.SetPathValue("id", "u2")
	req = req.WithContext(withPrincipal(req.Context(), &auth.Principal{Subject: "op-1"}))
	rr = httptest.NewRecorder()
	handleUserTargets(rr, req)
	if got := sessionsOf(t, myUsage(t, rr)).Series[0].Days[0].Value; got != 90 || s.asked[0] != "u2" {
		t.Errorf("the person page shows %v for %v, want u2's 90", got, s.asked)
	}
}

// The shape 6e renders: a counter's days, months and both sums (zero, not
// absent, for a window it counted nothing in), a gauge's latest day, the
// metric's gaps, series and metrics in reading order; and "reported false"
// for a target that reports nothing.
func TestUsageCarriesSumsLatestAndGaps(t *testing.T) {
	stubStats(t, &statsStore{})
	now := time.Date(2026, 11, 12, 6, 0, 0, 0, time.UTC)
	usage, err := statsUsageFor(context.Background(), "truenas", "u1", false, now)
	if err != nil || !usage.Reported || len(usage.Metrics) != 2 {
		t.Fatalf("%+v %v", usage, err)
	}
	if usage.Metrics[0].Label != "Home folder used" || usage.Metrics[1].Label != "Storage sessions" {
		t.Errorf("metrics in order %s, %s", usage.Metrics[0].Label, usage.Metrics[1].Label)
	}
	if usage.Today != "2026-11-12" {
		t.Errorf("today is the makerspace day the response was built on: %q", usage.Today)
	}
	home := usage.Metrics[0]
	if len(home.Series) != 1 || home.Series[0].Latest == nil || home.Series[0].Latest.Day != "2026-11-11" || home.Series[0].Latest.Value != 6000 ||
		home.Series[0].Sum30d != nil || home.Series[0].Months != nil || home.Against != "home_quota" {
		t.Errorf("gauge %+v", home.Series)
	}
	sessions := sessionsOf(t, usage)
	makers, projects := sessions.Series[0], sessions.Series[1]
	if makers.Dimensions["share"] != "makers" || *makers.Sum30d != 7 || *makers.Sum12m != 37 || len(makers.Months) != 2 || len(makers.Days) != 2 {
		t.Errorf("makers %+v", makers)
	}
	if projects.Dimensions["share"] != "projects" || projects.Sum30d == nil || *projects.Sum30d != 0 || *projects.Sum12m != 3 || len(projects.Days) != 0 {
		t.Errorf("a series with no day in 30 sums 0 there: %+v", projects)
	}
	if len(sessions.Gaps) != 0 {
		t.Errorf("a gap on a share u1 never used is on u1's page: %+v", sessions.Gaps)
	}

	totals, _ := statsUsageFor(context.Background(), "truenas", "", false, now)
	if g := sessionsOf(t, totals).Gaps; len(g) != 1 || g[0].Reason != "Auditing is off on this share" || g[0].End != "2026-11-10" {
		t.Errorf("gaps %+v", g)
	}
	if len(totals.Metrics) != 3 || sessionsOf(t, totals).Collected == nil || sessionsOf(t, totals).Collected.UnboundAccounts != 2 {
		t.Errorf("the totals carry every metric and the collection line: %+v", totals)
	}
	if none, _ := statsUsageFor(context.Background(), "unifi", "u1", false, now); none.Reported || none.Metrics != nil {
		t.Errorf("a target reporting nothing says so: %+v", none)
	}
}

// A person's page says from which day retention still keeps their points,
// and reads none from before it (the prune runs daily from start-up, so a day past
// retention can still be stored); every metric says its last final day. The
// totals keep five years and say neither.
func TestUsageSaysWhatRetentionKeptAndWhatIsFinal(t *testing.T) {
	inIST(t)
	now := time.Date(2026, 11, 12, 20, 0, 0, 0, time.UTC) // 13 Nov, 01:30 in the makerspace
	for _, c := range []struct {
		retention              int
		keptFrom, days, months string
	}{
		{0, "2025-10-10", "2026-10-15", "2025-12-01"},   // the 400-day ceiling: both windows whole
		{350, "2025-11-29", "2026-10-15", "2025-12-01"}, // 350 days reach past the 12 months' first day: whole
		{340, "2025-12-09", "2026-10-15", "2025-12-09"}, // 340 do not: the year starts where retention does
		{90, "2026-08-16", "2026-10-15", "2026-08-16"},  // as it does at 90
		{30, "2026-10-15", "2026-10-15", "2026-10-15"},  // 30 days are exactly the 30-day window
		{10, "2026-11-04", "2026-11-04", "2026-11-04"},  // 10 start both windows
	} {
		s := &statsStore{retention: c.retention}
		stubStats(t, s)
		usage, err := statsUsageFor(context.Background(), "truenas", "u1", false, now)
		if err != nil {
			t.Fatal(err)
		}
		if usage.Today != "2026-11-13" { // the makerspace's day, not UTC's 12th
			t.Errorf("retention %d: today %q, want the makerspace day 2026-11-13", c.retention, usage.Today)
		}
		if usage.KeptFrom != c.keptFrom {
			t.Errorf("retention %d: kept_from %q, want %q", c.retention, usage.KeptFrom, c.keptFrom)
		}
		if len(s.from) != 2 || s.from[0] != c.days || s.from[1] != c.months {
			t.Errorf("retention %d: points read from %v, want days from %s and months from %s", c.retention, s.from, c.days, c.months)
		}
		// The page names each window from the start the read used, never its own.
		if usage.Window30dFrom != c.days || usage.Window12mFrom != c.months {
			t.Errorf("retention %d: says windows from %s and %s, want %s and %s", c.retention, usage.Window30dFrom, usage.Window12mFrom, c.days, c.months)
		}
		for _, m := range usage.Metrics {
			if got, _ := json.Marshal(m.CountedThrough); string(got) != `"2026-11-10"` {
				t.Errorf("%s counted through %s, want 2026-11-10 (watermark midnight 11 Nov)", m.Name, got)
			}
		}
	}

	s := &statsStore{retention: 10}
	stubStats(t, s)
	totals, _ := statsUsageFor(context.Background(), "truenas", "", false, now)
	if totals.KeptFrom != "" || s.from[0] != "2026-10-15" || s.from[1] != "2025-12-01" || totals.Window30dFrom != "2026-10-15" || totals.Window12mFrom != "2025-12-01" {
		t.Errorf("the totals say kept_from %q and read from %v: a per-person setting is not theirs", totals.KeptFrom, s.from)
	}
	for _, m := range totals.Metrics {
		if want := m.Watermark != nil; (m.CountedThrough != nil) != want {
			t.Errorf("%s: counted_through %v with watermark %v", m.Name, m.CountedThrough, m.Watermark)
		}
	}
	raw, _ := json.Marshal(totals)
	if !strings.Contains(string(raw), `"counted_through":null`) || strings.Contains(string(raw), "kept_from") {
		t.Errorf("a metric never collected says counted_through null; the totals no kept_from: %s", raw)
	}
}

// A person's page carries a gap over the whole metric or over one of their own
// series, never one on a share only somebody else used: edge never sees the
// kiln, which only lita used; lita and the totals see every gap.
func TestAPersonSeesOnlyGapsOnTheirOwnSeries(t *testing.T) {
	stubStats(t, &statsStore{})
	used := map[string][]string{"edge": {"makers"}, "lita": {"makers", "kiln"}, "": {"makers", "kiln"}}
	dbStatsDays = func(_ context.Context, _, subject, _ string) ([]db.StatsValue, error) {
		var out []db.StatsValue
		for _, share := range used[subject] {
			out = append(out, db.StatsValue{Metric: "storage.sessions", Dims: map[string]string{"share": share}, When: "2026-11-11", Value: 1})
		}
		return out, nil
	}
	dbStatsMonths = func(context.Context, string, string, string) ([]db.StatsValue, error) { return nil, nil }
	dbStatsGaps = func(context.Context, string, string) ([]db.StatsGapRow, error) {
		var out []db.StatsGapRow
		for _, dims := range []map[string]string{{}, {"share": "makers"}, {"share": "kiln"}} {
			out = append(out, db.StatsGapRow{Metric: "storage.sessions", StatsGap: db.StatsGap{Dims: dims, Start: "2026-11-08", End: "2026-11-10", Reason: "Auditing was off"}})
		}
		return out, nil
	}
	now := time.Date(2026, 11, 12, 6, 0, 0, 0, time.UTC)
	for subject, want := range map[string]string{"edge": `{"share":"makers"} {}`, "lita": `{"share":"kiln"} {"share":"makers"} {}`, "": `{"share":"kiln"} {"share":"makers"} {}`} {
		usage, err := statsUsageFor(context.Background(), "truenas", subject, false, now)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, g := range sessionsOf(t, usage).Gaps {
			got = append(got, dimsText(g.Dimensions))
		}
		slices.Sort(got)
		if strings.Join(got, " ") != want {
			t.Errorf("%q sees gaps on %v, want %s", subject, got, want)
		}
	}
}

// A gap over part of a series' dimensions is on that series: on a metric by
// share and protocol, the gap {"share":"makers"} is on edge's makers/smb
// series, so edge sees it; lita never used makers and does not, nor a gap on
// makers over nfs, which edge never used either. Every key=value of the gap
// must be in one series, not just one of them; a key the series lacks is not
// in it, even with an empty value. A gap over the whole metric ({}) is on
// every person's page, orton's too, who has no series here.
func TestAGapOnPartOfASeriesIsOnThatSeries(t *testing.T) {
	stubStats(t, &statsStore{})
	dbStatsMetrics = func(context.Context, string) ([]db.StatsMetric, error) {
		return []db.StatsMetric{{Metric: "storage.sessions", Descriptor: json.RawMessage(`{"name":"storage.sessions","instrument":"counter","unit":"count","label":"Storage sessions","per_subject":true,"dimensions":[{"name":"share","label":"Share"},{"name":"protocol","label":"Protocol"}]}`)}}, nil
	}
	used := map[string]map[string]string{"edge": {"share": "makers", "protocol": "smb"}, "lita": {"share": "kiln", "protocol": "smb"}}
	dbStatsDays = func(_ context.Context, _, subject, _ string) ([]db.StatsValue, error) {
		if used[subject] == nil {
			return nil, nil
		}
		return []db.StatsValue{{Metric: "storage.sessions", Dims: used[subject], When: "2026-11-11", Value: 1}}, nil
	}
	dbStatsMonths = func(context.Context, string, string, string) ([]db.StatsValue, error) { return nil, nil }
	dbStatsGaps = func(context.Context, string, string) ([]db.StatsGapRow, error) {
		var out []db.StatsGapRow
		for _, dims := range []map[string]string{{}, {"share": "makers"}, {"share": "makers", "protocol": "nfs"}, {"share": "makers", "room": ""}} {
			out = append(out, db.StatsGapRow{Metric: "storage.sessions", StatsGap: db.StatsGap{Dims: dims, Start: "2026-11-08", End: "2026-11-10", Reason: "Auditing was off"}})
		}
		return out, nil
	}
	now := time.Date(2026, 11, 12, 6, 0, 0, 0, time.UTC)
	for subject, want := range map[string]string{"edge": `{} {"share":"makers"}`, "lita": `{}`, "orton": `{}`} {
		usage, err := statsUsageFor(context.Background(), "truenas", subject, false, now)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, g := range sessionsOf(t, usage).Gaps {
			got = append(got, dimsText(g.Dimensions))
		}
		if strings.Join(got, " ") != want {
			t.Errorf("%q sees gaps on %v, want %q", subject, got, want)
		}
	}
}

// The two windows are makerspace days (ruling O1): the four instants around
// India's midnight and UTC's each fall on their own Indian day.
func TestStatsWindowsAreMakerspaceDays(t *testing.T) {
	old := clock.Loc
	clock.Loc, _ = time.LoadLocation("Asia/Kolkata")
	t.Cleanup(func() { clock.Loc = old })
	for _, tc := range []struct {
		at                    time.Time
		today, from30, from12 string
	}{
		{time.Date(2026, 11, 30, 18, 29, 59, 0, time.UTC), "2026-11-30", "2026-11-01", "2025-12-01"},
		{time.Date(2026, 11, 30, 18, 30, 0, 0, time.UTC), "2026-12-01", "2026-11-02", "2026-01-01"},
		{time.Date(2026, 11, 30, 23, 59, 59, 0, time.UTC), "2026-12-01", "2026-11-02", "2026-01-01"},
		{time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC), "2026-12-01", "2026-11-02", "2026-01-01"},
	} {
		today, from30, from12 := statsWindows(tc.at)
		if today != tc.today || from30 != tc.from30 || from12 != tc.from12 {
			t.Errorf("at %s: %s %s %s, want %s %s %s", tc.at, today, from30, from12, tc.today, tc.from30, tc.from12)
		}
	}
}

// Insights, an admin's read, carry each per-person series' spread with no
// floor (one member still has a spread) and the people in it: their user id,
// the directory's name, and their value over each window as the read gave it,
// a counter's missing 30 days a 0 and a gauge's a null. An unreadable
// directory leaves the name "", never the id. A metric not reported per person
// is not among them.
func TestInsightsStatsNameThePeopleForAdmins(t *testing.T) {
	stubStats(t, &statsStore{})
	prevUsers := directoryUsers
	t.Cleanup(func() { directoryUsers = prevUsers })
	directoryUsers = func(context.Context) ([]models.UserProfile, error) {
		return []models.UserProfile{{ID: "sub-cena", Name: " John Cena "}}, nil
	}
	var asked []string
	four, nine := 4.0, 9.0
	dbStatsDistributions = func(_ context.Context, today, from30, from12m string) ([]db.StatsDistribution, error) {
		asked = []string{today, from30, from12m}
		return []db.StatsDistribution{
			{Target: "truenas", Metric: "storage.sessions", Dims: map[string]string{"share": "makers"}, From: "2026-11-03", From12m: "2026-01-01", Members: 1, P50: 4, P90: 4,
				People: []db.StatsPersonValue{{Subject: "sub-cena", Last30d: &four, Last12m: 40}, {Subject: "sub-lita", Last12m: 7}}},
			{Target: "truenas", Metric: "storage.home_bytes", Dims: map[string]string{}, From: "2026-11-03", From12m: "2026-01-01", Members: 1, P50: 9, P90: 9,
				People: []db.StatsPersonValue{{Subject: "sub-cena", Last30d: &nine, Last12m: 9}, {Subject: "sub-lita", Last12m: 3}}},
			{Target: "truenas", Metric: "storage.total_bytes", Dims: map[string]string{}, Members: 1, P50: 1, P90: 1, People: []db.StatsPersonValue{{Subject: "sub-cena", Last12m: 1}}},
		}, nil
	}
	read := func() []insightsStatsMetric {
		t.Helper()
		rr := httptest.NewRecorder()
		handleInsightsStats(rr, httptest.NewRequest(http.MethodGet, "/api/v1/insights/stats", nil))
		var body struct {
			Metrics []insightsStatsMetric `json:"metrics"`
		}
		if rr.Code != http.StatusOK || json.Unmarshal(rr.Body.Bytes(), &body) != nil || len(body.Metrics) != 2 {
			t.Fatalf("%d %s", rr.Code, rr.Body.String())
		}
		return body.Metrics
	}
	metrics := read()
	home, sessions := metrics[0], metrics[1]
	// The windows' starts are the read's own clamp, carried through, never recomputed.
	if sessions.Name != "storage.sessions" || sessions.Window30dFrom != "2026-11-03" || sessions.Window12mFrom != "2026-01-01" {
		t.Errorf("sessions %+v", sessions)
	}
	row := sessions.Series[0]
	if row.Members != 1 || row.P50 != 4 || len(row.People) != 2 {
		t.Fatalf("one member is still a spread, with its people: %+v", row)
	}
	cena, lita := row.People[0], row.People[1]
	if cena.UserID != "sub-cena" || cena.Name != "John Cena" || *cena.Last30d != 4 || cena.Last12m != 40 {
		t.Errorf("cena %+v", cena)
	}
	if lita.UserID != "sub-lita" || lita.Name != "" || lita.Last30d == nil || *lita.Last30d != 0 || lita.Last12m != 7 {
		t.Errorf("a counter with no day in 30 counted 0 there, and a name nobody can read is \"\": %+v", lita)
	}
	if g := home.Series[0].People[1]; g.Last30d != nil || g.Last12m != 3 {
		t.Errorf("a gauge with no sample in 30 has no value there: %+v", g)
	}
	if today, from30, from12m := statsWindows(clock.Now()); len(asked) != 3 || asked[0] != today || asked[1] != from30 || asked[2] != from12m {
		t.Errorf("distributions read with %v, want today %s and the windows' %s, %s", asked, today, from30, from12m)
	}

	directoryUsers = func(context.Context) ([]models.UserProfile, error) { return nil, errors.New("down") }
	if p := read()[1].Series[0].People[0]; p.Name != "" || p.UserID != "sub-cena" {
		t.Errorf("an unreadable directory: %+v, want no name and the id only as the id", p)
	}
}

// What names someone (named_subjects, set at collection) is an admin's to see:
// a member's own page leaves out a series with any point naming anyone but
// them and says how many, and gives such a gap Syndra's sentence for its
// reason, on its own line; an admin on
// that person's page, and the totals, see every value and reason as stored.
func TestAMemberNeverSeesWhatNamesAPerson(t *testing.T) {
	stubStats(t, &statsStore{})
	dbStatsDays = func(context.Context, string, string, string) ([]db.StatsValue, error) {
		return []db.StatsValue{
			{Metric: "storage.sessions", Dims: map[string]string{"share": "makers"}, When: "2026-11-10", Value: 2},
			{Metric: "storage.sessions", Dims: map[string]string{"share": "home-john_cena"}, When: "2026-11-10", Value: 5, NamedSubjects: []string{"sub-cena"}},
			{Metric: "storage.sessions", Dims: map[string]string{"share": "home-john_cena"}, When: "2026-11-11", Value: 1},
		}, nil
	}
	dbStatsMonths = func(context.Context, string, string, string) ([]db.StatsValue, error) {
		return []db.StatsValue{
			{Metric: "storage.sessions", Dims: map[string]string{"share": "makers"}, When: "2026-11", Value: 2},
			// Flagged in a month only: the 12 months are the member's page too.
			{Metric: "storage.sessions", Dims: map[string]string{"share": "lita-scratch"}, When: "2026-09", Value: 4, NamedSubjects: []string{"sub-lita", "u1"}},
		}, nil
	}
	dbStatsGaps = func(context.Context, string, string) ([]db.StatsGapRow, error) {
		return []db.StatsGapRow{
			{Metric: "storage.sessions", StatsGap: db.StatsGap{Dims: map[string]string{}, Start: "2026-11-08", End: "2026-11-09", Reason: "John Cena's home is offline", NamedSubjects: []string{"sub-cena"}}},
			{Metric: "storage.sessions", StatsGap: db.StatsGap{Dims: map[string]string{"share": "home-john_cena"}, Start: "2026-11-05", End: "2026-11-06", Reason: "Auditing was off", NamedSubjects: []string{"sub-cena"}}},
			// Names someone else by its reason, on a line the member keeps:
			// the window and its dimensions stay (they are the line's own),
			// the reason does not.
			{Metric: "storage.sessions", StatsGap: db.StatsGap{Dims: map[string]string{"share": "makers"}, Start: "2026-11-03", End: "2026-11-04", Reason: "Auditing was off", NamedSubjects: []string{"sub-makers"}}},
			{Metric: "storage.sessions", StatsGap: db.StatsGap{Dims: map[string]string{"share": "makers"}, Start: "2026-11-01", End: "2026-11-02", Reason: "Auditing was off"}},
		}, nil
	}
	now := time.Date(2026, 11, 12, 6, 0, 0, 0, time.UTC)
	names := []string{"john_cena", "John Cena", "lita-scratch"}

	member, err := statsUsageFor(context.Background(), "truenas", "u1", true, now)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(member)
	for _, n := range names {
		if strings.Contains(string(raw), n) {
			t.Errorf("the member's page carries %q: %s", n, raw)
		}
	}
	sessions := sessionsOf(t, member)
	if len(sessions.Series) != 1 || sessions.Series[0].Dimensions["share"] != "makers" || sessions.LeftOut != 2 {
		t.Errorf("the member sees %+v, left out %d: want makers only and 2 series said to be left out", sessions.Series, sessions.LeftOut)
	}
	var gaps []string
	for _, g := range sessions.Gaps {
		gaps = append(gaps, g.Start+" "+dimsText(g.Dimensions)+" "+g.Reason)
	}
	if want := []string{
		"2026-11-08 {} " + reasonWithheld,                 // the whole metric's, its reason withheld
		`2026-11-03 {"share":"makers"} ` + reasonWithheld, // names someone else: on its own line, reason withheld
		`2026-11-01 {"share":"makers"} Auditing was off`,  // as stored
	}; !slices.Equal(gaps, want) {
		t.Errorf("the member's gaps %q, want %q (none on a series left out)", gaps, want)
	}

	for _, subject := range []string{"u1", ""} { // an admin on u1's page, and the totals
		admin, err := statsUsageFor(context.Background(), "truenas", subject, false, now)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(admin)
		for _, n := range names {
			if !strings.Contains(string(raw), n) {
				t.Errorf("an admin's page (subject %q) lacks %q: %s", subject, n, raw)
			}
		}
		if s := sessionsOf(t, admin); s.LeftOut != 0 || len(s.Series) != 3 {
			t.Errorf("an admin's page (subject %q): %d series, left out %d", subject, len(s.Series), s.LeftOut)
		}
	}
}

// A member's own name hides nothing of theirs: a share named after its owner
// is on the owner's page as stored, and left out of anyone else's; a reason
// naming only them is theirs to read. An admin sees it on every page.
func TestAMembersOwnNameHidesNothingOfTheirs(t *testing.T) {
	stubStats(t, &statsStore{})
	dbStatsDays = func(context.Context, string, string, string) ([]db.StatsValue, error) {
		return []db.StatsValue{{Metric: "storage.sessions", Dims: map[string]string{"share": "home-john_cena"}, When: "2026-11-10", Value: 5, NamedSubjects: []string{"sub-cena"}}}, nil
	}
	dbStatsMonths = func(context.Context, string, string, string) ([]db.StatsValue, error) { return nil, nil }
	dbStatsGaps = func(context.Context, string, string) ([]db.StatsGapRow, error) {
		return []db.StatsGapRow{{Metric: "storage.sessions", StatsGap: db.StatsGap{Dims: map[string]string{}, Start: "2026-11-08", End: "2026-11-09", Reason: "john_cena's home is offline", NamedSubjects: []string{"sub-cena"}}}}, nil
	}
	now := time.Date(2026, 11, 12, 6, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		subject string
		member  bool
		shown   bool
	}{{"sub-cena", true, true}, {"sub-lita", true, false}, {"sub-lita", false, true}} {
		usage, err := statsUsageFor(context.Background(), "truenas", c.subject, c.member, now)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(usage)
		s := sessionsOf(t, usage)
		if got := strings.Contains(string(raw), "john_cena"); got != c.shown || (len(s.Series) == 1) != c.shown || (s.LeftOut == 1) == c.shown {
			t.Errorf("%s (member view %t): shown %t, series %d, left out %d; want shown %t: %s", c.subject, c.member, got, len(s.Series), s.LeftOut, c.shown, raw)
		}
	}
}

// Through the real handlers: /me/targets is the member's view, masked;
// /users/{id}/targets the admin's, as stored.
func TestOnlyTheMembersOwnPageIsMasked(t *testing.T) {
	h := &personStorageHarness{storageHarness: storageHarness{entitled: true, bound: true}}
	stubPersonStorage(t, h)
	manifest := addonsAcceptedShape
	t.Cleanup(func() { addonsAcceptedShape = manifest })
	addonsAcceptedShape = func(context.Context, string) (addons.Shape, error) {
		return addons.Shape{Kind: "storage", EntitlementSchema: truenasSchema}, nil
	}
	stubStats(t, &statsStore{})
	dbStatsGaps = func(context.Context, string, string) ([]db.StatsGapRow, error) {
		return []db.StatsGapRow{{Metric: "storage.sessions", StatsGap: db.StatsGap{Dims: map[string]string{}, Start: "2026-11-08", End: "2026-11-10", Reason: "rey_mysterio's share is offline", NamedSubjects: []string{"sub-rey"}}}}, nil
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/targets", nil)
	req = req.WithContext(withPrincipal(req.Context(), &auth.Principal{Subject: "u1"}))
	rr := httptest.NewRecorder()
	handleMyTargets(rr, req)
	if g := sessionsOf(t, myUsage(t, rr)).Gaps; len(g) != 1 || g[0].Reason != reasonWithheld || strings.Contains(rr.Body.String(), "rey_mysterio") {
		t.Errorf("the member's own page: %s", rr.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/api/v1/users/u1/targets", nil)
	req.SetPathValue("id", "u1")
	req = req.WithContext(withPrincipal(req.Context(), &auth.Principal{Subject: "op-1"}))
	rr = httptest.NewRecorder()
	handleUserTargets(rr, req)
	if g := sessionsOf(t, myUsage(t, rr)).Gaps; len(g) != 1 || g[0].Reason != "rey_mysterio's share is offline" {
		t.Errorf("the admin's person page: %s", rr.Body.String())
	}
}

// The one write of per-person retention: 1 to 400 days or cleared; anything
// else 422 with the schema's sentence; strict JSON; a known target. Days is
// required: {} is a 400 that writes nothing, never a silent clear; only
// {"days": null} clears.
func TestStatsRetentionIsSetOnlyWithinTheCeiling(t *testing.T) {
	stubStats(t, &statsStore{})
	prev, get := dbSetStatsRetention, addonsGet
	t.Cleanup(func() { dbSetStatsRetention, addonsGet = prev, get })
	addonsGet = func(target string) (*addons.Addon, error) {
		if target != "truenas" {
			return nil, addons.ErrNotRegistered
		}
		return &addons.Addon{}, nil
	}
	var wrote []*int
	dbSetStatsRetention = func(_ context.Context, _ string, days *int, _ string) error {
		wrote = append(wrote, days)
		if days != nil && (*days < 1 || *days > 400) {
			return db.ErrStatsRetentionInvalid
		}
		return nil
	}
	send := func(target, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPut, "/api/v1/targets/"+target+"/stats-retention", strings.NewReader(body))
		req.SetPathValue("target", target)
		rr := httptest.NewRecorder()
		handleSetStatsRetention(rr, req)
		return rr
	}
	put := func(target, body string) int { return send(target, body).Code }
	for body, want := range map[string]int{
		`{"days":90}`: http.StatusOK, `{"days":null}`: http.StatusOK, `{"days":401}`: http.StatusUnprocessableEntity,
		`{"days":0}`: http.StatusUnprocessableEntity, `{"days":90,"totals":1}`: http.StatusBadRequest, `{"days":"90"}`: http.StatusBadRequest,
		`{}`: http.StatusBadRequest, `{"totals":1}`: http.StatusBadRequest,
	} {
		if got := put("truenas", body); got != want {
			t.Errorf("%s: %d, want %d", body, got, want)
		}
	}
	if got := put("nowhere", `{"days":90}`); got != http.StatusNotFound {
		t.Errorf("an unknown target: %d", got)
	}
	// 90, null, 401 and 0 reached the store; nothing else did.
	if len(wrote) != 4 {
		t.Errorf("%d writes, want 4: a refused body was written", len(wrote))
	}
	var problem ErrorResponse
	if rr := send("truenas", `{}`); json.Unmarshal(rr.Body.Bytes(), &problem) != nil || problem.Error != "VALIDATION_FAILED" || problem.Details["days"] != "required" {
		t.Errorf("{} answered %d %s", rr.Code, rr.Body.String())
	}
}

// Through the real router: every statistics read but a member's own, and the
// retention write, are an admin's.
func TestStatsRoutesAreAdminOnly(t *testing.T) {
	stubMemberAuth(t, "u1")
	router := NewRouter()
	for _, c := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/targets/truenas/stats"},
		{http.MethodPut, "/api/v1/targets/truenas/stats-retention"},
		{http.MethodGet, "/api/v1/insights/stats"},
		{http.MethodGet, "/api/v1/users/u2/targets"},
	} {
		req := httptest.NewRequest(c.method, c.path, strings.NewReader(`{"days":1}`))
		req.Header.Set("Authorization", "Bearer member-token")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("%s %s as a member: %d", c.method, c.path, rr.Code)
		}
	}
	src, err := os.ReadFile("router.go")
	if err != nil {
		t.Fatal(err)
	}
	route := regexp.MustCompile(`mux\.HandleFunc\("(\w+ /api/v1/\S*stats\S*)", (.*)\)\n`)
	routes := route.FindAllStringSubmatch(string(src), -1)
	if len(routes) != 3 {
		t.Fatalf("found %d statistics routes, want 3", len(routes))
	}
	for _, r := range routes {
		if !strings.HasPrefix(r[2], "withCORS(withOperatorAuth(") {
			t.Errorf("%s is not behind withOperatorAuth: %s", r[1], r[2])
		}
	}
}

// Setting a password and a member action call describeMyTarget for its state
// checks only: neither reads a statistic. That the views do is
// TestMyUsageIsTheActorsOwn.
func TestOnlyTheViewsReadUsage(t *testing.T) {
	prev := dbStatsMetrics
	t.Cleanup(func() { dbStatsMetrics = prev })
	queried := 0
	dbStatsMetrics = func(context.Context, string) ([]db.StatsMetric, error) { queried++; return nil, nil }

	h := stubMemberAction(t)
	h.result = addonop.Result{OperationID: "op_1", Outcome: addons.OutcomeSucceeded}
	if rr, _ := setCredential(t, `{"password":"`+memberSecret+`"}`); rr.Code != http.StatusOK || len(h.confirms) != 1 {
		t.Fatalf("the password was not set: %d %s", rr.Code, rr.Body.String())
	}
	if rr, out := act(t, "item.checkout", previewBody); rr.Code != http.StatusOK || out["state"] != "preview" {
		t.Fatalf("the action was not previewed: %d %s", rr.Code, rr.Body.String())
	}
	if queried != 0 {
		t.Errorf("the password and action paths made %d statistics queries", queried)
	}

}
