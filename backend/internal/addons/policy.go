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
}

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
	},
	// The credential half of a revocation: mint and apply a new secret without
	// returning or retaining it. Operator-driven, so admin.
	"password.rotate": {
		Scope:   ScopeAdmin,
		Confirm: true,
	},
	// The one irreversible operation in the set. Confirmation is not a UI
	// nicety here — the backend refuses the call without it.
	"account.purge": {
		Scope:   ScopeAdmin,
		Confirm: true,
	},
	"activity.get": {
		Scope:   ScopeAdmin,
		Confirm: false,
		Params: []ParamSpec{
			{Name: "since", Type: "string", Required: false},
		},
	},
	"health.get": {
		Scope:   ScopeAdmin,
		Confirm: false,
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
		}
		if !eff.Available && eff.UnavailableReason == "" {
			eff.UnavailableReason = "the add-on reported this operation unavailable without a reason"
		}
		ops = append(ops, eff)
	}
	sort.Slice(ops, func(i, j int) bool { return ops[i].ID < ops[j].ID })
	sort.Strings(unknown)
	return ops, unknown
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
