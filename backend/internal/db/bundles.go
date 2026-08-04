package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"syndra/internal/models"
)

// -------------------------------------------------------------
// BUNDLES REPOSITORY
// -------------------------------------------------------------

// CreateBundle creates a bundle and publishes its empty v1 in one transaction.
//
// The empty v1 is deliberate. Every assignment pins a version, so a bundle with
// no published version could not be assigned at all — and blocking assignment
// on "publish something first" is a failure mode invented to avoid an
// uninteresting row in a history. v1 is honest: the bundle existed and granted
// nothing, which is what the empty-bundle copy has always said.
func CreateBundle(ctx context.Context, name string, description string, confirmationMode string) (string, error) {
	tx, err := PG.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	var id string
	if err := tx.QueryRow(ctx,
		`INSERT INTO bundles (name, description, confirmation_mode) VALUES ($1, $2, $3) RETURNING id;`,
		name, description, NormalizeConfirmationMode(confirmationMode)).Scan(&id); err != nil {
		return "", fmt.Errorf("failed to insert bundle: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO bundle_versions (bundle_id, version, note, published_by)
		 VALUES ($1, 1, 'Created empty.', 'system')`, id); err != nil {
		return "", fmt.Errorf("failed to publish initial bundle version: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return id, nil
}

// UpdateBundle renames a bundle and rewrites its description.
//
// It is deliberately not a cascade and deliberately not a version. A bundle's
// name is what operators call it, not what it grants: nobody's access changes,
// no holder falls behind, and publishing a version to fix a typo would put a
// meaningless entry in a history that exists to answer "what changed for whom".
func UpdateBundle(ctx context.Context, id, name, description string) error {
	tag, err := PG.Exec(ctx,
		`UPDATE bundles SET name = $2, description = $3 WHERE id = $1`, id, name, description)
	if err != nil {
		return fmt.Errorf("update bundle (id=%s): %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// DeleteBundleAndEnqueue deletes a bundle AND enqueues the caller-computed revokes for everyone
// who held it, in ONE tx (mirrors RemoveBundleFromUserAndEnqueue, which is the same operation
// for one person).
//
// The transaction is the point. Every table hanging off a bundle cascades on delete, so the
// assignment rows vanish the moment the bundle does — and a holder whose assignment disappeared
// without a revoke keeps the role in Zitadel with nothing in Syndra left to explain it. That is
// the definition of drift, and it would arrive with no actor, found weeks later by the sweep.
//
// params may be empty: a bundle nobody holds, or one whose every role each holder also gets
// somewhere else, deletes cleanly and enqueues nothing.
func DeleteBundleAndEnqueue(ctx context.Context, actor, bundleID string, params []EnqueueParams) ([]string, error) {
	tx, err := PG.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `DELETE FROM bundles WHERE id = $1`, bundleID)
	if err != nil {
		return nil, fmt.Errorf("delete bundle (id=%s): %w", bundleID, err)
	}
	if tag.RowsAffected() == 0 {
		return nil, pgx.ErrNoRows
	}
	ids, err := enqueueCascadeRows(ctx, tx,
		[]CascadeAudit{{Actor: actor, Action: "bundle.deleted", ResourceID: bundleID}},
		params)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return ids, nil
}

func GetAllBundles(ctx context.Context) ([]models.Bundle, error) {
	query := `
		SELECT b.id, b.name, b.description, b.is_welcome, b.confirmation_mode, b.created_at,
		       COALESCE((SELECT MAX(version) FROM bundle_versions v WHERE v.bundle_id = b.id), 0)
		FROM bundles b ORDER BY b.created_at DESC;`
	rows, err := PG.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bundles []models.Bundle
	for rows.Next() {
		var b models.Bundle
		if err := rows.Scan(&b.ID, &b.Name, &b.Description, &b.IsWelcome, &b.ConfirmationMode,
			&b.CreatedAt, &b.LatestVersion); err != nil {
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

// AssignBundleToUser pins the assignment to the bundle's latest published
// version. A new assignment always gets the current bundle; being left behind
// is something that happens to you later, by a decision somebody took.
func AssignBundleToUser(ctx context.Context, userID, bundleID string) error {
	query := `
		INSERT INTO user_bundle_assignments (user_id, bundle_id, version_id)
		SELECT $1, $2, id FROM bundle_versions
		WHERE bundle_id = $2 ORDER BY version DESC LIMIT 1
		ON CONFLICT DO NOTHING;`
	_, err := PG.Exec(ctx, query, userID, bundleID)
	if err != nil {
		return fmt.Errorf("failed to assign bundle: %w", err)
	}
	return nil
}

func GetBundlesForUser(ctx context.Context, userID string) ([]models.Bundle, error) {
	query := `
		SELECT b.id, b.name, b.description, b.is_welcome, b.confirmation_mode, b.created_at,
		       bv.version,
		       COALESCE((SELECT MAX(version) FROM bundle_versions v WHERE v.bundle_id = b.id), 0)
		FROM bundles b
		JOIN user_bundle_assignments uba ON b.id = uba.bundle_id
		JOIN bundle_versions bv ON bv.id = uba.version_id
		WHERE uba.user_id = $1;`

	rows, err := PG.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bundles []models.Bundle
	for rows.Next() {
		var b models.Bundle
		if err := rows.Scan(&b.ID, &b.Name, &b.Description, &b.IsWelcome, &b.ConfirmationMode,
			&b.CreatedAt, &b.PinnedVersion, &b.LatestVersion); err != nil {
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
