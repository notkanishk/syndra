package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// The rules TrueNAS 25.10.5 enforces on account writes, pinned as source guards.
//
// Every one of these was learned by being refused by a real NAS on 2026-08-18,
// and every one of them is invisible to a fake: the add-on's own recorded
// fixtures answered `user.create` with a success for a payload the target
// rejects outright. Account creation was broken against every supported release
// and no test could see it, because the only party that knew the rule was the
// target.
//
//	user_create.password        Password is required
//	user_create.password_disabled
//	                            Password authentication may not be disabled
//	                            for SMB users.
//	user_update.smb             Password must be reset in order to enable SMB
//	                            authentication
//
// These read the payload the add-on BUILDS rather than mocking a response,
// because the defect was never in how a response was handled — it was in what
// was sent.

// A create must decide the credential question, and Syndra's answer is "there
// is no credential yet".
func TestCreatePayloadDisablesPasswordAuthentication(t *testing.T) {
	src := source(t, "apply.go")
	if !strings.Contains(src, `"password_disabled": true`) {
		t.Error("user.create sends neither password nor password_disabled.\n" +
			"TrueNAS refuses that outright — `user_create.password: Password is " +
			"required` — so account creation fails for every subject, always.")
	}
	if strings.Contains(src, `"random_password"`) {
		t.Error("a random password at creation is a credential nobody asked for, " +
			"returned in the create response, and never delivered to the member")
	}
}

// SMB cannot be requested at creation, whatever the entitlement says.
func TestCreateNeverRequestsSMB(t *testing.T) {
	src := source(t, "apply.go")
	create := between(t, src, "create := map[string]any{", "}")
	if strings.Contains(create, "desired.smbEnabled") {
		t.Error("user.create requests SMB from the desired state.\n" +
			"TrueNAS refuses `smb: true` alongside disabled password " +
			"authentication — `Password authentication may not be disabled for " +
			"SMB users` — and Syndra has no password at creation, so this fails " +
			"every create for an SMB-entitled member.")
	}
	if !strings.Contains(create, `"smb":    false`) && !strings.Contains(create, `"smb": false`) {
		t.Error("user.create must state smb: false explicitly, so the omission is a decision")
	}
}

// The password call carries the two fields that only work together.
func TestPasswordCallClearsDisabledAndCanEnableSMB(t *testing.T) {
	src := source(t, "operations.go")
	if !strings.Contains(src, `"password_disabled": false`) {
		t.Error("the password update does not clear password_disabled.\n" +
			"TrueNAS accepts an update carrying only `password` and leaves " +
			"authentication DISABLED — the member is told their password was set " +
			"and the account still refuses them.")
	}
	if !strings.Contains(src, "desiredSMB(") {
		t.Error("the password update never enables SMB.\n" +
			"It is the only call that can: a later `user.update({smb: true})` is " +
			"refused with `Password must be reset in order to enable SMB " +
			"authentication`.")
	}
}

// A rotation is the credential half of a revocation and must not grant access.
func TestRotationDoesNotTouchSMB(t *testing.T) {
	src := source(t, "operations.go")
	rot := after(t, src, "func (s *server) rotatePassword")
	if strings.Contains(rot, `"smb"`) {
		t.Error("rotatePassword touches smb; a rotation exists to remove access, " +
			"not to grant it")
	}
	if !strings.Contains(rot, `"password_disabled": false`) {
		t.Error("rotatePassword leaves password_disabled as it found it, so a " +
			"rotation onto an account with disabled authentication reports " +
			"success and changes nothing usable")
	}
}

// The state read has to carry the fact the rules turn on.
func TestTheStateReadCarriesWhetherAPasswordExists(t *testing.T) {
	src := source(t, "subjects.go")
	if !strings.Contains(src, `"password_disabled"`) {
		t.Fatal("password_disabled is not selected in the user read, so nothing " +
			"can tell an account that does not want SMB from one that cannot " +
			"have it yet")
	}
	var subject Subject
	if err := json.Unmarshal([]byte(`{"password_set":true}`), &subject); err != nil || !subject.PasswordSet {
		t.Error("Subject does not carry PasswordSet through the mirror")
	}
}

// source reads a file in this package, through the helper apply_test.go already
// has — a second reader of the same files is a second thing to keep in step.
func source(t *testing.T, name string) string {
	t.Helper()
	s, err := readSource(name)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func between(t *testing.T, src, start, end string) string {
	t.Helper()
	i := strings.Index(src, start)
	if i < 0 {
		t.Fatalf("%q not found; the payload this guard reads has been renamed", start)
	}
	rest := src[i+len(start):]
	j := strings.Index(rest, "\n\t"+end)
	if j < 0 {
		t.Fatalf("no closing %q after %q", end, start)
	}
	return rest[:j]
}

func after(t *testing.T, src, marker string) string {
	t.Helper()
	i := strings.Index(src, marker)
	if i < 0 {
		t.Fatalf("%q not found", marker)
	}
	rest := src[i:]
	if j := strings.Index(rest[1:], "\nfunc "); j > 0 {
		return rest[:j]
	}
	return rest
}
