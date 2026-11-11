package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"syndra/internal/addons"
	"syndra/internal/auth"
	"syndra/internal/clock"
	"syndra/internal/db"
)

// Statistics, read (addon-contract 6.8): a member their own, an admin the
// rest, every day a makerspace day.

// statsStore is Syndra's statistics for one target, by subject ("" totals),
// and every subject a read asked for.
type statsStore struct {
	asked []string
}

func stubStats(t *testing.T, s *statsStore) {
	t.Helper()
	prev := []any{dbStatsMetrics, dbStatsDays, dbStatsMonths, dbStatsGaps, dbStatsDistributions, dbStatsRetention}
	t.Cleanup(func() {
		dbStatsMetrics = prev[0].(func(context.Context, string) ([]db.StatsMetric, error))
		dbStatsDays = prev[1].(func(context.Context, string, string, string) ([]db.StatsValue, error))
		dbStatsMonths = prev[2].(func(context.Context, string, string, string) ([]db.StatsValue, error))
		dbStatsGaps = prev[3].(func(context.Context, string, string) ([]db.StatsGapRow, error))
		dbStatsDistributions = prev[4].(func(context.Context, string) ([]db.StatsDistribution, error))
		dbStatsRetention = prev[5].(func(context.Context, string) (int, bool, error))
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
	dbStatsDays = func(_ context.Context, _, subject, _ string) ([]db.StatsValue, error) {
		s.asked = append(s.asked, subject)
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
	dbStatsMonths = func(_ context.Context, _, subject, _ string) ([]db.StatsValue, error) {
		s.asked = append(s.asked, subject)
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
	manifest := addonsAcceptedManifest
	t.Cleanup(func() { addonsAcceptedManifest = manifest })
	addonsAcceptedManifest = func(string) (addons.Manifest, error) {
		return addons.Manifest{Kind: "storage", EntitlementSchema: truenasSchema}, nil
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
	usage, err := statsUsageFor(context.Background(), "truenas", "u1", now)
	if err != nil || !usage.Reported || len(usage.Metrics) != 2 {
		t.Fatalf("%+v %v", usage, err)
	}
	if usage.Metrics[0].Label != "Home folder used" || usage.Metrics[1].Label != "Storage sessions" {
		t.Errorf("metrics in order %s, %s", usage.Metrics[0].Label, usage.Metrics[1].Label)
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
	if len(sessions.Gaps) != 1 || sessions.Gaps[0].Reason != "Auditing is off on this share" || sessions.Gaps[0].End != "2026-11-10" {
		t.Errorf("gaps %+v", sessions.Gaps)
	}

	totals, _ := statsUsageFor(context.Background(), "truenas", "", now)
	if len(totals.Metrics) != 3 || sessionsOf(t, totals).Collected == nil || sessionsOf(t, totals).Collected.UnboundAccounts != 2 {
		t.Errorf("the totals carry every metric and the collection line: %+v", totals)
	}
	if none, _ := statsUsageFor(context.Background(), "unifi", "u1", now); none.Reported || none.Metrics != nil {
		t.Errorf("a target reporting nothing says so: %+v", none)
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

// Insights spread each per-person series over members and name nobody; a
// metric not reported per person is not among them.
func TestInsightsStatsNameNobody(t *testing.T) {
	stubStats(t, &statsStore{})
	dbStatsDistributions = func(context.Context, string) ([]db.StatsDistribution, error) {
		return []db.StatsDistribution{
			{Target: "truenas", Metric: "storage.sessions", Dims: map[string]string{"share": "makers"}, Members: 12, P50: 4, P90: 19},
			{Target: "truenas", Metric: "storage.total_bytes", Dims: map[string]string{}, Members: 1, P50: 1, P90: 1},
		}, nil
	}
	rr := httptest.NewRecorder()
	handleInsightsStats(rr, httptest.NewRequest(http.MethodGet, "/api/v1/insights/stats", nil))
	var body struct {
		Metrics []insightsStatsMetric `json:"metrics"`
	}
	if rr.Code != http.StatusOK || json.Unmarshal(rr.Body.Bytes(), &body) != nil || len(body.Metrics) != 1 {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
	if m := body.Metrics[0]; m.Name != "storage.sessions" || m.Series[0].Members != 12 || m.Series[0].P90 != 19 {
		t.Errorf("insights %+v", m)
	}
	if strings.Contains(rr.Body.String(), `"subject`) {
		t.Errorf("insights carry a subject: %s", rr.Body.String())
	}
}

// The one write of per-person retention: 1 to 400 days or cleared; anything
// else 422 with the schema's sentence; strict JSON; a known target.
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
	put := func(target, body string) int {
		req := httptest.NewRequest(http.MethodPut, "/api/v1/targets/"+target+"/stats-retention", strings.NewReader(body))
		req.SetPathValue("target", target)
		rr := httptest.NewRecorder()
		handleSetStatsRetention(rr, req)
		return rr.Code
	}
	for body, want := range map[string]int{
		`{"days":90}`: http.StatusOK, `{"days":null}`: http.StatusOK, `{"days":401}`: http.StatusUnprocessableEntity,
		`{"days":0}`: http.StatusUnprocessableEntity, `{"days":90,"totals":1}`: http.StatusBadRequest, `{"days":"90"}`: http.StatusBadRequest,
	} {
		if got := put("truenas", body); got != want {
			t.Errorf("%s: %d, want %d", body, got, want)
		}
	}
	if got := put("nowhere", `{"days":90}`); got != http.StatusNotFound {
		t.Errorf("an unknown target: %d", got)
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
