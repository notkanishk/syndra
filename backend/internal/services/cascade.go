package services

import (
	"context"
	"log"
	"sort"

	"mkauth/internal/db"
	"mkauth/internal/models"
	"mkauth/internal/services/propagation"
)

// CascadeResult reports what a cascade enqueued and (auto only) drained.
type CascadeResult struct {
	Enqueued int                     `json:"enqueued"`
	Mode     string                  `json:"mode"`
	Drain    propagation.DrainResult `json:"drain"`

	// NoOp means the mutation did not happen because it was already true —
	// today, only "they already hold this bundle". Distinct from
	// `Enqueued == 0`, which means the write DID happen and changed nobody's
	// effective access. Reporting the two the same way is how an idempotent
	// call gets read as a successful change.
	NoOp bool `json:"no_op,omitempty"`
}

// --- injectable deps (swapped in cascade_test.go) ---
var (
	svcGetBundleByID = func(ctx context.Context, id string) (models.Bundle, error) {
		return db.GetBundleByID(ctx, id)
	}
	svcCascGetRolesForBundle = db.GetRolesForBundle
	// Version-aware: what each of a user's bundles grants THEM, resolved
	// through the version they are pinned to rather than the bundle's current
	// working copy. Every closure computation goes through this — asking what
	// the bundle holds is what made an edit reach everybody.
	svcGetUserBundleRolesGrouped = db.GetUserBundleRolesGrouped
	svcGetRolesForVersion        = db.GetRolesForVersion
	svcLatestVersion             = db.LatestVersion
	svcGetBundleHoldersByVersion = db.GetBundleHoldersByVersion
	svcListBundleVersions        = db.ListBundleVersions
	svcPublishVersionAndEnqueue  = db.PublishVersionAndEnqueue
	svcMoveHoldersAndEnqueue     = db.MoveHoldersAndEnqueue
	svcGetUsersForBundle         = db.GetUsersForBundle
	svcGetAllKnownUserIDs        = db.GetAllKnownUserIDs
	svcGetMappingRuleByID        = db.GetMappingRuleByID
	svcDrainBatch                = propagation.DrainBatch

	// atomic mutation+enqueue (one tx each — see design pivot: cascades write no
	// direct_role_grants row).
	svcAssignBundleAndEnqueue    = db.AssignBundleAndEnqueue
	svcAddRoleToBundleAndEnqueue = db.AddRoleToBundleAndEnqueue
	svcCreateRuleAndEnqueue      = db.CreateMappingRuleAndEnqueue

	// Revoke-side atomic mutation+enqueue (Task 21).
	svcRemoveBundleFromUserAndEnqueue = db.RemoveBundleFromUserAndEnqueue
	svcRemoveRoleFromBundleAndEnqueue = db.RemoveRoleFromBundleAndEnqueue
	svcUpdateRuleAndEnqueue           = db.UpdateMappingRuleAndEnqueue

	// The closure-diff helpers below (userBaseHoldings et al.) reuse the governance-layer
	// injectables already declared in deps.go (svcGetDirectGrantsForUser, svcGetBundlesForUser,
	// svcGetActiveMappingRules) — redeclaring them here would be a duplicate package-level var.
)

// applyMode drains the JUST-enqueued rows when the source is auto; manual leaves them pending.
// The rows were already persisted atomically with the source mutation by the caller, so a drain
// failure is NON-FATAL: it is captured in res.Drain (Halted/Reason) and applyMode returns nil,
// so the handler still responds 200 — the rows sit pending in the worklist, recoverable via
// "Resume now". (Returning derr here would 500 AFTER the mutation committed, inviting a retry
// that re-runs the whole cascade and mints duplicate outbox rows.)
func applyMode(ctx context.Context, mode string, ids []string) (CascadeResult, error) {
	mode = db.NormalizeConfirmationMode(mode)
	res := CascadeResult{Mode: mode, Enqueued: len(ids)}
	if mode == "auto" && len(ids) > 0 {
		dr, derr := svcDrainBatch(ctx, ids)
		res.Drain = dr
		if derr != nil {
			// non-fatal: log and surface via res.Drain; rows remain pending
			log.Printf("[CASCADE] auto-drain of %d row(s) failed (left pending): %v", len(ids), derr)
			res.Drain.Halted = true
			if res.Drain.Reason == "" {
				res.Drain.Reason = "drain_error"
			}
		}
	}
	return res, nil
}

// --- closure-diff helpers (P1a/P1b fix) ---
//
// Every cascade below projects a user's EFFECTIVE-ROLE CLOSURE delta (adds = after−before,
// revokes = before−after) rather than a bundle's literal roles or a single grant-index lookup.
// This inherently discovers rule-derived targets (P1a) from MkAuth's own tables regardless of
// whether the grant has reached Zitadel yet (P1b), and subsumes the old OtherSourceCovers check:
// a role still covered by another source stays in `after` and is never revoked; a role dropped by
// every source falls out of `after` and is revoked.
//
// Atomicity: before/after are computed from PRE-mutation committed reads plus a pure in-memory
// simulation of the change — never from a post-mutation read — so the delta can be handed to the
// existing atomic *AndEnqueue db functions (mutation + audit + enqueue in one tx).

// effectiveClosure applies the mapping-rule fixpoint over a base set of held (project,role) keys.
// This is a pure, set-only mirror of collectUserRoles' fixpoint loop (views.go, ~lines 791-812):
// same multi-hop, run-until-no-change semantics, minus the EffectiveRole/reason bookkeeping that
// makes the two awkward to share directly. Keep this in sync if that loop's semantics change.
func effectiveClosure(base map[roleKey]bool, rules []models.MappingRule) map[roleKey]bool {
	closure := make(map[roleKey]bool, len(base))
	for k := range base {
		closure[k] = true
	}
	for i := 0; i < len(rules); i++ {
		changed := false
		for _, rule := range rules {
			src := roleKey{projectID: rule.SourceProject, roleKey: rule.SourceRole}
			if !closure[src] {
				continue
			}
			tgt := roleKey{projectID: rule.TargetProject, roleKey: rule.TargetRole}
			if !closure[tgt] {
				closure[tgt] = true
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	return closure
}

// closureDelta returns adds (after−before) and revokes (before−after), sorted for a deterministic
// enqueue order.
func closureDelta(before, after map[roleKey]bool) (adds, revokes []roleKey) {
	for k := range after {
		if !before[k] {
			adds = append(adds, k)
		}
	}
	for k := range before {
		if !after[k] {
			revokes = append(revokes, k)
		}
	}
	sortRoleKeys(adds)
	sortRoleKeys(revokes)
	return adds, revokes
}

func sortRoleKeys(ks []roleKey) {
	sort.Slice(ks, func(i, j int) bool {
		if ks[i].projectID != ks[j].projectID {
			return ks[i].projectID < ks[j].projectID
		}
		return ks[i].roleKey < ks[j].roleKey
	})
}

// userBaseHoldings returns a user's PRE-rule base = direct grants ∪ bundle literal roles.
func userBaseHoldings(ctx context.Context, userID string) (map[roleKey]bool, error) {
	return userBaseHoldingsExcludingBundle(ctx, userID, "")
}

// userBaseHoldingsExcludingBundle is userBaseHoldings but omits one bundle's roles entirely — used
// to simulate "after bundle removal" from a PRE-mutation read (the bundle is still assigned in
// svcGetBundlesForUser at read time; the exclusion is the simulation).
func userBaseHoldingsExcludingBundle(ctx context.Context, userID, excludeBundleID string) (map[roleKey]bool, error) {
	base := make(map[roleKey]bool)
	directs, err := svcGetDirectGrantsForUser(ctx, userID, false)
	if err != nil {
		return nil, err
	}
	for _, g := range directs {
		base[roleKey{projectID: g.ProjectID, roleKey: g.RoleKey}] = true
	}
	byBundle, err := svcGetUserBundleRolesGrouped(ctx, userID)
	if err != nil {
		return nil, err
	}
	for bundleID, roles := range byBundle {
		if bundleID == excludeBundleID {
			continue
		}
		for _, ro := range roles {
			base[roleKey{projectID: ro.ProjectID, roleKey: ro.RoleKey}] = true
		}
	}
	return base, nil
}

// userBaseHoldingsWithBundleAt is userBaseHoldings with ONE bundle's
// contribution replaced by an explicit role set — "what would this person hold
// if their pin on bundle B were moved to a version containing exactly these".
//
// This is how both publishing and moving a holder are simulated. It replaces
// the old per-role add/remove simulations, which could only model one role
// changing at a time; a version is a whole set moving at once, and a person can
// gain and lose roles in the same step.
func userBaseHoldingsWithBundleAt(
	ctx context.Context,
	userID, bundleID string,
	roles []models.BundleRole,
) (map[roleKey]bool, error) {
	base, err := userBaseHoldingsExcludingBundle(ctx, userID, bundleID)
	if err != nil {
		return nil, err
	}
	for _, ro := range roles {
		base[roleKey{projectID: ro.ProjectID, roleKey: ro.RoleKey}] = true
	}
	return base, nil
}

// deltaParams converts a closure delta into enqueue params, all attributed to the ONE triggering
// source (bundle or rule) — every row from a single cascade trigger carries the same
// Source/SourceRef, whether it is an add or a revoke.
func deltaParams(userID string, adds, revokes []roleKey, actor, reason, source, sourceRef string) []db.EnqueueParams {
	params := make([]db.EnqueueParams, 0, len(adds)+len(revokes))
	for _, k := range adds {
		params = append(params, db.EnqueueParams{
			UserID: userID, ProjectID: k.projectID, RoleKeys: []string{k.roleKey},
			GrantedBy: actor, Reason: reason,
			Source: source, SourceRef: sourceRef, OpType: "add", PayloadJSON: "{}",
		})
	}
	for _, k := range revokes {
		params = append(params, db.EnqueueParams{
			UserID: userID, ProjectID: k.projectID, RoleKeys: []string{k.roleKey},
			GrantedBy: actor, Reason: reason,
			Source: source, SourceRef: sourceRef, OpType: "revoke", PayloadJSON: "{}",
		})
	}
	return params
}

// CascadeBundleAssignedToUser assigns the bundle AND enqueues the user's closure delta (bundle
// roles plus anything they derive via active mapping rules) in one tx (atomic — no committed
// assignment without its outbox rows), then (auto) drains those rows.
func CascadeBundleAssignedToUser(ctx context.Context, actor, userID, bundleID string) (CascadeResult, error) {
	bundle, err := svcGetBundleByID(ctx, bundleID)
	if err != nil {
		return CascadeResult{}, err
	}
	// The LATEST PUBLISHED version, not the working copy. AssignBundleAndEnqueue
	// pins the assignment to that version in the same transaction, so projecting
	// from `bundle_roles` would push unpublished edits to somebody who is not
	// pinned to them — a new member receiving a role nobody had published.
	version, roles, err := svcLatestVersionRoles(ctx, bundleID)
	if err != nil {
		return CascadeResult{}, err
	}
	rules, err := svcGetActiveMappingRules(ctx)
	if err != nil {
		return CascadeResult{}, err
	}
	before, err := userBaseHoldings(ctx, userID)
	if err != nil {
		return CascadeResult{}, err
	}
	after := make(map[roleKey]bool, len(before)+len(roles))
	for k := range before {
		after[k] = true
	}
	for _, ro := range roles {
		after[roleKey{projectID: ro.ProjectID, roleKey: ro.RoleKey}] = true
	}
	adds, revokes := closureDelta(effectiveClosure(before, rules), effectiveClosure(after, rules))
	params := deltaParams(userID, adds, revokes, actor, "Bundle membership cascade", "bundle", bundleID)

	ids, assigned, err := svcAssignBundleAndEnqueue(ctx, actor, userID, bundleID, version.ID, params)
	if err != nil {
		return CascadeResult{}, err // enqueue+assign rolled back together → handler returns 500
	}
	if !assigned {
		// They already hold it, on whichever version they were pinned to. The
		// delta computed above was against the LATEST version, so enqueuing it
		// would hand them newer access than their pin records — the exact
		// mismatch versioning exists to prevent. Moving them forward is a
		// separate, rehearsed action.
		return CascadeResult{Mode: bundle.ConfirmationMode, NoOp: true}, nil
	}
	return applyMode(ctx, bundle.ConfirmationMode, ids)
}

// EditBundleWorkingCopy adds or removes a role in the bundle's working copy.
//
// It cascades to nobody, and that is the change versioning makes. An edit used
// to reach every holder the moment it was saved, which is why editing a bundle
// fourteen people held was a decision nobody wanted to take in passing. Now the
// edit is free and the consequence is a separate, rehearsed step: PublishBundle.
func EditBundleWorkingCopy(ctx context.Context, actor, bundleID, projectID, roleKeyStr string, add bool) error {
	if add {
		_, err := svcAddRoleToBundleAndEnqueue(ctx, actor, bundleID, projectID, roleKeyStr, nil)
		return err
	}
	_, err := svcRemoveRoleFromBundleAndEnqueue(ctx, actor, bundleID, projectID, roleKeyStr, nil)
	return err
}

// CascadeRuleCreated creates the rule AND enqueues, for every known user, the closure delta caused
// by the new source→target edge (empty delta users are skipped), in one tx, then (auto) drains.
// Affected users come from GetAllKnownUserIDs — not a grant-index lookup — so a user who holds the
// source only via MkAuth's own tables (pending/manual/failed-drain) is still discovered (P1b). The
// enqueue params carry SourceRef="" here; CreateMappingRuleAndEnqueue stamps the new rule id onto
// them after the INSERT ... RETURNING, since the id doesn't exist yet at simulation time. Returns
// the new rule id for the handler response. The handler does cycle/self-ref validation first.
func CascadeRuleCreated(ctx context.Context, actor, sourceProject, sourceRole, targetProject, targetRole, mode string) (string, CascadeResult, error) {
	users, err := svcGetAllKnownUserIDs(ctx)
	if err != nil {
		return "", CascadeResult{}, err
	}
	rulesBefore, err := svcGetActiveMappingRules(ctx)
	if err != nil {
		return "", CascadeResult{}, err
	}
	rulesAfter := append(append([]models.MappingRule{}, rulesBefore...), models.MappingRule{
		SourceProject: sourceProject, SourceRole: sourceRole,
		TargetProject: targetProject, TargetRole: targetRole,
	})

	var params []db.EnqueueParams
	for _, u := range users {
		base, err := userBaseHoldings(ctx, u)
		if err != nil {
			return "", CascadeResult{}, err
		}
		adds, revokes := closureDelta(effectiveClosure(base, rulesBefore), effectiveClosure(base, rulesAfter))
		if len(adds) == 0 && len(revokes) == 0 {
			continue
		}
		params = append(params, deltaParams(u, adds, revokes, actor, "Mapping rule cascade", "rule", "")...)
	}

	ruleID, ids, err := svcCreateRuleAndEnqueue(ctx, actor,
		sourceProject, sourceRole, targetProject, targetRole,
		db.NormalizeConfirmationMode(mode), params)
	if err != nil {
		return "", CascadeResult{}, err
	}
	res, err := applyMode(ctx, mode, ids)
	return ruleID, res, err
}

// CascadeBundleRemovedFromUser computes the user's closure delta with the bundle simulated as
// already removed (userBaseHoldingsExcludingBundle), then atomically deletes the assignment +
// enqueues the delta, then (auto) drains. Because the delta is computed over the FULL effective
// closure, a role still reachable via another bundle/rule/direct grant simply never leaves
// `after` — no separate coverage check is needed (replaces the old OtherSourceCovers scan). Any
// concurrent change between the read and the delete is a reconciliation-tolerated race (design §7
// Q4).
func CascadeBundleRemovedFromUser(ctx context.Context, actor, userID, bundleID string) (CascadeResult, error) {
	bundle, err := svcGetBundleByID(ctx, bundleID)
	if err != nil {
		return CascadeResult{}, err
	}
	rules, err := svcGetActiveMappingRules(ctx)
	if err != nil {
		return CascadeResult{}, err
	}
	before, err := userBaseHoldings(ctx, userID)
	if err != nil {
		return CascadeResult{}, err
	}
	after, err := userBaseHoldingsExcludingBundle(ctx, userID, bundleID)
	if err != nil {
		return CascadeResult{}, err
	}
	adds, revokes := closureDelta(effectiveClosure(before, rules), effectiveClosure(after, rules))
	params := deltaParams(userID, adds, revokes, actor, "Bundle removal cascade", "bundle", bundleID)

	ids, err := svcRemoveBundleFromUserAndEnqueue(ctx, actor, userID, bundleID, params)
	if err != nil {
		return CascadeResult{}, err
	}
	return applyMode(ctx, bundle.ConfirmationMode, ids)
}

// CascadeRuleUpdated re-projects a rule whose matcher/target changed. `old` is the rule as it was
// BEFORE the update (captured by the handler, since the updated fields don't tell us the old
// target); sp/sr/tp/tr are the NEW fields. The update and the cascade commit atomically.
//
// For every known user, rulesBefore (containing `old` as currently persisted) and rulesAfter
// (old.ID's edge replaced by the new sp/sr/tp/tr) are each folded into that user's closure; the
// diff is enqueued attributed to old.ID. This closure-diff replaces the old add-pass/revoke-pass
// plus sameTriple/addSet bookkeeping: a user who ends up with the same effective roles either way
// (e.g. re-added identically, or still covered by another source) simply gets an empty delta.
func CascadeRuleUpdated(ctx context.Context, actor string, old models.MappingRule, sp, sr, tp, tr string) (CascadeResult, error) {
	users, err := svcGetAllKnownUserIDs(ctx)
	if err != nil {
		return CascadeResult{}, err
	}
	rulesBefore, err := svcGetActiveMappingRules(ctx)
	if err != nil {
		return CascadeResult{}, err
	}
	// ponytail: if old.ID isn't present in rulesBefore (shouldn't happen — the rule being updated
	// is always in the active-rules read), rulesAfter falls back to == rulesBefore and the delta
	// is a safe no-op rather than a guess.
	rulesAfter := make([]models.MappingRule, len(rulesBefore))
	copy(rulesAfter, rulesBefore)
	for i, ru := range rulesAfter {
		if ru.ID == old.ID {
			rulesAfter[i] = models.MappingRule{
				ID: old.ID, SourceProject: sp, SourceRole: sr,
				TargetProject: tp, TargetRole: tr,
			}
			break
		}
	}

	var params []db.EnqueueParams
	for _, u := range users {
		base, err := userBaseHoldings(ctx, u)
		if err != nil {
			return CascadeResult{}, err
		}
		adds, revokes := closureDelta(effectiveClosure(base, rulesBefore), effectiveClosure(base, rulesAfter))
		if len(adds) == 0 && len(revokes) == 0 {
			continue
		}
		params = append(params, deltaParams(u, adds, revokes, actor, "Mapping-rule update cascade", "rule", old.ID)...)
	}

	ids, err := svcUpdateRuleAndEnqueue(ctx, actor, old.ID, sp, sr, tp, tr, params)
	if err != nil {
		return CascadeResult{}, err
	}
	return applyMode(ctx, old.ConfirmationMode, ids)
}

// userBaseHoldingsExcludingGrant is userBaseHoldings but omits exactly one
// direct grant by id — the simulation of "after this direct access is removed",
// computed from a PRE-mutation read.
//
// It excludes by GRANT ID rather than by (project, role): another source may
// legitimately contribute the same pair, and blinding the base to the pair
// would make a role look lost when a bundle still carries it.
func userBaseHoldingsExcludingGrant(ctx context.Context, userID, excludeGrantID string) (map[roleKey]bool, error) {
	base := make(map[roleKey]bool)
	directs, err := svcGetDirectGrantsForUser(ctx, userID, false)
	if err != nil {
		return nil, err
	}
	for _, g := range directs {
		if g.ID == excludeGrantID {
			continue // this grant is the one being removed
		}
		base[roleKey{projectID: g.ProjectID, roleKey: g.RoleKey}] = true
	}
	byBundle, err := svcGetUserBundleRolesGrouped(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, roles := range byBundle {
		for _, ro := range roles {
			base[roleKey{projectID: ro.ProjectID, roleKey: ro.RoleKey}] = true
		}
	}
	return base, nil
}

// directGrantRoleKey resolves which (project, role) a grant id names. Absence is
// not an error here: the authoritative "does this grant exist" answer comes from
// the delete itself, which returns ErrGrantNotFound inside its transaction.
func directGrantRoleKey(ctx context.Context, userID, grantID string) (roleKey, bool) {
	grants, err := svcGetDirectGrantsForUser(ctx, userID, true)
	if err != nil {
		return roleKey{}, false
	}
	for _, g := range grants {
		if g.ID == grantID {
			return roleKey{projectID: g.ProjectID, roleKey: g.RoleKey}, true
		}
	}
	return roleKey{}, false
}
