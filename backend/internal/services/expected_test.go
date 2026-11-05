package services

import (
	"context"
	"testing"

	"syndra/internal/models"
)

func TestRuleDerived_HoldingTheSourceMakesTheTargetExpected(t *testing.T) {
	// Rule: holding p1:member derives p2:contributor.
	rules := []models.MappingRule{{SourceProject: "p1", SourceRole: "member", TargetProject: "p2", TargetRole: "contributor"}}
	got := RuleDerived(BuildHolderSet([]models.DirectGrant{{UserID: "u1", ProjectID: "p1", RoleKey: "member"}}, nil), rules)
	if !got[HolderKey{"u1", "p2", "contributor"}] {
		t.Fatal("target of a fired rule must be expected")
	}
	// A user NOT holding the source must not have the target explained by the rule.
	if got[HolderKey{"u2", "p2", "contributor"}] {
		t.Fatal("rule must not explain a target for a user lacking the source")
	}
}

// A→B→C: the user holds only A, so C is expected through B. A user holding
// nothing on the chain, and a role no rule reaches, are still not explained.
func TestRuleDerived_FollowsChains(t *testing.T) {
	rules := []models.MappingRule{
		{SourceProject: "p1", SourceRole: "a", TargetProject: "p1", TargetRole: "b"},
		{SourceProject: "p1", SourceRole: "b", TargetProject: "p2", TargetRole: "c"},
		{SourceProject: "p2", SourceRole: "c", TargetProject: "p1", TargetRole: "a"}, // cycle must terminate
	}
	got := RuleDerived(BuildHolderSet([]models.DirectGrant{{UserID: "u1", ProjectID: "p1", RoleKey: "a"}}, nil), rules)
	if !got[HolderKey{"u1", "p2", "c"}] {
		t.Fatal("a chained grant (A→B→C, user holds A) must be expected, not drift")
	}
	if got[HolderKey{"u2", "p2", "c"}] {
		t.Fatal("a user holding nothing on the chain must not be explained")
	}
	if got[HolderKey{"u1", "p3", "x"}] {
		t.Fatal("a role no rule reaches must not be explained")
	}
}

// Parity with the person view: for every person and every role, the sweep's
// rule answer is true exactly when collectUserRoles lists a "mapping" reason
// for that role. Same records in, same rules, same answer.
func TestRuleDerived_AgreesWithThePersonView(t *testing.T) {
	f := parityFixture()
	f.install(t)
	held := map[HolderKey]bool{}
	for u, roles := range f.records() {
		for k := range roles {
			held[HolderKey{u, k.ProjectID, k.RoleKey}] = true
		}
	}
	derived := RuleDerived(held, f.rules)
	for _, u := range f.people() {
		roleMap, _, err := collectUserRoles(context.Background(), u)
		if err != nil {
			t.Fatal(err)
		}
		for k, r := range roleMap {
			viaRule := false
			for _, reason := range r.Reasons {
				viaRule = viaRule || reason.Kind == "mapping"
			}
			if got := derived[HolderKey{u, k.projectID, k.roleKey}]; got != viaRule {
				t.Errorf("%s %s:%s: sweep says rule-derived=%v, person view says %v", u, k.projectID, k.roleKey, got, viaRule)
			}
		}
		for hk := range derived {
			if hk.UserID == u && roleMap[roleKey{hk.ProjectID, hk.RoleKey}] == nil {
				t.Errorf("%s %s:%s: sweep derives a role the person view does not list", u, hk.ProjectID, hk.RoleKey)
			}
		}
	}
}

func TestIsExcluded_MatchesTriple(t *testing.T) {
	ex := []models.ExternalGrantExclusion{{Target: "zitadel", UserID: "u1", ProjectID: "p1", RoleKey: "viewer"}}
	if !IsExcluded(ex, "zitadel", "u1", "p1", "viewer") {
		t.Fatal("marked-external triple must be excluded")
	}
	if IsExcluded(ex, "zitadel", "u1", "p1", "editor") {
		t.Fatal("a different role must not be excluded")
	}
}

// An exclusion is a decision about one target. Handed a set that spans targets
// — which any caller may do, since this function is exported and pure — it must
// not let a decision made about TrueNAS answer a question about Zitadel.
func TestIsExcluded_DoesNotCrossTargets(t *testing.T) {
	ex := []models.ExternalGrantExclusion{{Target: "truenas", UserID: "u1", ProjectID: "p1", RoleKey: "viewer"}}
	if IsExcluded(ex, "zitadel", "u1", "p1", "viewer") {
		t.Fatal("an exclusion recorded on another target must not silence this one")
	}
	if !IsExcluded(ex, "truenas", "u1", "p1", "viewer") {
		t.Fatal("the exclusion must still hold on the target it was recorded against")
	}
}
