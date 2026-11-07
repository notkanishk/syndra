package services

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"

	"syndra/internal/access"
	"syndra/internal/db"
	"syndra/internal/directory"
	"syndra/internal/models"
)

// The access-change planner (openspec/changes/architecture-deepening,
// candidate 1). One pure function decides what an access change does, from
// one snapshot read under the access-mutation lock: the writes it sends, how
// each is delivered, the queued changes it cancels, a refusal when a guard
// fails, and the words of the plan row. The preview renders that Change; the
// apply reads the snapshot again under the lock, plans again, and writes only
// when the Change is the one the operator reviewed. A preview computed by
// different code from the write it previews is a preview of nothing.
//
// First slice: bulk assign and remove bundle. Every other action still goes
// its old way (tasks.md, candidate 1).

// Action is one person's part of an access change.
type Action struct {
	Op       string
	UserID   string
	BundleID string
}

// How a write reaches Zitadel (decision 11): sent in the same request, held
// in Pending changes because its role waits for you, or not sent at all
// because it cancels a queued change and nothing is left to send.
const (
	DeliveryStraightAway = "straight_away"
	DeliveryWaits        = "waits_for_you"
	DeliveryNone         = "none"
)

// Write is one single-role outbox write, with how it is delivered.
type Write struct {
	db.EnqueueParams
	Delivery   string
	WaitReason string
}

// Change is what the planner decided for one action.
type Change struct {
	UserID string
	// Writes are every write the apply hands the outbox, in order. Those with
	// DeliveryNone cancel a queued change and leave no outbox row.
	Writes []Write
	// Refusal is the guard that stopped the action ("held", "not_offered",
	// "departed", ...), empty when it goes ahead. The row says why in words.
	Refusal string
	// Cancels are the queued changes the writes cancel, said on the row and
	// stored on the plan: the apply admits these and no other.
	Cancels []models.QueuedChange
	// VersionID is the bundle version an assignment pins.
	VersionID string
	// Row is the plan row: its words, effect, cancel ids and fingerprint. The
	// fingerprint is the Change's identity (its writes and cancel ids, and the
	// decision around them; see decided): the apply refuses when it differs.
	Row BulkOutcome
}

// PlanSnapshot is everything the planner reads, as plain data.
type PlanSnapshot struct {
	Bundle       models.Bundle
	Version      models.BundleVersion
	VersionRoles []models.BundleRole
	// Catalogue is the roles Zitadel offers, with their words. Unread means
	// Syndra couldn't read it: a give is refused rather than guessed.
	Catalogue       map[roleKey]roleWords
	CatalogueUnread bool
	// Profiles is the directory's people; absent means no such account.
	Profiles map[string]models.UserProfile
	Access   db.AccessSnapshot
}

type roleWords struct{ Role, Project string }

// Plan decides one action against the snapshot. Pure: no database, no
// directory, no clock.
func Plan(a Action, s PlanSnapshot) Change {
	ch := Change{UserID: a.UserID, Row: BulkOutcome{UserID: a.UserID}}
	profile, known := s.Profiles[a.UserID]
	if !known {
		return ch.decided(a, EffectBlocked, "absent", "No such account in the directory.", "")
	}
	ch.Row.Name, ch.Row.Email = profile.Name, profile.Email
	person := s.Access.People[a.UserID]
	bundle := displayBundle(s.Bundle.Name)
	holds := person.Bundles[a.BundleID]

	var before, after map[roleKey]bool
	switch a.Op {
	case BulkOpAssignBundle:
		if access.Left(profile.Status) {
			return ch.decided(a, EffectBlocked, "departed:"+profile.Status,
				fmt.Sprintf("Account is %s. Remove it from the selection to continue.", strings.ToLower(profile.Status)), "")
		}
		if s.Bundle.RetiredAt != nil {
			return ch.decided(a, EffectBlocked, "retired",
				fmt.Sprintf("%s was retired on %s, so nobody can join it.", capitalise(bundle), formatDay(*s.Bundle.RetiredAt)), "")
		}
		if holds {
			return ch.decided(a, EffectNoChange, "held", fmt.Sprintf("Already in %s.", bundle), "")
		}
		before = baseFrom(person, "")
		after = maps.Clone(before)
		for _, r := range s.VersionRoles {
			after[roleKey{projectID: r.ProjectID, roleKey: r.RoleKey}] = true
		}
		ch.VersionID = s.Version.ID
	case BulkOpRemoveBundle:
		if !holds {
			return ch.decided(a, EffectNoChange, "not_held", fmt.Sprintf("Not in %s.", bundle), "")
		}
		if s.Bundle.RetiredAt != nil {
			return ch.decided(a, EffectBlocked, "retired",
				fmt.Sprintf("%s was retired on %s, so it can't change.", capitalise(bundle), formatDay(*s.Bundle.RetiredAt)), "")
		}
		before = baseFrom(person, "")
		after = baseFrom(person, a.BundleID)
	default:
		return ch.decided(a, EffectBlocked, "unsupported", "Unsupported operation.", "")
	}

	rules := s.Access.Rules
	closureBefore, closureAfter := effectiveClosure(before, rules), effectiveClosure(after, rules)
	adds, revokes := closureDelta(closureBefore, closureAfter)

	// One guard for every give (decision 6): Zitadel still offers the role.
	if len(adds) > 0 {
		if s.CatalogueUnread {
			return ch.decided(a, EffectBlocked, "catalogue_unread",
				"Syndra couldn't read Zitadel's list of roles to check the ones this gives still exist. Try again shortly.", "")
		}
		var gone []roleKey
		for _, k := range adds {
			if _, offered := s.Catalogue[k]; !offered {
				gone = append(gone, k)
			}
		}
		if len(gone) > 0 {
			step := fmt.Sprintf("Take it out of %s and publish, then give the bundle again.", bundle)
			if !bundleGives(s.VersionRoles, gone[0]) {
				step = "Retire the automatic rule that gives it, then give the bundle again."
			}
			return ch.decided(a, EffectNoChange, "not_offered:"+keysField(gone),
				fmt.Sprintf("This would give %s, which no longer exists in Zitadel.", s.list(gone)), step)
		}
	}

	for _, p := range deltaWrites(a.UserID, adds, revokes, "bundle", a.BundleID) {
		w := Write{EnqueueParams: p, Delivery: DeliveryStraightAway}
		ref := db.RoleRef{ProjectID: p.ProjectID, RoleKey: p.RoleKeys[0]}
		answered := db.AnsweredBy(p, person.Queued)
		ch.Cancels = append(ch.Cancels, answered...)
		switch {
		case len(answered) > 0 && !db.OwedAfterCancel(p, person.Standing[ref]):
			w.Delivery = DeliveryNone
		case s.Access.RoleWaits[ref]:
			w.Delivery, w.WaitReason = DeliveryWaits, db.WaitReasonRoleWaits
		}
		ch.Writes = append(ch.Writes, w)
	}

	var detail string
	var said []string
	if a.Op == BulkOpAssignBundle {
		detail = fmt.Sprintf("Joins %s.", bundle)
		own, byRule := splitByBundle(adds, s.VersionRoles)
		switch {
		case len(own) > 0:
			said = append(said, fmt.Sprintf("Gains %s.", s.list(own)))
		case len(byRule) == 0:
			said = append(said, "Already holds every role it gives, so nothing is sent to Zitadel.")
		}
		for _, k := range byRule {
			said = append(said, fmt.Sprintf("Also gives %s, by the automatic rule from %s.", s.words(k), s.ruleSource(rules, closureAfter, k)))
		}
	} else {
		detail = fmt.Sprintf("Leaves %s.", bundle)
		pinned := person.ByBundle[a.BundleID]
		own, byRule := splitByBundle(revokes, pinned)
		var kept []roleKey
		for _, r := range pinned {
			if k := (roleKey{projectID: r.ProjectID, roleKey: r.RoleKey}); closureAfter[k] {
				kept = append(kept, k)
			}
		}
		switch {
		case len(revokes) == 0 && len(pinned) == 0:
			said = append(said, "It gave them no roles, so nothing is sent to Zitadel.")
		case len(revokes) == 0:
			said = append(said, "Keeps every role it gave another way, so nothing is sent to Zitadel.")
		default:
			if len(own) > 0 {
				said = append(said, fmt.Sprintf("Loses %s.", s.list(own)))
			}
			if len(kept) > 0 {
				said = append(said, fmt.Sprintf("Keeps %s another way.", s.list(kept)))
			}
		}
		for _, k := range byRule {
			said = append(said, fmt.Sprintf("Also loses %s, which the automatic rule from %s gave.", s.words(k), s.ruleSource(rules, closureBefore, k)))
		}
	}
	var waiting []roleKey
	for _, w := range ch.Writes {
		if w.Delivery == DeliveryWaits {
			waiting = append(waiting, roleKey{projectID: w.ProjectID, roleKey: w.RoleKeys[0]})
		}
	}
	if len(waiting) > 0 {
		verb := "Giving"
		if a.Op == BulkOpRemoveBundle {
			verb = "Removing"
		}
		said = append(said, fmt.Sprintf("%s %s waits for you to send.", verb, s.list(waiting)))
	}
	// The cancels line stays last: the plan renderer splits it off the end.
	if line := DescribeCancels(ch.Cancels); line != "" {
		said = append(said, line)
	}
	return ch.decided(a, EffectApply, "", detail, strings.Join(said, " "))
}

// decided fills the row and its fingerprint: the decision, the writes and the
// cancel ids. Words are left out, so renaming a bundle or a role does not
// make a plan stale; what would change what happens does.
func (ch Change) decided(a Action, effect, refusal, detail, consequence string) Change {
	ch.Refusal = refusal
	ch.Row.Effect, ch.Row.Detail, ch.Row.Consequence = effect, detail, consequence
	fields := []string{"planner/1", a.Op, a.UserID, a.BundleID, effect, refusal, ch.VersionID}
	for _, w := range ch.Writes {
		fields = append(fields, w.OpType, w.ProjectID, w.RoleKeys[0], w.Source, w.SourceRef, w.Delivery)
	}
	fields = append(fields, "cancels")
	ch.Row.CancelIDs = nil
	for _, c := range ch.Cancels {
		fields = append(fields, c.ID)
		ch.Row.CancelIDs = append(ch.Row.CancelIDs, c.ID)
	}
	ch.Row.Fingerprint = Fingerprint(fields...)
	return ch
}

// params are the writes as the outbox takes them, every one of them: a write
// that only cancels still has to reach the one place cancellations happen.
func (ch Change) params(actor, reason string) []db.EnqueueParams {
	out := make([]db.EnqueueParams, len(ch.Writes))
	for i, w := range ch.Writes {
		p := w.EnqueueParams
		p.GrantedBy, p.Reason, p.PayloadJSON = actor, reason, "{}"
		out[i] = p
	}
	return out
}

func (ch Change) changed() []roleKey {
	out := make([]roleKey, len(ch.Writes))
	for i, w := range ch.Writes {
		out[i] = roleKey{projectID: w.ProjectID, roleKey: w.RoleKeys[0]}
	}
	return out
}

func bundleGives(roles []models.BundleRole, k roleKey) bool {
	for _, r := range roles {
		if r.ProjectID == k.projectID && r.RoleKey == k.roleKey {
			return true
		}
	}
	return false
}

// splitByBundle parts roles into the bundle's own and the ones a rule follows with.
func splitByBundle(ks []roleKey, roles []models.BundleRole) (own, byRule []roleKey) {
	for _, k := range ks {
		if bundleGives(roles, k) {
			own = append(own, k)
		} else {
			byRule = append(byRule, k)
		}
	}
	return own, byRule
}

// words names a role as the catalogue does ("Laser cutter in Workshop"),
// falling back to its keys: a plan must never fail for want of a label.
func (s PlanSnapshot) words(k roleKey) string {
	w, ok := s.Catalogue[k]
	role, project := k.roleKey, k.projectID
	if ok && w.Role != "" {
		role = w.Role
	}
	if ok && w.Project != "" {
		project = w.Project
	}
	return role + " in " + project
}

// list names roles alphabetically, case-insensitively: "A, B and C".
func (s PlanSnapshot) list(ks []roleKey) string {
	names := make([]string, len(ks))
	for i, k := range ks {
		names[i] = s.words(k)
	}
	sort.SliceStable(names, func(i, j int) bool { return strings.ToLower(names[i]) < strings.ToLower(names[j]) })
	if len(names) == 1 {
		return names[0]
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

func (s PlanSnapshot) ruleSource(rules []models.MappingRule, closure map[roleKey]bool, k roleKey) string {
	if r := ruleReachingRule(rules, closure, k); r != nil {
		return s.words(roleKey{projectID: r.SourceProject, roleKey: r.SourceRole})
	}
	return "another role"
}

func keysField(ks []roleKey) string {
	parts := make([]string, len(ks))
	for i, k := range ks {
		parts[i] = k.projectID + "/" + k.roleKey
	}
	return strings.Join(parts, ",")
}

func capitalise(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// --- reading the snapshot, and the two acts that use it ---

// PlannedOp reports whether an operation goes through the planner yet.
func PlannedOp(op string) bool { return op == BulkOpAssignBundle || op == BulkOpRemoveBundle }

// PlannerCatalogue is Zitadel's catalogue as the role-offered guard reads it.
// handlers wires the same reader its own check uses, so a demo catalogue
// served while Zitadel is configured counts as unread there and here.
var PlannerCatalogue = func(ctx context.Context) ([]models.ProjectCatalog, error) { return directory.Default.Projects(ctx) }

var (
	svcReadAccessSnapshot = db.ReadAccessSnapshot
	svcClaimPlanJoined    = db.ClaimPlanJoined
)

// planDirectory is what the plan reads from Zitadel: the people and the
// catalogue. Read before the lock, which is never held across a network call.
func planDirectory(ctx context.Context, ids []string) (PlanSnapshot, error) {
	var s PlanSnapshot
	users, err := svcDirectoryUsers(ctx)
	if err != nil {
		return s, err
	}
	s.Profiles = make(map[string]models.UserProfile, len(ids))
	wanted := make(map[string]bool, len(ids))
	for _, id := range ids {
		wanted[id] = true
	}
	for _, u := range users {
		if wanted[u.ID] {
			s.Profiles[u.ID] = u
		}
	}
	for _, id := range ids {
		if _, ok := s.Profiles[id]; ok {
			continue
		}
		// The list is cached; a person created since its last fill is a real
		// person, not a bad id.
		if p, found, err := svcFindUser(ctx, id); err == nil && found {
			s.Profiles[id] = p
		}
	}
	projects, err := PlannerCatalogue(ctx)
	if err != nil {
		s.CatalogueUnread = true
		return s, nil
	}
	s.Catalogue = map[roleKey]roleWords{}
	for _, p := range projects {
		for _, r := range p.Roles {
			s.Catalogue[roleKey{projectID: p.ID, roleKey: r.Key}] = roleWords{Role: r.Label, Project: p.Name}
		}
	}
	return s, nil
}

// planSnapshot adds what Syndra's own tables say, read under the lock the
// caller holds: one pass for everyone in the plan.
func planSnapshot(ctx context.Context, req BulkRequest, s PlanSnapshot, ids []string) (PlanSnapshot, error) {
	var err error
	if s.Bundle, err = svcGetBundleByID(ctx, req.BundleID); err != nil {
		return s, err
	}
	if req.Op == BulkOpAssignBundle && s.Bundle.RetiredAt == nil {
		// The LATEST PUBLISHED version: what an assignment pins and gives.
		if s.Version, s.VersionRoles, err = svcLatestVersionRoles(ctx, req.BundleID); err != nil {
			return s, err
		}
	}
	s.Access, err = svcReadAccessSnapshot(ctx, ids)
	return s, err
}

// rehearsePlanned is the preview: the snapshot read once under the lock, and
// the planner's row for every selected person.
func rehearsePlanned(ctx context.Context, req BulkRequest) (BulkPlan, error) {
	changes, excluded, err := PlanBulk(ctx, req)
	if err != nil {
		return BulkPlan{}, err
	}
	plan := BulkPlan{Op: req.Op, Excluded: excluded}
	for _, ch := range changes {
		plan.Outcomes = append(plan.Outcomes, ch.Row)
	}
	sortOutcomes(plan.Outcomes)
	plan.Summary = summarize(plan.Outcomes)
	return plan, nil
}

// PlanBulk reads the snapshot once under the lock and plans every selected
// person, in the order selected. An operation that takes access away leaves
// out the person asking (#32, B4): their own page is where they change it.
func PlanBulk(ctx context.Context, req BulkRequest) ([]Change, []BulkExclusion, error) {
	ids := dedupeIDs(req.UserIDs)
	snap, err := planDirectory(ctx, ids)
	if err != nil {
		return nil, nil, err
	}
	if err := svcInTxLockingAccess(ctx, func(ctx context.Context) error {
		snap, err = planSnapshot(ctx, req, snap, ids)
		return err
	}); err != nil {
		return nil, nil, err
	}
	var changes []Change
	var excluded []BulkExclusion
	for _, uid := range ids {
		if req.Op == BulkOpRemoveBundle && req.Actor != "" && uid == req.Actor {
			excluded = append(excluded, BulkExclusion{UserID: uid, Reason: "self"})
			continue
		}
		changes = append(changes, Plan(Action{Op: req.Op, UserID: uid, BundleID: req.BundleID}, snap))
	}
	return changes, excluded, nil
}

// StalePlan is the apply's refusal when the fresh read gives a different
// Change for these people than the plan they reviewed: other writes, or a
// queued change the row never named (decisions 3, 4). Nothing was written and
// the plan is still unspent.
type StalePlan struct{ IDs []string }

func (e *StalePlan) Error() string {
	return fmt.Sprintf("the plan is stale for %d subject(s)", len(e.IDs))
}

// ApplyPlannedBulk is the apply. In one transaction under the lock it spends
// the plan, reads the snapshot again, plans every row again, and refuses the
// whole plan, unspent and with nothing written, if any row's Change differs
// from the one stored. Otherwise it writes every row, admitting only the
// cancellations the rows named. It returns the rows' words and, per person,
// the outbox rows to deliver: sending is the caller's, after the commit,
// because the lock is never held across a call to Zitadel.
func ApplyPlannedBulk(ctx context.Context, req BulkRequest, cite db.PlanCitation) (BulkPlan, map[string][]string, error) {
	plan := BulkPlan{Op: req.Op, Applied: true}
	sent := map[string][]string{}
	var subjects []db.PlanSubject
	var changes map[string]Change

	// The plan's people are the request's (the request fingerprint binds
	// them), so the directory is read before the lock like the preview's.
	fromZitadel, err := planDirectory(ctx, dedupeIDs(req.UserIDs))
	if err != nil {
		return BulkPlan{}, nil, err
	}
	err = withLockedAccess(ctx, func(ctx context.Context) error {
		var err error
		if _, subjects, err = svcClaimPlanJoined(ctx, cite); err != nil {
			return err
		}
		ids := make([]string, len(subjects))
		for i, s := range subjects {
			ids[i] = s.SubjectID
		}
		snap, err := planSnapshot(ctx, req, fromZitadel, ids)
		if err != nil {
			return err
		}
		changes = make(map[string]Change, len(subjects))
		var moved, named []string
		for _, s := range subjects {
			ch := Plan(Action{Op: req.Op, UserID: s.SubjectID, BundleID: req.BundleID}, snap)
			if ch.Row.Fingerprint != s.Fingerprint || !slices.Equal(ch.Row.CancelIDs, s.Outcome.CancelIDs) {
				moved = append(moved, s.SubjectID)
				continue
			}
			changes[s.SubjectID] = ch
			named = append(named, s.Outcome.CancelIDs...)
		}
		if len(moved) > 0 {
			sort.Strings(moved)
			return &StalePlan{IDs: moved}
		}
		db.CancellationsNamed(ctx, named)

		for _, s := range subjects {
			ch := changes[s.SubjectID]
			if ch.Row.Effect != EffectApply {
				continue
			}
			ids, err := writeChange(ctx, req, ch)
			if err != nil {
				return err
			}
			sent[s.SubjectID] = ids
		}
		return nil
	})
	if err != nil {
		return BulkPlan{}, nil, err
	}
	for _, s := range subjects {
		plan.Outcomes = append(plan.Outcomes, changes[s.SubjectID].Row)
	}
	if req.Op == BulkOpRemoveBundle && req.Actor != "" && slices.Contains(dedupeIDs(req.UserIDs), req.Actor) {
		plan.Excluded = []BulkExclusion{{UserID: req.Actor, Reason: "self"}}
	}
	sortOutcomes(plan.Outcomes)
	plan.Summary = summarize(plan.Outcomes)
	return plan, sent, nil
}

// errNotAssigned: the assignment found them already holding the bundle, which
// the plan just checked under the same lock. Unreachable; refused, not trusted.
var errNotAssigned = errors.New("they already hold this bundle, which the plan said they didn't")

// writeChange writes one row's Change: the bundle assignment or its removal,
// its audit row and its outbox rows, in the caller's transaction.
func writeChange(ctx context.Context, req BulkRequest, ch Change) ([]string, error) {
	if err := deferTargetConvergence(ctx, req.Actor, ch.UserID, ch.changed()); err != nil {
		return nil, err
	}
	switch req.Op {
	case BulkOpAssignBundle:
		ids, assigned, err := svcAssignBundleAndEnqueue(ctx, req.Actor, ch.UserID, req.BundleID, ch.VersionID,
			ch.params(req.Actor, "Bundle membership cascade"))
		if err == nil && !assigned {
			err = errNotAssigned
		}
		return ids, err
	case BulkOpRemoveBundle:
		return svcRemoveBundleFromUserAndEnqueue(ctx, req.Actor, ch.UserID, req.BundleID,
			ch.params(req.Actor, "Bundle removal cascade"))
	}
	return nil, fmt.Errorf("the planner does not apply %q", req.Op)
}
