package models

// HeldRole is one (project, role) pair someone holds.
type HeldRole struct{ ProjectID, RoleKey string }

// NewHeldRole is RuleClosure's key function for HeldRole.
func NewHeldRole(projectID, roleKey string) HeldRole { return HeldRole{projectID, roleKey} }

// RuleClosure is the one place the mapping rules are followed: base plus every
// role a chain of rules derives from it (A→B→C), run to a fixpoint.
//
// It is the only implementation on purpose. The claims compiler, the cascades
// that decide what Zitadel is sent, the access views and the mapping cohort all
// call it, so the token, the preview and Zitadel cannot disagree about what a
// chain reaches. They did once: the webhook orchestrator walked one hop while
// the compiler walked the whole chain, and the token promised C while Zitadel
// was only ever given B.
//
// Generic over the key so each caller keeps its own key type and pays no copy;
// key builds that key from a (project, role) pair. At most len(rules) passes:
// each pass that changes anything adds at least one target, and there are only
// len(rules) targets to add, so a cycle (refused on insert, possible in a
// restored backup) still ends.
func RuleClosure[K comparable](base map[K]bool, rules []MappingRule, key func(projectID, roleKey string) K) map[K]bool {
	closure := make(map[K]bool, len(base))
	for k, held := range base {
		if held {
			closure[k] = true
		}
	}
	for i := 0; i < len(rules); i++ {
		changed := false
		for _, r := range rules {
			if !closure[key(r.SourceProject, r.SourceRole)] {
				continue
			}
			if t := key(r.TargetProject, r.TargetRole); !closure[t] {
				closure[t] = true
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	return closure
}
