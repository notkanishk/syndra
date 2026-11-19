package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"syndra/internal/directory"
	"syndra/internal/models"
)

// failingDirectory answers every lookup with an error that is not "no such user".
type failingDirectory struct{ stubDirectory }

func (*failingDirectory) FindUser(context.Context, string) (models.UserProfile, bool, error) {
	return models.UserProfile{}, false, errors.New("directory: get user: connection reset")
}

func getUserAccess(t *testing.T, src directory.Source, id string) *httptest.ResponseRecorder {
	t.Helper()
	orig := directory.Default
	directory.Default = src
	t.Cleanup(func() { directory.Default = orig })
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+id+"/access", nil)
	req.SetPathValue("id", id)
	rr := httptest.NewRecorder()
	handleGetUserAccess(rr, req)
	return rr
}

// My access says "Zitadel has no account for you" on a 404, so a 404 must mean
// exactly that: Zitadel has no account with this id.
func TestHandleGetUserAccess_NoAccountInZitadelIs404(t *testing.T) {
	rr := getUserAccess(t, &stubDirectory{users: map[string]models.UserProfile{}}, "ghost")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("a person Zitadel has no account for: want 404, got %d (%s)", rr.Code, rr.Body.String())
	}
}

// Any other failure is Syndra failing to read, never "no such person".
func TestHandleGetUserAccess_AFailedReadIsNot404(t *testing.T) {
	rr := getUserAccess(t, &failingDirectory{}, "u1")
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("a failed read: want 500, got %d (%s)", rr.Code, rr.Body.String())
	}
}
