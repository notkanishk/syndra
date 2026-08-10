package handlers

import (
	"net/http"

	"syndra/internal/db"
	"syndra/internal/models"
	"syndra/internal/services"
)

// handleDrainPropagations is the operator's explicit "Resume now" action: it
// flushes the propagation_outbox outbox to Zitadel and returns the
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

// handleUnconfirmedRevocations lists the withdrawals that have not reached
// their target (change `addon-platform` 2.51, 9.9).
//
// Beside drift triage rather than inside it, because they are different
// questions. Drift is access that appeared without an explanation; this is
// access that was withdrawn and did not go away. The second one is worse and has
// no equivalent surface — the revocation drain runs in the background, which is
// the whole reason nobody notices when it stops.
func handleUnconfirmedRevocations(w http.ResponseWriter, r *http.Request) {
	rows, err := dbListUnconfirmedRevocations(r.Context(), 200)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	summary, err := dbCountUnconfirmedRevocations(r.Context())
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if rows == nil {
		rows = []db.UnconfirmedRevocation{}
	}
	jsonResponse(w, http.StatusOK, map[string]any{
		"revocations": rows,
		"summary":     summary,
		// The threshold travels with the answer so the surface renders the same
		// escalation the indicator counted, rather than picking its own.
		"escalated": summary.Escalated(services.RevocationEscalationThreshold()),
	})
}
