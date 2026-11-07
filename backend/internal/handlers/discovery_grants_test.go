package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"syndra/internal/db"
	"syndra/internal/models"
	"syndra/internal/services"
	"syndra/internal/zitadel"
)

// grantPipe stands in for the planner behind the Zitadel › Users handlers
// (task 1.9): every role is one planned act, as on a person's page. It
// records what was planned and applied, and with which plan.
type grantPipe struct {
	planned, applied []services.Action
	seen             map[string]*services.Seen // by role key
	gone, held       map[string]bool           // Zitadel no longer offers it; they already have it
	givenElsewhere   map[string]bool           // a bundle or a rule of Syndra's gives it
	stale            map[string]bool           // the role's plan moved since it was shown
}

// fake is the planner's answer for one act.
func (p *grantPipe) fake(a services.Action) services.Change {
	ch := services.Change{UserID: a.UserID, ProjectID: a.ProjectID, RoleKey: a.RoleKey,
		Row: services.BulkOutcome{Effect: services.EffectApply, Fingerprint: "fp-" + a.Op + "-" + a.RoleKey}}
	write := func(op string) {
		ch.Writes = append(ch.Writes, services.Write{Delivery: services.DeliveryStraightAway,
			EnqueueParams: db.EnqueueParams{UserID: a.UserID, ProjectID: a.ProjectID, RoleKeys: []string{a.RoleKey}, OpType: op, Source: "direct"}})
	}
	switch {
	case a.Op == services.OpGive && p.gone[a.RoleKey]:
		ch.Row.Effect, ch.Refusal, ch.Row.Detail = services.EffectNoChange, "not_offered:"+a.ProjectID+"/"+a.RoleKey, "This would give it, which no longer exists in Zitadel."
	case a.Op == services.OpGive && p.held[a.RoleKey]:
		ch.Row.Effect, ch.Refusal = services.EffectNoChange, "held"
	case a.Op == services.OpGive:
		write("add")
	case a.Op == services.OpRevoke && p.givenElsewhere[a.RoleKey]:
		ch.Row.Effect, ch.Refusal = services.EffectNoChange, "held"
	case p.givenElsewhere[a.RoleKey]: // a removal that keeps the role another way
	default:
		write("revoke")
	}
	return ch
}

// newGrantPipe stubs every seam the /zitadel/users/{id}/grants handlers
// touch. By default nobody holds anything, every role is offered, and
// nothing but a direct grant gives a role.
func newGrantPipe(t *testing.T) *grantPipe {
	t.Helper()
	origPlan, origApply, origIndex, origLive := svcPlanAct, svcApplyAct, dbGetGrantIndex, dbListUserGrantsLive
	origHolding, origRebuild, origDirect := dbRequestHolding, rebuildUserCacheDetachedFn, svcUserDirectGrants
	origStatus := dbGetPropagationStatus
	t.Cleanup(func() {
		svcPlanAct, svcApplyAct, dbGetGrantIndex, dbListUserGrantsLive = origPlan, origApply, origIndex, origLive
		dbRequestHolding, rebuildUserCacheDetachedFn, svcUserDirectGrants = origHolding, origRebuild, origDirect
		dbGetPropagationStatus = origStatus
	})
	dbGetPropagationStatus = func(context.Context, string) (string, error) { return "pending", nil }

	p := &grantPipe{seen: map[string]*services.Seen{}, gone: map[string]bool{}, held: map[string]bool{},
		givenElsewhere: map[string]bool{}, stale: map[string]bool{}}
	svcPlanAct = func(_ context.Context, a services.Action) (services.Change, error) {
		p.planned = append(p.planned, a)
		return p.fake(a), nil
	}
	svcApplyAct = func(_ context.Context, a services.Action, _ services.ActWrite, seen *services.Seen) (services.Change, error) {
		if p.stale[a.RoleKey] {
			return services.Change{}, &services.StalePlan{IDs: []string{a.UserID}}
		}
		ch := p.fake(a)
		if ch.Row.Effect != services.EffectApply {
			return services.Change{}, &services.Refused{Change: ch}
		}
		p.applied = append(p.applied, a)
		p.seen[a.RoleKey] = seen
		for i := range ch.Writes {
			ch.Writes[i].OutboxID = "ob-" + ch.Writes[i].OpType + "-" + a.RoleKey
		}
		return ch, nil
	}
	dbRequestHolding = func(_ context.Context, _, _, role string) (models.RequestHolding, error) {
		if p.held[role] {
			return models.RequestHolding{Kind: models.HoldingForGood}, nil
		}
		return models.RequestHolding{Kind: models.HoldingNone}, nil
	}
	rebuildUserCacheDetachedFn = func(context.Context, string) {}
	svcUserDirectGrants = func(context.Context, string) ([]models.DirectGrant, error) { return nil, nil }
	dbGetGrantIndex = func(_ context.Context, grantID string) (db.ZitadelGrantIndex, error) {
		return db.ZitadelGrantIndex{}, db.ErrGrantIndexNotFound
	}
	return p
}

func (p *grantPipe) ops() []string {
	var out []string
	for _, a := range p.applied {
		out = append(out, a.Op+":"+a.RoleKey+a.GrantID)
	}
	return out
}

func serveGrant(t *testing.T, h http.HandlerFunc, method, path, body string) (*httptest.ResponseRecorder, zitadelGrantChange) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.SetPathValue("id", "u1")
	req.SetPathValue("grantId", "g1")
	w := httptest.NewRecorder()
	h(w, req)
	var res zitadelGrantChange
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	return w, res
}

func TestHandleAssignZitadelGrant_OneGiveActPerRole(t *testing.T) {
	p := newGrantPipe(t)
	w, res := serveGrant(t, handleAssignZitadelGrant, http.MethodPost, "/api/v1/zitadel/users/u1/grants",
		`{"projectId":"p1","roleKeys":["r1","r2"," r1 "]}`)

	if w.Code != http.StatusAccepted {
		t.Fatalf("want 202, got %d: %s", w.Code, w.Body.String())
	}
	if got := p.ops(); !slices.Equal(got, []string{"give:r1", "give:r2"}) {
		t.Fatalf("want one planned give per distinct role, got %v", got)
	}
	if !slices.Equal(res.Given, []string{"p1/r1", "p1/r2"}) || len(res.OutboxIDs) != 2 {
		t.Fatalf("unexpected body: %+v", res)
	}
}

func TestHandleAssignZitadelGrant_RefusesAnUnofferedRole(t *testing.T) {
	p := newGrantPipe(t)
	p.gone["gone"] = true
	w, _ := serveGrant(t, handleAssignZitadelGrant, http.MethodPost, "/api/v1/zitadel/users/u1/grants",
		`{"projectId":"p1","roleKeys":["r1","gone"]}`)

	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "ROLE_NOT_OFFERED") {
		t.Fatalf("want 409 ROLE_NOT_OFFERED, got %d: %s", w.Code, w.Body.String())
	}
	if len(p.applied) != 0 {
		t.Fatalf("one refusal refuses the whole change; applied %v", p.ops())
	}
}

func TestHandleAssignZitadelGrant_RefusesARoleAlreadyHeld(t *testing.T) {
	p := newGrantPipe(t)
	p.held["r2"] = true
	w, _ := serveGrant(t, handleAssignZitadelGrant, http.MethodPost, "/api/v1/zitadel/users/u1/grants",
		`{"projectId":"p1","roleKeys":["r1","r2"]}`)

	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "ALREADY_HELD") || !strings.Contains(w.Body.String(), "r2:") {
		t.Fatalf("want 409 ALREADY_HELD naming r2, got %d: %s", w.Code, w.Body.String())
	}
	if len(p.applied) != 0 {
		t.Fatalf("nothing may be given; applied %v", p.ops())
	}
}

// Each role's write carries the plan that role was shown, so the planner
// admits exactly the cancellations the step named; a role with no plan
// admits none (the cancel gate's 409 answers it).
func TestHandleAssignZitadelGrant_EachRoleCarriesItsOwnPlan(t *testing.T) {
	p := newGrantPipe(t)
	w, _ := serveGrant(t, handleAssignZitadelGrant, http.MethodPost, "/api/v1/zitadel/users/u1/grants",
		`{"projectId":"p1","roleKeys":["r1","r2"],"plans":{"r1":{"fingerprint":"fp-1","cancel_ids":["11111111-1111-1111-1111-111111111111"]}}}`)
	if w.Code != http.StatusAccepted {
		t.Fatalf("want 202, got %d: %s", w.Code, w.Body.String())
	}
	if s := p.seen["r1"]; s == nil || s.Fingerprint != "fp-1" || len(s.CancelIDs) != 1 {
		t.Fatalf("r1 carries its plan: %+v", s)
	}
	if p.seen["r2"] != nil {
		t.Fatalf("r2 was shown no plan, so it admits no cancellation: %+v", p.seen["r2"])
	}
}

// A plan that moved before anything was written writes nothing (409
// PLAN_STALE, the sheet reads it again); one that moved after a role was
// written says what was recorded.
func TestHandleAssignZitadelGrant_AStalePlan(t *testing.T) {
	p := newGrantPipe(t)
	p.stale["r1"] = true
	w, _ := serveGrant(t, handleAssignZitadelGrant, http.MethodPost, "/api/v1/zitadel/users/u1/grants",
		`{"projectId":"p1","roleKeys":["r1","r2"]}`)
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "PLAN_STALE") || len(p.applied) != 0 {
		t.Fatalf("want 409 PLAN_STALE with nothing written, got %d %s %v", w.Code, w.Body.String(), p.ops())
	}

	p = newGrantPipe(t)
	p.stale["r2"] = true
	w, _ = serveGrant(t, handleAssignZitadelGrant, http.MethodPost, "/api/v1/zitadel/users/u1/grants",
		`{"projectId":"p1","roleKeys":["r1","r2"]}`)
	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), partlyRecordedCode) ||
		!strings.Contains(w.Body.String(), "p1/r1") {
		t.Fatalf("want PARTLY_RECORDED naming p1/r1, got %d %s", w.Code, w.Body.String())
	}
}

func TestHandleRemoveZitadelGrant_KeepsARoleSyndraStillGives(t *testing.T) {
	p := newGrantPipe(t)
	// Index miss → live lookup. The grant carries three roles: one given
	// directly, one only through a bundle, one with nothing behind it.
	dbListUserGrantsLive = func(_ context.Context, userID, grantID string) (zitadel.UserGrant, error) {
		return zitadel.UserGrant{ID: grantID, UserID: userID, ProjectID: "p7", RoleKeys: []string{"member", "lead", "stray"}}, nil
	}
	svcUserDirectGrants = func(context.Context, string) ([]models.DirectGrant, error) {
		return []models.DirectGrant{
			{ID: "dg-member", ProjectID: "p7", RoleKey: "member"},
			{ID: "dg-elsewhere", ProjectID: "p9", RoleKey: "lead"}, // another project: not this grant
		}, nil
	}
	p.givenElsewhere["lead"] = true

	w, res := serveGrant(t, handleRemoveZitadelGrant, http.MethodDelete, "/api/v1/zitadel/users/u1/grants/g1", "")

	if w.Code != http.StatusAccepted {
		t.Fatalf("want 202, got %d: %s", w.Code, w.Body.String())
	}
	// The direct grant is the removal of that grant; the others are revokes,
	// and the planner keeps the one Syndra still gives (under the lock).
	if got := p.ops(); !slices.Equal(got, []string{"remove:memberdg-member", "revoke:stray"}) {
		t.Fatalf("applied %v", got)
	}
	if !slices.Contains(res.Retained, "p7/lead") || !slices.Contains(res.Revoked, "p7/stray") || !slices.Contains(res.Revoked, "p7/member") {
		t.Fatalf("the body must say what stays and what goes: %+v", res)
	}
}

func TestHandleUpdateZitadelGrant_RemovesLeftOutAndGivesNew(t *testing.T) {
	p := newGrantPipe(t)
	dbGetGrantIndex = func(_ context.Context, grantID string) (db.ZitadelGrantIndex, error) {
		return db.ZitadelGrantIndex{GrantID: grantID, UserID: "u1", ProjectID: "p1", RoleKeys: []string{"keep", "old"}}, nil
	}
	svcUserDirectGrants = func(context.Context, string) ([]models.DirectGrant, error) {
		return []models.DirectGrant{{ID: "dg-old", ProjectID: "p1", RoleKey: "old"}}, nil
	}

	w, res := serveGrant(t, handleUpdateZitadelGrant, http.MethodPut, "/api/v1/zitadel/users/u1/grants/g1",
		`{"roleKeys":["keep","new"]}`)

	if w.Code != http.StatusAccepted {
		t.Fatalf("want 202, got %d: %s", w.Code, w.Body.String())
	}
	if got := p.ops(); !slices.Equal(got, []string{"remove:olddg-old", "give:new"}) {
		t.Fatalf("the left-out role is removed, then the new one given; applied %v", got)
	}
	if !slices.Equal(res.Given, []string{"p1/new"}) {
		t.Fatalf("unexpected body: %+v", res)
	}
}

func TestHandleUpdateZitadelGrant_ARefusedAddChangesNothing(t *testing.T) {
	p := newGrantPipe(t)
	dbGetGrantIndex = func(_ context.Context, grantID string) (db.ZitadelGrantIndex, error) {
		return db.ZitadelGrantIndex{GrantID: grantID, ProjectID: "p1", RoleKeys: []string{"old"}}, nil
	}
	p.gone["new"] = true

	w, _ := serveGrant(t, handleUpdateZitadelGrant, http.MethodPut, "/api/v1/zitadel/users/u1/grants/g1", `{"roleKeys":["new"]}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d: %s", w.Code, w.Body.String())
	}
	if len(p.applied) != 0 {
		t.Fatalf("a refused add must not leave the removal half done: %v", p.ops())
	}
}

// The removal preview is each role's plan (nothing written): what goes, what
// stays, and the plan each role's write carries back.
func TestHandleRemoveZitadelGrantPreview_IsEachRolesPlan(t *testing.T) {
	p := newGrantPipe(t)
	dbGetGrantIndex = func(_ context.Context, grantID string) (db.ZitadelGrantIndex, error) {
		return db.ZitadelGrantIndex{GrantID: grantID, ProjectID: "p1", RoleKeys: []string{"lead", "stray", "other"}}, nil
	}
	p.givenElsewhere["lead"] = true

	w, res := serveGrant(t, handleRemoveZitadelGrantPreview, http.MethodGet, "/api/v1/zitadel/users/u1/grants/g1/removal?roles=lead,stray", "")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	if len(p.applied) != 0 || len(p.planned) != 2 {
		t.Fatalf("a preview plans the asked roles and writes nothing: planned %v applied %v", p.planned, p.ops())
	}
	if !slices.Equal(res.Revoked, []string{"p1/stray"}) || !slices.Equal(res.Retained, []string{"p1/lead"}) {
		t.Fatalf("unexpected preview: %+v", res)
	}
	if res.Plans["stray"].Fingerprint != "fp-revoke-stray" || res.Plans["lead"].Effect != services.EffectNoChange {
		t.Fatalf("each role's plan comes back: %+v", res.Plans)
	}
}

// W8: each role reports its own outcome, not one "done" for the change.
func TestZitadelGrantChange_EachRoleReportsItsOwnStatus(t *testing.T) {
	newGrantPipe(t)
	origDeliver, origStatus := svcDeliverChange, dbGetPropagationStatus
	t.Cleanup(func() { svcDeliverChange, dbGetPropagationStatus = origDeliver, origStatus })
	svcDeliverChange = func(_ context.Context, ch services.Change) services.Delivery {
		d := services.Delivery{Reason: map[string]string{}}
		for _, w := range ch.Writes {
			if w.OutboxID == "ob-add-waits" {
				d.Waiting = append(d.Waiting, w.OutboxID)
				d.Reason[w.OutboxID] = db.WaitReasonRoleWaits
				continue
			}
			d.Sent = append(d.Sent, w.OutboxID)
		}
		return d
	}
	dbGetPropagationStatus = func(_ context.Context, id string) (string, error) {
		if id == "ob-add-goes" {
			return "applied", nil
		}
		return "pending", nil // requeued: Zitadel didn't take it
	}

	_, res := serveGrant(t, handleAssignZitadelGrant, http.MethodPost, "/api/v1/zitadel/users/u1/grants",
		`{"projectId":"p1","roleKeys":["goes","waits","later"]}`)

	got := map[string]string{}
	for _, rc := range res.Roles {
		got[rc.Role] = rc.Change + ":" + rc.Status
	}
	want := map[string]string{"p1/goes": "given:applied", "p1/waits": "given:waiting", "p1/later": "given:pending"}
	for role, w := range want {
		if got[role] != w {
			t.Fatalf("%s: want %s, got %s (all: %v)", role, w, got[role], got)
		}
	}
	if res.Status != "pending" || res.WaitReason != db.WaitReasonRoleWaits {
		t.Fatalf("the change is only as far as its least delivered role: %+v", res)
	}
}

func TestZitadelGrantChange_AllKeptSendsNothing(t *testing.T) {
	p := newGrantPipe(t)
	dbListUserGrantsLive = func(_ context.Context, userID, grantID string) (zitadel.UserGrant, error) {
		return zitadel.UserGrant{ID: grantID, UserID: userID, ProjectID: "p7", RoleKeys: []string{"lead"}}, nil
	}
	p.givenElsewhere["lead"] = true

	_, res := serveGrant(t, handleRemoveZitadelGrant, http.MethodDelete, "/api/v1/zitadel/users/u1/grants/g1", "")
	if len(p.applied) != 0 || res.Status != "unchanged" || len(res.Roles) != 1 || res.Roles[0].Status != "kept" {
		t.Fatalf("a kept role sends nothing and says so: %+v", res)
	}
}
