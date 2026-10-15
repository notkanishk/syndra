package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"syndra/internal/db"
	"syndra/internal/models"
)

// Never invisibly (owner, 15 Oct): a request that would cancel a queued
// change it wasn't shown is answered with the refusal and the list, whatever
// the handler wrote; shown exactly those, it goes ahead.

const queuedID = "0b3c7a52-6f0e-4b8e-9a51-2d9b1c3e4f50"

func cancellingHandler(admitted *bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		*admitted = db.AdmitCancellations(r.Context(), []models.QueuedChange{{ID: queuedID, OpType: "revoke", RoleKeys: []string{"community"}}})
		if !*admitted {
			jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", db.ErrWouldCancel.Error())
			return
		}
		jsonResponse(w, http.StatusAccepted, map[string]string{"status": "pending"})
	}
}

func TestACancellationNobodyWasShownIsRefusedWithTheList(t *testing.T) {
	var admitted bool
	rr := httptest.NewRecorder()
	withCancelGate(cancellingHandler(&admitted))(rr, httptest.NewRequest(http.MethodPost, "/api/v1/users/u1/grants", nil))
	if admitted || rr.Code != http.StatusConflict {
		t.Fatalf("want 409 and nothing admitted, got %d (admitted %v)", rr.Code, admitted)
	}
	var body WouldCancelResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("one JSON body, the refusal's: %v (%s)", err, rr.Body.String())
	}
	if body.Error != "WOULD_CANCEL_QUEUED" || len(body.Cancels) != 1 || body.Cancels[0].ID != queuedID {
		t.Fatalf("the refusal lists what it would cancel, got %+v", body)
	}
}

func TestACancellationTheOperatorWasShownGoesAhead(t *testing.T) {
	var admitted bool
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/u1/grants", nil)
	req.Header.Set(cancelAckHeader, queuedID)
	rr := httptest.NewRecorder()
	withCancelGate(cancellingHandler(&admitted))(rr, req)
	if !admitted || rr.Code != http.StatusAccepted {
		t.Fatalf("acknowledged, it goes ahead: %d (admitted %v)", rr.Code, admitted)
	}

	// An acknowledgement of something else doesn't stretch to this.
	req = httptest.NewRequest(http.MethodPost, "/api/v1/users/u1/grants", nil)
	req.Header.Set(cancelAckHeader, "11111111-2222-3333-4444-555555555555")
	rr = httptest.NewRecorder()
	withCancelGate(cancellingHandler(&admitted))(rr, req)
	if admitted || rr.Code != http.StatusConflict {
		t.Fatalf("a different acknowledgement is no acknowledgement: %d", rr.Code)
	}
}

func TestAReviewedPlanAndSystemWorkNeedNoAcknowledgement(t *testing.T) {
	var admitted bool
	reviewed := func(w http.ResponseWriter, r *http.Request) {
		db.CancellationsReviewed(r.Context())
		cancellingHandler(&admitted)(w, r)
	}
	rr := httptest.NewRecorder()
	withCancelGate(reviewed)(rr, httptest.NewRequest(http.MethodPost, "/api/v1/grants/bulk?apply=true", nil))
	if !admitted || rr.Code != http.StatusAccepted {
		t.Fatalf("a reviewed plan's rows named its cancellations: %d", rr.Code)
	}
	// No gate at all (expiry, onboarding): admitted, and audited by the chokepoint.
	if !db.AdmitCancellations(httptest.NewRequest(http.MethodGet, "/", nil).Context(), []models.QueuedChange{{ID: queuedID}}) {
		t.Fatal("system work has no operator to ask")
	}
}

// A member's own request (the welcome bundle on first sign-in) has nobody to
// ask: it goes ahead and the chokepoint audits it, as it does system work.
func TestAMembersOwnRequestIsNotAsked(t *testing.T) {
	t.Setenv("ZITADEL_DOMAIN", "example.zitadel.cloud")
	var admitted bool
	rr := httptest.NewRecorder()
	withCancelGate(cancellingHandler(&admitted))(rr, memberContext(httptest.NewRequest(http.MethodPost, "/api/v1/me/onboard", nil), "member-1"))
	if !admitted || rr.Code != http.StatusAccepted {
		t.Fatalf("a member is never asked: %d (admitted %v)", rr.Code, admitted)
	}
}
