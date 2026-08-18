package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// The rules TrueNAS enforces on account writes, read from a RECORDING of a real
// target and asserted against the payloads this add-on builds.
//
// `addons/contract/truenas_observed.json` is written by
// `scripts/record-truenas-fixtures.sh --write`, which asks a live NAS and
// writes down what it answered — including its REFUSALS, which are the half no
// hand-written fixture has ever contained and the half that hid a completely
// broken account creation behind two green suites.
//
// Every serious defect this add-on shipped had one shape: a fixture somebody
// wrote by hand, agreeing with the code that read it, disagreeing with the
// target. The recording cannot drift that way, and these tests derive what the
// code must do FROM it rather than from anybody's memory of a debugging
// session.

type observedRules struct {
	ProductVersion string `json:"product_version"`
	WriteRules     []struct {
		Case             string   `json:"case"`
		Method           string   `json:"method"`
		Accepted         bool     `json:"accepted"`
		RefusedFields    []string `json:"refused_fields"`
		PasswordDisabled *bool    `json:"password_disabled"`
	} `json:"write_rules"`
}

func observed(t *testing.T) observedRules {
	t.Helper()
	raw, err := os.ReadFile("../contract/truenas_observed.json")
	if err != nil {
		t.Skipf("no recording yet (%v). Run scripts/record-truenas-fixtures.sh --write "+
			"against a real target; these rules cannot be known any other way.", err)
	}
	var o observedRules
	if err := json.Unmarshal(raw, &o); err != nil {
		t.Fatalf("the recording does not parse: %v", err)
	}
	if o.ProductVersion == "" {
		t.Fatal("the recording does not name the release it came from; a fixture from " +
			"one major says nothing about another")
	}
	return o
}

func rule(t *testing.T, o observedRules, name string) (refused []string, accepted bool, found bool) {
	t.Helper()
	for _, r := range o.WriteRules {
		if r.Case == name {
			return r.RefusedFields, r.Accepted, true
		}
	}
	return nil, false, false
}

// Every rule below is DERIVED from the recording: the assertion fires only
// because the target refused something, and stops firing if a later release
// stops refusing it.
func TestTheCreatePayloadSatisfiesWhatTheTargetRefused(t *testing.T) {
	o := observed(t)
	src := source(t, "apply.go")
	create := between(t, src, "create := map[string]any{", "}")

	if fields, _, found := rule(t, o, "create with no credential decision"); found && len(fields) > 0 {
		// The target refused a create carrying no password decision, naming
		// exactly which field it wanted.
		if !strings.Contains(create, "password_disabled") && !strings.Contains(create, `"password"`) {
			t.Errorf("%s refused a create with no credential decision (%s), and this "+
				"payload still sends neither. Account creation cannot work.",
				o.ProductVersion, strings.Join(fields, ", "))
		}
	}

	if fields, _, found := rule(t, o, "create with smb and disabled password"); found && len(fields) > 0 {
		if strings.Contains(create, "desired.smbEnabled") {
			t.Errorf("%s refuses SMB alongside disabled password authentication (%s), "+
				"and this payload still asks for SMB from the desired state.",
				o.ProductVersion, strings.Join(fields, ", "))
		}
	}
}

func TestThePasswordPathSatisfiesWhatTheTargetRefused(t *testing.T) {
	o := observed(t)
	src := source(t, "operations.go")

	// The recording captured the trap directly: an update carrying only
	// `password` is ACCEPTED and leaves password_disabled true.
	for _, r := range o.WriteRules {
		if r.Case == "password alone, resulting password_disabled" && r.PasswordDisabled != nil && *r.PasswordDisabled {
			if !strings.Contains(src, `"password_disabled": false`) {
				t.Errorf("on %s an update carrying only a password leaves authentication "+
					"DISABLED, so the member is told their password was set by an account "+
					"that still refuses them. The password path must clear it.", o.ProductVersion)
			}
		}
	}

	if fields, _, found := rule(t, o, "enable smb in a later call"); found && len(fields) > 0 {
		if !strings.Contains(src, "desiredSMB(") {
			t.Errorf("%s refuses a standalone SMB enable (%s), so the only call that can "+
				"turn it on is the one setting the password — and this path never does.",
				o.ProductVersion, strings.Join(fields, ", "))
		}
		if !strings.Contains(source(t, "apply.go"), "smbPending") {
			t.Errorf("%s refuses a standalone SMB enable, so the convergence must not "+
				"attempt one; without that it fails on every pass for an account whose "+
				"member has not set a password.", o.ProductVersion)
		}
	}
}

// The recording must actually contain the refusals, or every assertion above is
// vacuous and the suite is green because nothing was recorded.
func TestTheRecordingCarriesTheRefusalsTheseRulesRestOn(t *testing.T) {
	o := observed(t)
	for _, name := range []string{
		"create with no credential decision",
		"create with smb and disabled password",
		"enable smb in a later call",
	} {
		fields, _, found := rule(t, o, name)
		if !found {
			t.Errorf("the recording has no case %q — re-run "+
				"scripts/record-truenas-fixtures.sh --write", name)
			continue
		}
		if len(fields) == 0 {
			t.Errorf("case %q recorded no refused field. Either the target stopped "+
				"refusing it — in which case the rules above are obsolete and should be "+
				"revisited — or the probe did not reach it.", name)
		}
	}
}

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
