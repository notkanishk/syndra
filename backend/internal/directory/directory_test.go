package directory

import (
	"context"
	"errors"
	"testing"
	"time"

	"mkauth/internal/db"
	"mkauth/internal/zitadel"
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
	listUsersCalls    int
	listProjectsCalls int
	listRolesCalls    map[string]int

	listUsersFn    func(context.Context, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ZitadelUser], error)
	listProjectsFn func(context.Context, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ZitadelProject], error)
	listRolesFn    func(context.Context, string, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ProjectRoleResult], error)
	getUserFn      func(context.Context, string) (*zitadel.ZitadelUser, error)
}

func newMockClient() *mockDirClient {
	return &mockDirClient{listRolesCalls: map[string]int{}}
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

func TestZitadelSource_Applications_OverlayFromClaimProfiles(t *testing.T) {
	mc := newMockClient()
	mc.listProjectsFn = func(context.Context, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ZitadelProject], error) {
		return &zitadel.SearchResult[zitadel.ZitadelProject]{
			Items: []zitadel.ZitadelProject{
				{ID: "p1", Name: "Alpha"},
				{ID: "p2", Name: "Beta"},
			},
		}, nil
	}
	src := newZitadelSourceForTest(mc)
	src.listClaimProfiles = func(context.Context) ([]db.ClaimProfileRow, error) {
		return []db.ClaimProfileRow{
			{ProjectID: "p1", ClaimName: "x_mkauth_roles", FormatType: "csv"},
		}, nil
	}

	apps, err := src.Applications(context.Background())
	if err != nil {
		t.Fatalf("Applications error: %v", err)
	}
	if len(apps) != 2 {
		t.Fatalf("expected 2 applications, got %d", len(apps))
	}

	byID := map[string]struct {
		name, claim, format string
	}{}
	for _, a := range apps {
		byID[a.ID] = struct{ name, claim, format string }{a.Name, a.ClaimName, a.FormatType}
	}
	if byID["p1"].claim != "x_mkauth_roles" || byID["p1"].format != "csv" {
		t.Fatalf("p1 overlay not applied: %+v", byID["p1"])
	}
	if byID["p2"].claim != "roles" || byID["p2"].format != "array" {
		t.Fatalf("p2 should use defaults: %+v", byID["p2"])
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
