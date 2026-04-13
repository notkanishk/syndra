package services

import (
	"context"

	"mkauth/internal/db"
)

// Onboarding DB injectable vars — allows tests to exercise TriggerOnboarding
// without a live database connection.
var (
	svcInsertOnboardingTrigger = func(ctx context.Context, userID, source, key string) (string, bool, error) {
		return db.InsertOnboardingTrigger(ctx, userID, source, key)
	}
	svcGetWelcomeBundle = func(ctx context.Context) (string, error) {
		return db.GetWelcomeBundle(ctx)
	}
	svcAssignBundleToUser = func(ctx context.Context, userID, bundleID string) error {
		return db.AssignBundleToUser(ctx, userID, bundleID)
	}
	svcInsertAuditLog = func(ctx context.Context, actorID, targetID, action, resourceID string) error {
		return db.InsertAuditLog(ctx, actorID, targetID, action, resourceID)
	}
	svcCompleteOnboardingTrigger = func(ctx context.Context, triggerID, bundleID string) error {
		return db.CompleteOnboardingTrigger(ctx, triggerID, bundleID)
	}
	svcFailOnboardingTrigger = func(ctx context.Context, triggerID, errMsg string) error {
		return db.FailOnboardingTrigger(ctx, triggerID, errMsg)
	}
)
