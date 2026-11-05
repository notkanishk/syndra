package models

import (
	"maps"
	"testing"
)

func rule(sp, sr, tp, tr string) MappingRule {
	return MappingRule{SourceProject: sp, SourceRole: sr, TargetProject: tp, TargetRole: tr}
}

// A→B→C reaches C whatever order the rules are listed in; a one-hop walk
// (the old webhook orchestrator) reached B only.
func TestRuleClosure_FollowsAChainToTheEnd(t *testing.T) {
	a, b, c := HeldRole{"p", "a"}, HeldRole{"q", "b"}, HeldRole{"r", "c"}
	for name, rules := range map[string][]MappingRule{
		"in order":   {rule("p", "a", "q", "b"), rule("q", "b", "r", "c")},
		"reversed":   {rule("q", "b", "r", "c"), rule("p", "a", "q", "b")},
		"with noise": {rule("x", "y", "r", "c"), rule("q", "b", "r", "c"), rule("p", "a", "q", "b")},
	} {
		got := RuleClosure(map[HeldRole]bool{a: true}, rules, NewHeldRole)
		want := map[HeldRole]bool{a: true, b: true, c: true}
		if !maps.Equal(got, want) {
			t.Errorf("%s: closure = %v, want %v", name, got, want)
		}
	}
}

// A cycle (refused on insert, possible in a restored backup) ends, and a rule
// whose source nobody holds gives nothing.
func TestRuleClosure_EndsOnACycleAndIgnoresUnheldSources(t *testing.T) {
	a, b := HeldRole{"p", "a"}, HeldRole{"p", "b"}
	rules := []MappingRule{rule("p", "a", "p", "b"), rule("p", "b", "p", "a"), rule("z", "z", "p", "x")}
	got := RuleClosure(map[HeldRole]bool{a: true}, rules, NewHeldRole)
	if !maps.Equal(got, map[HeldRole]bool{a: true, b: true}) {
		t.Fatalf("closure = %v", got)
	}
}

// The base is not mutated: callers diff before against after.
func TestRuleClosure_LeavesTheBaseAlone(t *testing.T) {
	base := map[HeldRole]bool{{"p", "a"}: true}
	RuleClosure(base, []MappingRule{rule("p", "a", "p", "b")}, NewHeldRole)
	if len(base) != 1 {
		t.Fatalf("base mutated: %v", base)
	}
}
