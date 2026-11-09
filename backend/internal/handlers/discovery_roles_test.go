package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"syndra/internal/models"
	"syndra/internal/services"
)

// The Zitadel › Projects role routes are thin callers of the one role pipe
// (services/roles.go). These pin that they call it, and how they answer its
// refusals; the pipe's own behaviour is tested in services.
func stubRolePipe(t *testing.T) {
	t.Helper()
	origCreate, origUpdate, origDelete := svcCreateRole, svcUpdateRole, svcDeleteRole
	t.Cleanup(func() { svcCreateRole, svcUpdateRole, svcDeleteRole = origCreate, origUpdate, origDelete })
}

func serveRole(h http.HandlerFunc, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.SetPathValue("id", "p1")
	req.SetPathValue("key", "trained")
	w := httptest.NewRecorder()
	h(w, req)
	return w
}

func TestZitadelRoleCreate_GoesThroughTheRolePipe(t *testing.T) {
	stubRolePipe(t)
	var got services.CreateRoleRequest
	svcCreateRole = func(_ context.Context, req services.CreateRoleRequest, _ string) (models.Role, error) {
		got = req
		return models.Role{}, nil
	}
	w := serveRole(handleCreateZitadelProjectRole, http.MethodPost, "/api/v1/zitadel/projects/p1/roles",
		`{"roleKey":" trained ","displayName":"Trained","group":"g"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d: %s", w.Code, w.Body.String())
	}
	if got.ProjectID != "p1" || got.RoleKey != "trained" || got.DisplayName != "Trained" || got.Group != "g" {
		t.Fatalf("unexpected request to the pipe: %+v", got)
	}
}

func TestZitadelRoleCreate_InvalidKeyIs400(t *testing.T) {
	stubRolePipe(t)
	svcCreateRole = func(context.Context, services.CreateRoleRequest, string) (models.Role, error) {
		return models.Role{}, services.ErrInvalidRoleKey
	}
	w := serveRole(handleCreateZitadelProjectRole, http.MethodPost, "/", `{"roleKey":"a b"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestZitadelRoleUpdate_ZitadelRefusalIs502(t *testing.T) {
	stubRolePipe(t)
	svcUpdateRole = func(context.Context, string, string, string, string, string) error {
		return fmt.Errorf("%w: zitadel api 400", services.ErrRoleNotChangedInZitadel)
	}
	w := serveRole(handleUpdateZitadelProjectRole, http.MethodPut, "/", `{"displayName":"New"}`)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("want 502, got %d", w.Code)
	}
}

// C6: an unknown outcome has its own code and never says nothing changed.
func TestZitadelRoleDelete_UnknownOutcomeIsItsOwnAnswer(t *testing.T) {
	stubRolePipe(t)
	svcDeleteRole = func(context.Context, string, string, string, int) error {
		return fmt.Errorf("%w: context deadline exceeded", services.ErrRoleChangeUnknown)
	}
	w := serveRole(handleDeleteZitadelProjectRole, http.MethodDelete, "/", "")
	var body ErrorResponse
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if w.Code != http.StatusBadGateway || body.Error != roleChangeUnknownCode {
		t.Fatalf("want 502 %s, got %d: %s", roleChangeUnknownCode, w.Code, w.Body.String())
	}
	if strings.Contains(body.Message, "Nothing was changed") || !strings.Contains(body.Message, "can't tell") {
		t.Fatalf("an unknown outcome must say so, never that nothing changed: %q", body.Message)
	}
	if strings.Contains(body.Message, "deadline") {
		t.Fatalf("raw error text leaked: %q", body.Message)
	}
}

func TestZitadelRoleDelete_HeldNamesWhoAndHowMany(t *testing.T) {
	stubRolePipe(t)
	var confirmed int
	svcDeleteRole = func(_ context.Context, _, _, _ string, confirm int) error {
		confirmed = confirm
		if confirm != 2 {
			return &services.RoleHeldError{Holders: []models.UserProfile{{ID: "u1", Name: "Charlotte Flair"}, {ID: "u2", Email: "sami@example.org"}}}
		}
		return nil
	}

	w := serveRole(handleDeleteZitadelProjectRole, http.MethodDelete, "/api/v1/zitadel/projects/p1/roles/trained", "")
	if w.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d: %s", w.Code, w.Body.String())
	}
	var body ErrorResponse
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body.Error != "ROLE_HELD" || body.Details["count"] != "2" || body.Details["holders"] != "Charlotte Flair, sami@example.org" {
		t.Fatalf("the refusal must name who and how many: %+v", body)
	}
	if confirmed != 0 {
		t.Fatalf("no confirm_holders means zero, got %d", confirmed)
	}

	w = serveRole(handleDeleteZitadelProjectRole, http.MethodDelete, "/api/v1/zitadel/projects/p1/roles/trained?confirm_holders=2", "")
	if w.Code != http.StatusOK || confirmed != 2 {
		t.Fatalf("a confirmed count must reach the pipe: %d, confirm=%d", w.Code, confirmed)
	}
}

func TestZitadelRoleDelete_InUseAndUnknownAreRefusals(t *testing.T) {
	stubRolePipe(t)
	cases := map[error]string{
		&services.RoleInUseError{Given: 1, Bundles: 2}: "ROLE_IN_USE",
		&services.RoleHeldError{Unknown: true}:         "HOLDERS_UNKNOWN",
	}
	for err, code := range cases {
		svcDeleteRole = func(context.Context, string, string, string, int) error { return err }
		w := serveRole(handleDeleteZitadelProjectRole, http.MethodDelete, "/", "")
		if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), code) {
			t.Fatalf("want 409 %s, got %d: %s", code, w.Code, w.Body.String())
		}
	}
}

func TestZitadelRoleDelete_BadConfirmIs400(t *testing.T) {
	stubRolePipe(t)
	svcDeleteRole = func(context.Context, string, string, string, int) error {
		t.Fatal("a bad confirm must not reach the pipe")
		return nil
	}
	w := serveRole(handleDeleteZitadelProjectRole, http.MethodDelete, "/?confirm_holders=-1", "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}
