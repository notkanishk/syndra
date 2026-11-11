// Package addons is the backend half of the add-on contract: what an add-on is
// allowed to say about itself, and what the backend does with it.
//
// An add-on is a target adapter — the analogue of internal/zitadel, a driver
// behind Syndra's policy engine — not an autonomous controller. It holds a
// target credential and talks to a third-party API, which makes it the least
// trusted component in the system, and this package is written from that
// premise: a manifest declares capability, never authorization.
package addons

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// ContractVersion is the backend-to-add-on contract this backend speaks.
//
// Add-ons are separately deployed containers, so one will eventually ship ahead
// of or behind the backend. Without a version that shows up as a field silently
// missing at the moment it matters; with one it is a refusal at registration,
// naming the mismatch. One integer now is cheaper than a compatibility matrix
// later, which is why this is a plain int and not a semver range.
const ContractVersion = 2

// Scope is who may invoke an operation. It is deliberately a two-value type
// rather than a free string: the whole point of the policy ceiling below is
// that scope is compared, and comparing arbitrary strings has no meaningful
// "more restrictive than".
type Scope string

const (
	// ScopeMember is the broader declaration — the operation is offered to the
	// member acting on themselves. A member-scoped operation additionally binds
	// its subject to the authenticated actor, enforced at the request boundary.
	ScopeMember Scope = "member"
	// ScopeAdmin is the narrower one: operators only.
	ScopeAdmin Scope = "admin"
)

// Valid reports whether s is a scope the backend understands. An unrecognised
// scope from a manifest is not defaulted — see mostRestrictiveScope.
func (s Scope) Valid() bool { return s == ScopeMember || s == ScopeAdmin }

// Manifest is what an add-on returns from GET /capabilities.
//
// It carries no target name on purpose. The registration names the target,
// because that is a deployment fact the backend already holds; a manifest
// declaring its own would create a mismatch case with nothing to gain from
// resolving it.
//
// Decoded TOLERANTLY (contract 2, design §2): a manifest may always carry more
// than this backend knows, because it is the one message meant to describe
// change. What it carries beyond these fields is kept in Unknown, quoted and
// capped, for the target page and one log line — never used for a decision.
type Manifest struct {
	ContractVersion int    `json:"contract_version"`
	Product         string `json:"product"`
	ProductVersion  string `json:"product_version"`
	// Kind is what kind of system the add-on fronts (registry/kinds.json): a
	// closed list, refused at registration when unknown.
	Kind string `json:"kind"`
	// Features are what it serves beyond the core (registry/features.json).
	// The backend calls a feature's route or sends a feature's request field
	// only to an add-on declaring it; an unknown feature is skipped.
	Features          []string           `json:"features"`
	EntitlementSchema []EntitlementField `json:"entitlement_schema"`
	Operations        []Operation        `json:"operations"`
	// Connection is how a MEMBER reaches this target, when the add-on's
	// deployment has said. Absent otherwise, and the member's page then omits
	// the instructions rather than inventing a host — a path that does not work
	// teaches somebody to distrust the whole page, and the next thing they
	// distrust is the part that was right.
	Connection *Connection `json:"connection,omitempty"`
	Limits     Limits      `json:"limits"`

	// Unknown is every key of the manifest this backend does not know, top
	// level or nested, as `path: value` with the value quoted, escaped and
	// capped: what the target page shows under "What the add-on also says".
	Unknown []string `json:"-"`
}

// Limits are what the add-on refuses beyond. SubjectsPerPlan bounds one plan:
// the backend plans in chunks below it (design §11). Zero is undeclared.
type Limits struct {
	SubjectsPerPlan int `json:"subjects_per_plan,omitempty"`
}

// Connection is the member-facing address of a target, and nothing else. Never
// the add-on's own base URL: that is an internal endpoint the backend calls,
// and it is not the name a member types into a file manager.
type Connection struct {
	Protocol string `json:"protocol"`
	Host     string `json:"host"`
}

// The field roles (registry/roles.json, design §3): what an entitlement field
// IS, from which behaviour is derived.
const (
	// RoleAccount is THE on/off of the subject's account. Exactly one.
	RoleAccount = "account"
	// RoleChannel is a way of using the account, off whenever it is off.
	RoleChannel = "channel"
	// RoleMembership is a set of target-defined values a role confers.
	RoleMembership = "membership"
	// RoleFlag is one thing a role gives or does not: ["true"] or [].
	RoleFlag = "flag"
	// RoleLimit is a numeric ceiling in a unit; the add-on combines.
	RoleLimit = "limit"
)

// knownRoles and knownKinds are the closed vocabularies of
// registry/roles.json and registry/kinds.json, held equal to them by test.
var (
	knownRoles = []string{RoleAccount, RoleChannel, RoleMembership, RoleFlag, RoleLimit}
	knownKinds = []string{"storage", "access", "inventory", "machine"}
)

// EntitlementField names one field of the target's desired state, and what it
// is (its role). Syndra fills these by role; it never learns what any
// particular value means to the target.
type EntitlementField struct {
	Name string `json:"name"`
	Role string `json:"role"`
	// Label is the field's name for a person ("Home folder"); the mapping
	// screen shows it, and no frontend names a field itself.
	Label string `json:"label,omitempty"`
	// Unit, Combine and Unlimited describe a limit.
	Unit      string `json:"unit,omitempty"`
	Combine   string `json:"combine,omitempty"`
	Unlimited string `json:"unlimited,omitempty"`
	// PerPerson means an operator may set this field for one person, over
	// what their roles give (an allowance with direction "set").
	PerPerson bool `json:"per_person,omitempty"`
}

// IsLifecycle is contract 1's `lifecycle` flag, derived from the role: the
// account and its channels are computed by the resolver from whether the
// subject holds any mapped role, overridable only by an allowance, and a
// role-to-target mapping naming one is refused (it would fight the derived
// state on every resolution).
func (f EntitlementField) IsLifecycle() bool { return f.Role == RoleAccount || f.Role == RoleChannel }

// IsSingle is contract 1's `single`: a person has one value whatever their
// roles say (a flag is given or not; a limit is combined by the add-on).
func (f EntitlementField) IsSingle() bool { return f.Role == RoleFlag || f.Role == RoleLimit }

// IsLimit is contract 1's `type: "int"`: a value that must parse as a
// non-negative whole number.
func (f EntitlementField) IsLimit() bool { return f.Role == RoleLimit }

// FieldsByRole is the role map, role → field names in declaration order.
func (m Manifest) FieldsByRole() map[string][]string {
	out := map[string][]string{}
	for _, f := range m.EntitlementSchema {
		out[f.Role] = append(out[f.Role], f.Name)
	}
	return out
}

// RoleMap is field name → role, the shape persisted in targets.field_roles.
func (m Manifest) RoleMap() map[string]string {
	out := make(map[string]string, len(m.EntitlementSchema))
	for _, f := range m.EntitlementSchema {
		out[f.Name] = f.Role
	}
	return out
}

// Declares reports whether the manifest declares a feature.
func (m Manifest) Declares(feature string) bool { return slices.Contains(m.Features, feature) }

// validate refuses a manifest whose meaning this backend cannot know, with the
// reason an operator reads on the target page: an unknown kind (it keys the
// credential-scope denylist, so it is never absent), a role outside the closed
// vocabulary, a field declared twice, or anything but exactly one account
// field (design §3, §4).
func (m Manifest) validate() error {
	if !slices.Contains(knownKinds, m.Kind) {
		return fmt.Errorf("the manifest declares kind %q, which is not one of %s", m.Kind, strings.Join(knownKinds, ", "))
	}
	seen := map[string]bool{}
	for _, f := range m.EntitlementSchema {
		switch {
		case strings.TrimSpace(f.Name) == "":
			return errors.New("the manifest declares a field with no name")
		case seen[f.Name]:
			return fmt.Errorf("the manifest declares field %q twice", f.Name)
		case !slices.Contains(knownRoles, f.Role):
			return fmt.Errorf("field %q has role %q, which is not one of %s", f.Name, f.Role, strings.Join(knownRoles, ", "))
		}
		seen[f.Name] = true
	}
	if accounts := m.FieldsByRole()[RoleAccount]; len(accounts) != 1 {
		return fmt.Errorf("the manifest declares %d fields with role account; exactly one is required", len(accounts))
	}
	return nil
}

// Operation is one entry in the manifest's operation set — the one-shot,
// event-shaped half of the two planes. Anything with a desired state belongs in
// the entitlement schema above instead.
type Operation struct {
	ID           string   `json:"id"`
	Scope        Scope    `json:"scope"`
	Confirm      bool     `json:"confirm"`
	SecretParams []string `json:"secret_params,omitempty"`
	// Available is per operation, not per target version: a supported target
	// release may still lack a specific method. An unavailable operation is
	// shown disabled with its reason rather than omitted or left to fail on use.
	Available         bool   `json:"available"`
	UnavailableReason string `json:"unavailable_reason,omitempty"`
}

// EntitlementField returns the named field of the schema.
func (m Manifest) EntitlementField(name string) (EntitlementField, bool) {
	for _, f := range m.EntitlementSchema {
		if f.Name == name {
			return f, true
		}
	}
	return EntitlementField{}, false
}
