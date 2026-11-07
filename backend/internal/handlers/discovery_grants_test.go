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

// grantPipe records what the Zitadel › Users handlers asked of the person
// page's pipe: every enqueue, and every direct-grant removal.
type grantPipe struct {
	enqueued []db.EnqueueParams
	deleted  []string
}

// newGrantPipe stubs every seam the /zitadel/users/{id}/grants handlers touch.
// By default nobody holds anything, every role is offered, and nothing but a
// direct grant gives a role.
func newGrantPipe(t *testing.T) *grantPipe {
	t.Helper()
	origEnqueue, origIndex, origLive := dbEnqueueDirectGrantPropagation, dbGetGrantIndex, dbListUserGrantsLive
	origHolding, origCheck, origRebuild := dbRequestHolding, checkRoleOffered, rebuildUserCacheDetachedFn
	origDirect, origDelete, origExpects := svcUserDirectGrants, svcDeleteDirectGrant, svcUserExpectsRole
	t.Cleanup(func() {
		dbEnqueueDirectGrantPropagation, dbGetGrantIndex, dbListUserGrantsLive = origEnqueue, origIndex, origLive
		dbRequestHolding, checkRoleOffered, rebuildUserCacheDetachedFn = origHolding, origCheck, origRebuild
		svcUserDirectGrants, svcDeleteDirectGrant, svcUserExpectsRole = origDirect, origDelete, origExpects
	})

	p := &grantPipe{}
	dbEnqueueDirectGrantPropagation = func(_ context.Context, ep db.EnqueueParams) (db.EnqueueResult, error) {
		p.enqueued = append(p.enqueued, ep)
		return db.EnqueueResult{OutboxID: "ob-" + ep.OpType + "-" + ep.RoleKeys[0], Status: "pending"}, nil
	}
	dbRequestHolding = func(context.Context, string, string, string) (models.RequestHolding, error) {
		return models.RequestHolding{Kind: models.HoldingNone}, nil
	}
	checkRoleOffered = func(context.Context, string, string) error { return nil }
	rebuildUserCacheDetachedFn = func(context.Context, string) {}
	svcUserDirectGrants = func(context.Context, string) ([]models.DirectGrant, error) { return nil, nil }
	svcDeleteDirectGrant = func(_ context.Context, _, grantID, _, _ string) (services.DirectGrantRemoval, error) {
		p.deleted = append(p.deleted, grantID)
		return services.DirectGrantRemoval{OutboxIDs: []string{"ob-del-" + grantID}, Revoked: []string{}, Retained: []string{}}, nil
	}
	svcUserExpectsRole = func(context.Context, string, string, string) (bool, error) { return false, nil }
	dbGetGrantIndex = func(_ context.Context, grantID string) (db.ZitadelGrantIndex, error) {
		return db.ZitadelGrantIndex{}, db.ErrGrantIndexNotFound
	}
	return p
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

func TestHandleAssignZitadelGrant_OneDirectGrantPerRole(t *testing.T) {
	p := newGrantPipe(t)
	w, res := serveGrant(t, handleAssignZitadelGrant, http.MethodPost, "/api/v1/zitadel/users/u1/grants",
		`{"projectId":"p1","roleKeys":["r1","r2"," r1 "]}`)

	if w.Code != http.StatusAccepted {
		t.Fatalf("want 202, got %d: %s", w.Code, w.Body.String())
	}
	if len(p.enqueued) != 2 {
		t.Fatalf("want one grant per distinct role, got %+v", p.enqueued)
	}
	for _, ep := range p.enqueued {
		if ep.OpType != "add" || ep.UserID != "u1" || ep.ProjectID != "p1" || len(ep.RoleKeys) != 1 || ep.Source != "direct" {
			t.Fatalf("unexpected enqueue: %+v", ep)
		}
	}
	if !slices.Equal(res.Given, []string{"p1/r1", "p1/r2"}) || len(res.OutboxIDs) != 2 {
		t.Fatalf("unexpected body: %+v", res)
	}
}

func TestHandleAssignZitadelGrant_RefusesAnUnofferedRole(t *testing.T) {
	p := newGrantPipe(t)
	checkRoleOffered = func(_ context.Context, _, role string) error {
		if role == "gone" {
			return errRoleNotOffered
		}
		return nil
	}
	w, _ := serveGrant(t, handleAssignZitadelGrant, http.MethodPost, "/api/v1/zitadel/users/u1/grants",
		`{"projectId":"p1","roleKeys":["r1","gone"]}`)

	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "ROLE_NOT_OFFERED") {
		t.Fatalf("want 409 ROLE_NOT_OFFERED, got %d: %s", w.Code, w.Body.String())
	}
	if len(p.enqueued) != 0 {
		t.Fatalf("one refusal refuses the whole change; enqueued %+v", p.enqueued)
	}
}

func TestHandleAssignZitadelGrant_RefusesARoleAlreadyHeld(t *testing.T) {
	p := newGrantPipe(t)
	dbRequestHolding = func(_ context.Context, _, _, role string) (models.RequestHolding, error) {
		if role == "r2" {
			return models.RequestHolding{Kind: models.HoldingForGood}, nil
		}
		return models.RequestHolding{Kind: models.HoldingNone}, nil
	}
	w, _ := serveGrant(t, handleAssignZitadelGrant, http.MethodPost, "/api/v1/zitadel/users/u1/grants",
		`{"projectId":"p1","roleKeys":["r1","r2"]}`)

	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "ALREADY_HELD") {
		t.Fatalf("want 409 ALREADY_HELD, got %d: %s", w.Code, w.Body.String())
	}
	if len(p.enqueued) != 0 {
		t.Fatalf("nothing may be given; enqueued %+v", p.enqueued)
	}
}

func TestHandleRemoveZitadelGrant_KeepsARoleABundleStillGives(t *testing.T) {
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
	svcUserExpectsRole = func(_ context.Context, _, project, role string) (bool, error) {
		return project == "p7" && role == "lead", nil
	}

	w, res := serveGrant(t, handleRemoveZitadelGrant, http.MethodDelete, "/api/v1/zitadel/users/u1/grants/g1", "")

	if w.Code != http.StatusAccepted {
		t.Fatalf("want 202, got %d: %s", w.Code, w.Body.String())
	}
	if !slices.Equal(p.deleted, []string{"dg-member"}) {
		t.Fatalf("the direct grant must go through DeleteDirectGrant, got %v", p.deleted)
	}
	for _, ep := range p.enqueued {
		if slices.Contains(ep.RoleKeys, "lead") {
			t.Fatalf("lead is still given by a bundle and must not be revoked: %+v", ep)
		}
	}
	if len(p.enqueued) != 1 || p.enqueued[0].OpType != "revoke" || p.enqueued[0].RoleKeys[0] != "stray" {
		t.Fatalf("only the role nothing gives is revoked directly, got %+v", p.enqueued)
	}
	if !slices.Contains(res.Retained, "p7/lead") || !slices.Contains(res.Revoked, "p7/stray") {
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
	if !slices.Equal(p.deleted, []string{"dg-old"}) {
		t.Fatalf("the left-out role must be removed through DeleteDirectGrant, got %v", p.deleted)
	}
	if len(p.enqueued) != 1 || p.enqueued[0].OpType != "add" || p.enqueued[0].RoleKeys[0] != "new" {
		t.Fatalf("only the new role is given, got %+v", p.enqueued)
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
	checkRoleOffered = func(context.Context, string, string) error { return errRoleNotOffered }

	w, _ := serveGrant(t, handleUpdateZitadelGrant, http.MethodPut, "/api/v1/zitadel/users/u1/grants/g1", `{"roleKeys":["new"]}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d: %s", w.Code, w.Body.String())
	}
	if len(p.enqueued) != 0 || len(p.deleted) != 0 {
		t.Fatalf("a refused add must not leave the removal half done: %+v %v", p.enqueued, p.deleted)
	}
}

// W8: each role reports its own outcome, not one "done" for the change.
func TestZitadelGrantChange_EachRoleReportsItsOwnStatus(t *testing.T) {
	newGrantPipe(t)
	origDeliver, origStatus := svcDeliver, dbGetPropagationStatus
	t.Cleanup(func() { svcDeliver, dbGetPropagationStatus = origDeliver, origStatus })
	svcDeliver = func(_ context.Context, ids []string, _ bool) services.Delivery {
		d := services.Delivery{Reason: map[string]string{}}
		for _, id := range ids {
			if id == "ob-add-waits" {
				d.Waiting = append(d.Waiting, id)
				d.Reason[id] = db.WaitReasonRoleWaits
				continue
			}
			d.Sent = append(d.Sent, id)
		}
		return d
	}
	dbGetPropagationStatus = func(_ context.Context, id string) (string, error) {
		if id == "ob-add-goes" {
			return "applied", nil
		}
		return "pending", nil // requeued: Zitadel didn't take it
	}

	_, res := serveGrant(t, handleAssignZitadelGrant, http.MethodPost, "/api/v1/zitadel/users/u1/grants?apply=true",
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
	svcUserExpectsRole = func(context.Context, string, string, string) (bool, error) { return true, nil }

	_, res := serveGrant(t, handleRemoveZitadelGrant, http.MethodDelete, "/api/v1/zitadel/users/u1/grants/g1", "")
	if len(p.enqueued) != 0 || res.Status != "unchanged" || len(res.Roles) != 1 || res.Roles[0].Status != "kept" {
		t.Fatalf("a kept role sends nothing and says so: %+v", res)
	}
}
