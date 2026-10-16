package handlers

import (
	"net/http"
	"time"

	"syndra/internal/services"
)

// The term dates are one setting (A-04 §14); the rules and the reading live in
// services/terms.go so Trends, the lens and these routes agree.

// GET /api/v1/terms
func handleGetTerms(w http.ResponseWriter, r *http.Request) {
	view, err := svcLoadTerms(r.Context(), time.Now())
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, view)
}

type putTermsRequest struct {
	Terms []services.Term `json:"terms"`
}

// PUT /api/v1/terms {"terms":[{name,start,end}]}
func handlePutTerms(w http.ResponseWriter, r *http.Request) {
	var req putTermsRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	terms, errs := services.ValidateTerms(req.Terms)
	if len(errs) > 0 {
		jsonValidationErrorResponse(w, "Those term dates can't be saved", errs)
		return
	}
	actor := resolveActor(r, "")
	if err := dbSetConfigSetting(r.Context(), services.ConfigKeyTerms, services.EncodeTerms(terms), actor); err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	_ = dbInsertAuditLog(r.Context(), actor, "", "terms.updated", "")
	now := time.Now().UTC()
	jsonResponse(w, http.StatusOK, services.TermsView{Terms: terms, SetBy: &actor, SetAt: &now})
}
