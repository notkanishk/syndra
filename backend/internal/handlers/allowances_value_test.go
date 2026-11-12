package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"syndra/internal/addons"
	"syndra/internal/db"
)

// 5.3e: an allowance answers to the same /values rule a mapping does, in the
// same answer shape. The mapping and the allowance are posted the same value
// against the same stubbed add-on and must come back identical.
func TestAnAllowanceNamesOnlyWhatTheAddonLists(t *testing.T) {
	h := stubMappingDeps(t, []addons.EntitlementField{{Name: "group", Label: "Group", Role: "membership"}})
	create, name := dbCreateAllowance, dbTargetName
	t.Cleanup(func() { dbCreateAllowance, dbTargetName = create, name })
	var written []db.Allowance
	dbCreateAllowance = func(_ context.Context, a db.Allowance) (db.Allowance, error) {
		written = append(written, a)
		a.ID = "a1"
		return a, nil
	}
	dbTargetName = func(context.Context, string) (string, error) { return "Shop NAS", nil }
	svcConverge := svcConvergeSubjectOn
	t.Cleanup(func() { svcConvergeSubjectOn = svcConverge })
	svcConvergeSubjectOn = func(context.Context, string, string, string) error { return nil }

	postAllowance := func(value string) *httptest.ResponseRecorder {
		rr := httptest.NewRecorder()
		handleCreateAllowance(rr, httptest.NewRequest(http.MethodPost, "/api/v1/allowances", strings.NewReader(
			`{"subject_id":"u1","target":"truenas","field":"group","value":"`+value+`","direction":"deny","reason":"safety review"}`)))
		return rr
	}
	body := func(rr *httptest.ResponseRecorder) ErrorResponse {
		var got ErrorResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		return got
	}

	t.Run("an unlisted value is refused like a mapping", func(t *testing.T) {
		h.valueErr = fmt.Errorf("%w: truenas has no group named %q", addons.ErrValueNotResolvable, "ghosts")
		rr := postAllowance("ghosts")
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("want 400, got %d (%s)", rr.Code, rr.Body.String())
		}
		got, want := body(rr), body(postMapping(`{"target":"truenas","project_id":"pLab","role_key":"maker","field":"group","value":"ghosts"}`))
		if got.Details["value"] != "unresolvable" || got.Message != want.Message || fmt.Sprint(got.Details) != fmt.Sprint(want.Details) {
			t.Errorf("allowance answered %q %v, mapping answered %q %v", got.Message, got.Details, want.Message, want.Details)
		}
		if len(written) != 0 {
			t.Error("nothing may be written")
		}
	})

	t.Run("a listed value passes, trimmed, and the add-on is asked about the canonical one", func(t *testing.T) {
		h.valueErr, h.resolved = nil, nil
		if rr := postAllowance("lab_makers "); rr.Code != http.StatusCreated {
			t.Fatalf("want 201, got %d (%s)", rr.Code, rr.Body.String())
		}
		if len(h.resolved) != 1 || h.resolved[0] != "truenas|group|lab_makers" {
			t.Errorf("resolved %v", h.resolved)
		}
		if len(written) != 1 || written[0].Value != "lab_makers" {
			t.Errorf("written %+v", written)
		}
	})

	t.Run("a membership the add-on cannot list gives the mapping's refusal", func(t *testing.T) {
		written = nil
		h.valueErr = fmt.Errorf("%w: truenas can't list the values of group, so a mapping can't name one", addons.ErrFieldNotListable)
		rr := postAllowance("lab_makers")
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("want 400, got %d (%s)", rr.Code, rr.Body.String())
		}
		got, want := body(rr), body(postMapping(`{"target":"truenas","project_id":"pLab","role_key":"maker","field":"group","value":"lab_makers"}`))
		if got.Details["field"] != "unlistable" || got.Message != want.Message || fmt.Sprint(got.Details) != fmt.Sprint(want.Details) {
			t.Errorf("allowance answered %q %v, mapping answered %q %v", got.Message, got.Details, want.Message, want.Details)
		}
		if len(written) != 0 {
			t.Error("nothing may be written")
		}
	})
}
