package repoguard

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The operator-facing log has one vocabulary, and it is a vocabulary because it
// is finite.
//
// Every line in this product is `[SUBSYSTEM] what happened`. That shape is the
// whole of how an operator finds anything: `grep '\[DRIFT\]'` is the question
// "what has the sweep been doing", and it is only answerable while every line
// the sweep writes carries that tag and nothing else does.
//
// Two ways to break it, and both had happened:
//
//   - severity in the tag. `[CACHE]`, `[CACHE WARN]` and `[CACHE ERROR]` are
//     three tags for one subsystem, so grepping the subsystem quietly returned
//     the lines that went well. Severity belongs in the sentence, where it does
//     not fragment the index.
//   - a second name for one subsystem. `drain.go` wrote both `[DRAIN]` and
//     `[PROPAGATION]`, in the same file, for the same work.
//
// The set below is the vocabulary. Adding a subsystem means adding it here,
// which is the point: it is a decision, taken once, rather than a string typed
// at three in the morning.
var subsystems = map[string]bool{
	"ACCESS": true, "ACTION": true, "ADDON": true, "ALLOWANCE": true,
	"AUTH": true, "CACHE": true, "CASCADE": true, "DATA PLANE": true,
	"DIRECTORY": true, "DORMANT": true, "DRIFT": true, "FINDINGS": true,
	"GOVERNANCE": true, "ONBOARDING": true, "PANIC": true, "PROPAGATION": true,
	"REVOKE": true, "ROLES": true, "SCHEDULER": true, "SEED": true,
	"SYSTEM": true, "TARGETS": true, "VAULT": true, "WEBHOOK": true,
	"ZITADEL": true,

	// The process itself, and the add-on's own subsystems. One product, one
	// operator log: an add-on runs in its own container but its lines are read
	// beside the backend's, by the same person, during the same incident.
	"STARTUP": true, "SHUTDOWN": true, "HTTP": true,
	"NAS": true, "STORE": true, "SUBJECTS": true,
}

// Severity words are what a tag drifts into. Named so the failure explains
// itself rather than only reporting an unknown string.
var severityInTag = regexp.MustCompile(`\b(WARN|WARNING|ERROR|ERR|INFO|DEBUG|FATAL|CRIT\w*)\b`)

var logTag = regexp.MustCompile(`log\.(?:Printf|Println|Fatalf|Fatal)\("\[([A-Z][A-Z_ ]*)\]`)

func TestTheLogSpeaksOneVocabulary(t *testing.T) {
	root := repoRoot(t)

	var unknown, severity []string
	walk := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// Vendored and generated trees are nobody's vocabulary.
			if name := info.Name(); name == ".gomodcache" || name == "vendor" || name == "node_modules" {
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
		for _, m := range logTag.FindAllStringSubmatch(string(b), -1) {
			tag := strings.TrimSpace(m[1])
			switch {
			case severityInTag.MatchString(tag):
				severity = append(severity, rel+": ["+tag+"]")
			case !subsystems[tag]:
				unknown = append(unknown, rel+": ["+tag+"]")
			}
		}
		return nil
	}
	for _, tree := range []string{"backend", "addons"} {
		if err := filepath.Walk(filepath.Join(root, tree), walk); err != nil {
			t.Fatalf("walk %s: %v", tree, err)
		}
	}

	if len(severity) > 0 {
		sort.Strings(severity)
		t.Errorf("severity belongs in the sentence, not in the tag — it splits the subsystem's own index:\n  %s",
			strings.Join(dedupe(severity), "\n  "))
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		t.Errorf("log tag outside the vocabulary (add it to `subsystems` deliberately, or use the existing name):\n  %s",
			strings.Join(dedupe(unknown), "\n  "))
	}
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
