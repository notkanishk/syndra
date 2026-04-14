package zitadel

import (
	"context"
	"fmt"
	"testing"

	"mkauth/internal/models"
)

// mockClient records calls to the ZitadelClient interface.
type mockClient struct {
	addGrantCalls    []addGrantCall
	addGrantErr      error
	removeGrantCalls []removeGrantCall
	listGrantsResult []UserGrant
	getUserResult    *ZitadelUser
}

type addGrantCall struct {
	UserID    string
	ProjectID string
	RoleKeys  []string
}

type removeGrantCall struct {
	UserID  string
	GrantID string
}

func (m *mockClient) AddUserGrant(_ context.Context, userID, projectID string, roleKeys []string) error {
	m.addGrantCalls = append(m.addGrantCalls, addGrantCall{userID, projectID, roleKeys})
	return m.addGrantErr
}

func (m *mockClient) RemoveUserGrant(_ context.Context, userID, grantID string) error {
	m.removeGrantCalls = append(m.removeGrantCalls, removeGrantCall{userID, grantID})
	return nil
}

func (m *mockClient) ListUserGrants(_ context.Context, _ string) ([]UserGrant, error) {
	return m.listGrantsResult, nil
}

func (m *mockClient) GetUser(_ context.Context, _ string) (*ZitadelUser, error) {
	return m.getUserResult, nil
}

// stubGetActiveMappingRules replaces db.GetActiveMappingRules for testing.
// We use the package-level variable from orchestrator.go's import of db.
// Since orchestrator.go calls db.GetActiveMappingRules directly, we need
// to use the injectable deps pattern. For now, we test the orchestrator
// logic by verifying that MgmtClient methods are called correctly.

func TestEnforceMappingRules_NilClientGraceful(t *testing.T) {
	resetDeps(t)
	MgmtClient = nil

	err := EnforceMappingRules(context.Background(), "user-1", "proj-src", "role-trigger")
	if err != nil {
		t.Fatalf("expected nil error for nil client, got: %v", err)
	}
}

func TestEnforceMappingRules_MatchingRule(t *testing.T) {
	resetDeps(t)

	mock := &mockClient{}
	MgmtClient = mock

	// We need to inject a mock for db.GetActiveMappingRules.
	// The orchestrator calls it directly via the db package.
	// We'll use the dbGetActiveMappingRules injectable if available,
	// otherwise we test through the full stack with a real mock.
	origGetRules := dbGetActiveMappingRules
	dbGetActiveMappingRules = func(_ context.Context) ([]models.MappingRule, error) {
		return []models.MappingRule{
			{ID: "r1", SourceProject: "proj-src", SourceRole: "trigger", TargetProject: "proj-tgt", TargetRole: "propagated"},
			{ID: "r2", SourceProject: "other-proj", SourceRole: "unrelated", TargetProject: "proj-x", TargetRole: "nope"},
		}, nil
	}
	t.Cleanup(func() { dbGetActiveMappingRules = origGetRules })

	err := EnforceMappingRules(context.Background(), "user-42", "proj-src", "trigger")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mock.addGrantCalls) != 1 {
		t.Fatalf("expected 1 AddUserGrant call, got %d", len(mock.addGrantCalls))
	}
	call := mock.addGrantCalls[0]
	if call.UserID != "user-42" || call.ProjectID != "proj-tgt" || call.RoleKeys[0] != "propagated" {
		t.Errorf("unexpected call: %+v", call)
	}
}

func TestEnforceMappingRules_GrantErrorContinues(t *testing.T) {
	resetDeps(t)

	mock := &mockClient{addGrantErr: fmt.Errorf("zitadel unavailable")}
	MgmtClient = mock

	origGetRules := dbGetActiveMappingRules
	dbGetActiveMappingRules = func(_ context.Context) ([]models.MappingRule, error) {
		return []models.MappingRule{
			{ID: "r1", SourceProject: "p", SourceRole: "r", TargetProject: "t1", TargetRole: "a"},
			{ID: "r2", SourceProject: "p", SourceRole: "r", TargetProject: "t2", TargetRole: "b"},
		}, nil
	}
	t.Cleanup(func() { dbGetActiveMappingRules = origGetRules })

	// Should not return error — grant failures are logged, not propagated.
	err := EnforceMappingRules(context.Background(), "user-1", "p", "r")
	if err != nil {
		t.Fatalf("expected nil error despite grant failures, got: %v", err)
	}

	// Both rules should have been attempted.
	if len(mock.addGrantCalls) != 2 {
		t.Errorf("expected 2 AddUserGrant calls, got %d", len(mock.addGrantCalls))
	}
}

func TestAssignUserToRole_Success(t *testing.T) {
	resetDeps(t)

	mock := &mockClient{}
	MgmtClient = mock

	err := AssignUserToRole(context.Background(), "user-1", "proj-1", "admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mock.addGrantCalls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(mock.addGrantCalls))
	}
	if mock.addGrantCalls[0].RoleKeys[0] != "admin" {
		t.Errorf("unexpected role: %v", mock.addGrantCalls[0].RoleKeys)
	}
}

func TestAssignUserToRole_NilClient(t *testing.T) {
	resetDeps(t)
	MgmtClient = nil

	err := AssignUserToRole(context.Background(), "u1", "p1", "r1")
	if err == nil {
		t.Fatal("expected error for nil client")
	}
	if err.Error() != "zitadel client uninitialized; operating in local-policy-only mode" {
		t.Errorf("unexpected error: %v", err)
	}
}
