package handlers

import (
	"net/http"

	"syndra/internal/models"
)

// handleDrainPropagations is the operator's explicit "Resume now" action: it
// flushes the pending_zitadel_propagations outbox to Zitadel and returns the
// drain summary. Operator-gated (see router.go). A drain error is a 502 because
// the failure is downstream (Zitadel), not a client mistake.
func handleDrainPropagations(w http.ResponseWriter, r *http.Request) {
	res, err := svcDrainPropagations(r.Context())
	if err != nil {
		jsonErrorResponse(w, http.StatusBadGateway, "DRAIN_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, res)
}

// handleListPendingPropagations returns the operator's "awaiting Zitadel"
// worklist (rows still pending or in_flight), oldest first.
func handleListPendingPropagations(w http.ResponseWriter, r *http.Request) {
	rows, err := dbGetPendingPropagations(r.Context())
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if rows == nil {
		rows = []models.PendingPropagation{} // encode as [] not null for the UI
	}
	jsonResponse(w, http.StatusOK, map[string]any{"pending": rows})
}
