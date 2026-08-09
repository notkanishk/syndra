package services

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"syndra/internal/db"
	"syndra/internal/models"
)

// DriftTriageQueue is the read behind Review › Unexplained access.
//
// Every row has to answer two questions in about two seconds: what is this, and
// what happens if I revoke it. That means the evidence sits ON the row rather
// than behind a click — who created it upstream, whether the role is safety-
// gated, whether the holder is a person or a machine, and how many other
// unexplained items the same person has.
//
// Ordering is by risk THEN age, not age alone: a safety-gated role found
// yesterday outranks a wiki role found last week. The row layout never changes
// with risk — only the ordering and a left border do — because a queue whose
// rows rearrange themselves is a queue nobody can scan.
func DriftTriageQueue(ctx context.Context) ([]models.DriftTriageItem, error) {
	items, err := svcGetPendingDriftItems(ctx)
	if err != nil {
		return nil, fmt.Errorf("load drift queue: %w", err)
	}
	if len(items) == 0 {
		return []models.DriftTriageItem{}, nil
	}

	catalog, err := GlobalRoleCatalog(ctx)
	if err != nil {
		return nil, fmt.Errorf("load role catalog for triage: %w", err)
	}
	groups := make(map[string]string, len(catalog))
	known := make(map[string]bool, len(catalog))
	for _, role := range catalog {
		key := role.ProjectID + ":" + role.RoleKey
		groups[key] = role.Group
		known[key] = true
	}

	// How many OTHER pending items each person has. "Marta has 2 more items"
	// is the single piece of context most likely to change a revoke decision
	// — one stray grant is a mistake, three is an offboarding that never ran.
	//
	// Counted across targets on purpose, unlike everything else here. The
	// question it answers is about the person, not the system: unexplained
	// access on the door controller AND in Zitadel is a stronger signal that
	// something went wrong with them than either count alone.
	perUser := make(map[string]int, len(items))
	for _, item := range items {
		perUser[item.UserID]++
	}

	out := make([]models.DriftTriageItem, 0, len(items))
	for _, item := range items {
		enriched := models.DriftTriageItem{
			DriftItem:            item,
			OtherItemsForUser:    perUser[item.UserID] - 1,
			RoleCatalogueApplies: hasRoleCatalogue(item.Target),
		}
		if enriched.RoleCatalogueApplies && len(item.RoleKeys) > 0 {
			key := item.ProjectID + ":" + item.RoleKeys[0]
			enriched.RoleGroup = groups[key]
			enriched.RoleInCatalogue = known[key]
		}
		if user, ok, err := directoryFindUser(ctx, item.UserID); err == nil && ok {
			enriched.UserStatus = user.Status
			enriched.UserIsServiceAccount = isServiceAccount(user)
		}
		out = append(out, enriched)
	}

	sort.SliceStable(out, func(i, j int) bool {
		ri, rj := driftRank(out[i]), driftRank(out[j])
		if ri != rj {
			return ri > rj
		}
		return out[i].DetectedAt.Before(out[j].DetectedAt)
	})
	return out, nil
}

// hasRoleCatalogue reports whether the target's access is described by Syndra's
// global role catalogue. Only Zitadel's is: the catalogue is built from Zitadel
// projects and roles, so a TrueNAS dataset permission is not absent from it, it
// is simply not the kind of thing it lists.
//
// One function, asked by both the enrichment and the ranking, so "this target
// has no catalogue" cannot be true in one place and false in the other. A
// target added to the platform is not added here until it genuinely has one.
func hasRoleCatalogue(target string) bool {
	return target == db.TargetZitadel
}

// driftRank is the risk half of "risk then age". Three tiers only: anything
// finer would be a judgement the data cannot support, and an operator cannot
// hold more than three levels in their head while triaging.
func driftRank(item models.DriftTriageItem) int {
	switch {
	case isSafetyGated(item.RoleGroup):
		return 2
	case item.RoleCatalogueApplies && !item.RoleInCatalogue:
		// A role Syndra no longer knows about. Adopting would recreate
		// something somebody deliberately retired, which is worth surfacing
		// above routine drift even though nothing physical is at stake.
		//
		// Gated on the catalogue applying at all. Without that gate every
		// add-on row ranks here — not because anything was retired, but
		// because a catalogue that never listed it cannot contain it, and
		// the whole triage queue would sort by which system found the drift
		// instead of by how much it matters.
		return 1
	default:
		return 0
	}
}

// isSafetyGated matches the identity provider's own role group. Deliberately a
// substring match on the operator's vocabulary rather than an enum: the group
// is free text upstream, and hard-coding an exact string would silently
// downgrade "Safety-gated (metal)" to routine.
func isSafetyGated(group string) bool {
	return strings.Contains(strings.ToLower(group), "safety")
}

// isServiceAccount marks machine accounts, for which "adopt" is the wrong verb
// — an integration that provisions itself on every deploy will re-create the
// grant tomorrow no matter what Syndra records.
func isServiceAccount(user models.UserProfile) bool {
	local := strings.ToLower(user.Email)
	if at := strings.Index(local, "@"); at >= 0 {
		local = local[:at]
	}
	return strings.EqualFold(user.Status, "service") ||
		strings.HasPrefix(strings.ToLower(user.Name), "svc-") ||
		strings.HasPrefix(local, "svc-")
}
