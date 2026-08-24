package seed

import (
	"context"
	"testing"

	"syndra/internal/zitadel"
)

// Regression: demoEnabled must agree with directory.Init on what "live mode"
// means. Keying off raw env presence caused a subtle mixed state — a broken
// key path would leave the directory falling back to demo while seed had
// already skipped, so the app served the demo catalog on an empty DB.

func withMgmtClient(t *testing.T, c zitadel.ZitadelClient) {
	t.Helper()
	orig := zitadel.MgmtClient
	t.Cleanup(func() { zitadel.MgmtClient = orig })
	zitadel.MgmtClient = c
}

func TestDemoEnabled_InitClientFailed_SeedStillRuns(t *testing.T) {
	// Operator set env pointing at a key path that fails to load. InitClient
	// returned an error, MgmtClient stays nil, directory falls back to
	// demoSource. The seed MUST still run — otherwise the app serves demo
	// catalog data on an empty DB.
	t.Setenv("ZITADEL_DOMAIN", "auth.example.com")
	t.Setenv("ZITADEL_MACHINE_KEY_PATH", "/tmp/does-not-exist.json")
	t.Setenv("SYNDRA_SEED_DEMO", "")
	withMgmtClient(t, nil)

	if !demoEnabled() {
		t.Fatal("demoEnabled returned false while MgmtClient is nil; seed would skip and leave an empty DB")
	}
	if liveDirectoryActive() {
		t.Fatal("liveDirectoryActive returned true while MgmtClient is nil")
	}
}

func TestDemoEnabled_LiveClientReady_SeedSkipped(t *testing.T) {
	t.Setenv("ZITADEL_DOMAIN", "auth.example.com")
	t.Setenv("ZITADEL_MACHINE_KEY_PATH", "/tmp/any.json")
	t.Setenv("SYNDRA_SEED_DEMO", "")
	withMgmtClient(t, stubClient{})

	if demoEnabled() {
		t.Fatal("demoEnabled returned true with a live MgmtClient; seed would pollute the live DB")
	}
	if !liveDirectoryActive() {
		t.Fatal("liveDirectoryActive returned false with a non-nil MgmtClient")
	}
}

func TestDemoEnabled_NoZitadel_SeedRuns(t *testing.T) {
	t.Setenv("ZITADEL_DOMAIN", "")
	t.Setenv("ZITADEL_MACHINE_KEY_PATH", "")
	t.Setenv("SYNDRA_SEED_DEMO", "")
	withMgmtClient(t, nil)

	if !demoEnabled() {
		t.Fatal("demoEnabled should default on in pure local-dev mode")
	}
}

func TestDemoEnabled_ExplicitOverrides(t *testing.T) {
	withMgmtClient(t, stubClient{})

	t.Setenv("SYNDRA_SEED_DEMO", "true")
	if !demoEnabled() {
		t.Fatal("SYNDRA_SEED_DEMO=true should force-enable even in live mode")
	}

	t.Setenv("SYNDRA_SEED_DEMO", "false")
	if demoEnabled() {
		t.Fatal("SYNDRA_SEED_DEMO=false should force-disable even in demo mode")
	}
}

// stubClient satisfies zitadel.ZitadelClient with no-op methods. Only the
// *value* of the package-level MgmtClient matters for the seed gate; no
// methods are exercised by demoEnabled / liveDirectoryActive.
type stubClient struct{}

func (stubClient) GetUser(context.Context, string) (*zitadel.ZitadelUser, error) { return nil, nil }
func (stubClient) ListUsers(context.Context, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ZitadelUser], error) {
	return &zitadel.SearchResult[zitadel.ZitadelUser]{}, nil
}
func (stubClient) ListProjects(context.Context, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ZitadelProject], error) {
	return &zitadel.SearchResult[zitadel.ZitadelProject]{}, nil
}
func (stubClient) AddProjectRole(context.Context, string, string, string, string) error { return nil }
func (stubClient) ListProjectRoles(context.Context, string, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ProjectRoleResult], error) {
	return &zitadel.SearchResult[zitadel.ProjectRoleResult]{}, nil
}
func (stubClient) UpdateProjectRole(context.Context, string, string, string, string) error {
	return nil
}
func (stubClient) DeleteProjectRole(context.Context, string, string) error         { return nil }
func (stubClient) AddUserGrant(context.Context, string, string, []string) error    { return nil }
func (stubClient) UpdateUserGrant(context.Context, string, string, []string) error { return nil }
func (stubClient) RemoveUserGrant(context.Context, string, string) error           { return nil }
func (stubClient) ListUserGrants(context.Context, string, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.UserGrant], error) {
	return &zitadel.SearchResult[zitadel.UserGrant]{}, nil
}
func (stubClient) ListAllGrants(context.Context, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.UserGrant], error) {
	return &zitadel.SearchResult[zitadel.UserGrant]{}, nil
}
func (stubClient) ListApplications(context.Context, string, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ZitadelApplication], error) {
	return &zitadel.SearchResult[zitadel.ZitadelApplication]{}, nil
}
func (stubClient) ListUserMetadata(context.Context, string, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.UserMetadata], error) {
	return &zitadel.SearchResult[zitadel.UserMetadata]{}, nil
}

// Zitadel's event log is not what this fake is about. Present so the type still
// satisfies the client interface; a test that cares stubs it explicitly.
func (fake stubClient) GrantOriginByID(context.Context, string) (*zitadel.GrantOrigin, error) {
	return nil, nil
}
