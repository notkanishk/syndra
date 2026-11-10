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
	deliveries db.Deliveries   // the latest outbox rows per role (db.RoleDeliveries)
	observed   map[string]bool // "project:role" in Zitadel's index
	basis      models.ObservationBasis
}

func newStandings(deliveries db.Deliveries, observed map[string]bool, basis models.ObservationBasis) Standings {
	return Standings{deliveries: deliveries, observed: observed, basis: basis}
}

// Of is the role's standing, nil when it can't be said (see standingOf).
func (s Standings) Of(projectID, roleKey string) *models.RoleStanding {
	q, queued := s.deliveries.LatestQueued(projectID, roleKey)
	d, settled := s.deliveries.LatestSettled(projectID, roleKey)
	return standingOf(roleKey, q, queued, d, settled, s.observed[projectID+":"+roleKey], s.basis)
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

// standingOf decides one role's standing from the latest queued and settled
// outbox rows for it, whether Zitadel's index shows it, and the read behind
// that index (the person's basis). design.md, W3b.
//
// Holding is observed, so Zitadel's answer is never overridden; a row only
// explains a state Zitadel doesn't show yet. A queued row comes first: it will
// still be sent, whatever was sent since. A sent row (applied, not confirmed)
// is unsettled only until a complete read lands after Zitadel answered it:
// that read is Zitadel's answer to it, so a give it doesn't show is not held,
// and a removal it still shows is held, never "on its way" for ever.
func standingOf(roleKey string, q db.RoleDelivery, queued bool, d db.RoleDelivery, settled, observed bool, basis models.ObservationBasis) *models.RoleStanding {
	switch {
	case queued && observed && !q.Gives(roleKey):
		return standing(models.StandingRemoving)
	case queued && !observed && q.Gives(roleKey):
		return standing(models.StandingWaiting)
	case !queued && settled && unanswered(d, basis) && observed && !d.Gives(roleKey):
		return standing(models.StandingRemoving)
	case !queued && settled && unanswered(d, basis) && !observed && d.Gives(roleKey):
		return standing(models.StandingSent)
	case observed:
		return standing(models.StandingHeld)
	case basis.ConcludesAbsence():
		return standing(models.StandingNotHeld)
	default:
		return nil
	}
}

// unanswered says a sent row hasn't been read back yet: Zitadel accepted it,
// nothing confirmed it, and no complete read has come after it.
func unanswered(d db.RoleDelivery, basis models.ObservationBasis) bool {
	if d.Status != "applied" || d.Confirmed {
		return false
	}
	readAfter := basis.ConcludesAbsence() && d.CompletedAt != nil && basis.ReadAt.After(*d.CompletedAt)
	return !readAfter
}

func standing(s models.RoleStanding) *models.RoleStanding { return &s }

// RequestHolding is what a person already has of a role, for a request or a
// give: the role's standing, plus the direct grant's end when there is one.
//
// ponytail: one personSnapshot per call, so the person's records plus about
// five point reads; the request list and a bulk approve call it once per
// request. Fine at tens of requests; share one snapshot across a call site's
// loop (personSnapshot memoises per person) if that ever grows.
//
//   - "for_good": they have it with no end: their records give it (a direct
//     grant, a bundle or a rule: asking again asks for nothing; a role given
//     and not in Zitadel is resent, not asked for), or the standing is held,
//     sent or waiting (given in Zitadel too).
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
func RequestHolding(ctx context.Context, userID, projectID, role string) (models.RequestHolding, error) {
	fail := func(err error) (models.RequestHolding, error) {
		return models.RequestHolding{}, fmt.Errorf("read what %s holds of %s/%s: %w", userID, projectID, role, err)
	}
	snap := personSnapshot(ctx)
	standings, err := snap.Standing(userID)
	if err != nil {
		return fail(err)
	}
	roleMap, _, err := snap.For(userID)
	if err != nil {
		return fail(err)
	}
	direct, err := directGrantsByRole(ctx, userID)
	if err != nil {
		return fail(err)
	}
	_, recorded := roleMap[roleKey{projectID: projectID, roleKey: role}]
	return holdingFrom(standings.Of(projectID, role), standings.Taking(projectID, role), recorded,
		direct[access.Role{ProjectID: projectID, RoleKey: role}]), nil
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
func holdingFrom(st *models.RoleStanding, taking, recorded bool, direct *models.DirectGrant) models.RequestHolding {
	is := func(want models.RoleStanding) bool { return st != nil && *st == want }
	if taking {
		return models.RequestHolding{Kind: models.HoldingNone}
	}
	waiting := is(models.StandingWaiting)
	switch {
	case direct != nil && direct.ExpiresAt != nil:
		return models.RequestHolding{Kind: models.HoldingUntil, Until: direct.ExpiresAt, Waiting: waiting}
	case recorded || direct != nil || is(models.StandingHeld) || is(models.StandingSent) || waiting:
		return models.RequestHolding{Kind: models.HoldingForGood, Waiting: waiting}
	default:
		return models.RequestHolding{Kind: models.HoldingNone}
	}
}
