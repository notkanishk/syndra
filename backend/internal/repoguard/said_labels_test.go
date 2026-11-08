package repoguard

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"syndra/internal/db"
	"syndra/internal/services"
)

// A reason column carries two things: an operator's own words, and fixed
// labels Syndra writes there itself ("Mapping rule cascade", "Extended from
// Home"). Every surface that quotes a reason quotes db.Said, which drops the
// labels, so a label never reads as words somebody said (owner: nothing says
// what doesn't happen; task 1.8a). This guard finds every fixed reason the
// backend or the console writes and fails when Said would quote it: a new
// label goes into db.syndraLabels in the same change.
var (
	backendLabels = []*regexp.Regexp{
		regexp.MustCompile(`systemParams\(ctx, \w+, "([^"]+)"`),
		regexp.MustCompile(`\.params\([\w.]+, "([^"]+)"\)`),
		regexp.MustCompile(`deltaParams\([^\n]*?"([^"]+)"`),
		regexp.MustCompile(`(?:params\[[^\]]*\]|own|p)\.Reason = "([^"]+)"`),
		regexp.MustCompile(`(?:CascadeAudit|EnqueueParams|ActWrite|BulkRequest)\{[^}\n]*Reason: "([^"]+)"`),
	}
	// a, b.Reason, c = x, "label", z: the literal at Reason's place.
	tupleAssign = regexp.MustCompile(`^\s*([\w.\[\], ]+\.Reason(?:, [\w.\[\]]+)*) = (.+)$`)
	// In the planner's own files: a reason built for its writes.
	plannerReason = regexp.MustCompile(`reason :?= (?:fmt\.Sprintf\()?"([^"%]+)`)

	consoleLabels = []*regexp.Regexp{
		regexp.MustCompile(`useExtend\([^)\n]*?"([^"]+)"`),
		regexp.MustCompile(`reason: "([^"]+)"`),
	}
	// Console files whose `reason: "…"` is never sent as a person's reason.
	consoleNotSent = map[string]string{
		"src/lib/term.ts":                  "why a term can't be offered, said on screen",
		"src/lib/queries/useBulkGrants.ts": "a type: the backend's exclusion code",
	}
)

func TestEveryFixedReasonIsNeverQuoted(t *testing.T) {
	root := repoRoot(t)
	var found []string
	check := func(rel string, line int, label string) {
		if db.Said(label) != "" {
			found = append(found, rel+":"+strconv.Itoa(line)+": "+strconv.Quote(label)+" would be quoted as someone's words: add it to db.syndraLabels")
		}
	}

	internal := filepath.Join(root, "backend", "internal")
	err := filepath.Walk(internal, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		// The access paths: the planner and its writes, the outbox insert.
		access := filepath.Dir(path) == filepath.Join(internal, "services") || filepath.Dir(path) == filepath.Join(internal, "db")
		planner := filepath.Dir(path) == filepath.Join(internal, "services") && strings.HasPrefix(filepath.Base(path), "planner")
		for i, line := range strings.Split(string(src), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			res := backendLabels
			if planner {
				res = append(res[:len(res):len(res)], plannerReason)
			}
			for _, re := range res {
				for _, m := range re.FindAllStringSubmatch(line, -1) {
					check(rel, i+1, m[1])
				}
			}
			if m := tupleAssign.FindStringSubmatch(line); access && m != nil && strings.Contains(m[1], ", ") {
				lhs, rhs := strings.Split(m[1], ", "), strings.Split(m[2], ", ")
				for j, l := range lhs {
					if strings.HasSuffix(l, ".Reason") && j < len(rhs) {
						if v, err := strconv.Unquote(strings.TrimSpace(rhs[j])); err == nil {
							check(rel, i+1, v)
						}
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range []services.ApprovalEffect{services.EffectGive, services.EffectExtend, services.EffectGiveAgain} {
		check("backend/internal/services/request_kinds.go", 0, services.ApprovalReason(e))
	}

	ui := filepath.Join(root, "ui")
	err = filepath.Walk(filepath.Join(ui, "src"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && (info.Name() == "__tests__" || info.Name() == "test-utils") {
			return filepath.SkipDir
		}
		if info.IsDir() || !(strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, ".tsx")) || strings.Contains(path, ".test.") {
			return nil
		}
		rel, _ := filepath.Rel(ui, path)
		if consoleNotSent[rel] != "" {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(src), "\n") {
			for _, re := range consoleLabels {
				for _, m := range re.FindAllStringSubmatch(line, -1) {
					check("ui/"+rel, i+1, m[1])
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) > 0 {
		t.Fatalf("fixed reasons Said would quote:\n%s", strings.Join(found, "\n"))
	}
}

// Said keeps a person's words and drops Syndra's: trimmed, nothing when empty.
func TestSaid(t *testing.T) {
	for in, want := range map[string]string{
		"  term start ":           "term start",
		"":                        "",
		"   ":                     "",
		"Mapping rule cascade":    "",
		"Direct access removal":   "",
		"Extended from Home":      "",
		"Bundle published at v12": "",
		"Grant expired, see desk": "Grant expired, see desk",
	} {
		if got := db.Said(in); got != want {
			t.Errorf("Said(%q) = %q, want %q", in, got, want)
		}
	}
}
