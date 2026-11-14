package main

import (
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

// The narrow image (assessment P4): Dockerfile.claims copies only the
// packages this binary imports, so a backend-only change cannot change it,
// and every package it imports is copied, so it builds. Both directions are
// checked against `go list`, so a new import fails here, not in the deploy.
func TestTheImageCopiesExactlyWhatTheBinaryImports(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", ".").Output()
	if err != nil {
		t.Skipf("go list unavailable: %v", err)
	}
	need := map[string]bool{}
	for _, pkg := range strings.Fields(string(out)) {
		if rest, ok := strings.CutPrefix(pkg, "syndra/"); ok {
			need[rest] = true
		}
	}
	for _, banned := range []string{"internal/db", "internal/handlers", "internal/zitadel", "internal/cache", "internal/services"} {
		if need[banned] {
			t.Errorf("the claims binary imports %s: the admin side is back in the data plane's crash domain", banned)
		}
	}

	df, err := os.ReadFile("../../Dockerfile.claims")
	if err != nil {
		t.Fatal(err)
	}
	copied := map[string]bool{}
	// The context is the repository root, so a backend path is backend/<path>.
	for _, m := range regexp.MustCompile(`(?m)^COPY backend/(\S+) \./(\S+)$`).FindAllStringSubmatch(string(df), -1) {
		copied[strings.TrimSuffix(m[1], "/")] = true
	}
	// Every module go.mod replaces with a directory: resolving the module
	// graph (`go mod download`) reads its go.mod, imported or not.
	mod, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range regexp.MustCompile(`(?m)^replace \S+ => \.\./(\S+)$`).FindAllStringSubmatch(string(mod), -1) {
		if !strings.Contains(string(df), "\nCOPY "+m[1]+"/go.mod /src/"+m[1]+"/go.mod\n") {
			t.Errorf("go.mod replaces a module with ../%s, but Dockerfile.claims does not copy its go.mod to /src/%[1]s", m[1])
		}
	}
	for pkg := range need {
		if !copied[pkg] {
			t.Errorf("the binary imports syndra/%s but Dockerfile.claims does not copy it", pkg)
		}
	}
	for pkg := range copied {
		if !need[pkg] && pkg != "go.mod" && pkg != "go.sum" {
			t.Errorf("Dockerfile.claims copies %s, which the binary does not import: a change there would rebuild the claims image", pkg)
		}
	}
}

func TestConfigProblems(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	if p := configProblems(env(map[string]string{})); len(p) != 0 {
		t.Fatalf("dev mode refused: %v", p)
	}
	for key, bad := range map[string]bool{"": true, "short": true, strings.Repeat("a", 31) + "\n": true, strings.Repeat("A1", 16): false} {
		p := configProblems(env(map[string]string{"ZITADEL_DOMAIN": "zitadel.example.edu", "ZITADEL_ACTION_SIGNING_KEY": key}))
		if (len(p) > 0) != bad {
			t.Errorf("key %q: %v, want bad=%v", key, p, bad)
		}
	}
}
