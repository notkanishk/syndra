package handlers

import (
	"net/http"
	"sync"
	"time"

	"syndra/internal/addons"
)

// What a mapping's value may be, for the mapping dialog's pickers (D6–D7).
//
// The target is the authority on its own groups and folders, so the dialog
// asks it rather than letting an operator type a name. When it can't be asked
// right now, the last answer is served and said to be the last one: a picker
// that empties during an outage would read as "TrueNAS has no groups".
//
// ponytail: in memory, per process. A restart forgets the last answer, and the
// dialog says "not read yet" until the target next answers.

type mappableValuesAnswer struct {
	Field  string `json:"field"`
	Label  string `json:"label"`
	Single bool   `json:"single"`
	addons.Values
	ReadAt  time.Time `json:"read_at"`
	Current bool      `json:"current"`
}

var lastMappableValues = struct {
	sync.Mutex
	byKey map[string]mappableValuesAnswer
}{byKey: map[string]mappableValuesAnswer{}}

func handleMappableValues(w http.ResponseWriter, r *http.Request) {
	target, field := r.PathValue("target"), r.URL.Query().Get("field")
	schema, err := addonsEntitlementSchema(target)
	if err != nil {
		jsonErrorResponse(w, http.StatusServiceUnavailable, "TARGET_CAPABILITIES_UNKNOWN",
			"Syndra hasn't heard what this system can do yet.")
		return
	}
	var spec *addons.EntitlementField
	for i := range schema {
		// What a mapping can set is every field the resolver doesn't compute
		// itself: `enabled` and `smb_enabled` follow whether someone holds a
		// mapped role at all.
		if schema[i].Name == field && !schema[i].Lifecycle {
			spec = &schema[i]
		}
	}
	if spec == nil {
		jsonErrorResponse(w, http.StatusNotFound, "FIELD_NOT_MAPPABLE", "A mapping can't set that on this system.")
		return
	}

	key := target + "\x00" + field
	got := addonsReadValues(r.Context(), target, field)
	lastMappableValues.Lock()
	defer lastMappableValues.Unlock()
	if got.Outcome == addons.OutcomeSucceeded {
		answer := mappableValuesAnswer{Field: field, Label: fieldLabel(*spec), Single: spec.Single,
			Values: got, ReadAt: time.Now().UTC(), Current: true}
		lastMappableValues.byKey[key] = answer
		jsonResponse(w, http.StatusOK, answer)
		return
	}
	if last, ok := lastMappableValues.byKey[key]; ok {
		last.Current = false
		jsonResponse(w, http.StatusOK, last)
		return
	}
	jsonErrorResponse(w, http.StatusConflict, "VALUES_NEVER_READ",
		"Syndra hasn't been able to read these from the system yet.")
}
