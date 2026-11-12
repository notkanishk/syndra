package addons

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"syndra/internal/clock"
)

// Feature stats at registration and on the wire (addon-contract 6.6).

// forgetMetrics clears the stored descriptors and the said-once lines.
func forgetMetrics(t *testing.T) {
	t.Helper()
	clear := func() {
		memMetrics.Lock()
		memMetrics.byTarget, memMetrics.saves = map[string]map[string]json.RawMessage{}, 0
		memMetrics.Unlock()
		statsSaid.Clear()
	}
	clear()
	t.Cleanup(clear)
}

// statsManifest is the TrueNAS fixture manifest (three metrics, feature
// stats), counted in this process's zone.
func statsManifest(t *testing.T) Manifest {
	t.Helper()
	m, err := decodeManifest(contractFixture(t, "manifest_truenas.json"))
	if err != nil {
		t.Fatal(err)
	}
	m.StatsZone = clock.Name
	return m
}

// logged captures the log for the rest of the test.
func logged(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	saved := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(saved) })
	return &buf
}

func metricNames(ms []Metric) []string {
	var out []string
	for _, m := range ms {
		out = append(out, m.Name)
	}
	return out
}

// The closed lists and Syndra's caps are the registry's and the schema's.
func TestStatsBoundsAreTheRegistrys(t *testing.T) {
	var units struct {
		Units map[string]any `json:"units"`
	}
	if err := json.Unmarshal(mustRead(t, "registry", "units.json"), &units); err != nil {
		t.Fatal(err)
	}
	if got := slices.Sorted(maps.Keys(units.Units)); !slices.Equal(got, slices.Sorted(slices.Values(knownUnits))) {
		t.Errorf("units: registry %v, backend %v", got, knownUnits)
	}
	var consts struct {
		Constants map[string]struct {
			Value any `json:"value"`
		} `json:"constants"`
	}
	if err := json.Unmarshal(mustRead(t, "registry", "constants.json"), &consts); err != nil {
		t.Fatal(err)
	}
	if got := consts.Constants["series_per_metric"].Value; got != float64(SeriesPerMetric) {
		t.Errorf("series_per_metric: registry %v, backend %d", got, SeriesPerMetric)
	}
	var schema struct {
		Properties map[string]struct {
			MaxItems int `json:"maxItems"`
		} `json:"properties"`
		Defs map[string]struct {
			Properties map[string]struct {
				MaxItems int      `json:"maxItems"`
				Enum     []string `json:"enum"`
			} `json:"properties"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(mustRead(t, "schema", "manifest.json"), &schema); err != nil {
		t.Fatal(err)
	}
	if got := schema.Properties["metrics"].MaxItems; got != statsMaxMetrics {
		t.Errorf("metrics maxItems: schema %d, backend %d", got, statsMaxMetrics)
	}
	metric := schema.Defs["Metric"].Properties
	if got := metric["dimensions"].MaxItems; got != statsMaxDimensions {
		t.Errorf("dimensions maxItems: schema %d, backend %d", got, statsMaxDimensions)
	}
	if got := metric["instrument"].Enum; !slices.Equal(got, []string{InstrumentCounter, InstrumentGauge}) {
		t.Errorf("instruments: schema %v", got)
	}
	for iso, want := range map[string]time.Duration{
		"P90D": 90 * 24 * time.Hour, "P30D": 30 * 24 * time.Hour, "": statsBufferDefault, "P1Y": statsBufferDefault,
		"P400D": statsBufferCeiling, "P999D": statsBufferCeiling,
	} {
		if got := (Manifest{Limits: Limits{StatsBuffer: iso}}).statsBuffer(); got != want {
			t.Errorf("stats_buffer %q reads %v, want %v", iso, got, want)
		}
	}
}

// The TrueNAS manifest's three metrics are accepted and their meanings
// stored; a refresh that changes nothing writes nothing; and a manifest that
// re-purposes a stored metric (another unit, another instrument) is refused
// with its reason and the last good one keeps serving.
func TestAMetricIsNeverRepurposed(t *testing.T) {
	registerTrueNAS(t)
	forgetRoles(t)
	forgetMetrics(t)
	m := statsManifest(t)
	if err := refreshWith(t, m); err != nil {
		t.Fatal(err)
	}
	metrics, buffer := StatsMetrics("truenas")
	if got := metricNames(metrics); !slices.Equal(got, []string{"storage.sessions", "storage.home_bytes", "storage.used_bytes"}) {
		t.Fatalf("accepted %v", got)
	}
	if buffer != 90*24*time.Hour {
		t.Errorf("buffer %v, want the declared P90D", buffer)
	}
	if n := len(memMetrics.byTarget["truenas"]); n != 3 || memMetrics.saves != 1 {
		t.Fatalf("stored %d descriptors in %d writes", n, memMetrics.saves)
	}
	if err := refreshWith(t, m); err != nil || memMetrics.saves != 1 {
		t.Fatalf("an unchanged manifest wrote its descriptors again (%d writes): %v", memMetrics.saves, err)
	}

	for _, change := range []func(*Metric){
		func(d *Metric) { d.Unit = "items" },
		func(d *Metric) { d.Instrument = InstrumentGauge },
	} {
		moved := statsManifest(t)
		moved.Metrics = slices.Clone(moved.Metrics)
		change(&moved.Metrics[0])
		err := refreshWith(t, moved)
		var refused *ErrManifestRefused
		if !errors.As(err, &refused) || !strings.Contains(refused.Reason, `"storage.sessions" was declared a counter in count`) {
			t.Fatalf("a re-purposed metric must refuse the manifest with its reason, got %v", err)
		}
		if got, _ := StatsMetrics("truenas"); len(got) != 3 || got[0].Unit != "count" || got[0].Instrument != InstrumentCounter {
			t.Errorf("the refused manifest changed the metrics in use: %+v", got)
		}
	}
}

// A descriptor that breaks a rule is dropped alone, and said once however
// often the manifest is read.
func TestABadDescriptorIsDroppedAloneAndSaidOnce(t *testing.T) {
	registerTrueNAS(t)
	forgetRoles(t)
	m := statsManifest(t)
	m.EntitlementSchema = append(slices.Clone(m.EntitlementSchema), EntitlementField{Name: "print_seconds", Role: RoleLimit, Unit: "seconds"})
	good := Metric{Name: "storage.home_bytes", Instrument: InstrumentGauge, Unit: "bytes", Label: "Home folder used", PerSubject: true, Against: "home_quota"}
	five := []Dimension{{"a", "A"}, {"b", "B"}, {"c", "C"}, {"d", "D"}, {"e", "E"}}
	for _, tc := range []struct {
		name string
		bad  Metric
		why  string
	}{
		{"a fifth dimension", Metric{Name: "x.five", Instrument: InstrumentCounter, Unit: "count", Dimensions: five}, "5 dimensions"},
		{"four dimensions", Metric{Name: "x.four", Instrument: InstrumentCounter, Unit: "count", Dimensions: five[:4]}, ""},
		{"a dimension twice", Metric{Name: "x.twice", Instrument: InstrumentCounter, Unit: "count", Dimensions: []Dimension{{"a", "A"}, {"a", "B"}}}, "declared twice"},
		{"an unnamed dimension", Metric{Name: "x.unnamed", Instrument: InstrumentCounter, Unit: "count", Dimensions: []Dimension{{"", "A"}}}, "unnamed"},
		{"a unit outside the registry", Metric{Name: "x.minutes", Instrument: InstrumentCounter, Unit: "minutes"}, `unit "minutes"`},
		{"a histogram", Metric{Name: "x.hist", Instrument: "histogram", Unit: "count"}, `"histogram" is neither`},
		{"no name", Metric{Instrument: InstrumentCounter, Unit: "count"}, "no name"},
		{"a name twice", good, "declared twice"},
		{"against another unit", Metric{Name: "x.other_unit", Instrument: InstrumentGauge, Unit: "bytes", PerSubject: true, Against: "print_seconds"}, `"print_seconds", which is not a limit field in bytes`},
		{"against no limit", Metric{Name: "x.not_limit", Instrument: InstrumentGauge, Unit: "bytes", PerSubject: true, Against: "enabled"}, "not a limit field"},
		{"against on a counter", Metric{Name: "x.counter", Instrument: InstrumentCounter, Unit: "bytes", PerSubject: true, Against: "home_quota"}, "per-person gauge"},
		{"against on a total", Metric{Name: "x.total", Instrument: InstrumentGauge, Unit: "bytes", Against: "home_quota"}, "per-person gauge"},
		{"against with a dimension", Metric{Name: "x.dims", Instrument: InstrumentGauge, Unit: "bytes", PerSubject: true, Against: "home_quota", Dimensions: []Dimension{{"share", "Share"}}}, "per-person gauge"},
		{"against the same unit", Metric{Name: "x.same_unit", Instrument: InstrumentGauge, Unit: "seconds", PerSubject: true, Against: "print_seconds"}, ""},
	} {
		forgetMetrics(t)
		out := logged(t)
		with := m
		with.Metrics = append(slices.Clone(m.Metrics), tc.bad)
		for range 2 {
			if err := refreshWith(t, with); err != nil {
				t.Fatalf("%s: a bad descriptor refused the whole manifest: %v", tc.name, err)
			}
		}
		got, _ := StatsMetrics("truenas")
		kept := len(got) == 4
		if kept != (tc.why == "") {
			t.Errorf("%s: kept %v", tc.name, metricNames(got))
		}
		if tc.why != "" && (strings.Count(out.String(), "is not collected") != 1 || !strings.Contains(out.String(), tc.why)) {
			t.Errorf("%s: said %q, want once with %q", tc.name, out.String(), tc.why)
		}
	}

	forgetMetrics(t)
	many := m
	many.Metrics = nil
	for i := range statsMaxMetrics + 1 {
		many.Metrics = append(many.Metrics, Metric{Name: "x.m" + string(rune('a'+i/26)) + string(rune('a'+i%26)), Instrument: InstrumentCounter, Unit: "count"})
	}
	if err := refreshWith(t, many); err != nil {
		t.Fatal(err)
	}
	if got, _ := StatsMetrics("truenas"); len(got) != statsMaxMetrics {
		t.Errorf("kept %d of %d metrics, want the first %d", len(got), len(many.Metrics), statsMaxMetrics)
	}
}

// Statistics counted in another zone are another day's: the feature is
// dropped (nothing collected, nothing stored), said once, never rebucketed.
func TestStatsCountedInAnotherZoneAreNotCollected(t *testing.T) {
	registerTrueNAS(t)
	forgetRoles(t)
	forgetMetrics(t)
	out := logged(t)
	m := statsManifest(t)
	m.StatsZone = "Pacific/Kiritimati"
	for range 3 {
		if err := refreshWith(t, m); err != nil {
			t.Fatalf("another zone refused the manifest: %v", err)
		}
	}
	if got, _ := StatsMetrics("truenas"); len(got) != 0 || memMetrics.saves != 0 {
		t.Errorf("collected %v and stored %d in another zone", metricNames(got), memMetrics.saves)
	}
	if n := strings.Count(out.String(), `counts days in "Pacific/Kiritimati"`); n != 1 {
		t.Errorf("said %d times: %q", n, out.String())
	}
}

// GET /stats is a read through the pipe: one metric, since an instant in the
// makerspace's zone, the cursor when there is one, and never fresh, even to
// an add-on declaring freshness. A reply about another metric is not one.
func TestReadStatsAsksForOneMetricAndNeverFresh(t *testing.T) {
	got := addonAnswering(t, "stats.json")
	declare(t, "stats")
	declare(t, "freshness")
	since := time.Date(2026, 11, 6, 18, 30, 0, 0, time.UTC)
	page, err := ReadStats(context.Background(), "truenas", "laser.job_seconds", since, "abc")
	if err != nil {
		t.Fatal(err)
	}
	if got.path != "/stats" || strings.Contains(got.rawQuery, "fresh") {
		t.Errorf("sent %s?%s", got.path, got.rawQuery)
	}
	wantSince := since.In(clock.Loc).Format(time.RFC3339)
	if q := got.rawQuery; !strings.Contains(q, "metric=laser.job_seconds") || !strings.Contains(q, "cursor=abc") ||
		!strings.Contains(q, "since="+strings.NewReplacer(":", "%3A", "+", "%2B").Replace(wantSince)) {
		t.Errorf("query %q, want metric, cursor and since %s", q, wantSince)
	}
	if len(page.Points) != 2 || page.Unbound != 2 || len(page.Gaps) != 1 || page.NextCursor == "" || page.Watermark.IsZero() {
		t.Errorf("page %+v", page)
	}
	if _, err := ReadStats(context.Background(), "truenas", "storage.sessions", since, ""); !errors.Is(err, ErrStatsReply) {
		t.Errorf("a reply about another metric read as %v", err)
	}
}

// A coded /stats answer is the add-on answering on purpose about its own
// data: five metrics answering 503 NO_CURRENT_READ leave the breaker shut.
// /stats only — the same answer on /subjects still counts — and a transport
// failure on /stats still counts, and the open breaker is still consulted
// before the next /stats is sent.
func TestACodedStatsAnswerIsNeverABreakerFailure(t *testing.T) {
	withBreaker(t, 3, breakerCooldown)
	var hits atomic.Int32
	abort := atomic.Bool{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		if abort.Load() {
			panic(http.ErrAbortHandler) // the connection drops: a transport failure
		}
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"type":"about:blank","title":"Not read yet","status":503,"code":"NO_CURRENT_READ"}`))
	}))
	t.Cleanup(srv.Close)
	signedAddon(t, srv.URL, "a-test-secret")
	declare(t, "stats")
	a, _ := Get("truenas")
	since := time.Date(2026, 11, 6, 18, 30, 0, 0, time.UTC)

	for _, metric := range []string{"a.one", "a.two", "a.three", "a.four", "a.five"} {
		if _, err := ReadStats(context.Background(), "truenas", metric, since, ""); err == nil {
			t.Fatalf("%s: a 503 read as a page", metric)
		}
	}
	if a.CircuitOpen() {
		t.Fatal("five metrics answering NO_CURRENT_READ opened the breaker")
	}

	for range breakerThreshold {
		Subjects(context.Background(), "truenas", false)
	}
	if !a.CircuitOpen() {
		t.Fatal("NO_CURRENT_READ on /subjects no longer counts")
	}

	signedAddon(t, srv.URL, "a-test-secret") // a fresh breaker
	declare(t, "stats")
	if a, _ = Get("truenas"); a.CircuitOpen() {
		t.Fatal("the new registration kept the old breaker")
	}
	abort.Store(true)
	for range breakerThreshold {
		ReadStats(context.Background(), "truenas", "a.one", since, "")
	}
	if !a.CircuitOpen() {
		t.Fatal("transport failures on /stats did not open the breaker")
	}
	before := hits.Load()
	if _, err := ReadStats(context.Background(), "truenas", "a.one", since, ""); !errors.Is(err, ErrCircuitOpen) || hits.Load() != before {
		t.Errorf("an open breaker let /stats through: %v, %d hits", err, hits.Load()-before)
	}
}

// A coded /stats answer does not clear the breaker either, even a 4xx one: a
// broken add-on answering /stats with a code between its 5xx would otherwise
// keep the breaker shut for ever. The answer pauses the count.
func TestACodedStatsAnswerDoesNotClearTheBreaker(t *testing.T) {
	b := &breaker{}
	now := time.Unix(1_700_000_000, 0)
	for range breakerThreshold - 1 {
		b.record(now, CallResponse{Outcome: OutcomeIndeterminate, Status: 500})
		b.record(now, CallResponse{Outcome: OutcomeRejected, Status: 400, Code: "WINDOW_OUT_OF_RANGE", StatsRefusal: true})
	}
	b.record(now, CallResponse{Outcome: OutcomeIndeterminate, Status: 500})
	if b.allow(now) {
		t.Fatal("coded /stats answers reset the count a failing add-on is accumulating")
	}
}
