package propagation

import (
	"context"
	"log"
	"sync/atomic"

	"syndra/internal/db"
	"syndra/internal/extcall"
	"syndra/internal/models"
)

// RetryRule is the outbox's rule in the one retry model (extcall): the first
// send plus OUTBOX_MAX_RETRIES more, spaced by the backoff the requeue uses.
func RetryRule() extcall.Policy {
	p := extcall.OutboxRetry
	p.Attempts = maxRetries + 1
	return p
}

// autoRetry is whether the background drain runs on this server
// (REVOCATION_DRAIN_ENABLED, cmd/api). Off, nothing sends a change again on
// its own, so no row may say "Syndra sends it again at 14:05".
var autoRetry atomic.Bool

func init() { autoRetry.Store(true) }

// SetAutoRetry records whether the background drain runs. Called once at start.
func SetAutoRetry(on bool) { autoRetry.Store(on) }

// AutoRetry reports whether Syndra sends a change again on its own here.
func AutoRetry() bool { return autoRetry.Load() }

// RowRetry is where an unresolved row stands, or nil when there is nothing to
// say beyond its wait reason: never tried, being sent now, or held for the
// operator (a row that waits is never retried on its own, so it is not
// "retrying" whatever its attempts say). With the background drain off, a
// row Zitadel didn't take is waiting: only Send moves it.
func RowRetry(row models.PendingPropagation) *extcall.RetryState {
	if row.Status != "pending" || row.Attempts == 0 || row.NotBefore == nil {
		return nil
	}
	if row.WaitReason == db.WaitReasonRoleWaits || row.WaitReason == db.WaitReasonManual {
		return nil
	}
	if !AutoRetry() {
		return &extcall.RetryState{State: extcall.RetryWaiting, Try: row.Attempts, Of: RetryRule().Attempts}
	}
	s := RetryRule().Retrying(row.Attempts, row.NotBefore)
	return &s
}

// auditRetry records one automatic try, so the history says Syndra sent it
// again on its own, which try it was, and how it went. Never fatal: the try
// already happened.
func auditRetry(ctx context.Context, row models.PendingPropagation, outcome string) {
	details := RetryRule().AuditMark(row.Attempts + 1)
	details["outcome"] = outcome
	details["target"] = row.Target
	details["op_type"] = row.OpType
	details["project_id"] = row.ProjectID
	details["role_keys"] = row.RoleKeys
	if err := insertAudit(ctx, "system", row.UserID, "propagation.retried", row.ID, details); err != nil {
		log.Printf("[PROPAGATION] audit automatic try of row=%s: %v (non-fatal)", row.ID, err)
	}
}

// retryOutcome reads what one row's processing did from the counters it moved.
func retryOutcome(before, after DrainResult) string {
	switch {
	case after.Applied > before.Applied:
		return "applied"
	case after.Exhausted > before.Exhausted:
		return "gave_up"
	case after.Failed > before.Failed:
		return "failed"
	case after.Requeued > before.Requeued:
		return "retrying"
	default:
		return "not_recorded"
	}
}
