package services

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"syndra/internal/db"
	"syndra/internal/models"
)

// A machine user is a service account: no trigger is recorded and no welcome
// bundle is assigned.
func TestTriggerOnboarding_MachineUserGetsNothing(t *testing.T) {
	resetOnboardingDeps(t)
	happyPathDeps()
	svcFindUser = func(context.Context, string) (models.UserProfile, bool, error) {
		return models.UserProfile{ID: "svc", Kind: models.UserKindMachine}, true, nil
	}
	svcInsertOnboardingTrigger = func(context.Context, string, string, string) (string, bool, error) {
		t.Error("a machine user must not record an onboarding trigger")
		return "", false, nil
	}
	svcCascadeWelcomeBundle = func(context.Context, string, string, string) (CascadeResult, error) {
		t.Error("a machine user must not be handed the welcome bundle")
		return CascadeResult{}, nil
	}

	if err := TriggerOnboarding(context.Background(), "svc", "webhook", "key-m"); err != nil {
		t.Fatalf("a machine user is skipped, not failed: %v", err)
	}
}

// A human is still onboarded; a failed read of who the user is is neither
// "person" nor "machine", so it fails and Zitadel redelivers.
func TestTriggerOnboarding_HumanOnboardedAndUnreadableUserFails(t *testing.T) {
	resetOnboardingDeps(t)
	happyPathDeps()
	assigned := false
	svcCascadeWelcomeBundle = func(context.Context, string, string, string) (CascadeResult, error) {
		assigned = true
		return CascadeResult{}, nil
	}
	svcFindUser = func(context.Context, string) (models.UserProfile, bool, error) {
		return models.UserProfile{ID: "u1", Kind: models.UserKindHuman}, true, nil
	}
	if err := TriggerOnboarding(context.Background(), "u1", "webhook", "key-h"); err != nil || !assigned {
		t.Fatalf("a human is onboarded: err=%v assigned=%v", err, assigned)
	}

	assigned = false
	svcFindUser = func(context.Context, string) (models.UserProfile, bool, error) {
		return models.UserProfile{}, false, errors.New("zitadel unreachable")
	}
	if err := TriggerOnboarding(context.Background(), "u1", "webhook", "key-h2"); err == nil || assigned {
		t.Fatalf("an unreadable user must fail the trigger: err=%v assigned=%v", err, assigned)
	}
}

// A service account is not a person missing a bundle: a machine user holding no
// welcome bundle never appears in the report, the people beside it still do.
func TestFindMissedOnboarding_ExcludesMachineAccounts(t *testing.T) {
	resetReconcileDeps(t)
	svcGetWelcomeBundle = func(context.Context) (string, error) { return "bundle-welcome", nil }
	svcGetBundleHolders = func(context.Context) ([]string, error) { return nil, nil }
	svcDirectoryUsers = func(context.Context) ([]models.UserProfile, error) {
		return []models.UserProfile{
			{ID: "u-person", Name: "Person", Status: "active", Kind: models.UserKindHuman},
			{ID: "u-netbird", Name: "netbird", Status: "active", Kind: models.UserKindMachine},
			{ID: "u-legacy", Name: "No Kind", Status: "active"},
		}, nil
	}

	got, err := FindMissedOnboarding(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Missed) != 2 || got.Missed[0].UserID != "u-person" || got.Missed[1].UserID != "u-legacy" {
		t.Fatalf("want the two people and no machine, got %+v", got.Missed)
	}
}

// The report carries when each person joined, so Home can put the newest first
// (A-07 §33); a person Zitadel gave no date for carries none.
func TestFindMissedOnboarding_CarriesJoined(t *testing.T) {
	resetReconcileDeps(t)
	joined := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	svcGetWelcomeBundle = func(context.Context) (string, error) { return "bundle-welcome", nil }
	svcGetBundleHolders = func(context.Context) ([]string, error) { return nil, nil }
	svcDirectoryUsers = func(context.Context) ([]models.UserProfile, error) {
		return []models.UserProfile{
			{ID: "u-dated", Name: "Dated", Status: "active", Joined: &joined},
			{ID: "u-undated", Name: "Undated", Status: "active"},
		}, nil
	}

	got, err := FindMissedOnboarding(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Missed) != 2 || got.Missed[0].Joined == nil || !got.Missed[0].Joined.Equal(joined) || got.Missed[1].Joined != nil {
		t.Fatalf("want the join time carried and nil where unknown, got %+v", got.Missed)
	}
	raw, _ := json.Marshal(got.Missed[1])
	if strings.Contains(string(raw), "joined") {
		t.Fatalf("an unknown join time is omitted, got %s", raw)
	}
}

// Machine accounts stay in the audit: unexplained access on a service account
// matters as much as on a person. The row is kept and says what it is by the
// directory's type, whatever the account is called.
func TestDriftTriageQueue_KeepsMachineAccountsByKind(t *testing.T) {
	withTriageDeps(t,
		[]models.DriftItem{{ID: "x", Target: db.TargetZitadel, UserID: "m1", ProjectID: "p_wiki", RoleKeys: []string{"read"}, DetectedAt: time.Now()}},
		map[string]models.UserProfile{
			"m1": {ID: "m1", Name: "netbird", Status: "active", Kind: models.UserKindMachine},
		})

	got, err := DriftTriageQueue(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !got[0].UserIsServiceAccount {
		t.Fatalf("a machine user's drift must stay, marked as a service account: %+v", got)
	}
}
