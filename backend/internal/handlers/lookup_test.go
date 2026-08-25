package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"syndra/internal/directory"
	"syndra/internal/models"
)

// lookupStubSource is a directory.Source with configurable users + projects.
// Names map IDs to display names; missing IDs return found=false.
type lookupStubSource struct {
	users    map[string]models.UserProfile
	projects map[string]models.ProjectCatalog
}

func (s lookupStubSource) Users(context.Context) ([]models.UserProfile, error) { return nil, nil }
func (s lookupStubSource) FindUser(_ context.Context, id string) (models.UserProfile, bool, error) {
	u, ok := s.users[id]
	return u, ok, nil
}
func (s lookupStubSource) Projects(context.Context) ([]models.ProjectCatalog, error) {
	return nil, nil
}
func (s lookupStubSource) FindProject(_ context.Context, id string) (models.ProjectCatalog, bool, error) {
	p, ok := s.projects[id]
	return p, ok, nil
}
func (s lookupStubSource) Applications(context.Context) ([]models.ApplicationCatalog, error) {
	return nil, nil
}
func (s lookupStubSource) FindApplication(context.Context, string) (models.ApplicationCatalog, bool, error) {
	return models.ApplicationCatalog{}, false, nil
}
func (s lookupStubSource) RoleKeysForProject(context.Context, string) ([]string, error) {
	return nil, nil
}
func (s lookupStubSource) ProjectName(context.Context, string) (string, error) { return "", nil }
func (s lookupStubSource) Tag() string                                         { return "stub" }
func (s lookupStubSource) InvalidateAll()                                      {}
func (s lookupStubSource) InvalidateProject(string)                            {}
func (s lookupStubSource) InvalidateUsers()                                    {}

func withLookupDeps(t *testing.T,
	src directory.Source,
	bundles []models.Bundle,
	roles map[string]models.Role, // key = "<project_id>:<role_key>"
) {
	t.Helper()

	origSource := directorySource
	origAllBundles := dbGetAllBundles
	origGetRole := dbGetRole

	directorySource = func() directory.Source { return src }
	dbGetAllBundles = func(context.Context) ([]models.Bundle, error) { return bundles, nil }
	dbGetRole = func(_ context.Context, projectID, roleKey string) (models.Role, error) {
		key := projectID + ":" + roleKey
		if r, ok := roles[key]; ok {
			return r, nil
		}
		return models.Role{}, errors.New("not found")
	}

	t.Cleanup(func() {
		directorySource = origSource
		dbGetAllBundles = origAllBundles
		dbGetRole = origGetRole
	})
}

func postLookup(t *testing.T, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	switch v := body.(type) {
	case string:
		buf.WriteString(v)
	default:
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/lookup", &buf)
	req.Header.Set("Content-Type", "application/json")
	handleLookup(rr, req)
	return rr
}

func TestHandleLookup_EmptyBody_ReturnsAllEmptyMaps(t *testing.T) {
	withLookupDeps(t, lookupStubSource{}, nil, nil)

	rr := postLookup(t, LookupRequest{})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	var got LookupResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// Top-level keys MUST be present; values are empty maps.
	if got.Users == nil || got.Projects == nil || got.Roles == nil || got.Bundles == nil {
		t.Fatalf("expected non-nil top-level maps, got %+v", got)
	}
	if len(got.Users)+len(got.Projects)+len(got.Roles)+len(got.Bundles) != 0 {
		t.Fatalf("expected zero entries, got %+v", got)
	}
}

func TestHandleLookup_MixedEntities_AllResolve(t *testing.T) {
	src := lookupStubSource{
		users: map[string]models.UserProfile{
			"u-1": {ID: "u-1", Name: "Anita Sharma", Email: "anita@example.org"},
		},
		projects: map[string]models.ProjectCatalog{
			"p-1": {ID: "p-1", Name: "3D Lab"},
		},
	}
	bundles := []models.Bundle{{ID: "b-1", Name: "Lab core"}}
	roles := map[string]models.Role{
		"p-1:lab_member": {ID: "r-1", ProjectID: "p-1", RoleKey: "lab_member", DisplayName: "Member"},
	}
	withLookupDeps(t, src, bundles, roles)

	rr := postLookup(t, LookupRequest{
		UserIDs:    []string{"u-1"},
		ProjectIDs: []string{"p-1"},
		RoleKeys:   []LookupRoleKey{{ProjectID: "p-1", RoleKey: "lab_member"}},
		BundleIDs:  []string{"b-1"},
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	var got LookupResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &got)

	if u, ok := got.Users["u-1"]; !ok || u.DisplayName != "Anita Sharma" || u.Email != "anita@example.org" {
		t.Fatalf("user mismatch: %+v", got.Users)
	}
	if p, ok := got.Projects["p-1"]; !ok || p.Name != "3D Lab" {
		t.Fatalf("project mismatch: %+v", got.Projects)
	}
	if r, ok := got.Roles["p-1:lab_member"]; !ok || r.DisplayName != "Member" {
		t.Fatalf("role mismatch: %+v", got.Roles)
	}
	if b, ok := got.Bundles["b-1"]; !ok || b.Name != "Lab core" {
		t.Fatalf("bundle mismatch: %+v", got.Bundles)
	}
}

func TestHandleLookup_PartialMiss_TolerantNoError(t *testing.T) {
	src := lookupStubSource{
		users: map[string]models.UserProfile{
			"u-1": {ID: "u-1", Name: "Known"},
		},
		projects: map[string]models.ProjectCatalog{
			"p-1": {ID: "p-1", Name: "Known project"},
		},
	}
	bundles := []models.Bundle{{ID: "b-1", Name: "Known bundle"}}
	roles := map[string]models.Role{
		"p-1:r1": {ID: "r-known", ProjectID: "p-1", RoleKey: "r1", DisplayName: "Known role"},
	}
	withLookupDeps(t, src, bundles, roles)

	rr := postLookup(t, LookupRequest{
		UserIDs:    []string{"u-1", "u-missing"},
		ProjectIDs: []string{"p-1", "p-missing"},
		RoleKeys:   []LookupRoleKey{{ProjectID: "p-1", RoleKey: "r1"}, {ProjectID: "p-missing", RoleKey: "x"}},
		BundleIDs:  []string{"b-1", "b-missing"},
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	var got LookupResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &got)

	// Each map must contain only the resolved IDs, never an entry for the misses.
	for _, missing := range []string{"u-missing", "p-missing", "p-missing:x", "b-missing"} {
		if _, found := got.Users[missing]; found {
			t.Fatalf("unexpected user entry for %q: %+v", missing, got.Users)
		}
		if _, found := got.Projects[missing]; found {
			t.Fatalf("unexpected project entry for %q: %+v", missing, got.Projects)
		}
		if _, found := got.Roles[missing]; found {
			t.Fatalf("unexpected role entry for %q: %+v", missing, got.Roles)
		}
		if _, found := got.Bundles[missing]; found {
			t.Fatalf("unexpected bundle entry for %q: %+v", missing, got.Bundles)
		}
	}
	if len(got.Users) != 1 || len(got.Projects) != 1 || len(got.Roles) != 1 || len(got.Bundles) != 1 {
		t.Fatalf("expected one resolved per type, got %+v", got)
	}
}

func TestHandleLookup_OversizedBatch_Rejected(t *testing.T) {
	withLookupDeps(t, lookupStubSource{}, nil, nil)

	tooMany := make([]string, lookupMaxBatchSize+1)
	for i := range tooMany {
		tooMany[i] = fmt.Sprintf("u-%d", i)
	}

	rr := postLookup(t, LookupRequest{UserIDs: tooMany})

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for oversized batch, got %d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "VALIDATION_FAILED") {
		t.Fatalf("expected VALIDATION_FAILED error, got %s", rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "user_ids") {
		t.Fatalf("expected error to name offending field, got %s", rr.Body.String())
	}
}

func TestHandleLookup_OversizedBatch_RoleKeys(t *testing.T) {
	withLookupDeps(t, lookupStubSource{}, nil, nil)

	tooMany := make([]LookupRoleKey, lookupMaxBatchSize+1)
	for i := range tooMany {
		tooMany[i] = LookupRoleKey{ProjectID: "p-1", RoleKey: fmt.Sprintf("r-%d", i)}
	}

	rr := postLookup(t, LookupRequest{RoleKeys: tooMany})

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "role_keys") {
		t.Fatalf("expected error to name offending field, got %s", rr.Body.String())
	}
}

func TestHandleLookup_MalformedBody_Rejected(t *testing.T) {
	withLookupDeps(t, lookupStubSource{}, nil, nil)

	// Unknown field should be rejected by decodeJSONStrict.
	rr := postLookup(t, `{"unknown_field": ["x"]}`)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown field, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestHandleLookup_DedupesAndTrimsInput(t *testing.T) {
	src := lookupStubSource{
		users: map[string]models.UserProfile{
			"u-1": {ID: "u-1", Name: "Resolved"},
		},
	}
	withLookupDeps(t, src, nil, nil)

	// Whitespace-only and empty entries dropped; duplicates collapse.
	rr := postLookup(t, LookupRequest{
		UserIDs: []string{"u-1", "u-1", "  ", "", "  u-1  "},
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	var got LookupResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &got)
	if len(got.Users) != 1 {
		t.Fatalf("expected dedupe + trim → 1 entry, got %+v", got.Users)
	}
}

func TestHandleLookup_InvalidRoleKey_Skipped(t *testing.T) {
	withLookupDeps(t, lookupStubSource{}, nil, nil)

	// project_id or role_key blank → silently skipped (not an error).
	rr := postLookup(t, LookupRequest{
		RoleKeys: []LookupRoleKey{
			{ProjectID: "", RoleKey: "x"},
			{ProjectID: "p-1", RoleKey: ""},
		},
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	var got LookupResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &got)
	if len(got.Roles) != 0 {
		t.Fatalf("expected empty roles map, got %+v", got.Roles)
	}
}
