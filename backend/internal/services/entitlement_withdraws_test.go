package services

import "testing"

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
