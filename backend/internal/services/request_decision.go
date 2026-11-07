package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"syndra/internal/db"
)

// Deciding a request (architecture deepening, decision 5): approving is an
// act the planner decides (ApplyAct, OpApprove); declining writes no access.
// The handler and the bulk decision both call DecideRequest, so the two can't
// drift apart.

var (
	ErrRequestNotFound        = errors.New("access request not found")
	ErrRequestAlreadyResolved = errors.New("access request is already resolved")
	// ErrRequestAlreadyHeld is approving a request for a role the person has
	// come to hold since asking. Nothing is granted; declining closes it.
	ErrRequestAlreadyHeld = errors.New("they already have this, so approving would change nothing; decline it to close the request")
)

// Decision is what deciding a request did: for an approval, the Change as
// written (send it with DeliverChange) and what approving did with the grant.
type Decision struct {
	UserID   string
	Change   Change
	Approval ApprovalPlan
}

// DecideRequest approves or declines one request. An approval goes through the
// planner: with seen, only the Change the reviewer saw is written (StalePlan
// otherwise); a refusal is a *Refused whose code says why ("held",
// "runs_past", "ask_ended", "resolved", "not_offered", "catalogue_unread",
// "departed"). Nothing is sent to Zitadel here.
func DecideRequest(ctx context.Context, requestID, status, reviewer, note string, seen *Seen) (Decision, error) {
	request, err := svcGetAccessRequestByID(ctx, requestID)
	if err != nil {
		return Decision{}, fmt.Errorf("%w: %s", ErrRequestNotFound, err.Error())
	}
	// Anything that is not pending is already settled, including 'withdrawn',
	// which the member settled themselves.
	if request.Status != "pending" {
		return Decision{}, fmt.Errorf("%w (already %s)", ErrRequestAlreadyResolved, request.Status)
	}
	d := Decision{UserID: request.RequesterID}
	if status == "approved" {
		ch, err := ApplyAct(ctx, Action{Op: OpApprove, UserID: request.RequesterID, RequestID: requestID},
			ActWrite{Actor: reviewer, Note: note}, seen)
		// The refusals a decision has always had their own words for.
		var refused *Refused
		if errors.As(err, &refused) {
			switch refused.Code() {
			case "resolved":
				err = ErrRequestAlreadyResolved
			case "held":
				err = ErrRequestAlreadyHeld
			case "runs_past":
				err = ErrExtensionShortens
			case "ask_ended":
				err = ErrAskEnded
			}
		}
		if err != nil {
			return Decision{}, err
		}
		d.Change, d.Approval = ch, ch.Approval
	} else {
		// A declined extension remembers the end it left alone. Best-effort: a
		// failed read must not stop a decline, it only leaves the end unsaid.
		var endedBefore *time.Time
		if request.Kind == RequestKindExtend {
			if h, err := svcRequestHolding(ctx, request.RequesterID, request.ProjectID, request.RoleKey); err != nil {
				log.Printf("[ACCESS] could not read the end %s has for declined request %s: %v", request.RequesterID, requestID, err)
			} else {
				endedBefore = HoldingEnd(h)
			}
		}
		if err := svcResolveAccessRequest(ctx, requestID, status, reviewer, note, endedBefore); err != nil {
			if errors.Is(err, db.ErrRequestNotPending) {
				return Decision{}, ErrRequestAlreadyResolved
			}
			return Decision{}, err
		}
	}

	action := "access_request." + status
	if status == "approved" && reviewer == request.RequesterID {
		// Allowed, and said (owner, 15 Oct): an operator approving their own
		// request is recorded as that, so Change history and the request row
		// show it rather than reading like a second person's decision.
		action = "access_request.self_approved"
	}
	_ = svcInsertAuditLog(ctx, reviewer, request.RequesterID, action, requestID)
	return d, nil
}
