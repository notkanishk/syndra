package handlers

import (
	"net/http"

	"syndra/internal/addons"
)

// What a mapping's value may be, for the mapping dialog's pickers (D6–D7).
//
// The target is the authority on its own groups and folders, so the dialog
// asks it rather than letting an operator type a name. When it can't be asked
// right now there is no copy to give (design §13.3: the add-on is the one
// keeper, and it keeps none of /values): 503 TARGET_NOT_READ, never an empty
// picker that would read as "TrueNAS has no groups".

type mappableValuesAnswer struct {
	Field  string `json:"field"`
	Label  string `json:"label"`
	Single bool   `json:"single"`
	addons.Values
}

func handleMappableValues(w http.ResponseWriter, r *http.Request) {
	target, field := r.PathValue("target"), r.URL.Query().Get("field")
	schema, err := addonsEntitlementSchema(target)
	if err != nil {
		// No manifest: what the system can do was never read.
		why, _ := addonsHealthOf(target).Down()
		if why.Reason == "" {
			why = addons.Why{Reason: addons.ReasonAddonUnreachable}
		}
		writeUnread(w, r, target, why)
		return
	}
	var spec *addons.EntitlementField
	for i := range schema {
		// What a mapping can set is every field the resolver doesn't compute
		// itself: `enabled` and `smb_enabled` follow whether someone holds a
		// mapped role at all.
		if schema[i].Name == field && !schema[i].IsLifecycle() {
			spec = &schema[i]
		}
	}
	if spec == nil {
		jsonErrorResponse(w, http.StatusNotFound, "FIELD_NOT_MAPPABLE", "A mapping can't set that on this system.")
		return
	}

	got := addonsReadValues(r.Context(), target, field)
	if got.Read.None() {
		writeUnread(w, r, target, got.Read.Why())
		return
	}
	jsonResponse(w, http.StatusOK, mappableValuesAnswer{Field: field, Label: fieldLabel(*spec), Single: spec.IsSingle(), Values: got})
}
