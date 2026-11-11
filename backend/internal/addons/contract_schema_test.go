package addons

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"sync"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// The schema half of the contract (see addons/contract/README.md).
//
// Every fixture validates against its schema with format assertion ON: in JSON
// Schema 2020-12 `format` is an annotation unless the validator is told
// otherwise, so a `date-time` that is never asserted is a comment. Schemas load
// from disk by `$id`; a test run fetches nothing.

// contractDir is the one major this backend speaks (contract 2). v1 was
// deleted when slice 1 shipped (design §1: one supported major).
var contractDir = v2Dir

const schemaBase = "https://syndra.example.org/addon-contract/2/schema/"

// contractMessages pairs every fixture with the schema it must satisfy. A
// fixture on disk that is not listed fails TestEveryFixtureIsCovered.
var contractMessages = []struct{ fixture, schema string }{
	{"manifest.json", "manifest.json"},
	{"manifest_truenas.json", "manifest.json"},
	{"manifest_inventory.json", "manifest.json"},
	{"health.json", "health.json"},
	{"subjects.json", "subjects.json"},
	{"subjects_stale.json", "subjects.json"},
	{"subjects_cooldown.json", "subjects.json"},
	{"values.json", "values.json"},
	{"plan_request.json", "plan_request.json"},
	{"plan_response.json", "plan_response.json"},
	{"apply_request.json", "apply_request.json"},
	{"apply_response.json", "apply_ok.json"},
	{"operation_request.json", "operation_request.json"},
	{"operation_request_fresh.json", "operation_request.json"},
	{"operation_request_preview.json", "operation_request.json"},
	{"operation_response.json", "operation_response.json"},
	{"operation_response_result.json", "operation_response.json"},
	{"operation_response_activity.json", "operation_response.json"},
	{"operation_response_storage.json", "operation_response.json"},
	{"operation_response_health.json", "operation_response.json"},
	{"operation_response_activity_result.json", "operation_response.json"},
	{"operation_response_health_result.json", "operation_response.json"},
	{"operation_response_records.json", "operation_response.json"},
	{"operation_response_no_change.json", "operation_response.json"},
	{"status.json", "common/status.json"},
	{"records.json", "common/records.json"},
	{"activity.json", "common/activity.json"},
	{"lifecycle_request.json", "lifecycle_request.json"},
	{"lifecycle_response.json", "lifecycle_response.json"},
	{"settings.json", "settings.json"},
	{"setting.json", "setting.json"},
	{"setting_put_request.json", "setting_put_request.json"},
	{"reach.json", "profiles/storage/reach.json"},
	{"homes.json", "profiles/storage/homes.json"},
	{"error.json", "error.json"},
	{"error_apply_blocked.json", "error.json"},
	{"error_binding_conflict.json", "error.json"},
	{"error_fresh_read_cooldown.json", "error.json"},
	{"error_target_unreachable.json", "error.json"},
	{"error_call_unsettled.json", "error.json"},
	{"error_unknown_resource.json", "error.json"},
	{"error_not_entitled.json", "error.json"},
	{"error_limit_reached.json", "error.json"},
	{"error_resource_taken.json", "error.json"},
	{"call.json", "call.json"},
	{"call_in_doubt.json", "call.json"},
	{"call_unknown.json", "call.json"},
	{"stats.json", "stats.json"},
	{"identity_apply.json", "identity/vector.json"},
	{"identity_operation.json", "identity/vector.json"},
	{"identity_lifecycle.json", "identity/vector.json"},
	{"identity_setting.json", "identity/vector.json"},
}

var (
	schemaOnce     sync.Once
	schemaCompiler *jsonschema.Compiler
	schemaErr      error
)

// compiledSchema returns the schema at a path under addons/contract/schema.
func compiledSchema(t *testing.T, path string) *jsonschema.Schema {
	t.Helper()
	schemaOnce.Do(func() {
		schemaCompiler = jsonschema.NewCompiler()
		schemaCompiler.AssertFormat()
		schemaErr = filepath.WalkDir(filepath.Join(contractDir, "schema"), func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || filepath.Ext(p) != ".json" {
				return err
			}
			raw, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
			if err != nil {
				return err
			}
			id, _ := doc.(map[string]any)["$id"].(string)
			return schemaCompiler.AddResource(id, doc)
		})
	})
	if schemaErr != nil {
		t.Fatalf("load contract schemas: %v", schemaErr)
	}
	sch, err := schemaCompiler.Compile(schemaBase + path)
	if err != nil {
		t.Fatalf("compile schema %s: %v", path, err)
	}
	return sch
}

// assertSchema validates a document and names the schema and the document in
// the failure, with the validator's own path to the offending field.
func assertSchema(t *testing.T, schema string, doc []byte, what string) {
	t.Helper()
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(doc))
	if err != nil {
		t.Fatalf("%s: not JSON: %v", what, err)
	}
	if err := compiledSchema(t, schema).Validate(v); err != nil {
		t.Errorf("%s does not satisfy schema/%s:\n%v", what, schema, err)
	}
}

func schemaFor(fixture string) string {
	for _, m := range contractMessages {
		if m.fixture == fixture {
			return m.schema
		}
	}
	return ""
}

func TestEveryFixtureValidatesAgainstItsSchema(t *testing.T) {
	for _, m := range contractMessages {
		t.Run(m.fixture, func(t *testing.T) {
			assertSchema(t, m.schema, contractFixture(t, m.fixture), "fixtures/"+m.fixture)
		})
	}
}

func TestEveryFixtureIsCovered(t *testing.T) {
	listed := map[string]bool{}
	for _, m := range contractMessages {
		listed[m.fixture] = true
	}
	entries, err := os.ReadDir(filepath.Join(contractDir, "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !listed[e.Name()] {
			t.Errorf("fixtures/%s is not paired with a schema in contractMessages", e.Name())
		}
	}
}

// What this side SENDS must satisfy the schema as well as equal the fixture:
// the request schemas are closed, mirroring the add-on's strict decode.
func TestRequestsTheBackendBuildsSatisfyTheirSchemas(t *testing.T) {
	for _, tc := range []struct {
		fixture string
		value   any
	}{
		{"apply_request.json", applyEnvelope{ContractVersion: ContractVersion, CallID: "c", Subject: "s", Fingerprint: "f", Desired: map[string]json.RawMessage{}}},
		{"apply_request.json", applyEnvelope{ContractVersion: ContractVersion, CallID: "c", Subject: "s", Email: "a@example.org", Fingerprint: "f", PlanID: "p", Actor: "op",
			Desired: map[string]json.RawMessage{"group": json.RawMessage(`[]`), "enabled": json.RawMessage(`false`)}}},
		{"plan_request.json", planEnvelope{ContractVersion: ContractVersion, Subjects: []planSubjectWire{{Subject: "s", Desired: map[string]json.RawMessage{}}}}},
		{"operation_request.json", callEnvelope{ContractVersion: ContractVersion, CallID: "c", Operation: "health.get"}},
	} {
		body, err := json.Marshal(tc.value)
		if err != nil {
			t.Fatal(err)
		}
		assertSchema(t, schemaFor(tc.fixture), body, "a minimal "+tc.fixture+" as the backend encodes it")
	}
}

// The schemas must be able to say no. A schema that accepts everything would
// pass the test above for every fixture, so each rule is shown a document that
// breaks it. A nil value drops the key; a func edits the decoded document.
func TestSchemasRefuseWhatTheyShould(t *testing.T) {
	set := func(key string, value any) func(map[string]any) {
		return func(d map[string]any) {
			if value == nil {
				delete(d, key)
				return
			}
			d[key] = value
		}
	}
	for _, tc := range []struct {
		name, schema, fixture string
		break_                func(map[string]any)
	}{
		{"request with an unknown field", "apply_request.json", "apply_request.json", set("surprise", true)},
		{"wrong contract version", "plan_request.json", "plan_request.json", set("contract_version", 1)},
		{"negative log_records", "health.json", "health.json", set("log_records", -1)},
		{"timestamp that is not RFC 3339", "subjects.json", "subjects.json", set("taken_at", "yesterday")},
		{"effect outside the closed set", "apply_ok.json", "apply_response.json", set("effect", "maybe")},
		{"a 200 apply that is blocked", "apply_ok.json", "apply_response.json", set("effect", "blocked")},
		{"account_id without standing", "apply_ok.json", "apply_response.json", set("standing", nil)},
		{"an apply with the acknowledgement contract 1 had", "plan_request.json", "plan_request.json", set("acknowledge_scope", true)},
		{"apply without a fingerprint", "apply_request.json", "apply_request.json", set("fingerprint", nil)},
		{"apply with a blank fingerprint", "apply_request.json", "apply_request.json", set("fingerprint", "  ")},
		{"apply with a blank call_id", "apply_request.json", "apply_request.json", set("call_id", " ")},
		{"apply with a blank subject", "apply_request.json", "apply_request.json", set("subject", "")},
		{"operation with a blank call_id", "operation_request.json", "operation_request.json", set("call_id", " ")},
		{"lifecycle with a blank reason", "lifecycle_request.json", "lifecycle_request.json", set("reason", " ")},
		{"desired list field null", "apply_request.json", "apply_request.json", func(d map[string]any) {
			d["desired"].(map[string]any)["group"] = nil
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var doc map[string]any
			if err := json.Unmarshal(contractFixture(t, tc.fixture), &doc); err != nil {
				t.Fatal(err)
			}
			tc.break_(doc)
			broken, _ := json.Marshal(doc)
			v, _ := jsonschema.UnmarshalJSON(bytes.NewReader(broken))
			if err := compiledSchema(t, tc.schema).Validate(v); err == nil {
				t.Errorf("schema/%s accepted %s", tc.schema, broken)
			}
		})
	}
}

// ---- the operation registry ---------------------------------------------

type operationRegistry struct {
	Operations map[string]struct {
		Scope        string   `json:"scope"`
		Confirm      bool     `json:"confirm"`
		Effect       string   `json:"effect"`
		Result       string   `json:"result"`
		Pocket       string   `json:"pocket"`
		SecretResult *bool    `json:"secret_result"`
		SecretParams []string `json:"secret_params"`
		Params       []struct {
			Name     string `json:"name"`
			Type     string `json:"type"`
			Required bool   `json:"required"`
			Secret   bool   `json:"secret"`
			Format   string `json:"format"`
		} `json:"params"`
	} `json:"operations"`
}

// The registry must say what the policy says (id, scope, confirm, params,
// secrets, effect, result), and a read is never confirmed.
func TestEveryPolicyOperationIsInTheRegistry(t *testing.T) {
	checkPolicyAgainstRegistry(t, contractDir)
}

func checkPolicyAgainstRegistry(t *testing.T, dir string) {
	raw, err := os.ReadFile(filepath.Join(dir, "registry", "operations.json"))
	if err != nil {
		t.Fatal(err)
	}
	var reg operationRegistry
	if err := json.Unmarshal(raw, &reg); err != nil {
		t.Fatalf("registry/operations.json: %v", err)
	}
	for id, p := range operationPolicy {
		r, ok := reg.Operations[id]
		if !ok {
			t.Errorf("operationPolicy has %s and registry/operations.json does not list it", id)
			continue
		}
		if r.Scope != string(p.Scope) || r.Confirm != p.Confirm {
			t.Errorf("%s: registry says scope %s confirm %v, policy says %s %v", id, r.Scope, r.Confirm, p.Scope, p.Confirm)
		}
		if r.Effect != p.Effect || r.Result != p.Result || r.Pocket != p.Pocket {
			t.Errorf("%s: registry says effect %q result %q pocket %q, policy says %q %q %q", id, r.Effect, r.Result, r.Pocket, p.Effect, p.Result, p.Pocket)
		}
		// Present for every id (absence is never a value), and the policy's.
		if r.SecretResult == nil || *r.SecretResult != p.SecretResult {
			t.Errorf("%s: registry secret_result %v, policy %v", id, r.SecretResult, p.SecretResult)
		}
		if p.Effect == EffectRead && p.Confirm {
			t.Errorf("%s is a read and asks for confirmation: a read is never confirmed", id)
		}
		if p.Pocket != "" && p.Effect != EffectRead {
			t.Errorf("%s has a legacy pocket and is not a read", id)
		}
		if len(r.Params) != len(p.Params) {
			t.Errorf("%s: registry lists %d params, policy %d", id, len(r.Params), len(p.Params))
			continue
		}
		var secrets []string
		for i, ps := range p.Params {
			rp := r.Params[i]
			if rp.Name != ps.Name || rp.Type != ps.Type || rp.Required != ps.Required || rp.Secret != ps.Secret || rp.Format != ps.Format {
				t.Errorf("%s param %d: registry %+v, policy %+v", id, i, rp, ps)
			}
			if ps.Secret {
				secrets = append(secrets, ps.Name)
			}
		}
		sort.Strings(secrets)
		sort.Strings(r.SecretParams)
		if !slices.Equal(secrets, r.SecretParams) {
			t.Errorf("%s: registry secret_params %v, policy %v", id, r.SecretParams, secrets)
		}
	}
	for id := range reg.Operations {
		if _, ok := operationPolicy[id]; !ok {
			t.Errorf("registry/operations.json lists %s and operationPolicy does not", id)
		}
	}
}

// Every legacy pocket the registry names is a property of the operation
// response, the one other place it is written: a pocket the reply schema does
// not have is one no add-on was ever allowed to send.
func TestEveryRegistryPocketIsAReplyProperty(t *testing.T) {
	var reg operationRegistry
	if err := json.Unmarshal(mustRead(t, "registry", "operations.json"), &reg); err != nil {
		t.Fatal(err)
	}
	var reply struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(mustRead(t, "schema", "operation_response.json"), &reply); err != nil {
		t.Fatal(err)
	}
	pockets := 0
	for id, op := range reg.Operations {
		if op.Pocket == "" {
			continue
		}
		pockets++
		if _, ok := reply.Properties[op.Pocket]; !ok {
			t.Errorf("%s's pocket %q is not a property of schema/operation_response.json", id, op.Pocket)
		}
	}
	if pockets == 0 {
		t.Fatal("the registry names no pocket: this test reads nothing")
	}
}

// Every reply fixture that answers under `result` validates against the schema
// the registry names for its operation, so a fixture cannot drift from the
// shape its id promises. An id that declares its own (health.get) needs only
// an object, which operation_response.json already holds it to.
func TestEveryResultFixtureMatchesItsRegistrySchema(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(contractDir, "registry", "operations.json"))
	if err != nil {
		t.Fatal(err)
	}
	var reg operationRegistry
	if err := json.Unmarshal(raw, &reg); err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, m := range contractMessages {
		if m.schema != "operation_response.json" {
			continue
		}
		var doc struct {
			Operation string          `json:"operation"`
			Result    json.RawMessage `json:"result"`
		}
		if err := json.Unmarshal(contractFixture(t, m.fixture), &doc); err != nil {
			t.Fatal(err)
		}
		if doc.Result == nil {
			continue
		}
		op, ok := reg.Operations[doc.Operation]
		if !ok {
			t.Errorf("fixtures/%s answers %q, which the registry lacks", m.fixture, doc.Operation)
			continue
		}
		// A read answers what it read; a physical operation the record it
		// made, or with preview the one it would make.
		if op.Effect != EffectRead && op.Effect != EffectPhysical {
			t.Errorf("fixtures/%s: %s answers a result and is neither a read nor physical", m.fixture, doc.Operation)
		}
		if op.Result != "" {
			assertSchema(t, op.Result, doc.Result, "fixtures/"+m.fixture+" result")
			checked++
		}
	}
	if checked < 3 {
		t.Errorf("only %d result fixtures were held to their registry schema; the three common shapes each have one", checked)
	}
}
