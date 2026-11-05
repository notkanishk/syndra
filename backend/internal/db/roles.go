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

// UpdateRole sets a Syndra-recorded role's display name and group. A role with
// no local row (made in Zitadel directly) updates nothing and is not an error.
func UpdateRole(ctx context.Context, projectID, roleKey, displayName, group string) error {
	_, err := querier(ctx).Exec(ctx, `
		UPDATE roles SET display_name = $3, role_group = $4, updated_at = NOW()
		 WHERE zitadel_project_id = $1 AND role_key = $2`, projectID, roleKey, displayName, group)
	if err != nil {
		return fmt.Errorf("update role: %w", err)
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return roles, nil
}

// bundleGivesSQL is every (bundle, role) a bundle gives anybody or will give:
// its working copy (own roles and the pinned versions it includes), its latest
// published version (what a new holder gets) and every version somebody is
// pinned to, each through bundle_version_effective_roles so an include counts.
// The one answer to "which bundles give this role"; retired bundles are
// filtered by the caller's join.
//
// The working copy alone (bundle_roles) missed two cases: a bundle made only
// of includes gave "nothing", and a role dropped from the draft but still held
// through an older version was "unused", which let its delete through.
const bundleGivesSQL = `
	SELECT br.bundle_id, br.zitadel_project_id, br.zitadel_role_key FROM bundle_roles br
	UNION
	SELECT i.parent_bundle_id, r.zitadel_project_id, r.zitadel_role_key
	  FROM bundle_includes i JOIN bundle_version_roles r ON r.version_id = i.child_version_id
	UNION
	SELECT v.bundle_id, r.zitadel_project_id, r.zitadel_role_key
	  FROM bundle_versions v JOIN bundle_version_effective_roles r ON r.version_id = v.id
	 WHERE v.version = (SELECT MAX(x.version) FROM bundle_versions x WHERE x.bundle_id = v.bundle_id)
	    OR EXISTS (SELECT 1 FROM user_bundle_assignments a WHERE a.version_id = v.id)`

// GetBundleRolesGiven is bundleGivesSQL for every live bundle, keyed by bundle
// id, in one read.
func GetBundleRolesGiven(ctx context.Context) (map[string][]models.BundleRole, error) {
	rows, err := querier(ctx).Query(ctx, `
		SELECT g.bundle_id, g.zitadel_project_id, g.zitadel_role_key
		  FROM (`+bundleGivesSQL+`) g
		  JOIN bundles b ON b.id = g.bundle_id AND b.retired_at IS NULL
		 ORDER BY g.bundle_id, g.zitadel_project_id, g.zitadel_role_key`)
	if err != nil {
		return nil, fmt.Errorf("query bundle roles given: %w", err)
	}
	defer rows.Close()
	out := map[string][]models.BundleRole{}
	for rows.Next() {
		var r models.BundleRole
		if err := rows.Scan(&r.BundleID, &r.ProjectID, &r.RoleKey); err != nil {
			return nil, fmt.Errorf("scan bundle role given: %w", err)
		}
		out[r.BundleID] = append(out[r.BundleID], r)
	}
	return out, rows.Err()
}

// GetRoleUsageCounts returns per-role usage from bundles (bundleGivesSQL) and
// mapping_rules. The key is "projectID:roleKey".
func GetRoleUsageCounts(ctx context.Context) (map[string]RoleUsage, error) {
	// Count the live bundles giving each (project, role).
	bundleQuery := `
		SELECT g.zitadel_project_id, g.zitadel_role_key, COUNT(DISTINCT g.bundle_id)
		FROM (` + bundleGivesSQL + `) g JOIN bundles b ON b.id = g.bundle_id AND b.retired_at IS NULL
		GROUP BY g.zitadel_project_id, g.zitadel_role_key`

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
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Count mapping_rules: a role can appear as source or target.
	ruleQuery := `
		SELECT project_id, role_key, SUM(cnt) FROM (
			SELECT source_zitadel_project_id AS project_id,
			       source_zitadel_role_key AS role_key,
			       COUNT(*) AS cnt
			FROM mapping_rules WHERE retired_at IS NULL
			GROUP BY source_zitadel_project_id, source_zitadel_role_key
			UNION ALL
			SELECT target_zitadel_project_id,
			       target_zitadel_role_key,
			       COUNT(*)
			FROM mapping_rules WHERE retired_at IS NULL
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
	if err := rows2.Err(); err != nil {
		return nil, err
	}

	return usage, nil
}

func GetAllReferencedRoleKeys(ctx context.Context) ([][2]string, error) {
	query := `
		SELECT DISTINCT project_id, role_key FROM (
			SELECT g.zitadel_project_id AS project_id, g.zitadel_role_key AS role_key
			  FROM (` + bundleGivesSQL + `) g JOIN bundles b ON b.id = g.bundle_id AND b.retired_at IS NULL
			UNION
			SELECT source_zitadel_project_id, source_zitadel_role_key FROM mapping_rules WHERE retired_at IS NULL
			UNION
			SELECT target_zitadel_project_id, target_zitadel_role_key FROM mapping_rules WHERE retired_at IS NULL
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return refs, nil
}
