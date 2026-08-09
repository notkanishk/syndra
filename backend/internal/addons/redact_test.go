package addons

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"strings"
	"testing"
)

const theSecret = "correct-horse-battery-staple"

// scanFor fails if the secret appears anywhere in s. Substring, not equality:
// the leak that matters is a password embedded in a longer rendering, not one
// sitting alone in a field somebody remembered to check.
func scanFor(t *testing.T, what, s string) {
	t.Helper()
	if strings.Contains(s, theSecret) {
		t.Fatalf("%s leaked the secret value: %s", what, s)
	}
}

// 2.7, 2.8 — the secret value survives no rendering of the request. %v and %#v
// are covered explicitly because they are the two verbs a future caller reaches
// for without thinking, and a redaction that depends on every caller
// remembering to redact is not a redaction.
func TestSecretParamNeverAppearsInAnyRendering(t *testing.T) {
	installAddon(t, Registration{Target: "truenas", BaseURL: "http://x", SigningKeyPath: "/k"}, goodManifest())
	req := passwordSet(map[string]any{"password": theSecret, "username": "kanishk"})

	scanFor(t, "String()", req.String())
	scanFor(t, "%v", fmt.Sprintf("%v", req))
	scanFor(t, "%s", fmt.Sprintf("%s", req))
	scanFor(t, "%#v", fmt.Sprintf("%#v", req))
	scanFor(t, "%+v", fmt.Sprintf("%+v", req))
	scanFor(t, "RedactedParams", fmt.Sprint(RedactedParams("truenas", "password.set", req.Params)))

	// The non-secret parameter must survive, or the redaction has destroyed the
	// diagnostic value that justifies logging anything at all.
	if !strings.Contains(req.String(), "kanishk") {
		t.Fatal("redaction removed a non-secret parameter")
	}
	if !strings.Contains(req.String(), redactedValue) {
		t.Fatal("the redacted parameter left no marker that something was withheld")
	}
}

// 2.7, 2.8 — the transport's own logging is part of the surface. A failed call
// is exactly when a diagnostic gets written and exactly when the request body
// is still in hand.
func TestFailedCallLoggingCarriesNoSecret(t *testing.T) {
	var buf bytes.Buffer
	saved := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(saved) })

	// No server: the call fails at dial, which is the path that logs.
	installAddon(t, Registration{Target: "truenas", BaseURL: "https://127.0.0.1:1", SigningKeyPath: mustKeyFile(t)}, goodManifest())
	resp := Call(context.Background(), passwordSet(map[string]any{"password": theSecret}))
	if resp.Outcome == OutcomeSucceeded {
		t.Fatal("setup: the call was supposed to fail")
	}

	scanFor(t, "transport log", buf.String())
	scanFor(t, "returned error", fmt.Sprint(resp.Err))
	if buf.Len() == 0 {
		t.Fatal("a failed dispatch logged nothing at all — the redaction test proves nothing")
	}
}

// 2.7 — redaction fails closed. The state in which the backend does not know
// which parameters are secret is the state in which it must treat all of them
// as secret, not none.
func TestRedactionFailsClosedWhenTheOperationCannotBeResolved(t *testing.T) {
	resetRegistry(t) // nothing registered at all

	out := RedactedParams("truenas", "password.set", map[string]any{
		"password": theSecret,
		"username": "kanishk",
	})
	for k, v := range out {
		if v != redactedValue {
			t.Fatalf("unresolvable operation left %q readable as %v", k, v)
		}
	}
	if len(out) != 2 {
		t.Fatalf("keys were dropped as well as values: %v", out)
	}
}

// 2.7, 2.34 — policy is authoritative on what is secret. A manifest that omits
// the declaration cannot thereby make the value loggable; the effective set
// takes the union.
func TestManifestOmittingAPolicySecretCannotMakeItLoggable(t *testing.T) {
	m := goodManifest()
	for i := range m.Operations {
		if m.Operations[i].ID == "password.set" {
			m.Operations[i].SecretParams = nil
		}
	}
	installAddon(t, Registration{Target: "truenas", BaseURL: "http://x", SigningKeyPath: "/k"}, m)

	out := RedactedParams("truenas", "password.set", map[string]any{"password": theSecret})
	if out["password"] != redactedValue {
		t.Fatalf("a manifest silence exposed a policy-declared secret: %v", out["password"])
	}
}

// 2.7 — the walk reaches nested values. Policy declares flat parameters, so
// this should not arise; "should not arise" is not a property worth betting a
// password on when the alternative costs a dozen lines.
func TestRedactionReachesNestedValues(t *testing.T) {
	installAddon(t, Registration{Target: "truenas", BaseURL: "http://x", SigningKeyPath: "/k"}, goodManifest())

	out := RedactedParams("truenas", "password.set", map[string]any{
		"batch": []any{
			map[string]any{"user": "a", "password": theSecret},
			map[string]any{"user": "b", "password": theSecret},
		},
	})
	scanFor(t, "nested redaction", fmt.Sprint(out))
}

// 2.7 — redaction copies. A redactor that mutated its input would strip the
// value the caller is about to send, turning a logging concern into a wrong
// password on the target.
func TestRedactionDoesNotMutateTheCallersParams(t *testing.T) {
	installAddon(t, Registration{Target: "truenas", BaseURL: "http://x", SigningKeyPath: "/k"}, goodManifest())

	params := map[string]any{"password": theSecret}
	_ = RedactedParams("truenas", "password.set", params)
	if params["password"] != theSecret {
		t.Fatalf("redaction mutated the caller's map: %v", params["password"])
	}
}
