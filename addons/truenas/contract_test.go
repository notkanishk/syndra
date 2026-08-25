package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// The add-on half of the wire contract (see ../contract/README.md).
//
// The backend's suite asserts that what it encodes IS these documents. This one
// asserts that what this add-on decodes strictly accepts them and keeps every
// field. Strictly is the operative word: `decodeStrict` refuses unknown fields,
// so a field the backend sends and a struct here does not declare is not an
// ignored extra — it is a 400 on every call in production, invisible to a suite
// that builds its own bodies.

func contractFixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "contract", name))
	if err != nil {
		t.Fatalf("read contract fixture %s: %v", name, err)
	}
	return raw
}

// noZeroFields fails on any field the decode left empty.
//
// The point is not tidiness. A field silently dropped decodes to its zero value
// and every downstream check then passes vacuously — an empty fingerprint
// verifies against nothing, an empty actor makes the mutation log answer two
// thirds of "who did what to whom". Every fixture is fully populated so that
// "still zero after decoding" means exactly "this field did not arrive".
func noZeroFields(t *testing.T, v any, what string) {
	t.Helper()
	rv := reflect.ValueOf(v)
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		if rv.Field(i).IsZero() {
			t.Errorf("%s: field %s did not arrive from the contract fixture", what, rt.Field(i).Name)
		}
	}
}

func TestApplyRequestAcceptsContract(t *testing.T) {
	var req ApplyRequest
	if err := decodeStrict(contractFixture(t, "apply_request.json"), &req); err != nil {
		t.Fatalf("the backend's apply body was refused: %v", err)
	}
	noZeroFields(t, req, "ApplyRequest")
	if req.ContractVersion != ContractVersion {
		t.Errorf("contract version = %d, want %d", req.ContractVersion, ContractVersion)
	}
	// The one field that is a map: `IsZero` on a non-nil empty map is false, so
	// its emptiness has to be asked about directly.
	if len(req.Desired) == 0 {
		t.Error("desired state did not arrive")
	}
}

func TestOperationRequestAcceptsContract(t *testing.T) {
	var req OperationRequest
	if err := decodeStrict(contractFixture(t, "operation_request.json"), &req); err != nil {
		t.Fatalf("the backend's operation body was refused: %v", err)
	}
	noZeroFields(t, req, "OperationRequest")
	if req.Actor == "" {
		// Named separately because this is the field whose absence the mutation
		// log cannot recover from: it records who did what to whom, and the
		// add-on knows only the whom.
		t.Error("no actor arrived, so the mutation log would name only the subject")
	}
}

func TestPlanRequestAcceptsContract(t *testing.T) {
	var req PlanRequest
	if err := decodeStrict(contractFixture(t, "plan_request.json"), &req); err != nil {
		t.Fatalf("the backend's plan body was refused: %v", err)
	}
	noZeroFields(t, req, "PlanRequest")
	if len(req.Subjects) != 1 {
		t.Fatalf("subjects = %d, want 1", len(req.Subjects))
	}
	noZeroFields(t, req.Subjects[0], "PlanSubject")
}

// TestContractBodiesReachTheHandlers is the other half: accepting the shape is
// necessary and not sufficient, since a handler can refuse a well-formed body
// for its own reasons. These drive the real handlers with the real fixtures and
// assert only that the refusal is not a contract refusal — the wire, not the
// business rule.
func TestContractBodiesReachTheHandlers(t *testing.T) {
	for _, tc := range []struct {
		name    string
		path    string
		fixture string
		handle  func(s *server) func(http.ResponseWriter, *http.Request, []byte)
	}{
		{"apply", "/apply", "apply_request.json", func(s *server) func(http.ResponseWriter, *http.Request, []byte) {
			return s.handleApply
		}},
		{"plan", "/plan", "plan_request.json", func(s *server) func(http.ResponseWriter, *http.Request, []byte) {
			return s.handlePlan
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := applyServer(t, fixtureUsers)
			body := contractFixture(t, tc.fixture)
			rr := httptest.NewRecorder()
			tc.handle(s)(rr, httptest.NewRequest(http.MethodPost, tc.path, bytes.NewReader(body)), body)

			if rr.Code == http.StatusBadRequest {
				t.Errorf("the backend's %s body was refused as malformed (400); the wire shapes have diverged: %s",
					tc.name, rr.Body.String())
			}
			if rr.Code == http.StatusUnprocessableEntity {
				// 422 also carries the business-rule refusals, so this
				// distinguishes the contract one from them.
				var out map[string]any
				_ = json.Unmarshal(rr.Body.Bytes(), &out)
				if out["error"] == "CONTRACT_VERSION_MISMATCH" {
					t.Errorf("the backend's %s body declared a contract version this add-on refuses: %s",
						tc.name, rr.Body.String())
				}
			}
		})
	}
}

// The reply leg, where this add-on is the producer.
//
// The other three fixtures pin what the backend sends and this module decodes
// strictly. This one pins what this module SENDS, because the field that
// carries a merge base is on the reply and nothing held the two ends together
// on that direction at all — the safety of adding `observed` rested on "the
// backend decodes leniently", which was true and was an assumption written in a
// comment. The backend's own suite now decodes this same document.
func TestTheApplyOutcomeMatchesContract(t *testing.T) {
	// Built from the fixture's own values, so a failure is about the SHAPE.
	body, err := json.Marshal(ApplyOutcome{
		Subject:     "289471021834760193",
		Effect:      EffectApplied,
		Detail:      "Added fabrication. Unlocked the account.",
		Consequence: "In lab_makers, fabrication. Enabled, SMB on.",
		Username:    "maya.chen",
		UID:         3042,
		Fingerprint: "sha256:9d1c4f0b7e2a",
		Observed: map[string]any{
			FieldGroup:      []string{"fabrication", "lab_makers"},
			FieldEnabled:    true,
			FieldSMBEnabled: true,
		},
	})
	if err != nil {
		t.Fatalf("encode outcome: %v", err)
	}
	sameContractJSON(t, body, contractFixture(t, "apply_response.json"), "ApplyOutcome")
}

// An unverified apply is the same document with the observation removed, and
// that is a contract statement rather than an implementation detail: a consumer
// storing `observed` as a merge base must be unable to receive one from a write
// nobody read back.
func TestAnUnverifiedOutcomeCarriesNoObservation(t *testing.T) {
	body, err := json.Marshal(ApplyOutcome{
		Subject: "289471021834760193", Effect: EffectApplied, Unverified: true,
		Detail:   "Unlocked the account. The account could not be read back afterwards, so what the target now holds has not been confirmed.",
		Username: "maya.chen", UID: 3042,
	})
	if err != nil {
		t.Fatalf("encode outcome: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["unverified"] != true {
		t.Errorf("an unverified outcome must say so on the wire: %s", body)
	}
	for _, absent := range []string{"observed", "fingerprint"} {
		if _, present := decoded[absent]; present {
			t.Errorf("an unverified outcome must carry no %s: %s", absent, body)
		}
	}
}

// sameContractJSON compares two documents by value, ignoring key order and
// whitespace. Anything else would make formatting a contract change.
func sameContractJSON(t *testing.T, got, want []byte, what string) {
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
		t.Errorf("%s: the encoded document does not match the contract fixture\n got: %s\nwant: %s", what, gn, wn)
	}
}
