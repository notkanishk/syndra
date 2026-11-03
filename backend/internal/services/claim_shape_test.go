package services

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"syndra/internal/claims"
	"syndra/internal/db"
	"syndra/internal/directory"
	"syndra/internal/models"
	"syndra/internal/zitadel"
)

var seedCutoff = time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)

// shapeDir is the directory the claims tests run against: NetBird and an old
// in-house app existed under Actions v1; GitLab came after the cutoff.
type shapeDir struct{ directory.Source }

var shapeApps = []models.ApplicationCatalog{
	{ID: "aNB", Name: "NetBird", ProjectID: "pNet", ClientID: "cNB", Created: seedCutoff.Add(-time.Hour)},
	{ID: "aInv", Name: "Inventory", ProjectID: "pInv", ClientID: "cInv"},
	{ID: "aGL", Name: "GitLab", ProjectID: "pCode", ClientID: "cGL", Created: seedCutoff.Add(time.Hour)},
}

var shapeProjects = map[string][]models.ProjectRole{
	"pNet":  {{Key: "staff", Label: "Staff"}, {Key: "admin", Label: "Admins"}, {Key: "guest", Label: "Guests"}},
	"pInv":  {{Key: "staff", Label: "Staff"}, {Key: "inv-admin", Label: "Admin"}, {Key: "trained", Label: "Trained"}},
	"pCode": {{Key: "users", Label: "Users"}},
}

func (shapeDir) Applications(context.Context) ([]models.ApplicationCatalog, error) {
	return shapeApps, nil
}
func (shapeDir) FindApplication(_ context.Context, id string) (models.ApplicationCatalog, bool, error) {
	for _, a := range shapeApps {
		if a.ID == id {
			return a, true, nil
		}
	}
	return models.ApplicationCatalog{}, false, nil
}
func (shapeDir) FindProject(_ context.Context, id string) (models.ProjectCatalog, bool, error) {
	roles, ok := shapeProjects[id]
	return models.ProjectCatalog{ID: id, Name: id, Roles: roles}, ok, nil
}
func (shapeDir) RoleKeysForProject(_ context.Context, id string) ([]string, error) {
	var out []string
	for _, r := range shapeProjects[id] {
		out = append(out, r.Key)
	}
	return out, nil
}
func (shapeDir) ProjectName(_ context.Context, id string) (string, error) { return id, nil }
func (shapeDir) Users(context.Context) ([]models.UserProfile, error)      { return nil, nil }

// shapeStore is an in-memory app_claim_shape_versions.
type shapeStore struct {
	versions  map[string][]db.ClaimShapeVersion
	defaults  map[string]db.ClaimProfileRow
	overrides []db.AppClaimOverrideRow
	stored    map[string]claims.Facts // user:project -> stored facts
	settings  *zitadel.TokenSettings
}

func stubClaimShapeDeps(t *testing.T) *shapeStore {
	t.Helper()
	s := &shapeStore{versions: map[string][]db.ClaimShapeVersion{}, defaults: map[string]db.ClaimProfileRow{}, stored: map[string]claims.Facts{},
		settings: &zitadel.TokenSettings{AccessTokenType: "JWT", IDTokenUserinfoAssertion: true, ProjectRoleAssertion: true, ProjectRoleCheck: true}}

	origDir := directory.Default
	o1, o2, o3, o4, o5, o6, o7 := svcGetClaimProfile, svcUpsertClaimProfile, svcListAppClaimOverrides, svcLatestClaimShapes, svcClaimShapeAt, svcClaimShapeHistory, svcInsertClaimShape
	o8, o9, o10, o11, o12, o13 := svcClaimShapeSeedCutoff, svcGetClaimFailureMode, svcStoredFacts, svcCompileFacts, svcTokenSettings, svcApplications
	origReach := reachOf
	reachOf = func(_ context.Context, _ string, changes []claims.Change) *Reach {
		if len(changes) == 0 {
			return nil
		}
		return &Reach{ByRole: map[string]int{"staff": 12}, People: 23}
	}
	t.Cleanup(func() {
		reachOf = origReach
		directory.Default = origDir
		svcGetClaimProfile, svcUpsertClaimProfile, svcListAppClaimOverrides, svcLatestClaimShapes, svcClaimShapeAt, svcClaimShapeHistory, svcInsertClaimShape = o1, o2, o3, o4, o5, o6, o7
		svcClaimShapeSeedCutoff, svcGetClaimFailureMode, svcStoredFacts, svcCompileFacts, svcTokenSettings, svcApplications = o8, o9, o10, o11, o12, o13
		lastSettings = map[string]tokenSettingsRead{}
	})
	directory.Default = shapeDir{}

	svcGetClaimProfile = func(_ context.Context, pid string) (db.ClaimProfileRow, bool, error) {
		r, ok := s.defaults[pid]
		return r, ok, nil
	}
	svcUpsertClaimProfile = func(_ context.Context, r db.ClaimProfileRow) error { s.defaults[r.ProjectID] = r; return nil }
	svcListAppClaimOverrides = func(context.Context) ([]db.AppClaimOverrideRow, error) { return s.overrides, nil }
	svcLatestClaimShapes = func(_ context.Context, pid string) ([]db.ClaimShapeVersion, error) {
		var out []db.ClaimShapeVersion
		for _, vs := range s.versions {
			if l := vs[len(vs)-1]; l.ProjectID == pid {
				out = append(out, l)
			}
		}
		return out, nil
	}
	svcClaimShapeAt = func(_ context.Context, app string, n int) (db.ClaimShapeVersion, bool, error) {
		vs := s.versions[app]
		if len(vs) == 0 || n > len(vs) {
			return db.ClaimShapeVersion{}, false, nil
		}
		if n == 0 {
			n = len(vs)
		}
		return vs[n-1], true, nil
	}
	svcClaimShapeHistory = func(_ context.Context, app string) ([]db.ClaimShapeVersion, error) {
		vs := s.versions[app]
		out := make([]db.ClaimShapeVersion, len(vs))
		for i := range vs {
			out[i] = vs[len(vs)-1-i]
		}
		return out, nil
	}
	svcInsertClaimShape = func(_ context.Context, v db.ClaimShapeVersion, onlyIfNone bool) (db.ClaimShapeVersion, bool, error) {
		if onlyIfNone && len(s.versions[v.ApplicationID]) > 0 {
			return db.ClaimShapeVersion{}, false, nil
		}
		// A round trip through JSON, as the JSONB column does.
		b, _ := json.Marshal(v.Profile)
		v.Profile = claims.Profile{}
		_ = json.Unmarshal(b, &v.Profile)
		v.Version = len(s.versions[v.ApplicationID]) + 1
		s.versions[v.ApplicationID] = append(s.versions[v.ApplicationID], v)
		return v, true, nil
	}
	svcClaimShapeSeedCutoff = func(context.Context) (time.Time, error) { return seedCutoff, nil }
	svcGetClaimFailureMode = func(context.Context, string) (string, map[string]interface{}, error) { return "fail_closed", nil, nil }
	svcStoredFacts = func(_ context.Context, uid, pid string) (claims.Facts, bool, error) {
		f, ok := s.stored[uid+":"+pid]
		return f, ok, nil
	}
	svcCompileFacts = func(_ context.Context, uid, pid string) (claims.Facts, error) {
		return claims.Facts{UserID: uid, ProjectID: pid, Roles: []string{"staff"}}, nil
	}
	svcTokenSettings = func(context.Context, string, string) (*zitadel.TokenSettings, error) {
		if s.settings == nil {
			return nil, errors.New("zitadel isn't answering")
		}
		cp := *s.settings
		return &cp, nil
	}
	svcApplications = func(context.Context) ([]models.ApplicationCatalog, error) { return shapeApps, nil }
	return s
}

func netbirdShape() claims.Profile {
	t, _ := claims.FindTemplate("netbird")
	p := t.Profile
	p.Roles = []claims.RoleMap{{Key: "staff", As: "netbird-staff", Included: true}, {Key: "admin", As: "netbird-admins", Included: true}}
	return p
}

// The data plane's read: each app's latest shape by client id, the default
// for everyone else — built-in when the project has none.
func TestResolveProjectShapes(t *testing.T) {
	s := stubClaimShapeDeps(t)
	ps, err := ResolveProjectShapes(context.Background(), "pNet")
	if err != nil || ps.DefaultSource != SourceBuiltin || !reflect.DeepEqual(ps.Default, claims.Builtin()) {
		t.Fatalf("no default set: %+v err=%v", ps, err)
	}
	if _, err := SaveAppClaimShape(context.Background(), "aNB", "op", SaveClaimShapeRequest{Profile: netbirdShape(), TemplateID: "netbird", AcknowledgeBreaking: true}, 0); err != nil {
		t.Fatal(err)
	}
	s.defaults["pNet"] = db.ClaimProfileRow{ProjectID: "pNet", ClaimName: "net.roles", FormatType: "array"}
	ps, _ = ResolveProjectShapes(context.Background(), "pNet")
	if got, own := ps.For("cNB"); !own || got.ApplicationID != "aNB" || got.Version != 1 {
		t.Fatalf("NetBird's own shape: %+v own=%v", got, own)
	}
	if got, own := ps.For("unknown-client"); own || got.Profile.Claims[0].Path[0] != "net.roles" {
		t.Fatalf("an unknown client gets the project default: %+v", got)
	}
	if _, own := ps.For(""); own {
		t.Fatal("no client id: the project default")
	}
}

// An app that existed under Actions v1 is seeded on first read with exactly
// v1's shape; one made after the cutoff starts on its project's default. An
// old per-project override rides along.
func TestGetAppClaimShape_SeedsOnlyAppsFromBeforeTheCutoff(t *testing.T) {
	s := stubClaimShapeDeps(t)
	s.overrides = []db.AppClaimOverrideRow{{ApplicationID: "aInv", ProjectID: "pInv", ClaimName: "inv.roles", FormatType: "csv"}}
	ctx := context.Background()

	nb, seeded, err := GetAppClaimShape(ctx, "aNB")
	if err != nil || !seeded || nb.Source != SourceApp || nb.Version.Note != SeedNote || nb.Version.SavedBy != "system" {
		t.Fatalf("NetBird seeded: %+v seeded=%v err=%v", nb.Version, seeded, err)
	}
	if !reflect.DeepEqual(nb.Profile, claims.ActionsV1()) || nb.Template.ID != "custom" {
		t.Fatalf("seed is the v1 shape on the Custom template: %+v", nb.Profile)
	}
	if _, again, _ := GetAppClaimShape(ctx, "aNB"); again {
		t.Fatal("a seeded app is never seeded twice")
	}

	inv, _, _ := GetAppClaimShape(ctx, "aInv") // no creation date: treated as existing
	if len(inv.Profile.Claims) != 6 || inv.Profile.Claims[5].Path[0] != "inv.roles" || inv.Version.Note == SeedNote {
		t.Fatalf("override folded into the seed: %+v note=%q", inv.Profile.Claims, inv.Version.Note)
	}

	gl, seeded, _ := GetAppClaimShape(ctx, "aGL")
	if seeded || gl.Source != SourceBuiltin || gl.Version != nil || gl.SuggestedTemplate != "gitlab" {
		t.Fatalf("GitLab is new: %+v seeded=%v", gl, seeded)
	}
	if _, err := SeedClaimShapes(ctx); err != nil || len(s.versions["aGL"]) != 0 {
		t.Fatalf("the hourly seed leaves new apps alone: err=%v", err)
	}
}

// A save the app will act on needs acknowledge_breaking; the refusal says
// what breaks. Custom never refuses. Role keys must be this project's.
func TestSaveAppClaimShape_Breaking(t *testing.T) {
	stubClaimShapeDeps(t)
	ctx := context.Background()
	if _, err := SaveAppClaimShape(ctx, "aNB", "op", SaveClaimShapeRequest{Profile: netbirdShape(), TemplateID: "netbird", AcknowledgeBreaking: true}, 0); err != nil {
		t.Fatal(err)
	}

	renamed := netbirdShape()
	renamed.Roles[0].As = "network-staff"
	_, err := SaveAppClaimShape(ctx, "aNB", "op", SaveClaimShapeRequest{Profile: renamed, TemplateID: "netbird"}, 0)
	var br *BreakingUnacknowledged
	if !errors.As(err, &br) || len(br.Breaking) != 1 || br.Breaking[0].From != "netbird-staff" || br.Breaking[0].To != "network-staff" ||
		br.Reach == nil || br.Reach.ByRole["staff"] != 12 {
		t.Fatalf("want a refused rename, got %v", err)
	}
	res, err := SaveAppClaimShape(ctx, "aNB", "op", SaveClaimShapeRequest{Profile: renamed, TemplateID: "netbird", AcknowledgeBreaking: true, Note: " renamed "}, 0)
	if err != nil || res.Version.Version != 2 || res.Version.Note != "renamed" || res.Version.TemplateVersion != 1 {
		t.Fatalf("acknowledged: %+v err=%v", res, err)
	}

	// The rule is the app's: a Custom shape on NetBird still breaks NetBird.
	custom := netbirdShape()
	custom.Roles[0].As = "stf"
	if _, err := SaveAppClaimShape(ctx, "aNB", "op", SaveClaimShapeRequest{Profile: custom, TemplateID: "custom"}, 0); !errors.As(err, &br) {
		t.Fatalf("NetBird acts on a rename whatever template the shape names: %v", err)
	}
	// An in-house app on Custom: every rename is said plainly, never refused.
	shape := claims.Profile{Claims: []claims.Claim{{Path: []string{"roles"}, Kind: claims.KindRoles}}, Roles: []claims.RoleMap{{Key: "staff", Included: true}}}
	for _, as := range []string{"", "stf"} {
		shape.Roles[0].As = as
		if _, err := SaveAppClaimShape(ctx, "aInv", "op", SaveClaimShapeRequest{Profile: shape, TemplateID: "custom"}, 0); err != nil {
			t.Fatalf("custom renames are never refused: %v", err)
		}
	}

	bad := netbirdShape()
	bad.Roles = append(bad.Roles, claims.RoleMap{Key: "inv-admin", Included: true})
	var inv *ClaimShapeInvalid
	if _, err := SaveAppClaimShape(ctx, "aNB", "op", SaveClaimShapeRequest{Profile: bad, AcknowledgeBreaking: true}, 0); !errors.As(err, &inv) {
		t.Fatalf("a role from another project is refused, got %v", err)
	}
	if _, err := SaveAppClaimShape(ctx, "nope", "op", SaveClaimShapeRequest{Profile: bad}, 0); !errors.Is(err, ErrClaimShapeNotFound) {
		t.Fatalf("unknown app: %v", err)
	}
}

// Go back saves the old version as a new one, through the same checks.
func TestRestoreAndHistory(t *testing.T) {
	s := stubClaimShapeDeps(t)
	ctx := context.Background()
	if _, _, err := GetAppClaimShape(ctx, "aNB"); err != nil { // v1: the seed
		t.Fatal(err)
	}
	if _, err := SaveAppClaimShape(ctx, "aNB", "op", SaveClaimShapeRequest{Profile: netbirdShape(), TemplateID: "netbird", AcknowledgeBreaking: true}, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := RestoreAppClaimShape(ctx, "aNB", "op", 1, false); err == nil {
		t.Fatal("going back from NetBird's shape drops `groups` words: breaking, needs the tick")
	}
	res, err := RestoreAppClaimShape(ctx, "aNB", "op", 1, true)
	if err != nil || res.Version.Version != 3 || res.Version.RestoredFrom != 1 || !reflect.DeepEqual(res.Version.Profile, claims.ActionsV1()) {
		t.Fatalf("restore: %+v err=%v", res.Version, err)
	}
	if _, err := RestoreAppClaimShape(ctx, "aNB", "op", 9, true); !errors.Is(err, ErrClaimShapeNotFound) {
		t.Fatalf("unknown version: %v", err)
	}
	hist, err := AppClaimShapeHistory(ctx, "aNB")
	if err != nil || len(hist) != 3 || hist[0].Version != 3 || len(hist[2].Changes) != 0 || len(hist[1].Changes) == 0 {
		t.Fatalf("history: %+v err=%v", hist, err)
	}
	_ = s
}

// Copying from another project copies role words only for keys this project
// has, lists the rest, and flags a role-dependent row that lost its role.
func TestDraftFrom(t *testing.T) {
	stubClaimShapeDeps(t)
	ctx := context.Background()
	src := claims.Profile{
		Claims: []claims.Claim{
			{Path: []string{"roles"}, Kind: claims.KindRoles, Format: claims.FormatArray},
			{Path: []string{"is_admin"}, Kind: claims.KindIfRole, Roles: []string{"inv-admin"}},
		},
		Roles:      []claims.RoleMap{{Key: "staff", As: "stf", Included: true}, {Key: "inv-admin", Included: true}, {Key: "trained", Included: false}},
		IncludeNew: true, Lands: claims.LandsBoth,
	}
	if _, err := SaveAppClaimShape(ctx, "aInv", "op", SaveClaimShapeRequest{Profile: src, TemplateID: "custom"}, 0); err != nil {
		t.Fatal(err)
	}

	d, err := DraftFrom(ctx, "aNB", DraftFromRequest{ApplicationID: "aInv"})
	if err != nil {
		t.Fatal(err)
	}
	if d.CopiedFrom == nil || d.CopiedFrom.ApplicationID != "aInv" || d.CopiedFrom.Version != 1 || d.TemplateID != "custom" {
		t.Fatalf("copied from: %+v", d)
	}
	if !reflect.DeepEqual(d.Profile.Roles, []claims.RoleMap{{Key: "staff", As: "stf", Included: true}}) {
		t.Fatalf("only this project's keys copy: %+v", d.Profile.Roles)
	}
	if !reflect.DeepEqual(d.NotCopied, []RoleRef{{RoleKey: "inv-admin", Name: "Admin"}, {RoleKey: "trained", Name: "Trained"}}) {
		t.Fatalf("not copied: %+v", d.NotCopied)
	}
	if len(d.MissingRoles) != 1 || d.MissingRoles[0].Row != 1 || len(d.Profile.Claims[1].Roles) != 0 {
		t.Fatalf("missing role flagged and dropped: %+v", d)
	}
	// The flagged row stops the save until a role from here is chosen.
	if _, err := SaveAppClaimShape(ctx, "aNB", "op", SaveClaimShapeRequest{Profile: d.Profile, AcknowledgeBreaking: true}, 0); err == nil {
		t.Fatal("a row with no role must not save")
	}

	tpl, err := DraftFrom(ctx, "aGL", DraftFromRequest{TemplateID: "gitlab"})
	if err != nil || tpl.TemplateID != "gitlab" || tpl.CopiedFrom != nil {
		t.Fatalf("template draft: %+v err=%v", tpl, err)
	}
	// Role words are always the operator's: on the built-in default every role
	// is included in Syndra's words, and the draft says so row by row.
	if !reflect.DeepEqual(tpl.Profile.Roles, []claims.RoleMap{{Key: "users", Included: true}}) {
		t.Fatalf("template draft roles: %+v", tpl.Profile.Roles)
	}
	if _, err := DraftFrom(ctx, "aNB", DraftFromRequest{}); err == nil {
		t.Fatal("exactly one source")
	}
	if _, err := DraftFrom(ctx, "aNB", DraftFromRequest{ApplicationID: "aGL"}); !errors.Is(err, ErrClaimShapeNotFound) {
		t.Fatalf("an app with no shape of its own can't be copied: %v", err)
	}
}

// The five checks, live and for a draft, through the one shaper.
func TestPreview_Checks(t *testing.T) {
	s := stubClaimShapeDeps(t)
	ctx := context.Background()
	if _, err := SaveAppClaimShape(ctx, "aNB", "op", SaveClaimShapeRequest{Profile: netbirdShape(), TemplateID: "netbird", AcknowledgeBreaking: true}, 0); err != nil {
		t.Fatal(err)
	}
	s.stored["ada:pNet"] = claims.Facts{UserID: "ada", ProjectID: "pNet", Roles: []string{"admin", "staff"}, CompiledAt: "2026-11-03T09:52:00Z"}

	renamed := netbirdShape()
	renamed.Roles[0].As = "network-staff"
	pv, err := PreviewAppClaimShape(ctx, "aNB", PreviewRequest{UserIDs: []string{"ada", "rohan"}, Draft: &renamed})
	if err != nil {
		t.Fatal(err)
	}
	ada := pv.People[0]
	for i, c := range ada.Now.Checks {
		if c.State != CheckPass {
			t.Fatalf("check %d for Ada: %+v", i+1, c)
		}
	}
	tok := func(o any) string { b, _ := json.Marshal(o); return string(b) }
	if got := tok(ada.Now.Token); got != `{"groups":["netbird-staff","netbird-admins"]}` {
		t.Fatalf("now: %s", got)
	}
	if got := tok(ada.Draft.Token); got != `{"groups":["network-staff","netbird-admins"]}` {
		t.Fatalf("draft: %s", got)
	}
	if !ada.Draft.Checks[3].Facts["draft"].(bool) || len(pv.Breaking) != 1 {
		t.Fatalf("draft check 4 and breaking: %+v %+v", ada.Draft.Checks[3], pv.Breaking)
	}

	// Rohan's claims were never built: check 3 is amber, and the token is what
	// the token path really sends (fail closed: nothing), with what he would get.
	rohan := pv.People[1]
	if rohan.Now.Checks[2].State != CheckWarn || rohan.Now.Checks[4].State != CheckFail || tok(rohan.Now.Token) != `{}` ||
		tok(rohan.Now.WouldReceive) != `{"groups":["netbird-staff"]}` {
		t.Fatalf("not built: %+v token=%s", rohan.Now.Checks, tok(rohan.Now.Token))
	}

	// A Bearer access token keeps NetBird's claims out of where it reads them.
	s.settings.AccessTokenType = "BEARER"
	pv, _ = PreviewAppClaimShape(ctx, "aNB", PreviewRequest{UserIDs: []string{"ada"}})
	if c := pv.People[0].Now.Checks[0]; c.State != CheckFail || c.Facts["reached"].(map[string]bool)[claims.ReadsAccessToken] {
		t.Fatalf("lands-fail: %+v", c)
	}
	if pv.People[0].Now.Checks[4].State != CheckFail {
		t.Fatal("check 5 turns red with check 1")
	}

	// Zitadel stops answering: its last answer, marked stale.
	s.settings.AccessTokenType = "JWT"
	_, _ = PreviewAppClaimShape(ctx, "aNB", PreviewRequest{UserIDs: []string{"ada"}})
	s.settings = nil
	pv, _ = PreviewAppClaimShape(ctx, "aNB", PreviewRequest{UserIDs: []string{"ada"}})
	if c := pv.People[0].Now.Checks[0]; c.State != CheckStale || c.Facts["read_at"] == nil {
		t.Fatalf("degraded check 1: %+v", c)
	}
	if _, err := PreviewAppClaimShape(ctx, "aNB", PreviewRequest{}); err == nil {
		t.Fatal("at least one person")
	}
}

// An app on its project's default whose name suggests a template that sends
// something else gets an amber check 4: the reason to pick a template.
func TestPreview_DefaultTheAppDoesNotRead(t *testing.T) {
	s := stubClaimShapeDeps(t)
	s.stored["ada:pCode"] = claims.Facts{UserID: "ada", ProjectID: "pCode", Roles: []string{"users"}, CompiledAt: "x"}
	pv, err := PreviewAppClaimShape(context.Background(), "aGL", PreviewRequest{UserIDs: []string{"ada"}})
	if err != nil {
		t.Fatal(err)
	}
	c4 := pv.People[0].Now.Checks[3]
	if c4.State != CheckWarn || c4.Facts["suggested_template"] != "gitlab" || c4.Facts["source"] != SourceBuiltin {
		t.Fatalf("check 4: %+v", c4)
	}
}

func TestProjectClaimShape(t *testing.T) {
	s := stubClaimShapeDeps(t)
	ctx := context.Background()
	if err := SaveProjectClaimProfile(ctx, "pNet", ProjectDefaultForm{ClaimName: "net.roles", AttributeClaims: map[string]string{"net.roles": claims.AttrEmail}}); err == nil {
		t.Fatal("a repeated key is refused")
	}
	if err := SaveProjectClaimProfile(ctx, "pNet", ProjectDefaultForm{ClaimName: "net.roles", AttributeClaims: map[string]string{"mail": claims.AttrEmail}}); err != nil {
		t.Fatal(err)
	}
	if s.defaults["pNet"].FormatType != claims.FormatArray {
		t.Fatalf("format defaults to array: %+v", s.defaults["pNet"])
	}
	if _, _, err := GetAppClaimShape(ctx, "aNB"); err != nil {
		t.Fatal(err)
	}
	view, err := GetProjectClaimShape(ctx, "pNet")
	if err != nil || !view.Configured || len(view.Applications) != 1 || view.Applications[0].Source != SourceApp ||
		len(view.Profile.Claims) != 2 || view.Overrides == nil {
		t.Fatalf("project view: %+v err=%v", view, err)
	}
}
