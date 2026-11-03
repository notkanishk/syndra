package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"syndra/internal/auth"
	"syndra/internal/claims"
	"syndra/internal/db"
	"syndra/internal/models"
	"syndra/internal/services"
)

type auditLine struct{ actor, target, action, resource string }

// stubClaimHandlers records audits and cache drops for the claim endpoints.
func stubClaimHandlers(t *testing.T) (*[]auditLine, *[]string) {
	t.Helper()
	origAudit, origDel := dbInsertAuditLog, redisDelKeys
	origFind, origUser, origSave := svcFindApplication, svcDebugFindUser, svcSaveAppClaimShape
	origGet := svcGetAppClaimShape
	t.Cleanup(func() {
		dbInsertAuditLog, redisDelKeys = origAudit, origDel
		svcFindApplication, svcDebugFindUser, svcSaveAppClaimShape, svcGetAppClaimShape = origFind, origUser, origSave, origGet
	})
	var audits []auditLine
	var dropped []string
	dbInsertAuditLog = func(_ context.Context, actor, target, action, resource string) error {
		audits = append(audits, auditLine{actor, target, action, resource})
		return nil
	}
	redisDelKeys = func(_ context.Context, keys ...string) error { dropped = append(dropped, keys...); return nil }
	svcFindApplication = func(_ context.Context, id string) (models.ApplicationCatalog, bool, error) {
		switch id {
		case "aNB":
			return models.ApplicationCatalog{ID: id, ProjectID: "pNet", ClientID: "cNB"}, true, nil
		case "aSAML":
			return models.ApplicationCatalog{ID: id, ProjectID: "pNet"}, true, nil
		}
		return models.ApplicationCatalog{}, false, nil
	}
	svcDebugFindUser = func(_ context.Context, id string) (models.UserProfile, bool, error) {
		return models.UserProfile{ID: id}, id == "aanya", nil
	}
	return &audits, &dropped
}

func asOperator(req *http.Request, id string) *http.Request {
	return req.WithContext(withPrincipal(req.Context(), &auth.Principal{Subject: id}))
}

func TestSaveAppClaimShape_HTTP(t *testing.T) {
	audits, dropped := stubClaimHandlers(t)
	svcSaveAppClaimShape = func(_ context.Context, appID, actor string, req services.SaveClaimShapeRequest, _ int) (services.SaveResult, error) {
		switch {
		case !req.AcknowledgeBreaking:
			return services.SaveResult{}, &services.BreakingUnacknowledged{Breaking: []claims.Change{{Kind: claims.ChangeRename, Role: "staff"}}}
		case len(req.Profile.Claims) == 0:
			return services.SaveResult{}, &services.ClaimShapeInvalid{RowError: claims.RowError{Row: 2, Msg: "bad row"}}
		}
		return services.SaveResult{Version: db.ClaimShapeVersion{ApplicationID: appID, ProjectID: "pNet", Version: 4, SavedBy: actor}}, nil
	}
	save := func(body string) *httptest.ResponseRecorder {
		rr := httptest.NewRecorder()
		req := asOperator(httptest.NewRequest(http.MethodPut, "/api/v1/applications/aNB/claim-shape", strings.NewReader(body)), "op_1")
		req.SetPathValue("id", "aNB")
		handleSaveAppClaimShape(rr, req)
		return rr
	}
	if rr := save(`{"profile":{"claims":[{"path":["groups"],"kind":"roles"}],"lands":"both"}}`); rr.Code != http.StatusConflict ||
		!strings.Contains(rr.Body.String(), `"BREAKING_UNACKNOWLEDGED"`) || !strings.Contains(rr.Body.String(), `"role":"staff"`) {
		t.Fatalf("unacknowledged: %d %s", rr.Code, rr.Body.String())
	}
	if rr := save(`{"profile":{"claims":[],"lands":"both"},"acknowledge_breaking":true}`); rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), `"row":"2"`) {
		t.Fatalf("invalid row: %d %s", rr.Code, rr.Body.String())
	}
	if rr := save(`{"profile":{},"surprise":1}`); rr.Code != http.StatusBadRequest {
		t.Fatalf("strict decoding: %d", rr.Code)
	}
	if len(*audits) != 0 || len(*dropped) != 0 {
		t.Fatal("a refused save writes nothing")
	}
	rr := save(`{"profile":{"claims":[{"path":["groups"],"kind":"roles"}],"lands":"both"},"acknowledge_breaking":true}`)
	if rr.Code != http.StatusOK || len(*audits) != 1 || (*audits)[0] != (auditLine{"op_1", "aNB", "claim_shape.saved", "aNB"}) ||
		len(*dropped) != 1 || (*dropped)[0] != claimShapeCacheKey("pNet") {
		t.Fatalf("saved: %d audits=%v dropped=%v", rr.Code, *audits, *dropped)
	}
}

func TestGetAppClaimShape_SeedDropsTheCache(t *testing.T) {
	_, dropped := stubClaimHandlers(t)
	svcGetAppClaimShape = func(context.Context, string) (services.AppClaimShape, bool, error) {
		return services.AppClaimShape{Application: models.ApplicationCatalog{ProjectID: "pNet"}}, true, nil
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/applications/aNB/claim-shape", nil)
	req.SetPathValue("id", "aNB")
	handleGetAppClaimShape(rr, req)
	if rr.Code != http.StatusOK || len(*dropped) != 1 {
		t.Fatalf("a seed on read reaches the data plane at once: %d %v", rr.Code, *dropped)
	}
	svcGetAppClaimShape = func(context.Context, string) (services.AppClaimShape, bool, error) {
		return services.AppClaimShape{}, false, fmt.Errorf("app: %w", services.ErrClaimShapeNotFound)
	}
	rr = httptest.NewRecorder()
	handleGetAppClaimShape(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("unknown app: %d", rr.Code)
	}
}

// Debug mode: the caller by default, one account, a switch keeps the end, each
// step audited; an app with no client can't be debugged.
func TestAppDebug_HTTP(t *testing.T) {
	audits, _ := stubClaimHandlers(t)
	t.Cleanup(func() { claimDebug.Stop("aNB", "test") })
	put := func(app, body string) *httptest.ResponseRecorder {
		rr := httptest.NewRecorder()
		req := asOperator(httptest.NewRequest(http.MethodPut, "/api/v1/applications/"+app+"/debug", strings.NewReader(body)), "kanishk")
		req.SetPathValue("id", app)
		handlePutAppDebug(rr, req)
		return rr
	}
	rr := put("aNB", "")
	var on struct {
		EndsAt  string `json:"ends_at"`
		Session struct {
			UserID string `json:"user_id"`
		} `json:"session"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &on)
	if rr.Code != http.StatusOK || on.Session.UserID != "kanishk" || on.EndsAt == "" {
		t.Fatalf("on for the caller: %d %s", rr.Code, rr.Body.String())
	}
	rr = put("aNB", `{"user_id":"aanya"}`)
	var sw struct {
		EndsAt   string `json:"ends_at"`
		Switched bool   `json:"switched"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &sw)
	if !sw.Switched || sw.EndsAt != on.EndsAt {
		t.Fatalf("a switch keeps the end: %s", rr.Body.String())
	}
	if rr := put("aNB", `{"user_id":"nobody"}`); rr.Code != http.StatusBadRequest {
		t.Fatalf("an unknown person: %d", rr.Code)
	}
	if rr := put("aSAML", ""); rr.Code != http.StatusBadRequest {
		t.Fatalf("no client id: %d", rr.Code)
	}
	if rr := put("missing", ""); rr.Code != http.StatusNotFound {
		t.Fatalf("unknown app: %d", rr.Code)
	}

	del := httptest.NewRecorder()
	req := asOperator(httptest.NewRequest(http.MethodDelete, "/api/v1/applications/aNB/debug", nil), "kanishk")
	req.SetPathValue("id", "aNB")
	handleDeleteAppDebug(del, req)
	if !strings.Contains(del.Body.String(), `"on":false`) || !strings.Contains(del.Body.String(), `"ended_by":"kanishk"`) {
		t.Fatalf("off: %s", del.Body.String())
	}
	want := []auditLine{
		{"kanishk", "kanishk", "claim_debug.started", "aNB"},
		{"kanishk", "aanya", "claim_debug.switched", "aNB"},
		{"kanishk", "aanya", "claim_debug.ended", "aNB"},
	}
	if len(*audits) != len(want) {
		t.Fatalf("audits: %v", *audits)
	}
	for i := range want {
		if (*audits)[i] != want[i] {
			t.Fatalf("audit %d: got %v want %v", i, (*audits)[i], want[i])
		}
	}
}
