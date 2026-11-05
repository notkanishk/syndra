package access

import (
	"maps"
	"testing"
	"time"

	"syndra/internal/models"
)

func rule(sp, sr, tp, tr string) models.MappingRule {
	return models.MappingRule{SourceProject: sp, SourceRole: sr, TargetProject: tp, TargetRole: tr}
}

// A→B→C reaches C whatever order the rules are listed in; a one-hop walk
// (the old webhook orchestrator) reached B only.
func TestClosure_FollowsAChainToTheEnd(t *testing.T) {
	a, b, c := Role{"p", "a"}, Role{"q", "b"}, Role{"r", "c"}
	for name, rules := range map[string][]models.MappingRule{
		"in order":   {rule("p", "a", "q", "b"), rule("q", "b", "r", "c")},
		"reversed":   {rule("q", "b", "r", "c"), rule("p", "a", "q", "b")},
		"with noise": {rule("x", "y", "r", "c"), rule("q", "b", "r", "c"), rule("p", "a", "q", "b")},
	} {
		got := Closure(map[Role]bool{a: true}, rules, NewRole)
		want := map[Role]bool{a: true, b: true, c: true}
		if !maps.Equal(got, want) {
			t.Errorf("%s: closure = %v, want %v", name, got, want)
		}
	}
}

// A cycle (refused on insert, possible in a restored backup) ends, and a rule
// whose source nobody holds gives nothing.
func TestClosure_EndsOnACycleAndIgnoresUnheldSources(t *testing.T) {
	a, b := Role{"p", "a"}, Role{"p", "b"}
	rules := []models.MappingRule{rule("p", "a", "p", "b"), rule("p", "b", "p", "a"), rule("z", "z", "p", "x")}
	got := Closure(map[Role]bool{a: true}, rules, NewRole)
	if !maps.Equal(got, map[Role]bool{a: true, b: true}) {
		t.Fatalf("closure = %v", got)
	}
}

// The base is not mutated: callers diff before against after. A false entry
// is not held.
func TestClosure_LeavesTheBaseAloneAndSkipsFalse(t *testing.T) {
	base := map[Role]bool{{"p", "a"}: true, {"p", "z"}: false}
	got := Closure(base, []models.MappingRule{rule("p", "a", "p", "b"), rule("p", "z", "p", "y")}, NewRole)
	if len(base) != 2 {
		t.Fatalf("base mutated: %v", base)
	}
	if !maps.Equal(got, map[Role]bool{{"p", "a"}: true, {"p", "b"}: true}) {
		t.Fatalf("closure = %v", got)
	}
}

var (
	june = time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)
	dec  = time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
)

// A rule target ends when its source ends, down the whole chain. This is the
// defect W2a fixes: the compiler marked every rule target as never ending.
func TestEnds_ARuleTargetEndsWhenItsSourceEnds(t *testing.T) {
	a, b, c := Role{"p", "a"}, Role{"q", "b"}, Role{"r", "c"}
	rules := []models.MappingRule{rule("q", "b", "r", "c"), rule("p", "a", "q", "b")}
	got := Ends(map[Role]time.Time{a: june}, rules, NewRole)
	for _, k := range []Role{a, b, c} {
		if !got[k].Equal(june) {
			t.Errorf("%v ends %v, want %v", k, got[k], june)
		}
	}
}

// Two sources: the target lasts as long as the later one. A source with no end
// makes the target endless; a direct grant of the target with a later end wins.
func TestEnds_TheLaterSourceWinsAndNoEndWinsOverAll(t *testing.T) {
	a, b, t1 := Role{"p", "a"}, Role{"p", "b"}, Role{"p", "t"}
	rules := []models.MappingRule{rule("p", "a", "p", "t"), rule("p", "b", "p", "t")}

	if got := Ends(map[Role]time.Time{a: june, b: dec}, rules, NewRole)[t1]; !got.Equal(dec) {
		t.Errorf("two dated sources: target ends %v, want %v", got, dec)
	}
	if got := Ends(map[Role]time.Time{a: june, b: {}}, rules, NewRole)[t1]; !got.IsZero() {
		t.Errorf("one endless source: target ends %v, want never", got)
	}
	if got := Ends(map[Role]time.Time{a: june, t1: dec}, rules, NewRole)[t1]; !got.Equal(dec) {
		t.Errorf("held directly until later: target ends %v, want %v", got, dec)
	}
	if got := Ends(map[Role]time.Time{a: dec, t1: june}, rules, NewRole)[t1]; !got.Equal(dec) {
		t.Errorf("rule outlasts the direct grant: target ends %v, want %v", got, dec)
	}
}

// A late value still travels the whole chain whatever the rule order, and a
// cycle carrying dates still ends.
func TestEnds_LateValuesTravelAndCyclesEnd(t *testing.T) {
	a, b, c, d := Role{"p", "a"}, Role{"p", "b"}, Role{"p", "c"}, Role{"p", "d"}
	// d→c→b and a cycle b→c; d is held until dec, a until june.
	rules := []models.MappingRule{rule("p", "b", "p", "c"), rule("p", "c", "p", "b"), rule("p", "a", "p", "b"), rule("p", "d", "p", "c")}
	got := Ends(map[Role]time.Time{a: june, d: dec}, rules, NewRole)
	if !got[b].Equal(dec) || !got[c].Equal(dec) {
		t.Fatalf("b ends %v, c ends %v; want both %v", got[b], got[c], dec)
	}
}

// Closure and Ends are one fixpoint: same keys for any input.
func TestClosureIsEndsWithoutDates(t *testing.T) {
	rules := []models.MappingRule{rule("p", "a", "p", "b"), rule("p", "b", "q", "c"), rule("x", "x", "q", "c")}
	base := map[Role]time.Time{{"p", "a"}: june}
	set := Closure(map[Role]bool{{"p", "a"}: true}, rules, NewRole)
	dated := Ends(base, rules, NewRole)
	if len(set) != len(dated) {
		t.Fatalf("closure %v, ends %v", set, dated)
	}
	for k := range dated {
		if !set[k] {
			t.Fatalf("%v in ends, not in closure", k)
		}
	}
}

// Fired names every rule whose source is held, including ones down a chain.
func TestFired_EveryRuleWhoseSourceIsInTheClosure(t *testing.T) {
	rules := []models.MappingRule{rule("p", "a", "p", "b"), rule("p", "b", "p", "c"), rule("p", "z", "p", "c")}
	closure := Closure(map[Role]bool{{"p", "a"}: true}, rules, NewRole)
	got := Fired(closure, rules, NewRole)
	if len(got) != 2 || got[0] != rules[0] || got[1] != rules[1] {
		t.Fatalf("fired = %v", got)
	}
}

func TestLeft(t *testing.T) {
	for _, s := range []string{"inactive", "Locked", " deleted ", "departed", "alumni", "deactivated"} {
		if !Left(s) {
			t.Errorf("Left(%q) = false", s)
		}
	}
	for _, s := range []string{"", "active", "initial", "unspecified", "unknown"} {
		if Left(s) {
			t.Errorf("Left(%q) = true", s)
		}
	}
}
