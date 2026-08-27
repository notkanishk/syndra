package repoguard

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Who is allowed to change Zitadel.
//
// The architecture's load-bearing claim is that a Syndra-mediated Zitadel
// mutation always leaves a trace BEFORE the Management API call — a ledger row
// for a direct grant, an outbox row for a bundle or rule cascade. That is what
// makes the drift sweep meaningful: a Zitadel-side change with no such trace is
// not trusted, it is triaged. If some other code path can write a grant with no
// row behind it, the sweep is reasoning about a world it cannot see all of.
//
// `deps.go` states the handler half of this ("the handlers no longer call
// Zitadel grant APIs directly"), and the handlers hold it. Nothing stated the
// rule for everybody else, and one path outside the handlers does not follow
// it — recorded below with its argument rather than quietly permitted.
//
// This guard does not fix that path. It stops a SECOND one appearing, which is
// how the first one arrived.
var zitadelGrantMutations = regexp.MustCompile(
	`MgmtClient\.(AddUserGrant|UpdateUserGrant|RemoveUserGrant|DeleteUserGrant)\(`)

// Files permitted to call a Zitadel grant mutation, each with the reason.
var mayWriteZitadelGrants = map[string]string{
	// The outbox drain IS the traced path: every row it dispatches was written
	// transactionally, with its audit line, before anything left the process.
	"backend/internal/services/propagation/deps.go": "the outbox drain's own seam",

	// KNOWN GAP, dormant. The webhook orchestrator propagates a mapping rule by
	// calling Zitadel directly: no ledger row, no outbox row, no audit line, and
	// a failure that is logged and stepped over. It is reachable only when a
	// mapping rule exists, and none does — so this is a trap that springs on
	// whoever creates the first rule, not a live hole.
	//
	// Tracked in openspec/NEXT.md. Listed here so it is impossible to believe
	// the invariant holds everywhere while reading the code that breaks it.
	"backend/internal/zitadel/orchestrator.go": "KNOWN GAP: untraced rule propagation (openspec/NEXT.md)",
}

func TestOnlyTheTracedPathWritesZitadelGrants(t *testing.T) {
	root := repoRoot(t)

	var offenders []string
	err := filepath.Walk(filepath.Join(root, "backend"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if name := info.Name(); name == ".gomodcache" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(path) // #nosec G304 -- walking the repo's own tree
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if _, ok := mayWriteZitadelGrants[rel]; ok {
			return nil
		}
		for _, m := range zitadelGrantMutations.FindAllStringSubmatch(string(b), -1) {
			offenders = append(offenders, rel+": "+m[1])
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk backend: %v", err)
	}

	// An allowlist entry for a file that no longer exists is a permission
	// granted to nobody, sitting where the next reader will trust it. Worse: a
	// file later created at that path inherits the exemption silently.
	for rel := range mayWriteZitadelGrants {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			t.Errorf("allowlisted file does not exist, drop the entry: %s", rel)
		}
	}

	if len(offenders) > 0 {
		sort.Strings(offenders)
		t.Errorf("a Zitadel grant mutation outside the traced path — enqueue it, "+
			"or add the file to `mayWriteZitadelGrants` with the argument:\n  %s",
			strings.Join(dedupe(offenders), "\n  "))
	}
}
