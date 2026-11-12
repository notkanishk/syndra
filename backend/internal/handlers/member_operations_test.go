package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"syndra/internal/addons"
	"syndra/internal/auth"
	"syndra/internal/db"
	"syndra/internal/services/addonop"
)

// Slice 6c: a member acting on the world (item.checkout) through the two
// member routes, and the one settle path (GET /calls) both the member's poll
// and the operator's list take.

var checkoutOp = addons.EffectiveOperation{
	ID: "item.checkout", Scope: addons.ScopeMember, Confirm: true, Available: true,
	Params: []addons.ParamSpec{{Name: "item_id", Type: "string", Required: true, Format: addons.FormatTargetID}},
	Effect: addons.EffectPhysical, Result: "common/records.json",
}

const recordsReply = `{"operation":"item.checkout","subject":"u1","outcome":"succeeded","result":{"records":[{"id":"item-2207","label":"Soldering iron"}],"taken_at":"2026-11-12T09:00:00Z","current":true}}`

type actionHarness struct {
	*storageHarness
	op        addons.EffectiveOperation
	opErr     error
	previews  []addonop.Request
	preview   addons.ReadReply
	previewEr error
	confirms  []addonop.Request
	result    addonop.Result
	resultErr error
	open      time.Time // when the subject's open call was sent; zero for none
}

func stubMemberAction(t *testing.T) *actionHarness {
	t.Helper()
	h := &actionHarness{storageHarness: &storageHarness{entitled: true, bound: true}, op: checkoutOp,
		preview: addons.ReadReply{Operation: "item.checkout", Schema: "common/records.json", Results: true,
			Outcome: addons.OutcomeSucceeded, Body: []byte(recordsReply)}}
	stubMyStorage(t, h.storageHarness)
	resolve, ops, preview, dispatch, open := addonsResolveOperation, addonsOperations, addonopPreview, svcDispatchOperation, dbOpenCall
	t.Cleanup(func() {
		addonsResolveOperation, addonsOperations, addonopPreview, svcDispatchOperation, dbOpenCall = resolve, ops, preview, dispatch, open
	})
	dbOpenCall = func(context.Context, string, string, string) (string, time.Time, bool, error) {
		return "op-open", h.open, !h.open.IsZero(), nil
	}
	addonsResolveOperation = func(string, string) (addons.EffectiveOperation, error) { return h.op, h.opErr }
	addonsOperations = func(string) []addons.EffectiveOperation { return []addons.EffectiveOperation{h.op} }
	addonopPreview = func(_ context.Context, req addonop.Request) (addons.ReadReply, error) {
		h.previews = append(h.previews, req)
		return h.preview, h.previewEr
	}
	svcDispatchOperation = func(_ context.Context, req addonop.Request) (addonop.Result, error) {
		h.confirms = append(h.confirms, req)
		return h.result, h.resultErr
	}
	return h
}

func act(t *testing.T, name, body string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/me/targets/truenas/operations/"+name, strings.NewReader(body))
	r.SetPathValue("target", "truenas")
	r.SetPathValue("name", name)
	r = r.WithContext(withPrincipal(r.Context(), &auth.Principal{Subject: "u1"}))
	rr := httptest.NewRecorder()
	handleMemberAction(rr, r)
	var out map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	return rr, out
}

const previewBody = `{"params":{"item_id":"item-2207"},"preview":true}`
const confirmBody = `{"params":{"item_id":"item-2207"},"confirm":true}`

// Preview, then confirm: the preview is sent unconfirmed and shows the
// would-be record; the confirm is dispatched confirmed, about the item, for the
// member alone, and answers by what the system said.
func TestAMemberActionIsPreviewedThenConfirmed(t *testing.T) {
	h := stubMemberAction(t)
	rr, out := act(t, "item.checkout", previewBody)
	if rr.Code != http.StatusOK || out["state"] != "preview" || out["outcome"] != "done" {
		t.Fatalf("preview %d %s", rr.Code, rr.Body.String())
	}
	if v, _ := out["view"].(map[string]any); v["readable"] != true || v["shape"] != "records" {
		t.Errorf("the preview's record is not shown: %v", out["view"])
	}
	if len(h.previews) != 1 || len(h.confirms) != 0 {
		t.Fatalf("a preview confirmed something: %d previews %d confirms", len(h.previews), len(h.confirms))
	}
	p := h.previews[0]
	if p.Confirmed || p.SubjectID != "u1" || p.ActorID != "u1" || p.About != "item-2207" {
		t.Errorf("preview request %+v", p)
	}

	h.preview.NoChange = true
	if _, out := act(t, "item.checkout", previewBody); out["outcome"] != "no_change" {
		t.Errorf("a preview of an item already held: %v", out)
	}

	view := addons.ViewOf("truenas", "item.checkout", []byte(recordsReply), addons.Clock{})
	for _, tc := range []struct {
		name   string
		result addonop.Result
		err    error
		code   int
		state  string
	}{
		{"done", addonop.Result{OperationID: "op_1", Outcome: addons.OutcomeSucceeded, View: &view}, nil, 200, "done"},
		{"no_change", addonop.Result{OperationID: "op_1", Outcome: addons.OutcomeSucceeded, NoChange: true}, nil, 200, "no_change"},
		{"a lost answer", addonop.Result{OperationID: "op_1", Outcome: addons.OutcomeIndeterminate}, nil, 202, "checking"},
		{"sent, outcome not written", addonop.Result{OperationID: "op_1", Outcome: addons.OutcomeSucceeded}, errors.New("db"), 202, "checking"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h.result, h.resultErr, h.confirms = tc.result, tc.err, nil
			rr, out := act(t, "item.checkout", confirmBody)
			if rr.Code != tc.code || out["state"] != tc.state || out["operation"] != "op_1" {
				t.Fatalf("%d %s, want %d %s", rr.Code, rr.Body.String(), tc.code, tc.state)
			}
			if c := h.confirms[0]; !c.Confirmed || c.SubjectID != "u1" || c.ActorID != "u1" || c.About != "item-2207" || c.RetryOf != "" {
				t.Errorf("confirm request %+v", c)
			}
		})
	}
}

// One of the two, always: neither, both, or a field the route does not know is
// refused before anything is sent.
func TestAMemberActionIsExactlyAPreviewOrAConfirm(t *testing.T) {
	h := stubMemberAction(t)
	for _, body := range []string{
		`{"params":{"item_id":"x"}}`,
		`{"params":{"item_id":"x"},"preview":true,"confirm":true}`,
		`{"params":{"item_id":"x"},"confirm":true,"call_id":"mine"}`,
	} {
		if rr, _ := act(t, "item.checkout", body); rr.Code != http.StatusBadRequest {
			t.Errorf("%s answered %d", body, rr.Code)
		}
	}
	if len(h.previews)+len(h.confirms) != 0 {
		t.Fatalf("a malformed request was sent")
	}
}

// Only a member-scoped physical operation the add-on offers now, for a member
// with an account on a system that answers: anything else is refused, and
// nothing goes out.
func TestOnlyAnOfferedActionGoesOut(t *testing.T) {
	cases := []struct {
		name string
		set  func(*actionHarness)
		code int
		err  string
	}{
		{"a credential operation", func(h *actionHarness) {
			h.op = addons.EffectiveOperation{ID: "password.set", Scope: addons.ScopeMember, Effect: addons.EffectCredential}
		}, 404, "NO_SUCH_ACTION"},
		{"an admin operation", func(h *actionHarness) { h.op.Scope = addons.ScopeAdmin }, 404, "NO_SUCH_ACTION"},
		{"withdrawn by the add-on", func(h *actionHarness) {
			h.opErr = &addons.ErrOperationUnavailable{Target: "truenas", ID: "item.checkout", Reason: "no calls"}
		}, 409, "ACTION_UNAVAILABLE"},
		{"no account yet", func(h *actionHarness) { h.bound = false }, 409, "ACCOUNT_PENDING"},
		{"paused", func(h *actionHarness) { h.lifecycle = "read_only" }, 409, "TARGET_PAUSED"},
		{"not answering", func(h *actionHarness) { h.unreachable = true }, 503, "TARGET_UNREACHABLE"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := stubMemberAction(t)
			tc.set(h)
			for _, body := range []string{previewBody, confirmBody} {
				rr, out := act(t, "item.checkout", body)
				if rr.Code != tc.code || out["error"] != tc.err {
					t.Errorf("%s: %d %s", body, rr.Code, rr.Body.String())
				}
			}
			if len(h.previews)+len(h.confirms) != 0 {
				t.Fatalf("sent %d previews %d confirms", len(h.previews), len(h.confirms))
			}
		})
	}
}

// Each refusal keeps its meaning on the way to the page (RFC 9110: 404 names
// nothing, 403 not entitled, 409 a conflict with the current state) and names
// what it is about, never a value; a refusal Syndra made itself (an earlier
// call unsettled) names that call.
func TestARefusalKeepsItsMeaning(t *testing.T) {
	limit, count := 1, 1
	for _, tc := range []struct {
		code    string
		refusal addons.Refusal
		status  int
		error   string
		details map[string]any
	}{
		{"UNKNOWN_RESOURCE", addons.Refusal{Param: "item_id"}, 404, "UNKNOWN_RESOURCE", map[string]any{"param": "item_id"}},
		{"NOT_ENTITLED", addons.Refusal{Param: "item_id"}, 403, "NOT_ENTITLED", map[string]any{"param": "item_id"}},
		{"RESOURCE_TAKEN", addons.Refusal{Param: "item_id"}, 409, "RESOURCE_TAKEN", map[string]any{"param": "item_id"}},
		{"LIMIT_REACHED", addons.Refusal{Limit: &limit, Unit: "items", Count: &count}, 409, "LIMIT_REACHED",
			map[string]any{"limit": "1", "count": "1", "unit": "items"}},
		{"CALL_UNSETTLED", addons.Refusal{}, 409, "CALL_UNSETTLED", nil},
		{"OPERATION_FAILED", addons.Refusal{}, 422, "ACTION_REFUSED", nil},
	} {
		t.Run(tc.code, func(t *testing.T) {
			h := stubMemberAction(t)
			h.preview = addons.ReadReply{Outcome: addons.OutcomeRejected, Code: tc.code, Refusal: tc.refusal}
			h.result = addonop.Result{OperationID: "op_1", Outcome: addons.OutcomeRejected, Code: tc.code, Refusal: tc.refusal}
			for _, body := range []string{previewBody, confirmBody} {
				rr, out := act(t, "item.checkout", body)
				if rr.Code != tc.status || out["error"] != tc.error {
					t.Fatalf("%s: %d %s", body, rr.Code, rr.Body.String())
				}
				got, _ := json.Marshal(out["details"])
				want, _ := json.Marshal(tc.details)
				if string(got) != string(want) {
					t.Errorf("details %s, want %s", got, want)
				}
				if strings.Contains(rr.Body.String(), "item-2207") {
					t.Errorf("a refusal echoed the value typed: %s", rr.Body.String())
				}
			}
		})
	}

	h := stubMemberAction(t)
	sent := time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)
	h.resultErr = &addonop.UnsettledError{OperationID: "op_earlier", SentAt: sent}
	rr, out := act(t, "item.checkout", confirmBody)
	if d, _ := out["details"].(map[string]any); rr.Code != 409 || out["error"] != "CALL_UNSETTLED" || d["operation"] != "op_earlier" || d["sent_at"] != "2026-11-12T09:00:00Z" {
		t.Errorf("an unsettled earlier call: %d %s", rr.Code, rr.Body.String())
	}
	h.resultErr = addonop.ErrCallUnsettled
	if rr, _ := act(t, "item.checkout", confirmBody); rr.Code != http.StatusServiceUnavailable {
		t.Errorf("an unreadable earlier call: %d", rr.Code)
	}
}

// pollHarness is the record table and the add-on's /calls, as the poll and
// the list read them.
type pollHarness struct {
	rows     map[string]db.AddonOperation
	answer   addons.Settlement
	answerEr error
	asked    []string
	resolved map[string]bool
	unsent   int // SettleUnclaimedAddonOperations calls
}

func stubPoll(t *testing.T) *pollHarness {
	t.Helper()
	p := &pollHarness{rows: map[string]db.AddonOperation{}, resolved: map[string]bool{}}
	byID, ask, resolve, unsent := dbAddonOperationByID, addonsCallAnswer, dbResolveAddonOperation, dbSettleUnclaimedAddonOperations
	t.Cleanup(func() {
		dbAddonOperationByID, addonsCallAnswer, dbResolveAddonOperation, dbSettleUnclaimedAddonOperations = byID, ask, resolve, unsent
	})
	// The table's own rule: an unclaimed dispatching row past the grace is unreached.
	dbSettleUnclaimedAddonOperations = func(_ context.Context, target string, grace time.Duration) (int64, error) {
		p.unsent++
		var n int64
		for id, o := range p.rows {
			if o.Target == target && o.ClaimedAt == nil && o.Status == db.AddonOpDispatching && time.Since(o.CreatedAt) > grace {
				o.Status = db.AddonOpUnreached
				p.rows[id] = o
				n++
			}
		}
		return n, nil
	}
	dbAddonOperationByID = func(_ context.Context, target, subject, id string) (db.AddonOperation, error) {
		o, ok := p.rows[id]
		if !ok || o.SubjectID != subject || o.Target != target {
			return db.AddonOperation{}, db.ErrAddonOperationNotFound
		}
		return o, nil
	}
	addonsCallAnswer = func(_ context.Context, target, op, id string) (addons.Settlement, error) {
		p.asked = append(p.asked, id)
		return p.answer, p.answerEr
	}
	dbResolveAddonOperation = func(_ context.Context, _, id string, happened bool, by string, _ time.Duration) (db.AddonOperation, error) {
		if by != db.SyndraReader {
			t.Errorf("settled in %q's name, want Syndra's reading", by)
		}
		p.resolved[id] = happened
		o := p.rows[id]
		o.Resolution = map[bool]string{true: db.AddonResolutionHappened, false: db.AddonResolutionNotHappened}[happened]
		p.rows[id] = o
		return o, nil
	}
	return p
}

const opID = "0b7d4c1e-6a5f-4e2b-9c3d-8f1a2e4b6c70"

func poll(t *testing.T, id string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/me/targets/truenas/operations/"+id, nil)
	r.SetPathValue("target", "truenas")
	r.SetPathValue("id", id)
	r = r.WithContext(withPrincipal(r.Context(), &auth.Principal{Subject: "u1"}))
	rr := httptest.NewRecorder()
	handleMemberActionStatus(rr, r)
	var out map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	return rr, out
}

// The poll asks the add-on what it recorded for an open confirm and settles
// the row on a clear answer (recorded happened, unknown within call_memory
// did not, an older unknown is the add-on forgetting); in_doubt settles nothing, and past constants.in_doubt_bound the member is told the
// desk has it. A settled row is answered from the record alone.
func TestTheMemberPollSettlesFromTheAddonsRecord(t *testing.T) {
	recent := time.Now().Add(-time.Minute)
	old := time.Now().Add(-addons.InDoubtBound - time.Minute)
	row := func(status string, claimed time.Time) db.AddonOperation {
		return db.AddonOperation{ID: opID, Target: "truenas", Operation: "item.checkout", SubjectID: "u1",
			Status: status, CreatedAt: claimed, ClaimedAt: &claimed}
	}
	for _, tc := range []struct {
		name     string
		row      db.AddonOperation
		answer   addons.Settlement
		state    string
		asked    bool
		resolved map[string]bool
		view     bool
	}{
		{"recorded", row(db.AddonOpIndeterminate, recent), addons.Settlement{State: addons.CallRecorded, Reply: []byte(recordsReply)},
			"done", true, map[string]bool{opID: true}, true},
		{"recorded, no change", row(db.AddonOpIndeterminate, recent), addons.Settlement{State: addons.CallRecorded,
			Reply: []byte(strings.Replace(recordsReply, `"succeeded"`, `"no_change"`, 1))}, "no_change", true, map[string]bool{opID: true}, true},
		{"unknown", row(db.AddonOpIndeterminate, recent), addons.Settlement{State: addons.CallUnknown}, "not_done", true, map[string]bool{opID: false}, false},
		{"in doubt", row(db.AddonOpIndeterminate, recent), addons.Settlement{State: addons.CallInDoubt}, "checking", true, map[string]bool{}, false},
		{"in doubt past the bound", row(db.AddonOpIndeterminate, old), addons.Settlement{State: addons.CallInDoubt}, "ask_desk", true, map[string]bool{}, false},
		{"unknown past call_memory", row(db.AddonOpIndeterminate, time.Now().Add(-addons.CallMemory-time.Hour)), addons.Settlement{State: addons.CallUnknown}, "ask_desk", true, map[string]bool{}, false},
		{"done", row(db.AddonOpSucceeded, recent), addons.Settlement{}, "done", false, map[string]bool{}, false},
		{"no change", row(db.AddonOpNoChange, recent), addons.Settlement{}, "no_change", false, map[string]bool{}, false},
		{"refused", row(db.AddonOpRejected, recent), addons.Settlement{}, "not_done", false, map[string]bool{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := stubPoll(t)
			p.rows[opID], p.answer = tc.row, tc.answer
			rr, out := poll(t, opID)
			if rr.Code != 200 || out["state"] != tc.state || out["operation"] != opID {
				t.Fatalf("%d %s, want %s", rr.Code, rr.Body.String(), tc.state)
			}
			if (len(p.asked) == 1) != tc.asked {
				t.Errorf("asked /calls %v, want %t", p.asked, tc.asked)
			}
			if len(p.resolved) != len(tc.resolved) || (len(tc.resolved) == 1 && p.resolved[opID] != tc.resolved[opID]) {
				t.Errorf("settled %v, want %v", p.resolved, tc.resolved)
			}
			if _, has := out["view"]; has != tc.view {
				t.Errorf("view present %t, want %t: %s", has, tc.view, rr.Body.String())
			}
		})
	}

	// Their own physical action only: someone else's, a password change, or
	// a word that is not an id is not found, and nothing is asked.
	p := stubPoll(t)
	other := row(db.AddonOpIndeterminate, recent)
	other.SubjectID = "u2"
	p.rows[opID] = other
	pw := row(db.AddonOpIndeterminate, recent)
	pw.ID, pw.Operation = "1b7d4c1e-6a5f-4e2b-9c3d-8f1a2e4b6c70", "password.set"
	p.rows[pw.ID] = pw
	for _, id := range []string{opID, pw.ID, "not-an-id"} {
		if rr, _ := poll(t, id); rr.Code != http.StatusNotFound {
			t.Errorf("%s answered %d", id, rr.Code)
		}
	}
	if len(p.asked) != 0 {
		t.Errorf("asked about %v", p.asked)
	}
}

// A record a crash left unclaimed was never sent. The member's page shows it
// as checking (it may be about to go), and the poll settles it as the
// operator's list does: unreached once past the grace, so not done. /calls is
// never asked about a call that never left.
func TestTheMemberPollSettlesAnUnsentCallAsUnreached(t *testing.T) {
	for _, tc := range []struct {
		name  string
		ago   time.Duration
		state string
	}{
		{"young", time.Second, "checking"},
		{"past the grace", unresolvedGrace + time.Minute, "not_done"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := stubPoll(t)
			p.rows[opID] = db.AddonOperation{ID: opID, Target: "truenas", Operation: "item.checkout", SubjectID: "u1",
				Status: db.AddonOpDispatching, CreatedAt: time.Now().Add(-tc.ago)}
			rr, out := poll(t, opID)
			if rr.Code != 200 || out["state"] != tc.state {
				t.Fatalf("%d %s, want %s", rr.Code, rr.Body.String(), tc.state)
			}
			if p.unsent != 1 || len(p.asked) != 0 || len(p.resolved) != 0 {
				t.Errorf("unsent settles %d, asked %v, resolved %v", p.unsent, p.asked, p.resolved)
			}
		})
	}
	// A sent row is the add-on's to settle, never Syndra's unreached.
	p := stubPoll(t)
	sent := time.Now().Add(-time.Hour)
	p.rows[opID] = db.AddonOperation{ID: opID, Target: "truenas", Operation: "item.checkout", SubjectID: "u1",
		Status: db.AddonOpIndeterminate, CreatedAt: sent, ClaimedAt: &sent}
	p.answer = addons.Settlement{State: addons.CallInDoubt}
	if poll(t, opID); p.unsent != 0 {
		t.Errorf("a sent row went to the unsent settle")
	}
}

// The operator's list takes the same path first. recorded settles any row as
// happened. unknown settles only a physical row sent within
// constants.call_memory as not happened: a non-physical row's add-on may keep
// no write-ahead record (TrueNAS in production), and an older row's add-on
// may have forgotten it; both stay for a person, as in_doubt does. An
// adoption or release is never asked, so it stays with a person whose repeat
// writes the backend's binding. An add-on that cannot answer settles nothing.
func TestCallsSettleOnlyWhatTheAddonCanVouchFor(t *testing.T) {
	p := stubPoll(t)
	answers := map[string]addons.Settlement{
		"rec": {State: addons.CallRecorded}, "unk": {State: addons.CallUnknown}, "doubt": {State: addons.CallInDoubt},
		"phys-new": {State: addons.CallUnknown}, "phys-old": {State: addons.CallUnknown}, "phys-edge": {State: addons.CallUnknown},
		"adopt": {State: addons.CallRecorded}, "release": {State: addons.CallUnknown},
	}
	addonsCallAnswer = func(_ context.Context, _, _, id string) (addons.Settlement, error) {
		p.asked = append(p.asked, id)
		return answers[id], nil
	}
	sent := func(ago time.Duration) *time.Time { at := time.Now().Add(-ago); return &at }
	ops := []db.AddonOperation{
		{ID: "rec", Target: "truenas", Operation: "account.delete", ClaimedAt: sent(time.Hour)},
		{ID: "unk", Target: "truenas", Operation: "account.delete", ClaimedAt: sent(time.Hour)},
		{ID: "doubt", Target: "truenas", Operation: "account.delete", ClaimedAt: sent(time.Hour)},
		{ID: "phys-new", Target: "inventory", Operation: "item.checkout", ClaimedAt: sent(2 * addons.InDoubtBound)},
		{ID: "phys-old", Target: "inventory", Operation: "item.checkout", ClaimedAt: sent(addons.CallMemory + time.Hour)},
		{ID: "phys-edge", Target: "inventory", Operation: "item.checkout", ClaimedAt: sent(addons.CallMemory - time.Second)},
		{ID: "adopt", Target: "truenas", Operation: "account.adopt", ClaimedAt: sent(time.Hour)},
		{ID: "release", Target: "truenas", Operation: "account.release", ClaimedAt: sent(time.Hour)},
	}
	var ids []string
	for _, o := range settleFromCalls(context.Background(), ops) {
		ids = append(ids, o.ID)
	}
	if got := strings.Join(ids, ","); got != "unk,doubt,phys-old,phys-edge,adopt,release" {
		t.Fatalf("open %s", got)
	}
	if len(p.resolved) != 2 || !p.resolved["rec"] || p.resolved["phys-new"] {
		t.Errorf("settled %v, want rec happened and phys-new not", p.resolved)
	}
	if slices.Contains(p.asked, "adopt") || slices.Contains(p.asked, "release") {
		t.Errorf("asked about an adoption or release: %v", p.asked)
	}

	p = stubPoll(t)
	p.answerEr = addons.ErrFeatureNotDeclared
	if open := settleFromCalls(context.Background(), ops[:3]); len(open) != 3 || len(p.asked) != 1 || len(p.resolved) != 0 {
		t.Errorf("an add-on without calls: open %d, asked %v, settled %v", len(open), p.asked, p.resolved)
	}
}

// The member's page offers the action with its form (an item id, typed or
// scanned) and keeps saying where an unsettled confirm stands across a reload.
func TestTheMemberViewOffersTheActionAndItsUnsettledCall(t *testing.T) {
	h := stubMemberAction(t)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/me/targets", nil)
	r = r.WithContext(withPrincipal(r.Context(), &auth.Principal{Subject: "u1"}))
	view, err := describeMyTarget(r, "truenas", "u1")
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Actions) != 1 || view.Actions[0].ID != "item.checkout" || !view.Actions[0].Confirm ||
		view.Actions[0].Params[0].Format != "target_id" || view.Unsettled != nil {
		t.Fatalf("actions %+v unsettled %+v", view.Actions, view.Unsettled)
	}

	h.open = time.Now().Add(-addons.InDoubtBound - time.Minute)
	view, _ = describeMyTarget(r, "truenas", "u1")
	if view.Unsettled == nil || view.Unsettled.ID != "op-open" || view.Unsettled.State != "ask_desk" {
		t.Errorf("unsettled %+v", view.Unsettled)
	}

	h.bound = false
	if view, _ = describeMyTarget(r, "truenas", "u1"); view.Actions != nil {
		t.Errorf("an action offered with no account: %+v", view.Actions)
	}
}

// The list asks /calls before it shows a row: one the add-on recorded is
// settled and gone, and the list passes the physical hold to the read.
func TestTheListSettlesFromTheAddonsRecordFirst(t *testing.T) {
	p := stubPoll(t)
	p.answer = addons.Settlement{State: addons.CallRecorded}
	unclaimed, list := dbSettleUnclaimedAddonOperations, dbListUnresolvedAddonOperations
	t.Cleanup(func() { dbSettleUnclaimedAddonOperations, dbListUnresolvedAddonOperations = unclaimed, list })
	dbSettleUnclaimedAddonOperations = func(context.Context, string, time.Duration) (int64, error) { return 0, nil }
	sent := time.Now().Add(-2 * addons.InDoubtBound)
	var hold db.Hold
	dbListUnresolvedAddonOperations = func(_ context.Context, target string, _ time.Duration, _ int, h db.Hold) ([]db.AddonOperation, error) {
		hold = h
		return []db.AddonOperation{{ID: opID, Target: target, Operation: "item.checkout", SubjectID: "u1", ClaimedAt: &sent}}, nil
	}
	r := httptest.NewRequest(http.MethodGet, "/api/v1/targets/truenas/unconfirmed", nil)
	r.SetPathValue("target", "truenas")
	rr := httptest.NewRecorder()
	handleListUnconfirmed(rr, r)
	if rr.Code != 200 || strings.Contains(rr.Body.String(), opID) || !p.resolved[opID] {
		t.Fatalf("%d %s, settled %v", rr.Code, rr.Body.String(), p.resolved)
	}
	if hold.For != addons.InDoubtBound || len(hold.Operations) != 1 || hold.Operations[0] != "item.checkout" {
		t.Errorf("the list read with hold %+v", hold)
	}
}
