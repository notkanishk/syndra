package services

import (
	"context"
	"sort"
	"strings"
	"time"

	"syndra/internal/directory"
	"syndra/internal/models"
)

// RoleMembers answers "who can currently use the laser cutter?" — the reverse
// of ExplainUserAccess, which only ever answered it one person at a time.
//
// Every row carries the access sources that produced it, because the screen's
// whole job is to make each row's removal action name the thing being removed.
// A generic "revoke role" against a person holding it three ways is either
// ambiguous or destructive.

// RoleMember is one holder of a (project, role) pair.
type RoleMember struct {
	User models.UserProfile `json:"user"`
	// Reasons is ordered direct → bundle → mapping, the fixed reading order
	// the Access source component uses everywhere.
	Reasons []models.RoleReason `json:"reasons"`
	// Since / Expires come from the direct grant when one exists.
	Since   string `json:"since,omitempty"`
	Expires string `json:"expires,omitempty"`
	// GrantID is the direct_role_grants row id, present only for a direct
	// source — it is what the removal endpoint takes.
	GrantID string `json:"grant_id,omitempty"`
}

// RoleMembersView is the response for GET /projects/{id}/roles/{key}/members.
type RoleMembersView struct {
	ProjectID   string       `json:"project_id"`
	ProjectName string       `json:"project_name"`
	RoleKey     string       `json:"role_key"`
	DisplayName string       `json:"display_name,omitempty"`
	Description string       `json:"description,omitempty"`
	Group       string       `json:"group,omitempty"`
	ClonedFrom  string       `json:"cloned_from,omitempty"`
	Members     []RoleMember `json:"members"`
	// Counts per source, for the filter pills. A person held two ways is
	// counted under both — the pills filter rows, they do not partition people.
	DirectCount    int `json:"direct_count"`
	BundleCount    int `json:"bundle_count"`
	AutomaticCount int `json:"automatic_count"`
}

// reasonRank fixes the Direct → Via bundle → Automatic order.
func reasonRank(kind string) int {
	switch kind {
	case "direct":
		return 0
	case "bundle":
		return 1
	case "mapping":
		return 2
	default:
		return 3
	}
}

// RoleMembers resolves every holder of one (project, role) pair.
func RoleMembers(ctx context.Context, projectID, key string) (RoleMembersView, error) {
	snap, err := newAccessSnapshot(ctx)
	if err != nil {
		return RoleMembersView{}, err
	}

	view := RoleMembersView{
		ProjectID:   projectID,
		ProjectName: projectID,
		RoleKey:     key,
		Members:     []RoleMember{},
	}
	if name, nerr := directory.Default.ProjectName(ctx, projectID); nerr == nil && name != "" {
		view.ProjectName = name
	}

	// Role metadata is best-effort: a role that exists only in Zitadel has no
	// local row, and the members list must still render rather than 404. The
	// UI states that scope limit rather than implying the list is exhaustive.
	if role, rerr := svcDbGetRole(ctx, projectID, key); rerr == nil {
		view.DisplayName = role.DisplayName
		view.Description = role.Description
		view.Group = role.Group
		if role.ClonedFromProject != "" && role.ClonedFromRole != "" {
			view.ClonedFrom = role.ClonedFromProject + " / " + role.ClonedFromRole
		}
	}

	// Direct grants carry the grant id, timestamps and expiry that the role
	// map does not — index them so a direct row can offer a real removal.
	grantsByUser := map[string]models.DirectGrant{}
	if grants, gerr := svcGetAllDirectGrants(ctx, false); gerr == nil {
		for _, g := range grants {
			if g.ProjectID == projectID && g.RoleKey == key {
				grantsByUser[g.UserID] = g
			}
		}
	}

	for _, user := range snap.Users() {
		roleMap, _, ferr := snap.For(user.ID)
		if ferr != nil {
			return RoleMembersView{}, ferr
		}
		role := roleMap[roleKey{projectID: projectID, roleKey: key}]
		if role == nil {
			continue
		}

		reasons := append([]models.RoleReason(nil), role.Reasons...)
		sort.SliceStable(reasons, func(i, j int) bool {
			return reasonRank(reasons[i].Kind) < reasonRank(reasons[j].Kind)
		})

		member := RoleMember{User: user, Reasons: reasons}
		for _, reason := range reasons {
			switch reason.Kind {
			case "direct":
				view.DirectCount++
			case "bundle":
				view.BundleCount++
			case "mapping":
				view.AutomaticCount++
			}
		}
		if grant, ok := grantsByUser[user.ID]; ok {
			member.GrantID = grant.ID
			member.Since = grant.CreatedAt.UTC().Format("2006-01-02")
			if grant.ExpiresAt != nil {
				member.Expires = grant.ExpiresAt.UTC().Format("2006-01-02")
			}
		}
		view.Members = append(view.Members, member)
	}

	sort.Slice(view.Members, func(i, j int) bool {
		return strings.ToLower(view.Members[i].User.Name) < strings.ToLower(view.Members[j].User.Name)
	})
	return view, nil
}

// expiryHorizon is how far ahead "expiring soon" looks. Shared by the Today
// summary and the sidebar indicators so a badge and the page it points at can
// never disagree about what counts as soon.
const expiryHorizon = 14 * 24 * time.Hour

// reviewHorizon is the wider window Review › Expiring access and the People
// index work to. Today deliberately stays at expiryHorizon: its job is a queue
// short enough to finish, and a 30-day list is a review, not a queue.
const reviewHorizon = 30 * 24 * time.Hour

// Indicators is the compact badge payload the sidebar polls. Four integers, so
// the rail never downloads every pending request object to render a "3".
type Indicators struct {
	PendingRequests    int  `json:"pending_requests"`
	ExpiringGrants     int  `json:"expiring_grants"`
	PendingPropagation int  `json:"pending_propagation"`
	Drift              int  `json:"drift"`
	ZitadelReachable   bool `json:"zitadel_reachable"`
	// UnconfirmedRevocations is access somebody decided to withdraw that has
	// not been withdrawn. Counted apart from PendingPropagation, which it is a
	// subset of, because the two mean opposite things about urgency: a queued
	// grant is somebody waiting, and a queued revocation is somebody still
	// holding what was taken away.
	UnconfirmedRevocations int `json:"unconfirmed_revocations"`
	// RevocationsEscalated says at least one of them is a finding rather than a
	// queue depth — spent, or old enough that it is not draining but stuck. The
	// badge changes on this rather than on the count, because a count cannot
	// carry the difference and an operator reading "3" cannot tell.
	RevocationsEscalated bool `json:"revocations_escalated"`
}

// revocationEscalation is how long a queued revocation may age before it stops
// being a queue and starts being a finding.
//
// A day: long enough that an operator who has not resumed the drain over a
// weekend afternoon is not paged, short enough that access somebody withdrew is
// never quietly retained for a week.
const revocationEscalation = 24 * time.Hour

// GovernanceIndicators counts the four badge signals directly, without
// building the full GovernanceSummary payload.
func GovernanceIndicators(ctx context.Context) (Indicators, error) {
	out := Indicators{ZitadelReachable: true}

	requests, err := svcGetAccessRequests(ctx, "pending")
	if err != nil {
		return Indicators{}, err
	}
	out.PendingRequests = len(requests)

	expiring, err := svcGetExpiringDirectGrants(ctx, expiryHorizon)
	if err != nil {
		return Indicators{}, err
	}
	out.ExpiringGrants = len(expiring)

	pending, err := svcCountPendingPropagations(ctx)
	if err != nil {
		return Indicators{}, err
	}
	out.PendingPropagation = pending

	drift, err := svcCountPendingDrift(ctx)
	if err != nil {
		return Indicators{}, err
	}
	out.Drift = drift

	revocations, err := svcCountUnconfirmedRevocations(ctx)
	if err != nil {
		return Indicators{}, err
	}
	out.UnconfirmedRevocations = revocations.Queued + revocations.Spent
	out.RevocationsEscalated = revocations.Escalated(revocationEscalation)

	// Only worth probing when there is something queued: the flag exists to
	// explain why "Resume now" is disabled, and with an empty outbox there is
	// nothing to resume.
	if pending > 0 {
		out.ZitadelReachable = svcZitadelReachable(ctx)
	}
	return out, nil
}

// DirectGrantRemoval reports what removing a direct grant actually did.
//
// Revoked and Retained exist so the caller can verify the promise the
// confirmation dialog made: "they will lose this role" versus "they will still
// hold this role via Lab Tech". A removal that claims retention and then
// revokes upstream is the failure this type is here to make visible.
type DirectGrantRemoval struct {
	OutboxIDs []string `json:"outbox_ids"`
	// Revoked lists "project/role" pairs that lost their last source and are
	// queued for removal from the identity provider.
	Revoked []string `json:"revoked_roles"`
	// Retained lists pairs the person keeps because another source still
	// covers them. Nothing is queued for these.
	Retained []string `json:"retained_roles"`
	Status   string   `json:"status"`
}

// DeleteDirectGrant removes one Syndra direct grant and enqueues only the
// access the person actually loses.
//
// The delta is computed the same way every other cascade computes it: the
// effective-role closure before the deletion versus the closure after it, from
// pre-mutation reads plus an in-memory simulation of the removal. A role still
// carried by a bundle or produced by a mapping rule stays in `after` and is
// never revoked; a rule-derived role this grant alone supported falls out of
// `after` and is.
//
// Enqueuing an unconditional revoke — which is what this did before — removed
// access upstream that the person demonstrably still held, contradicting the
// dialog and taking the role away until the next compile restored it.
//
// This is deliberately NOT the Zitadel-side grant delete: that removes a
// different object and leaves the Syndra row behind, so the next cache compile
// puts the access straight back. Deleting the ledger row is what actually ends
// the access; the outbox rows are what carry it upstream.
func DeleteDirectGrant(ctx context.Context, userID, grantID, actor string) (DirectGrantRemoval, error) {
	var ids, retained []string
	var revokes []roleKey

	// Read and write under one lock, for the same reason expiry does: a delta
	// computed while another cascade is mid-flight is a statement about a world
	// neither of them ends up in.
	if err := svcInTxLockingAccess(ctx, func(ctx context.Context) error {
		rules, err := svcGetActiveMappingRules(ctx)
		if err != nil {
			return err
		}

		before, err := userBaseHoldings(ctx, userID)
		if err != nil {
			return err
		}
		after, err := userBaseHoldingsExcludingGrant(ctx, userID, grantID)
		if err != nil {
			return err
		}

		afterClosure := effectiveClosure(after, rules)
		_, revokes = closureDelta(effectiveClosure(before, rules), afterClosure)

		// Retained is the dialog's exact claim: the role this grant carried, still
		// effective afterwards because a bundle or a rule also covers it. Reported,
		// never enqueued — it is precisely the role that must NOT be revoked.
		retained = make([]string, 0, 1)
		if target, found := directGrantRoleKey(ctx, userID, grantID); found && afterClosure[target] {
			retained = append(retained, target.projectID+"/"+target.roleKey)
		}

		params, err := deltaParams(ctx, userID, nil, revokes, actor, "Direct access removal", "direct", grantID)
		if err != nil {
			return err
		}

		ids, err = svcDeleteDirectGrantAndEnqueue(ctx, actor, userID, grantID, params)
		return err
	}); err != nil {
		return DirectGrantRemoval{}, err
	}

	revoked := make([]string, 0, len(revokes))
	for _, key := range revokes {
		revoked = append(revoked, key.projectID+"/"+key.roleKey)
	}

	return DirectGrantRemoval{
		OutboxIDs: ids,
		Revoked:   revoked,
		Retained:  retained,
		Status:    "pending",
	}, nil
}

// ExpiredGrantRevocation is what one expired grant produced.
//
// ProjectID and RoleKey come from the row the delete actually removed, not from
// the sweep's snapshot, so every downstream side effect names what went away.
type ExpiredGrantRevocation struct {
	ProjectID string
	RoleKey   string
	OutboxIDs []string
	// Revoked lists "project/role" pairs the subject genuinely lost.
	Revoked []string
	// Retained lists pairs a bundle or a mapping rule still covers. Nothing is
	// queued for these — the grant lapsed, the access did not.
	Retained []string
}

// ExpireDirectGrant ends one expired grant the way an operator's removal ends a
// live one: same closure delta, same single transaction, same outbox.
//
// Expiry used to delete the ledger row and then call Zitadel directly to revoke
// whatever mapping rules derived from it. Two things were wrong with that. The
// grant itself was never revoked upstream at all, so an expiring grant left the
// access live in Zitadel with no Syndra record explaining it — and the next
// drift sweep, correctly, raised it as unexplained access for a human to
// triage. Expiry manufactured drift out of its own inaction. And the derived
// revocations it did issue were unconditional: a rule-derived role the subject
// still holds through a bundle, or through another grant of the same source
// role, was taken away anyway, with nothing durable recording that it had been.
//
// Both follow from the same omission — expiry was computing no delta. It has
// one now, and it is the delta every other removal computes.
//
// `before` is built from `after` plus the expiring grant's own role rather than
// read separately, because the "before" read would not contain it: base
// holdings exclude expired grants by definition, so reading both sides would
// compare a world without this grant against a world without this grant and
// find nothing to revoke. The one fact that distinguishes the two states is the
// grant being expired, and that fact is the input, not something to rediscover.
func ExpireDirectGrant(ctx context.Context, userID, grantID, projectID, role, actor string) (ExpiredGrantRevocation, error) {
	var out ExpiredGrantRevocation

	// The lock is taken before the reads, not around the write. A delta is a
	// statement about a world, and the window that matters runs from the read
	// that observed that world to the commit that acts on it — a bundle
	// assignment landing in the middle makes the revoke a statement about a
	// world that no longer exists, and the add it queued lands first, so the
	// subject ends up without access they are currently owed.
	//
	// The reads deliberately stay on the pool rather than on this transaction.
	// What they need is not to see this transaction's own writes — there are
	// none yet — but for nothing that could invalidate them to be able to
	// commit while they run, and every enqueue must take this same lock.
	err := svcInTxLockingAccess(ctx, func(ctx context.Context) error {
		rules, err := svcGetActiveMappingRules(ctx)
		if err != nil {
			return err
		}
		after, err := userBaseHoldingsExcludingGrant(ctx, userID, grantID)
		if err != nil {
			return err
		}
		lapsed := roleKey{projectID: projectID, roleKey: role}
		before := make(map[roleKey]bool, len(after)+1)
		for k := range after {
			before[k] = true
		}
		before[lapsed] = true

		afterClosure := effectiveClosure(after, rules)
		_, revokes := closureDelta(effectiveClosure(before, rules), afterClosure)

		retained := make([]string, 0, 1)
		if afterClosure[lapsed] {
			retained = append(retained, lapsed.projectID+"/"+lapsed.roleKey)
		}

		params, err := deltaParams(ctx, userID, nil, revokes, actor, "Grant expired", "direct", grantID)
		if err != nil {
			return err
		}

		deletedProject, deletedRole, ids, err := svcDeleteExpiredDirectGrantAndEnqueue(ctx, actor, userID, grantID, params)
		if err != nil {
			return err
		}

		revoked := make([]string, 0, len(revokes))
		for _, key := range revokes {
			revoked = append(revoked, key.projectID+"/"+key.roleKey)
		}
		out = ExpiredGrantRevocation{
			ProjectID: deletedProject,
			RoleKey:   deletedRole,
			OutboxIDs: ids,
			Revoked:   revoked,
			Retained:  retained,
		}
		return nil
	})
	if err != nil {
		return ExpiredGrantRevocation{}, err
	}
	return out, nil
}

// RevocationEscalationThreshold exposes the one rule so a surface renders the
// same escalation the indicator counted.
//
// Exported rather than duplicated: two components deciding independently when a
// queued revocation becomes a finding is two components that will disagree, on
// the badge whose whole job is to agree with the page it links to.
func RevocationEscalationThreshold() time.Duration { return revocationEscalation }
