package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mkauth/internal/db"
	"mkauth/internal/models"
	"mkauth/internal/services"
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
	origRuleByID := dbGetMappingRuleByID
	t.Cleanup(func() {
		dbGetDriftItems = origGetItems
		dbGetDriftItem = origGetItem
		dbAttributeDriftAndEnqueue = origAttribute
		dbRevokeDriftAndEnqueue = origRevoke
		dbMarkDriftExternalTx = origMarkExternal
		svcDriftSweep = origSweep
		svcDrainOne = origDrainOne
		dbGetMappingRuleByID = origRuleByID
	})
	// Every triage action now drains its own outbox row — adoption as well as
	// revocation — so the default has to be a no-op rather than the real drain
	// reaching for a database that isn't there. Tests that care override it.
	svcDrainOne = func(context.Context, string) (propagation.DrainResult, error) {
		return propagation.DrainResult{Applied: 1}, nil
	}
}

// --- Adoption records a direct grant, and says only that ---
//
// Attribution used to accept "bundle" and "rule". It wrote a direct_role_grants
// row labelled with that source and nothing else: no bundle assignment, no
// rule-derived relationship. Cascades deliberately never touch the ledger, so
// the label had nothing behind it — the access survived removal of the very
// bundle it claimed to come from.
//
// Routing adoption through real ownership would be worse than the bug: assigning
// a bundle to explain ONE role hands over every other role it carries. So the
// endpoint accepts the one source it can honour.

func pendingDrift() models.DriftItem {
	return models.DriftItem{
		ID: "d1", UserID: "u1", ProjectID: "p_laser",
		RoleKeys: []string{"trained"}, DriftType: "zitadel_only", Status: "pending_triage",
	}
}

// attributeAttempt runs one attribution and reports the status plus whether the
// ledger write happened. Nothing may be written when validation fails.
func attributeAttempt(t *testing.T, body string) (int, bool, db.EnqueueParams) {
	t.Helper()
	var wrote bool
	var params db.EnqueueParams
	dbGetDriftItem = func(context.Context, string) (models.DriftItem, error) { return pendingDrift(), nil }
	dbAttributeDriftAndEnqueue = func(_ context.Context, _ string, p db.EnqueueParams) (string, error) {
		wrote = true
		params = p
		return "ob-1", nil
	}
	req := httptest.NewRequest("POST", "/api/v1/governance/drift/d1/attribute", strings.NewReader(body))
	req.SetPathValue("id", "d1")
	w := httptest.NewRecorder()
	handleAttributeDrift(w, req)
	return w.Code, wrote, params
}

func TestAttributeDrift_BundleSourceIsRejected(t *testing.T) {
	resetDriftDeps(t)
	code, wrote, _ := attributeAttempt(t, `{"source":"bundle"}`)
	if code != http.StatusBadRequest {
		t.Fatalf("bundle attribution cannot be honoured and must be 400, got %d", code)
	}
	if wrote {
		t.Fatal("must not record a bundle provenance it cannot create")
	}
}

func TestAttributeDrift_RuleSourceIsRejected(t *testing.T) {
	resetDriftDeps(t)
	code, wrote, _ := attributeAttempt(t, `{"source":"rule"}`)
	if code != http.StatusBadRequest {
		t.Fatalf("rule attribution cannot be honoured and must be 400, got %d", code)
	}
	if wrote {
		t.Fatal("must not record a rule provenance it cannot create")
	}
}

func TestAttributeDrift_RejectionSaysWhy(t *testing.T) {
	resetDriftDeps(t)
	req := httptest.NewRequest("POST", "/api/v1/governance/drift/d1/attribute",
		strings.NewReader(`{"source":"bundle"}`))
	req.SetPathValue("id", "d1")
	w := httptest.NewRecorder()
	handleAttributeDrift(w, req)

	// A bare "invalid source" leaves an integrator guessing. The reason is the
	// whole point: adoption writes a direct grant and cannot do otherwise.
	if !strings.Contains(w.Body.String(), "cannot create a bundle assignment") {
		t.Fatalf("the rejection must explain itself, got %s", w.Body.String())
	}
}

// A source_ref is no longer part of this contract. Strict decoding rejects it
// loudly rather than accepting a reference that would be silently discarded.
func TestAttributeDrift_SourceRefIsNoLongerAccepted(t *testing.T) {
	resetDriftDeps(t)
	code, wrote, _ := attributeAttempt(t, `{"source":"external_backfill","source_ref":"b1"}`)
	if code != http.StatusBadRequest {
		t.Fatalf("an unknown source_ref field must be 400, got %d", code)
	}
	if wrote {
		t.Fatal("must not write when the request carries a field the contract dropped")
	}
}

func TestAttributeDrift_ExternalBackfillRecordsADirectGrantWithNoRef(t *testing.T) {
	resetDriftDeps(t)
	code, wrote, params := attributeAttempt(t, `{"source":"external_backfill"}`)
	if code != http.StatusOK {
		t.Fatalf("external_backfill is the supported attribution, got %d", code)
	}
	if !wrote {
		t.Fatal("a valid attribution must reach the ledger")
	}
	if params.Source != "external_backfill" {
		t.Fatalf("recorded source = %q, want external_backfill", params.Source)
	}
	// The ledger must not carry a reference to an owner that does not exist.
	if params.SourceRef != "" {
		t.Fatalf("recorded source_ref = %q, want empty", params.SourceRef)
	}
	if params.OpType != "add" {
		t.Fatalf("adoption is an add that self-resolves upstream, got op %q", params.OpType)
	}
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

// Adoption is the operator saying "Zitadel is right, MkAuth was wrong". Leaving
// its outbox row pending said the opposite: the governance queue listed every
// adopted role as a change MkAuth still owed Zitadel, so accepting what Zitadel
// already had produced a queue of writes back to Zitadel. The row must resolve
// in the same request, exactly as the revoke path already did.
func TestHandleAttributeDrift_ResolvesItsOwnOutboxRow(t *testing.T) {
	resetDriftDeps(t)
	dbGetDriftItem = func(context.Context, string) (models.DriftItem, error) { return pendingDrift(), nil }
	dbAttributeDriftAndEnqueue = func(context.Context, string, db.EnqueueParams) (string, error) {
		return "ob_adopt", nil
	}
	var drained string
	svcDrainOne = func(_ context.Context, id string) (propagation.DrainResult, error) {
		drained = id
		return propagation.DrainResult{Applied: 1}, nil
	}

	req := httptest.NewRequest("POST", "/api/v1/governance/drift/d1/attribute",
		strings.NewReader(`{"source":"external_backfill"}`))
	req.SetPathValue("id", "d1")
	w := httptest.NewRecorder()
	handleAttributeDrift(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body)
	}
	if drained != "ob_adopt" {
		t.Fatalf("adoption must drain the row it enqueued, drained=%q", drained)
	}
}

// The bulk path shares attributeOneDrift, so it inherits the drain — but it is
// the path the operator actually used to adopt forty roles at once, and it is
// worth pinning that forty adoptions leave forty resolved rows, not a queue.
func TestHandleBulkAttributeDrift_ResolvesEveryOutboxRow(t *testing.T) {
	resetDriftDeps(t)
	dbGetDriftItem = func(_ context.Context, id string) (models.DriftItem, error) {
		item := pendingDrift()
		item.ID = id
		return item, nil
	}
	dbAttributeDriftAndEnqueue = func(_ context.Context, driftID string, _ db.EnqueueParams) (string, error) {
		return "ob_" + driftID, nil
	}
	var drained []string
	svcDrainOne = func(_ context.Context, id string) (propagation.DrainResult, error) {
		drained = append(drained, id)
		return propagation.DrainResult{Applied: 1}, nil
	}

	req := httptest.NewRequest("POST", "/api/v1/governance/drift/bulk-attribute?apply=true",
		strings.NewReader(`{"ids":["d1","d2","d3"],"source":"external_backfill"}`))
	w := httptest.NewRecorder()
	handleBulkAttributeDrift(w, req)

	if len(drained) != 3 {
		t.Fatalf("every adopted row must be projected, drained=%v", drained)
	}
}

// A drain that cannot reach Zitadel is not an adoption failure. The drift is
// resolved and the ledger records it; the outbox row stays pending because
// MkAuth genuinely could not confirm, and the next drain reclaims it.
func TestHandleAttributeDrift_DrainFailureStillAdopts(t *testing.T) {
	resetDriftDeps(t)
	dbGetDriftItem = func(context.Context, string) (models.DriftItem, error) { return pendingDrift(), nil }
	dbAttributeDriftAndEnqueue = func(context.Context, string, db.EnqueueParams) (string, error) {
		return "ob_adopt", nil
	}
	svcDrainOne = func(context.Context, string) (propagation.DrainResult, error) {
		return propagation.DrainResult{}, errors.New("zitadel unreachable")
	}

	req := httptest.NewRequest("POST", "/api/v1/governance/drift/d1/attribute",
		strings.NewReader(`{"source":"external_backfill"}`))
	req.SetPathValue("id", "d1")
	w := httptest.NewRecorder()
	handleAttributeDrift(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("an unreachable Zitadel must not undo a committed adoption, got %d: %s", w.Code, w.Body)
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
	dbAttributeDriftAndEnqueue = func(context.Context, string, db.EnqueueParams) (string, error) {
		attributed = true
		return "ob-1", nil
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
	dbAttributeDriftAndEnqueue = func(_ context.Context, _ string, p db.EnqueueParams) (string, error) {
		gotSource = p.Source
		return "ob-1", nil
	}

	req := httptest.NewRequest("POST", "/api/v1/governance/drift/bulk-attribute?apply=true", strings.NewReader(`{"ids":["d1"],"source":"external_backfill"}`))
	w := httptest.NewRecorder()
	handleBulkAttributeDrift(w, req)

	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"succeeded":1`) {
		t.Fatalf("valid bulk-attribute must succeed, got %d %s", w.Code, w.Body)
	}
	if gotSource != "external_backfill" {
		t.Fatalf("bulk-attribute must pass the validated source through, got %q", gotSource)
	}
}

// Rehearsal is the default here for the same reason it is on bulk grants:
// adopting writes ledger rows and marking-external suppresses future detection,
// and a triage queue is exactly where an operator is moving fast.
func TestBulkDrift_RehearsesByDefaultAndWritesNothing(t *testing.T) {
	resetDriftDeps(t)
	dbGetDriftItem = func(_ context.Context, id string) (models.DriftItem, error) {
		item := pendingDrift()
		item.ID = id
		return item, nil
	}
	writes := 0
	dbAttributeDriftAndEnqueue = func(context.Context, string, db.EnqueueParams) (string, error) {
		writes++
		return "ob-1", nil
	}

	req := httptest.NewRequest("POST", "/api/v1/governance/drift/bulk-attribute",
		strings.NewReader(`{"ids":["d1","d2"],"source":"external_backfill"}`))
	w := httptest.NewRecorder()
	handleBulkAttributeDrift(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("a rehearsal is a 200, got %d", w.Code)
	}
	if writes != 0 {
		t.Fatalf("a rehearsal must not write, got %d writes", writes)
	}
	var plan services.BulkPlan
	if err := json.Unmarshal(w.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	if plan.Applied {
		t.Error("a rehearsal must not report itself as applied")
	}
	if plan.Summary.Apply != 2 {
		t.Errorf("both rows are actionable, got %+v", plan.Summary)
	}
	// The same plan shape bulk grants uses, so one renderer serves both.
	if len(plan.Outcomes) != 2 || plan.Outcomes[0].Detail == "" {
		t.Errorf("every row must state what would happen: %+v", plan.Outcomes)
	}
}

// A queue two people are working produces this constantly, and it is
// information rather than an error.
func TestBulkDrift_ReportsRowsSomebodyElseAlreadyResolved(t *testing.T) {
	resetDriftDeps(t)
	dbGetDriftItem = func(_ context.Context, id string) (models.DriftItem, error) {
		item := pendingDrift()
		item.ID = id
		if id == "d2" {
			item.Status = "marked_external"
		}
		return item, nil
	}
	writes := 0
	dbAttributeDriftAndEnqueue = func(context.Context, string, db.EnqueueParams) (string, error) {
		writes++
		return "ob-1", nil
	}

	req := httptest.NewRequest("POST", "/api/v1/governance/drift/bulk-attribute?apply=true",
		strings.NewReader(`{"ids":["d1","d2"],"source":"external_backfill"}`))
	w := httptest.NewRecorder()
	handleBulkAttributeDrift(w, req)

	var plan services.BulkPlan
	if err := json.Unmarshal(w.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	if plan.Summary.NoChange != 1 || plan.Summary.Succeeded != 1 {
		t.Fatalf("want 1 already-resolved / 1 applied, got %+v", plan.Summary)
	}
	if writes != 1 {
		t.Errorf("an already-resolved row must not be re-written, got %d writes", writes)
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

// --- Bulk resolutions must report what happened, per id ---

func TestBulkAttributeDrift_NamesTheIdsThatFailed(t *testing.T) {
	resetDriftDeps(t)
	// d2 was resolved by somebody else a moment ago and no longer loads.
	dbGetDriftItem = func(_ context.Context, id string) (models.DriftItem, error) {
		if id == "d2" {
			return models.DriftItem{}, errors.New("no rows in result set")
		}
		item := pendingDrift()
		item.ID = id
		return item, nil
	}
	dbAttributeDriftAndEnqueue = func(context.Context, string, db.EnqueueParams) (string, error) { return "ob-1", nil }

	req := httptest.NewRequest("POST", "/api/v1/governance/drift/bulk-attribute?apply=true",
		strings.NewReader(`{"ids":["d1","d2","d3"],"source":"external_backfill"}`))
	w := httptest.NewRecorder()
	handleBulkAttributeDrift(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("a partially-failing batch is still a 200, got %d", w.Code)
	}
	var plan services.BulkPlan
	if err := json.Unmarshal(w.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	if plan.Summary.Succeeded != 2 || plan.Summary.Blocked != 1 {
		t.Fatalf("want 2 applied / 1 blocked, got %+v", plan.Summary)
	}
	// A count alone would leave the operator unable to retry only what failed;
	// the row itself carries the id and says why.
	blocked := outcomeByID(t, plan, "d2")
	if blocked.Effect != services.EffectBlocked {
		t.Fatalf("the vanished row must be named and blocked, got %+v", blocked)
	}
	if !strings.Contains(blocked.Detail, "No longer in the queue") {
		t.Errorf("it must say why, got %q", blocked.Detail)
	}
}

func outcomeByID(t *testing.T, plan services.BulkPlan, id string) services.BulkOutcome {
	t.Helper()
	for _, o := range plan.Outcomes {
		if o.UserID == id {
			return o
		}
	}
	t.Fatalf("no outcome for %s in %+v", id, plan.Outcomes)
	return services.BulkOutcome{}
}

func TestBulkMarkExternalDrift_NamesTheIdsThatFailed(t *testing.T) {
	resetDriftDeps(t)
	dbGetDriftItem = func(_ context.Context, id string) (models.DriftItem, error) {
		item := pendingDrift()
		item.ID = id
		return item, nil
	}
	dbMarkDriftExternalTx = func(_ context.Context, id, _, _ string, _ []string, _, _, _ string) error {
		if id == "d3" {
			return db.ErrDriftNotPending
		}
		return nil
	}

	req := httptest.NewRequest("POST", "/api/v1/governance/drift/bulk-mark-external?apply=true",
		strings.NewReader(`{"ids":["d1","d3"],"reason":""}`))
	w := httptest.NewRecorder()
	handleBulkMarkDriftExternal(w, req)

	var plan services.BulkPlan
	if err := json.Unmarshal(w.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	if plan.Summary.Succeeded != 1 || plan.Summary.Failed != 1 {
		t.Fatalf("want 1 marked / 1 failed, got %+v", plan.Summary)
	}
	failed := outcomeByID(t, plan, "d3")
	if failed.Effect != services.EffectFailed {
		t.Fatalf("the failing row must be named, got %+v", failed)
	}
	// The cause travels with the row, so a retry is aimed rather than blind.
	if !strings.Contains(failed.Detail, "go through") {
		t.Errorf("the failure must carry its cause, got %q", failed.Detail)
	}
}

func TestBulkResolutions_AlwaysReturnARowPerRequestedID(t *testing.T) {
	resetDriftDeps(t)
	dbGetDriftItem = func(_ context.Context, id string) (models.DriftItem, error) {
		item := pendingDrift()
		item.ID = id
		return item, nil
	}
	dbAttributeDriftAndEnqueue = func(context.Context, string, db.EnqueueParams) (string, error) { return "ob-1", nil }

	req := httptest.NewRequest("POST", "/api/v1/governance/drift/bulk-attribute?apply=true",
		strings.NewReader(`{"ids":["d1","d1",""],"source":"external_backfill"}`))
	w := httptest.NewRecorder()
	handleBulkAttributeDrift(w, req)

	var plan services.BulkPlan
	if err := json.Unmarshal(w.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	// Duplicates and blanks collapse; every distinct id still gets exactly one
	// row, so the plan an operator reads accounts for the whole selection.
	if len(plan.Outcomes) != 1 {
		t.Fatalf("want one row per distinct id, got %+v", plan.Outcomes)
	}
	if plan.Summary.Total != 1 {
		t.Fatalf("the summary must match the rows, got %+v", plan.Summary)
	}
}
