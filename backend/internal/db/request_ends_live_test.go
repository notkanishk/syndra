package db

import (
	"testing"
	"time"
)

// A settled extension remembers its ends, written in the same statement as the
// resolution and returned by every read: approval keeps both, a decline only
// the end they had, and a request that was never an extension keeps neither.
func TestRequestEnds_ResolveWritesThemAndReadsReturnThem(t *testing.T) {
	ctx := liveDB(t)
	user := freshSubject(t, ctx)
	t.Cleanup(func() { _, _ = PG.Exec(ctx, `DELETE FROM propagation_outbox WHERE user_id = $1`, user) })

	had := time.Now().Add(10 * 24 * time.Hour).UTC().Truncate(time.Microsecond)
	set := had.Add(31 * 24 * time.Hour)
	thirty := 31

	approved, err := CreateAccessRequestOfKind(ctx, "extend", user, "pEnds", "keep", "keep it", &thirty, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApproveRequestAndEnqueue(ctx, approved, "op", "ok", EnqueueParams{
		UserID: user, ProjectID: "pEnds", RoleKeys: []string{"keep"}, GrantedBy: "op",
		ExpiresAt: &set, EndRule: EndSet, Source: "direct", SourceRef: approved, OpType: "add", PayloadJSON: `{}`,
	}, &had, &set); err != nil {
		t.Fatal(err)
	}
	declined, err := CreateAccessRequestOfKind(ctx, "extend", user, "pEnds", "other", "keep it", &thirty, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := ResolveAccessRequest(ctx, declined, "rejected", "op", "no", &had); err != nil {
		t.Fatal(err)
	}
	plain, err := CreateAccessRequest(ctx, user, "pEnds", "plain", "need it", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := ResolveAccessRequest(ctx, plain, "rejected", "op", "no", nil); err != nil {
		t.Fatal(err)
	}

	want := map[string][2]*time.Time{approved: {&had, &set}, declined: {&had, nil}, plain: {nil, nil}}
	same := func(a, b *time.Time) bool { return (a == nil && b == nil) || (a != nil && b != nil && a.Equal(*b)) }
	check := func(via string, id string, before, after *time.Time) {
		t.Helper()
		if w := want[id]; !same(before, w[0]) || !same(after, w[1]) {
			t.Errorf("%s %s: ended_before %v ends_after %v, want %v %v", via, id, before, after, w[0], w[1])
		}
	}
	all, err := GetAccessRequests(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, r := range all {
		if _, ok := want[r.ID]; ok {
			seen++
			check("list", r.ID, r.EndedBefore, r.EndsAfter)
		}
	}
	if seen != 3 {
		t.Fatalf("list returned %d of the 3 requests", seen)
	}
	for id := range want {
		r, err := GetAccessRequestByID(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		check("by id", id, r.EndedBefore, r.EndsAfter)
	}

	// The member's own feed carries the same two ends.
	rows, err := ListMyChanges(ctx, user, 30)
	if err != nil {
		t.Fatal(err)
	}
	feed := map[string]MyChangeRow{}
	for _, r := range rows {
		if r.Kind == "request_approved" || r.Kind == "request_declined" {
			feed[r.RoleKey] = r
		}
	}
	if r := feed["keep"]; !same(r.EndedBefore, &had) || !same(r.EndsAfter, &set) {
		t.Errorf("feed approved extension: %v %v", r.EndedBefore, r.EndsAfter)
	}
	if r := feed["other"]; !same(r.EndedBefore, &had) || r.EndsAfter != nil {
		t.Errorf("feed declined extension: %v %v", r.EndedBefore, r.EndsAfter)
	}
	if r := feed["plain"]; r.EndedBefore != nil || r.EndsAfter != nil {
		t.Errorf("feed plain request: %v %v", r.EndedBefore, r.EndsAfter)
	}

	// A second decision loses the race and writes nothing.
	if err := ResolveAccessRequest(ctx, declined, "rejected", "op2", "again", nil); err != ErrRequestNotPending {
		t.Fatalf("a settled request is not decided twice: %v", err)
	}
	r, _ := GetAccessRequestByID(ctx, declined)
	check("after a lost race", declined, r.EndedBefore, r.EndsAfter)
}
