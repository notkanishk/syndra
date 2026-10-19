package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"syndra/internal/models"
	"syndra/internal/services"
)

var retiredOn = time.Date(2026, 10, 19, 9, 30, 0, 0, time.UTC)

func retiredBundle(ctx context.Context, id string) (models.Bundle, error) {
	return models.Bundle{ID: id, Name: "Old", RetiredAt: &retiredOn, RetiredBy: "admin"}, nil
}

func decodeRetireBody(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&out); err != nil {
		t.Fatalf("decode %q: %v", rr.Body.String(), err)
	}
	return out
}

// Every write to a retired bundle is refused before the handler runs, with the sentence the
// owner wrote and the date. Reads are not behind it.
func TestWithLiveBundle_RefusesAWriteToARetiredBundle(t *testing.T) {
	resetBundleDeps(t)
	dbGetBundleByID = retiredBundle
	reached := false
	h := withLiveBundle(func(w http.ResponseWriter, r *http.Request) { reached = true })

	req := httptest.NewRequest(http.MethodPost, "/api/v1/bundles/b1/publish", nil)
	req.SetPathValue("id", "b1")
	rr := httptest.NewRecorder()
	h(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409: %s", rr.Code, rr.Body.String())
	}
	if reached {
		t.Fatal("the handler must not run for a retired bundle")
	}
	body := decodeRetireBody(t, rr)
	if body["error"] != "RETIRED" {
		t.Fatalf("error = %v, want RETIRED", body["error"])
	}
	if want := "This bundle was retired on 19 Oct 2026. Nothing about it can change."; body["message"] != want {
		t.Fatalf("message = %q, want %q", body["message"], want)
	}
}

func TestWithLiveBundle_PassesLiveAndUnknownBundlesThrough(t *testing.T) {
	for name, get := range map[string]func(context.Context, string) (models.Bundle, error){
		"live": func(ctx context.Context, id string) (models.Bundle, error) {
			return models.Bundle{ID: id}, nil
		},
		"unknown": func(ctx context.Context, id string) (models.Bundle, error) {
			return models.Bundle{}, pgx.ErrNoRows
		},
	} {
		resetBundleDeps(t)
		dbGetBundleByID = get
		reached := false
		h := withLiveBundle(func(w http.ResponseWriter, r *http.Request) { reached = true })
		req := httptest.NewRequest(http.MethodPut, "/api/v1/bundles/b1", nil)
		req.SetPathValue("id", "b1")
		h(httptest.NewRecorder(), req)
		if !reached {
			t.Errorf("%s: the handler must run (an unknown bundle gets its own 404 there)", name)
		}
	}
}

func TestHandleRetireBundle_AlreadyRetiredIs409(t *testing.T) {
	resetBundleDeps(t)
	dbGetBundleByID = retiredBundle
	svcCascadeBundleRetired = func(ctx context.Context, actor, bundleID string) (services.CascadeResult, error) {
		t.Fatal("a retired bundle must not cascade again")
		return services.CascadeResult{}, nil
	}
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/bundles/b1", nil)
	req.SetPathValue("id", "b1")
	rr := httptest.NewRecorder()
	handleRetireBundle(rr, req)
	if rr.Code != http.StatusConflict || decodeRetireBody(t, rr)["error"] != "RETIRED" {
		t.Fatalf("status = %d: %s", rr.Code, rr.Body.String())
	}
}

// The service refuses assigning a retired bundle; the handler turns that into the same 409
// rather than a 500 the operator would read as a fault.
func TestHandleAssignBundle_RetiredIs409(t *testing.T) {
	resetCascadeHandlerDeps(t)
	svcCascadeBundleAssigned = func(ctx context.Context, actor, userID, bundleID string) (services.CascadeResult, error) {
		return services.CascadeResult{}, &services.ErrRetired{At: retiredOn}
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/u1/bundles", strings.NewReader(`{"bundle_id":"b1"}`))
	req.SetPathValue("id", "u1")
	rr := httptest.NewRecorder()
	handleAssignBundleToUser(rr, req)
	if rr.Code != http.StatusConflict || decodeRetireBody(t, rr)["error"] != "RETIRED" {
		t.Fatalf("status = %d: %s", rr.Code, rr.Body.String())
	}
}

// A retired bundle is still readable, by id, with when and by whom: its page needs both.
func TestHandleGetBundle_ReturnsARetiredBundleWithWhenAndBy(t *testing.T) {
	resetBundleDeps(t)
	dbGetBundleByID = retiredBundle
	req := httptest.NewRequest(http.MethodGet, "/api/v1/bundles/b1", nil)
	req.SetPathValue("id", "b1")
	rr := httptest.NewRecorder()
	handleGetBundle(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rr.Code, rr.Body.String())
	}
	body := decodeRetireBody(t, rr)
	if body["retired_by"] != "admin" || body["retired_at"] == nil {
		t.Fatalf("retired_at/retired_by missing: %v", body)
	}
}

func TestHandleGetBundle_UnknownIs404(t *testing.T) {
	resetBundleDeps(t)
	dbGetBundleByID = func(ctx context.Context, id string) (models.Bundle, error) {
		return models.Bundle{}, pgx.ErrNoRows
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/bundles/nope", nil)
	req.SetPathValue("id", "nope")
	rr := httptest.NewRecorder()
	handleGetBundle(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rr.Code)
	}
}

// The list is the bundles in force; ?retired=true is the other half, and never both.
func TestHandleGetBundles_RetiredFlagListsOnlyRetired(t *testing.T) {
	resetBundleDeps(t)
	origRetired := dbGetRetiredBundles
	t.Cleanup(func() { dbGetRetiredBundles = origRetired })
	dbGetAllBundles = func(ctx context.Context) ([]models.Bundle, error) {
		t.Fatal("the retired list must not read the live one")
		return nil, nil
	}
	dbGetRetiredBundles = func(ctx context.Context) ([]models.Bundle, error) {
		return []models.Bundle{{ID: "b1", Name: "Old", RetiredAt: &retiredOn}}, nil
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/bundles?retired=true", nil)
	rr := httptest.NewRecorder()
	handleGetBundles(rr, req)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"retired_at"`) {
		t.Fatalf("status = %d: %s", rr.Code, rr.Body.String())
	}

	// None retired is [], never null: the client calls .map on it.
	dbGetRetiredBundles = func(ctx context.Context) ([]models.Bundle, error) { return nil, nil }
	rr = httptest.NewRecorder()
	handleGetBundles(rr, httptest.NewRequest(http.MethodGet, "/api/v1/bundles?retired=true", nil))
	if strings.TrimSpace(rr.Body.String()) != "[]" {
		t.Fatalf("body = %q, want []", rr.Body.String())
	}
}

func TestRetiredRule_UpdateAndRetireAreRefused(t *testing.T) {
	for name, tc := range map[string]struct {
		method string
		call   func(http.ResponseWriter, *http.Request)
		body   string
	}{
		"update": {http.MethodPut, handleUpdateMappingRule, `{"source_project":"a","source_role":"b","target_project":"c","target_role":"d"}`},
		"retire": {http.MethodDelete, handleRetireMappingRule, ""},
	} {
		resetRulesDeps(t)
		dbGetMappingRuleByID = func(ctx context.Context, id string) (models.MappingRule, error) {
			return models.MappingRule{ID: id, RetiredAt: &retiredOn, RetiredBy: "admin"}, nil
		}
		svcCascadeRuleUpdated = func(ctx context.Context, actor string, old models.MappingRule, sp, sr, tp, tr string) (services.CascadeResult, error) {
			t.Fatalf("%s: a retired rule must not cascade", name)
			return services.CascadeResult{}, nil
		}
		svcCascadeRuleRetired = func(ctx context.Context, actor string, old models.MappingRule) (services.CascadeResult, error) {
			t.Fatalf("%s: a retired rule must not cascade", name)
			return services.CascadeResult{}, nil
		}
		req := httptest.NewRequest(tc.method, "/api/v1/rules/mapping/r1", strings.NewReader(tc.body))
		req.SetPathValue("id", "r1")
		rr := httptest.NewRecorder()
		tc.call(rr, req)
		body := decodeRetireBody(t, rr)
		if rr.Code != http.StatusConflict || body["error"] != "RETIRED" {
			t.Fatalf("%s: status = %d: %v", name, rr.Code, body)
		}
		if want := "This rule was retired on 19 Oct 2026. Nothing about it can change."; body["message"] != want {
			t.Fatalf("%s: message = %q", name, body["message"])
		}
	}
}

func TestHandleGetMappingRule_ReturnsARetiredRule(t *testing.T) {
	resetRulesDeps(t)
	dbGetMappingRuleByID = func(ctx context.Context, id string) (models.MappingRule, error) {
		return models.MappingRule{ID: id, RetiredAt: &retiredOn, RetiredBy: "admin"}, nil
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/rules/mapping/r1", nil)
	req.SetPathValue("id", "r1")
	rr := httptest.NewRecorder()
	handleGetMappingRule(rr, req)
	body := decodeRetireBody(t, rr)
	if rr.Code != http.StatusOK || body["retired_by"] != "admin" {
		t.Fatalf("status = %d: %v", rr.Code, body)
	}
}

func TestHandleGetMappingRules_RetiredFlagListsOnlyRetired(t *testing.T) {
	resetRulesDeps(t)
	orig := dbGetRetiredMappingRules
	t.Cleanup(func() { dbGetRetiredMappingRules = orig })
	dbGetActiveMappingRules = func(ctx context.Context) ([]models.MappingRule, error) {
		t.Fatal("the retired list must not read the live one")
		return nil, nil
	}
	dbGetRetiredMappingRules = func(ctx context.Context) ([]models.MappingRule, error) {
		return []models.MappingRule{{ID: "r1", RetiredAt: &retiredOn}}, nil
	}
	rr := httptest.NewRecorder()
	handleGetMappingRules(rr, httptest.NewRequest(http.MethodGet, "/api/v1/rules/mapping?retired=true", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"retired_at"`) {
		t.Fatalf("status = %d: %s", rr.Code, rr.Body.String())
	}
}

// Change history resolves a bundle id to a name. A retired bundle is still named there, never
// shown as an id.
func TestLookup_NamesARetiredBundle(t *testing.T) {
	resetBundleDeps(t)
	orig := dbGetRetiredBundles
	t.Cleanup(func() { dbGetRetiredBundles = orig })
	dbGetAllBundles = func(ctx context.Context) ([]models.Bundle, error) {
		return []models.Bundle{{ID: "live", Name: "Lab Tech"}}, nil
	}
	dbGetRetiredBundles = func(ctx context.Context) ([]models.Bundle, error) {
		return []models.Bundle{{ID: "gone", Name: "Old Door Access", RetiredAt: &retiredOn}}, nil
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/lookup", strings.NewReader(`{"bundle_ids":["live","gone"]}`))
	rr := httptest.NewRecorder()
	handleLookup(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rr.Code, rr.Body.String())
	}
	got := rr.Body.String()
	if !strings.Contains(got, "Old Door Access") || !strings.Contains(got, "Lab Tech") {
		t.Fatalf("both names must resolve: %s", got)
	}
}
