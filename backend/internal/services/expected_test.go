package services

import (
	"testing"

	"syndra/internal/models"
	"syndra/internal/zitadel"
)

func TestExpectedViaRule_UserHoldingSourceMakesTargetExpected(t *testing.T) {
	// Rule: holding p1:member derives p2:contributor.
	rules := []models.MappingRule{{SourceProject: "p1", SourceRole: "member", TargetProject: "p2", TargetRole: "contributor"}}
	// The user holds the source (as a Zitadel grant); the derived target is therefore expected, not drift.
	holder := BuildHolderSet(
		nil,
		[]zitadel.UserGrant{{UserID: "u1", ProjectID: "p1", RoleKeys: []string{"member"}}},
	)
	if !ExpectedViaRule(holder, rules, "u1", "p2", "contributor") {
		t.Fatal("target of a fired rule must be expected_via_rule")
	}
	// A user NOT holding the source must not have the target explained by the rule.
	if ExpectedViaRule(holder, rules, "u2", "p2", "contributor") {
		t.Fatal("rule must not explain a target for a user lacking the source")
	}
}

func TestIsExcluded_MatchesTriple(t *testing.T) {
	ex := []models.ExternalGrantExclusion{{UserID: "u1", ProjectID: "p1", RoleKey: "viewer"}}
	if !IsExcluded(ex, "u1", "p1", "viewer") {
		t.Fatal("marked-external triple must be excluded")
	}
	if IsExcluded(ex, "u1", "p1", "editor") {
		t.Fatal("a different role must not be excluded")
	}
}
