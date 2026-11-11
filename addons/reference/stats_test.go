package reference

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

// The stats half's rules a black-box kit cannot hold: the app taken offline,
// the clock moved, the buffer filled past its ceiling.

func statsAddon(t *testing.T, down func() bool, at *time.Time) *Addon {
	t.Helper()
	a, err := New(Config{Target: "reference", Secret: []byte("s"), Features: []string{"stats"}, Down: down})
	if err != nil {
		t.Fatal(err)
	}
	a.stats.now = func() time.Time { return *at }
	return a
}

type statsPage struct {
	Watermark string                 `json:"watermark"`
	Points    []point                `json:"points"`
	Gaps      []gap                  `json:"gaps"`
	Unbound   struct{ Accounts int } `json:"unbound"`
	Next      *string                `json:"next_cursor"`
}

func readStats(t *testing.T, a *Addon, metric string, since time.Time, cursor string) (int, statsPage) {
	t.Helper()
	q := url.Values{"metric": {metric}, "since": {since.UTC().Format(time.RFC3339)}}
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	rep := a.statsRead(httptest.NewRequest(http.MethodGet, "/stats?"+q.Encode(), nil), nil)
	var p statsPage
	if rep.status == 200 {
		if err := json.Unmarshal(rep.body, &p); err != nil {
			t.Fatal(err)
		}
	}
	return rep.status, p
}

// readAll follows the cursor to the end.
func readAll(t *testing.T, a *Addon, metric string, since time.Time) (statsPage, []point) {
	t.Helper()
	_, first := readStats(t, a, metric, since, "")
	all, page := first.Points, first
	for page.Next != nil {
		_, page = readStats(t, a, metric, since, *page.Next)
		all = append(all, page.Points...)
	}
	return first, all
}

var day1 = time.Date(2026, 11, 9, 10, 0, 0, 0, statsZone)

// While the app cannot be read nothing moves: the watermark stays at the last
// day read, the days since are one provisional gap, and once the app answers
// again they are read from its log, the gap withdrawn, the watermark moved on.
func TestTheWatermarkIsHeldWhileTheAppIsDown(t *testing.T) {
	var down atomic.Bool
	at := day1
	a := statsAddon(t, down.Load, &at)
	since := day1.AddDate(0, 0, -3)
	first, _ := readAll(t, a, "machine.jobs", since)
	if first.Watermark != "2026-11-09T00:00:00+05:30" {
		t.Fatalf("watermark %s, want the start of the day read", first.Watermark)
	}

	down.Store(true)
	at = day1.AddDate(0, 0, 2)
	dark, points := readAll(t, a, "machine.jobs", since)
	if dark.Watermark != first.Watermark {
		t.Errorf("with the app down the watermark moved to %s", dark.Watermark)
	}
	want := gap{Start: "2026-11-10T00:00:00+05:30", End: "2026-11-12T00:00:00+05:30", Reason: failureReason}
	if n := len(dark.Gaps); n == 0 || !equal(dark.Gaps[n-1], want) {
		t.Errorf("gaps %+v, want the days since the last read as one gap %+v", dark.Gaps, want)
	}
	for _, p := range points {
		if p.Start >= want.Start {
			t.Errorf("a point inside the unread days: %+v", p)
		}
	}

	down.Store(false)
	back, points := readAll(t, a, "machine.jobs", since)
	if back.Watermark != "2026-11-11T00:00:00+05:30" {
		t.Errorf("back up, watermark %s, want the start of today", back.Watermark)
	}
	for _, g := range back.Gaps {
		if g.Reason == failureReason {
			t.Errorf("the failure gap was not withdrawn: %+v", g)
		}
	}
	filled := false
	for _, p := range points {
		filled = filled || p.Start == "2026-11-10T00:00:00+05:30"
	}
	if !filled {
		t.Error("the day the app was down was not filled from its log")
	}
}

// A day's gaps join into one window, but never across the watermark: a final
// gap keeps its end when the days after it are added.
func TestGapsJoinButNeverAcrossTheWatermark(t *testing.T) {
	dims := map[string]string{"printer": "xl-1"}
	day := func(d int) string { return stamp3339(time.Date(2026, 11, d, 0, 0, 0, 0, statsZone)) }
	in := []gap{{day(1), day(2), dims, offReason}, {day(2), day(3), dims, offReason}, {day(3), day(4), dims, offReason}, {day(4), day(5), nil, offReason}}
	got := mergeGaps(in, day(3))
	want := []gap{{day(1), day(3), dims, offReason}, {day(3), day(4), dims, offReason}, {day(4), day(5), nil, offReason}}
	if !equal(got, want) {
		t.Errorf("merged %+v, want %+v", got, want)
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

// Past the size ceiling the oldest days go first, and a read from before the
// oldest day left is answered with a gap for what was dropped, then the points
// it still holds; every page fits, and the pages follow one another with no
// point twice and none left out.
func TestTheBufferDropsTheOldestAndSaysSo(t *testing.T) {
	at := day1
	a := statsAddon(t, nil, &at)
	for i := range 60 { // enough people that a month of them is over statsMaxPoints
		id := strconv.Itoa(2000 + i)
		a.accounts[id] = &account{id: id, username: "member-" + id, state: map[string]any{}}
		a.bindings["sub-"+id] = id
	}
	a.mu.Lock()
	a.collect(at)
	for id := range a.accounts { // their log starts 40 days back

		a.stats.seen[id] = dayOf(at.AddDate(0, 0, -40))
	}
	a.stats.watermark = time.Time{}
	a.collect(at)
	held, total := a.stats.heldFrom, 0
	for _, m := range a.stats.buf {
		for _, d := range m {
			total += len(d.points)
		}
	}
	a.mu.Unlock()
	if total > statsMaxPoints || !held.After(dayOf(at.AddDate(0, 0, -statsBufferDays))) {
		t.Fatalf("buffer holds %d points from %v; want at most %d, and fewer days than the buffer's length", total, held, statsMaxPoints)
	}

	since := at.AddDate(0, 0, -statsBufferDays)
	first, all := readAll(t, a, "machine.jobs", since)
	if len(first.Gaps) == 0 || !equal(first.Gaps[0], gap{Start: stamp3339(dayOf(since)), End: stamp3339(held), Reason: bufferReason}) {
		t.Errorf("first gap %+v, want the dropped days [%s, %s) with the buffer's reason", first.Gaps, stamp3339(dayOf(since)), stamp3339(held))
	}
	want := 0
	a.mu.Lock()
	for d, rec := range a.stats.buf["machine.jobs"] {
		if !d.Before(dayOf(since)) {
			want += len(rec.points)
		}
	}
	a.mu.Unlock()
	if len(all) != want || want == 0 {
		t.Errorf("the pages carried %d points, the buffer holds %d", len(all), want)
	}
	for i := 1; i < len(all); i++ {
		if compareKeys(keyOf(all[i-1]), keyOf(all[i])) >= 0 {
			t.Fatalf("point %d does not follow point %d in keyset order: %+v then %+v", i, i-1, all[i-1], all[i])
		}
	}
}
