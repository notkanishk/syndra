package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"syndra/internal/auth"
	"syndra/internal/directory"
	"syndra/internal/models"
)

type stubDirectory struct {
	users map[string]models.UserProfile
}

func (s *stubDirectory) Users(_ context.Context) ([]models.UserProfile, error) { return nil, nil }
func (s *stubDirectory) FindUser(_ context.Context, userID string) (models.UserProfile, bool, error) {
	u, ok := s.users[userID]
	return u, ok, nil
}
func (s *stubDirectory) Projects(_ context.Context) ([]models.ProjectCatalog, error) { return nil, nil }
func (s *stubDirectory) FindProject(_ context.Context, _ string) (models.ProjectCatalog, bool, error) {
	return models.ProjectCatalog{}, false, nil
}
func (s *stubDirectory) Applications(_ context.Context) ([]models.ApplicationCatalog, error) {
	return nil, nil
}
func (s *stubDirectory) FindApplication(_ context.Context, _ string) (models.ApplicationCatalog, bool, error) {
	return models.ApplicationCatalog{}, false, nil
}
func (s *stubDirectory) RoleKeysForProject(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}
func (s *stubDirectory) ProjectName(_ context.Context, _ string) (string, error) { return "", nil }
func (s *stubDirectory) Tag() string                                             { return "stub" }
func (s *stubDirectory) InvalidateAll()                                          {}
func (s *stubDirectory) InvalidateProject(_ string)                              {}
func (s *stubDirectory) InvalidateUsers()                                        {}

func TestHandleGetMyProfile_Success(t *testing.T) {
	orig := directory.Default
	directory.Default = &stubDirectory{users: map[string]models.UserProfile{
		"u1": {ID: "u1", Name: "Alice", Email: "alice@x.test", Title: "Director", Team: "Ops", Status: "active"},
	}}
	t.Cleanup(func() { directory.Default = orig })

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/profile", nil)
	req = req.WithContext(withPrincipal(req.Context(), &auth.Principal{Subject: "u1"}))
	rr := httptest.NewRecorder()

	handleGetMyProfile(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var got models.UserProfile
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Title != "Director" || got.Team != "Ops" {
		t.Fatalf("expected metadata-overlay populated, got %+v", got)
	}
}

func TestHandleGetMyProfile_NoActor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/profile", nil)
	rr := httptest.NewRecorder()

	handleGetMyProfile(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when no actor in context, got %d", rr.Code)
	}
}

func TestHandleGetMyProfile_NotFound(t *testing.T) {
	orig := directory.Default
	directory.Default = &stubDirectory{users: map[string]models.UserProfile{}}
	t.Cleanup(func() { directory.Default = orig })

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/profile", nil)
	req = req.WithContext(withPrincipal(req.Context(), &auth.Principal{Subject: "ghost"}))
	rr := httptest.NewRecorder()

	handleGetMyProfile(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}
