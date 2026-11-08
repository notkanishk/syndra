package services

import (
	"context"
	"maps"
	"slices"
	"sort"
	"strings"
	"time"

	"syndra/internal/access"
	"syndra/internal/clock"
	"syndra/internal/db"
	"syndra/internal/models"
	"syndra/internal/services/propagation"
)

// CascadeResult reports what a cascade enqueued and (auto only) drained.
type CascadeResult struct {
	Enqueued int                     `json:"enqueued"`
	Drain    propagation.DrainResult `json:"drain"`

	// Waiting counts rows held for the operator (a role that waits), and
	// WaitReason says why the change isn't in Zitadel yet: role_waits or
	// halted:<reason>. Empty when it all went.
	Waiting    int    `json:"waiting"`
	WaitReason string `json:"wait_reason,omitempty"`

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
	svcGetWorkingIncludes        = db.GetWorkingIncludes
	svcGetVersionIncludes        = db.GetVersionIncludes
	svcSnapshotVersionIncludes   = db.SnapshotVersionIncludes
	svcSetWorkingInclude         = db.SetWorkingInclude
	svcListIncluders             = db.ListIncluders
	svcRetireBlockers            = db.RetireBlockers

	// atomic mutation+enqueue (one tx each — see design pivot: cascades write no
	// direct_role_grants row).
	svcAssignBundleAndEnqueue    = db.AssignBundleAndEnqueue
	svcAddRoleToBundleAndEnqueue = db.AddRoleToBundleAndEnqueue
	svcCreateRuleAndEnqueue      = db.CreateMappingRuleAndEnqueue

	// Revoke-side atomic mutation+enqueue (Task 21).
	svcRemoveBundleFromUserAndEnqueue = db.RemoveBundleFromUserAndEnqueue
	svcRetireBundleAndEnqueue         = db.RetireBundleAndEnqueue
	svcRemoveRoleFromBundleAndEnqueue = db.RemoveRoleFromBundleAndEnqueue
	svcUpdateRuleAndEnqueue           = db.UpdateMappingRuleAndEnqueue
	svcRetireRuleAndEnqueue           = db.RetireMappingRuleAndEnqueue

	// The closure-diff helpers below (userBaseHoldings et al.) reuse the governance-layer
	// injectables already declared in deps.go (svcGetDirectGrantsForUser, svcGetBundlesForUser,
	// svcGetActiveMappingRules) — redeclaring them here would be a duplicate package-level var.
)

// ErrBundleIncluded refuses retiring a bundle other bundles include, naming them.
type ErrBundleIncluded struct{ Includers []string }

func (e *ErrBundleIncluded) Error() string {
	return "refused while " + strings.Join(e.Includers, " and ") + " include it"
}

func (e *ErrBundleIncluded) Unwrap() error { return db.ErrBundleIncluded }

// ErrRetired refuses a write to a bundle that was retired: it keeps its history and nothing
// about it can change.
type ErrRetired struct{ At time.Time }

func (e *ErrRetired) Error() string { return "retired " + clock.Day(e.At) }

// refuseRetired is the one test every bundle write here makes after reading the bundle.
func refuseRetired(b models.Bundle) error {
	if b.RetiredAt != nil {
		return &ErrRetired{At: *b.RetiredAt}
	}
	return nil
}

// --- closure-diff helpers (P1a/P1b fix) ---
//
// Every cascade below projects a user's EFFECTIVE-ROLE CLOSURE delta (adds = after−before,
// revokes = before−after) rather than a bundle's literal roles or a single grant-index lookup.
// This inherently discovers rule-derived targets (P1a) from Syndra's own tables regardless of
// whether the grant has reached Zitadel yet (P1b), and subsumes the old OtherSourceCovers check:
// a role still covered by another source stays in `after` and is never revoked; a role dropped by
// every source falls out of `after` and is revoked.
//
// Atomicity: before/after are computed from PRE-mutation committed reads plus a pure in-memory
// simulation of the change — never from a post-mutation read — so the delta can be handed to the
// existing atomic *AndEnqueue db functions (mutation + audit + enqueue in one tx).

// effectiveClosure applies the mapping-rule fixpoint over a base set of held (project,role) keys,
// through access.Closure — the one implementation the compiler, the views, drift and the mapping
// cohort share, so a cascade sends Zitadel exactly the chain the token carries.
func effectiveClosure(base map[roleKey]bool, rules []models.MappingRule) map[roleKey]bool {
	return access.Closure(base, rules, newRoleKey)
}

func newRoleKey(projectID, role string) roleKey { return roleKey{projectID: projectID, roleKey: role} }

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
	directs, err := svcGetDirectGrantsForUser(ctx, userID, false)
	if err != nil {
		return nil, err
	}
	byBundle, err := svcGetUserBundleRolesGrouped(ctx, userID)
	if err != nil {
		return nil, err
	}
	revoking, err := svcQueuedRevocations(ctx, userID)
	if err != nil {
		return nil, err
	}
	return baseFrom(db.PersonAccess{Direct: directs, ByBundle: byBundle, Revoking: revoking}, excludeBundleID), nil
}

// baseFrom is a person's base holdings from what was read: their direct
// grants and what their bundles give them (leaving out one bundle, to
// simulate its removal), less what a queued revocation is taking away. The
// one definition the cascades here and the planner share.
func baseFrom(p db.PersonAccess, excludeBundleID string) map[roleKey]bool {
	base := make(map[roleKey]bool)
	for _, g := range p.Direct {
		base[roleKey{projectID: g.ProjectID, roleKey: g.RoleKey}] = true
	}
	for bundleID, roles := range p.ByBundle {
		if bundleID == excludeBundleID {
			continue
		}
		for _, ro := range roles {
			base[roleKey{projectID: ro.ProjectID, roleKey: ro.RoleKey}] = true
		}
	}
	for _, r := range p.Revoking {
		delete(base, roleKey{projectID: r.ProjectID, roleKey: r.RoleKey})
	}
	return base
}

// userBaseHoldingsWithBundleAt is userBaseHoldings with ONE bundle's
// contribution replaced by an explicit role set: "what would this person hold
// if their pin on bundle B were a version containing exactly these" (the
// Insights what-if). A role a queued revocation is taking away stays away.
func userBaseHoldingsWithBundleAt(ctx context.Context, userID, bundleID string, roles []models.BundleRole) (map[roleKey]bool, error) {
	base, err := userBaseHoldingsExcludingBundle(ctx, userID, bundleID)
	if err != nil {
		return nil, err
	}
	for _, ro := range roles {
		base[roleKey{projectID: ro.ProjectID, roleKey: ro.RoleKey}] = true
	}
	queued, err := svcQueuedRevocations(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, r := range queued {
		delete(base, roleKey{projectID: r.ProjectID, roleKey: r.RoleKey})
	}
	return base, nil
}

// CascadeBundleAssignedToUser assigns the welcome bundle on a first sign-in
// (onboarding), through the planner (task 1.9): the person's closure delta
// (the bundle's roles at its latest published version, and what the rules
// follow with) planned on the snapshot under the lock, written with the
// assignment in one tx, then delivered as the plan decided (DeliverChange).
// It cancels only the queued changes its plan named (systemGate).
//
// No catalogue guard, as before: a welcome assignment is never refused for a
// role Zitadel no longer offers; that row is refused by Zitadel when it is
// sent, and says so in Pending changes.
func CascadeBundleAssignedToUser(ctx context.Context, actor, userID, bundleID string) (CascadeResult, error) {
	ctx = systemGate(ctx)
	var changes []Change
	assigned := false

	// Everything from the first read to the commit runs under the
	// access-mutation lock: a delta read outside it can describe a world an
	// expiry changed in between. The send is outside: the lock is never held
	// across a call to Zitadel.
	err := withLockedAccess(ctx, func(ctx context.Context) error {
		bundle, err := svcGetBundleByID(ctx, bundleID)
		if err != nil {
			return err
		}
		if err := refuseRetired(bundle); err != nil {
			return err
		}
		// The LATEST PUBLISHED version, not the working copy: the assignment
		// pins it in the same transaction.
		version, roles, err := svcLatestVersionRoles(ctx, bundleID)
		if err != nil {
			return err
		}
		s, err := systemSnapshot(ctx, []string{userID})
		if err != nil {
			return err
		}
		person := s.Access.People[userID]
		if person.Bundles[bundleID] {
			// Already a holder, on whichever version: re-assigning is a no-op,
			// never a repin (moving holders is its own reviewed act).
			return nil
		}
		before := baseFrom(person, "")
		after := maps.Clone(before)
		for _, ro := range roles {
			after[roleKey{projectID: ro.ProjectID, roleKey: ro.RoleKey}] = true
		}
		changes = []Change{systemChange(userID, person, s, before, after, s.Access.Rules, s.Access.Rules, "bundle", bundleID)}
		db.CancellationsNamed(ctx, plannedCancels(changes))
		params, err := systemParams(ctx, actor, "Bundle membership cascade", changes)
		if err != nil {
			return err
		}
		ids, ok, err := svcAssignBundleAndEnqueue(ctx, actor, "", userID, bundleID, version.ID, params)
		if err != nil || !ok {
			return err // !ok: they joined in the same instant, and nothing was written
		}
		assigned = true
		changes, err = pairWritten(changes, ids)
		return err
	})
	if err != nil {
		return CascadeResult{}, err
	}
	if !assigned {
		return CascadeResult{NoOp: true}, nil
	}
	return deliverSystem(ctx, changes), nil
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

// everyone plans every known person's closure delta between two rule sets on
// one snapshot: a rule's own edit (create, update, retire). Each row is the
// rule that reaches its role, the step before it in a chain (task 1.6a): the
// edited rule for its own target, the next rule down for the role after that
// ("" for a rule not yet created, which the insert stamps). People whose
// access doesn't move are left out.
func everyone(ctx context.Context, rulesAfter func([]models.MappingRule) []models.MappingRule) ([]Change, error) {
	users, err := svcGetAllKnownUserIDs(ctx)
	if err != nil {
		return nil, err
	}
	s, err := systemSnapshot(ctx, users)
	if err != nil {
		return nil, err
	}
	before := s.Access.Rules
	after := rulesAfter(slices.Clone(before))
	var changes []Change
	for _, u := range users {
		person := s.Access.People[u]
		base := baseFrom(person, "")
		if ch := systemChange(u, person, s, base, base, before, after, "rule", ""); len(ch.Writes) > 0 {
			changes = append(changes, ch)
		}
	}
	return changes, nil
}

// CascadeRuleCreated creates the rule AND enqueues, for every known user, the
// closure delta the new source→target edge causes, planned on one snapshot
// under the lock (task 1.9), in one tx, then delivers it as planned. Affected
// users come from GetAllKnownUserIDs, not the grant index, so a user who holds
// the source only in Syndra's own tables is still reached (P1b). The new
// rule's rows carry SourceRef="" here; CreateMappingRuleAndEnqueue stamps the
// new rule id onto them after the insert (a row further down a chain is the
// next rule's already). The handler validates cycles first. It cancels
// only what the operator's gate admits.
func CascadeRuleCreated(ctx context.Context, actor, sourceProject, sourceRole, targetProject, targetRole string) (string, CascadeResult, error) {
	var ruleID string
	var changes []Change
	// A rule reaches every holder of the source role, and which holders those
	// are is what these reads are for, which is why the lock is one lock.
	err := withLockedAccess(ctx, func(ctx context.Context) error {
		var err error
		changes, err = everyone(ctx, func(rules []models.MappingRule) []models.MappingRule {
			return append(rules, models.MappingRule{SourceProject: sourceProject, SourceRole: sourceRole,
				TargetProject: targetProject, TargetRole: targetRole})
		})
		if err != nil {
			return err
		}
		params, err := systemParams(ctx, actor, "Mapping rule cascade", changes)
		if err != nil {
			return err
		}
		var ids []string
		if ruleID, ids, err = svcCreateRuleAndEnqueue(ctx, actor, sourceProject, sourceRole, targetProject, targetRole, params); err != nil {
			return err
		}
		changes, err = pairWritten(changes, ids)
		return err
	})
	if err != nil {
		return "", CascadeResult{}, err
	}
	return ruleID, deliverSystem(ctx, changes), nil
}

// CascadeBundleRetired retires a bundle and takes back what only it was
// granting: each holder's closure delta without the bundle, planned on one
// snapshot under the lock (task 1.9) and committed with the retirement.
// Coverage is a property of a person, not of a bundle: two holders can lose
// different roles by it going, because one also has a direct grant. Holders
// are the assignment table's, which is exactly who a bundle reaches.
func CascadeBundleRetired(ctx context.Context, actor, bundleID string) (CascadeResult, error) {
	var changes []Change
	err := withLockedAccess(ctx, func(ctx context.Context) error {
		bundle, err := svcGetBundleByID(ctx, bundleID)
		if err != nil {
			return err
		}
		if err := refuseRetired(bundle); err != nil {
			return err
		}
		// Refused while another bundle includes it (B8): its people get these
		// roles through that bundle, and retiring would strip them unasked.
		blockers, err := svcRetireBlockers(ctx, bundleID)
		if err != nil {
			return err
		}
		if len(blockers) > 0 {
			return &ErrBundleIncluded{Includers: blockers}
		}
		holders, err := svcGetUsersForBundle(ctx, bundleID)
		if err != nil {
			return err
		}
		s, err := systemSnapshot(ctx, holders)
		if err != nil {
			return err
		}
		changes = nil
		for _, u := range holders {
			person := s.Access.People[u]
			ch := systemChange(u, person, s, baseFrom(person, ""), baseFrom(person, bundleID), s.Access.Rules, s.Access.Rules, "bundle", bundleID)
			if len(ch.Writes) > 0 {
				changes = append(changes, ch)
			}
		}
		params, err := systemParams(ctx, actor, "Bundle deletion cascade", changes)
		if err != nil {
			return err
		}
		ids, err := svcRetireBundleAndEnqueue(ctx, actor, bundleID, params)
		if err != nil {
			return err
		}
		changes, err = pairWritten(changes, ids)
		return err
	})
	if err != nil {
		return CascadeResult{}, err
	}
	return deliverSystem(ctx, changes), nil
}

// CascadeRuleUpdated re-projects a rule whose matcher or target changed. `old`
// is the rule as it was (the handler read it; the new fields don't say the old
// target); sp/sr/tp/tr are the new fields. For every known user the closure
// through the active rules is compared with the closure with old.ID's edge
// replaced, on one snapshot under the lock (task 1.9), and the difference is
// committed with the update, each row to the rule that reaches its role
// (old.ID for its own target, the next rule down a chain). Someone who ends up with
// the same access either way gets no rows.
func CascadeRuleUpdated(ctx context.Context, actor string, old models.MappingRule, sp, sr, tp, tr string) (CascadeResult, error) {
	var changes []Change
	err := withLockedAccess(ctx, func(ctx context.Context) error {
		var err error
		// ponytail: if old.ID isn't among the active rules (it always is),
		// the rules after are the rules before, and the delta a safe no-op.
		changes, err = everyone(ctx, func(rules []models.MappingRule) []models.MappingRule {
			for i, ru := range rules {
				if ru.ID == old.ID {
					rules[i] = models.MappingRule{ID: old.ID, SourceProject: sp, SourceRole: sr, TargetProject: tp, TargetRole: tr}
					break
				}
			}
			return rules
		})
		if err != nil {
			return err
		}
		params, err := systemParams(ctx, actor, "Mapping-rule update cascade", changes)
		if err != nil {
			return err
		}
		ids, err := svcUpdateRuleAndEnqueue(ctx, actor, old.ID, sp, sr, tp, tr, params)
		if err != nil {
			return err
		}
		changes, err = pairWritten(changes, ids)
		return err
	})
	if err != nil {
		return CascadeResult{}, err
	}
	return deliverSystem(ctx, changes), nil
}

// CascadeRuleRetired retires a rule and takes back what only it was granting:
// CascadeRuleUpdated with no replacement edge. A role someone still reaches
// through a bundle, a direct grant or another rule stays in their closure and
// is never revoked; that coverage falls out of the closure rather than being
// asked separately. Adds are possible in principle and are written if they
// occur: the difference is computed, not assumed.
func CascadeRuleRetired(ctx context.Context, actor string, old models.MappingRule) (CascadeResult, error) {
	var changes []Change
	err := withLockedAccess(ctx, func(ctx context.Context) error {
		var err error
		changes, err = everyone(ctx, func(rules []models.MappingRule) []models.MappingRule {
			return slices.DeleteFunc(rules, func(ru models.MappingRule) bool { return ru.ID == old.ID })
		})
		if err != nil {
			return err
		}
		params, err := systemParams(ctx, actor, "Mapping-rule deletion cascade", changes)
		if err != nil {
			return err
		}
		ids, err := svcRetireRuleAndEnqueue(ctx, actor, old.ID, params)
		if err != nil {
			return err
		}
		changes, err = pairWritten(changes, ids)
		return err
	})
	if err != nil {
		return CascadeResult{}, err
	}
	return deliverSystem(ctx, changes), nil
}
