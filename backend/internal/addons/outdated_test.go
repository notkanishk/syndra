package addons

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// 7t-3: an add-on older than attribution (its /subjects carries no
// `conflicts`) is told apart by one test, made on /subjects and remembered per
// add-on. /homes asks it there rather than guessing from its own rows, and
// reads /subjects once when nothing has said yet.
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
			subjectsBody, _ := json.Marshal(subjects)
			homesBody := contractFixture(t, "homes.json")
			asked := map[string]int{}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				asked[r.URL.Path]++
				w.Header()["Date"] = nil
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/subjects" {
					_, _ = w.Write(subjectsBody)
					return
				}
				_, _ = w.Write(homesBody)
			}))
			t.Cleanup(srv.Close)
			signedAddon(t, srv.URL, "a-test-secret")
			a, _ := Get("truenas")
			a.attribution.Store(attributionUnknown) // nothing has said yet

			h := ReadHomes(context.Background(), "truenas", false)
			if asked["/subjects"] != 1 {
				t.Fatalf("not known yet, /subjects is read once: %v", asked)
			}
			if h.Outdated == tc.conflicts || h.Read == !tc.conflicts {
				t.Fatalf("homes: %+v, want outdated %t", h, !tc.conflicts)
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
