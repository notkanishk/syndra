package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// A busy NAS, or a busy queue of the add-on's own, is never an outage; only
// silence is (8g fix round 2, 22 Nov 2026).

// pagingRPC is a NAS that is busy and answering: audit.query takes the time a
// NAS paging statsAuditRowsPerSecond takes over the page asked for, and
// answers it full until stop closes; every other call is passed on.
type pagingRPC struct {
	rpc
	stop chan struct{}
}

func (p pagingRPC) Call(method string, timeoutSeconds int64, params any) (json.RawMessage, error) {
	if method != "audit.query" {
		return p.rpc.Call(method, timeoutSeconds, params)
	}
	opts := params.([]any)[0].(map[string]any)["query-options"].(map[string]any)
	limit, _ := opts["limit"].(int)
	took := min(time.Duration(limit)*time.Second/statsAuditRowsPerSecond, time.Duration(timeoutSeconds)*time.Second)
	select {
	case <-p.stop:
		return json.RawMessage(`{"jsonrpc":"2.0","id":1,"result":[]}`), nil
	case <-time.After(took):
	}
	rows := make([]string, limit)
	for i := range rows {
		rows[i] = fmt.Sprintf(`{"message_timestamp":%d,"event":"CONNECT","success":true}`, time.Now().Unix())
	}
	return json.RawMessage(`{"jsonrpc":"2.0","id":1,"result":[` + strings.Join(rows, ",") + `]}`), nil
}

// Through a long collector run on a NAS that is slow and answering, /health
// stays reachable: each audit.query page is short (statsCallTarget), so the
// ping queued behind one waits one page, and each page's answer keeps the last
// read recent. With 1000-row pages one call held the session past
// recentReadCounts and /health read the NAS unreachable (the 8g staging run).
// The backend's monitor counts a /health with reachable false as
// app_unreachable, so reachable throughout is the monitor never seeing it.
func TestHealthStaysReachableThroughALongCollectorRun(t *testing.T) {
	t.Parallel()
	if statsCallTarget > recentReadCounts/2 {
		t.Fatalf("a collector call's target %s leaves no margin under the %s a recent answer counts", statsCallTarget, recentReadCounts)
	}
	s, nas, _ := statsServer(t)
	stop := make(chan struct{})
	paging := pagingRPC{rpc: nas, stop: stop}
	s.nas.dial = func() (rpc, error) { return paging, nil }
	s.nas.client, s.nas.lastRead = paging, time.Now()
	done := make(chan struct{})
	go func() { s.collectStatsOnce(context.Background()); close(done) }()
	t.Cleanup(func() { close(stop); <-done })

	for end := time.Now().Add(recentReadCounts + 3*time.Second); time.Now().Before(end); time.Sleep(time.Second) {
		if h := healthInTime(t, s); !h.Reachable {
			t.Fatalf("/health read the NAS unreachable %s into the collector's run, its last answer %s old",
				time.Until(end)+recentReadCounts+3*time.Second, time.Since(s.nas.LastRead()).Round(time.Millisecond))
		}
	}
}

// problemOf is a reply's status, code and Retry-After.
func problemOf(rr *httptest.ResponseRecorder) string {
	var p struct{ Code string }
	_ = json.Unmarshal(rr.Body.Bytes(), &p)
	return fmt.Sprintf("%d %s %s", rr.Code, p.Code, rr.Header().Get("Retry-After"))
}

// A read or a write that gave up waiting in the add-on's own queue, or a read
// that ran out on a NAS that still answers its ping, answers 503 TARGET_BUSY
// with Retry-After: nothing sent, never an outage.
func TestABusyNASAnswersTargetBusy(t *testing.T) {
	want := fmt.Sprintf("503 %s %d", CodeTargetBusy, busyRetryAfter)
	for name, tc := range map[string]struct {
		rig  func(*testing.T, *server, chan struct{}) string
		send func(*testing.T, *server, string) *httptest.ResponseRecorder
	}{
		"a read waiting in line": {
			rig: holdTheLine,
			send: func(t *testing.T, s *server, _ string) *httptest.ResponseRecorder {
				return postWithin(t, s, "health.get", `{"call_id":"c1","subject":"-"}`)
			},
		},
		"a read on a busy NAS": {
			rig: func(_ *testing.T, s *server, _ chan struct{}) string {
				s.nas.callWithin = time.Second
				s.nas.client, s.nas.probed, s.nas.lastRead = busyRPC{slow: "*"}, true, time.Now()
				return ""
			},
			send: func(t *testing.T, s *server, _ string) *httptest.ResponseRecorder {
				return postWithin(t, s, "health.get", `{"call_id":"c1","subject":"-"}`)
			},
		},
		"an apply waiting in line": {
			rig: func(t *testing.T, s *server, release chan struct{}) string {
				// Fingerprinted before the line is taken: the plan reads the NAS.
				body := withContractVersion(t, withFingerprint(t, s,
					`{"call_id":"c1","subject":"sub-1","email":"charlotte@example.org","desired":{"enabled":false}}`))
				return holdTheLine(t, s, release) + body
			},
			send: func(t *testing.T, s *server, body string) *httptest.ResponseRecorder {
				rr := httptest.NewRecorder()
				s.withBudget(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					s.handleApply(w, r, []byte(body))
				})).ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/apply", strings.NewReader(body)))
				return rr
			},
		},
		"a write waiting in line": {
			rig: holdTheLine,
			send: func(t *testing.T, s *server, _ string) *httptest.ResponseRecorder {
				return postWithin(t, s, "password.set", `{"call_id":"c1","subject":"sub-1","actor":"sub-1","params":{"password":"`+theSecret+`"}}`)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s, _ := opServer(t)
			release := make(chan struct{})
			t.Cleanup(func() { close(release) })
			if got := problemOf(tc.send(t, s, tc.rig(t, s, release))); got != want {
				t.Fatalf("answered %s; want %s", got, want)
			}
			if !s.nas.lastTry.IsZero() || s.nas.failures != 0 {
				t.Errorf("a busy NAS armed the floor (%v) or counted a failure (%d)", s.nas.lastTry, s.nas.failures)
			}
		})
	}
}

// holdTheLine gives the wire to a long call (the collector's page) until
// release, on a live session, and gives the request 1 s.
func holdTheLine(_ *testing.T, s *server, release chan struct{}) string {
	s.budget = time.Second
	s.nas.client, s.nas.probed, s.nas.lastRead = busyRPC{}, true, time.Now()
	s.nas.line <- struct{}{}
	go func() { <-release; s.nas.leave() }()
	return ""
}

// A kept copy served because the NAS was busy says so: stale.reason busy,
// with the wait, never unreachable.
func TestACopyServedWhileBusySaysBusy(t *testing.T) {
	f := &fresher{cooldown: time.Nanosecond}
	if _, _, err := f.read("k", true, func() (any, time.Time, error) { return 1, time.Now(), nil }); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond)
	for cause, want := range map[error]string{
		fmt.Errorf("%w: %w: in line", ErrNotSent, ErrBusy): "busy",
		ErrNotSent: "unreachable",
	} {
		_, obs, err := f.read("k", true, func() (any, time.Time, error) { return nil, time.Time{}, cause })
		if err != nil || obs.Stale == nil || obs.Stale.Reason != want {
			t.Errorf("a copy served after %q said %+v (%v); want stale %s", cause, obs.Stale, err, want)
		}
		time.Sleep(time.Millisecond)
	}
}

// A dial with less than its whole bound left of the request is not tried: no
// dial, no failure counted, no floor armed, and the answer is busy. Only a
// dial that really failed arms the floor; one that succeeded does not.
func TestADialWithTooLittleTimeLeftIsNotTried(t *testing.T) {
	var dials atomic.Int32
	fail := atomic.Bool{}
	n := newNAS(func() (rpc, error) {
		dials.Add(1)
		if fail.Load() {
			return nil, errors.New("dial tcp: connection refused")
		}
		return busyRPC{}, nil
	}, []string{"25.04"})
	n.dialWithin = time.Second
	short, cancel := context.WithTimeout(t.Context(), n.dialWithin/2)
	defer cancel()
	if _, err := n.session(short); !errors.Is(err, ErrBusy) || !errors.Is(err, ErrNotSent) {
		t.Fatalf("a dial with too little time left answered %v; want busy, nothing sent", err)
	}
	if dials.Load() != 0 || n.failures != 0 || !n.lastTry.IsZero() {
		t.Fatalf("not tried, yet dialled %d, counted %d failures, armed the floor %v", dials.Load(), n.failures, n.lastTry)
	}
	if _, err := n.session(t.Context()); err != nil || !n.lastTry.IsZero() {
		t.Fatalf("a dial that succeeded answered %v and armed the floor %v", err, n.lastTry)
	}
	n.client, fail = nil, atomic.Bool{}
	fail.Store(true)
	if _, err := n.session(t.Context()); errors.Is(err, ErrBusy) || n.lastTry.IsZero() || n.failures != 1 {
		t.Fatalf("a dial that failed answered %v, armed the floor %v, counted %d", err, n.lastTry, n.failures)
	}
}

// pingCount counts core.ping on a live session.
type pingCount struct{ pings *atomic.Int32 }

func (p pingCount) Call(method string, _ int64, _ any) (json.RawMessage, error) {
	if method == "core.ping" {
		p.pings.Add(1)
	}
	return json.RawMessage(`{"jsonrpc":"2.0","id":1,"result":"pong"}`), nil
}
func (pingCount) Close() error { return nil }

// An idle session's proof with less than the ping's whole bound left is not
// asked: nothing learned, so the session is kept, nothing is lost, and the
// answer is busy.
func TestAPingWithTooLittleTimeLeftIsNotAsked(t *testing.T) {
	pings := new(atomic.Int32)
	c := pingCount{pings}
	n := newNAS(func() (rpc, error) { return nil, errors.New("unused") }, []string{"25.04"})
	n.pingWithin = 3 * time.Second // half of it is a whole second: a ping cut short could be sent
	n.client, n.probed, n.lastRead = c, true, time.Now().Add(-2*idleProbeAfter)
	short, cancel := context.WithTimeout(t.Context(), n.pingWithin/2)
	defer cancel()
	if _, err := n.session(short); !errors.Is(err, ErrBusy) || !errors.Is(err, ErrNotSent) {
		t.Fatalf("a proof with too little time left answered %v; want busy, nothing sent", err)
	}
	if pings.Load() != 0 || n.client != c || !n.lastTry.IsZero() {
		t.Fatalf("not asked, yet pinged %d times, kept the session %v, armed the floor %v", pings.Load(), n.client == c, n.lastTry)
	}
	if got, err := n.session(t.Context()); err != nil || got != c || pings.Load() != 1 {
		t.Fatalf("with time to ask, the proof answered %v (pings %d)", err, pings.Load())
	}
}

// A context that has ended has no time to ask, whatever its deadline: nothing
// is sent on it.
func TestAnEndedContextSendsNothing(t *testing.T) {
	ended, cancel := context.WithTimeout(t.Context(), time.Hour)
	cancel()
	if got := seconds(ended, time.Minute); got != 0 {
		t.Fatalf("seconds on an ended context is %d; want 0", got)
	}
	pings := new(atomic.Int32)
	n := newNAS(nil, []string{"25.04"})
	if _, err := n.invoke(ended, pingCount{pings}, "core.ping", []any{}); !errors.Is(err, ErrNotSent) || pings.Load() != 0 {
		t.Fatalf("a call on an ended context answered %v and sent %d", err, pings.Load())
	}
}

// dialBounded gives up at ctx's deadline when it is sooner than its own
// bound, and closes the session a late dial opens.
func TestADialEndsAtTheRequestsDeadline(t *testing.T) {
	release := make(chan struct{})
	closed := new(atomic.Bool)
	short, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	began := time.Now()
	_, err := dialBounded(short, func() (rpc, error) {
		<-release
		return silentRPC{closed}, nil
	}, time.Minute)
	if took := time.Since(began); err == nil || took > time.Second {
		t.Fatalf("the dial answered %v after %s; want an error at the 100 ms deadline", err, took)
	}
	close(release)
	for end := time.Now().Add(time.Second); !closed.Load() && time.Now().Before(end); time.Sleep(10 * time.Millisecond) {
	}
	if !closed.Load() {
		t.Error("the session the late dial opened was not closed")
	}
}

// A caller waiting on another's dial or proof gives up when its own context
// ends, and is busy, not unreachable.
func TestTheWaitForTheSessionEndsWithTheContext(t *testing.T) {
	n := newNAS(nil, []string{"25.04"})
	n.settling = make(chan struct{}) // another caller is dialling, and does not finish
	short, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	began := time.Now()
	if _, err := n.session(short); !errors.Is(err, ErrBusy) || !errors.Is(err, ErrNotSent) || time.Since(began) > time.Second {
		t.Fatalf("the wait answered %v after %s; want busy at the 100 ms deadline", err, time.Since(began))
	}
}
