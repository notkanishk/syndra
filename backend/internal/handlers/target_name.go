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
	"syndra/internal/textsort"
)

// What a connected system is called (addon-contract 3.10a, design §6).
//
// A registration attribute, never the add-on's and never a map in the UI: two
// laser cutters from one add-on image share a manifest and must still be two
// names. systemName is the one place the name is decided, and every roster
// (GET /targets, GET /me/targets, a person's systems) carries what it says.

// Where a system's name came from, for the System page's Name setting.
const (
	nameFromOperator = "operator"
	nameFromProduct  = "product"
	nameFromID       = "id"
)

// systemName is the operator's name for a target, else its accepted
// manifest's product, else its id title-cased; from says which.
func systemName(ctx context.Context, target string) (name, from string) {
	stored, err := dbTargetName(ctx, target)
	if err != nil {
		// Non-fatal: a page that cannot read the name still has a true one.
		log.Printf("[TARGETS] could not read %s's name: %v (non-fatal)", target, err)
	}
	if stored != "" {
		return stored, nameFromOperator
	}
	if m, err := addonsAcceptedManifest(target); err == nil && strings.TrimSpace(m.Product) != "" {
		return m.Product, nameFromProduct
	}
	return titleCased(target), nameFromID
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
	shown, from := systemName(r.Context(), target)
	jsonResponse(w, http.StatusOK, map[string]any{"target": target, "name": shown, "name_from": from})
}

// memberMeets says whether members meet a registration at all, and its kind:
// only once a manifest is accepted and the kind is known. One not read yet is
// operators-only (owner, 11 Nov 2026), so both member reads (GET /me/systems,
// GET /me/targets) skip it here and neither payload ever carries it.
func memberMeets(target string) (kind string, ok bool) {
	kind, _ = acceptedShape(target)
	return kind, kind != ""
}

// mySystem is one row of a member's rail: what it is called and its kind.
type mySystem struct {
	Target string `json:"target"`
	Name   string `json:"name"`
	Kind   string `json:"kind"`
}

// handleMySystems is the systems a member meets, by name: the rail, the touch
// bar, Go to, the system page's gate and an operator's account sheet all draw
// from it. Database and registry only, never an add-on call, so a page's
// navigation is drawn in one cheap read rather than after every add-on answers.
func handleMySystems(w http.ResponseWriter, r *http.Request) {
	out := []mySystem{}
	for _, reg := range addonsRegistered() {
		kind, ok := memberMeets(reg.Target)
		if !ok {
			continue
		}
		name, _ := systemName(r.Context(), reg.Target)
		out = append(out, mySystem{Target: reg.Target, Name: name, Kind: kind})
	}
	textsort.ByName(out, func(s mySystem) string { return s.Name }, func(s mySystem) string { return s.Target })
	jsonResponse(w, http.StatusOK, map[string]any{"systems": out})
}
