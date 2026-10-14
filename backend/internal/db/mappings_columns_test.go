package db

import "testing"

// The column list and the scan list are one pair. Without a database this is
// the whole check; with one, TestMappingsForRoles_QueryMatchesScanner runs the
// real query through the real scanner.
func TestEveryRoleMappingReadSelectsWhatItScans(t *testing.T) {
	if got, want := len(roleMappingFields), len((&RoleMapping{}).scanDest()); got != want {
		t.Fatalf("role mapping reads select %d columns and scan %d", got, want)
	}
}
