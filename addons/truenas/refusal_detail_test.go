package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// A refusal must name the FIELD it rejected, and must never carry the target's
// own prose.
//
// The two halves are one decision. `user.update({password})` puts a member's
// credential in the parameters of the call most likely to produce a validation
// message, so the message is the one thing that cannot leave this package — but
// the field PATH is structural, names a key of the request schema, and cannot
// contain a value. Without it a caller sees `target error code -32602` and
// nothing else, which is exactly what hid a broken account creation across
// every supported release while the NAS was saying `user_create.password:
// Password is required` on every attempt.
func TestARefusalNamesTheFieldAndNotTheMessage(t *testing.T) {
	// Recorded verbatim from TrueNAS 25.10.5, 2026-08-18.
	const real = `{"code":-32602,"message":"Invalid params","data":{"extra":[["user_create.password","Password is required",22]]}}`
	var e rpcError
	if err := json.Unmarshal([]byte(real), &e); err != nil {
		t.Fatal(err)
	}

	fields := e.validationFields()
	if len(fields) != 1 || fields[0] != "user_create.password" {
		t.Fatalf("fields = %v, want [user_create.password]", fields)
	}
	for _, f := range fields {
		if strings.Contains(f, "Password is required") {
			t.Error("the target's message escaped through the field list")
		}
	}
}

// Anything that is not a schema path is dropped rather than forwarded on the
// assumption that it must be one.
func TestOnlyIdentifierShapedPathsAreForwarded(t *testing.T) {
	for name, payload := range map[string]string{
		"a sentence where a path belongs": `{"extra":[["the password 'hunter2' is too short","x",22]]}`,
		"a value-shaped entry":            `{"extra":[["hunter2 is weak","x",22]]}`,
		"not an array at all":             `{"extra":["user_create.password"]}`,
		"empty":                           `{"extra":[]}`,
	} {
		t.Run(name, func(t *testing.T) {
			var e rpcError
			if err := json.Unmarshal([]byte(`{"code":-32602,"data":`+payload+`}`), &e); err != nil {
				t.Fatal(err)
			}
			for _, f := range e.validationFields() {
				if strings.ContainsAny(f, " '\"") {
					t.Errorf("forwarded a non-path: %q", f)
				}
			}
		})
	}
}

// And the multi-field case, which is what a create with two problems returns.
func TestEveryRejectedFieldIsNamed(t *testing.T) {
	const two = `{"code":-32602,"data":{"extra":[` +
		`["user_create.password","Password is required",22],` +
		`["user_create.password_disabled","Password authentication may not be disabled for SMB users.",22]]}}`
	var e rpcError
	if err := json.Unmarshal([]byte(two), &e); err != nil {
		t.Fatal(err)
	}
	got := strings.Join(e.validationFields(), ",")
	if got != "user_create.password,user_create.password_disabled" {
		t.Fatalf("got %q; an operator fixing one of two problems and re-running is "+
			"the loop this exists to prevent", got)
	}
}
