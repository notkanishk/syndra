package db

import (
	"errors"
	"testing"
	"time"

	"syndra/internal/models"
)

// Included bundles against the real schema (B8): one level deep both ways,
// what a parent version gives through the view, who blocks a retire, and a
// discard that resets includes with roles.
func TestBundleIncludes_AgainstTheSchema(t *testing.T) {
	ctx := liveDB(t)
	stamp := time.Now().Format("150405.000000")
	mk := func(name string, roles ...string) string {
		var rs []models.BundleRole
		for _, r := range roles {
			rs = append(rs, models.BundleRole{ProjectID: "pInc", RoleKey: r})
		}
		id, err := CreateBundle(ctx, name+" "+stamp, "", "auto", rs)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	child := mk("Safety", "door", "lockers")
	parent := mk("New members", "door")
	other := mk("Staff", "office")
	t.Cleanup(func() {
		for _, id := range []string{parent, other, child} {
			_, _ = PG.Exec(ctx, `DELETE FROM bundles WHERE id = $1`, id)
		}
	})

	in, err := SetWorkingInclude(ctx, "live", parent, child, 0)
	if err != nil || in.ChildVersion != 1 {
		t.Fatalf("include child v1: %+v %v", in, err)
	}
	// One level deep, both ways.
	var nesting *NestingError
	if _, err := SetWorkingInclude(ctx, "live", other, parent, 0); !errors.As(err, &nesting) || !nesting.Includes {
		t.Fatalf("a bundle that includes others can't go inside another: %v", err)
	}
	if _, err := SetWorkingInclude(ctx, "live", child, other, 0); !errors.As(err, &nesting) || nesting.Includes {
		t.Fatalf("a bundle that is included can't include others: %v", err)
	}
	if _, err := SetWorkingInclude(ctx, "live", parent, parent, 0); !errors.Is(err, ErrIncludeSelf) {
		t.Fatalf("self: %v", err)
	}

	// Publish the parent with its include; the version gives both, door once.
	working, _ := GetRolesForBundle(ctx, parent)
	v2, _, err := PublishVersionAndEnqueue(ctx, "live", parent, "with safety", working, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	includes, _ := GetWorkingIncludes(ctx, parent)
	if err := SnapshotVersionIncludes(ctx, v2.ID, includes); err != nil {
		t.Fatal(err)
	}
	gives, err := GetRolesForVersion(ctx, v2.ID)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, r := range gives {
		got[r.RoleKey] = r.ViaBundleID
	}
	if len(gives) != 2 || got["door"] != "" || got["lockers"] != child {
		t.Fatalf("door is the parent's own, lockers comes through the child: %+v", gives)
	}

	// Who blocks retiring the child: the parent, by name.
	blockers, err := RetireBlockers(ctx, child)
	if err != nil || len(blockers) != 1 {
		t.Fatalf("the parent blocks retiring the child: %v %v", blockers, err)
	}
	includers, err := ListIncluders(ctx, child)
	if err != nil || len(includers) != 1 || includers[0].IncludedVersion != 1 {
		t.Fatalf("includers: %+v %v", includers, err)
	}

	// Discard puts the working copy's includes back to the latest version's.
	if _, err := RemoveWorkingInclude(ctx, "live", parent, child); err != nil {
		t.Fatal(err)
	}
	if err := DiscardBundleDraft(ctx, "live", parent); err != nil {
		t.Fatal(err)
	}
	if back, _ := GetWorkingIncludes(ctx, parent); len(back) != 1 {
		t.Fatalf("discard restores the include: %+v", back)
	}
}
