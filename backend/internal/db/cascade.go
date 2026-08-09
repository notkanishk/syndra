package db

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"syndra/internal/models"
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

// cascadeGroupSources are the outbox sources Change history groups into cascades.
//
// The audit stamp and the GetCascadeGroups filter MUST read this one list. They diverged once and
// the symptom was exactly what you would expect and would not think to look for: a direct grant's
// removal stamped a cascade id on its audit row, the console rendered it as a trace link, and the
// link landed on a page whose query excludes source='direct' — an empty history for a revoke that
// was really pending.
var cascadeGroupSources = []string{"bundle", "rule", "lifecycle_cascade"}

// IsCascadeGroupSource reports whether an outbox row with this source appears in Change history.
// Exported for the services package, which builds the params and must be able to assert what
// screen its writes will show up on.
func IsCascadeGroupSource(source string) bool {
	return slices.Contains(cascadeGroupSources, source)
}

// outboxSource is the source an EnqueueParams will be stored with. Empty means direct — an
// operator's own grant, which is the default because it is the only one nobody has to name.
func outboxSource(p EnqueueParams) string {
	if p.Source == "" {
		return "direct"
	}
	return p.Source
}

// cascadeGroupVisible reports whether these writes will appear in Change history, which is the
// only thing an audit row's cascade id is a handle for.
//
// A direct grant's removal is the case that matters. It goes through enqueueCascadeRows because
// its ledger delete, audit row and outbox rows must commit together — but its writes carry
// source='direct', and the surface for those is Pending changes. Stamping an id here would put a
// trace link on the audit row pointing at a screen that filters the write out.
func cascadeGroupVisible(params []EnqueueParams) bool {
	for _, p := range params {
		if IsCascadeGroupSource(outboxSource(p)) {
			return true
		}
	}
	return false
}

// CascadeAudit is one audit row a cascade writes about itself. Most cascades write exactly one
// ("a rule was changed"); MoveHoldersAndEnqueue writes one per person moved, and all of them
// belong to the same cascade.
type CascadeAudit struct {
	Actor      string
	Target     string // "-" when the event is about an object rather than a person
	Action     string
	ResourceID string
}

// enqueueCascadeRows writes the cascade's audit rows and one outbox row per param (with
// source/source_ref) on an EXISTING tx, and returns the outbox ids. It writes NO
// direct_role_grants row — cascade intent lives in the bundle/rule tables (see design pivot in
// global-constraints.md). Each outbox row still gets an idempotency key, same as the
// direct-grant enqueue path.
//
// The audit insert lives HERE rather than in each caller, and that is the whole point of C6
// (ISC-44). Every *AndEnqueue function used to write its audit row on the line immediately above
// its call to this one, which made "the audit row names the cascade it caused" a convention
// eleven functions had to remember. It is now structural: the id is minted, stamped on the audit
// rows and stamped on the outbox rows in one place, and a cascade added later cannot forget.
//
// The cascade id is stamped only when the writes will actually appear in Change history — see
// cascadeGroupVisible. It is a handle into that screen and nothing else, so a cascade that
// produced no writes, or writes that screen does not group, gets NULL. "This change reached
// nobody" is better said by an honest blank than by a dead link.
//
// A "revoke" param computed from a (user, project, role) triple (bundle/rule cascade revokes)
// never arrives with ZitadelGrantID set — unlike drift/discovery revokes, which already know the
// grant id from the triggering event or URL param. Resolve it here from the webhook-maintained
// grant index before the insert; a cache miss leaves it empty (the drain then fails just that
// row, non-fatally — see GetGrantIndexByUserProject).
func enqueueCascadeRows(ctx context.Context, tx pgx.Tx, audits []CascadeAudit, params []EnqueueParams) ([]string, error) {
	// Every access change passes through here or through enqueueWrites, which
	// is what makes this the place to take the subject lock: a caller that
	// forgot it would still be serialised against one that took it. Taken here
	// the lock does not protect THIS caller's delta — that was computed before
	// the call, and only a caller that locks before its own reads is safe — but
	// it does hold every writer back for as long as such a caller holds it,
	// which is what makes locking before the read worth anything at all.
	if err := LockAccessMutationTx(ctx, tx); err != nil {
		return nil, err
	}

	const insertOutbox = `
		INSERT INTO propagation_outbox
			(op_type, user_id, project_id, role_keys, zitadel_grant_id, payload_json,
			 idempotency_key, initiated_by, source, source_ref, cascade_id, target)
		VALUES ($1,$2,$3,$4,NULLIF($5,''),$6,$7,$8,$9,NULLIF($10,''),$11,'zitadel')
		RETURNING id`

	// One id for the whole batch: these rows are the writes ONE triggering
	// event produced, including the ones a chained rule contributed (the
	// closure diff is computed before this call, so a chain arrives here as a
	// single set). Grouping by it is what lets Pending changes say "they
	// confirm together or not at all" — a half-applied cascade is the thing
	// that creates unexplained access, and it has to be visible as one.
	cascadeID, err := newOutboxIdempotencyKey()
	if err != nil {
		return nil, err
	}

	// NULL, not the empty string: the column is UUID, and "this event caused no cascade" is
	// exactly what NULL means.
	var auditCascadeID *string
	if cascadeGroupVisible(params) {
		auditCascadeID = &cascadeID
	}
	const insertAudit = `
		INSERT INTO audit_logs
			(actor_zitadel_user_id, target_zitadel_user_id, action, resource_id, cascade_id)
		VALUES ($1,$2,$3,$4,$5)`
	for _, a := range audits {
		target := a.Target
		if target == "" {
			target = "-"
		}
		if _, err := tx.Exec(ctx, insertAudit, a.Actor, target, a.Action, a.ResourceID,
			auditCascadeID); err != nil {
			return nil, err
		}
	}

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
		var id string
		if err := tx.QueryRow(ctx, insertOutbox, p.OpType, p.UserID, p.ProjectID, p.RoleKeys,
			p.ZitadelGrantID, jsonOrEmpty(p.PayloadJSON), key, p.GrantedBy, outboxSource(p), p.SourceRef,
			cascadeID).Scan(&id); err != nil {
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

// GetAllKnownUserIDs returns every user id Syndra has any record of — the union of direct-grant
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
// It reports `assigned = false` when the person already held the bundle, in
// which case NOTHING is written: no outbox rows, no audit row, no repin.
func AssignBundleAndEnqueue(ctx context.Context, actor, userID, bundleID, versionID string, params []EnqueueParams) (ids []string, assigned bool, err error) {
	tx, owned, err := beginOrJoin(ctx)
	if err != nil {
		return nil, false, err
	}
	if owned {
		defer tx.Rollback(ctx)
	}

	// Pinned to the version the CALLER projected, not to whatever is latest at
	// this instant. Re-resolving "latest" here would race a concurrent publish:
	// the member would be pinned to v3 holding v2's roles, and the mismatch is
	// undetectable afterwards — both rows look correct on their own.
	//
	// A publish that lands mid-assignment simply leaves this member on the
	// version they were assigned, which is the same position as every other
	// holder who was not migrated.
	//
	// RETURNING is what makes the conflict visible. `ON CONFLICT DO NOTHING`
	// alone is silent, and silence here was a real defect: re-assigning a
	// bundle to somebody already on v1 left their v1 pin intact while the
	// caller's params — computed against v2 — were enqueued anyway. They got
	// v2's access and the records said v1. The conflict has to be ANSWERED, in
	// the transaction, before anything else is written; asking beforehand would
	// only move the race.
	var pinned string
	switch err := tx.QueryRow(ctx,
		`INSERT INTO user_bundle_assignments (user_id, bundle_id, version_id)
		 VALUES ($1, $2, $3) ON CONFLICT DO NOTHING
		 RETURNING version_id`, userID, bundleID, versionID).Scan(&pinned); {
	case errors.Is(err, pgx.ErrNoRows):
		// Already a holder. Re-assigning is a no-op, not a re-pin: moving
		// somebody between versions is its own rehearsed action, and doing it
		// as a side effect of an idempotent assign would move people nobody
		// planned to move.
		return nil, false, nil
	case err != nil:
		return nil, false, err
	}

	ids, err = enqueueCascadeRows(ctx, tx,
		[]CascadeAudit{{Actor: actor, Target: userID, Action: "bundle.assigned", ResourceID: bundleID}},
		params)
	if err != nil {
		return nil, false, err
	}
	if owned {
		if err := tx.Commit(ctx); err != nil {
			return nil, false, err
		}
	}
	return ids, true, nil
}

// AddRoleToBundleAndEnqueue adds a role to a bundle and enqueues the per-member cascade in one
// tx (design pivot: NO direct_role_grants write — the grant already lives in bundle_roles).
func AddRoleToBundleAndEnqueue(ctx context.Context, actor, bundleID, projectID, roleKey string, params []EnqueueParams) ([]string, error) {
	tx, owned, err := beginOrJoin(ctx)
	if err != nil {
		return nil, err
	}
	if owned {
		defer tx.Rollback(ctx)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO bundle_roles (bundle_id, zitadel_project_id, zitadel_role_key) VALUES ($1,$2,$3)
		 ON CONFLICT DO NOTHING`, bundleID, projectID, roleKey); err != nil {
		return nil, err
	}
	ids, err := enqueueCascadeRows(ctx, tx,
		[]CascadeAudit{{Actor: actor, Target: bundleID, Action: "bundle.role_added", ResourceID: projectID + "/" + roleKey}},
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

// CreateMappingRuleAndEnqueue creates a mapping rule and enqueues the caller-computed closure-diff
// params in one tx (design pivot: NO direct_role_grants write — the grant already lives in
// mapping_rules). params are built by the caller from a PRE-mutation closure simulation with
// SourceRef left empty (the rule id does not exist yet); this stamps SourceRef = the new rule id
// on every param right after the INSERT ... RETURNING, before enqueueing.
func CreateMappingRuleAndEnqueue(ctx context.Context, actor, sourceProject, sourceRole, targetProject, targetRole, mode string, params []EnqueueParams) (string, []string, error) {
	tx, owned, err := beginOrJoin(ctx)
	if err != nil {
		return "", nil, err
	}
	if owned {
		defer tx.Rollback(ctx)
	}

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

	for i := range params {
		params[i].SourceRef = ruleID
	}
	ids, err := enqueueCascadeRows(ctx, tx,
		[]CascadeAudit{{Actor: actor, Action: "mapping_rule.created", ResourceID: ruleID}},
		params)
	if err != nil {
		return "", nil, err
	}
	if owned {
		if err := tx.Commit(ctx); err != nil {
			return "", nil, err
		}
	}
	return ruleID, ids, nil
}

// RemoveBundleFromUserAndEnqueue deletes the assignment + audit + revoke outbox rows in ONE tx,
// so a committed removal always has its revoke rows (mirrors AssignBundleAndEnqueue). params may
// be empty (every role stayed covered by another source) — the assignment is still deleted; an
// empty enqueue is a no-op.
func RemoveBundleFromUserAndEnqueue(ctx context.Context, actor, userID, bundleID string, params []EnqueueParams) ([]string, error) {
	tx, owned, err := beginOrJoin(ctx)
	if err != nil {
		return nil, err
	}
	if owned {
		defer tx.Rollback(ctx)
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM user_bundle_assignments WHERE user_id=$1 AND bundle_id=$2`, userID, bundleID); err != nil {
		return nil, err
	}
	ids, err := enqueueCascadeRows(ctx, tx,
		[]CascadeAudit{{Actor: actor, Target: userID, Action: "bundle.unassigned", ResourceID: bundleID}},
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

// RemoveRoleFromBundleAndEnqueue deletes the bundle_role + audit + revoke outbox rows in one tx
// (mirrors AddRoleToBundleAndEnqueue). params may be empty (every holder stayed covered) — the
// bundle_role is still deleted.
func RemoveRoleFromBundleAndEnqueue(ctx context.Context, actor, bundleID, projectID, roleKey string, params []EnqueueParams) ([]string, error) {
	tx, owned, err := beginOrJoin(ctx)
	if err != nil {
		return nil, err
	}
	if owned {
		defer tx.Rollback(ctx)
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM bundle_roles WHERE bundle_id=$1 AND zitadel_project_id=$2 AND zitadel_role_key=$3`,
		bundleID, projectID, roleKey); err != nil {
		return nil, err
	}
	ids, err := enqueueCascadeRows(ctx, tx,
		[]CascadeAudit{{Actor: actor, Target: bundleID, Action: "bundle.role_removed", ResourceID: projectID + "/" + roleKey}},
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

// GetCascadeGroups is Change history's read: every cascade-originated write,
// grouped by the event that produced it, newest first.
//
// Deliberately NOT filtered to status='applied'. A cascade with two writes
// still waiting is exactly the entry an operator needs to see — "8 applied" and
// "2 writes waiting" are the same vocabulary, and hiding the unfinished ones is
// how a half-applied cascade goes unnoticed until it surfaces as unexplained
// access.
//
// Rows written before 000019 have no cascade_id; each falls back to its own
// outbox id so history stays complete rather than silently dropping them.
func GetCascadeGroups(ctx context.Context, limit int, cascadeID string) ([]models.CascadeGroup, error) {
	if limit <= 0 {
		limit = 50
	}
	// $1 is cascadeGroupSources throughout, in both the outer query and the subquery — the same
	// list enqueueCascadeRows consults before stamping an audit row, passed rather than spelled
	// out, so a source added to one is added to all three at once.
	const columns = `
		SELECT COALESCE(cascade_id::text, id::text) AS group_id,
		       id, op_type, user_id, project_id, role_keys, source, COALESCE(source_ref,''),
		       COALESCE(cascade_id::text,''), status, created_at, completed_at
		FROM propagation_outbox
		WHERE source = ANY($1::text[])`

	// The most recent N cascades — the glance list.
	q := columns + `
		  AND COALESCE(cascade_id::text, id::text) IN (
		      SELECT COALESCE(cascade_id::text, id::text)
		      FROM propagation_outbox
		      WHERE source = ANY($1::text[])
		      GROUP BY COALESCE(cascade_id::text, id::text)
		      ORDER BY MAX(created_at) DESC
		      LIMIT $2
		  )
		ORDER BY created_at DESC`
	args := []any{cascadeGroupSources, limit}

	// …or exactly one, named. This is what an audit row's trace link asks for, and it has to be
	// answered here rather than by filtering the glance list in the console: the audit tail is
	// walkable back to the first day, so a trace from a row older than the last 50 cascades
	// would otherwise land on a page that says nothing happened.
	if cascadeID != "" {
		q = columns + `
		  AND COALESCE(cascade_id::text, id::text) = $2
		ORDER BY created_at DESC`
		args = []any{cascadeGroupSources, cascadeID}
	}

	rows, err := PG.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("get cascade groups: %w", err)
	}
	defer rows.Close()

	byID := make(map[string]*models.CascadeGroup)
	order := make([]string, 0, limit)
	seenUser := make(map[string]map[string]bool)

	for rows.Next() {
		var groupID string
		var c models.CascadeSummary
		var createdAt time.Time
		if err := rows.Scan(&groupID, &c.ID, &c.OpType, &c.UserID, &c.ProjectID, &c.RoleKeys,
			&c.Source, &c.SourceRef, &c.CascadeID, &c.Status, &createdAt, &c.CompletedAt); err != nil {
			return nil, fmt.Errorf("scan cascade group row: %w", err)
		}

		g := byID[groupID]
		if g == nil {
			g = &models.CascadeGroup{
				CascadeID: groupID,
				Source:    c.Source,
				SourceRef: c.SourceRef,
				StartedAt: createdAt,
			}
			byID[groupID] = g
			order = append(order, groupID)
			seenUser[groupID] = make(map[string]bool)
		}
		if createdAt.Before(g.StartedAt) {
			g.StartedAt = createdAt
		}
		switch c.Status {
		case "applied":
			g.Applied++
			if c.CompletedAt != nil && (g.SettledAt == nil || c.CompletedAt.After(*g.SettledAt)) {
				g.SettledAt = c.CompletedAt
			}
		case "failed":
			g.Failed++
		default:
			g.Waiting++
		}
		if !seenUser[groupID][c.UserID] {
			seenUser[groupID][c.UserID] = true
			g.UserIDs = append(g.UserIDs, c.UserID)
		}
		g.Writes = append(g.Writes, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]models.CascadeGroup, 0, len(order))
	for _, id := range order {
		out = append(out, *byID[id])
	}
	return out, nil
}

// UpdateMappingRuleAndEnqueue updates the rule matcher/target AND enqueues the add/revoke
// cascade rows in ONE tx (mirrors CreateMappingRuleAndEnqueue). Uses the real
// source_zitadel_*/target_zitadel_* columns.
func UpdateMappingRuleAndEnqueue(ctx context.Context, actor, id, sp, sr, tp, tr string, params []EnqueueParams) ([]string, error) {
	tx, owned, err := beginOrJoin(ctx)
	if err != nil {
		return nil, err
	}
	if owned {
		defer tx.Rollback(ctx)
	}
	if _, err := tx.Exec(ctx,
		`UPDATE mapping_rules
		 SET source_zitadel_project_id=$2, source_zitadel_role_key=$3,
		     target_zitadel_project_id=$4, target_zitadel_role_key=$5
		 WHERE id=$1`, id, sp, sr, tp, tr); err != nil {
		return nil, err
	}
	ids, err := enqueueCascadeRows(ctx, tx,
		[]CascadeAudit{{Actor: actor, Action: "mapping_rule.updated", ResourceID: id}},
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

// DeleteMappingRuleAndEnqueue deletes the rule AND enqueues the caller-computed revoke rows in
// ONE tx (mirrors UpdateMappingRuleAndEnqueue). The two halves must commit together: a rule row
// removed without its revokes leaves everyone it ever granted holding access in Zitadel that no
// Syndra source explains — which is not a gap, it is drift, and the sweep would find it later
// with no actor to attribute it to.
//
// The outbox rows keep source_ref = the deleted rule's id. That column is plain text with no
// foreign key, deliberately: it records what caused the write, and the cause having since been
// deleted is exactly what a reader of the change history needs to know.
func DeleteMappingRuleAndEnqueue(ctx context.Context, actor, id string, params []EnqueueParams) ([]string, error) {
	tx, owned, err := beginOrJoin(ctx)
	if err != nil {
		return nil, err
	}
	if owned {
		defer tx.Rollback(ctx)
	}
	tag, err := tx.Exec(ctx, `DELETE FROM mapping_rules WHERE id=$1`, id)
	if err != nil {
		return nil, err
	}
	// Nobody deleted it out from under us in the window between the handler's read and here —
	// or somebody did, and enqueueing revokes for a rule a concurrent caller already removed
	// (and already revoked for) would double-write.
	if tag.RowsAffected() == 0 {
		return nil, pgx.ErrNoRows
	}
	ids, err := enqueueCascadeRows(ctx, tx,
		[]CascadeAudit{{Actor: actor, Action: "mapping_rule.deleted", ResourceID: id}},
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
