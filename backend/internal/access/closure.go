// Package access is the one core every "who holds what" answer goes through:
// the mapping-rule closure (with when each role ends), which rules fire, and
// who counts as a person still here. Pure: no I/O, no clock, so the token
// compiler, the cascades, the person view, drift and every count can call the
// same code and cannot disagree about what a chain reaches (ADR-006, W2a).
//
// What is held is read elsewhere (records by services and cache, Zitadel by the
// observation store); this package only follows the rules from it.
package access

import (
	"time"

	"syndra/internal/models"
)

// Role is one (project, role) pair.
type Role struct{ ProjectID, RoleKey string }

// NewRole is Role's key function for Closure, Ends and Fired.
func NewRole(projectID, roleKey string) Role { return Role{projectID, roleKey} }

// Ends is the one fixpoint: base plus every role a chain of rules derives from
// it (A→B→C), each with the time it ends. The zero time means it never ends.
//
// A role a rule derives ends when the last of its sources ends: it is held for
// as long as something that gives it is held, and no longer. A role held both
// directly and through a rule ends at the later of the two. The token compiler
// used to mark every rule target as never ending, so a token said a rule role
// outlived the grant it came from.
//
// Generic over the key so each caller keeps its own key type and pays no copy;
// key builds that key from a (project, role) pair. At most len(rules) passes:
// every value is the end of some base role carried along a path, the best path
// to any role is a simple one of at most len(rules) rules, and each pass
// carries every value at least one rule further. So a cycle (refused on insert,
// possible in a restored backup) still ends.
func Ends[K comparable](base map[K]time.Time, rules []models.MappingRule, key func(projectID, roleKey string) K) map[K]time.Time {
	out := make(map[K]time.Time, len(base))
	for k, end := range base {
		out[k] = end
	}
	for i := 0; i < len(rules); i++ {
		changed := false
		for _, r := range rules {
			src, held := out[key(r.SourceProject, r.SourceRole)]
			if !held {
				continue
			}
			t := key(r.TargetProject, r.TargetRole)
			cur, has := out[t]
			next := src
			if has {
				next = Later(cur, src)
			}
			if !has || !next.Equal(cur) {
				out[t] = next
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	return out
}

// Later is the end of something held through either of two sources: the zero
// time (never ends) wins, otherwise the later date.
func Later(a, b time.Time) time.Time {
	if a.IsZero() || b.IsZero() {
		return time.Time{}
	}
	if b.After(a) {
		return b
	}
	return a
}

// Closure is Ends without the dates: base plus every role the rules reach.
// The same fixpoint, so a set answer and a dated answer cannot disagree.
func Closure[K comparable](base map[K]bool, rules []models.MappingRule, key func(projectID, roleKey string) K) map[K]bool {
	dated := make(map[K]time.Time, len(base))
	for k, held := range base {
		if held {
			dated[k] = time.Time{}
		}
	}
	out := make(map[K]bool, len(dated))
	for k := range Ends(dated, rules, key) {
		out[k] = true
	}
	return out
}

// Fired is the rules that apply to somebody with this closure: those whose
// source they hold. Each one's target is held because of it, and its source is
// the reason. A rule whose target is also held another way still fires; the
// person view lists both reasons.
func Fired[K comparable](closure map[K]bool, rules []models.MappingRule, key func(projectID, roleKey string) K) []models.MappingRule {
	var out []models.MappingRule
	for _, r := range rules {
		if closure[key(r.SourceProject, r.SourceRole)] {
			out = append(out, r)
		}
	}
	return out
}
