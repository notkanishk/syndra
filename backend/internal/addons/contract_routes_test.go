package addons

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// The rest of the twelve routes, held from the backend's end (see
// addons/contract/README.md). Reads go through the real client functions
// against a server that answers with the fixture, so what is asserted is what
// this package keeps of the add-on's reply, field by named field. Writes whose
// body is built inline (lifecycle, setting, the three read operations) are
// captured as they leave the real function and held to the fixture or schema.

type seen struct {
	method, path, idempotencyKey string
	body                         []byte
}

// addonAnswering stands an add-on up that answers every request with the named
// fixture and reports what the backend sent.
func addonAnswering(t *testing.T, fixture string) *seen {
	t.Helper()
	return addonServing(t, contractFixture(t, fixture))
}

// addonServing is addonAnswering for a body the test has edited.
func addonServing(t *testing.T, body []byte) *seen {
	t.Helper()
	got := &seen{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.method, got.path, got.idempotencyKey = r.Method, r.URL.Path, r.Header.Get("Idempotency-Key")
		got.body, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	signedAddon(t, srv.URL, "a-test-secret")
	return got
}

func ts(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestManifestIsFetchedStrictlyAndEveryFieldLands(t *testing.T) {
	// The real fetch, strict decode and all, against an add-on serving the fixture.
	got := addonAnswering(t, "manifest.json")
	a, err := Get("truenas")
	if err != nil {
		t.Fatal(err)
	}
	m, err := httpFetchManifest(context.Background(), a.Registration)
	if err != nil {
		t.Fatalf("GET /capabilities fixtures/manifest.json was refused: %v", err)
	}
	if got.method != http.MethodGet || got.path != "/capabilities" {
		t.Errorf("sent %s %s", got.method, got.path)
	}
	if m.ContractVersion != ContractVersion || m.Product != "truenas" || m.ProductVersion != "TrueNAS-25.10.5" {
		t.Errorf("identity did not land: %+v", m)
	}
	if m.Connection == nil || m.Connection.Protocol != "smb" || m.Connection.Host != "nas.example.org" {
		t.Errorf("connection did not land: %+v", m.Connection)
	}
	home, ok := m.EntitlementField("home_quota")
	if !ok || home.Type != "int" || home.Lifecycle || home.Label == "" || !home.Single || !home.PerPerson {
		t.Errorf("an entitlement field did not land whole: %+v", home)
	}
	if f, _ := m.EntitlementField("enabled"); !f.Lifecycle {
		t.Error("the lifecycle flag did not land")
	}
	purge := map[string]Operation{}
	for _, o := range m.Operations {
		purge[o.ID] = o
	}
	if p := purge["account.purge"]; p.Scope != ScopeAdmin || !p.Confirm || p.Available || p.UnavailableReason == "" || len(p.SecretParams) != 1 {
		t.Errorf("an operation did not land whole: %+v", p)
	}
	if ops, unknown := resolveOperations(m); len(unknown) != 0 || len(ops) != len(m.Operations) {
		t.Errorf("the manifest names operations the policy does not know: %v", unknown)
	}
}

func TestHealthReplyLands(t *testing.T) {
	got := addonAnswering(t, "health.json")
	h := readHealth(context.Background(), "truenas")
	if got.method != http.MethodGet || got.path != "/health" {
		t.Fatalf("sent %s %s", got.method, got.path)
	}
	if h.Outcome != OutcomeSucceeded || !h.Reachable || h.Product != "truenas" || h.ProductVersion != "TrueNAS-25.10.5" || !h.VersionTested || h.VersionNote == "" {
		t.Errorf("health identity did not land: %+v", h)
	}
	if h.Lifecycle != "read_only" || h.LifecycleNote == "" || h.InFlight != 0 || !h.Drained || h.CircuitOpen {
		t.Errorf("lifecycle did not land: %+v", h)
	}
	if h.LogHead != "sha256:7be1c0a94d2f" || h.LogRecords != 1284 {
		t.Errorf("the anchored log head did not land: %q/%d", h.LogHead, h.LogRecords)
	}
	if h.SnapshotTakenAt == "" || h.LastReadAt == "" || h.KeyExpiresAt == "" || h.KeyExpiry != "set" {
		t.Errorf("health dates did not land: %+v", h)
	}
	if len(h.UnauditedShares) != 1 || !h.SharesReadable {
		t.Errorf("audit coverage did not land: %+v", h)
	}
}

func TestSubjectsReplyLands(t *testing.T) {
	got := addonAnswering(t, "subjects.json")
	r := Subjects(context.Background(), "truenas")
	if got.method != http.MethodGet || got.path != "/subjects" {
		t.Fatalf("sent %s %s", got.method, got.path)
	}
	if !r.Usable() || !r.TakenAt.Equal(ts(t, "2026-11-10T09:30:00Z")) || len(r.Accounts) != 2 {
		t.Fatalf("the read did not land usable: %+v", r)
	}
	a := r.Accounts[0]
	if a.Username != "rey_mysterio" || a.UID != 3051 || !a.PasswordSet || a.Self {
		t.Errorf("account did not land: %+v", a)
	}
	var groups []string
	if err := json.Unmarshal(a.State["group"], &groups); err != nil || len(groups) != 2 {
		t.Errorf("state.group did not land: %v %v", groups, err)
	}
	if len(a.State) != 4 {
		t.Errorf("state carries %d fields, want 4", len(a.State))
	}
	if !r.Accounts[1].Self {
		t.Error("the add-on's own account is not marked self")
	}
}

func TestValuesReplyLands(t *testing.T) {
	got := addonAnswering(t, "values.json")
	v := ReadValues(context.Background(), "truenas", "group")
	if got.method != http.MethodGet || got.path != "/values/group" {
		t.Fatalf("sent %s %s", got.method, got.path)
	}
	if v.Outcome != OutcomeSucceeded || !v.Enumerable || len(v.Values) != 2 || v.Labels["lab_makers"] != "Lab makers" ||
		v.BuiltinCount != 34 || len(v.BuiltinExamples) != 2 {
		t.Errorf("values did not land: %+v", v)
	}
	if res, err := ResolvesValue(context.Background(), "truenas", "group", "lab_makers"); err != nil || !res.Checked {
		t.Errorf("a listed value must resolve: %+v %v", res, err)
	}
	if _, err := ResolvesValue(context.Background(), "truenas", "group", "no_such_group"); err == nil {
		t.Error("an unlisted value must be refused")
	}
}

func TestReachReplyLands(t *testing.T) {
	got := addonAnswering(t, "reach.json")
	r := ReadReach(context.Background(), "truenas")
	if got.method != http.MethodGet || got.path != "/reach" {
		t.Fatalf("sent %s %s", got.method, got.path)
	}
	if !r.Read || !r.Current || !r.ReadAt.Equal(ts(t, "2026-11-10T09:20:00Z")) {
		t.Fatalf("reach header did not land: %+v", r)
	}
	p := r.Groups["fabrication"]
	if len(p) != 1 || p[0].Share != "makers" || p[0].Folder != "fabrication" || p[0].Path == "" || p[0].Access != "change" || p[0].Dataset == "" {
		t.Errorf("a group's place did not land: %+v", p)
	}
	h := r.Groups["lab_makers"]
	if len(h) != 1 || !h[0].Home || h[0].Usage == nil || h[0].Usage.UsedBytes != 19756849561 || h[0].Usage.QuotaBytes != 53687091200 {
		t.Errorf("a home place's usage did not land: %+v", h)
	}
	if len(r.Everyone) != 1 || r.Everyone[0].Access != "read" {
		t.Errorf("everyone did not land: %+v", r.Everyone)
	}
}

func TestSettingsAndHomesRepliesLand(t *testing.T) {
	got := addonAnswering(t, "settings.json")
	s, ok := ReadSettings(context.Background(), "truenas")
	if !ok || got.method != http.MethodGet || got.path != "/settings" || len(s) != 1 {
		t.Fatalf("settings: ok %v, sent %s %s, %d", ok, got.method, got.path, len(s))
	}
	if s[0].Name != "home_root" || s[0].Label == "" || s[0].Value != "/mnt/tank/homes" || s[0].SetBy == "" ||
		s[0].SetAt == nil || !s[0].SetAt.Equal(ts(t, "2026-11-09T14:05:00Z")) || len(s[0].Values) != 2 ||
		s[0].Labels["/mnt/tank/homes"] == "" || !s[0].ValuesRead {
		t.Errorf("a setting did not land: %+v", s[0])
	}

	got = addonAnswering(t, "homes.json")
	h := ReadHomes(context.Background(), "truenas")
	if got.method != http.MethodGet || got.path != "/homes" || !h.Read || !h.UsageRead || len(h.Homes) != 1 {
		t.Fatalf("homes: sent %s %s, %+v", got.method, got.path, h)
	}
	if u := h.Homes[0]; u.Username != "rey_mysterio" || u.UID != 3051 || u.Home == "" || u.UsedBytes != 19756849561 || u.QuotaBytes != 53687091200 {
		t.Errorf("a home did not land: %+v", u)
	}
}

// PUT /settings/{name}: the body leaves as the fixture, and the key that makes
// a replay safe travels in the header.
func TestSettingWriteSendsTheContractBody(t *testing.T) {
	got := addonAnswering(t, "setting.json")
	stubOperationRecord(t, nil)
	resp, ok := PutSetting(context.Background(), "truenas", "home_root", "/mnt/tank/homes", "operator-289471021834760199")
	if !ok || resp.Outcome != OutcomeSucceeded {
		t.Fatalf("resp %+v ok %v", resp, ok)
	}
	if got.method != http.MethodPut || got.path != "/settings/home_root" {
		t.Errorf("sent %s %s", got.method, got.path)
	}
	sameJSON(t, got.body, contractFixture(t, "setting_put_request.json"), "PUT /settings/home_root vs fixtures/setting_put_request.json")
	assertSchema(t, "setting_put_request.json", got.body, "PUT /settings/home_root as sent")
	if got.idempotencyKey != "rec-put-1" {
		t.Errorf("Idempotency-Key = %q, want the operation record's id", got.idempotencyKey)
	}
	var reply TargetSetting
	if err := json.Unmarshal(resp.Body, &reply); err != nil || reply.Value != "/mnt/tank/homes" || reply.Name != "home_root" {
		t.Errorf("the setting reply did not decode: %+v %v", reply, err)
	}
}

func TestPlanReplyLands(t *testing.T) {
	addonAnswering(t, "plan_response.json")
	r := Plan(context.Background(), "truenas", []PlanSubject{{Subject: "289471021834760193", Email: "rey_mysterio@example.org"}}, false)
	if r.Outcome != OutcomeSucceeded || !r.Current || r.Truncated || !r.TakenAt.Equal(ts(t, "2026-11-10T09:30:00Z")) || len(r.Outcomes) != 2 {
		t.Fatalf("the plan did not land: %+v", r)
	}
	o := r.Outcomes[0]
	if o.Subject != "289471021834760193" || o.Effect != "apply" || o.Detail == "" || o.Consequence == "" ||
		o.Username != "rey_mysterio" || o.Fingerprint != "sha256:9d1c4f0b7e2a" || o.Conflict != nil {
		t.Errorf("an outcome did not land: %+v", o)
	}
	c := r.Outcomes[1].Conflict
	if r.Outcomes[1].Effect != effectBlocked || c == nil || c.Username != "seth_rollins" || c.UID != 3077 || c.Adoptable || c.BoundTo != "289471021834760195" {
		t.Errorf("a binding conflict did not land: %+v", r.Outcomes[1])
	}
}

func TestLifecycleSendsTheContractBodyAndReadsTheReply(t *testing.T) {
	got := addonAnswering(t, "lifecycle_response.json")
	stubOperationRecord(t, nil)
	out, err := SetLifecycle(context.Background(), "truenas", "read_only", "Moving the pool to new disks.", "operator-289471021834760199")
	if err != nil {
		t.Fatal(err)
	}
	if got.method != http.MethodPost || got.path != "/lifecycle" {
		t.Errorf("sent %s %s", got.method, got.path)
	}
	sameJSON(t, got.body, contractFixture(t, "lifecycle_request.json"), "POST /lifecycle vs fixtures/lifecycle_request.json")
	assertSchema(t, "lifecycle_request.json", got.body, "POST /lifecycle as sent")
	if out["state"] != "read_only" || out["reason"] != "Moving the pool to new disks." || out["drained"] != true || out["in_flight"] != float64(0) {
		t.Errorf("the lifecycle reply did not land: %v", out)
	}
}

// The three reads that ride POST /operations/{name} build their bodies inline.
// Each leaves as a document the operation schema accepts, and the reply's
// pocket lands.
func TestReadOperationsSendTheContractShapeAndReadTheirPocket(t *testing.T) {
	ctx := context.Background()

	got := addonAnswering(t, "operation_response_storage.json")
	st := MyStorage(ctx, "truenas", "289471021834760193", "289471021834760193")
	assertOperationRequest(t, got, "storage.status")
	if st.Outcome != OutcomeSucceeded || st.Username != "rey_mysterio" || !st.Usable || !st.NeedsPassword || !st.SMBEnabled ||
		!st.UsageReadable || !st.NeedsNewPassword || !st.PlacesReadable {
		t.Errorf("storage status did not land: %+v", st)
	}
	if len(st.Shares) != 1 || st.Shares[0].Share != "makers" || st.Shares[0].UsedBytes != 19756849561 || st.Shares[0].QuotaBytes != 53687091200 {
		t.Errorf("storage shares did not land: %+v", st.Shares)
	}
	if len(st.Places) != 1 || !st.Places[0].Home || st.Places[0].Usage == nil || st.Places[0].Usage.UsedBytes != 19756849561 {
		t.Errorf("storage places did not land: %+v", st.Places)
	}

	got = addonAnswering(t, "operation_response_activity.json")
	act := Activity(ctx, "truenas", "289471021834760193", "2026-11-01T00:00:00Z")
	assertOperationRequest(t, got, "activity.get")
	if act.Outcome != OutcomeSucceeded || !act.Truncated || len(act.UncoveredShares) != 1 || len(act.Events) != 1 {
		t.Fatalf("the activity report did not land: %+v", act)
	}
	if e := act.Events[0]; e.At != "2026-11-10T08:12:30Z" || e.Event != "CONNECT" || e.Share != "makers" || e.Success || e.Address != "192.0.2.17" || e.Detail != "NT_STATUS_NO_SUCH_USER" {
		t.Errorf("an event did not land: %+v", e)
	}

	got = addonAnswering(t, "operation_response_health.json")
	sys := SystemHealth(ctx, "truenas")
	assertOperationRequest(t, got, "health.get")
	if sys.Outcome != OutcomeSucceeded || sys.System == nil || sys.System.Hostname != "nas.example.org" || sys.System.Version == "" || sys.System.UptimeSeconds != 864000.5 {
		t.Fatalf("the system block did not land: %+v", sys)
	}
	if len(sys.Alerts) != 1 || sys.Alerts[0].Level != "WARNING" || sys.Alerts[0].Class != "SMART" || sys.Alerts[0].Text == "" || sys.Alerts[0].At == "" || sys.Alerts[0].Dismissed {
		t.Errorf("an alert did not land: %+v", sys.Alerts)
	}
	if len(sys.Pools) != 1 || sys.Pools[0].Name != "tank" || !sys.Pools[0].Healthy || sys.Pools[0].Warning || sys.Pools[0].FreeBytes == 0 || sys.Pools[0].AllocatedBytes == 0 || sys.Pools[0].SizeBytes == 0 || sys.Pools[0].Status != "ONLINE" {
		t.Errorf("a pool did not land: %+v", sys.Pools)
	}
	if len(sys.Services) != 1 || sys.Services[0].Service != "cifs" || sys.Services[0].State != "RUNNING" || !sys.Services[0].Enabled || len(sys.Degraded) != 1 {
		t.Errorf("services did not land: %+v %v", sys.Services, sys.Degraded)
	}
}

func assertOperationRequest(t *testing.T, got *seen, operation string) {
	t.Helper()
	if got.method != http.MethodPost || got.path != "/operations/"+operation {
		t.Errorf("sent %s %s, want POST /operations/%s", got.method, got.path, operation)
	}
	assertSchema(t, "operation_request.json", got.body, "POST /operations/"+operation+" as sent")
	var env struct {
		Operation string `json:"operation"`
	}
	if err := json.Unmarshal(got.body, &env); err != nil || env.Operation != operation {
		t.Errorf("the body names operation %q, the path %q", env.Operation, operation)
	}
}

// The error body: what this side keeps of a refusal is its code.
func TestErrorBodyLands(t *testing.T) {
	var out ApplyResponse
	if err := json.Unmarshal(contractFixture(t, "error.json"), &out); err != nil {
		t.Fatal(err)
	}
	if out.Code != "CONTRACT_VERSION_MISMATCH" {
		t.Errorf("the error code did not land: %q", out.Code)
	}
}

// The manifest is the one reply decoded strictly, and that is a behaviour the
// schema's closed manifest mirrors: a field this side does not know is refused.
func TestManifestWithAnUnknownFieldIsRefused(t *testing.T) {
	var doc map[string]any
	if err := json.Unmarshal(contractFixture(t, "manifest.json"), &doc); err != nil {
		t.Fatal(err)
	}
	doc["surprise"] = true
	body, _ := json.Marshal(doc)
	addonServing(t, body)
	a, err := Get("truenas")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := httpFetchManifest(context.Background(), a.Registration); err == nil {
		t.Fatal("a manifest carrying an unknown field was accepted")
	}
}
