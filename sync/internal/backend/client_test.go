package backend

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClaimIntents_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/intents/claim" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode([]ProvisioningIntent{
			{ID: "i1", TargetUID: "u1", Action: "add", LLDAPGroup: "printing_member"},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-key")
	intents, err := c.ClaimIntents(context.Background(), 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(intents) != 1 {
		t.Fatalf("expected 1 intent, got %d", len(intents))
	}
	if intents[0].ID != "i1" {
		t.Errorf("expected id=i1, got %s", intents[0].ID)
	}
}

func TestClaimIntents_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]ProvisioningIntent{})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-key")
	intents, err := c.ClaimIntents(context.Background(), 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(intents) != 0 {
		t.Fatalf("expected 0 intents, got %d", len(intents))
	}
}

func TestClaimIntents_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("db down"))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-key")
	_, err := c.ClaimIntents(context.Background(), 50)
	if err == nil {
		t.Fatal("expected error on 500")
	}
}

func TestCompleteIntent_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/intents/i1/complete" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]string{"message": "ok"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-key")
	if err := c.CompleteIntent(context.Background(), "i1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFailIntent_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/intents/i1/fail" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["error_message"] != "ldap timeout" {
			t.Errorf("expected error_message=ldap timeout, got %s", body["error_message"])
		}
		json.NewEncoder(w).Encode(map[string]string{"message": "ok"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-key")
	if err := c.FailIntent(context.Background(), "i1", "ldap timeout"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetShadowCredentialHash_Found(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ShadowCredentialHash{
			CredentialHash: "$argon2id$v=19$m=65536,t=1,p=4$salt$key",
			Algorithm:      "argon2id",
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-key")
	hash, algo, err := c.GetShadowCredentialHash(context.Background(), "u1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hash == "" {
		t.Error("expected non-empty hash")
	}
	if algo != "argon2id" {
		t.Errorf("expected algorithm=argon2id, got %s", algo)
	}
}

func TestGetShadowCredentialHash_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"NOT_FOUND"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-key")
	hash, algo, err := c.GetShadowCredentialHash(context.Background(), "u1")
	if err != nil {
		t.Fatalf("expected nil error on 404, got: %v", err)
	}
	if hash != "" || algo != "" {
		t.Errorf("expected empty hash/algo on 404, got hash=%s algo=%s", hash, algo)
	}
}

func TestGetUserProfile_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/users/u1/profile" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(UserProfile{
			UserID:      "u1",
			DisplayName: "Alice Smith",
			Email:       "alice@example.com",
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-key")
	profile, err := c.GetUserProfile(context.Background(), "u1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile.DisplayName != "Alice Smith" {
		t.Errorf("expected Alice Smith, got %s", profile.DisplayName)
	}
}

func TestAuthorizationHeader(t *testing.T) {
	var capturedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		json.NewEncoder(w).Encode([]ProvisioningIntent{})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "my-secret-key")
	c.ClaimIntents(context.Background(), 1)
	if capturedAuth != "Bearer my-secret-key" {
		t.Errorf("expected Bearer my-secret-key, got %s", capturedAuth)
	}
}
