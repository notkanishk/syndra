package claims

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestFormatRoles(t *testing.T) {
	roles := []string{"admin", "viewer"}

	if got, ok := FormatRoles(roles, FormatArray).([]string); !ok || !reflect.DeepEqual(got, roles) {
		t.Fatalf("array format: got %#v", got)
	}
	if got := FormatRoles(roles, FormatCSV); got != "admin,viewer" {
		t.Fatalf("csv format: got %v", got)
	}
	if got := FormatRoles(roles, FormatSpaceDelimited); got != "admin viewer" {
		t.Fatalf("space_delimited format: got %v", got)
	}
	// An unknown format degrades to an array: roles in the wrong shape are
	// recoverable, roles missing entirely are a locked door.
	if got := FormatRoles(roles, "wat"); !reflect.DeepEqual(got, roles) {
		t.Fatalf("unknown format should fall back to array, got %#v", got)
	}
	if empty, ok := FormatRoles(nil, FormatArray).([]string); !ok || empty == nil || len(empty) != 0 {
		t.Fatalf("nil roles should render as an empty array, got %#v", empty)
	}
}

func tokenJSON(t *testing.T, p Profile, f Facts) string {
	t.Helper()
	b, err := json.Marshal(Shape(p, f))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The v1 seed must reproduce exactly what Actions v1 sent, in row order, with
// the shared parent merged into one object.
func TestShape_ActionsV1(t *testing.T) {
	f := Facts{Roles: []string{"staff", "admin"}, GivenName: "Ada", FamilyName: "Lovelace", Name: "Ada Lovelace"}
	want := `{"groups":["staff","admin"],"custom_claims":{"groups":["staff","admin"],"given_name":"Ada","family_name":"Lovelace","name":"Ada Lovelace"}}`
	if got := tokenJSON(t, ActionsV1(), f); got != want {
		t.Fatalf("v1 shape:\n got %s\nwant %s", got, want)
	}
}

// Filter, rename, then dedupe: a left-out role is gone, a renamed role is sent
// in the app's word, two roles sharing a word send it once, and a role added
// later follows IncludeNew.
func TestShape_RoleMap(t *testing.T) {
	p := Profile{
		Claims: groups("groups"),
		Roles: []RoleMap{
			{Key: "staff", As: "netbird-staff", Included: true},
			{Key: "admin", As: "netbird-admins", Included: true},
			{Key: "owner", As: "netbird-admins", Included: true},
			{Key: "guest", As: "netbird-guests", Included: false},
		},
		Lands: LandsBoth,
	}
	f := Facts{Roles: []string{"admin", "guest", "owner", "staff", "trained"}}
	if got, want := tokenJSON(t, p, f), `{"groups":["netbird-staff","netbird-admins"]}`; got != want {
		t.Fatalf("got %s want %s", got, want)
	}
	p.IncludeNew = true
	if got, want := tokenJSON(t, p, f), `{"groups":["netbird-staff","netbird-admins","trained"]}`; got != want {
		t.Fatalf("include new: got %s want %s", got, want)
	}
}

// Every roles row is sent even when nothing survives, nested ones included:
// Supabase keeps old groups when the claim is missing.
func TestShape_RolesAlwaysSent(t *testing.T) {
	p := Profile{Claims: groups("custom_claims", "groups"), Lands: LandsUserinfo}
	if got, want := tokenJSON(t, p, Facts{Roles: []string{"x"}}), `{"custom_claims":{"groups":[]}}`; got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

// One order for a token's roles, wherever they were read: Zitadel's rows
// deduplicated and sorted.
func TestRoleSet(t *testing.T) {
	if got := RoleSet([]string{"staff", "admin", "staff", "guest"}); !reflect.DeepEqual(got, []string{"admin", "guest", "staff"}) {
		t.Fatalf("got %v", got)
	}
	if got := RoleSet(nil); len(got) != 0 {
		t.Fatalf("none: %v", got)
	}
}

// With no facts, only the rows the token itself can answer stay: roles,
// if_role, statics, and the ids and role count. Nothing is sent empty.
func TestRolesOnly(t *testing.T) {
	p := Profile{Lands: LandsBoth, IncludeNew: true, Claims: []Claim{
		{Path: []string{"r"}, Kind: KindRoles},
		{Path: []string{"email"}, Kind: KindAttribute, Source: AttrEmail},
		{Path: []string{"svc"}, Kind: KindAttribute, Source: AttrIsServiceAccount},
		{Path: []string{"uid"}, Kind: KindAttribute, Source: AttrUserID},
		{Path: []string{"pid"}, Kind: KindAttribute, Source: AttrProjectID},
		{Path: []string{"n"}, Kind: KindAttribute, Source: AttrRoleCount},
		{Path: []string{"b"}, Kind: KindBundles},
		{Path: []string{"e"}, Kind: KindRoleEnds},
		{Path: []string{"is"}, Kind: KindIfRole, Roles: []string{"admin"}},
		{Path: []string{"s"}, Kind: KindStatic, Value: json.RawMessage(`1`)},
	}}
	got, _ := json.Marshal(Shape(p.RolesOnly(), Facts{UserID: "u", ProjectID: "p", Roles: []string{"admin"}}))
	if string(got) != `{"r":["admin"],"uid":"u","pid":"p","n":1,"is":true,"s":1}` {
		t.Fatalf("got %s", got)
	}
	if len(p.Claims) != 10 {
		t.Fatal("RolesOnly changed the profile it was given")
	}
}

func TestShape_EveryKind(t *testing.T) {
	p := Profile{
		Claims: []Claim{
			{Path: []string{"is_admin"}, Kind: KindIfRole, Roles: []string{"admin"}},
			{Path: []string{"is_owner"}, Kind: KindIfRole, Roles: []string{"owner"}},
			{Path: []string{"bundles"}, Kind: KindBundles},
			{Path: []string{"ends"}, Kind: KindRoleEnds},
			{Path: []string{"svc"}, Kind: KindAttribute, Source: AttrIsServiceAccount},
			{Path: []string{"title"}, Kind: KindAttribute, Source: AttrTitle},
			{Path: []string{"tenant"}, Kind: KindStatic, Value: json.RawMessage(`7`)},
			{Path: []string{"syndra.laser.roles"}, Kind: KindRoles, Format: FormatCSV},
		},
		Roles:      []RoleMap{{Key: "staff", As: "stf", Included: true}, {Key: "temp", Included: false}},
		IncludeNew: true,
		Lands:      LandsBoth,
	}
	f := Facts{
		Roles:    []string{"admin", "staff", "temp"},
		RoleEnds: map[string]string{"staff": "2027-05-31", "temp": "2026-12-01"},
	}
	// Title is empty, so it is left out rather than sent as null; the service
	// account flag is false and still sent; a left-out role's end is not sent;
	// the dotted key stays one flat key.
	want := `{"is_admin":true,"is_owner":false,"bundles":[],"ends":{"stf":"2027-05-31"},"svc":false,"tenant":7,"syndra.laser.roles":"stf,admin"}`
	if got := tokenJSON(t, p, f); got != want {
		t.Fatalf("\n got %s\nwant %s", got, want)
	}
}

func TestLandsIn(t *testing.T) {
	cases := []struct {
		lands, fn string
		want      bool
	}{
		{LandsBoth, FunctionPreAccessToken, true},
		{LandsBoth, FunctionPreUserinfo, true},
		{LandsAccessToken, FunctionPreUserinfo, false},
		{LandsAccessToken, FunctionPreAccessToken, true},
		{LandsUserinfo, FunctionPreAccessToken, false},
		{LandsUserinfo, FunctionPreUserinfo, true},
		{LandsUserinfo, "", true},
	}
	for _, c := range cases {
		if got := (Profile{Lands: c.lands}).LandsIn(c.fn); got != c.want {
			t.Errorf("lands=%s fn=%q: got %v", c.lands, c.fn, got)
		}
	}
}

func TestEffective(t *testing.T) {
	app, def := ActionsV1(), FromFlat("printing.roles", FormatArray, nil, nil)
	if got := Effective(&app, &def); !reflect.DeepEqual(got, app) {
		t.Fatal("an app's own shape wins")
	}
	if got := Effective(nil, &def); !reflect.DeepEqual(got, def) {
		t.Fatal("no shape of its own: the project default")
	}
	if got := Effective(nil, nil); !reflect.DeepEqual(got, Builtin()) {
		t.Fatal("no default either: the built-in roles list, never nothing")
	}
}

func TestFromFlat_OrderAndTypes(t *testing.T) {
	p := FromFlat("", "", map[string]string{"team": AttrTeam, "email": AttrEmail}, map[string]any{"n": 2.0, "a": "x"})
	got := tokenJSON(t, p, Facts{Roles: []string{"r"}, Email: "e@example.edu", Team: "T"})
	if want := `{"roles":["r"],"email":"e@example.edu","team":"T","a":"x","n":2}`; got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestValidate(t *testing.T) {
	ok := func(p Profile) {
		t.Helper()
		if err := Validate(Normalize(p)); err != nil {
			t.Fatalf("expected valid, got %v", err)
		}
	}
	bad := func(p Profile, want string) {
		t.Helper()
		err := Validate(Normalize(p))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("want error containing %q, got %v", want, err)
		}
	}
	ok(ActionsV1())
	for _, tpl := range Templates() {
		ok(tpl.Profile)
	}
	ok(Profile{Claims: []Claim{{Path: []string{"syndra.laser.roles"}, Kind: KindRoles}}})

	bad(Profile{Claims: []Claim{
		{Path: []string{"custom_claims"}, Kind: KindRoles},
		{Path: []string{"custom_claims", "groups"}, Kind: KindRoles},
	}}, "`custom_claims` is already an object here, holding `groups`.")
	bad(Profile{Claims: []Claim{
		{Path: []string{"groups"}, Kind: KindRoles},
		{Path: []string{"groups"}, Kind: KindBundles},
	}}, "already sent by another row")
	bad(Profile{Claims: []Claim{{Path: []string{"x"}, Kind: KindStatic, Value: json.RawMessage(`{"a":1}`)}}}, "fixed value")
	bad(Profile{Claims: []Claim{{Path: []string{"x"}, Kind: KindStatic, Value: json.RawMessage(`null`)}}}, "fixed value")
	bad(Profile{Claims: []Claim{{Path: []string{"x"}, Kind: KindIfRole}}}, "at least one role")
	bad(Profile{Claims: []Claim{{Path: []string{"x"}, Kind: KindAttribute, Source: "ssn"}}}, "unknown source")
	bad(Profile{Claims: []Claim{{Path: []string{"x"}, Kind: "wat"}}}, "unknown kind")
	bad(Profile{Claims: []Claim{{Path: []string{"has space"}, Kind: KindRoles}}}, "not a valid claim name")
	bad(Profile{Claims: []Claim{{Path: []string{"x"}, Kind: KindRoles, Format: "tsv"}}}, "format")
	bad(Profile{Lands: "id_token"}, "lands")
	bad(Profile{Roles: []RoleMap{{Key: "a"}, {Key: "a"}}}, "twice")

	// The error names its row, so the UI can put it under that row.
	err := Validate(Normalize(Profile{Claims: []Claim{{Path: []string{"ok"}, Kind: KindRoles}, {Path: []string{"x"}, Kind: "wat"}}}))
	if re, isRow := err.(RowError); !isRow || re.Row != 1 {
		t.Fatalf("want RowError on row 1, got %#v", err)
	}
}

func TestDiff(t *testing.T) {
	old := Profile{
		Claims: []Claim{
			{Path: []string{"roles"}, Kind: KindRoles, Format: FormatArray},
			{Path: []string{"team"}, Kind: KindAttribute, Source: AttrTeam},
			{Path: []string{"email"}, Kind: KindAttribute, Source: AttrEmail},
		},
		Roles: []RoleMap{
			{Key: "staff", As: "netbird-staff", Included: true},
			{Key: "guest", As: "netbird-guests", Included: true},
		},
		Lands: LandsBoth,
	}
	new := Profile{
		Claims: []Claim{
			{Path: []string{"groups"}, Kind: KindRoles, Format: FormatArray},
			{Path: []string{"team"}, Kind: KindAttribute, Source: AttrTitle},
			{Path: []string{"svc"}, Kind: KindAttribute, Source: AttrIsServiceAccount},
		},
		Roles: []RoleMap{
			{Key: "staff", As: "network-staff", Included: true},
			{Key: "guest", As: "netbird-guests", Included: false},
			{Key: "admin", Included: true},
		},
		IncludeNew: true,
		Lands:      LandsAccessToken,
	}
	got := Diff(old, new)
	want := []Change{
		{Kind: ChangeMove, Path: []string{"roles"}, NewPath: []string{"groups"}},
		{Kind: ChangeRetype, Path: []string{"team"}, From: "attribute:team", To: "attribute:title"},
		{Kind: ChangeAdd, Path: []string{"svc"}, To: "attribute:is_service_account"},
		{Kind: ChangeRemove, Path: []string{"email"}, From: "attribute:email"},
		{Kind: ChangeRename, Role: "staff", From: "netbird-staff", To: "network-staff"},
		{Kind: ChangeRemove, Role: "guest", From: "netbird-guests"},
		// admin was unmapped and IncludeNew was off, so it was left out.
		{Kind: ChangeAdd, Role: "admin", To: "admin"},
		{Kind: ChangeLands, From: LandsBoth, To: LandsAccessToken},
		{Kind: ChangeIncludeNew, From: "false", To: "true"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("diff:\n got %+v\nwant %+v", got, want)
	}
	if len(Diff(old, old)) != 0 {
		t.Fatal("a profile does not differ from itself")
	}
	if shape := ShapeOnly(got); len(shape) != 6 {
		t.Fatalf("role words never count as shape changes, got %+v", shape)
	}
}

func TestBreaking(t *testing.T) {
	netbird, _ := FindTemplate("netbird")
	ha, _ := FindTemplate("home-assistant")
	custom, _ := FindTemplate("custom")

	rename := Change{Kind: ChangeRename, Role: "staff", From: "netbird-staff", To: "network-staff"}
	leaveOut := Change{Kind: ChangeRemove, Role: "guest", From: "netbird-guests"}
	include := Change{Kind: ChangeAdd, Role: "admin", To: "admin"}
	addClaim := Change{Kind: ChangeAdd, Path: []string{"svc"}}
	dropClaim := Change{Kind: ChangeRemove, Path: []string{"groups"}}
	all := []Change{rename, leaveOut, include, addClaim, dropClaim}

	if got := Breaking(netbird, all); !reflect.DeepEqual(got, []Change{rename, leaveOut, dropClaim}) {
		t.Fatalf("netbird: any rename or removal breaks, got %+v", got)
	}
	if got := Breaking(custom, all); len(got) != 0 {
		t.Fatalf("custom: nothing is breaking, got %+v", got)
	}
	haRename := Change{Kind: ChangeRename, Role: "admin", From: "admins", To: "ha-admins"}
	if got := Breaking(ha, append(all, haRename)); !reflect.DeepEqual(got, []Change{dropClaim, haRename}) {
		t.Fatalf("home assistant: only admins/users and the claim itself, got %+v", got)
	}
}

// The catalog follows docs/research/2026-11-03-app-claims.md: claim keys, value
// shapes and placement. A change here is a change to what real apps read.
func TestTemplates_MatchTheResearch(t *testing.T) {
	type want struct {
		token string // for a person holding "a"
		lands string
		reads string
		rule  string
	}
	expect := map[string]want{
		"netbird":           {`{"groups":[]}`, LandsBoth, ReadsAccessToken, BreaksRenameOrRemove},
		"gitlab":            {`{"groups":["a"]}`, LandsUserinfo, ReadsUserinfo, BreaksRenameOrRemove},
		"supabase-oidc":     {`{"custom_claims":{"groups":["a"]}}`, LandsUserinfo, ReadsIDToken, BreaksRenameOrRemove},
		"supabase-keycloak": {`{"groups":["a"]}`, LandsUserinfo, ReadsUserinfo, BreaksRenameOrRemove},
		"home-assistant":    {`{"groups":["a"]}`, LandsUserinfo, ReadsIDToken, BreaksAdminsUsers},
		"sign-in-only":      {`{}`, LandsBoth, "", BreaksNever},
		"custom":            {`{"roles":["a"]}`, LandsBoth, "", BreaksNever},
	}
	order := []string{"netbird", "gitlab", "supabase-oidc", "supabase-keycloak", "home-assistant", "sign-in-only", "custom"}
	got := Templates()
	if len(got) != len(order) {
		t.Fatalf("want %d templates, got %d", len(order), len(got))
	}
	for i, tpl := range got {
		if tpl.ID != order[i] {
			t.Fatalf("template %d: want %s, got %s", i, order[i], tpl.ID)
		}
		w := expect[tpl.ID]
		if tok := tokenJSON(t, tpl.Profile, Facts{Roles: []string{"a"}}); tok != w.token {
			t.Errorf("%s token: got %s want %s", tpl.ID, tok, w.token)
		}
		if tpl.Profile.Lands != w.lands || tpl.Reads != w.reads || tpl.Breaks != w.rule {
			t.Errorf("%s: lands=%s reads=%s rule=%s", tpl.ID, tpl.Profile.Lands, tpl.Reads, tpl.Breaks)
		}
		for _, c := range tpl.Profile.Claims {
			if c.Kind == KindRoles && c.Format != FormatArray {
				t.Errorf("%s: templates pin array, got %s", tpl.ID, c.Format)
			}
		}
	}
	// Templates() hands out copies: editing one never edits the catalog.
	got[0].Profile.Claims[0].Path[0] = "mutated"
	if again, _ := FindTemplate("netbird"); again.Profile.Claims[0].Path[0] != "groups" {
		t.Fatal("the catalog was mutated through a copy")
	}
}

func TestSuggest(t *testing.T) {
	for name, want := range map[string]string{
		"NetBird clients": "netbird", "GitLab": "gitlab", "Supabase Studio": "supabase-oidc", "Home Assistant": "home-assistant",
	} {
		if got, ok := Suggest(name); !ok || got.ID != want {
			t.Errorf("%s: got %v", name, got.ID)
		}
	}
	if _, ok := Suggest("Inventory"); ok {
		t.Error("an in-house app has no suggestion")
	}
}
