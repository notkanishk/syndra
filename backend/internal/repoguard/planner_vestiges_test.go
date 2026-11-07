package repoguard

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Every access change goes through the planner (architecture-deepening task
// 1.3). What it replaced was a second way to decide a change: a rehearsal per
// row (rehearseOne, PlannedOp), a separate guess at what a write cancels
// (cancellingWrites, CancellationsReviewed, an acknowledgement carried in the
// X-Syndra-Cancel-Ack header) and a separate guess at what the rules give
// (RulesFollowing, GET /users/{id}/rule-preview). Each disagreed with the write
// it previewed at least once. Their names coming back is how a second decider
// comes back, so none may appear in the backend's own code, comments included.
var plannerVestiges = regexp.MustCompile(
	`\b(cancellingWrites|rehearseOne|CancellationsReviewed|RulesFollowing|PlannedOp)\b|X-Syndra-Cancel-Ack|rule-preview`)

func TestNothingDecidesAnAccessChangeBesideThePlanner(t *testing.T) {
	root := filepath.Join(repoRoot(t), "backend", "internal")
	var found []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(path) // #nosec G304 -- walking the repo's own tree
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		for i, line := range strings.Split(string(b), "\n") {
			if m := plannerVestiges.FindString(line); m != "" {
				found = append(found, rel+":"+strconv.Itoa(i+1)+": "+m)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	sort.Strings(found)
	if len(found) > 0 {
		t.Fatalf("a name the planner replaced is back; plan the change through services.PlanAct/RehearseBulk instead:\n  %s",
			strings.Join(found, "\n  "))
	}
}
