package services

import (
	"context"
	"log"

	"syndra/internal/db"
	"syndra/internal/extcall"
	"syndra/internal/services/propagation"
)

// Delivery is what happened to rows just written to the outbox: which went
// to Zitadel now, and which wait for the operator, with why.
type Delivery struct {
	Sent    []string // meant to go now (some may not have, if Drain halted)
	Waiting []string // held for the operator
	Reason  map[string]string
	Drain   propagation.DrainResult

	// retrying: a halted send's rows were given a next try (db.ScheduleRetry).
	retrying bool
}

// WaitReason says why the action's change isn't in Zitadel yet: the role
// waits (or, before the switch over, its mode did), the send halted, or
// Zitadel didn't take it and Syndra sends it again on its own
// (halted:retrying; Drain.Retry says when, and which try). "" only when
// everything went.
func (d Delivery) WaitReason() string {
	for _, id := range d.Waiting {
		return d.Reason[id]
	}
	if d.retrying {
		return db.WaitReasonHalted(db.HaltRetrying)
	}
	if d.Drain.Halted {
		return db.WaitReasonHalted(d.Drain.Reason)
	}
	if d.Drain.Requeued+d.Drain.Errored > 0 {
		return db.WaitReasonHalted(db.HaltRetrying)
	}
	return ""
}

// --- injectable deps (swapped in tests) ---
var (
	svcOutboxRowsWaitForRole = db.OutboxRowsWaitForRole
	svcSetWaitReason         = db.SetWaitReason
	svcGetConfigSetting      = db.GetConfigSetting
	svcGetPendingRows        = db.GetPendingPropagations
	svcScheduleRetry         = db.ScheduleRetry
)

// switchedOver reports whether the operator has moved to roles that wait. An
// unreadable setting counts as not yet: the old modes are the slower rule,
// and the slower rule is the safe one to fall back to.
func switchedOver(ctx context.Context) bool {
	v, err := svcGetConfigSetting(ctx, db.ConfigKeyConfirmationModel)
	if err != nil {
		log.Printf("[DELIVERY] read %s: %v (keeping the old modes)", db.ConfigKeyConfirmationModel, err)
		return false
	}
	return v == db.ConfirmationModelRoles
}

// Deliver is the one place that decides, for rows just written, whether each
// goes to Zitadel straight away or waits in Pending changes (N14 B, N15). A
// row waits when any role it carries is marked to wait, however it arrived.
//
// Until the operator switches over, the old rule still holds as well:
// legacyWait is what the caller's own mode said (a bundle or rule set to
// wait, a grant sent without apply). So nothing reaches Zitadel sooner than
// it did before the switch. After it, only the role decides.
//
// Rows meant to go that a halted send left behind record why
// (halted:zitadel_offline, …), so the outcome and Pending changes can say it.
// A failure to decide or record is non-fatal: the rows are already committed
// and stay pending, which is the safe way to be wrong.
func Deliver(ctx context.Context, ids []string, legacyWait bool) Delivery {
	d := Delivery{Reason: map[string]string{}}
	if len(ids) == 0 {
		return d
	}
	waits, err := svcOutboxRowsWaitForRole(ctx, ids)
	if err != nil {
		log.Printf("[DELIVERY] read roles that wait for %d row(s): %v (left waiting)", len(ids), err)
		d.Waiting = ids
		reason := db.WaitReasonHalted("delivery_unread")
		for _, id := range ids {
			d.Reason[id] = reason
		}
		if err := svcSetWaitReason(ctx, ids, reason); err != nil {
			log.Printf("[DELIVERY] record %s on %d row(s): %v", reason, len(ids), err)
		}
		return d
	}
	legacy := legacyWait && !switchedOver(ctx)

	var roleWaits, manual []string
	for _, id := range ids {
		switch {
		case waits[id]:
			roleWaits = append(roleWaits, id)
		case legacy:
			manual = append(manual, id)
		default:
			d.Sent = append(d.Sent, id)
		}
	}
	d.wait(ctx, roleWaits, db.WaitReasonRoleWaits)
	d.wait(ctx, manual, db.WaitReasonManual)

	if len(d.Sent) == 0 {
		return d
	}
	dr, derr := svcDrainBatch(ctx, d.Sent)
	d.Drain = dr
	if derr != nil {
		log.Printf("[DELIVERY] send of %d row(s) failed (left pending): %v", len(d.Sent), derr)
		d.Drain.Halted = true
		if d.Drain.Reason == "" {
			d.Drain.Reason = "drain_error"
		}
	}
	if d.Drain.Halted {
		// Which of them went is the outbox's to say. Those still pending were
		// meant to go now and hit a transient failure before they went, so
		// they are retried on their own like a 503 after a send (owner, 5 Nov
		// 2026): try 1 counted, a next try, each retry audited by the
		// background drain. If the schedule can't be written, the halt reason
		// is recorded and they wait for Send: the safe way to be wrong.
		scheduled, err := svcScheduleRetry(ctx, d.Sent, "halted: "+d.Drain.Reason)
		if err != nil {
			log.Printf("[DELIVERY] schedule a retry of %d row(s): %v (left for Send)", len(d.Sent), err)
			d.wait(ctx, d.Sent, db.WaitReasonHalted(d.Drain.Reason))
		}
		for _, id := range scheduled {
			d.Reason[id] = db.WaitReasonHalted(db.HaltRetrying)
		}
		d.retrying = len(scheduled) > 0
	}
	if d.Drain.Requeued > 0 || d.retrying {
		d.Drain.Retry = nextRetry(ctx, d.Sent)
	}
	return d
}

// nextRetry is where the first of these rows Zitadel didn't take stands: its
// next try and which try of how many, read from the row itself so the words
// the operator sees are the schedule the background drain keeps. nil when it
// can't be read; the outcome then says only that Syndra sends it again.
//
// ponytail: reads the whole unresolved queue (tens of rows) to find a few ids.
func nextRetry(ctx context.Context, ids []string) *extcall.RetryState {
	rows, err := svcGetPendingRows(ctx)
	if err != nil {
		log.Printf("[DELIVERY] read the next try of %d row(s): %v", len(ids), err)
		return nil
	}
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	for _, row := range rows {
		if want[row.ID] {
			if s := propagation.RowRetry(row); s != nil {
				return s
			}
		}
	}
	return nil
}

// wait records reason on rows still pending. Role and manual waits are
// Waiting; a halted send's rows stay in Sent, since some may have gone.
func (d *Delivery) wait(ctx context.Context, ids []string, reason string) {
	if len(ids) == 0 {
		return
	}
	if err := svcSetWaitReason(ctx, ids, reason); err != nil {
		log.Printf("[DELIVERY] record %s on %d row(s): %v", reason, len(ids), err)
	}
	if reason == db.WaitReasonRoleWaits || reason == db.WaitReasonManual {
		d.Waiting = append(d.Waiting, ids...)
	}
	for _, id := range ids {
		d.Reason[id] = reason
	}
}
