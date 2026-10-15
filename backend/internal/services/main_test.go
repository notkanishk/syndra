package services

import (
	"context"
	"os"
	"testing"

	"syndra/internal/db"
	"syndra/internal/models"
)

// TestMain answers "what would this cancel?" with nothing for tests about
// something else; the tests about it (bulk_test.go) stub their own. Likewise
// no role waits and nobody has switched over, so delivery follows each
// caller's own mode; delivery_test.go stubs its own.
func TestMain(m *testing.M) {
	svcWouldCancel = func(context.Context, db.EnqueueParams) ([]models.QueuedChange, error) { return nil, nil }
	svcOutboxRowsWaitForRole = func(context.Context, []string) (map[string]bool, error) { return map[string]bool{}, nil }
	svcSetWaitReason = func(context.Context, []string, string) error { return nil }
	svcGetConfigSetting = func(context.Context, string) (string, error) { return "", nil }
	svcListRoleWaits = func(context.Context) ([]db.RoleWait, error) { return nil, nil }
	os.Exit(m.Run())
}
