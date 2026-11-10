package services

import (
	"encoding/json"
	"testing"
)

// Only a disable no policy change confers back by itself drains without an
// operator: a departure (the directory reports the person gone) or an
// operator's denial of `enabled`. A lock because no mapped role reaches the
// target waits, like every other convergence.
func TestWithdrawsOnly(t *testing.T) {
	departed := EntitlementSet{Lifecycle: LifecycleState{NotAMember: true, Reason: "the directory reports this person as inactive"}}
	denied := EntitlementSet{Suppressed: []SuppressedEntitlement{{Field: FieldEnabled, Value: "true"}}}
	smbDeniedOnly := EntitlementSet{
		Lifecycle:  LifecycleState{Enabled: true},
		Suppressed: []SuppressedEntitlement{{Field: FieldSMBEnabled, Value: "true"}},
	}
	for _, tc := range []struct {
		name string
		set  EntitlementSet
		want bool
	}{
		{"a departure", departed, true},
		{"an operator's denial of the account", denied, true},
		{"no mapped role reaches the target (a cascade's lock)", EntitlementSet{}, false},
		{"an enabled account", EntitlementSet{Lifecycle: LifecycleState{Enabled: true, SMBEnabled: true}}, false},
		{"only SMB denied: the account stays on", smbDeniedOnly, false},
		{"a departure the resolver left enabled cannot be", EntitlementSet{Lifecycle: LifecycleState{NotAMember: true, Enabled: true}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.set.WithdrawsOnly(); got != tc.want {
				t.Fatalf("WithdrawsOnly() = %v, want %v", got, tc.want)
			}
			if c := tc.set.Convergence("truenas", "u1", "system:reconcile", "why"); c.WithdrawsOnly != tc.want || c.Desired == nil {
				t.Fatalf("Convergence carries WithdrawsOnly=%v (want %v) and Desired %v", c.WithdrawsOnly, tc.want, c.Desired)
			}
		})
	}
}

// A withdraw-only instruction is sent with nobody reviewing it, so it carries
// the switch and nothing that could give: groups, home and quota are left out
// (the add-on leaves an absent field alone). Every other set carries its fields,
// including an explicitly empty one, which means "make it empty".
func TestWithdrawOnlyDesiredIsTheSwitchAlone(t *testing.T) {
	fields := map[string][]string{"groups": {"staff", "lab"}, "home": {"yes"}}
	departed := EntitlementSet{Fields: fields, Lifecycle: LifecycleState{NotAMember: true}}
	got := departed.Desired()
	if len(got) != 2 || string(got[FieldEnabled]) != "false" || string(got[FieldSMBEnabled]) != "false" {
		t.Fatalf("a departure's desired = %s, want only enabled=false and smb_enabled=false", fmtDesired(got))
	}
	if c := departed.Convergence("truenas", "u1", "system:reconcile", "why"); len(c.Desired) != 2 {
		t.Fatalf("the queued row must carry the same map as Desired, got %s", fmtDesired(c.Desired))
	}

	enabled := EntitlementSet{Fields: fields, Lifecycle: LifecycleState{Enabled: true, SMBEnabled: true}}
	if got := enabled.Desired(); string(got["groups"]) != `["staff","lab"]` || got["home"] == nil {
		t.Fatalf("an enabled set must carry its fields, got %s", fmtDesired(got))
	}

	// Every value suppressed: locked, but not withdraw-only, so it still says
	// "no groups" and waits for Send like any other change.
	suspended := EntitlementSet{Fields: map[string][]string{"groups": nil}}
	if got := suspended.Desired(); string(got["groups"]) != `[]` {
		t.Fatalf("a fully suspended set must say groups=[], got %s", fmtDesired(got))
	}
}

func fmtDesired(m map[string]json.RawMessage) string {
	b, _ := json.Marshal(m)
	return string(b)
}
