package db

import (
	"context"
	"fmt"

	"syndra/internal/models"
)

// -------------------------------------------------------------
// BUNDLE VERSIONS
// -------------------------------------------------------------
//
// Three tables, one rule: `bundle_roles` is the working copy and reaches
// nobody; `bundle_versions` are published snapshots and are what holders are
// resolved through. A "draft" is not stored — it is the difference between the
// two, computed by DraftDiff.

// LatestVersion returns a bundle's highest published version.
func LatestVersion(ctx context.Context, bundleID string) (models.BundleVersion, error) {
	var v models.BundleVersion
	err := querier(ctx).QueryRow(ctx, `
		SELECT id, bundle_id, version, note, published_by, published_at
		FROM bundle_versions WHERE bundle_id = $1
		ORDER BY version DESC LIMIT 1`, bundleID).
		Scan(&v.ID, &v.BundleID, &v.Version, &v.Note, &v.PublishedBy, &v.PublishedAt)
	return v, err
}

// ListBundleVersions returns every published version of a bundle, newest first,
// with the number of people currently pinned to each.
//
// The holder count is the point of the list. "v2 · 11 people" and "v4 · 3
// people" is the sentence an operator needs before deciding whether the older
// version is a deliberate exception or something nobody got round to.
func ListBundleVersions(ctx context.Context, bundleID string) ([]models.BundleVersion, error) {
	rows, err := querier(ctx).Query(ctx, `
		SELECT bv.id, bv.bundle_id, bv.version, bv.note, bv.published_by, bv.published_at,
		       COUNT(uba.user_id)
		FROM bundle_versions bv
		LEFT JOIN user_bundle_assignments uba ON uba.version_id = bv.id
		WHERE bv.bundle_id = $1
		GROUP BY bv.id
		ORDER BY bv.version DESC`, bundleID)
	if err != nil {
		return nil, fmt.Errorf("list bundle versions: %w", err)
	}
	defer rows.Close()

	var out []models.BundleVersion
	for rows.Next() {
		var v models.BundleVersion
		if err := rows.Scan(&v.ID, &v.BundleID, &v.Version, &v.Note,
			&v.PublishedBy, &v.PublishedAt, &v.HolderCount); err != nil {
			return nil, fmt.Errorf("scan bundle version: %w", err)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// LatestVersionRoles returns what a bundle grants TODAY — the latest published
// version AND its roles, together.
//
// The version comes back with the roles because the caller has to pin the
// assignment to the version it projected. Resolving "latest" twice — once to
// read the roles, once inside the write transaction to pin — is a race: a
// publish committing between them pins the member to v3 while the outbox
// carries v2's roles, and nothing downstream can tell.
//
// Not the working copy either. Reading `bundle_roles` would project unpublished
// edits to somebody who is not pinned to them.
func LatestVersionRoles(ctx context.Context, bundleID string) (models.BundleVersion, []models.BundleRole, error) {
	var version models.BundleVersion
	if err := querier(ctx).QueryRow(ctx, `
		SELECT id, bundle_id, version, note, published_by, published_at
		FROM bundle_versions WHERE bundle_id = $1
		ORDER BY version DESC LIMIT 1`, bundleID).
		Scan(&version.ID, &version.BundleID, &version.Version, &version.Note,
			&version.PublishedBy, &version.PublishedAt); err != nil {
		return version, nil, fmt.Errorf("latest version: %w", err)
	}

	rows, err := querier(ctx).Query(ctx, `
		SELECT bv.bundle_id, r.zitadel_project_id, r.zitadel_role_key
		FROM bundle_version_roles r
		JOIN bundle_versions bv ON bv.id = r.version_id
		WHERE r.version_id = $1
		ORDER BY r.zitadel_project_id, r.zitadel_role_key`, version.ID)
	if err != nil {
		return version, nil, fmt.Errorf("latest version roles: %w", err)
	}
	defer rows.Close()

	var out []models.BundleRole
	for rows.Next() {
		var r models.BundleRole
		if err := rows.Scan(&r.BundleID, &r.ProjectID, &r.RoleKey); err != nil {
			return version, nil, fmt.Errorf("scan latest version role: %w", err)
		}
		out = append(out, r)
	}
	return version, out, rows.Err()
}

// VersionBelongsTo reports whether a version is one of a bundle's own.
func VersionBelongsTo(ctx context.Context, bundleID, versionID string) (bool, error) {
	var ok bool
	err := querier(ctx).QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM bundle_versions WHERE id = $1 AND bundle_id = $2)`,
		versionID, bundleID).Scan(&ok)
	return ok, err
}

// GetRolesForVersion returns what one published snapshot contained.
func GetRolesForVersion(ctx context.Context, versionID string) ([]models.BundleRole, error) {
	rows, err := querier(ctx).Query(ctx, `
		SELECT bv.bundle_id, r.zitadel_project_id, r.zitadel_role_key
		FROM bundle_version_roles r
		JOIN bundle_versions bv ON bv.id = r.version_id
		WHERE r.version_id = $1
		ORDER BY r.zitadel_project_id, r.zitadel_role_key`, versionID)
	if err != nil {
		return nil, fmt.Errorf("roles for version: %w", err)
	}
	defer rows.Close()

	var out []models.BundleRole
	for rows.Next() {
		var r models.BundleRole
		if err := rows.Scan(&r.BundleID, &r.ProjectID, &r.RoleKey); err != nil {
			return nil, fmt.Errorf("scan version role: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetUserBundleRolesGrouped returns, per bundle, the roles a user actually gets
// from it — resolved through the version THEY are pinned to, not through the
// bundle's current contents.
//
// This is the whole point of versioning expressed as one query, and it replaces
// the "list their bundles, then read each bundle's roles" loop every closure
// computation used to run. That loop is what made a bundle edit reach everyone:
// it asked what the bundle holds, when the question is what this person's
// version holds.
func GetUserBundleRolesGrouped(ctx context.Context, userID string) (map[string][]models.BundleRole, error) {
	rows, err := querier(ctx).Query(ctx, `
		SELECT uba.bundle_id, r.zitadel_project_id, r.zitadel_role_key
		FROM user_bundle_assignments uba
		JOIN bundle_version_roles r ON r.version_id = uba.version_id
		WHERE uba.user_id = $1`, userID)
	if err != nil {
		return nil, fmt.Errorf("user bundle roles: %w", err)
	}
	defer rows.Close()

	out := make(map[string][]models.BundleRole)
	for rows.Next() {
		var r models.BundleRole
		if err := rows.Scan(&r.BundleID, &r.ProjectID, &r.RoleKey); err != nil {
			return nil, fmt.Errorf("scan user bundle role: %w", err)
		}
		out[r.BundleID] = append(out[r.BundleID], r)
	}
	return out, rows.Err()
}

// GetBundleHoldersByVersion lists holders of one bundle with the version each
// sits on, newest version first then user id, so the list has a stable order.
func GetBundleHoldersByVersion(ctx context.Context, bundleID string) ([]models.BundleHolder, error) {
	rows, err := querier(ctx).Query(ctx, `
		SELECT uba.user_id, bv.id, bv.version, uba.assigned_at,
		       uba.assigned_by, uba.source, uba.moved_from_version
		FROM user_bundle_assignments uba
		JOIN bundle_versions bv ON bv.id = uba.version_id
		WHERE uba.bundle_id = $1
		ORDER BY bv.version DESC, uba.user_id`, bundleID)
	if err != nil {
		return nil, fmt.Errorf("bundle holders by version: %w", err)
	}
	defer rows.Close()

	var out []models.BundleHolder
	for rows.Next() {
		var h models.BundleHolder
		if err := rows.Scan(&h.UserID, &h.VersionID, &h.Version, &h.AssignedAt,
			&h.AssignedBy, &h.Source, &h.MovedFromVersion); err != nil {
			return nil, fmt.Errorf("scan bundle holder: %w", err)
		}
		h.BundleID = bundleID
		out = append(out, h)
	}
	return out, rows.Err()
}

// GetUserBundleVersions returns, for one user, which version of each bundle
// they hold. Drives the version chip on a person's page.
func GetUserBundleVersions(ctx context.Context, userID string) (map[string]models.BundleVersion, error) {
	rows, err := querier(ctx).Query(ctx, `
		SELECT bv.bundle_id, bv.id, bv.version, bv.note, bv.published_by, bv.published_at,
		       (SELECT MAX(version) FROM bundle_versions x WHERE x.bundle_id = bv.bundle_id)
		FROM user_bundle_assignments uba
		JOIN bundle_versions bv ON bv.id = uba.version_id
		WHERE uba.user_id = $1`, userID)
	if err != nil {
		return nil, fmt.Errorf("user bundle versions: %w", err)
	}
	defer rows.Close()

	out := make(map[string]models.BundleVersion)
	for rows.Next() {
		var v models.BundleVersion
		if err := rows.Scan(&v.BundleID, &v.ID, &v.Version, &v.Note,
			&v.PublishedBy, &v.PublishedAt, &v.LatestVersion); err != nil {
			return nil, fmt.Errorf("scan user bundle version: %w", err)
		}
		out[v.BundleID] = v
	}
	return out, rows.Err()
}

// GetStaleHolderCounts returns, per bundle, how many holders sit on something
// older than the latest published version.
//
// Surfaced on the bundle list rather than left to be discovered: a bundle whose
// holders are spread across three versions is a different object from one where
// everybody is current, and the list is where that has to be visible.
func GetStaleHolderCounts(ctx context.Context) (map[string]int, error) {
	rows, err := querier(ctx).Query(ctx, `
		SELECT uba.bundle_id, COUNT(*)
		FROM user_bundle_assignments uba
		JOIN bundle_versions bv ON bv.id = uba.version_id
		WHERE bv.version < (SELECT MAX(version) FROM bundle_versions x WHERE x.bundle_id = uba.bundle_id)
		GROUP BY uba.bundle_id`)
	if err != nil {
		return nil, fmt.Errorf("stale holder counts: %w", err)
	}
	defer rows.Close()

	out := make(map[string]int)
	for rows.Next() {
		var id string
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, fmt.Errorf("scan stale holder count: %w", err)
		}
		out[id] = n
	}
	return out, rows.Err()
}

// PublishVersionAndEnqueue writes the next version from the working copy and,
// for the holders being moved, repins them and enqueues their cascade — all in
// one transaction.
//
// Atomicity matters in both directions. A committed version whose migration
// rows were lost would leave holders pinned to a version whose roles were never
// projected; a committed repin with no version would point at nothing.
//
// `moved` may be empty: that is the "leave them where they are" answer, and it
// is a valid publish, not a no-op — the new version still exists and new
// assignments will get it.
func PublishVersionAndEnqueue(
	ctx context.Context,
	actor, bundleID, note string,
	roles []models.BundleRole,
	moved []string,
	params []EnqueueParams,
) (models.BundleVersion, []string, error) {
	var out models.BundleVersion

	tx, owned, err := beginOrJoin(ctx)
	if err != nil {
		return out, nil, err
	}
	if owned {
		defer tx.Rollback(ctx)
	}

	// Next version number is computed inside the tx. Two operators publishing
	// at once would otherwise both read the same max and collide on the unique
	// index — which is the correct failure, but only one of them should see it.
	const insertVersion = `
		INSERT INTO bundle_versions (bundle_id, version, note, published_by)
		VALUES ($1, COALESCE((SELECT MAX(version) FROM bundle_versions WHERE bundle_id = $1), 0) + 1, $2, $3)
		RETURNING id, bundle_id, version, note, published_by, published_at`
	if err := tx.QueryRow(ctx, insertVersion, bundleID, note, actor).
		Scan(&out.ID, &out.BundleID, &out.Version, &out.Note, &out.PublishedBy, &out.PublishedAt); err != nil {
		return out, nil, fmt.Errorf("insert bundle version: %w", err)
	}

	// The snapshot is the caller's role set, NOT a fresh SELECT from the working
	// copy. Re-selecting here would read the working copy at a different instant
	// from the one the deltas in `params` were computed against, so a concurrent
	// edit could leave holders pinned to a version whose contents the outbox
	// never projected. Same read, or the two can disagree.
	for _, r := range roles {
		if _, err := tx.Exec(ctx, `
			INSERT INTO bundle_version_roles (version_id, zitadel_project_id, zitadel_role_key)
			VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
			out.ID, r.ProjectID, r.RoleKey); err != nil {
			return out, nil, fmt.Errorf("snapshot bundle roles: %w", err)
		}
	}

	if len(moved) > 0 {
		if _, err := tx.Exec(ctx, repinHolders, out.ID, bundleID, moved, actor); err != nil {
			return out, nil, fmt.Errorf("repin holders: %w", err)
		}
	}

	ids, err := enqueueCascadeRows(ctx, tx,
		[]CascadeAudit{{
			Actor:      actor,
			Action:     "bundle.version_published",
			ResourceID: fmt.Sprintf("%s@v%d", bundleID, out.Version),
		}},
		params)
	if err != nil {
		return out, nil, err
	}
	if owned {
		if err := tx.Commit(ctx); err != nil {
			return out, nil, err
		}
	}
	return out, ids, nil
}

// repinHolders moves holders ($3) of bundle $2 onto version $1 and records the
// move: who did it ($4) and the version number they came from. Someone already
// on $1 is left alone, so a no-op move never claims they "moved from" where
// they still are.
const repinHolders = `
	UPDATE user_bundle_assignments uba
	SET version_id = $1, source = 'moved', assigned_by = $4, moved_from_version = old.version
	FROM bundle_versions old
	WHERE old.id = uba.version_id
	  AND uba.bundle_id = $2 AND uba.user_id = ANY($3) AND uba.version_id <> $1`

// DiscardBundleDraft resets the working copy (bundle_roles) to exactly the
// latest published version's roles, and audits it, in one transaction. Nobody's
// access changes: holders resolve through their pinned version, never through
// the working copy, so there is nothing to enqueue.
func DiscardBundleDraft(ctx context.Context, actor, bundleID string) error {
	tx, owned, err := beginOrJoin(ctx)
	if err != nil {
		return err
	}
	if owned {
		defer tx.Rollback(ctx)
	}

	// Without a published version there is nothing to reset to, and the
	// DELETE below would empty the working copy instead. Every bundle is
	// created with v1, so this is a missing bundle.
	var latestID string
	if err := tx.QueryRow(ctx, `
		SELECT id FROM bundle_versions WHERE bundle_id = $1
		ORDER BY version DESC LIMIT 1`, bundleID).Scan(&latestID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM bundle_roles WHERE bundle_id = $1`, bundleID); err != nil {
		return fmt.Errorf("clear working copy: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO bundle_roles (bundle_id, zitadel_project_id, zitadel_role_key)
		SELECT $1, zitadel_project_id, zitadel_role_key
		FROM bundle_version_roles WHERE version_id = $2`, bundleID, latestID); err != nil {
		return fmt.Errorf("restore working copy: %w", err)
	}
	// The audit row through the one cascade writer, with no params: no outbox
	// rows, no cascade id — this reached nobody.
	if _, err := enqueueCascadeRows(ctx, tx, []CascadeAudit{{
		Actor: actor, Target: bundleID, Action: "bundle.draft_discarded", ResourceID: latestID,
	}}, nil); err != nil {
		return fmt.Errorf("audit discard: %w", err)
	}
	if owned {
		return tx.Commit(ctx)
	}
	return nil
}

// MoveHoldersAndEnqueue repins a set of holders onto a version and enqueues
// their cascade in one tx. Used both for "catch these people up" after the fact
// and for putting somebody deliberately back onto an older version.
func MoveHoldersAndEnqueue(
	ctx context.Context,
	actor, bundleID, versionID string,
	userIDs []string,
	params []EnqueueParams,
) ([]string, error) {
	tx, owned, err := beginOrJoin(ctx)
	if err != nil {
		return nil, err
	}
	if owned {
		defer tx.Rollback(ctx)
	}

	// The version must belong to this bundle. Repinning somebody onto another
	// bundle's version would resolve their access through roles that bundle
	// never contained, and nothing downstream would notice.
	var owns bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM bundle_versions WHERE id = $1 AND bundle_id = $2)`,
		versionID, bundleID).Scan(&owns); err != nil {
		return nil, err
	}
	if !owns {
		return nil, fmt.Errorf("version %s does not belong to bundle %s", versionID, bundleID)
	}

	if _, err := tx.Exec(ctx, repinHolders, versionID, bundleID, userIDs, actor); err != nil {
		return nil, fmt.Errorf("repin holders: %w", err)
	}

	// One audit row per person moved, all of them naming the same cascade — moving eight people
	// onto a version is eight things that happened to eight people, and one thing that happened.
	audits := make([]CascadeAudit, 0, len(userIDs))
	for _, u := range userIDs {
		audits = append(audits, CascadeAudit{
			Actor: actor, Target: u, Action: "bundle.holder_moved", ResourceID: versionID,
		})
	}

	ids, err := enqueueCascadeRows(ctx, tx, audits, params)
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

// BundleDerivedGrant is one (person, project, role) a bundle assignment
// accounts for, resolved through the version that person is PINNED to.
type BundleDerivedGrant struct {
	UserID    string
	ProjectID string
	RoleKey   string
}

// GetAllBundleDerivedGrants returns every (person, project, role) that a bundle
// assignment explains, across the whole deployment, in one read.
//
// This exists for the drift sweep, and the reason is worth stating. The sweep
// classifies a live Zitadel grant as drift when Syndra has no intent behind it,
// and it was checking three sources: direct grants, mapping rules, exclusions.
// Bundles were not among them — so every role a bundle projected into Zitadel
// was reported as unexplained drift, permanently, and re-affirmed on every
// tick. The webhook's own drift check never had this hole: it goes through
// services.UserExpectsRole, which is documented as "direct grant, bundle, or
// mapping rule". The two detectors disagreed about what Syndra expects.
//
// Resolved through `a.version_id` — the holder's pin — and NOT through
// `bundle_roles`. The working copy is what the next version will contain;
// reading it here would explain away a live grant on the strength of an
// unpublished edit, which is the same confusion that put an operator's
// unpublished roles into an assignment preview.
//
// One query rather than per-user, because the sweep runs over every grant in
// the deployment and N round trips inside that loop is how a five-second sweep
// becomes a five-minute one.
func GetAllBundleDerivedGrants(ctx context.Context) ([]BundleDerivedGrant, error) {
	rows, err := querier(ctx).Query(ctx, `
		SELECT a.user_id, r.zitadel_project_id, r.zitadel_role_key
		FROM user_bundle_assignments a
		JOIN bundle_version_roles r ON r.version_id = a.version_id
		ORDER BY a.user_id, r.zitadel_project_id, r.zitadel_role_key`)
	if err != nil {
		return nil, fmt.Errorf("all bundle-derived grants: %w", err)
	}
	defer rows.Close()

	var out []BundleDerivedGrant
	for rows.Next() {
		var g BundleDerivedGrant
		if err := rows.Scan(&g.UserID, &g.ProjectID, &g.RoleKey); err != nil {
			return nil, fmt.Errorf("scan bundle-derived grant: %w", err)
		}
		out = append(out, g)
	}
	// Surfaced rather than swallowed: a partly-read inventory would explain
	// fewer grants than Syndra actually intends, and the sweep would raise the
	// remainder as drift. Same rule GetAllDirectGrants follows.
	return out, rows.Err()
}
