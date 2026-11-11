package addons

import (
	"context"
	"maps"
	"os"
	"sync"
	"testing"
)

// memRoles stands in for targets.field_roles in every test of this package:
// nothing here reaches a database, and Refresh persists the role map.
var memRoles = struct {
	sync.Mutex
	byTarget map[string]map[string]string
}{byTarget: map[string]map[string]string{}}

func TestMain(m *testing.M) {
	dbTargetFieldRoles = func(_ context.Context, target string) (map[string]string, bool, error) {
		memRoles.Lock()
		defer memRoles.Unlock()
		roles, found := memRoles.byTarget[target]
		return maps.Clone(roles), found, nil
	}
	dbSaveTargetFieldRoles = func(_ context.Context, target string, roles map[string]string) error {
		memRoles.Lock()
		defer memRoles.Unlock()
		memRoles.byTarget[target] = maps.Clone(roles)
		return nil
	}
	// No background manifest read races a test: the one test about the
	// re-read installs its own recorder.
	rereadAfterShapeRefusal = func(*Addon) {}
	os.Exit(m.Run())
}

// forgetRoles clears the persisted role maps, for a test that registers anew.
func forgetRoles(t *testing.T) {
	t.Helper()
	memRoles.Lock()
	memRoles.byTarget = map[string]map[string]string{}
	memRoles.Unlock()
	t.Cleanup(func() {
		memRoles.Lock()
		memRoles.byTarget = map[string]map[string]string{}
		memRoles.Unlock()
	})
}
