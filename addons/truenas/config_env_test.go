package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The add-on's configuration surface and the deployment's are two definitions
// of one thing, and nothing made them agree. `TRUENAS_SHARE_HOST`,
// `MUTATION_LOG_MAX_BYTES`, `MUTATION_LOG_KEEP` and the inline `SIGNING_KEY`
// were read here and passed by no Compose service, so a Compose deployment
// could not set them at any value — and the symptom for the first of those was
// not an error but an absence: the manifest omits its connection block exactly
// as it is designed to when the host is genuinely unknown, so the member page
// dropped the mount instructions and said nothing about why.
//
// A source guard rather than a runtime check because the failure is that the
// variable never arrives. Nothing inside the process can observe a value that
// was never passed to it.
func TestEveryEnvVarTheAddOnReadsIsPassedByCompose(t *testing.T) {
	read := envNamesReadBySource(t)
	block := composeAddOnBlock(t)

	var missing []string
	for _, name := range read {
		if !strings.Contains(block, "- "+name+"=") {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("the add-on reads these and docker-compose.yml passes none of them, "+
			"so no Compose deployment can set them: %s", strings.Join(missing, ", "))
	}
}

// envNamesReadBySource collects the literal names handed to the four config
// readers. Only literals: the `_FILE` suffix inside secretValue is built at run
// time, so it is expanded here from the call site's literal instead.
func envNamesReadBySource(t *testing.T) []string {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}

	call := regexp.MustCompile(`\b(os\.Getenv|envOr|envInt|envBool|secretValue)\("([A-Z][A-Z0-9_]*)"`)
	seen := map[string]bool{}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range call.FindAllStringSubmatch(string(src), -1) {
			seen[m[2]] = true
			if m[1] == "secretValue" {
				// Both forms are real configuration. The file is preferred and
				// the inline value is the fallback, and a deployment that can
				// only reach one of them must not find that the other is the
				// only one wired.
				seen[m[2]+"_FILE"] = true
			}
		}
	}
	if len(seen) == 0 {
		t.Fatal("no environment reads found; the call pattern this guard matches has been renamed")
	}

	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// composeAddOnBlock returns the truenas-addon service only. Scoping matters:
// the backend is passed ADDON_* variables of its own, and a guard that read the
// whole file would accept a name wired into the wrong container.
func composeAddOnBlock(t *testing.T) string {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join("..", "..", "docker-compose.yml"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)

	const header = "\n  truenas-addon:\n"
	start := strings.Index(body, header)
	if start < 0 {
		t.Fatal("no truenas-addon service in docker-compose.yml")
	}
	body = body[start+len(header):]

	// The next service, or the next top-level key, whichever comes first.
	if end := regexp.MustCompile(`(?m)^(  [a-z][a-z0-9_-]*:|[a-z][a-z0-9_-]*:)$`).FindStringIndex(body); end != nil {
		body = body[:end[0]]
	}
	return body
}

// The fixture recorder authenticates with the same API key this add-on does, so
// it verifies TLS on the same terms — and the same variable, because two switches
// spelled differently are two policies.
//
// It recorded with an unconditional `ssl._create_unverified_context()`: the
// wrapper refuses a `ws://` URL because TrueNAS revokes a key presented in
// clear, and then the probe accepted any certificate at all, which hands that
// same key to whatever can answer for the NAS's address. The `wss://` check
// bought nothing.
//
// A grep, deliberately. The recorder is Python run inside a throwaway container
// and there is no Go test that can execute it; what can be asserted from here is
// that the unconditional form is not in the file and the variable is.
func TestTheFixtureRecorderDoesNotSkipTLSVerificationByDefault(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "scripts", "lib", "record-truenas.py"))
	if err != nil {
		t.Fatalf("reading the recorder: %v", err)
	}
	src := string(raw)
	if strings.Contains(src, "ssl=ssl._create_unverified_context()") {
		t.Fatal("the recorder builds an unverified TLS context inline, so every recording sends " +
			"TRUENAS_API_KEY to whatever answers for the NAS's address. Verify by default and " +
			"let TRUENAS_VERIFY_TLS opt out, as the add-on does")
	}
	if !strings.Contains(src, "TRUENAS_VERIFY_TLS") {
		t.Fatal("the recorder must read TRUENAS_VERIFY_TLS — the add-on's own switch — rather " +
			"than deciding for itself")
	}
}
