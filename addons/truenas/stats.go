package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"
)

// Statistics (feature stats; guide chapter 4, "Statistics"). The collector
// (stats_collect.go) reads the NAS on its own timer into the bbolt buffer
// (stats_store.go); this answers Syndra from the buffer, never from the NAS.
//
// The model is the OpenTelemetry metrics data model's vocabulary: a metric
// with an instrument (counter: a monotonic sum; gauge: a level), a unit, and
// attributes as declared dimensions. Every bucket is one calendar day in the
// container's TZ (stats_zone), its start and end RFC 3339 instants with that
// zone's offset, and a point is the whole day: equal keys replace, never add.

// Metric is one descriptor of the manifest's metrics[].
type Metric struct {
	Name       string      `json:"name"`
	Instrument string      `json:"instrument"`
	Unit       string      `json:"unit"`
	Label      string      `json:"label"`
	PerSubject bool        `json:"per_subject"`
	Dimensions []Dimension `json:"dimensions,omitempty"`
	Against    string      `json:"against,omitempty"`
}

type Dimension struct {
	Name  string `json:"name"`
	Label string `json:"label"`
}

const (
	metricSessions  = "storage.sessions"
	metricHomeBytes = "storage.home_bytes"
	metricUsedBytes = "storage.used_bytes"
)

var shareDimension = []Dimension{{Name: "share", Label: "Share"}}

// statsMetrics is the one list the manifest declares and /stats answers for.
// The home folder is its own metric, metered against home_quota: a meter is
// one series per person, so a limit that holds in one place only is never a
// dimension of another metric.
var statsMetrics = []Metric{
	{Name: metricSessions, Instrument: "counter", Unit: "count", Label: "Storage sessions", PerSubject: true, Dimensions: shareDimension},
	{Name: metricHomeBytes, Instrument: "gauge", Unit: "bytes", Label: "Home folder used", PerSubject: true, Against: FieldHomeQuota},
	{Name: metricUsedBytes, Instrument: "gauge", Unit: "bytes", Label: "Storage used", PerSubject: true, Dimensions: shareDimension},
}

const (
	// statsBuffer is limits.stats_buffer: a day stays while its end is
	// within it.
	statsBuffer     = "P90D"
	statsBufferDays = 90
	// statsMaxReplyBytes is constants.max_reply_bytes, the largest reply
	// Syndra reads (TestStatsPagesFitTheContractsCeiling holds the two equal).
	statsMaxReplyBytes = 1 << 20

	reasonBuffer   = "The add-on no longer holds these days"
	reasonNAS      = "The NAS could not be read"
	reasonDark     = "The NAS could not be read that day"
	reasonBehind   = "Too much activity to count yet"
	reasonAudited  = "Auditing does not record everyone on this share"
	reasonPurged   = "The NAS no longer keeps its log of these days"
	reasonOverfull = "Too much activity to count"
)

func declaredMetric(name string) (Metric, bool) {
	i := slices.IndexFunc(statsMetrics, func(m Metric) bool { return m.Name == name })
	if i < 0 {
		return Metric{}, false
	}
	return statsMetrics[i], true
}

// StatPoint is one bucket; StatGap one window not observed.
type StatPoint struct {
	Start      string            `json:"start"`
	End        string            `json:"end"`
	Subject    string            `json:"subject,omitempty"`
	Dimensions map[string]string `json:"dimensions,omitempty"`
	Value      int64             `json:"value"`
}

type StatGap struct {
	Start      string            `json:"start"`
	End        string            `json:"end"`
	Dimensions map[string]string `json:"dimensions,omitempty"`
	Reason     string            `json:"reason"`
}

// statsClock is the zone days are counted in and the clock, both seams.
type statsClock struct {
	zone *time.Location // nil: UTC
	now  func() time.Time
	// pageBytes is the reply ceiling a page is cut under; 0 is
	// statsMaxReplyBytes. A seam, so a test sees several pages.
	pageBytes int
}

func (c statsClock) loc() *time.Location {
	if c.zone == nil {
		return time.UTC
	}
	return c.zone
}

func (c statsClock) clock() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

// day is the calendar day an instant falls in, as stored ("2006-01-02").
func (c statsClock) day(t time.Time) string { return t.In(c.loc()).Format(time.DateOnly) }

// midnight is a stored day's first instant in the zone: 00:00, or, on a day
// whose 00:00 a daylight-saving change skips (America/Santiago, 6 Sep 2026),
// the instant the clocks resume. Go resolves a skipped 00:00 to the previous
// day's 23:00, so that one case takes the end of the zone period it fell in.
// The day after a stored day is nextDayKey: a calendar day, never 24 h.
func (c statsClock) midnight(day string) time.Time {
	t, _ := time.ParseInLocation(time.DateOnly, day, c.loc())
	if t.Format(time.DateOnly) != day {
		_, t = t.ZoneBounds()
	}
	return t
}

func (c statsClock) stamp(day string) string { return c.midnight(day).Format(time.RFC3339) }

// statKey is a point's place in the keyset order (start, subject,
// dimensions), the total ("" subject) first.
type statKey struct{ day, subject, dims string }

func keyOf(day string, p StatPoint) statKey {
	dims := "{}"
	if len(p.Dimensions) > 0 {
		b, _ := json.Marshal(p.Dimensions) // map keys marshal sorted
		dims = string(b)
	}
	return statKey{day, p.Subject, dims}
}

func (k statKey) less(o statKey) bool {
	if k.day != o.day {
		return k.day < o.day
	}
	if k.subject != o.subject {
		return k.subject < o.subject
	}
	return k.dims < o.dims
}

func sortPoints(day string, ps []StatPoint) {
	slices.SortFunc(ps, func(a, b StatPoint) int {
		ka, kb := keyOf(day, a), keyOf(day, b)
		switch {
		case ka.less(kb):
			return -1
		case kb.less(ka):
			return 1
		}
		return 0
	})
}

// cursorOf is next_cursor: base64url (RFC 4648 §5) JSON of the last key sent.
func cursorOf(p StatPoint) string {
	raw, _ := json.Marshal(map[string]any{"s": p.Start, "u": p.Subject, "d": p.Dimensions})
	return base64.RawURLEncoding.EncodeToString(raw)
}

var errBadCursor = errors.New("not a cursor this add-on gave")

func (c statsClock) parseCursor(s string) (*statKey, error) {
	if s == "" {
		return nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, errBadCursor
	}
	var k struct {
		S string            `json:"s"`
		U string            `json:"u"`
		D map[string]string `json:"d"`
	}
	if json.Unmarshal(raw, &k) != nil {
		return nil, errBadCursor
	}
	at, err := time.Parse(time.RFC3339, k.S)
	if err != nil {
		return nil, errBadCursor
	}
	key := keyOf(c.day(at), StatPoint{Subject: k.U, Dimensions: k.D})
	return &key, nil
}

// fill is how many encoded points fit in room bytes, each with the comma
// before it.
func fill(encoded [][]byte, room int) int {
	used := 0
	for i, p := range encoded {
		if used+len(p)+1 > room {
			return i
		}
		used += len(p) + 1
	}
	return len(encoded)
}

// mergeGaps joins the days' gaps with the same dimensions and reason into
// windows, never across the watermark: a final gap keeps the end it had.
// Gaps are keyed by (dimensions, start), so two shares' gaps on one day stay
// two gaps.
func mergeGaps(gaps []StatGap, watermark string) []StatGap {
	series := func(g StatGap) string { d, _ := json.Marshal(g.Dimensions); return string(d) + g.Reason }
	gaps = slices.Clone(gaps)
	slices.SortStableFunc(gaps, func(a, b StatGap) int {
		if c := strings.Compare(series(a), series(b)); c != 0 {
			return c
		}
		return strings.Compare(a.Start, b.Start) // one zone, one offset format: text order is time order
	})
	var out []StatGap
	for _, g := range gaps {
		if n := len(out); n > 0 && out[n-1].End == g.Start && g.Start != watermark &&
			out[n-1].Reason == g.Reason && sameDims(out[n-1].Dimensions, g.Dimensions) {
			out[n-1].End = g.End
			continue
		}
		out = append(out, g)
	}
	slices.SortStableFunc(out, func(x, y StatGap) int { return strings.Compare(x.Start, y.Start) })
	return out
}

func sameDims(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// StatsReply is GET /stats (v2/schema/stats.json).
type StatsReply struct {
	ContractVersion int               `json:"contract_version"`
	Metric          string            `json:"metric"`
	TakenAt         string            `json:"taken_at"`
	Watermark       string            `json:"watermark"`
	Points          []json.RawMessage `json:"points"`
	Unbound         struct {
		Accounts int `json:"accounts"`
	} `json:"unbound"`
	Gaps       []StatGap `json:"gaps"`
	NextCursor *string   `json:"next_cursor"`
}

// handleStats answers one metric from since, one page at a time, from the
// buffer alone: what the collector last read, never the NAS now.
func (s *server) handleStats(w http.ResponseWriter, r *http.Request, _ []byte) {
	q := r.URL.Query()
	m, ok := declaredMetric(q.Get("metric"))
	if !ok {
		writeError(w, http.StatusNotFound, CodeUnknownMetric, "The manifest declares no such metric.", nil)
		return
	}
	since, err := time.Parse(time.RFC3339, q.Get("since"))
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "since must be an RFC 3339 instant.", nil)
		return
	}
	c := s.stats
	after, err := c.parseCursor(q.Get("cursor"))
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "The cursor is not one this add-on gave.", nil)
		return
	}
	meta, found, err := s.store.StatsMeta(m.Name)
	switch {
	case err != nil:
		writeError(w, http.StatusInternalServerError, CodeStoreUnreadable, "The add-on's statistics could not be read.", nil)
		return
	case !found || meta.TakenAt.IsZero():
		writeError(w, http.StatusServiceUnavailable, CodeNoCurrentRead, "The NAS has not been read for statistics yet.", nil)
		return
	}

	from, today := c.day(since), c.day(c.clock())
	out := StatsReply{ContractVersion: ContractVersion, Metric: m.Name, TakenAt: rfc3339(meta.TakenAt),
		Watermark: c.stamp(meta.Watermark), Points: []json.RawMessage{}, Gaps: []StatGap{}}
	unbound := map[string]bool{}
	for day, ids := range meta.Unbound {
		for _, id := range ids {
			unbound[id] = unbound[id] || day >= from
		}
	}
	for _, in := range unbound {
		if in {
			out.Unbound.Accounts++
		}
	}

	// The page is cut under the ceiling: the envelope, then as many points
	// as fit, each with its comma, and 512 bytes for the cursor. The scan
	// stops once it holds more than a page could, except on the first page,
	// which carries every day's gaps.
	limit := c.pageBytes
	if limit == 0 {
		limit = statsMaxReplyBytes
	}
	var dayGaps []StatGap
	var sent []StatPoint
	var encoded [][]byte
	held := 0
	err = s.store.StatsDays(m.Name, from, func(day string, rec statDay) bool {
		if after == nil {
			dayGaps = append(dayGaps, rec.Gaps...)
		}
		for _, p := range rec.Points {
			if held > limit || after != nil && !after.less(keyOf(day, p)) {
				continue
			}
			b, _ := json.Marshal(p)
			sent, encoded, held = append(sent, p), append(encoded, b), held+len(b)+1
		}
		return after == nil || held <= limit
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeStoreUnreadable, "The add-on's statistics could not be read.", nil)
		return
	}
	if after == nil { // gaps on the first page only
		if from < meta.HeldFrom {
			out.Gaps = append(out.Gaps, StatGap{Start: c.stamp(from), End: c.stamp(meta.HeldFrom), Reason: reasonBuffer})
		}
		out.Gaps = append(out.Gaps, mergeGaps(dayGaps, c.stamp(meta.Watermark))...)
		if meta.Behind != "" {
			out.Gaps = append(out.Gaps, StatGap{Start: c.stamp(meta.Behind), End: c.stamp(nextDayKey(today)), Reason: reasonBehind})
		}
		if lost := nextDayKey(c.day(meta.TakenAt)); meta.Failing && lost <= today {
			out.Gaps = append(out.Gaps, StatGap{Start: c.stamp(lost), End: c.stamp(nextDayKey(today)), Reason: reasonNAS})
		}
		out.Gaps = slices.DeleteFunc(out.Gaps, func(g StatGap) bool {
			end, _ := time.Parse(time.RFC3339, g.End)
			return !end.After(since)
		})
	}

	envelope, _ := json.Marshal(out)
	n := max(fill(encoded, limit-len(envelope)-512), min(1, len(sent)))
	for _, p := range encoded[:n] {
		out.Points = append(out.Points, p)
	}
	if n < len(sent) {
		next := cursorOf(sent[n-1])
		out.NextCursor = &next
	}
	writeJSON(w, http.StatusOK, out)
}
