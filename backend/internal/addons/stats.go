package addons

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"reflect"
	"slices"
	"sync"
	"time"

	"syndra/internal/clock"
)

// Feature stats, the backend's half (design §9): what a metric may declare,
// and the one read of GET /stats. Collection, storage and retention are
// services/stats and db; this file holds what the wire and the manifest mean.

// Metric is one manifest metric descriptor (design §9.3). Stored as given in
// addon_metrics.descriptor once accepted, so its meaning holds while the
// add-on is down and every page renders from it with no product code.
type Metric struct {
	Name       string      `json:"name"`
	Instrument string      `json:"instrument"`
	Unit       string      `json:"unit"`
	Label      string      `json:"label"`
	PerSubject bool        `json:"per_subject,omitempty"`
	Dimensions []Dimension `json:"dimensions,omitempty"`
	// Against names a limit field of the same unit: the gauge is a fraction
	// of the person's observed limit, drawn as a meter. Only on a per-subject
	// gauge with no dimensions (ruling O3).
	Against string `json:"against,omitempty"`
}

// Dimension is one declared dimension of a metric: a resource, never a person.
type Dimension struct {
	Name  string `json:"name"`
	Label string `json:"label"`
}

// The instruments, units and bounds of registry/units.json,
// registry/constants.json and the manifest schema, held equal to them by test.
const (
	InstrumentCounter = "counter"
	InstrumentGauge   = "gauge"

	statsMaxMetrics    = 50
	statsMaxDimensions = 4
	// SeriesPerMetric is constants.series_per_metric: the most dimension sets
	// Syndra keeps for one metric. Syndra's cap, never the add-on's to declare.
	SeriesPerMetric = 500

	statsBufferDefault = 90 * 24 * time.Hour  // limits.stats_buffer when undeclared
	statsBufferCeiling = 400 * 24 * time.Hour // and the most it may say
)

var knownUnits = []string{"count", "items", "bytes", "seconds", "grams"}

// statsBuffer is how far back a metric's first collection reaches: the
// declared limits.stats_buffer, P90D when absent or unreadable, at most P400D.
func (m Manifest) statsBuffer() time.Duration {
	d, ok := isoPositive(m.Limits.StatsBuffer)
	if !ok {
		return statsBufferDefault
	}
	return min(d, statsBufferCeiling)
}

// statsSaid is every line acceptMetrics has logged, per target, so the
// fifteen-minute refresh says each once rather than ninety-six times a day.
var statsSaid sync.Map // target + "\x00" + line -> struct{}

func sayOnce(target, line string) {
	if _, said := statsSaid.LoadOrStore(target+"\x00"+line, struct{}{}); !said {
		log.Printf("[STATS] %s: %s", target, line)
	}
}

// acceptMetrics is the metrics Syndra will collect from a manifest, checked
// against the meanings already stored (addon_metrics, never deleted):
//
//   - no feature stats, nothing;
//   - a stats_zone other than Syndra's one zone drops the whole feature, said
//     once: a day counted in another zone is another day, and Syndra never
//     rebuckets (ruling O1);
//   - a stored metric declared with another instrument or unit refuses the
//     manifest: a name never changes meaning, a new meaning is a new name;
//   - a descriptor that breaks a rule (the 51st, a name twice, an unknown
//     instrument or unit, more than four dimensions or one twice or unnamed,
//     an `against` that is not a same-unit limit of a per-subject gauge with
//     no dimensions) drops that metric alone, said once.
func acceptMetrics(ctx context.Context, target string, m Manifest) ([]Metric, error) {
	if !m.Declares("stats") {
		return nil, nil
	}
	if m.StatsZone != clock.Name {
		sayOnce(target, fmt.Sprintf("the add-on counts days in %q and Syndra in %q, so its statistics are not collected", m.StatsZone, clock.Name))
		return nil, nil
	}
	stored, err := dbStatsDescriptors(ctx, target)
	if err != nil {
		return nil, fmt.Errorf("the stored metrics could not be read, so a change of meaning could not be ruled out: %w", err)
	}
	for _, d := range m.Metrics {
		raw, known := stored[d.Name]
		if !known {
			continue
		}
		var was Metric
		if err := json.Unmarshal(raw, &was); err != nil {
			return nil, fmt.Errorf("metric %q's stored meaning could not be read: %w", d.Name, err)
		}
		if was.Instrument != d.Instrument || was.Unit != d.Unit {
			return nil, fmt.Errorf("metric %q was declared a %s in %s and is now a %s in %s; a metric is never re-purposed, a new meaning is a new name",
				d.Name, was.Instrument, was.Unit, d.Instrument, d.Unit)
		}
	}
	var out []Metric
	seen := map[string]bool{}
	for i, d := range m.Metrics {
		if why := metricProblem(m, d, i, seen); why != "" {
			sayOnce(target, fmt.Sprintf("metric %q is not collected: %s", d.Name, why))
			continue
		}
		seen[d.Name] = true
		out = append(out, d)
	}
	return out, nil
}

// metricProblem is why one descriptor is not collected, "" when it is.
func metricProblem(m Manifest, d Metric, i int, seen map[string]bool) string {
	switch {
	case i >= statsMaxMetrics:
		return fmt.Sprintf("a manifest declares at most %d metrics", statsMaxMetrics)
	case d.Name == "":
		return "it has no name"
	case seen[d.Name]:
		return "it is declared twice"
	case d.Instrument != InstrumentCounter && d.Instrument != InstrumentGauge:
		return fmt.Sprintf("instrument %q is neither counter nor gauge", d.Instrument)
	case !slices.Contains(knownUnits, d.Unit):
		return fmt.Sprintf("unit %q is not in the registry", d.Unit)
	case len(d.Dimensions) > statsMaxDimensions:
		return fmt.Sprintf("it declares %d dimensions; at most %d", len(d.Dimensions), statsMaxDimensions)
	}
	dims := map[string]bool{}
	for _, dim := range d.Dimensions {
		if dim.Name == "" || dims[dim.Name] {
			return "a dimension is unnamed or declared twice"
		}
		dims[dim.Name] = true
	}
	if d.Against == "" {
		return ""
	}
	if d.Instrument != InstrumentGauge || !d.PerSubject || len(d.Dimensions) > 0 {
		return "against is only for a per-person gauge with no dimensions"
	}
	if f, ok := m.EntitlementField(d.Against); !ok || f.Role != RoleLimit || f.Unit != d.Unit {
		return fmt.Sprintf("against names %q, which is not a limit field in %s", d.Against, d.Unit)
	}
	return ""
}

// saveMetrics stores each accepted descriptor whose stored form differs (a
// new metric, a new label): the meaning every page and the collector read.
func saveMetrics(ctx context.Context, target string, metrics []Metric) error {
	if len(metrics) == 0 {
		return nil
	}
	stored, err := dbStatsDescriptors(ctx, target)
	if err != nil {
		return err
	}
	changed := map[string]json.RawMessage{}
	for _, d := range metrics {
		var was Metric
		if raw, known := stored[d.Name]; known && json.Unmarshal(raw, &was) == nil && reflect.DeepEqual(was, d) {
			continue
		}
		raw, err := json.Marshal(d)
		if err != nil {
			return err
		}
		changed[d.Name] = raw
	}
	if len(changed) == 0 {
		return nil
	}
	return dbSaveStatsDescriptors(ctx, target, changed)
}

// StatsMetrics is what the collector reads of a target: the metrics its
// accepted manifest declares and acceptMetrics kept, and how far back a
// metric's first collection reaches (limits.stats_buffer).
func StatsMetrics(target string) ([]Metric, time.Duration) {
	a, err := Get(target)
	if err != nil {
		return nil, 0
	}
	registryMu.RLock()
	defer registryMu.RUnlock()
	if a.manifest == nil {
		return nil, 0
	}
	return slices.Clone(a.metrics), a.manifest.statsBuffer()
}

// StatPoint is one day of one series (design §9.3): the complete total for the
// day [Start, End), or a gauge's last sample in it. No Subject is the total.
type StatPoint struct {
	Start      time.Time         `json:"start"`
	End        time.Time         `json:"end"`
	Subject    string            `json:"subject,omitempty"`
	Dimensions map[string]string `json:"dimensions,omitempty"`
	// Value is nil when the add-on sent none, which is not a zero.
	Value *float64 `json:"value"`
}

// StatGap is a window the add-on could not observe, with its reason.
type StatGap struct {
	Start      time.Time         `json:"start"`
	End        time.Time         `json:"end"`
	Dimensions map[string]string `json:"dimensions,omitempty"`
	Reason     string            `json:"reason"`
}

// StatsPage is one page of a GET /stats reply.
type StatsPage struct {
	TakenAt   time.Time
	Watermark time.Time
	Points    []StatPoint
	Gaps      []StatGap
	Unbound   int
	// NextCursor is "" on the last page.
	NextCursor string
}

type statsWire struct {
	ContractVersion int         `json:"contract_version"`
	Metric          string      `json:"metric"`
	TakenAt         time.Time   `json:"taken_at"`
	Watermark       time.Time   `json:"watermark"`
	Points          []StatPoint `json:"points"`
	Unbound         struct {
		Accounts int `json:"accounts"`
	} `json:"unbound"`
	Gaps       []StatGap `json:"gaps"`
	NextCursor *string   `json:"next_cursor"`
}

// ErrStatsReply is a /stats reply that is not one: another metric, another
// contract, no watermark. The collection of that metric stops and keeps its
// watermark.
var ErrStatsReply = errors.New("addon: the /stats reply is not a reply about this metric")

// ReadStats reads one page of one metric since an instant (design §9.3):
// through the pipe and the breaker like every read, never with fresh (when
// the app is read for statistics is the add-on's), and checked as far as a
// page goes. What each point says is checked by the collector.
func ReadStats(ctx context.Context, target, metric string, since time.Time, cursor string) (StatsPage, error) {
	a, err := Get(target)
	if err != nil {
		return StatsPage{}, err
	}
	q := url.Values{"metric": {metric}, "since": {since.In(clock.Loc).Format(time.RFC3339)}}
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	resp, err := a.send(ctx, sendSpec{Method: http.MethodGet, Path: "/stats?" + q.Encode()})
	if err != nil {
		return StatsPage{}, err
	}
	if resp.Outcome != OutcomeSucceeded {
		return StatsPage{}, fmt.Errorf("addon %s: /stats %s answered %d %s", target, metric, resp.Status, resp.Code)
	}
	var w statsWire
	if err := json.Unmarshal(resp.Body, &w); err != nil {
		return StatsPage{}, fmt.Errorf("%w: %v", ErrStatsReply, err)
	}
	switch {
	case w.ContractVersion != ContractVersion:
		return StatsPage{}, fmt.Errorf("%w: contract %d", ErrStatsReply, w.ContractVersion)
	case w.Metric != metric:
		return StatsPage{}, fmt.Errorf("%w: it is about %q", ErrStatsReply, w.Metric)
	case w.Watermark.IsZero() || w.TakenAt.IsZero():
		return StatsPage{}, fmt.Errorf("%w: no watermark or taken_at", ErrStatsReply)
	}
	page := StatsPage{TakenAt: w.TakenAt, Watermark: w.Watermark, Points: w.Points, Gaps: w.Gaps, Unbound: max(0, w.Unbound.Accounts)}
	if w.NextCursor != nil {
		page.NextCursor = *w.NextCursor
	}
	return page, nil
}
