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

	mods := repoFiles(t, func(name string) bool { return filepath.Base(name) == "go.mod" })
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

	tag := golangImage(want)
	built := 0
	for _, df := range dockerfiles(t) {
		for _, ref := range fromImages(read(df)) {
			if !strings.HasPrefix(ref, "golang:") && !strings.HasPrefix(ref, "golang@") {
				continue
			}
			built++
			if !tag.MatchString(ref) {
				t.Errorf("%s builds with %s, want golang:%s-alpine<3.x>", df, ref, want)
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
	for _, rel := range dockerfiles(t) {
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

// golangImage matches the golang image for Go release want: its versioned
// Alpine tag, optionally pinned further by digest, as imageRefProblem allows.
func golangImage(want string) *regexp.Regexp {
	return regexp.MustCompile(`^golang:` + regexp.QuoteMeta(want) + `-alpine\d+\.\d+(@sha256:[0-9a-f]{64})?$`)
}

func TestGolangImageAgreesWithThePinRule(t *testing.T) {
	digest := "@sha256:" + strings.Repeat("ab", 32)
	for ref, ok := range map[string]bool{
		"golang:1.27.4-alpine3.24":          true,
		"golang:1.27.4-alpine3.24" + digest: true,
		"golang:1.27.4-alpine3.24@sha256:x": false,
		"golang:1.27.3-alpine3.24" + digest: false,
		"golang:1.27-alpine":                false,
	} {
		if golangImage("1.27.4").MatchString(ref) != ok {
			t.Errorf("%s: want match=%v", ref, ok)
		}
		if ok && imageRefProblem(ref) != "" {
			t.Errorf("%s: the Go guard accepts it and the pin guard does not: %s", ref, imageRefProblem(ref))
		}
	}
}

// dockerfiles is every tracked Dockerfile, by path from the repository root.
func dockerfiles(t *testing.T) []string {
	t.Helper()
	return repoFiles(t, func(name string) bool {
		base := filepath.Base(name)
		return strings.HasPrefix(base, "Dockerfile") && !strings.HasSuffix(base, ".dockerignore")
	})
}

// repoFiles is every tracked file (git ls-files: what CI checks out and
// builds, never a stray file on this disk) match accepts, by path from the
// repository root.
func repoFiles(t *testing.T, match func(path string) bool) []string {
	t.Helper()
	_, files := trackedFiles(t)
	var out []string
	for _, f := range files {
		if match(f) {
			out = append(out, f)
		}
	}
	if len(out) == 0 {
		t.Fatal("no such tracked file")
	}
	return out
}
