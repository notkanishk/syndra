package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"syndra/internal/models"
	"syndra/internal/services"
)

// The bundle list's holder number is observed; the assigned count rides beside
// it as the explanation. Never checked is null ("not checked"), never 0.
func TestHandleGetBundles_HolderNumberIsObserved(t *testing.T) {
	resetBundleDeps(t)
	origHolders, origRoles, origStale := dbGetBundleHolderCounts, dbGetBundleRoleCounts, dbGetStaleHolderCounts
	origDraft, origIncl, origBlock := svcBundleDraft, dbBundleIncludesAnything, dbRetireBlockers
	t.Cleanup(func() {
		dbGetBundleHolderCounts, dbGetBundleRoleCounts, dbGetStaleHolderCounts = origHolders, origRoles, origStale
		svcBundleDraft, dbBundleIncludesAnything, dbRetireBlockers = origDraft, origIncl, origBlock
	})
	dbGetAllBundles = func(context.Context) ([]models.Bundle, error) {
		return []models.Bundle{{ID: "b1", Name: "Laser"}}, nil
	}
	dbGetBundleHolderCounts = func(context.Context) (map[string]int, error) { return map[string]int{"b1": 12}, nil }
	dbGetStaleHolderCounts = func(context.Context) (map[string]int, error) { return nil, nil }
	dbGetBundleRoleCounts = func(context.Context) (map[string]int, error) { return nil, nil }
	svcBundleDraft = func(context.Context, string) (services.DraftDiff, error) { return services.DraftDiff{}, nil }
	dbBundleIncludesAnything = func(context.Context, string) (bool, error) { return false, nil }
	dbRetireBlockers = func(context.Context, string) ([]string, error) { return nil, nil }

	get := func() map[string]any {
		rr := httptest.NewRecorder()
		handleGetBundles(rr, httptest.NewRequest(http.MethodGet, "/api/v1/bundles", nil))
		var got []map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil || len(got) != 1 {
			t.Fatalf("bad body %q: %v", rr.Body.String(), err)
		}
		return got[0]
	}

	if b := get(); b["observed_holder_count"] != nil || b["holder_count"] != float64(12) {
		t.Fatalf("never read: observed %v (want null), assigned %v", b["observed_holder_count"], b["holder_count"])
	}

	now := time.Now()
	svcBundleHolding = func(context.Context) (services.HolderFacts, error) {
		return services.HolderFacts{Given: map[string]int{"b1": 12}, Observed: map[string]int{"b1": 9},
			Basis: models.ObservationBasis{ReadAt: &now, Current: true}}, nil
	}
	if b := get(); b["observed_holder_count"] != float64(9) || b["holder_count"] != float64(12) {
		t.Fatalf("observed %v (want 9: 3 assignments never reached Zitadel), assigned %v", b["observed_holder_count"], b["holder_count"])
	}
}

// The rule list: who the rule applies to (recorded) and who Zitadel shows
// holding its target (observed), both from services.RuleHolding.
func TestHandleGetMappingRules_ObservedBesideRecorded(t *testing.T) {
	origRules, origHolding := dbGetActiveMappingRules, svcRuleHolding
	t.Cleanup(func() { dbGetActiveMappingRules, svcRuleHolding = origRules, origHolding })
	dbGetActiveMappingRules = func(context.Context) ([]models.MappingRule, error) {
		return []models.MappingRule{{ID: "r1", SourceProject: "p", SourceRole: "a", TargetProject: "p", TargetRole: "b"}}, nil
	}
	now := time.Now()
	svcRuleHolding = func(context.Context, []models.MappingRule) (services.HolderFacts, error) {
		return services.HolderFacts{Given: map[string]int{"r1": 5}, Observed: map[string]int{"r1": 3},
			Basis: models.ObservationBasis{ReadAt: &now, Current: true}}, nil
	}
	rr := httptest.NewRecorder()
	handleGetMappingRules(rr, httptest.NewRequest(http.MethodGet, "/api/v1/rules/mapping", nil))
	var got []map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil || len(got) != 1 {
		t.Fatalf("bad body %q: %v", rr.Body.String(), err)
	}
	if got[0]["holder_count"] != float64(5) || got[0]["observed_holder_count"] != float64(3) {
		t.Fatalf("rule counts: %v", got[0])
	}
}
