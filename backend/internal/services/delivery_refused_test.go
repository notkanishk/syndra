package services

import (
	"context"
	"testing"

	"syndra/internal/db"
	"syndra/internal/services/propagation"
)

// Final verification N1: a row Zitadel refused outright used to come back with
// no wait reason, so the act read as done. It now says refused, with the
// plain cause, for the whole delivery and for that row alone.
func TestDeliverChange_ARefusalIsSaidWithItsCause(t *testing.T) {
	deliveryFixture(t, "")
	svcDrainBatch = func(_ context.Context, ids []string) (propagation.DrainResult, error) {
		return propagation.DrainResult{Failed: 1, Applied: len(ids) - 1,
			Refused: map[string]string{"own": db.RefusalNotAllowed}}, nil
	}
	d := DeliverChange(context.Background(), planned([]string{"own", "other", "rule"}, map[string]bool{"rule": true}))

	want := db.WaitReasonRefused(db.RefusalNotAllowed)
	if got := d.WaitReason(); got != want {
		t.Fatalf("a refusal outranks a role that waits: got %q, want %q", got, want)
	}
	if got := d.ReasonFor("own"); got != want {
		t.Fatalf("the refused row says so: got %q", got)
	}
	if got := d.ReasonFor("other"); got != "" {
		t.Fatalf("a row Zitadel took is not blamed for the refusal: got %q", got)
	}
	if got := d.WaitingBesides("own"); got != 1 {
		t.Fatalf("the rule's waiting row is still counted apart: got %d", got)
	}
}

// A refusal inside a halted send keeps its own reason: the halt's retry is
// for the rows still pending, never for a terminal one.
func TestDeliverChange_ARefusalSurvivesAHalt(t *testing.T) {
	deliveryFixture(t, "")
	svcScheduleRetry = func(_ context.Context, ids []string, _ string) ([]string, error) {
		return []string{"later"}, nil // the refused row is terminal, so not scheduled
	}
	svcDrainBatch = func(_ context.Context, ids []string) (propagation.DrainResult, error) {
		return propagation.DrainResult{Failed: 1, Halted: true, Reason: "zitadel_offline",
			Refused: map[string]string{"own": db.RefusalNotFound}}, nil
	}
	d := DeliverChange(context.Background(), planned([]string{"own", "later"}, nil))
	if got := d.ReasonFor("own"); got != db.WaitReasonRefused(db.RefusalNotFound) {
		t.Fatalf("refused row: got %q", got)
	}
	if got := d.ReasonFor("later"); got != db.WaitReasonHalted(db.HaltRetrying) {
		t.Fatalf("the row the halt left behind retries: got %q", got)
	}
}
