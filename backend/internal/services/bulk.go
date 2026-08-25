package services

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"syndra/internal/directory"
	"syndra/internal/models"
)

// Bulk access changes, rehearsed before they are applied.
//
// A bulk write touches dozens of people's access at once, which is exactly the
// shape of change an operator cannot audit from a summary count. So the plan is
// computed against the live database FIRST and handed back unapplied: for every
// selected person, what would actually happen, and what they would still hold
// afterwards. The confirmation dialog IS that plan. Apply then re-walks the same
// evaluation and executes only the rows the rehearsal marked actionable — a
// person whose state changed between rehearsal and apply is re-evaluated rather
// than acted on from a stale verdict.
//
// This mirrors the reset-runbook pattern already in the tree: rehearse against
// the database, then offer to run it.

// Bulk operation identifiers. Anything outside this set is a validation error —
// there is no default op, because guessing what an operator meant to do to
// forty people's access is not a thing this package does.
const (
	BulkOpAssignRole   = "assign_role"
	BulkOpRemoveRole   = "remove_role"
	BulkOpAssignBundle = "assign_bundle"
	BulkOpRemoveBundle = "remove_bundle"
	BulkOpExtend       = "extend"
)

// Effects a rehearsed row can carry.
//
//   - EffectApply    — this person's access will change.
//   - EffectNoChange — nothing to do; already in the target state.
//   - EffectBlocked  — refused, with a reason. Never silently skipped.
//   - EffectFailed   — attempted during apply and errored.
//   - EffectQueued   — recorded by Syndra, not yet confirmed by Zitadel.
const (
	EffectApply    = "apply"
	EffectNoChange = "no_change"
	EffectBlocked  = "blocked"
	EffectFailed   = "failed"
	// EffectApplied replaces EffectApply once the write has landed, so the
	// result an operator reads after the fact is diffable against the plan they
	// approved rather than a fresh document with no relationship to it.
	//
	// "Landed" means Zitadel confirmed it, not that Syndra wrote it down. The
	// two are not the same and the gap between them is where a bulk removal
	// reads as done while the role is still live on the door.
	EffectApplied = "applied"
	// EffectQueued is that gap, named. Syndra's records are updated and durable,
	// but the change has not reached Zitadel — the drain was refused, halted, or
	// could not confirm. Recoverable rather than wrong: the row stays in the
	// outbox and the next drain re-drives it. It is not EffectFailed, which
	// would claim nothing happened, and it must never be EffectApplied.
	EffectQueued = "queued"
)

// BulkRequest is one operation aimed at a set of people.
type BulkRequest struct {
	Op        string
	UserIDs   []string
	ProjectID string
	RoleKey   string
	BundleID  string
	Reason    string
	// DurationDays is the validity window for assign_role and the new window
	// for extend. Zero means no expiry for assign_role; extend rejects zero,
	// because "extend by nothing" is not an extension.
	DurationDays int
	// GrantIDs narrows `extend` to specific grants. Empty means every expiring
	// direct grant the named people hold, which is what an operator who selected
	// PEOPLE asked for.
	//
	// Review › Expiring access selects grant ROWS, and the difference is not
	// cosmetic: reducing those rows to their user ids and extending everything
	// those people hold renews grants the operator never saw — including ones
	// outside the 30-day window the screen is scoped to. A bulk write must be
	// able to say exactly what was ticked.
	GrantIDs []string
}

// BulkOutcome is one person's row in the plan (before) or the result (after).
type BulkOutcome struct {
	UserID string `json:"user_id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Effect string `json:"effect"`
	// Detail states the change in the operator's language: "gains trained",
	// "already holds it via the Safety bundle", "account is departed".
	Detail string `json:"detail"`
	// Consequence is the part operators get wrong from a summary count: what
	// this person is left holding afterwards. Empty when there is nothing
	// surprising to say.
	Consequence string `json:"consequence,omitempty"`
	// Grants the row would act on. Populated for remove_role and extend so the
	// apply pass acts on identified rows rather than re-guessing.
	GrantIDs []string `json:"grant_ids,omitempty"`
	// Fingerprint digests the state this row was rehearsed against, so an apply
	// citing the plan can tell whether the world it described is still the
	// world (design §8).
	//
	// Never serialised. It is an integrity value the backend compares against
	// its own recomputation; a client holding it could tell an operator's edited
	// request from a moved subject without asking, which is the backend's answer
	// to give, and a client that can send one is a client the comparison is no
	// longer about.
	Fingerprint string `json:"-"`
}

// BulkPlan is the whole rehearsal: every selected person, in a stable order,
// plus the counts the confirmation headline is written from.
type BulkPlan struct {
	Op string `json:"op"`
	// PlanID is the approval this rehearsal became. An apply cites it instead
	// of asking for the diff to be recomputed against a world that moved.
	PlanID   string        `json:"plan_id,omitempty"`
	Applied  bool          `json:"applied"`
	Outcomes []BulkOutcome `json:"outcomes"`
	Summary  BulkSummary   `json:"summary"`
}

type BulkSummary struct {
	Total     int `json:"total"`
	Apply     int `json:"apply"`
	NoChange  int `json:"no_change"`
	Blocked   int `json:"blocked"`
	Failed    int `json:"failed"`
	Succeeded int `json:"succeeded"`
	// Queued counts rows Syndra recorded but could not confirm upstream. Kept
	// apart from Succeeded so the headline cannot round them into success.
	Queued int `json:"queued"`
}

// ValidateBulkRequest reports the first structural problem with a request, or
// nil. Field-level so the handler can answer with a validation map rather than
// a sentence.
func ValidateBulkRequest(req BulkRequest) map[string]string {
	problems := map[string]string{}

	switch req.Op {
	case BulkOpAssignRole, BulkOpRemoveRole:
		if strings.TrimSpace(req.ProjectID) == "" {
			problems["project_id"] = "required"
		}
		if strings.TrimSpace(req.RoleKey) == "" {
			problems["role_key"] = "required"
		}
	case BulkOpAssignBundle, BulkOpRemoveBundle:
		if strings.TrimSpace(req.BundleID) == "" {
			problems["bundle_id"] = "required"
		}
	case BulkOpExtend:
		if req.DurationDays <= 0 {
			problems["duration_days"] = "min=1"
		}
	default:
		problems["op"] = "must be one of assign_role, remove_role, assign_bundle, remove_bundle, extend"
	}

	// grant_ids narrows `extend` and means nothing anywhere else. Accepted and ignored, it would
	// let a caller believe they had scoped a bundle or role operation they had not.
	if req.Op != BulkOpExtend && len(req.GrantIDs) > 0 {
		problems["grant_ids"] = "only valid for extend"
	}

	// A bulk change writes one audit row per person. An unexplained one is a
	// change nobody can account for later, multiplied by the size of the
	// selection — so the reason is required at the boundary, not merely in the
	// dialog. A caller reaching this endpoint directly gets the same rule.
	if strings.TrimSpace(req.Reason) == "" {
		problems["reason"] = "required"
	}

	if len(dedupeIDs(req.UserIDs)) == 0 {
		problems["user_ids"] = "at least one required"
	}
	if len(dedupeIDs(req.UserIDs)) > BulkMaxUsers {
		problems["user_ids"] = fmt.Sprintf("max %d", BulkMaxUsers)
	}
	if req.DurationDays < 0 {
		problems["duration_days"] = "min=0"
	}

	if len(problems) == 0 {
		return nil
	}
	return problems
}

// BulkMaxUsers bounds one operation. Selecting every person in the directory is
// a legitimate thing to want; doing it in one un-paged transaction is not.
const BulkMaxUsers = 500

// RehearseBulk evaluates the request against current state without writing
// anything. One directory read is shared across every person; role resolution
// reuses the same per-user resolver the rest of the product reads from, so the
// rehearsal cannot disagree with the screen it was launched from.
func RehearseBulk(ctx context.Context, req BulkRequest) (BulkPlan, error) {
	snap, err := newAccessSnapshot(ctx)
	if err != nil {
		return BulkPlan{}, err
	}

	profiles := make(map[string]models.UserProfile, len(snap.Users()))
	for _, u := range snap.Users() {
		profiles[u.ID] = u
	}

	var bundleName string
	if req.BundleID != "" {
		if b, err := svcGetBundleByID(ctx, req.BundleID); err == nil {
			bundleName = b.Name
		}
	}

	plan := BulkPlan{Op: req.Op}
	for _, uid := range dedupeIDs(req.UserIDs) {
		outcome, err := rehearseOne(ctx, snap, profiles, req, bundleName, uid)
		if err != nil {
			return BulkPlan{}, err
		}
		plan.Outcomes = append(plan.Outcomes, outcome)
	}

	sortOutcomes(plan.Outcomes)
	plan.Summary = summarize(plan.Outcomes)
	return plan, nil
}

func rehearseOne(
	ctx context.Context,
	snap *accessSnapshot,
	profiles map[string]models.UserProfile,
	req BulkRequest,
	bundleName string,
	uid string,
) (BulkOutcome, error) {
	out := BulkOutcome{UserID: uid}

	profile, known := profiles[uid]
	if !known {
		// Fall through to the directory's own lookup: the list is cached and a
		// person created since the last fill is a real person, not a bad id.
		if p, found, err := directory.Default.FindUser(ctx, uid); err == nil && found {
			profile, known = p, true
		}
	}
	if !known {
		out.Effect = EffectBlocked
		out.Detail = "No such account in the directory."
		// Absence is reviewed state like any other: an account created between
		// the review and the apply must invalidate the approval rather than be
		// acted on under a row that said it did not exist.
		out.Fingerprint = FingerprintUserAccess(uid, "absent", nil, nil)
		return out, nil
	}
	out.Name, out.Email = profile.Name, profile.Email

	if isDepartedStatus(profile.Status) && (req.Op == BulkOpAssignRole || req.Op == BulkOpAssignBundle || req.Op == BulkOpExtend) {
		// Granting access to someone who has left is the single most common way
		// a bulk selection goes wrong — they match the filter that found the
		// cohort and nobody notices in a count.
		out.Effect = EffectBlocked
		out.Detail = fmt.Sprintf("Account is %s — remove it from the selection to continue.", strings.ToLower(profile.Status))
		out.Fingerprint = FingerprintUserAccess(uid, profile.Status, nil, nil)
		return out, nil
	}

	roleMap, bundles, err := snap.For(uid)
	if err != nil {
		return BulkOutcome{}, err
	}
	grants, err := svcGetDirectGrantsForUser(ctx, uid, true)
	if err != nil {
		return BulkOutcome{}, err
	}
	// Taken from exactly what the rehearsal below reads, and recomputed at apply
	// by the same function over a fresh read. A fingerprint the two sides
	// compute differently verifies nothing.
	out.Fingerprint = FingerprintUserAccess(uid, profile.Status, effectiveRoleKeys(roleMap), grants)

	switch req.Op {
	case BulkOpAssignRole:
		return rehearseAssignRole(out, req, roleMap, grants), nil
	case BulkOpRemoveRole:
		return rehearseRemoveRole(out, req, roleMap, grants), nil
	case BulkOpAssignBundle:
		return rehearseAssignBundle(ctx, out, req, bundleName, bundles)
	case BulkOpRemoveBundle:
		return rehearseRemoveBundle(out, req, bundleName, bundles), nil
	case BulkOpExtend:
		return rehearseExtend(out, req, grants), nil
	}

	out.Effect = EffectBlocked
	out.Detail = "Unsupported operation."
	return out, nil
}

func rehearseAssignRole(
	out BulkOutcome,
	req BulkRequest,
	roleMap map[roleKey]*models.EffectiveRole,
	grants []models.DirectGrant,
) BulkOutcome {
	key := roleKey{projectID: req.ProjectID, roleKey: req.RoleKey}
	existing := findGrant(grants, req.ProjectID, req.RoleKey)
	effective := roleMap[key]

	out.Effect = EffectApply
	switch {
	case existing != nil:
		out.Detail = "Renews the direct grant they already hold."
		out.Consequence = expiryPhrase(existing.ExpiresAt, req.DurationDays)
		out.GrantIDs = []string{existing.ID}
	case effective != nil:
		// Effective via bundle or rule. Still a legitimate thing to do — a
		// direct grant survives bundle changes — but the operator should know
		// they are adding a second source, not first access.
		out.Detail = "Adds a direct grant."
		out.Consequence = fmt.Sprintf("Already effective %s — this becomes a second, independent source.", sourcePhrase(effective.Reasons))
	default:
		out.Detail = "Gains this role."
		out.Consequence = expiryPhrase(nil, req.DurationDays)
	}
	return out
}

func rehearseRemoveRole(
	out BulkOutcome,
	req BulkRequest,
	roleMap map[roleKey]*models.EffectiveRole,
	grants []models.DirectGrant,
) BulkOutcome {
	existing := findGrant(grants, req.ProjectID, req.RoleKey)
	if existing == nil {
		// Removal here is source-specific by design: this operation removes a
		// direct grant and nothing else. Someone holding the role only through
		// a bundle or a rule is untouched, and saying so is the whole point.
		out.Effect = EffectNoChange
		if effective := roleMap[roleKey{projectID: req.ProjectID, roleKey: req.RoleKey}]; effective != nil {
			out.Detail = "Holds no direct grant here."
			out.Consequence = fmt.Sprintf("Keeps the role %s — remove that source instead.", sourcePhrase(effective.Reasons))
		} else {
			out.Detail = "Doesn't hold this role."
		}
		return out
	}

	out.Effect = EffectApply
	out.GrantIDs = []string{existing.ID}
	if other := otherSources(roleMap, req.ProjectID, req.RoleKey); other != "" {
		out.Detail = "Direct grant removed."
		out.Consequence = fmt.Sprintf("Keeps the role %s — this does not take their access away.", other)
	} else {
		out.Detail = "Loses this role."
	}
	return out
}

func rehearseAssignBundle(
	ctx context.Context,
	out BulkOutcome,
	req BulkRequest,
	bundleName string,
	bundles []models.Bundle,
) (BulkOutcome, error) {
	if hasBundle(bundles, req.BundleID) {
		out.Effect = EffectNoChange
		out.Detail = fmt.Sprintf("Already in %s.", displayBundle(bundleName))
		return out, nil
	}

	// The rehearsal has to count what the assignment will actually pin — the
	// latest published version. Counting the working copy would promise roles
	// an unpublished edit added and the apply pass would never grant.
	_, roles, err := svcLatestVersionRoles(ctx, req.BundleID)
	if err != nil {
		return BulkOutcome{}, err
	}
	out.Effect = EffectApply
	out.Detail = fmt.Sprintf("Joins %s.", displayBundle(bundleName))
	out.Consequence = fmt.Sprintf("Cascades %s.", pluralRoles(len(roles)))
	return out, nil
}

func rehearseRemoveBundle(
	out BulkOutcome,
	req BulkRequest,
	bundleName string,
	bundles []models.Bundle,
) BulkOutcome {
	if !hasBundle(bundles, req.BundleID) {
		out.Effect = EffectNoChange
		out.Detail = fmt.Sprintf("Not in %s.", displayBundle(bundleName))
		return out
	}
	out.Effect = EffectApply
	out.Detail = fmt.Sprintf("Leaves %s.", displayBundle(bundleName))
	out.Consequence = "Loses every role this bundle was their only source for."
	return out
}

func rehearseExtend(out BulkOutcome, req BulkRequest, grants []models.DirectGrant) BulkOutcome {
	// When the caller named grants, those and nothing else. The set is flat across every selected
	// person, and a grant id belongs to exactly one of them, so this per-user pass cannot let one
	// person's selection reach another's access — an id that is not theirs simply matches nothing.
	var selected map[string]bool
	if len(req.GrantIDs) > 0 {
		selected = make(map[string]bool, len(req.GrantIDs))
		for _, id := range req.GrantIDs {
			selected[id] = true
		}
	}

	// Only direct grants that actually expire. A permanent grant has nothing to
	// extend, and quietly stamping an expiry onto one would be the opposite of
	// what the operator asked for.
	var ids []string
	for _, g := range grants {
		if g.ExpiresAt == nil {
			continue
		}
		if selected != nil && !selected[g.ID] {
			continue
		}
		if req.ProjectID != "" && g.ProjectID != req.ProjectID {
			continue
		}
		if req.RoleKey != "" && g.RoleKey != req.RoleKey {
			continue
		}
		ids = append(ids, g.ID)
	}

	if len(ids) == 0 {
		out.Effect = EffectNoChange
		// Two different facts, and an operator acting on a plan needs to know which. "None
		// selected" after ticking a row means the grant moved or was removed underneath them.
		if selected != nil {
			out.Detail = "None of the selected grants are theirs any more."
		} else {
			out.Detail = "No expiring direct grants."
		}
		return out
	}

	out.Effect = EffectApply
	out.GrantIDs = ids
	out.Detail = fmt.Sprintf("Extends %s.", pluralGrants(len(ids)))
	out.Consequence = fmt.Sprintf("New expiry %s.", formatDay(time.Now().UTC().Add(time.Duration(req.DurationDays)*24*time.Hour)))
	return out
}

// ---------------------------------------------------------------------------
// Small helpers — deliberately boring, because each one is quoted in a sentence
// an operator reads immediately before changing dozens of people's access.
// ---------------------------------------------------------------------------

func findGrant(grants []models.DirectGrant, projectID, roleKey string) *models.DirectGrant {
	for i := range grants {
		if grants[i].ProjectID == projectID && grants[i].RoleKey == roleKey {
			return &grants[i]
		}
	}
	return nil
}

// otherSources names the non-direct sources for a role, or "" when the direct
// grant is the only one. This is what turns "remove from 12 people" into "3 of
// them keep it anyway".
func otherSources(roleMap map[roleKey]*models.EffectiveRole, projectID, rk string) string {
	role := roleMap[roleKey{projectID: projectID, roleKey: rk}]
	if role == nil {
		return ""
	}
	var kept []models.RoleReason
	for _, reason := range role.Reasons {
		if reason.Kind != "direct" {
			kept = append(kept, reason)
		}
	}
	if len(kept) == 0 {
		return ""
	}
	return sourcePhrase(kept)
}

// sourcePhrase renders reasons as a prepositional phrase: "via the Safety
// bundle", "via an automatic rule", "via 2 other sources".
func sourcePhrase(reasons []models.RoleReason) string {
	var named []string
	rules := 0
	for _, reason := range reasons {
		switch {
		case reason.BundleName != "":
			named = append(named, fmt.Sprintf("the %s bundle", reason.BundleName))
		case reason.Kind == "rule":
			rules++
		}
	}
	if rules == 1 {
		named = append(named, "an automatic rule")
	} else if rules > 1 {
		named = append(named, fmt.Sprintf("%d automatic rules", rules))
	}
	switch len(named) {
	case 0:
		return "through another source"
	case 1:
		return "via " + named[0]
	case 2:
		return "via " + named[0] + " and " + named[1]
	default:
		return fmt.Sprintf("via %s and %d more", named[0], len(named)-1)
	}
}

func hasBundle(bundles []models.Bundle, bundleID string) bool {
	for _, b := range bundles {
		if b.ID == bundleID {
			return true
		}
	}
	return false
}

func displayBundle(name string) string {
	if strings.TrimSpace(name) == "" {
		return "this bundle"
	}
	return "the " + name + " bundle"
}

func expiryPhrase(current *time.Time, durationDays int) string {
	if durationDays <= 0 {
		if current != nil {
			return "Expiry removed — the grant becomes permanent."
		}
		return "No expiry."
	}
	return fmt.Sprintf("Expires %s.", formatDay(time.Now().UTC().Add(time.Duration(durationDays)*24*time.Hour)))
}

func formatDay(t time.Time) string { return t.Format("2 Jan 2006") }

func pluralRoles(n int) string {
	if n == 1 {
		return "1 role"
	}
	return fmt.Sprintf("%d roles", n)
}

func pluralGrants(n int) string {
	if n == 1 {
		return "1 grant"
	}
	return fmt.Sprintf("%d grants", n)
}

func isDepartedStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "departed", "inactive", "alumni", "deactivated":
		return true
	}
	return false
}

func dedupeIDs(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// sortOutcomes puts the rows that need reading first: blocked, then changes,
// then the no-ops. Within a group, by name — so a re-run of the same rehearsal
// produces the same list in the same order.
func sortOutcomes(outcomes []BulkOutcome) {
	rank := map[string]int{EffectBlocked: 0, EffectFailed: 1, EffectQueued: 2, EffectApply: 3, EffectNoChange: 4}
	sort.SliceStable(outcomes, func(i, j int) bool {
		if rank[outcomes[i].Effect] != rank[outcomes[j].Effect] {
			return rank[outcomes[i].Effect] < rank[outcomes[j].Effect]
		}
		if outcomes[i].Name != outcomes[j].Name {
			return outcomes[i].Name < outcomes[j].Name
		}
		return outcomes[i].UserID < outcomes[j].UserID
	})
}

// SummarizeOutcomes recounts a plan after the apply pass has rewritten it.
func SummarizeOutcomes(outcomes []BulkOutcome) BulkSummary {
	s := BulkSummary{Total: len(outcomes)}
	for _, o := range outcomes {
		switch o.Effect {
		case EffectApply:
			s.Apply++
		case EffectApplied:
			s.Succeeded++
		case EffectQueued:
			s.Queued++
		case EffectNoChange:
			s.NoChange++
		case EffectBlocked:
			s.Blocked++
		case EffectFailed:
			s.Failed++
		}
	}
	return s
}

func summarize(outcomes []BulkOutcome) BulkSummary { return SummarizeOutcomes(outcomes) }
