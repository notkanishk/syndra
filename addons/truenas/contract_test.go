package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"
)

// The add-on half of the wire contract, major 2 (see ../contract/README.md).
//
// What this add-on PRODUCES is built here as the Go value it marshals, from the
// fixture's own values, and compared with the fixture as a whole document; the
// real handlers' output is held to the schema (contract2_test.go loads it). What
// it CONSUMES is decoded strictly with every field required to land. Strictly is
// the operative word: `decodeStrict` refuses unknown fields, so a field the
// backend sends and a struct here does not declare is not an ignored extra — it
// is a 400 on every call in production, invisible to a suite that builds its
// own bodies.

func contractFixture(t *testing.T, name string) []byte {
	t.Helper()
	return v2Read(t, "fixtures", name)
}

// assertSchema validates a document against a v2 schema.
func assertSchema(t *testing.T, schema string, doc []byte, what string) {
	t.Helper()
	var v any
	if err := json.Unmarshal(doc, &v); err != nil {
		t.Fatalf("%s: not JSON: %v", what, err)
	}
	if err := v2Validate(t, schema, v); err != nil {
		t.Errorf("%s does not satisfy schema/%s:\n%v", what, schema, err)
	}
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

// withoutFeatureKeys is a request fixture less the keys a feature this add-on
// does not declare introduces (registry/features.json): the backend sends such
// a key only to an add-on that declared the feature, and this one did not.
func withoutFeatureKeys(t *testing.T, fixture, schema string) []byte {
	t.Helper()
	doc := v2JSON(t, "fixtures", fixture)
	for name, f := range v2Features(t) {
		if slices.Contains(features, name) {
			continue
		}
		for _, field := range f.RequestFields {
			if key, found := strings.CutPrefix(field, strings.TrimSuffix(schema, ".json")+"."); found {
				delete(doc, key)
			}
		}
	}
	raw, _ := json.Marshal(doc)
	return raw
}

func TestApplyRequestAcceptsContract(t *testing.T) {
	var req ApplyRequest
	if err := decodeStrict(contractFixture(t, "apply_request.json"), &req); err != nil {
		t.Fatalf("the backend's apply body was refused: %v", err)
	}
	noZeroFields(t, req, "ApplyRequest")
	if req.ContractVersion != ContractVersion || len(req.Desired) == 0 {
		t.Errorf("contract version %d, desired %v", req.ContractVersion, req.Desired)
	}
}

// The operation fixture carries preview (feature calls) and fresh (feature
// freshness). This add-on declares neither, so it refuses them with the strict
// decoder's own code, and accepts the rest with every field landing.
func TestOperationRequestAcceptsContract(t *testing.T) {
	err := decodeStrict(contractFixture(t, "operation_request.json"), &OperationRequest{})
	var unknown errUnknownRequestField
	if !errors.As(err, &unknown) {
		t.Fatalf("a feature field this add-on did not declare must be refused as an unknown field, got %v", err)
	}
	var req OperationRequest
	if err := decodeStrict(withoutFeatureKeys(t, "operation_request.json", "operation_request.json"), &req); err != nil {
		t.Fatalf("the backend's operation body was refused: %v", err)
	}
	noZeroFields(t, req, "OperationRequest")
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

func TestLifecycleRequestAcceptsContract(t *testing.T) {
	var req LifecycleRequest
	if err := decodeStrict(contractFixture(t, "lifecycle_request.json"), &req); err != nil {
		t.Fatalf("the backend's lifecycle body was refused: %v", err)
	}
	noZeroFields(t, req, "LifecycleRequest")
}

func TestSettingWriteRequestAcceptsContract(t *testing.T) {
	var req putSettingRequest
	if err := decodeStrict(contractFixture(t, "setting_put_request.json"), &req); err != nil {
		t.Fatalf("the backend's setting body was refused: %v", err)
	}
	noZeroFields(t, req, "putSettingRequest")
}

// Accepting the shape is necessary and not sufficient, since a handler can
// refuse a well-formed body for its own reasons. These drive the real handlers
// with the real fixtures and assert only that the refusal is not a contract
// refusal — the wire, not the business rule.
func TestContractBodiesReachTheHandlers(t *testing.T) {
	for _, tc := range []struct {
		name, path, fixture string
		handle              func(s *server) func(http.ResponseWriter, *http.Request, []byte)
	}{
		{"apply", "/apply", "apply_request.json", func(s *server) func(http.ResponseWriter, *http.Request, []byte) { return s.handleApply }},
		{"plan", "/plan", "plan_request.json", func(s *server) func(http.ResponseWriter, *http.Request, []byte) { return s.handlePlan }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := applyServer(t, fixtureUsers)
			body := contractFixture(t, tc.fixture)
			rr := httptest.NewRecorder()
			tc.handle(s)(rr, httptest.NewRequest(http.MethodPost, tc.path, bytes.NewReader(body)), body)
			if code := problemCode(t, rr); code == CodeBadRequest || code == CodeUnknownRequestField || code == CodeContractVersionMismatch {
				t.Errorf("the backend's %s body was refused for its shape: %d %s", tc.name, rr.Code, rr.Body.String())
			}
		})
	}
}

func ts(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func strp(s string) *string { return &s }

// notProduced lists, per fixture, the keys this add-on never sends, each with
// its reason. Anything else the fixture holds must be produced byte for byte.
var notProduced = map[string]map[string]string{
	"health.json": {
		"credential_scopes": "TrueNAS cannot read its own API key's roles; the field is optional and advisory (design §4)",
		"target_reads":      "feature freshness, not declared",
	},
}

func TestWhatTheAddOnSendsMatchesTheContract(t *testing.T) {
	member := &Standing{WaitingOn: "member", Note: "No password set yet", Next: "Set a storage password on your page"}
	placeHome := Place{Share: "homes", Folder: "rey_mysterio", Path: "/mnt/tank/homes/rey_mysterio", Access: accessChange,
		Dataset: "tank/homes", Home: true, Usage: &PlaceUsage{UsedBytes: 19756849561, QuotaBytes: 53687091200}}
	setting := SettingView{
		Name: "home_root", Label: "Where home folders go", Value: "/mnt/tank/homes", SetBy: "operator-289471021834760199",
		SetAt:  func() *time.Time { v := ts(t, "2026-11-09T14:05:00Z"); return &v }(),
		Values: []string{"/mnt/tank/homes", "/mnt/tank/members"},
		Labels: map[string]string{"/mnt/tank/homes": "tank › homes", "/mnt/tank/members": "tank › members"}, ValuesRead: true,
	}
	opOK := func(op, subject string) OperationResult {
		return OperationResult{Operation: op, Subject: subject, Outcome: "succeeded"}
	}
	activity := opOK("activity.get", "289471021834760193")
	activity.Activity = &ActivityReport{
		Events:          []ActivityEvent{{At: "2026-11-10T08:12:30Z", Event: "CONNECT", Share: "makers", Success: false, Address: "192.0.2.17", Detail: "NT_STATUS_NO_SUCH_USER"}},
		UncoveredShares: []string{"scratch"}, Truncated: true,
	}
	storage := opOK("storage.status", "289471021834760193")
	storage.Storage = &StorageStatus{
		Username: "rey_mysterio", Usable: true, NeedsPassword: true, SMBEnabled: true,
		Shares:        []ShareUsage{{Share: "makers", UsedBytes: 19756849561, QuotaBytes: 53687091200}},
		UsageReadable: true, NeedsNewPassword: true, Places: []Place{placeHome}, PlacesReadable: true, Standing: member,
	}
	health := opOK("health.get", "")
	health.Health = &TargetHealth{
		System:   &SystemInfo{Hostname: "nas.example.org", Version: "TrueNAS-25.10.5", UptimeSeconds: 864000.5},
		Alerts:   []TargetAlert{{Level: "WARNING", Class: "SMART", Text: "Disk sda reports pending sectors.", At: "2026-11-09T22:41:00Z"}},
		Pools:    []PoolStatus{{Name: "tank", Status: "ONLINE", Healthy: true, FreeBytes: 4398046511104, AllocatedBytes: 2199023255552, SizeBytes: 6597069766656}},
		Services: []ServiceState{{Service: "cifs", State: "RUNNING", Enabled: true}},
		Degraded: []string{"alert.list"},
	}
	adopt := opOK("account.adopt", "289471021834760193")
	adopt.Detail, adopt.Rotated, adopt.AccountID = "Bound the existing account rey_mysterio.", true, "3051"

	for _, tc := range []struct {
		route, fixture, schema string
		value                  any
	}{
		{"GET /capabilities", "manifest_truenas.json", "manifest.json", Manifest{
			ContractVersion: ContractVersion, Product: productName, ProductVersion: "TrueNAS-25.10.5",
			Kind: kindStorage, Features: features, EntitlementSchema: entitlementSchema(),
			Operations: []Operation{
				{ID: "password.set", Scope: "member", SecretParams: []string{"password"}, Available: true},
				{ID: "password.rotate", Scope: "admin", Available: true},
				{ID: "account.adopt", Scope: "admin", Confirm: true, Available: true},
				{ID: "storage.status", Scope: "member", Available: true},
				{ID: "account.release", Scope: "admin", Confirm: true, Available: true},
				{ID: "account.purge", Scope: "admin", Confirm: true, SecretParams: []string{"elevated_key"},
					UnavailableReason: "This TrueNAS release lacks the method the operation needs."},
				{ID: "activity.get", Scope: "admin", Available: true},
				{ID: "health.get", Scope: "admin", Available: true},
			},
			Connection: &Connection{Protocol: "smb", Host: "nas.example.org"},
			Limits:     Limits{SubjectsPerPlan: perRequestSubjectCap},
		}},
		{"GET /health", "health.json", "health.json", Health{
			Reachable: true, Product: productName, ProductVersion: "TrueNAS-25.10.5", VersionTested: true,
			VersionNote: "25.10 is a tested release.", LastReadAt: strp("2026-11-10T09:29:41Z"),
			CredentialExpiresAt: strp("2027-02-01T00:00:00Z"), CredentialExpiry: "set", UnauditedShares: []string{"scratch"},
			SharesReadable: true, Lifecycle: LifecycleReadOnly, LifecycleNote: "Maintenance window.", Drained: true,
			LogHead: "sha256:7be1c0a94d2f", LogRecords: 1284, SnapshotAge: strp("2026-11-10T09:25:00Z"),
			CheckedAt: ts(t, "2026-11-10T09:30:00Z"),
		}},
		{"GET /subjects", "subjects.json", "subjects.json", SubjectsResponse{
			Subjects: []SubjectView{
				{ID: apiID{raw: json.RawMessage(`17`)}, Username: "rey_mysterio", AccountID: "3051", Groups: []string{"fabrication", "lab_makers"},
					Home: "/mnt/tank/homes/rey_mysterio", Enabled: true, SMBEnabled: true,
					State: map[string]any{FieldGroup: []string{"fabrication", "lab_makers"}, FieldHome: []string{"true"},
						FieldHomeQuota: []string{"0", "5368709120"}, FieldEnabled: true, FieldSMBEnabled: true},
					Standing: member},
				{ID: apiID{raw: json.RawMessage(`"4"`)}, Username: "syndra_service", AccountID: "3001", Groups: []string{},
					Enabled: true, Self: true,
					State:    map[string]any{FieldGroup: []string{}, FieldHome: []string{}, FieldEnabled: true, FieldSMBEnabled: false},
					Standing: &Standing{Usable: true, WaitingOn: "none"}},
			},
			Current: true, TakenAt: "2026-11-10T09:30:00Z",
		}},
		{"GET /subjects (from the mirror)", "subjects_stale.json", "subjects.json", nil}, // driven below
		{"GET /values/{field}", "values.json", "values.json", ValuesResponse{
			Field: "group", Values: []string{"fabrication", "lab_makers"}, Enumerable: true,
			Labels:       map[string]string{"fabrication": "Fabrication", "lab_makers": "Lab makers"},
			BuiltinCount: 34, BuiltinExamples: []string{"builtin_users", "builtin_administrators"},
		}},
		{"GET /reach", "reach.json", "profiles/storage/reach.json", ReachResponse{
			TakenAt: ts(t, "2026-11-10T09:20:00Z"), Current: true,
			Groups: map[string][]Place{
				"fabrication": {{Share: "makers", Folder: "fabrication", Path: "/mnt/tank/makers/fabrication", Access: accessChange, Dataset: "tank/makers/fabrication"}},
				"lab_makers":  {placeHome},
			},
			Everyone: []Place{{Share: "makers", Path: "/mnt/tank/makers", Access: accessRead}},
		}},
		{"PUT /settings/{name} reply", "setting.json", "setting.json", setting},
		{"GET /homes", "homes.json", "profiles/storage/homes.json", HomesResponse{QuotaRead: true, TakenAt: "2026-11-10T09:30:00Z", Current: true,
			Homes: []HomeUsage{{Username: "rey_mysterio", AccountID: "3051", Home: "/mnt/tank/homes/rey_mysterio", UsedBytes: 19756849561, QuotaBytes: 53687091200}}}},
		{"POST /plan reply", "plan_response.json", "plan_response.json", PlanResponse{
			Outcomes: []ApplyOutcome{
				{Subject: "289471021834760193", Effect: EffectApply, Detail: "Would add fabrication.", Consequence: "In lab_makers, fabrication. Enabled, SMB on.",
					Username: "rey_mysterio", AccountID: "3051", Standing: member, Fingerprint: "sha256:9d1c4f0b7e2a"},
				{Subject: "289471021834760194", Effect: EffectBlocked, Detail: "An account named seth_rollins already exists and is not bound to anyone.",
					Consequence: "Nothing would be changed.",
					Conflict:    &BindingConflict{Username: "seth_rollins", AccountID: "3077", BoundTo: "289471021834760195"}},
			},
			Current: true, TakenAt: "2026-11-10T09:30:00Z",
		}},
		{"POST /apply reply", "apply_response.json", "apply_ok.json", ApplyOutcome{
			Subject: "289471021834760193", Effect: EffectApplied, Detail: "Added fabrication. Unlocked the account.",
			Consequence: "In lab_makers, fabrication. Enabled, SMB on.", Username: "bianca.belair",
			AccountID: "3042", Standing: member, Fingerprint: "sha256:9d1c4f0b7e2a",
			Observed: map[string]any{FieldGroup: []string{"fabrication", "lab_makers"}, FieldHome: []string{"true"},
				FieldHomeQuota: []string{"5368709120"}, FieldEnabled: true, FieldSMBEnabled: true},
		}},
		{"POST /operations/{name} reply", "operation_response.json", "operation_response.json", adopt},
		{"POST /operations/activity.get reply", "operation_response_activity.json", "operation_response.json", activity},
		{"POST /operations/storage.status reply", "operation_response_storage.json", "operation_response.json", storage},
		{"POST /operations/health.get reply", "operation_response_health.json", "operation_response.json", health},
	} {
		if tc.value == nil {
			continue
		}
		t.Run(tc.fixture, func(t *testing.T) {
			body, err := json.Marshal(tc.value)
			if err != nil {
				t.Fatalf("%s: encode: %v", tc.route, err)
			}
			want := v2JSON(t, "fixtures", tc.fixture)
			for key := range notProduced[tc.fixture] {
				delete(want, key)
			}
			wantRaw, _ := json.Marshal(want)
			sameContractJSON(t, body, wantRaw, tc.route+" vs fixtures/"+tc.fixture)
			assertSchema(t, tc.schema, body, tc.route+" as encoded")
		})
	}
}

// The refusals with an outcome: a blocked and a conflicting apply are problems
// whose extension members are the outcome, through the one function.
func TestABlockedApplyIsAProblemCarryingItsOutcome(t *testing.T) {
	for _, tc := range []struct {
		fixture, code string
		status        int
		out           ApplyOutcome
	}{
		{"error_apply_blocked.json", CodeApplyBlocked, http.StatusUnprocessableEntity, ApplyOutcome{
			Subject: "289471021834760193", Effect: EffectBlocked, Detail: "This TrueNAS release is not one the add-on has been tested to change.",
			Consequence: "Nothing was changed.", Username: "bianca.belair", AccountID: "3042", Fingerprint: "sha256:9d1c4f0b7e2a",
			Standing: &Standing{WaitingOn: "member", Note: "No password set yet", Next: "Set a storage password on your page"}}},
		{"error_binding_conflict.json", CodeBindingConflict, http.StatusConflict, ApplyOutcome{
			Subject: "289471021834760194", Effect: EffectBlocked, Detail: "An account named seth_rollins already exists and is not bound to anyone.",
			Consequence: "Nothing was changed.",
			Conflict:    &BindingConflict{Username: "seth_rollins", AccountID: "3077", BoundTo: "289471021834760195"}}},
	} {
		rr := httptest.NewRecorder()
		writeOutcomeRefusal(rr, tc.status, tc.code, tc.out)
		if rr.Code != tc.status || rr.Header().Get("Content-Type") != "application/problem+json" {
			t.Errorf("%s: %d %s", tc.code, rr.Code, rr.Header().Get("Content-Type"))
		}
		sameContractJSON(t, rr.Body.Bytes(), contractFixture(t, tc.fixture), tc.code+" vs fixtures/"+tc.fixture)
	}
}

// The error body, produced by the one function every contract refusal goes
// through.
func TestContractRefusalMatchesTheErrorFixture(t *testing.T) {
	rr := httptest.NewRecorder()
	if writeContractRefusal(rr, 1) {
		t.Fatal("version 1 must be refused")
	}
	sameContractJSON(t, rr.Body.Bytes(), contractFixture(t, "error.json"), "contract refusal vs fixtures/error.json")
	if rr.Code != http.StatusUnprocessableEntity || rr.Header().Get("Content-Type") != "application/problem+json" {
		t.Errorf("status %d %s, want 422 application/problem+json", rr.Code, rr.Header().Get("Content-Type"))
	}
}

// The real /subjects handler, live and from the mirror: current false carries
// stale with reason unreachable (the only mirror path this add-on has, task
// 1.9), and both validate.
func TestSubjectsAnswersCarryStaleExactlyWhenNotCurrent(t *testing.T) {
	fake := &fakeRPC{users: fixtureUsers, groups: fixtureGroups}
	s := testServer(t, fake)
	live := httptest.NewRecorder()
	s.handleSubjects(live, httptest.NewRequest(http.MethodGet, "/subjects", nil), nil)
	assertSchema(t, "subjects.json", live.Body.Bytes(), "a live /subjects")
	if strings.Contains(live.Body.String(), `"stale"`) || strings.Contains(live.Body.String(), `"uid"`) ||
		strings.Contains(live.Body.String(), `"password_set"`) || strings.Contains(live.Body.String(), `null`) {
		t.Errorf("a live read carries stale, a contract-1 key, or a null: %s", live.Body)
	}

	fake.err = errReadAfterWrite
	mirror := httptest.NewRecorder()
	s.handleSubjects(mirror, httptest.NewRequest(http.MethodGet, "/subjects", nil), nil)
	assertSchema(t, "subjects.json", mirror.Body.Bytes(), "a mirror /subjects")
	var doc struct {
		Current bool   `json:"current"`
		Stale   *Stale `json:"stale"`
	}
	if err := json.Unmarshal(mirror.Body.Bytes(), &doc); err != nil || doc.Current || doc.Stale == nil || doc.Stale.Reason != "unreachable" {
		t.Errorf("a mirror answer must be current false with stale.reason unreachable, got %s", mirror.Body)
	}
}

// The lifecycle reply is built inline by its handler, so it is produced by
// driving the handler with the fixture's own request.
func TestLifecycleRouteAnswersTheContract(t *testing.T) {
	s, _ := applyServer(t, fixtureUsers)
	request := contractFixture(t, "lifecycle_request.json")
	rr := httptest.NewRecorder()
	s.handleLifecycle(rr, httptest.NewRequest(http.MethodPost, "/lifecycle", nil), request)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST /lifecycle with fixtures/lifecycle_request.json answered %d: %s", rr.Code, rr.Body.String())
	}
	sameContractJSON(t, rr.Body.Bytes(), contractFixture(t, "lifecycle_response.json"), "POST /lifecycle vs fixtures/lifecycle_response.json")
}

// GET /settings is driven through the real handler. Its set_at is the moment of
// the write, so the body cannot be byte-equal to a fixture; what is held is the
// envelope key and every key of the setting, which is where a rename would hide.
func TestSettingsRouteAnswersTheContract(t *testing.T) {
	s := testServer(t, reachFake())
	if rr := putHomeRoot(t, s, "/mnt/tank/main/home_dir"); rr.Code != http.StatusOK {
		t.Fatalf("set: %d %s", rr.Code, rr.Body.String())
	}
	rr := httptest.NewRecorder()
	s.handleSettings(rr, httptest.NewRequest(http.MethodGet, "/settings", nil), nil)
	assertSchema(t, "settings.json", rr.Body.Bytes(), "GET /settings as served")
	keys := func(raw []byte) (outer, inner []string) {
		var doc map[string][]map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil || len(doc["settings"]) != 1 {
			t.Fatalf("GET /settings: %v %s", err, raw)
		}
		for k := range doc {
			outer = append(outer, k)
		}
		for k := range doc["settings"][0] {
			inner = append(inner, k)
		}
		sort.Strings(outer)
		sort.Strings(inner)
		return
	}
	gotOuter, gotInner := keys(rr.Body.Bytes())
	wantOuter, wantInner := keys(contractFixture(t, "settings.json"))
	if !slices.Equal(gotOuter, wantOuter) || !slices.Equal(gotInner, wantInner) {
		t.Errorf("GET /settings keys %v / %v, fixtures/settings.json has %v / %v", gotOuter, gotInner, wantOuter, wantInner)
	}
}

// An unverified apply is the same document with the observation removed, and
// that is a contract statement rather than an implementation detail: a consumer
// storing `observed` as a merge base must be unable to receive one from a write
// nobody read back.
func TestAnUnverifiedOutcomeCarriesNoObservation(t *testing.T) {
	body, err := json.Marshal(ApplyOutcome{
		Subject: "289471021834760193", Effect: EffectApplied, Unverified: true,
		Detail:   "Unlocked the account. The account could not be read back afterwards, so what the target now holds has not been confirmed.",
		Username: "bianca.belair", AccountID: "3042", Standing: &Standing{Usable: true, WaitingOn: "none"},
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
	assertSchema(t, "apply_ok.json", body, "an unverified outcome")
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
