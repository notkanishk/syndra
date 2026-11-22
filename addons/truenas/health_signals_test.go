package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// The three states of a credential's expiry, and why an absent date is not one
// of them.
//
// A key that genuinely has no expiry and a key whose expiry nobody recorded are
// the same empty field, and only the second is a problem — one that surfaces as
// an outage nobody can explain, on a day nobody chose. So the add-on reports
// which of the three it is and never leaves the surface to guess.
func TestHealthDistinguishesNoExpiryFromAnUnrecordedOne(t *testing.T) {
	for name, tc := range map[string]struct {
		expiry  time.Time
		never   bool
		want    string
		hasDate bool
	}{
		"a date the operator recorded": {expiry: time.Now().Add(24 * time.Hour), want: "set", hasDate: true},
		"a key issued without one":     {never: true, want: "none"},
		"nobody said either way":       {want: "unrecorded"},
	} {
		t.Run(name, func(t *testing.T) {
			s := healthServer(t)
			s.keyExpiry, s.keyNeverExpires = tc.expiry, tc.never

			var h Health
			decodeHealth(t, s, &h)

			if h.CredentialExpiry != tc.want {
				t.Errorf("credential_expiry = %q, want %q", h.CredentialExpiry, tc.want)
			}
			if (h.CredentialExpiresAt != nil) != tc.hasDate {
				t.Errorf("credential_expires_at present = %v, want %v", h.CredentialExpiresAt != nil, tc.hasDate)
			}
		})
	}
}

// A share list that could not be read is reported as unreadable, never as
// "nothing is unaudited".
//
// The whole value of putting this on health is that an operator learns SMB
// auditing is off WITHOUT running an activity report — so the one thing it must
// never do is answer the question wrongly when it could not look. Both live
// shares on the deployment this was written against have auditing off, which
// means member activity reports there can only ever come back empty.
func TestHealthSaysWhetherTheShareListCouldBeRead(t *testing.T) {
	t.Run("a target that answers", func(t *testing.T) {
		s := healthServer(t)
		var h Health
		decodeHealth(t, s, &h)
		if !h.SharesReadable {
			t.Error("a reachable target's share list was reported unreadable")
		}
	})

	t.Run("a target that does not answer", func(t *testing.T) {
		s := healthServer(t)
		// A NAS that cannot connect at all: the read fails rather than
		// returning an empty list, and the two must not look the same.
		s.nas = newNAS(func() (rpc, error) { return nil, errors.New("dial: no route to host") },
			[]string{"25.04"})

		var h Health
		decodeHealth(t, s, &h)
		if h.SharesReadable {
			t.Error("an unreachable target reported its share list as readable")
		}
		if len(h.UnauditedShares) != 0 {
			t.Error("shares were listed from a target that never answered")
		}
	})
}

func healthServer(t *testing.T) *server {
	t.Helper()
	s, _ := applyServer(t, `[]`)
	return s
}

func decodeHealth(t *testing.T, s *server, into *Health) {
	t.Helper()
	rec := httptest.NewRecorder()
	s.handleHealth(rec, httptest.NewRequest(http.MethodGet, "/health", nil), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("health status %d", rec.Code)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), into); err != nil {
		t.Fatal(err)
	}
}

// hungRPC is a NAS that answers nothing until released: a blackholed address,
// where a dial or a call waits out its own timeout rather than failing.
type hungRPC struct{ release chan struct{} }

func (h hungRPC) Call(string, int64, any) (json.RawMessage, error) {
	<-h.release
	return nil, errors.New("read tcp: i/o timeout")
}
func (h hungRPC) Ping() (string, error) { _, err := h.Call("core.ping", 0, nil); return "", err }
func (hungRPC) Close() error            { return nil }

// A NAS that hangs is unreachable, and /health says so within its bound,
// however long the dial or the call would wait (the 8g staging run, 22 Nov
// 2026: a blackholed NAS made /health outlast the backend's 5 s probe, which
// then blamed the add-on). Twice in a row: the second /health must not wait
// behind the first's dial, still hanging.
func TestHealthAnswersInTimeWhenTheNASHangs(t *testing.T) {
	for name, hangDial := range map[string]bool{"the dial hangs": true, "the session's call hangs": false} {
		t.Run(name, func(t *testing.T) {
			s := healthServer(t)
			hung := hungRPC{make(chan struct{})}
			t.Cleanup(func() { close(hung.release) })
			s.nas = newNAS(func() (rpc, error) {
				if hangDial {
					<-hung.release
					return nil, errors.New("dial tcp: i/o timeout")
				}
				return hung, nil
			}, []string{"25.04"})
			for i := range 2 {
				began := time.Now()
				var h Health
				decodeHealth(t, s, &h)
				if took := time.Since(began); took > healthReachWithin+time.Second {
					t.Errorf("/health %d took %v with the NAS hanging; want under %v", i+1, took, healthReachWithin+time.Second)
				}
				if h.Reachable || h.SharesReadable {
					t.Errorf("/health %d said reachable %v, shares readable %v with the NAS hanging", i+1, h.Reachable, h.SharesReadable)
				}
			}
		})
	}
}
