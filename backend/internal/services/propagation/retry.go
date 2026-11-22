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

// waitsForYou is a row the background drain never sends again on its own
// (db.ClaimDueRetries): it waits for the operator's Send.
func waitsForYou(row models.PendingPropagation) bool {
	return row.WaitReason == db.WaitReasonRoleWaits || row.WaitReason == db.WaitReasonManual
}

// sendsNext is who sends row again after nothing went out, the one sentence
// for it: Syndra by itself only when the background drain runs and claims it.
func sendsNext(system string, row models.PendingPropagation) string {
	if AutoRetry() && !waitsForYou(row) {
		return "Syndra sends it again by itself."
	}
	return "Send it again from Pending changes once " + system + " answers."
}

// RowRetry is where an unresolved row stands, or nil when there is nothing to
// say beyond its wait reason: never given a next try, being sent now, or held
// for the operator. A row deferred when nothing went out has a next try and
// no attempt spent (db.DeferPropagation): it reads "try 1 of N", the same
// rule as the count (db retryingRow) (a row that waits is never retried on its own, so it is not
// "retrying" whatever its attempts say). With the background drain off, a
// row Zitadel didn't take is waiting: only Send moves it.
func RowRetry(row models.PendingPropagation) *extcall.RetryState {
	if row.Status != "pending" || row.NotBefore == nil {
		return nil
	}
	if waitsForYou(row) {
		return nil
	}
	if !AutoRetry() {
		if row.Attempts == 0 {
			return nil // never tried, and nothing tries it: only its wait reason is true
		}
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
	if outcome == outcomeNotSent {
		// Nothing went out, so no try was spent: the line says how many are spent
		// so far, never "try 1" again each time the row is deferred.
		delete(details, "try")
		details["tries_spent"] = row.Attempts
	}
	details["outcome"] = outcome
	details["target"] = row.Target
	details["op_type"] = row.OpType
	details["project_id"] = row.ProjectID
	details["role_keys"] = row.RoleKeys
	if err := insertAudit(ctx, "system", row.UserID, "propagation.retried", row.ID, details); err != nil {
		log.Printf("[PROPAGATION] audit automatic try of row=%s: %v (non-fatal)", row.ID, err)
	}
}

// outcomeNotSent is an automatic try where nothing went out (the system was not
// answering, or maintenance held it back): the row was deferred, no attempt spent. The audit row
// names no cause.
const outcomeNotSent = "not_sent"

// retryOutcome reads what one row's processing did from the counters it moved.
func retryOutcome(before, after DrainResult) string {
	switch {
	case after.Applied > before.Applied:
		return "applied"
	case after.Exhausted > before.Exhausted:
		return "gave_up"
	case after.Failed > before.Failed:
		return "failed"
	case after.Deferred > before.Deferred:
		return outcomeNotSent
	case after.Requeued > before.Requeued:
		return "retrying"
	default:
		return "not_recorded"
	}
}
