package zitadel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func actionsClient(t *testing.T, handle func(req *http.Request, body map[string]any) *http.Response) *managementClient {
	t.Helper()
	resetDeps(t)
	httpDo = func(_ *http.Client, req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "/oauth/") {
			return jsonResp(200, map[string]any{"access_token": "t", "token_type": "Bearer", "expires_in": 3600}), nil
		}
		var body map[string]any
		if req.Body != nil {
			raw, _ := io.ReadAll(req.Body)
			_ = json.Unmarshal(raw, &body)
		}
		return handle(req, body), nil
	}
	tm := newTokenManager("test.zitadel.dev", &ServiceAccountKey{Type: "serviceaccount", KeyID: "k", UserID: "u"}, testRSAKey(t))
	return &managementClient{domain: "test.zitadel.dev", tokens: tm, http: &http.Client{}}
}

// The search pages by offset until it has totalResult, reading "result".
func TestListActionExecutions_Pages(t *testing.T) {
	var offsets []string
	c := actionsClient(t, func(req *http.Request, body map[string]any) *http.Response {
		if req.URL.Path != "/v2/actions/executions/search" || req.Method != http.MethodPost {
			t.Errorf("unexpected %s %s", req.Method, req.URL.Path)
		}
		p := body["pagination"].(map[string]any)
		off := p["offset"].(string)
		offsets = append(offsets, off)
		n := actionsPageSize
		if off == fmt.Sprint(actionsPageSize) {
			n = 3
		}
		res := make([]map[string]any, n)
		for i := range res {
			res[i] = map[string]any{"condition": map[string]any{"event": map[string]any{"event": "e"}}, "targets": []string{"t1"}}
		}
		return jsonResp(200, map[string]any{"pagination": map[string]any{"totalResult": fmt.Sprint(actionsPageSize + 3)}, "result": res})
	})
	got, err := c.ListActionExecutions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != actionsPageSize+3 || len(offsets) != 2 {
		t.Fatalf("got %d over offsets %v", len(got), offsets)
	}
	if ids := got[0].TargetIDs(); len(ids) != 1 || ids[0] != "t1" || got[0].EventName() != "e" {
		t.Errorf("execution = %+v", got[0])
	}
}

func TestListActionTargets_Forbidden(t *testing.T) {
	c := actionsClient(t, func(*http.Request, map[string]any) *http.Response {
		return jsonResp(403, map[string]any{"code": 7, "message": "No matching permissions found"})
	})
	_, err := c.ListActionTargets(context.Background())
	var se *StatusError
	if !errors.As(err, &se) || se.Code != 403 {
		t.Fatalf("err = %v, want a 403 StatusError", err)
	}
}

func TestActionExecution_TargetShapes(t *testing.T) {
	var e ActionExecution
	_ = json.Unmarshal([]byte(`{"condition":{"function":{"name":"function/preaccesstoken"}},"targets":["a",{"target":"b"}]}`), &e)
	if e.FunctionName() != "preaccesstoken" {
		t.Errorf("function = %q", e.FunctionName())
	}
	if ids := e.TargetIDs(); len(ids) != 2 || ids[0] != "a" || ids[1] != "b" {
		t.Errorf("ids = %v", ids)
	}
}

func TestTokenFlowActions(t *testing.T) {
	c := actionsClient(t, func(req *http.Request, _ map[string]any) *http.Response {
		if req.URL.Path != "/management/v1/flows/2" {
			t.Errorf("path %s", req.URL.Path)
		}
		return jsonResp(200, map[string]any{"flow": map[string]any{"triggerActions": []any{
			map[string]any{"actions": []any{map[string]any{"name": "roles", "script": `api.v1.claims.setClaim("roles", [])`, "state": "ACTION_STATE_ACTIVE"}}},
		}}})
	})
	got, err := c.TokenFlowActions(context.Background())
	if err != nil || len(got) != 1 || got[0].Name != "roles" {
		t.Fatalf("got %+v, %v", got, err)
	}
}
