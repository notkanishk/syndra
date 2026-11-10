package services

import (
	"context"
	"fmt"
	"slices"

	"syndra/internal/db"
	"syndra/internal/models"
)

// svcRoleDeliveries is the outbox read behind every standing; a seam so the
// standing's rules are tested without a database.
var svcRoleDeliveries = db.RoleDeliveries

// Standings answers the standing of each of one person's roles. Built once per
// person by accessSnapshot.Standing; Of is pure.
type Standings struct {
	byRole    map[string]db.RoleDelivery // "project:role" → latest add or revoke
	byProject map[string]db.RoleDelivery // project → latest replace
	observed  map[string]bool            // "project:role" in Zitadel's index
	concludes bool                       // the read behind observed was complete
}

func newStandings(deliveries []db.RoleDelivery, observed map[string]bool, basis models.ObservationBasis) Standings {
	s := Standings{
		byRole:    map[string]db.RoleDelivery{},
		byProject: map[string]db.RoleDelivery{},
		observed:  observed,
		concludes: basis.ConcludesAbsence(),
	}
	for _, d := range deliveries {
		if d.OpType == "replace" {
			s.byProject[d.ProjectID] = d
		} else {
			s.byRole[d.ProjectID+":"+d.RoleKey] = d
		}
	}
	return s
}

// Of is the role's standing, nil when it can't be said (see standingOf).
func (s Standings) Of(projectID, roleKey string) *models.RoleStanding {
	key := projectID + ":" + roleKey
	d, ok := s.byRole[key]
	if r, rok := s.byProject[projectID]; rok && (!ok || r.IntentSeq > d.IntentSeq) {
		d, ok = r, true
	}
	return standingOf(roleKey, d, ok, s.observed[key], s.concludes)
}

// standingOf decides one role's standing from the latest outbox row for it
// (when there is one), whether Zitadel's index shows it, and whether the read
// behind that index can conclude an absence. design.md, W3b.
//
// The latest row decides, so an add a later revoke took back is never read as
// both. A row that settled (confirmed, failed) says nothing Zitadel's own
// answer doesn't, so the index decides then.
func standingOf(roleKey string, d db.RoleDelivery, hasRow, observed, concludes bool) *models.RoleStanding {
	if hasRow {
		gives := d.OpType == "add" || (d.OpType == "replace" && slices.Contains(d.RoleKeys, roleKey))
		switch {
		case d.Status == "pending" || d.Status == "in_flight":
			if gives {
				return standing(models.StandingWaiting)
			}
			return standing(models.StandingRemoving)
		case d.Status == "applied" && !d.Confirmed && gives && !observed:
			return standing(models.StandingSent)
		case d.Status == "applied" && !d.Confirmed && !gives && observed:
			return standing(models.StandingRemoving)
		}
	}
	switch {
	case observed:
		return standing(models.StandingHeld)
	case concludes:
		return standing(models.StandingNotHeld)
	default:
		return nil
	}
}

func standing(s models.RoleStanding) *models.RoleStanding { return &s }

// RequestHolding is what a person already has of a role, for a request or a
// give: the role's standing, plus the direct grant's end when there is one.
//
//   - "for_good": they have it with no end: Syndra granted it for good, or
//     the standing is held, sent or waiting (a bundle, a rule, or given in
//     Zitadel). Asking again asks for nothing.
//   - "until": a grant from Syndra that ends at Until. Asking is asking to keep
//     it longer.
//   - "none": they don't have it, or it is being removed.
//
// Waiting says it is given and not yet sent; it still counts as had, so
// nobody asks for or gives it twice.
// One answer with the person's page (design.md, W3b): waiting, sent (owner,
// 10 Nov: on its way counts as had) and held are had; a role being removed is
// on its way out, so asking for it again is not refused as "already has this".
//
// Read through ctx's querier, so the planner's call under its lock reads in its
// transaction.
func RequestHolding(ctx context.Context, userID, projectID, roleKey string) (models.RequestHolding, error) {
	standings, err := personSnapshot(ctx).Standing(userID)
	if err != nil {
		return models.RequestHolding{}, fmt.Errorf("read what %s holds of %s/%s: %w", userID, projectID, roleKey, err)
	}
	grants, err := svcGetDirectGrantsForUser(ctx, userID, true)
	if err != nil {
		return models.RequestHolding{}, fmt.Errorf("read what %s holds of %s/%s: %w", userID, projectID, roleKey, err)
	}
	var direct *models.DirectGrant
	for i := range grants {
		if grants[i].ProjectID == projectID && grants[i].RoleKey == roleKey {
			direct = &grants[i]
		}
	}
	return holdingFrom(standings.Of(projectID, roleKey), direct), nil
}

// holdingFrom is RequestHolding's rule, pure.
func holdingFrom(st *models.RoleStanding, direct *models.DirectGrant) models.RequestHolding {
	leaving := st != nil && *st == models.StandingRemoving
	had := st != nil && (*st == models.StandingHeld || *st == models.StandingSent || *st == models.StandingWaiting)
	waiting := st != nil && *st == models.StandingWaiting
	granted := direct != nil && !leaving
	switch {
	case granted && direct.ExpiresAt != nil:
		return models.RequestHolding{Kind: models.HoldingUntil, Until: direct.ExpiresAt, Waiting: waiting}
	case granted || had:
		return models.RequestHolding{Kind: models.HoldingForGood, Waiting: waiting}
	default:
		return models.RequestHolding{Kind: models.HoldingNone}
	}
}
