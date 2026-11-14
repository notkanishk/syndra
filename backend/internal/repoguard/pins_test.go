package repoguard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Every version the repository builds or runs with is exact (owner, 14 Nov
// 2026): an image tag names its release, never `latest` or a bare major
// that moves under us; a GitHub Action is pinned to the full commit SHA its
// release tag points at, since a tag can be moved and a SHA cannot (GitHub,
// "Security hardening for GitHub Actions": pin actions to a full-length
// commit SHA), with the release in a comment; the UI's dependencies name
// exact versions, bun.lock pinning the rest; and one Bun builds, tests and
// runs the UI. Updating is then a reviewed diff, never a rebuild's surprise.
func TestEveryVersionIsPinnedExactly(t *testing.T) {
	root := repoRoot(t)
	read := func(path string) string {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}

	// An image's tag: its release, optionally its Alpine release. PostgreSQL
	// numbers a release major.minor (postgresql.org/support/versioning), so
	// 15.20 is exact; every other image here is major.minor.patch.
	exact := regexp.MustCompile(`^\d+\.\d+\.\d+(-alpine(\d+\.\d+(\.\d+)?)?)?$`)
	postgres := regexp.MustCompile(`^\d+\.\d+(-alpine\d+\.\d+)?$`)
	check := func(where, ref string) {
		name, tag, ok := strings.Cut(ref, ":")
		switch {
		case !ok:
			t.Errorf("%s: %s names no tag, so it is latest", where, ref)
		case name == "postgres" && postgres.MatchString(tag), exact.MatchString(tag):
		default:
			t.Errorf("%s: %s is not an exact release", where, ref)
		}
	}
	images := 0
	from := regexp.MustCompile(`(?m)^FROM\s+(\S+)`)
	for _, df := range dockerfiles(t, root) {
		for _, m := range from.FindAllStringSubmatch(read(df), -1) {
			images++
			check(df, m[1])
		}
	}
	image := regexp.MustCompile(`(?m)^\s*image:\s*(\S+)`)
	yamls := repoFiles(t, root, func(name string) bool {
		return strings.HasPrefix(name, "docker-compose") && strings.HasSuffix(name, ".yml")
	})
	workflows := repoFiles(t, root, func(name string) bool { return strings.HasSuffix(name, ".yml") })
	for _, f := range workflows {
		if strings.HasPrefix(f, filepath.Join(".github", "workflows")+string(filepath.Separator)) {
			yamls = append(yamls, f)
		}
	}
	for _, f := range yamls {
		for _, m := range image.FindAllStringSubmatch(read(f), -1) {
			images++
			check(f, m[1])
		}
	}
	if images < 10 {
		t.Errorf("found %d image references; the Dockerfiles, compose and CI hold more", images)
	}

	uses := regexp.MustCompile(`(?m)^\s*-?\s*uses:\s*(\S+)(.*)$`)
	pinned := regexp.MustCompile(`^[\w.-]+/[\w./-]+@[0-9a-f]{40}$`)
	release := regexp.MustCompile(`^\s+# v\d+\.\d+\.\d+$`)
	actions := 0
	for _, f := range yamls {
		for _, m := range uses.FindAllStringSubmatch(read(f), -1) {
			if strings.HasPrefix(m[1], "./") {
				continue // a workflow of this repository
			}
			actions++
			if !pinned.MatchString(m[1]) || !release.MatchString(m[2]) {
				t.Errorf("%s: uses %s%s; pin the release's commit SHA and name the release: owner/action@<sha> # vX.Y.Z", f, m[1], m[2])
			}
		}
	}
	if actions == 0 {
		t.Error("found no GitHub Action")
	}

	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal([]byte(read("ui/package.json")), &pkg); err != nil {
		t.Fatal(err)
	}
	semver := regexp.MustCompile(`^\d+\.\d+\.\d+$`)
	for section, deps := range map[string]map[string]string{"dependencies": pkg.Dependencies, "devDependencies": pkg.DevDependencies} {
		if len(deps) == 0 {
			t.Errorf("ui/package.json has no %s", section)
		}
		for dep, v := range deps {
			if !semver.MatchString(v) {
				t.Errorf("ui/package.json %s: %s is %q, not an exact version", section, dep, v)
			}
		}
	}

	bun := regexp.MustCompile(`(?m)^bun (\S+)$`).FindStringSubmatch(read(".tool-versions"))
	if bun == nil {
		t.Fatal(".tool-versions names no bun")
	}
	if got := regexp.MustCompile(`(?m)^\s*bun-version:\s*"?([^"\s]+)"?`).FindAllStringSubmatch(read(".github/workflows/ci.yml"), -1); len(got) == 0 {
		t.Error("ci.yml sets up no bun-version")
	} else {
		for _, m := range got {
			if m[1] != bun[1] {
				t.Errorf("ci.yml's bun-version is %s, .tool-versions' %s", m[1], bun[1])
			}
		}
	}
	for _, m := range regexp.MustCompile(`(?m)^FROM oven/bun:(\S+)`).FindAllStringSubmatch(read("ui/Dockerfile"), -1) {
		if m[1] != bun[1]+"-alpine" {
			t.Errorf("ui/Dockerfile builds FROM oven/bun:%s, want %s-alpine", m[1], bun[1])
		}
	}
}
