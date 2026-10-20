package handlers

import (
	"errors"
	"fmt"
	"time"

	"syndra/internal/db"
	"syndra/internal/models"
)

// Two kinds of request, one rule each (owner, 16 Oct 2026).
//
//   - access: for a role they don't have. Anything they hold refuses it.
//   - extend: "Ask to keep it longer" on a role that ends. It changes the end
//     of the grant they have, the same change Expiring access makes, and never
//     gives the role a second time. The time asked counts from the end they
//     have while it is still ahead, not from today.
//
// The rules live here, as plain functions of what is held, so asking,
// approving one, approving in bulk and an operator's own extension all decide
// the same way.
const (
	requestKindAccess = "access"
	requestKindExtend = "extend"
)

var (
	// errNothingToExtend: an extension asked of a role held with no end.
	errNothingToExtend = errors.New("there is no end to extend: they have this with no end date")
	// errExtensionShortens: the new end would not be later than the one they
	// have. Counting from the current end, only a malformed ask can reach it.
	errExtensionShortens = errors.New("it already runs past that date, so extending would shorten it")
	// errAskEnded: the end date the ask names has already passed, so approving
	// would give access that has already ended.
	errAskEnded = errors.New("the end date this asks for has passed, so approving would give access that has already ended")
)

// approvalEffect is what approving a request will do.
type approvalEffect string

const (
	effectGive      approvalEffect = "given"       // a role they don't have
	effectExtend    approvalEffect = "extended"    // a later end on the grant they have
	effectGiveAgain approvalEffect = "given_again" // asked to keep it, and it lapsed meanwhile
)

type approvalPlan struct {
	Effect    approvalEffect
	ExpiresAt *time.Time // nil: no end date
	EndRule   db.EndRule
}

// endAfter is now plus days, or no end for zero.
func endAfter(now time.Time, days *int) *time.Time {
	if days == nil || *days <= 0 {
		return nil
	}
	t := now.UTC().Add(time.Duration(*days) * 24 * time.Hour)
	return &t
}

// extendEnd is the end an extension gives: the date asked for when the ask
// names one (a term's end stays the term's end however late it is decided),
// else the days asked, counted from the end they have while it is still ahead
// ("another month" on a role that ends 18 Dec means 18 Jan), and from now when
// it has passed or there is none. Zero days or fewer means no end. (A-04 §2)
func extendEnd(now time.Time, h models.RequestHolding, days *int, until *time.Time) *time.Time {
	if until != nil {
		return askedEnd(until)
	}
	base := now
	if h.Kind == models.HoldingUntil && h.Until != nil && h.Until.After(now) {
		base = *h.Until
	}
	return endAfter(base, days)
}

// askedEnd is the fixed end date an ask carries, in UTC.
func askedEnd(until *time.Time) *time.Time {
	t := until.UTC()
	return &t
}

// checkExtension says whether a new end extends what is held: there must be an
// end to move, and the new one must be later (or none at all).
func checkExtension(h models.RequestHolding, newEnd *time.Time) error {
	if h.Kind != models.HoldingUntil || h.Until == nil {
		return errNothingToExtend
	}
	if newEnd != nil && !newEnd.After(*h.Until) {
		return errExtensionShortens
	}
	return nil
}

// checkRequestKind refuses a request that doesn't fit what they hold, when it
// is asked. The holding is read again when it is decided (planApproval).
func checkRequestKind(kind string, h models.RequestHolding, days *int, until *time.Time, now time.Time) error {
	if until != nil && !until.After(now) {
		return errAskEnded
	}
	switch kind {
	case requestKindExtend:
		if h.Kind == models.HoldingNone {
			return errNothingHeldToExtend
		}
		return checkExtension(h, extendEnd(now, h, days, until))
	default:
		if h.Kind != models.HoldingNone {
			return errRequestAlreadyHeld
		}
		return nil
	}
}

// errNothingHeldToExtend: asked to keep a role they don't have.
var errNothingHeldToExtend = errors.New("you don't have this, so there's nothing to keep longer; ask for it instead")

// planApproval is what approving will do, given what they hold now.
//
// An extension whose grant lapsed while it waited gives the role back for the
// time asked: that is what they asked for, and refusing it would leave them
// to ask again for the same thing. One that has come to have no end has
// nothing left to extend.
//
// An ask that names an end date (until the end of term) gives the grant that
// date, not now plus the days it was worth when asked; one whose date has
// passed is refused rather than given already ended.
func planApproval(r models.AccessRequest, h models.RequestHolding, now time.Time) (approvalPlan, error) {
	if r.AskedUntil != nil && !r.AskedUntil.After(now) {
		return approvalPlan{}, errAskEnded
	}
	end := endAfter(now, r.DurationDays)
	if r.AskedUntil != nil {
		end = askedEnd(r.AskedUntil)
	}
	if r.Kind == requestKindExtend {
		end = extendEnd(now, h, r.DurationDays, r.AskedUntil)
	}
	if r.Kind != requestKindExtend {
		if h.Kind != models.HoldingNone {
			return approvalPlan{}, errRequestAlreadyHeld
		}
		return approvalPlan{Effect: effectGive, ExpiresAt: end, EndRule: db.EndKept}, nil
	}
	if h.Kind == models.HoldingNone {
		return approvalPlan{Effect: effectGiveAgain, ExpiresAt: end, EndRule: db.EndKept}, nil
	}
	if err := checkExtension(h, end); err != nil {
		if errors.Is(err, errNothingToExtend) {
			return approvalPlan{}, errRequestAlreadyHeld
		}
		return approvalPlan{}, err
	}
	return approvalPlan{Effect: effectExtend, ExpiresAt: end, EndRule: db.EndSet}, nil
}

// holdingEnd is the end a holding has, nil when it has none.
func holdingEnd(h models.RequestHolding) *time.Time {
	if h.Kind == models.HoldingUntil {
		return h.Until
	}
	return nil
}

// endPhrase is "until 3 Nov 2026", or "with no end date".
func endPhrase(t *time.Time) string {
	if t == nil {
		return "with no end date"
	}
	return fmt.Sprintf("until %s", t.Format("2 Jan 2006"))
}

// approvalReason is the grant's recorded reason for what the approval did.
func approvalReason(e approvalEffect) string {
	switch e {
	case effectExtend:
		return "Kept longer, from a request"
	case effectGiveAgain:
		return "Given again, from a request to keep it longer"
	default:
		return "Approved from access request"
	}
}
