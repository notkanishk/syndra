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

// GetAllDirectGrants returns every MkAuth-direct grant in the system. When
// includeExpired is false the result is filtered to active grants only
// (expires_at NULL or in the future). Ordered by user/project/role for
// deterministic pairing during reconciliation.
func GetAllDirectGrants(ctx context.Context, includeExpired bool) ([]models.DirectGrant, error) {
	query := `
		SELECT id, user_id, zitadel_project_id, zitadel_role_key, granted_by, COALESCE(reason, ''), expires_at, created_at, updated_at
		FROM direct_role_grants`
	if !includeExpired {
		query += ` WHERE (expires_at IS NULL OR expires_at > NOW())`
	}
	query += ` ORDER BY user_id, zitadel_project_id, zitadel_role_key`

	rows, err := PG.Query(ctx, query)
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
	// Surface mid-stream iteration errors so reconciliation never compares
	// against a silently-truncated MkAuth inventory.
	if err := rows.Err(); err != nil {
		return nil, err
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

// GetExpiredDirectGrants returns direct role grants whose expires_at is in the
// past, ordered by expires_at ascending (oldest first). Used by the expiry
// scheduler to sweep grants needing cleanup. limit caps the batch size.
func GetExpiredDirectGrants(ctx context.Context, limit int) ([]models.DirectGrant, error) {
	query := `
		SELECT id, user_id, zitadel_project_id, zitadel_role_key, granted_by, COALESCE(reason, ''), expires_at, created_at, updated_at
		FROM direct_role_grants
		WHERE expires_at IS NOT NULL
		  AND expires_at <= NOW()
		ORDER BY expires_at ASC
		LIMIT $1`

	rows, err := PG.Query(ctx, query, limit)
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

// DeleteExpiredDirectGrantsByIDs atomically hard-deletes direct role grants
// for a specific user BUT only rows whose expires_at is still in the past at
// the moment of the DELETE. Rows that were concurrently renewed via
// UpsertDirectGrant (which uses ON CONFLICT DO UPDATE and therefore preserves
// the row ID while pushing expires_at forward) will not satisfy the predicate
// and will survive.
//
// The RETURNING clause guarantees the caller sees the exact set of rows that
// were actually removed; downstream steps (intent emission, audit, cascade)
// MUST be driven from this returned slice, not from the pre-fetch snapshot.
//
// The user_id scoping is defensive: it guarantees a caller cannot delete
// another user's grants even if IDs were mis-grouped upstream.
func DeleteExpiredDirectGrantsByIDs(ctx context.Context, userID string, ids []string) ([]models.DirectGrant, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	query := `
		DELETE FROM direct_role_grants
		WHERE user_id = $1
		  AND id = ANY($2::uuid[])
		  AND expires_at IS NOT NULL
		  AND expires_at <= NOW()
		RETURNING id, user_id, zitadel_project_id, zitadel_role_key, granted_by, COALESCE(reason, ''), expires_at, created_at, updated_at`

	rows, err := PG.Query(ctx, query, userID, ids)
	if err != nil {
		return nil, fmt.Errorf("failed to delete expired direct grants: %w", err)
	}
	defer rows.Close()

	var deleted []models.DirectGrant
	for rows.Next() {
		var g models.DirectGrant
		if err := rows.Scan(&g.ID, &g.UserID, &g.ProjectID, &g.RoleKey, &g.GrantedBy, &g.Reason, &g.ExpiresAt, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, err
		}
		deleted = append(deleted, g)
	}
	return deleted, nil
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
// CLAIM PROFILES
// -------------------------------------------------------------

// ClaimProfileRow is the application-facing view of the claim_profiles table.
// Used by the directory layer to overlay claim-shaping metadata on top of live
// Zitadel project discovery.
type ClaimProfileRow struct {
	ProjectID  string
	ClaimName  string
	FormatType string
}

// ListClaimProfiles returns every claim_profiles row. Used to overlay
// application metadata (ClaimName, FormatType) on live Zitadel projects when
// the directory layer is running in live mode.
func ListClaimProfiles(ctx context.Context) ([]ClaimProfileRow, error) {
	rows, err := PG.Query(ctx, `SELECT zitadel_project_id, claim_name, format_type FROM claim_profiles`)
	if err != nil {
		return nil, fmt.Errorf("list claim profiles: %w", err)
	}
	defer rows.Close()

	var out []ClaimProfileRow
	for rows.Next() {
		var r ClaimProfileRow
		if err := rows.Scan(&r.ProjectID, &r.ClaimName, &r.FormatType); err != nil {
			return nil, fmt.Errorf("scan claim profile row: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate claim profiles: %w", err)
	}
	return out, nil
}

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

// --- Webhook Events ---

// WebhookEvent is the DB representation of a webhook_events row.
type WebhookEvent struct {
	ID             string     `json:"id"`
	EventType      string     `json:"event_type"`
	UserID         string     `json:"user_id"`
	SourceProject  string     `json:"source_project"`
	RoleKey        string     `json:"role_key,omitempty"`
	IdempotencyKey string     `json:"idempotency_key"`
	Status         string     `json:"status"`
	ErrorMessage   string     `json:"error_message,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	ProcessedAt    *time.Time `json:"processed_at,omitempty"`
}

// InsertWebhookEvent records a received webhook event using the idempotency key.
// Returns (id, true, nil) on insert, ("", false, nil) if the key already exists.
func InsertWebhookEvent(ctx context.Context, eventType, userID, sourceProject, roleKey, idempotencyKey string) (string, bool, error) {
	query := `
		INSERT INTO webhook_events (event_type, user_id, source_project, role_key, idempotency_key)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (idempotency_key) DO NOTHING
		RETURNING id`

	var id string
	err := PG.QueryRow(ctx, query, eventType, userID, sourceProject, roleKey, idempotencyKey).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("insert webhook event (key=%s): %w", idempotencyKey, err)
	}
	return id, true, nil
}

// CompleteWebhookEvent marks an event as processed.
func CompleteWebhookEvent(ctx context.Context, eventID string) error {
	query := `
		UPDATE webhook_events
		SET status = 'processed', processed_at = NOW()
		WHERE id = $1`
	_, err := PG.Exec(ctx, query, eventID)
	if err != nil {
		return fmt.Errorf("complete webhook event: %w", err)
	}
	return nil
}

// FailWebhookEvent marks an event as failed with an error message.
func FailWebhookEvent(ctx context.Context, eventID, errMsg string) error {
	query := `
		UPDATE webhook_events
		SET status = 'failed', error_message = $2, processed_at = NOW()
		WHERE id = $1`
	_, err := PG.Exec(ctx, query, eventID, errMsg)
	if err != nil {
		return fmt.Errorf("fail webhook event: %w", err)
	}
	return nil
}

// GetWebhookEvents returns webhook events ordered by creation time.
// If statusFilter is non-empty, only events with that status are returned.
func GetWebhookEvents(ctx context.Context, statusFilter string) ([]WebhookEvent, error) {
	var query string
	var args []any

	if statusFilter != "" {
		query = `
			SELECT id, event_type, user_id, source_project, COALESCE(role_key, ''),
			       idempotency_key, status, COALESCE(error_message, ''),
			       created_at, processed_at
			FROM webhook_events
			WHERE status = $1
			ORDER BY created_at DESC`
		args = []any{statusFilter}
	} else {
		query = `
			SELECT id, event_type, user_id, source_project, COALESCE(role_key, ''),
			       idempotency_key, status, COALESCE(error_message, ''),
			       created_at, processed_at
			FROM webhook_events
			ORDER BY created_at DESC`
	}

	rows, err := PG.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query webhook events: %w", err)
	}
	defer rows.Close()

	var events []WebhookEvent
	for rows.Next() {
		var e WebhookEvent
		if err := rows.Scan(&e.ID, &e.EventType, &e.UserID, &e.SourceProject, &e.RoleKey,
			&e.IdempotencyKey, &e.Status, &e.ErrorMessage, &e.CreatedAt, &e.ProcessedAt); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, nil
}

// -------------------------------------------------------------
// ROLE MANAGEMENT
// -------------------------------------------------------------

// RoleUsage holds per-role usage counts from bundles and mapping rules.
type RoleUsage struct {
	BundleCount int
	RuleCount   int
}

// ErrDuplicateRole is returned when a role with the same (project, key) already exists.
var ErrDuplicateRole = errors.New("role already exists")

// CreateRole persists a new MkAuth-managed role. Returns the ID on success.
// Returns ErrDuplicateRole if the (project, roleKey) pair already exists.
func CreateRole(ctx context.Context, projectID, roleKey, displayName, description, group, createdBy string,
	clonedFromProject, clonedFromRole *string) (string, error) {
	query := `
		INSERT INTO roles (zitadel_project_id, role_key, display_name, description, role_group,
		                   created_by, cloned_from_project, cloned_from_role)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (zitadel_project_id, role_key) DO NOTHING
		RETURNING id`

	var id string
	err := PG.QueryRow(ctx, query, projectID, roleKey, displayName, description, group,
		createdBy, clonedFromProject, clonedFromRole).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrDuplicateRole
		}
		return "", fmt.Errorf("insert role: %w", err)
	}
	return id, nil
}

// DeleteRole removes a role by ID. Used for compensating rollback when
// Zitadel propagation fails after local insert.
func DeleteRole(ctx context.Context, id string) error {
	_, err := PG.Exec(ctx, `DELETE FROM roles WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete role: %w", err)
	}
	return nil
}

// GetRole retrieves a single role by its natural key.
func GetRole(ctx context.Context, projectID, roleKey string) (models.Role, error) {
	query := `
		SELECT id, zitadel_project_id, role_key, display_name, description, role_group,
		       COALESCE(cloned_from_project, ''), COALESCE(cloned_from_role, ''),
		       created_by, created_at, updated_at
		FROM roles
		WHERE zitadel_project_id = $1 AND role_key = $2`

	var r models.Role
	err := PG.QueryRow(ctx, query, projectID, roleKey).Scan(
		&r.ID, &r.ProjectID, &r.RoleKey, &r.DisplayName, &r.Description, &r.Group,
		&r.ClonedFromProject, &r.ClonedFromRole, &r.CreatedBy, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return models.Role{}, fmt.Errorf("get role: %w", err)
	}
	return r, nil
}

// GetAllLocalRoles returns all roles created through MkAuth.
func GetAllLocalRoles(ctx context.Context) ([]models.Role, error) {
	query := `
		SELECT id, zitadel_project_id, role_key, display_name, description, role_group,
		       COALESCE(cloned_from_project, ''), COALESCE(cloned_from_role, ''),
		       created_by, created_at, updated_at
		FROM roles
		ORDER BY created_at DESC`

	rows, err := PG.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query roles: %w", err)
	}
	defer rows.Close()

	var roles []models.Role
	for rows.Next() {
		var r models.Role
		if err := rows.Scan(&r.ID, &r.ProjectID, &r.RoleKey, &r.DisplayName, &r.Description, &r.Group,
			&r.ClonedFromProject, &r.ClonedFromRole, &r.CreatedBy, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		roles = append(roles, r)
	}
	return roles, nil
}

// GetRoleUsageCounts returns per-role usage from bundle_roles and mapping_rules.
// The key is "projectID:roleKey".
func GetRoleUsageCounts(ctx context.Context) (map[string]RoleUsage, error) {
	// Count bundle_roles per (project, role).
	bundleQuery := `
		SELECT zitadel_project_id, zitadel_role_key, COUNT(*)
		FROM bundle_roles
		GROUP BY zitadel_project_id, zitadel_role_key`

	rows, err := PG.Query(ctx, bundleQuery)
	if err != nil {
		return nil, fmt.Errorf("query bundle role counts: %w", err)
	}
	defer rows.Close()

	usage := make(map[string]RoleUsage)
	for rows.Next() {
		var pid, rk string
		var count int
		if err := rows.Scan(&pid, &rk, &count); err != nil {
			return nil, err
		}
		key := pid + ":" + rk
		u := usage[key]
		u.BundleCount = count
		usage[key] = u
	}

	// Count mapping_rules: a role can appear as source or target.
	ruleQuery := `
		SELECT project_id, role_key, SUM(cnt) FROM (
			SELECT source_zitadel_project_id AS project_id,
			       source_zitadel_role_key AS role_key,
			       COUNT(*) AS cnt
			FROM mapping_rules
			GROUP BY source_zitadel_project_id, source_zitadel_role_key
			UNION ALL
			SELECT target_zitadel_project_id,
			       target_zitadel_role_key,
			       COUNT(*)
			FROM mapping_rules
			GROUP BY target_zitadel_project_id, target_zitadel_role_key
		) sub
		GROUP BY project_id, role_key`

	rows2, err := PG.Query(ctx, ruleQuery)
	if err != nil {
		return nil, fmt.Errorf("query rule counts: %w", err)
	}
	defer rows2.Close()

	for rows2.Next() {
		var pid, rk string
		var count int
		if err := rows2.Scan(&pid, &rk, &count); err != nil {
			return nil, err
		}
		key := pid + ":" + rk
		u := usage[key]
		u.RuleCount = count
		usage[key] = u
	}

	return usage, nil
}

// GetAssignedUserCounts returns the count of distinct users per (projectID, roleKey)
// from direct_role_grants. The key is "projectID:roleKey".
func GetAssignedUserCounts(ctx context.Context) (map[string]int, error) {
	query := `
		SELECT zitadel_project_id, zitadel_role_key, COUNT(DISTINCT user_id)
		FROM direct_role_grants
		WHERE (expires_at IS NULL OR expires_at > NOW())
		GROUP BY zitadel_project_id, zitadel_role_key`

	rows, err := PG.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query assigned user counts: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var pid, rk string
		var count int
		if err := rows.Scan(&pid, &rk, &count); err != nil {
			return nil, err
		}
		counts[pid+":"+rk] = count
	}
	return counts, nil
}

// GetAllReferencedRoleKeys returns all unique (projectID, roleKey) pairs referenced
// across bundle_roles, mapping_rules, and direct_role_grants.
func GetAllReferencedRoleKeys(ctx context.Context) ([][2]string, error) {
	query := `
		SELECT DISTINCT project_id, role_key FROM (
			SELECT zitadel_project_id AS project_id, zitadel_role_key AS role_key FROM bundle_roles
			UNION
			SELECT source_zitadel_project_id, source_zitadel_role_key FROM mapping_rules
			UNION
			SELECT target_zitadel_project_id, target_zitadel_role_key FROM mapping_rules
			UNION
			SELECT zitadel_project_id, zitadel_role_key FROM direct_role_grants
		) all_refs
		ORDER BY project_id, role_key`

	rows, err := PG.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query referenced role keys: %w", err)
	}
	defer rows.Close()

	var refs [][2]string
	for rows.Next() {
		var pid, rk string
		if err := rows.Scan(&pid, &rk); err != nil {
			return nil, err
		}
		refs = append(refs, [2]string{pid, rk})
	}
	return refs, nil
}

// -------------------------------------------------------------
// PROVISIONING INTENTS
// -------------------------------------------------------------

// InsertProvisioningIntent records a new provisioning intent using the idempotency key.
// Returns (id, true, nil) on insert, ("", false, nil) if the key already exists.
func InsertProvisioningIntent(ctx context.Context, targetUID, action, lldapGroup,
	sourceProject, sourceRole, webhookEventID, idempotencyKey string) (string, bool, error) {
	query := `
		INSERT INTO provisioning_intents (target_uid, action, lldap_group,
			source_project, source_role, webhook_event_id, idempotency_key)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, '')::uuid, $7)
		ON CONFLICT (idempotency_key) DO NOTHING
		RETURNING id`

	var id string
	err := PG.QueryRow(ctx, query, targetUID, action, lldapGroup,
		sourceProject, sourceRole, webhookEventID, idempotencyKey).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("insert provisioning intent (key=%s): %w", idempotencyKey, err)
	}
	return id, true, nil
}

// ClaimPendingIntents atomically selects up to `limit` pending intents and transitions
// them to 'acknowledged' in a single operation. Uses FOR UPDATE SKIP LOCKED to prevent
// concurrent workers from claiming the same intents.
func ClaimPendingIntents(ctx context.Context, limit int) ([]models.ProvisioningIntent, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `
		WITH claimed AS (
			SELECT id FROM provisioning_intents
			WHERE status = 'pending'
			ORDER BY created_at ASC
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE provisioning_intents pi
		SET status = 'acknowledged', acknowledged_at = NOW()
		FROM claimed
		WHERE pi.id = claimed.id
		RETURNING pi.id, pi.target_uid, pi.action, pi.lldap_group, pi.source_project,
		          pi.source_role, COALESCE(pi.webhook_event_id::text, ''), pi.idempotency_key,
		          pi.status, COALESCE(pi.error_message, ''), pi.created_at,
		          pi.acknowledged_at, pi.completed_at`

	rows, err := PG.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("claim pending intents: %w", err)
	}
	defer rows.Close()

	var intents []models.ProvisioningIntent
	for rows.Next() {
		var i models.ProvisioningIntent
		if err := rows.Scan(&i.ID, &i.TargetUID, &i.Action, &i.LLDAPGroup,
			&i.SourceProject, &i.SourceRole, &i.WebhookEventID, &i.IdempotencyKey,
			&i.Status, &i.ErrorMessage, &i.CreatedAt, &i.AcknowledgedAt, &i.CompletedAt); err != nil {
			return nil, err
		}
		intents = append(intents, i)
	}
	return intents, nil
}

// CompleteIntent transitions an intent from 'acknowledged' to 'completed'.
func CompleteIntent(ctx context.Context, intentID string) error {
	query := `
		UPDATE provisioning_intents
		SET status = 'completed', completed_at = NOW()
		WHERE id = $1 AND status = 'acknowledged'`

	tag, err := PG.Exec(ctx, query, intentID)
	if err != nil {
		return fmt.Errorf("complete intent: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("intent not found or not in acknowledged status")
	}
	return nil
}

// FailIntent transitions an intent to 'failed' with an error message.
func FailIntent(ctx context.Context, intentID, errMsg string) error {
	query := `
		UPDATE provisioning_intents
		SET status = 'failed', error_message = $2, completed_at = NOW()
		WHERE id = $1 AND status IN ('pending', 'acknowledged')`

	tag, err := PG.Exec(ctx, query, intentID, errMsg)
	if err != nil {
		return fmt.Errorf("fail intent: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("intent not found or already terminal")
	}
	return nil
}

// GetProvisioningIntents returns provisioning intents ordered by creation time DESC.
// If statusFilter is non-empty, only intents with that status are returned.
func GetProvisioningIntents(ctx context.Context, statusFilter string) ([]models.ProvisioningIntent, error) {
	var query string
	var args []any

	if statusFilter != "" {
		query = `
			SELECT id, target_uid, action, lldap_group, source_project, source_role,
			       COALESCE(webhook_event_id::text, ''), idempotency_key, status,
			       COALESCE(error_message, ''), created_at, acknowledged_at, completed_at
			FROM provisioning_intents
			WHERE status = $1
			ORDER BY created_at DESC`
		args = []any{statusFilter}
	} else {
		query = `
			SELECT id, target_uid, action, lldap_group, source_project, source_role,
			       COALESCE(webhook_event_id::text, ''), idempotency_key, status,
			       COALESCE(error_message, ''), created_at, acknowledged_at, completed_at
			FROM provisioning_intents
			ORDER BY created_at DESC`
	}

	rows, err := PG.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query provisioning intents: %w", err)
	}
	defer rows.Close()

	var intents []models.ProvisioningIntent
	for rows.Next() {
		var i models.ProvisioningIntent
		if err := rows.Scan(&i.ID, &i.TargetUID, &i.Action, &i.LLDAPGroup,
			&i.SourceProject, &i.SourceRole, &i.WebhookEventID, &i.IdempotencyKey,
			&i.Status, &i.ErrorMessage, &i.CreatedAt, &i.AcknowledgedAt, &i.CompletedAt); err != nil {
			return nil, err
		}
		intents = append(intents, i)
	}
	return intents, nil
}

// ---------------------------------------------------------------------------
// Shadow Password Vault
// ---------------------------------------------------------------------------

// UpsertShadowCredential inserts or replaces a user's shadow credential.
// On conflict (same user_id), the credential is updated and rotated_at is set.
func UpsertShadowCredential(ctx context.Context, userID, hash, algorithm, saltParams string) (string, error) {
	var id string
	err := PG.QueryRow(ctx, `
		INSERT INTO shadow_credentials (user_id, credential_hash, algorithm, salt_params)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id) DO UPDATE SET
			credential_hash = EXCLUDED.credential_hash,
			algorithm       = EXCLUDED.algorithm,
			salt_params     = EXCLUDED.salt_params,
			updated_at      = NOW(),
			rotated_at      = NOW()
		RETURNING id`,
		userID, hash, algorithm, saltParams).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("upsert shadow credential (user=%s): %w", userID, err)
	}
	return id, nil
}

// GetShadowCredential returns the full credential row including the hash.
// Intended for the sync service only — never expose the hash to user-facing APIs.
func GetShadowCredential(ctx context.Context, userID string) (models.ShadowCredential, error) {
	var c models.ShadowCredential
	err := PG.QueryRow(ctx, `
		SELECT id, user_id, credential_hash, algorithm, salt_params,
		       created_at, updated_at, rotated_at, expires_at
		FROM shadow_credentials WHERE user_id = $1`, userID).
		Scan(&c.ID, &c.UserID, &c.CredentialHash, &c.Algorithm, &c.SaltParams,
			&c.CreatedAt, &c.UpdatedAt, &c.RotatedAt, &c.ExpiresAt)
	if err != nil {
		return c, fmt.Errorf("get shadow credential (user=%s): %w", userID, err)
	}
	return c, nil
}

// DeleteShadowCredential removes a user's shadow credential.
func DeleteShadowCredential(ctx context.Context, userID string) error {
	tag, err := PG.Exec(ctx, `DELETE FROM shadow_credentials WHERE user_id = $1`, userID)
	if err != nil {
		return fmt.Errorf("delete shadow credential (user=%s): %w", userID, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("shadow credential not found for user %s: %w", userID, pgx.ErrNoRows)
	}
	return nil
}

// HasShadowCredential checks whether a user has a shadow credential.
// The hash is deliberately excluded from the SELECT.
func HasShadowCredential(ctx context.Context, userID string) (models.ShadowCredentialStatus, error) {
	var s models.ShadowCredentialStatus
	var algorithm string
	var createdAt, updatedAt time.Time
	var rotatedAt, expiresAt *time.Time
	err := PG.QueryRow(ctx, `
		SELECT algorithm, created_at, updated_at, rotated_at, expires_at
		FROM shadow_credentials WHERE user_id = $1`, userID).
		Scan(&algorithm, &createdAt, &updatedAt, &rotatedAt, &expiresAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.ShadowCredentialStatus{HasCredential: false}, nil
		}
		return s, fmt.Errorf("check shadow credential (user=%s): %w", userID, err)
	}
	return models.ShadowCredentialStatus{
		HasCredential: true,
		Algorithm:     algorithm,
		CreatedAt:     &createdAt,
		UpdatedAt:     &updatedAt,
		RotatedAt:     rotatedAt,
		ExpiresAt:     expiresAt,
	}, nil
}

// InsertShadowCredentialAudit records a credential lifecycle event.
func InsertShadowCredentialAudit(ctx context.Context, userID, action, actorID, ipAddress string) error {
	_, err := PG.Exec(ctx, `
		INSERT INTO shadow_credential_audit (user_id, action, actor_id, ip_address)
		VALUES ($1, $2, $3, $4)`,
		userID, action, actorID, ipAddress)
	if err != nil {
		return fmt.Errorf("insert shadow credential audit (user=%s action=%s): %w", userID, action, err)
	}
	return nil
}

// GetShadowCredentialAudit returns the audit trail for a user's shadow credential.
func GetShadowCredentialAudit(ctx context.Context, userID string) ([]models.ShadowCredentialAudit, error) {
	rows, err := PG.Query(ctx, `
		SELECT id, user_id, action, actor_id, ip_address, created_at
		FROM shadow_credential_audit
		WHERE user_id = $1
		ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("query shadow credential audit (user=%s): %w", userID, err)
	}
	defer rows.Close()

	var entries []models.ShadowCredentialAudit
	for rows.Next() {
		var e models.ShadowCredentialAudit
		if err := rows.Scan(&e.ID, &e.UserID, &e.Action, &e.ActorID, &e.IPAddress, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}
