package services

import (
	"context"

	"syndra/internal/models"
	"syndra/internal/zitadel"
)

// IntentSource names one way Syndra can account for a live grant on a target.
//
// This vocabulary exists because it was implicit, and being implicit cost a
// production incident. Two pieces of code answer "does Syndra expect this
// grant?" — UserExpectsRole for the webhook, and the reconciliation sweep's own
// classification — and they were written separately. When bundles became a
// source of intent, one learned about them and the other did not. The sweep
// reported every bundle-derived role in Zitadel as unexplained drift, for ever,
// and re-affirmed it on every tick; the operator's only way to clear such a
// finding wrote a redundant direct grant that then stopped bundle removal from
// revoking anything.
//
// Nothing about that was subtle. It happened because there was no single place
// that said "these are the sources, and every detector must handle all of
// them", so a new source could be added to one reader and forgotten in the
// other with nothing to notice.
//
// THE RULE: every detector that decides whether Syndra accounts for a grant
// MUST handle every member of AllIntentSources, and must do so in a `switch`
// with no `default` — so adding a member here breaks the build or the test of
// each detector that has not been taught about it. That is the entire point of
// declaring it. See:
//
//	services.UserExpectsRole              (webhook)
//	services/drift.explained              (reconciliation sweep)
//
// and their exhaustiveness tests.
type IntentSource string

const (
	// IntentDirectGrant — a row in direct_role_grants.
	IntentDirectGrant IntentSource = "direct_grant"
	// IntentBundle — a bundle assignment, resolved through the version the
	// holder is PINNED to. Never the working copy: an unpublished edit is not
	// something anybody holds.
	IntentBundle IntentSource = "bundle"
	// IntentMappingRule — an active mapping rule whose source the person holds.
	IntentMappingRule IntentSource = "mapping_rule"
	// IntentExclusion — an operator said this tuple is legitimately external on
	// this target. Not an intent to grant, but it does account for the grant,
	// and a detector that ignores it raises a finding somebody already answered.
	IntentExclusion IntentSource = "exclusion"
)

// AllIntentSources is the closed vocabulary. Adding to it is a deliberate act
// that every detector has to answer for.
var AllIntentSources = []IntentSource{
	IntentDirectGrant,
	IntentBundle,
	IntentMappingRule,
	IntentExclusion,
}

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
