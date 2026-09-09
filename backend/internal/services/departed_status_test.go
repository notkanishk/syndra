package services

import "testing"

// The departed guard has to speak the vocabulary the directory produces.
//
// It did not. The list was "departed", "inactive", "alumni", "deactivated" —
// and `directory.normalizeUserState` emits `active | inactive | initial |
// locked | deleted`. Three of the four words were unreachable, and the two
// states that most obviously mean "do not give this person anything" were
// missing, so a LOCKED or DELETED Zitadel account passed the bulk-grant guard.
//
// Three separate copies of this predicate agreed with each other perfectly
// (this one, the people filter, the drift-triage label) and all three were
// wrong the same way, which is what agreement between copies is worth when
// none of them is checked against the producer.
//
// So this test is written against the PRODUCER's vocabulary rather than against
// the predicate's own list. A state added to normalizeUserState has to be
// classified here deliberately.
func TestIsDepartedStatus_CoversWhatTheDirectoryEmits(t *testing.T) {
	// Every value directory.normalizeUserState can return, and what a bulk
	// grant should do about it.
	emitted := map[string]bool{
		"active":   false, // the ordinary case
		"initial":  false, // invited, not yet signed in — arriving, not gone
		"inactive": true,
		"locked":   true,
		"deleted":  true,
		// normalizeUserState's own answer for an empty state. Not a leaver:
		// refusing on "we could not read it" would block grants on a directory
		// hiccup, and the rehearsal shows the operator each row before applying.
		"unspecified": false,
	}

	for state, wantDeparted := range emitted {
		if got := isDepartedStatus(state); got != wantDeparted {
			t.Errorf("isDepartedStatus(%q) = %v, want %v — the directory emits this state and the "+
				"guard has not been taught what it means", state, got, wantDeparted)
		}
	}
}

// Case and whitespace come from whatever fed the status in, and a leaver who
// slips through on " Deleted " is a leaver who slipped through.
func TestIsDepartedStatus_IgnoresCaseAndSpacing(t *testing.T) {
	for _, s := range []string{"DELETED", " deleted ", "Locked", "  INACTIVE"} {
		if !isDepartedStatus(s) {
			t.Errorf("isDepartedStatus(%q) = false, want true", s)
		}
	}
}
