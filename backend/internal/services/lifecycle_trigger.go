package services

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"syndra/internal/db"
)

// The lifecycle trigger (design §4, §12; change `addon-platform` 7.9).
//
// A role change reaches a target when somebody has mapped that role to it. That
// is the whole rule, and it is deliberately symmetric: gaining a first mapped
// role creates the account as part of the convergence, and losing the last one
// resolves the lifecycle fields to disabled. Nothing here special-cases
// restoration, because nothing special-cased suspension — an earlier draft had
// locking as a one-shot operation, and regaining a role could not undo it.
//
// It hangs off `deltaParams`, which is the one place every closure delta in this
// package becomes outbox rows: bundle assign and remove, bundle delete, rule
// create, update and delete, publish, direct-grant removal, and expiry. Hooking
// each of those instead would be nine hooks and one that gets forgotten — and
// the forgotten one is a person whose access changed in Zitadel and nowhere else.
//
// What it does NOT do is call the add-on. It runs inside the access-mutation
// lock, and that lock is deliberately never held across a network call: one
// unreachable target would serialise every access change in the deployment
// behind it. So the convergence is queued with a recorded intent and no
// fingerprint, and the drain re-reads live state before dispatching it.

// The trigger resolves AFTER the source write, and that is a structural
// property rather than a discipline.
//
// `deltaParams` is the one chokepoint every closure delta passes through, and
// it necessarily runs BEFORE the write it is building params for. Resolving
// there resolves the world the change has not happened in yet, and the answer
// is inverted in both directions: gaining a first mapped role resolves to "no
// mapped role, disable the account", and losing the last one resolves to "still
// holds it, keep the access". A level-triggered apply then converges the target
// to the opposite of the intent.
//
// So the chokepoint DEFERS and `withLockedAccess` FLUSHES: the resolve runs
// after fn has finished writing, still inside fn's transaction and still under
// the access-mutation lock. Registering rather than calling keeps the single
// hook — a cascade added later reaches its mapped targets without anybody
// remembering to wire it — and moves only the moment.

type pendingKeyType struct{}

var pendingKey pendingKeyType

// errConvergenceOutsideLock is returned rather than silently skipped. A closure
// delta computed outside the access-mutation lock is not a convergence that
// merely runs late; it is a role change with no path to its mapped targets, and
// it must fail the change rather than commit half of it.
var errConvergenceOutsideLock = errors.New(
	"closure delta built outside withLockedAccess: the lifecycle trigger has nowhere to register")

type pendingConvergence struct {
	actor   string
	userID  string
	changed []roleKey
}

type pendingConvergences struct{ items []pendingConvergence }

// add merges into an existing entry for the same subject and actor. One
// cascade can produce several deltas for one person — a bundle move is a
// removal and an addition — and each would otherwise queue its own convergence
// carrying the identical resolved set.
func (p *pendingConvergences) add(actor, userID string, changed []roleKey) {
	for i := range p.items {
		if p.items[i].actor == actor && p.items[i].userID == userID {
			p.items[i].changed = append(p.items[i].changed, changed...)
			return
		}
	}
	p.items = append(p.items, pendingConvergence{actor: actor, userID: userID, changed: changed})
}

// withLockedAccess is svcInTxLockingAccess plus the deferred lifecycle trigger.
//
// Every cascade in this package uses it instead of the raw lock, so the flush
// cannot be forgotten at a call site: a cascade that took the lock directly
// would register convergences into a context with no pending list and fail
// loudly at the first delta rather than quietly converging nothing.
func withLockedAccess(ctx context.Context, fn func(context.Context) error) error {
	return svcInTxLockingAccess(ctx, func(ctx context.Context) error {
		pending := &pendingConvergences{}
		ctx = context.WithValue(ctx, pendingKey, pending)
		if err := fn(ctx); err != nil {
			return err
		}
		// Indexed rather than ranged: nothing registers during the flush today,
		// and if something ever does it must be flushed too rather than dropped.
		for i := 0; i < len(pending.items); i++ {
			it := pending.items[i]
			if err := triggerTargetConvergence(ctx, it.actor, it.userID, it.changed); err != nil {
				return err
			}
		}
		return nil
	})
}

// deferTargetConvergence records that a subject's roles changed, to be resolved
// once the caller's writes are in the transaction.
func deferTargetConvergence(ctx context.Context, actor, userID string, changed []roleKey) error {
	if len(changed) == 0 {
		return nil
	}
	pending, ok := ctx.Value(pendingKey).(*pendingConvergences)
	if !ok || pending == nil {
		return errConvergenceOutsideLock
	}
	pending.add(actor, userID, changed)
	return nil
}

// triggerTargetConvergence queues a convergence on every target the changed
// roles reach.
//
// Errors are returned rather than logged-and-swallowed: this runs inside the
// caller's transaction, and a convergence that failed to queue beside a role
// change that committed is precisely the silent divergence the outbox exists to
// prevent. The caller rolls both back.
func triggerTargetConvergence(ctx context.Context, actor, userID string, changed []roleKey) error {
	if len(changed) == 0 {
		return nil
	}

	targets, err := targetsReachedBy(ctx, changed)
	if err != nil {
		return err
	}
	for _, target := range targets {
		// Resolved fresh, inside the lock, after the source write — which is
		// what the deferral above buys, and the whole reason the flush is not
		// at the chokepoint. The delta
		// being enqueued beside this is the role change itself; what the target
		// needs is the state that follows from it, which is the whole resolved
		// set and not the delta. A set computed from the delta alone would carry
		// only what moved and the apply is level-triggered, so it would remove
		// everything that did not.
		set, err := svcResolveEntitlements(ctx, userID, target)
		if err != nil {
			return fmt.Errorf("resolve %s on %s: %w", userID, target, err)
		}
		if _, _, err := dbRecordSystemConvergence(ctx, db.SystemConvergence{
			Target: target, SubjectID: userID, Actor: actor,
			Reason:  "Role change reached a mapped target",
			Desired: set.Desired(),
		}); err != nil {
			return fmt.Errorf("queue convergence for %s on %s: %w", userID, target, err)
		}
	}
	return nil
}

// targetsReachedBy is the union of the targets each changed role is mapped to.
//
// A role mapped to nothing reaches nothing, which is the case that must stay
// free: most role changes in this deployment touch no target at all, and the
// trigger has to cost one indexed read rather than a convergence per person.
func targetsReachedBy(ctx context.Context, changed []roleKey) ([]string, error) {
	seen := map[string]struct{}{}
	for _, k := range changed {
		targets, err := dbTargetsMappedToRole(ctx, k.projectID, k.roleKey)
		if err != nil {
			return nil, fmt.Errorf("read targets mapped to %s/%s: %w", k.projectID, k.roleKey, err)
		}
		for _, t := range targets {
			seen[t] = struct{}{}
		}
	}
	if len(seen) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(seen))
	for t := range seen {
		out = append(out, t)
	}
	// Sorted so a cascade touching two targets queues them in one order rather
	// than in map order, which makes a failure reproducible.
	sort.Strings(out)
	return out, nil
}
