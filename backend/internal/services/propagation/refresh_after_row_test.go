package propagation

import (
	"context"
	"strings"
	"testing"

	"syndra/internal/db"
	"syndra/internal/models"
)

// A failed row, and a row out of retries, still changed the ledger in the
// transaction that queued them; the cached claims must follow it.
func TestDrain_FailedAndExhaustedRowsRefreshTheCachedClaims(t *testing.T) {
	stubDrainDeps(t)
	failedRow := models.PendingPropagation{ID: "bad", Target: db.TargetZitadel, OpType: "add",
		UserID: "u1", ProjectID: "p1", RoleKeys: []string{"r"}}
	spent := models.PendingPropagation{ID: "spent", Target: db.TargetZitadel, OpType: "add",
		UserID: "u2", ProjectID: "p2", RoleKeys: []string{"r"}, Attempts: maxRetries}
	claimPending = func(context.Context, string, int) ([]models.PendingPropagation, error) {
		return []models.PendingPropagation{failedRow, spent}, nil
	}
	zitadelAddUserGrant = func(_ context.Context, userID, _ string, _ []string) error {
		if userID == "u1" {
			return statusErr(400)
		}
		return statusErr(500)
	}
	markFailed = func(context.Context, string, string) error { return nil }
	refreshed := map[string]string{}
	refreshClaims = func(_ context.Context, ids []string, p ...string) error {
		refreshed[ids[0]] = strings.Join(p, ",")
		return nil
	}

	if _, err := Drain(context.Background()); err != nil {
		t.Fatalf("Drain: %v", err)
	}
	if refreshed["u1"] != "p1" || refreshed["u2"] != "p2" {
		t.Fatalf("a failed or exhausted row left the facts stale: %v", refreshed)
	}
}
