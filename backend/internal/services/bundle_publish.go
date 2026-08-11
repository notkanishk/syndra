package services

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"syndra/internal/db"
	"syndra/internal/directory"
	"syndra/internal/models"
)

// Publishing a bundle version, and moving holders between versions.
//
// Both are the same operation underneath — repin a set of people onto a version
// and project the difference — and both are rehearsed before they apply, using
// the BulkPlan every other bulk surface in the product speaks. A version bump
// that reaches fourteen people is exactly the kind of change where the count is
// not the interesting part: who loses what, and who is unaffected because
// something else already grants it, is.

// DraftDiff is the difference between a bundle's working copy and its latest
// published version — what "Publish v3" would actually contain.
type DraftDiff struct {
	BundleID string `json:"bundle_id"`
	// LatestVersion is what is published today; NextVersion is what publishing
	// would create.
	LatestVersion int                 `json:"latest_version"`
	NextVersion   int                 `json:"next_version"`
	Added         []models.BundleRole `json:"added"`
	Removed       []models.BundleRole `json:"removed"`
	// HolderCount is how many people hold the bundle at all, on any version.
	HolderCount int `json:"holder_count"`

	// Working is the working copy this diff was computed from, carried so the
	// delta, the plan and the snapshot all come from ONE read. Recomputing it
	// downstream is how the outbox ends up describing a different version from
	// the one holders are pinned to.
	Working []models.BundleRole `json:"-"`
}

// Empty reports whether the working copy and the latest version agree, in which
// case there is nothing to publish.
func (d DraftDiff) Empty() bool { return len(d.Added) == 0 && len(d.Removed) == 0 }

// BundleDraft computes the unpublished difference for one bundle.
func BundleDraft(ctx context.Context, bundleID string) (DraftDiff, error) {
	out := DraftDiff{BundleID: bundleID}

	latest, err := svcLatestVersion(ctx, bundleID)
	if err != nil {
		return out, fmt.Errorf("latest version: %w", err)
	}
	out.LatestVersion = latest.Version
	out.NextVersion = latest.Version + 1

	working, err := svcCascGetRolesForBundle(ctx, bundleID)
	if err != nil {
		return out, err
	}
	published, err := svcGetRolesForVersion(ctx, latest.ID)
	if err != nil {
		return out, err
	}
	out.Working = working
	out.Added, out.Removed = diffRoles(published, working)

	holders, err := svcGetUsersForBundle(ctx, bundleID)
	if err != nil {
		return out, err
	}
	out.HolderCount = len(holders)
	return out, nil
}

// diffRoles reports what `next` adds relative to `prev`, and what it drops.
//
// Both are EMPTY, never nil. A Go nil slice marshals to JSON `null`, and these two travel straight
// to the console on four routes — where `null` crashed the bundles screen outright: `added.length`
// throws, and it throws for every bundle whose working copy matches its published version, which
// is the normal resting state. Same convention the read handlers already apply to their own lists.
func diffRoles(prev, next []models.BundleRole) (added, removed []models.BundleRole) {
	added, removed = []models.BundleRole{}, []models.BundleRole{}

	inPrev := map[roleKey]bool{}
	for _, r := range prev {
		inPrev[roleKey{projectID: r.ProjectID, roleKey: r.RoleKey}] = true
	}
	inNext := map[roleKey]bool{}
	for _, r := range next {
		inNext[roleKey{projectID: r.ProjectID, roleKey: r.RoleKey}] = true
	}
	for _, r := range next {
		if !inPrev[roleKey{projectID: r.ProjectID, roleKey: r.RoleKey}] {
			added = append(added, r)
		}
	}
	for _, r := range prev {
		if !inNext[roleKey{projectID: r.ProjectID, roleKey: r.RoleKey}] {
			removed = append(removed, r)
		}
	}
	sortRoles(added)
	sortRoles(removed)
	return added, removed
}

func sortRoles(rs []models.BundleRole) {
	sort.Slice(rs, func(i, j int) bool {
		if rs[i].ProjectID != rs[j].ProjectID {
			return rs[i].ProjectID < rs[j].ProjectID
		}
		return rs[i].RoleKey < rs[j].RoleKey
	})
}

// PublishRequest is one publish: the note that goes on the version, and whether
// the people already holding it come along.
type PublishRequest struct {
	BundleID string
	Note     string
	// Migrate moves every current holder onto the new version. False publishes
	// the version for new assignments only and leaves existing holders exactly
	// where they are — which is a deliberate answer, not a deferral.
	Migrate bool
}

// RehearseBundlePublish computes, per holder, what publishing would do to them.
//
// The plan is built from each holder's OWN pinned version, not from the latest:
// somebody on v2 and somebody on v4 are moving different distances, and a plan
// that assumed they were all on the same version would be wrong for one of them.
func RehearseBundlePublish(ctx context.Context, req PublishRequest) (BulkPlan, DraftDiff, error) {
	plan := BulkPlan{Op: "publish_bundle_version", Outcomes: []BulkOutcome{}}

	draft, err := BundleDraft(ctx, req.BundleID)
	if err != nil {
		return plan, draft, err
	}

	next := draft.Working
	rules, err := svcGetActiveMappingRules(ctx)
	if err != nil {
		return plan, draft, err
	}

	holders, err := svcGetBundleHoldersByVersion(ctx, req.BundleID)
	if err != nil {
		return plan, draft, err
	}

	for _, h := range holders {
		out := BulkOutcome{UserID: h.UserID}

		if !req.Migrate {
			out.Effect = EffectNoChange
			out.Detail = fmt.Sprintf("stays on v%d", h.Version)
			out.Consequence = "Nothing changes for them. The new version applies to new assignments only."
			plan.Outcomes = append(plan.Outcomes, out)
			continue
		}

		adds, revokes, err := holderDelta(ctx, h.UserID, req.BundleID, next, rules)
		if err != nil {
			return plan, draft, err
		}
		out.Effect, out.Detail, out.Consequence = describeMove(h.Version, draft.NextVersion, adds, revokes)
		plan.Outcomes = append(plan.Outcomes, out)
	}

	plan.Summary = SummarizeOutcomes(plan.Outcomes)
	return plan, draft, nil
}

// holderDelta is the closure difference for one person if their pin on this
// bundle moved to a version containing `next`.
func holderDelta(
	ctx context.Context,
	userID, bundleID string,
	next []models.BundleRole,
	rules []models.MappingRule,
) (adds, revokes []roleKey, err error) {
	before, err := userBaseHoldings(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	after, err := userBaseHoldingsWithBundleAt(ctx, userID, bundleID, next)
	if err != nil {
		return nil, nil, err
	}
	a, r := closureDelta(effectiveClosure(before, rules), effectiveClosure(after, rules))
	return a, r, nil
}

// describeMove turns one holder's delta into the three sentences the rehearsal
// renders. A revoke is called out separately from a gain: an operator scanning
// a plan of fourteen rows is looking for the person who LOSES something.
func describeMove(from, to int, adds, revokes []roleKey) (effect, detail, consequence string) {
	move := fmt.Sprintf("v%d → v%d", from, to)
	switch {
	case len(adds) == 0 && len(revokes) == 0:
		// The version moves but their access does not — another bundle, a
		// direct grant or a mapping rule already covers everything that
		// changed. Saying so is the difference between a plan somebody reads
		// and a plan somebody scrolls past.
		return EffectNoChange, move + ", no change to their access",
			"Everything in this version they already hold from another source."
	case len(revokes) == 0:
		return EffectApply, fmt.Sprintf("%s, gains %s", move, keyList(adds)), ""
	case len(adds) == 0:
		return EffectApply, fmt.Sprintf("%s, LOSES %s", move, keyList(revokes)),
			"Nothing else grants that — it is removed from the identity provider."
	default:
		return EffectApply,
			fmt.Sprintf("%s, gains %s and LOSES %s", move, keyList(adds), keyList(revokes)),
			"Nothing else grants what they lose — it is removed from the identity provider."
	}
}

func keyList(keys []roleKey) string {
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k.roleKey)
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}

// PublishBundleVersion cuts the version and, when asked, moves the holders.
//
// The rehearsal is recomputed here rather than taken from the caller. A plan
// posted back from a browser is a claim about the world as it was when the
// dialog opened; the writes have to be built from the world as it is.
func PublishBundleVersion(ctx context.Context, actor string, req PublishRequest) (BulkPlan, models.BundleVersion, error) {
	var plan BulkPlan
	var version models.BundleVersion
	var ids []string
	// Rehearsal and apply under one lock: the plan an operator approved is a
	// statement about a world, and another cascade committing between the two
	// makes it a statement about neither.
	if err := withLockedAccess(ctx, func(ctx context.Context) error {
		var draft DraftDiff
		var err error
		plan, draft, err = RehearseBundlePublish(ctx, req)
		if err != nil {
			return err
		}
		if draft.Empty() {
			return fmt.Errorf("nothing to publish: the bundle matches v%d", draft.LatestVersion)
		}

		// One read, from the rehearsal. Reading the working copy a second time here
		// would let an edit land in between: the deltas would describe one set of
		// roles and the snapshot would contain another.
		next := draft.Working
		rules, err := svcGetActiveMappingRules(ctx)
		if err != nil {
			return err
		}

		var moved []string
		var params []db.EnqueueParams
		if req.Migrate {
			holders, err := svcGetBundleHoldersByVersion(ctx, req.BundleID)
			if err != nil {
				return err
			}
			for _, h := range holders {
				moved = append(moved, h.UserID)
				adds, revokes, err := holderDelta(ctx, h.UserID, req.BundleID, next, rules)
				if err != nil {
					return err
				}
				rows, err := deltaParams(ctx, h.UserID, adds, revokes, actor,
					fmt.Sprintf("Bundle published at v%d", draft.NextVersion), "bundle", req.BundleID)
				if err != nil {
					return err
				}
				params = append(params, rows...)
			}
		}

		// `next` goes into the transaction rather than being re-selected there, so
		// the version contains exactly what the plan was computed from. An edit that
		// lands mid-publish is simply not in this version — it stays a draft for the
		// next one, which is the honest outcome.
		version, ids, err = svcPublishVersionAndEnqueue(ctx, actor, req.BundleID, req.Note, next, moved, params)
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		DecoratePlan(ctx, &plan)
		return plan, version, err
	}
	// Outside the lock: the plan is being rendered, not decided.
	DecoratePlan(ctx, &plan)

	bundle, err := svcGetBundleByID(ctx, req.BundleID)
	if err != nil {
		return plan, version, err
	}
	if _, err := applyMode(ctx, bundle.ConfirmationMode, ids); err != nil {
		return plan, version, err
	}

	plan.Applied = true
	markApplied(plan.Outcomes)
	plan.Summary = SummarizeOutcomes(plan.Outcomes)
	return plan, version, nil
}

// MoveHoldersRequest repins named holders onto one version of one bundle.
type MoveHoldersRequest struct {
	BundleID  string
	VersionID string
	UserIDs   []string
}

// RehearseMoveHolders computes what repinning would do to each named person.
// Moving somebody BACKWARDS is allowed and rehearsed the same way — putting a
// person back on v2 deliberately is a legitimate answer, and it revokes.
func RehearseMoveHolders(ctx context.Context, req MoveHoldersRequest) (BulkPlan, error) {
	plan := BulkPlan{Op: "move_bundle_holders", Outcomes: []BulkOutcome{}}

	// Ownership is checked HERE, not only in the write transaction. A version
	// from another bundle produced a plan anyway — one that read "v2 → v0",
	// because the target version number could not be found among this bundle's
	// holders. A rehearsal that renders a nonsense move and is rejected only on
	// apply is worse than no rehearsal: it is a plan somebody approved.
	owns, err := svcVersionBelongsTo(ctx, req.BundleID, req.VersionID)
	if err != nil {
		return plan, err
	}
	if !owns {
		return plan, fmt.Errorf("version %s does not belong to bundle %s", req.VersionID, req.BundleID)
	}

	target, err := svcGetRolesForVersion(ctx, req.VersionID)
	if err != nil {
		return plan, err
	}
	rules, err := svcGetActiveMappingRules(ctx)
	if err != nil {
		return plan, err
	}
	holders, err := svcGetBundleHoldersByVersion(ctx, req.BundleID)
	if err != nil {
		return plan, err
	}
	wanted := map[string]models.BundleHolder{}
	for _, h := range holders {
		wanted[h.UserID] = h
	}

	// The target's own number, from the version list rather than inferred from
	// whoever happens to be standing on it. Nobody is standing on a version that
	// was just published, and "v2 → v0" is what inferring produced.
	targetVersion := 0
	versions, err := svcListBundleVersions(ctx, req.BundleID)
	if err != nil {
		return plan, err
	}
	for _, v := range versions {
		if v.ID == req.VersionID {
			targetVersion = v.Version
			break
		}
	}

	for _, id := range req.UserIDs {
		out := BulkOutcome{UserID: id}
		h, holds := wanted[id]
		if !holds {
			out.Effect = EffectBlocked
			out.Detail = "does not hold this bundle"
			plan.Outcomes = append(plan.Outcomes, out)
			continue
		}
		if h.VersionID == req.VersionID {
			out.Effect = EffectNoChange
			out.Detail = fmt.Sprintf("already on v%d", h.Version)
			plan.Outcomes = append(plan.Outcomes, out)
			continue
		}
		adds, revokes, err := holderDelta(ctx, id, req.BundleID, target, rules)
		if err != nil {
			return plan, err
		}
		out.Effect, out.Detail, out.Consequence = describeMove(h.Version, targetVersion, adds, revokes)
		if out.Effect == EffectNoChange {
			out.Detail = fmt.Sprintf("v%d → v%d, no change to their access", h.Version, targetVersion)
		}
		plan.Outcomes = append(plan.Outcomes, out)
	}

	plan.Summary = SummarizeOutcomes(plan.Outcomes)
	return plan, nil
}

// MoveHolders applies what RehearseMoveHolders described.
// errNothingToMove ends the locked region without an error: there is nothing
// to move, which is a result rather than a failure.
var errNothingToMove = errors.New("nothing to move")

func MoveHolders(ctx context.Context, actor string, req MoveHoldersRequest) (BulkPlan, error) {
	var plan BulkPlan
	var ids []string
	// Same reason as PublishBundleVersion: the rehearsal and the apply are one
	// decision, and a cascade landing between them makes the plan describe a
	// world nobody approved.
	if err := withLockedAccess(ctx, func(ctx context.Context) error {
		var err error
		plan, err := RehearseMoveHolders(ctx, req)
		if err != nil {
			return err
		}

		target, err := svcGetRolesForVersion(ctx, req.VersionID)
		if err != nil {
			return err
		}
		rules, err := svcGetActiveMappingRules(ctx)
		if err != nil {
			return err
		}

		var actionable []string
		var params []db.EnqueueParams
		for _, out := range plan.Outcomes {
			if out.Effect == EffectBlocked {
				continue
			}
			actionable = append(actionable, out.UserID)
			adds, revokes, err := holderDelta(ctx, out.UserID, req.BundleID, target, rules)
			if err != nil {
				return err
			}
			rows, err := deltaParams(ctx, out.UserID, adds, revokes, actor,
				"Bundle holder moved between versions", "bundle", req.BundleID)
			if err != nil {
				return err
			}
			params = append(params, rows...)
		}
		if len(actionable) == 0 {
			return errNothingToMove
		}

		ids, err = svcMoveHoldersAndEnqueue(ctx, actor, req.BundleID, req.VersionID, actionable, params)
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		DecoratePlan(ctx, &plan)
		if errors.Is(err, errNothingToMove) {
			return plan, nil
		}
		return plan, err
	}
	DecoratePlan(ctx, &plan)
	bundle, err := svcGetBundleByID(ctx, req.BundleID)
	if err != nil {
		return plan, err
	}
	if _, err := applyMode(ctx, bundle.ConfirmationMode, ids); err != nil {
		return plan, err
	}

	plan.Applied = true
	markApplied(plan.Outcomes)
	plan.Summary = SummarizeOutcomes(plan.Outcomes)
	return plan, nil
}

// markApplied promotes the rows that were going to act. Rows that were already
// no-change or blocked keep their effect: an apply pass must not claim to have
// done something to somebody it skipped.
func markApplied(outcomes []BulkOutcome) {
	for i := range outcomes {
		if outcomes[i].Effect == EffectApply {
			outcomes[i].Effect = EffectApplied
		}
	}
}

// DecoratePlan fills in the display names a rehearsal renders.
//
// Kept out of the rehearsal itself because the rehearsal runs inside the
// access-mutation lock when an apply is what asked for it, and this reaches the
// directory — which in live mode is Zitadel, through a cache that can miss. A
// name nobody has looked up yet would then hold the one lock every expiry,
// grant and cascade in the deployment waits on, for as long as an unreachable
// identity provider takes to time out. Names are presentation; the lock exists
// for state.
//
// A lookup that fails leaves the row identified by its subject id, which is
// what it was identified by before anyone asked for a name.
func DecoratePlan(ctx context.Context, plan *BulkPlan) {
	if plan == nil {
		return
	}
	for i := range plan.Outcomes {
		if u, ok, err := directory.Default.FindUser(ctx, plan.Outcomes[i].UserID); err == nil && ok {
			plan.Outcomes[i].Name, plan.Outcomes[i].Email = u.Name, u.Email
		}
	}
}
