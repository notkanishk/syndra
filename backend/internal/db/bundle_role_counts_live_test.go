package db

import (
	"testing"
	"time"

	"syndra/internal/models"
)

// The bundles list counts what each bundle grants today: the distinct roles of
// its latest version, the same set LatestVersionRoles returns for its page.
func TestGetBundleRoleCounts_MatchesTheLatestVersion(t *testing.T) {
	ctx := liveDB(t)
	stamp := time.Now().Format("150405.000000")
	pid := "pCount" + stamp

	two, err := CreateBundle(ctx, "Counts two "+stamp, "", "auto",
		[]models.BundleRole{{ProjectID: pid, RoleKey: "door"}, {ProjectID: pid, RoleKey: "lab"}})
	if err != nil {
		t.Fatal(err)
	}
	none, err := CreateBundle(ctx, "Counts none "+stamp, "", "auto", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, b := range []string{two, none} {
			_, _ = PG.Exec(ctx, `DELETE FROM bundles WHERE id = $1`, b)
		}
	})

	counts, err := GetBundleRoleCounts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, roles, err := LatestVersionRoles(ctx, two)
	if err != nil {
		t.Fatal(err)
	}
	if counts[two] != 2 || counts[two] != len(roles) {
		t.Fatalf("two-role bundle: count %d, latest version holds %d, want 2", counts[two], len(roles))
	}
	if counts[none] != 0 {
		t.Fatalf("a bundle with no roles: count %d, want 0", counts[none])
	}
}
