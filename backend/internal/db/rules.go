package db

import (
	"context"
	"fmt"

	"mkauth/internal/models"
)

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
