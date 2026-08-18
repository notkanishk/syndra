package addons

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The backend half of the wire contract (see addons/contract/README.md).
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
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "addons", "contract", name))
	if err != nil {
		t.Fatalf("read contract fixture %s: %v", name, err)
	}
	return raw
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
	if !bytes.Equal(canonical(t, gn), canonical(t, wn)) {
		t.Errorf("%s: the encoded envelope does not match the contract fixture\n got: %s\nwant: %s", what, gn, wn)
	}
}

// canonical re-encodes through a map so key order is Go's sorted order on both
// sides.
func canonical(t *testing.T, in []byte) []byte {
	t.Helper()
	var v any
	if err := json.Unmarshal(in, &v); err != nil {
		t.Fatalf("canonicalise: %v", err)
	}
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("canonicalise: %v", err)
	}
	return out
}

func TestApplyEnvelopeMatchesContract(t *testing.T) {
	fixture := contractFixture(t, "apply_request.json")

	// Built from the fixture's own values, so a failure is about the SHAPE. A
	// field the add-on declares and this envelope omits shows up as a missing
	// key; a field this envelope sends and the add-on's struct does not declare
	// fails on the other side, where the decode is strict.
	body, err := json.Marshal(applyEnvelope{
		ContractVersion: ContractVersion,
		CallID:          "0f8b7a1c-6d2e-4c53-9a10-2b7c9d4e5f60",
		Subject:         "289471021834760193",
		Email:           "maya.chen@example.edu",
		Fingerprint:     "sha256:9d1c4f0b7e2a",
		PlanID:          "3c9e2f81-44ab-4d0e-8f21-77aa10bb33cd",
		Actor:           "operator-289471021834760199",
		Desired: map[string]json.RawMessage{
			"group":       json.RawMessage(`["lab_makers","fabrication"]`),
			"enabled":     json.RawMessage(`true`),
			"smb_enabled": json.RawMessage(`true`),
		},
	})
	if err != nil {
		t.Fatalf("encode apply envelope: %v", err)
	}
	sameJSON(t, body, fixture, "apply")
}

func TestCallEnvelopeMatchesContract(t *testing.T) {
	fixture := contractFixture(t, "operation_request.json")

	body, err := json.Marshal(callEnvelope{
		ContractVersion: ContractVersion,
		CallID:          "5a2d9e30-1b44-4f7c-9e08-6d3a2c1b0f95",
		Operation:       "password.set",
		Subject:         "289471021834760193",
		Actor:           "289471021834760193",
		PlanID:          "3c9e2f81-44ab-4d0e-8f21-77aa10bb33cd",
		Fingerprint:     "sha256:9d1c4f0b7e2a",
		Params:          map[string]any{"password": "fixture-value-not-a-credential"},
	})
	if err != nil {
		t.Fatalf("encode call envelope: %v", err)
	}
	sameJSON(t, body, fixture, "operation")
}

func TestPlanEnvelopeMatchesContract(t *testing.T) {
	fixture := contractFixture(t, "plan_request.json")

	body, err := json.Marshal(planEnvelope{
		ContractVersion: ContractVersion,
		Subjects: []planSubjectWire{{
			Subject: "289471021834760193",
			Email:   "maya.chen@example.edu",
			Desired: map[string]json.RawMessage{
				"group":       json.RawMessage(`["lab_makers","fabrication"]`),
				"enabled":     json.RawMessage(`true`),
				"smb_enabled": json.RawMessage(`true`),
			},
		}},
		AcknowledgeScope: true,
	})
	if err != nil {
		t.Fatalf("encode plan envelope: %v", err)
	}
	sameJSON(t, body, fixture, "plan")
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
		raw, present := fields["contract_version"]
		if !present {
			t.Errorf("%s envelope carries no contract_version", tc.name)
			continue
		}
		if string(raw) == "0" {
			t.Errorf("%s envelope sends contract_version 0, which every receiver reads as a caller from before the field existed", tc.name)
		}
	}
}

// The response direction, which the artifacts do not cover and which has now
// grown a field.
//
// The add-on reports `observed` — the managed fields as the TARGET holds them
// after a write — and `unverified` when the write landed and could not be read
// back. Neither is consumed here yet; the consumer is the merge base. What must
// be true today is that they cannot BREAK anything, and that is a property of
// this decoder rather than of anybody's intentions: the request direction is
// strict on the add-on's side, and the assumption that the reply direction is
// lenient is exactly the kind of assumption this contract exists to stop being
// an assumption.
func TestAnApplyOutcomeCarryingObservedValuesStillDecodes(t *testing.T) {
	body := []byte(`{"subject":"sub-1","effect":"applied","detail":"Updated ada.",
		"username":"ada","uid":3001,"fingerprint":"abc123",
		"observed":{"group":["lab_makers"],"enabled":true},"unverified":false}`)

	var out ApplyResponse
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("the backend must tolerate fields it does not read: %v", err)
	}
	if out.Effect != "applied" || out.Username != "ada" || out.Fingerprint != "abc123" {
		t.Fatalf("the fields this side does read must survive: %+v", out)
	}
}

// And the unverified case, whose sentence has to reach a surface. It travels in
// `detail` because that is the field this struct decodes — a statement carried
// only in `consequence` would be read by nobody, since nothing on this side
// looks at that key.
func TestAnUnverifiedApplyCarriesItsSentenceWhereTheBackendReadsIt(t *testing.T) {
	body := []byte(`{"subject":"sub-1","effect":"applied","unverified":true,
		"detail":"Updated ada. The account could not be read back afterwards, so what the target now holds has not been confirmed.",
		"consequence":"ignored by this side"}`)

	var out ApplyResponse
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !bytes.Contains([]byte(out.Detail), []byte("has not been confirmed")) {
		t.Fatalf("the operator-facing sentence must be in a field this side decodes: %q", out.Detail)
	}
}
