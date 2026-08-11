package main

import (
	"net/http"
	"sort"
)

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
	// Connection is how a MEMBER reaches this target, for the instructions on
	// their own page.
	//
	// It comes from the add-on's own configuration because that is where the
	// truth about the deployment lives: moving the NAS must not mean editing a
	// component in the frontend. Absent when the deployment has not said, and
	// the member's page then omits the instructions rather than inventing a
	// host — a path that does not work teaches somebody to distrust the whole
	// page, and the next thing they distrust is the part that was right.
	Connection *Connection `json:"connection,omitempty"`
}

// Connection is the member-facing address of the target, and nothing else.
//
// Deliberately not the API URL: the middleware endpoint this add-on talks to is
// frequently not the name a member types into a file manager, and sending one
// where the other is meant produces instructions that fail for everybody.
type Connection struct {
	// Protocol is what a member connects with — "smb" here. Named rather than
	// assumed, so a second storage add-on speaking something else cannot be
	// rendered with these instructions by accident.
	Protocol string `json:"protocol"`
	Host     string `json:"host"`
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

// Field names the backend and this add-on must agree on. Written out rather
// than imported: the two are separately deployed binaries, and a shared
// constant would be a shared module — the version skew that module would hide
// is exactly what the contract version exists to surface.
const (
	FieldGroup      = "group"
	FieldEnabled    = "enabled"
	FieldSMBEnabled = "smb_enabled"
)

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
		{Name: FieldGroup, Type: "string[]"},
		{Name: FieldEnabled, Type: "bool", Lifecycle: true},
		{Name: FieldSMBEnabled, Type: "bool", Lifecycle: true},
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
			// Adoption: bind an account the target already holds to a subject.
			// Confirmed, because its mistake is the invisible one — the wrong
			// account hands a member somebody else's home directory, and the
			// next convergence makes that look intended.
			ID: "account.adopt", Scope: "admin", Confirm: true,
		},
		{
			ID: "account.purge", Scope: "admin", Confirm: true,
			// The delete-capable key the backend injects for this one call.
			// Declared secret so every redaction rule that covers a member's
			// password covers it too — it is a far more dangerous value.
			SecretParams: []string{"elevated_key"},
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
func manifest(product, productVersion string, probe capabilityProbe, connection *Connection) Manifest {
	return Manifest{
		ContractVersion:   ContractVersion,
		Product:           product,
		ProductVersion:    productVersion,
		EntitlementSchema: entitlementSchema(),
		Operations:        operationSet(probe),
		Connection:        connection,
	}
}

// ValuesResponse enumerates what a value on one entitlement field may be.
//
// It exists so the backend can check that `lab_makers` names something on this
// target before a mapping binding a role to it is written. Syndra cannot answer
// that — it does not know what the value means — and until this endpoint existed
// the check accepted every value, so a typo bound a role to a group that has
// never existed and the failure surfaced later as an apply nobody could explain.
//
// A read, not part of the manifest, and that placement matters: group membership
// is runtime state on the target, and a manifest is cached. Enumerating it in
// the manifest would mean a group created five minutes ago is refused until the
// cache turns over.
type ValuesResponse struct {
	Field string `json:"field"`
	// Values is what exists right now, sorted.
	Values []string `json:"values"`
	// Enumerable says this add-on can answer the question at all. A field whose
	// values are unbounded — a path, a quota — reports false with an empty list,
	// which the backend must read as "structure only", never as "no value is
	// valid".
	Enumerable bool `json:"enumerable"`
}

// handleValues answers what a field's values may be.
func (s *server) handleValues(w http.ResponseWriter, r *http.Request, _ []byte) {
	field := r.PathValue("field")
	switch field {
	case FieldGroup:
		_, byName, err := s.groupIndex()
		if err != nil {
			// Unreadable, which is not the same as empty. 503 rather than an
			// empty enumerable list: the backend fails open on a read it could
			// not make, and an empty list would make it fail closed on every
			// mapping while the target is down.
			writeJSON(w, statusFor(err), map[string]string{"error": "TARGET_UNREADABLE"})
			return
		}
		names := make([]string, 0, len(byName))
		for name := range byName {
			names = append(names, name)
		}
		sort.Strings(names)
		writeJSON(w, http.StatusOK, ValuesResponse{Field: field, Values: names, Enumerable: true})
	case FieldEnabled, FieldSMBEnabled:
		// Lifecycle fields are resolver-computed and not mapping-bindable, so
		// there is no legitimate reason to ask. Answered rather than refused,
		// because the honest answer is a closed set of two.
		writeJSON(w, http.StatusOK, ValuesResponse{Field: field, Values: []string{"false", "true"}, Enumerable: true})
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "UNKNOWN_FIELD", "detail": "this field is not in the entitlement schema",
		})
	}
}
