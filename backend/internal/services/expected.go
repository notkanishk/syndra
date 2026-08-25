package services

import (
	"context"

	"syndra/internal/models"
	"syndra/internal/zitadel"
)

// HolderKey is one (user, project, role) tuple a user actually holds — union of
// Syndra direct grants and live Zitadel grants. It is the input to rule
// derivation: a mapping rule's target is "expected" only for users who hold the
// rule's source.
type HolderKey struct {
	UserID    string
	ProjectID string
	RoleKey   string
}

// BuildHolderSet unions Syndra direct grants and Zitadel grants into the set of
// tuples each user currently holds.
func BuildHolderSet(direct []models.DirectGrant, zit []zitadel.UserGrant) map[HolderKey]bool {
	h := make(map[HolderKey]bool)
	for _, g := range direct {
		h[HolderKey{g.UserID, g.ProjectID, g.RoleKey}] = true
	}
	for _, g := range zit {
		for _, rk := range g.RoleKeys {
			h[HolderKey{g.UserID, g.ProjectID, rk}] = true
		}
	}
	return h
}

// ExpectedViaRule reports whether (userID, projectID, roleKey) is the target of
// an active mapping rule the user qualifies for (holds the source). Single-hop:
// this covers the mandated "rule-derived grant is expected_via_rule" scenario.
// ponytail: single-hop only — a multi-hop rule chain (A→B→C) where the user
// holds only A would not classify C. Rules in this codebase are single-hop
// today; widen to a fixpoint (as collectUserRoles does) if chains appear.
func ExpectedViaRule(holder map[HolderKey]bool, rules []models.MappingRule, userID, projectID, roleKey string) bool {
	for _, r := range rules {
		if r.TargetProject == projectID && r.TargetRole == roleKey &&
			holder[HolderKey{userID, r.SourceProject, r.SourceRole}] {
			return true
		}
	}
	return false
}

// UserExpectsRole reports whether Syndra's effective-role computation already
// includes (projectID, roleKey) for the user (direct | bundle | rule). Used by
// the webhook to decide whether a surviving external grant event is drift.
func UserExpectsRole(ctx context.Context, userID, projectID, role string) (bool, error) {
	roleMap, _, err := collectUserRoles(ctx, userID)
	if err != nil {
		return false, err
	}
	_, ok := roleMap[roleKey{projectID: projectID, roleKey: role}]
	return ok, nil
}

// IsExcluded reports whether the tuple was marked legitimately-external on this
// target.
//
// The target is compared here as well as filtered in the read that produced the
// slice. That is not redundancy for its own sake: this function is exported and
// pure, so the set it is handed is whatever its caller loaded, and a caller that
// loads every exclusion would otherwise have a TrueNAS "known external" silence
// an unexplained Zitadel grant — a finding suppressed by a decision nobody made
// about it. Matching on the target makes the wrong set produce no answer rather
// than a wrong one.
func IsExcluded(exclusions []models.ExternalGrantExclusion, target, userID, projectID, roleKey string) bool {
	for _, e := range exclusions {
		if e.Target == target && e.UserID == userID && e.ProjectID == projectID && e.RoleKey == roleKey {
			return true
		}
	}
	return false
}
