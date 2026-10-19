package services

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"syndra/internal/db"
	"syndra/internal/directory"
	"syndra/internal/models"
)

// insightsFixture is the whole org as the snapshot sees it: who holds what, by
// which reasons. Nothing here touches a database.
type insightsFixture struct {
	users      []models.UserProfile
	roles      map[string]map[roleKey]*models.EffectiveRole
	held       map[string][]models.Bundle
	rules      []models.MappingRule
	allBundles []models.Bundle
	latest     map[string][]models.BundleRole // absent = nothing published
	projs      []models.ProjectCatalog
	referenced [][2]string
	readAt     *time.Time
	collects   int
}

func newInsightsFixture() *insightsFixture {
	return &insightsFixture{
		roles:  map[string]map[roleKey]*models.EffectiveRole{},
		held:   map[string][]models.Bundle{},
		latest: map[string][]models.BundleRole{},
	}
}

func rk(p, r string) roleKey { return roleKey{projectID: p, roleKey: r} }

func byBundle(id, name string) models.RoleReason {
	return models.RoleReason{Kind: "bundle", BundleID: id, BundleName: name}
}
func byDirect() models.RoleReason { return models.RoleReason{Kind: "direct"} }
func byRule(sp, sr string) models.RoleReason {
	return models.RoleReason{Kind: "mapping", TriggerProject: sp, TriggerRole: sr}
}

// person adds one user holding the given roles, each by its reasons, and the
// bundles they hold (pinned at v1).
func (f *insightsFixture) person(id string, bundles []models.Bundle, holds map[roleKey][]models.RoleReason) {
	f.users = append(f.users, models.UserProfile{ID: id, Name: "Name " + id, Email: id + "@example.org"})
	m := map[roleKey]*models.EffectiveRole{}
	for k, rs := range holds {
		m[k] = &models.EffectiveRole{ProjectID: k.projectID, RoleKey: k.roleKey, Reasons: rs}
	}
	f.roles[id] = m
	for _, b := range bundles {
		b.PinnedVersion = 1
		f.held[id] = append(f.held[id], b)
	}
}

// crowd adds n people, each holding the same roles by the same reasons.
func (f *insightsFixture) crowd(prefix string, n int, bundles []models.Bundle, holds map[roleKey][]models.RoleReason) {
	for i := 0; i < n; i++ {
		f.person(fmt.Sprintf("%s%d", prefix, i), bundles, holds)
	}
}

func (f *insightsFixture) install(t *testing.T) {
	t.Helper()
	origDir, origCollect := directory.Default, collectUserRolesHook
	origRules, origBundles, origLatest := svcGetActiveMappingRules, svcGetAllBundles, svcLatestVersionRoles
	origLocal, origRefs, origObs := svcDbGetAllLocalRoles, svcDbGetAllReferencedRoleKeys, svcLatestOrgObservation
	t.Cleanup(func() {
		directory.Default, collectUserRolesHook = origDir, origCollect
		svcGetActiveMappingRules, svcGetAllBundles, svcLatestVersionRoles = origRules, origBundles, origLatest
		svcDbGetAllLocalRoles, svcDbGetAllReferencedRoleKeys, svcLatestOrgObservation = origLocal, origRefs, origObs
	})

	directory.Default = &snapshotFixtureDirectory{users: f.users, projs: f.projs}
	collectUserRolesHook = func(_ context.Context, id string) (map[roleKey]*models.EffectiveRole, []models.Bundle, error) {
		f.collects++
		return f.roles[id], f.held[id], nil
	}
	svcGetActiveMappingRules = func(context.Context) ([]models.MappingRule, error) { return f.rules, nil }
	svcGetAllBundles = func(context.Context) ([]models.Bundle, error) { return f.allBundles, nil }
	svcLatestVersionRoles = func(_ context.Context, id string) (models.BundleVersion, []models.BundleRole, error) {
		rs, ok := f.latest[id]
		if !ok {
			return models.BundleVersion{}, nil, fmt.Errorf("latest version: %w", pgx.ErrNoRows)
		}
		return models.BundleVersion{BundleID: id}, rs, nil
	}
	svcDbGetAllLocalRoles = func(context.Context) ([]models.Role, error) { return nil, nil }
	svcDbGetAllReferencedRoleKeys = func(context.Context) ([][2]string, error) { return f.referenced, nil }
	svcLatestOrgObservation = func(context.Context) (db.Observation, error) {
		if f.readAt == nil {
			return db.Observation{}, db.ErrNoObservation
		}
		return db.Observation{Scope: "org", ObservedAt: *f.readAt, Complete: true}, nil
	}
}

func (f *insightsFixture) findings(t *testing.T) models.InsightsFindings {
	t.Helper()
	f.install(t)
	got, err := InsightsFindings(context.Background())
	if err != nil {
		t.Fatalf("InsightsFindings: %v", err)
	}
	return got
}

var (
	bundleA = models.Bundle{ID: "ba", Name: "Alpha"}
	bundleB = models.Bundle{ID: "bb", Name: "Beta"}
	roleX   = rk("p1", "x")
)

// --- single routes ---

func TestInsightsFindings_SingleRouteAtThreshold(t *testing.T) {
	f := newInsightsFixture()
	// 9 of 10 hold x only through Alpha; the tenth holds it directly. 90% is in.
	f.crowd("a", 9, []models.Bundle{bundleA}, map[roleKey][]models.RoleReason{roleX: {byBundle("ba", "Alpha")}})
	f.person("d", nil, map[roleKey][]models.RoleReason{roleX: {byDirect()}})

	got := f.findings(t).SingleRoutes
	want := []models.InsightSingleRoute{{
		ProjectID: "p1", RoleKey: "x",
		Source:      models.InsightSource{Kind: "bundle", ID: "ba", Name: "Alpha"},
		OnlyThatWay: 9, Holders: 10,
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("single routes = %+v, want %+v", got, want)
	}
	if f.collects != len(f.users) {
		t.Fatalf("roles resolved %d times for %d people — the snapshot must read each once", f.collects, len(f.users))
	}
}

func TestInsightsFindings_SingleRouteJustUnderThresholdIsNotReported(t *testing.T) {
	f := newInsightsFixture()
	// 8 of 9 = 88.9%.
	f.crowd("a", 8, []models.Bundle{bundleA}, map[roleKey][]models.RoleReason{roleX: {byBundle("ba", "Alpha")}})
	f.person("d", nil, map[roleKey][]models.RoleReason{roleX: {byDirect()}})

	if got := f.findings(t).SingleRoutes; len(got) != 0 {
		t.Fatalf("88.9%% must not be a single route, got %+v", got)
	}
}

func TestInsightsFindings_RoleHeldTwoWaysIsNotASingleRoute(t *testing.T) {
	f := newInsightsFixture()
	// Everyone holds x through Alpha AND directly: there is no single route.
	f.crowd("a", 10, []models.Bundle{bundleA}, map[roleKey][]models.RoleReason{
		roleX: {byBundle("ba", "Alpha"), byDirect()},
	})

	if got := f.findings(t).SingleRoutes; len(got) != 0 {
		t.Fatalf("a role held two ways is not single-route, got %+v", got)
	}
}

func TestInsightsFindings_SingleRouteNeedsFiveHolders(t *testing.T) {
	f := newInsightsFixture()
	f.crowd("a", 4, []models.Bundle{bundleA}, map[roleKey][]models.RoleReason{roleX: {byBundle("ba", "Alpha")}})
	if got := f.findings(t).SingleRoutes; len(got) != 0 {
		t.Fatalf("4 holders is too few to be a finding, got %+v", got)
	}

	f = newInsightsFixture()
	f.crowd("a", 5, []models.Bundle{bundleA}, map[roleKey][]models.RoleReason{roleX: {byBundle("ba", "Alpha")}})
	if got := f.findings(t).SingleRoutes; len(got) != 1 {
		t.Fatalf("5 holders is enough, got %+v", got)
	}
}

func TestInsightsFindings_SingleRouteThroughARule(t *testing.T) {
	f := newInsightsFixture()
	f.rules = []models.MappingRule{{ID: "r1", SourceProject: "p0", SourceRole: "s", TargetProject: "p1", TargetRole: "x"}}
	f.crowd("a", 6, nil, map[roleKey][]models.RoleReason{
		rk("p0", "s"): {byDirect()},
		roleX:         {byRule("p0", "s")},
	})

	got := f.findings(t).SingleRoutes
	if len(got) != 1 || got[0].Source.Kind != "rule" || got[0].Source.ID != "r1" || got[0].RoleKey != "x" {
		t.Fatalf("want one rule-sourced single route for x via r1, got %+v", got)
	}
}

func TestInsightsFindings_RoleThroughIncludedBundleIsStillThatBundle(t *testing.T) {
	f := newInsightsFixture()
	via := byBundle("ba", "Alpha")
	via.ViaBundleID, via.ViaBundleName = "bb", "Beta"
	f.crowd("a", 5, []models.Bundle{bundleA}, map[roleKey][]models.RoleReason{roleX: {via}})

	got := f.findings(t).SingleRoutes
	if len(got) != 1 || got[0].Source.ID != "ba" {
		t.Fatalf("the route is the bundle the person holds (Alpha), got %+v", got)
	}
}

// --- unused ---

func TestInsightsFindings_UnusedRolesAndProjects(t *testing.T) {
	f := newInsightsFixture()
	f.projs = []models.ProjectCatalog{
		{ID: "p1", Name: "Laser", Roles: []models.ProjectRole{{Key: "x"}, {Key: "idle"}}},
		{ID: "p2", Name: "Kiln", Roles: []models.ProjectRole{{Key: "z"}}},
	}
	f.referenced = [][2]string{{"p3", "ghost"}}
	f.person("u", nil, map[roleKey][]models.RoleReason{roleX: {byDirect()}})

	got := f.findings(t).Unused
	wantRoles := []models.RoleGrant{{ProjectID: "p1", RoleKey: "idle"}, {ProjectID: "p2", RoleKey: "z"}, {ProjectID: "p3", RoleKey: "ghost"}}
	if !reflect.DeepEqual(got.Roles, wantRoles) {
		t.Fatalf("unused roles = %+v, want %+v", got.Roles, wantRoles)
	}
	wantProjects := []models.InsightProjectRef{{ProjectID: "p2", Name: "Kiln"}}
	if !reflect.DeepEqual(got.Projects, wantProjects) {
		t.Fatalf("unused projects = %+v, want %+v", got.Projects, wantProjects)
	}
}

func TestInsightsFindings_BundleThatGivesNothingNew(t *testing.T) {
	f := newInsightsFixture()
	// Alpha gives x, and both holders have x directly too: nothing new.
	f.person("u1", []models.Bundle{bundleA}, map[roleKey][]models.RoleReason{roleX: {byBundle("ba", "Alpha"), byDirect()}})
	f.person("u2", []models.Bundle{bundleA}, map[roleKey][]models.RoleReason{roleX: {byBundle("ba", "Alpha"), byDirect()}})
	// Beta gives y to one person, who holds nothing else: it gives something new.
	f.person("u3", []models.Bundle{bundleB}, map[roleKey][]models.RoleReason{rk("p1", "y"): {byBundle("bb", "Beta")}})
	f.person("u4", []models.Bundle{bundleB}, map[roleKey][]models.RoleReason{rk("p1", "y"): {byDirect(), byBundle("bb", "Beta")}})
	// Gamma has no holders: not reported.
	f.allBundles = []models.Bundle{bundleA, bundleB, {ID: "bg", Name: "Gamma"}}

	got := f.findings(t).Unused.Bundles
	want := []models.InsightUnusedBundle{{BundleID: "ba", Name: "Alpha", Holders: 2}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unused bundles = %+v, want %+v", got, want)
	}
}

func TestInsightsFindings_BundleGivingOneNewRoleIsNotUnused(t *testing.T) {
	f := newInsightsFixture()
	// Alpha gives x (also direct) and y (only here), to the same person.
	f.person("u1", []models.Bundle{bundleA}, map[roleKey][]models.RoleReason{
		roleX:         {byBundle("ba", "Alpha"), byDirect()},
		rk("p1", "y"): {byBundle("ba", "Alpha")},
	})
	if got := f.findings(t).Unused.Bundles; len(got) != 0 {
		t.Fatalf("one new role makes the bundle useful, got %+v", got)
	}
}

func TestInsightsFindings_ReadAt(t *testing.T) {
	f := newInsightsFixture()
	if got := f.findings(t).ReadAt; got != nil {
		t.Fatalf("never observed must be nil, got %v", got)
	}
	at := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	f = newInsightsFixture()
	f.readAt = &at
	if got := f.findings(t).ReadAt; got == nil || !got.Equal(at) {
		t.Fatalf("read_at = %v, want %v", got, at)
	}
}

func TestInsightsFindings_EmptyOrgSerialisesEmptyLists(t *testing.T) {
	got := newInsightsFixture().findings(t)
	if got.SingleRoutes == nil || got.Overlaps == nil || got.Unused.Roles == nil || got.Unused.Bundles == nil || got.Unused.Projects == nil {
		t.Fatalf("lists must be empty, not nil (JSON null): %+v", got)
	}
}

// --- overlaps ---

func br(p, r string) models.BundleRole { return models.BundleRole{ProjectID: p, RoleKey: r} }

func TestInsightsFindings_OverlapOfExactlyTwo(t *testing.T) {
	f := newInsightsFixture()
	f.allBundles = []models.Bundle{bundleB, bundleA, {ID: "bc", Name: "Gamma"}, {ID: "bd", Name: "Draft"}}
	f.latest["ba"] = []models.BundleRole{br("p1", "x"), br("p1", "y"), br("p1", "a")}
	f.latest["bb"] = []models.BundleRole{br("p1", "x"), br("p1", "y"), br("p1", "b")}
	f.latest["bc"] = []models.BundleRole{br("p1", "x"), br("p1", "c")} // shares only x with each: not reported
	// bd has nothing published: skipped, not an error.

	got := f.findings(t).Overlaps
	if len(got) != 1 {
		t.Fatalf("want exactly the Alpha/Beta pair, got %+v", got)
	}
	o := got[0]
	if o.A.BundleID != "ba" || o.B.BundleID != "bb" {
		t.Fatalf("pair order is by name: %+v", o)
	}
	if !reflect.DeepEqual(o.Shared, []models.RoleGrant{{ProjectID: "p1", RoleKey: "x"}, {ProjectID: "p1", RoleKey: "y"}}) ||
		!reflect.DeepEqual(o.AOnly, []models.RoleGrant{{ProjectID: "p1", RoleKey: "a"}}) ||
		!reflect.DeepEqual(o.BOnly, []models.RoleGrant{{ProjectID: "p1", RoleKey: "b"}}) {
		t.Fatalf("shared/a_only/b_only wrong: %+v", o)
	}
}

func TestInsightsFindings_OverlapOfOneIsNotReported(t *testing.T) {
	f := newInsightsFixture()
	f.allBundles = []models.Bundle{bundleA, bundleB}
	f.latest["ba"] = []models.BundleRole{br("p1", "x"), br("p1", "a")}
	f.latest["bb"] = []models.BundleRole{br("p1", "x"), br("p1", "b")}
	if got := f.findings(t).Overlaps; len(got) != 0 {
		t.Fatalf("one shared role is not an overlap, got %+v", got)
	}
}

func TestInsightsFindings_OverlapRealErrorSurfaces(t *testing.T) {
	f := newInsightsFixture()
	f.allBundles = []models.Bundle{bundleA}
	f.install(t)
	boom := errors.New("connection lost")
	svcLatestVersionRoles = func(context.Context, string) (models.BundleVersion, []models.BundleRole, error) {
		return models.BundleVersion{}, nil, boom
	}
	if _, err := InsightsFindings(context.Background()); !errors.Is(err, boom) {
		t.Fatalf("a real read failure must not read as 'no overlaps': %v", err)
	}
}

// --- who has X and not Y ---

func TestParseInsightRef(t *testing.T) {
	ok := map[string]InsightRef{
		"role:p1:x":           {Kind: "role", ProjectID: "p1", RoleKey: "x"},
		"role:123456:lab.use": {Kind: "role", ProjectID: "123456", RoleKey: "lab.use"},
		"bundle:b-1_2":        {Kind: "bundle", BundleID: "b-1_2"},
	}
	for in, want := range ok {
		got, err := ParseInsightRef(in)
		if err != nil || got != want {
			t.Errorf("ParseInsightRef(%q) = %+v, %v; want %+v", in, got, err, want)
		}
	}
	bad := []string{
		"", "role", "role:", "role:p1", "role:p1:", "role::x", "role:p1:x:y",
		"bundle", "bundle:", "bundle:a:b", "bundle: b1", "bundle:b 1",
		"user:u1", "Role:p1:x", "role:p1:x/../y", "bundle:b1\n",
	}
	for _, in := range bad {
		if _, err := ParseInsightRef(in); !errors.Is(err, ErrInvalidInsightRef) {
			t.Errorf("ParseInsightRef(%q) should be ErrInvalidInsightRef, got %v", in, err)
		}
	}
}

func TestInsightsWho_HasAndWithout(t *testing.T) {
	f := newInsightsFixture()
	f.person("u1", []models.Bundle{bundleA}, map[roleKey][]models.RoleReason{roleX: {byBundle("ba", "Alpha")}})
	f.person("u2", nil, map[roleKey][]models.RoleReason{roleX: {byDirect()}, rk("p1", "y"): {byDirect()}})
	f.person("u3", nil, map[roleKey][]models.RoleReason{rk("p1", "y"): {byDirect()}})
	f.install(t)

	// has x, without y -> u1 only, with how they hold x.
	got, err := InsightsWho(context.Background(), InsightRef{Kind: "role", ProjectID: "p1", RoleKey: "x"}, InsightRef{Kind: "role", ProjectID: "p1", RoleKey: "y"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.People) != 1 || got.People[0].UserID != "u1" || got.People[0].Email != "u1@example.org" || got.People[0].Name != "Name u1" {
		t.Fatalf("people = %+v", got.People)
	}
	if how := got.People[0].How; len(how) != 1 || how[0].Kind != "bundle" || how[0].BundleName != "Alpha" {
		t.Fatalf("how = %+v", how)
	}

	// No `without`: everyone with x, sorted by name.
	got, _ = InsightsWho(context.Background(), InsightRef{Kind: "role", ProjectID: "p1", RoleKey: "x"}, InsightRef{})
	if len(got.People) != 2 || got.People[0].UserID != "u1" || got.People[1].UserID != "u2" {
		t.Fatalf("people = %+v", got.People)
	}

	// has a bundle, without a role it gives: holds the bundle at any version.
	got, _ = InsightsWho(context.Background(), InsightRef{Kind: "bundle", BundleID: "ba"}, InsightRef{Kind: "role", ProjectID: "p1", RoleKey: "y"})
	if len(got.People) != 1 || got.People[0].UserID != "u1" || got.People[0].How[0].BundleID != "ba" {
		t.Fatalf("bundle ref: %+v", got.People)
	}

	// Nobody matches: an empty list, not null.
	got, _ = InsightsWho(context.Background(), InsightRef{Kind: "bundle", BundleID: "nope"}, InsightRef{})
	if got.People == nil || len(got.People) != 0 {
		t.Fatalf("want empty non-nil, got %#v", got.People)
	}
	if f.collects != len(f.users)*4 {
		// One snapshot per request, each person resolved once in it.
		t.Fatalf("roles resolved %d times for 4 requests over %d people", f.collects, len(f.users))
	}
}
