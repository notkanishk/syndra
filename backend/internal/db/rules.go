package db

import (
	"context"

	"mkauth/internal/models"
)

// -------------------------------------------------------------
// MAPPING RULES REPOSITORY
// -------------------------------------------------------------

// GetMappingRuleByID fetches a single mapping rule by id, used by the cascade orchestrator.
func GetMappingRuleByID(ctx context.Context, id string) (models.MappingRule, error) {
	var r models.MappingRule
	err := PG.QueryRow(ctx,
		`SELECT id, source_zitadel_project_id, source_zitadel_role_key,
		        target_zitadel_project_id, target_zitadel_role_key, confirmation_mode, created_at
		 FROM mapping_rules WHERE id = $1`, id).
		Scan(&r.ID, &r.SourceProject, &r.SourceRole, &r.TargetProject, &r.TargetRole, &r.ConfirmationMode, &r.CreatedAt)
	return r, err
}

func GetActiveMappingRules(ctx context.Context) ([]models.MappingRule, error) {
	query := `
		SELECT id, source_zitadel_project_id, source_zitadel_role_key, target_zitadel_project_id, target_zitadel_role_key, confirmation_mode, created_at
		FROM mapping_rules;`

	rows, err := PG.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []models.MappingRule
	for rows.Next() {
		var r models.MappingRule
		if err := rows.Scan(&r.ID, &r.SourceProject, &r.SourceRole, &r.TargetProject, &r.TargetRole, &r.ConfirmationMode, &r.CreatedAt); err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, nil
}
