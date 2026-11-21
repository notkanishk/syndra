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

	bolt "go.etcd.io/bbolt"
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
	off, _ := opts["offset"].(int)
	limit := opts["limit"].(int)
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
		"pool.dataset.query": `[{"name":"tank/main","mountpoint":"/mnt/tank/main"},
			{"name":"tank/main/home_dir","mountpoint":"/mnt/tank/main/home_dir"},
			{"name":"tank/open","mountpoint":"/mnt/tank/open"},
			{"name":"tank/scratch","mountpoint":"/mnt/tank/scratch"}]`,
	}}
	nas := &statsNAS{fakeRPC: fake}
	// The log has kept rows since long before: no day the tests count is
	// one the NAS has purged.
	nas.rows = []map[string]any{{"message_timestamp": time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).Unix(), "event": "AUTHENTICATION",
		"success": true, "username": "rey_mysterio", "audit_id": "old", "service_data": map[string]any{}}}
	s := testServer(t, fake)
	s.nas = newNAS(func() (rpc, error) { return nas, nil }, []string{"25.04"})
	s.nas.version, s.nas.probed, s.nas.selfName = "25.04.2", true, "syndra"
	at := day0
	s.stats = statsClock{zone: kolkata, now: func() time.Time { return at }}
	// Bound by uid, under the name the account had when it was bound. A rename is followed by number
	// only for a number this add-on hands out (never reused), so the fixture's range holds 3051 and
	// 3077; an adopted account outside the range is a conflict instead (binding_uid_test.go).
	s.idFloor, s.idCeiling = 3000, 3999
	if err := s.store.PutBinding(Binding{SubjectID: "sub-rey", Username: "rey_old_name", UID: 3051, UIDKind: uidKindAccount}); err != nil {
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
	// A disabled share audited for everyone is not a known share (auditRow.share): not counted.
	nas.shares = strings.TrimSuffix(statsShares, "]") +
		`,{"name":"old","path":"/mnt/tank/old","enabled":false,"audit":{"enable":true,"watch_list":[],"ignore_list":[]}}]`
	nas.connect(day0.Add(-10*time.Minute), "seth_rollins", "old", "s5")
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
			t.Errorf("a point on a share that is not a known share audited for everyone: %+v", p)
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
		if g.Reason() == reasonAudited.text && g.Start == "2026-11-09T00:00:00+05:30" {
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
		want := gapOf("2026-11-10T00:00:00+05:30", "2026-11-12T00:00:00+05:30", nil, reasonNAS)
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
	if back.Watermark != "2026-11-11T00:00:00+05:30" || slices.ContainsFunc(back.Gaps, func(g StatGap) bool { return g.Reason() == reasonNAS.text }) {
		t.Errorf("back up: watermark %s, gaps %+v; want the start of today and the failure gap withdrawn", back.Watermark, back.Gaps)
	}
	if v, _ := onDay(points, "2026-11-10", "sub-rey", "main"); v != 1 {
		t.Errorf("the dark day's session was not read from the log once the NAS answered: %+v", points)
	}
	used, _, _ := allStats(t, s, metricUsedBytes, since)
	if !slices.ContainsFunc(used.Gaps, func(g StatGap) bool {
		return g.Reason() == reasonDark.text && g.Start == "2026-11-10T00:00:00+05:30" && g.End == "2026-11-11T00:00:00+05:30"
	}) {
		t.Errorf("used_bytes on the dark day: gaps %+v, want a final gap: a level cannot be read after the fact", used.Gaps)
	}
}

func sameGap(a, b StatGap) bool {
	return a.Start == b.Start && a.End == b.End && a.why == b.why && sameDims(a.Dimensions, b.Dimensions)
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
		return gapOf(c.stamp(day), c.stamp(nextDayKey(day)), map[string]string{"share": share}, reasonAudited)
	}
	got := mergeGaps([]StatGap{g("2026-11-08", "open"), g("2026-11-11", "scratch"), g("2026-11-09", "open"), g("2026-11-10", "open")}, c.stamp("2026-11-10"))
	want := []StatGap{
		gapOf(c.stamp("2026-11-08"), c.stamp("2026-11-10"), map[string]string{"share": "open"}, reasonAudited),
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
	if len(first.Gaps) == 0 || !sameGap(first.Gaps[0], gapOf("2026-11-09T00:00:00+05:30", s.stats.stamp(held), nil, reasonBuffer)) {
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
	if nas.reads != 1+statsAuditPages { // the oldest row the log keeps, then the pages
		t.Errorf("one run asked the audit log %d times, want at most 1+%d", nas.reads, statsAuditPages)
	}
	if first.Watermark != "2026-11-10T00:00:00+05:30" ||
		!slices.ContainsFunc(first.Gaps, func(g StatGap) bool {
			return g.Reason() == reasonBehind.text && g.Start == "2026-11-10T00:00:00+05:30"
		}) {
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
	// main reaches tank/main and the home_dir dataset inside it.
	if v, _ := onDay(used, "2026-11-09", "", "main"); v != 9000+1300 {
		t.Errorf("main's total %d, want 10300 (everyone in it, home_dir included)", v)
	}
	if v, _ := onDay(used, "2026-11-09", "sub-rey", "main"); v != 1200+900 {
		t.Errorf("Rey's bytes on main %d, want 2100", v)
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

// nestedStatsServer is statsServer with the folders the live read resolves:
// waddle and quack datasets of their own inside main, Rey's home a dataset of
// its own, Seth's a plain folder in home_dir, and nobody with a row on main's
// own dataset but its root.
func nestedStatsServer(t *testing.T) (*server, *statsNAS, *time.Time) {
	t.Helper()
	s, nas, at := statsServer(t)
	for k, v := range reachFake().answers {
		nas.answers[k] = v // the folders, their ACLs and their datasets
	}
	nas.answers["pool.dataset.query"] = `[{"name":"tank/main","mountpoint":"/mnt/tank/main"},
		{"name":"tank/main/waddle","mountpoint":"/mnt/tank/main/waddle"},
		{"name":"tank/main/quack","mountpoint":"/mnt/tank/main/quack"},
		{"name":"tank/main/home_dir","mountpoint":"/mnt/tank/main/home_dir"},
		{"name":"tank/main/home_dir/rey_mysterio","mountpoint":"/mnt/tank/main/home_dir/rey_mysterio"},
		{"name":"tank/open","mountpoint":"/mnt/tank/open"},
		{"name":"tank/scratch","mountpoint":"/mnt/tank/scratch"}]`
	nas.answers["pool.dataset.get_quota tank/main"] = `[{"id":0,"used_bytes":33344}]`
	nas.answers["pool.dataset.get_quota tank/main/waddle"] = `[{"id":3051,"used_bytes":67036288},{"id":3077,"used_bytes":10}]`
	nas.answers["pool.dataset.get_quota tank/main/quack"] = `[{"id":3051,"used_bytes":5}]`
	nas.answers["pool.dataset.get_quota tank/main/home_dir"] = `[{"id":3077,"used_bytes":400}]`
	nas.answers["pool.dataset.get_quota tank/main/home_dir/rey_mysterio"] = `[{"id":3051,"used_bytes":2500,"quota":5000}]`

	return s, nas, at
}

// Staging, 21 Nov 2026: a team folder that is its own dataset inside a share
// read 67,036,288 bytes live while the share's collected total was the top
// dataset's 33,344, and members had no per-person series. The collector now
// counts by the live read's resolution: a share's level is every dataset it
// reaches, each once; a home is its own dataset's.
func TestCollectedLevelsAreWhatTheLiveReadSees(t *testing.T) {
	s, _, _ := nestedStatsServer(t)
	s.collectStatsOnce()

	staff := []apiID{{raw: json.RawMessage("51")}}
	live, _, err := placesOf(s, 3051, staff, "/mnt/tank/main/home_dir/rey_mysterio")
	if err != nil {
		t.Fatal(err)
	}
	liveAt := map[string]int64{} // by folder, "" the home
	for _, p := range live {
		if p.Usage == nil {
			continue
		}
		if p.Home {
			liveAt[""] = p.Usage.UsedBytes
		} else {
			liveAt[p.Folder] = p.Usage.UsedBytes
		}
	}
	if liveAt["waddle"] != 67036288 || liveAt["quack"] != 5 || liveAt[""] != 2500 {
		t.Fatalf("the live read's usage %v, not the fixture's", liveAt)
	}

	_, used, _ := allStats(t, s, metricUsedBytes, day0.Add(-time.Hour))
	// Rey has no row on tank/main: everything of his on main is in the
	// datasets the live read names for his places there.
	rey, ok := onDay(used, "2026-11-09", "sub-rey", "main")
	if !ok {
		t.Fatalf("no per-person series for Rey on main: %+v", used)
	}
	if want := liveAt["waddle"] + liveAt["quack"] + liveAt[""]; rey != want {
		t.Errorf("Rey's collected bytes on main %d, live %d", rey, want)
	}
	// Each dataset once: tank/main, waddle, quack, home_dir, Rey's home.
	if v, _ := onDay(used, "2026-11-09", "", "main"); v != 33344+67036288+10+5+400+2500 {
		t.Errorf("main's total %d, want 67072547 (every dataset in it once)", v)
	}
	_, home, _ := allStats(t, s, metricHomeBytes, day0.Add(-time.Hour))
	if v, _ := onDay(home, "2026-11-09", "sub-rey", ""); v != liveAt[""] {
		t.Errorf("Rey's home folder collected %d, live %d", v, liveAt[""])
	}
	if v, _ := onDay(home, "2026-11-09", "", ""); v != 2500+400 {
		t.Errorf("the home folders' total %d, want 2900 (his dataset and home_dir, once each)", v)
	}
}

// One number per thing (design.md, "The storage numbers, reconciled"): at one
// moment storage.status's share figure is storage.used_bytes's, and /homes'
// used bytes are storage.home_bytes's, for a home that is a dataset of its own
// (Rey's) and one that is a plain folder (Seth's). On staging, 21 Nov 2026, a
// member with no row on a share's own dataset had no live figure but 67 MB
// collected, and /homes said 0 B for a home its person page said 2.5 KB of.
func TestLiveAndCollectedStorageAreOneNumber(t *testing.T) {
	s, _, _ := nestedStatsServer(t)
	if err := s.store.PutBinding(Binding{SubjectID: "sub-seth", Username: "seth_rollins", UID: 3077, UIDKind: uidKindAccount}); err != nil {
		t.Fatal(err)
	}
	s.collectStatsOnce()
	_, used, _ := allStats(t, s, metricUsedBytes, day0.Add(-time.Hour))
	_, home, _ := allStats(t, s, metricHomeBytes, day0.Add(-time.Hour))
	homes, err := s.readHomes()
	if err != nil {
		t.Fatal(err)
	}
	for uid, subject := range map[int64]string{3051: "sub-rey", 3077: "sub-seth"} {
		meters, err := metersOf(s, uid)
		if err != nil {
			t.Fatalf("%s: %v", subject, err)
		}
		live := map[string]int64{}
		for _, m := range meters {
			live[m.Label] = m.Used
		}
		for _, share := range []string{"main", "open", "scratch"} {
			if got, _ := onDay(used, "2026-11-09", subject, share); got != live[share] {
				t.Errorf("%s on %s: collected %d, live %d", subject, share, got, live[share])
			}
		}
		if live["main"] == 0 {
			t.Errorf("%s has no live figure on main: %v", subject, meters)
		}
		var homeLive int64 = -1
		for _, h := range homes.Homes {
			if h.Subject == subject {
				homeLive = h.UsedBytes
			}
		}
		if got, _ := onDay(home, "2026-11-09", subject, ""); got != homeLive || got == 0 {
			t.Errorf("%s's home: collected %d, /homes %d", subject, got, homeLive)
		}
	}
	// The limit stays the share's own dataset's, and a home's the parent's.
	for _, h := range homes.Homes {
		if h.Subject == "sub-rey" && h.QuotaBytes != 0 {
			t.Errorf("Rey's home limit %d, want home_dir's (none): the write sets the parent's", h.QuotaBytes)
		}
	}
}

// A nested dataset that cannot be read (an encrypted one whose key is not
// loaded refuses get_quota) leaves the share counted without it, with a gap
// naming it: never the whole share unsampled, never that folder as zero.
func TestAnUnreadableNestedDatasetIsAGapNotTheShare(t *testing.T) {
	s, nas, at := nestedStatsServer(t)
	if nas.refuse == nil {
		nas.refuse = map[string]string{}
	}
	nas.refuse["pool.dataset.get_quota tank/main/quack"] = "[EFAULT] tank/main/quack is locked"
	s.collectStatsOnce()
	*at = day0.Add(24 * time.Hour) // the day closes: its gap is final, not "could not be read that day"
	s.collectStatsOnce()
	page, used, _ := allStats(t, s, metricUsedBytes, day0.Add(-time.Hour))
	if v, _ := onDay(used, "2026-11-09", "", "main"); v != 33344+67036288+10+400+2500 {
		t.Errorf("main's total %d, want every readable dataset in it", v)
	}
	if v, _ := onDay(used, "2026-11-09", "sub-rey", "main"); v != 67036288+2500 {
		t.Errorf("Rey on main %d, want waddle and his home, quack left out", v)
	}
	var mainGaps []StatGap
	for _, g := range page.Gaps {
		if g.Dimensions["share"] == "main" {
			mainGaps = append(mainGaps, g)
		}
	}
	if len(mainGaps) == 0 || mainGaps[0].Start != "2026-11-09T00:00:00+05:30" {
		t.Errorf("main's gaps %+v, want them from the first day", mainGaps)
	}
	for _, g := range mainGaps { // the closed day keeps its reason, not "could not be read that day"
		if g.Kind() != kindUnread || !strings.Contains(g.Reason(), "quack") {
			t.Errorf("main's gap %+v, want unread, naming quack", g)
		}
	}
	meters, err := metersOf(s, 3051)
	if err == nil || !strings.Contains(err.Error(), "main") {
		t.Errorf("the live read passed a partial share off as whole: %v", err)
	}
	for _, m := range meters {
		if m.Label == "main" && m.Used != 67036288+2500 {
			t.Errorf("live main %d, want what was read, as collected", m.Used)
		}
	}
}

// A share that is a plain folder inside another share's dataset is not that
// dataset: nothing of the parent is counted for it, live or collected, and it
// is a gap saying the NAS does not count it on its own.
func TestAPlainFolderShareNeverCountsItsParent(t *testing.T) {
	s, nas, at := nestedStatsServer(t)
	nas.shares = strings.TrimSuffix(statsShares, "]") +
		`,{"name":"notes","path":"/mnt/tank/main/notes","enabled":true,"audit":{"enable":false,"watch_list":[],"ignore_list":[]}}]`
	s.collectStatsOnce()
	*at = day0.Add(24 * time.Hour)
	s.collectStatsOnce()
	page, used, _ := allStats(t, s, metricUsedBytes, day0.Add(-time.Hour))
	for _, p := range used {
		if p.Dimensions["share"] == "notes" {
			t.Errorf("notes has a point %+v: a plain folder counted its parent dataset", p)
		}
	}
	if v, _ := onDay(used, "2026-11-09", "", "main"); v != 33344+67036288+10+5+400+2500 {
		t.Errorf("main's total %d, changed by a share inside it", v)
	}
	var gaps []StatGap
	for _, g := range page.Gaps {
		if g.Dimensions["share"] == "notes" {
			gaps = append(gaps, g)
		}
	}
	if len(gaps) == 0 || gaps[0].Start != "2026-11-09T00:00:00+05:30" {
		t.Errorf("notes' gaps %+v, want them from the first day", gaps)
	}
	for _, g := range gaps { // the closed day keeps its reason, not "could not be read that day"
		if g.Kind() != kindUnobserved || g.Reason() != reasonFolder.text {
			t.Errorf("notes' gap %+v, want %q", g, reasonFolder.text)
		}
	}
	meters, err := metersOf(s, 3051)
	if err != nil {
		t.Errorf("a plain folder made the live read partial: %v (it has no table to read)", err)
	}
	for _, m := range meters {
		if m.Label == "notes" {
			t.Errorf("live notes %+v: a plain folder counted its parent dataset", m)
		}
	}
}

// The folder boundary: /mnt/tank/share2 is beside /mnt/tank/share, not in it.
func TestASharesDatasetsStopAtItsFolder(t *testing.T) {
	mounts := datasetMap{at: map[string]string{"/mnt/tank/share": "tank/share", "/mnt/tank/share2": "tank/share2",
		"/mnt/tank/share2/x": "tank/share2/x", "/mnt/tank/share/x": "tank/share/x"}}
	read := map[string]bool{}
	u := usageUnder(mounts, "/mnt/tank/share", func(d string) map[int64]PlaceUsage {
		read[d] = true
		return map[int64]PlaceUsage{1: {UsedBytes: 1}}
	}, nil)
	if !read["tank/share"] || !read["tank/share/x"] || read["tank/share2"] || read["tank/share2/x"] || u.ByUID[1] != 2 {
		t.Errorf("share read %v, counted %d: want share and share/x only", read, u.ByUID[1])
	}
}

// lockedAt is nestedStatsServer's datasets with one locked, answering no
// mountpoint, as a locked dataset likely does (UNVERIFIED on the real NAS).
func lockedAt(name string) string {
	out := []string{}
	for _, d := range []string{"tank/main", "tank/main/waddle", "tank/main/quack", "tank/main/home_dir",
		"tank/main/home_dir/rey_mysterio", "tank/open", "tank/scratch"} {
		if d == name {
			out = append(out, `{"name":"`+d+`","mountpoint":null,"locked":true}`)
		} else {
			out = append(out, `{"name":"`+d+`","mountpoint":"/mnt/`+d+`","locked":false}`)
		}
	}
	return "[" + strings.Join(out, ",") + "]"
}

// A locked dataset inside a share (encrypted, its key not loaded) answers no
// mountpoint, so a count by mountpoint left it out of the share's sum with no
// gap at all. It is found by its name, never read, and makes the share's
// figure partial, said to be because it is locked: live, on the card, and
// collected.
func TestALockedNestedDatasetIsNeverZero(t *testing.T) {
	s, nas, _ := nestedStatsServer(t)
	nas.answers["pool.dataset.query"] = lockedAt("tank/main/quack")
	s.collectStatsOnce()
	page, used, _ := allStats(t, s, metricUsedBytes, day0.Add(-time.Hour))
	if v, _ := onDay(used, "2026-11-09", "sub-rey", "main"); v != 67036288+2500 {
		t.Errorf("Rey on main %d, want waddle and his home, quack not read", v)
	}
	var said bool
	for _, g := range page.Gaps {
		said = said || g.Dimensions["share"] == "main" && g.Kind() == kindUnread && strings.Contains(g.Reason(), "quack (locked)")
	}
	if !said {
		t.Errorf("main's gaps %+v, want one naming quack as locked", page.Gaps)
	}
	if _, err := metersOf(s, 3051); err == nil || !strings.Contains(err.Error(), "main") {
		t.Errorf("the live read passed main off as whole with quack locked: %v", err)
	}
	staff := []apiID{{raw: json.RawMessage("51")}}
	places, _, err := placesOf(s, 3051, staff, "/mnt/tank/main/home_dir/rey_mysterio")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range places {
		if p.Folder == "quack" && (p.Usage != nil || p.NotCounted != notCountedLocked) {
			t.Errorf("quack on the card %+v, want not counted, locked", p)
		}
	}
	for _, params := range nas.params {
		if args, ok := params.([]any); ok && len(args) > 0 && args[0] == "tank/main/quack" {
			t.Errorf("a locked dataset's table was read: %v", args)
		}
	}
}

// A locked home answers no mountpoint, so its folder resolved to the dataset
// around it, where its owner has no row: it read 0, as a whole read. It is
// "not counted, locked" on the card, /homes is partial, and the collector
// does not sample it: never 0.
func TestALockedHomeIsNotCountedNotZero(t *testing.T) {
	s, nas, _ := nestedStatsServer(t)
	nas.answers["pool.dataset.query"] = lockedAt("tank/main/home_dir/rey_mysterio")
	home := "/mnt/tank/main/home_dir/rey_mysterio"
	places, _, err := placesOf(s, 3051, nil, home)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, p := range places {
		if p.Home {
			found = true
			if p.Usage != nil || p.NotCounted != notCountedLocked {
				t.Errorf("the locked home on the card %+v, want not counted, locked", p)
			}
		}
	}
	if !found {
		t.Fatalf("no home place: %s", describe(places))
	}
	homes, err := s.readHomes()
	if err != nil {
		t.Fatal(err)
	}
	if homes.QuotaRead {
		t.Errorf("/homes read whole with Rey's home locked: %+v", homes.Homes)
	}
	s.collectStatsOnce()
	_, got, _ := allStats(t, s, metricHomeBytes, day0.Add(-time.Hour))
	if v, ok := onDay(got, "2026-11-09", "sub-rey", ""); ok {
		t.Errorf("the collector sampled a locked home as %d", v)
	}
}

// cardServer is nestedStatsServer with the card's three kinds of place open to
// staff: main's root (a share with datasets inside it), notes (a share that is
// a plain folder in main), and a home.
func cardServer(t *testing.T) (*server, *statsNAS) {
	t.Helper()
	s, nas, _ := nestedStatsServer(t)
	nas.shares = strings.TrimSuffix(statsShares, "]") +
		`,{"name":"notes","path":"/mnt/tank/main/notes","enabled":true,"audit":{"enable":false,"watch_list":[],"ignore_list":[]}}]`
	open := aclJSON(0, ace("GROUP", 4000, "ALLOW", permModify))
	nas.answers["filesystem.getacl /mnt/tank/main"] = open
	nas.answers["filesystem.getacl /mnt/tank/main/notes"] = open
	nas.answers["filesystem.listdir /mnt/tank/main/notes"] = `[]`
	nas.answers["filesystem.listdir /mnt/tank/scratch"] = `[]`
	nas.answers["filesystem.getacl /mnt/tank/scratch"] = aclJSON(0)
	return s, nas
}

// One number per thing, on the member's card too: a place's figure is its
// share's meter and its collected level, a home's is /homes', and a share
// that is a plain folder is never its parent dataset's whole: not counted on
// its own, said so.
func TestTheCardsPlacesAreItsMetersAndHomes(t *testing.T) {
	s, _ := cardServer(t)
	s.collectStatsOnce()
	_, used, _ := allStats(t, s, metricUsedBytes, day0.Add(-time.Hour))
	_, homeLevels, _ := allStats(t, s, metricHomeBytes, day0.Add(-time.Hour))
	homes, err := s.readHomes()
	if err != nil {
		t.Fatal(err)
	}
	home := "/mnt/tank/main/home_dir/rey_mysterio"
	c, err := s.countFor(3051, home)
	if err != nil {
		t.Fatal(err)
	}
	meters, err := c.meters()
	if err != nil {
		t.Fatal(err)
	}
	staff := []apiID{{raw: json.RawMessage("51")}}
	places, _, err := s.memberPlaces(true, staff, home, &c)
	if err != nil {
		t.Fatal(err)
	}
	byShare := map[string]int64{}
	for _, m := range meters {
		byShare[m.Label] = m.Used
	}
	seen := map[string]bool{}
	for _, p := range places {
		switch {
		case p.Home:
			seen["home"] = true
			var want HomeUsage
			for _, h := range homes.Homes {
				if h.Subject == "sub-rey" {
					want = h
				}
			}
			collected, _ := onDay(homeLevels, "2026-11-09", "sub-rey", "")
			if p.Usage == nil || p.Usage.UsedBytes != want.UsedBytes || p.Usage.QuotaBytes != want.QuotaBytes || collected != want.UsedBytes {
				t.Errorf("home: card %+v, /homes %+v, collected %d", p.Usage, want, collected)
			}
		case p.Share == "main" && p.Folder == "":
			seen["main"] = true
			collected, _ := onDay(used, "2026-11-09", "sub-rey", "main")
			if p.Usage == nil || p.Usage.UsedBytes != byShare["main"] || collected != byShare["main"] || collected == 0 {
				t.Errorf("main: card %+v, meter %d, collected %d", p.Usage, byShare["main"], collected)
			}
		case p.Share == "notes":
			seen["notes"] = true
			if p.Usage != nil || p.NotCounted != notCountedFolder {
				t.Errorf("notes on the card %+v, want not counted on its own", p)
			}
			if _, metered := byShare["notes"]; metered {
				t.Errorf("notes has a meter: %v", meters)
			}
		}
	}
	if len(seen) != 3 {
		t.Fatalf("the card's places %s, want main, notes and a home", describe(places))
	}
}

// A share that is a plain folder has no table to read: it never makes a
// member's read partial. Someone who cannot open it gets a whole read and no
// word of it; someone who can sees it on their card, not counted on its own,
// and the read is still whole.
func TestAPlainFolderNeverMakesTheReadPartial(t *testing.T) {
	for _, opens := range []bool{false, true} {
		s, nas := cardServer(t)
		if !opens {
			nas.answers["filesystem.getacl /mnt/tank/main/notes"] = aclJSON(0)
		}
		nas.users = strings.Replace(statsUsers, `"uid":3051,"locked":false,"smb":true,"groups":[]`, `"uid":3051,"locked":false,"smb":true,"groups":[51]`, 1)
		res, _, err := s.storageStatus(OperationRequest{Subject: "sub-rey"})
		if err != nil {
			t.Fatal(err)
		}
		st := res.Result.(*StorageStatus)
		if !st.QuotaRead {
			t.Errorf("opens notes %v: the read is partial (meters %+v)", opens, st.Meters)
		}
		var notes *Place
		for i := range st.Places {
			if st.Places[i].Share == "notes" {
				notes = &st.Places[i]
			}
		}
		switch {
		case !opens && notes != nil:
			t.Errorf("notes is on the card of someone who cannot open it: %+v", *notes)
		case opens && (notes == nil || notes.NotCounted != notCountedFolder):
			t.Errorf("notes on the card of someone who opens it: %+v, want not counted on its own (%s)", notes, describe(st.Places))
		}
	}
}

// A card load reads each table at most once, and never another member's home:
// with a share of N per-user homes it makes the same NAS reads for any N.
func TestACardLoadsReadsDoNotGrowWithTheHomes(t *testing.T) {
	quotaReads := func(n int) int {
		s, nas := cardServer(t)
		datasets := `{"name":"tank/main","mountpoint":"/mnt/tank/main"},{"name":"tank/main/waddle","mountpoint":"/mnt/tank/main/waddle"},` +
			`{"name":"tank/main/quack","mountpoint":"/mnt/tank/main/quack"},{"name":"tank/main/home_dir","mountpoint":"/mnt/tank/main/home_dir"},` +
			`{"name":"tank/open","mountpoint":"/mnt/tank/open"},{"name":"tank/scratch","mountpoint":"/mnt/tank/scratch"}`
		users := []string{`{"id":11,"username":"rey_mysterio","uid":3051,"locked":false,"smb":true,"groups":[51],"home":"/mnt/tank/main/home_dir/rey_mysterio"}`}
		for i := range n {
			name := fmt.Sprintf("jobber_%d", i)
			datasets += `,{"name":"tank/main/home_dir/` + name + `","mountpoint":"/mnt/tank/main/home_dir/` + name + `"}`
			users = append(users, fmt.Sprintf(`{"id":%d,"username":"%s","uid":%d,"locked":false,"smb":true,"groups":[],"home":"/mnt/tank/main/home_dir/%s"}`, 100+i, name, 3100+i, name))
		}
		datasets += `,{"name":"tank/main/home_dir/rey_mysterio","mountpoint":"/mnt/tank/main/home_dir/rey_mysterio"}`
		nas.answers["pool.dataset.query"] = "[" + datasets + "]"
		nas.users = "[" + strings.Join(users, ",") + "]"
		before := len(nas.calls)
		if _, _, err := s.storageStatus(OperationRequest{Subject: "sub-rey"}); err != nil {
			t.Fatal(err)
		}
		reads := 0
		for i, m := range nas.calls[before:] {
			if m == "pool.dataset.get_quota" {
				reads++
				if args := nas.params[before+i].([]any); strings.HasPrefix(args[0].(string), "tank/main/home_dir/jobber") {
					t.Errorf("another member's home was read: %v", args[0])
				}
			}
		}
		return reads
	}
	few, many := quotaReads(2), quotaReads(40)
	// main, waddle, quack, home_dir, Rey's home, open, scratch: each once.
	if few != many || many != 7 {
		t.Errorf("table reads %d with 2 homes, %d with 40: want 7 for any number", few, many)
	}
}

// metersOf is storage.status's meters for uid, a member with no home.
func metersOf(s *server, uid int64) ([]Meter, error) {
	c, err := s.countFor(uid, "")
	if err != nil {
		return nil, err
	}
	return c.meters()
}

// placesOf is storage.status's places for uid, counted as it counts them.
func placesOf(s *server, uid int64, groups []apiID, home string) ([]Place, Observed, error) {
	c, err := s.countFor(uid, home)
	if err != nil {
		return nil, Observed{}, err
	}
	return s.memberPlaces(true, groups, home, &c)
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

// santiago skips 00:00 on 6 Sep 2026 (the clocks go from 23:59:59 -04 to
// 01:00 -03) and has a 25-hour 4 Apr 2026.
var santiago = func() *time.Location { l, _ := time.LoadLocation("America/Santiago"); return l }()

// A day's start is its first instant: 00:00, or 01:00 on a day whose 00:00
// does not exist; the next day is a calendar day, 23 or 25 hours across a
// change. The collector's runs across both changes finish and count the
// day where it is.
func TestADayWhoseMidnightIsSkippedStartsWhenTheClocksResume(t *testing.T) {
	c := statsClock{zone: santiago}
	for day, want := range map[string]string{
		"2026-09-05": "2026-09-05T00:00:00-04:00",
		"2026-09-06": "2026-09-06T01:00:00-03:00",
		"2026-09-07": "2026-09-07T00:00:00-03:00",
		"2026-04-04": "2026-04-04T00:00:00-03:00",
		"2026-04-05": "2026-04-05T00:00:00-04:00",
	} {
		if got := c.stamp(day); got != want {
			t.Errorf("%s starts at %s, want %s", day, got, want)
		}
	}
	if h := c.midnight("2026-04-05").Sub(c.midnight("2026-04-04")); h != 25*time.Hour {
		t.Errorf("4 Apr 2026 in Santiago is %v long, want 25h", h)
	}
	if h := c.midnight("2026-09-07").Sub(c.midnight("2026-09-06")); h != 23*time.Hour {
		t.Errorf("6 Sep 2026 in Santiago is %v long, want 23h", h)
	}

	for _, tc := range []struct {
		name            string
		first, session  time.Time
		day, end, water string
	}{
		{"a skipped midnight", time.Date(2026, 9, 5, 10, 0, 0, 0, santiago), time.Date(2026, 9, 6, 1, 30, 0, 0, santiago),
			"2026-09-06T01:00:00-03:00", "2026-09-07T00:00:00-03:00", "2026-09-08T00:00:00-03:00"},
		{"a 25-hour day", time.Date(2026, 4, 3, 10, 0, 0, 0, santiago), time.Date(2026, 4, 5, 3, 30, 0, 0, time.UTC), // 23:30 -04 on 4 Apr, the second time round
			"2026-04-04T00:00:00-03:00", "2026-04-05T00:00:00-04:00", "2026-04-06T00:00:00-04:00"},
	} {
		s, nas, at := statsServer(t)
		s.stats.zone = santiago
		run := func(now time.Time) {
			*at = now
			done := make(chan struct{})
			go func() { s.collectStatsOnce(); close(done) }()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatalf("%s: a run at %v never finished", tc.name, now)
			}
		}
		run(tc.first)
		nas.connect(tc.session, "rey_mysterio", "main", "dst")
		run(tc.first.AddDate(0, 0, 3))
		for _, m := range []string{metricSessions, metricUsedBytes, metricHomeBytes} {
			if first, _, _ := allStats(t, s, m, tc.first.Add(-time.Hour)); first.Watermark != tc.water {
				t.Errorf("%s: %s's watermark %s, want %s", tc.name, m, first.Watermark, tc.water)
			}
		}
		_, points, _ := allStats(t, s, metricSessions, tc.first.Add(-time.Hour))
		if !slices.ContainsFunc(points, func(p StatPoint) bool {
			return p.Start == tc.day && p.End == tc.end && p.Subject == "sub-rey" && p.Value == 1
		}) {
			t.Errorf("%s: the session is not one day %s..%s: %+v", tc.name, tc.day, tc.end, points)
		}
	}
}

// An outage longer than the NAS keeps its audit log: the days it purged,
// and the day of the oldest row it kept (part-purged, perhaps), are a final
// gap, never zeros; counting goes on from the day after.
func TestAnOutageLongerThanTheLogIsKeptIsAGapNotZeros(t *testing.T) {
	s, nas, at := statsServer(t)
	nas.connect(day0.Add(time.Hour), "rey_mysterio", "main", "before")
	s.collectStatsOnce() // 9 Nov
	setDown := func(down bool) {
		nas.mu.Lock()
		nas.down = down
		nas.mu.Unlock()
	}
	setDown(true)
	for _, d := range []int{3, 6, 7, 9} { // 12, 15, 16 and 18 Nov
		nas.connect(day0.AddDate(0, 0, d), "rey_mysterio", "main", fmt.Sprint("s", d))
	}
	*at = day0.AddDate(0, 0, 5)
	s.collectStatsOnce()

	nas.mu.Lock() // the NAS purges its log before 15 Nov 09:00
	nas.rows = slices.DeleteFunc(nas.rows, func(r map[string]any) bool {
		return r["message_timestamp"].(int64) < time.Date(2026, 11, 15, 9, 0, 0, 0, kolkata).Unix()
	})
	nas.mu.Unlock()
	setDown(false)
	s.nas.mu.Lock()
	s.nas.failures, s.nas.openUntil, s.nas.lastTry = 0, time.Time{}, time.Time{}
	s.nas.mu.Unlock()
	*at = day0.AddDate(0, 0, 10) // 19 Nov
	s.collectStatsOnce()

	first, points, _ := allStats(t, s, metricSessions, day0.Add(-time.Hour))
	want := gapOf("2026-11-09T00:00:00+05:30", "2026-11-16T00:00:00+05:30", nil, reasonPurged)
	if !slices.ContainsFunc(first.Gaps, func(g StatGap) bool { return sameGap(g, want) }) {
		t.Errorf("gaps %+v, want the purged days as %+v", first.Gaps, want)
	}
	if first.Watermark != "2026-11-19T00:00:00+05:30" {
		t.Errorf("watermark %s, want the start of today", first.Watermark)
	}
	for _, p := range points {
		if p.Start < want.End {
			t.Errorf("a point inside the purged window: %+v", p)
		}
	}
	for _, day := range []string{"2026-11-16", "2026-11-18"} {
		if v, _ := onDay(points, day, "sub-rey", "main"); v != 1 {
			t.Errorf("%s counted %d after the boundary, want 1: %+v", day, v, points)
		}
	}
}

// A day with more rows than one run may read is a final gap, and the
// watermark moves past it; it never holds the metric there for ever.
func TestADayTooBusyToCountIsAGapAndTheWatermarkMoves(t *testing.T) {
	page, pages := statsAuditPage, statsAuditPages
	statsAuditPage, statsAuditPages = 2, 1
	t.Cleanup(func() { statsAuditPage, statsAuditPages = page, pages })
	s, nas, at := statsServer(t)
	s.collectStatsOnce() // 9 Nov
	for i := range 3 {
		nas.connect(day0.AddDate(0, 0, 1).Add(time.Duration(i)*time.Minute), "rey_mysterio", "main", fmt.Sprint("busy", i))
	}
	nas.connect(day0.AddDate(0, 0, 2), "rey_mysterio", "main", "quiet")
	*at = day0.AddDate(0, 0, 2) // 11 Nov
	for range 3 {
		s.collectStatsOnce()
	}
	first, points, _ := allStats(t, s, metricSessions, day0.Add(-time.Hour))
	want := gapOf("2026-11-10T00:00:00+05:30", "2026-11-11T00:00:00+05:30", nil, reasonOverfull)
	if first.Watermark != "2026-11-11T00:00:00+05:30" || !slices.ContainsFunc(first.Gaps, func(g StatGap) bool { return sameGap(g, want) }) {
		t.Errorf("watermark %s, gaps %+v; want 11 Nov and %+v", first.Watermark, first.Gaps, want)
	}
	if v, _ := onDay(points, "2026-11-11", "sub-rey", "main"); v != 1 {
		t.Errorf("the day after the busy one counted %d, want 1: %+v", v, points)
	}
}

// A level's unbound accounts are the last run's: an account whose usage is
// gone by the day's last sample is not unbound that day.
func TestALevelsUnboundIsTheLastSamples(t *testing.T) {
	s, nas, _ := statsServer(t)
	s.collectStatsOnce()
	for _, m := range []string{metricHomeBytes, metricUsedBytes} {
		if first, _, _ := allStats(t, s, m, day0.Add(-time.Hour)); first.Unbound.Accounts != 1 {
			t.Fatalf("%s: unbound.accounts %d, want 1 (Seth)", m, first.Unbound.Accounts)
		}
	}
	nas.fakeRPC.answers["pool.dataset.get_quota tank/main"] = `[{"id":0,"used_bytes":7000},{"id":3051,"used_bytes":1200}]`
	nas.fakeRPC.answers["pool.dataset.get_quota tank/open"] = `[]`
	nas.fakeRPC.answers["pool.dataset.get_quota tank/main/home_dir"] = `[{"id":3051,"used_bytes":900,"quota":5000}]`
	s.collectStatsOnce()
	for _, m := range []string{metricHomeBytes, metricUsedBytes} {
		if first, _, _ := allStats(t, s, m, day0.Add(-time.Hour)); first.Unbound.Accounts != 0 {
			t.Errorf("%s: unbound.accounts %d after Seth's usage went, want 0", m, first.Unbound.Accounts)
		}
	}
}

// The TZ refused at startup is what the manifest's stats_zone refuses.
func TestTheZonePatternIsTheSchemas(t *testing.T) {
	schema := v2JSON(t, "schema", "manifest.json")["properties"].(map[string]any)["stats_zone"].(map[string]any)["pattern"]
	if schema != statsZonePattern.String() {
		t.Errorf("statsZonePattern %s, the schema's stats_zone pattern %v", statsZonePattern, schema)
	}
	for _, legacy := range []string{"Singapore", "EST5EDT", "Local"} {
		if _, err := loadZone(legacy); err == nil {
			t.Errorf("TZ=%s, which tzdb links but stats_zone refuses, was accepted", legacy)
		}
	}
	for _, ok := range []string{"UTC", "Etc/GMT+5", "America/Argentina/Buenos_Aires"} {
		if _, err := loadZone(ok); err != nil {
			t.Errorf("TZ=%s refused: %v", ok, err)
		}
	}
}

// Days before the first collection were never counted, and the reason says so (with the day)
// rather than claiming the add-on lost them; once the buffer has trimmed past it, they were lost.
// Every metric, including the levels.
func TestADayBeforeCountingBeganIsNotCollectedYet(t *testing.T) {
	for _, metric := range []string{metricSessions, metricUsedBytes, metricHomeBytes} {
		t.Run(metric, func(t *testing.T) {
			s, _, at := statsServer(t)
			s.collectStatsOnce()
			first, _, _ := allStats(t, s, metric, day0.AddDate(0, 0, -3))
			want := s.stats.reasonBegan(s.stats.day(day0))
			if !strings.HasPrefix(want, "Not collected yet: counting began on ") {
				t.Fatalf("reason %q", want)
			}
			if len(first.Gaps) == 0 || first.Gaps[0].Reason() != want || first.Gaps[0].Kind() != kindBegan {
				t.Errorf("gaps %+v, want the first to say %q", first.Gaps, want)
			}
			// An add-on older than its buffer: the window splits at Began. Before it, never held,
			// still began (the record's start is not lost); from it to what is held, dropped.
			*at = day0.AddDate(0, 0, 95)
			s.collectStatsOnce()
			later, _, _ := allStats(t, s, metric, day0.AddDate(0, 0, -3))
			held := metaOf(t, s, metric).HeldFrom
			if len(later.Gaps) < 2 ||
				later.Gaps[0].Kind() != kindBegan || !strings.HasPrefix(later.Gaps[0].Reason(), want) || later.Gaps[0].End != s.stats.stamp(s.stats.day(day0)) ||
				later.Gaps[1].Kind() != kindDropped || later.Gaps[1].Reason() != reasonBuffer.text || later.Gaps[1].Start != s.stats.stamp(s.stats.day(day0)) || later.Gaps[1].End != s.stats.stamp(held) {
				t.Errorf("after the buffer trimmed past Began, gaps %+v: want began up to %s, then dropped to %s", later.Gaps, s.stats.day(day0), held)
			}
			// Asked from after Began, there is no began window, only what was dropped.
			after, _, _ := allStats(t, s, metric, day0.AddDate(0, 0, 2))
			if len(after.Gaps) == 0 || after.Gaps[0].Kind() != kindDropped {
				t.Errorf("asked from after Began, gaps %+v: want the dropped window alone", after.Gaps)
			}
		})
	}
}

// Every declared reason has a kind the contract knows (the compiler holds that each has one: a
// gapReason is written positionally, sentence and kind; this holds that the kind is a real one).
func TestEveryGapReasonHasAKind(t *testing.T) {
	for _, r := range gapReasons {
		if r.text == "" || !slices.Contains([]gapKind{kindUnread, kindDropped, kindUnobserved}, r.kind) {
			t.Errorf("reason %q has kind %q", r.text, r.kind)
		}
	}
	// A gap made without gapOf has no reason and no kind, and is refused on the wire, never sent.
	if _, err := json.Marshal(StatGap{Start: "a", End: "b"}); err == nil {
		t.Error("a gap with no kind was served")
	}
}

// Every gap a reply carries, one stored before kinds existed included, validates against stats.json
// (kind required).
func TestEveryGapGoesOutWithItsKind(t *testing.T) {
	s, _, _ := statsServer(t)
	s.collectStatsOnce()
	// A day stored by an older binary: its gap has no kind.
	var legacy statDay
	if err := json.Unmarshal(fmt.Appendf(nil, `{"gaps":[{"start":%q,"end":%q,"dimensions":{"share":"scratch"},"reason":%q}]}`, s.stats.stamp(s.stats.day(day0)), s.stats.stamp(nextDayKey(s.stats.day(day0))), reasonAudited.text), &legacy); err != nil {
		t.Fatal(err)
	}
	if err := s.store.PutStats(metricSessions, metaOf(t, s, metricSessions), map[string]statDay{s.stats.day(day0): legacy}, s.stats.oldest(day0)); err != nil {
		t.Fatal(err)
	}
	rr, page := getStats(t, s, metricSessions, day0.AddDate(0, 0, -3), "")
	var doc any
	if err := json.Unmarshal(rr.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if err := v2Validate(t, "stats.json", doc); err != nil {
		t.Fatalf("the reply is not stats.json: %v", err)
	}
	kinds := map[string]gapKind{}
	for _, g := range page.Gaps {
		kinds[g.Reason()] = g.Kind()
	}
	if kinds[reasonAudited.text] != kindUnobserved || kinds[s.stats.reasonBegan(s.stats.day(day0))] != kindBegan {
		t.Errorf("gaps by reason %v: want the legacy one unobserved and the first collection's began", kinds)
	}
}

// putMeta writes a metric's meta as stored, bypassing PutStats: a state.db from an older binary.
func putMeta(t *testing.T, s *server, metric string, meta statsMeta) {
	t.Helper()
	raw, _ := json.Marshal(meta)
	if err := s.store.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketStats).Bucket([]byte(metric)).Put(statsMetaKey, raw)
	}); err != nil {
		t.Fatal(err)
	}
}

func metaOf(t *testing.T, s *server, metric string) statsMeta {
	t.Helper()
	m, _, err := s.store.StatsMeta(metric)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// Began is the exact signal: the first collection that succeeded (the stored watermark was empty and
// this run's is not). A failed first run decides nothing, and once set it never changes.
func TestBeganIsTheFirstSuccessfulCollectionAndNeverChanges(t *testing.T) {
	for _, metric := range []string{metricSessions, metricUsedBytes, metricHomeBytes} {
		t.Run(metric, func(t *testing.T) {
			s, nas, at := statsServer(t)
			nas.down = true
			s.collectStatsOnce()
			if m := metaOf(t, s, metric); m.Began != "" {
				t.Fatalf("a failed first run is not the first collection, Began %q", m.Began)
			}
			nas.down = false
			s.nas.lastTry = time.Time{} // the connection's reconnect cooldown is over
			*at = day0.AddDate(0, 0, 1)
			s.collectStatsOnce()
			first := s.stats.day(*at)
			if m := metaOf(t, s, metric); m.Began != first || m.Version != statsMetaVersion {
				t.Fatalf("Began %q version %d, want %q the first successful run, version %d", m.Began, m.Version, first, statsMetaVersion)
			}
			for _, move := range []time.Duration{24 * time.Hour, -48 * time.Hour, 24 * 100 * time.Hour} {
				*at = at.Add(move)
				s.collectStatsOnce()
				if m := metaOf(t, s, metric); m.Began != first {
					t.Fatalf("after moving the clock %v, Began %q, want it to stay %q", move, m.Began, first)
				}
			}
		})
	}
}

// State from before Began existed (version 0) is judged once, by the one rule left to it, and then
// never again: a HeldFrom after the oldest day the buffer could hold has never trimmed, so it is the day
// counting began; at or before it the buffer may have trimmed, and Began stays empty for good.
func TestStateWithoutBeganIsMigratedOnce(t *testing.T) {
	for _, metric := range []string{metricSessions, metricUsedBytes, metricHomeBytes} {
		for _, trimmed := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/trimmed=%v", metric, trimmed), func(t *testing.T) {
				s, _, at := statsServer(t)
				s.collectStatsOnce()
				meta := metaOf(t, s, metric)
				meta.Began, meta.Version = "", 0
				if trimmed {
					meta.HeldFrom = s.stats.oldest(*at)
				}
				putMeta(t, s, metric, meta)
				*at = at.Add(time.Hour)
				s.collectStatsOnce()
				got, _, _ := allStats(t, s, metric, day0.AddDate(0, 0, -200))
				want := s.stats.reasonBegan(s.stats.day(day0))
				if trimmed {
					want = reasonBuffer.text
				}
				if len(got.Gaps) == 0 || got.Gaps[0].Reason() != want {
					t.Errorf("gaps %+v, want the first to say %q", got.Gaps, want)
				}
				decided := metaOf(t, s, metric)
				if decided.Version != statsMetaVersion || (decided.Began != "") == trimmed {
					t.Fatalf("migrated meta %+v", decided)
				}
				// Never judged again: the clock moving back, so the old rule would now read
				// the same HeldFrom the other way, changes nothing.
				*at = day0.AddDate(0, 0, -30)
				s.collectStatsOnce()
				*at = day0.AddDate(0, 0, 1)
				s.collectStatsOnce()
				if again := metaOf(t, s, metric); again.Began != decided.Began {
					t.Errorf("Began %q after the clock moved, was decided %q", again.Began, decided.Began)
				}
			})
		}
	}
}

// A version-0 meta that never collected (the first runs all failed) has nothing to migrate: its first
// successful collection is the first collection, and Began is set then, not frozen empty.
func TestAVersionZeroMetaThatNeverCollectedBeginsAtItsFirstCollection(t *testing.T) {
	for _, metric := range []string{metricSessions, metricUsedBytes, metricHomeBytes} {
		t.Run(metric, func(t *testing.T) {
			s, nas, at := statsServer(t)
			nas.down = true
			s.collectStatsOnce()
			meta := metaOf(t, s, metric)
			meta.Version, meta.Began = 0, ""
			putMeta(t, s, metric, meta)
			nas.down = false
			s.nas.lastTry = time.Time{}
			*at = day0.AddDate(0, 0, 1)
			s.collectStatsOnce()
			if m := metaOf(t, s, metric); m.Began != s.stats.day(*at) || m.Version != statsMetaVersion {
				t.Fatalf("Began %q version %d, want the first collection %q", m.Began, m.Version, s.stats.day(*at))
			}
		})
	}
}
