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
    "primary_domain": "makerspace.example.edu"
  },
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
      "projectId": "pDoors",
      "projectName": "Door Access",
      "grantId": "",
      "roles": ["3d_lab_pin"]
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
		switch key {
		case "mapping:250124622953808001:pPrinting":
			return factsJSON("250124622953808001", "pPrinting", "operator"), nil
		case "mapping:250124622953808001:pDoors":
			return factsJSON("250124622953808001", "pDoors", "3d_lab_pin"), nil
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
	// Neither project has a configured claim profile, so both fall back to the
	// built-in "roles" key. The merge step must keep BOTH sets by namespacing
	// the collision rather than letting one project's roles land under a key
	// the other project's application also reads.
	if len(got.AppendClaims) != 2 {
		t.Fatalf("expected 2 append_claims (one per project), got %v", got.AppendClaims)
	}
	printing, okP := claimByKey(got.AppendClaims, "syndra.pPrinting.roles")
	doors, okD := claimByKey(got.AppendClaims, "syndra.pDoors.roles")
	if !okP || !okD {
		t.Fatalf("expected the colliding default key to be namespaced per project, got %v", got.AppendClaims)
	}
	if !rolesEqual(printing.Value, "operator") {
		t.Errorf("expected printing roles=[operator], got %v", printing.Value)
	}
	if !rolesEqual(doors.Value, "3d_lab_pin") {
		t.Errorf("expected doors roles=[3d_lab_pin], got %v", doors.Value)
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
