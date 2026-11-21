package handlers

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"syndra/internal/addons"
)

// A mapping or an allowance whose value could not be checked is not saved
// (design §13.3, owner Q2): one path, 503 TARGET_NOT_READ with the sentence.
func TestAnUncheckedValueIsNotSaved(t *testing.T) {
	orig := addonsResolvesValue
	t.Cleanup(func() { addonsResolvesValue = orig })
	addonsResolvesValue = func(context.Context, string, string, string) (addons.Resolution, error) {
		return addons.Resolution{}, addons.NotRead{Why: addons.Why{Reason: addons.ReasonAddonUnreachable, Cause: addons.CauseNoAnswer}}
	}
	_, err := resolveValue(context.Background(), "truenas", "group", "lab_makers", nil)
	for name, write := range map[string]func(*httptest.ResponseRecorder){
		"mapping":   func(rr *httptest.ResponseRecorder) { writeMappingError(rr, err) },
		"allowance": func(rr *httptest.ResponseRecorder) { writeAllowanceError(rr, err) },
	} {
		rr := httptest.NewRecorder()
		write(rr)
		var body ErrorResponse
		_ = json.Unmarshal(rr.Body.Bytes(), &body)
		if rr.Code != 503 || body.Error != codeTargetNotRead || body.Details["cause"] != addons.CauseNoAnswer ||
			!strings.HasPrefix(body.Message, "Not saved. Syndra couldn't check this value on ") {
			t.Errorf("%s: %d %s", name, rr.Code, rr.Body)
		}
	}
}
