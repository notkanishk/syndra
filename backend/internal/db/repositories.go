package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"mkauth/internal/models"
)

// Define internal representations if needed, but we'll use `models.*`

// -------------------------------------------------------------
// BUNDLES REPOSITORY
// -------------------------------------------------------------

func CreateBundle(ctx context.Context, name string, description string) (string, error) {
	query := `INSERT INTO bundles (name, description) VALUES ($1, $2) RETURNING id;`
	var id string
	err := PG.QueryRow(ctx, query, name, description).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("failed to insert bundle: %w", err)
	}
	return id, nil
}

func AddRoleToBundle(ctx context.Context, bundleID, zitadelProjectID, zitadelRoleKey string) error {
	query := `
		INSERT INTO bundle_roles (bundle_id, zitadel_project_id, zitadel_role_key)
		VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING;`
	_, err := PG.Exec(ctx, query, bundleID, zitadelProjectID, zitadelRoleKey)
	if err != nil {
		return fmt.Errorf("failed to map role to bundle: %w", err)
	}
	return nil
}

func GetAllBundles(ctx context.Context) ([]models.Bundle, error) {
	query := `SELECT id, name, description, created_at FROM bundles ORDER BY created_at DESC;`
	rows, err := PG.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bundles []models.Bundle
	for rows.Next() {
		var b models.Bundle
		if err := rows.Scan(&b.ID, &b.Name, &b.Description, &b.CreatedAt); err != nil {
			return nil, err
		}
		bundles = append(bundles, b)
	}
	return bundles, nil
}

func GetRolesForBundle(ctx context.Context, bundleID string) ([]models.BundleRole, error) {
	query := `SELECT bundle_id, zitadel_project_id, zitadel_role_key FROM bundle_roles WHERE bundle_id = $1;`
	rows, err := PG.Query(ctx, query, bundleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []models.BundleRole
	for rows.Next() {
		var r models.BundleRole
		if err := rows.Scan(&r.BundleID, &r.ProjectID, &r.RoleKey); err != nil {
			return nil, err
		}
		roles = append(roles, r)
	}
	return roles, nil
}

func AssignBundleToUser(ctx context.Context, userID, bundleID string) error {
	query := `
		INSERT INTO user_bundle_assignments (user_id, bundle_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING;`
	_, err := PG.Exec(ctx, query, userID, bundleID)
	if err != nil {
		return fmt.Errorf("failed to assign bundle: %w", err)
	}
	return nil
}

func RemoveBundleFromUser(ctx context.Context, userID, bundleID string) error {
	query := `DELETE FROM user_bundle_assignments WHERE user_id = $1 AND bundle_id = $2;`
	_, err := PG.Exec(ctx, query, userID, bundleID)
	if err != nil {
		return fmt.Errorf("failed to remove bundle: %w", err)
	}
	return nil
}

func GetBundlesForUser(ctx context.Context, userID string) ([]models.Bundle, error) {
	query := `
		SELECT b.id, b.name, b.description, b.created_at
		FROM bundles b
		JOIN user_bundle_assignments uba ON b.id = uba.bundle_id
		WHERE uba.user_id = $1;`
	
	rows, err := PG.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bundles []models.Bundle
	for rows.Next() {
		var b models.Bundle
		if err := rows.Scan(&b.ID, &b.Name, &b.Description, &b.CreatedAt); err != nil {
			return nil, err
		}
		bundles = append(bundles, b)
	}
	return bundles, nil
}

// -------------------------------------------------------------
// MAPPING RULES REPOSITORY
// -------------------------------------------------------------

func CreateMappingRule(ctx context.Context, sourceProject, sourceRole, targetProject, targetRole string) (string, error) {
	query := `
		INSERT INTO mapping_rules 
		(source_zitadel_project_id, source_zitadel_role_key, target_zitadel_project_id, target_zitadel_role_key) 
		VALUES ($1, $2, $3, $4) 
		RETURNING id;`

	var id string
	err := PG.QueryRow(ctx, query, sourceProject, sourceRole, targetProject, targetRole).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("failed to insert mapping rule (may be duplicate): %w", err)
	}
	return id, nil
}

func UpdateMappingRule(ctx context.Context, id string) error {
	// Increment version only, indicating this rule's logic or downstream effects were reviewed/refreshed
	query := `UPDATE mapping_rules SET version = version + 1 WHERE id = $1;`
	tag, err := PG.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to update mapping rule version: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("mapping rule not found")
	}
	return nil
}

func GetActiveMappingRules(ctx context.Context) ([]models.MappingRule, error) {
	query := `
		SELECT id, source_zitadel_project_id, source_zitadel_role_key, target_zitadel_project_id, target_zitadel_role_key, version, created_at 
		FROM mapping_rules;`
	
	rows, err := PG.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []models.MappingRule
	for rows.Next() {
		var r models.MappingRule
		if err := rows.Scan(&r.ID, &r.SourceProject, &r.SourceRole, &r.TargetProject, &r.TargetRole, &r.Version, &r.CreatedAt); err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, nil
}

// -------------------------------------------------------------
// AUDIT LOG REPOSITORY
// -------------------------------------------------------------

func InsertAuditLog(ctx context.Context, actorID, targetID, action, resourceID string) error {
	query := `INSERT INTO audit_logs (actor_zitadel_user_id, target_zitadel_user_id, action, resource_id) VALUES ($1, $2, $3, $4)`
	_, err := PG.Exec(ctx, query, actorID, targetID, action, resourceID)
	if err != nil {
		return fmt.Errorf("failed to insert audit log: %w", err)
	}
	return nil
}

// -------------------------------------------------------------
// DIRECT ROLE GRANTS
// -------------------------------------------------------------

func UpsertDirectGrant(ctx context.Context, userID, projectID, roleKey, grantedBy, reason string, expiresAt *time.Time) (string, error) {
	query := `
		INSERT INTO direct_role_grants (
			user_id, zitadel_project_id, zitadel_role_key, granted_by, reason, expires_at
		)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, zitadel_project_id, zitadel_role_key)
		DO UPDATE SET
			granted_by = EXCLUDED.granted_by,
			reason = EXCLUDED.reason,
			expires_at = EXCLUDED.expires_at,
			updated_at = CURRENT_TIMESTAMP
		RETURNING id;`

	var id string
	if err := PG.QueryRow(ctx, query, userID, projectID, roleKey, grantedBy, reason, expiresAt).Scan(&id); err != nil {
		return "", fmt.Errorf("failed to upsert direct grant: %w", err)
	}
	return id, nil
}

func GetDirectGrantsForUser(ctx context.Context, userID string, includeExpired bool) ([]models.DirectGrant, error) {
	query := `
		SELECT id, user_id, zitadel_project_id, zitadel_role_key, granted_by, COALESCE(reason, ''), expires_at, created_at, updated_at
		FROM direct_role_grants
		WHERE user_id = $1`
	if !includeExpired {
		query += ` AND (expires_at IS NULL OR expires_at > NOW())`
	}
	query += ` ORDER BY created_at DESC`

	rows, err := PG.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var grants []models.DirectGrant
	for rows.Next() {
		var grant models.DirectGrant
		if err := rows.Scan(&grant.ID, &grant.UserID, &grant.ProjectID, &grant.RoleKey, &grant.GrantedBy, &grant.Reason, &grant.ExpiresAt, &grant.CreatedAt, &grant.UpdatedAt); err != nil {
			return nil, err
		}
		grants = append(grants, grant)
	}
	return grants, nil
}

func GetExpiringDirectGrants(ctx context.Context, within time.Duration) ([]models.DirectGrant, error) {
	query := `
		SELECT id, user_id, zitadel_project_id, zitadel_role_key, granted_by, COALESCE(reason, ''), expires_at, created_at, updated_at
		FROM direct_role_grants
		WHERE expires_at IS NOT NULL
		  AND expires_at > NOW()
		  AND expires_at <= NOW() + $1::interval
		ORDER BY expires_at ASC`

	rows, err := PG.Query(ctx, query, fmt.Sprintf("%f seconds", within.Seconds()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var grants []models.DirectGrant
	for rows.Next() {
		var grant models.DirectGrant
		if err := rows.Scan(&grant.ID, &grant.UserID, &grant.ProjectID, &grant.RoleKey, &grant.GrantedBy, &grant.Reason, &grant.ExpiresAt, &grant.CreatedAt, &grant.UpdatedAt); err != nil {
			return nil, err
		}
		grants = append(grants, grant)
	}
	return grants, nil
}

// -------------------------------------------------------------
// ACCESS REQUESTS
// -------------------------------------------------------------

func CreateAccessRequest(ctx context.Context, requesterID, projectID, roleKey, justification string, durationDays *int) (string, error) {
	query := `
		INSERT INTO access_requests (
			requester_user_id, zitadel_project_id, zitadel_role_key, justification, duration_days
		)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`

	var id string
	if err := PG.QueryRow(ctx, query, requesterID, projectID, roleKey, justification, durationDays).Scan(&id); err != nil {
		return "", fmt.Errorf("failed to create access request: %w", err)
	}
	return id, nil
}

func GetAccessRequests(ctx context.Context, status string) ([]models.AccessRequest, error) {
	query := `
		SELECT id, requester_user_id, zitadel_project_id, zitadel_role_key, justification, duration_days, status, COALESCE(reviewer_user_id, ''), COALESCE(review_note, ''), created_at, resolved_at
		FROM access_requests`
	args := []interface{}{}
	if status != "" {
		query += ` WHERE status = $1`
		args = append(args, status)
	}
	query += ` ORDER BY created_at DESC`

	rows, err := PG.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var requests []models.AccessRequest
	for rows.Next() {
		var req models.AccessRequest
		if err := rows.Scan(&req.ID, &req.RequesterID, &req.ProjectID, &req.RoleKey, &req.Justification, &req.DurationDays, &req.Status, &req.ReviewerID, &req.ReviewNote, &req.CreatedAt, &req.ResolvedAt); err != nil {
			return nil, err
		}
		requests = append(requests, req)
	}
	return requests, nil
}

func GetAccessRequestByID(ctx context.Context, id string) (models.AccessRequest, error) {
	query := `
		SELECT id, requester_user_id, zitadel_project_id, zitadel_role_key, justification, duration_days, status, COALESCE(reviewer_user_id, ''), COALESCE(review_note, ''), created_at, resolved_at
		FROM access_requests
		WHERE id = $1`

	var req models.AccessRequest
	if err := PG.QueryRow(ctx, query, id).Scan(&req.ID, &req.RequesterID, &req.ProjectID, &req.RoleKey, &req.Justification, &req.DurationDays, &req.Status, &req.ReviewerID, &req.ReviewNote, &req.CreatedAt, &req.ResolvedAt); err != nil {
		return req, fmt.Errorf("failed to fetch access request: %w", err)
	}
	return req, nil
}

func ResolveAccessRequest(ctx context.Context, id, status, reviewerID, reviewNote string) error {
	query := `
		UPDATE access_requests
		SET status = $2,
			reviewer_user_id = $3,
			review_note = $4,
			resolved_at = CURRENT_TIMESTAMP
		WHERE id = $1`

	tag, err := PG.Exec(ctx, query, id, status, reviewerID, reviewNote)
	if err != nil {
		return fmt.Errorf("failed to resolve access request: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("access request not found")
	}
	return nil
}

// -------------------------------------------------------------
// CLAIM FAILURE MODE (Data Plane Security Boundary)
// -------------------------------------------------------------

// GetClaimFailureMode returns the configured degraded-mode behavior for a project's
// claim profile. Returns ("fail_closed", nil, nil) if the project has no claim
// profile configured — fail_closed is always the safe default.
func GetClaimFailureMode(ctx context.Context, projectID string) (string, map[string]interface{}, error) {
	query := `
		SELECT claim_failure_mode, minimal_safe_claims
		FROM claim_profiles
		WHERE zitadel_project_id = $1`

	var mode string
	var rawClaims []byte
	err := PG.QueryRow(ctx, query, projectID).Scan(&mode, &rawClaims)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// No claim profile configured for this project — fail_closed is the safe default
			return "fail_closed", nil, nil
		}
		// Real DB fault: surface it so the caller can log and operators can see it
		return "fail_closed", nil, fmt.Errorf("query claim failure mode for project %s: %w", projectID, err)
	}

	if rawClaims == nil {
		return mode, nil, nil
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(rawClaims, &claims); err != nil {
		return "fail_closed", nil, fmt.Errorf("malformed minimal_safe_claims for project %s: %w", projectID, err)
	}
	return mode, claims, nil
}

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

// GetWelcomeBundle returns the ID of the first bundle marked as a welcome bundle,
// or the first bundle in the system if none is specifically designated.
// Returns an error if no bundles exist.
func GetWelcomeBundle(ctx context.Context) (string, error) {
	// Prefer a bundle explicitly named "Welcome" or "welcome" (convention-based)
	query := `
		SELECT id FROM bundles
		WHERE LOWER(name) LIKE '%welcome%'
		ORDER BY created_at ASC
		LIMIT 1`

	var id string
	err := PG.QueryRow(ctx, query).Scan(&id)
	if err == nil {
		return id, nil
	}

	// Fallback: first bundle in the system
	query = `SELECT id FROM bundles ORDER BY created_at ASC LIMIT 1`
	err = PG.QueryRow(ctx, query).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("no bundles available for welcome assignment: %w", err)
	}
	return id, nil
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
