package handlers

import (
	"errors"
	"net/http"

	"syndra/internal/db"
	"syndra/internal/models"
	"syndra/internal/services"
	"syndra/internal/services/propagation"
)

// handleDrainPropagations is the operator's explicit "Resume now" action: it
// flushes the propagation outbox to EVERY registered target and returns the
// drain summary, one pass per target. Operator-gated (see router.go). A drain
// error is a 502 because the failure is downstream (the target), not a client
// mistake.
func handleDrainPropagations(w http.ResponseWriter, r *http.Request) {
	res, err := svcDrainPropagations(r.Context())
	if err != nil {
		jsonErrorResponse(w, http.StatusBadGateway, "DRAIN_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, res)
}

// handleDrainTarget resumes ONE add-on target.
//
// The whole-deployment drain above already covers this target; this exists for
// the case that produced it — a target that was unreachable while everything
// else drained, and is now back. Resuming it should not require re-running
// every other target's pass, and an operator watching one NAS come back wants a
// result about that NAS rather than a summary they have to read twice.
func handleDrainTarget(w http.ResponseWriter, r *http.Request) {
	target := r.PathValue("target")
	res, err := svcDrainAddon(r.Context(), target)
	if err != nil {
		// A refusal about WHICH target, not about the target's health: the
		// built-in target has its own dispatcher, and asking this route for it
		// is a caller error rather than a downstream one.
		if errors.Is(err, propagation.ErrBuiltInTarget) {
			jsonErrorResponse(w, http.StatusBadRequest, "BUILT_IN_TARGET",
				"Zitadel drains with the whole deployment, not on its own.")
			return
		}
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
