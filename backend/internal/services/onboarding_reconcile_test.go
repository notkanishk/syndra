package services

import (
	"context"
	"errors"
	"testing"

	"syndra/internal/db"
	"syndra/internal/directory"
	"syndra/internal/models"
)

func resetReconcileDeps(t *testing.T) {
	t.Helper()
	origBundle := svcGetWelcomeBundle
	origHolders := svcGetUsersForBundle
	origUsers := svcDirectoryUsers
	t.Cleanup(func() {
		svcGetWelcomeBundle = origBundle
		svcGetUsersForBundle = origHolders
		svcDirectoryUsers = origUsers
	})
}

// The whole point of the reconciler: a person Zitadel says exists, with no
// row anywhere saying they were ever considered, still gets found — because
// this asks the state of the world, not the onboarding_triggers log.
func TestFindMissedOnboarding_FindsPersonWithNoTriggerRowAtAll(t *testing.T) {
	resetReconcileDeps(t)
	svcGetWelcomeBundle = func(context.Context) (string, error) { return "bundle-welcome", nil }
	svcGetUsersForBundle = func(context.Context, string) ([]string, error) { return []string{"u-has-it"}, nil }
	svcDirectoryUsers = func(context.Context) ([]models.UserProfile, error) {
		return []models.UserProfile{
			{ID: "u-has-it", Name: "Has It", Status: "active"},
			{ID: "u-silent-miss", Name: "Silent Miss", Status: "active"},
		}, nil
	}

	got, err := FindMissedOnboarding(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.WelcomeBundleConfigured {
		t.Fatal("expected WelcomeBundleConfigured=true")
	}
	if len(got.Missed) != 1 || got.Missed[0].UserID != "u-silent-miss" {
		t.Fatalf("expected exactly [u-silent-miss], got %+v", got.Missed)
	}
}

// Nobody deactivated or locked is "missed" — they are not who the welcome
// bundle exists for.
func TestFindMissedOnboarding_IgnoresInactivePeople(t *testing.T) {
	resetReconcileDeps(t)
	svcGetWelcomeBundle = func(context.Context) (string, error) { return "bundle-welcome", nil }
	svcGetUsersForBundle = func(context.Context, string) ([]string, error) { return nil, nil }
	svcDirectoryUsers = func(context.Context) ([]models.UserProfile, error) {
		return []models.UserProfile{
			{ID: "u-locked", Status: "locked"},
			{ID: "u-deleted", Status: "deleted"},
		}, nil
	}

	got, err := FindMissedOnboarding(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Missed) != 0 {
		t.Fatalf("expected no missed people, got %+v", got.Missed)
	}
}

// A deployment that has never set a welcome bundle has nothing to have missed
// — reported as a fault-free empty list, with the gap itself carried
// explicitly in WelcomeBundleConfigured rather than left for the reader to
// infer from an empty Missed (that conflation is exactly what let five real
// accounts read as "failed" — migration 000046).
func TestFindMissedOnboarding_NoWelcomeBundleConfigured_ReturnsGapNotError(t *testing.T) {
	resetReconcileDeps(t)
	svcGetWelcomeBundle = func(context.Context) (string, error) { return "", db.ErrNoWelcomeBundleConfigured }
	svcDirectoryUsers = func(context.Context) ([]models.UserProfile, error) {
		t.Error("should not list people when there is no bundle to check against")
		return nil, nil
	}

	got, err := FindMissedOnboarding(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.WelcomeBundleConfigured {
		t.Fatal("expected WelcomeBundleConfigured=false")
	}
	if len(got.Missed) != 0 {
		t.Fatalf("expected an empty Missed, got %+v", got.Missed)
	}
}

// A real fault reading state must propagate — a silent empty result here
// would read as "nobody was missed" when the truth is "nobody could check".
func TestFindMissedOnboarding_DirectoryFault_Propagates(t *testing.T) {
	resetReconcileDeps(t)
	svcGetWelcomeBundle = func(context.Context) (string, error) { return "bundle-welcome", nil }
	svcGetUsersForBundle = func(context.Context, string) ([]string, error) { return nil, nil }
	svcDirectoryUsers = func(context.Context) ([]models.UserProfile, error) {
		return nil, errors.New("zitadel unreachable")
	}

	_, err := FindMissedOnboarding(context.Background())
	if err == nil {
		t.Fatal("expected the directory fault to propagate")
	}
}

// usersOnly is a directory that knows exactly these people and nothing else.
type usersOnly struct {
	directory.Source
	people []models.UserProfile
}

func (u usersOnly) Users(context.Context) ([]models.UserProfile, error) { return u.people, nil }

// svcDirectoryUsers used to be `directory.Default.Users`, a method value bound
// when the package initialised, while Default was still the demo source. Init()
// swaps Default later, so production kept reading the five demo identities and
// Home listed "dev_admin" and friends as new people without a bundle: none of
// them exist in Zitadel, so no name could ever resolve for them. The seam must
// follow Default at call time.
func TestFindMissedOnboarding_ReadsTheDirectoryInUseNotTheOneAtInit(t *testing.T) {
	origDir := directory.Default
	origBundle := svcGetWelcomeBundle
	origHolders := svcGetUsersForBundle
	t.Cleanup(func() {
		directory.Default = origDir
		svcGetWelcomeBundle = origBundle
		svcGetUsersForBundle = origHolders
	})
	svcGetWelcomeBundle = func(context.Context) (string, error) { return "bundle-welcome", nil }
	svcGetUsersForBundle = func(context.Context, string) ([]string, error) { return nil, nil }
	// What directory.Init() does at startup, after the package-level seams were bound.
	directory.Default = usersOnly{people: []models.UserProfile{
		{ID: "1234", Name: "Real Person", Status: "active"},
	}}

	got, err := FindMissedOnboarding(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Missed) != 1 || got.Missed[0].UserID != "1234" {
		t.Fatalf("expected only the person in the live directory, got %+v", got.Missed)
	}
}
