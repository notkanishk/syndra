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

// CreateBundle creates a bundle and publishes its v1, containing the roles it
// was created with, in one transaction.
//
// v1 at creation is deliberate. Every assignment pins a version, so a bundle
// with no published version could not be assigned at all — and blocking
// assignment on "publish something first" is a failure mode invented to avoid
// an uninteresting row in a history.
//
// What was NOT deliberate is v1 being EMPTY. A bundle that grants nothing is
// not a state anybody wants a bundle to pass through: assigning it hands out
// nothing, the operator's first edit lands in a working copy they then have to
// notice and publish, and until they do the screen reports "2 unpublished
// changes" about the only two roles the bundle has ever had. The caller is
// required to name at least one role (see handleCreateBundle), so the first
// published version is already the thing the operator described.
//
// The working copy and v1 are written from ONE slice. Publishing diffs the
// working copy against the latest version, so a v1 that disagreed with
// `bundle_roles` by even one row would surface as an unpublished change nobody
// made.
func CreateBundle(
	ctx context.Context,
	name string,
	description string,
	confirmationMode string,
	roles []models.BundleRole,
) (string, error) {
	tx, owned, err := beginOrJoin(ctx)
	if err != nil {
		return "", err
	}
	if owned {
		defer tx.Rollback(ctx)
	}

	var id string
	if err := tx.QueryRow(ctx,
		`INSERT INTO bundles (name, description, confirmation_mode) VALUES ($1, $2, $3) RETURNING id;`,
		name, description, NormalizeConfirmationMode(confirmationMode)).Scan(&id); err != nil {
		return "", fmt.Errorf("failed to insert bundle: %w", err)
	}

	var versionID string
	if err := tx.QueryRow(ctx,
		`INSERT INTO bundle_versions (bundle_id, version, note, published_by)
		 VALUES ($1, 1, $2, 'system') RETURNING id`, id, InitialVersionNote(len(roles))).
		Scan(&versionID); err != nil {
		return "", fmt.Errorf("failed to publish initial bundle version: %w", err)
	}

	for _, role := range roles {
		if _, err := tx.Exec(ctx,
			`INSERT INTO bundle_roles (bundle_id, zitadel_project_id, zitadel_role_key)
			 VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
			id, role.ProjectID, role.RoleKey); err != nil {
			return "", fmt.Errorf("failed to add %s to the new bundle: %w", role.RoleKey, err)
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO bundle_version_roles (version_id, zitadel_project_id, zitadel_role_key)
			 VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
			versionID, role.ProjectID, role.RoleKey); err != nil {
			return "", fmt.Errorf("failed to record %s in the bundle's first version: %w", role.RoleKey, err)
		}
	}

	if owned {
		if err := tx.Commit(ctx); err != nil {
			return "", err
		}
	}
	return id, nil
}

// InitialVersionNote is what v1's history entry says.
//
// "Created empty." stays reachable: bundles created before the role
// requirement carry it, and a note records what happened rather than asserting
// what is allowed now.
func InitialVersionNote(roles int) string {
	switch roles {
	case 0:
		return "Created empty."
	case 1:
		return "Created with 1 role."
	default:
		return fmt.Sprintf("Created with %d roles.", roles)
	}
}

// UpdateBundle renames a bundle and rewrites its description.
//
// It is deliberately not a cascade and deliberately not a version. A bundle's
// name is what operators call it, not what it grants: nobody's access changes,
// no holder falls behind, and publishing a version to fix a typo would put a
// meaningless entry in a history that exists to answer "what changed for whom".
func UpdateBundle(ctx context.Context, id, name, description string) error {
	tag, err := querier(ctx).Exec(ctx,
		`UPDATE bundles SET name = $2, description = $3 WHERE id = $1 AND retired_at IS NULL`, id, name, description)
	if err != nil {
		return fmt.Errorf("update bundle (id=%s): %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// RetireBundleAndEnqueue retires a bundle AND enqueues the caller-computed revokes for everyone
// who held it, in ONE tx (mirrors RemoveBundleFromUserAndEnqueue, which is the same operation
// for one person).
//
// Retiring keeps the row, its versions and what they gave: only the assignments go, because an
// assignment is "this person is given this bundle" and a retired bundle is given to nobody. The
// transaction is still the point. An assignment that vanished without a revoke would leave the
// holder with the role in Zitadel and nothing in Syndra to explain it — drift with no actor,
// found weeks later by the sweep.
//
// params may be empty: a bundle nobody holds, or one whose every role each holder also gets
// somewhere else, retires cleanly and enqueues nothing.
//
// The welcome flag goes with it: a retired bundle is not what new members are given, and the
// partial unique index would otherwise keep the flag from moving to the bundle that is.
func RetireBundleAndEnqueue(ctx context.Context, actor, bundleID string, params []EnqueueParams) ([]string, error) {
	tx, owned, err := beginOrJoin(ctx)
	if err != nil {
		return nil, err
	}
	if owned {
		defer tx.Rollback(ctx)
	}

	tag, err := tx.Exec(ctx,
		`UPDATE bundles SET retired_at = NOW(), retired_by = $2, is_welcome = FALSE
		  WHERE id = $1 AND retired_at IS NULL`, bundleID, actor)
	if err != nil {
		return nil, fmt.Errorf("retire bundle (id=%s): %w", bundleID, err)
	}
	if tag.RowsAffected() == 0 {
		return nil, pgx.ErrNoRows
	}
	if _, err := tx.Exec(ctx, `DELETE FROM user_bundle_assignments WHERE bundle_id = $1`, bundleID); err != nil {
		return nil, fmt.Errorf("take back the retired bundle's assignments (id=%s): %w", bundleID, err)
	}
	ids, err := enqueueCascadeRows(ctx, tx,
		[]CascadeAudit{{Actor: actor, Action: "bundle.retired", ResourceID: bundleID}},
		params)
	if err != nil {
		return nil, err
	}
	if owned {
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
	}
	return ids, nil
}

const bundleListColumns = `b.id, b.name, b.description, b.is_welcome, b.confirmation_mode, b.created_at,
		       COALESCE((SELECT MAX(version) FROM bundle_versions v WHERE v.bundle_id = b.id), 0)`

// GetAllBundles lists the bundles in force. A retired one is not on it; see GetRetiredBundles.
func GetAllBundles(ctx context.Context) ([]models.Bundle, error) {
	return listBundles(ctx, `
		SELECT `+bundleListColumns+`
		FROM bundles b WHERE b.retired_at IS NULL ORDER BY b.created_at DESC;`, false)
}

// GetRetiredBundles lists the bundles that stopped being given, most recently retired first. They
// keep their history, so the screen that shows it needs them; nothing that decides access does.
func GetRetiredBundles(ctx context.Context) ([]models.Bundle, error) {
	return listBundles(ctx, `
		SELECT `+bundleListColumns+`, b.retired_at, COALESCE(b.retired_by, '')
		FROM bundles b WHERE b.retired_at IS NOT NULL ORDER BY b.retired_at DESC;`, true)
}

func listBundles(ctx context.Context, query string, retired bool) ([]models.Bundle, error) {
	rows, err := querier(ctx).Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bundles []models.Bundle
	for rows.Next() {
		var b models.Bundle
		dest := []any{&b.ID, &b.Name, &b.Description, &b.IsWelcome, &b.ConfirmationMode,
			&b.CreatedAt, &b.LatestVersion}
		if retired {
			dest = append(dest, &b.RetiredAt, &b.RetiredBy)
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		bundles = append(bundles, b)
	}
	return bundles, rows.Err()
}

// GetBundleByID fetches a single bundle by id, retired or not: the cascade orchestrator reads
// its confirmation_mode before fanning out, and a retired bundle's page reads when it was
// retired. Callers that write must refuse a retired one (RetiredAt != nil).
func GetBundleByID(ctx context.Context, id string) (models.Bundle, error) {
	var b models.Bundle
	err := querier(ctx).QueryRow(ctx,
		`SELECT id, name, description, is_welcome, confirmation_mode, created_at,
		        retired_at, COALESCE(retired_by, '')
		 FROM bundles WHERE id = $1`, id).
		Scan(&b.ID, &b.Name, &b.Description, &b.IsWelcome, &b.ConfirmationMode, &b.CreatedAt,
			&b.RetiredAt, &b.RetiredBy)
	return b, err
}

func GetRolesForBundle(ctx context.Context, bundleID string) ([]models.BundleRole, error) {
	query := `SELECT bundle_id, zitadel_project_id, zitadel_role_key FROM bundle_roles WHERE bundle_id = $1;`
	rows, err := querier(ctx).Query(ctx, query, bundleID)
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

func GetBundlesForUser(ctx context.Context, userID string) ([]models.Bundle, error) {
	query := `
		SELECT b.id, b.name, b.description, b.is_welcome, b.confirmation_mode, b.created_at,
		       bv.version,
		       COALESCE((SELECT MAX(version) FROM bundle_versions v WHERE v.bundle_id = b.id), 0)
		FROM bundles b
		JOIN user_bundle_assignments uba ON b.id = uba.bundle_id
		JOIN bundle_versions bv ON bv.id = uba.version_id
		WHERE uba.user_id = $1 AND b.retired_at IS NULL;`

	rows, err := querier(ctx).Query(ctx, query, userID)
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
	rows, err := querier(ctx).Query(ctx, q)
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
