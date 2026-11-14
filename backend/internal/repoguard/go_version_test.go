package repoguard

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// One Go for every build: each Go image builds with the Go minor its module
// says, and CI tests each module with the Go its go.mod says (go-version-file,
// never a version of its own). An image a minor behind CI builds code CI
// never ran on that Go: json.Decoder.Token bounds nesting in 1.27 and not in
// 1.25, which strictjson's own limit exists for (slice 7p).
func TestEveryGoBuildUsesTheModulesGoMinor(t *testing.T) {
	root := repoRoot(t)
	read := func(path string) string {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	minorOf := func(mod string) string {
		t.Helper()
		m := regexp.MustCompile(`(?m)^go (\d+\.\d+)(\.\d+)?$`).FindStringSubmatch(read(mod))
		if m == nil {
			t.Fatalf("%s has no go directive", mod)
		}
		return m[1]
	}

	images := map[string]string{ // Dockerfile -> the go.mod its build uses
		"backend/Dockerfile":        "backend/go.mod",
		"backend/Dockerfile.claims": "backend/go.mod",
		"addons/truenas/Dockerfile": "addons/truenas/go.mod",
	}
	from := regexp.MustCompile(`(?m)^FROM golang:(\S+)`)
	for df, mod := range images {
		tags := from.FindAllStringSubmatch(read(df), -1)
		if len(tags) == 0 {
			t.Errorf("%s builds from no golang image", df)
		}
		for _, tag := range tags {
			if want := minorOf(mod) + "-alpine"; tag[1] != want {
				t.Errorf("%s builds with golang:%s, want golang:%s (%s)", df, tag[1], want, mod)
			}
		}
	}
	// Every Go Dockerfile is listed above, so a new image cannot slip by.
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == "node_modules" || d.Name() == ".git" || d.Name() == ".claude" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if strings.HasPrefix(d.Name(), "Dockerfile") && !strings.HasSuffix(d.Name(), ".dockerignore") &&
			from.MatchString(read(rel)) && images[rel] == "" {
			t.Errorf("%s builds Go and is not in this test's list", rel)
		}
		return nil
	})

	ci := read(".github/workflows/ci.yml")
	if regexp.MustCompile(`(?m)^\s*go-version:`).MatchString(ci) {
		t.Error("ci.yml pins a go-version of its own; use go-version-file, the module's go directive")
	}
	files := regexp.MustCompile(`(?m)^\s*go-version-file:\s*(\S+)`).FindAllStringSubmatch(ci, -1)
	if len(files) == 0 {
		t.Fatal("ci.yml sets up Go from no go.mod")
	}
	want := minorOf("backend/go.mod")
	for _, f := range files {
		if got := minorOf(f[1]); got != want {
			t.Errorf("CI's %s is Go %s, the backend's is %s", f[1], got, want)
		}
	}
}
