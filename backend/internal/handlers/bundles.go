package handlers

import (
	"encoding/json"
	"net/http"
	
	"mkauth/internal/db"
)

type CreateBundleRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
}

type CreateBundleResponse struct {
	ID      string `json:"id"`
	Message string `json:"message"`
}

func handleGetBundles(w http.ResponseWriter, r *http.Request) {
	bundles, err := db.GetAllBundles(r.Context())
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	
	// Pre-init empty slices so JSON outputs '[]' instead of 'null'
	if bundles == nil {
		jsonResponse(w, http.StatusOK, []interface{}{})
		return
	}
	jsonResponse(w, http.StatusOK, bundles)
}

func handleCreateBundle(w http.ResponseWriter, r *http.Request) {
	var req CreateBundleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErrorResponse(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON payload")
		return
	}

	if req.Name == "" {
		jsonErrorResponse(w, http.StatusBadRequest, "VALIDATION_FAILED", "Name is required")
		return
	}

	id, err := db.CreateBundle(r.Context(), req.Name, req.Description)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	// Future Audit Log: db.InsertAuditLog(...)

	jsonResponse(w, http.StatusCreated, CreateBundleResponse{
		ID:      id,
		Message: "Bundle created successfully",
	})
}
