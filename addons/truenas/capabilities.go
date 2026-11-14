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
	ContractVersion int    `json:"contract_version"`
	Product         string `json:"product"`
	ProductVersion  string `json:"product_version"`
	// Kind is what kind of system this add-on fronts (registry/kinds.json).
	Kind string `json:"kind"`
	// Features are the capabilities beyond the core this add-on serves
	// (registry/features.json): the backend calls a feature's route, or sends a
	// feature's request field, only to an add-on that declares it.
	Features          []string           `json:"features"`
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
	Limits     Limits      `json:"limits"`
	// Metrics and StatsZone are feature stats (stats.go): the descriptors,
	// and the zone whose calendar days the buckets are.
	Metrics   []Metric `json:"metrics,omitempty"`
	StatsZone string   `json:"stats_zone,omitempty"`
}

// Limits are what this add-on refuses beyond: a plan above SubjectsPerPlan is
// PLAN_TOO_LARGE, and the backend chunks below it (design §11).
type Limits struct {
	SubjectsPerPlan int `json:"subjects_per_plan"`
	// FreshReadCooldown is how soon a fresh read of one thing may read the NAS
	// again (feature freshness, fresh.go), so the UI can say when.
	FreshReadCooldown string `json:"fresh_read_cooldown,omitempty"`
	// StatsBuffer is how long a day stays in the statistics buffer.
	StatsBuffer string `json:"stats_buffer,omitempty"`
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

// EntitlementField names one field of desired state Syndra may fill, and what
// it IS: its role (registry/roles.json, design §3), from which the backend
// derives behaviour. Syndra never learns what a value means here; it fills the
// field by its role and this add-on translates.
type EntitlementField struct {
	Name string `json:"name"`
	// Role: account (the on/off, exactly one), channel (a way of using the
	// account, off whenever it is), membership (a set of values), flag
	// (["true"] or []), limit (a ceiling in a unit).
	Role string `json:"role"`
	// Label is the field's name for a person ("Home folder"). The mapping
	// screen shows it; nothing in the frontend names a field itself.
	Label string `json:"label,omitempty"`
	// Unit, Combine and Unlimited say what a limit is: its unit, how several
	// combine (this add-on combines, once, and reports the effective limit),
	// and the value that means no limit at all, which dominates.
	Unit      string `json:"unit,omitempty"`
	Combine   string `json:"combine,omitempty"`
	Unlimited string `json:"unlimited,omitempty"`
	// PerPerson means an operator may set this one field for one person, over
	// what their roles give (an allowance with direction "set").
	PerPerson bool `json:"per_person,omitempty"`
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
	// FieldHome is a role giving its holders a home folder ("true"). Each
	// person's is <the system's home location>/<account name>, settings.go.
	FieldHome = "home"
	// FieldHomeQuota is the most a person may store in the home folder's
	// dataset, in bytes. Several roles: the largest wins.
	FieldHomeQuota = "home_quota"
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
		{Name: FieldGroup, Role: "membership", Label: "Group"},
		{Name: FieldHome, Role: "flag", Label: "Gives a home folder"},
		// TrueNAS's "no quota" is 0, which is why "0" is the unlimited value.
		{Name: FieldHomeQuota, Role: "limit", Label: "Home folder limit", Unit: "bytes", Combine: "max", Unlimited: "0", PerPerson: true},
		{Name: FieldEnabled, Role: "account"},
		{Name: FieldSMBEnabled, Role: "channel", Label: "SMB"},
	}
}

// kindStorage is what this add-on fronts, and features are what it serves
// beyond the core (design §2, §4).
const kindStorage = "storage"

// Every read operation answers one `result` in the shape the registry names
// for it (core in contract 2, no feature).
//
// freshness: reads answer from a copy unless asked for a fresh one, within a
// cooldown, one read of the NAS for concurrent asks (fresh.go). Declared last
// and only with the backend that sends `fresh`: a backend that never does
// would read every /subjects as a copy, and its drift sweep would skip every
// pass. The two roll back together.
var features = []string{"operations", "settings", "storage.reach", "storage.homes", "freshness", "calls", "stats"}

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
			// A member asking about their own account: can I use this yet, and
			// how much room is left. No parameters at all, so there is no shape
			// in which one member asks about another. The subject is the call's
			// own: the backend sets it to the member on their page, or to the
			// person an operator is reading on that person's page (read-only).
			ID: "storage.status", Scope: "member", Confirm: false,
		},
		{
			// Stop managing an account without touching it. The safe half of a
			// purge, and the other resolution for a binding that points at the
			// wrong person or at an account that is gone.
			ID: "account.release", Scope: "admin", Confirm: true,
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
		if probe != nil { // nil: the declaration alone (publicParams)
			ops[i].Available, ops[i].UnavailableReason = probe.availability(ops[i].ID)
		}
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

// productName is what people call the product: the manifest's and health's
// `product`, which Syndra shows as the system's name until an operator names
// the system. A display name, never a key (contract 2 schema).
const productName = "TrueNAS"

// manifest composes the whole answer.
func manifest(product, productVersion string, probe capabilityProbe, connection *Connection, freshCooldown, statsZone string) Manifest {
	return Manifest{
		ContractVersion:   ContractVersion,
		Product:           product,
		ProductVersion:    productVersion,
		Kind:              kindStorage,
		Features:          features,
		EntitlementSchema: entitlementSchema(),
		Operations:        operationSet(probe),
		Connection:        connection,
		Limits:            Limits{SubjectsPerPlan: perRequestSubjectCap, FreshReadCooldown: freshCooldown, StatsBuffer: statsBuffer},
		Metrics:           statsMetrics,
		StatsZone:         statsZone,
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
	// Labels names a value the way a person says it, where that differs from
	// the value itself: a home folder's path reads as `main › home_dir`.
	Labels map[string]string `json:"labels,omitempty"`
	// BuiltinCount and BuiltinExamples say what was left out, for a NAS whose
	// groups are all built in: "It lists 34 groups, all of them built in, like
	// builtin_users and builtin_administrators." The examples are the first
	// two the read returned, in its order.
	BuiltinCount    int      `json:"builtin_count,omitempty"`
	BuiltinExamples []string `json:"builtin_examples,omitempty"`
}

// handleValues answers what a field's values may be.
func (s *server) handleValues(w http.ResponseWriter, r *http.Request, _ []byte) {
	field := r.PathValue("field")
	switch field {
	case FieldGroup:
		groups, err := s.readGroups()
		if err != nil {
			// Unreadable, which is not the same as empty. 503 rather than an
			// empty enumerable list: the backend fails open on a read it could
			// not make, and an empty list would make it fail closed on every
			// mapping while the target is down.
			writeError(w, statusFor(err), CodeTargetUnreadable, "The NAS's groups could not be read.", nil)
			return
		}
		out := ValuesResponse{Field: field, Values: []string{}, Enumerable: true}
		for _, g := range groups {
			// Left out of the list rather than marked in it. The backend's value
			// check reads this list, so a built-in group is unresolvable to a
			// mapping as well as absent from the picker.
			if !g.Builtin {
				out.Values = append(out.Values, g.Name)
				continue
			}
			out.BuiltinCount++
			if len(out.BuiltinExamples) < 2 {
				out.BuiltinExamples = append(out.BuiltinExamples, g.Name)
			}
		}
		sort.Strings(out.Values)
		writeJSON(w, http.StatusOK, out)
	case FieldHome:
		// A role gives a home folder or names nothing. One value, so the
		// backend's value check refuses anything else; where homes go is the
		// system's own setting (settings.go).
		writeJSON(w, http.StatusOK, ValuesResponse{Field: field, Values: []string{"true"}, Enumerable: true,
			Labels: map[string]string{"true": "Gives a home folder"}})
	case FieldHomeQuota:
		// A byte count, not a list: structure only, which the backend reads as
		// "any value of the declared type", never as "no value is valid".
		writeJSON(w, http.StatusOK, ValuesResponse{Field: field, Values: []string{}, Enumerable: false})
	case FieldEnabled, FieldSMBEnabled:
		// The account and its channel are resolver-computed and not
		// mapping-bindable, so there is no legitimate reason to ask. Answered rather than refused,
		// because the honest answer is a closed set of two.
		writeJSON(w, http.StatusOK, ValuesResponse{Field: field, Values: []string{"false", "true"}, Enumerable: true})
	default:
		writeError(w, http.StatusNotFound, CodeUnknownField, "This field is not in the entitlement schema.", nil)
	}
}
