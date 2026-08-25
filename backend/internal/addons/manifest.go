// Package addons is the backend half of the add-on contract: what an add-on is
// allowed to say about itself, and what the backend does with it.
//
// An add-on is a target adapter — the analogue of internal/zitadel, a driver
// behind Syndra's policy engine — not an autonomous controller. It holds a
// target credential and talks to a third-party API, which makes it the least
// trusted component in the system, and this package is written from that
// premise: a manifest declares capability, never authorization.
package addons

// ContractVersion is the backend-to-add-on contract this backend speaks.
//
// Add-ons are separately deployed containers, so one will eventually ship ahead
// of or behind the backend. Without a version that shows up as a field silently
// missing at the moment it matters; with one it is a refusal at registration,
// naming the mismatch. One integer now is cheaper than a compatibility matrix
// later, which is why this is a plain int and not a semver range.
const ContractVersion = 1

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
type Manifest struct {
	ContractVersion   int                `json:"contract_version"`
	Product           string             `json:"product"`
	ProductVersion    string             `json:"product_version"`
	EntitlementSchema []EntitlementField `json:"entitlement_schema"`
	Operations        []Operation        `json:"operations"`
	// Connection is how a MEMBER reaches this target, when the add-on's
	// deployment has said. Absent otherwise, and the member's page then omits
	// the instructions rather than inventing a host — a path that does not work
	// teaches somebody to distrust the whole page, and the next thing they
	// distrust is the part that was right.
	Connection *Connection `json:"connection,omitempty"`
}

// Connection is the member-facing address of a target, and nothing else. Never
// the add-on's own base URL: that is an internal endpoint the backend calls,
// and it is not the name a member types into a file manager.
type Connection struct {
	Protocol string `json:"protocol"`
	Host     string `json:"host"`
}

// EntitlementField names one field of the target's desired state. Syndra fills
// these; it never learns what any particular value means to the target.
type EntitlementField struct {
	Name string `json:"name"`
	// Type is the shape Syndra must supply: "string", "string[]", "bool", "int".
	Type string `json:"type"`
	// Lifecycle marks a field the resolver computes from whether the subject
	// holds any mapped role for this target, overridable only by an allowance.
	// Structural validation rejects a role-to-target mapping naming one: a
	// mapping binding a role to a disabled account would contradict the derived
	// lifecycle state on every resolution, and the two rules would fight.
	Lifecycle bool `json:"lifecycle"`
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
