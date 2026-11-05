package services

import (
	"context"
	"fmt"
	"maps"
	"strings"

	"syndra/internal/db"
	"syndra/internal/directory"
	"syndra/internal/models"
)

// Rules on the pipeline (openspec/changes/rules-on-the-pipeline).
//
// A mapping rule used to fire from Zitadel's grant webhook: the orchestrator
// called the Management API directly, with no trace, one hop deep, and only for
// changes made outside Syndra, because the webhook drops Syndra's own. Every
// cascade in this package already followed the rules through the closure and
// the outbox. The direct writes did not: a role given by hand, an approved
// request, a bulk give, an adopted drift reached Zitadel alone and its rule
// targets never followed, while the token and the person's page said they did.
//
// followRules closes that gap at the one place every direct write passes
// (db.enqueueWrites), so add and remove are symmetric and both go through the
// same closure the token is compiled from.
func init() { db.FollowRules = followRules }

// followRules runs write between two reads of the person's base, inside the
// caller's transaction and under the access-mutation lock, and returns the
// writes the closure difference owes Zitadel beyond the write's own roles.
//
// Removal follows the whole chain and keeps anything still held another way:
// a role another bundle, grant or rule still reaches stays in the closure after
// the write, so it is never in the difference. That is the same "still
// expected?" answer every cascade gets, from the same function.
//
// No loop: the rows returned are outbox rows with no ledger row, so delivering
// them changes nothing this reads, and the webhook their delivery echoes is
// Syndra's own and dropped.
func followRules(ctx context.Context, p db.EnqueueParams, write func() error) ([]db.EnqueueParams, error) {
	rules, err := svcGetActiveMappingRules(ctx)
	if err != nil {
		return nil, err
	}
	if len(rules) == 0 {
		// Nothing derives anything: the write goes alone, and costs no reads.
		return nil, write()
	}
	before, err := userBaseHoldings(ctx, p.UserID)
	if err != nil {
		return nil, err
	}
	if err := write(); err != nil {
		return nil, err
	}
	after, err := userBaseHoldings(ctx, p.UserID)
	if err != nil {
		return nil, err
	}
	adds, revokes, closureBefore, closureAfter := ruleDelta(before, after, rules)
	if len(adds) == 0 && len(revokes) == 0 {
		return nil, nil
	}

	// The systems those roles are mapped to converge too, as for any cascade.
	// This is the flush point deltaParams defers to: after the write, in its
	// transaction, under the lock.
	if err := triggerTargetConvergence(ctx, p.GrantedBy, p.UserID,
		append(append([]roleKey(nil), adds...), revokes...)); err != nil {
		return nil, err
	}

	params := deltaWrites(p.UserID, adds, revokes, "rule", "")
	for i := range params {
		k := roleKey{projectID: params[i].ProjectID, roleKey: params[i].RoleKeys[0]}
		reached := closureAfter
		if params[i].OpType == "revoke" {
			reached = closureBefore
		}
		params[i].SourceRef = ruleReaching(rules, reached, k)
		params[i].GrantedBy, params[i].Reason, params[i].PayloadJSON = p.GrantedBy, "Mapping rule cascade", "{}"
	}
	return params, nil
}

// ruleDelta is what the rules owe beyond a change to a person's base: the
// closure difference, less the roles the change itself moved (those are the
// caller's own row). followRules queues it; RulesFollowing names it before
// anyone confirms. One function, so the preview is the queue.
func ruleDelta(before, after map[roleKey]bool, rules []models.MappingRule) (adds, revokes []roleKey, closureBefore, closureAfter map[roleKey]bool) {
	closureBefore, closureAfter = effectiveClosure(before, rules), effectiveClosure(after, rules)
	adds, revokes = closureDelta(closureBefore, closureAfter)
	derived := func(ks []roleKey) []roleKey {
		out := ks[:0]
		for _, k := range ks {
			if before[k] == after[k] {
				out = append(out, k)
			}
		}
		return out
	}
	return derived(adds), derived(revokes), closureBefore, closureAfter
}

// ruleReaching names a rule that derives k within closure: the source_ref
// Change history groups by. The first in the list when several do.
func ruleReaching(rules []models.MappingRule, closure map[roleKey]bool, k roleKey) string {
	if r := ruleReachingRule(rules, closure, k); r != nil {
		return r.ID
	}
	return ""
}

func ruleReachingRule(rules []models.MappingRule, closure map[roleKey]bool, k roleKey) *models.MappingRule {
	for i, r := range rules {
		if r.TargetProject == k.projectID && r.TargetRole == k.roleKey &&
			closure[roleKey{projectID: r.SourceProject, roleKey: r.SourceRole}] {
			return &rules[i]
		}
	}
	return nil
}

// RuleRow is one role an automatic rule gives with a write, said before the
// operator confirms it (coherence audit C1, C2): every consent surface that
// gives a role (give, approve, bulk give, drift adopt) lists these.
type RuleRow struct {
	ProjectID string `json:"project_id"`
	RoleKey   string `json:"role_key"`
	RuleID    string `json:"rule_id"`
	// Sentence is the row in the operator's words: "Also gives Laser cutter in
	// Workshop, by the automatic rule from Trained."
	Sentence string `json:"sentence"`
}

// RulesFollowing is followRules asked in advance: the roles the rules would
// queue if userID were given roles now. The same reads (userBaseHoldings), the
// same delta (ruleDelta), the same closure (models.RuleClosure), so what the
// dialog names is what the write queues. Nil when no rule follows.
func RulesFollowing(ctx context.Context, userID string, roles []models.HeldRole) ([]RuleRow, error) {
	rules, err := svcGetActiveMappingRules(ctx)
	if err != nil || len(rules) == 0 {
		return nil, err
	}
	before, err := userBaseHoldings(ctx, userID)
	if err != nil {
		return nil, err
	}
	after := maps.Clone(before)
	for _, r := range roles {
		after[roleKey{projectID: r.ProjectID, roleKey: r.RoleKey}] = true
	}
	adds, _, _, closureAfter := ruleDelta(before, after, rules)
	if len(adds) == 0 {
		return nil, nil
	}
	name := roleNamer(ctx)
	rows := make([]RuleRow, 0, len(adds))
	for _, k := range adds {
		row := RuleRow{ProjectID: k.projectID, RoleKey: k.roleKey}
		role, project := name(k.projectID, k.roleKey)
		row.Sentence = fmt.Sprintf("Also gives %s in %s, by an automatic rule.", role, project)
		if r := ruleReachingRule(rules, closureAfter, k); r != nil {
			from, _ := name(r.SourceProject, r.SourceRole)
			row.RuleID = r.ID
			row.Sentence = fmt.Sprintf("Also gives %s in %s, by the automatic rule from %s.", role, project, from)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// RuleSentences is rows' sentences, joined: a plan row's consequence.
func RuleSentences(rows []RuleRow) string {
	parts := make([]string, len(rows))
	for i, r := range rows {
		parts[i] = r.Sentence
	}
	return strings.Join(parts, " ")
}

// roleNamer names a role and its project as the directory does, falling back
// to the keys: a plan an operator reads must never fail for want of a label.
func roleNamer(ctx context.Context) func(projectID, roleKey string) (role, project string) {
	seen := map[string]models.ProjectCatalog{}
	return func(projectID, key string) (string, string) {
		p, ok := seen[projectID]
		if !ok {
			if found, has, err := directory.Default.FindProject(ctx, projectID); err == nil && has {
				p = found
			}
			seen[projectID] = p
		}
		role, project := key, projectID
		if p.Name != "" {
			project = p.Name
		}
		for _, r := range p.Roles {
			if r.Key == key && r.Label != "" {
				role = r.Label
			}
		}
		return role, project
	}
}
