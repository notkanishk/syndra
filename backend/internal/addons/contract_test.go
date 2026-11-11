package addons

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The backend half of the wire contract, major 2 (see addons/contract/README.md).
//
// These assert that what this package MARSHALS is byte-for-byte the document
// the add-on's own suite decodes strictly. Two modules, no shared type, so the
// artifact is the only thing that can hold them together — and it has to,
// because the defect this replaces was invisible from either side alone: the
// backend sent `contract_version` and the add-on's `ApplyRequest` did not
// declare it, so every real apply would have been refused as a malformed body
// while both suites passed.
//
// The comparison is over the WHOLE document rather than over a field list. A
// list is a third place to forget something, and forgetting there fails nothing.

// contractFixture reads one artifact. The path is relative and deliberately so:
// the fixture lives with the add-on it is a contract with, not inside either
// module, and a copy under `internal/` would be a second document free to
// disagree with the first.
func contractFixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(contractDir, "fixtures", name))
	if err != nil {
		t.Fatalf("read contract fixture %s: %v", name, err)
	}
	return raw
}

// fixtureWithout is a fixture less some keys: the keys a feature this backend
// does not send introduces, or a value only the run decides (a call_id minted
// from a row).
func fixtureWithout(t *testing.T, name string, keys ...string) []byte {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(contractFixture(t, name), &doc); err != nil {
		t.Fatal(err)
	}
	for _, k := range keys {
		delete(doc, k)
	}
	out, _ := json.Marshal(doc)
	return out
}

// sameJSON compares two documents by value, ignoring key order and whitespace.
// Anything else would make `gofmt`-of-JSON a contract change.
func sameJSON(t *testing.T, got, want []byte, what string) {
	t.Helper()
	var g, w any
	if err := json.Unmarshal(got, &g); err != nil {
		t.Fatalf("%s: produced document does not parse: %v", what, err)
	}
	if err := json.Unmarshal(want, &w); err != nil {
		t.Fatalf("%s: fixture does not parse: %v", what, err)
	}
	gn, _ := json.Marshal(g)
	wn, _ := json.Marshal(w)
	if !bytes.Equal(gn, wn) {
		t.Errorf("%s: the encoded envelope does not match the contract fixture\n got: %s\nwant: %s", what, gn, wn)
	}
}

var fixtureDesired = map[string]json.RawMessage{
	"group":       json.RawMessage(`["lab_makers","fabrication"]`),
	"home":        json.RawMessage(`["true"]`),
	"home_quota":  json.RawMessage(`["5368709120"]`),
	"enabled":     json.RawMessage(`true`),
	"smb_enabled": json.RawMessage(`true`),
}

func TestApplyEnvelopeMatchesContract(t *testing.T) {
	// Built from the fixture's own values, so a failure is about the SHAPE. A
	// field the add-on declares and this envelope omits shows up as a missing
	// key; a field this envelope sends and the add-on's struct does not declare
	// fails on the other side, where the decode is strict.
	body, err := json.Marshal(applyEnvelope{
		ContractVersion: ContractVersion,
		CallID:          "0f8b7a1c-6d2e-4c53-9a10-2b7c9d4e5f60",
		Subject:         "289471021834760193",
		Email:           "bianca.belair@example.org",
		Fingerprint:     "sha256:9d1c4f0b7e2a",
		PlanID:          "3c9e2f81-44ab-4d0e-8f21-77aa10bb33cd",
		Actor:           "operator-289471021834760199",
		Desired:         fixtureDesired,
	})
	if err != nil {
		t.Fatalf("encode apply envelope: %v", err)
	}
	sameJSON(t, body, contractFixture(t, "apply_request.json"), "apply")
}

// The operation fixture carries preview (feature calls) and fresh (feature
// freshness), which this backend never sends: it knows neither feature yet,
// and a strict add-on that did not declare one refuses the key. Everything
// else is the fixture, byte for byte.
func TestCallEnvelopeMatchesContract(t *testing.T) {
	body, err := json.Marshal(callEnvelope{
		ContractVersion: ContractVersion,
		CallID:          "5a2d9e30-1b44-4f7c-9e08-6d3a2c1b0f95",
		Operation:       "storage.status",
		Subject:         "289471021834760193",
		Actor:           "289471021834760193",
		PlanID:          "3c9e2f81-44ab-4d0e-8f21-77aa10bb33cd",
		Fingerprint:     "sha256:9d1c4f0b7e2a",
		Params:          map[string]any{"since": "2026-11-01T00:00:00Z"},
	})
	if err != nil {
		t.Fatalf("encode call envelope: %v", err)
	}
	sameJSON(t, body, fixtureWithout(t, "operation_request.json", "preview", "fresh"), "operation")
	for _, key := range []string{`"preview"`, `"fresh"`} {
		if bytes.Contains(body, []byte(key)) {
			t.Errorf("the backend sends %s, a feature key it does not know", key)
		}
	}
}

func TestPlanEnvelopeMatchesContract(t *testing.T) {
	body, err := json.Marshal(planEnvelope{
		ContractVersion: ContractVersion,
		Subjects: []planSubjectWire{{
			Subject: "289471021834760193",
			Email:   "bianca.belair@example.org",
			Desired: fixtureDesired,
		}},
	})
	if err != nil {
		t.Fatalf("encode plan envelope: %v", err)
	}
	sameJSON(t, body, contractFixture(t, "plan_request.json"), "plan")
}

// TestContractVersionIsDeclaredOnEveryMutatingEnvelope is the guard for the
// class rather than the instance. A fourth envelope added without the field
// would reach an add-on that refuses it, and would do so only in production —
// the version is the one field whose absence is indistinguishable from an old
// caller, which is exactly what the receiver refuses on.
func TestContractVersionIsDeclaredOnEveryMutatingEnvelope(t *testing.T) {
	for _, tc := range []struct {
		name string
		v    any
	}{
		{"apply", applyEnvelope{ContractVersion: ContractVersion}},
		{"operation", callEnvelope{ContractVersion: ContractVersion}},
		{"plan", planEnvelope{ContractVersion: ContractVersion}},
	} {
		body, err := json.Marshal(tc.v)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(body, &fields); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if raw := fields["contract_version"]; string(raw) != "2" {
			t.Errorf("%s envelope sends contract_version %s, want 2", tc.name, raw)
		}
	}
}

// applyAgainst runs the real Apply against an add-on answering status and body.
func applyAgainst(t *testing.T, status int, body []byte) ApplyResponse {
	t.Helper()
	addonServingStatus(t, status, body)
	return Apply(context.Background(), ApplyRequest{Target: "truenas", Subject: "289471021834760193",
		CallID: "row-1", Fingerprint: "f", Desired: fixtureDesired})
}

// The reply leg: what the backend keeps of the add-on's apply outcome, through
// the real Apply.
func TestTheAddOnsApplyOutcomeLandsHere(t *testing.T) {
	out := applyAgainst(t, 200, contractFixture(t, "apply_response.json"))
	if out.Outcome != OutcomeSucceeded || out.Effect != "applied" || out.Detail == "" {
		t.Fatalf("the outcome did not land: %+v", out)
	}
	if out.Username != "bianca.belair" || out.AccountID != "3042" {
		t.Errorf("the account identity did not land: %q/%q", out.Username, out.AccountID)
	}
	if out.Standing == nil || out.Standing.WaitingOn != "member" || out.Standing.Usable || out.Standing.Note == "" || out.Standing.Next == "" {
		t.Errorf("standing did not land: %+v", out.Standing)
	}
	if out.Fingerprint == "" || len(out.Observed) != 5 || out.Unverified {
		t.Errorf("fingerprint and observation did not land: %q %v", out.Fingerprint, out.Observed)
	}
}

// A blocked or conflicting apply is a problem whose extension members are the
// outcome (contract 2): rejected, with the code, the detail and the account.
func TestABlockedApplyLandsAsARejectionWithItsOutcome(t *testing.T) {
	for _, tc := range []struct {
		fixture, code, username string
		status                  int
	}{
		{"error_apply_blocked.json", "APPLY_BLOCKED", "bianca.belair", 422},
		{"error_binding_conflict.json", "BINDING_CONFLICT", "", 409},
	} {
		out := applyAgainst(t, tc.status, contractFixture(t, tc.fixture))
		if out.Outcome != OutcomeRejected || out.Code != tc.code || out.Effect != effectBlocked || out.Detail == "" || out.Username != tc.username {
			t.Errorf("%s: %+v", tc.fixture, out)
		}
		if out.Fingerprint != "" || out.Observed != nil {
			t.Errorf("%s: a refusal must carry no observation: %+v", tc.fixture, out)
		}
	}
}

// And the leniency, asserted rather than assumed: a field this side does not
// declare cannot break the call; an outcome that names no effect is not one.
func TestAnOutcomeCarryingFieldsThisSideIgnoresStillDecodes(t *testing.T) {
	out := applyAgainst(t, 200, []byte(`{"subject":"sub-1","effect":"applied","detail":"Updated charlotte.",
		"username":"charlotte","account_id":"3001","standing":{"usable":true,"waiting_on":"none"},"fingerprint":"abc123",
		"observed":{"group":["lab_makers"],"enabled":true},"consequence":"ignored here","a_field_from_a_newer_addon":42}`))
	if out.Outcome != OutcomeSucceeded || out.Effect != "applied" || out.Username != "charlotte" || out.Fingerprint != "abc123" {
		t.Fatalf("the fields this side does read must survive: %+v", out)
	}
	out = applyAgainst(t, 200, []byte(`{"subject":"sub-1","detail":"Updated charlotte.","username":"charlotte"}`))
	if out.Outcome != OutcomeIndeterminate {
		t.Fatalf("a 200 naming no effect said what it did not do: %+v", out)
	}
}

// And the unverified case, whose sentence has to reach a surface: in `detail`,
// which this side reads.
func TestAnUnverifiedApplyCarriesItsSentenceWhereTheBackendReadsIt(t *testing.T) {
	out := applyAgainst(t, 200, []byte(`{"subject":"sub-1","effect":"applied","unverified":true,
		"detail":"Updated charlotte. The account could not be read back afterwards, so what the target now holds has not been confirmed.",
		"consequence":"ignored by this side"}`))
	if !out.Unverified || !bytes.Contains([]byte(out.Detail), []byte("has not been confirmed")) {
		t.Fatalf("the operator-facing sentence must be in a field this side decodes: %+v", out)
	}
}
