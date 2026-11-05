package db

import (
	"testing"
	"time"

	"syndra/internal/models"
)

// What a bundle gives, for counts and the map (one-path W2a): a role that
// reaches a bundle only through an include, and a role only in the version
// somebody is pinned to (dropped from the draft), are both given. The working
// copy alone counted neither.
func TestBundleGives_FollowsIncludesAndHeldVersions(t *testing.T) {
	ctx := liveDB(t)
	stamp := time.Now().Format("150405.000000")
	proj := "pGives-" + stamp
	mk := func(name string, roles ...string) string {
		var rs []models.BundleRole
		for _, r := range roles {
			rs = append(rs, models.BundleRole{ProjectID: proj, RoleKey: r})
		}
		id, err := CreateBundle(ctx, name+" "+stamp, "", rs)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	child := mk("Safety", "lockers")
	parent := mk("Members only by include")
	held := mk("Held older version", "kiln")
	holder := "gives-holder-" + stamp
	t.Cleanup(func() {
		_, _ = PG.Exec(ctx, `DELETE FROM user_bundle_assignments WHERE user_id = $1`, holder)
		for _, id := range []string{parent, held, child} {
			_, _ = PG.Exec(ctx, `DELETE FROM bundles WHERE id = $1`, id)
		}
	})

	// parent gives nothing of its own; it includes child v1.
	if _, err := SetWorkingInclude(ctx, "live", parent, child, 0); err != nil {
		t.Fatal(err)
	}
	// held: somebody sits on v1 (kiln); the draft drops kiln.
	var v1 string
	if err := PG.QueryRow(ctx, `SELECT id FROM bundle_versions WHERE bundle_id = $1 AND version = 1`, held).Scan(&v1); err != nil {
		t.Fatal(err)
	}
	if _, err := PG.Exec(ctx, `INSERT INTO user_bundle_assignments (user_id, bundle_id, version_id, assigned_by) VALUES ($1, $2, $3, 'live')`, holder, held, v1); err != nil {
		t.Fatal(err)
	}
	if _, err := PG.Exec(ctx, `DELETE FROM bundle_roles WHERE bundle_id = $1`, held); err != nil {
		t.Fatal(err)
	}

	given, err := GetBundleRolesGiven(ctx)
	if err != nil {
		t.Fatal(err)
	}
	has := func(bundle, role string) bool {
		for _, r := range given[bundle] {
			if r.ProjectID == proj && r.RoleKey == role {
				return true
			}
		}
		return false
	}
	if !has(parent, "lockers") || !has(child, "lockers") || !has(held, "kiln") {
		t.Fatalf("given: parent %v, child %v, held %v", given[parent], given[child], given[held])
	}

	usage, err := GetRoleUsageCounts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := usage[proj+":lockers"].BundleCount; got != 2 {
		t.Errorf("lockers is given by 2 bundles (the child, and the parent through it), got %d", got)
	}
	if got := usage[proj+":kiln"].BundleCount; got != 1 {
		t.Errorf("kiln is still given through the held version, got %d", got)
	}

	refs, err := GetAllReferencedRoleKeys(ctx)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, r := range refs {
		seen[r[0]+":"+r[1]] = true
	}
	if !seen[proj+":kiln"] || !seen[proj+":lockers"] {
		t.Errorf("referenced roles miss the held or included ones: kiln=%v lockers=%v", seen[proj+":kiln"], seen[proj+":lockers"])
	}
}
