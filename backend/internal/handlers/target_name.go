package handlers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"

	"syndra/internal/addons"
	"syndra/internal/db"
	"syndra/internal/textsort"
)

// What a connected system is called (addon-contract 3.10a, design §6).
//
// A registration attribute, never the add-on's and never a map in the UI: two
// laser cutters from one add-on image share a manifest and must still be two
// names. systemName is the one place the name is decided, and every roster
// (GET /targets, GET /me/targets, a person's systems) carries what it says.

// systemName is addons.SystemNameOf (the operator's name, else the accepted
// product, else the id) over this package's seams for the two reads.
func systemName(ctx context.Context, target string) (name, from string) {
	return addons.SystemNameFrom(ctx, target, dbTargetName, addonsAcceptedShape)
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
// only once a manifest has been accepted and the kind is known. One never
// accepted is operators-only (owner, 11 Nov 2026), so both member reads
// (GET /me/systems, GET /me/targets) skip it here and neither payload ever
// carries it. One accepted before and unreachable now is still met: the kind
// is the saved one (acceptedShape), and its page says what it could not read.
func memberMeets(ctx context.Context, target string) (kind string, ok bool) {
	kind, _ = acceptedShape(ctx, target)
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
		kind, ok := memberMeets(r.Context(), reg.Target)
		if !ok {
			continue
		}
		name, _ := systemName(r.Context(), reg.Target)
		out = append(out, mySystem{Target: reg.Target, Name: name, Kind: kind})
	}
	textsort.ByName(out, func(s mySystem) string { return s.Name }, func(s mySystem) string { return s.Target })
	jsonResponse(w, http.StatusOK, map[string]any{"systems": out})
}
