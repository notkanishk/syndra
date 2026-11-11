package contract

import (
	"io/fs"
	"os"
	"regexp"
	"strings"
	"testing"
)

// The author's guide (../AUTHORING.md) inlines fixtures so an agent reads a
// route's shape beside its rules. The fixtures stay the one source: every
// inlined block is the file it names, byte for byte, every marker names a
// file that exists, and every route openapi.yaml declares has a heading in
// the guide that names it. Read from disk: run with -count=1 after editing
// the guide (README, "Go's test cache").
const guidePath = "../AUTHORING.md"

var (
	fixtureMarker = regexp.MustCompile(`<!-- fixture: (\S+) -->`)
	// A marker, then the code block it labels, closed on its own line.
	fixtureBlock = regexp.MustCompile("(?s)<!-- fixture: (\\S+) -->\n```json\n(.*?)```\n")
	openapiPath  = regexp.MustCompile(`(?m)^  (/\S*):\s*$`)
)

func readGuide(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(guidePath)
	if err != nil {
		t.Fatalf("the guide: %v", err)
	}
	return string(b)
}

func TestTheGuidesInlineFixturesAreTheFiles(t *testing.T) {
	guide := readGuide(t)
	markers := fixtureMarker.FindAllStringSubmatch(guide, -1)
	blocks := fixtureBlock.FindAllStringSubmatch(guide, -1)
	if len(markers) == 0 {
		t.Fatal("the guide inlines no fixture; design §8 has it inline every core route's")
	}
	if len(blocks) != len(markers) {
		t.Errorf("%d fixture markers but %d marked json blocks: a marker must sit directly above its ```json block", len(markers), len(blocks))
	}
	for _, m := range markers {
		if _, err := fs.Stat(FS(), m[1]); err != nil {
			t.Errorf("marker %q names no contract file: %v", m[1], err)
		}
	}
	for _, b := range blocks {
		want, err := fs.ReadFile(FS(), b[1])
		if err != nil {
			continue // reported above
		}
		if b[2] != string(want) {
			t.Errorf("the guide's copy of %s is not the file byte for byte; paste the file again", b[1])
		}
	}
}

func TestEveryRouteHasASectionInTheGuide(t *testing.T) {
	guide := readGuide(t)
	var headings []string
	for _, line := range strings.Split(guide, "\n") {
		if strings.HasPrefix(line, "#") {
			headings = append(headings, line)
		}
	}
	spec, err := fs.ReadFile(FS(), "v2/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	paths := openapiPath.FindAllStringSubmatch(string(spec), -1)
	if len(paths) == 0 {
		t.Fatal("no paths read from openapi.yaml")
	}
	for _, p := range paths {
		// The path itself, not a longer one it prefixes: /settings is not
		// /settings/{name}.
		named := regexp.MustCompile(`(^|[^\w./{}-])` + regexp.QuoteMeta(p[1]) + `($|[^\w./{}-])`)
		found := false
		for _, h := range headings {
			found = found || named.MatchString(h)
		}
		if !found {
			t.Errorf("openapi.yaml declares %s and no heading in the guide names it", p[1])
		}
	}
}
