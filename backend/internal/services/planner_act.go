package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"syndra/internal/access"
	"syndra/internal/clock"
	"syndra/internal/db"
	"syndra/internal/models"
)

// Single acts through the planner (architecture deepening, task 1.2;
// decisions 5, 9, 11). One person, one act: give or remove a direct grant,
// approve a request, assign or remove a bundle. The sheet asks for the Change
// before anything is written (PlanAct, the preview), says what it cancels
// before the press, and the write carries the fingerprint and the cancel ids
// the operator saw (Seen). ApplyAct plans again under the lock and writes
// only that Change, admitting exactly those cancellations; anything else is a
// StalePlan and nothing is written.

// The single acts on a role. A bundle's own assignment and removal use the
// bulk ops' names, and the bundle planner.
const (
	OpGive        = "give"
	OpRemoveGrant = "remove"
	OpApprove     = "approve"
)

// OpExtend moves the end of grants a person has (a ledger edit: Zitadel has
// no end dates, Syndra ends access on the day; settled 5 Nov). The bulk op
// and the single act share it.
const OpExtend = BulkOpExtend

func roleOp(op string) bool {
	return op == OpGive || op == OpRemoveGrant || op == OpApprove || op == BulkOpAssignRole || op == BulkOpRemoveRole
}

// SingleOp reports whether op is an act the planner plans for one person.
func SingleOp(op string) bool {
	return op == OpGive || op == OpRemoveGrant || op == OpApprove || op == OpExtend ||
		op == BulkOpAssignBundle || op == BulkOpRemoveBundle
}

// planRole is the planner for an act on one role: the same guards, closure,
// rule rows and delivery as a bundle's row (decision 6), and its own words.
func planRole(ch Change, a Action, s PlanSnapshot, profile models.UserProfile, person db.PersonAccess) Change {
	k := roleKey{projectID: a.ProjectID, roleKey: a.RoleKey}
	ch.ProjectID, ch.RoleKey = a.ProjectID, a.RoleKey
	role := s.words(k)
	before := baseFrom(person, "")
	after := baseFrom(person, "")
	var own *db.EnqueueParams
	var detail string

	var said []string
	switch a.Op {
	case OpGive, OpApprove, BulkOpAssignRole:
		if access.Left(profile.Status) {
			if a.Op == BulkOpAssignRole {
				return ch.decided(a, EffectBlocked, "departed:"+profile.Status,
					fmt.Sprintf("Account is %s. Remove it from the selection to continue.", strings.ToLower(profile.Status)), "")
			}
			return ch.decided(a, EffectBlocked, "departed:"+profile.Status,
				fmt.Sprintf("Their account is %s in Zitadel, so it can't be given access.", strings.ToLower(profile.Status)),
				"If they still need it, make the account active in Zitadel first.")
		}
		p := db.EnqueueParams{UserID: a.UserID, ProjectID: a.ProjectID, RoleKeys: []string{a.RoleKey},
			Source: "direct", OpType: "add", EndRule: db.EndKept}
		detail = fmt.Sprintf("Gains %s.", role)
		if a.Op == BulkOpAssignRole {
			// Giving is for people without the role (owner, 15 Oct); an end
			// that needs moving is an extension, its own operation. Judged
			// from the plan's one snapshot, not a read per person.
			if g := findGrant(person.Direct, a.ProjectID, a.RoleKey); g != nil && g.ExpiresAt == nil {
				return ch.decided(a, EffectNoChange, "held", "Already has this, with no end.", "")
			} else if g != nil {
				return ch.decided(a, EffectNoChange, "held",
					fmt.Sprintf("Already has this, until %s.", formatDay(*g.ExpiresAt)), "Extend it instead.")
			}
			if effectiveClosure(before, s.Access.Rules)[k] {
				return ch.decided(a, EffectNoChange, "held", "Already has this another way, through a bundle or an automatic rule.", "")
			}
			p.ExpiresAt = endAfter(s.Now, &a.Days)
			said = append(said, expiryLine(p.ExpiresAt))
		}
		if a.Op == OpApprove {
			if s.Request.Status != "pending" {
				return ch.decided(a, EffectNoChange, "resolved", SettledDetail(s.Request.Status), "")
			}
			plan, err := PlanApproval(s.Request, s.Holding, s.Now)
			switch {
			case errors.Is(err, ErrRequestAlreadyHeld):
				return ch.decided(a, EffectNoChange, "held", fmt.Sprintf("Already has %s.", role),
					"Decline it to close the request.")
			case errors.Is(err, ErrExtensionShortens):
				return ch.decided(a, EffectNoChange, "runs_past",
					fmt.Sprintf("Already has %s past the end this asks for.", role), "Decline it to close the request.")
			case err != nil:
				return ch.decided(a, EffectBlocked, "ask_ended", "The end date this asked for has already passed.",
					"Decline it, and they can ask again.")
			}
			ch.Approval = plan
			p.SourceRef, p.ExpiresAt, p.EndRule, p.Reason = s.Request.ID, plan.ExpiresAt, plan.EndRule, ApprovalReason(plan.Effect)
			switch plan.Effect {
			case EffectExtend:
				detail = fmt.Sprintf("Keeps %s %s.", role, EndPhrase(plan.ExpiresAt))
			case EffectGiveAgain:
				detail = fmt.Sprintf("Gains %s again.", role)
			}
		} else if a.Op == OpGive && s.Holding.Kind != models.HoldingNone {
			return ch.decided(a, EffectNoChange, "held", fmt.Sprintf("Already has %s.", role), "")
		}
		own = &p
		after[k] = true
	case OpRemoveGrant, BulkOpRemoveRole:
		if a.Op == BulkOpRemoveRole {
			// Removal in bulk is source-specific by design: it removes a direct
			// grant and nothing else. Holding the role only another way is
			// untouched, and saying so is the whole point.
			g := findGrant(person.Direct, a.ProjectID, a.RoleKey)
			switch {
			case g == nil && effectiveClosure(before, s.Access.Rules)[k]:
				return ch.decided(a, EffectNoChange, "not_held", "Holds no direct grant here.",
					"Keeps the role another way, through a bundle or an automatic rule. Remove that instead.")
			case g == nil:
				return ch.decided(a, EffectNoChange, "not_held", "Doesn't hold this role.", "")
			}
			a.GrantID = g.ID
			ch.Row.GrantIDs = []string{g.ID}
		}
		var kept []models.DirectGrant
		for _, g := range person.Direct {
			if g.ID == a.GrantID {
				// The role is the grant's: a removal names only the grant.
				k = roleKey{projectID: g.ProjectID, roleKey: g.RoleKey}
				role = s.words(k)
				continue
			}
			kept = append(kept, g)
		}
		if len(kept) == len(person.Direct) {
			return ch.decided(a, EffectNoChange, "not_held",
				"No direct grant of this to revoke. It may already have been removed or ended.", "")
		}
		without := person
		without.Direct = kept
		after = baseFrom(without, "")
	}

	ch.ProjectID, ch.RoleKey = k.projectID, k.roleKey
	rules := s.Access.Rules
	closureBefore, closureAfter := effectiveClosure(before, rules), effectiveClosure(after, rules)
	adds, revokes := closureDelta(closureBefore, closureAfter)
	refs := ruleRefs(before, after, closureBefore, closureAfter, rules, rules)

	// One guard for every give (decision 6): Zitadel still offers the role,
	// the act's own and every one a rule follows with.
	gives := adds
	if own != nil && !slices.Contains(gives, k) {
		gives = append([]roleKey{k}, gives...)
	}
	if len(gives) > 0 {
		if s.CatalogueUnread {
			return ch.decided(a, EffectBlocked, "catalogue_unread",
				"Syndra couldn't read Zitadel's list of roles to check this one exists. Try again shortly.", "")
		}
		var gone []roleKey
		for _, g := range gives {
			if _, offered := s.Catalogue[g]; !offered {
				gone = append(gone, g)
			}
		}
		if len(gone) > 0 {
			step := ""
			if gone[0] != k {
				step = fmt.Sprintf("Retire the automatic rule that gives %s, then try again.", s.them(gone))
			}
			return ch.decided(a, EffectNoChange, "not_offered:"+keysField(gone),
				fmt.Sprintf("This would give %s.", s.goneWords(gone)), step)
		}
	}

	var writes []db.EnqueueParams
	if own != nil {
		// The act's own row, then the rows the rules owe beyond it: what
		// db.FollowRules would write, decided here (ruleDelta, the same delta).
		ruleAdds, ruleRevokes, _, _ := ruleDelta(before, after, rules)
		writes = append([]db.EnqueueParams{*own}, asRuleRows(deltaWrites(a.UserID, ruleAdds, ruleRevokes, "rule", ""), refs)...)
	} else {
		writes = asRuleRows(deltaWrites(a.UserID, adds, revokes, "direct", a.GrantID), refs)
	}
	ch = ch.deliver(writes, person, s)

	for _, r := range adds {
		if r != k {
			ch.Rules = append(ch.Rules, fmt.Sprintf("Also gives %s, by the automatic rule from %s.", s.words(r), s.ruleSource(rules, closureAfter, r)))
		}
	}
	for _, r := range revokes {
		if r != k {
			ch.Rules = append(ch.Rules, fmt.Sprintf("Also loses %s, which the automatic rule from %s gave.", s.words(r), s.ruleSource(rules, closureBefore, r)))
		}
	}

	switch {
	case own != nil && ch.Writes[0].Delivery == DeliveryNone:
		detail = fmt.Sprintf("Cancels removing %s, which was waiting for you to send, so nothing is sent to Zitadel. They keep it.", role)
	case own == nil && len(ch.Writes) > 0 && ch.onlyCancels():
		detail = fmt.Sprintf("Cancels giving %s, which was waiting for you to send, so nothing is sent to Zitadel.", role)
	case own == nil && slices.Contains(revokes, k):
		detail = fmt.Sprintf("Loses %s.", role)
	case own == nil:
		detail = fmt.Sprintf("Keeps %s another way, so nothing is sent to Zitadel.", role)
	}
	return ch.said(a, s, detail, said)
}

// onlyCancels: every write cancels a queued change and nothing is sent.
func (ch Change) onlyCancels() bool {
	for _, w := range ch.Writes {
		if w.Delivery != DeliveryNone {
			return false
		}
	}
	return true
}

// Delivery is the act's delivery on the row (A-21 §6): waits for you when any
// write waits, none when every write only cancels, otherwise straight away.
// Empty when it doesn't go ahead or writes nothing.
func (ch Change) Delivery() string {
	if ch.Row.Effect != EffectApply || len(ch.Writes) == 0 {
		return ""
	}
	if len(ch.waiting()) > 0 {
		return DeliveryWaits
	}
	if ch.onlyCancels() {
		return DeliveryNone
	}
	return DeliveryStraightAway
}

// --- reading one person's snapshot, and the act ---

// Seen is what the operator was shown: the Change's fingerprint and the
// queued changes it said it cancels. A write that carries it is applied only
// if a fresh plan is exactly that Change (decision 3), and admits only those
// cancellations (decision 4).
type Seen struct {
	Fingerprint string
	CancelIDs   []string
}

// ActWrite is what the write records beside the Change: who, why, and for a
// give the end and the request body it keeps.
type ActWrite struct {
	Actor, Reason string
	// Note is a request decision's review note.
	Note string
	// Give only: the end the operator chose, and the body as sent.
	Give db.EnqueueParams
}

// Refused is an act the planner stopped: the Change says why (Refusal, and
// the row's words). Nothing was written.
type Refused struct{ Change Change }

func (e *Refused) Error() string { return e.Change.Row.Detail }

// Code is the refusal's code without its detail ("not_offered", "departed").
func (e *Refused) Code() string { code, _, _ := strings.Cut(e.Change.Refusal, ":"); return code }

var (
	svcGetAccessRequestByID     = db.GetAccessRequestByID
	svcRequestHolding           = db.RequestHolding
	svcApproveRequestAndEnqueue = db.ApproveRequestAndEnqueue
	svcEnqueueDirectGrant       = db.EnqueueDirectGrantPropagation
	svcResolveAccessRequest     = db.ResolveAccessRequest
	svcGetDriftItem             = db.GetDriftItem
	svcAttributeDriftTx         = db.AttributeDriftTx
	svcRevokeDriftAndEnqueue    = db.RevokeDriftAndEnqueue
)

// actDirectory is what the act reads from Zitadel, before the lock: the
// person and the catalogue. A person the directory can't answer for is still
// planned, with no name and no status: a single act never refused anyone for
// the directory being slow, and a removal must work for an account that has
// gone.
func actDirectory(ctx context.Context, a Action) PlanSnapshot {
	var s PlanSnapshot
	profile, found, err := svcFindUser(ctx, a.UserID)
	if err != nil || !found {
		profile = models.UserProfile{ID: a.UserID}
	}
	s.Profiles = map[string]models.UserProfile{a.UserID: profile}
	projects, err := PlannerCatalogue(ctx)
	if err != nil {
		s.CatalogueUnread = true
		return s
	}
	s.Catalogue = map[roleKey]roleWords{}
	for _, p := range projects {
		for _, r := range p.Roles {
			s.Catalogue[roleKey{projectID: p.ID, roleKey: r.Key}] = roleWords{Role: r.Label, Project: p.Name}
		}
	}
	return s
}

// actSnapshot adds what Syndra's own tables say, under the lock the caller
// holds. An approval reads its request again here: another reviewer may have
// decided it since.
func actSnapshot(ctx context.Context, a Action, s PlanSnapshot) (PlanSnapshot, error) {
	var err error
	if a.Op == BulkOpAssignBundle || a.Op == BulkOpRemoveBundle || a.DriftID != "" {
		s, _, err = planSnapshot(ctx, BulkRequest{Op: a.Op, BundleID: a.BundleID}, s, []string{a.subject()})
		return s, err
	}
	s.Now = clock.Now()
	if a.Op == OpApprove {
		if s.Request, err = svcGetAccessRequestByID(ctx, a.RequestID); err != nil {
			return s, fmt.Errorf("%w: %v", ErrRequestNotFound, err)
		}
	}
	if a.Op == OpGive || a.Op == OpApprove {
		if s.Holding, err = svcRequestHolding(ctx, a.UserID, a.ProjectID, a.RoleKey); err != nil {
			return s, err
		}
	}
	s.Access, err = svcReadAccessSnapshot(ctx, []string{a.UserID})
	return s, err
}

// resolveAction fills an approval's person and role from its request, and a
// removal's role from the grant, so the act names what it is about.
func resolveAction(ctx context.Context, a Action) (Action, error) {
	if a.DriftID != "" {
		// The person behind the drift row (an adoption or a revoke), for the directory; a row that has
		// gone is the plan's to say ("No longer in the queue").
		if item, err := svcGetDriftItem(ctx, a.DriftID); err == nil {
			a.UserID = item.UserID
		}
		return a, nil
	}
	if a.Op != OpApprove {
		return a, nil
	}
	r, err := svcGetAccessRequestByID(ctx, a.RequestID)
	if err != nil {
		return a, fmt.Errorf("%w: %v", ErrRequestNotFound, err)
	}
	if a.UserID != "" && a.UserID != r.RequesterID {
		return a, ErrRequestNotFound
	}
	a.UserID, a.ProjectID, a.RoleKey = r.RequesterID, r.ProjectID, r.RoleKey
	return a, nil
}

// PlanAct is the preview: one person's snapshot read under the lock, and the
// planner's Change for the act. Nothing is written.
func PlanAct(ctx context.Context, a Action) (Change, error) {
	a, err := resolveAction(ctx, a)
	if err != nil {
		return Change{}, err
	}
	snap := actDirectory(ctx, a)
	if err := svcInTxLockingAccess(ctx, func(ctx context.Context) error {
		snap, err = actSnapshot(ctx, a, snap)
		return err
	}); err != nil {
		return Change{}, err
	}
	return Plan(a, snap), nil
}

// ApplyAct is the act. Under the lock it reads the snapshot again and plans
// again; with seen, it refuses with nothing written (StalePlan) unless the
// Change is the one the operator saw, and admits only the cancellations they
// saw. Without seen (an older client, a bulk decision that reviewed its own
// rows) nothing is admitted beyond what the request's gate already allows.
// It returns the Change as written, each write with its outbox row: sending
// it is the caller's (DeliverChange), after the commit.
func ApplyAct(ctx context.Context, a Action, w ActWrite, seen *Seen) (Change, error) {
	a, err := resolveAction(ctx, a)
	if err != nil {
		return Change{}, err
	}
	fromZitadel := actDirectory(ctx, a)
	var ch Change
	err = withLockedAccess(ctx, func(ctx context.Context) error {
		snap, err := actSnapshot(ctx, a, fromZitadel)
		if err != nil {
			return err
		}
		ch = Plan(a, snap)
		if seen != nil && (ch.Row.Fingerprint != seen.Fingerprint || !slices.Equal(ch.Row.CancelIDs, seen.CancelIDs)) {
			return &StalePlan{IDs: []string{a.UserID}}
		}
		if ch.Row.Effect != EffectApply {
			return &Refused{Change: ch}
		}
		var named []string
		if seen != nil {
			named = seen.CancelIDs
		}
		db.CancellationsNamed(ctx, named)
		ch, err = writeAct(ctx, a, w, ch, snap)
		return err
	})
	if err != nil {
		return Change{}, err
	}
	return ch, nil
}

// writeAct writes one act's Change in the caller's transaction and returns it
// with each write's outbox row.
func writeAct(ctx context.Context, a Action, w ActWrite, ch Change, snap PlanSnapshot) (Change, error) {
	switch a.Op {
	case BulkOpAssignBundle, BulkOpRemoveBundle:
		return writeChange(ctx, BulkRequest{Op: a.Op, Actor: w.Actor, Reason: w.Reason, BundleID: a.BundleID}, ch)
	case OpExtend:
		return writeExtend(ctx, w, ch)
	case OpAdopt:
		return writeAdopt(ctx, a, w, ch)
	case OpRevoke:
		return writeRevoke(ctx, a, w, ch)
	}
	if err := deferTargetConvergence(ctx, w.Actor, a.UserID, ch.changed()); err != nil {
		return ch, err
	}
	params := ch.params(w.Actor, w.Reason)
	var ids []string
	switch a.Op {
	case OpRemoveGrant, BulkOpRemoveRole:
		// The operator's words or none: the audit line and Pending changes
		// quote it (task 1.8a), so a label of Syndra's would read as theirs.
		grantID := a.GrantID
		if a.Op == BulkOpRemoveRole {
			grantID = ch.Row.GrantIDs[0]
		}
		var err error
		if ids, err = svcDeleteDirectGrantAndEnqueue(ctx, w.Actor, a.UserID, grantID, w.Reason, params); err != nil {
			return ch, err
		}
	case OpGive, OpApprove, BulkOpAssignRole:
		// The act's own write keeps what the planner and the operator set on
		// it; the rule rows are FollowRules' shape, decided by the planner.
		own := ch.Writes[0].EnqueueParams
		own.GrantedBy = w.Actor
		for i := range params[1:] {
			params[1+i].Reason = "Mapping rule cascade"
		}
		var res db.EnqueueResult
		var err error
		switch a.Op {
		case OpGive:
			own.Reason, own.ExpiresAt, own.EndRule, own.PayloadJSON = w.Reason, w.Give.ExpiresAt, w.Give.EndRule, w.Give.PayloadJSON
			res, err = svcEnqueueDirectGrant(ctx, own, params[1:]...)
		case BulkOpAssignRole:
			// The end is the plan's (from the snapshot's day).
			payload, _ := json.Marshal(map[string]any{"project_id": a.ProjectID, "role_key": a.RoleKey,
				"reason": w.Reason, "duration_days": a.Days, "bulk": true})
			own.Reason, own.PayloadJSON = w.Reason, string(payload)
			res, err = svcEnqueueDirectGrant(ctx, own, params[1:]...)
		default:
			payload, _ := json.Marshal(map[string]string{"user_id": a.UserID, "project_id": a.ProjectID, "role_key": a.RoleKey, "from_request": a.RequestID})
			own.PayloadJSON = string(payload)
			var endedBefore, endsAfter *time.Time
			if snap.Request.Kind == RequestKindExtend {
				endedBefore, endsAfter = HoldingEnd(snap.Holding), ch.Approval.ExpiresAt
			}
			res, err = svcApproveRequestAndEnqueue(ctx, a.RequestID, w.Actor, w.Note, own, endedBefore, endsAfter, params[1:]...)
			if errors.Is(err, db.ErrRequestNotPending) {
				err = ErrRequestAlreadyResolved
			}
		}
		if err != nil {
			return ch, err
		}
		ids = res.OutboxIDs()
	}
	return ch.written(ids)
}
