package db

import (
	"context"
	"testing"
	"time"

	"syndra/internal/models"
)

// How each holder got a bundle, and discarding a draft, against the real
// schema: assign is direct with the actor, welcome names nobody, publish with
// migrate and a move both record the version they came from, and a discard
// puts the working copy back to the latest version without queueing anything.
func TestBundleHolderSourceAndDiscardDraft_AgainstTheSchema(t *testing.T) {
	ctx := liveDB(t)
	suffix := time.Now().Format("150405.000000000")
	alice, bob := "live-holder-a-"+suffix, "live-holder-b-"+suffix
	v1Role := models.BundleRole{ProjectID: "live-holders", RoleKey: "v1." + suffix}

	bundleID, err := CreateBundle(ctx, "live holders "+suffix, "", "auto", []models.BundleRole{v1Role})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = PG.Exec(ctx, `DELETE FROM bundles WHERE id = $1`, bundleID)
		_, _ = PG.Exec(ctx, `DELETE FROM audit_logs WHERE target_zitadel_user_id = ANY($1)`, []string{bundleID, alice, bob})
	})
	v1, err := LatestVersion(ctx, bundleID)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok, err := AssignBundleAndEnqueue(ctx, "op-1", alice, bundleID, v1.ID, nil); err != nil || !ok {
		t.Fatalf("assign: %v %v", ok, err)
	}
	if _, ok, err := AssignBundleAndEnqueue(ctx, WelcomeActor, bob, bundleID, v1.ID, nil); err != nil || !ok {
		t.Fatalf("welcome: %v %v", ok, err)
	}
	holders := holdersByUser(t, ctx, bundleID)
	expectHolder(t, holders[alice], "direct", "op-1", 0)
	expectHolder(t, holders[bob], "welcome", "", 0)

	// Discard: an unpublished add goes away, nothing is queued, it is audited.
	if _, err := AddRoleToBundleAndEnqueue(ctx, "op-1", bundleID, "live-holders", "draft."+suffix, nil); err != nil {
		t.Fatal(err)
	}
	truncateOutbox(t, ctx)
	if err := DiscardBundleDraft(ctx, "op-2", bundleID); err != nil {
		t.Fatal(err)
	}
	working, err := GetRolesForBundle(ctx, bundleID)
	if err != nil || len(working) != 1 || working[0].RoleKey != v1Role.RoleKey {
		t.Fatalf("working copy is back to v1 exactly: %+v %v", working, err)
	}
	var queued, audited int
	_ = PG.QueryRow(ctx, `SELECT count(*) FROM propagation_outbox`).Scan(&queued)
	_ = PG.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE action = 'bundle.draft_discarded'
		AND actor_zitadel_user_id = 'op-2' AND target_zitadel_user_id = $1`, bundleID).Scan(&audited)
	if queued != 0 || audited != 1 {
		t.Fatalf("discard touches nobody's access and is audited once: queued=%d audited=%d", queued, audited)
	}
	if err := DiscardBundleDraft(ctx, "op-2", "00000000-0000-0000-0000-000000000000"); err == nil {
		t.Fatal("discarding an unknown bundle's draft is refused, not an emptied working copy")
	}

	// Publish v2, moving alice only; then move her back.
	v2, _, err := PublishVersionAndEnqueue(ctx, "op-3", bundleID, "v2", working, []string{alice}, nil)
	if err != nil {
		t.Fatal(err)
	}
	holders = holdersByUser(t, ctx, bundleID)
	expectHolder(t, holders[alice], "moved", "op-3", 1)
	expectHolder(t, holders[bob], "welcome", "", 0)

	if _, err := MoveHoldersAndEnqueue(ctx, "op-4", bundleID, v1.ID, []string{alice}, nil); err != nil {
		t.Fatal(err)
	}
	expectHolder(t, holdersByUser(t, ctx, bundleID)[alice], "moved", "op-4", v2.Version)

	// A move onto the version they already sit on records nothing.
	if _, err := MoveHoldersAndEnqueue(ctx, "op-5", bundleID, v1.ID, []string{bob}, nil); err != nil {
		t.Fatal(err)
	}
	expectHolder(t, holdersByUser(t, ctx, bundleID)[bob], "welcome", "", 0)
}

func holdersByUser(t *testing.T, ctx context.Context, bundleID string) map[string]models.BundleHolder {
	t.Helper()
	hs, err := GetBundleHoldersByVersion(ctx, bundleID)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]models.BundleHolder{}
	for _, h := range hs {
		out[h.UserID] = h
	}
	return out
}

func expectHolder(t *testing.T, h models.BundleHolder, source, by string, from int) {
	t.Helper()
	if h.Source == nil || *h.Source != source {
		t.Fatalf("%s: source = %v, want %s", h.UserID, h.Source, source)
	}
	if (by == "") != (h.AssignedBy == nil) || (h.AssignedBy != nil && *h.AssignedBy != by) {
		t.Fatalf("%s: assigned_by = %v, want %q", h.UserID, h.AssignedBy, by)
	}
	if (from == 0) != (h.MovedFromVersion == nil) || (h.MovedFromVersion != nil && *h.MovedFromVersion != from) {
		t.Fatalf("%s: moved_from_version = %v, want %d", h.UserID, h.MovedFromVersion, from)
	}
}
