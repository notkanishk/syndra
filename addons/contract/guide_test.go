package contract

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The author's guide (../AUTHORING.md) inlines fixtures so an agent reads a
// route's shape beside its rules. The fixtures stay the one source: every
// JSON block is marked, every inlined block is the file it names, byte for
// byte, every marker names a file that exists, every route openapi.yaml
// declares has a heading naming its method and path, every
// "`symbol` in `path`" citation names a symbol that path declares, and every
// link to a section lands on a heading. Read from
// disk: run with -count=1 after editing the guide (README, "Go's test cache").
const guidePath = "../AUTHORING.md"

// repoRoot is where the guide's paths start, except those under v2/ and the
// files directly in addons/contract/, which start here.
const repoRoot = "../.."

var (
	fixtureMarker = regexp.MustCompile(`<!-- fixture: (\S+) -->`)
	// A marker, then the code block it labels, closed on its own line.
	fixtureBlock = regexp.MustCompile("(?s)<!-- fixture: (\\S+) -->\n```json\n(.*?)```\n")
	// A path at two spaces, then each of its methods at four.
	openapiPath   = regexp.MustCompile(`(?m)^  (/\S*):\s*$`)
	openapiMethod = regexp.MustCompile(`(?m)^    (get|put|post|delete|patch):\s*$`)
	// `name` in `path/to/file.ext`
	// A link to a section of the guide, and what GitHub keeps of a heading
	// in its anchor.
	anchorLink = regexp.MustCompile(`\]\(#([^)]+)\)`)
	notSlug    = regexp.MustCompile(`[^\p{L}\p{N} _-]`)
	citation   = regexp.MustCompile("`([A-Za-z_][A-Za-z0-9_]*)`\\s+in\\s+`([A-Za-z0-9_./-]+\\.[a-z]+)`")
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
	if n := strings.Count(guide, "```json"); n != len(markers) {
		t.Errorf("%d json blocks and %d fixture markers: every json block is a fixture, marked", n, len(markers))
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

// Every route openapi.yaml declares, as METHOD /path, is named in a heading:
// a heading that names the path but not the method is not the route's.
func TestEveryRouteHasASectionInTheGuide(t *testing.T) {
	guide := readGuide(t)
	var headings []string
	for _, line := range strings.Split(guide, "\n") {
		if strings.HasPrefix(line, "#") {
			headings = append(headings, line)
		}
	}
	routes := openapiRoutes(t)
	if len(routes) == 0 {
		t.Fatal("no routes read from openapi.yaml")
	}
	for _, route := range routes {
		// The path itself, not a longer one it prefixes: /settings is not
		// /settings/{name}.
		named := regexp.MustCompile(`(^|[^\w./{}-])` + regexp.QuoteMeta(route) + `($|[^\w./{}-])`)
		found := false
		for _, h := range headings {
			found = found || named.MatchString(h)
		}
		if !found {
			t.Errorf("openapi.yaml declares %s and no heading in the guide names it", route)
		}
	}
}

// openapiRoutes is every "METHOD /path" openapi.yaml declares.
func openapiRoutes(t *testing.T) []string {
	t.Helper()
	spec, err := fs.ReadFile(FS(), "v2/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	text := string(spec)
	paths := openapiPath.FindAllStringSubmatchIndex(text, -1)
	var routes []string
	for i, p := range paths {
		end := len(text)
		if i+1 < len(paths) {
			end = paths[i+1][0]
		}
		methods := openapiMethod.FindAllStringSubmatch(text[p[1]:end], -1)
		if len(methods) == 0 {
			t.Errorf("openapi.yaml declares %s with no method", text[p[2]:p[3]])
		}
		for _, m := range methods {
			routes = append(routes, strings.ToUpper(m[1])+" "+text[p[2]:p[3]])
		}
	}
	return routes
}

// Every "`symbol` in `path`" names a file that exists and declares the symbol,
// so a citation survives edits to the file and fails when the symbol goes.
// A Go file must declare it (func, method, type, var, const, or a name in a
// var or const block); any other file must contain it as a whole word.
func TestEveryCitedSymbolExists(t *testing.T) {
	guide := readGuide(t)
	cites := citation.FindAllStringSubmatch(guide, -1)
	if len(cites) == 0 {
		t.Fatal("the guide cites no symbol in a file")
	}
	for _, c := range cites {
		name, path := c[1], c[2]
		file := filepath.Join(repoRoot, path)
		if strings.HasPrefix(path, "v2/") || !strings.Contains(path, "/") {
			file = path
		}
		src, err := os.ReadFile(file)
		if err != nil {
			t.Errorf("`%s` in `%s`: %v", name, path, err)
			continue
		}
		q := regexp.QuoteMeta(name)
		declared := regexp.MustCompile(`\b` + q + `\b`)
		if strings.HasSuffix(path, ".go") {
			declared = regexp.MustCompile(`(?m)^(func (\([^)]*\) )?` + q + `\b|(type|var|const) ` + q + `\b|\t` + q + `\s+(\S+\s+)?=)`)
		}
		if !declared.Match(src) {
			t.Errorf("the guide cites `%s` in `%s`, which does not declare it", name, path)
		}
	}
}

// Every "](#anchor)" names a heading of the guide, by GitHub's slug: one place
// per rule works only if the link to it lands.
func TestEveryAnchorLinkLands(t *testing.T) {
	guide := readGuide(t)
	slugs := map[string]bool{}
	for _, line := range strings.Split(guide, "\n") {
		if title, ok := strings.CutPrefix(line, "#"); ok {
			title = strings.TrimLeft(title, "#")
			slugs[strings.ReplaceAll(notSlug.ReplaceAllString(strings.ToLower(strings.TrimSpace(title)), ""), " ", "-")] = true
		}
	}
	links := anchorLink.FindAllStringSubmatch(guide, -1)
	if len(links) == 0 {
		t.Fatal("the guide links to none of its sections")
	}
	for _, l := range links {
		if !slugs[l[1]] {
			t.Errorf("the guide links to #%s, which no heading makes", l[1])
		}
	}
}
