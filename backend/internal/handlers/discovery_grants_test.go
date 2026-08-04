package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"syndra/internal/db"
	"syndra/internal/zitadel"
)

// resetDiscoveryGrantDeps captures/restores the injectables the rewired
// /zitadel/* grant CRUD handlers touch.
func resetDiscoveryGrantDeps(t *testing.T) {
	t.Helper()
	origEnqueue := dbEnqueueDirectGrantPropagation
	origIndex := dbGetGrantIndex
	origLive := dbListUserGrantsLive
	t.Cleanup(func() {
		dbEnqueueDirectGrantPropagation = origEnqueue
		dbGetGrantIndex = origIndex
		dbListUserGrantsLive = origLive
	})
}

func TestHandleAssignZitadelGrant_GoesThroughOutbox(t *testing.T) {
	resetDiscoveryGrantDeps(t)
	var got db.EnqueueParams
	dbEnqueueDirectGrantPropagation = func(_ context.Context, p db.EnqueueParams) (db.EnqueueResult, error) {
		got = p
		return db.EnqueueResult{OutboxID: "obz", IdempotencyKey: "k", Status: "pending"}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/zitadel/users/u1/grants",
		strings.NewReader(`{"projectId":"p1","roleKeys":["r1","r2"]}`))
	req.SetPathValue("id", "u1")
	w := httptest.NewRecorder()
	handleAssignZitadelGrant(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("want 202, got %d: %s", w.Code, w.Body.String())
	}
	if got.OpType != "add" || got.UserID != "u1" || got.ProjectID != "p1" || len(got.RoleKeys) != 2 {
		t.Fatalf("unexpected enqueue params: %+v", got)
	}
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["outbox_id"] != "obz" {
		t.Fatalf("unexpected body: %v", body)
	}
}

func TestHandleUpdateZitadelGrant_EnqueuesReplace(t *testing.T) {
	resetDiscoveryGrantDeps(t)
	dbGetGrantIndex = func(_ context.Context, grantID string) (db.ZitadelGrantIndex, error) {
		return db.ZitadelGrantIndex{GrantID: grantID, UserID: "u1", ProjectID: "p1", RoleKeys: []string{"old"}}, nil
	}
	var got db.EnqueueParams
	dbEnqueueDirectGrantPropagation = func(_ context.Context, p db.EnqueueParams) (db.EnqueueResult, error) {
		got = p
		return db.EnqueueResult{OutboxID: "obu", Status: "pending"}, nil
	}

	req := httptest.NewRequest(http.MethodPut, "/api/v1/zitadel/users/u1/grants/g1",
		strings.NewReader(`{"roleKeys":["r9"]}`))
	req.SetPathValue("id", "u1")
	req.SetPathValue("grantId", "g1")
	w := httptest.NewRecorder()
	handleUpdateZitadelGrant(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("want 202, got %d: %s", w.Code, w.Body.String())
	}
	if got.OpType != "replace" || got.ZitadelGrantID != "g1" || got.ProjectID != "p1" || got.RoleKeys[0] != "r9" {
		t.Fatalf("unexpected replace enqueue: %+v", got)
	}
}

func TestHandleRemoveZitadelGrant_EnqueuesRevokeWithResolvedRoles(t *testing.T) {
	resetDiscoveryGrantDeps(t)
	// Index miss → fall back to live lookup for project/roles.
	dbGetGrantIndex = func(_ context.Context, _ string) (db.ZitadelGrantIndex, error) {
		return db.ZitadelGrantIndex{}, db.ErrGrantIndexNotFound
	}
	dbListUserGrantsLive = func(_ context.Context, userID, grantID string) (zitadel.UserGrant, error) {
		return zitadel.UserGrant{ID: grantID, UserID: userID, ProjectID: "p7", RoleKeys: []string{"member", "lead"}}, nil
	}
	var got db.EnqueueParams
	dbEnqueueDirectGrantPropagation = func(_ context.Context, p db.EnqueueParams) (db.EnqueueResult, error) {
		got = p
		return db.EnqueueResult{OutboxID: "obr", Status: "pending"}, nil
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/zitadel/users/u1/grants/g1", nil)
	req.SetPathValue("id", "u1")
	req.SetPathValue("grantId", "g1")
	w := httptest.NewRecorder()
	handleRemoveZitadelGrant(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("want 202, got %d: %s", w.Code, w.Body.String())
	}
	if got.OpType != "revoke" || got.ZitadelGrantID != "g1" || got.ProjectID != "p7" || len(got.RoleKeys) != 2 {
		t.Fatalf("unexpected revoke enqueue (roles must resolve via live lookup): %+v", got)
	}
}
