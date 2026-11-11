package handlers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"syndra/internal/db"
)

// What a connected system is called (addon-contract 3.10a, design §6).
//
// A registration attribute, never the add-on's and never a map in the UI: two
// laser cutters from one add-on image share a manifest and must still be two
// names. systemName is the one place the name is decided, and every roster
// (GET /targets, GET /me/targets, a person's systems) carries what it says.

// systemName is the operator's name for a target, else its accepted
// manifest's product, else its id title-cased; set says the operator named it.
func systemName(ctx context.Context, target string) (name string, set bool) {
	stored, err := dbTargetName(ctx, target)
	if err != nil {
		// Non-fatal: a page that cannot read the name still has a true one.
		log.Printf("[TARGETS] could not read %s's name: %v (non-fatal)", target, err)
	}
	if stored != "" {
		return stored, true
	}
	if m, err := addonsAcceptedManifest(target); err == nil && strings.TrimSpace(m.Product) != "" {
		return m.Product, false
	}
	return titleCased(target), false
}

// titleCased is the id with its first letter capital, the same rule the UI's
// targetLabel draws a row with before any roster has loaded.
func titleCased(id string) string {
	r, size := utf8.DecodeRuneInString(id)
	return string(unicode.ToUpper(r)) + id[size:]
}

type targetNameRequest struct {
	// Name is what members see. Blank clears it, back to the default.
	Name string `json:"name"`
}

// handleSetTargetName names a connected system, or clears its name. A
// Syndra-side write that changes nobody's access; audited with the old and
// new name in the same transaction as the write.
func handleSetTargetName(w http.ResponseWriter, r *http.Request) {
	target := r.PathValue("target")
	var req targetNameRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	if _, err := addonsGet(target); err != nil {
		jsonErrorResponse(w, http.StatusNotFound, "TARGET_NOT_FOUND", target+" is not a connected system.")
		return
	}
	name := strings.TrimSpace(req.Name)
	actor := resolveActor(r, "")
	switch err := dbSetTargetName(r.Context(), target, name, actor); {
	case errors.Is(err, db.ErrTargetNameInvalid):
		jsonValidationErrorResponse(w, err.Error(), map[string]string{"name": "too_long"})
		return
	case err != nil:
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	log.Printf("[TARGETS] %s named %s %q", actor, target, name)
	shown, set := systemName(r.Context(), target)
	jsonResponse(w, http.StatusOK, map[string]any{"target": target, "name": shown, "name_set": set})
}
