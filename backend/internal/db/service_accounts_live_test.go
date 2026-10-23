package db

import (
	"testing"
	"time"
)

// A mark is stored once: changing kind, purpose or owner keeps who marked it
// and when, and removing it forgets it.
func TestServiceAccounts_MarkChangeUnmark(t *testing.T) {
	ctx := liveDB(t)
	id := "u-sa-" + time.Now().Format("150405.000000")
	t.Cleanup(func() { _, _ = PG.Exec(ctx, `DELETE FROM service_accounts WHERE user_id = $1`, id) })

	first, err := UpsertServiceAccount(ctx, id, "device", "Front panel", "", "admin-1")
	if err != nil {
		t.Fatal(err)
	}
	if first.OwnerID != "" || first.MarkedBy != "admin-1" {
		t.Fatalf("first mark: %+v", first)
	}
	changed, err := UpsertServiceAccount(ctx, id, "agent", "Books rooms", "u-owner", "admin-2")
	if err != nil {
		t.Fatal(err)
	}
	if changed.Kind != "agent" || changed.OwnerID != "u-owner" || changed.MarkedBy != "admin-1" || !changed.MarkedAt.Equal(first.MarkedAt) {
		t.Fatalf("a change keeps the original mark: %+v", changed)
	}
	if _, err := UpsertServiceAccount(ctx, id, "robot", "", "", "admin-1"); err == nil {
		t.Fatal("the kind CHECK must refuse an unknown kind")
	}
	rows, err := ListServiceAccounts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range rows {
		found = found || r.UserID == id
	}
	if !found {
		t.Fatal("listed")
	}
	if ok, err := DeleteServiceAccount(ctx, id); err != nil || !ok {
		t.Fatalf("unmark: %v %v", ok, err)
	}
	if ok, _ := DeleteServiceAccount(ctx, id); ok {
		t.Fatal("nothing left to remove")
	}
}
