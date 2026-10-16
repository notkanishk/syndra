package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"syndra/internal/models"
)

// Included bundles (reconcile B8). A bundle can include another bundle's
// pinned version, one level deep: a bundle that includes others can't be
// included, and a bundle that is included can't include others. Includes live
// in the working copy like roles and are snapshotted on publish; what a
// version gives is read through bundle_version_effective_roles.

var (
	// ErrBundleNesting refuses an include that would go two levels deep.
	ErrBundleNesting = errors.New("bundles go one level deep")
	// ErrIncludeSelf refuses a bundle including itself.
	ErrIncludeSelf = errors.New("a bundle can't include itself")
	// ErrBundleIncluded refuses retiring a bundle that other bundles include.
	ErrBundleIncluded = errors.New("other bundles include this bundle")
)

// NestingError names the bundle that makes an include two levels deep.
type NestingError struct {
	// Bundle is the bundle in the way, and Includes says which way: true when
	// it includes others (so it can't be included), false when it is included
	// by Other (so it can't include).
	Bundle   string
	Includes bool
	Other    string
}

func (e *NestingError) Error() string {
	if e.Includes {
		return fmt.Sprintf("%s includes other bundles, so it can't go inside another", e.Bundle)
	}
	return fmt.Sprintf("%s is inside %s, so it can't include other bundles", e.Bundle, e.Other)
}

func (e *NestingError) Unwrap() error { return ErrBundleNesting }

const includeColumns = `
	i.child_bundle_id::text, b.name, i.child_version_id::text, cv.version,
	(SELECT MAX(version) FROM bundle_versions WHERE bundle_id = i.child_bundle_id)`

func scanIncludes(rows pgx.Rows) ([]models.BundleInclude, error) {
	defer rows.Close()
	out := []models.BundleInclude{}
	for rows.Next() {
		var in models.BundleInclude
		if err := rows.Scan(&in.ChildBundleID, &in.ChildName, &in.ChildVersionID, &in.ChildVersion, &in.ChildLatest); err != nil {
			return nil, fmt.Errorf("scan include: %w", err)
		}
		out = append(out, in)
	}
	return out, rows.Err()
}

// GetWorkingIncludes is what a bundle's working copy includes.
func GetWorkingIncludes(ctx context.Context, bundleID string) ([]models.BundleInclude, error) {
	rows, err := querier(ctx).Query(ctx, `SELECT `+includeColumns+`
		FROM bundle_includes i
		JOIN bundles b ON b.id = i.child_bundle_id
		JOIN bundle_versions cv ON cv.id = i.child_version_id
		WHERE i.parent_bundle_id = $1
		ORDER BY b.name`, bundleID)
	if err != nil {
		return nil, fmt.Errorf("working includes: %w", err)
	}
	return scanIncludes(rows)
}

// GetVersionIncludes is what a published version included.
func GetVersionIncludes(ctx context.Context, versionID string) ([]models.BundleInclude, error) {
	rows, err := querier(ctx).Query(ctx, `SELECT `+includeColumns+`
		FROM bundle_version_includes i
		JOIN bundles b ON b.id = i.child_bundle_id
		JOIN bundle_versions cv ON cv.id = i.child_version_id
		WHERE i.version_id = $1
		ORDER BY b.name`, versionID)
	if err != nil {
		return nil, fmt.Errorf("version includes: %w", err)
	}
	return scanIncludes(rows)
}

// includesAnything: the bundle includes others in its working copy or in any
// version anybody holds or will be given (the latest).
const includesAnything = `
	SELECT EXISTS (SELECT 1 FROM bundle_includes WHERE parent_bundle_id = $1)
	    OR EXISTS (SELECT 1 FROM bundle_version_includes i
	                 JOIN bundle_versions v ON v.id = i.version_id
	                WHERE v.bundle_id = $1
	                  AND (v.version = (SELECT MAX(version) FROM bundle_versions WHERE bundle_id = $1)
	                       OR EXISTS (SELECT 1 FROM user_bundle_assignments a WHERE a.version_id = v.id)))`

// includerOf names one bundle that includes this one in its working copy, its
// latest version, or a version somebody holds; "" when none does.
const includerOf = `
	SELECT b.name FROM bundles b
	 WHERE b.id <> $1 AND (
	       EXISTS (SELECT 1 FROM bundle_includes WHERE parent_bundle_id = b.id AND child_bundle_id = $1)
	    OR EXISTS (SELECT 1 FROM bundle_version_includes i
	                 JOIN bundle_versions v ON v.id = i.version_id
	                WHERE v.bundle_id = b.id AND i.child_bundle_id = $1
	                  AND (v.version = (SELECT MAX(version) FROM bundle_versions WHERE bundle_id = b.id)
	                       OR EXISTS (SELECT 1 FROM user_bundle_assignments a WHERE a.version_id = v.id))))
	 ORDER BY b.name`

// BundleIncludesAnything reports whether a bundle includes others, in its
// working copy, its latest version or a version somebody holds.
func BundleIncludesAnything(ctx context.Context, bundleID string) (bool, error) {
	var has bool
	err := querier(ctx).QueryRow(ctx, includesAnything, bundleID).Scan(&has)
	return has, err
}

func bundleName(ctx context.Context, tx pgx.Tx, id string) (string, error) {
	var name string
	err := tx.QueryRow(ctx, `SELECT name FROM bundles WHERE id = $1`, id).Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", pgx.ErrNoRows
	}
	return name, err
}

// SetWorkingInclude puts child, pinned to childVersion (0: its latest), into
// parent's working copy, or moves an existing include to that version. Nobody's
// access changes until parent publishes. Checked under the access lock, so two
// operators can't build two levels between them.
func SetWorkingInclude(ctx context.Context, actor, parentID, childID string, childVersion int) (models.BundleInclude, error) {
	var out models.BundleInclude
	if parentID == childID {
		return out, ErrIncludeSelf
	}
	tx, owned, err := beginOrJoin(ctx)
	if err != nil {
		return out, err
	}
	if owned {
		defer tx.Rollback(ctx)
	}
	if err := LockAccessMutationTx(ctx, tx); err != nil {
		return out, err
	}

	parentName, err := bundleName(ctx, tx, parentID)
	if err != nil {
		return out, err
	}
	childName, err := bundleName(ctx, tx, childID)
	if err != nil {
		return out, err
	}
	var childIncludes bool
	if err := tx.QueryRow(ctx, includesAnything, childID).Scan(&childIncludes); err != nil {
		return out, fmt.Errorf("does the child include others: %w", err)
	}
	if childIncludes {
		return out, &NestingError{Bundle: childName, Includes: true}
	}
	var parentIncluder string
	err = tx.QueryRow(ctx, includerOf, parentID).Scan(&parentIncluder)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return out, fmt.Errorf("is the parent included: %w", err)
	}
	if parentIncluder != "" {
		return out, &NestingError{Bundle: parentName, Other: parentIncluder}
	}

	var versionID string
	q := `SELECT id FROM bundle_versions WHERE bundle_id = $1 AND version = $2`
	args := []any{childID, childVersion}
	if childVersion <= 0 {
		q, args = `SELECT id FROM bundle_versions WHERE bundle_id = $1 ORDER BY version DESC LIMIT 1`, []any{childID}
	}
	if err := tx.QueryRow(ctx, q, args...).Scan(&versionID); errors.Is(err, pgx.ErrNoRows) {
		return out, pgx.ErrNoRows
	} else if err != nil {
		return out, fmt.Errorf("child version: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO bundle_includes (parent_bundle_id, child_bundle_id, child_version_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (parent_bundle_id, child_bundle_id) DO UPDATE SET child_version_id = EXCLUDED.child_version_id`,
		parentID, childID, versionID); err != nil {
		return out, fmt.Errorf("set include: %w", err)
	}
	if _, err := enqueueCascadeRows(ctx, tx, []CascadeAudit{{
		Actor: actor, Target: parentID, Action: "bundle.include_set", ResourceID: childID,
	}}, nil); err != nil {
		return out, fmt.Errorf("audit include: %w", err)
	}
	ins, err := GetWorkingIncludes(context.WithValue(ctx, txKey, tx), parentID)
	if err != nil {
		return out, err
	}
	if owned {
		if err := tx.Commit(ctx); err != nil {
			return out, err
		}
	}
	for _, in := range ins {
		if in.ChildBundleID == childID {
			return in, nil
		}
	}
	return out, nil
}

// IncludeInFirstVersion includes child (its latest version) in a bundle being
// created, in both its working copy and its v1, so it can be given straight
// away with what it includes.
func IncludeInFirstVersion(ctx context.Context, actor, bundleID, childID string) error {
	tx, owned, err := beginOrJoin(ctx)
	if err != nil {
		return err
	}
	if owned {
		defer tx.Rollback(ctx)
	}
	in, err := SetWorkingInclude(context.WithValue(ctx, txKey, tx), actor, bundleID, childID, 0)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO bundle_version_includes (version_id, child_bundle_id, child_version_id)
		SELECT v.id, $2, $3 FROM bundle_versions v WHERE v.bundle_id = $1 AND v.version = 1`,
		bundleID, childID, in.ChildVersionID); err != nil {
		return fmt.Errorf("include in v1: %w", err)
	}
	if owned {
		return tx.Commit(ctx)
	}
	return nil
}

// RemoveWorkingInclude takes child out of parent's working copy. Nobody's
// access changes until parent publishes.
func RemoveWorkingInclude(ctx context.Context, actor, parentID, childID string) (bool, error) {
	tx, owned, err := beginOrJoin(ctx)
	if err != nil {
		return false, err
	}
	if owned {
		defer tx.Rollback(ctx)
	}
	tag, err := tx.Exec(ctx, `DELETE FROM bundle_includes WHERE parent_bundle_id = $1 AND child_bundle_id = $2`, parentID, childID)
	if err != nil {
		return false, fmt.Errorf("remove include: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	if _, err := enqueueCascadeRows(ctx, tx, []CascadeAudit{{
		Actor: actor, Target: parentID, Action: "bundle.include_removed", ResourceID: childID,
	}}, nil); err != nil {
		return false, fmt.Errorf("audit include removal: %w", err)
	}
	if owned {
		return true, tx.Commit(ctx)
	}
	return true, nil
}

// SnapshotVersionIncludes writes what a new version includes. Called inside
// the publish's transaction, so the version and its includes land together.
func SnapshotVersionIncludes(ctx context.Context, versionID string, includes []models.BundleInclude) error {
	tx, owned, err := beginOrJoin(ctx)
	if err != nil {
		return err
	}
	if owned {
		defer tx.Rollback(ctx)
	}
	for _, in := range includes {
		if _, err := tx.Exec(ctx, `
			INSERT INTO bundle_version_includes (version_id, child_bundle_id, child_version_id)
			VALUES ($1, $2, $3)`, versionID, in.ChildBundleID, in.ChildVersionID); err != nil {
			return fmt.Errorf("snapshot include: %w", err)
		}
	}
	if owned {
		return tx.Commit(ctx)
	}
	return nil
}

// Includer is a bundle that includes another, for the child's page.
type Includer struct {
	BundleID string `json:"bundle_id"`
	Name     string `json:"name"`
	// IncludedVersion is the child version the parent's latest version gives;
	// 0 when only its working copy includes the child so far.
	IncludedVersion int `json:"included_version"`
	// DraftVersion is the child version its working copy has; it differs from
	// IncludedVersion when an update is waiting to be published.
	DraftVersion int `json:"draft_version,omitempty"`
	// People is how many hold a version of the parent that includes the child.
	People int `json:"people"`
}

// ListIncluders lists every bundle that includes childID, with what it gives
// and to how many.
func ListIncluders(ctx context.Context, childID string) ([]Includer, error) {
	rows, err := querier(ctx).Query(ctx, `
		WITH latest AS (
			SELECT DISTINCT ON (bundle_id) bundle_id, id FROM bundle_versions ORDER BY bundle_id, version DESC
		)
		SELECT b.id::text, b.name,
		       COALESCE((SELECT cv.version FROM latest l
		                   JOIN bundle_version_includes i ON i.version_id = l.id AND i.child_bundle_id = $1
		                   JOIN bundle_versions cv ON cv.id = i.child_version_id
		                  WHERE l.bundle_id = b.id), 0),
		       COALESCE((SELECT cv.version FROM bundle_includes w
		                   JOIN bundle_versions cv ON cv.id = w.child_version_id
		                  WHERE w.parent_bundle_id = b.id AND w.child_bundle_id = $1), 0),
		       (SELECT COUNT(*) FROM user_bundle_assignments a
		          JOIN bundle_version_includes i ON i.version_id = a.version_id AND i.child_bundle_id = $1
		         WHERE a.bundle_id = b.id)
		  FROM bundles b
		 WHERE EXISTS (SELECT 1 FROM bundle_includes w WHERE w.parent_bundle_id = b.id AND w.child_bundle_id = $1)
		    OR EXISTS (SELECT 1 FROM bundle_version_includes i JOIN bundle_versions v ON v.id = i.version_id
		                WHERE v.bundle_id = b.id AND i.child_bundle_id = $1)
		 ORDER BY b.name`, childID)
	if err != nil {
		return nil, fmt.Errorf("includers: %w", err)
	}
	defer rows.Close()
	out := []Includer{}
	for rows.Next() {
		var in Includer
		if err := rows.Scan(&in.BundleID, &in.Name, &in.IncludedVersion, &in.DraftVersion, &in.People); err != nil {
			return nil, fmt.Errorf("scan includer: %w", err)
		}
		if in.DraftVersion == in.IncludedVersion {
			in.DraftVersion = 0
		}
		out = append(out, in)
	}
	return out, rows.Err()
}

// RetireBlockers names the bundles whose working copy, latest version, or a
// version somebody holds includes childID. Retiring is refused while any do;
// includes in older versions nobody holds are history and are let go with it.
func RetireBlockers(ctx context.Context, childID string) ([]string, error) {
	rows, err := querier(ctx).Query(ctx, includerOf, childID)
	if err != nil {
		return nil, fmt.Errorf("retire blockers: %w", err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}
