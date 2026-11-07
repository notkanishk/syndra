package services

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"syndra/internal/textsort"

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

	// Includes are the bundles the working copy includes (B8), and
	// WorkingEffective is what the next version would give: its own roles and
	// each included version's, from the same read. Added and Removed compare
	// what people get, so a role present both ways is held once.
	Includes         []models.BundleInclude `json:"includes"`
	WorkingEffective []models.BundleRole    `json:"-"`
	// IncludeChanges are the includes the next version adds, drops or moves
	// to another version of the included bundle.
	IncludeChanges []IncludeChange `json:"include_changes"`
}

// IncludeChange is one include the next version changes. From 0: added. To 0:
// removed. Both set: moved to another version of the included bundle.
type IncludeChange struct {
	ChildBundleID string `json:"child_bundle_id"`
	ChildName     string `json:"child_name"`
	From          int    `json:"from_version"`
	To            int    `json:"to_version"`
}

// Empty reports whether the working copy and the latest version agree, in which
// case there is nothing to publish.
func (d DraftDiff) Empty() bool {
	return len(d.Added) == 0 && len(d.Removed) == 0 && len(d.IncludeChanges) == 0
}

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
	includes, err := svcGetWorkingIncludes(ctx, bundleID)
	if err != nil {
		return out, err
	}
	effective := append([]models.BundleRole(nil), working...)
	for _, in := range includes {
		childRoles, err := svcGetRolesForVersion(ctx, in.ChildVersionID)
		if err != nil {
			return out, err
		}
		for _, r := range childRoles {
			r.BundleID, r.ViaBundleID = bundleID, in.ChildBundleID
			effective = append(effective, r)
		}
	}
	publishedIncludes, err := svcGetVersionIncludes(ctx, latest.ID)
	if err != nil {
		return out, err
	}
	out.Working = working
	out.Includes = includes
	out.WorkingEffective = effective
	out.Added, out.Removed = diffRoles(published, effective)
	out.IncludeChanges = diffIncludes(publishedIncludes, includes)

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

// diffIncludes reports what the working copy's includes change against the
// published version's. Never nil, for the same reason as diffRoles.
func diffIncludes(prev, next []models.BundleInclude) []IncludeChange {
	out := []IncludeChange{}
	was := map[string]models.BundleInclude{}
	for _, p := range prev {
		was[p.ChildBundleID] = p
	}
	for _, n := range next {
		p, ok := was[n.ChildBundleID]
		switch {
		case !ok:
			out = append(out, IncludeChange{ChildBundleID: n.ChildBundleID, ChildName: n.ChildName, To: n.ChildVersion})
		case p.ChildVersionID != n.ChildVersionID:
			out = append(out, IncludeChange{ChildBundleID: n.ChildBundleID, ChildName: n.ChildName, From: p.ChildVersion, To: n.ChildVersion})
		}
		delete(was, n.ChildBundleID)
	}
	for _, p := range was {
		out = append(out, IncludeChange{ChildBundleID: p.ChildBundleID, ChildName: p.ChildName, From: p.ChildVersion})
	}
	textsort.ByName(out, func(c IncludeChange) string { return c.ChildName }, nil)
	return out
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
	// Includers answers, for every bundle whose working copy includes this
	// one, whether its next version takes the new one (B8). No default: each
	// must be answered, or the publish is refused naming those left.
	Includers []IncluderChoice
}

// IncluderChoice is "update" or "leave" for one bundle that includes this one.
type IncluderChoice struct {
	BundleID string `json:"bundle_id"`
	Action   string `json:"action"`
}

// ErrIncludersUnanswered names the including bundles a publish didn't answer for.
type ErrIncludersUnanswered struct{ Names []string }

func (e *ErrIncludersUnanswered) Error() string {
	return "say whether each bundle that includes this one takes the new version: " + strings.Join(e.Names, ", ")
}

// checkIncluderChoices returns the parents to update, refusing a publish that
// leaves any including bundle unanswered or answers one that doesn't include it.
func checkIncluderChoices(ctx context.Context, req PublishRequest) ([]string, error) {
	includers, err := svcListIncluders(ctx, req.BundleID)
	if err != nil {
		return nil, err
	}
	asked := map[string]string{}
	for _, c := range req.Includers {
		if c.Action != "update" && c.Action != "leave" {
			return nil, fmt.Errorf(`includer %s: action must be "update" or "leave"`, c.BundleID)
		}
		asked[c.BundleID] = c.Action
	}
	var missing, update []string
	for _, in := range includers {
		// Only a working copy that includes it can take a new version.
		if in.DraftVersion == 0 && in.IncludedVersion == 0 {
			continue
		}
		switch asked[in.BundleID] {
		case "update":
			update = append(update, in.BundleID)
		case "leave":
		default:
			missing = append(missing, in.Name)
		}
		delete(asked, in.BundleID)
	}
	if len(missing) > 0 {
		return nil, &ErrIncludersUnanswered{Names: missing}
	}
	for id := range asked {
		return nil, fmt.Errorf("bundle %s doesn't include this one", id)
	}
	return update, nil
}

// FingerprintBundlePublish digests everything a publish was reviewed against
// that is not specific to one holder: which bundle, whether the holders move,
// what the new version will contain, and exactly who holds it at which version.
//
// The version CONTENTS belong here rather than in the per-holder rows. An edit
// to the working copy between the review and the apply changes what v_next is,
// and it can do so without changing any holder's delta — adding a role somebody
// already holds from a rule moves nobody, and still means the version being
// published is not the version that was approved.
//
// The cohort belongs here for the reason RequestFingerprint exists: a person
// assigned the bundle after the review is not a subject on the approval, so no
// amount of per-subject verification will notice them.
// publishedContents is what the version being approved contains, for its
// fingerprint: what it gives, and which version of each bundle it includes
// (B8). Moving an include to another version is a different version even when
// the roles come out the same.
func publishedContents(draft DraftDiff) []models.BundleRole {
	out := append([]models.BundleRole(nil), draft.WorkingEffective...)
	if draft.WorkingEffective == nil {
		out = append(out, draft.Working...)
	}
	for _, in := range draft.Includes {
		out = append(out, models.BundleRole{ProjectID: "include:" + in.ChildBundleID, RoleKey: in.ChildVersionID})
	}
	return out
}

func FingerprintBundlePublish(
	bundleID string,
	migrate bool,
	working []models.BundleRole,
	holders []models.BundleHolder,
) string {
	fields := []string{"bundle_publish", bundleID, "migrate", strconv.FormatBool(migrate), "contains"}

	contents := make([]string, 0, len(working))
	for _, r := range working {
		contents = append(contents, r.ProjectID+"\x00"+r.RoleKey)
	}
	sort.Strings(contents)
	fields = append(fields, contents...)

	fields = append(fields, "holders")
	fields = append(fields, holderFields(holders)...)
	return Fingerprint(fields...)
}

// FingerprintMoveHolders digests what a holder move was reviewed against: the
// bundle, the version being moved onto, what that version contains, and the
// people named. The named cohort is part of the request here, but the VERSION
// contents are not — a version is immutable once published, so including them
// costs nothing and closes the case of a caller citing an approval for one
// version against another.
func FingerprintMoveHolders(bundleID, versionID string, target []models.BundleRole, userIDs []string) string {
	fields := []string{"move_bundle_holders", bundleID, versionID, "contains"}

	contents := make([]string, 0, len(target))
	for _, r := range target {
		contents = append(contents, r.ProjectID+"\x00"+r.RoleKey)
	}
	sort.Strings(contents)
	fields = append(fields, contents...)

	ids := append([]string(nil), userIDs...)
	sort.Strings(ids)
	fields = append(fields, "subjects")
	fields = append(fields, ids...)
	return Fingerprint(fields...)
}

// holderFields renders the holder set in a read-order-independent way. Who
// holds the bundle AND which version each of them stands on: somebody moved
// from v2 to v4 by another operator is a different cohort, not the same one.
func holderFields(holders []models.BundleHolder) []string {
	rows := make([]string, 0, len(holders))
	for _, h := range holders {
		rows = append(rows, h.UserID+"\x00"+strconv.Itoa(h.Version))
	}
	sort.Strings(rows)
	return rows
}

// MoveHoldersRequest repins named holders onto one version of one bundle.
type MoveHoldersRequest struct {
	BundleID  string
	VersionID string
	UserIDs   []string
}

// RehearseBundlePublish is the plan for a publish: every holder's Change from
// the planner (planMove), on one snapshot read under the lock. Each holder
// moves from their OWN pinned version: somebody on v2 and somebody on v4 move
// different distances.
func RehearseBundlePublish(ctx context.Context, req PublishRequest) (BulkPlan, DraftDiff, error) {
	plan, snap, err := planBulk(ctx, BulkRequest{Op: OpPublish, BundleID: req.BundleID, Migrate: req.Migrate,
		Includers: req.Includers, Note: req.Note})
	plan.Op = OpPublish
	return plan, snap.Draft, err
}

// RehearseMoveHolders is the plan for moving named holders onto a version.
// Moving somebody backwards is allowed and planned the same way.
func RehearseMoveHolders(ctx context.Context, req MoveHoldersRequest) (BulkPlan, error) {
	plan, _, err := planBulk(ctx, BulkRequest{Op: OpMove, BundleID: req.BundleID, VersionID: req.VersionID, UserIDs: req.UserIDs})
	plan.Op = OpMove
	return plan, err
}
