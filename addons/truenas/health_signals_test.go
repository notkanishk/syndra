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

			if h.KeyExpiry != tc.want {
				t.Errorf("key_expiry = %q, want %q", h.KeyExpiry, tc.want)
			}
			if (h.KeyExpiresAt != nil) != tc.hasDate {
				t.Errorf("key_expires_at present = %v, want %v", h.KeyExpiresAt != nil, tc.hasDate)
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
