package reference

import (
	"slices"
	"testing"
)

// lobby is outside the managed set: an apply emptying the field keeps it on
// the target, and state never shows it. The kit sees only the second half,
// so this test holds the first.
func TestAnUnlistedValueIsKeptAndNotShown(t *testing.T) {
	acct := &account{state: map[string]any{"enabled": true, "printers": []string{lobby, "mk4-a"}}}
	acct.state = converge(acct.state, map[string]any{"printers": []string{}})
	if got := acct.state["printers"].([]string); !slices.Equal(got, []string{lobby}) {
		t.Errorf("an apply emptying printers left %v on the target, want [%s]", got, lobby)
	}
	if got := acct.shown()["printers"].([]string); len(got) != 0 {
		t.Errorf("state shows %v, want []: lobby is not in the managed set", got)
	}
}
