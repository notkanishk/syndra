package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"syndra/internal/db"
	"syndra/internal/services"
)

func TestHandleDeleteUserDirectGrant_DeletesAndQueuesRevoke(t *testing.T) {
	origDelete, origRebuild := svcDeleteDirectGrant, rebuildUserCacheDetachedFn
	t.Cleanup(func() {
		svcDeleteDirectGrant = origDelete
		rebuildUserCacheDetachedFn = origRebuild
	})

	var gotUser, gotGrant, gotActor string
	svcDeleteDirectGrant = func(_ context.Context, userID, grantID, actor string) (services.DirectGrantRemoval, error) {
		gotUser, gotGrant, gotActor = userID, grantID, actor
		return services.DirectGrantRemoval{
			OutboxIDs: []string{"ob_1"},
			Revoked:   []string{"pLaser/trained"},
			Retained:  []string{},
			Status:    "pending",
		}, nil
	}
	var rebuilt string
	rebuildUserCacheDetachedFn = func(_ context.Context, userID string) { rebuilt = userID }

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/users/u_2f81/grants/g_88", nil)
	req.SetPathValue("id", "u_2f81")
	req.SetPathValue("grantId", "g_88")
	rr := httptest.NewRecorder()
	handleDeleteUserDirectGrant(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d (%s)", rr.Code, rr.Body.String())
	}
	if gotUser != "u_2f81" || gotGrant != "g_88" {
		t.Errorf("wrong grant targeted: user=%q grant=%q", gotUser, gotGrant)
	}
	if gotActor == "" {
		t.Error("the actor must be recorded — a revoke with no attributable actor is unauditable")
	}
	// Without the recompile the token path keeps serving the role until the
	// next unrelated cache rebuild, so the removal would appear to do nothing.
	if rebuilt != "u_2f81" {
		t.Errorf("expected the user's cache to be rebuilt, got %q", rebuilt)
	}

	var res services.DirectGrantRemoval
	if err := json.Unmarshal(rr.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(res.OutboxIDs) != 1 || res.OutboxIDs[0] != "ob_1" || res.Status != "pending" {
		t.Errorf("expected the outbox handles back, got %#v", res)
	}
}

// Removing a grant that is already gone is an ordinary race (two operators,
// or a click after the expiry sweep), not a server fault.
func TestHandleDeleteUserDirectGrant_MissingGrantIs404(t *testing.T) {
	orig := svcDeleteDirectGrant
	t.Cleanup(func() { svcDeleteDirectGrant = orig })
	svcDeleteDirectGrant = func(context.Context, string, string, string) (services.DirectGrantRemoval, error) {
		return services.DirectGrantRemoval{}, db.ErrGrantNotFound
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/users/u1/grants/nope", nil)
	req.SetPathValue("id", "u1")
	req.SetPathValue("grantId", "nope")
	rr := httptest.NewRecorder()
	handleDeleteUserDirectGrant(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for an already-removed grant, got %d", rr.Code)
	}
}

func TestHandleDeleteUserDirectGrant_RequiresBothPathValues(t *testing.T) {
	orig := svcDeleteDirectGrant
	t.Cleanup(func() { svcDeleteDirectGrant = orig })
	svcDeleteDirectGrant = func(context.Context, string, string, string) (services.DirectGrantRemoval, error) {
		t.Error("service called without a grant id")
		return services.DirectGrantRemoval{}, nil
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/users/u1/grants/", nil)
	req.SetPathValue("id", "u1")
	rr := httptest.NewRecorder()
	handleDeleteUserDirectGrant(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestHandleGetGovernanceIndicators_ReturnsScalars(t *testing.T) {
	orig := svcGovernanceIndicators
	t.Cleanup(func() { svcGovernanceIndicators = orig })
	svcGovernanceIndicators = func(context.Context) (services.Indicators, error) {
		return services.Indicators{PendingRequests: 3, ExpiringGrants: 1, PendingPropagation: 2, Drift: 12}, nil
	}

	rr := httptest.NewRecorder()
	handleGetGovernanceIndicators(rr, httptest.NewRequest(http.MethodGet, "/api/v1/governance/indicators", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// The whole point is that the rail gets numbers, not arrays it must count.
	for key, want := range map[string]float64{
		"pending_requests": 3, "expiring_grants": 1, "pending_propagation": 2, "drift": 12,
	} {
		if got[key] != want {
			t.Errorf("%s: got %v want %v", key, got[key], want)
		}
	}
}

func TestHandleGetRoleMembers_RequiresProjectAndKey(t *testing.T) {
	orig := svcRoleMembers
	t.Cleanup(func() { svcRoleMembers = orig })
	svcRoleMembers = func(context.Context, string, string) (services.RoleMembersView, error) {
		t.Error("service called without both path values")
		return services.RoleMembersView{}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/p1/roles//members", nil)
	req.SetPathValue("id", "p1")
	rr := httptest.NewRecorder()
	handleGetRoleMembers(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestHandleGetRoleMembers_PassesThroughView(t *testing.T) {
	orig := svcRoleMembers
	t.Cleanup(func() { svcRoleMembers = orig })
	svcRoleMembers = func(_ context.Context, projectID, key string) (services.RoleMembersView, error) {
		if projectID != "pLaser" || key != "trained" {
			return services.RoleMembersView{}, fmt.Errorf("unexpected role %s/%s", projectID, key)
		}
		return services.RoleMembersView{ProjectID: projectID, RoleKey: key, DirectCount: 3}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/pLaser/roles/trained/members", nil)
	req.SetPathValue("id", "pLaser")
	req.SetPathValue("key", "trained")
	rr := httptest.NewRecorder()
	handleGetRoleMembers(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	var got services.RoleMembersView
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.DirectCount != 3 {
		t.Errorf("expected the view through unchanged, got %#v", got)
	}
}
