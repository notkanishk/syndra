package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"syndra/internal/models"
)

// A token carries bundle names, so a rename must refresh every holder.
func TestHandleUpdateBundle_RefreshesEveryHolder(t *testing.T) {
	resetBundleDeps(t)
	dbUpdateBundle = func(context.Context, string, string, string) error { return nil }
	dbInsertAuditLog = func(context.Context, string, string, string, string) error { return nil }
	bundleHolders = func(context.Context, string) ([]models.BundleHolder, error) {
		return []models.BundleHolder{{UserID: "u1"}, {UserID: "u2"}}, nil
	}
	var got []string
	bundleRefresh = func(_ context.Context, ids []string, _ ...string) error { got = ids; return nil }

	req := httptest.NewRequest(http.MethodPut, "/api/v1/bundles/b1", strings.NewReader(`{"name":"Lab Tech","description":""}`))
	req.SetPathValue("id", "b1")
	rr := httptest.NewRecorder()
	handleUpdateBundle(rr, req)

	if rr.Code != http.StatusOK || len(got) != 2 || got[0] != "u1" || got[1] != "u2" {
		t.Fatalf("code=%d refreshed=%v", rr.Code, got)
	}
}

func TestHandleUpdateBundle_NoRefreshWhenTheRenameFails(t *testing.T) {
	resetBundleDeps(t)
	dbUpdateBundle = func(context.Context, string, string, string) error { return context.DeadlineExceeded }
	bundleRefresh = func(context.Context, []string, ...string) error {
		t.Fatal("nothing changed, nothing to refresh")
		return nil
	}
	req := httptest.NewRequest(http.MethodPut, "/api/v1/bundles/b1", strings.NewReader(`{"name":"Lab Tech","description":""}`))
	req.SetPathValue("id", "b1")
	rr := httptest.NewRecorder()
	handleUpdateBundle(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("code=%d", rr.Code)
	}
}
