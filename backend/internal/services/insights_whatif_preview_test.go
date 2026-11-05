package services

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"syndra/internal/models"
)

// The rule editor's preview follows the whole chain from the core: adding a→b
// where b→c exists gives the holder of a both b and c. The editor used to say
// "would give them b" and name the chained rule in a separate line.
func TestWhatIf_RuleAddedFollowsTheChain(t *testing.T) {
	a, b, c := roleKey{"p", "a"}, roleKey{"p", "b"}, roleKey{"q", "c"}
	w := newWhatIfWorld("ann", "bea")
	w.direct["ann"] = []roleKey{a}
	w.rules = []models.MappingRule{{ID: "r2", SourceProject: "p", SourceRole: "b", TargetProject: "q", TargetRole: "c"}}
	got := w.ask(t, models.InsightsWhatIfChange{Kind: "rule_added", ProjectID: "p", RoleKey: "a", TargetProjectID: "p", TargetRoleKey: "b"})
	if len(got.Gains) != 1 || got.Gains[0].UserID != "ann" || !reflect.DeepEqual(got.Gains[0].Roles, wiGrants(b, c)) {
		t.Fatalf("gains = %+v", got.Gains)
	}
	if len(got.Losses) != 0 || got.Unchanged != 1 {
		t.Fatalf("losses %+v unchanged %d", got.Losses, got.Unchanged)
	}
}

// An edit replaces the rule it names: moving r1's target from b to d takes b
// away and gives d.
func TestWhatIf_RuleAddedReplacingARuleIsAnEdit(t *testing.T) {
	a, b, d := roleKey{"p", "a"}, roleKey{"p", "b"}, roleKey{"p", "d"}
	w := newWhatIfWorld("ann")
	w.direct["ann"] = []roleKey{a}
	w.rules = []models.MappingRule{{ID: "r1", SourceProject: "p", SourceRole: "a", TargetProject: "p", TargetRole: "b"}}
	got := w.ask(t, models.InsightsWhatIfChange{Kind: "rule_added", RuleID: "r1", ProjectID: "p", RoleKey: "a", TargetProjectID: "p", TargetRoleKey: "d"})
	if len(got.Gains) != 1 || !reflect.DeepEqual(got.Gains[0].Roles, wiGrants(d)) ||
		len(got.Losses) != 1 || !reflect.DeepEqual(got.Losses[0].Roles, wiGrants(b)) {
		t.Fatalf("gains %+v losses %+v", got.Gains, got.Losses)
	}
}

// One person's bundle dialog: removing b1 takes what only b1 gave, plus every
// rule target that only it reached; a role another source gives is kept.
func TestWhatIf_BundleUnassignedForOnePerson(t *testing.T) {
	a, x, b := roleKey{"p", "a"}, roleKey{"p", "x"}, roleKey{"p", "b"}
	w := newWhatIfWorld("ann", "bea")
	w.holds("ann", a, x)
	w.direct["ann"] = []roleKey{x}
	w.holds("bea", a)
	w.rules = []models.MappingRule{{ID: "r1", SourceProject: "p", SourceRole: "a", TargetProject: "p", TargetRole: "b"}}
	got := w.ask(t, models.InsightsWhatIfChange{Kind: "bundle_unassigned", BundleID: "b1", UserID: "ann"})
	if len(got.Losses) != 1 || got.Losses[0].UserID != "ann" || !reflect.DeepEqual(got.Losses[0].Roles, wiGrants(a, b)) {
		t.Fatalf("losses = %+v (bea must not appear; x stays through the direct grant)", got.Losses)
	}
}

// Assigning gives the latest version's roles and what the rules derive from them.
func TestWhatIf_BundleAssignedForOnePerson(t *testing.T) {
	a, b := roleKey{"p", "a"}, roleKey{"p", "b"}
	w := newWhatIfWorld("ann")
	w.rules = []models.MappingRule{{ID: "r1", SourceProject: "p", SourceRole: "a", TargetProject: "p", TargetRole: "b"}}
	w.install(t)
	svcLatestVersionRoles = func(context.Context, string) (models.BundleVersion, []models.BundleRole, error) {
		return models.BundleVersion{}, []models.BundleRole{{BundleID: "b1", ProjectID: "p", RoleKey: "a"}}, nil
	}
	got, err := InsightsWhatIf(context.Background(), models.InsightsWhatIfChange{Kind: "bundle_assigned", BundleID: "b1", UserID: "ann"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Gains) != 1 || !reflect.DeepEqual(got.Gains[0].Roles, wiGrants(a, b)) {
		t.Fatalf("gains = %+v", got.Gains)
	}
}

func TestWhatIf_NewKindsValidate(t *testing.T) {
	for _, c := range []models.InsightsWhatIfChange{
		{Kind: "rule_added", ProjectID: "p", RoleKey: "a", TargetProjectID: "p"},
		{Kind: "bundle_assigned", BundleID: "b1"},
		{Kind: "bundle_unassigned", UserID: "u", BundleID: "b1", RoleKey: "x"},
		{Kind: "rule_gone", RuleID: "r1", UserID: "u"},
	} {
		if err := validateWhatIf(c); !errors.Is(err, ErrInvalidWhatIf) {
			t.Errorf("%+v: err = %v, want invalid", c, err)
		}
	}
}
