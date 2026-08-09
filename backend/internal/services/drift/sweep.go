package drift

import (
	"context"
	"errors"
	"fmt"
	"log"

	"syndra/internal/db"
	"syndra/internal/services"
	"syndra/internal/zitadel"
)

// DriftResult summarizes one sweep for logs + the [Reconcile now] response.
type DriftResult struct {
	// Target names what was reconciled. A result that does not say what it
	// swept reads as a statement about everything, and "no drift" about
	// everything is the one conclusion this sweep is not entitled to draw:
	// it looked at one target.
	Target string `json:"target"`

	ZitadelGrants     int    `json:"zitadel_grants"`
	DriftItemsCreated int    `json:"drift_items_created"` // target_only, deduped
	ReEnqueued        int    `json:"re_enqueued"`         // syndra_only replays
	Truncated         bool   `json:"truncated"`
	Halted            bool   `json:"halted"`
	Reason            string `json:"reason,omitempty"`

	// Reconciliation is how current Syndra's picture of the target now is —
	// when it last saw the target for itself, and since when it has not. A
	// sweep that reports zero findings is otherwise indistinguishable from a
	// sweep that could not look, and the second one is not good news.
	//
	// Nil only when the record could not be written: the sweep's own work
	// still happened, so a failure to record its currency is reported rather
	// than allowed to discard it.
	Reconciliation *db.TargetReconciliation `json:"reconciliation,omitempty"`
}

// Sweep reconciles Zitadel grants against Syndra's expected set. Callable by the
// scheduler and by the operator's [Reconcile now]. Two outcomes per role:
//   - target_only (in Zitadel, unexplained by direct/rule/exclusion) → drift_items
//   - syndra_only (direct grant Syndra expects, absent from Zitadel)  → outbox re-enqueue
//
// Bundle/rule-derived expected roles that are ABSENT from Zitadel are NOT drift
// in sub-phase 2 — cascade projection is sub-phase 3, so they are legitimately
// unprojected. Only source-mediated direct grants can be syndra_only here.
//
// The target is a constant, not a parameter, and deliberately so: this function
// pages the Zitadel Management API and compares role keys against Zitadel
// projects. Accepting a target it cannot actually reach would be a signature
// that promises something the body does not do. Add-on targets get their own
// sweep, over their own reads, in group 4 — what they share is this one, which
// every write below now names, so two sweeps can run against the same person
// without either one's findings landing under the other's name.
func Sweep(ctx context.Context) (DriftResult, error) {
	const target = db.TargetZitadel

	// A target that cannot answer produces no findings — and says so. The
	// alternative is not "no drift", it is silence that reads as no drift.
	//
	// This pre-flight tests whether Zitadel is CONFIGURED, not whether it
	// answers: it is a nil check on the client. Passing it is not evidence of
	// anything, so the read below has to be treated as the real test.
	if !zitadelReachable(ctx) {
		return DriftResult{
			Target:         target,
			Halted:         true,
			Reason:         "zitadel_offline",
			Reconciliation: recordUnreconciled(ctx, target, db.UnreconciledUnreachable),
		}, nil
	}

	direct, err := svcAllDirectGrants(ctx)
	if err != nil {
		return DriftResult{}, err
	}
	// The read failing IS the outage, on any page of it. Returning the error
	// here would have made a live network failure the one kind of outage that
	// goes unrecorded — and the row left behind would keep reporting the last
	// current read for the duration, so the surface built to say "Syndra has
	// not seen this target since Tuesday" would say "seen Tuesday" instead. It
	// is the same halt as the branch above, reached by the honest test rather
	// than the cheap one.
	//
	// A partly-read list is discarded rather than diffed: what did not arrive
	// is unseen, not absent (see the truncation branch below for the same
	// distinction reached a different way).
	zit, truncated, err := fetchAllZitadelGrants(ctx)
	if err != nil {
		reason, currency := classifyReadFailure(err)
		log.Printf("[DRIFT] target read failed for %s (%s): %v (recorded unreconciled, nothing diffed)", target, reason, err)
		return DriftResult{
			Target:         target,
			Halted:         true,
			Reason:         reason,
			Reconciliation: recordUnreconciled(ctx, target, currency),
		}, nil
	}
	// The reads below are Syndra's own. Their failure is not a statement about
	// the target and must not be recorded as one — an operator sent to check
	// Zitadel because a Syndra query is broken is an operator looking in the
	// wrong system. They abort, and the last current read stays visibly old.
	// A lookup failure MUST NOT degrade to an empty set — that would flag
	// rule-derived / excluded grants as false drift. Abort the sweep instead;
	// the scheduler retries next tick and no noisy drift rows are written.
	rules, err := svcGetActiveMappingRules(ctx)
	if err != nil {
		return DriftResult{}, fmt.Errorf("drift sweep: load rules: %w", err)
	}
	exclusions, err := svcGetExclusions(ctx, target)
	if err != nil {
		return DriftResult{}, fmt.Errorf("drift sweep: load exclusions: %w", err)
	}
	holder := buildHolderSet(direct, zit)

	res := DriftResult{Target: target, ZitadelGrants: len(zit), Truncated: truncated}

	// --- target_only: unexplained live grants → drift_items ---
	directSet := buildHolderSet(direct, nil) // Syndra's own direct intent
	for _, g := range zit {
		for _, rk := range g.RoleKeys {
			k := services.HolderKey{UserID: g.UserID, ProjectID: g.ProjectID, RoleKey: rk}
			if directSet[k] {
				continue // Syndra has a direct intent for this — not drift
			}
			if expectedViaRule(holder, rules, g.UserID, g.ProjectID, rk) {
				continue // expected_via_rule — not drift
			}
			if isExcluded(exclusions, target, g.UserID, g.ProjectID, rk) {
				continue // marked external on THIS target — silently filtered
			}
			if _, inserted, err := upsertDriftItem(ctx, target, g.UserID, g.ProjectID,
				[]string{rk}, g.ID, "reconciliation_sweep", "target_only"); err != nil {
				log.Printf("[DRIFT] upsert target_only failed user=%s project=%s role=%s: %v", g.UserID, g.ProjectID, rk, err)
			} else if inserted {
				res.DriftItemsCreated++
			}
		}
	}

	// --- syndra_only: direct grants Syndra expects but Zitadel lacks → re-enqueue ---
	//
	// This half concludes from an ABSENCE, and a capped read cannot observe
	// one: everything past the cap is unseen, not missing. Run over a truncated
	// list it would re-enqueue an `add` for every direct grant beyond the cap —
	// grants that already exist — filling Pending changes with work the drain
	// then discovers is redundant, which is exactly the manufactured finding
	// this sweep is supposed to be incapable of. The half above is unaffected:
	// it concludes from grants actually seen, and those were seen.
	if truncated {
		res.Reconciliation = recordUnreconciled(ctx, target, db.UnreconciledTruncated)
		log.Printf("[DRIFT] Sweep complete: target=%s zitadel_grants=%d drift_created=%d re_enqueued=0 truncated=true (absence not concluded)",
			res.Target, res.ZitadelGrants, res.DriftItemsCreated)
		return res, nil
	}

	zitSet := buildHolderSet(nil, zit)
	for _, dg := range direct {
		k := services.HolderKey{UserID: dg.UserID, ProjectID: dg.ProjectID, RoleKey: dg.RoleKey}
		if zitSet[k] {
			continue // present in Zitadel — no drift
		}
		// Skip if an undrained add is already queued for this triple — otherwise a
		// persistently-missing grant would pile a fresh duplicate outbox row every
		// sweep tick. On a lookup error, log and proceed to re-enqueue rather than
		// silently swallowing a real drift signal.
		if queued, err := pendingOutboxAddExists(ctx, target, dg.UserID, dg.ProjectID, dg.RoleKey); err != nil {
			log.Printf("[DRIFT] pending outbox add lookup failed user=%s project=%s role=%s: %v", dg.UserID, dg.ProjectID, dg.RoleKey, err)
		} else if queued {
			continue
		}
		key, kerr := newIdempotencyKey()
		if kerr != nil {
			log.Printf("[DRIFT] mint idempotency key failed user=%s: %v (skipping re-enqueue)", dg.UserID, kerr)
			continue
		}
		switch _, err := insertPending(ctx, "add", dg.UserID, dg.ProjectID, []string{dg.RoleKey},
			"", "{}", key, "system:drift-sweep"); {
		case errors.Is(err, db.ErrSupersededByRevocation):
			// The role is absent because somebody asked for it to be. The
			// ledger still carries it only because a revocation keeps its row
			// until the target confirms, which is exactly what this comparison
			// cannot tell apart from a grant the target lost.
			log.Printf("[DRIFT] not replaying user=%s project=%s role=%s: a revocation for it is still in flight",
				dg.UserID, dg.ProjectID, dg.RoleKey)
		case err != nil:
			log.Printf("[DRIFT] re-enqueue syndra_only failed user=%s project=%s role=%s: %v", dg.UserID, dg.ProjectID, dg.RoleKey, err)
		default:
			res.ReEnqueued++
		}
	}

	// Only here: a complete read, consumed in full, both halves concluded. This
	// is the one moment that entitles Syndra to say it has seen the target for
	// itself, and it ends any unreconciled period in the same statement.
	res.Reconciliation = recordReconciled(ctx, target)

	log.Printf("[DRIFT] Sweep complete: target=%s zitadel_grants=%d drift_created=%d re_enqueued=%d truncated=%v",
		res.Target, res.ZitadelGrants, res.DriftItemsCreated, res.ReEnqueued, res.Truncated)
	return res, nil
}

// classifyReadFailure separates a target that did not answer from one that
// answered and declined to serve the read, returning the result's reason code
// and the durable currency reason.
//
// The split is answered / not answered, and deliberately no finer. A typed
// status error means bytes came back from Zitadel: the network is fine, the
// host is up, and the thing to fix is a credential, a permission, or Zitadel
// itself. Reported as unreachable, an expired service-account key looks like
// weather — something to wait out rather than repair, and the sweep would go on
// failing every tick while the record said the target was down.
//
// Splitting further, 401 from 500, would be guessing at Zitadel's status
// semantics to pick an operator's next move. The code is in the log; the
// durable reason says only what the sweep actually established.
func classifyReadFailure(err error) (reason, currency string) {
	var status *zitadel.StatusError
	if errors.As(err, &status) {
		return "zitadel_read_refused", db.UnreconciledReadRefused
	}
	// Distinct from `zitadel_offline`: not wired up and not answering send an
	// operator to different places again.
	return "zitadel_unreachable", db.UnreconciledUnreachable
}

// recordUnreconciled and recordReconciled write the target's currency and hand
// it back for the result. A failure to record is logged and reported as an
// absent Reconciliation rather than as a failed sweep: the findings the pass
// did produce are real, and the database being unreachable is not a reason to
// throw them away. The absent field is honest — Syndra cannot say how current
// its picture is if it cannot read the record that holds it.
func recordUnreconciled(ctx context.Context, target, reason string) *db.TargetReconciliation {
	rec, err := markUnreconciled(ctx, target, reason)
	if err != nil {
		log.Printf("[DRIFT] could not record %s as unreconciled (%s): %v (non-fatal)", target, reason, err)
		return nil
	}
	return &rec
}

func recordReconciled(ctx context.Context, target string) *db.TargetReconciliation {
	rec, err := markReconciled(ctx, target)
	if err != nil {
		log.Printf("[DRIFT] could not record %s as reconciled: %v (non-fatal)", target, err)
		return nil
	}
	return &rec
}

// fetchAllZitadelGrants pages ListAllGrants, capped at driftSafetyCap (B2).
// Mirrors handlers/reconciliation.go:fetchAllZitadelGrants; kept here so the
// drift package is self-contained (avoids a handlers→drift→handlers cycle).
func fetchAllZitadelGrants(ctx context.Context) ([]zitadel.UserGrant, bool, error) {
	var all []zitadel.UserGrant
	offset := 0
	for {
		page, err := zitadelListAllGrants(ctx, zitadel.SearchParams{Limit: zitadelPageSize, Offset: offset})
		if err != nil {
			return nil, false, err
		}
		all = append(all, page.Items...)
		if len(all) >= page.Total || len(page.Items) == 0 {
			return all, false, nil
		}
		if len(all) >= driftSafetyCap {
			return all, true, nil
		}
		offset += len(page.Items)
	}
}
