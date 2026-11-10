package repoguard

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// Every list of names is alphabetical the way a person reads it: case and
// accents aside, "Lab 2" before "Lab 10" (owner, 4 Nov 2026; ui/src/lib/sort.ts).
// internal/textsort is the one place the backend does that. This guard refuses
// the two ways a name list was sorted another way: sort.Strings on a list of
// names (byte order, so "netbird" sorts after "Zack"), and a sort.Slice whose
// less function compares a name with < or strings.ToLower (the same byte order,
// or case folded and nothing more).
//
// Lists of ids and keys that only need a stable order (fingerprints, sets) are
// not names and are not touched.
var (
	nameStringsSort = regexp.MustCompile(`(?:^|[^\w])sort\.Strings\(\s*\w*([Nn]ames?|[Ll]abels?|[Tt]itles?|[Pp]rojects)\s*\)`)
	sortSliceCall   = regexp.MustCompile(`sort\.(Slice|SliceStable)\(`)
	nameLess        = regexp.MustCompile(`(?:strings\.ToLower\()?[\w.\[\]]*\.(Name|Username|ChildName|DisplayName|Label|Title)\)?\s*<`)
)

func TestNameListsSortThroughTextsort(t *testing.T) {
	root := filepath.Join(repoRoot(t), "backend")
	var found []string
	for _, dir := range []string{"internal", "cmd"} {
		err := filepath.Walk(filepath.Join(root, dir), func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() && path == filepath.Join(root, "internal", "textsort") {
				return filepath.SkipDir
			}
			if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			src, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			lines := strings.Split(string(src), "\n")
			rel, _ := filepath.Rel(root, path)
			for i, line := range lines {
				if strings.HasPrefix(strings.TrimSpace(line), "//") {
					continue
				}
				if nameStringsSort.MatchString(line) {
					found = append(found, rel+":"+strconv.Itoa(i+1)+": sort.Strings on names: use textsort.Strings")
				}
				if sortSliceCall.MatchString(line) {
					// The less function is on this line or the next two.
					end := min(i+3, len(lines))
					if nameLess.MatchString(strings.Join(lines[i:end], "\n")) {
						found = append(found, rel+":"+strconv.Itoa(i+1)+": a name compared with <: use textsort.ByName")
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(found) > 0 {
		t.Fatalf("names must sort through internal/textsort:\n%s", strings.Join(found, "\n"))
	}
}

// The guard has to see the shapes it bans, or it passes by reading nothing.
func TestNameSortGuardSeesWhatItBans(t *testing.T) {
	for _, bad := range []string{
		`sort.Strings(bundleNames)`,
		`sort.Slice(out, func(i, j int) bool { return out[i].ChildName < out[j].ChildName })`,
		"sort.SliceStable(out, func(i, j int) bool {\n\treturn strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)\n})",
	} {
		first := strings.SplitN(bad, "\n", 2)[0]
		if !nameStringsSort.MatchString(first) && !(sortSliceCall.MatchString(first) && nameLess.MatchString(bad)) {
			t.Errorf("guard would miss: %s", bad)
		}
	}
	for _, fine := range []string{`sort.Strings(keyProjectIDs)`, `sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })`} {
		if nameStringsSort.MatchString(fine) || (sortSliceCall.MatchString(fine) && nameLess.MatchString(fine)) {
			t.Errorf("guard would flag a sort that is not by name: %s", fine)
		}
	}
}
