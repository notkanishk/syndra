package demo

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// The reset script cannot import Go, so it carries its own copy of the demo
// fixture ids. That copy is the failure mode this test exists for: a fixture
// project added to the catalog and missed in the script survives
// `reset-data.sh demo --apply` and goes on being served as real data on a
// deployment the operator believes they just cleaned.
//
// Parsing the shell is deliberate. Asserting against a second Go constant
// would only prove Go agrees with Go.
const resetScript = "../../../scripts/reset-data.sh"

func TestResetScriptFixtureIDsMatchCatalog(t *testing.T) {
	raw, err := os.ReadFile(resetScript)
	if err != nil {
		t.Fatalf("read %s: %v", resetScript, err)
	}
	script := string(raw)

	for _, tc := range []struct {
		varName string
		want    []string
	}{
		{"DEMO_PROJECTS", ProjectIDs()},
		{"DEMO_USERS", UserIDs()},
	} {
		got := shellListValues(t, script, tc.varName)

		slices.Sort(got)
		want := slices.Clone(tc.want)
		slices.Sort(want)

		if !slices.Equal(got, want) {
			t.Errorf("%s in %s is %v; catalog has %v.\n"+
				"Fixtures present in the catalog but missing from the script survive a reset.",
				tc.varName, resetScript, got, want)
		}
	}
}

// shellListValues pulls the quoted entries out of a `NAME='a','b','c'` line.
func shellListValues(t *testing.T, script, varName string) []string {
	t.Helper()

	line := regexp.MustCompile(`(?m)^` + varName + `=(.*)$`).FindStringSubmatch(script)
	if line == nil {
		t.Fatalf("%s not assigned in %s", varName, resetScript)
	}

	quoted := regexp.MustCompile(`'([^']*)'`).FindAllStringSubmatch(line[1], -1)
	if len(quoted) == 0 {
		t.Fatalf("%s=%q holds no quoted ids", varName, strings.TrimSpace(line[1]))
	}

	out := make([]string, 0, len(quoted))
	for _, match := range quoted {
		out = append(out, match[1])
	}
	return out
}

func TestFixtureIDAccessorsCoverEveryEntry(t *testing.T) {
	if len(ProjectIDs()) != len(Projects()) {
		t.Errorf("ProjectIDs() returned %d ids for %d projects", len(ProjectIDs()), len(Projects()))
	}
	if len(UserIDs()) != len(Users()) {
		t.Errorf("UserIDs() returned %d ids for %d users", len(UserIDs()), len(Users()))
	}
	for _, id := range ProjectIDs() {
		if _, ok := FindProject(id); !ok {
			t.Errorf("ProjectIDs() returned %q, which FindProject does not resolve", id)
		}
	}
	for _, id := range UserIDs() {
		if _, ok := FindUser(id); !ok {
			t.Errorf("UserIDs() returned %q, which FindUser does not resolve", id)
		}
	}
}
