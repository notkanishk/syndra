package services

import (
	"context"

	"syndra/internal/access"
	"syndra/internal/db"
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

// HolderKey is one (user, project, role) tuple.
type HolderKey struct {
	UserID    string
	ProjectID string
	RoleKey   string
}

// BuildHolderSet unions Syndra direct grants and Zitadel grants into one set
// of tuples. Rule derivation is never seeded from the Zitadel half (see
// RuleDerived); the sweep uses it to see what Zitadel holds.
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

// ObservedToUserGrants adapts what the observation store holds to the shape
// every existing detector (this file, the reconciliation diff) already
// classifies against. There is one store and one Go type for what it holds
// (db.ObservedGrant); this is the seam so drift's sweep and reconciliation's
// diff can keep asking the same question of it that they always asked of a
// live Zitadel listing, without either one becoming a second definition of
// what a grant is.
func ObservedToUserGrants(observed []db.ObservedGrant) []zitadel.UserGrant {
	out := make([]zitadel.UserGrant, len(observed))
	for i, g := range observed {
		out[i] = zitadel.UserGrant{ID: g.GrantID, UserID: g.UserID, ProjectID: g.ProjectID, RoleKeys: g.RoleKeys}
	}
	return out
}

// RuleDerived is every (user, role) a mapping rule gives, for everybody at
// once: the targets of the rules that fire on each person's closure.
//
// held is what Syndra RECORDS each person holding (direct grants and pinned
// bundle roles), never what Zitadel was seen to hold. That is the seed the
// person view, the webhook (UserExpectsRole) and the token use, so the sweep
// and the person page now agree about a rule target. They did not: the sweep
// seeded from observed grants as well, so a target whose source somebody added
// by hand in Zitadel was "explained" to the sweep while the person page called
// the source unexplained. The token never carries such a target (rules fire
// from records only), so the sweep now raises it with its source.
//
// Forward, through access.Closure and access.Fired, rather than a reverse walk
// from each target: one closure per person answers every target, and it is the
// same code that decides what the token carries.
func RuleDerived(held map[HolderKey]bool, rules []models.MappingRule) map[HolderKey]bool {
	out := map[HolderKey]bool{}
	if len(rules) == 0 {
		return out
	}
	byUser := map[string]map[access.Role]bool{}
	for k := range held {
		if byUser[k.UserID] == nil {
			byUser[k.UserID] = map[access.Role]bool{}
		}
		byUser[k.UserID][access.Role{ProjectID: k.ProjectID, RoleKey: k.RoleKey}] = true
	}
	for u, base := range byUser {
		for _, r := range access.Fired(access.Closure(base, rules, access.NewRole), rules, access.NewRole) {
			out[HolderKey{u, r.TargetProject, r.TargetRole}] = true
		}
	}
	return out
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
