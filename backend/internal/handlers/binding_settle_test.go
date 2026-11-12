package handlers

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"testing"
	"time"

	"syndra/internal/addons"
	"syndra/internal/db"
)

// 6.3r: an adoption or release whose live answer was lost, and that the
// add-on recorded (GET /calls), is settled with the very binding half its
// handler runs after a live success, so the backend's copy ends as a live
// success would leave it. Anything less stays with a person.

// stubBindingSettle answers /calls with s and runs ResolveBindingCall's half
// as the database would (a stub, so nothing is resolved twice by it: the
// row-lock and the one-transaction half are db's, tested live there).
func stubBindingSettle(t *testing.T, s addons.Settlement) (ran *int) {
	t.Helper()
	ask, resolveBinding, resolve := addonsCallAnswer, dbResolveBindingCall, dbResolveAddonOperation
	t.Cleanup(func() { addonsCallAnswer, dbResolveBindingCall, dbResolveAddonOperation = ask, resolveBinding, resolve })
	addonsCallAnswer = func(context.Context, string, string, string) (addons.Settlement, error) { return s, nil }
	dbResolveAddonOperation = func(context.Context, string, string, bool, string, time.Duration) (db.AddonOperation, error) {
		t.Error("an adoption or release was settled by its status alone")
		return db.AddonOperation{}, nil
	}
	n := 0
	dbResolveBindingCall = func(ctx context.Context, _, _ string, _ time.Duration, half func(context.Context) error) (db.AddonOperation, error) {
		n++
		return db.AddonOperation{}, half(ctx)
	}
	return &n
}

func recorded(reply string) addons.Settlement {
	return addons.Settlement{State: addons.CallRecorded, Reply: json.RawMessage(reply)}
}

func TestARecordedAdoptionOrReleaseSettlesWithTheHandlersBindingHalf(t *testing.T) {
	sent := time.Now().Add(-time.Hour)

	t.Run("adopt", func(t *testing.T) {
		h := &adoptHarness{uid: "200001"}
		stubAdopt(t, h)
		if rr := adopt(t, `{"subject_id":"u-becky","confirmed":true}`); rr.Code != 200 || len(h.recorded) != 1 {
			t.Fatalf("live adoption: %d %s", rr.Code, rr.Body.String())
		}
		live := h.recorded[0]

		ran := stubBindingSettle(t, recorded(`{"operation":"account.adopt","subject":"u-becky","outcome":"succeeded","account_id":"200001"}`))
		row := db.AddonOperation{ID: "op_1", Target: "truenas", Operation: "account.adopt", SubjectID: "u-becky",
			ActorID: live.BoundBy, About: "charlotte", ClaimedAt: &sent}
		if open := settleFromCalls(context.Background(), []db.AddonOperation{row}); len(open) != 0 || *ran != 1 {
			t.Fatalf("a recorded adoption must settle with its half: open %v, ran %d", open, *ran)
		}
		if len(h.recorded) != 2 || !reflect.DeepEqual(h.recorded[1], live) {
			t.Fatalf("the settle must leave the binding the live handler leaves: live %+v, settle %+v", live, h.recorded[1:])
		}
	})

	t.Run("adopt recorded without its account id", func(t *testing.T) {
		h := &adoptHarness{}
		stubAdopt(t, h)
		ran := stubBindingSettle(t, recorded(`{"operation":"account.adopt","subject":"u-becky","outcome":"succeeded"}`))
		row := db.AddonOperation{ID: "op_1", Target: "truenas", Operation: "account.adopt", SubjectID: "u-becky",
			ActorID: "op_lita", About: "charlotte", ClaimedAt: &sent}
		if open := settleFromCalls(context.Background(), []db.AddonOperation{row}); len(open) != 1 || *ran != 0 || len(h.recorded) != 0 {
			t.Fatalf("no account id: want it left for a person, open %d, ran %d, recorded %v", len(open), *ran, h.recorded)
		}
	})

	t.Run("release", func(t *testing.T) {
		h := &releaseHarness{}
		stubRelease(t, h)
		if rr := release(t, `{"confirmed":true}`); rr.Code != 200 {
			t.Fatalf("live release: %d %s", rr.Code, rr.Body.String())
		}
		live := []string{h.forgotten[0], h.basesForgotten[0]}

		ran := stubBindingSettle(t, recorded(`{"operation":"account.release","subject":"u1","outcome":"succeeded"}`))
		row := db.AddonOperation{ID: "op_r", Target: "truenas", Operation: "account.release", SubjectID: "u1",
			ActorID: "op_lita", About: "roman.reigns", ClaimedAt: &sent}
		if open := settleFromCalls(context.Background(), []db.AddonOperation{row}); len(open) != 0 || *ran != 1 {
			t.Fatalf("a recorded release must settle with its half: open %v, ran %d", open, *ran)
		}
		if len(h.forgotten) != 2 || len(h.basesForgotten) != 2 ||
			!slices.Equal([]string{h.forgotten[1], h.basesForgotten[1]}, live) {
			t.Fatalf("the settle must forget what the live handler forgets: %v %v", h.forgotten, h.basesForgotten)
		}
	})

	// in_doubt and unknown are unchanged: neither is physical, so neither
	// settles, and no binding is touched.
	for _, state := range []string{addons.CallInDoubt, addons.CallUnknown} {
		t.Run(state, func(t *testing.T) {
			h := &adoptHarness{}
			stubAdopt(t, h)
			ran := stubBindingSettle(t, addons.Settlement{State: state})
			rows := []db.AddonOperation{
				{ID: "a", Target: "truenas", Operation: "account.adopt", SubjectID: "u-becky", About: "charlotte", ClaimedAt: &sent},
				{ID: "r", Target: "truenas", Operation: "account.release", SubjectID: "u1", ClaimedAt: &sent},
			}
			if open := settleFromCalls(context.Background(), rows); len(open) != 2 || *ran != 0 || len(h.recorded) != 0 {
				t.Fatalf("%s must stay with a person: open %d, ran %d", state, len(open), *ran)
			}
		})
	}

	// The binding was acted on again since: db refuses, nothing is settled.
	t.Run("decided since", func(t *testing.T) {
		stubBindingSettle(t, recorded(`{"account_id":"200001"}`))
		dbResolveBindingCall = func(context.Context, string, string, time.Duration, func(context.Context) error) (db.AddonOperation, error) {
			return db.AddonOperation{}, db.ErrBindingDecidedSince
		}
		row := db.AddonOperation{ID: "a", Target: "truenas", Operation: "account.adopt", SubjectID: "u-becky", About: "charlotte", ClaimedAt: &sent}
		if open := settleFromCalls(context.Background(), []db.AddonOperation{row}); len(open) != 1 {
			t.Fatal("a row whose binding was decided since must stay with a person")
		}
	})
}
