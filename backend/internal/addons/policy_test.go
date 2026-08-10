package addons

import (
	"reflect"
	"testing"
)

// effective resolves one manifest operation and returns it, failing if policy
// dropped it. Most tests below are about a single dimension of the ceiling.
func effective(t *testing.T, op Operation) EffectiveOperation {
	t.Helper()
	ops, unknown := resolveOperations(Manifest{ContractVersion: ContractVersion, Operations: []Operation{op}})
	if len(unknown) > 0 {
		t.Fatalf("operation %q was dropped as unknown", op.ID)
	}
	if len(ops) != 1 {
		t.Fatalf("expected one effective operation, got %d", len(ops))
	}
	return ops[0]
}

// 2.34 — a manifest cannot widen an operation's scope. This is the whole reason
// the policy table exists: a compromised add-on declaring `scope: "member"` on
// account.purge must not have Syndra render deletion to members.
func TestManifestCannotWidenScope(t *testing.T) {
	got := effective(t, Operation{ID: "account.purge", Scope: ScopeMember, Confirm: true, Available: true})
	if got.Scope != ScopeAdmin {
		t.Errorf("policy pins account.purge to admin; manifest claimed member and got %q", got.Scope)
	}
}

// The mirror: a manifest MAY narrow. Backend policy is a ceiling, not a floor,
// so an add-on choosing to offer less than it is permitted is honoured.
func TestManifestMayNarrowScope(t *testing.T) {
	got := effective(t, Operation{ID: "password.set", Scope: ScopeAdmin, Available: true})
	if got.Scope != ScopeAdmin {
		t.Errorf("policy permits password.set at member, but a manifest narrowing to admin must be honoured; got %q", got.Scope)
	}
}

// An unrecognised scope is not defaulted to the permissive value. A manifest
// that says something the backend does not understand has said nothing that can
// widen anything.
func TestUnrecognisedScopeResolvesToAdmin(t *testing.T) {
	got := effective(t, Operation{ID: "password.set", Scope: Scope("everyone"), Available: true})
	if got.Scope != ScopeAdmin {
		t.Errorf("an unrecognised scope must resolve to the narrower value, got %q", got.Scope)
	}
}

// 2.34 — a manifest cannot drop a confirmation requirement.
func TestManifestCannotDropConfirmation(t *testing.T) {
	got := effective(t, Operation{ID: "account.purge", Scope: ScopeAdmin, Confirm: false, Available: true})
	if !got.Confirm {
		t.Error("policy requires confirmation on account.purge; a manifest saying otherwise must not remove it")
	}
}

// And it may add one, for the same reason it may narrow scope: more restrictive
// wins on every dimension, whichever side asked for it.
func TestManifestMayAddConfirmation(t *testing.T) {
	got := effective(t, Operation{ID: "health.get", Scope: ScopeAdmin, Confirm: true, Available: true})
	if !got.Confirm {
		t.Error("a manifest requiring confirmation where policy does not must be honoured")
	}
}

// 2.34 — an operation id with no backend policy is unavailable, and stays that
// way no matter which add-on version declares it. Adding an operation is a
// backend change on purpose: a new add-on version cannot quietly grow its
// surface.
func TestUnknownOperationFailsClosed(t *testing.T) {
	ops, unknown := resolveOperations(Manifest{
		ContractVersion: ContractVersion,
		Operations: []Operation{
			{ID: "account.nuke", Scope: ScopeAdmin, Available: true},
			{ID: "health.get", Scope: ScopeAdmin, Available: true},
		},
	})
	if len(ops) != 1 || ops[0].ID != "health.get" {
		t.Errorf("only the policied operation may survive, got %+v", ops)
	}
	if !reflect.DeepEqual(unknown, []string{"account.nuke"}) {
		t.Errorf("the dropped id must be reported so registration can log it once, got %v", unknown)
	}
}

// 2.7 groundwork — secret parameters are the union of policy and manifest, not
// the manifest's word for it. A manifest that forgets to declare `password`
// must not thereby make the value loggable, and an add-on that knows one of its
// own parameters carries a secret may add it.
func TestSecretParamsAreTheUnionAndPolicyCannotBeDropped(t *testing.T) {
	got := effective(t, Operation{ID: "password.set", Scope: ScopeMember, Available: true})
	if !got.IsSecretParam("password") {
		t.Error("policy marks password secret; a manifest omitting it cannot unmark it")
	}

	got = effective(t, Operation{
		ID: "password.set", Scope: ScopeMember, Available: true,
		SecretParams: []string{"recovery_token"},
	})
	if !got.IsSecretParam("password") || !got.IsSecretParam("recovery_token") {
		t.Errorf("secret params must be the union of policy and manifest, got %v", got.SecretParams)
	}
}

// The parameter schema itself comes from policy, never the manifest, so an
// add-on cannot introduce a parameter the backend has never heard of.
func TestParameterSchemaComesFromPolicy(t *testing.T) {
	got := effective(t, Operation{ID: "password.set", Scope: ScopeMember, Available: true})
	if len(got.Params) != 1 || got.Params[0].Name != "password" || !got.Params[0].Required {
		t.Errorf("password.set must carry policy's parameter schema, got %+v", got.Params)
	}
}

// Every policied operation must name a scope the backend understands. A blank
// or bogus scope in the table would silently resolve everything to admin and
// hide the mistake behind a safe-looking default.
func TestEveryPolicyEntryDeclaresAValidScope(t *testing.T) {
	for id, p := range operationPolicy {
		if !p.Scope.Valid() {
			t.Errorf("policy entry %q declares scope %q, which is neither member nor admin", id, p.Scope)
		}
	}
}

// The one irreversible operation must require confirmation at the policy layer,
// where no manifest can reach it.
func TestPurgeRequiresConfirmationInPolicy(t *testing.T) {
	p, ok := operationPolicy["account.purge"]
	if !ok {
		t.Fatal("account.purge must have a backend policy entry")
	}
	if !p.Confirm || p.Scope != ScopeAdmin {
		t.Errorf("account.purge must be admin-scoped and confirmed in policy, got %+v", p)
	}
}

// §13's whole argument is that the least trusted component must not widen its
// own authority. A manifest declaring one id twice — once withheld, once
// offered — resolved to whichever row a non-stable sort happened to put first,
// which is the add-on choosing which of its own declarations the backend
// honours.
func TestADuplicateOperationIdResolvesToTheMoreRestrictiveDeclaration(t *testing.T) {
	for _, order := range [][]Operation{
		{
			{ID: "password.set", Scope: ScopeMember, Available: false, UnavailableReason: "this target does not expose user.update"},
			{ID: "password.set", Scope: ScopeMember, Available: true},
		},
		// The other order must give the same answer, which is the property a
		// non-stable sort took away.
		{
			{ID: "password.set", Scope: ScopeMember, Available: true},
			{ID: "password.set", Scope: ScopeMember, Available: false, UnavailableReason: "this target does not expose user.update"},
		},
	} {
		ops, unknown := resolveOperations(Manifest{ContractVersion: ContractVersion, Operations: order})
		if len(unknown) != 0 {
			t.Fatalf("nothing should be unknown: %v", unknown)
		}
		if len(ops) != 1 {
			t.Fatalf("one id must resolve to one operation, got %d", len(ops))
		}
		if ops[0].Available {
			t.Errorf("a withheld declaration must win: %+v", ops[0])
		}
		if ops[0].UnavailableReason == "" {
			t.Errorf("and it must carry its reason: %+v", ops[0])
		}
	}
}

// The same rule on every other dimension, so a duplicate cannot be used to
// widen scope, drop a confirmation, or unmark a secret either.
func TestADuplicateOperationCannotWidenAnyDimension(t *testing.T) {
	ops, _ := resolveOperations(Manifest{ContractVersion: ContractVersion, Operations: []Operation{
		{ID: "password.set", Scope: ScopeAdmin, Confirm: true, Available: true, SecretParams: []string{"password"}},
		{ID: "password.set", Scope: ScopeMember, Confirm: false, Available: true},
	}})
	if len(ops) != 1 {
		t.Fatalf("one id must resolve to one operation, got %d", len(ops))
	}
	got := ops[0]
	if got.Scope != ScopeAdmin {
		t.Errorf("scope must stay admin: %+v", got)
	}
	if !got.Confirm {
		t.Errorf("confirmation must stay required: %+v", got)
	}
	if !reflect.DeepEqual(got.SecretParams, []string{"password"}) {
		t.Errorf("a secret parameter must not be unmarked by a second declaration: %+v", got.SecretParams)
	}
}
