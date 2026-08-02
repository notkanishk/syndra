// Package handlers: drift triage API (B2). Every triage action (attribute,
// revoke, mark-external) resolves the drift row and writes its side effect in
// ONE atomic transaction via the db.*AndEnqueue / db.MarkDriftExternalTx
// helpers — the handlers never resolve a drift row outside that transaction.
// A lost concurrent-triage race surfaces as db.ErrDriftNotPending (409), with
// nothing written on either side.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"mkauth/internal/db"
	"mkauth/internal/models"
)

// handleListDrift serves the triage queue. Unfiltered, it returns the enriched
// view — risk, holder status and the other-items count each row needs to be
// decided on without a click, ordered by risk then age. A filtered request
// (user/project/source) stays on the raw listing: those callers are looking up
// specific rows, not triaging a queue, and the enrichment cost isn't theirs to
// pay.
func handleListDrift(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := db.DriftFilter{
		UserID:          q.Get("user_id"),
		ProjectID:       q.Get("project_id"),
		DetectionSource: q.Get("source"),
	}

	if filter.UserID == "" && filter.ProjectID == "" && filter.DetectionSource == "" {
		items, err := svcDriftTriageQueue(r.Context())
		if err != nil {
			jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}
		if items == nil {
			items = []models.DriftTriageItem{}
		}
		jsonResponse(w, http.StatusOK, map[string]any{"drift": items})
		return
	}

	items, err := dbGetDriftItems(r.Context(), filter)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if items == nil {
		items = []models.DriftItem{}
	}
	jsonResponse(w, http.StatusOK, map[string]any{"drift": items})
}

type attributeRequest struct {
	Source string `json:"source"` // external_backfill — the only attribution this endpoint can honour
}

// validAttributionSource gates the source an operator may attribute drift to.
// An empty/unknown source must never reach enqueueWrites, which would silently
// default it to "direct" and mislabel attributed drift as an ordinary grant.
//
// "bundle" and "rule" were once accepted here and no longer are. Adoption writes
// a direct_role_grants row; it does not assign a bundle to the person or create
// a rule-derived relationship — cascades deliberately never touch the ledger, so
// a row labelled source='bundle' had nothing whatsoever behind the label. The
// access then survived removal of the very bundle it claimed to come from, and
// the ledger said otherwise.
//
// Routing adoption through real ownership was the alternative, and it is worse.
// Assigning the bundle to explain ONE drifting role hands the person every other
// role that bundle carries; making a rule produce the role means granting them
// the rule's input role, which is frequently safety-gated. Triage explains or
// removes access that already exists. It must not be a way to grant more.
//
// So adoption means exactly what the screen says it means: MkAuth records a
// direct grant, the operator becomes granter of record, nothing changes
// upstream. That is external_backfill, and it is the only honest option.
func validAttributionSource(s string) bool {
	return s == "external_backfill"
}

func handleAttributeDrift(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req attributeRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonErrorResponse(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	if !validAttributionSource(req.Source) {
		jsonErrorResponse(w, http.StatusBadRequest, "BAD_SOURCE", badSourceMessage)
		return
	}
	item, err := dbGetDriftItem(r.Context(), id)
	if err != nil {
		jsonErrorResponse(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	if err := attributeOneDrift(r.Context(), item, req, resolveActor(r, "operator")); err != nil {
		writeDriftActionError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{"status": "attributed"})
}

// attributeOneDrift writes the ledger intent for a zitadel_only drift and marks
// it attributed. It enqueues nothing, and that is the whole point: adoption is
// the operator saying "Zitadel is right, MkAuth was wrong", so there is no
// mutation owed upstream and no outbox row to carry one.
//
// Two versions of this got it wrong. The first left an `add` row pending
// forever, and the governance queue then listed every adopted role as a change
// MkAuth still owed Zitadel — accepting what Zitadel already had produced a
// queue of writes back to Zitadel. The second drained that row inline, which
// only narrowed the window: an outbox row is a live instruction, and a drain
// that cannot reach Zitadel leaves it behind. Either way, an operator who adopts
// a role and then removes it upstream by hand gets it re-created by a later
// drain. The row must not exist.
//
// The recorded source is always external_backfill and carries no source_ref:
// there is no bundle or rule owning this access, and saying otherwise would be
// a provenance the ledger cannot honour. See validAttributionSource.
//
// Atomicity: a SINGLE transaction (db.AttributeDriftTx) guard-transitions the
// drift row to 'attributed' AND writes the ledger+audit rows together. A lost
// concurrent-triage race returns ErrDriftNotPending (→409) with the whole tx
// rolled back; a write failure rolls back the resolution too — the drift never
// leaves the triage queue without its durable ledger row.
func attributeOneDrift(ctx context.Context, item models.DriftItem, req attributeRequest, actor string) error {
	payload, _ := json.Marshal(req)
	return dbAttributeDriftTx(ctx, item.ID, db.EnqueueParams{
		UserID: item.UserID, ProjectID: item.ProjectID, RoleKeys: item.RoleKeys,
		GrantedBy: actor, Reason: "drift attribution", Source: req.Source,
		OpType: "add", ZitadelGrantID: item.ZitadelGrantID, PayloadJSON: string(payload),
		// Stated here, where the intent is, and enforced again inside
		// AttributeDriftTx, which must never propagate whoever calls it.
		NoPropagation: true,
	})
}

func handleRevokeDrift(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	item, err := dbGetDriftItem(r.Context(), id)
	if err != nil {
		jsonErrorResponse(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	actor := resolveActor(r, "operator")
	// ONE tx: guard-transition to 'revoked' AND enqueue the revoke outbox row
	// together. A lost race 409s with nothing written; a write failure rolls the
	// resolution back too. Drain is best-effort AFTER commit — the durable revoke
	// row stays pending in the worklist if Zitadel is unreachable.
	outboxID, err := dbRevokeDriftAndEnqueue(r.Context(), id, db.EnqueueParams{
		UserID: item.UserID, ProjectID: item.ProjectID, RoleKeys: item.RoleKeys,
		GrantedBy: actor, OpType: "revoke",
		ZitadelGrantID: item.ZitadelGrantID, PayloadJSON: "{}",
	})
	if err != nil {
		writeDriftActionError(w, err) // ErrDriftNotPending → 409, else 500
		return
	}
	_, _ = svcDrainOne(r.Context(), outboxID)
	jsonResponse(w, http.StatusOK, map[string]any{"status": "revoked", "outbox_id": outboxID})
}

type markExternalRequest struct {
	Reason string `json:"reason"`
}

func handleMarkDriftExternal(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req markExternalRequest
	// reason is optional, so an empty body (io.EOF) is fine — but malformed JSON
	// or unknown fields must 400, since mark-external permanently suppresses
	// future detection for the triple and must not act on garbage input.
	if err := decodeJSONStrict(r.Body, &req); err != nil && !errors.Is(err, io.EOF) {
		jsonErrorResponse(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	item, err := dbGetDriftItem(r.Context(), id)
	if err != nil {
		jsonErrorResponse(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	payload, _ := json.Marshal(req)
	// ONE tx: guard-transition to 'marked_external' AND insert the exclusion rows
	// together. A lost race 409s with no exclusion written; a write failure rolls
	// the resolution back too.
	if err := dbMarkDriftExternalTx(r.Context(), id, item.UserID, item.ProjectID,
		item.RoleKeys, resolveActor(r, "operator"), req.Reason, string(payload)); err != nil {
		writeDriftActionError(w, err) // ErrDriftNotPending → 409, else 500
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{"status": "marked_external"})
}

type bulkAttributeRequest struct {
	IDs    []string `json:"ids"`
	Source string   `json:"source"`
}

func handleBulkAttributeDrift(w http.ResponseWriter, r *http.Request) {
	var req bulkAttributeRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonErrorResponse(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	// Same source gate as the single-item endpoint: an empty/invalid source must
	// not slip through to enqueueWrites and default to "direct" for the whole batch.
	if !validAttributionSource(req.Source) {
		jsonErrorResponse(w, http.StatusBadRequest, "BAD_SOURCE", badSourceMessage)
		return
	}

	plan := rehearseDriftBatch(r.Context(), req.IDs, driftOpAdopt)
	if r.URL.Query().Get("apply") != "true" {
		jsonResponse(w, http.StatusOK, plan)
		return
	}

	applyDriftPlan(r, &plan, driftOpAdopt, resolveActor(r, "operator"), "")
	jsonResponse(w, http.StatusOK, plan)
}

type bulkMarkExternalRequest struct {
	IDs    []string `json:"ids"`
	Reason string   `json:"reason"`
}

// handleBulkMarkDriftExternal is the second of exactly two bulk resolutions.
//
// There is deliberately no bulk revoke, and that absence is a design decision
// rather than an omission: adopting and marking-as-external are reversible
// bookkeeping, but revoking removes real access from real machines. Reading
// twelve consequences at once is not something anyone actually does, so revoke
// stays one row, one dialog, one decision.
// POST /api/v1/governance/drift/bulk-mark-external
func handleBulkMarkDriftExternal(w http.ResponseWriter, r *http.Request) {
	var req bulkMarkExternalRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonErrorResponse(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	plan := rehearseDriftBatch(r.Context(), req.IDs, driftOpExternal)
	if r.URL.Query().Get("apply") != "true" {
		jsonResponse(w, http.StatusOK, plan)
		return
	}

	applyDriftPlan(r, &plan, driftOpExternal, resolveActor(r, "operator"), req.Reason)
	jsonResponse(w, http.StatusOK, plan)
}

func handleReconcileDrift(w http.ResponseWriter, r *http.Request) {
	res, err := svcDriftSweep(r.Context())
	if err != nil {
		jsonErrorResponse(w, http.StatusBadGateway, "SWEEP_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, res)
}

// badSourceMessage names the only attribution this endpoint can honour, and
// says why the other two are gone — an operator or integration that used to
// send them deserves the reason, not just a rejection.
const badSourceMessage = "source must be external_backfill; adopting records a direct grant and cannot create a bundle assignment or a rule-derived relationship"

// writeDriftActionError maps a triage action error to its HTTP status.
// db.ErrDriftNotPending → 409 is load-bearing: it is how a lost
// concurrent-triage race is reported (the atomic claim+enqueue transaction
// guarantees the whole action rolled back — no side effect ran).
func writeDriftActionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, db.ErrDriftNotPending):
		jsonErrorResponse(w, http.StatusConflict, "DRIFT_NOT_PENDING", err.Error())
	default:
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
	}
}
