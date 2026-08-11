package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"syndra/internal/models"
)

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

// CreateRole persists a new Syndra-managed role. Returns the ID on success.
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
	err := querier(ctx).QueryRow(ctx, query, projectID, roleKey, displayName, description, group,
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
	_, err := querier(ctx).Exec(ctx, `DELETE FROM roles WHERE id = $1`, id)
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
	err := querier(ctx).QueryRow(ctx, query, projectID, roleKey).Scan(
		&r.ID, &r.ProjectID, &r.RoleKey, &r.DisplayName, &r.Description, &r.Group,
		&r.ClonedFromProject, &r.ClonedFromRole, &r.CreatedBy, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return models.Role{}, fmt.Errorf("get role: %w", err)
	}
	return r, nil
}

// GetAllLocalRoles returns all roles created through Syndra.
func GetAllLocalRoles(ctx context.Context) ([]models.Role, error) {
	query := `
		SELECT id, zitadel_project_id, role_key, display_name, description, role_group,
		       COALESCE(cloned_from_project, ''), COALESCE(cloned_from_role, ''),
		       created_by, created_at, updated_at
		FROM roles
		ORDER BY created_at DESC`

	rows, err := querier(ctx).Query(ctx, query)
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

	rows, err := querier(ctx).Query(ctx, bundleQuery)
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

	rows2, err := querier(ctx).Query(ctx, ruleQuery)
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

	rows, err := querier(ctx).Query(ctx, query)
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

	rows, err := querier(ctx).Query(ctx, query)
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
