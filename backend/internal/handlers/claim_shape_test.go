package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"syndra/internal/auth"
	"syndra/internal/claimdebug"
	"syndra/internal/claims"
	"syndra/internal/db"
	"syndra/internal/models"
	"syndra/internal/services"
)

type auditLine struct{ actor, target, action, resource string }

// stubClaimHandlers records audits and the projects each claim endpoint
// published to the data plane's snapshot.
func stubClaimHandlers(t *testing.T) (*[]auditLine, *[]string) {
	t.Helper()
	origAudit := dbInsertAuditLog
	origFind, origUser, origSave := svcFindApplication, svcDebugFindUser, svcSaveAppClaimShape
	origGet := svcGetAppClaimShape
	t.Cleanup(func() {
		dbInsertAuditLog = origAudit
		svcFindApplication, svcDebugFindUser, svcSaveAppClaimShape, svcGetAppClaimShape = origFind, origUser, origSave, origGet
	})
	var audits []auditLine
	dropped := publishRecorder(t)
	dbInsertAuditLog = func(_ context.Context, actor, target, action, resource string) error {
		audits = append(audits, auditLine{actor, target, action, resource})
		return nil
	}
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
	return &audits, dropped
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
		len(*dropped) != 1 || (*dropped)[0] != "pNet" {
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
// The preview carries, per person, their last sign-in debug mode saw against
// what it previews: a match, a mismatch, and nothing for someone never seen.
func TestPreview_LiveCheck_HTTP(t *testing.T) {
	stubClaimHandlers(t)
	orig, origPreview := claimDebug, svcPreviewAppClaimShape
	t.Cleanup(func() { claimDebug, svcPreviewAppClaimShape = orig, origPreview })
	claimDebug = claimdebug.New(time.Now, nil, nil)
	claimDebug.Start("aNB", "cNB", "aanya", "kanishk")

	tok := claims.NewObject()
	tok.Set("groups", []any{"netbird-staff"})
	trail := services.Explanation{Token: tok, Checks: []services.Check{{}, {}, {}, {}, {Facts: map[string]any{"lands": claims.LandsBoth}}}}
	svcPreviewAppClaimShape = func(_ context.Context, _ string, req services.PreviewRequest) (services.Preview, error) {
		pv := services.Preview{}
		for _, id := range req.UserIDs {
			pv.People = append(pv.People, services.PersonPreview{UserID: id, Now: trail})
		}
		return pv, nil
	}
	preview := func() map[string]services.LiveCheck {
		rr := httptest.NewRecorder()
		req := asOperator(httptest.NewRequest(http.MethodPost, "/api/v1/applications/aNB/claim-shape/preview",
			strings.NewReader(`{"user_ids":["aanya","grace"]}`)), "kanishk")
		req.SetPathValue("id", "aNB")
		handlePreviewAppClaimShape(rr, req)
		var out struct {
			Live map[string]services.LiveCheck `json:"live"`
		}
		if rr.Code != http.StatusOK || json.Unmarshal(rr.Body.Bytes(), &out) != nil {
			t.Fatalf("preview: %d %s", rr.Code, rr.Body.String())
		}
		return out.Live
	}
	capture := func(answer string) {
		n := len(claimDebug.SignIns("aNB", 0))
		claimDebug.Record(claimdebug.Capture{At: time.Now(), Function: claims.FunctionPreAccessToken, ClientID: "cNB", UserID: "aanya",
			ProjectID: "pNet", Request: []byte(`{}`), Response: []byte(answer)})
		for deadline := time.Now().Add(2 * time.Second); len(claimDebug.SignIns("aNB", 0)) == n; {
			if time.Now().After(deadline) {
				t.Fatal("the capture never arrived")
			}
			time.Sleep(5 * time.Millisecond)
		}
	}

	if live := preview(); len(live) != 0 {
		t.Fatalf("nobody seen yet: %+v", live)
	}
	capture(`{"append_claims":[{"key":"groups","value":["netbird-staff"]}]}`)
	live := preview()
	if lc, ok := live["aanya"]; !ok || !lc.Match || lc.Function != claims.FunctionPreAccessToken {
		t.Fatalf("match: %+v", live)
	}
	if _, ok := live["grace"]; ok {
		t.Fatal("grace was never seen, so she has no live check")
	}
	// The newest capture is the one compared.
	capture(`{"append_claims":[]}`)
	if lc := preview()["aanya"]; lc.Match || len(lc.Served) != 0 || len(lc.Expected) != 1 {
		t.Fatalf("mismatch: %+v", lc)
	}
}

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
