package main

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

// Plan and apply read ONE decision (decide), so an operator never approves a
// change the apply then makes differently. This drives the same subject through
// both and checks they agree on whether anything is written and on what the
// account is left holding.
func TestPlanAndApplyAgreeOnEveryConvergence(t *testing.T) {
	const bound = `{"subject":"sub-1","email":"ada@example.edu","desired":`
	cases := []struct {
		name    string
		current string
		desired string
		effect  string // what both must say
		writes  bool   // whether user.update must be called
	}{
		{"already converged",
			`"locked":false,"smb":true,"groups":[42]`,
			`{"group":["lab_makers"],"enabled":true,"smb_enabled":true}`, EffectNoChange, false},
		{"groups differ",
			`"locked":false,"smb":true,"groups":[41]`,
			`{"group":["lab_makers"],"enabled":true,"smb_enabled":true}`, EffectApplied, true},
		{"revoked to nothing, locked, smb off",
			`"locked":false,"smb":true,"groups":[41,42]`,
			`{"group":[],"enabled":false,"smb_enabled":false}`, EffectApplied, true},
		{"restored; smb waits for a password",
			`"locked":true,"smb":false,"groups":[]`,
			`{"group":["lab_makers"],"enabled":true,"smb_enabled":true}`, EffectApplied, true},
		{"only smb wanted on: nothing to write, still waiting",
			`"locked":false,"smb":false,"groups":[42]`,
			`{"group":["lab_makers"],"enabled":true,"smb_enabled":true}`, EffectNoChange, false},
		{"only smb off",
			`"locked":false,"smb":true,"groups":[42]`,
			`{"group":["lab_makers"],"enabled":true,"smb_enabled":false}`, EffectApplied, true},
		{"an unmanaged field is left alone",
			`"locked":true,"smb":true,"groups":[41]`,
			`{"group":[]}`, EffectNoChange, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, m := applyServer(t, `[{"username":"ada","id":11,"uid":3001,`+c.current+`}]`)
			if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "ada", UID: 3001}); err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			planInto(t, s, rr, `{"subjects":[`+bound+c.desired+`}]}`)
			var plan PlanResponse
			if err := json.Unmarshal(rr.Body.Bytes(), &plan); err != nil || len(plan.Outcomes) != 1 {
				t.Fatalf("plan: %v %s", err, rr.Body.String())
			}
			planned := plan.Outcomes[0]
			if len(m.updates) != 0 {
				t.Fatal("a plan wrote something")
			}

			applied := decodeOutcome(t, postApply(t, s, `{"call_id":"c1","subject":"sub-1","email":"ada@example.edu","desired":`+c.desired+`}`))

			// A plan says "apply" for what an apply then reports as "applied".
			plannedWant := c.effect
			if plannedWant == EffectApplied {
				plannedWant = EffectApply
			}
			if planned.Effect != plannedWant {
				t.Errorf("plan said %q, want %q (%+v)", planned.Effect, plannedWant, planned)
			}
			if applied.Effect != c.effect {
				t.Errorf("apply said %q, want %q (%+v)", applied.Effect, c.effect, applied)
			}
			if wrote := len(m.updates) > 0; wrote != c.writes {
				t.Errorf("apply wrote=%v, want %v", wrote, c.writes)
			}
			if c.effect == EffectApplied && planned.Consequence != applied.Consequence {
				t.Errorf("the plan promised %q and the apply left %q", planned.Consequence, applied.Consequence)
			}
			if planned.Effect == EffectNoChange && planned.Detail != applied.Detail {
				t.Errorf("plan detail %q and apply detail %q disagree", planned.Detail, applied.Detail)
			}
		})
	}
}

// decide is pure, so it is checked without a NAS at all.
func TestDecideIsPureAndWritesOnlyWhatDiffers(t *testing.T) {
	desired, err := decodeDesiredFields(map[string]json.RawMessage{
		"group": json.RawMessage(`["lab_makers"]`), "enabled": json.RawMessage(`true`), "smb_enabled": json.RawMessage(`true`),
	})
	if err != nil {
		t.Fatal(err)
	}
	same := &Subject{Username: "ada", Groups: []string{"lab_makers"}, Enabled: true, SMBEnabled: true}
	if c := decide(same, desired, homeDecision{}); c.writes() || c.smbPending {
		t.Errorf("a converged subject needs no write: %+v", c)
	}
	smbOff := &Subject{Username: "ada", Groups: []string{"lab_makers"}, Enabled: true}
	if c := decide(smbOff, desired, homeDecision{}); c.writes() || !c.smbPending {
		t.Errorf("SMB wanted on is pending and never a write: %+v", c)
	}
	if c := decide(nil, desired, homeDecision{}); c.groups == nil || c.locked == nil || *c.locked {
		t.Errorf("a new account writes every managed field, unlocked: %+v", c)
	}
}
