package db

import (
	"context"
	"fmt"

	"mkauth/internal/models"
)

// DetectCycleOnInsert checks whether adding a new mapping rule would create
// a circular dependency in the rule graph.
//
// Algorithm: DFS from the proposed target → follow existing rules → if we reach
// the proposed source, we have a cycle.
func DetectCycleOnInsert(ctx context.Context, sourceProject, sourceRole, targetProject, targetRole string) error {
	rules, err := GetActiveMappingRules(ctx)
	if err != nil {
		return fmt.Errorf("cycle detection: failed to load rules: %w", err)
	}

	if hasCycleWithRules(rules, sourceProject, sourceRole, targetProject, targetRole) {
		return fmt.Errorf(
			"circular dependency detected: adding %s:%s → %s:%s would create a cycle",
			sourceProject, sourceRole, targetProject, targetRole,
		)
	}

	return nil
}

// DetectCycleOnUpdate is DetectCycleOnInsert but excludes the rule being edited from the graph
// before adding the proposed (new) edge. DetectCycleOnInsert would still include the edited
// rule's OLD edge (it loads ALL current rules), so a valid retarget that only cycles WITH that
// old edge present would be falsely rejected — e.g. re-pointing a rule to break an existing chain
// it was itself part of. Shares hasCycleWithRules's DFS with DetectCycleOnInsert (DRY).
func DetectCycleOnUpdate(ctx context.Context, excludeRuleID, sourceProject, sourceRole, targetProject, targetRole string) error {
	rules, err := GetActiveMappingRules(ctx)
	if err != nil {
		return fmt.Errorf("cycle detection: failed to load rules: %w", err)
	}

	if hasCycleWithRules(excludeRuleFromGraph(rules, excludeRuleID), sourceProject, sourceRole, targetProject, targetRole) {
		return fmt.Errorf(
			"circular dependency detected: updating rule to %s:%s → %s:%s would create a cycle",
			sourceProject, sourceRole, targetProject, targetRole,
		)
	}

	return nil
}

// excludeRuleFromGraph drops the rule being edited from the loaded set, so DetectCycleOnUpdate's
// DFS runs on the graph WITHOUT the edited rule's old edge. Pure (no DB access) so it — and the
// update-vs-insert cycle difference it exists for — is unit-testable without a live database.
func excludeRuleFromGraph(rules []models.MappingRule, excludeRuleID string) []models.MappingRule {
	filtered := make([]models.MappingRule, 0, len(rules))
	for _, r := range rules {
		if r.ID == excludeRuleID {
			continue
		}
		filtered = append(filtered, r)
	}
	return filtered
}

func hasCycleWithRules(rules []models.MappingRule, sourceProject, sourceRole, targetProject, targetRole string) bool {

	type node struct {
		Project string
		Role    string
	}

	adj := make(map[node][]node)
	for _, r := range rules {
		src := node{r.SourceProject, r.SourceRole}
		tgt := node{r.TargetProject, r.TargetRole}
		adj[src] = append(adj[src], tgt)
	}

	proposedSrc := node{sourceProject, sourceRole}
	proposedTgt := node{targetProject, targetRole}

	// Add the proposed edge temporarily
	adj[proposedSrc] = append(adj[proposedSrc], proposedTgt)

	// DFS from proposedTgt to see if we can reach proposedSrc
	visited := make(map[node]bool)
	var hasCycle func(current node) bool
	hasCycle = func(current node) bool {
		if current == proposedSrc {
			return true
		}
		if visited[current] {
			return false
		}
		visited[current] = true
		for _, neighbor := range adj[current] {
			if hasCycle(neighbor) {
				return true
			}
		}
		return false
	}

	if hasCycle(proposedTgt) {
		return true
	}

	return false
}

// GetAuditLogs retrieves the most recent audit log entries.
func GetAuditLogs(ctx context.Context, limit int) ([]models.AuditLog, error) {
	return GetAuditLogsForUser(ctx, "", limit)
}

// GetAuditLogsForUser returns the audit tail, optionally narrowed to one person.
//
// "Involved in" deliberately means actor OR target: a person's activity is both
// what they did and what was done to them, and splitting those into two feeds
// would make a grant look like it happened to nobody. An empty userID returns
// the unfiltered tail, which is what the global audit page wants.
func GetAuditLogsForUser(ctx context.Context, userID string, limit int) ([]models.AuditLog, error) {
	query := `SELECT id, actor_zitadel_user_id, target_zitadel_user_id, action, resource_id, created_at
	          FROM audit_logs ORDER BY created_at DESC LIMIT $1;`
	args := []any{limit}

	if userID != "" {
		query = `SELECT id, actor_zitadel_user_id, target_zitadel_user_id, action, resource_id, created_at
		         FROM audit_logs
		         WHERE actor_zitadel_user_id = $1 OR target_zitadel_user_id = $1
		         ORDER BY created_at DESC LIMIT $2;`
		args = []any{userID, limit}
	}

	rows, err := PG.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []models.AuditLog
	for rows.Next() {
		var l models.AuditLog
		if err := rows.Scan(&l.ID, &l.ActorID, &l.TargetID, &l.Action, &l.ResourceID, &l.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, nil
}
