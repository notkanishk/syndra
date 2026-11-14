package addons

import (
	"context"
	"encoding/json"
	"maps"
	"os"
	"slices"
	"sync"
	"testing"

	"syndra/internal/db"
)

// memRoles stands in for targets.field_roles in every test of this package:
// nothing here reaches a database, and Refresh persists the role map.
var memRoles = struct {
	sync.Mutex
	byTarget map[string]db.FieldRoleMap
	saves    int
}{byTarget: map[string]db.FieldRoleMap{}}

// memMetrics stands in for addon_metrics: the stored descriptors by target,
// and how many times they were written.
var memMetrics = struct {
	sync.Mutex
	byTarget map[string]map[string]json.RawMessage
	saves    int
}{byTarget: map[string]map[string]json.RawMessage{}}

func TestMain(m *testing.M) {
	dbTargetFieldRoles = func(_ context.Context, target string) (db.FieldRoleMap, bool, error) {
		memRoles.Lock()
		defer memRoles.Unlock()
		m, found := memRoles.byTarget[target]
		return db.FieldRoleMap{Roles: maps.Clone(m.Roles), Retired: slices.Clone(m.Retired), Shape: slices.Clone(m.Shape)}, found, nil
	}
	dbSaveTargetFieldRoles = func(_ context.Context, target string, m db.FieldRoleMap) error {
		memRoles.Lock()
		defer memRoles.Unlock()
		memRoles.byTarget[target] = db.FieldRoleMap{Roles: maps.Clone(m.Roles), Retired: slices.Clone(m.Retired), Shape: slices.Clone(m.Shape)}
		memRoles.saves++
		return nil
	}
	dbStatsDescriptors = func(_ context.Context, target string) (map[string]json.RawMessage, error) {
		memMetrics.Lock()
		defer memMetrics.Unlock()
		return maps.Clone(memMetrics.byTarget[target]), nil
	}
	dbSaveStatsDescriptors = func(_ context.Context, target string, d map[string]json.RawMessage) error {
		memMetrics.Lock()
		defer memMetrics.Unlock()
		if memMetrics.byTarget[target] == nil {
			memMetrics.byTarget[target] = map[string]json.RawMessage{}
		}
		maps.Copy(memMetrics.byTarget[target], d)
		memMetrics.saves++
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
	memRoles.byTarget = map[string]db.FieldRoleMap{}
	memRoles.Unlock()
	t.Cleanup(func() {
		memRoles.Lock()
		memRoles.byTarget = map[string]db.FieldRoleMap{}
		memRoles.Unlock()
	})
}
