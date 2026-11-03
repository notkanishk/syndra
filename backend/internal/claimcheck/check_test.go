package claimcheck

import (
	"encoding/json"
	"errors"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"syndra/internal/claims"
	"syndra/internal/claimserve"
	"syndra/internal/claimtel"
	"syndra/internal/zitadel"
)

const claimsKey = "test-signing-key-claims-0123456789abcdef"

func load[T any](t *testing.T, name string) []T {
	t.Helper()
	raw, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	var res struct {
		Result []T `json:"result"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		t.Fatal(err)
	}
	return res.Result
}

// baseline is a correct configuration, read from the recorded responses.
func baseline(t *testing.T) Inputs {
	t.Helper()
	groups := claims.Profile{Claims: []claims.Claim{{Path: []string{"groups"}, Kind: claims.KindRoles}},
		Roles: []claims.RoleMap{{Key: "laser", Included: true}}, Lands: claims.LandsBoth}
	return Inputs{
		ExternalURL: "https://syndra.example.edu",
		Targets:     load[zitadel.ActionTarget](t, "targets.json"),
		Executions:  load[zitadel.ActionExecution](t, "executions.json"),
		Apps: []App{
			{ID: "app-1", Name: "NetBird", ClientID: "c1", ProjectID: "p1", OIDC: true,
				Settings: &zitadel.TokenSettings{AccessTokenType: "JWT", IDTokenUserinfoAssertion: true, ProjectRoleAssertion: true}},
			{ID: "app-2", Name: "Wiki", ClientID: "c2", ProjectID: "p1", OIDC: true,
				Settings: &zitadel.TokenSettings{AccessTokenType: "BEARER"}},
		},
		Snapshot: &claimserve.Config{Projects: map[string]claimserve.Project{
			"p1": {Shapes: claimserve.ProjectShapes{Default: claims.Builtin(), ByClient: map[string]claimserve.AppShape{
				"c1": {ApplicationID: "app-1", TemplateID: "netbird", Profile: groups},
			}}},
		}},
		ProjectRoles: map[string][]string{"p1": {"laser", "cnc"}},
		HeldKeys:     []string{KeyCheck(claimsKey)},
		EnvKey:       KeyCheck(claimsKey),
	}
}

func drifts(r Report) []string {
	var out []string
	for _, row := range r.Rows {
		if row.State == Drift || row.State == Cannot {
			out = append(out, row.Subject)
		}
	}
	sort.Strings(out)
	return out
}

func TestEvaluate_BaselineIsClean(t *testing.T) {
	r := Evaluate(baseline(t))
	if d := drifts(r); len(d) != 0 {
		t.Fatalf("a correct configuration drifts: %v", d)
	}
	seen := map[string]bool{}
	for _, row := range r.Rows {
		seen[row.Check] = true
	}
	for _, c := range []string{"R1", "R2", "R3", "R4", "R5", "R6", "R7", "R8", "R9"} {
		if !seen[c] {
			t.Errorf("no row for %s", c)
		}
	}
	if !r.JWT["c1"] || r.JWT["c2"] {
		t.Errorf("token types = %v", r.JWT)
	}
	if len(r.Ran()) != 9 {
		t.Errorf("ran = %v", r.Ran())
	}
}

// Every field of the recorded target and executions, mutated in turn, raises
// exactly its own drift.
func TestEvaluate_EachFieldMutated(t *testing.T) {
	claim := func(in *Inputs) *zitadel.ActionTarget { return &in.Targets[0] }
	exec := func(in *Inputs, i int) *zitadel.ActionExecution { return &in.Executions[i] }
	for _, tc := range []struct {
		name   string
		mutate func(*Inputs)
		want   []string
	}{
		{"endpoint changed", func(in *Inputs) { claim(in).Endpoint = "https://old.example.edu/api/action/inject" }, []string{"R1:endpoint"}},
		{"type webhook", func(in *Inputs) {
			claim(in).RestCall, claim(in).RestWebhook = nil, json.RawMessage(`{}`)
		}, []string{"R1:type"}},
		{"type async", func(in *Inputs) {
			claim(in).RestCall, claim(in).RestAsync = nil, json.RawMessage(`{}`)
		}, []string{"R1:type"}},
		{"interrupt on error", func(in *Inputs) { claim(in).RestCall.InterruptOnError = true }, []string{"R1:interrupt_on_error"}},
		{"timeout lowered", func(in *Inputs) { claim(in).Timeout = "0.500s" }, []string{"R1:timeout"}},
		{"payload JWT", func(in *Inputs) { claim(in).PayloadType = "PAYLOAD_TYPE_JWT" }, []string{"R1:payload_type"}},
		{"key rotated", func(in *Inputs) { claim(in).SigningKey = "a-new-key-the-data-plane-has-not-read" }, []string{"R1:signing_key"}},
		{"target deleted", func(in *Inputs) { in.Targets = in.Targets[1:] }, []string{"R1:present", "R2:preaccesstoken", "R2:preuserinfo"}},
		{"target renamed", func(in *Inputs) { claim(in).Name = "claims" }, []string{"R1:present", "R2:preaccesstoken", "R2:preuserinfo"}},
		{"preaccesstoken unbound", func(in *Inputs) { in.Executions = in.Executions[1:] }, []string{"R2:preaccesstoken"}},
		{"preuserinfo list replaced", func(in *Inputs) {
			exec(in, 1).Targets = []json.RawMessage{json.RawMessage(`"339871032650000002"`)}
		}, []string{"R2:preuserinfo"}},
		{"a second target on preaccesstoken", func(in *Inputs) {
			exec(in, 0).Targets = append(exec(in, 0).Targets, json.RawMessage(`"999"`))
		}, []string{"R2:preaccesstoken"}},
		{"function prefixed is the same", func(in *Inputs) {
			exec(in, 0).Condition.Function.Name = "function/preaccesstoken"
		}, nil},
		{"event unbound", func(in *Inputs) { in.Executions = append(in.Executions[:9], in.Executions[10:]...) }, []string{"R3:oidc_session.added"}},
		{"listener endpoint", func(in *Inputs) { in.Targets[1].Endpoint = "https://syndra.example.edu/hooks" }, []string{"R3:target"}},
		{"listener made restCall", func(in *Inputs) {
			in.Targets[1].RestAsync, in.Targets[1].RestCall = nil, &zitadel.RestCall{}
		}, []string{"R3:target"}},
		{"app turned bearer", func(in *Inputs) { in.Apps[0].Settings.AccessTokenType = "BEARER" }, []string{"R5:app-1"}},
		{"app recreated: shape names a gone client", func(in *Inputs) { in.Apps[0].ClientID = "c1-new" }, []string{"R6:shape:c1"}},
		{"project unpublished", func(in *Inputs) { in.Apps[1].ProjectID = "p2" }, []string{"R6:app:app-2"}},
		{"role deleted", func(in *Inputs) { in.ProjectRoles["p1"] = []string{"cnc"} }, []string{"R7:p1:laser"}},
		{"external url http", func(in *Inputs) {
			in.ExternalURL = "http://syndra.example.edu"
			for i := range in.Targets {
				in.Targets[i].Endpoint = strings.Replace(in.Targets[i].Endpoint, "https", "http", 1)
			}
		}, []string{"R9:url_https"}},
		{"external url does not resolve", func(in *Inputs) { in.ResolveErr = errors.New("no such host") }, []string{"R9:url_resolves"}},
		{"no env key", func(in *Inputs) { in.EnvKey = "" }, []string{"R9:key_present"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := baseline(t)
			tc.mutate(&in)
			got := drifts(Evaluate(in))
			sort.Strings(tc.want)
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("drifts = %v, want %v", got, tc.want)
			}
		})
	}
}

// No permission is never silent: each check that could not read is
// CANNOT_CHECK, and none of its findings settle on a pass it never made.
func TestEvaluate_ForbiddenIsCannotCheck(t *testing.T) {
	in := baseline(t)
	forbidden := &zitadel.StatusError{Code: 403, Message: "No matching permissions found"}
	in.Targets, in.TargetsErr = nil, forbidden
	in.Executions, in.ExecutionsErr = nil, forbidden
	r := Evaluate(in)
	if d := drifts(r); strings.Join(d, ",") != "R1,R2,R3" {
		t.Fatalf("drifts = %v", d)
	}
	for _, c := range r.Ran() {
		if c == "R1" || c == "R2" || c == "R3" {
			t.Errorf("%s did not run", c)
		}
	}
	drift, cannotObs := r.Observations(time.Now())
	for _, o := range drift {
		if strings.HasPrefix(o.Subject, "R1") || strings.HasPrefix(o.Subject, "R2") || strings.HasPrefix(o.Subject, "R3") {
			t.Errorf("an unread check produced %+v", o)
		}
	}
	fails := 0
	for _, o := range cannotObs {
		if o.Fail {
			fails++
			if o.Kind != claimtel.KindCannotCheck || o.Sample["status"] != 403 {
				t.Errorf("cannot = %+v", o)
			}
		}
	}
	if fails != 3 {
		t.Errorf("cannot-check failures = %d, want 3", fails)
	}
}

func TestEvaluate_V1IsInformational(t *testing.T) {
	in := baseline(t)
	in.V1 = []zitadel.TokenAction{
		{Name: "roles", Script: `function c(ctx, api) { api.v1.claims.setClaim("groups", []); api.v1.claims.setClaim('dept', 'x') }`, State: "ACTION_STATE_ACTIVE"},
		{Name: "old", Script: `api.v1.claims.setClaim("gone", 1)`, State: "ACTION_STATE_INACTIVE"},
	}
	r := Evaluate(in)
	if d := drifts(r); len(d) != 0 {
		t.Fatalf("v1 is informational, drifts = %v", d)
	}
	for _, row := range r.Rows {
		if row.Subject != "R4:v1" {
			continue
		}
		if row.State != Info || row.Facts["keys"] != 2 || len(row.Facts["collide"].([]string)) != 1 {
			t.Errorf("R4 = %+v", row)
		}
		return
	}
	t.Fatal("no R4 row")
}

func TestEvaluate_EnvKeyBehindTheKeyringIsInfo(t *testing.T) {
	in := baseline(t)
	in.Targets[0].SigningKey = "rotated-key-that-the-keyring-already-follows"
	in.HeldKeys = append(in.HeldKeys, KeyCheck("rotated-key-that-the-keyring-already-follows"))
	r := Evaluate(in)
	if d := drifts(r); len(d) != 0 {
		t.Fatalf("drifts = %v", d)
	}
	found := false
	for _, row := range r.Rows {
		found = found || (row.Subject == "R9:env_key" && row.State == Info)
	}
	if !found {
		t.Error("an env key behind Zitadel's is said")
	}
}

func TestEvaluate_NoSnapshotCannotCheckShapes(t *testing.T) {
	in := baseline(t)
	in.Snapshot = nil
	got := drifts(Evaluate(in))
	if strings.Join(got, ",") != "R5,R6,R7" {
		t.Fatalf("drifts = %v", got)
	}
}

func TestKeyCheckNeverEchoesTheKey(t *testing.T) {
	k := KeyCheck(claimsKey)
	if strings.Contains(k, claimsKey) || len(k) != 64 || KeyCheck("") != "" {
		t.Errorf("KeyCheck = %q", k)
	}
}

// The manifest here is targets.json's: a binding added there and not here
// would never be checked.
func TestManifestMatchesTargetsJSON(t *testing.T) {
	raw, err := os.ReadFile("../../../zitadel/actions/targets.json")
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Targets []struct {
			Name, Endpoint, Timeout string
			RestCall                *struct{ InterruptOnError bool }
		}
		Executions []struct {
			Target    string
			Condition struct {
				Function *struct{ Name string }
				Event    *struct{ Event string }
			}
		}
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	var fns, evs []string
	for _, e := range m.Executions {
		switch {
		case e.Condition.Function != nil && e.Target == ClaimTarget:
			fns = append(fns, e.Condition.Function.Name)
		case e.Condition.Event != nil && e.Target == EventTarget:
			evs = append(evs, e.Condition.Event.Event)
		default:
			t.Errorf("unexpected execution %+v", e)
		}
	}
	if strings.Join(fns, ",") != strings.Join(ClaimFunctions, ",") || strings.Join(evs, ",") != strings.Join(ManifestEvents, ",") {
		t.Errorf("manifest: functions %v events %v", fns, evs)
	}
	for _, tg := range m.Targets {
		if tg.Name == ClaimTarget && (tg.Endpoint != "${SYNDRA_EXTERNAL_URL}"+ClaimPath || tg.Timeout != ClaimTimeout.String() || tg.RestCall == nil) {
			t.Errorf("claims target %+v", tg)
		}
		if tg.Name == EventTarget && tg.Endpoint != "${SYNDRA_EXTERNAL_URL}"+EventPath {
			t.Errorf("event target %+v", tg)
		}
	}
}
