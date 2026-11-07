package services

import (
	"context"
	"maps"
	"slices"

	"syndra/internal/clock"
	"syndra/internal/db"
	"syndra/internal/models"
)

// System work through the planner (architecture deepening, task 1.9;
// decision 1, "system work joins later through the same entry"). A grant's
// expiry, the welcome bundle, a rule's own edit and a bundle's retirement each
// read the one snapshot (db.ReadAccessSnapshot) under the access lock, plan
// every person on it, write exactly the Change planned, and hand it to
// DeliverChange after the commit. How each write is delivered (straight away,
// or waiting for you because its role waits) and which queued changes it
// cancels are the planner's (Change.deliver, db.Answers); nothing after the
// commit decides either again.
//
// System work has nobody to ask. Expiry and the welcome bundle run under a
// gate of their own that admits exactly the cancellations their plan named
// (systemGate): planned under the same lock as the write, so the same ones
// the outbox finds, and any other is refused with nothing written. A rule's
// edit and a bundle's retirement are an operator's, and keep the operator's
// gate: they cancel only what the operator was shown (`?cancel_ids=`).

// systemChange plans one person's part of system work: what they hold before
// and after, through the rules before and after, attributed to source and
// ref. A role a rule follows with is the rule's row (decision 7), except in a
// rule's own edit, where every row is the edited rule's (task 1.6a).
func systemChange(userID string, person db.PersonAccess, s PlanSnapshot, before, after map[roleKey]bool,
	rulesBefore, rulesAfter []models.MappingRule, source, ref string) Change {
	closureBefore, closureAfter := effectiveClosure(before, rulesBefore), effectiveClosure(after, rulesAfter)
	adds, revokes := closureDelta(closureBefore, closureAfter)
	writes := deltaWrites(userID, adds, revokes, source, ref)
	if source != "rule" {
		writes = asRuleRows(writes, ruleRefs(before, after, closureBefore, closureAfter, rulesAfter))
	}
	ch := Change{UserID: userID}.deliver(writes, person, s)
	for _, c := range ch.Cancels {
		ch.Row.CancelIDs = append(ch.Row.CancelIDs, c.ID)
	}
	return ch
}

// planExpiry plans the end of one grant whose end has passed: the access it
// alone gave, taken back, as an operator's removal would. A grant still held
// under the lock (renewed since the sweep read it), or whose end is still
// ahead of the snapshot's clock, is not ended: "until 30 Nov" lasts the whole
// of 30 Nov in the makerspace's zone (clock.EndOfDay), so it ends at midnight.
func planExpiry(g models.DirectGrant, person db.PersonAccess, s PlanSnapshot) Change {
	for _, d := range person.Direct {
		if d.ID == g.ID {
			g = d
		}
	}
	if g.ExpiresAt == nil || g.ExpiresAt.After(s.Now) {
		return Change{UserID: g.UserID, Refusal: "renewed"}
	}
	without := person
	without.Direct = slices.DeleteFunc(slices.Clone(person.Direct), func(d models.DirectGrant) bool { return d.ID == g.ID })
	after := baseFrom(without, "")
	// Built from after, not read: base holdings leave out a grant that has
	// ended, so a "before" read would not have it either.
	before := maps.Clone(after)
	before[roleKey{projectID: g.ProjectID, roleKey: g.RoleKey}] = true
	rules := s.Access.Rules
	return systemChange(g.UserID, person, s, before, after, rules, rules, "direct", g.ID)
}

// systemSnapshot reads the snapshot system work plans on, for these people,
// under the lock the caller holds.
func systemSnapshot(ctx context.Context, userIDs []string) (PlanSnapshot, error) {
	snap, err := svcReadAccessSnapshot(ctx, userIDs)
	return PlanSnapshot{Access: snap, Now: clock.Now()}, err
}

// systemGate gives system work a gate of its own: nobody is asked, so it
// admits only what plannedCancels names once the plan is read (the gate's
// list starts empty).
func systemGate(ctx context.Context) context.Context {
	ctx, _ = db.GuardCancellations(ctx, nil, nil)
	return ctx
}

// plannedCancels is every queued change these Changes cancel.
func plannedCancels(changes []Change) []string {
	var ids []string
	for _, ch := range changes {
		ids = append(ids, ch.Row.CancelIDs...)
	}
	return ids
}

// systemParams are the Changes' writes as the outbox takes them, with the
// target convergence each person's changed roles reach registered first.
func systemParams(ctx context.Context, actor, reason string, changes []Change) ([]db.EnqueueParams, error) {
	var params []db.EnqueueParams
	for _, ch := range changes {
		if err := deferTargetConvergence(ctx, actor, ch.UserID, ch.changed()); err != nil {
			return nil, err
		}
		params = append(params, ch.params(actor, reason)...)
	}
	return params, nil
}

// pairWritten hands each Change its outbox rows: the rows come back in the
// order the writes went in, and each Change takes as many as its writes
// leave (every write but one that only cancels).
func pairWritten(changes []Change, ids []string) ([]Change, error) {
	out := make([]Change, len(changes))
	next := 0
	for i, ch := range changes {
		n := len(ch.Writes) - len(ch.delivered(DeliveryNone))
		if next+n > len(ids) {
			return nil, errWrittenOtherwise
		}
		var err error
		if out[i], err = ch.written(ids[next : next+n]); err != nil {
			return nil, err
		}
		next += n
	}
	if next != len(ids) {
		return nil, errWrittenOtherwise
	}
	return out, nil
}

// joined is every Change's writes as one, so one send carries them all.
func joined(changes []Change) Change {
	var all Change
	for _, ch := range changes {
		all.Writes = append(all.Writes, ch.Writes...)
	}
	return all
}

// deliverSystem sends what the plan said goes now (DeliverChange) and says
// what became of it. A send that fails is non-fatal: the rows are committed
// and stay in Pending changes, retried or waiting for Send, which is the safe
// way to be wrong (returning an error after the commit would invite a retry
// that runs the whole change again).
func deliverSystem(ctx context.Context, changes []Change) CascadeResult {
	all := joined(changes)
	d := DeliverChange(ctx, all)
	n := 0
	for _, w := range all.Writes {
		if w.OutboxID != "" {
			n++
		}
	}
	return CascadeResult{Enqueued: n, Drain: d.Drain, Waiting: len(d.Waiting), WaitReason: d.WaitReason()}
}

// deltaWrites are a delta's single-role writes, without side effects: what the
// planner decides (and asks db.Answers about) is what the apply enqueues.
func deltaWrites(userID string, adds, revokes []roleKey, source, sourceRef string) []db.EnqueueParams {
	params := make([]db.EnqueueParams, 0, len(adds)+len(revokes))
	for _, k := range adds {
		params = append(params, db.EnqueueParams{UserID: userID, ProjectID: k.projectID, RoleKeys: []string{k.roleKey},
			Source: source, SourceRef: sourceRef, OpType: "add"})
	}
	for _, k := range revokes {
		params = append(params, db.EnqueueParams{UserID: userID, ProjectID: k.projectID, RoleKeys: []string{k.roleKey},
			Source: source, SourceRef: sourceRef, OpType: "revoke"})
	}
	return params
}
