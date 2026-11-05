package services

import (
	"context"
	"fmt"
	"sort"

	"syndra/internal/access"
	"syndra/internal/db"
	"syndra/internal/directory"
	"syndra/internal/models"
)

// svcHeldRecords is everybody's recorded base holding in one read; a seam so
// the cohort's logic needs no database.
var svcHeldRecords = db.HeldRecords

// MappingHolders lists the people a mapping on (projectID, roleKey) reaches:
// the cohort a mapping edit or delete would move, and who the add-on pass
// gives an account. Expected, from records, never observed: the mapping
// resolves from what Syndra gives (collectUserRoles), so its cohort does too.
//
// Read from the effective-access seed (direct grants and pinned bundle
// versions, includes followed), with the rules followed by access.Closure, so a
// cohort agrees with the person page and the token by construction.
//
// People the directory says have left are not in it: the entitlement path
// already disables their account (applyDirectoryStanding), so a mapping edit
// moves nothing for them, and counting them overstated every blast radius.
// Only a positive report drops anyone (access.Left), so a person the directory
// does not return is still counted.
func MappingHolders(ctx context.Context, projectID, roleKey string) ([]string, error) {
	held, err := svcHeldRecords(ctx)
	if err != nil {
		return nil, err
	}
	rules, err := svcGetActiveMappingRules(ctx)
	if err != nil {
		return nil, fmt.Errorf("read mapping rules: %w", err)
	}
	left, err := leftPeople(ctx)
	if err != nil {
		return nil, fmt.Errorf("read the directory: %w", err)
	}
	want := access.Role{ProjectID: projectID, RoleKey: roleKey}
	var out []string
	for u, base := range held {
		if left[u] {
			continue
		}
		if access.Closure(base, rules, access.NewRole)[want] {
			out = append(out, u)
		}
	}
	sort.Strings(out)
	return out, nil
}

// leftPeople is the people the directory positively reports as gone.
func leftPeople(ctx context.Context) (map[string]bool, error) {
	users, err := directory.Default.Users(ctx)
	if err != nil {
		return nil, err
	}
	left := map[string]bool{}
	for _, u := range users {
		if access.Left(u.Status) {
			left[u.ID] = true
		}
	}
	return left, nil
}

// BundleHolding is who holds each live bundle, keyed by bundle id, read from
// the one snapshot every holder count comes from (RoleHolderFacts' shape).
//
// Given is recorded: people Syndra assigned the bundle to. Observed is how many
// of them Zitadel shows holding every role their pinned version gives (own and
// included). Confirmed is the same number, since a bundle holder is by
// definition assigned. Observed is nil when Zitadel has never been read: "not
// checked", never a recorded number dressed as holding.
//
// ponytail: one directory listing plus a collectUserRoles per person, like
// RoleHolderFacts. Fine at ~200 people; cache the snapshot across requests
// past it, never a second way to count.
func BundleHolding(ctx context.Context) (HolderFacts, error) {
	snap, err := newAccessSnapshot(ctx)
	if err != nil {
		return HolderFacts{}, err
	}
	basis, err := snap.Basis()
	if err != nil {
		return HolderFacts{}, err
	}
	f := HolderFacts{Given: map[string]int{}, Basis: basis}
	if basis.ReadAt != nil {
		f.Observed = map[string]int{}
	}
	for _, u := range snap.Users() {
		roleMap, bundles, err := snap.For(u.ID)
		if err != nil {
			return HolderFacts{}, err
		}
		var observed map[string]bool
		if f.Observed != nil {
			if observed, err = snap.Observed(u.ID); err != nil {
				return HolderFacts{}, err
			}
		}
		for _, b := range bundles {
			f.Given[b.ID]++
			if f.Observed != nil && holdsAllFrom(roleMap, observed, func(r models.RoleReason) bool {
				return r.Kind == "bundle" && r.BundleID == b.ID
			}) {
				f.Observed[b.ID]++
			}
		}
	}
	f.Confirmed = f.Observed
	return f, nil
}

// RuleHolding is who has each rule's target because of it, keyed by rule id,
// from the same snapshot. Given is the people the rule fires for (they hold
// its source in Syndra's records, so they should have its target); Observed is
// how many of them Zitadel shows holding the target. nil when never read.
func RuleHolding(ctx context.Context, rules []models.MappingRule) (HolderFacts, error) {
	snap, err := newAccessSnapshot(ctx)
	if err != nil {
		return HolderFacts{}, err
	}
	basis, err := snap.Basis()
	if err != nil {
		return HolderFacts{}, err
	}
	f := HolderFacts{Given: map[string]int{}, Basis: basis}
	if basis.ReadAt != nil {
		f.Observed = map[string]int{}
	}
	for _, u := range snap.Users() {
		roleMap, _, err := snap.For(u.ID)
		if err != nil {
			return HolderFacts{}, err
		}
		var observed map[string]bool
		if f.Observed != nil {
			if observed, err = snap.Observed(u.ID); err != nil {
				return HolderFacts{}, err
			}
		}
		for _, r := range rules {
			target := roleMap[roleKey{projectID: r.TargetProject, roleKey: r.TargetRole}]
			if target == nil || !hasReason(target, func(x models.RoleReason) bool {
				return x.Kind == "mapping" && x.TriggerProject == r.SourceProject && x.TriggerRole == r.SourceRole
			}) {
				continue
			}
			f.Given[r.ID]++
			if f.Observed != nil && observed[r.TargetProject+":"+r.TargetRole] {
				f.Observed[r.ID]++
			}
		}
	}
	f.Confirmed = f.Observed
	return f, nil
}

// holdsAllFrom reports whether Zitadel shows the person holding every role one
// source gives them. A source that gives nothing is not held.
func holdsAllFrom(roleMap map[roleKey]*models.EffectiveRole, observed map[string]bool, from func(models.RoleReason) bool) bool {
	any := false
	for k, r := range roleMap {
		if !hasReason(r, from) {
			continue
		}
		any = true
		if !observed[k.projectID+":"+k.roleKey] {
			return false
		}
	}
	return any
}

func hasReason(r *models.EffectiveRole, match func(models.RoleReason) bool) bool {
	for _, x := range r.Reasons {
		if match(x) {
			return true
		}
	}
	return false
}
