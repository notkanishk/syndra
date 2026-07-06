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

func handleListDrift(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	items, err := dbGetDriftItems(r.Context(), db.DriftFilter{
		UserID:          q.Get("user_id"),
		ProjectID:       q.Get("project_id"),
		DetectionSource: q.Get("source"),
	})
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
	Source    string `json:"source"`     // external_backfill | bundle | rule
	SourceRef string `json:"source_ref"` // bundle_id / rule_id
}

// validAttributionSource gates the source an operator may attribute drift to.
// An empty/unknown source must never reach enqueueWrites, which would silently
// default it to "direct" and mislabel attributed drift as an ordinary grant.
func validAttributionSource(s string) bool {
	switch s {
	case "external_backfill", "bundle", "rule":
		return true
	}
	return false
}

func handleAttributeDrift(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req attributeRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonErrorResponse(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	if !validAttributionSource(req.Source) {
		jsonErrorResponse(w, http.StatusBadRequest, "BAD_SOURCE", "source must be external_backfill, bundle, or rule")
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
// it attributed. The grant already exists in Zitadel, so it flows through the
// outbox as an `add` that self-resolves (grant-index short-circuit / 409→applied)
// — one code path, no special "skip Zitadel" branch. Bundle attribution is
// source-remap-validated: the bundle must actually contain the drift role.
//
// Atomicity: read-only validation first, then a SINGLE transaction
// (db.AttributeDriftAndEnqueue) that guard-transitions the drift row to
// 'attributed' AND writes the ledger+audit+outbox rows together. A lost
// concurrent-triage race returns ErrDriftNotPending (→409) with the whole tx
// rolled back; a write failure rolls back the resolution too — the drift never
// leaves the triage queue without its durable outbox row.
func attributeOneDrift(ctx context.Context, item models.DriftItem, req attributeRequest, actor string) error {
	if req.Source == "bundle" {
		roles, err := svcGetRolesForBundleDrift(ctx, req.SourceRef)
		if err != nil {
			return err
		}
		for _, rk := range item.RoleKeys {
			if !bundleHasRole(roles, item.ProjectID, rk) {
				return errDriftBadRemap // handler maps to 400
			}
		}
	}
	payload, _ := json.Marshal(req)
	return dbAttributeDriftAndEnqueue(ctx, item.ID, db.EnqueueParams{
		UserID: item.UserID, ProjectID: item.ProjectID, RoleKeys: item.RoleKeys,
		GrantedBy: actor, Reason: "drift attribution", Source: req.Source, SourceRef: req.SourceRef,
		OpType: "add", ZitadelGrantID: item.ZitadelGrantID, PayloadJSON: string(payload),
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
	IDs       []string `json:"ids"`
	Source    string   `json:"source"`
	SourceRef string   `json:"source_ref"`
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
		jsonErrorResponse(w, http.StatusBadRequest, "BAD_SOURCE", "source must be external_backfill, bundle, or rule")
		return
	}
	actor := resolveActor(r, "operator")
	attributed, failed := 0, 0
	for _, id := range req.IDs {
		item, err := dbGetDriftItem(r.Context(), id)
		if err != nil {
			failed++
			continue
		}
		if err := attributeOneDrift(r.Context(), item, attributeRequest{Source: req.Source, SourceRef: req.SourceRef}, actor); err != nil {
			failed++
			continue
		}
		attributed++
	}
	jsonResponse(w, http.StatusOK, map[string]any{"attributed": attributed, "failed": failed})
}

func handleReconcileDrift(w http.ResponseWriter, r *http.Request) {
	res, err := svcDriftSweep(r.Context())
	if err != nil {
		jsonErrorResponse(w, http.StatusBadGateway, "SWEEP_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, res)
}

func bundleHasRole(roles []models.BundleRole, projectID, roleKey string) bool {
	for _, r := range roles {
		if r.ProjectID == projectID && r.RoleKey == roleKey {
			return true
		}
	}
	return false
}

// errDriftBadRemap signals a bundle attribution whose target bundle does not
// actually contain the drift role — the handler maps this to 400.
var errDriftBadRemap = errors.New("bundle does not contain the drift role")

// writeDriftActionError maps a triage action error to its HTTP status.
// db.ErrDriftNotPending → 409 is load-bearing: it is how a lost
// concurrent-triage race is reported (the atomic claim+enqueue transaction
// guarantees the whole action rolled back — no side effect ran).
func writeDriftActionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, db.ErrDriftNotPending):
		jsonErrorResponse(w, http.StatusConflict, "DRIFT_NOT_PENDING", err.Error())
	case errors.Is(err, errDriftBadRemap):
		jsonErrorResponse(w, http.StatusBadRequest, "BAD_REMAP", err.Error())
	default:
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
	}
}
