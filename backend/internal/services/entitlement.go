package services

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"syndra/internal/db"
)

// Resolving a subject's entitlement set on a target (design §4, §6).
//
// Two layers and nothing else. The role layer is derived from
// `target_role_mappings` and from NOTHING ELSE — not from a convention, not
// from a name that happens to match, not from an add-on's opinion — so that
// "why does this person reach X" has exactly one answer per field. The
// allowance layer is an explicit per-user overlay on top, and in phase 1 it is
// subtractive only.
//
// Syndra decides who and what; the add-on decides how. Nothing in this file
// knows what `lab_makers` means.

// EntitlementSet is what the backend has decided a subject should hold on a
// target. It is the whole desired state, not a delta: `/apply` is
// level-triggered, so the set is the instruction.
type EntitlementSet struct {
	SubjectID string `json:"subject_id"`
	Target    string `json:"target"`
	// Fields maps an entitlement field to the values that survive resolution,
	// sorted, deduplicated, and never nil for a field that resolved to nothing
	// — an absent field and an empty one mean different things to a
	// level-triggered apply, and the add-on must be able to tell "no groups"
	// from "groups not managed here".
	Fields map[string][]string `json:"fields"`
	// Suppressed records what a denial took away, so a surface can render the
	// carve-out wherever the role appears rather than showing full access.
	Suppressed []SuppressedEntitlement `json:"suppressed,omitempty"`
	// Lifecycle is resolver-computed and never mapping-bindable (§4).
	Lifecycle LifecycleState `json:"lifecycle"`
}

// SuppressedEntitlement is one thing a subject would hold and does not, with
// the decision that took it away attached.
//
// A subject holding a role whose access they do not have is a trap unless it is
// visible, so the resolver returns the reason rather than merely the absence.
type SuppressedEntitlement struct {
	Field       string `json:"field"`
	Value       string `json:"value"`
	AllowanceID string `json:"allowance_id"`
	ActorID     string `json:"actor_id"`
	Reason      string `json:"reason"`
}

// LifecycleState is the account's derived existence and usability.
//
// Desired state, so it lives in the entitlement plane and converges through the
// same apply as any other field. An earlier draft had locking as a one-shot
// operation, which was an edge-triggered leak with a real consequence:
// deprovisioning left an account locked, and regaining a mapped role could not
// bring it back, because a create-if-absent `ensure` sees an existing account
// and does nothing. The account would stay dark while Syndra believed access
// was restored.
type LifecycleState struct {
	// Enabled is false exactly when the subject holds no mapped role for this
	// target, or an allowance denies it. The two are deliberately different
	// things — a derived lock clears itself when a role returns, and an
	// operator suspension survives re-resolution until it lapses or is lifted —
	// and Suppressed is what tells them apart.
	Enabled bool `json:"enabled"`
	// SMBEnabled follows Enabled unless something denies it separately.
	SMBEnabled bool `json:"smb_enabled"`
}

// Lifecycle field names. Reserved: structural mapping validation refuses them
// as mapping targets (task 7.4), because a mapping binding `role_key=X →
// enabled=false` would mean holding a role disables the account, colliding
// head-on with the derived lifecycle lock and fighting it on every resolution.
const (
	FieldEnabled    = "enabled"
	FieldSMBEnabled = "smb_enabled"
)

// IsLifecycleField reports whether a field is resolver-computed rather than
// mapping-bindable.
func IsLifecycleField(field string) bool {
	return field == FieldEnabled || field == FieldSMBEnabled
}

// ResolveEntitlements computes a subject's desired state on one target.
//
// The order is the model: derive from roles, then subtract denials, then
// compute lifecycle from what survived. Computing lifecycle first would make it
// a statement about the roles rather than about the access, and a subject whose
// every mapped entitlement is suspended would be reported as enabled with
// nothing to reach.
func ResolveEntitlements(ctx context.Context, subjectID, target string) (EntitlementSet, error) {
	set := EntitlementSet{SubjectID: subjectID, Target: target, Fields: map[string][]string{}}

	held, err := svcEffectiveRoleRefs(ctx, subjectID)
	if err != nil {
		return EntitlementSet{}, fmt.Errorf("resolve entitlements for %s: %w", subjectID, err)
	}
	mappings, err := dbMappingsForRoles(ctx, target, held)
	if err != nil {
		return EntitlementSet{}, fmt.Errorf("resolve entitlements for %s: %w", subjectID, err)
	}
	allowances, err := dbAllowancesInForce(ctx, subjectID, target)
	if err != nil {
		return EntitlementSet{}, fmt.Errorf("resolve entitlements for %s: %w", subjectID, err)
	}

	// Deny beats allow, and it beats derivation. A denial that lost to the role
	// layer would be a suspension an operator recorded and the system ignored.
	denied := map[string]db.Allowance{}
	for _, a := range allowances {
		if a.Direction != db.AllowanceDeny {
			// The additive arm is refused at the write, so reaching here means
			// a row predates that refusal or was written around it. Ignored
			// rather than applied: resolving an arm that was never built would
			// confer access from a code path nobody reviewed.
			continue
		}
		denied[a.Field+"\x00"+a.Value] = a
	}

	// A mapped role reaches the target; the values it confers are what the
	// mappings say and nothing else.
	mappedAny := false
	values := map[string]map[string]struct{}{}
	for _, m := range mappings {
		if IsLifecycleField(m.Field) {
			// Defence in depth. Mapping validation refuses these at the write,
			// and honouring one here would let a row that got in another way
			// fight the derived lifecycle on every resolution.
			continue
		}
		mappedAny = true
		if a, isDenied := denied[m.Field+"\x00"+m.Value]; isDenied {
			set.Suppressed = append(set.Suppressed, SuppressedEntitlement{
				Field: m.Field, Value: m.Value,
				AllowanceID: a.ID, ActorID: a.ActorID, Reason: a.Reason,
			})
			continue
		}
		if values[m.Field] == nil {
			values[m.Field] = map[string]struct{}{}
		}
		values[m.Field][m.Value] = struct{}{}
	}

	// Every mapped field appears, even when everything in it was suppressed. An
	// absent field and an empty one mean different things to a level-triggered
	// apply: one says "do not manage this", the other says "make it empty", and
	// a fully suspended subject means the second.
	for _, m := range mappings {
		if IsLifecycleField(m.Field) {
			continue
		}
		if _, seen := set.Fields[m.Field]; seen {
			continue
		}
		set.Fields[m.Field] = sortedKeys(values[m.Field])
	}

	set.Lifecycle = resolveLifecycle(mappedAny, denied, &set)
	sort.Slice(set.Suppressed, func(i, j int) bool {
		if set.Suppressed[i].Field != set.Suppressed[j].Field {
			return set.Suppressed[i].Field < set.Suppressed[j].Field
		}
		return set.Suppressed[i].Value < set.Suppressed[j].Value
	})
	return set, nil
}

// resolveLifecycle derives existence and usability from whether the subject
// reaches this target at all, then lets an allowance override it.
//
// This is what disambiguates two locks a target has no field to tell apart. A
// lifecycle lock is derived — the subject holds no mapped role — and clears
// itself when they do. An operator suspension is an allowance on `enabled`,
// carries a bound like every subtractive allowance, and survives re-resolution
// until it lapses or is lifted. So a deliberate suspension cannot be undone by
// a role grant, and a lifecycle lock cannot outlive the condition that caused
// it.
func resolveLifecycle(mappedAny bool, denied map[string]db.Allowance, set *EntitlementSet) LifecycleState {
	state := LifecycleState{Enabled: mappedAny, SMBEnabled: mappedAny}

	// A lifecycle denial names the field; its value is the state being refused,
	// and "true" is the only one that means anything — denying `enabled=false`
	// would be a double negative nobody should have to read.
	for _, field := range []string{FieldEnabled, FieldSMBEnabled} {
		a, isDenied := denied[field+"\x00"+"true"]
		if !isDenied {
			continue
		}
		switch field {
		case FieldEnabled:
			// Disabling the account takes SMB with it: an account that cannot
			// be used cannot be used over SMB either, and reporting otherwise
			// would leave the two fields describing a state the target cannot
			// hold.
			state.Enabled, state.SMBEnabled = false, false
		case FieldSMBEnabled:
			state.SMBEnabled = false
		}
		set.Suppressed = append(set.Suppressed, SuppressedEntitlement{
			Field: field, Value: "true",
			AllowanceID: a.ID, ActorID: a.ActorID, Reason: a.Reason,
		})
	}
	return state
}

func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ValidateAllowanceTerm refuses an allowance that would suppress nothing.
//
// The failure this exists for is silent and total: `resolveLifecycle` honours a
// lifecycle denial only when the value is exactly "true", and nothing else
// reads an allowance's field at all. So `enabled=false` — the way most people
// would write "disable this account" — is recorded, rendered in the lineage
// band as in force with an actor and a reason attached, reviewed on its review
// date, and suppresses precisely nothing. A misspelled field is the same
// failure with a different spelling.
//
// An allowance the resolver cannot act on is worse than a rejected one, because
// the operator has evidence they suspended somebody.
func ValidateAllowanceTerm(declared []string, field, value string) error {
	field, value = strings.TrimSpace(field), strings.TrimSpace(value)
	if field == "" || value == "" {
		return fmt.Errorf("%w: an allowance needs a field and a value", db.ErrAllowanceInvalid)
	}
	declares := false
	for _, d := range declared {
		if d == field {
			declares = true
			break
		}
	}
	if !declares {
		// The declared set is named, not the rejected field: an operator whose
		// field is not in the schema needs to know what is.
		return fmt.Errorf("%w: this target has no %s to deny — it declares %s",
			db.ErrAllowanceInvalid, field, strings.Join(declared, ", "))
	}
	if IsLifecycleField(field) && value != "true" {
		// The rejected value IS echoed here, unlike the mapping refusals a few
		// lines down and unlike anything on the operation path. The never-echo
		// rule is scoped to values that can be SECRETS — an operation's declared
		// secret parameters, where an error string is logged, returned and
		// captured in traces. An allowance value is a lifecycle state or a group
		// name, chosen by an operator from a schema, and showing them what they
		// typed is most of what makes this refusal actionable. Stated so the next
		// reader does not "correct" it in either direction.
		return fmt.Errorf(
			"%w: a %s denial is written %s=true, because the value names the state being refused; %q denies nothing",
			db.ErrAllowanceInvalid, field, field, value)
	}
	return nil
}

// ValidateMappingField refuses a mapping the backend can judge structurally,
// before the add-on is asked anything.
//
// Two of the three checks the design splits out live here: the field must be
// one the add-on's manifest declares, and it must not be a lifecycle field. The
// third — that the value resolves on the target — is the add-on's, because
// Syndra does not know what `lab_makers` means.
func ValidateMappingField(declared []string, field string) error {
	field = strings.TrimSpace(field)
	if field == "" {
		return fmt.Errorf("%w: a mapping needs a field", db.ErrMappingInvalid)
	}
	if IsLifecycleField(field) {
		return fmt.Errorf("%w: %s is computed from whether the subject holds any mapped role, so binding a role to it would make holding that role disable the account", db.ErrMappingInvalid, field)
	}
	for _, d := range declared {
		if d == field {
			return nil
		}
	}
	// The declared set is named, not the rejected field: an operator whose
	// field is not in the schema needs to know what is.
	return fmt.Errorf("%w: the add-on declares %s", db.ErrMappingInvalid, strings.Join(declared, ", "))
}

// effectiveRoleRefs lists every role a subject holds, from every source.
//
// Direct grants alone would be wrong in the direction that matters: a role held
// through a bundle or derived by a rule is held just as much, and a resolver
// that missed those would deprovision somebody the moment their access stopped
// being hand-granted. It reuses the same collector the access views are built
// from, so "what does this person hold" has one answer in the product.
func effectiveRoleRefs(ctx context.Context, subjectID string) ([]db.RoleRef, error) {
	roleMap, _, err := collectUserRolesHook(ctx, subjectID)
	if err != nil {
		return nil, err
	}
	refs := make([]db.RoleRef, 0, len(roleMap))
	for k, r := range roleMap {
		if r == nil {
			continue
		}
		refs = append(refs, db.RoleRef{ProjectID: k.projectID, RoleKey: k.roleKey})
	}
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].ProjectID != refs[j].ProjectID {
			return refs[i].ProjectID < refs[j].ProjectID
		}
		return refs[i].RoleKey < refs[j].RoleKey
	})
	return refs, nil
}
