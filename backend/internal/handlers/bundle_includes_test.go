package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"syndra/internal/db"
	"syndra/internal/models"
	"syndra/internal/services"
)

func includeRequest(method, parent, child, body string) *http.Request {
	req := httptest.NewRequest(method, "/api/v1/bundles/"+parent+"/draft/includes/"+child, strings.NewReader(body))
	req.SetPathValue("id", parent)
	req.SetPathValue("child", child)
	return req
}

// Two levels deep is refused in words naming the bundle in the way.
func TestSetBundleInclude_RefusesNesting(t *testing.T) {
	orig := dbSetWorkingInclude
	t.Cleanup(func() { dbSetWorkingInclude = orig })
	dbSetWorkingInclude = func(context.Context, string, string, string, int) (models.BundleInclude, error) {
		return models.BundleInclude{}, &db.NestingError{Bundle: "Staff", Includes: true}
	}
	rr := httptest.NewRecorder()
	handleSetBundleInclude(rr, includeRequest(http.MethodPut, "nm", "staff", `{}`))
	if rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), "BUNDLE_NESTING") ||
		!strings.Contains(rr.Body.String(), "Staff includes other bundles, so it can't go inside another") {
		t.Fatalf("got %d %s", rr.Code, rr.Body.String())
	}
}

// A good include answers with the fresh draft.
func TestSetBundleInclude_AnswersWithTheDraft(t *testing.T) {
	origSet, origDraft := dbSetWorkingInclude, svcBundleDraft
	t.Cleanup(func() { dbSetWorkingInclude, svcBundleDraft = origSet, origDraft })
	var pinned int
	dbSetWorkingInclude = func(_ context.Context, _, parent, child string, version int) (models.BundleInclude, error) {
		pinned = version
		return models.BundleInclude{}, nil
	}
	svcBundleDraft = func(context.Context, string) (services.DraftDiff, error) {
		return services.DraftDiff{BundleID: "nm", IncludeChanges: []services.IncludeChange{{ChildName: "Safety basics", To: 4}}}, nil
	}
	rr := httptest.NewRecorder()
	handleSetBundleInclude(rr, includeRequest(http.MethodPut, "nm", "safety", `{"version":4}`))
	if rr.Code != http.StatusOK || pinned != 4 || !strings.Contains(rr.Body.String(), "include_changes") {
		t.Fatalf("got %d (v%d) %s", rr.Code, pinned, rr.Body.String())
	}
}

// Retiring an included bundle is refused, naming who includes it.
func TestDeleteBundle_RefusedWhileIncluded(t *testing.T) {
	origGet, origCascade := dbGetBundleByID, svcCascadeBundleDeleted
	t.Cleanup(func() { dbGetBundleByID, svcCascadeBundleDeleted = origGet, origCascade })
	dbGetBundleByID = func(context.Context, string) (models.Bundle, error) { return models.Bundle{ID: "safety"}, nil }
	svcCascadeBundleDeleted = func(context.Context, string, string) (services.CascadeResult, error) {
		return services.CascadeResult{}, &services.ErrBundleIncluded{Includers: []string{"New members", "Staff"}}
	}
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/bundles/safety", nil)
	req.SetPathValue("id", "safety")
	rr := httptest.NewRecorder()
	handleDeleteBundle(rr, req)
	if rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), "Refused while New members and Staff include it") {
		t.Fatalf("got %d %s", rr.Code, rr.Body.String())
	}
}
