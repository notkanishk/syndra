package db

import (
	"testing"
)

// Roles that wait, against the real schema: marking is idempotent and keeps
// who marked first, a row carrying a waiting role is seen as waiting, and the
// reason is recorded only on rows still pending.
func TestRoleWaits_MarkReadAndRecordAgainstTheSchema(t *testing.T) {
	ctx := liveDB(t)
	user := freshSubject(t, ctx)
	project, role := "live-waits-project", "live.waits."+user[len(user)-6:]
	t.Cleanup(func() {
		_, _ = PG.Exec(ctx, `DELETE FROM role_waits WHERE project_id = $1`, project)
		_, _ = PG.Exec(ctx, `DELETE FROM propagation_outbox WHERE user_id = $1`, user)
	})

	if created, err := MarkRoleWaits(ctx, project, role, "first"); err != nil || !created {
		t.Fatalf("mark: %v %v", created, err)
	}
	if created, err := MarkRoleWaits(ctx, project, role, "second"); err != nil || created {
		t.Fatalf("marking twice is a no-op: %v %v", created, err)
	}
	waits, err := ListRoleWaits(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var found *RoleWait
	for i := range waits {
		if waits[i].ProjectID == project && waits[i].RoleKey == role {
			found = &waits[i]
		}
	}
	if found == nil || found.MarkedBy != "first" {
		t.Fatalf("who marked it first is kept: %+v", found)
	}

	res, err := EnqueueDirectGrantPropagation(ctx, EnqueueParams{
		UserID: user, ProjectID: project, RoleKeys: []string{role},
		GrantedBy: "live", Source: "direct", OpType: "add", PayloadJSON: "{}",
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := OutboxRowsWaitForRole(ctx, []string{res.OutboxID})
	if err != nil || !got[res.OutboxID] {
		t.Fatalf("a row carrying a waiting role waits: %v %v", got, err)
	}
	if err := SetWaitReason(ctx, []string{res.OutboxID}, WaitReasonRoleWaits); err != nil {
		t.Fatal(err)
	}
	var reason string
	if err := PG.QueryRow(ctx, `SELECT wait_reason FROM propagation_outbox WHERE id = $1`, res.OutboxID).Scan(&reason); err != nil || reason != WaitReasonRoleWaits {
		t.Fatalf("wait_reason: %q %v", reason, err)
	}
	if err := SetWaitReason(ctx, []string{res.OutboxID}, "not-a-reason"); err == nil {
		t.Fatal("the schema refuses a wait reason it doesn't know")
	}

	if removed, err := UnmarkRoleWaits(ctx, project, role); err != nil || !removed {
		t.Fatalf("unmark: %v %v", removed, err)
	}
	got, _ = OutboxRowsWaitForRole(ctx, []string{res.OutboxID})
	if got[res.OutboxID] {
		t.Fatal("unmarked, the role no longer waits")
	}
	if _, err := ListWaitSuggestions(ctx); err != nil {
		t.Fatalf("suggestions query: %v", err)
	}
}
