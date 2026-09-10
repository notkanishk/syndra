package services

import (
	"context"
	"errors"
	"testing"

	"syndra/internal/db"
)

// resetOnboardingDeps restores all onboarding injectable vars after a test.
func resetOnboardingDeps(t *testing.T) (
	origInsert func(context.Context, string, string, string) (string, bool, error),
	origBundle func(context.Context) (string, error),
	origAssign func(context.Context, string, string, string) (CascadeResult, error),
	origAudit func(context.Context, string, string, string, string) error,
	origComplete func(context.Context, string, string) error,
	origFail func(context.Context, string, string) error,
) {
	t.Helper()
	origInsert = svcInsertOnboardingTrigger
	origBundle = svcGetWelcomeBundle
	origAssign = svcCascadeWelcomeBundle
	origAudit = svcInsertAuditLog
	origComplete = svcCompleteOnboardingTrigger
	origFail = svcFailOnboardingTrigger
	origUnconfigured := svcMarkOnboardingTriggerUnconfigured

	t.Cleanup(func() {
		svcInsertOnboardingTrigger = origInsert
		svcGetWelcomeBundle = origBundle
		svcCascadeWelcomeBundle = origAssign
		svcInsertAuditLog = origAudit
		svcCompleteOnboardingTrigger = origComplete
		svcFailOnboardingTrigger = origFail
		svcMarkOnboardingTriggerUnconfigured = origUnconfigured
	})
	return
}

func happyPathDeps() {
	svcInsertOnboardingTrigger = func(_ context.Context, _, _, _ string) (string, bool, error) {
		return "trigger-id-1", true, nil
	}
	svcGetWelcomeBundle = func(_ context.Context) (string, error) {
		return "bundle-id-1", nil
	}
	svcCascadeWelcomeBundle = func(context.Context, string, string, string) (CascadeResult, error) { return CascadeResult{}, nil }
	svcInsertAuditLog = func(_ context.Context, _, _, _, _ string) error { return nil }
	svcCompleteOnboardingTrigger = func(_ context.Context, _, _ string) error { return nil }
	svcFailOnboardingTrigger = func(_ context.Context, _, _ string) error { return nil }
	svcMarkOnboardingTriggerUnconfigured = func(_ context.Context, _, _ string) error { return nil }
}

func TestTriggerOnboarding_HappyPath(t *testing.T) {
	resetOnboardingDeps(t)
	happyPathDeps()

	if err := TriggerOnboarding(context.Background(), "u1", "webhook", "key-1"); err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
}

func TestTriggerOnboarding_IdempotentDuplicate(t *testing.T) {
	// Duplicate key: InsertOnboardingTrigger returns (_, false, nil).
	// TriggerOnboarding must return nil (not an error) and must NOT attempt any
	// further mutations — verifying that the idempotency gate stops all downstream work.
	resetOnboardingDeps(t)

	svcInsertOnboardingTrigger = func(_ context.Context, _, _, _ string) (string, bool, error) {
		return "", false, nil // duplicate — already processed
	}
	bundleCalled := false
	svcGetWelcomeBundle = func(_ context.Context) (string, error) {
		bundleCalled = true
		return "bundle-id-1", nil
	}
	svcCascadeWelcomeBundle = func(context.Context, string, string, string) (CascadeResult, error) {
		t.Error("AssignBundleToUser called on duplicate trigger — should not happen")
		return CascadeResult{}, nil
	}

	if err := TriggerOnboarding(context.Background(), "u1", "webhook", "key-dup"); err != nil {
		t.Fatalf("duplicate trigger should return nil, got: %v", err)
	}
	if bundleCalled {
		t.Fatal("GetWelcomeBundle should not be called for duplicate trigger")
	}
}

func TestTriggerOnboarding_DBFaultOnInsert_Propagates(t *testing.T) {
	// A real DB fault on insertion must propagate so the caller knows onboarding
	// did not complete and can log/retry.
	resetOnboardingDeps(t)

	svcInsertOnboardingTrigger = func(_ context.Context, _, _, _ string) (string, bool, error) {
		return "", false, errors.New("DB connection refused")
	}
	svcGetWelcomeBundle = func(_ context.Context) (string, error) {
		t.Error("GetWelcomeBundle should not be called after insert failure")
		return "", nil
	}

	err := TriggerOnboarding(context.Background(), "u1", "webhook", "key-fault")
	if err == nil {
		t.Fatal("expected error on DB insert fault, got nil")
	}
}

func TestTriggerOnboarding_NoBundleAvailable_MarksUnconfiguredNotFailed(t *testing.T) {
	// No welcome bundle configured is not a fault (migration 000046): the
	// trigger must be marked 'unconfigured', never 'failed', and the named
	// sentinel still propagates so callers (the webhook handler) can
	// errors.Is against it to decide not to retry.
	resetOnboardingDeps(t)

	svcInsertOnboardingTrigger = func(_ context.Context, _, _, _ string) (string, bool, error) {
		return "trigger-id-2", true, nil
	}
	svcGetWelcomeBundle = func(_ context.Context) (string, error) {
		return "", db.ErrNoWelcomeBundleConfigured
	}
	unconfiguredID := ""
	svcMarkOnboardingTriggerUnconfigured = func(_ context.Context, triggerID, _ string) error {
		unconfiguredID = triggerID
		return nil
	}
	svcFailOnboardingTrigger = func(_ context.Context, triggerID, _ string) error {
		t.Errorf("FailOnboardingTrigger called for a missing bundle (trigger=%s) — that reads as a fault, and it is not one", triggerID)
		return nil
	}
	svcCascadeWelcomeBundle = func(context.Context, string, string, string) (CascadeResult, error) {
		t.Error("AssignBundleToUser called when no bundle available")
		return CascadeResult{}, nil
	}

	err := TriggerOnboarding(context.Background(), "u1", "webhook", "key-nobundle")
	if err == nil {
		t.Fatal("expected error when no bundle available")
	}
	if !errors.Is(err, db.ErrNoWelcomeBundleConfigured) {
		t.Fatalf("expected ErrNoWelcomeBundleConfigured, got %v", err)
	}
	if unconfiguredID != "trigger-id-2" {
		t.Fatalf("expected MarkOnboardingTriggerUnconfigured called with trigger-id-2, got %q", unconfiguredID)
	}
}

func TestTriggerOnboarding_WelcomeBundleLookupDBFault_MarksFailed(t *testing.T) {
	// A real fault reading the bundle table (not the named sentinel) is a
	// defect, unlike the case above, and belongs in 'failed' — an operator
	// needs to be able to tell "nothing is configured" from "something broke"
	// without reading the error text.
	resetOnboardingDeps(t)

	svcInsertOnboardingTrigger = func(_ context.Context, _, _, _ string) (string, bool, error) {
		return "trigger-id-fault", true, nil
	}
	svcGetWelcomeBundle = func(_ context.Context) (string, error) {
		return "", errors.New("connection refused")
	}
	failedID := ""
	svcFailOnboardingTrigger = func(_ context.Context, triggerID, _ string) error {
		failedID = triggerID
		return nil
	}
	svcMarkOnboardingTriggerUnconfigured = func(_ context.Context, triggerID, _ string) error {
		t.Errorf("MarkOnboardingTriggerUnconfigured called for a DB fault (trigger=%s) — that hides a real defect", triggerID)
		return nil
	}

	err := TriggerOnboarding(context.Background(), "u1", "webhook", "key-dbfault")
	if err == nil {
		t.Fatal("expected error on DB fault reading the welcome bundle")
	}
	if errors.Is(err, db.ErrNoWelcomeBundleConfigured) {
		t.Fatal("a connection error must not match the no-bundle-configured sentinel")
	}
	if failedID != "trigger-id-fault" {
		t.Fatalf("expected FailOnboardingTrigger called with trigger-id-fault, got %q", failedID)
	}
}

func TestTriggerOnboarding_AssignmentFails_MarksFailedAndReturnsError(t *testing.T) {
	// Assignment failure must also mark the trigger failed, not leave it in 'pending'.
	resetOnboardingDeps(t)

	svcInsertOnboardingTrigger = func(_ context.Context, _, _, _ string) (string, bool, error) {
		return "trigger-id-3", true, nil
	}
	svcGetWelcomeBundle = func(_ context.Context) (string, error) {
		return "bundle-id-1", nil
	}
	svcCascadeWelcomeBundle = func(context.Context, string, string, string) (CascadeResult, error) {
		return CascadeResult{}, errors.New("FK constraint violation")
	}
	failedID := ""
	svcFailOnboardingTrigger = func(_ context.Context, triggerID, _ string) error {
		failedID = triggerID
		return nil
	}
	svcCompleteOnboardingTrigger = func(_ context.Context, _, _ string) error {
		t.Error("CompleteOnboardingTrigger called after assignment failure")
		return nil
	}

	err := TriggerOnboarding(context.Background(), "u1", "webhook", "key-failassign")
	if err == nil {
		t.Fatal("expected error when assignment fails")
	}
	if failedID != "trigger-id-3" {
		t.Fatalf("expected FailOnboardingTrigger called with trigger-id-3, got %q", failedID)
	}
}

func TestTriggerOnboarding_CompletionMarkedAfterSuccess(t *testing.T) {
	// On success the trigger must be marked completed (not left pending).
	resetOnboardingDeps(t)
	happyPathDeps()

	completedID := ""
	svcCompleteOnboardingTrigger = func(_ context.Context, triggerID, _ string) error {
		completedID = triggerID
		return nil
	}

	if err := TriggerOnboarding(context.Background(), "u1", "webhook", "key-complete"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if completedID != "trigger-id-1" {
		t.Fatalf("expected CompleteOnboardingTrigger called with trigger-id-1, got %q", completedID)
	}
}
