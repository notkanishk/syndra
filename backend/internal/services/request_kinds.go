package services

import (
	"errors"
	"fmt"
	"time"

	"syndra/internal/clock"
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
	RequestKindAccess = "access"
	RequestKindExtend = "extend"
)

var (
	// ErrNothingToExtend: an extension asked of a role held with no end.
	ErrNothingToExtend = errors.New("there is no end to extend: they have this with no end date")
	// ErrExtensionShortens: the new end would not be later than the one they
	// have. Counting from the current end, only a malformed ask can reach it.
	ErrExtensionShortens = errors.New("it already runs past that date, so extending would shorten it")
	// ErrAskEnded: the end date the ask names has already passed, so approving
	// would give access that has already ended.
	ErrAskEnded = errors.New("the end date this asks for has passed, so approving would give access that has already ended")
)

// ApprovalEffect is what approving a request will do.
type ApprovalEffect string

const (
	EffectGive      ApprovalEffect = "given"       // a role they don't have
	EffectExtend    ApprovalEffect = "extended"    // a later end on the grant they have
	EffectGiveAgain ApprovalEffect = "given_again" // asked to keep it, and it lapsed meanwhile
)

type ApprovalPlan struct {
	Effect    ApprovalEffect
	ExpiresAt *time.Time // nil: no end date
	EndRule   db.EndRule
}

// endAfter is the end of the days-th day after now's (clock.EndAfterDays), or no end for zero.
func endAfter(now time.Time, days *int) *time.Time {
	if days == nil || *days <= 0 {
		return nil
	}
	t := clock.EndAfterDays(now, *days)
	return &t
}

// extendEnd is the end an extension gives: the date asked for when the ask
// names one (a term's end stays the term's end however late it is decided),
// else the days asked, counted from the end they have while it is still ahead
// ("another month" on a role that ends 18 Dec means 18 Jan), and from now when
// it has passed or there is none. Zero days or fewer means no end. (A-04 §2)
func extendEnd(now time.Time, h models.RequestHolding, days *int, until *time.Time) *time.Time {
	if until != nil {
		return AskedEnd(until)
	}
	base := now
	if h.Kind == models.HoldingUntil && h.Until != nil && h.Until.After(now) {
		base = *h.Until
	}
	return endAfter(base, days)
}

// AskedEnd is the fixed end date an ask carries: the end of its day in the
// makerspace's zone (clock.EndOfDay), whatever instant on that day was sent. A
// client that sent UTC midnight ("until 30 Nov" as 05:30 IST) still gets all
// of 30 Nov.
func AskedEnd(until *time.Time) *time.Time {
	t, _ := clock.EndOfDay(clock.Day(*until)) // a Day is always a valid day
	return &t
}

// CheckExtension says whether a new end extends what is held: there must be an
// end to move, and the new one must be later (or none at all).
func CheckExtension(h models.RequestHolding, newEnd *time.Time) error {
	if h.Kind != models.HoldingUntil || h.Until == nil {
		return ErrNothingToExtend
	}
	if newEnd != nil && !newEnd.After(*h.Until) {
		return ErrExtensionShortens
	}
	return nil
}

// CheckRequestKind refuses a request that doesn't fit what they hold, when it
// is asked. The holding is read again when it is decided (PlanApproval).
func CheckRequestKind(kind string, h models.RequestHolding, days *int, until *time.Time, now time.Time) error {
	if until != nil && !AskedEnd(until).After(now) { // an ask covers its whole last day
		return ErrAskEnded
	}
	switch kind {
	case RequestKindExtend:
		if h.Kind == models.HoldingNone {
			return ErrNothingHeldToExtend
		}
		return CheckExtension(h, extendEnd(now, h, days, until))
	default:
		if h.Kind != models.HoldingNone {
			return ErrRequestAlreadyHeld
		}
		return nil
	}
}

// ErrNothingHeldToExtend: asked to keep a role they don't have.
var ErrNothingHeldToExtend = errors.New("you don't have this, so there's nothing to keep longer; ask for it instead")

// PlanApproval is what approving will do, given what they hold now.
//
// An extension whose grant lapsed while it waited gives the role back for the
// time asked: that is what they asked for, and refusing it would leave them
// to ask again for the same thing. One that has come to have no end has
// nothing left to extend.
//
// An ask that names an end date (until the end of term) gives the grant that
// date, not now plus the days it was worth when asked; one whose date has
// passed is refused rather than given already ended.
func PlanApproval(r models.AccessRequest, h models.RequestHolding, now time.Time) (ApprovalPlan, error) {
	if r.AskedUntil != nil && !AskedEnd(r.AskedUntil).After(now) {
		return ApprovalPlan{}, ErrAskEnded
	}
	end := endAfter(now, r.DurationDays)
	if r.AskedUntil != nil {
		end = AskedEnd(r.AskedUntil)
	}
	if r.Kind == RequestKindExtend {
		end = extendEnd(now, h, r.DurationDays, r.AskedUntil)
	}
	if r.Kind != RequestKindExtend {
		if h.Kind != models.HoldingNone {
			return ApprovalPlan{}, ErrRequestAlreadyHeld
		}
		return ApprovalPlan{Effect: EffectGive, ExpiresAt: end, EndRule: db.EndKept}, nil
	}
	if h.Kind == models.HoldingNone {
		return ApprovalPlan{Effect: EffectGiveAgain, ExpiresAt: end, EndRule: db.EndKept}, nil
	}
	if err := CheckExtension(h, end); err != nil {
		if errors.Is(err, ErrNothingToExtend) {
			return ApprovalPlan{}, ErrRequestAlreadyHeld
		}
		return ApprovalPlan{}, err
	}
	return ApprovalPlan{Effect: EffectExtend, ExpiresAt: end, EndRule: db.EndSet}, nil
}

// HoldingEnd is the end a holding has, nil when it has none.
func HoldingEnd(h models.RequestHolding) *time.Time {
	if h.Kind == models.HoldingUntil {
		return h.Until
	}
	return nil
}

// EndPhrase is "until 3 Nov 2026", or "with no end date".
func EndPhrase(t *time.Time) string {
	if t == nil {
		return "with no end date"
	}
	return fmt.Sprintf("until %s", t.In(clock.Loc).Format("2 Jan 2006"))
}

// ApprovalReason is the grant's recorded reason for what the approval did.
func ApprovalReason(e ApprovalEffect) string {
	switch e {
	case EffectExtend:
		return "Kept longer, from a request"
	case EffectGiveAgain:
		return "Given again, from a request to keep it longer"
	default:
		return "Approved from access request"
	}
}
