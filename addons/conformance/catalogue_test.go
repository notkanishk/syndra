package conformance

import (
	"bytes"
	"io/fs"
	"os"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	contractfiles "syndra/addons/contract"
)

// The author has the guide, not the design: every check cites a section of
// AUTHORING.md by an anchor a heading makes (GitHub's slug, as
// addons/contract/guide_test.go reads it), and no text the kit prints cites
// the design's numbering.
func TestEveryCheckCitesAGuideSection(t *testing.T) {
	guide, err := os.ReadFile("../AUTHORING.md")
	if err != nil {
		t.Fatal(err)
	}
	notSlug := regexp.MustCompile(`[^\p{L}\p{N} _-]`)
	slugs := map[string]bool{}
	for _, line := range strings.Split(string(guide), "\n") {
		if title, ok := strings.CutPrefix(line, "#"); ok {
			title = strings.ToLower(strings.TrimSpace(strings.TrimLeft(title, "#")))
			slugs[strings.ReplaceAll(notSlug.ReplaceAllString(title, ""), " ", "-")] = true
		}
	}
	design := regexp.MustCompile(`§|decision \d|design|slice \d|admin\.adopt`)
	for _, e := range catalogue {
		if !slugs[e.guide] {
			t.Errorf("%s cites AUTHORING.md#%s, which no heading makes", e.id, e.guide)
		}
		if design.MatchString(e.must) {
			t.Errorf("%s cites the design, which the author does not have: %q", e.id, e.must)
		}
	}
	for _, f := range []string{"kit.go", "reads.go", "mutate.go", "owner.go", "report.go"} {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, lit := range regexp.MustCompile(`"(?:[^"\\]|\\.)*"`).FindAll(src, -1) {
			if design.Match(lit) {
				t.Errorf("%s prints %s, which cites the design", f, lit)
			}
		}
	}
}

// Every registry operation says whether it is about a member (`subject`):
// the kit reads it to excuse a reads-only 422 only for a read about a
// subject, so an entry that does not say is refused at load, never read as
// either.
func TestTheRegistryMustSayWhetherAnOperationIsAboutASubject(t *testing.T) {
	if _, err := loadContract(contractfiles.FS()); err != nil {
		t.Fatalf("the embedded contract: %v", err)
	}
	files := fstest.MapFS{}
	err := fs.WalkDir(contractfiles.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		raw, err := fs.ReadFile(contractfiles.FS(), p)
		if p == "v2/registry/operations.json" {
			raw = bytes.Replace(raw, []byte(`"subject": false,`), nil, 1)
		}
		files[p] = &fstest.MapFile{Data: raw}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := loadContract(files); err == nil || !strings.Contains(err.Error(), "health.get does not say") {
		t.Errorf("a registry entry without subject loaded: %v", err)
	}
}
