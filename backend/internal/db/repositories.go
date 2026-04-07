package db

import (
	"context"
	"fmt"

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
	query := `INSERT INTO bundle_roles (bundle_id, zitadel_project_id, zitadel_role_key) VALUES ($1, $2, $3);`
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
