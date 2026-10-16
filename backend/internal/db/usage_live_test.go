package db

import (
	"testing"
	"time"
)

// usage_last_seen against the real schema. Users carry a test-only prefix so
// only this test's rows are read or removed.

func TestUsageLastSeen_AgainstTheSchema(t *testing.T) {
	ctx := liveDB(t)
	clean := func() { _, _ = PG.Exec(ctx, `DELETE FROM usage_last_seen WHERE user_id LIKE 'usage-live-%'`) }
	clean()
	t.Cleanup(clean)

	now := time.Now().UTC().Truncate(time.Second)
	old, newer := now.Add(-48*time.Hour), now.Add(-time.Hour)

	// Upsert: the later time wins whichever order they arrive in.
	for _, at := range []time.Time{newer, old} {
		if err := RecordUsage(ctx, "usage-live-a", "pA", UsageSignIn, at); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := GetUserUsage(ctx, "usage-live-a")
	if err != nil || len(rows) != 1 || !rows[0].LastAt.Equal(newer) || rows[0].ProjectID != "pA" || rows[0].Kind != UsageSignIn {
		t.Fatalf("after newer then older: %+v, %v; want one row at %v", rows, err, newer)
	}
	if err := RecordUsage(ctx, "usage-live-a", "pA", UsageSignIn, now); err != nil {
		t.Fatal(err)
	}
	if rows, _ = GetUserUsage(ctx, "usage-live-a"); !rows[0].LastAt.Equal(now) {
		t.Fatalf("a later time must replace: %+v", rows)
	}

	// Rows are per project and per kind; '' is "app unknown".
	for _, r := range []struct{ user, project, kind string }{
		{"usage-live-a", "pB", UsageSignIn}, {"usage-live-a", "", UsageSignIn},
		{"usage-live-a", "pA", UsageBadge}, {"usage-live-b", "pA", UsageSignIn},
	} {
		if err := RecordUsage(ctx, r.user, r.project, r.kind, old); err != nil {
			t.Fatalf("%+v: %v", r, err)
		}
	}
	if rows, _ = GetUserUsage(ctx, "usage-live-a"); len(rows) != 4 || !rows[0].LastAt.Equal(now) {
		t.Fatalf("rows = %+v; want 4, most recent first", rows)
	}
	if rows, err = GetUserUsage(ctx, "usage-live-nobody"); err != nil || rows == nil || len(rows) != 0 {
		t.Fatalf("unknown user = %+v, %v; want empty non-nil", rows, err)
	}

	// The kind constraint holds.
	if err := RecordUsage(ctx, "usage-live-a", "pA", "telepathy", now); err == nil {
		t.Error("an unknown kind must be refused by the CHECK constraint")
	}

	// Who used what in a window: '' is left out; the window is inclusive of its start.
	used, err := UsersWhoUsedProjects(ctx, UsageSignIn, old.Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if !used["usage-live-a"]["pA"] || !used["usage-live-a"]["pB"] || used["usage-live-a"][""] || !used["usage-live-b"]["pA"] {
		t.Errorf("used = %v", used["usage-live-a"])
	}
	used, _ = UsersWhoUsedProjects(ctx, UsageSignIn, now.Add(-time.Minute))
	if !used["usage-live-a"]["pA"] || used["usage-live-a"]["pB"] || used["usage-live-b"] != nil {
		t.Errorf("narrow window: a=%v b=%v", used["usage-live-a"], used["usage-live-b"])
	}

	// Latest by kind: sign_in is `now`; badge only counts inside the window.
	latest, err := UsageLatestByKind(ctx, now.Add(-time.Minute))
	if err != nil || latest[UsageSignIn].Before(now) {
		t.Errorf("latest = %v, %v", latest, err)
	}
	if _, ok := latest[UsageBadge]; ok {
		if latest[UsageBadge].Before(now.Add(-time.Minute)) {
			t.Errorf("badge outside the window reported: %v", latest[UsageBadge])
		}
	}

	// Prune: only this test's old rows are asserted on.
	if _, err := PruneUsage(ctx, now.Add(-24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	rows, _ = GetUserUsage(ctx, "usage-live-a")
	if len(rows) != 1 || rows[0].ProjectID != "pA" || rows[0].Kind != UsageSignIn {
		t.Errorf("after prune: %+v; want only the fresh row", rows)
	}
	if rows, _ = GetUserUsage(ctx, "usage-live-b"); len(rows) != 0 {
		t.Errorf("b after prune: %+v", rows)
	}
}
