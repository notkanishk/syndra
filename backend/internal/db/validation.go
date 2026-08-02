package db

import (
	"context"
	"fmt"
	"strings"
	"time"

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

// AuditCursor is the position of the last row a client already has.
//
// A plain OFFSET would be wrong here for two reasons. The tail grows at the
// head, so a page fetched after new rows arrive is shifted by however many
// landed in between — and, more sharply, `created_at` is the TRANSACTION
// timestamp, so a cascade that writes eight audit rows in one transaction
// writes eight rows with the identical instant. Paging on the timestamp alone
// would either skip the rest of that batch or return it forever.
//
// (created_at, id) is unique and totally ordered, which is what keyset paging
// needs. The id is not chronological within a same-instant batch and does not
// need to be: it only has to break the tie the same way every time.
type AuditCursor struct {
	CreatedAt time.Time
	ID        string
}

// GetAuditLogs retrieves the most recent audit log entries.
func GetAuditLogs(ctx context.Context, limit int) ([]models.AuditLog, error) {
	return GetAuditLogsForUser(ctx, "", limit, nil)
}

// GetAuditLogsForUser returns one page of the audit tail, optionally narrowed
// to one person and optionally starting after a cursor.
//
// "Involved in" deliberately means actor OR target: a person's activity is both
// what they did and what was done to them, and splitting those into two feeds
// would make a grant look like it happened to nobody. An empty userID returns
// the unfiltered tail, which is what the global audit page wants.
//
// A nil cursor returns the newest page.
func GetAuditLogsForUser(
	ctx context.Context,
	userID string,
	limit int,
	after *AuditCursor,
) ([]models.AuditLog, error) {
	query, args := buildAuditQuery(userID, limit, after)

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

// buildAuditQuery is separated from execution so the argument numbering can be
// tested without a database. Every clause here is optional and each one shifts
// the placeholder indices of the ones after it, which is precisely the kind of
// arithmetic that fails silently — a mis-numbered LIMIT does not error, it
// returns the wrong page.
func buildAuditQuery(userID string, limit int, after *AuditCursor) (string, []any) {
	const columns = `SELECT id, actor_zitadel_user_id, target_zitadel_user_id, action, resource_id, created_at
	                 FROM audit_logs`

	var where []string
	var args []any
	if userID != "" {
		args = append(args, userID)
		where = append(where, fmt.Sprintf(
			"(actor_zitadel_user_id = $%d OR target_zitadel_user_id = $%d)", len(args), len(args)))
	}
	if after != nil {
		args = append(args, after.CreatedAt, after.ID)
		where = append(where, fmt.Sprintf("(created_at, id) < ($%d, $%d)", len(args)-1, len(args)))
	}
	args = append(args, limit)

	query := columns
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	// The id tiebreak is not cosmetic — without it the ordering of a
	// same-instant batch is undefined, and an undefined order makes the cursor
	// above meaningless.
	query += fmt.Sprintf(" ORDER BY created_at DESC, id DESC LIMIT $%d;", len(args))
	return query, args
}
