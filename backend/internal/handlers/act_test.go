package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"syndra/internal/db"
	"syndra/internal/models"
	"syndra/internal/services"
)

// actProbe stands in for the planner (svcApplyAct), the send (svcDeliverChange)
// and the cache rebuild, and records what the handler passed and in what order.
// What the planner decides, and what it writes, is the services' tests' subject;
// here the handler owns the status, the codes, the order and the shapes.
type actProbe struct {
	Actions []services.Action
	Writes  []services.ActWrite
	Seens   []*services.Seen

	Change   services.Change // what ApplyAct returns
	Err      error
	Delivery services.Delivery // what DeliverChange returns

	Order        []string // "apply", "rebuild", "deliver"
	RebuildUsers []string
	// RebuildDetached is true when every rebuild ran on a context the request's
	// cancellation could not reach.
	RebuildDetached bool
	Delivered       []services.Change
}

func stubAct(t *testing.T) *actProbe {
	t.Helper()
	p := &actProbe{RebuildDetached: true}
	origApply, origDeliver := svcApplyAct, svcDeliverChange
	origFn, origRebuild := rebuildUserCacheDetachedFn, cacheRebuildUser
	t.Cleanup(func() {
		svcApplyAct, svcDeliverChange = origApply, origDeliver
		rebuildUserCacheDetachedFn, cacheRebuildUser = origFn, origRebuild
	})
	svcApplyAct = func(_ context.Context, a services.Action, w services.ActWrite, seen *services.Seen) (services.Change, error) {
		p.Order = append(p.Order, "apply")
		p.Actions, p.Writes, p.Seens = append(p.Actions, a), append(p.Writes, w), append(p.Seens, seen)
		return p.Change, p.Err
	}
	svcDeliverChange = func(_ context.Context, ch services.Change) services.Delivery {
		p.Order = append(p.Order, "deliver")
		p.Delivered = append(p.Delivered, ch)
		return p.Delivery
	}
	// Both rebuild paths: the planned acts call one or the other.
	rebuildUserCacheDetachedFn = func(ctx context.Context, userID string) {
		p.Order = append(p.Order, "rebuild")
		p.RebuildUsers = append(p.RebuildUsers, userID)
		p.RebuildDetached = p.RebuildDetached && ctx.Err() == nil
	}
	cacheRebuildUser = func(ctx context.Context, userID string, _ []string) {
		p.Order = append(p.Order, "rebuild")
		p.RebuildUsers = append(p.RebuildUsers, userID)
		p.RebuildDetached = p.RebuildDetached && ctx.Err() == nil
	}
	return p
}

// one call, the Action the handler built.
func (p *actProbe) only(t *testing.T) (services.Action, services.ActWrite, *services.Seen) {
	t.Helper()
	if len(p.Actions) != 1 {
		t.Fatalf("expected one planned act, got %d", len(p.Actions))
	}
	return p.Actions[0], p.Writes[0], p.Seens[0]
}

func (p *actProbe) wantOrder(t *testing.T, want ...string) {
	t.Helper()
	if !reflect.DeepEqual(p.Order, want) {
		t.Fatalf("order = %v, want %v", p.Order, want)
	}
}

func decodeMap(t *testing.T, body string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		t.Fatalf("decode %q: %v", body, err)
	}
	return m
}

func wantError(t *testing.T, rr *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if rr.Code != status || decodeMap(t, rr.Body.String())["error"] != code {
		t.Fatalf("got %d %s, want %d %s", rr.Code, rr.Body.String(), status, code)
	}
}

// A StalePlan answers 409 PLAN_STALE and nothing after the write is touched:
// no rebuild, no send.
func TestWriteActError_Mapping(t *testing.T) {
	refused := func(code string) error {
		return &services.Refused{Change: services.Change{Refusal: code}}
	}
	cases := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"stale", &services.StalePlan{IDs: []string{"q1"}}, http.StatusConflict, "PLAN_STALE"},
		{"held", refused("held"), http.StatusConflict, "ALREADY_HELD"},
		{"not offered", refused("not_offered:p/r"), http.StatusConflict, "ROLE_NOT_OFFERED"},
		{"catalogue", refused("catalogue_unread"), http.StatusServiceUnavailable, "CATALOGUE_UNREAD"},
		{"departed", refused("departed:x"), http.StatusConflict, "DEPARTED"},
		{"retired", refused("retired"), http.StatusConflict, "RETIRED"},
		{"anything else refused", refused("odd"), http.StatusConflict, "REFUSED"},
		{"grant gone", fmt.Errorf("wrapped: %w", db.ErrGrantNotFound), http.StatusNotFound, "GRANT_NOT_FOUND"},
		{"failure", errors.New("tx failed"), http.StatusInternalServerError, "DB_ERROR"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			writeActError(rr, c.err)
			wantError(t, rr, c.status, c.code)
		})
	}
}

// GET /users/{id}/plan: what each op needs, what it hands the planner, and the
// sheet's shape.
func TestHandleActPlan_RequiredFieldsPerOp(t *testing.T) {
	orig := svcPlanAct
	t.Cleanup(func() { svcPlanAct = orig })
	svcPlanAct = func(context.Context, services.Action) (services.Change, error) {
		t.Fatal("an incomplete ask must not reach the planner")
		return services.Change{}, nil
	}
	cases := []struct{ query, field string }{
		{"op=give&role_key=r", "project_id"},
		{"op=give&project_id=p", "role_key"},
		{"op=remove", "grant_id"},
		{"op=approve", "request_id"},
		{"op=assign_bundle", "bundle_id"},
		{"op=remove_bundle", "bundle_id"},
		{"op=extend&role_key=r&duration_days=30", "project_id"},
		{"op=extend&project_id=p&duration_days=30", "role_key"},
		{"op=adopt", "drift_id"},
		{"op=adopt&drift_id=%20", "drift_id"},
		{"op=nonsense", "op"},
		{"", "op"},
	}
	for _, c := range cases {
		t.Run(c.query, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/users/u1/plan?"+c.query, nil)
			req.SetPathValue("id", "u1")
			rr := httptest.NewRecorder()
			handleActPlan(rr, req)
			wantError(t, rr, http.StatusBadRequest, "VALIDATION_FAILED")
			details, _ := decodeMap(t, rr.Body.String())["details"].(map[string]any)
			if details[c.field] != "required" {
				t.Fatalf("expected details.%s=required, got %v", c.field, details)
			}
		})
	}
}

func TestHandleActPlan_PassesTheActionThrough(t *testing.T) {
	orig := svcPlanAct
	t.Cleanup(func() { svcPlanAct = orig })
	var got services.Action
	svcPlanAct = func(_ context.Context, a services.Action) (services.Change, error) {
		got = a
		return services.Change{}, nil
	}
	cases := []struct {
		query string
		want  services.Action
	}{
		{"op=give&project_id=%20p1%20&role_key=trained", services.Action{Op: "give", UserID: "u1", ProjectID: "p1", RoleKey: "trained"}},
		{"op=remove&grant_id=g1", services.Action{Op: "remove", UserID: "u1", GrantID: "g1"}},
		{"op=approve&request_id=r1", services.Action{Op: "approve", UserID: "u1", RequestID: "r1"}},
		{"op=assign_bundle&bundle_id=b1", services.Action{Op: "assign_bundle", UserID: "u1", BundleID: "b1"}},
		{"op=remove_bundle&bundle_id=b1", services.Action{Op: "remove_bundle", UserID: "u1", BundleID: "b1"}},
		{"op=extend&project_id=p1&role_key=trained&duration_days=30",
			services.Action{Op: services.OpExtend, UserID: "u1", ProjectID: "p1", RoleKey: "trained", Days: 30}},
		{"op=adopt&drift_id=%20d1%20", services.Action{Op: services.OpAdopt, UserID: "u1", DriftID: "d1"}},
	}
	for _, c := range cases {
		t.Run(c.query, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/users/u1/plan?"+c.query, nil)
			req.SetPathValue("id", "u1")
			rr := httptest.NewRecorder()
			handleActPlan(rr, req)
			if rr.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
			}
			if got != c.want {
				t.Fatalf("planned %+v, want %+v", got, c.want)
			}
		})
	}
}

// An extension's days are a count: anything else is refused before the planner.
func TestHandleActPlan_ABadDurationIs400(t *testing.T) {
	orig := svcPlanAct
	t.Cleanup(func() { svcPlanAct = orig })
	svcPlanAct = func(context.Context, services.Action) (services.Change, error) {
		t.Fatal("a bad duration must not reach the planner")
		return services.Change{}, nil
	}
	for _, days := range []string{"soon", "-1", "3.5"} {
		t.Run(days, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/users/u1/plan?op=extend&project_id=p&role_key=r&duration_days="+days, nil)
			req.SetPathValue("id", "u1")
			rr := httptest.NewRecorder()
			handleActPlan(rr, req)
			wantError(t, rr, http.StatusBadRequest, "VALIDATION_FAILED")
			if details, _ := decodeMap(t, rr.Body.String())["details"].(map[string]any); details["duration_days"] != "min=0" {
				t.Fatalf("expected details.duration_days=min=0, got %v", details)
			}
		})
	}
}

func TestHandleActPlan_UnknownRequestIs404AndFailureIs500(t *testing.T) {
	orig := svcPlanAct
	t.Cleanup(func() { svcPlanAct = orig })
	for _, c := range []struct {
		err    error
		status int
		code   string
	}{
		{services.ErrRequestNotFound, http.StatusNotFound, "NOT_FOUND"},
		{errors.New("snapshot failed"), http.StatusInternalServerError, "DB_ERROR"},
	} {
		svcPlanAct = func(context.Context, services.Action) (services.Change, error) { return services.Change{}, c.err }
		req := httptest.NewRequest(http.MethodGet, "/api/v1/users/u1/plan?op=approve&request_id=r1", nil)
		req.SetPathValue("id", "u1")
		rr := httptest.NewRecorder()
		handleActPlan(rr, req)
		wantError(t, rr, c.status, c.code)
	}
}

// The sheet reads every list without asking whether it is there.
func TestHandleActPlan_SerialisesTheSheetAndNeverNullLists(t *testing.T) {
	orig, origUsers := svcPlanAct, directoryUsers
	t.Cleanup(func() { svcPlanAct, directoryUsers = orig, origUsers })
	directoryUsers = func(context.Context) ([]models.UserProfile, error) {
		return []models.UserProfile{{ID: "admin-1", Name: "Ada"}}, nil
	}
	const cancelID = "11111111-1111-1111-1111-111111111111"

	get := func(ch services.Change) map[string]any {
		svcPlanAct = func(context.Context, services.Action) (services.Change, error) { return ch, nil }
		req := httptest.NewRequest(http.MethodGet, "/api/v1/users/u1/plan?op=give&project_id=p&role_key=r", nil)
		req.SetPathValue("id", "u1")
		rr := httptest.NewRecorder()
		handleActPlan(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
		}
		return decodeMap(t, rr.Body.String())
	}

	// A give that cancels a queued change and has a rule that waits.
	full := get(services.Change{
		Writes: []services.Write{{EnqueueParams: db.EnqueueParams{ProjectID: "p", RoleKeys: []string{"safety"}},
			Delivery: services.DeliveryWaits, OutboxID: "ob1"}},
		Cancels: []models.QueuedChange{
			{ID: cancelID, OpType: "revoke", InitiatedBy: "admin-1"},
		},
		Rules: []string{"Also gives Safety, by the automatic rule from trained."},
		Said:  []string{"Also gives Safety, by the automatic rule from trained."},
		Waits: []string{"p/safety"}, Goes: []string{"p/laser"},
		Row: services.BulkOutcome{
			Effect: services.EffectApply, Detail: "gains trained", Fingerprint: "fp-1", CancelIDs: []string{cancelID},
		},
	})
	if full["effect"] != services.EffectApply || full["detail"] != "gains trained" || full["fingerprint"] != "fp-1" {
		t.Fatalf("unexpected sheet: %v", full)
	}
	if full["delivery"] != services.DeliveryWaits {
		t.Errorf("delivery = %v, want %s", full["delivery"], services.DeliveryWaits)
	}
	for k, n := range map[string]int{"cancel_ids": 1, "cancels": 1, "rules": 1, "waits": 1, "goes": 1, "said": 1} {
		if l, ok := full[k].([]any); !ok || len(l) != n {
			t.Errorf("%s = %v, want a list of %d", k, full[k], n)
		}
	}
	if c := full["cancels"].([]any)[0].(map[string]any); c["id"] != cancelID || c["initiated_by_name"] != "Ada" {
		t.Errorf("cancel not named: %v", c)
	}
	if _, has := full["consequence"]; has {
		t.Errorf("an applying act has no refusal step: %v", full["consequence"])
	}

	// A refusal with nothing else: the lists are still lists, and it says why.
	refused := get(services.Change{
		Refusal: "held",
		Row:     services.BulkOutcome{Effect: services.EffectBlocked, Detail: "already holds it", Consequence: "Nothing to give."},
	})
	for _, k := range []string{"cancel_ids", "cancels", "rules", "waits", "goes", "said"} {
		if l, ok := refused[k].([]any); !ok || len(l) != 0 {
			t.Errorf("%s = %v, want []", k, refused[k])
		}
	}
	if refused["refusal"] != "held" || refused["consequence"] != "Nothing to give." {
		t.Errorf("refusal not said: %v", refused)
	}
	if _, has := refused["delivery"]; has {
		t.Errorf("a refusal sends nothing, so it has no delivery: %v", refused["delivery"])
	}
}

type decideCall struct {
	ID, Status, Reviewer, Note string
	Seen                       *services.Seen
}

// decideProbe is actProbe plus the request decision (svcDecideRequest).
type decideProbe struct {
	*actProbe
	Calls     []decideCall
	Decision  services.Decision
	DecideErr error
}

func stubDecide(t *testing.T) *decideProbe {
	t.Helper()
	p := &decideProbe{actProbe: stubAct(t)}
	orig := svcDecideRequest
	t.Cleanup(func() { svcDecideRequest = orig })
	svcDecideRequest = func(_ context.Context, id, status, reviewer, note string, seen *services.Seen) (services.Decision, error) {
		p.Order = append(p.Order, "decide")
		p.Calls = append(p.Calls, decideCall{id, status, reviewer, note, seen})
		return p.Decision, p.DecideErr
	}
	return p
}
