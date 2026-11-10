package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The Go roleLabel and the UI's humanizeKey read one fixture, so a role key
// made readable is the same words on the map's placeholder node as on every
// screen (one-path W2d). ui/src/lib/__tests__/names.test.ts reads the same file.
func TestRoleLabelMatchesHumanizeKey(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "ui", "src", "lib", "__fixtures__", "humanize-key.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct{ Key, Label string }
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) < 6 {
		t.Fatalf("fixture has %d cases", len(cases))
	}
	for _, c := range cases {
		if got := roleLabel(c.Key); got != c.Label {
			t.Errorf("roleLabel(%q) = %q, UI says %q", c.Key, got, c.Label)
		}
	}
}
