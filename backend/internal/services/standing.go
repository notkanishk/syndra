package services

import (
	"context"
	"fmt"

	"syndra/internal/access"
	"syndra/internal/db"
	"syndra/internal/models"
)

// svcRoleDeliveries is the outbox read behind every standing; a seam so the
// standing's rules are tested without a database.
var svcRoleDeliveries = db.RoleDeliveries

// Standings answers the standing of each of one person's roles. Built once per
// person by accessSnapshot.Standing; Of is pure.
type Standings struct {
	deliveries db.Deliveries   // the latest outbox row per role (db.RoleDeliveries)
	observed   map[string]bool // "project:role" in Zitadel's index
	concludes  bool            // the read behind observed was complete
}

func newStandings(deliveries db.Deliveries, observed map[string]bool, basis models.ObservationBasis) Standings {
	return Standings{deliveries: deliveries, observed: observed, concludes: basis.ConcludesAbsence()}
}

// Of is the role's standing, nil when it can't be said (see standingOf).
func (s Standings) Of(projectID, roleKey string) *models.RoleStanding {
	d, ok := s.deliveries.Latest(projectID, roleKey)
	return standingOf(roleKey, d, ok, s.observed[projectID+":"+roleKey], s.concludes)
}

// Taking says a removal of the role is queued (db.Deliveries.Taking): decided,
// whatever Zitadel shows yet. The request check reads it, since asking again
// for a role on its way out is not asking for one already had.
func (s Standings) Taking(projectID, roleKey string) bool {
	return s.deliveries.Taking(projectID, roleKey)
}

// Giving says a give of the role is queued (db.Deliveries.Giving).
func (s Standings) Giving(projectID, roleKey string) bool {
	return s.deliveries.Giving(projectID, roleKey)
}

// standingOf decides one role's standing from the latest outbox row for it
// (when there is one), whether Zitadel's index shows it, and whether the read
// behind that index can conclude an absence. design.md, W3b.
//
// Holding is observed, so Zitadel's answer is never overridden: a row that is
// still on its way only explains a state Zitadel doesn't show yet. A give on
// its way to a role Zitadel already shows (a second source) leaves it held; a
// removal on its way of one Zitadel doesn't show leaves it not held. The latest
// row decides (db.Deliveries.Latest, the answer the planner reads too), so an
// add a later revoke took back is never read as both. A row that settled
// (confirmed, failed) says nothing Zitadel's own answer doesn't.
func standingOf(roleKey string, d db.RoleDelivery, hasRow, observed, concludes bool) *models.RoleStanding {
	unsettled := hasRow && (d.Queued() || (d.Status == "applied" && !d.Confirmed))
	switch {
	case unsettled && observed && !d.Gives(roleKey):
		return standing(models.StandingRemoving)
	case unsettled && !observed && d.Gives(roleKey) && d.Queued():
		return standing(models.StandingWaiting)
	case unsettled && !observed && d.Gives(roleKey):
		return standing(models.StandingSent)
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
// ponytail: one personSnapshot per call, so about five point reads; the request
// list and a bulk approve call it once per request. Fine at tens of requests;
// share one snapshot across a call site's loop if that ever grows.
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
	direct, err := directGrantsByRole(ctx, userID)
	if err != nil {
		return models.RequestHolding{}, fmt.Errorf("read what %s holds of %s/%s: %w", userID, projectID, roleKey, err)
	}
	return holdingFrom(standings.Of(projectID, roleKey), standings.Taking(projectID, roleKey), direct[access.Role{ProjectID: projectID, RoleKey: roleKey}]), nil
}

// directGrantsByRole is the person's ledger rows (direct_role_grants: every
// give Syndra queued, from any source, bundle and rule included), expired ones
// too (an ended grant can be asked to keep), keyed by role: the recorded half
// of holdingFrom.
func directGrantsByRole(ctx context.Context, userID string) (map[access.Role]*models.DirectGrant, error) {
	grants, err := svcGetDirectGrantsForUser(ctx, userID, true)
	if err != nil {
		return nil, err
	}
	out := make(map[access.Role]*models.DirectGrant, len(grants))
	for i := range grants {
		out[access.Role{ProjectID: grants[i].ProjectID, RoleKey: grants[i].RoleKey}] = &grants[i]
	}
	return out, nil
}

// holdingFrom is RequestHolding's rule, pure: the one rule for "already has
// it", read by the request check and sent with the access view.
func holdingFrom(st *models.RoleStanding, taking bool, direct *models.DirectGrant) models.RequestHolding {
	is := func(want models.RoleStanding) bool { return st != nil && *st == want }
	if taking {
		return models.RequestHolding{Kind: models.HoldingNone}
	}
	waiting := is(models.StandingWaiting)
	switch {
	case direct != nil && direct.ExpiresAt != nil:
		return models.RequestHolding{Kind: models.HoldingUntil, Until: direct.ExpiresAt, Waiting: waiting}
	case direct != nil || is(models.StandingHeld) || is(models.StandingSent) || waiting:
		return models.RequestHolding{Kind: models.HoldingForGood, Waiting: waiting}
	default:
		return models.RequestHolding{Kind: models.HoldingNone}
	}
}
