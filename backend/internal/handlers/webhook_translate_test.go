package handlers

import (
	"fmt"
	"slices"
	"testing"
)

// realZitadelBody renders a Zitadel ContextInfoEvent body for the supplied
// fields. The body matches zitadel/zitadel:internal/repository/execution/queue.go
// exactly — top-level flat fields, snake_case for event_type/event_payload/created_at,
// camelCase-with-uppercase-ID for aggregateID/aggregateType/instanceID/userID.
func realZitadelBody(aggID, aggType, eventType, editorID, payload string) []byte {
	return fmt.Appendf(nil,
		`{"aggregateID":%q,"aggregateType":%q,"resourceOwner":"org","instanceID":"inst","version":"v1","sequence":1,"event_type":%q,"created_at":"2026-05-07T00:00:00Z","userID":%q,"event_payload":%s}`,
		aggID, aggType, eventType, editorID, payload,
	)
}

func TestTranslateZitadelEvent_DetectsZitadelShape(t *testing.T) {
	body := realZitadelBody("user-123", "user", "user.human.added", "editor-9", `{}`)

	got, ok, err := translateZitadelEvent(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected zitadel shape detected")
	}
	if got.EventType != "user_created" {
		t.Errorf("expected user_created, got %q", got.EventType)
	}
	if got.UserID != "user-123" {
		t.Errorf("expected user_id=user-123, got %q", got.UserID)
	}
}

func TestTranslateZitadelEvent_PassesInternalShape(t *testing.T) {
	internal := []byte(`{"event_type":"user_created","user_id":"u1","source_project":"p1"}`)
	_, ok, err := translateZitadelEvent(internal)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatalf("expected ok=false for internal shape (no aggregateID field)")
	}
}

func TestTranslateZitadelEvent_RealWireFormat(t *testing.T) {
	// Each case is a real-shape Zitadel ContextInfoEvent body. UserID at the
	// top level is the editor; the subject of grant events lives in
	// event_payload.userId; the subject of user.* events is the aggregateID.
	cases := []struct {
		name        string
		eventType   string
		aggID       string
		aggType     string
		editorID    string
		payloadJSON string
		wantType    string
		wantUserID  string
		wantProject string
		wantRoles   []string
		wantGrantID string
	}{
		{"user added", "user.human.added", "u1", "user", "editor-1", `{"userName":"u1@example"}`, "user_created", "u1", "", nil, ""},
		{"self registered", "user.human.selfregistered", "u2", "user", "editor-2", `{"userName":"u2@example"}`, "user_created", "u2", "", nil, ""},
		{"deactivated", "user.deactivated", "u3", "user", "editor-3", `null`, "user_deactivated", "u3", "", nil, ""},
		{"locked", "user.locked", "u4", "user", "editor-4", `null`, "user_locked", "u4", "", nil, ""},
		{"grant added", "user.grant.added", "g-aggr-1", "user_grant", "editor-5", `{"userId":"u5","projectId":"p1","grantId":"g-aggr-1","roleKeys":["alpha","beta"]}`, "grant_added", "u5", "p1", []string{"alpha", "beta"}, "g-aggr-1"},
		{"grant changed (no projectId in payload)", "user.grant.changed", "g-aggr-2", "user_grant", "editor-6", `{"userId":"u6","roleKeys":["gamma"]}`, "grant_changed", "u6", "", []string{"gamma"}, "g-aggr-2"},
		{"grant removed (no roleKeys in payload)", "user.grant.removed", "g-aggr-3", "user_grant", "editor-7", `{"userId":"u7","projectId":"p3","grantId":"g-aggr-3"}`, "grant_removed", "u7", "p3", nil, "g-aggr-3"},
		{"unknown event", "user.password.changed", "u8", "user", "editor-8", `{}`, "", "", "", nil, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := realZitadelBody(tc.aggID, tc.aggType, tc.eventType, tc.editorID, tc.payloadJSON)
			got, ok, err := translateZitadelEvent(body)
			if err != nil {
				t.Fatalf("err = %v, want nil", err)
			}
			if !ok {
				t.Fatalf("ok = false, want true")
			}
			if got.EventType != tc.wantType {
				t.Errorf("EventType = %q, want %q", got.EventType, tc.wantType)
			}
			if got.UserID != tc.wantUserID {
				t.Errorf("UserID = %q, want %q", got.UserID, tc.wantUserID)
			}
			if got.SourceProject != tc.wantProject {
				t.Errorf("SourceProject = %q, want %q", got.SourceProject, tc.wantProject)
			}
			if !slices.Equal(got.RoleKeys, tc.wantRoles) {
				t.Errorf("RoleKeys = %v, want %v", got.RoleKeys, tc.wantRoles)
			}
			if got.GrantID != tc.wantGrantID {
				t.Errorf("GrantID = %q, want %q", got.GrantID, tc.wantGrantID)
			}
		})
	}
}

func TestTranslateZitadelEvent_SelfMutationGuard_RealWireFormat(t *testing.T) {
	t.Setenv("ZITADEL_M2M_USER_ID", "service-user-99")

	body := realZitadelBody("u1", "user_grant", "user.grant.added", "service-user-99",
		`{"userId":"u1","projectId":"p1","roleKeys":["x"]}`)

	_, ok, err := translateZitadelEvent(body)
	if !ok {
		t.Fatal("expected ok=true (Zitadel-shape body must be recognized even when guard fires)")
	}
	if err != errSelfMutation {
		t.Fatalf("expected errSelfMutation, got %v", err)
	}
}
