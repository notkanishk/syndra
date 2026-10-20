package db

import (
	"testing"
	"time"
)

// An ask that names an end date (until the end of term) keeps it: filed with
// the request and returned by both reads. A days ask returns none.
func TestRequestAskedUntil_FiledAndRead(t *testing.T) {
	ctx := liveDB(t)
	user := freshSubject(t, ctx)

	end := time.Now().Add(20 * 24 * time.Hour).UTC().Truncate(time.Microsecond)
	ninety := 90
	termAsk, err := CreateAccessRequestOfKind(ctx, "access", user, "pUntil", "term", "need it", &ninety, &end)
	if err != nil {
		t.Fatal(err)
	}
	daysAsk, err := CreateAccessRequest(ctx, user, "pUntil", "days", "need it", &ninety)
	if err != nil {
		t.Fatal(err)
	}

	got, err := GetAccessRequestByID(ctx, termAsk)
	if err != nil || got.AskedUntil == nil || !got.AskedUntil.Equal(end) {
		t.Fatalf("by id: asked_until %v (%v), want %v", got.AskedUntil, err, end)
	}
	got, err = GetAccessRequestByID(ctx, daysAsk)
	if err != nil || got.AskedUntil != nil {
		t.Fatalf("a days ask: asked_until %v (%v), want none", got.AskedUntil, err)
	}
	all, err := GetAccessRequests(ctx, "pending")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range all {
		if r.ID == termAsk && (r.AskedUntil == nil || !r.AskedUntil.Equal(end)) {
			t.Fatalf("list: asked_until %v, want %v", r.AskedUntil, end)
		}
	}
}
