package directory

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"syndra/internal/zitadel"
)

// Over the staging stand-in, Init picks the live Zitadel source unchanged, so
// everything downstream runs the real code; with neither env it keeps the demo.
func TestInitOverTheStandIn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "world.json")
	world := `{"v":1,"users":[{"id":"u1","username":"charlotte","displayName":"Charlotte Example","email":"charlotte@example.org","state":"USER_STATE_ACTIVE","type":"human","metadata":[{"key":"title","value":"Steward"}]}],` +
		`"projects":[{"id":"p1","name":"Laser","state":"PROJECT_STATE_ACTIVE","roles":[{"key":"member","displayName":"Member","group":""}],"apps":[]}],"grants":[]}`
	if err := os.WriteFile(path, []byte(world), 0o600); err != nil {
		t.Fatal(err)
	}
	origClient, origDefault := zitadel.MgmtClient, Default
	t.Cleanup(func() { zitadel.MgmtClient, Default = origClient, origDefault })
	t.Setenv("ZITADEL_DOMAIN", "")
	t.Setenv("ZITADEL_FAKE_WORLD", path)

	zitadel.MgmtClient = nil
	if err := zitadel.InitClient(); err != nil {
		t.Fatal(err)
	}
	Init()
	if Default.Tag() != "zitadel" {
		t.Fatalf("source %q, want the live zitadel source over the stand-in", Default.Tag())
	}
	users, err := Default.Users(context.Background())
	if err != nil || len(users) != 1 || users[0].Title != "Steward" {
		t.Fatalf("users = %+v, %v", users, err)
	}
	keys, err := Default.RoleKeysForProject(context.Background(), "p1")
	if err != nil || len(keys) != 1 || keys[0] != "member" {
		t.Fatalf("role keys = %v, %v", keys, err)
	}

	t.Setenv("ZITADEL_FAKE_WORLD", "")
	zitadel.MgmtClient = nil
	if err := zitadel.InitClient(); err != nil {
		t.Fatal(err)
	}
	Init()
	if Default.Tag() != "demo" {
		t.Fatalf("source %q with neither env, want demo", Default.Tag())
	}
}
