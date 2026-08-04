package drift

import (
	"context"
	"fmt"
	"log"

	"syndra/internal/services"
	"syndra/internal/zitadel"
)

// DriftResult summarizes one sweep for logs + the [Reconcile now] response.
type DriftResult struct {
	ZitadelGrants     int    `json:"zitadel_grants"`
	DriftItemsCreated int    `json:"drift_items_created"` // zitadel_only, deduped
	ReEnqueued        int    `json:"re_enqueued"`         // syndra_only replays
	Truncated         bool   `json:"truncated"`
	Halted            bool   `json:"halted"`
	Reason            string `json:"reason,omitempty"`
}

// Sweep reconciles Zitadel grants against Syndra's expected set. Callable by the
// scheduler and by the operator's [Reconcile now]. Two outcomes per role:
//   - zitadel_only  (in Zitadel, unexplained by direct/rule/exclusion) → drift_items
//   - syndra_only   (direct grant Syndra expects, absent from Zitadel)  → outbox re-enqueue
//
// Bundle/rule-derived expected roles that are ABSENT from Zitadel are NOT drift
// in sub-phase 2 — cascade projection is sub-phase 3, so they are legitimately
// unprojected. Only source-mediated direct grants can be syndra_only here.
func Sweep(ctx context.Context) (DriftResult, error) {
	if !zitadelReachable(ctx) {
		return DriftResult{Halted: true, Reason: "zitadel_offline"}, nil
	}

	direct, err := svcAllDirectGrants(ctx)
	if err != nil {
		return DriftResult{}, err
	}
	zit, truncated, err := fetchAllZitadelGrants(ctx)
	if err != nil {
		return DriftResult{}, err
	}
	// A lookup failure MUST NOT degrade to an empty set — that would flag
	// rule-derived / excluded grants as false drift. Abort the sweep instead;
	// the scheduler retries next tick and no noisy drift rows are written.
	rules, err := svcGetActiveMappingRules(ctx)
	if err != nil {
		return DriftResult{}, fmt.Errorf("drift sweep: load rules: %w", err)
	}
	exclusions, err := svcGetExclusions(ctx)
	if err != nil {
		return DriftResult{}, fmt.Errorf("drift sweep: load exclusions: %w", err)
	}
	holder := buildHolderSet(direct, zit)

	res := DriftResult{ZitadelGrants: len(zit), Truncated: truncated}

	// --- zitadel_only: unexplained live grants → drift_items ---
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
			if isExcluded(exclusions, g.UserID, g.ProjectID, rk) {
				continue // marked external — silently filtered
			}
			if _, inserted, err := upsertDriftItem(ctx, g.UserID, g.ProjectID,
				[]string{rk}, g.ID, "reconciliation_sweep", "zitadel_only"); err != nil {
				log.Printf("[DRIFT] upsert zitadel_only failed user=%s project=%s role=%s: %v", g.UserID, g.ProjectID, rk, err)
			} else if inserted {
				res.DriftItemsCreated++
			}
		}
	}

	// --- syndra_only: direct grants Syndra expects but Zitadel lacks → re-enqueue ---
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
		if queued, err := pendingOutboxAddExists(ctx, dg.UserID, dg.ProjectID, dg.RoleKey); err != nil {
			log.Printf("[DRIFT] pending outbox add lookup failed user=%s project=%s role=%s: %v", dg.UserID, dg.ProjectID, dg.RoleKey, err)
		} else if queued {
			continue
		}
		key, kerr := newIdempotencyKey()
		if kerr != nil {
			log.Printf("[DRIFT] mint idempotency key failed user=%s: %v (skipping re-enqueue)", dg.UserID, kerr)
			continue
		}
		if _, err := insertPending(ctx, "add", dg.UserID, dg.ProjectID, []string{dg.RoleKey},
			"", "{}", key, "system:drift-sweep"); err != nil {
			log.Printf("[DRIFT] re-enqueue syndra_only failed user=%s project=%s role=%s: %v", dg.UserID, dg.ProjectID, dg.RoleKey, err)
		} else {
			res.ReEnqueued++
		}
	}

	log.Printf("[DRIFT] Sweep complete: zitadel_grants=%d drift_created=%d re_enqueued=%d truncated=%v",
		res.ZitadelGrants, res.DriftItemsCreated, res.ReEnqueued, res.Truncated)
	return res, nil
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
