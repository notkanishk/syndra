package addons

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// What a target's groups open crosses the wire whole, and an add-on that can't
// say (never read, or not answering) is "not read yet", never "opens nothing".
func TestReadReachDecodesTheAnswerAndSaysWhenNothingWasRead(t *testing.T) {
	status, body := http.StatusOK, `{"read_at":"2026-10-14T09:02:00Z","current":true,
		"groups":{"community":[{"share":"main","folder":"waddle_share","path":"/mnt/p/main/waddle_share","access":"change"}],"interns":[]},
		"everyone":[{"share":"main","path":"/mnt/p/main","access":"read"}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/reach" {
			t.Errorf("asked %s", r.URL.Path)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	signedAddon(t, srv.URL, "a-test-secret")
	withBreaker(t, 1000, time.Minute)

	got := ReadReach(context.Background(), "truenas")
	if !got.Read || !got.Current || got.ReadAt.IsZero() {
		t.Fatalf("got %+v", got)
	}
	if p := got.Groups["community"]; len(p) != 1 || p[0].Folder != "waddle_share" || p[0].Access != "change" {
		t.Fatalf("community = %+v", p)
	}
	if p, ok := got.Groups["interns"]; !ok || len(p) != 0 {
		t.Fatalf("interns = %v %v", p, ok)
	}

	status, body = http.StatusServiceUnavailable, `{"error":"REACH_NEVER_READ"}`
	if got := ReadReach(context.Background(), "truenas"); got.Read {
		t.Fatalf("a refusal read as an answer: %+v", got)
	}
	if got := ReadReach(context.Background(), "nobody"); got.Read {
		t.Fatalf("an unregistered target read as an answer: %+v", got)
	}
}
