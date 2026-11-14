package reference

import (
	// Zone data in the binary, so statsZone loads wherever the add-on runs.
	_ "time/tzdata"

	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"
)

// Statistics (feature stats; guide chapter 4, "Statistics"). Two metrics from
// one job log: machine.jobs, a counter of print jobs per printer, and
// machine.longest_job_seconds, a gauge of the longest job of the day, metered
// against the job_seconds limit. Every bucket is one calendar day in
// statsZone (RFC 3339 with its offset), and a point is the whole day: equal
// keys replace, never add (not OpenTelemetry delta temporality).
//
// The collector is the add-on's: here it reads the in-memory app on every
// /stats, before answering, which costs an in-memory app nothing (a real
// add-on reads its app on its own timer, as the TrueNAS add-on does). It
// fills every day from the watermark through today into a bounded buffer;
// days before today are final, and the watermark, the start of today, stays
// where it was while the app cannot be read (Config.Down).

const (
	// defaultStatsZone is the reference's zone when TZ is unset.
	defaultStatsZone = "Asia/Kolkata"
	// statsBufferDays is limits.stats_buffer (P30D): a day stays while its end
	// is within it.
	statsBufferDays = 30
	// statsMaxPoints is the buffer's size ceiling; the oldest days go first.
	statsMaxPoints = 2000
	// statsPageBytes bounds a page far under constants.max_reply_bytes, so the
	// kit sees several pages and their cursors.
	statsPageBytes = 4 << 10
	// guest is the print queue's shared walk-up login: never bound to anyone,
	// so its jobs count in the totals and in unbound, never per person.
	guest = "guest"

	bufferReason  = "The add-on no longer holds these days"
	failureReason = "The print queue could not be read"
	offReason     = "The xl-1 printer's job log could not be read"
)

// statsZoneName is TZ (POSIX allows a leading colon), else the default: the
// zone every deployment of Syndra's services follows, so the day buckets here
// agree with the container's own clock. The TrueNAS add-on reads TZ the same
// way (loadZone). An unreadable TZ is a misconfiguration, refused at start.
var statsZoneName, statsZone = zoneFromTZ(os.Getenv("TZ"))

func zoneFromTZ(tz string) (string, *time.Location) {
	name := strings.TrimPrefix(strings.TrimSpace(tz), ":")
	if name == "" {
		name = defaultStatsZone
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic("reference add-on: TZ=" + name + " is not an IANA zone: " + err.Error())
	}
	return name, loc
}

// statsMetrics are the manifest's descriptors, and the one list /stats
// answers for.
var statsMetrics = []map[string]any{
	{"name": "machine.jobs", "instrument": "counter", "unit": "count", "label": "Print jobs", "per_subject": true,
		"dimensions": []map[string]string{{"name": "printer", "label": "Printer"}}},
	{"name": "machine.longest_job_seconds", "instrument": "gauge", "unit": "seconds", "label": "Longest print job",
		"per_subject": true, "against": "job_seconds"},
}

func statsMetric(name string) map[string]any {
	for _, m := range statsMetrics {
		if m["name"] == name {
			return m
		}
	}
	return nil
}

type point struct {
	Start      string            `json:"start"`
	End        string            `json:"end"`
	Subject    string            `json:"subject,omitempty"`
	Dimensions map[string]string `json:"dimensions,omitempty"`
	Value      int64             `json:"value"`
}

type gap struct {
	Start      string            `json:"start"`
	End        string            `json:"end"`
	Dimensions map[string]string `json:"dimensions,omitempty"`
	Reason     string            `json:"reason"`
}

// statDay is one metric's day in the buffer: its points in key order, its
// gaps, and who printed without a binding.
type statDay struct {
	points  []point
	gaps    []gap
	unbound []string
}

// statsState is the buffer, under Addon.mu.
type statsState struct {
	now       func() time.Time                 // nil: time.Now; the add-on's own test moves it
	buf       map[string]map[time.Time]statDay // metric → day → record
	started   time.Time                        // the first day collected; xl-1's log was lost the week before
	seen      map[string]time.Time             // account id → the first day the log has it
	watermark time.Time                        // days before it are final; the start of the last day read
	heldFrom  time.Time                        // the oldest day the buffer holds
	takenAt   time.Time                        // when the app was last read
	failing   bool                             // the last attempt could not read the app
}

// dayOf is the first instant of the day t falls in: 00:00, or, on a day
// whose 00:00 a daylight-saving change skips (America/Santiago, 6 Sep 2026),
// the instant the clocks resume. Go resolves a skipped 00:00 to the previous
// day's 23:00, so that one case takes the end of the zone period it fell in.
func dayOf(t time.Time) time.Time {
	l := t.In(statsZone)
	d := time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, statsZone)
	if d.Day() != l.Day() {
		_, d = d.ZoneBounds()
	}
	return d
}

// next is the next day's first instant: a calendar day, never 24 h (a zone
// with DST has days of 23 and 25 hours). Noon is inside the next day
// whatever the zone does at midnight.
func next(day time.Time) time.Time {
	l := day.In(statsZone)
	return dayOf(time.Date(l.Year(), l.Month(), l.Day()+1, 12, 0, 0, 0, statsZone))
}

func stamp3339(t time.Time) string { return t.In(statsZone).Format(time.RFC3339) }

func (a *Addon) statsNow() time.Time {
	if a.stats.now != nil {
		return a.stats.now()
	}
	return time.Now()
}

// collect reads the app into the buffer: every day from the watermark
// through today, each replaced whole. Under a.mu.
func (a *Addon) collect(at time.Time) {
	st := &a.stats
	today := dayOf(at)
	if st.buf == nil {
		st.buf, st.seen, st.started = map[string]map[time.Time]statDay{}, map[string]time.Time{}, today
		for _, m := range statsMetrics {
			st.buf[m["name"].(string)] = map[time.Time]statDay{}
		}
	}
	if a.cfg.Down != nil && a.cfg.Down() {
		st.failing = true // nothing read, so nothing moves
		return
	}
	a.fresh.reads++
	for id := range a.accounts {
		if _, ok := st.seen[id]; !ok {
			st.seen[id] = today
		}
	}
	oldest := dayOf(at.AddDate(0, 0, -statsBufferDays))
	from := st.watermark
	if from.IsZero() {
		from = oldest
	}
	for d := from; !d.After(today); d = next(d) {
		jobs, longest := a.statsDay(d)
		st.buf["machine.jobs"][d], st.buf["machine.longest_job_seconds"][d] = jobs, longest
	}
	st.watermark, st.takenAt, st.failing = today, at, false

	// The buffer's bounds: a day leaves once its end is older than the
	// buffer, or, past the size ceiling, oldest first.
	days := slices.SortedFunc(func(yield func(time.Time) bool) {
		for d := range st.buf["machine.jobs"] {
			if !yield(d) {
				return
			}
		}
	}, time.Time.Compare)
	count := 0
	for _, d := range days {
		for _, m := range st.buf {
			count += len(m[d].points)
		}
	}
	if st.heldFrom.Before(oldest) {
		st.heldFrom = oldest // the held window only ever moves forwards
	}
	for _, d := range days {
		if !d.Before(st.heldFrom) && count <= statsMaxPoints {
			break
		}
		for _, m := range st.buf {
			count -= len(m[d].points)
			delete(m, d)
		}
		if next(d).After(st.heldFrom) {
			st.heldFrom = next(d)
		}
	}
}

// who printed: every account the log has by d, by its binding's subject
// ("" when unbound), and the guest login.
type sender struct{ key, subject string }

func (a *Addon) sendersOn(d time.Time) []sender {
	subjectOf := map[string]string{}
	for subj, id := range a.bindings {
		subjectOf[id] = subj
	}
	out := []sender{{key: guest}}
	for id, first := range a.stats.seen {
		if !first.After(d) {
			out = append(out, sender{key: id, subject: subjectOf[id]})
		}
	}
	slices.SortFunc(out, func(x, y sender) int { return strings.Compare(x.key, y.key) })
	return out
}

// jobsOn is the job log: how many jobs who sent to a printer that day,
// deterministic, and at least one a day on the first printer, so every
// account has usage on every day it exists.
func jobsOn(who, p string, d time.Time) int64 {
	h := fnv.New32a()
	fmt.Fprintf(h, "%s|%s|%s", who, p, d.Format(time.DateOnly))
	n := int64(h.Sum32() % 3)
	if p == printers[0] {
		n++
	}
	return n
}

func longestOn(who string, d time.Time) int64 {
	h := fnv.New32a()
	fmt.Fprintf(h, "%s|%s|longest", who, d.Format(time.DateOnly))
	return 60 + int64(h.Sum32()%3540)
}

// statsDay derives one day of both metrics from the log.
func (a *Addon) statsDay(d time.Time) (jobs, longest statDay) {
	start, end := stamp3339(d), stamp3339(next(d))
	off := !d.Before(a.stats.started.AddDate(0, 0, -6)) && d.Before(a.stats.started.AddDate(0, 0, -4))
	who := a.sendersOn(d)
	unbound := map[string]bool{}
	var maxAll int64
	var perLongest []point
	for _, p := range printers {
		dims := map[string]string{"printer": p}
		if p == "xl-1" && off {
			jobs.gaps = append(jobs.gaps, gap{Start: start, End: end, Dimensions: dims, Reason: offReason})
			continue
		}
		var total int64
		for _, w := range who {
			n := jobsOn(w.key, p, d)
			if n == 0 {
				continue
			}
			total += n
			if w.subject == "" {
				unbound[w.key] = true
			} else {
				jobs.points = append(jobs.points, point{Start: start, End: end, Subject: w.subject, Dimensions: dims, Value: n})
			}
		}
		if total > 0 {
			jobs.points = append(jobs.points, point{Start: start, End: end, Dimensions: dims, Value: total})
		}
	}
	for _, w := range who {
		printed := false
		for _, p := range printers {
			printed = printed || jobsOn(w.key, p, d) > 0 && !(p == "xl-1" && off)
		}
		if !printed {
			continue
		}
		l := longestOn(w.key, d)
		maxAll = max(maxAll, l)
		if w.subject != "" {
			perLongest = append(perLongest, point{Start: start, End: end, Subject: w.subject, Value: l})
		}
	}
	if maxAll > 0 {
		longest.points = append([]point{{Start: start, End: end, Value: maxAll}}, perLongest...)
	}
	for k := range unbound {
		jobs.unbound = append(jobs.unbound, k)
	}
	longest.unbound = jobs.unbound
	slices.SortFunc(jobs.points, func(x, y point) int { return compareKeys(keyOf(x), keyOf(y)) })
	slices.SortFunc(longest.points, func(x, y point) int { return compareKeys(keyOf(x), keyOf(y)) })
	return jobs, longest
}

// key is a point's place in the keyset order: (start, subject, dimensions),
// the total ("" subject) first.
type key struct {
	start   time.Time
	subject string
	dims    string // canonical JSON: encoding/json sorts map keys
}

func keyOf(p point) key {
	start, _ := time.Parse(time.RFC3339, p.Start)
	dims := []byte("{}")
	if len(p.Dimensions) > 0 {
		dims, _ = json.Marshal(p.Dimensions)
	}
	return key{start, p.Subject, string(dims)}
}

func compareKeys(x, y key) int {
	if c := x.start.Compare(y.start); c != 0 {
		return c
	}
	if c := strings.Compare(x.subject, y.subject); c != 0 {
		return c
	}
	return strings.Compare(x.dims, y.dims)
}

// cursorOf is the opaque next_cursor for a point: base64url JSON of its key.
func cursorOf(p point) string {
	raw, _ := json.Marshal(map[string]any{"s": p.Start, "u": p.Subject, "d": p.Dimensions})
	return base64.RawURLEncoding.EncodeToString(raw)
}

func parseCursor(c string) (*key, error) {
	if c == "" {
		return nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil {
		return nil, err
	}
	var k struct {
		S string            `json:"s"`
		U string            `json:"u"`
		D map[string]string `json:"d"`
	}
	if err := json.Unmarshal(raw, &k); err != nil {
		return nil, err
	}
	p := point{Start: k.S, Subject: k.U, Dimensions: k.D}
	if _, err := time.Parse(time.RFC3339, k.S); err != nil {
		return nil, err
	}
	kk := keyOf(p)
	return &kk, nil
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

// mergeGaps joins a metric's day gaps with the same dimensions and reason
// into windows, never across the watermark: a final gap keeps the end it had.
func mergeGaps(gaps []gap, watermark string) []gap {
	series := func(g gap) string { d, _ := json.Marshal(g.Dimensions); return string(d) + g.Reason }
	gaps = slices.Clone(gaps)
	slices.SortStableFunc(gaps, func(x, y gap) int {
		if c := strings.Compare(series(x), series(y)); c != 0 {
			return c
		}
		return strings.Compare(x.Start, y.Start) // one zone, one offset format: text order is time order
	})
	var out []gap
	for _, g := range gaps {
		if n := len(out); n > 0 && out[n-1].End == g.Start && g.Start != watermark &&
			out[n-1].Reason == g.Reason && equal(out[n-1].Dimensions, g.Dimensions) {
			out[n-1].End = g.End
			continue
		}
		out = append(out, g)
	}
	slices.SortStableFunc(out, func(x, y gap) int { return strings.Compare(x.Start, y.Start) })
	return out
}

// statsRead answers GET /stats?metric=&since=[&cursor=].
func (a *Addon) statsRead(r *http.Request, _ []byte) reply {
	q := r.URL.Query()
	m := statsMetric(q.Get("metric"))
	if m == nil {
		return problem(404, "UNKNOWN_METRIC", "The manifest declares no such metric.", nil)
	}
	since, err := time.Parse(time.RFC3339, q.Get("since"))
	if err != nil {
		return problem(400, "BAD_REQUEST", "since must be an RFC 3339 instant.", nil)
	}
	after, err := parseCursor(q.Get("cursor"))
	if err != nil {
		return problem(400, "BAD_REQUEST", "The cursor is not one this add-on gave.", nil)
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.collect(a.statsNow())
	st := &a.stats
	if st.takenAt.IsZero() {
		return problem(503, "NO_CURRENT_READ", "The print queue has not been read yet.", nil)
	}
	name, from := m["name"].(string), dayOf(since)
	days := st.buf[name]
	var points []point
	var gaps []gap
	unbound := map[string]bool{}
	for d := from; !d.After(dayOf(st.takenAt)); d = next(d) {
		rec := days[d]
		for _, p := range rec.points {
			if after == nil || compareKeys(keyOf(p), *after) > 0 {
				points = append(points, p)
			}
		}
		gaps = append(gaps, rec.gaps...)
		for _, u := range rec.unbound {
			unbound[u] = true
		}
	}

	watermark := stamp3339(st.watermark)
	doc := map[string]any{"contract_version": ContractVersion, "metric": name,
		"taken_at": st.takenAt.UTC().Format(time.RFC3339), "watermark": watermark,
		"unbound": map[string]int{"accounts": len(unbound)}, "gaps": []gap{}, "next_cursor": nil}
	if after == nil { // gaps on the first page only
		all := []gap{}
		if from.Before(st.heldFrom) {
			all = append(all, gap{Start: stamp3339(from), End: stamp3339(st.heldFrom), Reason: bufferReason})
		}
		all = append(all, mergeGaps(gaps, watermark)...)
		if lost := next(dayOf(st.takenAt)); st.failing && lost.Before(next(dayOf(a.statsNow()))) {
			all = append(all, gap{Start: stamp3339(lost), End: stamp3339(next(dayOf(a.statsNow()))), Reason: failureReason})
		}
		doc["gaps"] = all
	}

	encoded := make([][]byte, len(points))
	for i, p := range points {
		encoded[i], _ = json.Marshal(p)
	}
	doc["points"] = []json.RawMessage{}
	envelope, _ := json.Marshal(doc)
	n := max(fill(encoded, statsPageBytes-len(envelope)-256), min(1, len(points))) // 256: the cursor
	page := make([]json.RawMessage, n)
	for i := range page {
		page[i] = encoded[i]
	}
	doc["points"] = page
	if n < len(points) {
		doc["next_cursor"] = cursorOf(points[n-1])
	}
	return ok(doc)
}
