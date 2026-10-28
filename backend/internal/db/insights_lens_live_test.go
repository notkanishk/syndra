package db

import (
	"testing"
	"time"
)

// Insights lens reads against the real schema; gated like every live test by
// liveDB. Each test is about its own user id only.

func TestGrantsWithExpiryAndFirstSeen_AgainstTheSchema(t *testing.T) {
	ctx := liveDB(t)
	user := "live-lens-" + liveSuffix()
	t.Cleanup(func() { _, _ = PG.Exec(ctx, `DELETE FROM direct_role_grants WHERE user_id = $1`, user) })

	now := time.Now()
	for _, g := range []struct {
		role    string
		expires any
	}{
		{"soon", now.Add(48 * time.Hour)}, {"ended", now.Add(-time.Hour)}, {"never", nil},
	} {
		created := now.Add(-72 * time.Hour)
		if _, err := PG.Exec(ctx, `
			INSERT INTO direct_role_grants (user_id, zitadel_project_id, zitadel_role_key, granted_by, expires_at, created_at)
			VALUES ($1, 'live-lens-p', $2, 'op', $3, $4)`, user, g.role, g.expires, created); err != nil {
			t.Fatal(err)
		}
	}
	got, err := GrantsWithExpiry(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	var roles []string
	for _, g := range got {
		if g.UserID == user {
			roles = append(roles, g.RoleKey)
		}
	}
	if len(roles) != 1 || roles[0] != "soon" {
		t.Errorf("roles = %v, want only the one that has an end still ahead", roles)
	}

	seen, err := UsersFirstSeen(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if at, ok := seen[user]; !ok || now.Sub(at) < 71*time.Hour {
		t.Errorf("first seen = %v, %v; want the earliest created_at", at, ok)
	}

	// A person only the grants index knows is absent: that table is rebuilt
	// every sweep, so its created_at is the sweep, not the person.
	indexOnly := "live-lens-idx-" + liveSuffix()
	if _, err := PG.Exec(ctx, `INSERT INTO zitadel_grants_index (grant_id, user_id, project_id, role_keys) VALUES ($1,$2,'live-lens-p',ARRAY['r'])`, "g-"+indexOnly, indexOnly); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = PG.Exec(ctx, `DELETE FROM zitadel_grants_index WHERE user_id = $1`, indexOnly) })
	if seen, err = UsersFirstSeen(ctx); err != nil {
		t.Fatal(err)
	} else if _, ok := seen[indexOnly]; ok {
		t.Error("a person known only to the grants index must not be dated by it")
	}
}
