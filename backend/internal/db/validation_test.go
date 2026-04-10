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
