package addons

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"syndra/internal/db"
)

// The rest of the routes, held from the backend's end (see
// addons/contract/README.md). Reads go through the real client functions
// against a server that answers with the fixture, so what is asserted is what
// this package keeps of the add-on's reply, field by named field. Writes whose
// body is built inline (lifecycle, setting, the three read operations) are
// captured as they leave the real function and held to the fixture or schema.

type seen struct {
	method, path, rawQuery, idempotencyKey string
	body                                   []byte
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
	return addonServingStatus(t, http.StatusOK, body)
}

func addonServingStatus(t *testing.T, status int, body []byte) *seen {
	t.Helper()
	got := &seen{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.method, got.path, got.rawQuery, got.idempotencyKey = r.Method, r.URL.Path, r.URL.RawQuery, r.Header.Get("Idempotency-Key")
		got.body, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		if status >= 400 {
			w.Header().Set("Content-Type", "application/problem+json")
		}
		w.WriteHeader(status)
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

func fetchFixtureManifest(t *testing.T, body []byte) (Manifest, *seen) {
	t.Helper()
	got := addonServing(t, body)
	a, err := Get("truenas")
	if err != nil {
		t.Fatal(err)
	}
	m, err := httpFetchManifest(context.Background(), a.Registration)
	if err != nil {
		t.Fatalf("GET /capabilities was refused: %v", err)
	}
	return m, got
}

func TestTheTrueNASManifestIsFetchedAndEveryFieldLands(t *testing.T) {
	m, got := fetchFixtureManifest(t, contractFixture(t, "manifest_truenas.json"))
	if got.method != http.MethodGet || got.path != "/capabilities" {
		t.Errorf("sent %s %s", got.method, got.path)
	}
	if m.ContractVersion != ContractVersion || m.Product != "TrueNAS" || m.ProductVersion != "TrueNAS-25.10.5" || m.Kind != "storage" {
		t.Errorf("identity did not land: %+v", m)
	}
	if !m.Declares("storage.homes") || !m.Declares("operations") || m.Declares("calls") || m.Limits.SubjectsPerPlan != 200 {
		t.Errorf("features and limits did not land: %v %+v", m.Features, m.Limits)
	}
	if m.Connection == nil || m.Connection.Protocol != "smb" || m.Connection.Host != "nas.example.org" {
		t.Errorf("connection did not land: %+v", m.Connection)
	}
	quota, ok := m.EntitlementField("home_quota")
	if !ok || quota.Role != RoleLimit || quota.Unit != "bytes" || quota.Combine != "max" || quota.Unlimited != "0" ||
		quota.Label == "" || !quota.PerPerson || !quota.IsLimit() || !quota.IsSingle() || quota.IsLifecycle() {
		t.Errorf("a limit field did not land whole: %+v", quota)
	}
	if roles := m.FieldsByRole(); len(roles[RoleAccount]) != 1 || roles[RoleAccount][0] != "enabled" || roles[RoleChannel][0] != "smb_enabled" {
		t.Errorf("roles did not land: %v", roles)
	}
	if err := m.validate(); err != nil || len(m.Unknown) != 0 {
		t.Errorf("the TrueNAS manifest must validate whole: %v %v", err, m.Unknown)
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

// The manifest is decoded TOLERANTLY (contract 2, design §2): a key this
// backend does not know — top level, inside a field, an operation, the limits,
// or vendor data — is kept aside and shown, never refused.
func TestAManifestCarryingMoreIsAcceptedAndWhatItAlsoSaysIsKept(t *testing.T) {
	m, _ := fetchFixtureManifest(t, contractFixture(t, "manifest.json"))
	if err := m.validate(); err != nil {
		t.Fatalf("the generic manifest must validate: %v", err)
	}
	if got := strings.Join(m.Unknown, "\n"); !strings.Contains(got, `"entitlement_schema[0].x-truenas-pool": "tank"`) {
		t.Errorf("a vendor key inside a field was not kept: %s", got)
	}
	var doc map[string]any
	_ = json.Unmarshal(contractFixture(t, "manifest_truenas.json"), &doc)
	doc["surprise"] = strings.Repeat("<b>", 200)
	doc["entitlement_schema"].([]any)[0].(map[string]any)["unlimted"] = "0"
	doc["limits"].(map[string]any)["a_later_limit"] = 3
	body, _ := json.Marshal(doc)
	m, _ = fetchFixtureManifest(t, body)
	if err := m.validate(); err != nil {
		t.Fatalf("a manifest carrying more must still validate: %v", err)
	}
	got := strings.Join(m.Unknown, "\n")
	for _, want := range []string{`"entitlement_schema[0].unlimted": "0"`, `"limits.a_later_limit": 3`, "\"surprise\": \"\\u003cb\\u003e"} {
		if !strings.Contains(got, want) {
			t.Errorf("unknown keys %q lack %s", got, want)
		}
	}
	for _, line := range m.Unknown {
		if len(line) > alsoSaysCap+50 {
			t.Errorf("a shown value is not capped: %d bytes", len(line))
		}
	}
}

func TestHealthReplyLands(t *testing.T) {
	got := addonAnswering(t, "health.json")
	h := readHealth(context.Background(), "truenas")
	if got.method != http.MethodGet || got.path != "/health" {
		t.Fatalf("sent %s %s", got.method, got.path)
	}
	if h.Outcome != OutcomeSucceeded || !h.Reachable || h.Product != "TrueNAS" || h.ProductVersion != "TrueNAS-25.10.5" || !h.VersionTested || h.VersionNote == "" {
		t.Errorf("health identity did not land: %+v", h)
	}
	if h.Lifecycle != "read_only" || h.LifecycleNote == "" || h.InFlight != 0 || !h.Drained || h.CircuitOpen {
		t.Errorf("lifecycle did not land: %+v", h)
	}
	if h.LogHead != "sha256:7be1c0a94d2f" || h.LogRecords != 1284 {
		t.Errorf("the anchored log head did not land: %q/%d", h.LogHead, h.LogRecords)
	}
	// Contract 2's credential_expiry lands in the view's key_expiry.
	if h.SnapshotTakenAt == "" || h.LastReadAt == "" || h.KeyExpiresAt != "2027-02-01T00:00:00Z" || h.KeyExpiry != "set" {
		t.Errorf("health dates did not land: %+v", h)
	}
	if len(h.UnauditedShares) != 1 || !h.SharesReadable {
		t.Errorf("audit coverage did not land: %+v", h)
	}
}

func TestSubjectsReplyLands(t *testing.T) {
	got := addonAnswering(t, "subjects.json")
	r := Subjects(context.Background(), "truenas", false)
	if got.method != http.MethodGet || got.path != "/subjects" {
		t.Fatalf("sent %s %s", got.method, got.path)
	}
	if !r.Usable() || !r.TakenAt.Equal(ts(t, "2026-11-10T09:30:00Z")) || len(r.Accounts) != 2 {
		t.Fatalf("the read did not land usable: %+v", r)
	}
	a := r.Accounts[0]
	if a.Username != "rey_mysterio" || a.AccountID != "3051" || a.Self || a.Standing == nil || a.Standing.WaitingOn != "member" || a.Standing.Next == "" {
		t.Errorf("account did not land: %+v", a)
	}
	var groups []string
	if err := json.Unmarshal(a.State["group"], &groups); err != nil || len(groups) != 2 {
		t.Errorf("state.group did not land: %v %v", groups, err)
	}
	if len(a.State) != 5 {
		t.Errorf("state carries %d fields, want 5", len(a.State))
	}
	if !r.Accounts[1].Self || r.Accounts[1].Standing == nil || !r.Accounts[1].Standing.Usable {
		t.Error("the add-on's own account is not marked self and usable")
	}

	addonAnswering(t, "subjects_stale.json")
	if r := Subjects(context.Background(), "truenas", false); r.Outcome != OutcomeSucceeded || r.Current || r.Usable() {
		t.Errorf("a mirror answer must land not current: %+v", r)
	}
}

// Absence is never a value (design §2(b)): each field the §4 table lists for a
// reply, removed, makes that read one nothing may be concluded from — never Go's
// zero value, which would read a forgotten `truncated` as "complete".
func TestAnAbsentReplyFieldMakesTheReadUnusable(t *testing.T) {
	edit := func(fixture string, change func(map[string]any)) []byte {
		var doc map[string]any
		if err := json.Unmarshal(contractFixture(t, fixture), &doc); err != nil {
			t.Fatal(err)
		}
		change(doc)
		out, _ := json.Marshal(doc)
		return out
	}
	drop := func(key string) func(map[string]any) { return func(d map[string]any) { delete(d, key) } }
	for _, change := range []func(map[string]any){drop("taken_at"), func(d map[string]any) { d["taken_at"] = "yesterday" }} {
		addonServing(t, edit("subjects.json", change))
		if r := Subjects(context.Background(), "truenas", false); r.Usable() || !r.TakenAt.IsZero() {
			t.Errorf("subjects with no readable taken_at is usable or dated: %+v", r)
		}
		addonServing(t, edit("plan_response.json", change))
		if r := Plan(context.Background(), "truenas", []PlanSubject{{Subject: "289471021834760193"}}); r.Outcome == OutcomeSucceeded {
			t.Errorf("a current plan with no readable taken_at was issued: %+v", r)
		}
	}
	for _, key := range []string{"current", "truncated"} {
		addonServing(t, edit("subjects.json", drop(key)))
		if r := Subjects(context.Background(), "truenas", false); r.Usable() {
			t.Errorf("subjects without %s is usable: %+v", key, r)
		}
	}
	addonServing(t, edit("subjects.json", func(d map[string]any) {
		delete(d["subjects"].([]any)[0].(map[string]any)["standing"].(map[string]any), "usable")
	}))
	if r := Subjects(context.Background(), "truenas", false); r.Usable() {
		t.Errorf("subjects with an account lacking standing.usable is usable: %+v", r)
	}

	plan := func() PlanResult {
		return Plan(context.Background(), "truenas", []PlanSubject{{Subject: "289471021834760193"}})
	}
	for _, key := range []string{"current", "truncated"} {
		addonServing(t, edit("plan_response.json", drop(key)))
		if r := plan(); r.Outcome == OutcomeSucceeded && r.Current && !r.Truncated {
			t.Errorf("a plan without %s is current and complete: %+v", key, r)
		}
	}
	addonServing(t, edit("plan_response.json", func(d map[string]any) { delete(d, "current"); delete(d, "taken_at") }))
	if r := plan(); r.Outcome == OutcomeSucceeded {
		t.Errorf("a plan that is neither current nor dated was issued: %+v", r)
	}
	addonServing(t, edit("plan_response.json", func(d map[string]any) {
		delete(d["outcomes"].([]any)[0].(map[string]any), "effect")
	}))
	if r := plan(); r.Outcome == OutcomeSucceeded {
		t.Errorf("a plan with an outcome naming no effect was issued: %+v", r)
	}

	for _, key := range []string{"taken_at", "current"} {
		addonServing(t, edit("reach.json", drop(key)))
		if r := ReadReach(context.Background(), "truenas", false); r.Read && r.Current {
			t.Errorf("reach without %s reads as current: %+v", key, r)
		}
		addonServing(t, edit("homes.json", drop(key)))
		if h := ReadHomes(context.Background(), "truenas", false); h.Read {
			t.Errorf("homes without %s reads as a reading: %+v", key, h)
		}
	}
	addonServing(t, edit("homes.json", drop("quota_read")))
	if h := ReadHomes(context.Background(), "truenas", false); h.UsageRead {
		t.Errorf("homes without quota_read reads as complete: %+v", h)
	}
}

// Task 1.0: a field the backend sets for itself is never the add-on's to set.
// Contract 1 decoded /homes and /reach into the very types the UI receives, so
// an add-on could send "read": true; the views are now built from wire types.
func TestAnAddOnCannotSetTheBackendsOwnReadFlags(t *testing.T) {
	addonServing(t, []byte(`{"read":true,"usage_read":true,"homes":[],"quota_read":false,"taken_at":"2026-11-10T09:30:00Z","current":true}`))
	if h := ReadHomes(context.Background(), "truenas", false); h.UsageRead {
		t.Errorf("the add-on set usage_read through the view's own key: %+v", h)
	}
	addonServingStatus(t, http.StatusServiceUnavailable, []byte(`{"read":true,"read_at":"2026-11-10T09:30:00Z","current":true}`))
	if r := ReadReach(context.Background(), "truenas", false); r.Read {
		t.Errorf("a refusal claiming read reads as read: %+v", r)
	}
	addonServing(t, []byte(`{"read":true,"read_at":"2026-11-10T09:30:00Z","current":true}`))
	if r := ReadReach(context.Background(), "truenas", false); r.Read {
		t.Errorf("a reply with contract 1's read_at and the view's read reads as read: %+v", r)
	}
	// A place's addresses and roles are Syndra's to fill, from the connection
	// host and the member's own roles: an add-on sending them sets nothing.
	planted := `{"share":"makers","path":"/mnt/tank/makers","access":"read","smb_address":"smb://evil.example.org/x","unc_address":"\\\\evil\\x","roles":[{"project_id":"p","role_key":"r"}]}`
	addonServing(t, []byte(`{"taken_at":"2026-11-10T09:30:00Z","current":true,"groups":{"g":[`+planted+`]},"everyone":[`+planted+`]}`))
	r := ReadReach(context.Background(), "truenas", false)
	for _, p := range append(r.Groups["g"], r.Everyone...) {
		if p.SMBAddress != "" || p.UNCAddress != "" || len(p.Roles) != 0 || p.Share != "makers" {
			t.Errorf("a place in /reach carries what the add-on planted: %+v", p)
		}
	}
	addonServing(t, []byte(`{"operation":"storage.status","subject":"s","outcome":"succeeded","storage":{"username":"rey_mysterio","places":[`+planted+`]}}`))
	st := Read(context.Background(), "truenas", "storage.status", "s", "s", nil, false).Storage()
	if len(st.Places) != 1 || st.Places[0].SMBAddress != "" || st.Places[0].UNCAddress != "" || len(st.Places[0].Roles) != 0 {
		t.Errorf("a storage place carries what the add-on planted: %+v", st.Places)
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

// A membership the add-on cannot list is refused at the mapping; a limit it
// cannot list is not (structure only).
func TestResolvesValueRefusesUnlistableMembership(t *testing.T) {
	forgetRoles(t)
	memRoles.Lock()
	memRoles.byTarget["truenas"] = db.FieldRoleMap{Roles: map[string]string{"group": RoleMembership, "quota": RoleLimit}}
	memRoles.Unlock()
	addonServing(t, []byte(`{"field":"group","enumerable":false,"values":[]}`))
	_, err := ResolvesValue(context.Background(), "truenas", "group", "Hulk Hogan")
	if !errors.Is(err, ErrFieldNotListable) || !errors.Is(err, ErrValueNotResolvable) || !strings.Contains(err.Error(), "can't list the values of group") {
		t.Errorf("a non-enumerable membership must be refused, got %v", err)
	}
	addonServing(t, []byte(`{"field":"quota","enumerable":false,"values":[]}`))
	if res, err := ResolvesValue(context.Background(), "truenas", "quota", "1"); err != nil || res.Checked {
		t.Errorf("a non-enumerable limit stays structure-only: %+v %v", res, err)
	}
}

func TestReachReplyLands(t *testing.T) {
	got := addonAnswering(t, "reach.json")
	r := ReadReach(context.Background(), "truenas", false)
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
	h := ReadHomes(context.Background(), "truenas", false)
	if got.method != http.MethodGet || got.path != "/homes" || !h.Read || !h.UsageRead || len(h.Homes) != 1 {
		t.Fatalf("homes: sent %s %s, %+v", got.method, got.path, h)
	}
	if u := h.Homes[0]; u.Username != "rey_mysterio" || u.AccountID != "3051" || u.Home == "" || u.UsedBytes != 19756849561 || u.QuotaBytes != 53687091200 {
		t.Errorf("a home did not land: %+v", u)
	}
}

// PUT /settings/{name}: the body leaves as the fixture, the call_id in it the
// operation record's id; no Idempotency-Key header any more (contract 2).
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
	assertCallID(t, got, "rec-put-1")
	sameJSON(t, withoutKey(t, got.body, "call_id"), fixtureWithout(t, "setting_put_request.json", "call_id"), "PUT /settings/home_root vs fixtures/setting_put_request.json")
	assertSchema(t, "setting_put_request.json", got.body, "PUT /settings/home_root as sent")
	var reply TargetSetting
	if err := json.Unmarshal(resp.Body, &reply); err != nil || reply.Value != "/mnt/tank/homes" || reply.Name != "home_root" {
		t.Errorf("the setting reply did not decode: %+v %v", reply, err)
	}
}

func assertCallID(t *testing.T, got *seen, want string) {
	t.Helper()
	var body struct {
		CallID string `json:"call_id"`
	}
	if err := json.Unmarshal(got.body, &body); err != nil || body.CallID != want {
		t.Errorf("the body's call_id is %q, want %q", body.CallID, want)
	}
	if got.idempotencyKey != "" {
		t.Errorf("an Idempotency-Key header was sent (%q); contract 2 carries the call_id in the body", got.idempotencyKey)
	}
}

func withoutKey(t *testing.T, raw []byte, key string) []byte {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	delete(doc, key)
	out, _ := json.Marshal(doc)
	return out
}

func TestPlanReplyLands(t *testing.T) {
	got := addonAnswering(t, "plan_response.json")
	r := Plan(context.Background(), "truenas", []PlanSubject{{Subject: "289471021834760193", Email: "rey_mysterio@example.org",
		Desired: map[string]json.RawMessage{"enabled": json.RawMessage(`true`)}}})
	if r.Outcome != OutcomeSucceeded || !r.Current || r.Truncated || !r.TakenAt.Equal(ts(t, "2026-11-10T09:30:00Z")) || len(r.Outcomes) != 2 {
		t.Fatalf("the plan did not land: %+v", r)
	}
	assertSchema(t, "plan_request.json", got.body, "POST /plan as sent")
	o := r.Outcomes[0]
	if o.Subject != "289471021834760193" || o.Effect != "apply" || o.Detail == "" || o.Consequence == "" ||
		o.Username != "rey_mysterio" || o.AccountID != "3051" || o.Standing == nil || o.Fingerprint != "sha256:9d1c4f0b7e2a" || o.Conflict != nil {
		t.Errorf("an outcome did not land: %+v", o)
	}
	c := r.Outcomes[1].Conflict
	if r.Outcomes[1].Effect != effectBlocked || c == nil || c.Username != "seth_rollins" || c.AccountID != "3077" || c.Adoptable || c.BoundTo != "289471021834760195" {
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
	assertCallID(t, got, "rec-put-1")
	sameJSON(t, withoutKey(t, got.body, "call_id"), fixtureWithout(t, "lifecycle_request.json", "call_id"), "POST /lifecycle vs fixtures/lifecycle_request.json")
	assertSchema(t, "lifecycle_request.json", got.body, "POST /lifecycle as sent")
	if out["state"] != "read_only" || out["reason"] != "Moving the pool to new disks." || out["drained"] != true || out["in_flight"] != float64(0) {
		t.Errorf("the lifecycle reply did not land: %v", out)
	}
}

// The three reads that ride POST /operations/{name} build their bodies inline.
// Each leaves as a document the operation schema accepts, and what lands is
// the same from the legacy pocket (an add-on without results) and from
// `result` (one declaring it). The backend reads both shapes, so it may deploy
// first or together with the add-on; never the add-on alone, whose `result` a
// pre-6a backend does not read (deploy.yml ships both from one commit). The
// operator reads name the person as subject and the operator as actor.
func TestReadOperationsSendTheContractShapeAndReadTheirAnswer(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		results                  bool
		storage, activity, other string
	}{
		{"the legacy pockets", false, "operation_response_storage.json", "operation_response_activity.json", "operation_response_health.json"},
		{"results", true, "operation_response_result.json", "operation_response_activity_result.json", "operation_response_health_result.json"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			serve := func(fixture string) *seen {
				got := addonAnswering(t, fixture)
				if tc.results {
					declareResults(t)
				}
				return got
			}

			got := serve(tc.storage)
			st := Read(ctx, "truenas", "storage.status", "289471021834760193", "289471021834760193", nil, false).Storage()
			assertOperationRequest(t, got, "storage.status")
			if st.Outcome != OutcomeSucceeded || !st.NeedsPassword || !st.PlacesReadable || st.NeedsNewPassword == tc.results {
				t.Errorf("storage status did not land: %+v", st)
			}
			if len(st.Places) != 1 || !st.Places[0].Home || st.Places[0].Usage == nil || st.Places[0].Usage.UsedBytes != 19756849561 {
				t.Errorf("storage places did not land: %+v", st.Places)
			}
			if st.Standing == nil || st.Standing.Usable || st.Standing.WaitingOn != "member" || st.Standing.Next == "" {
				t.Errorf("the account's standing did not land: %+v", st.Standing)
			}
			// When the add-on read it lands with it; the legacy pocket does not say.
			if want := time.Date(2026, 11, 10, 9, 30, 0, 0, time.UTC); tc.results != (st.Observation != nil) ||
				tc.results && (!st.Observation.TakenAt.Equal(want) || !st.Observation.Current) {
				t.Errorf("the read's observation did not land: %+v", st.Observation)
			}

			got = serve(tc.activity)
			act := Activity(ctx, "truenas", "289471021834760193", "operator-1", "2026-11-01T00:00:00Z", false)
			assertOperationRequest(t, got, "activity.get")
			if subject, actor := sentAs(got); subject != "289471021834760193" || actor != "operator-1" {
				t.Errorf("activity.get sent subject %q actor %q: the person is the subject, the operator the actor", subject, actor)
			}
			if act.Outcome != OutcomeSucceeded || !act.Truncated || len(act.Uncovered) != 1 || act.Uncovered[0] != "scratch" || len(act.Events) != 1 {
				t.Fatalf("the activity report did not land: %+v", act)
			}
			if e := act.Events[0]; e.At != "2026-11-10T08:12:30Z" || e.Event != "CONNECT" || e.Resource != "makers" || e.Success || e.Address != "192.0.2.17" || e.Detail != "NT_STATUS_NO_SUCH_USER" {
				t.Errorf("an event did not land: %+v", e)
			}
			if (act.Observation != nil) != tc.results {
				t.Errorf("when the log was read: %+v, want it exactly from a results reply", act.Observation)
			}

			got = serve(tc.other)
			sys := SystemHealth(ctx, "truenas", "operator-1")
			assertOperationRequest(t, got, "health.get")
			if subject, actor := sentAs(got); subject != "" || actor != "operator-1" {
				t.Errorf("health.get sent subject %q actor %q: about the target, asked by the operator", subject, actor)
			}
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
		})
	}
}

// sentAs is who a captured operation request named.
func sentAs(got *seen) (subject, actor string) {
	var sent struct{ Subject, Actor string }
	_ = json.Unmarshal(got.body, &sent)
	return sent.Subject, sent.Actor
}

// declareResults is the installed add-on's manifest declaring feature results.
func declareResults(t *testing.T) {
	t.Helper()
	a, err := Get("truenas")
	if err != nil {
		t.Fatal(err)
	}
	registryMu.Lock()
	a.manifest.Features = append(slices.Clone(a.manifest.Features), "results")
	registryMu.Unlock()
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

// The error body: what this side keeps of a refusal is its code, and only for
// classification it also reads a contract-1 add-on's `error`.
func TestErrorBodyLands(t *testing.T) {
	if code := refusalCode(contractFixture(t, "error.json")); code != "CONTRACT_VERSION_MISMATCH" {
		t.Errorf("the problem's code did not land: %q", code)
	}
	if code := refusalCode([]byte(`{"error":"LIFECYCLE_REFUSAL","lifecycle":"draining"}`)); code != "LIFECYCLE_REFUSAL" {
		t.Errorf("a contract-1 refusal's code did not land: %q", code)
	}
	if code := refusalCode([]byte(`{"code":"PLAN_STALE","error":"LIFECYCLE_REFUSAL"}`)); code != "PLAN_STALE" {
		t.Errorf("the problem's code must win over a legacy key: %q", code)
	}
}
