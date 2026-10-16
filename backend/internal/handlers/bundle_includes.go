package handlers

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"

	"syndra/internal/db"
)

// Included bundles (reconcile B8). Includes live in the working copy like
// roles: setting or removing one reaches nobody until the parent publishes.

var (
	dbSetWorkingInclude      = db.SetWorkingInclude
	dbRemoveWorkingInclude   = db.RemoveWorkingInclude
	dbListIncluders          = db.ListIncluders
	dbBundleIncludesAnything = db.BundleIncludesAnything
	dbRetireBlockers         = db.RetireBlockers
	dbIncludeInFirstVersion  = db.IncludeInFirstVersion
	dbInTxLockingAccess      = db.InTxLockingAccess
)

type setIncludeRequest struct {
	// Version pins the included bundle; 0 or absent means its latest.
	Version int `json:"version,omitempty"`
}

// writeIncludeError answers a refused include in words.
func writeIncludeError(w http.ResponseWriter, err error) bool {
	var nesting *db.NestingError
	switch {
	case errors.As(err, &nesting):
		jsonErrorResponse(w, http.StatusConflict, "BUNDLE_NESTING", nesting.Error()+". Bundles go one level deep. Nothing was changed.")
	case errors.Is(err, db.ErrIncludeSelf):
		jsonValidationErrorResponse(w, "A bundle can't include itself.", map[string]string{"child": "self"})
	case errors.Is(err, pgx.ErrNoRows):
		jsonErrorResponse(w, http.StatusNotFound, "NOT_FOUND", "That bundle or version doesn't exist.")
	default:
		return false
	}
	return true
}

// PUT /api/v1/bundles/{id}/draft/includes/{child}
func handleSetBundleInclude(w http.ResponseWriter, r *http.Request) {
	parent, child := r.PathValue("id"), r.PathValue("child")
	if !trimmedNonEmpty(parent) || !trimmedNonEmpty(child) {
		jsonValidationErrorResponse(w, "id and child are required", map[string]string{"id": "required", "child": "required"})
		return
	}
	var req setIncludeRequest
	if r.ContentLength != 0 {
		if err := decodeJSONStrict(r.Body, &req); err != nil {
			jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
			return
		}
	}
	if req.Version < 0 {
		jsonValidationErrorResponse(w, "version must be a published version", map[string]string{"version": "min=1"})
		return
	}
	if _, err := dbSetWorkingInclude(r.Context(), resolveActor(r, ""), parent, child, req.Version); err != nil {
		if !writeIncludeError(w, err) {
			jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		}
		return
	}
	writeDraft(w, r, parent)
}

// DELETE /api/v1/bundles/{id}/draft/includes/{child}
func handleRemoveBundleInclude(w http.ResponseWriter, r *http.Request) {
	parent, child := r.PathValue("id"), r.PathValue("child")
	if _, err := dbRemoveWorkingInclude(r.Context(), resolveActor(r, ""), parent, child); err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	writeDraft(w, r, parent)
}

// GET /api/v1/bundles/{id}/includers
func handleGetBundleIncluders(w http.ResponseWriter, r *http.Request) {
	includers, err := dbListIncluders(r.Context(), r.PathValue("id"))
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, includers)
}

// writeDraft answers an include edit with the fresh draft, so the page shows
// what the next version would give without asking again.
func writeDraft(w http.ResponseWriter, r *http.Request, bundleID string) {
	draft, err := svcBundleDraft(r.Context(), bundleID)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, draft)
}
