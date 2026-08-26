package db

import "testing"

// The enumerated diff behind the mapping screen's version band (design M2).
//
// `Unpublished` answered whether an operator would be undoing something. This
// answers WHAT — and once the edits are named, "rolling back undoes work listed
// nowhere" stops being true, which is the whole argument for the band.
//
// No database: `classifyMappingChanges` is separated from the holder counts for
// that reason, and this package has no live-DB harness.

func live(project, role, field, value, by string) RoleMapping {
	return RoleMapping{
		Target: "truenas", ProjectID: project, RoleKey: role,
		Field: field, Value: value, CreatedBy: by,
	}
}

func published(project, role, field, value string) MappingVersionEntry {
	return MappingVersionEntry{ProjectID: project, RoleKey: role, Field: field, Value: value}
}

func only(t *testing.T, changes []MappingChange) MappingChange {
	t.Helper()
	if len(changes) != 1 {
		t.Fatalf("want exactly one change, got %d: %+v", len(changes), changes)
	}
	return changes[0]
}

// The one that matters. (target, project, role, field) is the table's own
// uniqueness constraint, so a differing value across that key cannot be two
// rows — and reporting it as an addition beside a removal would tell an
// operator that a rollback touches two mappings when it touches one, and would
// double the cohort the ceremony states.
func TestAValueThatMovedIsOneChangeAndNotTwo(t *testing.T) {
	changes := classifyMappingChanges(
		[]RoleMapping{live("p_archive", "admin", "group", "archive-write", "a.devi")},
		[]MappingVersionEntry{published("p_archive", "admin", "group", "archive-admins")},
	)

	c := only(t, changes)
	if c.Kind != "changed" {
		t.Fatalf("want changed, got %q", c.Kind)
	}
	if c.WasValue != "archive-admins" || c.Value != "archive-write" {
		t.Errorf("both sides must be carried: was %q, is %q", c.WasValue, c.Value)
	}
}

func TestABindingTheVersionDoesNotHoldIsAnAddition(t *testing.T) {
	changes := classifyMappingChanges(
		[]RoleMapping{live("p_laser", "operator", "group", "laser-users", "a.devi")},
		[]MappingVersionEntry{},
	)

	c := only(t, changes)
	if c.Kind != "added" {
		t.Fatalf("want added, got %q", c.Kind)
	}
	if c.WasValue != "" {
		t.Errorf("an addition has no previous value: %q", c.WasValue)
	}
}

// A removal has no row left, so it has no `updated_by` and no timestamp. The
// surface must be able to say "no longer here" without attributing it — naming
// the version's publisher would name somebody who did not remove it.
func TestARemovalCarriesWhatItWasAndNobodyToBlame(t *testing.T) {
	changes := classifyMappingChanges(
		nil,
		[]MappingVersionEntry{published("p_fab", "member", "group", "fabrication-read")},
	)

	c := only(t, changes)
	switch {
	case c.Kind != "removed":
		t.Fatalf("want removed, got %q", c.Kind)
	case c.WasValue != "fabrication-read":
		t.Errorf("a removal must still say what it was: %q", c.WasValue)
	case c.Value != "":
		t.Errorf("a removal has no current value: %q", c.Value)
	case c.Actor != "" || c.At != nil:
		t.Errorf("a deleted row takes its attribution with it: %q / %v", c.Actor, c.At)
	}
}

func TestAMatchingWorkingCopyHasNothingToPublish(t *testing.T) {
	set := []RoleMapping{
		live("p_fab", "lead", "group", "fabrication", "a.devi"),
		live("p_archive", "admin", "group", "archive-admins", "r.iyer"),
	}
	snapshot := []MappingVersionEntry{
		published("p_archive", "admin", "group", "archive-admins"),
		published("p_fab", "lead", "group", "fabrication"),
	}

	// Order-independently: the version is stored in its own order and the
	// working copy is read in another.
	if changes := classifyMappingChanges(set, snapshot); len(changes) != 0 {
		t.Fatalf("want nothing to publish, got %+v", changes)
	}
}

// The live deployment's real state on day two: bindings exist and no version
// does. Each one is an addition against the empty set, which is what an
// operator needs listed before they publish a first version.
func TestWithNothingPublishedEveryBindingIsAnAddition(t *testing.T) {
	changes := classifyMappingChanges(
		[]RoleMapping{
			live("p_fab", "lead", "group", "fabrication", "a.devi"),
			live("p_archive", "admin", "group", "archive-admins", "r.iyer"),
		},
		nil,
	)

	if len(changes) != 2 {
		t.Fatalf("want two additions, got %+v", changes)
	}
	for _, c := range changes {
		if c.Kind != "added" {
			t.Errorf("want added, got %q for %s/%s", c.Kind, c.ProjectID, c.RoleKey)
		}
	}
}

// Two edits on one role are one cohort. Counting them per change is the
// arithmetic the whole-version plan exists to refuse — and the classifier is
// where the two changes stay distinguishable while the role stays one.
func TestTwoChangesOnOneRoleStayTwoChangesOnOneRole(t *testing.T) {
	changes := classifyMappingChanges(
		[]RoleMapping{
			live("p_fab", "lead", "group", "fabrication-2026", "a.devi"),
			live("p_fab", "lead", "quota", "500G", "a.devi"),
		},
		[]MappingVersionEntry{published("p_fab", "lead", "group", "fabrication")},
	)

	if len(changes) != 2 {
		t.Fatalf("want two changes, got %+v", changes)
	}
	for _, c := range changes {
		if c.ProjectID != "p_fab" || c.RoleKey != "lead" {
			t.Errorf("both belong to one role: %+v", c)
		}
	}
}

// An attribution prefers who last CHANGED a row over who created it: an
// unpublished edit belongs to the edit, not to the row's origin.
func TestAnEditIsAttributedToWhoeverMadeItRatherThanWhoeverCreatedTheRow(t *testing.T) {
	m := live("p_fab", "lead", "group", "fabrication-2026", "r.iyer")
	m.UpdatedBy = "a.devi"

	c := only(t, classifyMappingChanges(
		[]RoleMapping{m},
		[]MappingVersionEntry{published("p_fab", "lead", "group", "fabrication")},
	))
	if c.Actor != "a.devi" {
		t.Errorf("want the editor, got %q", c.Actor)
	}
}
