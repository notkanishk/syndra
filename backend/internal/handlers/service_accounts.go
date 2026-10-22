package handlers

import (
	"errors"
	"net/http"

	"syndra/internal/services"
)

// Service accounts (A-07 §26-31): accounts made for a purpose, not a person.
// Whether a user is a machine is Zitadel's to say; these routes read and write
// only Syndra's own record (the operator's mark, kind, purpose, owner), never
// Zitadel. The rules live in services/service_accounts.go.

// accountError maps a service-account refusal onto its response.
func accountError(w http.ResponseWriter, err error) {
	var invalid *services.ServiceAccountInvalid
	switch {
	case errors.As(err, &invalid):
		jsonValidationErrorResponse(w, invalid.Message, map[string]string{invalid.Field: invalid.Message})
	case errors.Is(err, services.ErrServiceAccountNotFound):
		jsonErrorResponse(w, http.StatusNotFound, "NOT_FOUND", err.Error())
	case errors.Is(err, services.ErrMachineAccount):
		jsonErrorResponse(w, http.StatusConflict, "MACHINE_ACCOUNT", err.Error())
	default:
		viewError(w, err, http.StatusInternalServerError, "DB_ERROR")
	}
}

// GET /api/v1/service-accounts
func handleListServiceAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := svcListServiceAccounts(r.Context())
	if err != nil {
		accountError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{"accounts": accounts})
}

// GET /api/v1/service-accounts/{id}
func handleGetServiceAccount(w http.ResponseWriter, r *http.Request) {
	account, err := svcGetServiceAccount(r.Context(), r.PathValue("id"))
	if err != nil {
		accountError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, account)
}

type putServiceAccountRequest struct {
	Kind    string `json:"kind"`
	Purpose string `json:"purpose"`
	OwnerID string `json:"owner_id"`
}

// PUT /api/v1/service-accounts/{id} {"kind","purpose","owner_id"}
// Marks a human-type account, or changes any service account's kind, purpose
// and owner.
func handlePutServiceAccount(w http.ResponseWriter, r *http.Request) {
	var req putServiceAccountRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	id, actor := r.PathValue("id"), resolveActor(r, "")
	account, err := svcSaveServiceAccount(r.Context(), id,
		services.ServiceAccountInput{Kind: req.Kind, Purpose: req.Purpose, OwnerID: req.OwnerID}, actor)
	if err != nil {
		accountError(w, err)
		return
	}
	_ = dbInsertAuditLog(r.Context(), actor, id, "service_account.saved", req.Kind)
	jsonResponse(w, http.StatusOK, account)
}

// DELETE /api/v1/service-accounts/{id}: "Mark as a person". A machine user is
// 409 MACHINE_ACCOUNT: Zitadel says it is one, so nobody can unmark it.
func handleDeleteServiceAccount(w http.ResponseWriter, r *http.Request) {
	id, actor := r.PathValue("id"), resolveActor(r, "")
	if err := svcUnmarkServiceAccount(r.Context(), id); err != nil {
		accountError(w, err)
		return
	}
	_ = dbInsertAuditLog(r.Context(), actor, id, "service_account.unmarked", "")
	w.WriteHeader(http.StatusNoContent)
}
