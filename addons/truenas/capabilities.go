package main

// The manifest: what this add-on can do, and against this target (design §5).
//
// It is a CEILING that the backend intersects with its own policy, never a
// grant. Declaring `scope: member` on a destructive operation buys nothing —
// the backend's policy wins every disagreement, and an operation absent from
// that policy is unavailable whatever this file says. That is deliberate and it
// is why the add-on may be the least trusted component without being dangerous.

// Manifest is the shape `GET /capabilities` returns. Field names match the
// backend's decoder exactly; the two are separately deployed, which is what
// ContractVersion exists to make survivable.
type Manifest struct {
	ContractVersion   int                `json:"contract_version"`
	Product           string             `json:"product"`
	ProductVersion    string             `json:"product_version"`
	EntitlementSchema []EntitlementField `json:"entitlement_schema"`
	Operations        []Operation        `json:"operations"`
}

// EntitlementField names one field of desired state Syndra may fill. Syndra
// never learns what a value means here; it fills the field and this add-on
// translates.
type EntitlementField struct {
	Name string `json:"name"`
	Type string `json:"type"`
	// Lifecycle marks a field the backend's resolver computes from whether the
	// subject holds any mapped role, overridable only by an allowance. Declared
	// so the backend refuses a role-to-target mapping naming one: a mapping
	// binding a role to a disabled account would fight the derived lifecycle
	// state on every resolution.
	Lifecycle bool `json:"lifecycle"`
}

type Operation struct {
	ID                string   `json:"id"`
	Scope             string   `json:"scope"`
	Confirm           bool     `json:"confirm"`
	SecretParams      []string `json:"secret_params,omitempty"`
	Available         bool     `json:"available"`
	UnavailableReason string   `json:"unavailable_reason,omitempty"`
}

// entitlementSchema is the desired state this add-on converges.
//
// `enabled` and `smb_enabled` are here rather than in the operation set, and
// that placement is a decision with a scar behind it. An earlier draft had
// `account.lock` as a one-shot operation, which was an edge-triggered leak:
// deprovisioning left an account locked with SMB cleared, and regaining a role
// could not bring it back, because a create-if-absent path sees an existing
// account and does nothing. The account stayed dark while Syndra believed
// access was restored. As entitlement fields they converge like any other, and
// nothing special-cases restoration because nothing special-cased suspension.
func entitlementSchema() []EntitlementField {
	return []EntitlementField{
		{Name: "group", Type: "string[]"},
		{Name: "enabled", Type: "bool", Lifecycle: true},
		{Name: "smb_enabled", Type: "bool", Lifecycle: true},
	}
}

// operationSet is the one-shot, event-shaped half. Anything with a desired
// state belongs in the schema above instead.
//
// Availability is per operation rather than per target version, because a
// supported release may still lack a specific method — the research behind this
// design found methods moving across TrueNAS releases. An operation the target
// cannot perform is declared unavailable with a reason and shown disabled,
// rather than omitted (leaving an operator wondering whether it exists) or left
// to fail on use.
func operationSet(probe capabilityProbe) []Operation {
	ops := []Operation{
		{
			ID: "password.set", Scope: "member",
			// The one operation a member drives, and the reason nothing here is
			// ever queued: a durable intent row would write the member's
			// password into Postgres and retain it.
			SecretParams: []string{"password"},
		},
		{ID: "password.rotate", Scope: "admin"},
		{
			ID: "account.purge", Scope: "admin", Confirm: true,
		},
		{ID: "activity.get", Scope: "admin"},
		{ID: "health.get", Scope: "admin"},
	}
	for i := range ops {
		ops[i].Available, ops[i].UnavailableReason = probe.availability(ops[i].ID)
	}
	return ops
}

// capabilityProbe answers whether the target this add-on is attached to can
// actually perform an operation.
//
// An interface because the answer is a live fact about a particular NAS at a
// particular version, and because the manifest must still be servable when the
// NAS is unreachable — a capability set that vanishes during an outage would
// make the backend withdraw operations that are merely unobservable.
type capabilityProbe interface {
	availability(operationID string) (available bool, reason string)
}

// manifest composes the whole answer.
func manifest(product, productVersion string, probe capabilityProbe) Manifest {
	return Manifest{
		ContractVersion:   ContractVersion,
		Product:           product,
		ProductVersion:    productVersion,
		EntitlementSchema: entitlementSchema(),
		Operations:        operationSet(probe),
	}
}
