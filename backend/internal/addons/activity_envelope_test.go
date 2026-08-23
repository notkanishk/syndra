package addons

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The one thing that can silently disagree across the two modules.
//
// `Activity` decodes `{"activity": {...}}` out of the operation response, and
// the add-on decides that key in its own `OperationResult`. Nothing else joins
// the two: the cross-binary test reaches `/capabilities` and stops, and the
// contract artifacts describe the REQUEST. So each side is tested against its
// own fixture, both pass, and a rename on either one produces a report that
// decodes to nothing while every suite stays green.
//
// That is the defect shape this whole platform kept producing — two internally
// consistent definitions of one thing — and it is worth one file to close on
// the operation that had no caller at all until now.
func TestActivityEnvelopeKeyMatchesTheAddOn(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "..", "addons", "truenas", "operations.go"))
	if err != nil {
		// Loud, not skipped. A guard that quietly passes when it cannot find
		// what it guards is worse than no guard: it reports agreement it never
		// checked.
		t.Fatalf("could not read the add-on's operations.go, so this guard proved nothing: %v", err)
	}

	field := regexp.MustCompile(`Activity\s+\*ActivityReport\s+` + "`" + `json:"([^",]+)`)
	match := field.FindSubmatch(source)
	if match == nil {
		t.Fatal("the add-on no longer declares Activity on its OperationResult under a json tag this can read")
	}
	if got := string(match[1]); got != "activity" {
		t.Fatalf("the add-on emits the report under %q; addons.Activity decodes \"activity\"", got)
	}

	// The fields the surface actually renders. A rename here does not fail to
	// decode — it decodes to a zero value, which renders as "no events" and
	// "nothing unaudited", the two answers this read exists to distinguish
	// from their opposites.
	for _, tag := range []string{`json:"events"`, `json:"unaudited_shares,omitempty"`} {
		if !strings.Contains(string(source), tag) {
			t.Fatalf("the add-on no longer carries %s; the report would decode to a reassuring zero", tag)
		}
	}
}
