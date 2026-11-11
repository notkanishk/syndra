package handlers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"syndra/internal/addons"
	"syndra/internal/db"
)

// Changes nobody can confirm (A-09 §3–4, revised by A-13 §5).
//
// A change sent to a target whose answer was lost may or may not have
// happened, and its parameters were never kept, so nothing can resend it. It
// waits on the system's page, under Waiting on you, until a person says what
// they found. Only the rows a person must settle reach it:
//
//   - a record never claimed was never sent: Syndra settles it as `unreached`;
//   - setting.put is read back from the target's own setting, and settled by
//     Syndra when the read can tell;
//   - password.set and password.rotate never appear (db.SelfSettledOperations);
//   - on an add-on declaring calls, every row is first settled by asking it
//     what it recorded (GET /calls, settleFromCalls), the same question the
//     member's own page asks;
//   - a physical row is the add-on's to settle for constants.in_doubt_bound
//     and only then listed (addons.PhysicalHold), naming the desk.
//
// What is left in practice is deletes, and the adopt or release whose answer
// was lost: those two write only the add-on's binding, which no read Syndra
// makes can see, so a person looks (the inventory and the binding conflict
// are where it shows).

// unresolvedGrace is how long a claimed call may go unanswered before it is a
// question rather than a call in flight; one value for the read, the answer
// and the indicators' count.
const unresolvedGrace = db.AddonUnresolvedGrace

// unresolvedLimit bounds one read. These are rare by construction; a target
// with this many is a broken target, and the page says how many it shows.
const unresolvedLimit = 100

var (
	dbListUnresolvedAddonOperations  = db.ListUnresolvedAddonOperations
	dbSettleUnclaimedAddonOperations = db.SettleUnclaimedAddonOperations
	dbResolveAddonOperation          = db.ResolveAddonOperation
	addonsCallAnswer                 = addons.CallAnswer
)

type unconfirmedView struct {
	ID        string    `json:"id"`
	Operation string    `json:"operation"`
	SubjectID string    `json:"subject_id"`
	ActorID   string    `json:"actor_id"`
	About     string    `json:"about,omitempty"`
	SentAt    time.Time `json:"sent_at"`
}

// handleListUnconfirmed is GET /targets/{target}/unconfirmed: the changes on
// this target waiting for a person's answer, newest first.
func handleListUnconfirmed(w http.ResponseWriter, r *http.Request) {
	target := r.PathValue("target")
	ctx := r.Context()

	// What Syndra can settle itself goes first, so it never reaches the list.
	if _, err := dbSettleUnclaimedAddonOperations(ctx, target, unresolvedGrace); err != nil {
		log.Printf("[ADDON] could not settle unsent records on %s: %v (non-fatal)", target, err)
	}
	ops, err := dbListUnresolvedAddonOperations(ctx, target, unresolvedGrace, unresolvedLimit, addons.PhysicalHold())
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "UNCONFIRMED_READ_FAILED", err.Error())
		return
	}
	ops = settleFromRead(ctx, target, settleFromCalls(ctx, ops))

	out := make([]unconfirmedView, 0, len(ops))
	for _, o := range ops {
		out = append(out, unconfirmedView{
			ID: o.ID, Operation: o.Operation, SubjectID: o.SubjectID,
			ActorID: o.ActorID, About: o.About, SentAt: o.SentAt(),
		})
	}
	jsonResponse(w, http.StatusOK, map[string]any{"operations": out})
}

// settleFromCalls settles each row its add-on can answer for (GET /calls) and
// returns the rows still open. The first error stops the asking: an add-on
// without calls, or one not answering now, concludes nothing, and the rows
// stay as they are.
func settleFromCalls(ctx context.Context, ops []db.AddonOperation) []db.AddonOperation {
	open := ops[:0:0]
	for i, o := range ops {
		settled, _, err := settleByCall(ctx, o)
		if err != nil {
			return append(open, ops[i:]...)
		}
		if !settled {
			open = append(open, o)
		}
	}
	return open
}

// settleByCall asks the add-on what it recorded for one open row, whose id is
// the call_id it was sent with, and settles the row on a clear answer:
// recorded happened, unknown did not (no effect was recorded, so trying again
// is safe). in_doubt settles nothing: the add-on is still finding out. A row
// settled meanwhile by someone else counts as settled. The answer is returned
// for the page that asked (the recorded reply's view).
func settleByCall(ctx context.Context, o db.AddonOperation) (settled bool, s addons.Settlement, err error) {
	s, err = addonsCallAnswer(ctx, o.Target, o.Operation, o.ID)
	if err != nil || s.State == addons.CallInDoubt {
		return false, s, err
	}
	_, err = dbResolveAddonOperation(ctx, o.Target, o.ID, s.State == addons.CallRecorded, db.SyndraReader, unresolvedGrace)
	if errors.Is(err, db.ErrAddonOperationNotUnresolved) {
		return true, s, nil
	}
	if err != nil {
		log.Printf("[ADDON] could not settle %s from its add-on's record on %s: %v (non-fatal)", o.ID, o.Target, err)
		return false, s, nil
	}
	return true, s, nil
}

// settleFromRead settles the setting changes a read of the target can answer,
// and returns the rows still open.
//
// setting.put's value is the record's `about`, and the add-on keeps who set
// each setting and when. It happened when the setting holds that value, set by
// the same person, no earlier than the record (a minute's leeway for the two
// clocks). It didn't when the setting was last written before the record was:
// nothing has written it since. Anything else (changed again since, by
// somebody else) a read cannot settle, and the row stays for a person.
func settleFromRead(ctx context.Context, target string, ops []db.AddonOperation) []db.AddonOperation {
	var settings map[string]addons.TargetSetting
	open := ops[:0:0]
	for _, o := range ops {
		if o.Operation != "setting.put" {
			open = append(open, o)
			continue
		}
		if settings == nil {
			read, ok := addonsReadSettings(ctx, target)
			if !ok {
				// Unreadable now: nothing is concluded, and the rows stay.
				return ops
			}
			settings = make(map[string]addons.TargetSetting, len(read))
			for _, s := range read {
				settings[s.Name] = s
			}
		}
		s, known := settings[o.SubjectID]
		earliest := o.CreatedAt.Add(-time.Minute)
		var happened, decided bool
		switch {
		case !known:
		case s.SetAt == nil || s.SetAt.Before(earliest):
			happened, decided = false, true
		case s.Value == o.About && s.SetBy == o.ActorID:
			happened, decided = true, true
		}
		if !decided {
			open = append(open, o)
			continue
		}
		if _, err := dbResolveAddonOperation(ctx, target, o.ID, happened, db.SyndraReader, unresolvedGrace); err != nil {
			log.Printf("[ADDON] could not settle %s from a read of %s: %v (non-fatal)", o.ID, target, err)
			open = append(open, o)
		}
	}
	return open
}

// operationID is a record id's shape. Anything else names no record, and is
// answered as one that is not waiting rather than as a database error.
var operationID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type settleUnconfirmedRequest struct {
	// A pointer, so a body without it is refused rather than read as "no".
	Happened *bool `json:"happened"`
}

// handleSettleUnconfirmed is POST /targets/{target}/unconfirmed/{id}/answer: an
// operator's answer to "did it happen?". Nothing is sent to the target; this
// records what the operator found, with their name.
func handleSettleUnconfirmed(w http.ResponseWriter, r *http.Request) {
	var req settleUnconfirmedRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	if req.Happened == nil {
		jsonValidationErrorResponse(w, "Say whether it happened", map[string]string{"happened": "required"})
		return
	}
	actor := resolveActor(r, "")
	if actor == "system" || strings.HasPrefix(actor, "system:") {
		jsonErrorResponse(w, http.StatusUnauthorized, "NO_ACTOR", "An answer is recorded with a person's name.")
		return
	}

	id := r.PathValue("id")
	var op db.AddonOperation
	err := db.ErrAddonOperationNotUnresolved
	if operationID.MatchString(id) {
		op, err = dbResolveAddonOperation(r.Context(), r.PathValue("target"), id, *req.Happened, actor, unresolvedGrace)
	}
	if errors.Is(err, db.ErrAddonOperationNotUnresolved) {
		// Already answered, by someone else or by a read, or not a change that
		// waits on a person at all. One answer for all of them.
		jsonErrorResponse(w, http.StatusConflict, "NOT_WAITING",
			"That change isn't waiting for an answer any more. Nothing was recorded.")
		return
	}
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "ANSWER_NOT_RECORDED", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{"id": op.ID, "happened": *req.Happened})
}
