package zitadel

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestAddProjectRole_HappyPath(t *testing.T) {
	resetDeps(t)

	var capturedPath string
	var capturedBody map[string]any

	httpDo = func(_ *http.Client, req *http.Request) (*http.Response, error) {
		// Token endpoint — return a fake token.
		if strings.Contains(req.URL.Path, "/oauth/") {
			return jsonResp(200, map[string]any{
				"access_token": "test-token",
				"token_type":   "Bearer",
				"expires_in":   3600,
			}), nil
		}

		capturedPath = req.URL.Path
		body, _ := io.ReadAll(req.Body)
		json.Unmarshal(body, &capturedBody)

		return jsonResp(200, map[string]string{"message": "ok"}), nil
	}

	key := testRSAKey(t)
	tm := newTokenManager("test.zitadel.dev", &ServiceAccountKey{
		Type:   "serviceaccount",
		KeyID:  "key-1",
		UserID: "user-1",
	}, key)

	client := &managementClient{
		domain: "test.zitadel.dev",
		tokens: tm,
		http:   &http.Client{},
	}

	err := client.AddProjectRole(context.Background(), "proj-123", "editor", "Editor", "staff")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedPath := "/management/v1/projects/proj-123/roles"
	if capturedPath != expectedPath {
		t.Errorf("expected path %s, got %s", expectedPath, capturedPath)
	}
	if capturedBody["roleKey"] != "editor" {
		t.Errorf("expected roleKey=editor, got %v", capturedBody["roleKey"])
	}
	if capturedBody["displayName"] != "Editor" {
		t.Errorf("expected displayName=Editor, got %v", capturedBody["displayName"])
	}
}

func TestListProjectRoles_ParsesResponse(t *testing.T) {
	resetDeps(t)

	httpDo = func(_ *http.Client, req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "/oauth/") {
			return jsonResp(200, map[string]any{
				"access_token": "test-token",
				"token_type":   "Bearer",
				"expires_in":   3600,
			}), nil
		}

		return jsonResp(200, map[string]any{
			"result": []map[string]string{
				{"key": "admin", "displayName": "Administrator", "group": "ops"},
				{"key": "member", "displayName": "Member", "group": "users"},
			},
		}), nil
	}

	key := testRSAKey(t)
	tm := newTokenManager("test.zitadel.dev", &ServiceAccountKey{
		Type: "serviceaccount", KeyID: "key-1", UserID: "user-1",
	}, key)

	client := &managementClient{
		domain: "test.zitadel.dev", tokens: tm, http: &http.Client{},
	}

	result, err := client.ListProjectRoles(context.Background(), "proj-456", SearchParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Items) != 2 {
		t.Fatalf("expected 2 roles, got %d", len(result.Items))
	}
	if result.Items[0].Key != "admin" || result.Items[0].DisplayName != "Administrator" {
		t.Errorf("unexpected first role: %+v", result.Items[0])
	}
	if result.Items[1].Key != "member" {
		t.Errorf("unexpected second role: %+v", result.Items[1])
	}
}

func TestUpdateProjectRole_CorrectEndpoint(t *testing.T) {
	resetDeps(t)

	var capturedPath, capturedMethod string
	var capturedBody map[string]any

	httpDo = func(_ *http.Client, req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "/oauth/") {
			return jsonResp(200, map[string]any{
				"access_token": "test-token",
				"token_type":   "Bearer",
				"expires_in":   3600,
			}), nil
		}

		capturedPath = req.URL.Path
		capturedMethod = req.Method
		body, _ := io.ReadAll(req.Body)
		json.Unmarshal(body, &capturedBody)

		return jsonResp(200, map[string]string{"message": "ok"}), nil
	}

	key := testRSAKey(t)
	tm := newTokenManager("test.zitadel.dev", &ServiceAccountKey{
		Type: "serviceaccount", KeyID: "key-1", UserID: "user-1",
	}, key)

	client := &managementClient{
		domain: "test.zitadel.dev", tokens: tm, http: &http.Client{},
	}

	err := client.UpdateProjectRole(context.Background(), "proj-789", "editor", "Senior Editor", "staff")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedPath := "/management/v1/projects/proj-789/roles/editor"
	if capturedPath != expectedPath {
		t.Errorf("expected path %s, got %s", expectedPath, capturedPath)
	}
	if capturedMethod != http.MethodPut {
		t.Errorf("expected PUT, got %s", capturedMethod)
	}
	if capturedBody["displayName"] != "Senior Editor" {
		t.Errorf("expected displayName=Senior Editor, got %v", capturedBody["displayName"])
	}
}
