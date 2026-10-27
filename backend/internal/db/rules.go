package db

import (
	"context"

	"syndra/internal/models"
)

// -------------------------------------------------------------
// MAPPING RULES REPOSITORY
// -------------------------------------------------------------

// GetMappingRuleByID fetches a single mapping rule by id, retired or not: the cascade
// orchestrator reads it before changing it, and a retired rule's page reads when it was retired.
// Callers that write must refuse a retired one (RetiredAt != nil).
func GetMappingRuleByID(ctx context.Context, id string) (models.MappingRule, error) {
	var r models.MappingRule
	err := querier(ctx).QueryRow(ctx,
		`SELECT id, source_zitadel_project_id, source_zitadel_role_key,
		        target_zitadel_project_id, target_zitadel_role_key, confirmation_mode, created_at,
		        retired_at, COALESCE(retired_by, '')
		 FROM mapping_rules WHERE id = $1`, id).
		Scan(&r.ID, &r.SourceProject, &r.SourceRole, &r.TargetProject, &r.TargetRole, &r.ConfirmationMode, &r.CreatedAt,
			&r.RetiredAt, &r.RetiredBy)
	return r, err
}

// GetActiveMappingRules is the rules in force: everything that evaluates a rule — cascades,
// closures, the claims cache, drift — reads this, so a retired rule is skipped by all of them at
// once.
func GetActiveMappingRules(ctx context.Context) ([]models.MappingRule, error) {
	return listMappingRules(ctx, `
		SELECT id, source_zitadel_project_id, source_zitadel_role_key, target_zitadel_project_id, target_zitadel_role_key, confirmation_mode, created_at
		FROM mapping_rules WHERE retired_at IS NULL;`, false)
}

// GetRetiredMappingRules lists the rules that stopped being given, most recently retired first.
// They are history to read, never input to a decision.
func GetRetiredMappingRules(ctx context.Context) ([]models.MappingRule, error) {
	return listMappingRules(ctx, `
		SELECT id, source_zitadel_project_id, source_zitadel_role_key, target_zitadel_project_id, target_zitadel_role_key, confirmation_mode, created_at,
		       retired_at, COALESCE(retired_by, '')
		FROM mapping_rules WHERE retired_at IS NOT NULL ORDER BY retired_at DESC;`, true)
}

func listMappingRules(ctx context.Context, query string, retired bool) ([]models.MappingRule, error) {
	rows, err := querier(ctx).Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []models.MappingRule
	for rows.Next() {
		var r models.MappingRule
		dest := []any{&r.ID, &r.SourceProject, &r.SourceRole, &r.TargetProject, &r.TargetRole, &r.ConfirmationMode, &r.CreatedAt}
		if retired {
			dest = append(dest, &r.RetiredAt, &r.RetiredBy)
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return rules, nil
}
