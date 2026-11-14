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
// 2026): an image names its release (imageRefProblem), never `latest` or a
// bare major that moves under us; a GitHub Action is pinned to the full
// commit SHA its release tag points at, since a tag can be moved and a SHA
// cannot (GitHub, "Security hardening for GitHub Actions": pin actions to a
// full-length commit SHA), with the release in a comment; the UI's
// dependencies name exact versions, bun.lock pinning the rest; and one Bun
// builds, tests and runs the UI. Updating is then a reviewed diff, never a
// rebuild's surprise. Only tracked files count: they are what CI builds.
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

	images := 0
	for _, df := range dockerfiles(t) {
		for _, ref := range fromImages(read(df)) {
			images++
			if p := imageRefProblem(ref); p != "" {
				t.Errorf("%s: %s %s", df, ref, p)
			}
		}
	}
	yaml := func(name string) bool { return strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml") }
	var composes, workflows []string
	for _, f := range repoFiles(t, func(string) bool { return true }) {
		switch {
		case strings.HasPrefix(f, ".github/workflows/") && yaml(f):
			workflows = append(workflows, f)
		case strings.HasPrefix(filepath.Base(f), "docker-compose") && yaml(f):
			composes = append(composes, f)
		}
	}
	image := regexp.MustCompile(`(?m)^\s*image:\s*["']?([^"'\s]+)`)
	for _, f := range append(composes, workflows...) {
		for _, m := range image.FindAllStringSubmatch(read(f), -1) {
			images++
			if p := imageRefProblem(m[1]); p != "" {
				t.Errorf("%s: %s %s", f, m[1], p)
			}
		}
	}
	if images < 10 {
		t.Errorf("found %d image references; the Dockerfiles, compose and CI hold more", images)
	}

	uses := regexp.MustCompile(`(?m)^\s*-?\s*uses:\s*(\S+)(.*)$`)
	pinned := regexp.MustCompile(`^[\w.-]+/[\w./-]+@[0-9a-f]{40}$`)
	release := regexp.MustCompile(`^\s+# v\d+\.\d+\.\d+$`)
	actions := 0
	for _, f := range workflows {
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
	for _, ref := range fromImages(read("ui/Dockerfile")) {
		if tag, _, _ := strings.Cut(strings.TrimPrefix(ref, "oven/bun:"), "@"); tag != bun[1]+"-alpine" {
			t.Errorf("ui/Dockerfile builds FROM %s, want oven/bun:%s-alpine", ref, bun[1])
		}
	}
}

// fromImages is the image each FROM of a Dockerfile builds on: `--platform`
// and other flags skipped, `scratch` and an earlier stage's name not images.
func fromImages(dockerfile string) []string {
	var out []string
	stages := map[string]bool{"scratch": true}
	for _, line := range strings.Split(dockerfile, "\n") {
		f := strings.Fields(line)
		if len(f) < 2 || !strings.EqualFold(f[0], "FROM") {
			continue
		}
		f = f[1:]
		for len(f) > 0 && strings.HasPrefix(f[0], "--") {
			f = f[1:]
		}
		if len(f) == 0 {
			continue
		}
		if !stages[strings.ToLower(f[0])] {
			out = append(out, f[0])
		}
		if len(f) >= 3 && strings.EqualFold(f[1], "AS") {
			stages[strings.ToLower(f[2])] = true
		}
	}
	return out
}

// imageRefProblem says what keeps an image reference from being exact, or "".
//
// A digest (`@sha256:`) pins everything. Otherwise the tag names a release:
// major.minor.patch, or major.minor for PostgreSQL, which numbers its
// releases so (postgresql.org/support/versioning); and an `-alpine` tag names
// its Alpine release, which `-alpine` alone moves. PostgreSQL must stay on
// Alpine even with a digest: text indexes sort by the C library's collation,
// musl's compares byte by byte, and every index here was built under musl;
// glibc's en_US.utf8 orders text differently, so a Debian image would read
// those indexes in the wrong order, silently, until a REINDEX.
func imageRefProblem(ref string) string {
	ref, digest, pinned := strings.Cut(ref, "@")
	if pinned && !regexp.MustCompile(`^sha256:[0-9a-f]{64}$`).MatchString(digest) {
		return "carries a malformed digest"
	}
	name, tag := ref, ""
	if i := strings.LastIndex(ref, ":"); i > strings.LastIndex(ref, "/") {
		name, tag = ref[:i], ref[i+1:]
	}
	if name == "postgres" || strings.HasSuffix(name, "/postgres") {
		if !strings.Contains(tag, "-alpine") {
			return "is not on Alpine: its musl collation is the order every text index was built in"
		}
		if !pinned && !regexp.MustCompile(`^\d+\.\d+-alpine\d+\.\d+(\.\d+)?$`).MatchString(tag) {
			return "is not an exact release (major.minor-alpineX.Y)"
		}
		return ""
	}
	switch {
	case pinned:
		return ""
	case tag == "":
		return "names no tag, so it is latest"
	case !regexp.MustCompile(`^\d+\.\d+\.\d+(-alpine\d+\.\d+(\.\d+)?)?$`).MatchString(tag):
		return "is not an exact release (X.Y.Z, and -alpineX.Y where Alpine), nor a digest"
	}
	return ""
}

func TestImageRefRule(t *testing.T) {
	digest := "@sha256:" + strings.Repeat("ab", 32)
	for ref, ok := range map[string]bool{
		"golang:1.27.4-alpine3.24":                 true,
		"alpine:3.24.4":                            true,
		"postgres:15.20-alpine3.24":                true,
		"redis:7.4.12-alpine3.22":                  true,
		"oven/bun:1.4.4-alpine" + digest:           true,
		"registry.example.org:5000/x/y:1.2.3":      true,
		"postgres:15.20-alpine3.24" + digest:       true,
		"postgres:15.20":                           false, // Debian, glibc collation
		"postgres:15.20-bookworm" + digest:         false,
		"postgres:15-alpine":                       false,
		"postgres:15.20-alpine":                    false,
		"oven/bun:1.4.4-alpine":                    false, // Alpine moves under it
		"golang:1.27-alpine":                       false,
		"alpine:latest":                            false,
		"alpine":                                   false,
		"registry.example.org:5000/x/y":            false,
		"alpine:3.24.4@sha256:short":               false,
		"redis:7-alpine":                           false,
		"ghcr.io/example/thing:1.2.3-alpine3.24.1": true,
	} {
		if got := imageRefProblem(ref); (got == "") != ok {
			t.Errorf("%s: problem %q, want ok=%v", ref, got, ok)
		}
	}

	df := "FROM --platform=$BUILDPLATFORM golang:1.27.4-alpine3.24 AS build\n" +
		"from build as test\n" +
		"FROM scratch\n" +
		"FROM alpine:3.24.4\n" +
		"COPY --from=build /out /out\n"
	if got := strings.Join(fromImages(df), " "); got != "golang:1.27.4-alpine3.24 alpine:3.24.4" {
		t.Errorf("fromImages = %q", got)
	}
}
