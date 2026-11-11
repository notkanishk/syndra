package services

import (
	"context"
	"encoding/json"
	"testing"

	"syndra/internal/addons"
	"syndra/internal/db"
)

// Two targets that share no field name (addon-contract 2.1): TrueNAS, and one
// whose account is `active` with a channel `web`. Every rule below is by role.
var withdrawShapes = []struct {
	name             string
	roles            addons.Roles
	account, channel string
}{
	{"truenas", truenasRoles, FieldEnabled, FieldSMBEnabled},
	{"an account called active", addons.Roles{"groups": addons.RoleMembership, "home": addons.RoleFlag,
		"active": addons.RoleAccount, "web": addons.RoleChannel}, "active", "web"},
}

// Only a disable no policy change confers back by itself drains without an
// operator: a departure (the directory reports the person gone) or an
// operator's denial of the account field. A lock because no mapped role
// reaches the target waits, like every other convergence.
func TestWithdrawsOnly(t *testing.T) {
	for _, shape := range withdrawShapes {
		on := map[string]bool{shape.channel: true}
		departed := EntitlementSet{Roles: shape.roles, Lifecycle: LifecycleState{NotAMember: true, Reason: "the directory reports this person as inactive"}}
		denied := EntitlementSet{Roles: shape.roles, Suppressed: []SuppressedEntitlement{{Field: shape.account, Value: "true"}}}
		channelDeniedOnly := EntitlementSet{
			Roles:      shape.roles,
			Lifecycle:  LifecycleState{Account: true},
			Suppressed: []SuppressedEntitlement{{Field: shape.channel, Value: "true"}},
		}
		for _, tc := range []struct {
			name string
			set  EntitlementSet
			want bool
		}{
			{"a departure", departed, true},
			{"an operator's denial of the account", denied, true},
			{"no mapped role reaches the target (a cascade's lock)", EntitlementSet{Roles: shape.roles}, false},
			{"an enabled account", EntitlementSet{Roles: shape.roles, Lifecycle: LifecycleState{Account: true, Channels: on}}, false},
			{"only the channel denied: the account stays on", channelDeniedOnly, false},
			{"a departure the resolver left enabled cannot be", EntitlementSet{Roles: shape.roles, Lifecycle: LifecycleState{NotAMember: true, Account: true}}, false},
			{"a channel left on is not withdraw-only", EntitlementSet{Roles: shape.roles, Lifecycle: LifecycleState{NotAMember: true, Channels: on}}, false},
			{"a denial of a channel's name on another target's account is not the account", EntitlementSet{Roles: shape.roles,
				Suppressed: []SuppressedEntitlement{{Field: shape.channel, Value: "true"}}}, false},
		} {
			t.Run(shape.name+"/"+tc.name, func(t *testing.T) {
				if got := tc.set.WithdrawsOnly(); got != tc.want {
					t.Fatalf("WithdrawsOnly() = %v, want %v", got, tc.want)
				}
				if c := tc.set.Convergence("truenas", "u1", "system:reconcile", "why"); c.WithdrawsOnly != tc.want || c.Desired == nil {
					t.Fatalf("Convergence carries WithdrawsOnly=%v (want %v) and Desired %v", c.WithdrawsOnly, tc.want, c.Desired)
				}
			})
		}
	}
}

// A withdraw-only instruction is sent with nobody reviewing it, so it carries
// the switch and nothing that could give: groups, home and quota are left out
// (the add-on leaves an absent field alone). Every other set carries its fields,
// including an explicitly empty one, which means "make it empty".
func TestWithdrawOnlyDesiredIsTheSwitchAlone(t *testing.T) {
	for _, shape := range withdrawShapes {
		t.Run(shape.name, func(t *testing.T) {
			fields := map[string][]string{"groups": {"staff", "lab"}, "home": {"yes"}}
			departed := EntitlementSet{Roles: shape.roles, Fields: fields, Lifecycle: LifecycleState{NotAMember: true}}
			got := departed.Desired()
			if len(got) != 2 || string(got[shape.account]) != "false" || string(got[shape.channel]) != "false" {
				t.Fatalf("a departure's desired = %s, want only %s=false and %s=false", fmtDesired(got), shape.account, shape.channel)
			}
			if c := departed.Convergence("truenas", "u1", "system:reconcile", "why"); len(c.Desired) != 2 {
				t.Fatalf("the queued row must carry the same map as Desired, got %s", fmtDesired(c.Desired))
			}

			enabled := EntitlementSet{Roles: shape.roles, Fields: fields, Lifecycle: LifecycleState{Account: true, Channels: map[string]bool{shape.channel: true}}}
			if got := enabled.Desired(); string(got["groups"]) != `["staff","lab"]` || got["home"] == nil ||
				string(got[shape.account]) != "true" || string(got[shape.channel]) != "true" {
				t.Fatalf("an enabled set must carry its fields and switches, got %s", fmtDesired(got))
			}

			// Every value suppressed: locked, but not withdraw-only, so it still
			// says "no groups" and waits for Send like any other change; every
			// lifecycle field is written, the channel off by omission.
			suspended := EntitlementSet{Roles: shape.roles, Fields: map[string][]string{"groups": nil}}
			if got := suspended.Desired(); string(got["groups"]) != `[]` || string(got[shape.account]) != "false" || string(got[shape.channel]) != "false" {
				t.Fatalf("a fully suspended set must say groups=[] and both switches off, got %s", fmtDesired(got))
			}
		})
	}
}

func fmtDesired(m map[string]json.RawMessage) string {
	b, _ := json.Marshal(m)
	return string(b)
}

// The resolver names no field (addon-contract 2.1): against a target whose
// account is `active` and whose channel is `web`, a denial of `active` locks
// both and is withdraw-only, a departure is the switch alone, a mapping that
// names `active` is ignored, and a target with no account field cannot be
// resolved at all, because there would be nothing to switch off.
func TestTheResolverReadsTheLifecycleByRole(t *testing.T) {
	ruby := addons.Roles{"groups": addons.RoleMembership, "active": addons.RoleAccount, "web": addons.RoleChannel}
	orig := svcFieldRoles
	t.Cleanup(func() { svcFieldRoles = orig })
	svcFieldRoles = func(context.Context, string) (addons.Roles, error) { return ruby, nil }
	maker := []db.RoleRef{{ProjectID: "pLab", RoleKey: "maker"}}
	groups := []db.RoleMapping{mapping("maker", "groups", "staff"), mapping("maker", "active", "false")}

	held := resolve(t, &resolverFixture{roles: maker, mappings: groups})
	if !held.Lifecycle.Account || !held.Lifecycle.Channels["web"] || held.WithdrawsOnly() {
		t.Fatalf("a held role switches the account and its channel on: %+v", held.Lifecycle)
	}
	if _, bound := held.Fields["active"]; bound {
		t.Fatalf("a mapping naming the account field must be ignored: %v", held.Fields)
	}
	if got := held.Desired(); string(got["active"]) != "true" || string(got["web"]) != "true" || string(got["groups"]) != `["staff"]` {
		t.Fatalf("desired %s", fmtDesired(got))
	}

	denied := resolve(t, &resolverFixture{roles: maker, mappings: groups, allowances: []db.Allowance{deny("active", "true")}})
	if denied.Lifecycle.Account || denied.Lifecycle.Channels["web"] || !denied.WithdrawsOnly() {
		t.Fatalf("denying the account takes its channel and drains on its own: %+v", denied.Lifecycle)
	}
	if got := denied.Desired(); len(got) != 2 || string(got["active"]) != "false" || string(got["web"]) != "false" {
		t.Fatalf("a withdrawal is the switch alone, got %s", fmtDesired(got))
	}

	channelOnly := resolve(t, &resolverFixture{roles: maker, mappings: groups, allowances: []db.Allowance{deny("web", "true")}})
	if !channelOnly.Lifecycle.Account || channelOnly.Lifecycle.Channels["web"] || channelOnly.WithdrawsOnly() {
		t.Fatalf("a channel denial leaves the account on: %+v", channelOnly.Lifecycle)
	}

	departed := resolve(t, &resolverFixture{roles: maker, mappings: groups, status: "inactive"})
	if got := departed.Desired(); !departed.WithdrawsOnly() || len(got) != 2 || string(got["active"]) != "false" {
		t.Fatalf("a departure is the switch alone, got %s", fmtDesired(got))
	}

	svcFieldRoles = func(context.Context, string) (addons.Roles, error) {
		return addons.Roles{"groups": addons.RoleMembership}, nil
	}
	(&resolverFixture{roles: maker, mappings: groups}).install(t)
	if _, err := ResolveEntitlements(context.Background(), "u1", "truenas"); err == nil {
		t.Fatal("a target with no account field must not resolve")
	}
	svcFieldRoles = func(context.Context, string) (addons.Roles, error) { return nil, addons.ErrNoManifest }
	if _, err := ResolveEntitlements(context.Background(), "u1", "truenas"); err == nil {
		t.Fatal("a target whose roles cannot be read must not resolve")
	}
}

// A channel the manifest dropped is retired (addons.FieldRoles leaves it out),
// and the target refuses a field outside its schema: so a departure's
// withdraw-only lock is exactly the account switched off, never the dropped
// channel as false (which would make every lock a target refusal), and a
// mapping still naming a retired field confers nothing on anyone (§37,
// addon-contract 2.1).
func TestARetiredChannelIsNeverSent(t *testing.T) {
	retired := addons.Roles{"group": addons.RoleMembership, FieldEnabled: addons.RoleAccount} // smb_enabled dropped
	orig := svcFieldRoles
	t.Cleanup(func() { svcFieldRoles = orig })
	svcFieldRoles = func(context.Context, string) (addons.Roles, error) { return retired, nil }
	maker := []db.RoleRef{{ProjectID: "pLab", RoleKey: "maker"}}
	mappings := []db.RoleMapping{mapping("maker", "group", "staff"), mapping("maker", "home", "true")} // home retired too

	departed := resolve(t, &resolverFixture{roles: maker, mappings: mappings, status: "inactive"})
	if got := departed.Desired(); !departed.WithdrawsOnly() || len(got) != 1 || string(got[FieldEnabled]) != "false" {
		t.Fatalf("the lock must be exactly {enabled:false}, got %s", fmtDesired(got))
	}
	ownLimit := db.Allowance{ID: "a2", Target: "truenas", Field: "home_quota", Value: "1", Direction: db.AllowanceSet, ActorID: "op_1", Reason: "r"}
	held := resolve(t, &resolverFixture{roles: maker, mappings: mappings, allowances: []db.Allowance{ownLimit}})
	if got := held.Desired(); len(got) != 2 || string(got[FieldEnabled]) != "true" || string(got["group"]) != `["staff"]` {
		t.Fatalf("only declared fields are sent, got %s", fmtDesired(got))
	}
	// A role whose only mapping names a retired field reaches nothing.
	only := resolve(t, &resolverFixture{roles: maker, mappings: []db.RoleMapping{mapping("maker", "home", "true")}})
	if only.Lifecycle.Account || len(only.Given) != 0 {
		t.Fatalf("a retired field's mapping confers nothing, not even the account: %+v", only)
	}
}
