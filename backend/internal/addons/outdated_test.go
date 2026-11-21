package addons

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"syndra/internal/db"
)

// 7t-3: an add-on older than attribution (its /subjects carries no
// `conflicts`) is told apart by one test, made on /subjects and remembered per
// add-on. /homes asks it there (Attribution) rather than guessing from its own
// rows.

// standUp serves /subjects with subjectsStatus and subjectsBody, everything
// else with homes.json, counts what was asked, and leaves the verdict unknown.
func standUp(t *testing.T, subjectsStatus int, subjectsBody []byte) map[string]int {
	t.Helper()
	homesBody := contractFixture(t, "homes.json")
	asked := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked[r.URL.Path]++
		w.Header()["Date"] = nil
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/subjects" {
			if subjectsStatus >= 400 {
				w.Header().Set("Content-Type", "application/problem+json")
			}
			w.WriteHeader(subjectsStatus)
			_, _ = w.Write(subjectsBody)
			return
		}
		_, _ = w.Write(homesBody)
	}))
	t.Cleanup(srv.Close)
	signedAddon(t, srv.URL, "a-test-secret")
	a, _ := Get("truenas")
	a.attribution.Store(attributionUnknown) // nothing has said yet
	return asked
}

func TestAnAddOnOlderThanAttributionIsOutdatedForSubjectsAndHomes(t *testing.T) {
	for _, tc := range []struct {
		name      string
		conflicts bool
	}{{"older than attribution", false}, {"says whose", true}} {
		t.Run(tc.name, func(t *testing.T) {
			var subjects map[string]any
			if err := json.Unmarshal(contractFixture(t, "subjects.json"), &subjects); err != nil {
				t.Fatal(err)
			}
			if !tc.conflicts {
				delete(subjects, "conflicts")
			}
			body, _ := json.Marshal(subjects)
			asked := standUp(t, http.StatusOK, body)

			h := ReadHomes(context.Background(), "truenas", false)
			if asked["/subjects"] != 1 {
				t.Fatalf("not known yet, /subjects is read once: %v", asked)
			}
			wantReason := ""
			if !tc.conflicts {
				wantReason = db.UnreconciledAddonOutdated
			}
			if h.Reason != wantReason || h.Read != tc.conflicts {
				t.Fatalf("homes: %+v, want reason %q", h, wantReason)
			}
			// Remembered: the next /homes asks /subjects nothing.
			ReadHomes(context.Background(), "truenas", false)
			if asked["/subjects"] != 1 {
				t.Fatalf("remembered, /subjects is not read again: %v", asked)
			}
			if r := Subjects(context.Background(), "truenas", false); r.Outdated == tc.conflicts || (r.Usable() != tc.conflicts) {
				t.Fatalf("subjects: outdated %t usable %t, want outdated %t", r.Outdated, r.Usable(), !tc.conflicts)
			}
		})
	}
}

// A verdict that cannot be learned (/subjects keeps failing) is never homes
// read with nothing matched: not read, with why. And the read that tries to
// learn it is one at a time and at most once per the add-on's fresh-read
// cooldown, however often the page loads; past the cooldown it tries again.
func TestHomesNeverReadAsNobodysWhenWhoseIsUnknown(t *testing.T) {
	asked := standUp(t, http.StatusServiceUnavailable,
		[]byte(`{"type":"urn:syndra:addon:error:NO_CURRENT_READ","title":"No current read","status":503,"code":"NO_CURRENT_READ","retry":"backoff","detail":"TrueNAS could not be read."}`))
	for range 5 {
		h := ReadHomes(context.Background(), "truenas", false)
		if h.Read || h.Reason != db.UnreconciledAppUnreachable || len(h.Homes) != 0 {
			t.Fatalf("whose unknown: want not read, app_unreachable (NO_CURRENT_READ), got %+v", h)
		}
	}
	if asked["/subjects"] != 1 || asked["/homes"] != 5 {
		t.Fatalf("five page loads inside the cooldown read /subjects once: %v", asked)
	}
	a, _ := Get("truenas")
	a.attributionAskedAt.Store(time.Now().Add(-FreshCooldown("truenas") - time.Second).UnixNano())
	ReadHomes(context.Background(), "truenas", false)
	if asked["/subjects"] != 2 {
		t.Fatalf("past the cooldown it asks again: %v", asked)
	}
}

// The one mapping (SubjectsResult.Halt) puts what a reply says ahead of any
// cooldown: an outdated add-on answering its copy inside the cooldown is
// outdated, a store it cannot read is refused, and only a refusal with
// nothing to serve is the cooldown's read_stale.
func TestHaltSaysWhatTheReplySaysAheadOfTheCooldown(t *testing.T) {
	retry := 20
	for _, tc := range []struct {
		name string
		read SubjectsResult
		want string
	}{
		{"outdated, its copy inside the cooldown", SubjectsResult{Outcome: OutcomeSucceeded, Outdated: true,
			Stale: &Stale{Reason: StaleCooldown, RetryAfter: &retry}}, db.UnreconciledAddonOutdated},
		{"store unreadable", SubjectsResult{Outcome: OutcomeIndeterminate, Code: CodeStoreUnreadable, RetryAfter: 5}, db.UnreconciledReadRefused},
		{"cooldown, nothing to serve", SubjectsResult{Outcome: OutcomeUnreached, RetryAfter: 20}, db.UnreconciledStaleRead},
		{"not answered", SubjectsResult{Outcome: OutcomeUnreached}, db.UnreconciledAddonUnreachable},
		{"the app down", SubjectsResult{Outcome: OutcomeUnreached, Status: 503, Code: codeTargetUnreachable}, db.UnreconciledAppUnreachable},
	} {
		if reason, _, halted := tc.read.Halt(); !halted || reason != tc.want {
			t.Errorf("%s: got %q (halted %t), want %q", tc.name, reason, halted, tc.want)
		}
	}
	if _, _, halted := (SubjectsResult{Outcome: OutcomeSucceeded, Current: true}).Halt(); halted {
		t.Error("a usable read is not halted")
	}
}
