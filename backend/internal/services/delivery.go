package services

import (
	"context"
	"log"

	"syndra/internal/db"
	"syndra/internal/services/propagation"
)

// Delivery is what happened to rows just written to the outbox: which went
// to Zitadel now, and which wait for the operator, with why.
type Delivery struct {
	Sent    []string // meant to go now (some may not have, if Drain halted)
	Waiting []string // held for the operator
	Reason  map[string]string
	Drain   propagation.DrainResult
}

// WaitReason says why the action's change isn't in Zitadel yet: the role
// waits (or, before the switch over, its mode did), or the send halted. ""
// when everything went.
func (d Delivery) WaitReason() string {
	for _, id := range d.Waiting {
		return d.Reason[id]
	}
	if d.Drain.Halted {
		return db.WaitReasonHalted(d.Drain.Reason)
	}
	return ""
}

// --- injectable deps (swapped in tests) ---
var (
	svcOutboxRowsWaitForRole = db.OutboxRowsWaitForRole
	svcSetWaitReason         = db.SetWaitReason
	svcGetConfigSetting      = db.GetConfigSetting
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
		for _, id := range ids {
			d.Reason[id] = db.WaitReasonHalted("delivery_unread")
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
		// Which of them went is the outbox's to say; the reason is recorded
		// only on the rows still pending.
		d.wait(ctx, d.Sent, db.WaitReasonHalted(d.Drain.Reason))
	}
	return d
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
