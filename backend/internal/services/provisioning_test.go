package services

import (
	"context"
	"fmt"
	"testing"
)

func resetProvisioningDeps(t *testing.T) {
	t.Helper()
	origInsert := svcInsertProvisioningIntent
	origAudit := svcInsertAuditLog
	t.Cleanup(func() {
		svcInsertProvisioningIntent = origInsert
		svcInsertAuditLog = origAudit
	})
}

func noopProvisioningDeps() {
	svcInsertProvisioningIntent = func(_ context.Context, _, _, _, _, _, _, _ string) (string, bool, error) {
		return "intent-1", true, nil
	}
	svcInsertAuditLog = func(_ context.Context, _, _, _, _ string) error { return nil }
}

func TestEmitProvisioningIntent_Success(t *testing.T) {
	resetProvisioningDeps(t)
	noopProvisioningDeps()

	var capturedGroup, capturedAction string
	svcInsertProvisioningIntent = func(_ context.Context, _, action, group, _, _, _, _ string) (string, bool, error) {
		capturedAction = action
		capturedGroup = group
		return "intent-new", true, nil
	}

	err := EmitProvisioningIntent(context.Background(), "u1", "add", "printing", "member", "evt-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedAction != "add" {
		t.Errorf("expected action=add, got %s", capturedAction)
	}
	// P1 fix: group derived from stable project ID, not display name.
	if capturedGroup != "printing_member" {
		t.Errorf("expected group=printing_member, got %s", capturedGroup)
	}
}

func TestEmitProvisioningIntent_Duplicate(t *testing.T) {
	resetProvisioningDeps(t)
	noopProvisioningDeps()

	svcInsertProvisioningIntent = func(_ context.Context, _, _, _, _, _, _, _ string) (string, bool, error) {
		return "", false, nil // duplicate
	}

	err := EmitProvisioningIntent(context.Background(), "u1", "add", "printing", "member", "evt-1")
	if err != nil {
		t.Fatalf("duplicate should return nil, got: %v", err)
	}
}

func TestEmitProvisioningIntent_StableGroupFromID(t *testing.T) {
	resetProvisioningDeps(t)
	noopProvisioningDeps()

	// P1 fix verification: the group should use the project ID directly, not a resolved name.
	var capturedGroup string
	svcInsertProvisioningIntent = func(_ context.Context, _, _, group, _, _, _, _ string) (string, bool, error) {
		capturedGroup = group
		return "intent-2", true, nil
	}

	err := EmitProvisioningIntent(context.Background(), "u1", "add", "my_project", "role1", "evt-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Group should be "my_project_role1" — derived from stable ID, not a display name.
	if capturedGroup != "my_project_role1" {
		t.Errorf("expected my_project_role1, got %s", capturedGroup)
	}
}

func TestEmitProvisioningIntent_DBFailure(t *testing.T) {
	resetProvisioningDeps(t)
	noopProvisioningDeps()

	svcInsertProvisioningIntent = func(_ context.Context, _, _, _, _, _, _, _ string) (string, bool, error) {
		return "", false, fmt.Errorf("connection refused")
	}

	err := EmitProvisioningIntent(context.Background(), "u1", "add", "p1", "r1", "evt-3")
	if err == nil {
		t.Fatal("expected error on DB failure")
	}
}

func TestEmitProvisioningIntent_AuditFailureNonFatal(t *testing.T) {
	resetProvisioningDeps(t)
	noopProvisioningDeps()

	svcInsertAuditLog = func(_ context.Context, _, _, _, _ string) error {
		return fmt.Errorf("audit unavailable")
	}

	// Should succeed despite audit failure.
	err := EmitProvisioningIntent(context.Background(), "u1", "add", "p1", "r1", "evt-4")
	if err != nil {
		t.Fatalf("audit failure should be non-fatal, got: %v", err)
	}
}
