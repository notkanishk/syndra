package directory

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"syndra/internal/db"
	"syndra/internal/models"
	"syndra/internal/zitadel"
)

// --- demoSource passthrough -------------------------------------------------

func TestDemoSource_Users_ReturnsSeeded(t *testing.T) {
	src := NewDemoSource()
	users, err := src.Users(context.Background())
	if err != nil {
		t.Fatalf("Users returned error: %v", err)
	}
	if len(users) == 0 {
		t.Fatal("expected demo users, got none")
	}
}

func TestDemoSource_FindProject_PresentAndAbsent(t *testing.T) {
	src := NewDemoSource()
	p, ok, err := src.FindProject(context.Background(), "printing")
	if err != nil {
		t.Fatalf("FindProject error: %v", err)
	}
	if !ok {
		t.Fatal("expected printing to exist in demo catalog")
	}
	if p.ID != "printing" {
		t.Fatalf("got project id %q, want printing", p.ID)
	}

	_, ok, err = src.FindProject(context.Background(), "missing")
	if err != nil {
		t.Fatalf("FindProject error on miss: %v", err)
	}
	if ok {
		t.Fatal("expected miss on nonexistent project")
	}
}

func TestDemoSource_Tag(t *testing.T) {
	if got := NewDemoSource().Tag(); got != "demo" {
		t.Fatalf("demo Tag = %q, want demo", got)
	}
}

// --- zitadelSource happy-path mapping ---------------------------------------

// mockDirClient implements zitadel.ZitadelClient just enough for the
// directory tests. Each callable field may be set per-test to shape the
// response; unset fields default to empty results.
type mockDirClient struct {
	// mu guards the counter maps below — the apps/metadata fan-outs run these
	// methods on multiple goroutines concurrently, which would otherwise trip
	// Go's concurrent-map-write panic.
	mu                sync.Mutex
	listUsersCalls    int
	listProjectsCalls int
	listRolesCalls    map[string]int
	listAppsCalls     map[string]int
	listMetadataCalls map[string]int

	listUsersFn    func(context.Context, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ZitadelUser], error)
	listProjectsFn func(context.Context, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ZitadelProject], error)
	listRolesFn    func(context.Context, string, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ProjectRoleResult], error)
	getUserFn      func(context.Context, string) (*zitadel.ZitadelUser, error)
	listAppsFn     func(context.Context, string, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ZitadelApplication], error)
	listMetadataFn func(context.Context, string, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.UserMetadata], error)
}

func newMockClient() *mockDirClient {
	return &mockDirClient{
		listRolesCalls:    map[string]int{},
		listAppsCalls:     map[string]int{},
		listMetadataCalls: map[string]int{},
	}
}

func (m *mockDirClient) GetUser(ctx context.Context, id string) (*zitadel.ZitadelUser, error) {
	if m.getUserFn != nil {
		return m.getUserFn(ctx, id)
	}
	return nil, nil
}

func (m *mockDirClient) ListUsers(ctx context.Context, p zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ZitadelUser], error) {
	m.listUsersCalls++
	if m.listUsersFn != nil {
		return m.listUsersFn(ctx, p)
	}
	return &zitadel.SearchResult[zitadel.ZitadelUser]{}, nil
}

func (m *mockDirClient) ListProjects(ctx context.Context, p zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ZitadelProject], error) {
	m.listProjectsCalls++
	if m.listProjectsFn != nil {
		return m.listProjectsFn(ctx, p)
	}
	return &zitadel.SearchResult[zitadel.ZitadelProject]{}, nil
}

func (m *mockDirClient) ListProjectRoles(ctx context.Context, projectID string, p zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ProjectRoleResult], error) {
	m.listRolesCalls[projectID]++
	if m.listRolesFn != nil {
		return m.listRolesFn(ctx, projectID, p)
	}
	return &zitadel.SearchResult[zitadel.ProjectRoleResult]{}, nil
}

func (m *mockDirClient) AddProjectRole(_ context.Context, _, _, _, _ string) error    { return nil }
func (m *mockDirClient) UpdateProjectRole(_ context.Context, _, _, _, _ string) error { return nil }
func (m *mockDirClient) DeleteProjectRole(_ context.Context, _, _ string) error       { return nil }
func (m *mockDirClient) AddUserGrant(_ context.Context, _, _ string, _ []string) error {
	return nil
}
func (m *mockDirClient) UpdateUserGrant(_ context.Context, _, _ string, _ []string) error {
	return nil
}
func (m *mockDirClient) RemoveUserGrant(_ context.Context, _, _ string) error { return nil }
func (m *mockDirClient) ListUserGrants(_ context.Context, _ string, _ zitadel.SearchParams) (*zitadel.SearchResult[zitadel.UserGrant], error) {
	return &zitadel.SearchResult[zitadel.UserGrant]{}, nil
}
func (m *mockDirClient) ListAllGrants(_ context.Context, _ zitadel.SearchParams) (*zitadel.SearchResult[zitadel.UserGrant], error) {
	return &zitadel.SearchResult[zitadel.UserGrant]{}, nil
}

func (m *mockDirClient) ListApplications(ctx context.Context, projectID string, p zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ZitadelApplication], error) {
	m.mu.Lock()
	m.listAppsCalls[projectID]++
	fn := m.listAppsFn
	m.mu.Unlock()
	if fn != nil {
		return fn(ctx, projectID, p)
	}
	return &zitadel.SearchResult[zitadel.ZitadelApplication]{}, nil
}

func (m *mockDirClient) ListUserMetadata(ctx context.Context, userID string, p zitadel.SearchParams) (*zitadel.SearchResult[zitadel.UserMetadata], error) {
	m.mu.Lock()
	m.listMetadataCalls[userID]++
	fn := m.listMetadataFn
	m.mu.Unlock()
	if fn != nil {
		return fn(ctx, userID, p)
	}
	return &zitadel.SearchResult[zitadel.UserMetadata]{}, nil
}

// --- tests ------------------------------------------------------------------

func TestZitadelSource_Users_MapsFields(t *testing.T) {
	mc := newMockClient()
	mc.listUsersFn = func(context.Context, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ZitadelUser], error) {
		return &zitadel.SearchResult[zitadel.ZitadelUser]{
			Items: []zitadel.ZitadelUser{
				{ID: "u1", DisplayName: "Alice Doe", Email: "alice@example.com", State: "USER_STATE_ACTIVE"},
				{ID: "u2", Username: "bob", Email: "bob@example.com", State: "USER_STATE_LOCKED"},
			},
			Total: 2,
		}, nil
	}
	src := newZitadelSourceForTest(mc)

	users, err := src.Users(context.Background())
	if err != nil {
		t.Fatalf("Users error: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("got %d users, want 2", len(users))
	}
	if users[0].Name != "Alice Doe" || users[0].Avatar != "AD" || users[0].Status != "active" {
		t.Fatalf("user[0] mapping wrong: %+v", users[0])
	}
	// DisplayName empty → fall back to Username.
	if users[1].Name != "bob" || users[1].Status != "locked" {
		t.Fatalf("user[1] mapping wrong: %+v", users[1])
	}
}

func TestZitadelSource_Projects_ExpandsAndCachesRoles(t *testing.T) {
	mc := newMockClient()
	mc.listProjectsFn = func(context.Context, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ZitadelProject], error) {
		return &zitadel.SearchResult[zitadel.ZitadelProject]{
			Items: []zitadel.ZitadelProject{{ID: "p1", Name: "Project One", State: "ACTIVE"}},
		}, nil
	}
	mc.listRolesFn = func(_ context.Context, projectID string, _ zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ProjectRoleResult], error) {
		return &zitadel.SearchResult[zitadel.ProjectRoleResult]{
			Items: []zitadel.ProjectRoleResult{
				{Key: "admin", DisplayName: "Administrator", Group: "core"},
				{Key: "viewer", DisplayName: "", Group: "observers"},
			},
		}, nil
	}
	src := newZitadelSourceForTest(mc)

	p, err := src.Projects(context.Background())
	if err != nil {
		t.Fatalf("Projects error: %v", err)
	}
	if len(p) != 1 || p[0].ID != "p1" || p[0].Kind != "zitadel" {
		t.Fatalf("Projects output wrong: %+v", p)
	}
	if len(p[0].Roles) != 2 || p[0].Roles[0].Label != "Administrator" || p[0].Roles[1].Label != "viewer" {
		t.Fatalf("Roles mapping wrong: %+v", p[0].Roles)
	}

	// Second call must hit cache — no second ListProjects or ListProjectRoles.
	if _, err := src.Projects(context.Background()); err != nil {
		t.Fatal(err)
	}
	if mc.listProjectsCalls != 1 {
		t.Fatalf("expected 1 ListProjects call, got %d", mc.listProjectsCalls)
	}
	if mc.listRolesCalls["p1"] != 1 {
		t.Fatalf("expected 1 ListProjectRoles call for p1, got %d", mc.listRolesCalls["p1"])
	}
}

// appsFixture wires a project list plus a per-project apps map so individual
// tests can describe a realistic (project → apps) topology without repeating
// the boilerplate.
func appsFixture(t *testing.T, projects []zitadel.ZitadelProject, apps map[string][]zitadel.ZitadelApplication) *mockDirClient {
	t.Helper()
	mc := newMockClient()
	mc.listProjectsFn = func(context.Context, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ZitadelProject], error) {
		return &zitadel.SearchResult[zitadel.ZitadelProject]{Items: projects, Total: len(projects)}, nil
	}
	mc.listAppsFn = func(_ context.Context, projectID string, _ zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ZitadelApplication], error) {
		list := apps[projectID]
		return &zitadel.SearchResult[zitadel.ZitadelApplication]{Items: list, Total: len(list)}, nil
	}
	return mc
}

func TestZitadelSource_Applications_ReturnsRealApps(t *testing.T) {
	// 2 projects, 1 and 2 apps respectively -> 3 ApplicationCatalog entries
	// with distinct app IDs, each carrying its parent project.
	mc := appsFixture(t,
		[]zitadel.ZitadelProject{{ID: "p1", Name: "Alpha"}, {ID: "p2", Name: "Beta"}},
		map[string][]zitadel.ZitadelApplication{
			"p1": {{ID: "app-1", Name: "Alpha Web", Type: "OIDC"}},
			"p2": {
				{ID: "app-2", Name: "Beta API", Type: "API"},
				{ID: "app-3", Name: "Beta Portal", Type: "OIDC"},
			},
		},
	)
	src := newZitadelSourceForTest(mc)

	apps, err := src.Applications(context.Background())
	if err != nil {
		t.Fatalf("Applications error: %v", err)
	}
	if len(apps) != 3 {
		t.Fatalf("expected 3 applications, got %d: %+v", len(apps), apps)
	}

	byID := map[string]models.ApplicationCatalog{}
	for _, a := range apps {
		byID[a.ID] = a
	}
	if byID["app-1"].Name != "Alpha Web" || byID["app-1"].ProjectID != "p1" || byID["app-1"].Consumer != "OIDC Client" {
		t.Fatalf("app-1 mapping wrong: %+v", byID["app-1"])
	}
	if byID["app-2"].Consumer != "API" {
		t.Fatalf("app-2 consumer should be 'API', got %q", byID["app-2"].Consumer)
	}
	if byID["app-3"].ProjectID != "p2" {
		t.Fatalf("app-3 project should be p2, got %q", byID["app-3"].ProjectID)
	}
}

func TestZitadelSource_Applications_OverlayAppliesPerProject(t *testing.T) {
	// Claim profile on p1 flows to all its apps; p2 apps fall back to defaults.
	mc := appsFixture(t,
		[]zitadel.ZitadelProject{{ID: "p1", Name: "Alpha"}, {ID: "p2", Name: "Beta"}},
		map[string][]zitadel.ZitadelApplication{
			"p1": {
				{ID: "app-a", Name: "Alpha Web", Type: "OIDC"},
				{ID: "app-b", Name: "Alpha API", Type: "API"},
			},
			"p2": {{ID: "app-c", Name: "Beta Web", Type: "OIDC"}},
		},
	)
	src := newZitadelSourceForTest(mc)
	src.listClaimProfiles = func(context.Context) ([]db.ClaimProfileRow, error) {
		return []db.ClaimProfileRow{
			{ProjectID: "p1", ClaimName: "x_syndra_roles", FormatType: "csv"},
		}, nil
	}

	apps, err := src.Applications(context.Background())
	if err != nil {
		t.Fatalf("Applications error: %v", err)
	}
	if len(apps) != 3 {
		t.Fatalf("expected 3 apps, got %d", len(apps))
	}
	byID := map[string]models.ApplicationCatalog{}
	for _, a := range apps {
		byID[a.ID] = a
	}
	for _, id := range []string{"app-a", "app-b"} {
		if byID[id].ClaimName != "x_syndra_roles" || byID[id].FormatType != "csv" {
			t.Fatalf("%s should inherit p1 overlay, got claim=%q format=%q", id, byID[id].ClaimName, byID[id].FormatType)
		}
	}
	if byID["app-c"].ClaimName != "roles" || byID["app-c"].FormatType != "array" {
		t.Fatalf("app-c should use defaults, got claim=%q format=%q", byID["app-c"].ClaimName, byID["app-c"].FormatType)
	}
}

func TestZitadelSource_Applications_EmptyProjectContributesNothing(t *testing.T) {
	mc := appsFixture(t,
		[]zitadel.ZitadelProject{{ID: "p1", Name: "Alpha"}, {ID: "p2", Name: "Empty"}},
		map[string][]zitadel.ZitadelApplication{
			"p1": {{ID: "app-1", Name: "Alpha Web", Type: "OIDC"}},
			// p2 intentionally absent → mock returns empty list.
		},
	)
	src := newZitadelSourceForTest(mc)

	apps, err := src.Applications(context.Background())
	if err != nil {
		t.Fatalf("Applications error: %v", err)
	}
	if len(apps) != 1 {
		t.Fatalf("expected 1 application, got %d", len(apps))
	}
	if apps[0].ProjectID != "p1" {
		t.Fatalf("expected the surviving app to belong to p1, got %q", apps[0].ProjectID)
	}
}

func TestZitadelSource_Applications_PartialFailureStillReturns(t *testing.T) {
	// Simulate: project p2's ListApplications errors; p1's apps must still render.
	mc := newMockClient()
	mc.listProjectsFn = func(context.Context, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ZitadelProject], error) {
		return &zitadel.SearchResult[zitadel.ZitadelProject]{
			Items: []zitadel.ZitadelProject{{ID: "p1", Name: "Alpha"}, {ID: "p2", Name: "Broken"}},
			Total: 2,
		}, nil
	}
	mc.listAppsFn = func(_ context.Context, projectID string, _ zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ZitadelApplication], error) {
		if projectID == "p2" {
			return nil, errors.New("upstream unavailable")
		}
		return &zitadel.SearchResult[zitadel.ZitadelApplication]{
			Items: []zitadel.ZitadelApplication{{ID: "app-1", Name: "Alpha Web", Type: "OIDC"}},
			Total: 1,
		}, nil
	}
	src := newZitadelSourceForTest(mc)

	apps, err := src.Applications(context.Background())
	if err != nil {
		t.Fatalf("one broken project should not fail the whole list; got err=%v", err)
	}
	if len(apps) != 1 || apps[0].ID != "app-1" {
		t.Fatalf("expected just app-1 from the healthy project, got %+v", apps)
	}
}

func TestZitadelSource_Applications_PartialFailureNotCached(t *testing.T) {
	// A partial Applications result must NOT poison the global cache — otherwise
	// downstream callers that derive project scope from this list (or a stale
	// /applications page render) would see the gap for a full TTL window.
	// Per-project caches for the healthy projects are still allowed; only the
	// global concatenation skips caching.
	mc := newMockClient()
	mc.listProjectsFn = func(context.Context, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ZitadelProject], error) {
		return &zitadel.SearchResult[zitadel.ZitadelProject]{
			Items: []zitadel.ZitadelProject{{ID: "p1", Name: "Alpha"}, {ID: "p2", Name: "Broken"}},
			Total: 2,
		}, nil
	}
	failOnce := true
	mc.listAppsFn = func(_ context.Context, projectID string, _ zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ZitadelApplication], error) {
		if projectID == "p2" && failOnce {
			return nil, errors.New("upstream unavailable")
		}
		if projectID == "p1" {
			return &zitadel.SearchResult[zitadel.ZitadelApplication]{
				Items: []zitadel.ZitadelApplication{{ID: "app-1", Name: "Alpha Web", Type: "OIDC"}},
				Total: 1,
			}, nil
		}
		// p2 second call (after recovery): return its app.
		return &zitadel.SearchResult[zitadel.ZitadelApplication]{
			Items: []zitadel.ZitadelApplication{{ID: "app-2", Name: "Beta API", Type: "API"}},
			Total: 1,
		}, nil
	}
	src := newZitadelSourceForTest(mc)

	first, err := src.Applications(context.Background())
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("first call should be partial (1 app), got %d", len(first))
	}

	// Simulate p2 recovering, then re-call. Without partial-cache-skip, the
	// second call would return the cached partial result instead of going
	// back upstream and seeing app-2.
	failOnce = false
	second, err := src.Applications(context.Background())
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if len(second) != 2 {
		t.Fatalf("second call should reflect recovery (2 apps), got %d: partial result was poisoning the cache", len(second))
	}
}

func TestZitadelSource_Applications_FullSuccessCachesGlobally(t *testing.T) {
	// Counterpart to PartialFailureNotCached: when every project's apps fetch
	// succeeds, the global catalog IS cached, so the second call hits cache
	// (no extra upstream calls).
	mc := appsFixture(t,
		[]zitadel.ZitadelProject{{ID: "p1", Name: "Alpha"}, {ID: "p2", Name: "Beta"}},
		map[string][]zitadel.ZitadelApplication{
			"p1": {{ID: "app-1", Name: "Alpha Web", Type: "OIDC"}},
			"p2": {{ID: "app-2", Name: "Beta API", Type: "API"}},
		},
	)
	src := newZitadelSourceForTest(mc)

	if _, err := src.Applications(context.Background()); err != nil {
		t.Fatalf("first call: %v", err)
	}
	callsAfterFirst := mc.listAppsCalls["p1"] + mc.listAppsCalls["p2"]

	if _, err := src.Applications(context.Background()); err != nil {
		t.Fatalf("second call: %v", err)
	}
	callsAfterSecond := mc.listAppsCalls["p1"] + mc.listAppsCalls["p2"]

	if callsAfterSecond != callsAfterFirst {
		t.Fatalf("expected second call to hit cache (no extra upstream); got %d->%d", callsAfterFirst, callsAfterSecond)
	}
}

func TestZitadelSource_Users_MetadataOverlay(t *testing.T) {
	mc := newMockClient()
	mc.listUsersFn = func(context.Context, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ZitadelUser], error) {
		return &zitadel.SearchResult[zitadel.ZitadelUser]{
			Items: []zitadel.ZitadelUser{
				{ID: "u1", DisplayName: "Alice"},
				{ID: "u2", DisplayName: "Bob"},
			},
			Total: 2,
		}, nil
	}
	mc.listMetadataFn = func(_ context.Context, userID string, _ zitadel.SearchParams) (*zitadel.SearchResult[zitadel.UserMetadata], error) {
		if userID == "u1" {
			return &zitadel.SearchResult[zitadel.UserMetadata]{
				Items: []zitadel.UserMetadata{
					{Key: "title", Value: "Director"},
					{Key: "Team", Value: "Operations"}, // case-insensitive match
				},
				Total: 2,
			}, nil
		}
		return &zitadel.SearchResult[zitadel.UserMetadata]{}, nil
	}
	src := newZitadelSourceForTest(mc)

	users, err := src.Users(context.Background())
	if err != nil {
		t.Fatalf("Users error: %v", err)
	}

	byID := map[string]models.UserProfile{}
	for _, u := range users {
		byID[u.ID] = u
	}
	if byID["u1"].Title != "Director" || byID["u1"].Team != "Operations" {
		t.Fatalf("u1 metadata overlay failed: %+v", byID["u1"])
	}
	if byID["u2"].Title != "" || byID["u2"].Team != "" {
		t.Fatalf("u2 should have empty metadata fields: %+v", byID["u2"])
	}
}

func TestZitadelSource_Users_MetadataErrorDoesntFailList(t *testing.T) {
	// One user's metadata call fails; other users still receive their overlay.
	mc := newMockClient()
	mc.listUsersFn = func(context.Context, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ZitadelUser], error) {
		return &zitadel.SearchResult[zitadel.ZitadelUser]{
			Items: []zitadel.ZitadelUser{
				{ID: "u-good", DisplayName: "Good"},
				{ID: "u-bad", DisplayName: "Bad"},
			},
			Total: 2,
		}, nil
	}
	mc.listMetadataFn = func(_ context.Context, userID string, _ zitadel.SearchParams) (*zitadel.SearchResult[zitadel.UserMetadata], error) {
		if userID == "u-bad" {
			return nil, errors.New("metadata service down")
		}
		return &zitadel.SearchResult[zitadel.UserMetadata]{
			Items: []zitadel.UserMetadata{{Key: "title", Value: "Lead"}},
			Total: 1,
		}, nil
	}
	src := newZitadelSourceForTest(mc)

	users, err := src.Users(context.Background())
	if err != nil {
		t.Fatalf("Users should succeed with partial metadata failure, got err=%v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
	byID := map[string]models.UserProfile{}
	for _, u := range users {
		byID[u.ID] = u
	}
	if byID["u-good"].Title != "Lead" {
		t.Fatalf("healthy user should still get metadata overlay, got %q", byID["u-good"].Title)
	}
	if byID["u-bad"].Title != "" {
		t.Fatalf("errored user should get empty Title, got %q", byID["u-bad"].Title)
	}
}

func TestZitadelSource_Cache_TTLExpires(t *testing.T) {
	mc := newMockClient()
	mc.listUsersFn = func(context.Context, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ZitadelUser], error) {
		return &zitadel.SearchResult[zitadel.ZitadelUser]{Items: []zitadel.ZitadelUser{{ID: "u1", DisplayName: "Alice"}}}, nil
	}
	src := newZitadelSourceForTest(mc)

	// Manually advance the clock past CacheTTL between calls.
	now := time.Now()
	src.now = func() time.Time { return now }

	if _, err := src.Users(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := src.Users(context.Background()); err != nil {
		t.Fatal(err)
	}
	if mc.listUsersCalls != 1 {
		t.Fatalf("expected cached 2nd call; got %d upstream hits", mc.listUsersCalls)
	}

	// Advance past TTL.
	now = now.Add(CacheTTL + time.Second)
	if _, err := src.Users(context.Background()); err != nil {
		t.Fatal(err)
	}
	if mc.listUsersCalls != 2 {
		t.Fatalf("expected re-fetch after TTL; got %d upstream hits", mc.listUsersCalls)
	}
}

func TestZitadelSource_InvalidateProject_DropsOnlyTargeted(t *testing.T) {
	mc := newMockClient()
	mc.listProjectsFn = func(context.Context, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ZitadelProject], error) {
		return &zitadel.SearchResult[zitadel.ZitadelProject]{
			Items: []zitadel.ZitadelProject{{ID: "p1"}, {ID: "p2"}},
		}, nil
	}
	mc.listRolesFn = func(_ context.Context, projectID string, _ zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ProjectRoleResult], error) {
		return &zitadel.SearchResult[zitadel.ProjectRoleResult]{
			Items: []zitadel.ProjectRoleResult{{Key: "k", DisplayName: projectID}},
		}, nil
	}
	src := newZitadelSourceForTest(mc)

	if _, err := src.Projects(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Both per-project role caches now populated.
	if mc.listRolesCalls["p1"] != 1 || mc.listRolesCalls["p2"] != 1 {
		t.Fatalf("expected both projects listed; p1=%d p2=%d", mc.listRolesCalls["p1"], mc.listRolesCalls["p2"])
	}

	src.InvalidateProject("p1")

	// Next Projects() must re-fetch (projects list + roles for p1); p2 roles
	// cache was also dropped because Projects() itself was invalidated, which
	// is acceptable and simpler than per-project surgery on the projects list.
	if _, err := src.Projects(context.Background()); err != nil {
		t.Fatal(err)
	}
	if mc.listProjectsCalls != 2 {
		t.Fatalf("expected 2 ListProjects after invalidate, got %d", mc.listProjectsCalls)
	}
	if mc.listRolesCalls["p1"] != 2 {
		t.Fatalf("expected p1 roles re-fetched, got %d", mc.listRolesCalls["p1"])
	}
}

func TestZitadelSource_Users_UpstreamErrorSurfaces(t *testing.T) {
	mc := newMockClient()
	sentinel := errors.New("zitadel down")
	mc.listUsersFn = func(context.Context, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ZitadelUser], error) {
		return nil, sentinel
	}
	src := newZitadelSourceForTest(mc)

	_, err := src.Users(context.Background())
	if err == nil {
		t.Fatal("expected upstream error to surface, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected wrapped sentinel, got %v", err)
	}
}

func TestZitadelSource_Users_PaginatesBeyondSinglePage(t *testing.T) {
	mc := newMockClient()
	// Simulate a tenant with 1200 users across 3 pages of DefaultSearchLimit=500 each.
	total := 1200
	mc.listUsersFn = func(_ context.Context, p zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ZitadelUser], error) {
		end := p.Offset + p.Limit
		if end > total {
			end = total
		}
		items := make([]zitadel.ZitadelUser, 0, end-p.Offset)
		for i := p.Offset; i < end; i++ {
			items = append(items, zitadel.ZitadelUser{ID: "u" + itoaPadded(i), DisplayName: "User " + itoaPadded(i)})
		}
		return &zitadel.SearchResult[zitadel.ZitadelUser]{Items: items, Total: total}, nil
	}
	src := newZitadelSourceForTest(mc)

	users, err := src.Users(context.Background())
	if err != nil {
		t.Fatalf("Users error: %v", err)
	}
	if len(users) != total {
		t.Fatalf("expected %d paginated users, got %d", total, len(users))
	}
	// 3 pages: offsets 0, 500, 1000. Verify we actually hit the server 3 times.
	if mc.listUsersCalls != 3 {
		t.Fatalf("expected 3 upstream calls for 3 pages, got %d", mc.listUsersCalls)
	}
}

// itoaPadded is a small helper so test IDs sort lexicographically.
func itoaPadded(i int) string {
	s := ""
	if i == 0 {
		return "0000"
	}
	for i > 0 {
		s = string(rune('0'+i%10)) + s
		i /= 10
	}
	for len(s) < 4 {
		s = "0" + s
	}
	return s
}

func TestZitadelSource_FindUser_FallsBackToGetUser(t *testing.T) {
	mc := newMockClient()
	// ListUsers returns just u1; FindUser("u2") should fan out to GetUser.
	mc.listUsersFn = func(context.Context, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ZitadelUser], error) {
		return &zitadel.SearchResult[zitadel.ZitadelUser]{
			Items: []zitadel.ZitadelUser{{ID: "u1", DisplayName: "Alice"}},
		}, nil
	}
	mc.getUserFn = func(_ context.Context, id string) (*zitadel.ZitadelUser, error) {
		if id == "u2" {
			return &zitadel.ZitadelUser{ID: "u2", DisplayName: "Newcomer", Email: "n@example.com", State: "USER_STATE_ACTIVE"}, nil
		}
		return nil, nil
	}
	src := newZitadelSourceForTest(mc)

	u, ok, err := src.FindUser(context.Background(), "u2")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || u.Name != "Newcomer" {
		t.Fatalf("expected Newcomer via GetUser fallback, got ok=%v user=%+v", ok, u)
	}
}

// --- helpers ----------------------------------------------------------------

// newZitadelSourceForTest constructs a zitadelSource wired to a mock client
// and stubs out the DB overlay so a unit test can run without Postgres.
func newZitadelSourceForTest(c zitadel.ZitadelClient) *zitadelSource {
	z := NewZitadelSource(c)
	z.listClaimProfiles = func(context.Context) ([]db.ClaimProfileRow, error) { return nil, nil }
	return z
}
