package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"mkauth/internal/models"
)

// IsUniqueViolation reports whether err is a Postgres unique-constraint violation
// (SQLSTATE 23505) — e.g. a duplicate mapping rule colliding with
// idx_mapping_rules_logic. Handlers use this to turn the raw db error from
// CreateMappingRuleAndEnqueue/UpdateMappingRuleAndEnqueue into a 409 instead of
// a generic 500.
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// enqueueCascadeRows inserts one outbox row per param (with source/source_ref) on an EXISTING
// tx and returns the outbox ids. It writes NO direct_role_grants row — cascade intent lives in
// the bundle/rule tables (see design pivot in global-constraints.md). Each row still gets an
// idempotency key, same as the direct-grant enqueue path.
//
// A "revoke" param computed from a (user, project, role) triple (bundle/rule cascade revokes)
// never arrives with ZitadelGrantID set — unlike drift/discovery revokes, which already know the
// grant id from the triggering event or URL param. Resolve it here from the webhook-maintained
// grant index before the insert; a cache miss leaves it empty (the drain then fails just that
// row, non-fatally — see GetGrantIndexByUserProject).
func enqueueCascadeRows(ctx context.Context, tx pgx.Tx, params []EnqueueParams) ([]string, error) {
	const insertOutbox = `
		INSERT INTO pending_zitadel_propagations
			(op_type, user_id, project_id, role_keys, zitadel_grant_id, payload_json,
			 idempotency_key, initiated_by, source, source_ref)
		VALUES ($1,$2,$3,$4,NULLIF($5,''),$6,$7,$8,$9,NULLIF($10,''))
		RETURNING id`
	ids := make([]string, 0, len(params))
	for _, p := range params {
		if p.OpType == "revoke" && p.ZitadelGrantID == "" {
			if idx, err := GetGrantIndexByUserProject(ctx, p.UserID, p.ProjectID); err == nil {
				p.ZitadelGrantID = idx.GrantID
			} else if !errors.Is(err, ErrGrantIndexNotFound) {
				return nil, err
			}
		}
		key, err := newOutboxIdempotencyKey()
		if err != nil {
			return nil, err
		}
		src := p.Source
		if src == "" {
			src = "direct"
		}
		var id string
		if err := tx.QueryRow(ctx, insertOutbox, p.OpType, p.UserID, p.ProjectID, p.RoleKeys,
			p.ZitadelGrantID, jsonOrEmpty(p.PayloadJSON), key, p.GrantedBy, src, p.SourceRef).Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// jsonOrEmpty defaults an empty payload to "{}" — payload_json is JSONB NOT NULL.
func jsonOrEmpty(s string) string {
	if s == "" {
		return "{}"
	}
	return s
}

// GetUsersForBundle lists the user ids currently assigned to a bundle.
func GetUsersForBundle(ctx context.Context, bundleID string) ([]string, error) {
	return scanUserIDs(ctx, `SELECT user_id FROM user_bundle_assignments WHERE bundle_id = $1`, bundleID)
}

// GetAllKnownUserIDs returns every user id MkAuth has any record of — the union of direct-grant
// holders, bundle-assignment holders, and Zitadel-grant-index holders. Rule create/update cascades
// use this (not a single index) to discover which users' effective closures might change, since a
// rule's source role can be held via any of the three tables.
// ponytail: unindexed UNION scan across three tables on every rule create/update — fine at the
// documented ~200-user scale; add a materialized users table/index if that bound is ever crossed.
func GetAllKnownUserIDs(ctx context.Context) ([]string, error) {
	return scanUserIDs(ctx, `
		SELECT user_id FROM direct_role_grants
		UNION
		SELECT user_id FROM user_bundle_assignments
		UNION
		SELECT user_id FROM zitadel_grants_index`)
}

func scanUserIDs(ctx context.Context, q string, args ...any) ([]string, error) {
	rows, err := PG.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []string
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// AssignBundleAndEnqueue assigns a bundle to a user and enqueues its cascade outbox rows in
// ONE tx, so a committed assignment always has its projection rows (design pivot: NO
// direct_role_grants write — bundle membership already lives in user_bundle_assignments).
func AssignBundleAndEnqueue(ctx context.Context, actor, userID, bundleID string, params []EnqueueParams) ([]string, error) {
	tx, err := PG.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx,
		`INSERT INTO user_bundle_assignments (user_id, bundle_id) VALUES ($1,$2)
		 ON CONFLICT DO NOTHING`, userID, bundleID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO audit_logs (actor_zitadel_user_id, target_zitadel_user_id, action, resource_id)
		 VALUES ($1,$2,'bundle.assigned',$3)`, actor, userID, bundleID); err != nil {
		return nil, err
	}
	ids, err := enqueueCascadeRows(ctx, tx, params)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return ids, nil
}

// AddRoleToBundleAndEnqueue adds a role to a bundle and enqueues the per-member cascade in one
// tx (design pivot: NO direct_role_grants write — the grant already lives in bundle_roles).
func AddRoleToBundleAndEnqueue(ctx context.Context, actor, bundleID, projectID, roleKey string, params []EnqueueParams) ([]string, error) {
	tx, err := PG.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx,
		`INSERT INTO bundle_roles (bundle_id, zitadel_project_id, zitadel_role_key) VALUES ($1,$2,$3)
		 ON CONFLICT DO NOTHING`, bundleID, projectID, roleKey); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO audit_logs (actor_zitadel_user_id, target_zitadel_user_id, action, resource_id)
		 VALUES ($1,$2,'bundle.role_added',$3)`, actor, bundleID, projectID+"/"+roleKey); err != nil {
		return nil, err
	}
	ids, err := enqueueCascadeRows(ctx, tx, params)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return ids, nil
}

// CreateMappingRuleAndEnqueue creates a mapping rule and enqueues the caller-computed closure-diff
// params in one tx (design pivot: NO direct_role_grants write — the grant already lives in
// mapping_rules). params are built by the caller from a PRE-mutation closure simulation with
// SourceRef left empty (the rule id does not exist yet); this stamps SourceRef = the new rule id
// on every param right after the INSERT ... RETURNING, before enqueueing.
func CreateMappingRuleAndEnqueue(ctx context.Context, actor, sourceProject, sourceRole, targetProject, targetRole, mode string, params []EnqueueParams) (string, []string, error) {
	tx, err := PG.Begin(ctx)
	if err != nil {
		return "", nil, err
	}
	defer tx.Rollback(ctx)

	const insertRule = `
		INSERT INTO mapping_rules
			(source_zitadel_project_id, source_zitadel_role_key, target_zitadel_project_id, target_zitadel_role_key, confirmation_mode)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING id`
	var ruleID string
	if err := tx.QueryRow(ctx, insertRule, sourceProject, sourceRole, targetProject, targetRole,
		NormalizeConfirmationMode(mode)).Scan(&ruleID); err != nil {
		return "", nil, err
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO audit_logs (actor_zitadel_user_id, target_zitadel_user_id, action, resource_id)
		 VALUES ($1,$2,'mapping_rule.created',$3)`, actor, "-", ruleID); err != nil {
		return "", nil, err
	}

	for i := range params {
		params[i].SourceRef = ruleID
	}
	ids, err := enqueueCascadeRows(ctx, tx, params)
	if err != nil {
		return "", nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", nil, err
	}
	return ruleID, ids, nil
}

// RemoveBundleFromUserAndEnqueue deletes the assignment + audit + revoke outbox rows in ONE tx,
// so a committed removal always has its revoke rows (mirrors AssignBundleAndEnqueue). params may
// be empty (every role stayed covered by another source) — the assignment is still deleted; an
// empty enqueue is a no-op.
func RemoveBundleFromUserAndEnqueue(ctx context.Context, actor, userID, bundleID string, params []EnqueueParams) ([]string, error) {
	tx, err := PG.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx,
		`DELETE FROM user_bundle_assignments WHERE user_id=$1 AND bundle_id=$2`, userID, bundleID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO audit_logs (actor_zitadel_user_id, target_zitadel_user_id, action, resource_id)
		 VALUES ($1,$2,'bundle.unassigned',$3)`, actor, userID, bundleID); err != nil {
		return nil, err
	}
	ids, err := enqueueCascadeRows(ctx, tx, params)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return ids, nil
}

// RemoveRoleFromBundleAndEnqueue deletes the bundle_role + audit + revoke outbox rows in one tx
// (mirrors AddRoleToBundleAndEnqueue). params may be empty (every holder stayed covered) — the
// bundle_role is still deleted.
func RemoveRoleFromBundleAndEnqueue(ctx context.Context, actor, bundleID, projectID, roleKey string, params []EnqueueParams) ([]string, error) {
	tx, err := PG.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx,
		`DELETE FROM bundle_roles WHERE bundle_id=$1 AND zitadel_project_id=$2 AND zitadel_role_key=$3`,
		bundleID, projectID, roleKey); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO audit_logs (actor_zitadel_user_id, target_zitadel_user_id, action, resource_id)
		 VALUES ($1,$2,'bundle.role_removed',$3)`, actor, bundleID, projectID+"/"+roleKey); err != nil {
		return nil, err
	}
	ids, err := enqueueCascadeRows(ctx, tx, params)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return ids, nil
}

// SetRuleConfirmationMode bulk-updates the confirmation_mode of the given mapping rules in one
// statement (one implicit tx). mode is normalized so an invalid literal can never persist.
func SetRuleConfirmationMode(ctx context.Context, ids []string, mode string) error {
	_, err := PG.Exec(ctx,
		`UPDATE mapping_rules SET confirmation_mode = $1 WHERE id = ANY($2)`,
		NormalizeConfirmationMode(mode), ids)
	return err
}

// SetBundleConfirmationMode bulk-updates the confirmation_mode of the given bundles in one
// statement (one implicit tx). Same shape as SetRuleConfirmationMode.
func SetBundleConfirmationMode(ctx context.Context, ids []string, mode string) error {
	_, err := PG.Exec(ctx,
		`UPDATE bundles SET confirmation_mode = $1 WHERE id = ANY($2)`,
		NormalizeConfirmationMode(mode), ids)
	return err
}

// GetRecentCascades returns the most recently applied cascade-originated outbox rows (source ∈
// {bundle, rule, lifecycle_cascade}), newest first. This is a superset of "automated" — the
// outbox does not persist whether a row drained automatically or via an operator's "Resume now"
// (the confirmation decision isn't recorded per row) — so this surfaces every cascade projection
// that reached Zitadel, which is the right thing to show an operator (never invisible).
func GetRecentCascades(ctx context.Context, limit int) ([]models.CascadeSummary, error) {
	const q = `
		SELECT id, op_type, user_id, project_id, role_keys, source, COALESCE(source_ref,''), status, completed_at
		FROM pending_zitadel_propagations
		WHERE source IN ('bundle','rule','lifecycle_cascade') AND status = 'applied'
		ORDER BY completed_at DESC NULLS LAST LIMIT $1`
	rows, err := PG.Query(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("get recent cascades: %w", err)
	}
	defer rows.Close()

	var out []models.CascadeSummary
	for rows.Next() {
		var c models.CascadeSummary
		if err := rows.Scan(&c.ID, &c.OpType, &c.UserID, &c.ProjectID, &c.RoleKeys,
			&c.Source, &c.SourceRef, &c.Status, &c.CompletedAt); err != nil {
			return nil, fmt.Errorf("scan cascade summary: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// UpdateMappingRuleAndEnqueue updates the rule matcher/target AND enqueues the add/revoke
// cascade rows in ONE tx (mirrors CreateMappingRuleAndEnqueue). Uses the real
// source_zitadel_*/target_zitadel_* columns.
func UpdateMappingRuleAndEnqueue(ctx context.Context, actor, id, sp, sr, tp, tr string, params []EnqueueParams) ([]string, error) {
	tx, err := PG.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx,
		`UPDATE mapping_rules
		 SET source_zitadel_project_id=$2, source_zitadel_role_key=$3,
		     target_zitadel_project_id=$4, target_zitadel_role_key=$5
		 WHERE id=$1`, id, sp, sr, tp, tr); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO audit_logs (actor_zitadel_user_id, target_zitadel_user_id, action, resource_id)
		 VALUES ($1,'-','mapping_rule.updated',$2)`, actor, id); err != nil {
		return nil, err
	}
	ids, err := enqueueCascadeRows(ctx, tx, params)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return ids, nil
}
