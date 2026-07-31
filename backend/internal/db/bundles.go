package db

import (
	"context"
	"fmt"

	"mkauth/internal/models"
)

// -------------------------------------------------------------
// BUNDLES REPOSITORY
// -------------------------------------------------------------

func CreateBundle(ctx context.Context, name string, description string, confirmationMode string) (string, error) {
	query := `INSERT INTO bundles (name, description, confirmation_mode) VALUES ($1, $2, $3) RETURNING id;`
	var id string
	err := PG.QueryRow(ctx, query, name, description, NormalizeConfirmationMode(confirmationMode)).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("failed to insert bundle: %w", err)
	}
	return id, nil
}

func GetAllBundles(ctx context.Context) ([]models.Bundle, error) {
	query := `SELECT id, name, description, is_welcome, confirmation_mode, created_at FROM bundles ORDER BY created_at DESC;`
	rows, err := PG.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bundles []models.Bundle
	for rows.Next() {
		var b models.Bundle
		if err := rows.Scan(&b.ID, &b.Name, &b.Description, &b.IsWelcome, &b.ConfirmationMode, &b.CreatedAt); err != nil {
			return nil, err
		}
		bundles = append(bundles, b)
	}
	return bundles, nil
}

// GetBundleByID fetches a single bundle by id, used by the cascade orchestrator to read its
// confirmation_mode before fanning out.
func GetBundleByID(ctx context.Context, id string) (models.Bundle, error) {
	var b models.Bundle
	err := PG.QueryRow(ctx,
		`SELECT id, name, description, is_welcome, confirmation_mode, created_at
		 FROM bundles WHERE id = $1`, id).
		Scan(&b.ID, &b.Name, &b.Description, &b.IsWelcome, &b.ConfirmationMode, &b.CreatedAt)
	return b, err
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

func GetBundlesForUser(ctx context.Context, userID string) ([]models.Bundle, error) {
	query := `
		SELECT b.id, b.name, b.description, b.is_welcome, b.confirmation_mode, b.created_at
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
		if err := rows.Scan(&b.ID, &b.Name, &b.Description, &b.IsWelcome, &b.ConfirmationMode, &b.CreatedAt); err != nil {
			return nil, err
		}
		bundles = append(bundles, b)
	}
	return bundles, nil
}

// GetBundleHolderCounts returns holder counts keyed by bundle id, in one query
// rather than one per bundle. Editing a bundle changes access for every holder
// at once, so the number belongs on the list beside the name — and a list that
// fanned out a query per row would be a list nobody keeps open.
func GetBundleHolderCounts(ctx context.Context) (map[string]int, error) {
	const q = `SELECT bundle_id, COUNT(DISTINCT user_id) FROM user_bundle_assignments GROUP BY bundle_id`
	rows, err := PG.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("query bundle holder counts: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var id string
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, fmt.Errorf("scan bundle holder count: %w", err)
		}
		counts[id] = n
	}
	return counts, rows.Err()
}
