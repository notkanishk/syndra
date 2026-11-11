package addons

import (
	"fmt"
	"sort"
)

// Backend operation policy: what the backend permits, independent of what any
// manifest declares.
//
// An add-on declares what it *can* do. It does not decide what it is *allowed*
// to do. Without this table the manifest is an authorization source, and a
// compromised or misconfigured add-on can declare `scope: "member"` on
// account.purge and have Syndra render it to members. The add-on holds the
// target credential and talks to a third-party API, so it must not be able to
// widen its own authority.
//
// This is code, not configuration, and that is the point: a policy an operator
// could edit at runtime is a second manifest with a friendlier name. The cost is
// intended — adding an operation requires an entry here, so a new add-on
// version cannot quietly grow its surface.

// ParamSpec is one parameter the backend will accept for an operation. The
// backend owns this schema rather than trusting the manifest's, so an add-on
// cannot introduce a parameter the backend has never heard of.
type ParamSpec struct {
	Name     string
	Type     string
	Required bool
	// Secret marks a value that is forwarded and never persisted — absent from
	// every table, audit row, outbox payload, plan row, log line, error body,
	// and panic capture. Policy marking a parameter secret is authoritative:
	// a manifest that forgets to declare it cannot make it loggable.
	Secret bool
}

// OperationPolicy is the ceiling for one operation id.
type OperationPolicy struct {
	// Scope is the BROADEST scope the operation may be offered at. A manifest
	// declaring a broader one is narrowed to this.
	Scope Scope
	// Confirm is whether the backend requires confirmation. A manifest saying
	// no cannot remove it; a manifest saying yes adds it.
	Confirm bool
	Params  []ParamSpec
	// Effect is what the operation changes (registry/operations.json): a read
	// is served through Read alone, never dispatched, logged or confirmed.
	Effect string
	// Result is the schema a feature-results add-on answers under `result`,
	// relative to v2/schema; "" for an id that declares its own.
	Result string
	// Pocket is the reply key a read was answered under before feature
	// results: the add-on's own typed report, which the core compiled. Read
	// only from an add-on without results; a read with none needs results.
	//
	// ponytail: deleted at major 3, with the pockets; then every read needs results.
	Pocket string
}

// The effect vocabulary (registry/operations.json). A new class is a core
// change: the retry rule and the read path key on it.
const (
	EffectRead       = "read"
	EffectAccount    = "account"
	EffectCredential = "credential"
	EffectPhysical   = "physical"
)

// operationPolicy is keyed by operation id, not by (target, id): an operation
// id is a contract-level name, and two targets offering `password.set` mean the
// same thing by it. A target-specific rule would be a target-specific
// authorization decision, which is exactly what §13 exists to prevent.
var operationPolicy = map[string]OperationPolicy{
	// The member's own storage credential. Member scope, and the subject
	// binding at the request boundary is what keeps that from meaning "any
	// member may reset anyone's password".
	"password.set": {
		Scope:   ScopeMember,
		Confirm: false,
		Params: []ParamSpec{
			{Name: "password", Type: "string", Required: true, Secret: true},
		},
		Effect: EffectCredential,
	},
	// The credential half of a revocation: mint and apply a new secret without
	// returning or retaining it. Operator-driven, so admin.
	"password.rotate": {
		Scope:   ScopeAdmin,
		Confirm: true,
		Effect:  EffectCredential,
	},
	// Adoption: attach an account the target already holds to a subject.
	//
	// Confirmed, and not because it is destructive — it writes one binding. It
	// is because it is the one operation whose mistake is invisible: adopting
	// the wrong account hands a member somebody else's home directory, their
	// shares and their group memberships, and the next convergence then makes
	// that look like the intended state. There is no undo that gives the data
	// back to whoever it belonged to.
	"account.adopt": {
		Scope:   ScopeAdmin,
		Confirm: true,
		Params: []ParamSpec{
			// The account being adopted, by the name the inventory showed. The
			// subject is not a parameter: it is the request's subject, checked
			// by the same binding rule every operation goes through.
			{Name: "username", Type: "string", Required: true},
		},
		Effect: EffectAccount,
	},
	// The one irreversible operation in the set. Confirmation is not a UI
	// nicety here — the backend refuses the call without it.
	"account.purge": {
		Scope:   ScopeAdmin,
		Confirm: true,
		Params: []ParamSpec{
			// A delete-capable credential the OPERATOR supplies at the moment
			// of the call. The add-on deliberately holds none: its long-lived
			// session can read and write accounts and cannot remove one, so a
			// compromise of the add-on cannot destroy anybody's files.
			//
			// Declared here because it was not, and the omission made this
			// operation uncallable end to end: the add-on refuses a purge with
			// no `elevated_key`, and this policy refused to send one as an
			// unknown parameter. The same class of disagreement as §17, on the
			// one operation nobody had exercised.
			{Name: "elevated_key", Type: "string", Required: true, Secret: true},
		},
		Effect: EffectAccount,
	},
	"activity.get": {
		Scope:   ScopeAdmin,
		Confirm: false,
		Params: []ParamSpec{
			{Name: "since", Type: "string", Required: false},
		},
		Effect: EffectRead,
		Result: "common/activity.json",
		Pocket: "activity",
	},
	// Declares its own result, shown raw until a kind module reads it.
	"health.get": {
		Scope:   ScopeAdmin,
		Confirm: false,
		Effect:  EffectRead,
		Pocket:  "health",
	},
	// Stop managing an account without touching it.
	//
	// The safe half of a purge, and the resolution the reconciliation surface
	// names when a binding points at an account that is gone. Confirmed, because
	// it changes who Syndra believes an account belongs to — but it is not
	// destructive, and it must stay available when the target is unreachable,
	// which is exactly when an operator is most likely to need it.
	"account.release": {
		Scope:   ScopeAdmin,
		Confirm: true,
		Effect:  EffectAccount,
	},
	// A member asking about their own storage account: whether it can be used
	// yet, and how much room is left.
	//
	// Member scope and NO parameters — deliberately none, rather than a subject
	// parameter the handler would have to check. A member-scoped operation acts
	// only on the authenticated actor, and an operation that takes no subject at
	// all cannot be pointed at somebody else by any caller, including a future
	// one that forgets to check.
	//
	// No confirmation: it writes nothing.
	//
	// The operator case (A-10): a person's page reads the same answer about
	// that person, read-only, so an operator sees what the member sees. It is
	// not a second operation and not a subject parameter. It is a read
	// (addons.Read, never dispatched), and the operator-only route
	// GET /users/{id}/targets names the subject and passes the operator as the
	// actor. Members still reach it only through /me/targets, for themselves.
	"storage.status": {
		Scope:   ScopeMember,
		Confirm: false,
		Effect:  EffectRead,
		Result:  "profiles/storage/status.json",
		Pocket:  "storage",
	},
	// What the member holds on the target (a borrowed item, a booked room),
	// read live and never stored: the same read path and the same two routes
	// as storage.status, so an admin looking at a person's page writes no
	// operations-log row. It has no legacy pocket, so only an add-on declaring
	// feature results can serve it.
	"holdings.list": {
		Scope:   ScopeMember,
		Confirm: false,
		Effect:  EffectRead,
		Result:  "common/records.json",
	},
}

// EffectiveOperation is an operation as the backend will actually offer it:
// the manifest's declaration bounded by policy, resolved dimension by
// dimension. Nothing downstream reads the raw manifest operation.
type EffectiveOperation struct {
	ID                string
	Scope             Scope
	Confirm           bool
	SecretParams      []string
	Params            []ParamSpec
	Available         bool
	UnavailableReason string
	// Effect, Result and Pocket are the policy's alone: a manifest cannot
	// make a write a read, nor name another reply shape.
	Effect string
	Result string
	Pocket string
}

// resolveOperations intersects a manifest's operation set with backend policy.
//
// The rule is one sentence and it holds on every dimension: **the effective
// operation is the more restrictive of the two.** Scope resolves to admin if
// either says admin, confirmation to required if either requires it, secret
// parameters to the union, and availability to unavailable if either withholds
// it. Stating it as one rule rather than four cases is what keeps a later
// dimension from being added with the comparison accidentally inverted.
//
// An operation id absent from policy is dropped entirely rather than returned
// unavailable-with-a-reason. Unavailability is the presentation for a target
// that lacks a method — something an operator can act on. An id the backend has
// no policy for is not an operator's problem to see; it is a deployment that
// shipped an add-on ahead of its backend, and it fails closed. Dropped ids are
// returned separately so registration can log them once.
func resolveOperations(m Manifest) (ops []EffectiveOperation, unknown []string) {
	// Folded by id, most-restrictively, because a manifest may declare one id
	// twice. Emitting a row per declaration and letting the first match win
	// would let the least trusted component in the system pick which of its own
	// declarations applies — declare `password.set` unavailable and then
	// available, and which one the backend honours comes down to the order two
	// equal keys happen to land in after a non-stable sort.
	//
	// The rule is the same one every dimension follows, applied once more: two
	// declarations of one id resolve to the more restrictive of the two.
	byID := map[string]EffectiveOperation{}
	for _, mo := range m.Operations {
		p, ok := operationPolicy[mo.ID]
		if !ok {
			unknown = append(unknown, mo.ID)
			continue
		}

		eff := EffectiveOperation{
			ID:                mo.ID,
			Scope:             mostRestrictiveScope(p.Scope, mo.Scope),
			Confirm:           p.Confirm || mo.Confirm,
			Params:            p.Params,
			SecretParams:      unionSecretParams(p, mo.SecretParams),
			Available:         mo.Available,
			UnavailableReason: mo.UnavailableReason,
			Effect:            p.Effect,
			Result:            p.Result,
			Pocket:            p.Pocket,
		}
		if !eff.Available && eff.UnavailableReason == "" {
			eff.UnavailableReason = "the add-on reported this operation unavailable without a reason"
		}
		if eff.Available && p.Effect == EffectRead && p.Pocket == "" && !m.Declares("results") {
			// A read with no pocket to answer under: without results the
			// add-on has no shape Syndra reads, so it is shown, not dropped.
			eff.Available = false
			eff.UnavailableReason = "the add-on does not declare feature results, which this read needs"
		}
		if previous, seen := byID[mo.ID]; seen {
			eff = mostRestrictiveOperation(previous, eff)
		}
		byID[mo.ID] = eff
	}
	for _, eff := range byID {
		ops = append(ops, eff)
	}
	// Stable, not merely sorted: `sort.Slice` is not stable, and a total order
	// on the key it sorts by is what makes that irrelevant. Duplicates are
	// folded above, so every remaining id is unique.
	sort.Slice(ops, func(i, j int) bool { return ops[i].ID < ops[j].ID })
	sort.Strings(unknown)
	return ops, dedupeStrings(unknown)
}

// mostRestrictiveOperation resolves two declarations of one id.
func mostRestrictiveOperation(a, b EffectiveOperation) EffectiveOperation {
	out := EffectiveOperation{
		ID:      a.ID,
		Scope:   mostRestrictiveScope(a.Scope, b.Scope),
		Confirm: a.Confirm || b.Confirm,
		Params:  a.Params,
		Effect:  a.Effect,
		Result:  a.Result,
		Pocket:  a.Pocket,
		// Union, for the reason the policy/manifest union exists: a second
		// declaration that omits a secret parameter must not make the value
		// loggable.
		SecretParams: unionStrings(a.SecretParams, b.SecretParams),
		Available:    a.Available && b.Available,
	}
	switch {
	case !a.Available && a.UnavailableReason != "":
		out.UnavailableReason = a.UnavailableReason
	case !b.Available:
		out.UnavailableReason = b.UnavailableReason
	}
	return out
}

func unionStrings(a, b []string) []string {
	return dedupeStrings(append(append([]string(nil), a...), b...))
}

func dedupeStrings(in []string) []string {
	if len(in) == 0 {
		return in
	}
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}

// mostRestrictiveScope returns whichever of the two admits fewer principals.
// `member` is the broader declaration — it is the one a compromised add-on
// would claim — so admin wins any disagreement, and an unrecognised scope from
// a manifest is treated as admin rather than defaulted to the permissive value
// or rejected outright.
func mostRestrictiveScope(policy, manifest Scope) Scope {
	if policy == ScopeAdmin || manifest != ScopeMember {
		return ScopeAdmin
	}
	return ScopeMember
}

// unionSecretParams merges the policy's secret parameters with the manifest's.
// A union, not an intersection: policy prevailing means never less protective,
// and a manifest that omits a secret declaration must not thereby make the
// value loggable. The manifest may add — it knows its own target's parameters
// better than the backend does.
func unionSecretParams(p OperationPolicy, declared []string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(n string) {
		if n != "" && !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	for _, ps := range p.Params {
		if ps.Secret {
			add(ps.Name)
		}
	}
	for _, n := range declared {
		add(n)
	}
	sort.Strings(out)
	return out
}

// IsSecretParam reports whether the named parameter of this operation carries a
// secret. Redaction reads this rather than the manifest directly.
func (e EffectiveOperation) IsSecretParam(name string) bool {
	for _, n := range e.SecretParams {
		if n == name {
			return true
		}
	}
	return false
}

// ErrOperationUnavailable is returned for an operation the add-on declared but
// cannot currently perform. It carries the add-on's reason so a surface can
// explain the refusal rather than reporting a generic failure.
type ErrOperationUnavailable struct {
	Target, ID, Reason string
}

func (e *ErrOperationUnavailable) Error() string {
	return fmt.Sprintf("addon %s: operation %q is unavailable: %s", e.Target, e.ID, e.Reason)
}
