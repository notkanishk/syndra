package db

import (
	"testing"

	"mkauth/internal/models"
)

func TestHasCycleWithRules(t *testing.T) {
	tests := []struct {
		name          string
		rules         []models.MappingRule
		sourceProject string
		sourceRole    string
		targetProject string
		targetRole    string
		wantCycle     bool
	}{
		{
			name: "no cycle with disconnected graph",
			rules: []models.MappingRule{
				{SourceProject: "p1", SourceRole: "r1", TargetProject: "p2", TargetRole: "r2"},
			},
			sourceProject: "p3",
			sourceRole:    "r3",
			targetProject: "p4",
			targetRole:    "r4",
			wantCycle:     false,
		},
		{
			name: "direct cycle",
			rules: []models.MappingRule{
				{SourceProject: "p1", SourceRole: "r1", TargetProject: "p2", TargetRole: "r2"},
			},
			sourceProject: "p2",
			sourceRole:    "r2",
			targetProject: "p1",
			targetRole:    "r1",
			wantCycle:     true,
		},
		{
			name: "indirect cycle",
			rules: []models.MappingRule{
				{SourceProject: "p1", SourceRole: "r1", TargetProject: "p2", TargetRole: "r2"},
				{SourceProject: "p2", SourceRole: "r2", TargetProject: "p3", TargetRole: "r3"},
			},
			sourceProject: "p3",
			sourceRole:    "r3",
			targetProject: "p1",
			targetRole:    "r1",
			wantCycle:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasCycleWithRules(tt.rules, tt.sourceProject, tt.sourceRole, tt.targetProject, tt.targetRole)
			if got != tt.wantCycle {
				t.Fatalf("hasCycleWithRules()=%v, want %v", got, tt.wantCycle)
			}
		})
	}
}

// TestExcludeRuleFromGraph_UpdateVsInsertCycleDifference guards the reason DetectCycleOnUpdate
// exists: DetectCycleOnInsert loads ALL rules, so re-pointing an existing rule would still see its
// OWN old edge in the graph and could be falsely rejected as a cycle. DetectCycleOnUpdate must
// exclude that old edge first.
func TestExcludeRuleFromGraph_UpdateVsInsertCycleDifference(t *testing.T) {
	rules := []models.MappingRule{
		{ID: "rule1", SourceProject: "p1", SourceRole: "r1", TargetProject: "p2", TargetRole: "r2"},
	}
	// Retargeting rule1 to p2:r2 -> p1:r1 (the reverse edge) only cycles because rule1's OWN old
	// edge (p1:r1 -> p2:r2) is still in the graph.
	if !hasCycleWithRules(rules, "p2", "r2", "p1", "r1") {
		t.Fatal("sanity check: the raw (unfiltered) graph should see this as a cycle")
	}

	// DetectCycleOnInsert's behavior: no exclusion, so the update would be falsely rejected.
	if !hasCycleWithRules(rules, "p2", "r2", "p1", "r1") {
		t.Fatal("expected DetectCycleOnInsert-style check to (falsely) flag a cycle")
	}

	// DetectCycleOnUpdate's behavior: exclude rule1's own old edge first — the retarget is valid.
	filtered := excludeRuleFromGraph(rules, "rule1")
	if hasCycleWithRules(filtered, "p2", "r2", "p1", "r1") {
		t.Fatal("excluding the edited rule's own old edge should accept this retarget, but it was flagged as a cycle")
	}
}

func TestExcludeRuleFromGraph_KeepsOtherRules(t *testing.T) {
	rules := []models.MappingRule{
		{ID: "rule1", SourceProject: "p1", SourceRole: "r1", TargetProject: "p2", TargetRole: "r2"},
		{ID: "rule2", SourceProject: "p2", SourceRole: "r2", TargetProject: "p3", TargetRole: "r3"},
	}
	filtered := excludeRuleFromGraph(rules, "rule1")
	if len(filtered) != 1 || filtered[0].ID != "rule2" {
		t.Fatalf("expected only rule2 to survive, got %+v", filtered)
	}
}
