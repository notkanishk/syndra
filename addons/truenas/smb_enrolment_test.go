package main

import (
	"net/http"
	"strings"
	"testing"
)

// The member's first password is the only call TrueNAS accepts SMB on, so it
// must carry what Syndra WANTS for SMB. It used to carry what the NAS HAD: the
// snapshot, which for a new account says SMB is off because nothing could turn
// it on yet. Every account Syndra created stayed off SMB for good.
func TestTheFirstPasswordSwitchesOnTheSMBTheEntitlementWants(t *testing.T) {
	s, m := applyServer(t, `[{"username":"ada","id":11,"uid":3001,"locked":false,"smb":false,"password_disabled":true,"groups":[42]}]`)
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "ada", UID: 3001}); err != nil {
		t.Fatal(err)
	}

	rr := postApply(t, s, `{"call_id":"c1","subject":"sub-1","email":"ada@example.edu",
		"desired":{"group":["lab_makers"],"enabled":true,"smb_enabled":true}}`)
	if out := decodeOutcome(t, rr); out.Effect != EffectNoChange {
		t.Fatalf("SMB waits for the password, want no change, got %s (%s)", out.Effect, rr.Body.String())
	}

	pw := postOperation(t, s, "password.set",
		`{"call_id":"c2","subject":"sub-1","actor":"sub-1","params":{"password":"`+theSecret+`"}}`)
	if pw.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", pw.Code, pw.Body.String())
	}
	last := m.updates[len(m.updates)-1]
	if last["smb"] != true {
		t.Fatalf("the first password must switch on the SMB the entitlement wants, got %v", last)
	}
}

func TestThePasswordNeverSwitchesOnSMBNobodyWants(t *testing.T) {
	s, m := applyServer(t, `[{"username":"ada","id":11,"uid":3001,"locked":false,"smb":false,"password_disabled":true,"groups":[42]}]`)
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "ada", UID: 3001}); err != nil {
		t.Fatal(err)
	}
	postApply(t, s, `{"call_id":"c1","subject":"sub-1","email":"ada@example.edu",
		"desired":{"group":["lab_makers"],"enabled":true,"smb_enabled":false}}`)

	pw := postOperation(t, s, "password.set",
		`{"call_id":"c2","subject":"sub-1","actor":"sub-1","params":{"password":"`+theSecret+`"}}`)
	if pw.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", pw.Code, pw.Body.String())
	}
	if _, set := m.updates[len(m.updates)-1]["smb"]; set {
		t.Fatal("a password change must never grant share access the entitlement does not")
	}
}

// An account with a password and SMB off (switched off by a revocation, or
// stranded by the old password path). A standalone `smb: true` is refused by
// TrueNAS and took the group change in the same update down with it.
func TestSMBIsNeverSwitchedOnAloneAndTheGroupsStillLand(t *testing.T) {
	s, m := applyServer(t, `[{"username":"ada","id":11,"uid":3001,"locked":false,"smb":false,"password_disabled":false,"groups":[]}]`)
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "ada", UID: 3001}); err != nil {
		t.Fatal(err)
	}
	rr := postApply(t, s, `{"call_id":"c1","subject":"sub-1","email":"ada@example.edu",
		"desired":{"group":["lab_makers"],"enabled":true,"smb_enabled":true}}`)
	out := decodeOutcome(t, rr)
	if out.Effect != EffectApplied {
		t.Fatalf("the groups must land, got %s (%s)", out.Effect, rr.Body.String())
	}
	if _, sent := m.updates[0]["smb"]; sent {
		t.Fatalf("smb: true alone is refused by TrueNAS, and must not be sent: %v", m.updates[0])
	}
	if !strings.Contains(out.Detail, "set their password again") {
		t.Errorf("the outcome must say SMB is still waiting, got %q", out.Detail)
	}

	pw := postOperation(t, s, "password.set",
		`{"call_id":"c2","subject":"sub-1","actor":"sub-1","params":{"password":"`+theSecret+`"}}`)
	if pw.Code != http.StatusOK || m.updates[len(m.updates)-1]["smb"] != true {
		t.Fatalf("the next password must carry SMB on: %d %v", pw.Code, m.updates[len(m.updates)-1])
	}
}
