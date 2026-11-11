package addonop

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"syndra/internal/addons"
	"syndra/internal/db"
)

// Slice 6c: the physical rules the backend holds before a record exists, and
// what it records after.

const member = "289471021834760193"

// physicalHarness is the harness with item.checkout resolved (the policy's
// entry) and the subject's earlier calls readable.
func physicalHarness(t *testing.T) (*harness, *unsettledStub) {
	t.Helper()
	h := newHarness(t)
	u := &unsettledStub{passwordOp: h.op}
	h.op = addons.EffectiveOperation{
		ID: "item.checkout", Scope: addons.ScopeMember, Confirm: true, Available: true,
		Params: []addons.ParamSpec{{Name: "item_id", Type: "string", Required: true, Format: addons.FormatTargetID}},
		Effect: addons.EffectPhysical, Result: "common/records.json",
	}
	saved := unconfirmedChangeAt
	unconfirmedChangeAt = func(_ context.Context, target, subject, operation string, grace time.Duration) (string, time.Time, bool, error) {
		h.record("unsettled?")
		u.asked = append(u.asked, strings.Join([]string{target, subject, operation, grace.String()}, "|"))
		return u.id, u.at, u.open, u.err
	}
	t.Cleanup(func() { unconfirmedChangeAt = saved })
	return h, u
}

type unsettledStub struct {
	// passwordOp is the harness's own password.set, for the tests that
	// need a non-physical operation.
	passwordOp addons.EffectiveOperation
	id         string
	at         time.Time
	open       bool
	err        error
	asked      []string
}

func checkout() Request {
	return Request{Target: "inventory", Operation: "item.checkout", ActorID: member, SubjectID: member,
		Confirmed: true, Params: map[string]any{"item_id": "item-2207"}, About: "item-2207"}
}

// Physical rule 2: nobody sends a physical operation again. The one automatic
// resend password.set has (RetryOf) is refused, before any record.
func TestAPhysicalOperationIsNeverResent(t *testing.T) {
	h, _ := physicalHarness(t)
	req := checkout()
	req.RetryOf = "rec-0000"
	_, err := Dispatch(context.Background(), req)
	if !errors.Is(err, ErrPhysicalNotResent) {
		t.Fatalf("err %v, want ErrPhysicalNotResent", err)
	}
	if len(h.begun) != 0 || len(h.calls) != 0 {
		t.Fatalf("a resend left a record or a call: %s", h.order())
	}
}

// Physical rule 2, the backend's half of CALL_UNSETTLED: while the subject's
// earlier call of this operation is sent and unsettled (grace 0: a claimed
// call still awaiting its answer counts), a new one is refused before it has
// a record, naming the earlier one. Not knowing is refusing.
func TestAnUnsettledPhysicalCallRefusesTheNext(t *testing.T) {
	h, u := physicalHarness(t)
	sent := time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)
	u.id, u.at, u.open = "rec-earlier", sent, true
	_, err := Dispatch(context.Background(), checkout())
	var unsettled *UnsettledError
	if !errors.As(err, &unsettled) || !errors.Is(err, ErrCallUnsettled) || unsettled.OperationID != "rec-earlier" || !unsettled.SentAt.Equal(sent) {
		t.Fatalf("err %v, want the earlier call named", err)
	}
	if len(h.begun) != 0 || len(h.calls) != 0 {
		t.Fatalf("a refused retry left a record or a call: %s", h.order())
	}
	if want := "inventory|" + member + "|item.checkout|0s"; len(u.asked) != 1 || u.asked[0] != want {
		t.Errorf("asked %v, want %q", u.asked, want)
	}

	u.open, u.err = false, errors.New("db down")
	h.begun, h.calls = nil, nil
	if _, err := Dispatch(context.Background(), checkout()); !errors.Is(err, ErrCallUnsettled) || len(h.calls) != 0 {
		t.Fatalf("an unreadable record let a call through: %v, %d calls", err, len(h.calls))
	}

	// A settled earlier call is no obstacle, and the check comes after
	// confirmation and before the record.
	u.err = nil
	h.steps = nil
	if _, err := Dispatch(context.Background(), checkout()); err != nil {
		t.Fatal(err)
	}
	if got := h.order(); !strings.Contains(got, "unsettled? -> begin -> mint -> call") {
		t.Errorf("order %s", got)
	}
}

// A non-physical operation never asks: password.set has its own settling.
func TestOnlyAPhysicalCallAsksWhetherOneIsUnsettled(t *testing.T) {
	h, u := physicalHarness(t)
	h.op = u.passwordOp
	if _, err := Dispatch(context.Background(), passwordSet()); err != nil {
		t.Fatal(err)
	}
	if len(u.asked) != 0 {
		t.Errorf("password.set asked %v", u.asked)
	}
}

// 6.3a: an add-on answering no_change is recorded no_change, never succeeded,
// and the result says so with the record it returned; a refusal carries its
// code and what it names to the page.
func TestANoChangeReplyIsRecordedNoChange(t *testing.T) {
	h, _ := physicalHarness(t)
	h.resp = addons.CallResponse{Outcome: addons.OutcomeSucceeded, Status: 200, NoChange: true,
		Body: []byte(`{"outcome":"no_change","result":{"records":[{"id":"item-2207","label":"Soldering iron"}],"taken_at":"2026-11-12T09:00:00Z","current":true}}`)}
	res, err := Dispatch(context.Background(), checkout())
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != db.AddonOpNoChange || !res.NoChange || h.settled[0] != "rec-0001="+db.AddonOpNoChange {
		t.Fatalf("recorded %v, result %+v", h.settled, res)
	}
	if res.View == nil || !res.View.Readable || res.View.Shape != "records" {
		t.Errorf("the record it returned is not shown: %+v", res.View)
	}
	if h.begun[0].About != "item-2207" {
		t.Errorf("the record is about %q, want the item", h.begun[0].About)
	}

	h.resp = addons.CallResponse{Outcome: addons.OutcomeRejected, Status: 409, Code: "LIMIT_REACHED",
		Refusal: addons.Refusal{Unit: "items"}}
	res, _ = Dispatch(context.Background(), checkout())
	if res.Code != "LIMIT_REACHED" || res.Refusal.Unit != "items" || res.View != nil {
		t.Errorf("a refusal's code and what it names did not reach the page: %+v", res)
	}
}

// Physical rule 1: each confirm is its own record, so its own fresh call_id;
// nothing reuses an id a preview or an earlier confirm had.
func TestTwoConfirmsCarryTwoFreshCallIDs(t *testing.T) {
	h, _ := physicalHarness(t)
	ids := []string{"rec-a", "rec-b"}
	beginOperation = func(_ context.Context, p db.AddonOperationParams) (string, error) {
		h.begun = append(h.begun, p)
		id := ids[0]
		ids = ids[1:]
		return id, nil
	}
	for range 2 {
		if _, err := Dispatch(context.Background(), checkout()); err != nil {
			t.Fatal(err)
		}
	}
	if len(h.begun) != 2 || len(h.verified) != 2 || !strings.HasPrefix(h.verified[0], "rec-a|") || !strings.HasPrefix(h.verified[1], "rec-b|") {
		t.Fatalf("records %v, minted for %v", h.begun, h.verified)
	}
}

// The preview runs the confirm's checks and none of its writes: no record, no
// rate count, no question about earlier calls, nothing claimed; a member
// operation previews only for the member acting.
func TestAPreviewRecordsAndCountsNothing(t *testing.T) {
	h, u := physicalHarness(t)
	var previewed []string
	saved := previewAddon
	previewAddon = func(_ context.Context, target, op, subject, actor string, params map[string]any) addons.ReadReply {
		previewed = append(previewed, target+"|"+op+"|"+subject+"|"+actor)
		return addons.ReadReply{Outcome: addons.OutcomeSucceeded}
	}
	t.Cleanup(func() { previewAddon = saved })

	req := checkout()
	req.Confirmed = false
	if r, err := Preview(context.Background(), req); err != nil || r.Outcome != addons.OutcomeSucceeded {
		t.Fatalf("%+v %v", r, err)
	}
	if len(h.begun) != 0 || len(h.calls) != 0 || len(u.asked) != 0 || len(h.rateQueries) != 0 || len(previewed) != 1 {
		t.Fatalf("a preview wrote or counted: %s", h.order())
	}

	req.SubjectID = "someone-else"
	if _, err := Preview(context.Background(), req); !errors.Is(err, ErrSubjectNotActor) {
		t.Errorf("a preview for another member: %v", err)
	}
	req.SubjectID, req.Params = member, map[string]any{"item_id": 7}
	if _, err := Preview(context.Background(), req); !errors.Is(err, addons.ErrInvalidParams) {
		t.Errorf("a preview with a bad param: %v", err)
	}
	h.op = u.passwordOp
	if _, err := Preview(context.Background(), passwordSet()); !errors.Is(err, addons.ErrNotPhysical) {
		t.Errorf("a password previewed: %v", err)
	}
	if len(previewed) != 1 {
		t.Errorf("a refused preview was sent: %v", previewed)
	}
}
