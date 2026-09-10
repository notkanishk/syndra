package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"syndra/internal/services"
)

func TestHandleGetMissedOnboarding_NoWelcomeBundle_ReportsGapExplicitly(t *testing.T) {
	orig := svcFindMissedOnboarding
	t.Cleanup(func() { svcFindMissedOnboarding = orig })
	svcFindMissedOnboarding = func(context.Context) (services.MissedOnboardingReport, error) {
		return services.MissedOnboardingReport{WelcomeBundleConfigured: false, Missed: []services.MissedOnboarding{}}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/onboarding/missed", nil)
	rr := httptest.NewRecorder()
	handleGetMissedOnboarding(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var got services.MissedOnboardingReport
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.WelcomeBundleConfigured {
		t.Fatal("expected welcome_bundle_configured=false to survive the round trip")
	}
	if got.Missed == nil || len(got.Missed) != 0 {
		t.Fatalf("expected an empty (not null) missed list, got %+v", got.Missed)
	}
}

func TestHandleGetMissedOnboarding_ReturnsFoundPeople(t *testing.T) {
	orig := svcFindMissedOnboarding
	t.Cleanup(func() { svcFindMissedOnboarding = orig })
	svcFindMissedOnboarding = func(context.Context) (services.MissedOnboardingReport, error) {
		return services.MissedOnboardingReport{
			WelcomeBundleConfigured: true,
			Missed:                  []services.MissedOnboarding{{UserID: "u1", Name: "Jamie", Email: "jamie@example.edu"}},
		}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/onboarding/missed", nil)
	rr := httptest.NewRecorder()
	handleGetMissedOnboarding(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var got services.MissedOnboardingReport
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !got.WelcomeBundleConfigured || len(got.Missed) != 1 || got.Missed[0].UserID != "u1" {
		t.Fatalf("expected configured=true, missed=[u1], got %+v", got)
	}
}

func TestHandleGetMissedOnboarding_PropagatesFault(t *testing.T) {
	orig := svcFindMissedOnboarding
	t.Cleanup(func() { svcFindMissedOnboarding = orig })
	svcFindMissedOnboarding = func(context.Context) (services.MissedOnboardingReport, error) {
		return services.MissedOnboardingReport{}, errors.New("zitadel unreachable")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/onboarding/missed", nil)
	rr := httptest.NewRecorder()
	handleGetMissedOnboarding(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 (a fault must not read as an empty result), got %d", rr.Code)
	}
}
