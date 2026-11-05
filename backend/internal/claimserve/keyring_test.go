package claimserve

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"syndra/internal/claimserve/claimservetest"
)

// The keyring's moves (action-key-autorotation): a change keeps the previous
// key PreviousKeyKeep and stamps ChangedAt; a read of the key just moved off
// is a lagging read, never a rotation back; Hold only ever extends.
func TestKeyring_NextAndHold(t *testing.T) {
	t0 := time.Unix(1_800_000_000, 0)
	k := Keyring{}.Next("a", t0)
	if k.Current != "a" || k.Previous != "" || k.ChangedAt != 0 {
		t.Fatalf("first read is not a change: %+v", k)
	}
	k = k.Next("b", t0.Add(time.Hour))
	if k.Current != "b" || k.Previous != "a" || k.ChangedAt != t0.Add(time.Hour).Unix() ||
		k.PreviousUntil != t0.Add(time.Hour+PreviousKeyKeep).Unix() {
		t.Fatalf("change: %+v", k)
	}
	if got := k.Next("a", t0.Add(time.Hour+time.Minute)); got != k {
		t.Fatalf("a stale read of the previous key swapped the keyring: %+v", got)
	}
	// Once the previous key has lapsed, Zitadel answering it is a change (a
	// restore from backup, say), not ignored forever.
	if got := k.Next("a", t0.Add(2*time.Hour)); got.Current != "a" {
		t.Fatalf("after the overlap: %+v", got)
	}
	held := k.Hold(t0.Add(7 * 24 * time.Hour))
	if held.PreviousUntil != t0.Add(7*24*time.Hour).Unix() {
		t.Fatalf("hold: %+v", held)
	}
	if held.Hold(t0) != held {
		t.Fatal("a hold shortened the overlap")
	}
	if (Keyring{Current: "a"}).Hold(t0.Add(time.Hour)).PreviousUntil != 0 {
		t.Fatal("a hold with no previous key made one up")
	}
}

// A door's second look: a key published since the last poll verifies on the
// first call; a forged call is refused without waiting when nothing is in
// flight.
func TestSignatureCheck_RefreshesOnceOnAMismatch(t *testing.T) {
	r := claimservetest.New()
	p := New(r, nil)
	_ = PublishKeyring(context.Background(), r, Keyring{Current: "old-key-0123456789abcdef0123456789ab"})
	_ = p.Poll(context.Background())
	_ = PublishKeyring(context.Background(), r, Keyring{}.Next("old-key-0123456789abcdef0123456789ab", time.Now()).Next("new-key-0123456789abcdef0123456789ab", time.Now()))

	var mismatches atomic.Int32
	h := SignatureCheck("K", p.SigningKeys, p.RefreshKeys, func() { mismatches.Add(1) }, func() bool { return false },
		func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	call := func(key string) int {
		b := `{"x":1}`
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(b))
		req.Header.Set(SignatureHeader, SignatureHeaderValue(time.Now(), []byte(b), key))
		rr := httptest.NewRecorder()
		h(rr, req)
		return rr.Code
	}
	if code := call("new-key-0123456789abcdef0123456789ab"); code != http.StatusOK || mismatches.Load() != 0 {
		t.Fatalf("new key before the poll: HTTP %d, %d mismatches", code, mismatches.Load())
	}
	start := time.Now()
	if code := call("forged-0123456789abcdef0123456789abcd"); code != http.StatusUnauthorized || mismatches.Load() != 1 {
		t.Fatalf("forged: HTTP %d, %d mismatches", code, mismatches.Load())
	}
	if time.Since(start) > 300*time.Millisecond {
		t.Fatalf("a forged call waited %s with nothing in flight", time.Since(start))
	}
}

// The in-flight mark ends itself: a crash mid-replacement leaves no door
// waiting.
func TestRotationInFlight_EndsItself(t *testing.T) {
	r := claimservetest.New()
	ctx := context.Background()
	now := time.Now()
	_ = MarkRotation(ctx, r, now.Add(30*time.Second))
	if !RotationInFlight(ctx, r, now) {
		t.Fatal("not in flight")
	}
	if RotationInFlight(ctx, r, now.Add(31*time.Second)) {
		t.Fatal("still in flight after its end")
	}
	_ = MarkRotation(ctx, r, time.Time{})
	if RotationInFlight(ctx, r, now) {
		t.Fatal("cleared mark still in flight")
	}
}
