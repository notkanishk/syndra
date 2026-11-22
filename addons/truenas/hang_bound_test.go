package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// silentRPC is a session to a NAS that answers nothing: each call waits out
// the timeout it is given, as the client's Call does, then fails; once
// closed, a call fails at once on the closed socket.
type silentRPC struct{ closed *atomic.Bool }

func (r silentRPC) Call(_ string, timeoutSeconds int64, _ any) (json.RawMessage, error) {
	if r.closed.Load() {
		return nil, errors.New("failed to send call: use of closed network connection")
	}
	time.Sleep(time.Duration(timeoutSeconds) * time.Second)
	return nil, errors.New("call timed out")
}
func (r silentRPC) Close() error { r.closed.Store(true); return nil }

// busyRPC is a NAS that is alive and busy: core.ping answers at once, the
// method slow ("*" for all) waits out the timeout they are given, and the
// rest answer.
type busyRPC struct{ slow string }

func (r busyRPC) Call(method string, timeoutSeconds int64, _ any) (json.RawMessage, error) {
	if r.slow == method || r.slow == "*" && method != "core.ping" {
		time.Sleep(time.Duration(timeoutSeconds) * time.Second)
		return nil, errors.New("call timed out")
	}
	return json.RawMessage(`{"jsonrpc":"2.0","id":1,"result":[]}`), nil
}
func (busyRPC) Close() error { return nil }

// The request path's one budget leaves the add-on's answer inside the kit's
// 25 s for a read and the backend's 30 s call timeout (AUTHORING.md), and no
// single step's bound is longer than the whole: the deadline, not their sum,
// is what a request waits (TestAWholeRequestStaysInsideItsBudget).
func TestNASBoundsFitUnderTheBackendsCallTimeout(t *testing.T) {
	if requestWithin >= 25*time.Second {
		t.Fatalf("a request may take %s; a read must answer within the kit's 25 s", requestWithin)
	}
	for name, d := range map[string]time.Duration{"dial": nasDialTimeout, "call": callTimeout, "ping": pingTimeout} {
		if d > requestWithin {
			t.Errorf("the %s bound %s is longer than the request's whole budget %s", name, d, requestWithin)
		}
	}
	rr := httptest.NewRecorder()
	var left time.Duration
	(&server{}).withBudget(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		d, ok := r.Context().Deadline()
		if !ok {
			t.Fatal("a request carries no deadline")
		}
		left = time.Until(d)
	})).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/subjects", nil))
	if left <= 0 || left > requestWithin {
		t.Fatalf("a request has %s left at its start; want at most %s", left, requestWithin)
	}
}

// postWithin is postOperation inside the request budget, as routes serves it.
func postWithin(t *testing.T, s *server, name, body string) *httptest.ResponseRecorder {
	t.Helper()
	body = withContractVersion(t, body)
	rr := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/operations/"+name, strings.NewReader(body))
	r.SetPathValue("name", name)
	s.withBudget(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.handleOperation(w, r, []byte(body))
	})).ServeHTTP(rr, r)
	return rr
}

// health.get with the NAS blackholed or hanging answers within its bound and
// with the no-copy code /subjects gives, never after the backend gave up on
// the add-on (the 8g staging rerun, 22 Nov 2026: addon_unreachable). Four
// sources, one wait: the dial's or the first call's and its ping's, then the
// floor.
func TestHealthGetAnswersInTimeWhenTheNASHangs(t *testing.T) {
	for name, tc := range map[string]struct {
		hangDial bool
		code     string
	}{
		"the dial hangs":           {hangDial: true, code: CodeTargetUnreachable},
		"the session's call hangs": {hangDial: false, code: CodeNoCurrentRead},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s := healthServer(t)
			silent := silentRPC{new(atomic.Bool)}
			release := make(chan struct{})
			t.Cleanup(func() { close(release) })
			s.nas = newNAS(func() (rpc, error) {
				if tc.hangDial {
					<-release
					return nil, errors.New("dial tcp: i/o timeout")
				}
				return silentRPC{new(atomic.Bool)}, nil // a new socket, as silent as the last
			}, []string{"25.04"})
			s.nas.dialWithin, s.nas.callWithin, s.nas.pingWithin = 100*time.Millisecond, time.Second, time.Second
			if !tc.hangDial {
				s.nas.client, s.nas.probed, s.nas.lastRead = silent, true, time.Now()
			}
			bound := s.nas.dialWithin + s.nas.callWithin + s.nas.pingWithin + 500*time.Millisecond

			done := make(chan int)
			var code atomic.Value
			go func() {
				rr := postOperation(t, s, "health.get", `{"call_id":"c1","subject":"-"}`)
				var p struct{ Code string }
				_ = json.Unmarshal(rr.Body.Bytes(), &p)
				code.Store(p.Code)
				done <- rr.Code
			}()
			select {
			case status := <-done:
				if status != http.StatusServiceUnavailable || code.Load() != tc.code {
					t.Errorf("health.get answered %d %v; want 503 %s", status, code.Load(), tc.code)
				}
			case <-time.After(bound):
				t.Fatalf("health.get did not answer within %s with the NAS hanging", bound)
			}
		})
	}
}

// The whole request path fits one deadline, whatever it meets: an idle
// proof that gets no answer, a dial, four calls the NAS is too busy to
// answer in time, or a wire held by the collector's long call. Each alone
// fits its own bound; together they would not.
func TestAWholeRequestStaysInsideItsBudget(t *testing.T) {
	const budget = 2500 * time.Millisecond
	for name, rig := range map[string]func(*NAS, chan struct{}){
		"four calls on a busy NAS": func(n *NAS, _ chan struct{}) {
			n.client, n.probed, n.lastRead = busyRPC{slow: "*"}, true, time.Now()
		},
		"an idle proof, a dial and the calls": func(n *NAS, _ chan struct{}) {
			// The cached session answers nothing, so its proof waits out
			// the ping; the new one is alive and busy.
			n.client, n.probed, n.lastRead = silentRPC{new(atomic.Bool)}, true, time.Now().Add(-2*idleProbeAfter)
			n.dial = func() (rpc, error) {
				time.Sleep(900 * time.Millisecond)
				return busyRPC{slow: "*"}, nil
			}
		},
		"waiting in line behind the collector": func(n *NAS, release chan struct{}) {
			n.client, n.probed, n.lastRead = busyRPC{}, true, time.Now()
			n.line <- struct{}{} // the collector's 60 s page holds the wire
			go func() { <-release; n.leave() }()
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s := healthServer(t)
			s.budget = budget
			s.nas = newNAS(func() (rpc, error) { return nil, errors.New("unused") }, []string{"25.04"})
			s.nas.dialWithin, s.nas.callWithin, s.nas.pingWithin = time.Second, time.Second, time.Second
			release := make(chan struct{})
			t.Cleanup(func() { close(release) })
			rig(s.nas, release)

			done := make(chan struct{})
			go func() { postWithin(t, s, "health.get", `{"call_id":"c1","subject":"-"}`); close(done) }()
			select {
			case <-done:
			case <-time.After(budget + 300*time.Millisecond):
				t.Fatalf("health.get did not answer within its %s budget", budget)
			}
		})
	}
}

// A NAS that takes longer than a call's bound to answer it, while it answers
// core.ping, is busy, not gone: the call fails as unreachable (a write behind
// it may have landed), the session stays, nothing arms the reconnect floor,
// and the next call goes out on it.
func TestABusyNASIsNotLost(t *testing.T) {
	busy := busyRPC{slow: "audit.query"}
	var dials atomic.Int32
	n := newNAS(func() (rpc, error) { dials.Add(1); return busy, nil }, []string{"25.04"})
	n.callWithin = time.Second
	n.client, n.probed, n.lastRead = busy, true, time.Now()

	err := n.call(t.Context(), "audit.query", []any{}, nil)
	if !errors.Is(err, ErrTargetUnreachable) || errors.Is(err, ErrNotSent) {
		t.Fatalf("the slow call answered %v; want unreachable, and sent", err)
	}
	if n.client == nil || !n.lastTry.IsZero() {
		t.Fatalf("a busy NAS lost its session (client %v) or armed the floor (lastTry %v)", n.client, n.lastTry)
	}
	if err := n.call(t.Context(), "system.info", []any{}, nil); err != nil {
		t.Fatalf("the next call was refused after a busy one: %v", err)
	}
	if dials.Load() != 0 {
		t.Fatalf("dialled %d times; the busy session was still good", dials.Load())
	}
}

// Silence, a call and then its ping unanswered, arms the floor: the next call
// answers ErrNotSent at once, without dialling.
func TestASilentNASIsLost(t *testing.T) {
	var dials atomic.Int32
	n := newNAS(func() (rpc, error) { dials.Add(1); return silentRPC{new(atomic.Bool)}, nil }, []string{"25.04"})
	n.callWithin, n.pingWithin = time.Second, time.Second
	silent := silentRPC{new(atomic.Bool)}
	n.client, n.probed, n.lastRead = silent, true, time.Now()

	if err := n.call(t.Context(), "system.info", []any{}, nil); !errors.Is(err, ErrTargetUnreachable) {
		t.Fatalf("the silent call answered %v", err)
	}
	if n.client != nil || n.lastTry.IsZero() || !silent.closed.Load() {
		t.Fatal("silence must drop and close the session and arm the floor")
	}
	began := time.Now()
	if err := n.call(t.Context(), "system.info", []any{}, nil); !errors.Is(err, ErrNotSent) || time.Since(began) > 50*time.Millisecond {
		t.Fatalf("the next call answered %v after %s; want ErrNotSent at once", err, time.Since(began))
	}
	if dials.Load() != 0 {
		t.Fatalf("dialled %d times inside the floor", dials.Load())
	}
}

// The collector's calls have their own limit (collectCallTimeout), longer
// than a request's, set on its context: a busy NAS paging its audit log is
// still answering, and no request waits on it.
func TestTheCollectorsCallsGetTheirOwnLimit(t *testing.T) {
	s, nas, _ := statsServer(t)
	rec := &recordLimits{rpc: nas}
	s.nas.client, s.nas.probed, s.nas.lastRead = rec, true, time.Now()
	s.collectStatsOnce(context.Background())
	seen := 0
	for _, got := range rec.limits {
		method, limit, _ := strings.Cut(got, " ")
		if method == "audit.query" || method == "pool.dataset.get_quota" {
			seen++
			if limit != collectCallTimeout.String() {
				t.Errorf("the collector's %s was given %s; want %s", method, limit, collectCallTimeout)
			}
		}
	}
	if seen == 0 {
		t.Fatal("the collector made no audit.query or get_quota call")
	}

	// And a call that takes longer than a request's bound succeeds within it.
	slow := newNAS(nil, []string{"25.04"})
	slow.callWithin = time.Second
	slow.client, slow.probed, slow.lastRead = lateRPC{1500 * time.Millisecond}, true, time.Now()
	if err := slow.call(t.Context(), "audit.query", []any{}, nil); err == nil {
		t.Fatal("a 1.5 s answer beat a 1 s request bound")
	}
	slow.client, slow.lastTry = lateRPC{1500 * time.Millisecond}, time.Time{}
	if err := slow.call(withCallLimit(t.Context(), 3*time.Second), "audit.query", []any{}, nil); err != nil {
		t.Fatalf("a 1.5 s answer within the collector's 3 s limit failed: %v", err)
	}
}

// recordLimits passes each call to rpc after noting the limit it was given.
type recordLimits struct {
	rpc
	limits []string
}

func (r *recordLimits) Call(method string, timeoutSeconds int64, params any) (json.RawMessage, error) {
	r.limits = append(r.limits, method+" "+(time.Duration(timeoutSeconds)*time.Second).String())
	return r.rpc.Call(method, timeoutSeconds, params)
}

// lateRPC answers every call after `after`, or times out first if its
// timeout is shorter; core.ping answers at once.
type lateRPC struct{ after time.Duration }

func (r lateRPC) Call(method string, timeoutSeconds int64, _ any) (json.RawMessage, error) {
	if method != "core.ping" {
		if limit := time.Duration(timeoutSeconds) * time.Second; limit < r.after {
			time.Sleep(limit)
			return nil, errors.New("call timed out")
		}
		time.Sleep(r.after)
	}
	return json.RawMessage(`{"jsonrpc":"2.0","id":1,"result":[]}`), nil
}
func (lateRPC) Close() error { return nil }

// lost forgets only the session that went silent, in one critical section:
// a session dialled since is kept and the floor is not armed for it, and the
// silent one is closed, not the new one.
func TestLostForgetsOnlyTheSilentSession(t *testing.T) {
	n := newNAS(nil, []string{"25.04"})
	old, current := silentRPC{new(atomic.Bool)}, silentRPC{new(atomic.Bool)}
	n.client = current
	n.lost(old)
	if n.client != current || !n.lastTry.IsZero() || current.closed.Load() || !old.closed.Load() {
		t.Fatalf("lost(old) touched the newer session: client kept %v, floor %v, new closed %v, old closed %v",
			n.client == current, n.lastTry, current.closed.Load(), old.closed.Load())
	}
	n.lost(current)
	if n.client != nil || n.lastTry.IsZero() || !current.closed.Load() {
		t.Fatal("lost(current) must drop, close and arm the floor")
	}
}

// After a dial that got no session in time, the next call inside the
// reconnect floor answers ErrNotSent at once, without dialling again.
func TestACallAfterAFailedDialFailsFast(t *testing.T) {
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	var dials atomic.Int32
	n := newNAS(func() (rpc, error) {
		dials.Add(1)
		<-release
		return nil, errors.New("dial tcp: i/o timeout")
	}, []string{"25.04"})
	n.dialWithin = 100 * time.Millisecond

	for i, within := range []time.Duration{n.dialWithin + 400*time.Millisecond, 50 * time.Millisecond} {
		began := time.Now()
		err := n.call(t.Context(), "system.info", []any{}, nil)
		if took := time.Since(began); took > within || !errors.Is(err, ErrNotSent) {
			t.Fatalf("call %d took %s and answered %v; want ErrNotSent within %s", i+1, took, err, within)
		}
	}
	if got := dials.Load(); got != 1 {
		t.Fatalf("dialled %d times; the second call is inside the floor", got)
	}
}
