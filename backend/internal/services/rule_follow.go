package services

import (
	"context"

	"syndra/internal/db"
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
	closureBefore, closureAfter := effectiveClosure(before, rules), effectiveClosure(after, rules)
	adds, revokes := closureDelta(closureBefore, closureAfter)
	// The write's own change to the base is the caller's row; the rules owe
	// only what follows from it.
	derived := func(ks []roleKey) []roleKey {
		out := ks[:0]
		for _, k := range ks {
			if before[k] == after[k] {
				out = append(out, k)
			}
		}
		return out
	}
	adds, revokes = derived(adds), derived(revokes)
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

// ruleReaching names a rule that derives k within closure: the source_ref
// Change history groups by. The first in the list when several do.
func ruleReaching(rules []models.MappingRule, closure map[roleKey]bool, k roleKey) string {
	for _, r := range rules {
		if r.TargetProject == k.projectID && r.TargetRole == k.roleKey &&
			closure[roleKey{projectID: r.SourceProject, roleKey: r.SourceRole}] {
			return r.ID
		}
	}
	return ""
}
