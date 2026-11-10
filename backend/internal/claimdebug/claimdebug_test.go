package claimdebug

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *clock) add(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func newTest(t *testing.T) (*Recorder, *clock, *[]Session) {
	t.Helper()
	c := &clock{t: time.Date(2026, 11, 3, 10, 6, 0, 0, time.UTC)}
	var ended []Session
	r := New(c.now, func(context.Context, Capture) any { return "checks" }, func(s Session) { ended = append(ended, s) })
	return r, c, &ended
}

// add is Record without the queue, so a test sees the entry at once.
func capture(r *Recorder, c *clock, user string) {
	r.add(Capture{At: c.now(), ClientID: "cNB", UserID: user, Request: []byte(`{"user":{"id":"` + user + `"}}`), Response: []byte(`{"append_claims":[]}`)})
}

// One account at a time, filtered before anything is held; a switch keeps
// the end; the end is enforced server-side and recorded once.
func TestRecorder_OneAccountFifteenMinutes(t *testing.T) {
	r, c, ended := newTest(t)
	if r.Wants("cNB", "charlotte") {
		t.Fatal("off by default")
	}
	s, switched := r.Start("aNB", "cNB", "charlotte", "kanishk")
	if switched || !s.EndsAt.Equal(c.now().Add(Window)) {
		t.Fatalf("start: %+v switched=%v", s, switched)
	}
	if !r.Wants("cNB", "charlotte") || r.Wants("cNB", "sami") || r.Wants("cOther", "charlotte") {
		t.Fatal("only Charlotte's sign-ins to NetBird")
	}
	capture(r, c, "charlotte")
	capture(r, c, "sami") // never held

	c.add(5 * time.Minute)
	s2, switched := r.Start("aNB", "cNB", "roman", "kanishk")
	if !switched || !s2.EndsAt.Equal(s.EndsAt) || s2.UserID != "roman" {
		t.Fatalf("a switch moves the capture and never the end: %+v", s2)
	}
	if r.Wants("cNB", "charlotte") || !r.Wants("cNB", "roman") {
		t.Fatal("after the switch only Roman")
	}
	capture(r, c, "roman")

	list := r.SignIns("aNB", 0)
	if len(list) != 2 || list[0].UserID != "roman" || list[1].UserID != "charlotte" || list[0].Checks != "checks" {
		t.Fatalf("newest first, both people, checks kept: %+v", list)
	}
	if after := r.SignIns("aNB", list[0].Seq); len(after) != 0 {
		t.Fatalf("after= the newest seq is empty: %+v", after)
	}

	c.add(10 * time.Minute)
	if r.Wants("cNB", "roman") {
		t.Fatal("15 minutes is enforced here, not by the page")
	}
	r.Sweep()
	r.Sweep()
	if len(*ended) != 1 || (*ended)[0].EndedBy != "system" || !(*ended)[0].EndedAt.Equal(s.EndsAt) {
		t.Fatalf("ended by itself, recorded once: %+v", *ended)
	}
	if _, ok := r.Stop("aNB", "kanishk"); ok {
		t.Fatal("nothing to stop once ended")
	}
	if got := r.SignIns("aNB", 0); len(got) != 2 {
		t.Fatal("what it saw stays readable after the end")
	}

	c.add(Keep)
	if got := r.SignIns("aNB", 0); len(got) != 0 {
		t.Fatalf("forgotten after an hour: %+v", got)
	}
}

func TestRecorder_StopAndCap(t *testing.T) {
	r, c, ended := newTest(t)
	r.Start("aNB", "cNB", "charlotte", "kanishk")
	for i := 0; i < Cap+5; i++ {
		capture(r, c, "charlotte")
	}
	if got := r.SignIns("aNB", 0); len(got) != Cap || got[0].Seq != int64(Cap+5) {
		t.Fatalf("cap %d, newest kept: len=%d", Cap, len(got))
	}
	s, ok := r.Stop("aNB", "kanishk")
	if !ok || s.EndedBy != "kanishk" || r.Wants("cNB", "charlotte") || len(*ended) != 0 {
		t.Fatalf("stopped by hand: %+v", s)
	}
	// Turn on again: a fresh 15 minutes.
	c.add(time.Minute)
	s2, switched := r.Start("aNB", "cNB", "charlotte", "kanishk")
	if switched || !s2.EndsAt.Equal(c.now().Add(Window)) {
		t.Fatalf("on again: %+v", s2)
	}
}

// Record never blocks the data plane, even with nobody draining the queue.
func TestRecord_NeverBlocks(t *testing.T) {
	r := &Recorder{queue: make(chan Capture, 1)}
	done := make(chan struct{})
	go func() {
		for i := 0; i < 10; i++ {
			r.Record(Capture{})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Record blocked")
	}
	var nilRec *Recorder
	if nilRec.Wants("c", "u") {
		t.Fatal("a nil recorder wants nothing")
	}
}

// explain reads Zitadel and Postgres off the latency path; a panic there loses
// that capture, never the worker or the process (assessment S13).
func TestRecorder_PanicInExplainIsSurvived(t *testing.T) {
	c := &clock{t: time.Date(2026, 11, 3, 10, 6, 0, 0, time.UTC)}
	var calls atomic.Int32
	r := New(c.now, func(context.Context, Capture) any {
		if calls.Add(1) == 1 {
			panic("explain bug")
		}
		return "checks"
	}, nil)
	r.Start("aNB", "cNB", "charlotte", "kanishk")
	r.Record(Capture{At: c.now(), ClientID: "cNB", UserID: "charlotte"})
	r.Record(Capture{At: c.now(), ClientID: "cNB", UserID: "charlotte"})
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && len(r.SignIns("aNB", 0)) == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	if got := r.SignIns("aNB", 0); len(got) != 1 {
		t.Fatalf("after a panicking capture the worker filed %d, want the second one", len(got))
	}
}
