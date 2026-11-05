package services

import (
	"context"
	"testing"
	"time"

	"syndra/internal/db"
	"syndra/internal/extcall"
	"syndra/internal/models"
	"syndra/internal/services/propagation"
)

// A send Zitadel didn't take (503, no answer, 429) is requeued with a backoff
// and sent again by the background drain. The outcome must not say "Sent": it
// says halted:retrying, with the next try and which try, read from the row.
func TestDeliver_ARequeuedSendSaysItIsSentAgainAndWhen(t *testing.T) {
	deliveryFixture(t, map[string]bool{}, db.ConfirmationModelRoles, "")
	svcDrainBatch = func(_ context.Context, ids []string) (propagation.DrainResult, error) {
		return propagation.DrainResult{Requeued: len(ids)}, nil
	}
	next := time.Date(2026, 11, 5, 8, 35, 0, 0, time.UTC) // 14:05 IST
	origRows := svcGetPendingRows
	t.Cleanup(func() { svcGetPendingRows = origRows })
	svcGetPendingRows = func(context.Context) ([]models.PendingPropagation, error) {
		return []models.PendingPropagation{
			{ID: "other", Status: "pending", Attempts: 3, NotBefore: &next},
			{ID: "r1", Status: "pending", OpType: "add", Attempts: 1, NotBefore: &next},
		}, nil
	}

	d := Deliver(context.Background(), []string{"r1"}, false)
	if got := d.WaitReason(); got != "halted:retrying" {
		t.Fatalf("WaitReason = %q, want halted:retrying (never \"\", which reads as Sent)", got)
	}
	r := d.Drain.Retry
	if r == nil || r.State != extcall.RetryRetrying || r.Try != 2 || r.Of != propagation.RetryRule().Attempts || !r.NextTry.Equal(next) {
		t.Fatalf("Retry = %+v, want retrying, try 2 of %d, next %v", r, propagation.RetryRule().Attempts, next)
	}
}

// A state write that failed (Errored) leaves the row for the next drain to
// reclaim: still not "Sent".
func TestDeliver_AnUnrecordedSendIsNotSent(t *testing.T) {
	deliveryFixture(t, map[string]bool{}, db.ConfirmationModelRoles, "")
	svcDrainBatch = func(context.Context, []string) (propagation.DrainResult, error) {
		return propagation.DrainResult{Errored: 1}, nil
	}
	if got := Deliver(context.Background(), []string{"r1"}, false).WaitReason(); got != "halted:retrying" {
		t.Fatalf("WaitReason = %q, want halted:retrying", got)
	}
}

// A role that waits still outranks a retry: the operator's wait is the reason.
func TestDeliver_ARoleThatWaitsOutranksARetry(t *testing.T) {
	deliveryFixture(t, map[string]bool{"r1": true}, db.ConfirmationModelRoles, "")
	svcDrainBatch = func(_ context.Context, ids []string) (propagation.DrainResult, error) {
		return propagation.DrainResult{Requeued: len(ids)}, nil
	}
	origRows := svcGetPendingRows
	t.Cleanup(func() { svcGetPendingRows = origRows })
	svcGetPendingRows = func(context.Context) ([]models.PendingPropagation, error) { return nil, nil }
	if got := Deliver(context.Background(), []string{"r1", "r2"}, false).WaitReason(); got != db.WaitReasonRoleWaits {
		t.Fatalf("WaitReason = %q, want role_waits", got)
	}
}
