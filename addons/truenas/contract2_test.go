package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// The schema suite of contract 2 (addons/contract/v2, written by slice 1a of
// openspec/changes/addon-contract): every schema compiles, every fixture
// validates and is covered, the schemas say no where they should, and the
// registries agree with them. contract_test.go holds this add-on's own
// messages to the same files. It loads exactly one major (v1 was deleted when
// slice 1 shipped, design §1).

var v2Dir = filepath.Join("..", "contract", "v2")

const v2Base = "https://syndra.example.org/addon-contract/2/schema/"

var (
	v2Once sync.Once
	v2Comp *jsonschema.Compiler
	v2Err  error
)

type v2m = map[string]any

func v2Read(t *testing.T, rel ...string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(append([]string{v2Dir}, rel...)...))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func v2JSON(t *testing.T, rel ...string) v2m {
	t.Helper()
	var d v2m
	if err := json.Unmarshal(v2Read(t, rel...), &d); err != nil {
		t.Fatalf("%v: %v", rel, err)
	}
	return d
}

// v2Schema compiles a schema under v2/schema, loading every schema file by its
// own $id with format assertion ON and nothing fetched.
func v2Schema(t *testing.T, path string) *jsonschema.Schema {
	t.Helper()
	v2Once.Do(func() {
		v2Comp = jsonschema.NewCompiler()
		v2Comp.AssertFormat()
		v2Err = filepath.WalkDir(filepath.Join(v2Dir, "schema"), func(p string, d fs.DirEntry, err error) error {
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
			return v2Comp.AddResource(id, doc)
		})
	})
	if v2Err != nil {
		t.Fatalf("load contract 2 schemas: %v", v2Err)
	}
	sch, err := v2Comp.Compile(v2Base + path)
	if err != nil {
		t.Fatalf("compile schema %s: %v", path, err)
	}
	return sch
}

func v2Validate(t *testing.T, schema string, doc any) error {
	t.Helper()
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	return v2Schema(t, schema).Validate(v)
}

var v2Messages = []struct{ fixture, schema string }{
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

func TestV2EverySchemaCompilesAndIsNamedByItsPath(t *testing.T) {
	n := 0
	err := filepath.WalkDir(filepath.Join(v2Dir, "schema"), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(p) != ".json" {
			return err
		}
		rel, _ := filepath.Rel(filepath.Join(v2Dir, "schema"), p)
		rel = filepath.ToSlash(rel)
		var doc v2m
		if err := json.Unmarshal(v2Read(t, "schema", rel), &doc); err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		if doc["$id"] != v2Base+rel {
			t.Errorf("schema/%s has $id %v, want %s", rel, doc["$id"], v2Base+rel)
		}
		v2Schema(t, rel)
		n++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Fatal("no schemas found")
	}
}

func TestV2EveryFixtureValidatesAgainstItsSchema(t *testing.T) {
	for _, m := range v2Messages {
		t.Run(m.fixture, func(t *testing.T) {
			if err := v2Validate(t, m.schema, v2JSON(t, "fixtures", m.fixture)); err != nil {
				t.Errorf("fixtures/%s does not satisfy schema/%s:\n%v", m.fixture, m.schema, err)
			}
		})
	}
}

func TestV2EveryFixtureIsCovered(t *testing.T) {
	listed := map[string]bool{}
	for _, m := range v2Messages {
		listed[m.fixture] = true
	}
	entries, err := os.ReadDir(filepath.Join(v2Dir, "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !listed[e.Name()] {
			t.Errorf("fixtures/%s is not paired with a schema in v2Messages", e.Name())
		}
		delete(listed, e.Name())
	}
	for f := range listed {
		t.Errorf("v2Messages lists fixtures/%s and it is not on disk", f)
	}
}

// ---- the schemas must be able to say no, and yes ------------------------

func v2At(path ...any) func(v2m) any {
	return func(d v2m) any {
		var cur any = d
		for _, p := range path {
			switch k := p.(type) {
			case string:
				cur = cur.(map[string]any)[k]
			case int:
				cur = cur.([]any)[k]
			}
		}
		return cur
	}
}

// v2Edit sets (or with nil deletes) key on the object at path.
func v2Edit(key string, val any, path ...any) func(v2m) {
	return func(d v2m) {
		o := v2At(path...)(d).(map[string]any)
		if val == nil {
			delete(o, key)
		} else {
			o[key] = val
		}
	}
}

func v2Also(fs ...func(v2m)) func(v2m) {
	return func(d v2m) {
		for _, f := range fs {
			f(d)
		}
	}
}

type v2Case struct {
	name, schema, fixture string
	edit                  func(v2m)
}

func v2Run(t *testing.T, cases []v2Case, wantErr bool) {
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := v2JSON(t, "fixtures", tc.fixture)
			tc.edit(doc)
			err := v2Validate(t, tc.schema, doc)
			if wantErr && err == nil {
				b, _ := json.Marshal(doc)
				t.Errorf("schema/%s accepted %s", tc.schema, b)
			}
			if !wantErr && err != nil {
				t.Errorf("schema/%s refused a valid document: %v", tc.schema, err)
			}
		})
	}
}

func TestV2SchemasRefuseWhatTheyShould(t *testing.T) {
	const man = "manifest.json"
	fld := func(i int) []any { return []any{"entitlement_schema", i} }
	cases := []v2Case{
		// requests are closed
		{"apply request with an unknown field", "apply_request.json", "apply_request.json", v2Edit("surprise", true)},
		{"plan request with an unknown field", "plan_request.json", "plan_request.json", v2Edit("surprise", true)},
		{"plan request still carrying acknowledge_scope", "plan_request.json", "plan_request.json", v2Edit("acknowledge_scope", true)},
		{"operation request with an unknown field", "operation_request.json", "operation_request.json", v2Edit("surprise", true)},
		{"lifecycle request with an unknown field", "lifecycle_request.json", "lifecycle_request.json", v2Edit("surprise", true)},
		{"setting request with an unknown field", "setting_put_request.json", "setting_put_request.json", v2Edit("surprise", true)},
		{"contract 1 on an apply", "apply_request.json", "apply_request.json", v2Edit("contract_version", 1)},
		{"apply without a call_id", "apply_request.json", "apply_request.json", v2Edit("call_id", nil)},
		{"non-preview operation without a call_id", "operation_request.json", "operation_request.json", v2Edit("call_id", nil)},
		{"preview carrying a call_id", "operation_request.json", "operation_request.json", v2Also(v2Edit("preview", true))},
		{"the preview fixture given a call_id", "operation_request.json", "operation_request_preview.json", v2Edit("call_id", "c-1")},
		{"an in_doubt call carrying a reply", "call.json", "call_in_doubt.json", v2Edit("reply", map[string]any{"outcome": "succeeded"})},
		{"an unknown call carrying a reply", "call.json", "call_unknown.json", v2Edit("reply", map[string]any{"outcome": "succeeded"})},
		{"a call state outside the three", "call.json", "call.json", v2Edit("state", "done")},
		{"an outcome outside the vocabulary", "operation_response.json", "operation_response_no_change.json", v2Edit("outcome", "conflict")},
		{"LIMIT_REACHED without its count", "error.json", "error_limit_reached.json", v2Edit("count", nil)},
		{"LIMIT_REACHED without its unit", "error.json", "error_limit_reached.json", v2Edit("unit", nil)},
		{"LIMIT_REACHED with a unit outside the registry", "error.json", "error_limit_reached.json", v2Edit("unit", "widgets")},
		{"UNKNOWN_RESOURCE without the param it names", "error.json", "error_unknown_resource.json", v2Edit("param", nil)},
		{"NOT_ENTITLED without the param it names", "error.json", "error_not_entitled.json", v2Edit("param", nil)},
		{"RESOURCE_TAKEN without the param it names", "error.json", "error_resource_taken.json", v2Edit("param", nil)},
		{"blocked apply problem without an effect", "error.json", "error_apply_blocked.json", v2Edit("effect", nil)},
		{"blocked apply problem whose effect is applied", "error.json", "error_apply_blocked.json", v2Edit("effect", "applied")},
		{"blocked apply problem without its subject", "error.json", "error_apply_blocked.json", v2Edit("subject", nil)},
		{"conflict problem without the conflict", "error.json", "error_binding_conflict.json", v2Edit("conflict", nil)},
		{"conflict whose account_id is a number", "error.json", "error_binding_conflict.json", v2Edit("account_id", 3077, "conflict")},
		{"homes entry still carrying uid only", "profiles/storage/homes.json", "homes.json", v2Also(v2Edit("account_id", nil, "homes", 0), v2Edit("uid", 3051, "homes", 0))},
		{"homes account_id as a number", "profiles/storage/homes.json", "homes.json", v2Edit("account_id", 3051, "homes", 0)},
		{"adopt reply account_id as a number", "operation_response.json", "operation_response.json", v2Edit("account_id", 3051)},
		{"operation with a blank call_id", "operation_request.json", "operation_request.json", v2Edit("call_id", " ")},
		{"lifecycle without a call_id", "lifecycle_request.json", "lifecycle_request.json", v2Edit("call_id", nil)},
		{"lifecycle with a blank reason", "lifecycle_request.json", "lifecycle_request.json", v2Edit("reason", " ")},
		{"setting write without a call_id", "setting_put_request.json", "setting_put_request.json", v2Edit("call_id", nil)},
		{"setting write without a contract_version", "setting_put_request.json", "setting_put_request.json", v2Edit("contract_version", nil)},
		{"apply without a fingerprint", "apply_request.json", "apply_request.json", v2Edit("fingerprint", nil)},
		{"desired list field null", "apply_request.json", "apply_request.json", func(d v2m) { d["desired"].(v2m)["group"] = nil }},
		{"desired with a bare string", "apply_request.json", "apply_request.json", func(d v2m) { d["desired"].(v2m)["group"] = "lab_makers" }},

		// the manifest
		{"field without a role", man, man, v2Edit("role", nil, fld(1)...)},
		{"field with an unknown role", man, man, v2Edit("role", "toggle", fld(1)...)},
		{"limit without a unit", man, man, v2Edit("unit", nil, fld(2)...)},
		{"limit without combine", man, man, v2Edit("combine", nil, fld(2)...)},
		{"limit without unlimited", man, man, v2Edit("unlimited", nil, fld(2)...)},
		{"misspelt unlimited is not a quota of zero", man, man, v2Also(v2Edit("unlimited", nil, fld(2)...), v2Edit("unlimted", "0", fld(2)...))},
		{"unlimited that is not a number", man, man, v2Edit("unlimited", "none", fld(2)...)},
		{"limit with a unit outside the registry", man, man, v2Edit("unit", "minutes", fld(2)...)},
		{"contract 1's type key on a field", man, man, v2Edit("type", "bool", fld(1)...)},
		{"contract 1's lifecycle key on a field", man, man, v2Edit("lifecycle", false, fld(1)...)},
		{"contract 1's single key on a field", man, man, v2Edit("single", true, fld(1)...)},
		{"unknown key on a field", man, man, v2Edit("colour", "red", fld(1)...)},
		{"unknown key on an operation", man, man, v2Edit("colour", "red", "operations", 0)},
		{"unknown key on a metric", man, man, v2Edit("colour", "red", "metrics", 0)},
		{"unknown key on a dimension", man, man, v2Edit("colour", "red", "metrics", 0, "dimensions", 0)},
		{"unknown top-level key", man, man, v2Edit("colour", "red")},
		{"unknown key in limits", man, man, v2Edit("requests_per_minute", 60, "limits")},
		{"x- key in limits, which is closed", man, man, v2Edit("x-vendor", 1, "limits")},
		{"cooldown that is not ISO 8601", man, man, v2Edit("fresh_read_cooldown", "30s", "limits")},
		{"stats buffer that is not ISO 8601", man, man, v2Edit("stats_buffer", "90 days", "limits")},
		{"subjects_per_plan of zero", man, man, v2Edit("subjects_per_plan", 0, "limits")},
		{"manifest without a kind", man, man, v2Edit("kind", nil)},
		{"kind outside the closed list", man, man, v2Edit("kind", "door")},
		{"manifest without features", man, man, v2Edit("features", nil)},
		{"duplicate features", man, man, v2Edit("features", []any{"operations", "operations"})},
		{"contract 1 manifest", man, man, v2Edit("contract_version", 1)},
		{"no account field", man, man, v2Edit("role", "channel", fld(3)...)},
		{"two account fields", man, man, v2Edit("role", "account", fld(4)...)},
		{"metric with a unit outside the registry", man, man, v2Edit("unit", "minutes", "metrics", 0)},
		{"metric that is a histogram", man, man, v2Edit("instrument", "histogram", "metrics", 0)},
		{"metric with five dimensions", man, man, v2Edit("dimensions", []any{
			v2m{"name": "a", "label": "A"}, v2m{"name": "b", "label": "B"}, v2m{"name": "c", "label": "C"}, v2m{"name": "d", "label": "D"}, v2m{"name": "e", "label": "E"}}, "metrics", 0)},
		{"metric name with no namespace", man, man, v2Edit("name", "used", "metrics", 0)},
		{"metric name in capitals", man, man, v2Edit("name", "Storage.Used", "metrics", 0)},

		// the error envelope
		{"error whose type is not a urn", "error.json", "error.json", v2Edit("type", "about:blank")},
		{"error whose status is a success", "error.json", "error.json", v2Edit("status", 200)},
		{"error without a code", "error.json", "error.json", v2Edit("code", nil)},
		{"error with contract 1's key and no code", "error.json", "error.json", v2Also(v2Edit("code", nil), v2Edit("error", "BAD_REQUEST"))},

		// freshness and standing
		{"current false without stale", "subjects.json", "subjects_stale.json", v2Edit("stale", nil)},
		{"current true with stale", "subjects.json", "subjects.json", v2Edit("stale", v2m{"reason": "cached"})},
		{"stale reason outside the set", "subjects.json", "subjects_stale.json", v2Edit("reason", "slow", "stale")},
		{"stale retry_after as text", "subjects.json", "subjects_stale.json", v2Edit("retry_after", "20s", "stale")},
		{"subjects without taken_at", "subjects.json", "subjects.json", v2Edit("taken_at", nil)},
		{"subjects with a timestamp that is not RFC 3339", "subjects.json", "subjects.json", v2Edit("taken_at", "yesterday")},
		{"subjects without truncated", "subjects.json", "subjects.json", v2Edit("truncated", nil)},
		{"subject without standing", "subjects.json", "subjects.json", v2Edit("standing", nil, "subjects", 0)},
		{"subject whose account_id is a number", "subjects.json", "subjects.json", v2Edit("account_id", 3051, "subjects", 0)},
		{"subject without account_id", "subjects.json", "subjects.json", v2Edit("account_id", nil, "subjects", 0)},
		{"standing waiting on nobody we know", "subjects.json", "subjects.json", v2Edit("waiting_on", "admin", "subjects", 0, "standing")},
		{"plan without truncated", "plan_response.json", "plan_response.json", v2Edit("truncated", nil)},
		{"plan without current", "plan_response.json", "plan_response.json", v2Edit("current", nil)},
		{"apply 200 body without a detail", "apply_ok.json", "apply_response.json", v2Edit("detail", nil)},
		{"apply 200 body with account_id and no standing", "apply_ok.json", "apply_response.json", v2Edit("standing", nil)},
		{"apply 200 body with effect blocked", "apply_ok.json", "apply_response.json", v2Edit("effect", "blocked")},
		{"apply 200 body with effect apply (a plan's word)", "apply_ok.json", "apply_response.json", v2Edit("effect", "apply")},
		{"apply 200 body carrying a conflict", "apply_ok.json", "apply_response.json", v2Edit("conflict", v2m{"username": "seth_rollins", "account_id": "3077", "adoptable": false})},
		{"blocked problem with account_id and no standing", "error.json", "error_apply_blocked.json", v2Edit("standing", nil)},
		{"blocked problem with standing and no account_id", "error.json", "error_apply_blocked.json", v2Edit("account_id", nil)},
		{"apply account_id without standing", "apply_response.json", "apply_response.json", v2Edit("standing", nil)},
		{"apply standing without account_id", "apply_response.json", "apply_response.json", v2Edit("account_id", nil)},
		{"binding conflict without account_id", "plan_response.json", "plan_response.json", v2Edit("account_id", nil, "outcomes", 1, "conflict")},
		{"reach still named read_at", "profiles/storage/reach.json", "reach.json", v2Also(v2Edit("taken_at", nil), v2Edit("read_at", "2026-11-10T09:20:00Z"))},
		{"reach not current without stale", "profiles/storage/reach.json", "reach.json", v2Edit("current", false)},
		{"homes without taken_at", "profiles/storage/homes.json", "homes.json", v2Edit("taken_at", nil)},
		{"homes still named usage_read", "profiles/storage/homes.json", "homes.json", v2Also(v2Edit("quota_read", nil), v2Edit("usage_read", true))},
		{"homes not current without stale", "profiles/storage/homes.json", "homes.json", v2Edit("current", false)},
		{"status without standing", "common/status.json", "status.json", v2Edit("standing", nil)},
		{"status meter with a negative limit", "common/status.json", "status.json", v2Edit("limit", -1, "meters", 0)},
		{"status meter with a unit outside the registry", "common/status.json", "status.json", v2Edit("unit", "minutes", "meters", 0)},
		{"records not current without stale", "common/records.json", "records.json", v2Edit("current", false)},
		{"record with an unknown key", "common/records.json", "records.json", v2Edit("colour", "red", "records", 0)},
		{"record without a label", "common/records.json", "records.json", v2Edit("label", nil, "records", 0)},
		{"activity without truncated", "common/activity.json", "activity.json", v2Edit("truncated", nil)},
		{"activity event with the share word", "common/activity.json", "activity.json", v2Edit("share", "makers", "events", 0)},
		{"health with a negative target_reads", "health.json", "health.json", v2Edit("target_reads", -1)},

		// calls and stats
		{"call in doubt carrying a reply", "call.json", "call.json", v2Edit("state", "in_doubt")},
		{"call with a state outside the set", "call.json", "call.json", v2Edit("state", "lost")},
		{"stats without a watermark", "stats.json", "stats.json", v2Edit("watermark", nil)},
		{"stats with a watermark that is not RFC 3339", "stats.json", "stats.json", v2Edit("watermark", "2026-11-08")},
		{"stats point without an end", "stats.json", "stats.json", v2Edit("end", nil, "points", 0)},
		{"stats gap without a reason", "stats.json", "stats.json", v2Edit("reason", nil, "gaps", 0)},
		{"stats without gaps (absence is not a value)", "stats.json", "stats.json", v2Edit("gaps", nil)},
		{"stats without unbound", "stats.json", "stats.json", v2Edit("unbound", nil)},
		{"stats on contract 1", "stats.json", "stats.json", v2Edit("contract_version", 1)},

		// identity
		{"apply identity carrying a fingerprint", "identity/vector.json", "identity_apply.json", v2Edit("fingerprint", FP2, "input")},
		{"apply identity carrying a plan_id", "identity/vector.json", "identity_apply.json", v2Edit("plan_id", "p", "input")},
		{"apply identity carrying an email", "identity/vector.json", "identity_apply.json", v2Edit("email", "a@example.org", "input")},
		{"operation identity carrying the password", "identity/vector.json", "identity_operation.json", v2Edit("password", "hunter2", "input", "params")},
		{"operation identity carrying the elevated key", "identity/vector.json", "identity_operation.json", v2Edit("elevated_key", "k", "input", "params")},
		{"operation identity without params", "identity/vector.json", "identity_operation.json", v2Edit("params", nil, "input")},
		{"operation identity carrying a preview", "identity/vector.json", "identity_operation.json", v2Edit("preview", true, "input")},
		{"lifecycle identity with a blank reason", "identity/vector.json", "identity_lifecycle.json", v2Edit("reason", " ", "input")},
		{"setting identity carrying the actor", "identity/vector.json", "identity_setting.json", v2Edit("actor", "a", "input")},
		{"identity vector with an upper-case digest", "identity/vector.json", "identity_setting.json", v2Edit("sha256", strings.Repeat("A", 64))},
		{"identity vector of a kind that does not exist", "identity/vector.json", "identity_setting.json", v2Edit("kind", "plan")},
	}
	v2Run(t, cases, true)
}

const FP2 = "sha256:0000"

func TestV2SchemasAcceptWhatTheyShould(t *testing.T) {
	const man = "manifest.json"
	cases := []v2Case{
		{"x- key at the top of a manifest", man, man, v2Edit("x-anything", v2m{"a": 1})},
		{"x- key on a field", man, man, v2Edit("x-anything", 1, "entitlement_schema", 1)},
		{"x- key on an operation", man, man, v2Edit("x-anything", 1, "operations", 0)},
		{"x- key on a metric", man, man, v2Edit("x-anything", 1, "metrics", 0)},
		{"x- key on a dimension", man, man, v2Edit("x-anything", 1, "metrics", 0, "dimensions", 0)},
		{"a feature nobody here has heard of is skipped, not refused", man, man, v2Edit("features", []any{"operations", "fancy.new"})},
		{"a manifest with no operations, metrics, limits or connection", man, man, v2Also(
			v2Edit("operations", nil), v2Edit("metrics", nil), v2Edit("limits", nil), v2Edit("connection", nil))},
		{"a limit field that is not per_person", man, man, v2Edit("per_person", nil, "entitlement_schema", 2)},
		{"a flag may omit its label", man, man, v2Edit("label", nil, "entitlement_schema", 1)},
		{"an unknown key in a reply is allowed", "subjects.json", "subjects.json", v2Edit("surprise", 1)},
		{"the last page of stats has a null cursor", "stats.json", "stats.json", v2Edit("next_cursor", json.RawMessage("null"))},
		{"a stats page with no points and one gap", "stats.json", "stats.json", v2Edit("points", []any{})},
		{"an apply outcome for a subject with no account has neither account_id nor standing", "apply_response.json", "apply_response.json", v2Also(v2Edit("account_id", nil), v2Edit("standing", nil))},
		{"a stale reply that says only why", "subjects.json", "subjects_stale.json", v2Also(v2Edit("retry_after", nil, "stale"), v2Edit("note", nil, "stale"))},
		{"every stale reason", "subjects.json", "subjects_stale.json", v2Edit("reason", "never_read", "stale")},
		{"standing with only the required keys", "subjects.json", "subjects.json", v2Also(v2Edit("note", nil, "subjects", 0, "standing"), v2Edit("next", nil, "subjects", 0, "standing"))},
		{"a call that is unknown carries no reply", "call.json", "call.json", v2Also(v2Edit("state", "unknown"), v2Edit("reply", nil))},
		{"a redacted recorded call carries no reply", "call.json", "call.json", v2Edit("reply", nil)},
		{"a non-current reach with its stale", "profiles/storage/reach.json", "reach.json", v2Also(v2Edit("current", false), v2Edit("stale", v2m{"reason": "unreachable", "retry_after": 60}))},
		{"a preview carries no call_id", "operation_request.json", "operation_request.json", v2Also(v2Edit("preview", true), v2Edit("call_id", nil))},
		{"a gauge point of zero", "stats.json", "stats.json", v2Edit("value", 0, "points", 1)},
	}
	v2Run(t, cases, false)
}

// ---- semantics a schema cannot express, asserted on the fixtures ---------

type v2Feature struct {
	Routes        []string `json:"routes"`
	RequestFields []string `json:"request_fields"`
	ManifestKeys  []string `json:"manifest_keys"`
	HealthFields  []string `json:"health_fields"`
	ReplyFields   []string `json:"reply_fields"`
	RequiresKind  string   `json:"requires_kind"`
}

func v2Features(t *testing.T) map[string]v2Feature {
	t.Helper()
	var f struct {
		Features map[string]v2Feature `json:"features"`
	}
	if err := json.Unmarshal(v2Read(t, "registry", "features.json"), &f); err != nil {
		t.Fatal(err)
	}
	return f.Features
}

// Every key a feature introduces is listed under it in features.json AND marked
// x-feature where it lives, in both directions, so a key cannot join a feature
// in one place and not the other.
func TestV2FeatureKeysAreListedAndMarkedBothWays(t *testing.T) {
	listed := map[string]string{} // "file:prop" or "METHOD /path[?param]" -> feature
	for name, f := range v2Features(t) {
		for _, r := range f.Routes {
			listed[r] = name
		}
		for _, r := range f.RequestFields {
			listed[r] = name // "METHOD /route?param" or "schema.property"
		}
		for _, k := range f.ManifestKeys {
			listed["manifest."+k[strings.LastIndex(k, ".")+1:]] = name
		}
		for _, k := range f.HealthFields {
			listed["health."+k] = name
		}
		for _, k := range f.ReplyFields {
			listed[k] = name
		}
	}
	marked := map[string]string{}
	var walk func(file string, v any)
	walk = func(file string, v any) {
		switch x := v.(type) {
		case map[string]any:
			// A closed word joining a vocabulary under a feature (outcome
			// no_change): a oneOf member {const, x-feature}, listed file.word.
			if c, ok := x["const"].(string); ok {
				if f, ok := x["x-feature"].(string); ok {
					marked[file+"."+c] = f
				}
			}
			if props, ok := x["properties"].(map[string]any); ok {
				for p, ps := range props {
					if sm, ok := ps.(map[string]any); ok {
						if f, ok := sm["x-feature"].(string); ok {
							marked[file+"."+p] = f
						}
					}
				}
			}
			for _, c := range x {
				walk(file, c)
			}
		case []any:
			for _, c := range x {
				walk(file, c)
			}
		}
	}
	err := filepath.WalkDir(filepath.Join(v2Dir, "schema"), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(p) != ".json" {
			return err
		}
		var doc any
		raw, _ := os.ReadFile(p)
		if err := json.Unmarshal(raw, &doc); err != nil {
			return err
		}
		walk(strings.TrimSuffix(filepath.Base(p), ".json"), doc)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// OpenAPI: routes carry x-feature; the Fresh parameter is the query field.
	pathRe := regexp.MustCompile(`^  (/\S+):$`)
	methRe := regexp.MustCompile(`^    (get|put|post):$`)
	featRe := regexp.MustCompile(`^      x-feature: (\S+)$`)
	var path, meth string
	for _, line := range strings.Split(string(v2Read(t, "openapi.yaml")), "\n") {
		switch {
		case line == "components:":
			path = ""
		case pathRe.MatchString(line):
			path = pathRe.FindStringSubmatch(line)[1]
		case methRe.MatchString(line):
			meth = strings.ToUpper(methRe.FindStringSubmatch(line)[1])
		case featRe.MatchString(line) && path != "":
			marked[meth+" "+path] = featRe.FindStringSubmatch(line)[1]
		case strings.Contains(line, "#/components/parameters/Fresh") && path != "":
			marked[meth+" "+path+"?fresh"] = "freshness"
		}
	}
	for k, f := range listed {
		if marked[k] != f {
			t.Errorf("features.json lists %q under %q; the schema or openapi marks it %q", k, f, marked[k])
		}
	}
	for k, f := range marked {
		if listed[k] != f {
			t.Errorf("%q is marked x-feature %q and features.json lists it under %q", k, f, listed[k])
		}
	}
}

func TestV2RegistriesAgreeWithTheSchemas(t *testing.T) {
	keys := func(m map[string]any) []string {
		var out []string
		for k := range m {
			out = append(out, k)
		}
		sort.Strings(out)
		return out
	}
	enumOf := func(file string, path ...any) []string {
		doc := v2JSON(t, "schema", file)
		var cur any = doc
		for _, p := range path {
			cur = cur.(map[string]any)[p.(string)]
		}
		var out []string
		for _, e := range cur.(map[string]any)["enum"].([]any) {
			out = append(out, e.(string))
		}
		sort.Strings(out)
		return out
	}
	for _, tc := range []struct {
		what string
		reg  []string
		sch  []string
	}{
		{"kinds", keys(v2JSON(t, "registry", "kinds.json")["kinds"].(map[string]any)), enumOf("manifest.json", "properties", "kind")},
		{"units", keys(v2JSON(t, "registry", "units.json")["units"].(map[string]any)), enumOf("common/field.json", "$defs", "Unit")},
		{"roles", keys(v2JSON(t, "registry", "roles.json")["roles"].(map[string]any)), enumOf("common/field.json", "$defs", "Role")},
	} {
		if !slices.Equal(tc.reg, tc.sch) {
			t.Errorf("registry %s %v and the schema's closed list %v differ", tc.what, tc.reg, tc.sch)
		}
	}
	for name, k := range v2JSON(t, "registry", "kinds.json")["kinds"].(map[string]any) {
		km := k.(map[string]any)
		if _, ok := km["scope_denylist"].([]any); !ok {
			t.Errorf("kind %s has no scope_denylist list", name)
		}
		if p, ok := km["profile"].(string); ok {
			if _, err := os.Stat(filepath.Join(v2Dir, "schema", p)); err != nil {
				t.Errorf("kind %s names profile %s: %v", name, p, err)
			}
		}
	}
	if d := v2JSON(t, "registry", "kinds.json")["kinds"].(map[string]any)["access"].(map[string]any)["scope_denylist"].([]any); len(d) != 1 || d[0] != "edit:space" {
		t.Errorf("the access kind must deny edit:space, has %v", d)
	}
}

func TestV2ConstantsAreTheDocumentedValuesAndDurationsAreISO(t *testing.T) {
	want := map[string]any{"max_reply_bytes": float64(1 << 20), "call_memory": "P30D", "in_doubt_bound": "PT1H", "series_per_metric": float64(500), "cooldown_default": "PT30S"}
	got := v2JSON(t, "registry", "constants.json")["constants"].(map[string]any)
	if len(got) != len(want) {
		t.Errorf("constants.json has %d entries, want %d", len(got), len(want))
	}
	dur := v2Schema(t, "common/field.json#/$defs/Duration")
	for k, w := range want {
		c, ok := got[k].(map[string]any)
		if !ok {
			t.Errorf("constants.json lacks %s", k)
			continue
		}
		if c["value"] != w {
			t.Errorf("%s = %v, want %v", k, c["value"], w)
		}
		if c["type"] == "duration" {
			if err := dur.Validate(c["value"]); err != nil {
				t.Errorf("%s = %v is not an ISO 8601 duration: %v", k, c["value"], err)
			}
		}
	}
	for _, bad := range []string{"30s", "1h", "30 days", "PT", "P-1D", ""} {
		if dur.Validate(bad) == nil {
			t.Errorf("the Duration schema accepted %q", bad)
		}
	}
	for _, ok := range []string{"PT30S", "P30D", "PT1H", "P90D", "P400D", "P1D"} {
		if err := dur.Validate(ok); err != nil {
			t.Errorf("the Duration schema refused %q: %v", ok, err)
		}
	}
}

func TestV2ErrorRegistryAndTheErrorFixture(t *testing.T) {
	var reg struct {
		Codes map[string]struct {
			Title    string `json:"title"`
			Statuses []int  `json:"statuses"`
			Retry    string `json:"retry"`
			Meaning  string `json:"meaning"`
		} `json:"codes"`
	}
	if err := json.Unmarshal(v2Read(t, "registry", "errors.json"), &reg); err != nil {
		t.Fatal(err)
	}
	classes := []string{"never", "backoff", "retry_after", "replan", "by_status"}
	for code, c := range reg.Codes {
		if c.Title == "" || c.Meaning == "" || len(c.Statuses) == 0 || !slices.Contains(classes, c.Retry) {
			t.Errorf("%s is incomplete: %+v", code, c)
		}
		for _, s := range c.Statuses {
			if s < 400 || s > 599 {
				t.Errorf("%s lists status %d", code, s)
			}
		}
	}
	for code, status := range map[string]int{
		"UNKNOWN_REQUEST_FIELD": 400, "CALL_IN_PROGRESS": 409, "CALL_UNSETTLED": 409, "UNKNOWN_METRIC": 404,
		"FRESH_READ_COOLDOWN": 429, "TARGET_UNREACHABLE": 503, "PLAN_TOO_LARGE": 422, "CALL_ID_REUSED": 422,
		"APPLY_BLOCKED": 422, "BINDING_CONFLICT": 409,
	} {
		c, ok := reg.Codes[code]
		if !ok || !slices.Equal(c.Statuses, []int{status}) {
			t.Errorf("%s must be registered with status %d only, has %+v", code, status, c)
		}
	}
	if _, ok := reg.Codes["SCOPE_ACKNOWLEDGEMENT_REQUIRED"]; ok {
		t.Error("SCOPE_ACKNOWLEDGEMENT_REQUIRED left the add-on in major 2 (design §11)")
	}
	// The schema's retry enum is the registry's classes less by_status, which is
	// resolved to one of the others for a given reply.
	doc := v2JSON(t, "schema", "error.json")["properties"].(map[string]any)["retry"].(map[string]any)["enum"].([]any)
	var sch []string
	for _, e := range doc {
		sch = append(sch, e.(string))
	}
	sort.Strings(sch)
	want := []string{"backoff", "never", "replan", "retry_after"}
	if !slices.Equal(sch, want) {
		t.Errorf("error.json retry enum %v, want %v", sch, want)
	}
	for _, name := range []string{"error.json", "error_apply_blocked.json", "error_binding_conflict.json"} {
		fx := v2JSON(t, "fixtures", name)
		code, _ := fx["code"].(string)
		c, ok := reg.Codes[code]
		switch {
		case !ok:
			t.Errorf(name+" carries code %q the registry lacks", code)
		default:
			if fx["type"] != "urn:syndra:addon:error:"+code {
				t.Errorf("type %v does not name code %s", fx["type"], code)
			}
			if fx["title"] != c.Title {
				t.Errorf("title %v is not the registry's %q", fx["title"], c.Title)
			}
			if !slices.Contains(c.Statuses, int(fx["status"].(float64))) {
				t.Errorf("status %v is not one of %v for %s", fx["status"], c.Statuses, code)
			}
			if c.Retry != "by_status" && fx["retry"] != c.Retry {
				t.Errorf("retry %v is not the registry's %q", fx["retry"], c.Retry)
			}
		}
	}
	// A blocked or conflicting apply is a problem and carries its outcome as extension members.
	for name, ext := range map[string][]string{"error_apply_blocked.json": {"subject", "effect", "consequence"}, "error_binding_conflict.json": {"subject", "effect", "conflict"}} {
		fx := v2JSON(t, "fixtures", name)
		for _, k := range ext {
			if _, ok := fx[k]; !ok {
				t.Errorf("%s lacks the outcome extension %q", name, k)
			}
		}
		if fx["retry"] != "never" {
			t.Errorf("%s retry %v", name, fx["retry"])
		}
	}
}

func TestV2OperationRegistry(t *testing.T) {
	var reg struct {
		Operations map[string]struct {
			Scope        string   `json:"scope"`
			Confirm      bool     `json:"confirm"`
			Effect       string   `json:"effect"`
			Result       string   `json:"result"`
			SecretResult bool     `json:"secret_result"`
			SecretParams []string `json:"secret_params"`
			Meaning      string   `json:"meaning"`
		} `json:"operations"`
	}
	if err := json.Unmarshal(v2Read(t, "registry", "operations.json"), &reg); err != nil {
		t.Fatal(err)
	}
	var secrets []string
	for id, o := range reg.Operations {
		if !slices.Contains([]string{"read", "account", "credential", "physical"}, o.Effect) || o.Meaning == "" {
			t.Errorf("%s: effect %q, meaning %q", id, o.Effect, o.Meaning)
		}
		if o.Effect == "read" && o.Confirm {
			t.Errorf("%s is a read and asks for confirmation", id)
		}
		if o.Result != "" {
			v2Schema(t, o.Result) // a result names a schema that exists
			// A read answers its result; a physical operation answers the
			// record it made (a preview, the one it would make).
			if o.Effect != "read" && o.Effect != "physical" {
				t.Errorf("%s declares a result and is neither a read nor physical", id)
			}
		}
		if o.Effect == "physical" && !o.Confirm {
			t.Errorf("%s is physical and not confirmed: a physical operation always is", id)
		}
		secrets = append(secrets, o.SecretParams...)
	}
	// identity/operation.json refuses exactly the secret params the registry names.
	props := v2JSON(t, "schema", "identity", "operation.json")["properties"].(map[string]any)["params"].(map[string]any)
	var refused []string
	for _, e := range props["propertyNames"].(map[string]any)["not"].(map[string]any)["enum"].([]any) {
		refused = append(refused, e.(string))
	}
	sort.Strings(secrets)
	secrets = slices.Compact(secrets)
	sort.Strings(refused)
	if !slices.Equal(secrets, refused) {
		t.Errorf("registry secret params %v, identity/operation.json refuses %v", secrets, refused)
	}
	// The manifest fixtures declare only registered operations, with the registry's secrecy.
	for _, f := range []string{"manifest.json", "manifest_truenas.json"} {
		for _, o := range v2JSON(t, "fixtures", f)["operations"].([]any) {
			om := o.(map[string]any)
			r, ok := reg.Operations[om["id"].(string)]
			if !ok {
				t.Errorf("%s declares %v, which the registry lacks", f, om["id"])
				continue
			}
			var got []string
			if sp, ok := om["secret_params"].([]any); ok {
				for _, s := range sp {
					got = append(got, s.(string))
				}
			}
			if !slices.Equal(got, r.SecretParams) && !(len(got) == 0 && len(r.SecretParams) == 0) {
				t.Errorf("%s: %v secret_params %v, registry %v", f, om["id"], got, r.SecretParams)
			}
		}
	}
	// A result validates against the schema the registry names for its operation.
	fx := v2JSON(t, "fixtures", "operation_response_result.json")
	r := reg.Operations[fx["operation"].(string)]
	if r.Result == "" {
		t.Fatalf("%v names no result schema", fx["operation"])
	}
	if err := v2Validate(t, r.Result, fx["result"]); err != nil {
		t.Errorf("result does not satisfy %s: %v", r.Result, err)
	}
}

func TestV2ManifestFixturesAreCoherent(t *testing.T) {
	feats := v2Features(t)
	for _, f := range []string{"manifest.json", "manifest_truenas.json"} {
		m := v2JSON(t, "fixtures", f)
		listed := map[string]bool{}
		for _, n := range m["features"].([]any) {
			name := n.(string)
			listed[name] = true
			spec, ok := feats[name]
			if !ok {
				t.Errorf("%s lists feature %q that features.json does not know", f, name)
			} else if spec.RequiresKind != "" && spec.RequiresKind != m["kind"] {
				t.Errorf("%s: feature %s needs kind %s, manifest is %v", f, name, spec.RequiresKind, m["kind"])
			}
		}
		// A manifest key a feature introduces appears only when the feature is listed.
		for name, spec := range feats {
			for _, k := range spec.ManifestKeys {
				top, sub, nested := strings.Cut(k, ".")
				var cur any = m[top]
				if nested {
					if mm, ok := m[top].(map[string]any); ok {
						cur = mm[sub]
					} else {
						cur = nil
					}
				}
				if cur != nil && !listed[name] {
					t.Errorf("%s carries %s without feature %s", f, k, name)
				}
			}
		}
		// Field names are unique; a metric's `against` names a limit of the same unit.
		units := map[string]string{}
		seen := map[string]bool{}
		for _, fl := range m["entitlement_schema"].([]any) {
			fm := fl.(map[string]any)
			n := fm["name"].(string)
			if seen[n] {
				t.Errorf("%s: field %s twice", f, n)
			}
			seen[n] = true
			if fm["role"] == "limit" {
				units[n] = fm["unit"].(string)
			}
		}
		names := map[string]bool{}
		for _, mt := range asList2(m["metrics"]) {
			mm := mt.(map[string]any)
			if names[mm["name"].(string)] {
				t.Errorf("%s: metric %v twice", f, mm["name"])
			}
			names[mm["name"].(string)] = true
			if a, ok := mm["against"].(string); ok && units[a] != mm["unit"] {
				t.Errorf("%s: metric %v is against %q (%q), unit %v", f, mm["name"], a, units[a], mm["unit"])
			}
		}
	}
}

func asList2(v any) []any {
	l, _ := v.([]any)
	return l
}

func TestV2StatsFixtureWindowsAreUTCDays(t *testing.T) {
	st := v2JSON(t, "fixtures", "stats.json")
	type win struct{ start, end time.Time }
	parse := func(o map[string]any) win {
		s, err1 := time.Parse(time.RFC3339, o["start"].(string))
		e, err2 := time.Parse(time.RFC3339, o["end"].(string))
		if err1 != nil || err2 != nil {
			t.Fatal(err1, err2)
		}
		return win{s, e}
	}
	totals := map[string]bool{}
	var subjectBuckets []string
	key := func(o map[string]any, w win) string {
		b, _ := json.Marshal(o["dimensions"]) // map keys marshal sorted
		return w.start.Format(time.RFC3339) + string(b)
	}
	for _, p := range st["points"].([]any) {
		pm := p.(map[string]any)
		w := parse(pm)
		if w.start != w.start.Truncate(24*time.Hour) || w.end.Sub(w.start) != 24*time.Hour {
			t.Errorf("point %v..%v is not one UTC day", pm["start"], pm["end"])
		}
		if _, ok := pm["subject"]; ok {
			subjectBuckets = append(subjectBuckets, key(pm, w))
		} else {
			totals[key(pm, w)] = true
		}
	}
	for _, b := range subjectBuckets {
		if !totals[b] {
			t.Errorf("bucket %s has a per-subject point and no total", b)
		}
	}
	wm, _ := time.Parse(time.RFC3339, st["watermark"].(string))
	for _, g := range st["gaps"].([]any) {
		w := parse(g.(map[string]any))
		if !w.end.After(w.start) || w.start.Before(wm) && w.end.After(wm) {
			t.Errorf("gap %v is empty or straddles the watermark", g)
		}
	}
}

// ---- RFC 8785 (JCS) ------------------------------------------------------
//
// A minimal canonicaliser, test-only, written rather than imported: identity
// objects hold strings, booleans and lists (and objects of them), so the one
// hard part of JCS, ECMAScript number formatting, is refused here instead of
// approximated. A vector with a float needs a library; none has one.

func v2JCS(v any) (string, error) {
	var b strings.Builder
	var w func(any) error
	w = func(v any) error {
		switch x := v.(type) {
		case nil:
			b.WriteString("null")
		case bool:
			fmt.Fprint(&b, x)
		case json.Number:
			n, err := strconv.ParseInt(string(x), 10, 64)
			if err != nil || n > 1<<53-1 || n < -(1<<53-1) || (n == 0 && string(x) != "0") {
				return fmt.Errorf("number %s: this test canonicaliser handles integers within +-(2^53-1) only", x)
			}
			b.WriteString(string(x))
		case string:
			b.WriteByte('"')
			for _, r := range x {
				switch {
				case r == '"':
					b.WriteString(`\"`)
				case r == '\\':
					b.WriteString(`\\`)
				case r == '\b':
					b.WriteString(`\b`)
				case r == '\t':
					b.WriteString(`\t`)
				case r == '\n':
					b.WriteString(`\n`)
				case r == '\f':
					b.WriteString(`\f`)
				case r == '\r':
					b.WriteString(`\r`)
				case r < 0x20:
					fmt.Fprintf(&b, `\u%04x`, r)
				default:
					b.WriteRune(r)
				}
			}
			b.WriteByte('"')
		case []any:
			b.WriteByte('[')
			for i, e := range x {
				if i > 0 {
					b.WriteByte(',')
				}
				if err := w(e); err != nil {
					return err
				}
			}
			b.WriteByte(']')
		case map[string]any:
			keys := make([]string, 0, len(x))
			for k := range x {
				keys = append(keys, k)
			}
			// RFC 8785 §3.2.3: by UTF-16 code units, not by bytes and not by runes.
			sort.Slice(keys, func(i, j int) bool {
				return slices.Compare(utf16.Encode([]rune(keys[i])), utf16.Encode([]rune(keys[j]))) < 0
			})
			b.WriteByte('{')
			for i, k := range keys {
				if i > 0 {
					b.WriteByte(',')
				}
				if err := w(k); err != nil {
					return err
				}
				b.WriteByte(':')
				if err := w(x[k]); err != nil {
					return err
				}
			}
			b.WriteByte('}')
		default:
			return fmt.Errorf("unsupported %T", v)
		}
		return nil
	}
	err := w(v)
	return b.String(), err
}

func v2Decode(t *testing.T, raw []byte) any {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

// RFC 8785 §3.2.3's own example: the keys must sort by UTF-16 code unit, which
// puts U+D83D (the emoji) before U+FB33, the opposite of a sort by code point.
func TestV2JCSAgainstTheRFCKeyOrderingExample(t *testing.T) {
	in := `{"\u20ac":"Euro Sign","\r":"Carriage Return","\ufb33":"Hebrew Letter Dalet With Dagesh","1":"One","\ud83d\ude00":"Emoji: Grinning Face","\u0080":"Control","\u00f6":"Latin Small Letter O With Diaeresis"}`
	want := "{\"\\r\":\"Carriage Return\",\"1\":\"One\",\"\u0080\":\"Control\",\"\u00f6\":\"Latin Small Letter O With Diaeresis\",\"\u20ac\":\"Euro Sign\",\"\U0001F600\":\"Emoji: Grinning Face\",\"\ufb33\":\"Hebrew Letter Dalet With Dagesh\"}"
	got, err := v2JCS(v2Decode(t, []byte(in)))
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("JCS\n got  %q\n want %q", got, want)
	}
	for _, bad := range []string{`{"n":1.5}`, `{"n":1e3}`, `{"n":9007199254740992}`, `{"n":-9007199254740992}`, `{"n":99999999999999999999}`, `{"n":-0}`} {
		if _, err := v2JCS(v2Decode(t, []byte(bad))); err == nil {
			t.Errorf("the test canonicaliser accepted %s rather than refusing it", bad)
		}
	}
	for _, ok := range []string{`{"n":9007199254740991}`, `{"n":-9007199254740991}`, `{"n":0}`} {
		if _, err := v2JCS(v2Decode(t, []byte(ok))); err != nil {
			t.Errorf("refused %s: %v", ok, err)
		}
	}
}

// v2TopLevelKeysSorted reads an object's keys in source order, which a decode
// into a map would lose, and says whether they are already UTF-16 sorted.
func v2TopLevelKeysSorted(t *testing.T, raw []byte) bool {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(raw))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		t.Fatalf("not an object: %v", err)
	}
	var keys []string
	for dec.More() {
		k, err := dec.Token()
		if err != nil {
			t.Fatal(err)
		}
		keys = append(keys, k.(string))
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			t.Fatal(err)
		}
	}
	return slices.IsSortedFunc(keys, func(a, b string) int { return slices.Compare(utf16.Encode([]rune(a)), utf16.Encode([]rune(b))) })
}

func v2Section(t *testing.T, raw []byte, key string) []byte {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m[key]
}

func TestV2IdentityVectorsReproduce(t *testing.T) {
	for _, kind := range []string{"apply", "operation", "lifecycle", "setting"} {
		t.Run(kind, func(t *testing.T) {
			raw := v2Read(t, "fixtures", "identity_"+kind+".json")
			var vec struct {
				Kind      string          `json:"kind"`
				Input     json.RawMessage `json:"input"`
				Canonical string          `json:"canonical"`
				SHA256    string          `json:"sha256"`
			}
			if err := json.Unmarshal(raw, &vec); err != nil {
				t.Fatal(err)
			}
			if vec.Kind != kind {
				t.Fatalf("kind %q", vec.Kind)
			}
			got, err := v2JCS(v2Decode(t, vec.Input))
			if err != nil {
				t.Fatal(err)
			}
			if got != vec.Canonical {
				t.Errorf("canonical bytes\n got  %q\n want %q", got, vec.Canonical)
			}
			sum := sha256.Sum256([]byte(vec.Canonical))
			if hex.EncodeToString(sum[:]) != vec.SHA256 {
				t.Errorf("sha256 %x, vector says %s", sum, vec.SHA256)
			}
			// The vector's input is not already sorted, or it proves nothing about sorting.
			if kind == "apply" && v2TopLevelKeysSorted(t, vec.Input) {
				t.Error("the apply vector's top-level keys are already in UTF-16 order, so it proves nothing about sorting")
			}
			if kind == "apply" && v2TopLevelKeysSorted(t, v2Section(t, vec.Input, "desired")) {
				t.Error("the apply vector's desired keys are already in UTF-16 order")
			}
		})
	}
}

// ---- the raw query joins the MAC (decision 24) ----------------------------

func v2Sign(unix, method, target string, body, key []byte) string {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(unix + "." + method + "." + target + "."))
	m.Write(body)
	return hex.EncodeToString(m.Sum(nil))
}

func TestV2RawQuerySignatureVectorsReproduce(t *testing.T) {
	var d struct {
		Derivations []struct {
			Target     string `json:"target"`
			HMACKeyHex string `json:"hmac_key_hex"`
		} `json:"derivations"`
		Signature struct {
			MACHex string `json:"mac_hex"`
		} `json:"signature"`
		Raw struct {
			Major   int `json:"introduced_by_major"`
			Vectors []struct {
				Name          string `json:"name"`
				Method        string `json:"method"`
				PathAsSent    string `json:"path_as_sent"`
				RawQuery      string `json:"raw_query"`
				Target        string `json:"target"`
				Body          string `json:"body_utf8"`
				Unix          string `json:"unix"`
				TargetName    string `json:"target_name"`
				KeyHex        string `json:"hmac_key_hex"`
				MACHex        string `json:"mac_hex"`
				DecodedMACHex string `json:"decoded_path_mac_hex"`
			} `json:"vectors"`
		} `json:"raw_query_signatures"`
	}
	raw, err := os.ReadFile(filepath.Join(v2Dir, "..", "transport_derivation.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		t.Fatal(err)
	}
	if d.Raw.Major != 2 {
		t.Errorf("raw_query_signatures introduced_by_major = %d, want 2", d.Raw.Major)
	}
	if len(d.Raw.Vectors) != 4 {
		t.Fatalf("%d vectors, design §4 asks for four", len(d.Raw.Vectors))
	}
	derived := map[string]string{}
	for _, x := range d.Derivations {
		derived[x.Target] = x.HMACKeyHex
	}
	seen := map[string]bool{}
	for _, v := range d.Raw.Vectors {
		t.Run(v.Name, func(t *testing.T) {
			seen[v.Name] = true
			want := v.PathAsSent
			if v.RawQuery != "" {
				want += "?" + v.RawQuery
			}
			if v.Target != want {
				t.Fatalf("target %q, want path as sent + ? + raw query = %q", v.Target, want)
			}
			if v.KeyHex != derived[v.TargetName] {
				t.Fatalf("key is not the one derived for target %q", v.TargetName)
			}
			key, _ := hex.DecodeString(v.KeyHex)
			if got := v2Sign(v.Unix, v.Method, v.Target, []byte(v.Body), key); got != v.MACHex {
				t.Errorf("MAC %s, vector says %s", got, v.MACHex)
			}
			if got := v2ProductionMAC(v.Unix, v.Method, v.Target, []byte(v.Body), key); got != v.MACHex {
				t.Errorf("the shipped signing function, given the target as its path, says %s; vector %s", got, v.MACHex)
			}
			// A query that is reordered or altered is a different request.
			if v.RawQuery != "" {
				for _, alt := range []string{v.RawQuery + "&x=1", reverseQuery(v.RawQuery)} {
					if alt != v.RawQuery && v2Sign(v.Unix, v.Method, v.PathAsSent+"?"+alt, []byte(v.Body), key) == v.MACHex {
						t.Errorf("query %q verifies as %q", alt, v.RawQuery)
					}
				}
			}
			switch v.Name {
			case "no_query":
				if v.MACHex != d.Signature.MACHex {
					t.Errorf("a query-less MAC must equal major 1's %s, got %s", d.Signature.MACHex, v.MACHex)
				}
			case "escaped_path":
				dec, err := url.PathUnescape(v.PathAsSent)
				if err != nil || dec == v.PathAsSent {
					t.Fatalf("the path does not need decoding: %q", v.PathAsSent)
				}
				decoded := v2Sign(v.Unix, v.Method, dec+"?"+v.RawQuery, []byte(v.Body), key)
				if decoded != v.DecodedMACHex || decoded == v.MACHex {
					t.Errorf("decoded-path MAC %s (vector %s, escaped %s): signing the decoded path must differ", decoded, v.DecodedMACHex, v.MACHex)
				}
			}
		})
	}
	for _, n := range []string{"no_query", "base64url_cursor", "reserved_and_repeated", "escaped_path"} {
		if !seen[n] {
			t.Errorf("vector %s is missing", n)
		}
	}
}

func reverseQuery(q string) string {
	parts := strings.Split(q, "&")
	slices.Reverse(parts)
	return strings.Join(parts, "&")
}

// v2ProductionMAC signs with the add-on's own function, given the target as its
// path argument: both ends take the string to sign, so the major-2 change is in
// what the caller passes (slice 1), not in this function.
func v2ProductionMAC(unix, method, target string, body, key []byte) string {
	var u int64
	fmt.Sscan(unix, &u)
	return hex.EncodeToString(computeMAC(u, method, target, body, key))
}

// ---- freshness (6.2) -------------------------------------------------------

// fresh rides on the reads in scope and on nothing else (design §4): never
// /plan, /apply or /stats. The listing is exact; TestV2FeatureKeysAreListed-
// AndMarkedBothWays holds openapi's Fresh parameter to it, and the closed
// request schemas refuse the key outright.
func TestV2FreshIsNeverOnPlanApplyStats(t *testing.T) {
	want := []string{"GET /homes?fresh", "GET /reach?fresh", "GET /subjects?fresh", "GET /values/{field}?fresh", "operation_request.fresh"}
	got := slices.Sorted(slices.Values(v2Features(t)["freshness"].RequestFields))
	if !slices.Equal(got, want) {
		t.Errorf("freshness request fields %v, want exactly %v", got, want)
	}
	for _, f := range got {
		for _, never := range []string{"/plan", "/apply", "/stats", "plan_request", "apply_request"} {
			if strings.Contains(f, never) {
				t.Errorf("fresh is listed on %s", f)
			}
		}
	}
	for _, schema := range []string{"plan_request.json", "apply_request.json"} {
		doc := v2JSON(t, "fixtures", schema)
		doc["fresh"] = true
		if v2Validate(t, schema, doc) == nil {
			t.Errorf("schema/%s accepts fresh", schema)
		}
	}
}

// current == false ⇔ stale present, failed both ways on every reply whose
// schema holds the invariant, against a fixture that passes untouched.
func TestV2StaleInvariantFailsBothWays(t *testing.T) {
	var holders []string
	err := filepath.WalkDir(filepath.Join(v2Dir, "schema"), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		raw, _ := os.ReadFile(p)
		rel, _ := filepath.Rel(filepath.Join(v2Dir, "schema"), p)
		if rel != filepath.Join("common", "stale.json") && bytes.Contains(raw, []byte("stale.json#/$defs/Invariant")) {
			holders = append(holders, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil || len(holders) == 0 {
		t.Fatalf("no schema holds the invariant (%v)", err)
	}
	for _, schema := range holders {
		fixtures := 0
		for _, m := range v2Messages {
			if m.schema != schema {
				continue
			}
			fixtures++
			doc := v2JSON(t, "fixtures", m.fixture)
			if err := v2Validate(t, schema, doc); err != nil {
				t.Fatalf("fixtures/%s does not pass untouched: %v", m.fixture, err)
			}
			notCurrent := v2JSON(t, "fixtures", m.fixture)
			notCurrent["current"] = false
			delete(notCurrent, "stale")
			stale := v2JSON(t, "fixtures", m.fixture)
			stale["current"] = true
			stale["stale"] = map[string]any{"reason": "cached"}
			for name, bad := range map[string]v2m{"current false without stale": notCurrent, "current true with stale": stale} {
				if v2Validate(t, schema, bad) == nil {
					t.Errorf("schema/%s accepts %s (fixtures/%s)", schema, name, m.fixture)
				}
			}
		}
		if fixtures == 0 {
			t.Errorf("schema/%s holds the invariant and no fixture exercises it", schema)
		}
	}
}

// The fixtures the freshness half added are what each end produces and reads.
func TestV2FreshOperationRequestIsAccepted(t *testing.T) {
	var req OperationRequest
	if err := decodeStrict(contractFixture(t, "operation_request_fresh.json"), &req); err != nil || !req.Fresh {
		t.Fatalf("a read asking for a fresh read was refused or lost it: %v %+v", err, req)
	}
}
