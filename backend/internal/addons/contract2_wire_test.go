package addons

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Contract 2 at the backend (openspec/changes/addon-contract, tasks 1.3, 1.4a,
// 1.5, 1.9): what the manifest may mean, how a refusal is classified, how the
// query is signed, and how a cohort is planned.

// ---- 1.3: the manifest's meaning is checked at registration --------------

func refreshWith(t *testing.T, m Manifest) error {
	t.Helper()
	withManifest(t, m, nil)
	return Refresh(context.Background(), "truenas")
}

func withField(m Manifest, name, role string) Manifest {
	out := m
	out.EntitlementSchema = nil
	for _, f := range m.EntitlementSchema {
		if f.Name == name {
			f.Role = role
		}
		out.EntitlementSchema = append(out.EntitlementSchema, f)
	}
	return out
}

// The role map is persisted at registration, and a manifest that moves the
// account to smb_enabled is refused with its reason on the target page: the
// last good manifest and role map stay, so revocation still finds `enabled`.
func TestAManifestThatMovesTheAccountIsRefusedAndTheLastGoodMapKept(t *testing.T) {
	registerTrueNAS(t)
	forgetRoles(t)
	if err := refreshWith(t, goodManifest()); err != nil {
		t.Fatalf("the first manifest: %v", err)
	}
	roles, err := FieldRoles(context.Background(), "truenas")
	if err != nil || roles["enabled"] != RoleAccount || roles["smb_enabled"] != RoleChannel || roles["group"] != RoleMembership {
		t.Fatalf("the role map was not persisted at registration: %v %v", roles, err)
	}

	moved := withField(withField(goodManifest(), "enabled", RoleChannel), "smb_enabled", RoleAccount)
	err = refreshWith(t, moved)
	var refused *ErrManifestRefused
	if !errors.As(err, &refused) || !strings.Contains(refused.Reason, "never") {
		t.Fatalf("a manifest moving the account must be refused with its reason, got %v", err)
	}
	a, _ := Get("truenas")
	if a.LastError() == nil || !strings.Contains(a.LastError().Error(), `"enabled" was declared with role account`) {
		t.Errorf("the reason does not reach the target page: %v", a.LastError())
	}
	if roles, _ := FieldRoles(context.Background(), "truenas"); roles["enabled"] != RoleAccount || roles["smb_enabled"] != RoleChannel {
		t.Errorf("the refused manifest changed the persisted roles: %v", roles)
	}
	if m, err := a.Manifest(); err != nil || m.FieldsByRole()[RoleAccount][0] != "enabled" {
		t.Errorf("the last good manifest is not the one serving: %v %v", m.FieldsByRole(), err)
	}

	// A field never changes role, even one dropped and declared again.
	for name, m := range map[string]Manifest{
		"a membership becomes a flag": withField(goodManifest(), "group", RoleFlag),
		"the account moved to a new field": func() Manifest {
			m := goodManifest()
			m.EntitlementSchema = append([]EntitlementField{{Name: "active", Role: RoleAccount}}, m.EntitlementSchema[0], m.EntitlementSchema[2])
			return m
		}(),
		"another field is the account": func() Manifest {
			m := withField(goodManifest(), "enabled", RoleChannel)
			m.EntitlementSchema = append(m.EntitlementSchema, EntitlementField{Name: "active", Role: RoleAccount})
			return m
		}(),
	} {
		if err := refreshWith(t, m); !errors.As(err, &refused) {
			t.Errorf("%s: accepted (%v)", name, err)
		}
	}
	dropped := goodManifest()
	dropped.EntitlementSchema = dropped.EntitlementSchema[1:]
	if err := refreshWith(t, dropped); err != nil {
		t.Fatalf("dropping a field is not a change of meaning: %v", err)
	}
	if err := refreshWith(t, withField(goodManifest(), "group", RoleLimit)); !errors.As(err, &refused) {
		t.Errorf("a dropped field came back meaning something else: %v", err)
	}
}

// A kind outside the closed list, a role outside the vocabulary, or anything
// but one account field is refused at registration with a reason (design §3,
// §4).
func TestAManifestWhoseMeaningIsUnknownIsRefused(t *testing.T) {
	for name, edit := range map[string]func(*Manifest){
		"no kind":             func(m *Manifest) { m.Kind = "" },
		"an unknown kind":     func(m *Manifest) { m.Kind = "spaceship" },
		"an unknown role":     func(m *Manifest) { m.EntitlementSchema[0].Role = "toggle" },
		"no account":          func(m *Manifest) { m.EntitlementSchema = m.EntitlementSchema[:1] },
		"two accounts":        func(m *Manifest) { m.EntitlementSchema[2].Role = RoleAccount },
		"a field twice":       func(m *Manifest) { m.EntitlementSchema[1].Name = "group" },
		"a field unnamed":     func(m *Manifest) { m.EntitlementSchema[0].Name = " " },
		"the account demoted": func(m *Manifest) { m.EntitlementSchema[1].Role = RoleMembership },
	} {
		t.Run(name, func(t *testing.T) {
			registerTrueNAS(t)
			forgetRoles(t)
			m := goodManifest()
			edit(&m)
			var refused *ErrManifestRefused
			if err := refreshWith(t, m); !errors.As(err, &refused) {
				t.Fatalf("accepted: %v", err)
			}
			if _, err := FieldRoles(context.Background(), "truenas"); err == nil {
				t.Error("a refused manifest persisted a role map")
			}
		})
	}
}

// ---- 1.5: refusals classified by code; the rollback shape ----------------

// The NEW backend against a contract-1 add-on (the rollback shape): its
// {"error":"CONTRACT_VERSION_MISMATCH"} 422 is unreached — queued, never the
// target refusing the change, so never target_refused — and the manifest is
// read again; its 503 lifecycle refusal is a lifecycle refusal, not in doubt;
// its BAD_REQUEST for a missing subject stays rejected.
func TestTheNewBackendAgainstAContractOneAddOn(t *testing.T) {
	var reread atomic.Int32
	saved := rereadAfterShapeRefusal
	rereadAfterShapeRefusal = func(*Addon) { reread.Add(1) }
	t.Cleanup(func() { rereadAfterShapeRefusal = saved })

	for _, tc := range []struct {
		name, body string
		status     int
		want       Outcome
		lifecycle  bool
		shape      bool
		rereads    int32
	}{
		{"contract mismatch", `{"error":"CONTRACT_VERSION_MISMATCH","caller_version":2,"addon_version":1}`, 422, OutcomeUnreached, false, true, 1},
		{"lifecycle refusal", `{"error":"LIFECYCLE_REFUSAL","lifecycle":"draining"}`, 503, OutcomeUnreached, true, false, 1},
		{"missing subject", `{"error":"BAD_REQUEST"}`, 400, OutcomeRejected, false, false, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := applyAgainst(t, tc.status, []byte(tc.body))
			if out.Outcome != tc.want || out.LifecycleRefusal != tc.lifecycle {
				t.Errorf("outcome %s lifecycle %t, want %s %t", out.Outcome, out.LifecycleRefusal, tc.want, tc.lifecycle)
			}
			if reread.Load() != tc.rereads {
				t.Errorf("manifest re-reads %d, want %d", reread.Load(), tc.rereads)
			}
		})
	}

	// A 404 on a route the backend called under a feature is unreached and
	// re-read too; on a core route it stays a refusal.
	addonServingStatus(t, http.StatusNotFound, []byte(`{"type":"urn:syndra:addon:error:UNKNOWN_ROUTE","title":"Unknown route","status":404,"code":"UNKNOWN_ROUTE","retry":"never"}`))
	before := reread.Load()
	if r := ReadReach(context.Background(), "truenas", false); r.Read || reread.Load() != before+1 {
		t.Errorf("a 404 on /reach did not re-read the manifest: %+v (%d)", r, reread.Load()-before)
	}
	if out := Apply(context.Background(), ApplyRequest{Target: "truenas", Subject: "s", CallID: "c", Fingerprint: "f", Desired: fixtureDesired}); out.Outcome != OutcomeRejected {
		t.Errorf("a 404 on /apply is %s, want rejected", out.Outcome)
	}
	// A 404 on a feature's route that names another code is the add-on
	// answering about the request (no such setting): a refusal, no re-read.
	addonServingStatus(t, http.StatusNotFound, []byte(`{"type":"urn:syndra:addon:error:NO_SUCH_SETTING","title":"No such setting","status":404,"code":"NO_SUCH_SETTING","retry":"never"}`))
	stubOperationRecord(t, nil)
	before = reread.Load()
	if resp, _ := PutSetting(context.Background(), "truenas", "nope", "x", "op"); resp.Outcome != OutcomeRejected || resp.ShapeRefusal || reread.Load() != before {
		t.Errorf("a 404 NO_SUCH_SETTING is %s shape=%t (re-reads %d)", resp.Outcome, resp.ShapeRefusal, reread.Load()-before)
	}
}

// ---- 1.4a: the escaped path and the raw query are signed, as sent ---------

func TestTheEscapedPathAndRawQueryAreSigned(t *testing.T) {
	const secret = "a-test-secret"
	key := signingKeyFor(t, secret, "truenas")
	var target string
	var verified bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get(SignatureHeader)
		ts, mac, _ := strings.Cut(strings.TrimPrefix(header, "t="), ",v1=")
		unix, _ := strconv.ParseInt(ts, 10, 64)
		target = r.URL.EscapedPath()
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}
		m := hmac.New(sha256.New, key)
		fmt.Fprintf(m, "%d.%s.%s.", unix, r.Method, target)
		verified = hex.EncodeToString(m.Sum(nil)) == mac
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"field":"lab makers","values":[],"enumerable":true}`))
	}))
	t.Cleanup(srv.Close)
	signedAddon(t, srv.URL, secret)
	ReadValues(context.Background(), "truenas", "lab makers")
	if target != "/values/lab%20makers" || !verified {
		t.Fatalf("signed %q verified=%t: the escaped path as sent must be what is signed", target, verified)
	}
	// And a query: the signer is handed the URL as it goes on the wire.
	u, _ := http.NewRequest(http.MethodGet, "https://addon.example.org/stats?metric=a&since=2026-11-07T00%3A00%3A00Z", nil)
	if got := signedTarget(u.URL); got != "/stats?metric=a&since=2026-11-07T00%3A00%3A00Z" {
		t.Errorf("signedTarget = %q", got)
	}
}

// ---- 1.9: the backend plans in chunks below limits.subjects_per_plan --------

func TestACohortLargerThanAPlanIsPlannedInChunks(t *testing.T) {
	var sizes []int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req planEnvelope
		_ = json.NewDecoder(r.Body).Decode(&req)
		sizes = append(sizes, len(req.Subjects))
		outcomes := []map[string]any{}
		for _, s := range req.Subjects {
			outcomes = append(outcomes, map[string]any{"subject": s.Subject, "effect": "no_change", "detail": "Already."})
		}
		taken := "2026-11-10T09:30:00Z"
		if len(sizes) == 2 {
			taken = "2026-11-10T09:29:00Z"
		}
		body, _ := json.Marshal(map[string]any{"outcomes": outcomes, "current": len(sizes) != 3, "taken_at": taken, "truncated": false,
			"stale": map[string]any{"reason": "unreachable"}})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	signedAddon(t, srv.URL, "a-test-secret")
	a, _ := Get("truenas")
	registryMu.Lock()
	a.manifest.Limits.SubjectsPerPlan = 2
	registryMu.Unlock()

	subjects := make([]PlanSubject, 5)
	for i := range subjects {
		subjects[i] = PlanSubject{Subject: fmt.Sprintf("s-%d", i), Desired: map[string]json.RawMessage{"enabled": json.RawMessage(`false`)}}
	}
	r := Plan(context.Background(), "truenas", subjects)
	if fmt.Sprint(sizes) != "[2 2 1]" || len(r.Outcomes) != 5 || r.Outcome != OutcomeSucceeded {
		t.Fatalf("chunks %v, outcomes %d, %s", sizes, len(r.Outcomes), r.Outcome)
	}
	if r.Current || !r.TakenAt.Equal(time.Date(2026, 11, 10, 9, 29, 0, 0, time.UTC)) {
		t.Errorf("one plan is current only if every chunk was, dated by the oldest: %+v", r)
	}
}

// ---- 1.0: a reply is decoded into a wire type, never into a view ----------

// Every field a reply's decode target names must be a property the contract
// schema gives that reply: a field the backend keeps for itself (a Read, an
// Outcome) must not have a JSON name an add-on could send.
func TestEveryReplyDecodesIntoAWireTypeOfContractFields(t *testing.T) {
	type pair struct {
		v      any
		schema string
		path   []string
	}
	for _, p := range []pair{
		{subjectsWire{}, "subjects.json", nil},
		{TargetAccount{}, "subjects.json", []string{"$defs", "Subject"}},
		{healthWire{}, "health.json", nil},
		{planWire{}, "plan_response.json", nil},
		{SubjectOutcome{}, "apply_response.json", nil},
		{BindingConflict{}, "apply_response.json", []string{"$defs", "BindingConflict"}},
		{applyWire{}, "apply_response.json", nil},
		{reachWire{}, "profiles/storage/reach.json", nil},
		{homesWire{}, "profiles/storage/homes.json", nil},
		{HomeUsage{}, "profiles/storage/homes.json", []string{"properties", "homes", "items"}},
		{Values{}, "values.json", nil},
		{TargetSetting{}, "setting.json", nil},
		{Standing{}, "common/standing.json", nil},
		{placeWire{}, "profiles/storage/reach.json", []string{"$defs", "Place"}},
		{storageWire{}, "operation_response.json", []string{"$defs", "StorageStatus"}},
	} {
		var doc map[string]any
		if err := json.Unmarshal(mustRead(t, "schema", p.schema), &doc); err != nil {
			t.Fatal(err)
		}
		node := doc
		for _, k := range p.path {
			node, _ = node[k].(map[string]any)
		}
		props, _ := node["properties"].(map[string]any)
		rt := reflect.TypeOf(p.v)
		for i := range rt.NumField() {
			f := rt.Field(i)
			name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
			if f.Anonymous || name == "-" || name == "" {
				continue
			}
			if _, ok := props[name]; !ok {
				t.Errorf("%s decodes %q, which schema/%s does not give the add-on to send", rt.Name(), name, p.schema)
			}
		}
	}
}

// No wire type holds a view type anywhere inside it (a field, a slice, a map,
// a pointer): a view's backend-filled fields (Place's addresses and roles,
// Homes' and Reach's read) would otherwise be the add-on's to set again.
func TestNoWireTypeHoldsAView(t *testing.T) {
	views := map[reflect.Type]bool{}
	for _, v := range []any{Place{}, StorageStatus{}, Homes{}, Reach{}, TargetHealth{}, ApplyResponse{}, PlanResult{}, SubjectsResult{}} {
		views[reflect.TypeOf(v)] = true
	}
	var walk func(path string, rt reflect.Type, seen map[reflect.Type]bool)
	walk = func(path string, rt reflect.Type, seen map[reflect.Type]bool) {
		for rt.Kind() == reflect.Pointer || rt.Kind() == reflect.Slice || rt.Kind() == reflect.Map {
			rt = rt.Elem()
		}
		if rt.Kind() != reflect.Struct || seen[rt] {
			return
		}
		if views[rt] {
			t.Errorf("%s decodes into the view %s", path, rt.Name())
			return
		}
		seen[rt] = true
		for i := range rt.NumField() {
			walk(path+"."+rt.Field(i).Name, rt.Field(i).Type, seen)
		}
	}
	for _, w := range []any{subjectsWire{}, healthWire{}, planWire{}, applyWire{}, reachWire{}, homesWire{}, storageWire{}} {
		walk(reflect.TypeOf(w).Name(), reflect.TypeOf(w), map[reflect.Type]bool{})
	}
}

func mustRead(t *testing.T, parts ...string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(append([]string{contractDir}, parts...)...))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// ---- B6: a feature's route is called only when the feature is declared ----

// An add-on whose manifest declares no feature is never asked for a feature's
// route: nothing is sent, no operation row is begun, and the breaker never
// hears of it (a burst of such calls must not open the circuit).
func TestAnUndeclaredFeatureIsNeverCalled(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	signedAddon(t, srv.URL, "a-test-secret")
	a, _ := Get("truenas")
	registryMu.Lock()
	a.manifest.Features = nil
	registryMu.Unlock()
	withBreaker(t, 1, time.Hour)
	began, _ := stubOperationRecord(t, nil)

	ctx := context.Background()
	if r := ReadReach(ctx, "truenas", false); r.Read {
		t.Error("an undeclared storage.reach was read")
	}
	if h := ReadHomes(ctx, "truenas", false); h.Read {
		t.Error("an undeclared storage.homes was read")
	}
	if _, ok := ReadSettings(ctx, "truenas"); ok {
		t.Error("undeclared settings were read")
	}
	if resp, _ := PutSetting(ctx, "truenas", "home_root", "/mnt/x", "op"); !errors.Is(resp.Err, ErrFeatureNotDeclared) {
		t.Errorf("an undeclared setting write: %+v", resp)
	}
	if st := Read(ctx, "truenas", "storage.status", "s", "s", nil, false).Storage(); !errors.Is(st.Err, ErrFeatureNotDeclared) {
		t.Errorf("an undeclared operation was dispatched: %+v", st)
	}
	if hits.Load() != 0 || len(*began) != 0 || a.CircuitOpen() {
		t.Fatalf("an undeclared feature reached the add-on (%d), began a row (%d) or moved the breaker (%t)", hits.Load(), len(*began), a.CircuitOpen())
	}
	// A core route still goes, and still feeds the breaker.
	Subjects(ctx, "truenas", false)
	if hits.Load() != 1 || !a.CircuitOpen() {
		t.Errorf("a core route must still be called and recorded: hits %d open %t", hits.Load(), a.CircuitOpen())
	}
}
