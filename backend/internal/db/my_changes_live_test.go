package db

import (
	"strings"
	"testing"
	"time"
)

// A member's month against the real schema (B10, N19): every kind, newest
// first; an approval still queued says it waits and why; a fulfilled request,
// a cancelled send, another person's change and last quarter's are left out;
// and the row an approval queued isn't repeated as "given".
func TestListMyChanges_AgainstTheSchema(t *testing.T) {
	ctx := liveDB(t)
	user := freshSubject(t, ctx)
	other := freshSubject(t, ctx)
	dropOutboxAfter(t, ctx, user)
	dropOutboxAfter(t, ctx, other)

	declined, err := CreateAccessRequest(ctx, user, "pLab", "member", "need it", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := ResolveAccessRequest(ctx, declined, "rejected", "op", "Book the induction first."); err != nil {
		t.Fatal(err)
	}
	withdrawn, err := CreateAccessRequest(ctx, user, "pLab", "door", "need it", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := WithdrawAccessRequest(ctx, withdrawn, user); err != nil {
		t.Fatal(err)
	}
	ten := 10
	approved, err := CreateAccessRequest(ctx, user, "pLab", "supervisor", "trained", &ten)
	if err != nil {
		t.Fatal(err)
	}
	expiry := time.Now().Add(10 * 24 * time.Hour)
	res, err := ApproveRequestAndEnqueue(ctx, approved, "op", "Congratulations.", EnqueueParams{
		UserID: user, ProjectID: "pLab", RoleKeys: []string{"supervisor"}, GrantedBy: "op",
		Reason: "Approved from access request", ExpiresAt: &expiry, Source: "direct",
		SourceRef: approved, OpType: "add", PayloadJSON: `{}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := SetWaitReason(ctx, []string{res.OutboxID}, WaitReasonRoleWaits); err != nil {
		t.Fatal(err)
	}
	// Given by hand, then revoked.
	if _, err := EnqueueDirectGrantPropagation(ctx, EnqueueParams{
		UserID: user, ProjectID: "pWood", RoleKeys: []string{"bandsaw"}, GrantedBy: "op",
		Reason: "For the capstone.", Source: "direct", OpType: "add", PayloadJSON: `{}`,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := PG.Exec(ctx, `UPDATE propagation_outbox SET status = 'applied', completed_at = NOW() WHERE user_id = $1 AND project_id = 'pWood'`, user); err != nil {
		t.Fatal(err)
	}
	if _, err := EnqueueDirectGrantPropagation(ctx, EnqueueParams{
		UserID: user, ProjectID: "pWood", RoleKeys: []string{"bandsaw"}, GrantedBy: "op",
		Source: "direct", OpType: "revoke", PayloadJSON: `{}`,
	}); err != nil {
		t.Fatal(err)
	}
	// Through a bundle; a send cancelled before it went; another person's; last quarter's.
	for _, row := range []struct{ user, status, source, ref, age string }{
		{user, "applied", "bundle", "bundle-1", "0"},
		{user, "superseded", "bundle", "bundle-1", "0"},
		{other, "applied", "direct", "", "0"},
		{user, "applied", "direct", "", "45"},
	} {
		if _, err := PG.Exec(ctx, `
			INSERT INTO propagation_outbox
				(op_type, user_id, project_id, role_keys, payload_json, idempotency_key, initiated_by, source, source_ref, target, status, created_at)
			VALUES ('add', $1, 'pBundle', ARRAY['door'], '{}', uuid_generate_v4(), 'op', $3, NULLIF($4,''), 'zitadel', $2,
			        NOW() - make_interval(days => $5::int))`,
			row.user, row.status, row.source, row.ref, row.age); err != nil {
			t.Fatal(err)
		}
	}
	old, err := CreateAccessRequest(ctx, user, "pOld", "x", "long ago", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := ResolveAccessRequest(ctx, old, "rejected", "op", "no"); err != nil {
		t.Fatal(err)
	}
	if _, err := PG.Exec(ctx, `UPDATE access_requests SET resolved_at = NOW() - interval '45 days' WHERE id = $1`, old); err != nil {
		t.Fatal(err)
	}

	got, err := ListMyChanges(ctx, user, 30)
	if err != nil {
		t.Fatal(err)
	}
	byKind := map[string][]MyChangeRow{}
	for i, c := range got {
		if i > 0 && c.At.After(got[i-1].At) {
			t.Fatalf("newest first: %v after %v", c.At, got[i-1].At)
		}
		byKind[c.Kind] = append(byKind[c.Kind], c)
	}
	if len(got) != 6 {
		t.Fatalf("want 6 changes (declined, withdrawn, approved, given by hand, given via bundle, revoked), got %d: %+v", len(got), got)
	}
	if d := byKind["request_declined"]; len(d) != 1 || d[0].Actor != "op" || d[0].Note != "Book the induction first." {
		t.Fatalf("declined: %+v", d)
	}
	if w := byKind["request_withdrawn"]; len(w) != 1 || w[0].Actor != "" {
		t.Fatalf("withdrawn: %+v", w)
	}
	a := byKind["request_approved"]
	if len(a) != 1 || !a[0].Pending || a[0].WaitReason != WaitReasonRoleWaits || a[0].Until == nil || a[0].Note != "Congratulations." {
		t.Fatalf("approved and waiting: %+v", a)
	}
	given := byKind["given"]
	if len(given) != 2 {
		t.Fatalf("given by hand and via the bundle, not the approval's own row: %+v", given)
	}
	for _, g := range given {
		if g.ProjectID == "pBundle" && (g.Source != "bundle" || g.SourceRef != "bundle-1") {
			t.Fatalf("via the bundle: %+v", g)
		}
		if g.ProjectID == "pWood" && g.Actor != "op" {
			t.Fatalf("given by hand: %+v", g)
		}
	}
	if ta := byKind["taken_away"]; len(ta) != 1 || ta[0].RoleKey != "bandsaw" {
		t.Fatalf("revoked: %+v", ta)
	}

	// Once staff send it, it no longer waits.
	if _, err := PG.Exec(ctx, `UPDATE propagation_outbox SET status = 'applied', completed_at = NOW() WHERE id = $1`, res.OutboxID); err != nil {
		t.Fatal(err)
	}
	got, err = ListMyChanges(ctx, user, 30)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range got {
		if c.Kind == "request_approved" && c.Pending {
			t.Fatalf("sent, so not waiting: %+v", c)
		}
	}
}

// Without a database, the statement still has to say the three things the
// live test proves: cancelled sends aren't changes, a fulfilled request is its
// grant's row, and an approval's own queued row isn't repeated.
func TestListMyChanges_StatementShape(t *testing.T) {
	for _, want := range []string{
		"o.status NOT IN ('superseded', 'abandoned')",
		"ar.status IN ('approved', 'rejected', 'withdrawn')",
		"NOT EXISTS (SELECT 1 FROM access_requests ar WHERE ar.id::text = o.source_ref)",
		"o.status = 'pending'",
		"ar.requester_user_id = $1",
		"o.user_id = $1",
	} {
		if !strings.Contains(myChangesSQL, want) {
			t.Errorf("the feed lost %q", want)
		}
	}
}
