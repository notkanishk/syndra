package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// The statistics collector and GET /stats against a fake NAS: two members
// (Rey Mysterio bound, Seth Rollins not), shares main (audited for everyone),
// open and scratch (not), a home folder dataset, and an SMB audit log the
// fake filters and pages as audit.query does. The clock is moved by hand.

var kolkata = func() *time.Location { l, _ := time.LoadLocation("Asia/Kolkata"); return l }()

// statsNAS is the fake NAS with an audit log it filters by message_timestamp
// and pages by offset and limit, and an outage switch.
type statsNAS struct {
	*fakeRPC
	mu    sync.Mutex
	rows  []map[string]any
	down  bool
	reads int // audit.query calls
}

func (n *statsNAS) Call(method string, timeout int64, params any) (json.RawMessage, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.down {
		return nil, errors.New("connection refused")
	}
	if method != "audit.query" {
		return n.fakeRPC.Call(method, timeout, params)
	}
	n.reads++
	q := params.([]any)[0].(map[string]any)
	var from int64
	for _, f := range q["query-filters"].([]any) {
		if f := f.([]any); f[0] == "message_timestamp" {
			from = f[2].(int64)
		}
	}
	opts := q["query-options"].(map[string]any)
	var kept []map[string]any
	for _, r := range n.rows {
		if r["message_timestamp"].(int64) >= from {
			kept = append(kept, r)
		}
	}
	slices.SortStableFunc(kept, func(a, b map[string]any) int {
		return int(a["message_timestamp"].(int64) - b["message_timestamp"].(int64))
	})
	off, limit := opts["offset"].(int), opts["limit"].(int)
	kept = kept[min(off, len(kept)):min(off+limit, len(kept))]
	b, _ := json.Marshal(kept)
	return envelope(string(b)), nil
}

// connect is one successful SMB tree connect, as audit.query returns it.
func (n *statsNAS) connect(at time.Time, user, share, session string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.rows = append(n.rows, map[string]any{"message_timestamp": at.Unix(), "event": "CONNECT", "success": true,
		"username": user, "session": session, "audit_id": fmt.Sprint(len(n.rows)),
		"service_data": map[string]any{"service": share, "session_id": "1"}, "event_data": map[string]any{"host": "192.0.2.4"}})
}

const (
	statsUsers = `[{"id":11,"username":"rey_mysterio","uid":3051,"locked":false,"smb":true,"groups":[],"home":"/mnt/tank/main/home_dir/rey_mysterio"},
	               {"id":12,"username":"seth_rollins","uid":3077,"locked":false,"smb":true,"groups":[],"home":"/mnt/tank/main/home_dir/seth_rollins"}]`
	statsShares = `[{"name":"main","path":"/mnt/tank/main","enabled":true,"audit":{"enable":true,"watch_list":[],"ignore_list":[]}},
	                {"name":"open","path":"/mnt/tank/open","enabled":true,"audit":{"enable":false,"watch_list":[],"ignore_list":[]}},
	                {"name":"scratch","path":"/mnt/tank/scratch","enabled":true,"audit":{"enable":true,"watch_list":["lab_makers"],"ignore_list":[]}}]`
)

var day0 = time.Date(2026, 11, 9, 10, 0, 0, 0, kolkata)

func statsServer(t *testing.T) (*server, *statsNAS, *time.Time) {
	t.Helper()
	fake := &fakeRPC{users: statsUsers, groups: reachGroups, shares: statsShares, answers: map[string]string{
		"pool.dataset.get_quota tank/main":          `[{"id":0,"used_bytes":7000},{"id":3051,"used_bytes":1200},{"id":3077,"used_bytes":800}]`,
		"pool.dataset.get_quota tank/open":          `[{"id":3077,"used_bytes":500}]`,
		"pool.dataset.get_quota tank/scratch":       `[]`,
		"pool.dataset.get_quota tank/main/home_dir": `[{"id":3051,"used_bytes":900,"quota":5000},{"id":3077,"used_bytes":400}]`,
	}}
	nas := &statsNAS{fakeRPC: fake}
	s := testServer(t, fake)
	s.nas = newNAS(func() (rpc, error) { return nas, nil }, []string{"25.04"})
	s.nas.version, s.nas.probed, s.nas.selfName = "25.04.2", true, "syndra"
	at := day0
	s.stats = statsClock{zone: kolkata, now: func() time.Time { return at }}
	// Bound by uid, under the name the account had when it was bound.
	if err := s.store.PutBinding(Binding{SubjectID: "sub-rey", Username: "rey_old_name", UID: 3051}); err != nil {
		t.Fatal(err)
	}
	return s, nas, &at
}

type statsPage struct {
	Watermark string      `json:"watermark"`
	TakenAt   string      `json:"taken_at"`
	Points    []StatPoint `json:"points"`
	Gaps      []StatGap   `json:"gaps"`
	Unbound   struct {
		Accounts int `json:"accounts"`
	} `json:"unbound"`
	Next *string `json:"next_cursor"`
}

func getStats(t *testing.T, s *server, metric string, since time.Time, cursor string) (*httptest.ResponseRecorder, statsPage) {
	t.Helper()
	q := url.Values{"metric": {metric}, "since": {since.Format(time.RFC3339)}}
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	rr := httptest.NewRecorder()
	s.handleStats(rr, httptest.NewRequest(http.MethodGet, "/stats?"+q.Encode(), nil), nil)
	var p statsPage
	if rr.Code == http.StatusOK {
		assertSchema(t, "stats.json", rr.Body.Bytes(), "GET /stats "+metric)
		_ = json.Unmarshal(rr.Body.Bytes(), &p)
	}
	return rr, p
}

// allStats follows the cursor to the end, and returns the raw pages too.
func allStats(t *testing.T, s *server, metric string, since time.Time) (statsPage, []StatPoint, []byte) {
	t.Helper()
	rr, first := getStats(t, s, metric, since, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("%s: %d %s", metric, rr.Code, rr.Body)
	}
	raw := slices.Clone(rr.Body.Bytes())
	points, page := first.Points, first
	for page.Next != nil {
		rr, page = getStats(t, s, metric, since, *page.Next)
		raw = append(raw, rr.Body.Bytes()...)
		points = append(points, page.Points...)
	}
	return first, points, raw
}

func onDay(points []StatPoint, day, subject, share string) (int64, bool) {
	for _, p := range points {
		if strings.HasPrefix(p.Start, day) && p.Subject == subject && p.Dimensions["share"] == share {
			return p.Value, true
		}
	}
	return 0, false
}

// Sessions are counted per account, share and day, once each; a member's are
// theirs by uid (bound under an old name, still theirs); an account no
// binding names counts in the total and in unbound, never per person, and no
// name, uid or path is ever on the wire.
func TestSessionsAreCountedByUIDAndNoNameCrossesTheWire(t *testing.T) {
	s, nas, at := statsServer(t)
	nas.connect(day0.Add(-time.Hour), "rey_mysterio", "main", "s1")
	nas.connect(day0.Add(-50*time.Minute), "rey_mysterio", "main", "s1") // the same session again
	nas.connect(day0.Add(-40*time.Minute), "rey_mysterio", "main", "s2")
	nas.connect(day0.Add(-30*time.Minute), "seth_rollins", "main", "s3")
	nas.connect(day0.Add(-20*time.Minute), "seth_rollins", "open", "s4") // auditing off there: not counted
	s.collectStatsOnce()
	*at = day0.Add(24 * time.Hour)
	s.collectStatsOnce()

	since := day0.Add(-48 * time.Hour)
	first, points, raw := allStats(t, s, metricSessions, since)
	if v, _ := onDay(points, "2026-11-09", "", "main"); v != 3 {
		t.Errorf("main's total on 9 Nov is %d, want 3 (two of Rey's sessions, one of Seth's): %+v", v, points)
	}
	if v, ok := onDay(points, "2026-11-09", "sub-rey", "main"); !ok || v != 2 {
		t.Errorf("Rey's sessions on 9 Nov %d (found %v), want 2: usage is matched by uid, not the name bound", v, ok)
	}
	for _, p := range points {
		if p.Subject != "" && p.Subject != "sub-rey" {
			t.Errorf("a per-person point for someone no binding names: %+v", p)
		}
		if p.Dimensions["share"] != "main" {
			t.Errorf("a point on a share whose auditing does not record everyone: %+v", p)
		}
	}
	if first.Unbound.Accounts != 1 {
		t.Errorf("unbound.accounts %d, want 1 (Seth)", first.Unbound.Accounts)
	}
	for _, m := range []string{metricHomeBytes, metricUsedBytes} {
		_, _, r := allStats(t, s, m, since)
		raw = append(raw, r...)
	}
	for _, local := range []string{"rey_mysterio", "seth_rollins", "rey_old_name", "3051", "3077", "/mnt/", "home_dir"} {
		if strings.Contains(string(raw), local) {
			t.Errorf("%q is on the wire", local)
		}
	}
}

// A share whose auditing does not record everyone is a gap for each day,
// never a zero; two such shares keep a gap each on the same day.
func TestAShareNotAuditedForEveryoneIsAGapNotAZero(t *testing.T) {
	s, _, at := statsServer(t)
	s.collectStatsOnce()
	*at = day0.Add(48 * time.Hour)
	s.collectStatsOnce()
	first, points, _ := allStats(t, s, metricSessions, day0.Add(-time.Hour))
	gapped := map[string]StatGap{}
	for _, g := range first.Gaps {
		if g.Reason == reasonAudited && g.Start == "2026-11-09T00:00:00+05:30" {
			gapped[g.Dimensions["share"]] = g
		}
	}
	for _, share := range []string{"open", "scratch"} {
		g, ok := gapped[share]
		if !ok || g.Start != "2026-11-09T00:00:00+05:30" || g.End != "2026-11-11T00:00:00+05:30" {
			t.Errorf("share %s: gap %+v (found %v), want 9-10 Nov joined, up to the watermark", share, g, ok)
		}
	}
	if _, ok := gapped["main"]; ok {
		t.Error("main records everyone and was given a gap")
	}
	for _, p := range points {
		if p.Dimensions["share"] != "main" {
			t.Errorf("a value where auditing was off: %+v", p)
		}
	}
}

// While the NAS cannot be read nothing moves: the watermark stays, the days
// since the last read are one gap, a gauge's dark day becomes a final gap
// once the NAS answers, and the sessions of the dark days are read from the
// log then, filling the window.
func TestTheWatermarkIsHeldWhileTheNASIsDown(t *testing.T) {
	s, nas, at := statsServer(t)
	s.collectStatsOnce() // 9 Nov
	nas.mu.Lock()
	nas.down = true
	nas.mu.Unlock()
	nas.connect(day0.Add(24*time.Hour), "rey_mysterio", "main", "dark")
	*at = day0.Add(26 * time.Hour) // 10 Nov
	s.collectStatsOnce()
	*at = day0.Add(48 * time.Hour) // 11 Nov
	s.collectStatsOnce()

	since := day0.Add(-time.Hour)
	for _, m := range []string{metricSessions, metricUsedBytes} {
		dark, _, _ := allStats(t, s, m, since)
		if dark.Watermark != "2026-11-09T00:00:00+05:30" {
			t.Errorf("%s: with the NAS down the watermark moved to %s", m, dark.Watermark)
		}
		want := StatGap{Start: "2026-11-10T00:00:00+05:30", End: "2026-11-12T00:00:00+05:30", Reason: reasonNAS}
		if !slices.ContainsFunc(dark.Gaps, func(g StatGap) bool { return sameGap(g, want) }) {
			t.Errorf("%s: gaps %+v, want the unread days as %+v", m, dark.Gaps, want)
		}
	}

	nas.mu.Lock()
	nas.down = false
	nas.mu.Unlock()
	s.nas.mu.Lock()
	s.nas.failures, s.nas.openUntil, s.nas.lastTry = 0, time.Time{}, time.Time{} // the breaker's own cooldown, not this test's
	s.nas.mu.Unlock()
	s.collectStatsOnce()
	back, points, _ := allStats(t, s, metricSessions, since)
	if back.Watermark != "2026-11-11T00:00:00+05:30" || slices.ContainsFunc(back.Gaps, func(g StatGap) bool { return g.Reason == reasonNAS }) {
		t.Errorf("back up: watermark %s, gaps %+v; want the start of today and the failure gap withdrawn", back.Watermark, back.Gaps)
	}
	if v, _ := onDay(points, "2026-11-10", "sub-rey", "main"); v != 1 {
		t.Errorf("the dark day's session was not read from the log once the NAS answered: %+v", points)
	}
	used, _, _ := allStats(t, s, metricUsedBytes, since)
	if !slices.ContainsFunc(used.Gaps, func(g StatGap) bool {
		return g.Reason == reasonDark && g.Start == "2026-11-10T00:00:00+05:30" && g.End == "2026-11-11T00:00:00+05:30"
	}) {
		t.Errorf("used_bytes on the dark day: gaps %+v, want a final gap: a level cannot be read after the fact", used.Gaps)
	}
}

func sameGap(a, b StatGap) bool {
	return a.Start == b.Start && a.End == b.End && a.Reason == b.Reason && sameDims(a.Dimensions, b.Dimensions)
}

// A day once final keeps its value: a row the log gains later for it is not
// counted, and since is honoured, nothing ending at or before it.
func TestAFinalDayNeverChangesAndSinceIsHonoured(t *testing.T) {
	s, nas, at := statsServer(t)
	nas.connect(day0.Add(-time.Hour), "rey_mysterio", "main", "s1")
	s.collectStatsOnce()
	*at = day0.Add(24 * time.Hour)
	s.collectStatsOnce() // 9 Nov is final now
	nas.connect(day0.Add(-30*time.Minute), "rey_mysterio", "main", "late")
	*at = day0.Add(48 * time.Hour)
	s.collectStatsOnce()
	_, points, _ := allStats(t, s, metricSessions, day0.Add(-time.Hour))
	if v, _ := onDay(points, "2026-11-09", "", "main"); v != 1 {
		t.Errorf("9 Nov, final, now says %d; a final point keeps its value", v)
	}
	since := time.Date(2026, 11, 10, 9, 0, 0, 0, kolkata)
	first, points, _ := allStats(t, s, metricUsedBytes, since)
	for _, p := range points {
		if end, _ := time.Parse(time.RFC3339, p.End); !end.After(since) {
			t.Errorf("a point ending at or before since: %+v", p)
		}
	}
	for _, g := range first.Gaps {
		if end, _ := time.Parse(time.RFC3339, g.End); !end.After(since) {
			t.Errorf("a gap ending at or before since: %+v", g)
		}
	}
	if len(points) == 0 {
		t.Error("nothing answered from since")
	}
}

// Gaps join into windows, never across the watermark, never across
// dimensions: two shares' gaps are two gaps, even end to end.
func TestGapsJoinNeverAcrossTheWatermarkOrAShare(t *testing.T) {
	c := statsClock{zone: kolkata}
	g := func(day, share string) StatGap {
		return StatGap{Start: c.stamp(day), End: c.stamp(c.next(day)), Dimensions: map[string]string{"share": share}, Reason: reasonAudited}
	}
	got := mergeGaps([]StatGap{g("2026-11-08", "open"), g("2026-11-11", "scratch"), g("2026-11-09", "open"), g("2026-11-10", "open")}, c.stamp("2026-11-10"))
	want := []StatGap{
		{Start: c.stamp("2026-11-08"), End: c.stamp("2026-11-10"), Dimensions: map[string]string{"share": "open"}, Reason: reasonAudited},
		g("2026-11-10", "open"), g("2026-11-11", "scratch")}
	if !slices.EqualFunc(got, want, sameGap) {
		t.Errorf("merged %+v\nwant   %+v", got, want)
	}
}

// A day is the makerspace's: a session at 00:30 in Kolkata (19:00 UTC the
// day before) is counted on its own day, as every other count Syndra keeps.
func TestADayIsTheMakerspacesNotUTCs(t *testing.T) {
	s, nas, at := statsServer(t)
	s.collectStatsOnce()
	nas.connect(time.Date(2026, 11, 10, 0, 30, 0, 0, kolkata), "rey_mysterio", "main", "night")
	*at = time.Date(2026, 11, 10, 9, 0, 0, 0, kolkata)
	s.collectStatsOnce()
	_, points, _ := allStats(t, s, metricSessions, day0.Add(-time.Hour))
	if v, _ := onDay(points, "2026-11-10T00:00:00+05:30", "sub-rey", "main"); v != 1 {
		t.Errorf("a session at 00:30 on 10 Nov was not counted on 10 Nov: %+v", points)
	}
	if _, ok := onDay(points, "2026-11-09", "sub-rey", "main"); ok {
		t.Errorf("a session at 00:30 on 10 Nov was counted on 9 Nov, the UTC day: %+v", points)
	}
}

// A point fits when it and its comma fit exactly; one byte more does not.
func TestAPointFitsToTheLastByte(t *testing.T) {
	pts := [][]byte{[]byte("12345"), []byte("12345")}
	if n := fill(pts, 12); n != 2 {
		t.Errorf("12 bytes hold %d of two 5-byte points with their commas, want 2", n)
	}
	if n := fill(pts, 11); n != 1 {
		t.Errorf("11 bytes hold %d, want 1", n)
	}
}

// Pages fit under the ceiling and follow one another: every point once, in
// keyset order, the cursor past its key.
func TestPagesFollowOneAnotherUnderTheCeiling(t *testing.T) {
	s, nas, at := statsServer(t)
	for d := range 6 {
		for i := range 4 {
			nas.connect(day0.AddDate(0, 0, d).Add(time.Duration(i)*time.Minute), "rey_mysterio", "main", fmt.Sprint(d, i))
		}
		*at = day0.AddDate(0, 0, d)
		s.collectStatsOnce()
	}
	since := day0.AddDate(0, 0, -1)
	s.stats.pageBytes = 0
	_, whole, _ := allStats(t, s, metricSessions, since)
	s.stats.pageBytes = 1900
	pages := 0
	var paged []StatPoint
	rr, page := getStats(t, s, metricSessions, since, "")
	for {
		pages++
		if rr.Body.Len() > s.stats.pageBytes {
			t.Errorf("page %d is %d bytes, above %d", pages, rr.Body.Len(), s.stats.pageBytes)
		}
		paged = append(paged, page.Points...)
		if page.Next == nil {
			break
		}
		rr, page = getStats(t, s, metricSessions, since, *page.Next)
	}
	if pages < 2 || len(paged) != len(whole) || len(whole) == 0 {
		t.Fatalf("%d pages carried %d points, one page %d", pages, len(paged), len(whole))
	}
	for i := 1; i < len(paged); i++ {
		a, b := paged[i-1], paged[i]
		if ka, kb := keyOf(s.stats.day(mustTime(a.Start)), a), keyOf(s.stats.day(mustTime(b.Start)), b); !ka.less(kb) {
			t.Fatalf("point %d does not follow point %d: %+v then %+v", i, i-1, a, b)
		}
	}
}

func mustTime(s string) time.Time { t, _ := time.Parse(time.RFC3339, s); return t }

// The buffer keeps P90D: a day older goes, and a read from before what it
// holds is answered with a gap for that window, then what it holds.
func TestTheBufferDropsTheOldestAndSaysSo(t *testing.T) {
	s, _, at := statsServer(t)
	s.collectStatsOnce()
	*at = day0.AddDate(0, 0, 95)
	s.collectStatsOnce()
	first, points, _ := allStats(t, s, metricUsedBytes, day0.Add(-time.Hour))
	held := s.stats.oldest(*at)
	if len(first.Gaps) == 0 || !sameGap(first.Gaps[0], StatGap{Start: "2026-11-09T00:00:00+05:30", End: s.stats.stamp(held), Reason: reasonBuffer}) {
		t.Errorf("first gap %+v, want 9 Nov up to %s with the buffer's reason", first.Gaps, held)
	}
	for _, p := range points {
		if p.Start < s.stats.stamp(held) {
			t.Errorf("a point from before what the buffer holds: %+v", p)
		}
	}
	if len(points) == 0 {
		t.Error("the points it holds were not answered")
	}
}

// One run asks the audit log for at most statsAuditPages pages. Past that,
// the days from the last one read are a provisional gap, and the next run
// goes on from there.
func TestTheAuditReadIsBoundedAndCatchesUp(t *testing.T) {
	page, pages := statsAuditPage, statsAuditPages
	statsAuditPage, statsAuditPages = 2, 2
	t.Cleanup(func() { statsAuditPage, statsAuditPages = page, pages })
	s, nas, at := statsServer(t)
	s.collectStatsOnce() // 9 Nov: the watermark begins here
	for d := range 3 {
		for i := range 2 {
			nas.connect(day0.AddDate(0, 0, d).Add(time.Duration(i)*time.Minute), "rey_mysterio", "main", fmt.Sprint(d, i))
		}
	}
	*at = day0.AddDate(0, 0, 2)
	nas.reads = 0
	s.collectStatsOnce()
	first, _, _ := allStats(t, s, metricSessions, day0.Add(-time.Hour))
	if nas.reads != 2 {
		t.Errorf("one run asked the audit log %d times, want at most %d", nas.reads, statsAuditPages)
	}
	if first.Watermark != "2026-11-10T00:00:00+05:30" ||
		!slices.ContainsFunc(first.Gaps, func(g StatGap) bool { return g.Reason == reasonBehind && g.Start == "2026-11-10T00:00:00+05:30" }) {
		t.Errorf("capped run: watermark %s, gaps %+v; want 10 Nov and a gap from it", first.Watermark, first.Gaps)
	}
	s.collectStatsOnce()
	s.collectStatsOnce()
	_, points, _ := allStats(t, s, metricSessions, day0.Add(-time.Hour))
	for _, day := range []string{"2026-11-09", "2026-11-10", "2026-11-11"} {
		if v, _ := onDay(points, day, "", "main"); v != 2 {
			t.Errorf("%s counted %d after catching up, want 2", day, v)
		}
	}
}

// The collector is a read path: it writes nothing to the NAS, nothing to the
// mutation log, and leaves the read cache alone.
func TestTheCollectorOnlyReads(t *testing.T) {
	s, nas, at := statsServer(t)
	nas.connect(day0.Add(-time.Hour), "rey_mysterio", "main", "s1")
	_, before := s.log.Head()
	gen := s.fresh.gen
	s.collectStatsOnce()
	*at = day0.Add(24 * time.Hour)
	s.collectStatsOnce()
	if _, after := s.log.Head(); after != before {
		t.Errorf("a collection moved log_records from %d to %d", before, after)
	}
	if s.fresh.gen != gen || len(s.fresh.kept) != 0 {
		t.Error("a collection touched the read cache")
	}
	for _, m := range nas.calls {
		if !readMethods[m] {
			t.Errorf("a collection called %s, which is not a read", m)
		}
	}
}

// Before the first collection there is nothing to answer; an undeclared
// metric is 404 UNKNOWN_METRIC and a since that is not RFC 3339 400.
func TestStatsRefusals(t *testing.T) {
	s, _, _ := statsServer(t)
	for _, tc := range []struct {
		target string
		status int
		code   string
	}{
		{"/stats?metric=storage.sessions&since=2026-11-01T00:00:00Z", 503, CodeNoCurrentRead},
		{"/stats?metric=storage.nothing&since=2026-11-01T00:00:00Z", 404, CodeUnknownMetric},
		{"/stats?metric=storage.sessions&since=2026-11-01", 400, CodeBadRequest},
		{"/stats?metric=storage.sessions&since=2026-11-01T00:00:00Z&cursor=e30", 400, CodeBadRequest},
	} {
		if tc.status != 503 {
			s.collectStatsOnce()
		}
		rr := httptest.NewRecorder()
		s.handleStats(rr, httptest.NewRequest(http.MethodGet, tc.target, nil), nil)
		if rr.Code != tc.status || problemCode(t, rr) != tc.code {
			t.Errorf("%s: %d %s, want %d %s", tc.target, rr.Code, rr.Body, tc.status, tc.code)
		}
	}
}

// The levels: per share, everyone's bytes in the total and a member's own
// point; the home folder against its limit, a level for every bound member
// with a home.
func TestLevelsArePerShareAndPerHome(t *testing.T) {
	s, _, _ := statsServer(t)
	s.collectStatsOnce()
	_, used, _ := allStats(t, s, metricUsedBytes, day0.Add(-time.Hour))
	if v, _ := onDay(used, "2026-11-09", "", "main"); v != 9000 {
		t.Errorf("main's total %d, want 9000 (everyone in it)", v)
	}
	if v, _ := onDay(used, "2026-11-09", "sub-rey", "main"); v != 1200 {
		t.Errorf("Rey's bytes on main %d, want 1200", v)
	}
	first, home, _ := allStats(t, s, metricHomeBytes, day0.Add(-time.Hour))
	if v, _ := onDay(home, "2026-11-09", "sub-rey", ""); v != 900 {
		t.Errorf("Rey's home folder %d, want 900: %+v", v, home)
	}
	if v, _ := onDay(home, "2026-11-09", "", ""); v != 1300 {
		t.Errorf("the home folders' total %d, want 1300", v)
	}
	if first.Unbound.Accounts != 1 {
		t.Errorf("unbound.accounts %d, want 1 (Seth's home)", first.Unbound.Accounts)
	}
}

func TestStatsPagesFitTheContractsCeiling(t *testing.T) {
	c := v2JSON(t, "registry", "constants.json")["constants"].(map[string]any)["max_reply_bytes"].(map[string]any)["value"]
	if n, _ := c.(float64); int(n) != statsMaxReplyBytes {
		t.Errorf("statsMaxReplyBytes %d, constants.max_reply_bytes %v", statsMaxReplyBytes, c)
	}
}

func TestTheCollectorsScheduleAndZoneAreRead(t *testing.T) {
	for raw, want := range map[string]time.Duration{"0": 0, "PT1H": time.Hour, "PT30M": 30 * time.Minute, "PT45S": 45 * time.Second} {
		if got, err := collectEvery(raw); err != nil || got != want {
			t.Errorf("%s read as %v %v, want %v", raw, got, err, want)
		}
	}
	for _, bad := range []string{"", "1h", "P1D", "PT", "PT-1H", "PT1.5H"} {
		if _, err := collectEvery(bad); err == nil {
			t.Errorf("%q was accepted", bad)
		}
	}
	if z, err := loadZone(":Asia/Kolkata"); err != nil || z.String() != "Asia/Kolkata" {
		t.Errorf("TZ=:Asia/Kolkata read as %v %v", z, err)
	}
	if z, _ := loadZone(""); z != time.UTC {
		t.Errorf("no TZ read as %v, want UTC (what the clock then reads)", z)
	}
	if _, err := loadZone("Mars/Olympus_Mons"); err == nil {
		t.Error("a zone tzdb lacks was accepted")
	}
	done := make(chan struct{})
	go func() { collectStats(t.Context(), nil, 0); close(done) }() // 0 never touches the server
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Error("STATS_COLLECT_EVERY=0 still ran the collector")
	}
}
