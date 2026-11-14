package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"syndra/internal/models"
)

func decodeErrorResponse(t *testing.T, rr *httptest.ResponseRecorder) ErrorResponse {
	t.Helper()
	var got ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	return got
}

func TestHandleCreateAccessRequestRejectsUnknownField(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/requests", strings.NewReader(`{"requester_id":"u1","project_id":"p1","role_key":"r1","justification":"need access","extra":"x"}`))
	rr := httptest.NewRecorder()

	handleCreateAccessRequest(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}
	got := decodeErrorResponse(t, rr)
	if got.Error != "VALIDATION_FAILED" {
		t.Fatalf("expected VALIDATION_FAILED, got %s", got.Error)
	}
	if got.Details["body"] == "" {
		t.Fatalf("expected body validation detail to be present")
	}
}

func TestHandleCreateAccessRequestRejectsNegativeDuration(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/requests", strings.NewReader(`{"requester_id":"u1","project_id":"p1","role_key":"r1","justification":"need access","duration_days":-2}`))
	rr := httptest.NewRecorder()

	handleCreateAccessRequest(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}
	got := decodeErrorResponse(t, rr)
	if got.Details["duration_days"] == "" {
		t.Fatalf("expected duration_days detail")
	}
}

func TestHandleResolveAccessRequestRequiresReviewerOnApprove(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/requests/abc/decision", strings.NewReader(`{"status":"approved"}`))
	req.SetPathValue("id", "abc")
	rr := httptest.NewRecorder()

	handleResolveAccessRequest(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}
	got := decodeErrorResponse(t, rr)
	if got.Details["reviewer_id"] == "" {
		t.Fatalf("expected reviewer_id detail")
	}
}

func TestHandleCreateMappingRuleRejectsSelfEdge(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/rules/mapping", strings.NewReader(`{"source_project":"p1","source_role":"r1","target_project":"p1","target_role":"r1"}`))
	rr := httptest.NewRecorder()

	handleCreateMappingRule(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}
	got := decodeErrorResponse(t, rr)
	if got.Error != "VALIDATION_FAILED" {
		t.Fatalf("expected VALIDATION_FAILED, got %s", got.Error)
	}
}

func TestHandleActionInjectRejectsInvalidMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/action/inject", nil)
	rr := httptest.NewRecorder()

	HandleActionInject(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status 405, got %d", rr.Code)
	}
	got := decodeErrorResponse(t, rr)
	if got.Error != "METHOD_NOT_ALLOWED" {
		t.Fatalf("expected METHOD_NOT_ALLOWED, got %s", got.Error)
	}
}

func TestHandleActionInjectRejectsMissingFields(t *testing.T) {
	// v2 payload without user.id must be rejected deterministically. Unknown
	// top-level fields (like the legacy "user_id") are accepted silently by
	// the lenient decoder used for the Zitadel-owned schema.
	req := httptest.NewRequest(http.MethodPost, "/api/action/inject", strings.NewReader(`{"user":{"id":""},"user_grants":[{"projectId":"p1","roles":["admin"]}]}`))
	rr := httptest.NewRecorder()

	HandleActionInject(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}
	got := decodeErrorResponse(t, rr)
	if got.Details["user.id"] == "" {
		t.Fatalf("expected user.id detail, got %v", got.Details)
	}
}

func TestHandleZitadelWebhookRejectsMissingRequiredFields(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/zitadel", strings.NewReader(`{"event_type":"grant_added","user_id":"u1","source_project":"p1"}`))
	rr := httptest.NewRecorder()

	HandleZitadelWebhook(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}
	got := decodeErrorResponse(t, rr)
	if got.Details["role_key"] == "" {
		t.Fatalf("expected role_key detail")
	}
}

// One decoding rule for every mutation body: a member name exactly as the
// request type spells it (RFC 8259 §4), never twice in one object (RFC 7493
// §2.3). encoding/json alone folded `Reason` and `ſtatus` into their fields
// and kept the last of two names. Each variant must get the very refusal the
// endpoint already gives an unknown field: same status, same code, and
// nothing done.
func TestMutationBodiesMatchNamesExactlyAndOnce(t *testing.T) {
	saved, audits := stubTerms(t)
	lookups := stubLookup(t)
	writes := stubRequestLookup(t, map[string]models.AccessRequest{"r1": pendingRequest("r1", "u1", "trained")})
	origDrift := dbGetDriftItem
	t.Cleanup(func() { dbGetDriftItem = origDrift })
	driftRead := 0
	dbGetDriftItem = func(context.Context, string) (models.DriftItem, error) {
		driftRead++
		return models.DriftItem{}, errors.New("no such drift item")
	}

	call := func(h http.HandlerFunc, r *http.Request) *httptest.ResponseRecorder {
		rr := httptest.NewRecorder()
		h(rr, r)
		return rr
	}
	for _, ep := range []struct {
		name    string
		do      func(body string) *httptest.ResponseRecorder
		unknown string // the endpoint's own unknown-field refusal, the shape to match
		bodies  map[string]string
	}{
		{"PUT /terms", func(b string) *httptest.ResponseRecorder {
			return call(handlePutTerms, httptest.NewRequest(http.MethodPut, "/api/v1/terms", strings.NewReader(b)))
		}, `{"terms":[{"name":"A","start":"2026-01-01","end":"2026-02-01"}],"x":1}`, map[string]string{
			"a case variant":        `{"Terms":[{"name":"A","start":"2026-01-01","end":"2026-02-01"}]}`,
			"a nested case variant": `{"terms":[{"Name":"A","start":"2026-01-01","end":"2026-02-01"}]}`,
			"a duplicate":           `{"terms":[],"terms":[{"name":"A","start":"2026-01-01","end":"2026-02-01"}]}`,
			"a duplicate inside":    `{"terms":[{"name":"","name":"A","start":"2026-01-01","end":"2026-02-01"}]}`,
		}},
		{"POST /sign-ins/lookup", lookUp, `{"user_id":"u1","days":30,"reason":"a complaint","everyone":true}`, map[string]string{
			"a fold variant (long s)": `{"user_id":"u1","days":30,"reaſon":"a complaint"}`,
			"a duplicate":             `{"user_id":"u1","days":30,"reason":"","reason":"a complaint"}`,
		}},
		{"POST /requests/bulk-decision", func(b string) *httptest.ResponseRecorder {
			return call(handleBulkDecideRequests, bulkDecisionReq(t, b, true))
		}, `{"ids":["r1"],"status":"approved","force":true}`, map[string]string{
			"a case variant": `{"IDS":["r1"],"status":"approved"}`,
			"a duplicate":    `{"ids":["r1"],"status":"rejected","status":"approved"}`,
		}},
		{"POST /drift/{id}/mark-external", func(b string) *httptest.ResponseRecorder {
			r := httptest.NewRequest(http.MethodPost, "/api/v1/drift/d1/mark-external", strings.NewReader(b))
			r.SetPathValue("id", "d1")
			return call(handleMarkDriftExternal, r)
		}, `{"nope":1}`, map[string]string{
			"a case variant": `{"Reason":"seen to"}`,
			"a duplicate":    `{"reason":"a","reason":"b"}`,
		}},
	} {
		want := ep.do(ep.unknown)
		var wantErr ErrorResponse
		if want.Code != http.StatusBadRequest || json.Unmarshal(want.Body.Bytes(), &wantErr) != nil || wantErr.Error == "" {
			t.Fatalf("%s: the unknown-field refusal is %d %s", ep.name, want.Code, want.Body)
		}
		for why, body := range ep.bodies {
			got := ep.do(body)
			var gotErr ErrorResponse
			_ = json.Unmarshal(got.Body.Bytes(), &gotErr)
			if got.Code != want.Code || gotErr.Error != wantErr.Error {
				t.Errorf("%s, %s: got %d %s, want the unknown-field refusal %d %s", ep.name, why, got.Code, gotErr.Error, want.Code, wantErr.Error)
			}
		}
	}
	if *saved != "" || len(*audits) != 0 || len(lookups.audits) != 0 || len(lookups.searched) != 0 || *writes != 0 || driftRead != 0 {
		t.Errorf("a refused body did something: saved %q, audits %v, lookups %d/%d, writes %d, drift reads %d",
			*saved, *audits, len(lookups.audits), len(lookups.searched), *writes, driftRead)
	}
}

// An empty body is io.EOF, which the routes with an optional body read as "no
// body"; a body cut short is never that.
func TestDecodeJSONStrictTellsAnEmptyBodyFromATruncatedOne(t *testing.T) {
	var v struct {
		Reason string `json:"reason"`
	}
	for _, body := range []string{"", " \n"} {
		if err := decodeJSONStrict(strings.NewReader(body), &v); err != io.EOF {
			t.Errorf("%q: got %v, want io.EOF", body, err)
		}
	}
	for _, body := range []string{`{"reason":`, `{"reason":"x"`, `{"reason":"x"}{}`} {
		if err := decodeJSONStrict(strings.NewReader(body), &v); err == nil || errors.Is(err, io.EOF) {
			t.Errorf("%q: got %v, want an error other than io.EOF", body, err)
		}
	}
}
