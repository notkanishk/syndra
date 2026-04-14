package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"mkauth/internal/db"
	"mkauth/internal/models"
	"mkauth/internal/services"
)

func resetRoleDeps(t *testing.T) {
	t.Helper()
	origCreate := svcCreateRole
	origCatalog := svcGlobalRoleCatalog
	t.Cleanup(func() {
		svcCreateRole = origCreate
		svcGlobalRoleCatalog = origCatalog
	})
}

func postRole(t *testing.T, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/roles", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handleCreateRole(rr, req)
	return rr
}

func getGlobalCatalog(t *testing.T, query string) *httptest.ResponseRecorder {
	t.Helper()
	url := "/api/v1/roles"
	if query != "" {
		url += "?" + query
	}
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rr := httptest.NewRecorder()
	handleGetGlobalRoleCatalog(rr, req)
	return rr
}

func TestCreateRole_HappyPath(t *testing.T) {
	resetRoleDeps(t)

	svcCreateRole = func(_ context.Context, req services.CreateRoleRequest, _ string) (models.Role, error) {
		return models.Role{
			ID: "role-1", ProjectID: req.ProjectID, RoleKey: req.RoleKey,
			DisplayName: req.DisplayName, Description: req.Description,
		}, nil
	}

	body := []byte(`{"project_id":"laser","role_key":"safety_marshall","display_name":"Safety Marshall","description":"Oversees safety"}`)
	rr := postRole(t, body)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var role models.Role
	if err := json.Unmarshal(rr.Body.Bytes(), &role); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if role.ID != "role-1" || role.RoleKey != "safety_marshall" {
		t.Errorf("unexpected role: %+v", role)
	}
}

func TestCreateRole_EmptyProjectID(t *testing.T) {
	resetRoleDeps(t)

	body := []byte(`{"project_id":"","role_key":"r","display_name":"R"}`)
	rr := postRole(t, body)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestCreateRole_EmptyRoleKey(t *testing.T) {
	resetRoleDeps(t)

	body := []byte(`{"project_id":"p","role_key":"","display_name":"R"}`)
	rr := postRole(t, body)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestCreateRole_EmptyDisplayName(t *testing.T) {
	resetRoleDeps(t)

	body := []byte(`{"project_id":"p","role_key":"r","display_name":""}`)
	rr := postRole(t, body)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestCreateRole_UnknownField(t *testing.T) {
	resetRoleDeps(t)

	body := []byte(`{"project_id":"p","role_key":"r","display_name":"R","unknown_field":"x"}`)
	rr := postRole(t, body)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown field, got %d", rr.Code)
	}
}

func TestCreateRole_WithCloneFrom(t *testing.T) {
	resetRoleDeps(t)

	var receivedReq services.CreateRoleRequest
	svcCreateRole = func(_ context.Context, req services.CreateRoleRequest, _ string) (models.Role, error) {
		receivedReq = req
		return models.Role{
			ID: "role-2", ProjectID: req.ProjectID, RoleKey: req.RoleKey,
			DisplayName: "Admin", Description: "Full project access",
			ClonedFromProject: "printing", ClonedFromRole: "admin",
		}, nil
	}

	body := []byte(`{"project_id":"laser","role_key":"new_admin","display_name":"","clone_from":{"project_id":"printing","role_key":"admin"}}`)
	rr := postRole(t, body)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	if receivedReq.CloneFrom == nil {
		t.Fatal("expected clone_from to be passed through")
	}
	if receivedReq.CloneFrom.ProjectID != "printing" || receivedReq.CloneFrom.RoleKey != "admin" {
		t.Errorf("unexpected clone_from: %+v", receivedReq.CloneFrom)
	}
}

func TestCreateRole_CloneFromNotFound(t *testing.T) {
	resetRoleDeps(t)

	svcCreateRole = func(_ context.Context, _ services.CreateRoleRequest, _ string) (models.Role, error) {
		return models.Role{}, services.ErrCloneSourceNotFound
	}

	body := []byte(`{"project_id":"laser","role_key":"r","display_name":"","clone_from":{"project_id":"nonexistent","role_key":"x"}}`)
	rr := postRole(t, body)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCreateRole_Duplicate(t *testing.T) {
	resetRoleDeps(t)

	svcCreateRole = func(_ context.Context, _ services.CreateRoleRequest, _ string) (models.Role, error) {
		return models.Role{}, db.ErrDuplicateRole
	}

	body := []byte(`{"project_id":"p","role_key":"existing","display_name":"E"}`)
	rr := postRole(t, body)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestGetGlobalRoleCatalog_MergesSources(t *testing.T) {
	resetRoleDeps(t)

	svcGlobalRoleCatalog = func(_ context.Context) ([]models.CatalogRole, error) {
		return []models.CatalogRole{
			{ProjectID: "p1", RoleKey: "admin", Source: "mkauth", DisplayLabel: "P1: admin"},
			{ProjectID: "p2", RoleKey: "member", Source: "demo", DisplayLabel: "P2: member"},
			{ProjectID: "p3", RoleKey: "viewer", Source: "referenced", DisplayLabel: "P3: viewer"},
		}, nil
	}

	rr := getGlobalCatalog(t, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var catalog []models.CatalogRole
	if err := json.Unmarshal(rr.Body.Bytes(), &catalog); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(catalog) != 3 {
		t.Fatalf("expected 3 roles, got %d", len(catalog))
	}
	sources := map[string]bool{}
	for _, cr := range catalog {
		sources[cr.Source] = true
	}
	if !sources["mkauth"] || !sources["demo"] || !sources["referenced"] {
		t.Errorf("expected all three sources, got %v", sources)
	}
}

func TestGetGlobalRoleCatalog_UnusedFlag(t *testing.T) {
	resetRoleDeps(t)

	svcGlobalRoleCatalog = func(_ context.Context) ([]models.CatalogRole, error) {
		return []models.CatalogRole{
			{RoleKey: "active", BundleCount: 1, AssignedUserCount: 2, IsUnused: false},
			{RoleKey: "orphan", BundleCount: 0, RuleCount: 0, AssignedUserCount: 0, IsUnused: true},
		}, nil
	}

	rr := getGlobalCatalog(t, "")
	var catalog []models.CatalogRole
	if err := json.Unmarshal(rr.Body.Bytes(), &catalog); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if catalog[0].IsUnused {
		t.Error("active role should not be unused")
	}
	if !catalog[1].IsUnused {
		t.Error("orphan role should be flagged as unused")
	}
}

func TestGetGlobalRoleCatalog_ProjectFilter(t *testing.T) {
	resetRoleDeps(t)

	svcGlobalRoleCatalog = func(_ context.Context) ([]models.CatalogRole, error) {
		return []models.CatalogRole{
			{ProjectID: "printing", RoleKey: "admin"},
			{ProjectID: "printing", RoleKey: "member"},
			{ProjectID: "laser", RoleKey: "trainee"},
		}, nil
	}

	rr := getGlobalCatalog(t, "project_id=printing")
	var catalog []models.CatalogRole
	if err := json.Unmarshal(rr.Body.Bytes(), &catalog); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(catalog) != 2 {
		t.Fatalf("expected 2 roles for printing, got %d", len(catalog))
	}
	for _, cr := range catalog {
		if cr.ProjectID != "printing" {
			t.Errorf("unexpected project: %s", cr.ProjectID)
		}
	}
}
