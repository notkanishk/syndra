package db

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// The version-history read (change `addon-platform` 9.7/9.8; design §24).
//
// `internal/db` has no live-database harness, so these are the two things that
// can be asserted without one: the pure comparison that decides whether the
// working copy has drifted, and the SQL text that decides whether an empty
// version can appear in the history at all.

// A published version is what a rollback restores, so "has anything changed
// since" is the question the surface actually asks. Comparing by count would
// call a set with one edited value unchanged — which is precisely the edit
// worth flagging.
func TestDriftIsComparedByContentRatherThanByCount(t *testing.T) {
	live := []RoleMapping{
		{ProjectID: "p", RoleKey: "maker", Field: "group", Value: "lab_makers"},
		{ProjectID: "p", RoleKey: "lead", Field: "group", Value: "leads"},
	}
	same := []MappingVersionEntry{
		// Deliberately the other order: a working copy and a snapshot are two
		// query results, and neither promises the other's ordering.
		{ProjectID: "p", RoleKey: "lead", Field: "group", Value: "leads"},
		{ProjectID: "p", RoleKey: "maker", Field: "group", Value: "lab_makers"},
	}
	if !sameBindings(live, same) {
		t.Error("the same set in a different order is the same set")
	}

	edited := []MappingVersionEntry{
		{ProjectID: "p", RoleKey: "maker", Field: "group", Value: "lab_makers"},
		{ProjectID: "p", RoleKey: "lead", Field: "group", Value: "OLD_leads"},
	}
	if sameBindings(live, edited) {
		t.Error("one edited value is a drifted working copy, not an equal one")
	}
}

func TestAnAddedOrRemovedBindingIsDrift(t *testing.T) {
	live := []RoleMapping{{ProjectID: "p", RoleKey: "maker", Field: "group", Value: "g"}}
	if sameBindings(live, nil) {
		t.Error("a binding that exists live and not in the version is drift")
	}
	if sameBindings(nil, []MappingVersionEntry{{ProjectID: "p", RoleKey: "maker", Field: "group", Value: "g"}}) {
		t.Error("a binding in the version and not live is drift")
	}
}

// A version published against an empty working set is a real event — an
// operator recording that a target intentionally maps nothing. An inner join
// would drop it from the history entirely, and the gap in the version numbers
// would be the only trace.
func TestAnEmptyVersionStillAppearsInTheHistory(t *testing.T) {
	src, err := os.ReadFile("mappings.go")
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	query := regexp.MustCompile(`(?s)FROM target_mapping_versions v(.*?)ORDER BY`).
		FindStringSubmatch(string(src))
	if query == nil {
		t.Fatal("the history query is gone; if it moved, move this guard with it")
	}
	if !strings.Contains(query[1], "LEFT JOIN target_mapping_version_entries") {
		t.Error("the entries join must be a LEFT JOIN, or a version with no bindings vanishes")
	}
}
