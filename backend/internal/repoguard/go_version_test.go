package repoguard

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// One Go for every build, exact to the patch: every module's go directive
// names the same release, every Go image builds FROM that release's golang
// tag, CI sets Go up from a go.mod (go-version-file, never a version of its
// own) and .tool-versions names it for a developer. An image a release
// behind CI builds code CI never ran on that Go: json.Decoder.Token bounds
// nesting in 1.27 and not in 1.25, and strictjson relies on it (slice 7q).
// A full version in the go directive is the Go toolchain's own form for the
// minimum release (go.dev/doc/toolchain), so no toolchain line is needed.
func TestEveryGoBuildUsesTheModulesGo(t *testing.T) {
	root := repoRoot(t)
	read := func(path string) string {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	goOf := func(mod string) string {
		t.Helper()
		m := regexp.MustCompile(`(?m)^go (\S+)$`).FindStringSubmatch(read(mod))
		if m == nil {
			t.Fatalf("%s has no go directive", mod)
		}
		return m[1]
	}
	want := goOf("backend/go.mod")
	if !regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(want) {
		t.Fatalf("backend/go.mod says go %s; name the exact release, as 1.27.4", want)
	}

	mods := repoFiles(t, root, func(name string) bool { return name == "go.mod" })
	if len(mods) < 2 {
		t.Fatalf("found only %v", mods)
	}
	for _, mod := range mods {
		if got := goOf(mod); got != want {
			t.Errorf("%s says go %s, backend/go.mod %s", mod, got, want)
		}
		if regexp.MustCompile(`(?m)^toolchain `).MatchString(read(mod)) {
			t.Errorf("%s has a toolchain line; the go directive alone names the release", mod)
		}
	}

	from := regexp.MustCompile(`(?m)^FROM golang:(\S+)`)
	tag := regexp.MustCompile(`^` + regexp.QuoteMeta(want) + `-alpine\d+\.\d+$`)
	built := 0
	for _, df := range dockerfiles(t, root) {
		for _, m := range from.FindAllStringSubmatch(read(df), -1) {
			built++
			if !tag.MatchString(m[1]) {
				t.Errorf("%s builds with golang:%s, want golang:%s-alpine<3.x>", df, m[1], want)
			}
		}
	}
	if built < 3 {
		t.Errorf("found %d golang stages, want the backend's, claims' and the TrueNAS add-on's", built)
	}

	ci := read(".github/workflows/ci.yml")
	if regexp.MustCompile(`(?m)^\s*go-version:`).MatchString(ci) {
		t.Error("ci.yml pins a go-version of its own; use go-version-file, the module's go directive")
	}
	files := regexp.MustCompile(`(?m)^\s*go-version-file:\s*(\S+)`).FindAllStringSubmatch(ci, -1)
	if len(files) == 0 {
		t.Fatal("ci.yml sets up Go from no go.mod")
	}
	for _, f := range files {
		if got := goOf(f[1]); got != want {
			t.Errorf("CI's %s is Go %s, the backend's is %s", f[1], got, want)
		}
	}

	if !regexp.MustCompile(`(?m)^golang ` + regexp.QuoteMeta(want) + `$`).MatchString(read(".tool-versions")) {
		t.Errorf(".tool-versions does not name golang %s", want)
	}
}

// An image builds exactly what go.mod and go.sum pin, as CI tests it: no
// `go get` and no `go mod tidy` in any Dockerfile, either of which resolves
// versions at build time. backend/Dockerfile ran both until 7p's review,
// so production built newer dependencies than CI ever ran, and broke the day
// one of them (go-redis v9.23) needed a newer Go than the image's.
func TestNoImageResolvesItsOwnDependencies(t *testing.T) {
	root := repoRoot(t)
	resolve := regexp.MustCompile(`\bgo\s+(get|mod\s+tidy|install\s+\S+@)`)
	for _, rel := range dockerfiles(t, root) {
		b, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(b), "\n") {
			if l := strings.TrimSpace(line); !strings.HasPrefix(l, "#") && resolve.MatchString(l) {
				t.Errorf("%s:%d resolves dependencies at build time (%s); copy go.mod and go.sum and `go mod download`", rel, i+1, l)
			}
		}
	}
}

// dockerfiles is every Dockerfile in the repository, by path from its root.
func dockerfiles(t *testing.T, root string) []string {
	t.Helper()
	return repoFiles(t, root, func(name string) bool {
		return strings.HasPrefix(name, "Dockerfile") && !strings.HasSuffix(name, ".dockerignore")
	})
}

// repoFiles is every file in the repository whose name match accepts, by path
// from its root, outside dependencies and worktrees.
func repoFiles(t *testing.T, root string, match func(name string) bool) []string {
	t.Helper()
	var out []string
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
		if match(d.Name()) {
			rel, _ := filepath.Rel(root, path)
			out = append(out, rel)
		}
		return nil
	})
	if len(out) == 0 {
		t.Fatal("no such file found")
	}
	return out
}
