package services

import (
	"context"
	"fmt"
	"log"
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
	// The whole pending queue is its own population: every row is counted, and
	// each one's "other items" is the rest of it.
	return enrichForTriage(ctx, items, items)
}

// DriftTriageRows enriches a caller-supplied subset of the queue — a filtered
// listing — with the same evidence and the same ordering.
//
// A filtered response used to be raw drift rows. That is not a smaller answer,
// it is a differently-shaped one: the surface reads `role_in_catalogue` off
// every row, and an absent field is indistinguishable from a false one, so
// filtering the queue silently retracted the "role not in catalogue" warning
// from rows that had earned it. One shape, whether or not a filter was applied.
//
// The other-items count is taken over the whole pending queue rather than over
// the subset, because "Marta has 2 more items" is a fact about Marta, not about
// the query. Counted within a filter it would shrink to match whatever the
// operator happened to be looking at, and read as reassurance.
func DriftTriageRows(ctx context.Context, items []models.DriftItem) ([]models.DriftTriageItem, error) {
	if len(items) == 0 {
		return []models.DriftTriageItem{}, nil
	}
	population, err := svcGetPendingDriftItems(ctx)
	if err != nil {
		return nil, fmt.Errorf("load drift queue: %w", err)
	}
	return enrichForTriage(ctx, items, population)
}

// enrichForTriage attaches the evidence a triage decision needs to `items`,
// counting per-person context over `population`.
func enrichForTriage(ctx context.Context, items, population []models.DriftItem) ([]models.DriftTriageItem, error) {
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
	perUser := make(map[string]int, len(population))
	counted := make(map[string]bool, len(population))
	for _, item := range population {
		perUser[item.UserID]++
		counted[item.ID] = true
	}

	// The history behind a `syndra_only` row, looked up once for the whole
	// queue rather than once per row.
	provenance := grantProvenance(ctx, items)

	out := make([]models.DriftTriageItem, 0, len(items))
	for _, item := range items {
		// Subtract this row only if the population actually contains it.
		// Otherwise a row outside the counted set — a resolved one fetched by a
		// status filter — would report one fewer item than the person has.
		self := 0
		if counted[item.ID] {
			self = 1
		}
		enriched := models.DriftTriageItem{
			DriftItem:            item,
			OtherItemsForUser:    perUser[item.UserID] - self,
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
		if len(item.RoleKeys) > 0 {
			if p, found := provenance[grantKey(item.UserID, item.ProjectID, item.RoleKeys[0])]; found {
				enriched.Provenance = &p
			}
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

func grantKey(userID, projectID, roleKey string) string {
	return userID + "\x00" + projectID + "\x00" + roleKey
}

// grantProvenance answers, per drift row, whether this is the same entitlement
// Syndra applied — and if so, what Syndra knows about it.
//
// Only for `syndra_only` rows. A `target_only` row is by definition access
// Syndra has no record of, and attaching a history to one would be attaching
// somebody else's.
//
// Two sources, and each answers half the question. The ledger says who decided
// this access, when, why, and whether a person granted it or a rule derived it.
// The merge base says when a complete read last saw the TARGET holding it. The
// pair is what makes a removal legible: it existed, it was live this morning,
// it is gone now — rather than a row that appeared from nowhere and reads like a
// stranger.
//
// Both reads fail soft. A row without its history is still a finding worth
// triaging; refusing to render the queue because a lookup failed would hide the
// findings themselves, which is the more expensive silence.
func grantProvenance(ctx context.Context, items []models.DriftItem) map[string]models.GrantProvenance {
	out := map[string]models.GrantProvenance{}
	wanted := map[string]bool{}
	for _, item := range items {
		if item.DriftType != db.DriftSyndraOnly || len(item.RoleKeys) == 0 {
			continue
		}
		wanted[grantKey(item.UserID, item.ProjectID, item.RoleKeys[0])] = true
	}
	if len(wanted) == 0 {
		return out
	}

	// Expired grants included: a grant that lapsed while a removal was standing
	// is exactly the case where "it was due to end anyway" changes what an
	// operator does, and excluding it would report the history as absent.
	grants, err := svcGetAllDirectGrants(ctx, true)
	if err != nil {
		log.Printf("[DRIFT-TRIAGE] could not read the ledger for provenance: %v", err)
		return out
	}
	for _, g := range grants {
		key := grantKey(g.UserID, g.ProjectID, g.RoleKey)
		if !wanted[key] {
			continue
		}
		granted := g.CreatedAt
		out[key] = models.GrantProvenance{
			GrantedBy: g.GrantedBy, GrantedAt: &granted, Reason: g.Reason,
			Source: g.Source, SourceRef: g.SourceRef, ExpiresAt: g.ExpiresAt,
		}
	}

	// When the target was last seen holding it. Keyed by subject, so one read
	// per queue rather than one per row.
	bases, err := svcMergeBases(ctx, db.TargetZitadel)
	if err != nil {
		log.Printf("[DRIFT-TRIAGE] could not read observations for provenance: %v", err)
		return out
	}
	for _, item := range items {
		if len(item.RoleKeys) == 0 {
			continue
		}
		key := grantKey(item.UserID, item.ProjectID, item.RoleKeys[0])
		p, found := out[key]
		if !found {
			continue
		}
		base, seen := bases[item.UserID]
		if !seen {
			continue
		}
		if _, held := base.Base[item.ProjectID+"/"+item.RoleKeys[0]]; !held {
			// The base does not name this grant, so the last complete read did
			// not see the target holding it. Left absent rather than dated with
			// the subject's observation time, which would be a claim about a
			// grant nobody observed.
			continue
		}
		observed := base.ObservedAt
		p.LastObservedAt = &observed
		out[key] = p
	}
	return out
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
