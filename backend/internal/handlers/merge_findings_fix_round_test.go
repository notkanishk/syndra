package handlers

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"syndra/internal/addons"
	"syndra/internal/db"
	"syndra/internal/services/addonop"
)

// 7t-7 review round: the merge base an adoption forgets wherever it lands, the account_id
// precondition on every adopt and release Syndra sends, and the reservation a failed call gives back.

func strp(s string) *string { return &s }

// Every adoption path lands in recordAdoptedBinding, which forgets the old account's observation
// when, and only when, the person moves to another account.
func TestEveryAdoptionPathForgetsTheOldAccountsBaseWhenThePersonMoves(t *testing.T) {
	old := &db.TargetBinding{Target: "truenas", SubjectID: "u-becky", Username: "bob", AccountID: strp("3051")}
	same := &db.TargetBinding{Target: "truenas", SubjectID: "u-becky", Username: "charlotte", AccountID: strp("200001")}
	for _, tc := range []struct {
		name  string
		was   *db.TargetBinding
		moved bool
	}{{"moved from another account", old, true}, {"the same account again", same, false}, {"a first adoption", nil, false}} {
		t.Run("inventory: "+tc.name, func(t *testing.T) {
			h := &adoptHarness{uid: "200001", was: tc.was}
			stubAdopt(t, h)
			if rr := adopt(t, `{"subject_id":"u-becky","confirmed":true,"account_id":"200001"}`); rr.Code != http.StatusOK {
				t.Fatalf("%d %s", rr.Code, rr.Body.String())
			}
			if got := len(h.basesForgotten) == 1; got != tc.moved {
				t.Fatalf("base forgotten %v, want %v", h.basesForgotten, tc.moved)
			}
			if p := h.dispatchd[0].Params; p["account_id"] != "200001" || p["username"] != "charlotte" {
				t.Fatalf("the inventory sends the account it showed: %+v", p)
			}
		})
		t.Run("calls settle: "+tc.name, func(t *testing.T) {
			h := &adoptHarness{was: tc.was}
			stubAdopt(t, h)
			stubBindingSettle(t, recorded(`{"operation":"account.adopt","subject":"u-becky","outcome":"succeeded","account_id":"200001"}`))
			sent := time.Now().Add(-time.Hour)
			row := db.AddonOperation{ID: "op_1", Target: "truenas", Operation: "account.adopt", SubjectID: "u-becky",
				ActorID: "op_lita", About: "charlotte", ClaimedAt: &sent}
			if open := settleFromCalls(context.Background(), []db.AddonOperation{row}); len(open) != 0 || len(h.recorded) != 1 {
				t.Fatalf("settled with its half: open %v recorded %v", open, h.recorded)
			}
			if got := len(h.basesForgotten) == 1; got != tc.moved {
				t.Fatalf("base forgotten %v, want %v", h.basesForgotten, tc.moved)
			}
		})
	}
	t.Run("binding conflict resolution: moved", func(t *testing.T) {
		h := stubConflictActions(t, bindingConflict("f1", "u-becky", numberHeldAdoptable))
		h.accountID = "3051"
		was := db.TargetBinding{Target: "truenas", SubjectID: "u-becky", Username: "bob", AccountID: strp("3050")}
		var forgot []string
		get, base := dbGetTargetBinding, dbForgetMergeBase
		t.Cleanup(func() { dbGetTargetBinding, dbForgetMergeBase = get, base })
		dbGetTargetBinding = func(context.Context, string, string) (db.TargetBinding, bool, error) { return was, true, nil }
		dbForgetMergeBase = func(_ context.Context, _, subject string) error { forgot = append(forgot, subject); return nil }
		if rr := resolveFinding(t, `{"resolution":"adopted","reason":"it is theirs"}`); rr.Code != http.StatusOK {
			t.Fatalf("%d %s", rr.Code, rr.Body.String())
		}
		if strings.Join(forgot, ",") != "u-becky" || len(h.recorded) != 1 {
			t.Fatalf("the old account's base goes once, with the new copy: forgot %v recorded %v", forgot, h.recorded)
		}
	})
}

// adopted needs the account the operator was shown, or it is not offered (the add-on refuses a move
// without it too).
func TestAnAdoptionWithNoShownAccountIsRefusedAtTheGate(t *testing.T) {
	h := stubConflictActions(t, bindingConflict("f1", "sub-1", `{"reason":"name_held","username":"eve","adoptable":true}`))
	if rr := resolveFinding(t, `{"resolution":"adopted","reason":"x"}`); rr.Code != http.StatusUnprocessableEntity || len(h.dispatched) != 0 {
		t.Fatalf("want 422 and nothing sent, got %d %s", rr.Code, rr.Body.String())
	}
}

// Every release from a finding names the account it means: the one two people claim for keep and
// for Stop tracking on claimed_twice, the backend's copy for any other.
func TestEveryReleaseFromAFindingNamesTheAccountItMeans(t *testing.T) {
	for _, tc := range []struct {
		name, theirs, resolution, subject, want string
	}{
		{"keep it for this person", claimedTwice, "kept", "sub-2", "3051"},
		{"stop tracking, claimed twice", claimedTwice, "unbound", "sub-1", "3051"},
		{"stop tracking, name held", nameHeldAdoptable, "unbound", "sub-1", "3040"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := stubConflictActions(t, bindingConflict("f1", "sub-1", tc.theirs))
			get := dbGetTargetBinding
			t.Cleanup(func() { dbGetTargetBinding = get })
			dbGetTargetBinding = func(context.Context, string, string) (db.TargetBinding, bool, error) {
				return db.TargetBinding{Username: "rey", AccountID: strp("3040")}, true, nil
			}
			if rr := resolveFinding(t, `{"resolution":"`+tc.resolution+`","reason":"x"}`); rr.Code != http.StatusOK {
				t.Fatalf("%d %s", rr.Code, rr.Body.String())
			}
			d := h.dispatched[0]
			if d.Operation != "account.release" || d.SubjectID != tc.subject || d.Params["account_id"] != tc.want {
				t.Fatalf("want a release of %s meaning %s, got %+v", tc.subject, tc.want, d)
			}
		})
	}
}

// The other person was released and bound again since: the add-on refuses the stale release, the
// operator reads its words, nothing is forgotten here, and both reservations are given back.
func TestAStaleKeepIsRefusedAndGivesBothReservationsBack(t *testing.T) {
	h := stubConflictActions(t, bindingConflict("f1", "sub-1", claimedTwice))
	h.others = []db.MergeFinding{bindingConflict("f2", "sub-2", `{"subject":"sub-2","reason":"claimed_twice","bound_to":"sub-1"}`)}
	cleared := releasedReservations(t)
	dispatch := svcDispatchOperation
	t.Cleanup(func() { svcDispatchOperation = dispatch })
	svcDispatchOperation = func(_ context.Context, req addonop.Request) (addonop.Result, error) {
		h.dispatched = append(h.dispatched, req)
		return addonop.Result{Outcome: addons.OutcomeRejected,
			Err: errString("This person's record on TrueNAS now names a different account from the one Syndra meant, so nothing was released. Check again to see it as it is now.")}, nil
	}
	rr := resolveFinding(t, `{"resolution":"kept","reason":"bob is sub-1's"}`)
	if rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), "RELEASE_REFUSED") || !strings.Contains(rr.Body.String(), "Check again") {
		t.Fatalf("want 409 in the add-on's words, got %d %s", rr.Code, rr.Body.String())
	}
	if len(h.forgotten) != 0 || len(h.resolvedOn) != 0 {
		t.Fatalf("nothing forgotten or settled: %v %v", h.forgotten, h.resolvedOn)
	}
	if strings.Join(*cleared, ",") != "f1:kept,f2:unbound" {
		t.Fatalf("both reservations given back: %v", *cleared)
	}
}

// The system page lists people /subjects attributes no account to, so it names none: Syndra's mirror
// id can be stale and only an apply refreshes it, which made the same refusal repeat.
func TestTheSystemPagesReleaseSendsNoAccountFromTheMirror(t *testing.T) {
	h := &releaseHarness{}
	stubRelease(t, h)
	dbGetTargetBinding = func(context.Context, string, string) (db.TargetBinding, bool, error) {
		return db.TargetBinding{Username: "roman.reigns", AccountID: strp("3999")}, true, nil
	}
	if rr := release(t, `{"confirmed":true}`); rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d %+v", rr.Code, h.dispatched)
	}
	if _, named := h.dispatched[0].Params["account_id"]; named {
		t.Fatalf("a stale mirror id must not be sent: %+v", h.dispatched[0].Params)
	}
}

// releasedReservations records every reservation given back, by finding.
func releasedReservations(t *testing.T) *[]string {
	t.Helper()
	release := dbReleaseMergeDecision
	t.Cleanup(func() { dbReleaseMergeDecision = release })
	var out []string
	dbReleaseMergeDecision = func(_ context.Context, id, _, decision string) error {
		out = append(out, id+":"+decision)
		return nil
	}
	return &out
}

// Keeping it for a person who is no longer tracked (untracked elsewhere since) would leave the
// account bound to nobody: refused in words with a next step, nothing released, both reservations
// given back. The same on a Finish, which runs the same call again.
func TestKeepingItForSomeoneNoLongerTrackedIsRefused(t *testing.T) {
	for _, decided := range []string{"", "kept"} {
		t.Run("decided="+decided, func(t *testing.T) {
			f := bindingConflict("f1", "sub-1", claimedTwice)
			f.Decision = decided
			h := stubConflictActions(t, f)
			h.tracked = false
			h.others = []db.MergeFinding{bindingConflict("f2", "sub-2", `{"subject":"sub-2","reason":"claimed_twice","bound_to":"sub-1"}`)}
			cleared := releasedReservations(t)
			rr := resolveFinding(t, `{"resolution":"kept","reason":"bob is sub-1's"}`)
			if rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), "no longer tracked") || !strings.Contains(rr.Body.String(), "Check again") {
				t.Fatalf("want 409 in words with a next step, got %d %s", rr.Code, rr.Body.String())
			}
			if len(h.dispatched) != 0 || len(h.forgotten) != 0 {
				t.Fatalf("nothing released or forgotten: %v %v", h.dispatched, h.forgotten)
			}
			if decided == "" && strings.Join(*cleared, ",") != "f1:kept,f2:unbound" {
				t.Fatalf("both reservations given back: %v", *cleared)
			}
		})
	}
}

type errString string

func (e errString) Error() string { return string(e) }
