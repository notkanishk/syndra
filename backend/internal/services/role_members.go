package services

import (
	"context"
	"fmt"
	"log"
	"slices"
	"sort"
	"syndra/internal/textsort"
	"time"

	"syndra/internal/addons"
	"syndra/internal/clock"
	"syndra/internal/db"
	"syndra/internal/directory"
	"syndra/internal/extcall"
	"syndra/internal/models"
	"syndra/internal/services/propagation"
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
	// Standing is whether this person holds the role (models.RoleStanding),
	// the same answer their own page gives; null when Zitadel hasn't been read
	// completely and no outbox row settles it.
	Standing *models.RoleStanding `json:"standing"`
	// Reasons is ordered direct → bundle → mapping, the fixed reading order
	// the Access source component uses everywhere.
	Reasons []models.RoleReason `json:"reasons"`
	// Since / Expires come from the direct grant when one exists.
	Since   string `json:"since,omitempty"`
	Expires string `json:"expires,omitempty"`
	// GrantID is the direct_role_grants row id, present only for a direct
	// source — it is what the removal endpoint takes.
	GrantID string `json:"grant_id,omitempty"`
	// Withheld is what this role confers on a target and this person does not
	// have, with the decision that took it away attached.
	//
	// On the holder list because §6 says the carve-out has to be visible
	// EVERYWHERE the role appears, and this is the screen where an operator
	// decides who to act on. Without it the list says "these forty people have
	// this role" and means "thirty-nine of them do" — which is the trap the
	// resolver's Suppressed exists to close, closed on the member's own page
	// and left open on the operator's.
	Withheld []WithheldOnRole `json:"withheld,omitempty"`
}

// WithheldOnRole is one thing this role would give somebody and does not.
type WithheldOnRole struct {
	Target      string `json:"target"`
	Field       string `json:"field"`
	Value       string `json:"value"`
	Reason      string `json:"reason"`
	ActorID     string `json:"actor_id"`
	AllowanceID string `json:"allowance_id"`
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
	// ObservedOnly is who Zitadel shows holding this role with no Syndra record
	// behind it — drift, listed here so the page cannot say "4 people hold
	// this role" while Zitadel holds twelve. Nil when never observed. Each has
	// no reasons and the same standing a member's row has: held, or being
	// removed while a removal is on its way.
	ObservedOnly []RoleMember            `json:"observed_only"`
	Observation  models.ObservationBasis `json:"observation"`
	// WithheldCount is how many of them are holding the role with something
	// taken away. Its own number rather than a length the client computes: the
	// filter pills below count people per SOURCE, and a carve-out is orthogonal
	// to how somebody came to hold the role.
	WithheldCount int `json:"withheld_count"`
	// WithheldUnavailable says the carve-out read failed and this list does not
	// know whether anybody is holding the role with something taken away.
	//
	// It exists because the degradation is otherwise INVISIBLE: a zero count
	// and no rows is byte-identical to a cohort that genuinely has none, and a
	// server log is not loud to the person reading the page. Rendering "no
	// carve-outs" from a read that never happened reproduces, on this exact
	// screen, the failure the field was added to close.
	WithheldUnavailable bool `json:"withheld_unavailable,omitempty"`
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
		ProjectID: projectID,
		RoleKey:   key,
		Members:   []RoleMember{},
	}
	if name, ok, nerr := directory.Default.ProjectName(ctx, projectID); nerr == nil && ok {
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

	// What this role confers, and what is currently taken away from it. Read
	// once for the whole cohort rather than per member: the per-subject read is
	// the only one that existed, and a screen listing forty holders would have
	// needed forty calls — which is why this list carried no carve-out at all.
	// The shape of the available read decided what the screen said.
	withheldBySubject, werr := withheldOnRole(ctx, projectID, key)
	if werr != nil {
		// Non-fatal — the holder list is still the answer to the question asked
		// — but SAID, not merely logged. The count stays zero and the flag is
		// what stops that zero from reading as "nobody has anything withheld",
		// which is the one thing this page must not say when it does not know.
		log.Printf("[ROLE-MEMBERS] could not read carve-outs for %s/%s: %v (rendered without them)", projectID, key, werr)
		view.WithheldUnavailable = true
	}

	basis, err := snap.Basis()
	if err != nil {
		return RoleMembersView{}, fmt.Errorf("observation basis: %w", err)
	}
	view.Observation = basis
	if basis.ReadAt != nil {
		view.ObservedOnly = []RoleMember{}
	}

	// Who the list is about: the members (Syndra gave it) and, once Zitadel
	// has been read, whoever Zitadel shows holding it with no record. Their
	// standings rest on their outbox rows and own reads: read for all of them
	// at once, never per person, and never for the rest of the directory. For
	// and Observed are memoised, so the loop below reads nothing again.
	want := projectID + ":" + key
	var asked []string
	for _, user := range snap.Users() {
		roleMap, _, ferr := snap.For(user.ID)
		if ferr != nil {
			return RoleMembersView{}, ferr
		}
		if roleMap[roleKey{projectID: projectID, roleKey: key}] != nil {
			asked = append(asked, user.ID)
			continue
		}
		if basis.ReadAt != nil {
			observed, oerr := snap.Observed(user.ID)
			if oerr != nil {
				return RoleMembersView{}, oerr
			}
			if observed[want] {
				asked = append(asked, user.ID)
			}
		}
	}
	if len(asked) > 0 {
		if err := snap.Prime(asked); err != nil {
			return RoleMembersView{}, err
		}
	}

	for _, user := range snap.Users() {
		roleMap, _, ferr := snap.For(user.ID)
		if ferr != nil {
			return RoleMembersView{}, ferr
		}
		role := roleMap[roleKey{projectID: projectID, roleKey: key}]
		if role == nil {
			if basis.ReadAt != nil {
				observed, oerr := snap.Observed(user.ID)
				if oerr != nil {
					return RoleMembersView{}, oerr
				}
				if observed[want] {
					// Held in Zitadel with no reason in Syndra, and said with the
					// same standing a member's row has: held, or being removed
					// once somebody has queued its removal.
					standings, serr := snap.Standing(user.ID)
					if serr != nil {
						return RoleMembersView{}, serr
					}
					view.ObservedOnly = append(view.ObservedOnly, RoleMember{
						User: user, Reasons: []models.RoleReason{}, Standing: standings.Of(projectID, key),
					})
				}
			}
			continue
		}
		standings, serr := snap.Standing(user.ID)
		if serr != nil {
			return RoleMembersView{}, serr
		}

		reasons := append([]models.RoleReason(nil), role.Reasons...)
		sort.SliceStable(reasons, func(i, j int) bool {
			return reasonRank(reasons[i].Kind) < reasonRank(reasons[j].Kind)
		})

		member := RoleMember{User: user, Reasons: reasons, Standing: standings.Of(projectID, key)}
		// Pills count people, not reasons: two bundles granting the same role
		// are one person on the Bundle pill, so each kind counts once per member.
		seen := map[string]bool{}
		for _, reason := range reasons {
			if seen[reason.Kind] {
				continue
			}
			seen[reason.Kind] = true
			switch reason.Kind {
			case "direct":
				view.DirectCount++
			case "bundle":
				view.BundleCount++
			case "mapping":
				view.AutomaticCount++
			}
		}
		if held := withheldBySubject[user.ID]; len(held) > 0 {
			member.Withheld = held
			view.WithheldCount++
		}
		if grant, ok := grantsByUser[user.ID]; ok {
			member.GrantID = grant.ID
			member.Since = clock.Day(grant.CreatedAt)
		}
		// When the role ends for them, from every source (access.Ends): a
		// direct grant's end only when nothing else gives it for longer, a
		// rule role's with its source. The token says the same.
		if role.Ends != nil {
			member.Expires = clock.Day(*role.Ends)
		}
		view.Members = append(view.Members, member)
	}

	textsort.ByName(view.Members, func(m RoleMember) string { return m.User.Name }, nil)
	return view, nil
}

// withheldOnRole is, per subject, what this role confers that they do not have.
//
// Two reads and an intersection, and the intersection is the whole point: an
// allowance is a denial on a (target, field, value), and it belongs on THIS
// role's row only when this role is what would otherwise confer that value. A
// person suspended from a share their OTHER role grants is not holding this one
// with something taken away, and saying so here would move the carve-out onto
// the wrong role — which is worse than omitting it, because an operator would
// act on this row.
//
// Lifecycle denials are the exception and they belong on every role that
// reaches the target: they are bound to no mapping, so the intersection cannot
// find them, and they are matched on the target alone.
//
// The justification differs between the two, and the difference will matter.
// The `account` field denied (TrueNAS `enabled = true`) means the ACCOUNT is
// off — nothing the role confers there is reachable, unconditionally, whatever
// fields exist now or later. A `channel` (TrueNAS `smb_enabled`) is narrower:
// it withholds only what SMB carries. Which is which is the target's persisted
// role map (targets.field_roles), never a field name. Today `group`
// is the sole entitlement field and it is SMB-mediated, so the two coincide and
// showing an SMB denial against any role is exactly true. It stops being true
// the moment a non-SMB field lands — quotas and path grants are in phase 2 —
// at which point an SMB denial on a role that only binds a quota would claim to
// withhold something it does not touch.
//
// `TestTheLifecycleExceptionRestsOnEverySchemaFieldBeingSMBMediated` fails when
// a field is added, so the next person has to decide rather than inherit.
func withheldOnRole(ctx context.Context, projectID, key string) (map[string][]WithheldOnRole, error) {
	targets, err := dbTargetsMappedToRole(ctx, projectID, key)
	if err != nil {
		return nil, fmt.Errorf("read targets mapped to %s/%s: %w", projectID, key, err)
	}
	if len(targets) == 0 {
		// A role mapped to nothing confers nothing on any target, so nothing can
		// be withheld from it. The common case, and it costs one indexed read.
		return nil, nil
	}

	// What this role binds, per target: the set an allowance has to match.
	conferred := map[string]map[string]bool{} // target -> "field\x00value"
	roles := map[string]addons.Roles{}        // which fields are the account and its channels
	for _, target := range targets {
		r, err := svcFieldRoles(ctx, target)
		if err != nil {
			return nil, fmt.Errorf("read the field roles of %s: %w", target, err)
		}
		roles[target] = r
		mappings, err := dbMappingsForRoles(ctx, target, []db.RoleRef{{ProjectID: projectID, RoleKey: key}})
		if err != nil {
			return nil, fmt.Errorf("read mappings for %s/%s on %s: %w", projectID, key, target, err)
		}
		bound := map[string]bool{}
		for _, m := range mappings {
			bound[m.Field+"\x00"+m.Value] = true
		}
		conferred[target] = bound
	}

	allowances, err := dbAllowancesOnTargets(ctx, targets)
	if err != nil {
		return nil, fmt.Errorf("read allowances on the targets %s/%s reaches: %w", projectID, key, err)
	}

	out := map[string][]WithheldOnRole{}
	for _, a := range allowances {
		if a.Direction != db.AllowanceDeny {
			// The additive arm is refused at the write, so reaching here means
			// the write path grew one. Skipped rather than rendered as a
			// carve-out: an allowance that GIVES something is not access taken
			// away, and drawing it as one would say the opposite of the truth.
			continue
		}
		bound, ok := conferred[a.Target]
		if !ok {
			continue
		}
		if !roles[a.Target].IsLifecycle(a.Field) && !bound[a.Field+"\x00"+a.Value] {
			continue
		}
		out[a.SubjectID] = append(out[a.SubjectID], WithheldOnRole{
			Target: a.Target, Field: a.Field, Value: a.Value,
			Reason: a.Reason, ActorID: a.ActorID, AllowanceID: a.ID,
		})
	}
	return out, nil
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
	// HoldsDue is holds whose review date has passed and which are still in
	// force. Its own count beside ExpiringGrants, never folded into it, because
	// inaction means the opposite thing in each: an expiring grant lapses if
	// nobody acts, and a hold stays. A badge that summed them would be counting
	// "access about to end" together with "access still being withheld".
	HoldsDue int `json:"holds_due"`
	// UnconfirmedOperations is, per target, the changes it never answered that
	// wait for a person to say what happened (A-09 §3). Passwords are never in
	// it: they settle themselves (A-13 §5). Targets with none are absent.
	UnconfirmedOperations map[string]int `json:"unconfirmed_operations"`
	// Retrying is the changes Zitadel didn't take that Syndra sends again on
	// its own (a subset of PendingPropagation), and GaveUp the ones it stopped
	// trying in the last week. Said on Home and the rail, so an automatic
	// retry is never silent, nor its end (owner, 5 Nov 2026). The *By maps
	// split each by the system the change was for (zitadel, truenas, ...), so a
	// TrueNAS change is never said to be Zitadel's (re-audit R11).
	Retrying   int            `json:"retrying"`
	GaveUp     int            `json:"gave_up"`
	RetryingBy map[string]int `json:"retrying_by"`
	GaveUpBy   map[string]int `json:"gave_up_by"`
	// AutoRetry is whether the background drain runs on this server
	// (REVOCATION_DRAIN_ENABLED). Off, nothing is sent again on its own, so
	// every surface says "press Send" instead of a next try.
	AutoRetry bool `json:"auto_retry"`
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
func GovernanceIndicators(ctx context.Context, actor string) (Indicators, error) {
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

	holds, err := svcAllowancesDueForReview(ctx)
	if err != nil {
		return Indicators{}, err
	}
	out.HoldsDue = len(holds)

	unconfirmed, err := svcCountUnresolvedAddonOperations(ctx, db.AddonUnresolvedGrace, addons.PhysicalHold())
	if err != nil {
		return Indicators{}, err
	}
	out.UnconfirmedOperations = unconfirmed

	retrying, gaveUp, err := svcCountRetries(ctx, actor)
	if err != nil {
		return Indicators{}, err
	}
	out.RetryingBy, out.GaveUpBy = retrying, gaveUp
	for _, n := range retrying {
		out.Retrying += n
	}
	for _, n := range gaveUp {
		out.GaveUp += n
	}
	out.AutoRetry = propagation.AutoRetry()

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
	// WaitReason: why the removal isn't in Zitadel yet, when it isn't.
	WaitReason string `json:"wait_reason,omitempty"`
	// Retry is the next try and which try, when Zitadel didn't take it.
	Retry *extcall.RetryState `json:"retry,omitempty"`
	// Revoked lists "project/role" pairs that lost their last source and are
	// queued for removal from the identity provider.
	Revoked []string `json:"revoked_roles"`
	// Retained lists pairs the person keeps because another source still
	// covers them. Nothing is queued for these.
	Retained []string `json:"retained_roles"`
	Status   string   `json:"status"`
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
	// Change is what the planner decided and the delete wrote, each write
	// with its row: the sweep sends it (DeliverChange).
	Change Change
}

// ExpireDirectGrant ends one grant whose end has passed the way an operator's
// removal ends a live one, through the planner (task 1.9): the closure delta
// planned on the snapshot read under the access lock (planExpiry), written
// with the guarded ledger delete in one transaction, each write marked to go
// straight away or to wait for you because its role waits. It returns the
// Change; the caller sends it after the commit.
//
// Expiry used to compute no delta at all: base holdings exclude an ended
// grant, so reading both sides compared a world without it against a world
// without it, and the access stayed live in Zitadel with no record (drift
// manufactured by expiry). The delta is now every removal's.
//
// It cancels only the queued changes its plan named (systemGate): a give of
// the lapsed role still waiting to be sent is cancelled, as before (sending
// it later would restore access that ended), and nothing else. The DELETE's
// own predicate stays the authority on whether the grant ended: a grant
// renewed since the sweep read it is db.ErrGrantRenewed, a vanished one
// db.ErrGrantNotFound.
func ExpireDirectGrant(ctx context.Context, g models.DirectGrant, actor string) (ExpiredGrantRevocation, error) {
	ctx = systemGate(ctx)
	var out ExpiredGrantRevocation

	// The lock is taken before the reads, not around the write: a delta is a
	// statement about a world, and a bundle assignment landing between the read
	// and the commit would make the revoke one about a world that no longer
	// exists.
	err := withLockedAccess(ctx, func(ctx context.Context) error {
		s, err := systemSnapshot(ctx, []string{g.UserID})
		if err != nil {
			return err
		}
		ch := planExpiry(g, s.Access.People[g.UserID], s)
		if ch.Refusal == "renewed" {
			return db.ErrGrantRenewed
		}
		db.CancellationsNamed(ctx, ch.Row.CancelIDs)
		params, err := systemParams(ctx, actor, "Grant expired", []Change{ch})
		if err != nil {
			return err
		}
		deletedProject, deletedRole, ids, err := svcDeleteExpiredDirectGrantAndEnqueue(ctx, actor, g.UserID, g.ID, params)
		if err != nil {
			return err
		}
		if ch, err = ch.written(ids); err != nil {
			return err
		}
		// Retained is the lapsed role when a bundle or a rule still gives it:
		// nothing is queued for it, the grant lapsed and the access did not.
		lapsed := g.ProjectID + "/" + g.RoleKey
		out = ExpiredGrantRevocation{ProjectID: deletedProject, RoleKey: deletedRole, OutboxIDs: ids,
			Revoked: []string{}, Retained: []string{lapsed}, Change: ch}
		for _, w := range ch.Writes {
			out.Revoked = append(out.Revoked, w.ProjectID+"/"+w.RoleKeys[0])
		}
		if slices.Contains(out.Revoked, lapsed) {
			out.Retained = []string{}
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
