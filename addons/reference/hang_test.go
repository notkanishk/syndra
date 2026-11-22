package reference

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"
)

// A hung app is still reachable while its last answer is within
// recentReadWindow, and not once that answer is older (design §13.2).
func TestAHungAppIsReachableOnlyWhileItsLastAnswerIsRecent(t *testing.T) {
	var hang bool
	a, err := New(Config{Target: "reference", Secret: []byte("s"), Hang: func() bool { return hang }})
	if err != nil {
		t.Fatal(err)
	}
	reachable := func() bool {
		r := a.health(httptest.NewRequest("GET", "/health", nil), nil)
		var h struct{ Reachable bool }
		if err := json.Unmarshal(r.body, &h); err != nil {
			t.Fatal(err)
		}
		return h.Reachable
	}
	if !reachable() {
		t.Fatal("an answering app is reachable")
	}
	hang = true
	if !reachable() {
		t.Fatal("a hung app that answered a moment ago is reachable")
	}
	a.mu.Lock()
	a.readTime = time.Now().Add(-2 * recentReadWindow)
	a.mu.Unlock()
	if reachable() {
		t.Fatal("a hung app whose last answer is old is not reachable")
	}
}
