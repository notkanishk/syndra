package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mkauth/internal/db"
	"mkauth/internal/models"
	"mkauth/internal/services/drift"
	"mkauth/internal/services/propagation"
)

func resetDriftDeps(t *testing.T) {
	t.Helper()
	origGetItems := dbGetDriftItems
	origGetItem := dbGetDriftItem
	origAttribute := dbAttributeDriftAndEnqueue
	origRevoke := dbRevokeDriftAndEnqueue
	origMarkExternal := dbMarkDriftExternalTx
	origSweep := svcDriftSweep
	origDrainOne := svcDrainOne
	origRolesForBundle := svcGetRolesForBundleDrift
	t.Cleanup(func() {
		dbGetDriftItems = origGetItems
		dbGetDriftItem = origGetItem
		dbAttributeDriftAndEnqueue = origAttribute
		dbRevokeDriftAndEnqueue = origRevoke
		dbMarkDriftExternalTx = origMarkExternal
		svcDriftSweep = origSweep
		svcDrainOne = origDrainOne
		svcGetRolesForBundleDrift = origRolesForBundle
	})
}

func TestHandleMarkExternal_ResolvesAtomically(t *testing.T) {
	resetDriftDeps(t)
	dbGetDriftItem = func(context.Context, string) (models.DriftItem, error) {
		return models.DriftItem{ID: "d1", UserID: "u1", ProjectID: "p1", RoleKeys: []string{"viewer"}, DriftType: "zitadel_only", Status: "pending_triage"}, nil
	}
	var gotUser, gotRole string
	dbMarkDriftExternalTx = func(_ context.Context, _, user, _ string, roles []string, _, _, _ string) error {
		gotUser = user
		if len(roles) > 0 {
			gotRole = roles[0]
		}
		return nil
	}

	req := httptest.NewRequest("POST", "/api/v1/governance/drift/d1/mark-external", strings.NewReader(`{"reason":"partner org"}`))
	req.SetPathValue("id", "d1")
	w := httptest.NewRecorder()
	handleMarkDriftExternal(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body)
	}
	if gotUser != "u1" || gotRole != "viewer" {
		t.Fatalf("mark-external must pass the drift triple to the atomic tx helper (user=%q role=%q)", gotUser, gotRole)
	}
}

func TestHandleRevokeDrift_EnqueuesRevokeAtomicallyThenDrains(t *testing.T) {
	resetDriftDeps(t)
	dbGetDriftItem = func(context.Context, string) (models.DriftItem, error) {
		return models.DriftItem{ID: "d1", UserID: "u1", ProjectID: "p1", RoleKeys: []string{"viewer"}, ZitadelGrantID: "g1", DriftType: "zitadel_only", Status: "pending_triage"}, nil
	}
	var gotOp string
	dbRevokeDriftAndEnqueue = func(_ context.Context, _ string, p db.EnqueueParams) (string, error) {
		gotOp = p.OpType
		return "o1", nil
	}
	var drained string
	svcDrainOne = func(_ context.Context, id string) (propagation.DrainResult, error) {
		drained = id
		return propagation.DrainResult{Applied: 1}, nil
	}

	req := httptest.NewRequest("POST", "/api/v1/governance/drift/d1/revoke", nil)
	req.SetPathValue("id", "d1")
	w := httptest.NewRecorder()
	handleRevokeDrift(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body)
	}
	if gotOp != "revoke" || drained != "o1" {
		t.Fatalf("revoke must enqueue op=revoke atomically then drain that row (op=%q drained=%q)", gotOp, drained)
	}
}

func TestHandleAttributeToBundle_RejectsBundleWithoutRole(t *testing.T) {
	resetDriftDeps(t)
	dbGetDriftItem = func(context.Context, string) (models.DriftItem, error) {
		return models.DriftItem{ID: "d1", UserID: "u1", ProjectID: "p1", RoleKeys: []string{"viewer"}, Status: "pending_triage"}, nil
	}
	// The chosen bundle does NOT contain the drift role → source-remap validation fails.
	svcGetRolesForBundleDrift = func(context.Context, string) ([]models.BundleRole, error) {
		return []models.BundleRole{{ProjectID: "p1", RoleKey: "editor"}}, nil
	}

	req := httptest.NewRequest("POST", "/api/v1/governance/drift/d1/attribute", strings.NewReader(`{"source":"bundle","source_ref":"b1"}`))
	req.SetPathValue("id", "d1")
	w := httptest.NewRecorder()
	handleAttributeDrift(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("attributing to a bundle lacking the drift role must be 400, got %d", w.Code)
	}
}

func TestHandleRevokeDrift_LostRaceIs409(t *testing.T) {
	resetDriftDeps(t)
	dbGetDriftItem = func(context.Context, string) (models.DriftItem, error) {
		return models.DriftItem{ID: "d1", UserID: "u1", ProjectID: "p1", RoleKeys: []string{"viewer"}, ZitadelGrantID: "g1", Status: "pending_triage"}, nil
	}
	// The atomic claim+enqueue's guarded UPDATE matched nothing (already resolved
	// by another operator); the whole tx rolled back — nothing was written.
	dbRevokeDriftAndEnqueue = func(context.Context, string, db.EnqueueParams) (string, error) { return "", db.ErrDriftNotPending }
	var drained bool
	svcDrainOne = func(context.Context, string) (propagation.DrainResult, error) {
		drained = true
		return propagation.DrainResult{}, nil
	}

	req := httptest.NewRequest("POST", "/api/v1/governance/drift/d1/revoke", nil)
	req.SetPathValue("id", "d1")
	w := httptest.NewRecorder()
	handleRevokeDrift(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("a lost triage race must be 409, got %d", w.Code)
	}
	if drained {
		t.Fatal("no drain when the atomic claim+enqueue tx rolled back")
	}
}

func TestHandleMarkExternal_MalformedBodyIs400(t *testing.T) {
	resetDriftDeps(t)
	dbGetDriftItem = func(context.Context, string) (models.DriftItem, error) {
		return models.DriftItem{ID: "d1", UserID: "u1", ProjectID: "p1", RoleKeys: []string{"viewer"}, Status: "pending_triage"}, nil
	}
	var marked bool
	dbMarkDriftExternalTx = func(context.Context, string, string, string, []string, string, string, string) error {
		marked = true
		return nil
	}

	// Unknown field → decodeJSONStrict errors (not io.EOF) → must 400 before any write.
	req := httptest.NewRequest("POST", "/api/v1/governance/drift/d1/mark-external", strings.NewReader(`{"bogus":"x"}`))
	req.SetPathValue("id", "d1")
	w := httptest.NewRecorder()
	handleMarkDriftExternal(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("malformed mark-external body must be 400, got %d", w.Code)
	}
	if marked {
		t.Fatal("mark-external must not suppress detection on garbage input")
	}
}

func TestHandleMarkExternal_EmptyBodyIsAllowed(t *testing.T) {
	resetDriftDeps(t)
	dbGetDriftItem = func(context.Context, string) (models.DriftItem, error) {
		return models.DriftItem{ID: "d1", UserID: "u1", ProjectID: "p1", RoleKeys: []string{"viewer"}, Status: "pending_triage"}, nil
	}
	var gotReason string
	var marked bool
	dbMarkDriftExternalTx = func(_ context.Context, _, _, _ string, _ []string, _, reason, _ string) error {
		marked = true
		gotReason = reason
		return nil
	}

	// Empty body (io.EOF) is fine — reason is optional.
	req := httptest.NewRequest("POST", "/api/v1/governance/drift/d1/mark-external", nil)
	req.SetPathValue("id", "d1")
	w := httptest.NewRecorder()
	handleMarkDriftExternal(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("empty mark-external body must be allowed (200), got %d: %s", w.Code, w.Body)
	}
	if !marked || gotReason != "" {
		t.Fatalf("empty body must mark-external with empty reason (marked=%v reason=%q)", marked, gotReason)
	}
}

func TestHandleBulkAttributeDrift_MissingSourceIs400(t *testing.T) {
	resetDriftDeps(t)
	var attributed bool
	dbGetDriftItem = func(context.Context, string) (models.DriftItem, error) {
		return models.DriftItem{ID: "d1", UserID: "u1", ProjectID: "p1", RoleKeys: []string{"viewer"}, Status: "pending_triage"}, nil
	}
	dbAttributeDriftAndEnqueue = func(context.Context, string, db.EnqueueParams) error {
		attributed = true
		return nil
	}

	// Source omitted → must 400 before the loop, never defaulting to "direct".
	req := httptest.NewRequest("POST", "/api/v1/governance/drift/bulk-attribute", strings.NewReader(`{"ids":["d1"]}`))
	w := httptest.NewRecorder()
	handleBulkAttributeDrift(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("bulk-attribute with missing source must be 400, got %d", w.Code)
	}
	if attributed {
		t.Fatal("bulk-attribute must not enqueue anything when source is invalid")
	}
}

func TestHandleBulkAttributeDrift_ValidSourceAttributes(t *testing.T) {
	resetDriftDeps(t)
	dbGetDriftItem = func(context.Context, string) (models.DriftItem, error) {
		return models.DriftItem{ID: "d1", UserID: "u1", ProjectID: "p1", RoleKeys: []string{"viewer"}, Status: "pending_triage"}, nil
	}
	var gotSource string
	dbAttributeDriftAndEnqueue = func(_ context.Context, _ string, p db.EnqueueParams) error {
		gotSource = p.Source
		return nil
	}

	req := httptest.NewRequest("POST", "/api/v1/governance/drift/bulk-attribute", strings.NewReader(`{"ids":["d1"],"source":"external_backfill"}`))
	w := httptest.NewRecorder()
	handleBulkAttributeDrift(w, req)

	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"attributed":1`) {
		t.Fatalf("valid bulk-attribute must succeed, got %d %s", w.Code, w.Body)
	}
	if gotSource != "external_backfill" {
		t.Fatalf("bulk-attribute must pass the validated source through, got %q", gotSource)
	}
}

func TestHandleReconcileNow_TriggersSweep(t *testing.T) {
	resetDriftDeps(t)
	svcDriftSweep = func(context.Context) (drift.DriftResult, error) { return drift.DriftResult{DriftItemsCreated: 2}, nil }
	req := httptest.NewRequest("POST", "/api/v1/governance/drift/reconcile", nil)
	w := httptest.NewRecorder()
	handleReconcileDrift(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"drift_items_created":2`) {
		t.Fatalf("reconcile-now must run the sweep, got %d %s", w.Code, w.Body)
	}
}
