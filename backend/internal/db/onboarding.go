package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// -------------------------------------------------------------
// ONBOARDING TRIGGERS (Backend-Owned Onboarding)
// -------------------------------------------------------------

// InsertOnboardingTrigger records a new onboarding event using the idempotency key.
// Returns (id, true, nil) on insert, ("", false, nil) if the key already exists (duplicate).
func InsertOnboardingTrigger(ctx context.Context, userID, source, idempotencyKey string) (string, bool, error) {
	query := `
		INSERT INTO onboarding_triggers (user_id, source, idempotency_key)
		VALUES ($1, $2, $3)
		ON CONFLICT (idempotency_key) DO NOTHING
		RETURNING id`

	var id string
	err := PG.QueryRow(ctx, query, userID, source, idempotencyKey).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// ON CONFLICT DO NOTHING — idempotency key already exists, safe to skip
			return "", false, nil
		}
		// Real DB fault: surface it so the caller records the failure and retries
		return "", false, fmt.Errorf("insert onboarding trigger (key=%s): %w", idempotencyKey, err)
	}
	return id, true, nil
}

// CompleteOnboardingTrigger marks a trigger as completed with the assigned bundle.
func CompleteOnboardingTrigger(ctx context.Context, triggerID, bundleID string) error {
	query := `
		UPDATE onboarding_triggers
		SET status = 'completed', bundle_id = $2, completed_at = NOW()
		WHERE id = $1`
	_, err := PG.Exec(ctx, query, triggerID, bundleID)
	if err != nil {
		return fmt.Errorf("failed to complete onboarding trigger: %w", err)
	}
	return nil
}

// FailOnboardingTrigger marks a trigger as failed with an error message.
func FailOnboardingTrigger(ctx context.Context, triggerID, errMsg string) error {
	query := `
		UPDATE onboarding_triggers
		SET status = 'failed', error_message = $2, completed_at = NOW()
		WHERE id = $1`
	_, err := PG.Exec(ctx, query, triggerID, errMsg)
	if err != nil {
		return fmt.Errorf("failed to record onboarding failure: %w", err)
	}
	return nil
}

// GetOnboardingTriggers returns all onboarding triggers ordered by creation time.
func GetOnboardingTriggers(ctx context.Context) ([]OnboardingTrigger, error) {
	query := `
		SELECT id, user_id, source, idempotency_key, status,
		       COALESCE(bundle_id::text, ''), COALESCE(error_message, ''),
		       created_at, completed_at
		FROM onboarding_triggers
		ORDER BY created_at DESC`

	rows, err := PG.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query onboarding triggers: %w", err)
	}
	defer rows.Close()

	var triggers []OnboardingTrigger
	for rows.Next() {
		var t OnboardingTrigger
		if err := rows.Scan(&t.ID, &t.UserID, &t.Source, &t.IdempotencyKey,
			&t.Status, &t.BundleID, &t.ErrorMessage, &t.CreatedAt, &t.CompletedAt); err != nil {
			return nil, err
		}
		triggers = append(triggers, t)
	}
	return triggers, nil
}

// ErrNoWelcomeBundleConfigured is returned by GetWelcomeBundle when no row in
// the bundles table has is_welcome=TRUE. Onboarding propagates this verbatim
// so operators see a named cause in the trigger UI instead of getting the
// "first bundle by created_at" silent default that the May 2026 audit (D1)
// flagged as a trust hazard.
var ErrNoWelcomeBundleConfigured = errors.New("no welcome bundle configured")

// GetWelcomeBundle returns the ID of the bundle marked is_welcome=TRUE, or
// ErrNoWelcomeBundleConfigured if no bundle has been designated. The
// at-most-one constraint is enforced at the schema layer
// (idx_bundles_welcome_unique).
func GetWelcomeBundle(ctx context.Context) (string, error) {
	var id string
	err := PG.QueryRow(ctx, `SELECT id FROM bundles WHERE is_welcome = TRUE`).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNoWelcomeBundleConfigured
	}
	if err != nil {
		return "", fmt.Errorf("query welcome bundle: %w", err)
	}
	return id, nil
}

// SetWelcomeBundle marks bundleID as the welcome bundle. Transactional
// clear-then-set: any previously-flagged bundle is unset before the new one
// is set, so the partial unique index never trips. Returns pgx.ErrNoRows if
// bundleID does not exist.
func SetWelcomeBundle(ctx context.Context, bundleID string) error {
	tx, err := PG.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin set-welcome-bundle: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `UPDATE bundles SET is_welcome = FALSE WHERE is_welcome = TRUE`); err != nil {
		return fmt.Errorf("clear previous welcome bundle: %w", err)
	}
	tag, err := tx.Exec(ctx, `UPDATE bundles SET is_welcome = TRUE WHERE id = $1`, bundleID)
	if err != nil {
		return fmt.Errorf("mark welcome bundle: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit set-welcome-bundle: %w", err)
	}
	return nil
}

// OnboardingTrigger is the DB representation of an onboarding_triggers row.
type OnboardingTrigger struct {
	ID             string     `json:"id"`
	UserID         string     `json:"user_id"`
	Source         string     `json:"source"`
	IdempotencyKey string     `json:"idempotency_key"`
	Status         string     `json:"status"`
	BundleID       string     `json:"bundle_id,omitempty"`
	ErrorMessage   string     `json:"error_message,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
}
