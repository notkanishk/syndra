package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// canonicalV2Payload is the full shape Zitadel sends for a
// `function/preaccesstoken` trigger. Fields beyond user and user_grants are
// included to prove the lenient decoder ignores them without error, even as
// Zitadel extends the schema over time.
const canonicalV2Payload = `{
  "function": "function/preaccesstoken",
  "userinfo": {
    "sub": "250124622953808001",
    "name": "Test User",
    "email": "test@example.com",
    "email_verified": true
  },
  "user": {
    "id": "250124622953808001",
    "creationDate": "2024-02-01T10:00:00Z",
    "changeDate": "2024-04-01T12:00:00Z",
    "state": "USER_STATE_ACTIVE",
    "human": {
      "profile": {
        "firstName": "Test",
        "lastName": "User",
        "displayName": "Test User",
        "preferredLanguage": "en"
      },
      "email": {
        "email": "test@example.com",
        "isEmailVerified": true
      }
    }
  },
  "user_metadata": [
    {"key": "department", "value": "aGFyZHdhcmU="}
  ],
  "org": {
    "id": "250124592953808001",
    "name": "Ashoka Makerspace",
    "primary_domain": "makerspace.example.org"
  },
  "application": {"client_id": "printing-portal@example"},
  "user_grants": [
    {
      "id": "grant-1",
      "projectId": "pPrinting",
      "projectName": "Printing Portal",
      "grantId": "",
      "roles": ["operator"]
    },
    {
      "id": "grant-2",
      "projectId": "pPrinting",
      "projectName": "Printing Portal",
      "grantId": "",
      "roles": ["viewer"]
    }
  ]
}`

// TestHandleActionInject_CanonicalV2Payload_Accepted proves the reshaped
// handler consumes the full Zitadel Actions v2 function payload (with every
// documented field populated) without rejecting it, and returns the v2
// envelope. This is the wire-level contract round-trip test.
func TestHandleActionInject_CanonicalV2Payload_Accepted(t *testing.T) {
	origRedis, origMode := redisGetClaims, dbGetClaimFailureMode
	resetActionDeps(t, origRedis, origMode)

	redisGetClaims = func(ctx context.Context, key string) (string, error) {
		if key == "mapping:250124622953808001:pPrinting" {
			return factsJSON("250124622953808001", "pPrinting", "operator"), nil
		}
		return "", fmt.Errorf("unexpected key: %s", key)
	}
	dbGetClaimFailureMode = func(ctx context.Context, projectID string) (string, map[string]interface{}, error) {
		t.Errorf("failure mode queried despite cache hits for project=%s", projectID)
		return "fail_closed", nil, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/action/inject", strings.NewReader(canonicalV2Payload))
	rr := httptest.NewRecorder()
	HandleActionInject(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("canonical v2 payload rejected: %d (body=%s)", rr.Code, rr.Body.String())
	}

	got := decodeActionResponse(t, rr)
	// Zitadel lists only the token's own project; its two rows (one per role)
	// are one project, and with no configured profile it emits the built-in key.
	// Roles come from Syndra's compiled facts, not the payload's role list.
	if len(got.AppendClaims) != 1 {
		t.Fatalf("expected 1 append_claim, got %v", got.AppendClaims)
	}
	printing, ok := claimByKey(got.AppendClaims, "roles")
	if !ok || !rolesEqual(printing.Value, "operator") {
		t.Fatalf("expected roles=[operator], got %v", got.AppendClaims)
	}
}

// rolesEqual compares a decoded array-format roles claim against expected values.
func rolesEqual(value interface{}, want ...string) bool {
	list, ok := value.([]interface{})
	if !ok || len(list) != len(want) {
		return false
	}
	for i, w := range want {
		if list[i] != w {
			return false
		}
	}
	return true
}
