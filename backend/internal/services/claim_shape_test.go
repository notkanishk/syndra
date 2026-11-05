package services

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"syndra/internal/claims"
	"syndra/internal/claimtel"
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

// FindUser knows nobody: the facts' identity stands (the equivalence test
// draws Zitadel's apart).
func (shapeDir) FindUser(context.Context, string) (models.UserProfile, bool, error) {
	return models.UserProfile{}, false, nil
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
	versions map[string][]db.ClaimShapeVersion
	defaults map[string]db.ClaimProfileRow
	stored   map[string]claims.Facts // user:project -> stored facts
	observed map[string][]db.ObservedGrant
	settings *zitadel.TokenSettings
}

func stubClaimShapeDeps(t *testing.T) *shapeStore {
	t.Helper()
	s := &shapeStore{versions: map[string][]db.ClaimShapeVersion{}, defaults: map[string]db.ClaimProfileRow{}, stored: map[string]claims.Facts{},
		settings: &zitadel.TokenSettings{AccessTokenType: "JWT", IDTokenUserinfoAssertion: true, ProjectRoleAssertion: true, ProjectRoleCheck: true}}

	origDir := directory.Default
	o1, o2, o4, o5, o6, o7 := svcGetClaimProfile, svcUpsertClaimProfile, svcLatestClaimShapes, svcClaimShapeAt, svcClaimShapeHistory, svcInsertClaimShape
	o8, o9, o10, o11, o12, o13 := svcClaimShapeSeedCutoff, svcObservedGrantsFor, svcStoredFacts, svcCompileFacts, svcTokenSettings, svcApplications
	origReach := reachOf
	o14, o15, o16 := svcClaimDeliveryLast, svcLastSignInPerApp, svcLatestOrgObservation
	recent := time.Now().Add(-5 * time.Minute)
	svcClaimDeliveryLast = func(context.Context) ([]db.ClaimDeliveryLastRow, error) {
		return []db.ClaimDeliveryLastRow{{ClientID: "cNB", Function: "function/preaccesstoken", LastAt: recent, LastOutcome: "served", LastServedAt: &recent}}, nil
	}
	svcLastSignInPerApp = func(context.Context) (map[string]time.Time, error) { return map[string]time.Time{}, nil }
	reachOf = func(_ context.Context, _ string, changes []claims.Change) *Reach {
		if len(changes) == 0 {
			return nil
		}
		return &Reach{ByRole: map[string]int{"staff": 12}, People: 23}
	}
	t.Cleanup(func() {
		reachOf = origReach
		svcClaimDeliveryLast, svcLastSignInPerApp, svcLatestOrgObservation = o14, o15, o16
		directory.Default = origDir
		svcGetClaimProfile, svcUpsertClaimProfile, svcLatestClaimShapes, svcClaimShapeAt, svcClaimShapeHistory, svcInsertClaimShape = o1, o2, o4, o5, o6, o7
		svcClaimShapeSeedCutoff, svcObservedGrantsFor, svcStoredFacts, svcCompileFacts, svcTokenSettings, svcApplications = o8, o9, o10, o11, o12, o13
		lastSettings = map[string]tokenSettingsRead{}
	})
	directory.Default = shapeDir{}

	svcGetClaimProfile = func(_ context.Context, pid string) (db.ClaimProfileRow, bool, error) {
		r, ok := s.defaults[pid]
		return r, ok, nil
	}
	svcUpsertClaimProfile = func(_ context.Context, r db.ClaimProfileRow) error { s.defaults[r.ProjectID] = r; return nil }
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
	// Zitadel, as last observed: s.observed when a test sets it, else the
	// roles of each person's stored facts (Zitadel holds what Syndra gave).
	svcObservedGrantsFor = func(_ context.Context, uid string) ([]db.ObservedGrant, error) {
		if s.observed != nil {
			return s.observed[uid], nil
		}
		var out []db.ObservedGrant
		for k, f := range s.stored {
			if strings.HasPrefix(k, uid+":") {
				out = append(out, db.ObservedGrant{UserID: uid, ProjectID: f.ProjectID, RoleKeys: f.Roles})
			}
		}
		return out, nil
	}
	svcLatestOrgObservation = func(context.Context) (db.Observation, error) {
		return db.Observation{Scope: "org", ObservedAt: time.Now(), Complete: true}, nil
	}
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
// v1's shape; one made after the cutoff starts on its project's default.
func TestGetAppClaimShape_SeedsOnlyAppsFromBeforeTheCutoff(t *testing.T) {
	s := stubClaimShapeDeps(t)
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
	if !reflect.DeepEqual(inv.Profile, claims.ActionsV1()) || inv.Version.Note != SeedNote {
		t.Fatalf("an app with no date is seeded with exactly v1: %+v note=%q", inv.Profile.Claims, inv.Version.Note)
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
	s.observed = map[string][]db.ObservedGrant{
		"ada":   {{UserID: "ada", ProjectID: "pNet", RoleKeys: []string{"staff", "admin"}}},
		"rohan": {{UserID: "rohan", ProjectID: "pNet", RoleKeys: []string{"staff"}}},
	}

	renamed := netbirdShape()
	renamed.Roles[0].As = "network-staff"
	// And a row only facts fill, so a person not built is short of it.
	renamed.Claims = append(renamed.Claims, claims.Claim{Path: []string{"team"}, Kind: claims.KindAttribute, Source: claims.AttrTeam})
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
	// the token path really sends (the shape over Zitadel's roles and identity
	// alone). The live shape needs no facts, so it is whole: check 5 passes, as
	// live counts it served. The draft asks for his team: amber, with what he
	// would get.
	rohan := pv.People[1]
	if rohan.Now.Checks[2].State != CheckWarn || rohan.Now.Checks[4].State != CheckPass || tok(rohan.Now.Token) != `{"groups":["netbird-staff"]}` ||
		rohan.Now.WouldReceive != nil {
		t.Fatalf("not built: %+v token=%s", rohan.Now.Checks, tok(rohan.Now.Token))
	}
	if rohan.Draft.Checks[4].State != CheckWarn || tok(rohan.Draft.Token) != `{"groups":["network-staff"]}` || rohan.Draft.WouldReceive == nil {
		t.Fatalf("not built, a fact asked for: %+v token=%s", rohan.Draft.Checks[4], tok(rohan.Draft.Token))
	}

	// The roles are Zitadel's, never Syndra's records: admin, still queued
	// for Ada in Zitadel, is not previewed though her facts hold it; and a
	// failed read of Zitadel's grants says so rather than showing none.
	s.observed["ada"] = []db.ObservedGrant{{UserID: "ada", ProjectID: "pNet", RoleKeys: []string{"staff"}}}
	pv, _ = PreviewAppClaimShape(ctx, "aNB", PreviewRequest{UserIDs: []string{"ada"}})
	if now := pv.People[0].Now; tok(now.Token) != `{"groups":["netbird-staff"]}` || tok(now.Checks[1].Facts["roles"]) != `["staff"]` {
		t.Fatalf("queued role previewed: %s %+v", tok(now.Token), now.Checks[1])
	}
	orig := svcObservedGrantsFor
	svcObservedGrantsFor = func(context.Context, string) ([]db.ObservedGrant, error) { return nil, errors.New("down") }
	pv, _ = PreviewAppClaimShape(ctx, "aNB", PreviewRequest{UserIDs: []string{"ada"}})
	svcObservedGrantsFor = orig
	if c := pv.People[0].Now.Checks[1]; c.State != CheckStale || c.Facts["unread"] != true {
		t.Fatalf("unread grants: %+v", c)
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

// Zitadel lists a project in the token only for someone it holds a grant for
// there. With none, the door answers nothing, so the preview does too and
// says why; a grant with no roles still lists the project.
func TestPreview_ZitadelDoesNotListTheProject(t *testing.T) {
	s := stubClaimShapeDeps(t)
	ctx := context.Background()
	if _, err := SaveAppClaimShape(ctx, "aNB", "op", SaveClaimShapeRequest{Profile: netbirdShape(), TemplateID: "netbird", AcknowledgeBreaking: true}, 0); err != nil {
		t.Fatal(err)
	}
	s.observed = map[string][]db.ObservedGrant{
		"grace": {{UserID: "grace", ProjectID: "pElse", RoleKeys: []string{"staff"}}},
		"aanya": {{UserID: "aanya", ProjectID: "pNet"}},
	}
	pv, err := PreviewAppClaimShape(ctx, "aNB", PreviewRequest{UserIDs: []string{"grace", "aanya"}})
	if err != nil {
		t.Fatal(err)
	}
	tok := func(o any) string { b, _ := json.Marshal(o); return string(b) }
	grace := pv.People[0].Now
	if c5 := grace.Checks[4]; c5.State != CheckNone || c5.Facts["unlisted"] != true || tok(grace.Token) != `{}` || grace.WouldReceive != nil {
		t.Fatalf("unlisted: %+v token=%s", c5, tok(grace.Token))
	}
	aanya := pv.People[1].Now
	if aanya.Checks[4].Facts["unlisted"] != nil || tok(aanya.Token) != `{"groups":[]}` {
		t.Fatalf("a grant with no roles lists the project: %+v token=%s", aanya.Checks[4], tok(aanya.Token))
	}
}

// Without application.client_id the data plane can't tell the app, so an app
// with its own shape gets the project's default: check 4 says so, from the
// newest call per step. A call that names the app means Zitadel sends it.
func TestPreview_ClientIDMissing(t *testing.T) {
	s := stubClaimShapeDeps(t)
	ctx := context.Background()
	nb := netbirdShape()
	nb.Lands = claims.LandsAccessToken
	if _, err := SaveAppClaimShape(ctx, "aNB", "op", SaveClaimShapeRequest{Profile: nb, TemplateID: "netbird", AcknowledgeBreaking: true}, 0); err != nil {
		t.Fatal(err)
	}
	s.stored["ada:pNet"] = claims.Facts{UserID: "ada", ProjectID: "pNet", Roles: []string{"staff"}, CompiledAt: "x"}
	now := time.Now()
	older, newer := now.Add(-time.Hour), now.Add(-time.Minute)
	set := func(rows ...db.ClaimDeliveryLastRow) {
		svcClaimDeliveryLast = func(context.Context) ([]db.ClaimDeliveryLastRow, error) { return rows, nil }
	}
	c4 := func() Check {
		pv, err := PreviewAppClaimShape(ctx, "aNB", PreviewRequest{UserIDs: []string{"ada"}})
		if err != nil {
			t.Fatal(err)
		}
		return pv.People[0].Now.Checks[3]
	}

	set(db.ClaimDeliveryLastRow{ClientID: claimtel.None, Function: claims.FunctionPreAccessToken, LastAt: newer, LastOutcome: "served"},
		db.ClaimDeliveryLastRow{ClientID: "cNB", Function: claims.FunctionPreAccessToken, LastAt: older, LastOutcome: "served"})
	if c := c4(); c.State != CheckWarn || tok2(c.Facts["client_id_missing"]) != `["function/preaccesstoken"]` {
		t.Fatalf("missing: %+v", c)
	}

	set(db.ClaimDeliveryLastRow{ClientID: claimtel.None, Function: claims.FunctionPreAccessToken, LastAt: older, LastOutcome: "served"},
		db.ClaimDeliveryLastRow{ClientID: "cNB", Function: claims.FunctionPreAccessToken, LastAt: newer, LastOutcome: "served"})
	if c := c4(); c.State != CheckPass || c.Facts["client_id_missing"] != nil {
		t.Fatalf("present: %+v", c)
	}

	// Missing only at a step the shape doesn't land in: nothing changes.
	set(db.ClaimDeliveryLastRow{ClientID: claimtel.None, Function: claims.FunctionPreUserinfo, LastAt: newer, LastOutcome: "served"})
	if c := c4(); c.State != CheckPass {
		t.Fatalf("this shape lands in the access token only: %+v", c)
	}
	// A day old says nothing about now.
	set(db.ClaimDeliveryLastRow{ClientID: claimtel.None, Function: claims.FunctionPreAccessToken, LastAt: now.Add(-48 * time.Hour), LastOutcome: "served"})
	if c := c4(); c.State != CheckPass {
		t.Fatalf("old call: %+v", c)
	}
}

func tok2(o any) string { b, _ := json.Marshal(o); return string(b) }

// The live check: a captured answer equal to the preview matches, whatever a
// nested object's key order; a different one, one at a step the shape
// doesn't land in, or a degraded empty answer, does not.
func TestCompareServed(t *testing.T) {
	tokObj := claims.NewObject()
	tokObj.Set("groups", []any{"netbird-staff"})
	tokObj.Set("x", map[string]any{"a": 1, "b": 2})
	e := Explanation{Token: tokObj, Checks: []Check{{}, {}, {}, {}, {Facts: map[string]any{"lands": claims.LandsAccessToken}}}}
	at := time.Now()

	same := []byte(`{"append_claims":[{"key":"groups","value":["netbird-staff"]},{"key":"x","value":{"b":2,"a":1}}]}`)
	if lc := CompareServed(e, claims.FunctionPreAccessToken, same, at); !lc.Match || len(lc.Expected) != 2 {
		t.Fatalf("same: %+v", lc)
	}
	other := []byte(`{"append_claims":[{"key":"groups","value":["netbird-admins"]},{"key":"x","value":{"a":1,"b":2}}]}`)
	if lc := CompareServed(e, claims.FunctionPreAccessToken, other, at); lc.Match {
		t.Fatalf("a different role matched: %+v", lc)
	}
	reordered := []byte(`{"append_claims":[{"key":"x","value":{"a":1,"b":2}},{"key":"groups","value":["netbird-staff"]}]}`)
	if lc := CompareServed(e, claims.FunctionPreAccessToken, reordered, at); lc.Match {
		t.Fatal("the claims' order is the token's, and counts")
	}
	empty := []byte(`{"append_claims":[]}`)
	if lc := CompareServed(e, claims.FunctionPreAccessToken, empty, at); lc.Match {
		t.Fatal("a degraded empty answer matched a preview with claims")
	}
	// Userinfo: the shape lands in the access token only, so nothing is right.
	if lc := CompareServed(e, claims.FunctionPreUserinfo, empty, at); !lc.Match || len(lc.Expected) != 0 {
		t.Fatalf("not landing: %+v", lc)
	}
	if lc := CompareServed(e, claims.FunctionPreUserinfo, same, at); lc.Match {
		t.Fatal("claims served where the shape doesn't land matched")
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
		len(view.Profile.Claims) != 2 {
		t.Fatalf("project view: %+v err=%v", view, err)
	}
}

// The apps index's Claims column: each app's own key and template, its
// project's default for an app with none, and check 1's verdict only once
// Zitadel's settings for that app have been read.
func TestAttachClaimLines(t *testing.T) {
	s := stubClaimShapeDeps(t)
	ctx := context.Background()
	if _, err := SaveAppClaimShape(ctx, "aNB", "op", SaveClaimShapeRequest{Profile: netbirdShape(), TemplateID: "netbird", AcknowledgeBreaking: true}, 0); err != nil {
		t.Fatal(err)
	}
	views := func() []models.ApplicationView {
		v := []models.ApplicationView{{Application: shapeApps[0]}, {Application: shapeApps[2]}}
		attachClaimLines(ctx, v)
		return v
	}
	v := views()
	nb, gl := v[0].Claims, v[1].Claims
	if nb.Source != SourceApp || nb.TemplateID != "netbird" || nb.TemplateChanged || strings.Join(nb.Keys, ",") != "groups" || nb.Reaches != nil {
		t.Fatalf("NetBird before any read: %+v", nb)
	}
	if gl.Source != SourceBuiltin || strings.Join(gl.Keys, ",") != "roles" || gl.Lands != claims.LandsBoth {
		t.Fatalf("GitLab on the built-in default: %+v", gl)
	}

	// Opening NetBird's page reads Zitadel: a Bearer token keeps its claims out.
	s.settings.AccessTokenType = "BEARER"
	if _, err := PreviewAppClaimShape(ctx, "aNB", PreviewRequest{UserIDs: []string{"ada"}}); err != nil {
		t.Fatal(err)
	}
	if r := views()[0].Claims.Reaches; r == nil || *r {
		t.Fatalf("NetBird with a Bearer token: reaches=%v", r)
	}

	// Shapes that can't be read leave no line, rather than a guessed one.
	svcLatestClaimShapes = func(context.Context, string) ([]db.ClaimShapeVersion, error) { return nil, errors.New("db down") }
	if views()[0].Claims != nil {
		t.Fatal("unread shapes give no line")
	}
}

// Check 6: whether Zitadel reached Syndra for the app lately, from the data
// plane's last call and the app's last sign-in.
func TestReachedSyndra(t *testing.T) {
	o1, o2 := svcClaimDeliveryLast, svcLastSignInPerApp
	t.Cleanup(func() { svcClaimDeliveryLast, svcLastSignInPerApp = o1, o2 })
	app := models.ApplicationCatalog{ID: "aNB", ClientID: "cNB"}
	now := time.Now()
	ago := func(d time.Duration) *time.Time { at := now.Add(-d); return &at }
	for _, tc := range []struct {
		name    string
		last    *time.Time
		outcome string
		signIn  *time.Time
		readErr error
		want    string
	}{
		{"called and served", ago(time.Hour), "served", ago(time.Hour), nil, CheckPass},
		{"called, shape lands elsewhere", ago(time.Hour), "not_landing", nil, nil, CheckPass},
		{"called, no claims", ago(time.Hour), "degraded_no_facts", nil, nil, CheckWarn},
		{"signed in, never called", nil, "", ago(time.Hour), nil, CheckFail},
		{"signed in after the last call", ago(20 * time.Hour), "served", ago(time.Hour), nil, CheckPass},
		{"last call days ago, signed in since", ago(72 * time.Hour), "served", ago(time.Hour), nil, CheckFail},
		{"nobody lately", ago(72 * time.Hour), "served", ago(72 * time.Hour), nil, CheckNone},
		{"never anything", nil, "", nil, nil, CheckNone},
		{"telemetry unread", nil, "", nil, errors.New("down"), CheckStale},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svcClaimDeliveryLast = func(context.Context) ([]db.ClaimDeliveryLastRow, error) {
				if tc.readErr != nil {
					return nil, tc.readErr
				}
				if tc.last == nil {
					return []db.ClaimDeliveryLastRow{{ClientID: "other", LastAt: now}}, nil
				}
				return []db.ClaimDeliveryLastRow{{ClientID: "cNB", Function: "function/preuserinfo", LastAt: *tc.last, LastOutcome: tc.outcome}}, nil
			}
			svcLastSignInPerApp = func(context.Context) (map[string]time.Time, error) {
				if tc.signIn == nil {
					return map[string]time.Time{}, nil
				}
				return map[string]time.Time{"aNB": *tc.signIn}, nil
			}
			if got := reachedSyndra(context.Background(), app); got.State != tc.want {
				t.Fatalf("state = %s (%v), want %s", got.State, got.Facts, tc.want)
			}
		})
	}
}
