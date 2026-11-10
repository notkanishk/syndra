package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"sort"
	"testing"
	"time"
)

// The rest of the twelve routes, held from the add-on's end (see
// ../contract/README.md). The four fixtures in contract_test.go cover the
// bodies the backend sends for apply, operations and plan; these cover every
// other message.
//
// What this add-on PRODUCES is built here as the Go value it would marshal and
// compared with the fixture as a whole document. What it CONSUMES (the two
// request bodies not covered there) is decoded strictly with every field
// required to land.

func ts(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func strp(s string) *string { return &s }

func TestWhatTheAddOnSendsMatchesTheContract(t *testing.T) {
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
		UsageReadable: true, NeedsNewPassword: true, Places: []Place{placeHome}, PlacesReadable: true,
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
	adopt.Detail, adopt.Rotated, adopt.AccountUID = "Bound the existing account rey_mysterio.", true, 3051

	for _, tc := range []struct {
		route, fixture string
		value          any
	}{
		{"GET /capabilities", "manifest.json", Manifest{
			ContractVersion: ContractVersion, Product: "truenas", ProductVersion: "TrueNAS-25.10.5",
			EntitlementSchema: []EntitlementField{
				{Name: FieldGroup, Type: "string[]", Label: "Group"},
				{Name: FieldHome, Type: "bool", Label: "Gives a home folder", Single: true},
				{Name: FieldHomeQuota, Type: "int", Label: "Home folder limit", Single: true, PerPerson: true},
				{Name: FieldEnabled, Type: "bool", Lifecycle: true},
				{Name: FieldSMBEnabled, Type: "bool", Lifecycle: true},
			},
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
		}},
		{"GET /health", "health.json", Health{
			Reachable: true, Product: "truenas", ProductVersion: "TrueNAS-25.10.5", VersionTested: true,
			VersionNote: "25.10 is a tested release.", LastReadAt: strp("2026-11-10T09:29:41Z"),
			KeyExpiresAt: strp("2027-02-01T00:00:00Z"), KeyExpiry: "set", UnauditedShares: []string{"scratch"},
			SharesReadable: true, Lifecycle: LifecycleReadOnly, LifecycleNote: "Maintenance window.", Drained: true,
			LogHead: "sha256:7be1c0a94d2f", LogRecords: 1284, SnapshotAge: strp("2026-11-10T09:25:00Z"),
			CheckedAt: ts(t, "2026-11-10T09:30:00Z"),
		}},
		{"GET /subjects", "subjects.json", SubjectsResponse{
			// Through the real subjectViews, so `state` is what entitlementState
			// emits: an account with no groups reports "group": null today.
			Subjects: subjectViews([]Subject{
				{ID: apiID{raw: json.RawMessage(`17`)}, Username: "rey_mysterio", UID: 3051, Groups: []string{"lab_makers", "fabrication"},
					Home: "/mnt/tank/homes/rey_mysterio", Enabled: true, SMBEnabled: true, PasswordSet: true},
				{ID: apiID{raw: json.RawMessage(`"4"`)}, Username: "syndra_service", UID: 3001, Groups: []string{},
					Enabled: true, PasswordSet: true, Self: true},
			}),
			Current: true, TakenAt: "2026-11-10T09:30:00Z",
		}},
		{"GET /values/{field}", "values.json", ValuesResponse{
			Field: "group", Values: []string{"fabrication", "lab_makers"}, Enumerable: true,
			Labels:       map[string]string{"fabrication": "Fabrication", "lab_makers": "Lab makers"},
			BuiltinCount: 34, BuiltinExamples: []string{"builtin_users", "builtin_administrators"},
		}},
		{"GET /reach", "reach.json", ReachResponse{
			ReadAt: ts(t, "2026-11-10T09:20:00Z"), Current: true,
			Groups: map[string][]Place{
				"fabrication": {{Share: "makers", Folder: "fabrication", Path: "/mnt/tank/makers/fabrication", Access: accessChange, Dataset: "tank/makers/fabrication"}},
				"lab_makers":  {placeHome},
			},
			Everyone: []Place{{Share: "makers", Path: "/mnt/tank/makers", Access: accessRead}},
		}},
		{"PUT /settings/{name} reply", "setting.json", setting},
		{"GET /homes", "homes.json", HomesResponse{UsageRead: true, Homes: []HomeUsage{
			{Username: "rey_mysterio", UID: 3051, Home: "/mnt/tank/homes/rey_mysterio", UsedBytes: 19756849561, QuotaBytes: 53687091200}}}},
		{"POST /plan reply", "plan_response.json", PlanResponse{
			Outcomes: []ApplyOutcome{
				{Subject: "289471021834760193", Effect: EffectApply, Detail: "Would add fabrication.", Consequence: "In lab_makers, fabrication. Enabled, SMB on.",
					Username: "rey_mysterio", UID: 3051, Fingerprint: "sha256:9d1c4f0b7e2a"},
				{Subject: "289471021834760194", Effect: EffectBlocked, Detail: "An account named seth_rollins already exists and is not bound to anyone.",
					Consequence: "Nothing would be changed.",
					Conflict:    &BindingConflict{Username: "seth_rollins", UID: 3077, BoundTo: "289471021834760195"}},
			},
			Current: true, TakenAt: "2026-11-10T09:30:00Z",
		}},
		{"POST /operations/{name} reply", "operation_response.json", adopt},
		{"POST /operations/activity.get reply", "operation_response_activity.json", activity},
		{"POST /operations/storage.status reply", "operation_response_storage.json", storage},
		{"POST /operations/health.get reply", "operation_response_health.json", health},
	} {
		t.Run(tc.fixture, func(t *testing.T) {
			body, err := json.Marshal(tc.value)
			if err != nil {
				t.Fatalf("%s: encode: %v", tc.route, err)
			}
			sameContractJSON(t, body, contractFixture(t, tc.fixture), tc.route+" vs fixtures/"+tc.fixture)
			assertSchema(t, schemaFor(tc.fixture), body, tc.route+" as encoded")
		})
	}
}

// schemaFor is the schema a fixture is paired with.
func schemaFor(fixture string) string {
	for _, m := range contractMessages {
		if m.fixture == fixture {
			return m.schema
		}
	}
	return ""
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

// The error body, produced by the one function every contract refusal goes
// through.
func TestContractRefusalMatchesTheErrorFixture(t *testing.T) {
	rr := httptest.NewRecorder()
	if writeContractRefusal(rr, 2) {
		t.Fatal("version 2 must be refused")
	}
	sameContractJSON(t, rr.Body.Bytes(), contractFixture(t, "error.json"), "contract refusal vs fixtures/error.json")
	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("status %d, want 422", rr.Code)
	}
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
