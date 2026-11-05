package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"syndra/internal/models"
	"syndra/internal/services"
)

// The give, approve and adopt dialogs read this before the operator confirms:
// every role the preview is asked about reaches the shared closure, and its
// rows come back as said.
func TestRulePreview_ListsTheRuleRows(t *testing.T) {
	orig := svcRulesFollowing
	t.Cleanup(func() { svcRulesFollowing = orig })
	var asked []models.HeldRole
	svcRulesFollowing = func(_ context.Context, uid string, roles []models.HeldRole) ([]services.RuleRow, error) {
		if uid != "u1" {
			t.Errorf("asked about %q", uid)
		}
		asked = roles
		return []services.RuleRow{{ProjectID: "p_shop", RoleKey: "bench", RuleID: "r1",
			Sentence: "Also gives Bench in Workshop, by the automatic rule from Trained."}}, nil
	}
	req := httptest.NewRequest("GET", "/api/v1/users/u1/rule-preview?project_id=p_laser&role_key=trained&role_key=advanced", nil)
	req.SetPathValue("id", "u1")
	w := httptest.NewRecorder()
	handleRulePreview(w, req)

	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Also gives Bench in Workshop") {
		t.Fatalf("got %d %s", w.Code, w.Body)
	}
	if len(asked) != 2 || asked[1] != (models.HeldRole{ProjectID: "p_laser", RoleKey: "advanced"}) {
		t.Fatalf("every role_key is previewed together, got %v", asked)
	}
}

func TestRulePreview_NoRowsIsAnEmptyListAndNoRoleIs422(t *testing.T) {
	orig := svcRulesFollowing
	t.Cleanup(func() { svcRulesFollowing = orig })
	svcRulesFollowing = func(context.Context, string, []models.HeldRole) ([]services.RuleRow, error) { return nil, nil }

	req := httptest.NewRequest("GET", "/api/v1/users/u1/rule-preview?project_id=p&role_key=a", nil)
	req.SetPathValue("id", "u1")
	w := httptest.NewRecorder()
	handleRulePreview(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"rows":[]`) {
		t.Fatalf("no rules is an empty list, got %d %s", w.Code, w.Body)
	}

	req = httptest.NewRequest("GET", "/api/v1/users/u1/rule-preview?project_id=p", nil)
	req.SetPathValue("id", "u1")
	w = httptest.NewRecorder()
	handleRulePreview(w, req)
	if w.Code < 400 {
		t.Fatalf("no role to preview must be refused, got %d", w.Code)
	}
}
