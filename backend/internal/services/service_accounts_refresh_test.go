package services

import (
	"context"
	"errors"
	"testing"

	"syndra/internal/db"
	"syndra/internal/models"
)

// IsServiceAccount rides in the facts, so a mark and an unmark are writes the
// cached claims must follow; and a cache that is down never fails the write.
func TestServiceAccountWritesRefreshTheCachedClaims(t *testing.T) {
	stubServiceMarks(t, db.ServiceAccountRow{UserID: "u-kiosk", Kind: "device"})
	saveDeps(t, models.UserProfile{ID: "u-kiosk", Name: "Kiosk", Kind: models.UserKindHuman})
	oDel := svcDeleteServiceAccount
	t.Cleanup(func() { svcDeleteServiceAccount = oDel })
	svcDeleteServiceAccount = func(context.Context, string) (bool, error) { return true, nil }

	var refreshed []string
	svcRefresh = func(_ context.Context, ids []string, _ ...string) error {
		refreshed = append(refreshed, ids...)
		return errors.New("redis is down")
	}
	ctx := context.Background()
	if _, err := SaveServiceAccount(ctx, "u-kiosk", ServiceAccountInput{Kind: "device"}, "admin"); err != nil {
		t.Fatalf("a cache failure must not fail the mark: %v", err)
	}
	if err := UnmarkServiceAccount(ctx, "u-kiosk"); err != nil {
		t.Fatalf("a cache failure must not fail the unmark: %v", err)
	}
	if len(refreshed) != 2 || refreshed[0] != "u-kiosk" || refreshed[1] != "u-kiosk" {
		t.Fatalf("mark and unmark must each refresh the account: %v", refreshed)
	}
}
