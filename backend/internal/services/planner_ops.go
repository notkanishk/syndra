package services

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"strings"
	"time"

	"syndra/internal/access"
	"syndra/internal/db"
	"syndra/internal/models"
)

// The rest of the access changes through the planner (architecture
// deepening, task 1.3): extending grants (a ledger edit, nothing sent to
// Zitadel), adopting drift, and repinning a bundle's holders (publish, move).
// Each is the same Change as every other act: the same guards (decision 6),
// the same closure and rule rows (decision 7), the same delivery per write
// (decision 11), the same cancels (db.Answers).

const (
	// OpAdopt records access Zitadel has and Syndra can't explain as a
	// direct grant. Nothing is sent for it; the rules follow it.
	OpAdopt = "adopt"
	// OpPublish moves a bundle's holders onto the version being published;
	// OpMove moves named holders onto a published version.
	OpPublish = "publish_bundle_version"
	OpMove    = "move_bundle_holders"
)

// refusal is a guard that stopped an action before its person was read.
type refusal struct{ effect, code, detail, step string }

// resolve fills in what an action is about from the snapshot: a bulk
// decision's request and what its requester holds, an adoption's drift row.
func (s PlanSnapshot) resolve(a Action) (Action, PlanSnapshot, *refusal) {
	switch {
	case a.Op == OpApprove && s.Requests != nil:
		r, ok := s.Requests[a.RequestID]
		if !ok {
			return a, s, &refusal{EffectBlocked, "absent", "No such request. It may have been withdrawn.", "Nothing for you to do."}
		}
		s.Request, s.Holding = r, s.Holdings[a.RequestID]
		a.UserID, a.ProjectID, a.RoleKey = r.RequesterID, r.ProjectID, r.RoleKey
	case a.Op == OpAdopt:
		item, ok := s.Drift[a.DriftID]
		if !ok {
			return a, s, &refusal{EffectBlocked, "absent", "No longer in the queue. Somebody may have resolved it already.", "Nothing for you to do."}
		}
		a.UserID, a.ProjectID = item.UserID, item.ProjectID
		if item.Status != "" && item.Status != "pending_triage" {
			return a, s, &refusal{EffectNoChange, "resolved", fmt.Sprintf("Already resolved as %s.", HumanDriftStatus(item.Status)), ""}
		}
		if item.DriftType != db.DriftTargetOnly {
			// Only access Zitadel has can be adopted.
			return a, s, &refusal{EffectBlocked, "nothing_to_adopt", "Zitadel doesn't have this, so there's nothing to adopt.",
				"Revoke it or mark it as owned elsewhere from its row in Drift."}
		}
	}
	return a, s, nil
}

// planExtend moves the end of the person's direct grants that end: the ones
// the operator ticked, or every one of the role, or every one they have. A
// ledger edit: Zitadel has no end dates, so nothing is sent (settled 5 Nov),
// unless the new end cancels a removal still waiting for you to send.
func planExtend(ch Change, a Action, s PlanSnapshot, profile models.UserProfile, person db.PersonAccess) Change {
	if access.Left(profile.Status) {
		return ch.decided(a, EffectBlocked, "departed:"+profile.Status,
			fmt.Sprintf("Their account is %s in Zitadel, so it can't be extended.", strings.ToLower(profile.Status)),
			"If they still need it, make the account active in Zitadel first.")
	}
	end := endAfter(s.Now, &a.Days)
	selected := s.Ticked
	var grants []models.DirectGrant
	later := 0
	for _, g := range person.Direct {
		switch {
		// Only grants that end: stamping an end onto one that never does
		// would be the opposite of what was asked.
		case g.ExpiresAt == nil,
			selected != nil && !selected[g.ID],
			a.ProjectID != "" && g.ProjectID != a.ProjectID,
			a.RoleKey != "" && g.RoleKey != a.RoleKey:
		// An extension never shortens (db.EndSet only to a later end).
		case end != nil && !end.After(*g.ExpiresAt):
			later++
		default:
			grants = append(grants, g)
		}
	}
	switch {
	case len(grants) == 0 && later > 0:
		return ch.decided(a, EffectNoChange, "runs_past", "Already ends later than this would make it.", "")
	case len(grants) == 0 && selected != nil:
		return ch.decided(a, EffectNoChange, "nothing_to_extend", "None of the selected grants are theirs any more.",
			"Read the list again to see what they have now.")
	case len(grants) == 0:
		return ch.decided(a, EffectNoChange, "nothing_to_extend", "Has no direct grant with an end to extend.", "")
	}

	var writes []db.EnqueueParams
	var roles []roleKey
	for _, g := range grants {
		ch.Row.GrantIDs = append(ch.Row.GrantIDs, g.ID)
		p := db.EnqueueParams{UserID: a.UserID, ProjectID: g.ProjectID, RoleKeys: []string{g.RoleKey},
			Source: "direct", OpType: "add", EndRule: db.EndSet, ExpiresAt: end}
		// Only the ledger moves, unless the new end answers a removal still
		// waiting for you to send: then the write goes to the one place
		// cancellations happen, and cancels it.
		p.NoPropagation = len(db.AnsweredBy(p, person.Queued)) == 0
		writes = append(writes, p)
		roles = append(roles, roleKey{projectID: g.ProjectID, roleKey: g.RoleKey})
	}
	ch = ch.deliver(writes, person, s)
	if len(roles) == 1 {
		ch.ProjectID, ch.RoleKey = roles[0].projectID, roles[0].roleKey
	}
	said := []string{"Nothing is sent to Zitadel, which has no end dates. Syndra ends the access on the day."}
	if end == nil {
		said = []string{"Nothing is sent to Zitadel."}
	}
	return ch.said(a, s, fmt.Sprintf("Keeps %s %s.", s.list(roles), EndPhrase(end)), said)
}

// planAdopt records the drift row's roles as a direct grant. Nothing is sent
// for them (Zitadel already has them); what the rules give with them is
// written and sent like any other act's rule rows (ADR-006, decision 11).
func planAdopt(ch Change, a Action, s PlanSnapshot, person db.PersonAccess) Change {
	item := s.Drift[a.DriftID]
	own := db.EnqueueParams{UserID: item.UserID, ProjectID: item.ProjectID, RoleKeys: item.RoleKeys,
		Source: "external_backfill", OpType: "add", ZitadelGrantID: item.ZitadelGrantID, NoPropagation: true}
	var roles []roleKey
	before := baseFrom(person, "")
	after := maps.Clone(before)
	for _, rk := range item.RoleKeys {
		k := roleKey{projectID: item.ProjectID, roleKey: rk}
		roles = append(roles, k)
		after[k] = true
	}
	rules := s.Access.Rules
	adds, revokes, closureBefore, closureAfter := ruleDelta(before, after, rules)
	refs := ruleRefs(before, after, closureBefore, closureAfter, rules)
	if refused, gone := s.offered(adds); refused != nil {
		return ch.decided(a, EffectBlocked, "catalogue_unread",
			"Syndra couldn't read Zitadel's list of roles to check the ones the rules give still exist. Try again shortly.", "")
	} else if len(gone) > 0 {
		return ch.decided(a, EffectNoChange, "not_offered:"+keysField(gone),
			fmt.Sprintf("Adopting it would give %s, which no longer exists in Zitadel.", s.list(gone)),
			"Retire the automatic rule that gives it, then adopt it again.")
	}
	ch = ch.deliver(append([]db.EnqueueParams{own}, asRuleRows(deltaWrites(a.UserID, adds, revokes, "rule", ""), refs)...), person, s)
	for _, r := range adds {
		ch.Rules = append(ch.Rules, fmt.Sprintf("Also gives %s, by the automatic rule from %s.", s.words(r), s.ruleSource(rules, closureAfter, r)))
	}
	return ch.said(a, s, fmt.Sprintf("Adopted into Syndra (%s).", s.list(roles)),
		[]string{"Records a direct grant, so the access stops being unexplained and starts being owned. Nothing is sent to Zitadel for it."})
}

// offered is the one guard for a give (decision 6): Zitadel still offers the
// roles. A non-nil Change is the refusal when its list couldn't be read.
func (s PlanSnapshot) offered(gives []roleKey) (*Change, []roleKey) {
	if len(gives) == 0 {
		return nil, nil
	}
	if s.CatalogueUnread {
		return &Change{Refusal: "catalogue_unread"}, nil
	}
	var gone []roleKey
	for _, g := range gives {
		if _, ok := s.Catalogue[g]; !ok {
			gone = append(gone, g)
		}
	}
	return nil, gone
}

// planMove repins one holder of the bundle onto the version a publish cuts
// or a move names (s.Version, giving s.VersionRoles). The writes are the
// closure difference, the bundle's own rows and the rules' rows.
func planMove(ch Change, a Action, s PlanSnapshot, person db.PersonAccess) Change {
	h, holds := s.Holders[a.UserID]
	if !holds {
		return ch.decided(a, EffectBlocked, "not_held", "Doesn't hold this bundle.", "Take them out of the selection.")
	}
	ch.VersionID = s.Version.ID
	if a.Op == OpPublish && s.LeaveHolders {
		return ch.decided(a, EffectNoChange, "stays", fmt.Sprintf("Stays on v%d.", h.Version),
			"Nothing changes for them. The new version applies to new assignments only.")
	}
	if a.Op == OpMove && h.VersionID == s.Version.ID {
		return ch.decided(a, EffectNoChange, "held", fmt.Sprintf("Already on v%d.", h.Version), "")
	}
	detail := fmt.Sprintf("Moves from v%d to v%d.", h.Version, s.Version.Version)
	// As an assignment does: what the new version gives is held after, even
	// a role whose removal is queued (the give answers that removal), and
	// what the old version gave counts only if something else still gives it.
	before := baseFrom(person, "")
	after := baseFrom(person, a.BundleID)
	for _, r := range s.VersionRoles {
		after[roleKey{projectID: r.ProjectID, roleKey: r.RoleKey}] = true
	}
	rules := s.Access.Rules
	closureBefore, closureAfter := effectiveClosure(before, rules), effectiveClosure(after, rules)
	adds, revokes := closureDelta(closureBefore, closureAfter)
	if refused, gone := s.offered(adds); refused != nil {
		return ch.decided(a, EffectBlocked, "catalogue_unread",
			"Syndra couldn't read Zitadel's list of roles to check the ones this gives still exist. Try again shortly.", "")
	} else if len(gone) > 0 {
		step := "Take it out of the bundle and publish again."
		if !bundleGives(s.VersionRoles, gone[0]) {
			step = "Retire the automatic rule that gives it, then move them again."
		}
		return ch.decided(a, EffectNoChange, "not_offered:"+keysField(gone),
			fmt.Sprintf("Moving them would give %s, which no longer exists in Zitadel. They stay on v%d.", s.list(gone), h.Version), step)
	}
	if len(adds) == 0 && len(revokes) == 0 {
		return ch.decided(a, EffectNoChange, "", detail,
			"Everything in this version they already hold another way, so nothing is sent to Zitadel.")
	}
	ch = ch.deliver(asRuleRows(deltaWrites(a.UserID, adds, revokes, "bundle", a.BundleID),
		ruleRefs(before, after, closureBefore, closureAfter, rules)), person, s)
	var said []string
	gains, ruleGains := splitByBundle(adds, s.VersionRoles)
	loses, ruleLoses := splitByBundle(revokes, person.ByBundle[a.BundleID])
	if len(gains) > 0 {
		said = append(said, fmt.Sprintf("Gains %s.", s.list(gains)))
	}
	if len(loses) > 0 {
		said = append(said, fmt.Sprintf("Loses %s.", s.list(loses)))
	}
	for _, k := range ruleGains {
		ch.Rules = append(ch.Rules, fmt.Sprintf("Also gives %s, by the automatic rule from %s.", s.words(k), s.ruleSource(rules, closureAfter, k)))
	}
	for _, k := range ruleLoses {
		ch.Rules = append(ch.Rules, fmt.Sprintf("Also loses %s, which the automatic rule from %s gave.", s.words(k), s.ruleSource(rules, closureBefore, k)))
	}
	return ch.said(a, s, detail, said)
}

// expiryLine is a give's end, said on its row.
func expiryLine(end *time.Time) string {
	if end == nil {
		return "No expiry."
	}
	return fmt.Sprintf("Expires %s.", formatDay(*end))
}

// SettledDetail names how a request was already settled, in the queue's words.
func SettledDetail(status string) string {
	switch status {
	case "approved":
		return "Already approved."
	case "rejected":
		return "Already declined."
	case "withdrawn":
		return "Withdrawn by the person who asked."
	case "fulfilled":
		return "Already closed: the role was given another way."
	default:
		return fmt.Sprintf("Already %s.", status)
	}
}

// writeExtend writes an extension: each grant's new end on the ledger, with
// its audit row. Nothing reaches the outbox but a write that cancels a removal
// still waiting (planExtend), which the outbox cancels.
func writeExtend(ctx context.Context, w ActWrite, ch Change) (Change, error) {
	var ids []string
	for _, wr := range ch.Writes {
		p := wr.EnqueueParams
		p.GrantedBy, p.Reason, p.PayloadJSON = w.Actor, w.Reason, w.Give.PayloadJSON
		res, err := svcEnqueueDirectGrant(ctx, p)
		if err != nil {
			return ch, err
		}
		ids = append(ids, res.OutboxIDs()...)
	}
	return ch.written(ids)
}

// writeAdopt marks the drift row adopted and records its roles, in one
// transaction with the rows the rules owe, as the plan decided them.
func writeAdopt(ctx context.Context, a Action, w ActWrite, ch Change) (Change, error) {
	if err := deferTargetConvergence(ctx, w.Actor, a.UserID, ch.changed()[1:]); err != nil {
		return ch, err
	}
	own := ch.Writes[0].EnqueueParams
	payload, _ := json.Marshal(map[string]string{"source": own.Source})
	own.GrantedBy, own.Reason, own.PayloadJSON = w.Actor, "drift attribution", string(payload)
	rules := ch.params(w.Actor, "Mapping rule cascade")[1:]
	ids, err := svcAttributeDriftTx(ctx, a.DriftID, own, rules...)
	if err != nil {
		return ch, err
	}
	return ch.written(ids)
}

// decisionAudit records who decided a request: an operator approving their
// own is recorded as that (owner, 15 Oct).
func decisionAudit(ctx context.Context, reviewer string, request models.AccessRequest) error {
	action := "access_request.approved"
	if reviewer == request.RequesterID {
		action = "access_request.self_approved"
	}
	return svcInsertAuditLog(ctx, reviewer, request.RequesterID, action, request.ID)
}

// HumanDriftStatus is a resolved drift row's status in the queue's words.
func HumanDriftStatus(status string) string {
	switch status {
	case "attributed":
		return "adopted"
	case "marked_external":
		return "owned elsewhere"
	}
	return status
}
