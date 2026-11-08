package zitadel

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// A made-up world: example.org addresses and invented ids only.
func testWorld() World {
	return World{
		V: WorldVersion,
		Users: []WorldUser{
			{ID: "u1", Username: "ada", DisplayName: "Ada Example", GivenName: "Ada", FamilyName: "Example", Email: "ada@example.org", State: "USER_STATE_ACTIVE", Type: "human",
				Created: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), Metadata: []UserMetadata{{Key: "title", Value: "Steward"}}},
			{ID: "u2", Username: "bo", DisplayName: "Bo Example", Email: "bo@example.org", State: "USER_STATE_ACTIVE", Type: "human"},
			{ID: "m1", Username: "sync-bot", State: "USER_STATE_ACTIVE", Type: "machine"},
		},
		Projects: []WorldProject{
			{ID: "p1", Name: "Laser", State: "PROJECT_STATE_ACTIVE",
				Roles: []ProjectRoleResult{{Key: "member", DisplayName: "Member"}, {Key: "mentor", DisplayName: "Mentor", Group: "staff"}},
				Apps: []WorldApp{
					{ID: "a1", Name: "Laser web", State: "APP_STATE_ACTIVE", Type: "OIDC", ClientID: "c1@laser", Token: &TokenSettings{AccessTokenType: "JWT", ProjectRoleAssertion: true}},
					{ID: "a2", Name: "Laser api", State: "APP_STATE_ACTIVE", Type: "API"},
				}},
			{ID: "p2", Name: "Wiki", State: "PROJECT_STATE_ACTIVE", Roles: []ProjectRoleResult{{Key: "editor"}}, Apps: []WorldApp{}},
		},
		Grants: []UserGrant{
			{ID: "g1", UserID: "u1", ProjectID: "p1", RoleKeys: []string{"member", "mentor"}},
			{ID: "g2", UserID: "u2", ProjectID: "p2", RoleKeys: []string{"editor"}},
		},
	}
}

func writeWorld(t *testing.T, w World) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "world.json")
	b, err := json.Marshal(w)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func loadWorld(t *testing.T, path string) *WorldClient {
	t.Helper()
	c, err := NewWorldClient(path)
	if err != nil {
		t.Fatalf("load world: %v", err)
	}
	return c
}

func wantStatus(t *testing.T, err error, code int) {
	t.Helper()
	var se *StatusError
	if !errors.As(err, &se) || se.Code != code {
		t.Fatalf("got %v, want a StatusError %d", err, code)
	}
}

func grantsOf(t *testing.T, c ZitadelClient) []UserGrant {
	t.Helper()
	res, err := c.ListAllGrants(context.Background(), SearchParams{})
	if err != nil {
		t.Fatal(err)
	}
	return res.Items
}

func TestWorldReads(t *testing.T) {
	ctx := context.Background()
	c := loadWorld(t, writeWorld(t, testWorld()))

	u, err := c.GetUser(ctx, "u1")
	if err != nil || u.Email != "ada@example.org" || u.GivenName != "Ada" || u.Machine || u.Created.IsZero() {
		t.Fatalf("GetUser u1 = %+v, %v", u, err)
	}
	if m, _ := c.GetUser(ctx, "m1"); !m.Machine {
		t.Fatalf("m1 should read as a machine user")
	}
	_, err = c.GetUser(ctx, "nobody")
	wantStatus(t, err, http.StatusNotFound)

	md, _ := c.ListUserMetadata(ctx, "u1", SearchParams{})
	if len(md.Items) != 1 || md.Items[0].Value != "Steward" {
		t.Fatalf("metadata = %+v", md)
	}
	roles, _ := c.ListProjectRoles(ctx, "p1", SearchParams{})
	if roles.Total != 2 || roles.Items[1].Group != "staff" {
		t.Fatalf("roles = %+v", roles)
	}
	apps, _ := c.ListApplications(ctx, "p1", SearchParams{})
	if apps.Total != 2 || apps.Items[0].ClientID != "c1@laser" || apps.Items[1].Type != "API" {
		t.Fatalf("apps = %+v", apps)
	}
	ts, err := c.TokenSettings(ctx, "p1", "a1")
	if err != nil || ts.AccessTokenType != "JWT" || !ts.ProjectRoleAssertion {
		t.Fatalf("TokenSettings = %+v, %v", ts, err)
	}
	_, err = c.TokenSettings(ctx, "p1", "a2") // exported without settings
	wantStatus(t, err, http.StatusNotFound)

	// Unknown ids read as nothing there, as Zitadel's searches do.
	for name, total := range map[string]int{
		"roles": must(c.ListProjectRoles(ctx, "nope", SearchParams{})).Total,
		"apps":  must(c.ListApplications(ctx, "nope", SearchParams{})).Total,
		"md":    must(c.ListUserMetadata(ctx, "nope", SearchParams{})).Total,
		"user":  must(c.ListUserGrants(ctx, "nope", SearchParams{})).Total,
	} {
		if total != 0 {
			t.Errorf("%s of an unknown id: total %d, want 0", name, total)
		}
	}
	mine, _ := c.ListUserGrants(ctx, "u1", SearchParams{})
	if mine.Total != 1 || mine.Items[0].ID != "g1" {
		t.Fatalf("u1 grants = %+v", mine)
	}

	// A read hands out copies: changing one does not change the world.
	mine.Items[0].RoleKeys[0] = "changed"
	if grantsOf(t, c)[0].RoleKeys[0] != "member" {
		t.Fatal("a read's slice aliases the world")
	}
}

func must[T any](r *SearchResult[T], err error) *SearchResult[T] {
	if err != nil {
		panic(err)
	}
	return r
}

func TestWorldPaging(t *testing.T) {
	ctx := context.Background()
	c := loadWorld(t, writeWorld(t, testWorld()))

	var ids []string
	for off := 0; ; off++ {
		res, _ := c.ListUsers(ctx, SearchParams{Limit: 1, Offset: off})
		if res.Total != 3 {
			t.Fatalf("total %d, want 3", res.Total)
		}
		if len(res.Items) == 0 {
			break
		}
		ids = append(ids, res.Items[0].ID)
	}
	if !slices.Equal(ids, []string{"u1", "u2", "m1"}) {
		t.Fatalf("paged ids %v", ids)
	}
	if res, _ := c.ListUsers(ctx, SearchParams{Offset: 99}); len(res.Items) != 0 || res.Total != 3 {
		t.Fatalf("past the end = %+v", res)
	}
	if res, _ := c.ListUsers(ctx, SearchParams{}); len(res.Items) != 3 {
		t.Fatalf("no limit is the default page, got %d", len(res.Items))
	}
}

func TestWorldGrantWritesPersist(t *testing.T) {
	ctx := context.Background()
	path := writeWorld(t, testWorld())
	c := loadWorld(t, path)

	if err := c.AddUserGrant(ctx, "u2", "p1", []string{"member"}); err != nil {
		t.Fatal(err)
	}
	if err := c.UpdateUserGrant(ctx, "u1", "g1", []string{"member"}); err != nil {
		t.Fatal(err)
	}
	if err := c.RemoveUserGrant(ctx, "u2", "g2"); err != nil {
		t.Fatal(err)
	}

	// A restart reads what the writes left.
	reloaded := loadWorld(t, path)
	got := grantsOf(t, reloaded)
	if len(got) != 2 {
		t.Fatalf("grants after reload = %+v", got)
	}
	if got[0].ID != "g1" || !slices.Equal(got[0].RoleKeys, []string{"member"}) {
		t.Fatalf("updated grant = %+v", got[0])
	}
	added := got[1]
	if added.ID == "" || added.ID == "g1" || added.ID == "g2" || added.UserID != "u2" || added.ProjectID != "p1" {
		t.Fatalf("added grant = %+v", added)
	}
	// The added grant's new id is the one to change it by.
	if err := reloaded.RemoveUserGrant(ctx, "u2", added.ID); err != nil {
		t.Fatal(err)
	}
	if len(grantsOf(t, loadWorld(t, path))) != 1 {
		t.Fatal("removing the added grant did not persist")
	}
	if matches, _ := filepath.Glob(filepath.Join(filepath.Dir(path), ".world-*")); len(matches) != 0 {
		t.Fatalf("temp files left behind: %v", matches)
	}
}

func TestWorldGrantWritesRefuseLikeZitadel(t *testing.T) {
	ctx := context.Background()
	path := writeWorld(t, testWorld())
	before, _ := os.ReadFile(path)
	c := loadWorld(t, path)

	wantStatus(t, c.AddUserGrant(ctx, "u1", "p1", []string{"member"}), http.StatusConflict) // one grant per user and project
	wantStatus(t, c.AddUserGrant(ctx, "nobody", "p1", []string{"member"}), http.StatusNotFound)
	wantStatus(t, c.AddUserGrant(ctx, "u2", "nope", []string{"member"}), http.StatusNotFound)
	wantStatus(t, c.AddUserGrant(ctx, "u2", "p1", []string{"pilot"}), http.StatusBadRequest)
	wantStatus(t, c.UpdateUserGrant(ctx, "u1", "nope", []string{"member"}), http.StatusNotFound)
	wantStatus(t, c.UpdateUserGrant(ctx, "u2", "g1", []string{"member"}), http.StatusNotFound) // g1 is u1's
	wantStatus(t, c.UpdateUserGrant(ctx, "u1", "g1", []string{"pilot"}), http.StatusBadRequest)
	wantStatus(t, c.RemoveUserGrant(ctx, "u1", "nope"), http.StatusNotFound)

	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("a refused write changed the file")
	}
}

func TestWorldFailedSaveChangesNothing(t *testing.T) {
	ctx := context.Background()
	path := writeWorld(t, testWorld())
	c := loadWorld(t, path)
	if err := os.RemoveAll(filepath.Dir(path)); err != nil {
		t.Fatal(err)
	}
	if err := c.RemoveUserGrant(ctx, "u1", "g1"); err == nil {
		t.Fatal("a write that could not be saved reported success")
	}
	if len(grantsOf(t, c)) != 2 {
		t.Fatal("a write that could not be saved still changed the world in memory")
	}
}

func TestWorldProjectRoleWrites(t *testing.T) {
	ctx := context.Background()
	path := writeWorld(t, testWorld())
	c := loadWorld(t, path)

	if err := c.AddProjectRole(ctx, "p2", "reader", "Reader", "people"); err != nil {
		t.Fatal(err)
	}
	wantStatus(t, c.AddProjectRole(ctx, "p2", "reader", "", ""), http.StatusConflict)
	wantStatus(t, c.AddProjectRole(ctx, "nope", "x", "", ""), http.StatusNotFound)
	if err := c.UpdateProjectRole(ctx, "p1", "member", "Members", "people"); err != nil {
		t.Fatal(err)
	}
	wantStatus(t, c.UpdateProjectRole(ctx, "p1", "nope", "", ""), http.StatusNotFound)
	// Deleting a role takes it out of every grant, as Zitadel does.
	if err := c.DeleteProjectRole(ctx, "p1", "mentor"); err != nil {
		t.Fatal(err)
	}
	wantStatus(t, c.DeleteProjectRole(ctx, "p1", "mentor"), http.StatusNotFound)

	r := loadWorld(t, path)
	p2 := must(r.ListProjectRoles(ctx, "p2", SearchParams{}))
	p1 := must(r.ListProjectRoles(ctx, "p1", SearchParams{}))
	if p2.Total != 2 || p1.Total != 1 || p1.Items[0].DisplayName != "Members" {
		t.Fatalf("roles after reload: p1 %+v, p2 %+v", p1, p2)
	}
	if g := grantsOf(t, r)[0]; !slices.Equal(g.RoleKeys, []string{"member"}) {
		t.Fatalf("g1 still holds the deleted role: %+v", g)
	}
}

func TestWorldEventsAreEmpty(t *testing.T) {
	c := loadWorld(t, writeWorld(t, testWorld()))
	origin, err := c.GrantOriginByID(context.Background(), "g1")
	if origin != nil || err != nil {
		t.Fatalf("GrantOriginByID = %+v, %v; want the log's honest nothing", origin, err)
	}
	evs, more, err := c.SearchEvents(context.Background(), EventSearch{Types: []string{"user.grant.added"}})
	if len(evs) != 0 || more || err != nil {
		t.Fatalf("SearchEvents = %v, %v, %v", evs, more, err)
	}
}

func TestWorldRefusesAnotherVersion(t *testing.T) {
	w := testWorld()
	w.V = 2
	if _, err := NewWorldClient(writeWorld(t, w)); err == nil {
		t.Fatal("a version 2 world loaded")
	}
	if _, err := NewWorldClient(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatal("a missing world loaded")
	}
}

// noWrites is a full client whose every write fails the test.
type noWrites struct {
	*WorldClient
	t *testing.T
}

func (n noWrites) AddUserGrant(context.Context, string, string, []string) error {
	n.t.Fatal("export called AddUserGrant")
	return nil
}
func (n noWrites) UpdateUserGrant(context.Context, string, string, []string) error {
	n.t.Fatal("export called UpdateUserGrant")
	return nil
}
func (n noWrites) RemoveUserGrant(context.Context, string, string) error {
	n.t.Fatal("export called RemoveUserGrant")
	return nil
}
func (n noWrites) AddProjectRole(context.Context, string, string, string, string) error {
	n.t.Fatal("export called AddProjectRole")
	return nil
}
func (n noWrites) UpdateProjectRole(context.Context, string, string, string, string) error {
	n.t.Fatal("export called UpdateProjectRole")
	return nil
}
func (n noWrites) DeleteProjectRole(context.Context, string, string) error {
	n.t.Fatal("export called DeleteProjectRole")
	return nil
}

var _ ZitadelClient = noWrites{}

func TestExportPagesFullyAndOnlyReads(t *testing.T) {
	orig := exportPageSize
	exportPageSize = 1 // every list takes several pages
	t.Cleanup(func() { exportPageSize = orig })

	src := testWorld()
	src.Users[0].Metadata = append(src.Users[0].Metadata, UserMetadata{Key: "Team", Value: "Lasers"}, UserMetadata{Key: "phone", Value: "private"})
	c := loadWorld(t, writeWorld(t, src))

	var client ZitadelClient = noWrites{c, t}
	got, err := ExportWorld(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}

	// Only the keys the directory reads leave production.
	want := testWorld()
	want.Users[0].Metadata = append(want.Users[0].Metadata, UserMetadata{Key: "Team", Value: "Lasers"})
	gb, _ := json.Marshal(got)
	wb, _ := json.Marshal(want)
	if string(gb) != string(wb) {
		t.Fatalf("export differs from the world it read:\n got %s\nwant %s", gb, wb)
	}

	// And it loads back as a world.
	if u, _, g := loadWorld(t, writeWorld(t, *got)).Counts(); u != 3 || g != 2 {
		t.Fatalf("exported world counts %d users, %d grants", u, g)
	}
}

// failingReads fails one read; the export must fail rather than write half a world.
type failingReads struct{ *WorldClient }

func (failingReads) ListAllGrants(context.Context, SearchParams) (*SearchResult[UserGrant], error) {
	return nil, &StatusError{Code: http.StatusServiceUnavailable}
}

func TestExportFailsOnAFailedRead(t *testing.T) {
	c := loadWorld(t, writeWorld(t, testWorld()))
	if w, err := ExportWorld(context.Background(), failingReads{c}); err == nil || w != nil {
		t.Fatalf("export = %v, %v; want an error and no world", w, err)
	}
}

func TestInitClientWorldWiring(t *testing.T) {
	path := writeWorld(t, testWorld())

	t.Run("both set refuses", func(t *testing.T) {
		resetDeps(t)
		MgmtClient = nil
		t.Setenv("ZITADEL_FAKE_WORLD", path)
		t.Setenv("ZITADEL_DOMAIN", "auth.example.org")
		t.Setenv("ZITADEL_MACHINE_KEY_PATH", "/nonexistent")
		if err := InitClient(); !errors.Is(err, ErrWorldBesideZitadel) {
			t.Fatalf("InitClient = %v, want ErrWorldBesideZitadel", err)
		}
		if MgmtClient != nil || IsStandIn() {
			t.Fatal("a client was set beside a refusal")
		}
	})

	t.Run("only the world uses the world", func(t *testing.T) {
		resetDeps(t)
		MgmtClient = nil
		t.Setenv("ZITADEL_FAKE_WORLD", path)
		t.Setenv("ZITADEL_DOMAIN", "")
		if err := InitClient(); err != nil {
			t.Fatal(err)
		}
		if !IsStandIn() {
			t.Fatalf("MgmtClient = %T, want the stand-in", MgmtClient)
		}
	})

	t.Run("a world that won't load is an error, not the demo", func(t *testing.T) {
		resetDeps(t)
		MgmtClient = nil
		t.Setenv("ZITADEL_FAKE_WORLD", filepath.Join(t.TempDir(), "missing.json"))
		t.Setenv("ZITADEL_DOMAIN", "")
		if err := InitClient(); err == nil || MgmtClient != nil {
			t.Fatalf("InitClient = %v, MgmtClient %T", err, MgmtClient)
		}
	})

	t.Run("neither keeps the demo", func(t *testing.T) {
		resetDeps(t)
		MgmtClient = nil
		t.Setenv("ZITADEL_FAKE_WORLD", "")
		t.Setenv("ZITADEL_DOMAIN", "")
		if err := InitClient(); err != nil || MgmtClient != nil || IsStandIn() {
			t.Fatalf("InitClient = %v, MgmtClient %T", err, MgmtClient)
		}
	})
}
