package services

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"syndra/internal/clock"
	"syndra/internal/extcall"
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
	// Actor is who asks. A bulk operation that takes access away leaves them
	// out (#32, B4): an operator never takes access from themselves in bulk;
	// their own page is where they change their own access.
	Actor string
	// Note is a publish's version note, or a request decision's review note.
	Note string
	// A publish moves its holders (Migrate) and says what each bundle that
	// includes this one does (B8); a move repins onto VersionID.
	Migrate   bool
	Includers []IncluderChoice
	VersionID string
}

// BulkExclusion is a selected person a plan leaves out, and why.
type BulkExclusion struct {
	UserID string `json:"user_id"`
	Reason string `json:"reason"` // "self"
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
	// WaitReason and Retry say, for a queued row, why it isn't in Zitadel
	// yet (role_waits, manual, halted:<reason>, halted:retrying) and, when it
	// is a retry, its next try, so the result tells a row Syndra sends again
	// on its own from one that waits for Send (re-audit R2).
	WaitReason string              `json:"wait_reason,omitempty"`
	Retry      *extcall.RetryState `json:"retry,omitempty"`
	// RuleWaiting counts the rows automatic rules queued with this row's act
	// that wait for Send. Apart from Effect, so a rule's wait is never blamed
	// on the act (final verification N4): an approval or an adoption that
	// took effect stays applied, and the result still ends on Send.
	RuleWaiting int `json:"rule_waiting,omitempty"`
	// Delivery is how the row reaches Zitadel, as the planner decided it
	// (straight_away, waits_for_you, or none when nothing is sent); Waits and
	// Goes name the roles that wait for you and the ones sent straight away.
	// On the plan and on the result, so a screen can group rows by them.
	// Person is who the row changes when the row is not a person (a
	// request, a drift row): whose queued changes it may cancel, and whose
	// Pending changes a result row's "Waiting for you to send" opens (A-21
	// §2.6). Absent when the row is the person.
	Person   string   `json:"person,omitempty"`
	Delivery string   `json:"delivery,omitempty"`
	Waits    []string `json:"waits,omitempty"`
	Goes     []string `json:"goes,omitempty"`
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
	// CancelIDs are the queued changes this row said it cancels (planned ops
	// only): stored on the plan, and the only ones its apply admits. Never
	// serialised, like Fingerprint; the row's words say them.
	CancelIDs []string `json:"-"`
}

// BulkPlan is the whole rehearsal: every selected person, in a stable order,
// plus the counts the confirmation headline is written from.
type BulkPlan struct {
	Op string `json:"op"`
	// PlanID is the approval this rehearsal became. An apply cites it instead
	// of asking for the diff to be recomputed against a world that moved.
	PlanID  string `json:"plan_id,omitempty"`
	Applied bool   `json:"applied"`
	// RequestFingerprint binds an approval to the thing it approved, for the
	// surfaces whose COHORT is derived from the world rather than named in the
	// request body.
	//
	// A bulk grant lists its user ids, so its request fingerprint is computable
	// from the request and the cohort cannot change under the approval. A bundle
	// publish cannot: its cohort is whoever holds the bundle, and somebody
	// joining between the review and the apply would be moved under an approval
	// that never mentioned them. Per-subject verification does not catch that —
	// it checks the subjects ON the approval, and a new one is not on it. So the
	// reviewed cohort goes in here, and a cohort that moved makes the citation a
	// mismatch rather than a silent widening.
	//
	// Not serialised: it is the backend's own record of what it computed, never
	// something a client sends back.
	RequestFingerprint string `json:"-"`
	// Version is the version a publish cut. Not serialised: the publish
	// answers with it beside the plan.
	Version  models.BundleVersion `json:"-"`
	Outcomes []BulkOutcome        `json:"outcomes"`
	// Excluded are selected people the plan leaves out: today only the
	// requester, from an operation that takes access away.
	Excluded []BulkExclusion `json:"excluded,omitempty"`
	Summary  BulkSummary     `json:"summary"`
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

// RehearseBulk is the plan: every selected person's Change from the planner,
// on one snapshot read under the lock (planner_bulk.go). Nothing is written.
func RehearseBulk(ctx context.Context, req BulkRequest) (BulkPlan, error) {
	return rehearsePlanned(ctx, req)
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

func displayBundle(name string) string {
	if strings.TrimSpace(name) == "" {
		return "this bundle"
	}
	return "the " + name + " bundle"
}

func formatDay(t time.Time) string { return t.In(clock.Loc).Format("2 Jan 2006") }

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
