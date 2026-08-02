package services

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"mkauth/internal/db"
	"mkauth/internal/models"
	"mkauth/internal/zitadel"
)

func resetRoleDeps(t *testing.T) {
	t.Helper()
	origCreate := svcDbCreateRole
	origGet := svcDbGetRole
	origDelete := svcDbDeleteRole
	origAll := svcDbGetAllLocalRoles
	origUsage := svcDbGetRoleUsageCounts
	origUsers := svcDbGetAssignedUserCounts
	origRefs := svcDbGetAllReferencedRoleKeys
	origAudit := svcInsertAuditLog
	origMgmt := zitadel.MgmtClient

	t.Cleanup(func() {
		svcDbCreateRole = origCreate
		svcDbGetRole = origGet
		svcDbDeleteRole = origDelete
		svcDbGetAllLocalRoles = origAll
		svcDbGetRoleUsageCounts = origUsage
		svcDbGetAssignedUserCounts = origUsers
		svcDbGetAllReferencedRoleKeys = origRefs
		svcInsertAuditLog = origAudit
		zitadel.MgmtClient = origMgmt
	})
}

func noopRoleDeps() {
	svcInsertAuditLog = func(_ context.Context, _, _, _, _ string) error { return nil }
	svcDbCreateRole = func(_ context.Context, projectID, roleKey, displayName, _, _, _ string, _, _ *string) (string, error) {
		return "role-new", nil
	}
	svcDbDeleteRole = func(_ context.Context, _ string) error { return nil }
	svcDbGetRole = func(_ context.Context, projectID, roleKey string) (models.Role, error) {
		return models.Role{ID: "role-new", ProjectID: projectID, RoleKey: roleKey, DisplayName: "Test", CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
	}
	svcDbGetAllLocalRoles = func(_ context.Context) ([]models.Role, error) { return nil, nil }
	svcDbGetRoleUsageCounts = func(_ context.Context) (map[string]db.RoleUsage, error) {
		return map[string]db.RoleUsage{}, nil
	}
	svcDbGetAssignedUserCounts = func(_ context.Context) (map[string]int, error) {
		return map[string]int{}, nil
	}
	svcDbGetAllReferencedRoleKeys = func(_ context.Context) ([][2]string, error) {
		return nil, nil
	}
}

func TestCreateRole_PropagatesCloneMetadata(t *testing.T) {
	resetRoleDeps(t)
	noopRoleDeps()
	zitadel.MgmtClient = nil

	// Source role exists in demo catalog (platform:admin → Label: "Administrator").
	// svcDbGetRole should return pgx.ErrNoRows for the clone source lookup so it falls through to demo catalog.
	var capturedDisplayName string
	callCount := 0
	svcDbGetRole = func(_ context.Context, projectID, roleKey string) (models.Role, error) {
		callCount++
		if callCount == 1 {
			// Clone source lookup — not in local DB, fall through to demo.
			return models.Role{}, fmt.Errorf("get role: %w", pgx.ErrNoRows)
		}
		// Second call: fetch the created role.
		return models.Role{ID: "role-cloned", ProjectID: projectID, RoleKey: roleKey, DisplayName: capturedDisplayName}, nil
	}
	svcDbCreateRole = func(_ context.Context, _, _, displayName, _, _, _ string, clonedFromProject, clonedFromRole *string) (string, error) {
		capturedDisplayName = displayName
		if clonedFromProject == nil || *clonedFromProject != "platform" {
			t.Error("expected cloned_from_project=platform")
		}
		return "role-cloned", nil
	}

	req := CreateRoleRequest{
		ProjectID: "laser", RoleKey: "new_admin",
		CloneFrom: &CloneRef{ProjectID: "platform", RoleKey: "admin"},
	}
	role, err := CreateRole(context.Background(), req, "test-user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// DisplayName should be filled from demo catalog source (platform:admin → "Administrator").
	if role.DisplayName != "Administrator" {
		t.Errorf("expected cloned display_name 'Administrator', got %q", role.DisplayName)
	}
}

func TestCreateRole_SkipsZitadelWhenNilClient(t *testing.T) {
	resetRoleDeps(t)
	noopRoleDeps()
	zitadel.MgmtClient = nil

	req := CreateRoleRequest{
		ProjectID: "p1", RoleKey: "r1", DisplayName: "R1",
	}
	_, err := CreateRole(context.Background(), req, "admin")
	if err != nil {
		t.Fatalf("expected success in local-policy-only mode, got: %v", err)
	}
}

func TestGlobalRoleCatalog_Deduplicates(t *testing.T) {
	resetRoleDeps(t)
	noopRoleDeps()

	// Same role in local DB and demo catalog — should appear once with "mkauth" source.
	svcDbGetAllLocalRoles = func(_ context.Context) ([]models.Role, error) {
		return []models.Role{
			{ProjectID: "printing", RoleKey: "admin", DisplayName: "Admin Override"},
		}, nil
	}
	// Demo catalog also has printing:admin.

	catalog, err := GlobalRoleCatalog(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	count := 0
	for _, cr := range catalog {
		if cr.ProjectID == "printing" && cr.RoleKey == "admin" {
			count++
			if cr.Source != "mkauth" {
				t.Errorf("expected source=mkauth (local DB wins), got %s", cr.Source)
			}
			if cr.DisplayName != "Admin Override" {
				t.Errorf("expected local display name, got %s", cr.DisplayName)
			}
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 entry for printing:admin, got %d", count)
	}
}

func TestCreateRole_ZitadelFailureRollsBackLocalRow(t *testing.T) {
	resetRoleDeps(t)
	noopRoleDeps()

	// Mock a Zitadel client that always fails.
	zitadel.MgmtClient = &failingRoleClient{}

	var deletedID string
	svcDbDeleteRole = func(_ context.Context, id string) error {
		deletedID = id
		return nil
	}

	req := CreateRoleRequest{
		ProjectID: "p1", RoleKey: "r1", DisplayName: "R1",
	}
	_, err := CreateRole(context.Background(), req, "admin")
	if err == nil {
		t.Fatal("expected error from Zitadel failure")
	}
	if deletedID != "role-new" {
		t.Errorf("expected compensating delete of role-new, got %q", deletedID)
	}
}

func TestResolveRoleMetadata_DBErrorSurfaces(t *testing.T) {
	resetRoleDeps(t)
	noopRoleDeps()
	zitadel.MgmtClient = nil

	// Real DB error (not pgx.ErrNoRows) should propagate, not fall through to demo.
	svcDbGetRole = func(_ context.Context, _, _ string) (models.Role, error) {
		return models.Role{}, fmt.Errorf("get role: connection refused")
	}

	req := CreateRoleRequest{
		ProjectID: "laser", RoleKey: "new_admin",
		CloneFrom: &CloneRef{ProjectID: "platform", RoleKey: "admin"},
	}
	_, err := CreateRole(context.Background(), req, "admin")
	if err == nil {
		t.Fatal("expected error when DB is unhealthy")
	}
	// Should NOT be ErrCloneSourceNotFound — it's a real backend fault.
	if errors.Is(err, ErrCloneSourceNotFound) {
		t.Error("DB error should not be masked as clone source not found")
	}
}

// failingRoleClient is a minimal ZitadelClient stub where AddProjectRole always fails.
type failingRoleClient struct{}

func (f *failingRoleClient) AddUserGrant(_ context.Context, _, _ string, _ []string) error {
	return nil
}
func (f *failingRoleClient) UpdateUserGrant(_ context.Context, _, _ string, _ []string) error {
	return nil
}
func (f *failingRoleClient) RemoveUserGrant(_ context.Context, _, _ string) error { return nil }
func (f *failingRoleClient) ListUserGrants(_ context.Context, _ string, _ zitadel.SearchParams) (*zitadel.SearchResult[zitadel.UserGrant], error) {
	return &zitadel.SearchResult[zitadel.UserGrant]{}, nil
}
func (f *failingRoleClient) GetUser(_ context.Context, _ string) (*zitadel.ZitadelUser, error) {
	return nil, nil
}
func (f *failingRoleClient) AddProjectRole(_ context.Context, _, _, _, _ string) error {
	return fmt.Errorf("zitadel unavailable")
}
func (f *failingRoleClient) ListProjectRoles(_ context.Context, _ string, _ zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ProjectRoleResult], error) {
	return &zitadel.SearchResult[zitadel.ProjectRoleResult]{}, nil
}
func (f *failingRoleClient) UpdateProjectRole(_ context.Context, _, _, _, _ string) error { return nil }
func (f *failingRoleClient) DeleteProjectRole(_ context.Context, _, _ string) error       { return nil }
func (f *failingRoleClient) ListUsers(_ context.Context, _ zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ZitadelUser], error) {
	return &zitadel.SearchResult[zitadel.ZitadelUser]{}, nil
}
func (f *failingRoleClient) ListProjects(_ context.Context, _ zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ZitadelProject], error) {
	return &zitadel.SearchResult[zitadel.ZitadelProject]{}, nil
}
func (f *failingRoleClient) ListAllGrants(_ context.Context, _ zitadel.SearchParams) (*zitadel.SearchResult[zitadel.UserGrant], error) {
	return &zitadel.SearchResult[zitadel.UserGrant]{}, nil
}
func (f *failingRoleClient) ListApplications(_ context.Context, _ string, _ zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ZitadelApplication], error) {
	return &zitadel.SearchResult[zitadel.ZitadelApplication]{}, nil
}
func (f *failingRoleClient) ListUserMetadata(_ context.Context, _ string, _ zitadel.SearchParams) (*zitadel.SearchResult[zitadel.UserMetadata], error) {
	return &zitadel.SearchResult[zitadel.UserMetadata]{}, nil
}
