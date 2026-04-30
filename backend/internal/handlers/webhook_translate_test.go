package handlers

import (
	"slices"
	"testing"
)

func TestTranslateZitadelEvent_DetectsZitadelShape(t *testing.T) {
	zitadelBody := []byte(`{
		"aggregate": {"id": "user-123", "type": "user", "resourceOwner": "org-1"},
		"event": "user.human.added",
		"editorUserId": "editor-9",
		"payload": {}
	}`)

	got, ok, err := translateZitadelEvent(zitadelBody)
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
		t.Fatalf("expected ok=false for internal shape (no aggregate field)")
	}
}

func TestTranslateEventName_AllMappings(t *testing.T) {
	cases := []struct {
		name      string
		event     string
		aggID     string
		payload   string
		wantType  string
		wantUser  string
		wantRoles []string
	}{
		{"user added", "user.human.added", "u1", `{}`, "user_created", "u1", nil},
		{"self registered", "user.human.selfregistered", "u2", `{}`, "user_created", "u2", nil},
		{"deactivated", "user.human.deactivated", "u3", `{}`, "user_deactivated", "u3", nil},
		{"locked", "user.human.locked", "u4", `{}`, "user_locked", "u4", nil},
		{"grant added", "user.grant.added", "g1", `{"userId":"u5","projectId":"p1","roleKeys":["alpha","beta"]}`, "grant_added", "u5", []string{"alpha", "beta"}},
		{"grant changed", "user.grant.changed", "g2", `{"userId":"u6","projectId":"p2","roleKeys":["gamma"]}`, "grant_changed", "u6", []string{"gamma"}},
		{"grant removed", "user.grant.removed", "g3", `{"userId":"u7","projectId":"p3","roleKeys":["delta"]}`, "grant_removed", "u7", []string{"delta"}},
		{"unknown", "user.password.changed", "u8", `{}`, "", "", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := []byte(`{"aggregate":{"id":"` + tc.aggID + `"},"event":"` + tc.event + `","payload":` + tc.payload + `}`)
			got, ok, err := translateZitadelEvent(body)
			if err != nil || !ok {
				t.Fatalf("translate failed: ok=%v err=%v", ok, err)
			}
			if got.EventType != tc.wantType {
				t.Errorf("event_type: want %q, got %q", tc.wantType, got.EventType)
			}
			if got.UserID != tc.wantUser {
				t.Errorf("user_id: want %q, got %q", tc.wantUser, got.UserID)
			}
			if !slices.Equal(got.RoleKeys, tc.wantRoles) {
				t.Errorf("role_keys: want %v, got %v", tc.wantRoles, got.RoleKeys)
			}
		})
	}
}

func TestTranslateZitadelEvent_SelfMutationGuard(t *testing.T) {
	t.Setenv("ZITADEL_M2M_USER_ID", "service-user-99")

	// Each shape carries the editor id in a different documented location
	// (see design.md note 2). All three must trigger the guard.
	cases := []struct {
		name string
		body string
	}{
		{
			"top-level editorUserId",
			`{"aggregate":{"id":"u1"},"event":"user.grant.added","editorUserId":"service-user-99","payload":{}}`,
		},
		{
			"aggregate.editorUserId",
			`{"aggregate":{"id":"u1","editorUserId":"service-user-99"},"event":"user.grant.added","payload":{}}`,
		},
		{
			"editor.userId",
			`{"aggregate":{"id":"u1"},"event":"user.grant.added","editor":{"userId":"service-user-99"},"payload":{}}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, ok, err := translateZitadelEvent([]byte(tc.body))
			if !ok {
				t.Fatal("expected ok=true (zitadel shape detected)")
			}
			if err != errSelfMutation {
				t.Fatalf("expected errSelfMutation, got %v", err)
			}
		})
	}
}
