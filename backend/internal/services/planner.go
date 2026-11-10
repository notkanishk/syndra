package services

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

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
// Planned: every operator action that changes access. The single acts on one
// person (give, remove a direct grant, approve, extend, adopt, a person's
// bundles; planner_act.go, planner_ops.go) and every bulk op (give, remove,
// extend, assign and remove bundle, request decisions, adopting drift,
// publishing and moving a bundle's holders; planner_bulk.go). System work
// (expiry, the welcome bundle, a rule's own edit) is task 1.9.

// Action is one person's part of an access change.
type Action struct {
	Op       string
	UserID   string
	BundleID string
	// ProjectID and RoleKey are the role a give or an approval gives.
	ProjectID, RoleKey string
	// GrantID is the direct grant a removal takes back.
	GrantID string
	// RequestID is the request an approval decides.
	RequestID string
	// DriftID is the drift row an adoption resolves.
	DriftID string
	// Days is how long a give lasts, or an extension runs, from the day the
	// snapshot was read (clock.Now); 0 is no end.
	Days int
	// Alongside names roles (comma separated, so an Action stays comparable), in ProjectID, that the same change gives first
	// (a Zitadel › Users replace gives the new roles, then takes the old
	// away). A removal or revoke is planned as if they were already held, so
	// a role the old and new roles' rules both reach stays held and is never
	// revoked and given in one change. Applied after the gives are written
	// they are already held and this adds nothing.
	Alongside string
}

// withAlongside is the person as the act sees them: holding what the same
// change gives first.
func (a Action) withAlongside(p db.PersonAccess) db.PersonAccess {
	for _, r := range strings.Split(a.Alongside, ",") {
		if r == "" {
			continue
		}
		if findGrant(p.Direct, a.ProjectID, r) == nil {
			p.Direct = append(slices.Clone(p.Direct), models.DirectGrant{ID: "alongside:" + r, UserID: a.UserID, ProjectID: a.ProjectID, RoleKey: r})
		}
	}
	return p
}

// subject is the plan row an action is: the person, or for a request
// decision and an adoption the request and the drift row they resolve.
func (a Action) subject() string {
	switch a.Op {
	case OpApprove:
		return a.RequestID
	case OpAdopt:
		return a.DriftID
	}
	if a.DriftID != "" {
		return a.DriftID // a drift row's revoke
	}
	return a.UserID
}

// How a write reaches Zitadel (decision 11): sent in the same request, held
// in Pending changes because its role waits for you, or not sent at all
// because it cancels a queued change and nothing is left to send.
const (
	DeliveryStraightAway = "straight_away"
	DeliveryWaits        = "waits_for_you"
	DeliveryNone         = "none"
)

// Write is one single-role outbox write, with how it is delivered. A write
// that waits carries its reason (EnqueueParams.WaitReason) onto its row.
type Write struct {
	db.EnqueueParams
	Delivery string
	// OutboxID is the row the apply wrote for it; none for DeliveryNone.
	OutboxID string
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
	// Approval is what approving a request does (given, extended, given
	// again), with the end it sets.
	Approval ApprovalPlan
	// Rules are the row's sentences for what the automatic rules add or take
	// away with it ("Also gives …, by the automatic rule from …"): the one
	// source of those words (A-21 §4), said apart for the single acts.
	Rules []string
	// Said is the row's sentences after Detail, without the cancels line,
	// which a single act says in its own step.
	Said []string
	// Waits names the roles whose write waits for you to send, and Goes the
	// ones sent straight away, alphabetical.
	Waits, Goes []string
	// ProjectID and RoleKey are a single act's own role (for a removal, the
	// grant's).
	ProjectID, RoleKey string
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
	// Holding is what a give or an approval's person holds of its role
	// (db.RequestHolding), read under the lock with the rest.
	Holding models.RequestHolding
	// Request is the request an approval decides, read under the lock.
	Request models.AccessRequest
	// A bulk decision's requests and what each requester holds of the role,
	// and an adoption's drift rows, by id: read under the lock with the rest.
	Requests map[string]models.AccessRequest
	Holdings map[string]models.RequestHolding
	Drift    map[string]models.DriftItem
	// Holders is who holds the bundle a publish or move repins, by person,
	// with the version each is on.
	Holders map[string]models.BundleHolder
	// Ticked narrows an extension to the grants the operator ticked
	// (Expiring access selects grant rows, not people); nil is every one.
	Ticked map[string]bool
	// LeaveHolders: a publish that leaves its holders where they are.
	LeaveHolders bool
	// Draft is what a publish cuts: the next version and what it gives.
	Draft DraftDiff
	// Now is when the snapshot was read: the day an approval's end counts from.
	Now time.Time
}

type roleWords struct{ Role, Project string }

// Plan decides one action against the snapshot. Pure: no database, no
// directory, no clock.
func Plan(a Action, s PlanSnapshot) Change {
	a, s, refused := s.resolve(a)
	ch := Change{UserID: a.UserID, Row: BulkOutcome{UserID: a.subject()}}
	if a.subject() != a.UserID {
		ch.Row.Person = a.UserID
	}
	profile, known := s.Profiles[a.UserID]
	if known {
		ch.Row.Name, ch.Row.Email = profile.Name, profile.Email
	}
	if refused != nil {
		return ch.decided(a, refused.effect, refused.code, refused.detail, refused.step)
	}
	if !known {
		return ch.decided(a, EffectBlocked, "absent", "No such account in the directory.",
			"Take them out of the selection, or check the account in Zitadel.")
	}
	person := s.Access.People[a.UserID]
	switch {
	case roleOp(a.Op):
		return planRole(ch, a, s, profile, person)
	case a.Op == OpExtend:
		return planExtend(ch, a, s, profile, person)
	case a.Op == OpAdopt:
		return planAdopt(ch, a, s, person)
	case a.Op == OpRevoke:
		return planRevoke(ch, a, s, person)
	case a.Op == OpPublish || a.Op == OpMove:
		return planMove(ch, a, s, person)
	}
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
			step := fmt.Sprintf("Take %s out of %s and publish, then give the bundle again.", s.them(gone), bundle)
			if !bundleGives(s.VersionRoles, gone[0]) {
				step = fmt.Sprintf("Retire the automatic rule that gives %s, then give the bundle again.", s.them(gone))
			}
			return ch.decided(a, EffectNoChange, "not_offered:"+keysField(gone),
				fmt.Sprintf("This would give %s.", s.goneWords(gone)), step)
		}
	}

	// The bundle's own roles are its rows; a role a rule follows with is the
	// rule's row, with the rule that reaches it (decision 7).
	ch = ch.deliver(asRuleRows(deltaWrites(a.UserID, adds, revokes, "bundle", a.BundleID),
		ruleRefs(before, after, closureBefore, closureAfter, rules, rules)), person, s)

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
			ch.Rules = append(ch.Rules, fmt.Sprintf("Also gives %s, by the automatic rule from %s.", s.words(k), s.ruleSource(rules, closureAfter, k)))
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
			ch.Rules = append(ch.Rules, fmt.Sprintf("Also loses %s, which the automatic rule from %s gave.", s.words(k), s.ruleSource(rules, closureBefore, k)))
		}
	}
	return ch.said(a, s, detail, said)
}

// said finishes a row that goes ahead: its own sentences, then the rules'
// (A-21 §4.1), then which writes wait for you, then what it cancels.
func (ch Change) said(a Action, s PlanSnapshot, detail string, said []string) Change {
	said = append(said, ch.Rules...)
	ch.Waits, ch.Goes = s.names(ch.delivered(DeliveryWaits)), s.names(ch.delivered(DeliveryStraightAway))
	if waiting := ch.waiting(); len(waiting) > 0 {
		var gives, removes []roleKey
		for _, w := range ch.Writes {
			if w.Delivery == DeliveryWaits && w.OpType == "revoke" {
				removes = append(removes, roleKey{projectID: w.ProjectID, roleKey: w.RoleKeys[0]})
			} else if w.Delivery == DeliveryWaits {
				gives = append(gives, roleKey{projectID: w.ProjectID, roleKey: w.RoleKeys[0]})
			}
		}
		if len(gives) > 0 {
			said = append(said, fmt.Sprintf("Giving %s waits for you to send.", s.list(gives)))
		}
		if len(removes) > 0 {
			said = append(said, fmt.Sprintf("Removing %s waits for you to send.", s.list(removes)))
		}
	}
	ch.Said = said
	// The cancels line stays last: the plan renderer splits it off the end.
	if line := DescribeCancels(ch.Cancels); line != "" {
		said = append(slices.Clone(said), line)
	}
	return ch.decided(a, EffectApply, "", detail, strings.Join(said, " "))
}

// waiting is the roles whose write waits for you to send.
func (ch Change) waiting() []roleKey { return ch.delivered(DeliveryWaits) }

// delivered is the roles whose write is delivered this way.
func (ch Change) delivered(how string) []roleKey {
	var out []roleKey
	for _, w := range ch.Writes {
		if w.Delivery == how {
			out = append(out, roleKey{projectID: w.ProjectID, roleKey: w.RoleKeys[0]})
		}
	}
	return out
}

// names is roles in words, alphabetical, each name said once (two unlisted
// projects can read the same); nil for none.
func (s PlanSnapshot) names(ks []roleKey) []string {
	var out []string
	seen := map[string]bool{}
	for _, k := range ks {
		if w := s.words(k); !seen[w] {
			seen[w] = true
			out = append(out, w)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return strings.ToLower(out[i]) < strings.ToLower(out[j]) })
	return out
}

// deliver adds the writes, each with how it is delivered (decision 11), and
// the queued changes they cancel (db.Answers, the one predicate).
func (ch Change) deliver(writes []db.EnqueueParams, person db.PersonAccess, s PlanSnapshot) Change {
	for _, p := range writes {
		p.RulesDecided = true
		w := Write{EnqueueParams: p, Delivery: DeliveryStraightAway}
		if p.NoPropagation {
			// A ledger edit only (an extension, an adopted role): it never
			// reaches the outbox, so it neither sends nor cancels anything.
			w.Delivery = DeliveryNone
			ch.Writes = append(ch.Writes, w)
			continue
		}
		ref := db.RoleRef{ProjectID: p.ProjectID, RoleKey: p.RoleKeys[0]}
		answered := db.AnsweredBy(p, person.Queued)
		ch.Cancels = append(ch.Cancels, answered...)
		switch {
		case len(answered) > 0 && !db.OwedAfterCancel(p, person.Delivered[ref]):
			w.Delivery = DeliveryNone
		case s.Access.RoleWaits[ref]:
			w.Delivery, w.WaitReason = DeliveryWaits, db.WaitReasonRoleWaits
		}
		ch.Writes = append(ch.Writes, w)
	}
	return ch
}

// decided fills the row and its fingerprint: the decision, the writes and the
// cancel ids. Words are left out, so renaming a bundle or a role does not
// make a plan stale; what would change what happens does.
func (ch Change) decided(a Action, effect, refusal, detail, consequence string) Change {
	ch.Refusal = refusal
	ch.Row.Effect, ch.Row.Detail, ch.Row.Consequence = effect, detail, consequence
	fields := []string{"planner/1", a.Op, a.UserID, a.BundleID, effect, refusal, ch.VersionID}
	if roleOp(a.Op) {
		fields = append(fields, a.ProjectID, a.RoleKey, a.GrantID, a.RequestID, string(ch.Approval.Effect))
	}
	if a.DriftID != "" || a.Days != 0 {
		fields = append(fields, "drift", a.DriftID, "days", strconv.Itoa(a.Days))
	}
	for _, w := range ch.Writes {
		fields = append(fields, w.OpType, w.ProjectID, strings.Join(w.RoleKeys, ","), w.Source, w.SourceRef, w.Delivery)
		if w.EndRule == db.EndSet {
			// An extension's whole decision is the end it sets.
			fields = append(fields, "ends", endField(w.ExpiresAt))
		}
	}
	fields = append(fields, "cancels")
	ch.Row.CancelIDs = nil
	for _, c := range ch.Cancels {
		fields = append(fields, c.ID)
		ch.Row.CancelIDs = append(ch.Row.CancelIDs, c.ID)
	}
	ch.Row.Fingerprint = Fingerprint(fields...)
	// How the row reaches Zitadel, on the plan and on the result (A-21 §Q1):
	// straight away, waits for you, or nothing sent; which roles go now and
	// which wait.
	ch.Row.Delivery, ch.Row.Waits, ch.Row.Goes = ch.Delivery(), ch.Waits, ch.Goes
	if effect == EffectApply && ch.Row.Delivery == "" {
		// Goes ahead and writes nothing (they already hold it another way):
		// nothing is sent, so the row sits under Straight away like any other
		// row that sends nothing. Every row that goes ahead carries a
		// delivery, which is what lets the screen group a plan at all (A-21
		// §2.9). Delivery() keeps "" for the single acts, whose sheets read
		// "none" as "this only cancels".
		ch.Row.Delivery = DeliveryNone
	}
	return ch
}

func endField(t *time.Time) string {
	if t == nil {
		return "none"
	}
	return t.UTC().Format(time.RFC3339)
}

// params are the writes as the outbox takes them, every one of them: a write
// that only cancels still has to reach the one place cancellations happen.
// reason is the operator's, from the request.
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
	role, project := k.roleKey, ""
	if ok && w.Role != "" {
		role = w.Role
	}
	if ok && w.Project != "" {
		project = w.Project
	}
	if project == "" {
		// The role is gone; its project may still be listed under another role.
		for other, ow := range s.Catalogue {
			if other.projectID == k.projectID && ow.Project != "" {
				project = ow.Project
				break
			}
		}
	}
	if project == "" {
		// Never a raw project id as a label (owner, rule "no raw ids").
		project = "a project Zitadel no longer lists"
	}
	return role + " in " + project
}

// list names roles alphabetically, case-insensitively: "A, B and C".
func (s PlanSnapshot) list(ks []roleKey) string {
	names := s.names(ks)
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

// errWrittenOtherwise: the outbox wrote other rows than the Change says, in
// the transaction that is about to roll back. The planner and the outbox ask
// the same questions of the same snapshot under the same lock, so this is
// unreachable; refused, not trusted.
var errWrittenOtherwise = errors.New("the outbox wrote other rows than the plan said, so nothing was written")

// written pairs the rows the outbox wrote, in order, with the writes that
// leave one: every write but one that only cancels a queued change.
func (ch Change) written(ids []string) (Change, error) {
	ch.Writes = slices.Clone(ch.Writes)
	next := 0
	for i := range ch.Writes {
		if ch.Writes[i].Delivery == DeliveryNone {
			continue
		}
		if next == len(ids) {
			return ch, errWrittenOtherwise
		}
		ch.Writes[i].OutboxID = ids[next]
		next++
	}
	if next != len(ids) {
		return ch, errWrittenOtherwise
	}
	return ch, nil
}

// RoleWords names one role the way plan rows do ("Member in Laser Lab"), from
// the same catalogue; a role Zitadel no longer lists falls back to its key, as
// words does.
func RoleWords(ctx context.Context, projectID, roleKeyStr string) string {
	s := PlanSnapshot{Catalogue: map[roleKey]roleWords{}}
	if projects, err := PlannerCatalogue(ctx); err == nil {
		for _, p := range projects {
			for _, r := range p.Roles {
				s.Catalogue[roleKey{projectID: p.ID, roleKey: r.Key}] = roleWords{Role: r.Label, Project: p.Name}
			}
		}
	}
	return s.words(roleKey{projectID: projectID, roleKey: roleKeyStr})
}

// goneWords says which roles Zitadel no longer has, in agreement with how many.
func (s PlanSnapshot) goneWords(ks []roleKey) string {
	if len(s.names(ks)) > 1 {
		return s.list(ks) + ", which no longer exist in Zitadel"
	}
	return s.list(ks) + ", which no longer exists in Zitadel"
}

// them is "it" or "them" for the roles a step names.
func (s PlanSnapshot) them(ks []roleKey) string {
	if len(s.names(ks)) > 1 {
		return "them"
	}
	return "it"
}
