package services

import (
	"context"
	"os"
	"testing"

	"syndra/internal/addons"
	"syndra/internal/db"
	"syndra/internal/models"
)

// The TrueNAS field names and role map, which most tests resolve against: the
// core itself names no field (design §3). A test that needs another target's
// shape (an account called `active`) says so.
const (
	FieldEnabled    = "enabled"
	FieldSMBEnabled = "smb_enabled"
)

var truenasRoles = addons.Roles{
	"group": addons.RoleMembership, "home": addons.RoleFlag, "home_quota": addons.RoleLimit,
	FieldEnabled: addons.RoleAccount, FieldSMBEnabled: addons.RoleChannel,
}

// TestMain: no role waits, and nothing is recorded on a row after a send;
// delivery_test.go stubs its own.
func TestMain(m *testing.M) {
	svcFieldRoles = func(context.Context, string) (addons.Roles, error) { return truenasRoles, nil }
	svcSetWaitReason = func(context.Context, []string, string) error { return nil }
	// A halted send's rows are scheduled for a retry (db.ScheduleRetry), and
	// nothing is pending unless a test says so.
	svcScheduleRetry = func(_ context.Context, ids []string, _ string) ([]string, error) { return ids, nil }
	svcGetPendingRows = func(context.Context) ([]models.PendingPropagation, error) { return nil, nil }
	svcGetConfigSetting = func(context.Context, string) (string, error) { return "", nil }
	svcListRoleWaits = func(context.Context) ([]db.RoleWait, error) { return nil, nil }
	// No bundle includes another unless a test says so (B8).
	svcGetWorkingIncludes = func(context.Context, string) ([]models.BundleInclude, error) { return nil, nil }
	svcGetVersionIncludes = func(context.Context, string) ([]models.BundleInclude, error) { return nil, nil }
	svcListIncluders = func(context.Context, string) ([]db.Includer, error) { return nil, nil }
	svcRetireBlockers = func(context.Context, string) ([]string, error) { return nil, nil }
	svcSnapshotVersionIncludes = func(context.Context, string, []models.BundleInclude) error { return nil }
	svcSetWorkingInclude = func(context.Context, string, string, string, int) (models.BundleInclude, error) {
		return models.BundleInclude{}, nil
	}
	// No outbox rows behind any standing, and nobody read on their own, unless a test says so.
	svcLatestUserObservations = func(context.Context, []string) (map[string]db.Observation, error) { return nil, nil }
	svcRoleDeliveries = func(context.Context, []string) (map[string]db.Deliveries, error) { return nil, nil }
	// The term dates are the defaults unless a test sets them.
	termsSetting = func(context.Context, string) (db.ConfigSetting, bool, error) { return db.ConfigSetting{}, false, nil }
	os.Exit(m.Run())
}
