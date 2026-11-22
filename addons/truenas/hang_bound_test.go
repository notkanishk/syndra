package main

import (
	"encoding/json"
	"errors"
	"net/http"
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
func (r silentRPC) Ping() (string, error) { return "", errors.New("call timed out") }
func (r silentRPC) Close() error          { r.closed.Store(true); return nil }

// The bounds must leave the add-on's answer inside the backend's 30 s call
// timeout and the kit's 25 s for a read (AUTHORING.md): a dial and one call.
func TestNASBoundsFitUnderTheBackendsCallTimeout(t *testing.T) {
	if worst := nasDialTimeout + time.Duration(callTimeoutSeconds)*time.Second; worst >= 25*time.Second {
		t.Fatalf("a dial and a call may take %s; a read must answer within the kit's 25 s", worst)
	}
}

// health.get with the NAS blackholed or hanging answers within its bound and
// with the no-copy code /subjects gives, never after the backend gave up on
// the add-on (the 8g staging rerun, 22 Nov 2026: addon_unreachable). Four
// sources, one wait: the dial's or the first call's, then the floor.
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
			s.nas.dialWithin, s.nas.callWithin = 100*time.Millisecond, 1
			if !tc.hangDial {
				s.nas.client, s.nas.probed, s.nas.lastRead = silent, true, time.Now()
			}
			bound := s.nas.dialWithin + time.Duration(s.nas.callWithin)*time.Second + 500*time.Millisecond

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
		err := n.call("system.info", []any{}, nil)
		if took := time.Since(began); took > within || !errors.Is(err, ErrNotSent) {
			t.Fatalf("call %d took %s and answered %v; want ErrNotSent within %s", i+1, took, err, within)
		}
	}
	if got := dials.Load(); got != 1 {
		t.Fatalf("dialled %d times; the second call is inside the floor", got)
	}
}
