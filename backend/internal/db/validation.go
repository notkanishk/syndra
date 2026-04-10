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
	query := `SELECT id, actor_zitadel_user_id, target_zitadel_user_id, action, resource_id, created_at 
	          FROM audit_logs ORDER BY created_at DESC LIMIT $1;`

	rows, err := PG.Query(ctx, query, limit)
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
