package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"syndra/internal/db"
	"syndra/internal/services"
)

func removeGrant(query string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/users/u_2f81/grants/g_88"+query, nil)
	req.SetPathValue("id", "u_2f81")
	req.SetPathValue("grantId", "g_88")
	rr := httptest.NewRecorder()
	handleDeleteUserDirectGrant(rr, req)
	return rr
}

func revokeWrite(role, outbox string) services.Write {
	return services.Write{EnqueueParams: db.EnqueueParams{OpType: "revoke", ProjectID: "pLaser", RoleKeys: []string{role}}, OutboxID: outbox}
}

// A removal is the planner's act (the ledger row goes and the revokes are queued, in one
// transaction: services' tests). The handler hands it the grant, answers with what the Change
// revoked and kept, and rebuilds the cache before it sends.
func TestHandleDeleteUserDirectGrant_PlansTheRemovalRebuildsThenSends(t *testing.T) {
	probe := stubAct(t)
	probe.Change = services.Change{ProjectID: "pLaser", RoleKey: "trained",
		Writes: []services.Write{revokeWrite("trained", "ob_1")}}

	rr := removeGrant("")

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d (%s)", rr.Code, rr.Body.String())
	}
	a, w, seen := probe.only(t)
	if a != (services.Action{Op: services.OpRemoveGrant, UserID: "u_2f81", GrantID: "g_88"}) {
		t.Errorf("wrong grant targeted: %+v", a)
	}
	if w.Actor == "" {
		t.Error("the actor must be recorded: a revoke with no attributable actor is unauditable")
	}
	if seen != nil {
		t.Errorf("no fingerprint was sent, so nothing was seen: %+v", seen)
	}
	// Without the recompile the token path keeps serving the role until the
	// next unrelated cache rebuild, so the removal would appear to do nothing.
	probe.wantOrder(t, "apply", "rebuild", "deliver")
	if len(probe.RebuildUsers) != 1 || probe.RebuildUsers[0] != "u_2f81" || !probe.RebuildDetached {
		t.Errorf("expected the user's cache rebuilt on a detached context, got %v detached=%v", probe.RebuildUsers, probe.RebuildDetached)
	}

	var res services.DirectGrantRemoval
	if err := json.Unmarshal(rr.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(res.OutboxIDs) != 1 || res.OutboxIDs[0] != "ob_1" || res.Status != "pending" ||
		len(res.Revoked) != 1 || res.Revoked[0] != "pLaser/trained" || len(res.Retained) != 0 {
		t.Errorf("expected the outbox handles and what was revoked back, got %#v", res)
	}
}

// A role the person still holds another way is never revoked: the answer says it is kept,
// and what the send did to the row is its status.
func TestHandleDeleteUserDirectGrant_ARoleHeldAnotherWayIsRetained(t *testing.T) {
	probe := stubAct(t)
	// Removing the grant of trained also takes back a role a rule gave; trained itself stays.
	probe.Change = services.Change{ProjectID: "pLaser", RoleKey: "trained",
		Writes: []services.Write{revokeWrite("bench", "ob_9")}}
	probe.Delivery = services.Delivery{Sent: []string{"ob_9"}, Waiting: []string{"ob_x"}, Reason: map[string]string{"ob_x": "role_waits"}}
	orig := dbGetPropagationStatus
	t.Cleanup(func() { dbGetPropagationStatus = orig })
	dbGetPropagationStatus = func(context.Context, string) (string, error) { return "failed", nil }

	rr := removeGrant("")

	var res services.DirectGrantRemoval
	if err := json.Unmarshal(rr.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(res.Revoked) != 1 || res.Revoked[0] != "pLaser/bench" || len(res.Retained) != 1 || res.Retained[0] != "pLaser/trained" {
		t.Errorf("revoked and retained: %#v", res)
	}
	if res.Status != "failed" || res.WaitReason != "role_waits" {
		t.Errorf("the send's outcome is said: %#v", res)
	}
}

// A grant that was only waiting is cancelled: nothing is revoked or sent, and the lists are
// lists.
func TestHandleDeleteUserDirectGrant_ACancelledGiveRevokesNothing(t *testing.T) {
	probe := stubAct(t)
	probe.Change = services.Change{ProjectID: "pLaser", RoleKey: "trained"}

	rr := removeGrant("")

	body := decodeMap(t, rr.Body.String())
	if rr.Code != http.StatusAccepted || body["revoked_roles"] == nil || body["retained_roles"] == nil {
		t.Fatalf("got %d %v", rr.Code, body)
	}
	if l, _ := body["revoked_roles"].([]any); len(l) != 0 {
		t.Fatalf("nothing revoked: %v", body)
	}
}

// The plan the sheet was shown rides in the query: ?fingerprint= and ?cancel_ids=.
func TestHandleDeleteUserDirectGrant_CarriesWhatTheSheetWasShownInTheQuery(t *testing.T) {
	probe := stubAct(t)
	const id = "55555555-5555-5555-5555-555555555555"

	removeGrant("?fingerprint=fp-5&cancel_ids=" + id + ",not-an-id")

	_, _, seen := probe.only(t)
	if seen == nil || seen.Fingerprint != "fp-5" || len(seen.CancelIDs) != 1 || seen.CancelIDs[0] != id {
		t.Fatalf("seen = %+v (an id that is not one is dropped)", seen)
	}
}

// Removing a grant that is already gone is an ordinary race (two operators,
// or a click after the expiry sweep), not a server fault. The planner says it as "not_held".
func TestHandleDeleteUserDirectGrant_MissingGrantIs404(t *testing.T) {
	probe := stubAct(t)
	for _, err := range []error{db.ErrGrantNotFound, &services.Refused{Change: services.Change{Refusal: "not_held"}}} {
		probe.Err = err
		wantError(t, removeGrant(""), http.StatusNotFound, "GRANT_NOT_FOUND")
	}
	probe.wantOrder(t, "apply", "apply")
}

func TestHandleDeleteUserDirectGrant_StaleIs409AndFailureIs500(t *testing.T) {
	probe := stubAct(t)

	probe.Err = &services.StalePlan{IDs: []string{"q1"}}
	wantError(t, removeGrant("?fingerprint=old"), http.StatusConflict, "PLAN_STALE")

	probe.Err = errors.New("tx failed")
	wantError(t, removeGrant(""), http.StatusInternalServerError, "DB_ERROR")
	probe.wantOrder(t, "apply", "apply") // nothing rebuilt, nothing sent
}

func TestHandleDeleteUserDirectGrant_RequiresBothPathValues(t *testing.T) {
	probe := stubAct(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/users/u1/grants/", nil)
	req.SetPathValue("id", "u1")
	rr := httptest.NewRecorder()
	handleDeleteUserDirectGrant(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	if len(probe.Actions) != 0 {
		t.Error("planner called without a grant id")
	}
}

func TestHandleGetGovernanceIndicators_ReturnsScalars(t *testing.T) {
	orig := svcGovernanceIndicators
	t.Cleanup(func() { svcGovernanceIndicators = orig })
	svcGovernanceIndicators = func(context.Context, string) (services.Indicators, error) {
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
