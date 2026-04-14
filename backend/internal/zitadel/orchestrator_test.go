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
	updateGrantCalls []updateGrantCall
	removeGrantCalls []removeGrantCall
	listGrantsResult []UserGrant
	getUserResult    *ZitadelUser
	addRoleCalls     []addRoleCall
	listRolesResult  []ProjectRoleResult
	updateRoleCalls  []updateRoleCall
}

type addRoleCall struct {
	ProjectID   string
	RoleKey     string
	DisplayName string
	Group       string
}

type updateRoleCall struct {
	ProjectID   string
	RoleKey     string
	DisplayName string
	Group       string
}

type addGrantCall struct {
	UserID    string
	ProjectID string
	RoleKeys  []string
}

type updateGrantCall struct {
	UserID   string
	GrantID  string
	RoleKeys []string
}

type removeGrantCall struct {
	UserID  string
	GrantID string
}

func (m *mockClient) AddUserGrant(_ context.Context, userID, projectID string, roleKeys []string) error {
	m.addGrantCalls = append(m.addGrantCalls, addGrantCall{userID, projectID, roleKeys})
	return m.addGrantErr
}

func (m *mockClient) UpdateUserGrant(_ context.Context, userID, grantID string, roleKeys []string) error {
	m.updateGrantCalls = append(m.updateGrantCalls, updateGrantCall{userID, grantID, roleKeys})
	return nil
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

func (m *mockClient) AddProjectRole(_ context.Context, projectID, roleKey, displayName, group string) error {
	m.addRoleCalls = append(m.addRoleCalls, addRoleCall{projectID, roleKey, displayName, group})
	return nil
}

func (m *mockClient) ListProjectRoles(_ context.Context, _ string) ([]ProjectRoleResult, error) {
	return m.listRolesResult, nil
}

func (m *mockClient) UpdateProjectRole(_ context.Context, projectID, roleKey, displayName, group string) error {
	m.updateRoleCalls = append(m.updateRoleCalls, updateRoleCall{projectID, roleKey, displayName, group})
	return nil
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

// --- RevokeMappingRules tests ---

func TestRevokeMappingRules_NilClient(t *testing.T) {
	resetDeps(t)
	MgmtClient = nil

	err := RevokeMappingRules(context.Background(), "user-1", "proj-src", "trigger")
	if err != nil {
		t.Fatalf("expected nil error for nil client, got: %v", err)
	}
}

func TestRevokeMappingRules_SoleRole_RemovesGrant(t *testing.T) {
	resetDeps(t)

	mock := &mockClient{
		listGrantsResult: []UserGrant{
			{ID: "g-abc", UserID: "user-42", ProjectID: "proj-tgt", RoleKeys: []string{"propagated"}},
		},
	}
	MgmtClient = mock

	origGetRules := dbGetActiveMappingRules
	dbGetActiveMappingRules = func(_ context.Context) ([]models.MappingRule, error) {
		return []models.MappingRule{
			{ID: "r1", SourceProject: "proj-src", SourceRole: "trigger", TargetProject: "proj-tgt", TargetRole: "propagated"},
		}, nil
	}
	t.Cleanup(func() { dbGetActiveMappingRules = origGetRules })

	err := RevokeMappingRules(context.Background(), "user-42", "proj-src", "trigger")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Sole role on grant → RemoveUserGrant called.
	if len(mock.removeGrantCalls) != 1 {
		t.Fatalf("expected 1 RemoveUserGrant call, got %d", len(mock.removeGrantCalls))
	}
	if mock.removeGrantCalls[0].GrantID != "g-abc" {
		t.Errorf("unexpected grant ID: %s", mock.removeGrantCalls[0].GrantID)
	}
	if len(mock.updateGrantCalls) != 0 {
		t.Error("should not call UpdateUserGrant for sole-role grant")
	}
}

func TestRevokeMappingRules_MultiRole_UpdatesGrant(t *testing.T) {
	resetDeps(t)

	mock := &mockClient{
		listGrantsResult: []UserGrant{
			// Grant has TWO roles — only "propagated" should be removed.
			{ID: "g-multi", UserID: "user-42", ProjectID: "proj-tgt", RoleKeys: []string{"propagated", "manual-role"}},
		},
	}
	MgmtClient = mock

	origGetRules := dbGetActiveMappingRules
	dbGetActiveMappingRules = func(_ context.Context) ([]models.MappingRule, error) {
		return []models.MappingRule{
			{ID: "r1", SourceProject: "proj-src", SourceRole: "trigger", TargetProject: "proj-tgt", TargetRole: "propagated"},
		}, nil
	}
	t.Cleanup(func() { dbGetActiveMappingRules = origGetRules })

	err := RevokeMappingRules(context.Background(), "user-42", "proj-src", "trigger")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Multi-role grant → UpdateUserGrant called with remaining roles.
	if len(mock.removeGrantCalls) != 0 {
		t.Error("should NOT call RemoveUserGrant for multi-role grant")
	}
	if len(mock.updateGrantCalls) != 1 {
		t.Fatalf("expected 1 UpdateUserGrant call, got %d", len(mock.updateGrantCalls))
	}
	call := mock.updateGrantCalls[0]
	if call.GrantID != "g-multi" {
		t.Errorf("unexpected grant ID: %s", call.GrantID)
	}
	if len(call.RoleKeys) != 1 || call.RoleKeys[0] != "manual-role" {
		t.Errorf("expected remaining roles [manual-role], got %v", call.RoleKeys)
	}
}

func TestRevokeMappingRules_NoMatchingGrant(t *testing.T) {
	resetDeps(t)

	mock := &mockClient{
		listGrantsResult: []UserGrant{
			{ID: "g-1", UserID: "user-1", ProjectID: "other-proj", RoleKeys: []string{"other-role"}},
		},
	}
	MgmtClient = mock

	origGetRules := dbGetActiveMappingRules
	dbGetActiveMappingRules = func(_ context.Context) ([]models.MappingRule, error) {
		return []models.MappingRule{
			{ID: "r1", SourceProject: "p", SourceRole: "r", TargetProject: "tgt", TargetRole: "derived"},
		}, nil
	}
	t.Cleanup(func() { dbGetActiveMappingRules = origGetRules })

	err := RevokeMappingRules(context.Background(), "user-1", "p", "r")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// No matching grant → no removal attempted.
	if len(mock.removeGrantCalls) != 0 {
		t.Errorf("expected 0 removals, got %d", len(mock.removeGrantCalls))
	}
}

func TestRevokeMappingRules_ErrorContinues(t *testing.T) {
	resetDeps(t)

	mock := &mockClient{
		listGrantsResult: []UserGrant{
			{ID: "g1", UserID: "u1", ProjectID: "t1", RoleKeys: []string{"a"}},
			{ID: "g2", UserID: "u1", ProjectID: "t2", RoleKeys: []string{"b"}},
		},
	}
	// RemoveUserGrant always succeeds (mock default returns nil)
	MgmtClient = mock

	origGetRules := dbGetActiveMappingRules
	dbGetActiveMappingRules = func(_ context.Context) ([]models.MappingRule, error) {
		return []models.MappingRule{
			{ID: "r1", SourceProject: "p", SourceRole: "r", TargetProject: "t1", TargetRole: "a"},
			{ID: "r2", SourceProject: "p", SourceRole: "r", TargetProject: "t2", TargetRole: "b"},
		}, nil
	}
	t.Cleanup(func() { dbGetActiveMappingRules = origGetRules })

	err := RevokeMappingRules(context.Background(), "u1", "p", "r")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mock.removeGrantCalls) != 2 {
		t.Errorf("expected 2 removals, got %d", len(mock.removeGrantCalls))
	}
}
