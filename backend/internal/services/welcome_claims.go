package services

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"syndra/internal/claims"
	"syndra/internal/claimserve"
	"syndra/internal/db"
)

// ConfigKeyWelcomeClaims is Makerspace settings' switch for welcome roles on
// a new member's first sign-in (owner, 3 Nov 2026): "true" is on, anything
// else off. Edited only on /settings.
const ConfigKeyWelcomeClaims = "claims.welcome_on_first_sign_in"

// WelcomeClaims is what the data plane may send a brand-new member before
// onboarding has given them anything (assessment SC4 option b).
//
// It is what CascadeBundleAssignedToUser would send a person who holds
// nothing: the welcome bundle's latest published roles and what the mapping
// rules derive from them (the same effectiveClosure), less every role that
// would wait for the operator. Under the old modes a welcome bundle set to
// wait sends nothing at all. Off, it reads nothing else.
//
// An error leaves whatever was published last in place; a welcome bundle not
// configured, or retired, is on with nothing to send.
func WelcomeClaims(ctx context.Context) (claimserve.Welcome, error) {
	v, err := svcGetConfigSetting(ctx, ConfigKeyWelcomeClaims)
	if err != nil {
		return claimserve.Welcome{}, fmt.Errorf("welcome claims: read the switch: %w", err)
	}
	if v != "true" {
		return claimserve.Welcome{}, nil
	}
	on := claimserve.Welcome{On: true}
	bundleID, err := svcGetWelcomeBundle(ctx)
	if errors.Is(err, db.ErrNoWelcomeBundleConfigured) {
		return on, nil
	}
	if err != nil {
		return claimserve.Welcome{}, fmt.Errorf("welcome claims: %w", err)
	}
	bundle, err := svcGetBundleByID(ctx, bundleID)
	if err != nil {
		return claimserve.Welcome{}, fmt.Errorf("welcome claims: read bundle: %w", err)
	}
	if bundle.RetiredAt != nil {
		return on, nil
	}
	_, roles, err := svcLatestVersionRoles(ctx, bundleID)
	if err != nil {
		return claimserve.Welcome{}, fmt.Errorf("welcome claims: %w", err)
	}
	rules, err := svcGetActiveMappingRules(ctx)
	if err != nil {
		return claimserve.Welcome{}, fmt.Errorf("welcome claims: rules: %w", err)
	}
	waitRows, err := svcListRoleWaits(ctx)
	if err != nil {
		return claimserve.Welcome{}, fmt.Errorf("welcome claims: roles that wait: %w", err)
	}
	apps, err := svcApplications(ctx)
	if err != nil {
		return claimserve.Welcome{}, fmt.Errorf("welcome claims: apps: %w", err)
	}

	waits := make(map[roleKey]bool, len(waitRows))
	for _, w := range waitRows {
		waits[roleKey{projectID: w.ProjectID, roleKey: w.RoleKey}] = true
	}
	base := make(map[roleKey]bool, len(roles))
	own := map[string]bool{} // projects the bundle itself gives a role in: their facts name it
	for _, r := range roles {
		base[roleKey{projectID: r.ProjectID, roleKey: r.RoleKey}] = true
		own[r.ProjectID] = true
	}
	byProject := map[string][]string{}
	for k := range effectiveClosure(base, rules) {
		if !waits[k] {
			byProject[k.projectID] = append(byProject[k.projectID], k.roleKey)
		}
	}
	if len(byProject) == 0 {
		return on, nil
	}
	on.Facts = make(map[string]claims.Facts, len(byProject))
	for p, rs := range byProject {
		sort.Strings(rs)
		f := claims.Facts{ProjectID: p, Roles: rs}
		if own[p] {
			f.Bundles = []string{bundle.Name}
		}
		on.Facts[p] = f
	}
	on.Clients = map[string]string{}
	for _, a := range apps {
		if _, ok := on.Facts[a.ProjectID]; ok && a.ClientID != "" {
			on.Clients[a.ClientID] = a.ProjectID
		}
	}
	return on, nil
}
