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

// The search pages by offset until it has totalResult. Zitadel v4 lists
// executions under "executions" (seen on prod, 3 Nov 2026), not "result".
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
		return jsonResp(200, map[string]any{"pagination": map[string]any{"totalResult": fmt.Sprint(actionsPageSize + 3)}, "executions": res})
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

// Zitadel v4's target search answers {"pagination":…, "targets":[…]}. Read
// as "result", the list was empty and the reconciler reported the claims
// target missing; the older "result" key still reads; rows under a key we
// don't know are an error, never "none exist".
func TestListActionTargets_ReadsTheResourceKey(t *testing.T) {
	for _, tc := range []struct {
		name    string
		body    map[string]any
		want    int
		wantErr bool
	}{
		{"targets", map[string]any{"pagination": map[string]any{"totalResult": "2"}, "targets": []map[string]any{{"name": "syndra-claim-injector", "restCall": map[string]any{}}, {"name": "syndra-event-listener", "restAsync": map[string]any{}}}}, 2, false},
		{"legacy result", map[string]any{"pagination": map[string]any{"totalResult": "1"}, "result": []map[string]any{{"name": "syndra-claim-injector"}}}, 1, false},
		{"none", map[string]any{"pagination": map[string]any{"totalResult": "0"}}, 0, false},
		{"unknown key", map[string]any{"pagination": map[string]any{"totalResult": "2"}, "items": []map[string]any{{}, {}}}, 0, true},
	} {
		c := actionsClient(t, func(*http.Request, map[string]any) *http.Response { return jsonResp(200, tc.body) })
		got, err := c.ListActionTargets(context.Background())
		if (err != nil) != tc.wantErr || len(got) != tc.want {
			t.Errorf("%s: got %d, err %v", tc.name, len(got), err)
		}
		if tc.name == "targets" && (got[0].Name != "syndra-claim-injector" || got[0].Type() != "restCall" || got[1].Type() != "restAsync") {
			t.Errorf("targets decoded wrong: %+v", got)
		}
	}
}

// The rotation is rotate.sh's call, exactly: POST /v2/actions/targets/{id}
// with expirationSigningKey "0s", and the new key read from the answer. An
// answer without a key, or a refusal, is an error, never an empty key.
func TestRotateTargetSigningKey(t *testing.T) {
	c := actionsClient(t, func(req *http.Request, body map[string]any) *http.Response {
		if req.Method != http.MethodPost || req.URL.Path != "/v2/actions/targets/T1" || body["expirationSigningKey"] != "0s" || len(body) != 1 {
			t.Errorf("unexpected %s %s %v", req.Method, req.URL.Path, body)
		}
		return jsonResp(200, map[string]any{"changeDate": "2026-11-05T03:00:00Z", "signingKey": "NEWKEY"})
	})
	if k, err := c.RotateTargetSigningKey(context.Background(), "T1"); err != nil || k != "NEWKEY" {
		t.Fatalf("got %q, %v", k, err)
	}
	if _, err := c.RotateTargetSigningKey(context.Background(), "../x"); err == nil {
		t.Error("a target id with a path separator went out")
	}

	empty := actionsClient(t, func(*http.Request, map[string]any) *http.Response { return jsonResp(200, map[string]any{}) })
	if _, err := empty.RotateTargetSigningKey(context.Background(), "T1"); err == nil {
		t.Error("an answer with no signingKey was taken as a key")
	}
	denied := actionsClient(t, func(*http.Request, map[string]any) *http.Response {
		return jsonResp(403, map[string]any{"message": "no permission"})
	})
	var se *StatusError
	if _, err := denied.RotateTargetSigningKey(context.Background(), "T1"); !errors.As(err, &se) || se.Code != 403 {
		t.Errorf("a refusal is a 403 StatusError, got %v", err)
	}
}
