//go:build ignore

// This file wrote state-contract-1.db with PRODUCTION's store code (contract 1,
// origin/main 981efba3), so TestAStoreWrittenByContractOneOpensWhole reads a
// file the old binary really wrote, the way testdata/mutations-before-intents.log
// holds the mutation log (7l). Kept for the record, never built here: the old
// package's types are not this one's.
//
// To write it again:
//
//	git archive 981efba3 addons/truenas | tar -x -C "$TMP"
//	cp state-contract-1.gen.go "$TMP/addons/truenas/gen_state_test.go"   # drop the build tag
//	cd "$TMP/addons/truenas" && STATE_OUT=/abs/state-contract-1.db go test -run TestWriteContractOneState .
//
// WWE ring names and example.org only.
package main

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestWriteContractOneState(t *testing.T) {
	out := os.Getenv("STATE_OUT")
	if out == "" {
		t.Skip("STATE_OUT is not set")
	}
	_ = os.Remove(out)
	st, err := OpenStore(out)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	at := time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)
	st.now = func() time.Time { return at }
	yes, no := true, false

	for _, b := range []Binding{
		{SubjectID: "sub-rey", Username: "rey_mysterio", UID: 3051, BoundBy: "apply", WantsSMB: &yes},
		{SubjectID: "sub-becky", Username: "becky_lynch", UID: 3052, BoundBy: "adopt:op-undertaker"},
		{SubjectID: "sub-cena", Username: "john_cena", UID: 3053, BoundBy: "apply", WantsSMB: &no},
	} {
		if err := st.PutBinding(b); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.PutSnapshot(Snapshot{TakenAt: at, Subjects: []Subject{
		{ID: apiID{raw: json.RawMessage("11")}, Username: "rey_mysterio", UID: 3051, Groups: []string{"lab_makers"}, Home: "/mnt/tank/homes/rey_mysterio", Enabled: true, SMBEnabled: true, PasswordSet: true},
		{ID: apiID{raw: json.RawMessage("12")}, Username: "becky_lynch", UID: 3052, Groups: []string{}, Enabled: true, PasswordSet: false},
		{ID: apiID{raw: json.RawMessage("13")}, Username: "john_cena", UID: 3053, Groups: []string{"fabrication"}, Enabled: false, PasswordSet: true},
		{ID: apiID{raw: json.RawMessage(`"4"`)}, Username: "syndra", UID: 3001, Groups: []string{}, Enabled: true, PasswordSet: true, Self: true},
	}}); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		id, kind string
		result   any
	}{
		{"c1-apply-rey", kindApply, ApplyOutcome{Subject: "sub-rey", Effect: "applied", Detail: "Created rey_mysterio.",
			Username: "rey_mysterio", UID: 3051, Fingerprint: "fp-rey"}},
		{"c1-adopt-becky", kindOperation + "account.adopt", OperationResult{Operation: "account.adopt", Subject: "sub-becky",
			Outcome: "succeeded", AccountUID: 3052}},
		{"c1-password-rey", kindOperation + "password.set", OperationResult{Operation: "password.set", Subject: "sub-rey", Outcome: "succeeded"}},
		{"c1-rotate-cena", kindOperation + "password.rotate", OperationResult{Operation: "password.rotate", Subject: "sub-cena",
			Outcome: "succeeded", Rotated: true}},
		{"c1-setting-home", kindSetting + settingHomeRoot, SettingView{Name: settingHomeRoot, Label: "Home folders",
			Value: "/mnt/tank/homes", SetBy: "op-undertaker", SetAt: &at, Values: []string{"/mnt/tank/homes"}, ValuesRead: true}},
	} {
		if err := st.Remember(c.id, c.kind, c.result); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.PutSetting(settingHomeRoot, Setting{Value: "/mnt/tank/homes", SetBy: "op-undertaker", SetAt: at}); err != nil {
		t.Fatal(err)
	}
	if id, err := st.ClaimID(3000, 60000, 3053); err != nil || id != 3054 {
		t.Fatalf("claim: %d %v", id, err)
	}
}
