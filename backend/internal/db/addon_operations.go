package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Statuses an addon_operations row may hold. Kept as constants beside the SQL
// that writes them, and checked against the migration's CHECK by a coherence
// guard: a status Go writes and the database refuses is a failed dispatch
// discovered as a constraint violation on an operator's screen.
const (
	// AddonOpDispatching is the pre-call state: the record is committed, the
	// add-on has not answered. Non-terminal.
	AddonOpDispatching = "dispatching"
	// The four terminal states mirror the transport's dispatch outcomes. They
	// are duplicated here rather than imported because internal/addons already
	// imports this package; the guard test is what keeps the two in step.
	AddonOpSucceeded     = "succeeded"
	AddonOpRejected      = "rejected"
	AddonOpUnreached     = "unreached"
	AddonOpIndeterminate = "indeterminate"
)

// AddonOperationStatuses is every status the column accepts, in the order the
// migration's CHECK lists them.
var AddonOperationStatuses = []string{
	AddonOpDispatching,
	AddonOpSucceeded,
	AddonOpRejected,
	AddonOpUnreached,
	AddonOpIndeterminate,
}

// AddonUnresolvedStatuses are the states in which nobody knows whether the
// target was mutated: awaiting an answer, or having lost one.
var AddonUnresolvedStatuses = []string{AddonOpDispatching, AddonOpIndeterminate}

// AddonOperationParams is everything committed before the call.
//
// There is no parameter field, and that is the point of the type. The secret
// rides the dispatch and is discarded with it; nothing that could reconstruct
// the call is written, so nothing that could replay it can be read back.
type AddonOperationParams struct {
	Target    string
	Operation string
	ActorID   string
	SubjectID string
}

// AddonOperation is one record as an operator surface reads it.
type AddonOperation struct {
	ID        string     `json:"id"`
	Target    string     `json:"target"`
	Operation string     `json:"operation"`
	ActorID   string     `json:"actor_id"`
	SubjectID string     `json:"subject_id"`
	Status    string     `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	SettledAt *time.Time `json:"settled_at,omitempty"`
}

// Unresolved reports whether nobody yet knows what this operation did.
func (o AddonOperation) Unresolved() bool {
	return o.Status == AddonOpDispatching || o.Status == AddonOpIndeterminate
}

// BeginAddonOperation commits the record and its audit row in one transaction
// and returns the record id, which is also the id sent to the add-on and
// deduplicated on.
//
// This runs BEFORE the call, and that ordering is the whole contract. A record
// written afterwards would be missing for exactly the dispatches that need it —
// the ones where the backend died mid-call — and the parameters are never
// retained, so a missing record is not recoverable from anywhere else.
func BeginAddonOperation(ctx context.Context, p AddonOperationParams) (string, error) {
	tx, err := PG.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin addon operation tx: %w", err)
	}
	defer tx.Rollback(ctx) // no-op after a successful Commit

	id, err := insertAddonOperation(ctx, tx, p)
	if err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit addon operation tx: %w", err)
	}
	return id, nil
}

// insertAddonOperation writes the record and its audit row on an existing
// transaction. Both or neither: a record with no audit row is a mutation with
// no trace, and an audit row with no record is a trace pointing at nothing.
func insertAddonOperation(ctx context.Context, tx pgx.Tx, p AddonOperationParams) (string, error) {
	const insertOp = `
		INSERT INTO addon_operations (target, operation, actor_id, subject_id, status)
		VALUES ($1, $2, $3, $4, 'dispatching')
		RETURNING id`
	var id string
	if err := tx.QueryRow(ctx, insertOp, p.Target, p.Operation, p.ActorID, p.SubjectID).Scan(&id); err != nil {
		return "", fmt.Errorf("insert addon operation: %w", err)
	}

	const insertAudit = `INSERT INTO audit_logs
		(actor_zitadel_user_id, target_zitadel_user_id, action, resource_id) VALUES ($1,$2,$3,$4)`
	action := "addon." + p.Target + "." + p.Operation + ".dispatched"
	if _, err := tx.Exec(ctx, insertAudit, p.ActorID, p.SubjectID, action, id); err != nil {
		return "", fmt.Errorf("insert addon operation audit: %w", err)
	}
	return id, nil
}

// ErrAddonOperationNotOpen means no record with this id is awaiting an outcome:
// it does not exist, or it has already settled.
var ErrAddonOperationNotOpen = fmt.Errorf("no addon operation is open under this id")

// LoadOpenAddonOperation reads back a record that is still awaiting its
// outcome. The transport calls this to verify that the dispatch it is about to
// make is described by a durable row — see addons.OperationRecord.
//
// Scoped to `dispatching` on purpose. A settled record must not authorise a
// second dispatch: the settle is what closes the window, and the window is what
// a replay would need.
func LoadOpenAddonOperation(ctx context.Context, id string) (AddonOperation, error) {
	const q = `
		SELECT id, target, operation, actor_id, subject_id, status, created_at, settled_at
		  FROM addon_operations
		 WHERE id = $1 AND status = 'dispatching'`
	var o AddonOperation
	err := PG.QueryRow(ctx, q, id).Scan(&o.ID, &o.Target, &o.Operation, &o.ActorID,
		&o.SubjectID, &o.Status, &o.CreatedAt, &o.SettledAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return AddonOperation{}, fmt.Errorf("%w: %s", ErrAddonOperationNotOpen, id)
	}
	if err != nil {
		return AddonOperation{}, fmt.Errorf("load addon operation: %w", err)
	}
	return o, nil
}

// ErrAddonOperationAlreadySettled means the row was not in `dispatching` when a
// terminal status was written.
var ErrAddonOperationAlreadySettled = fmt.Errorf("addon operation is not awaiting an outcome")

// SettleAddonOperation writes the terminal status, and only from `dispatching`.
//
// The WHERE clause is the guard: an outcome may be recorded once. Without it a
// late or duplicated settle could overwrite `indeterminate` with `succeeded`,
// which is the one direction that must never happen — it would resolve, on no
// evidence, the exact question the unresolved surface exists to raise.
func SettleAddonOperation(ctx context.Context, id, status string) error {
	const q = `
		UPDATE addon_operations
		   SET status = $2, settled_at = NOW()
		 WHERE id = $1 AND status = 'dispatching'`
	tag, err := PG.Exec(ctx, q, id, status)
	if err != nil {
		return fmt.Errorf("settle addon operation: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: %s", ErrAddonOperationAlreadySettled, id)
	}
	return nil
}

// ListUnresolvedAddonOperations returns operations nobody has an answer for,
// oldest first, excluding `dispatching` rows young enough to still be in flight.
//
// The grace period is what stops the surface flickering: a call in progress is
// indistinguishable by status alone from one whose backend died, and only time
// separates them. Callers pass something comfortably longer than the dispatch
// timeout.
func ListUnresolvedAddonOperations(ctx context.Context, grace time.Duration, limit int) ([]AddonOperation, error) {
	const q = `
		SELECT id, target, operation, actor_id, subject_id, status, created_at, settled_at
		  FROM addon_operations
		 WHERE status = 'indeterminate'
		    OR (status = 'dispatching' AND created_at < NOW() - $1::interval)
		 ORDER BY created_at ASC
		 LIMIT $2`
	rows, err := PG.Query(ctx, q, grace.String(), limit)
	if err != nil {
		return nil, fmt.Errorf("list unresolved addon operations: %w", err)
	}
	defer rows.Close()

	out := []AddonOperation{}
	for rows.Next() {
		var o AddonOperation
		if err := rows.Scan(&o.ID, &o.Target, &o.Operation, &o.ActorID, &o.SubjectID,
			&o.Status, &o.CreatedAt, &o.SettledAt); err != nil {
			return nil, fmt.Errorf("scan unresolved addon operation: %w", err)
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// AddonOperationCounts separates the three answers. Unresolved is its own
// number and belongs to neither of the others: counting it as a success claims
// something the backend does not know, and counting it as a failure tells a
// member to try again on a target that may already hold their new credential.
type AddonOperationCounts struct {
	Succeeded  int `json:"succeeded"`
	Failed     int `json:"failed"`
	Unresolved int `json:"unresolved"`
}

// CountAddonOperations summarises a target's operations. An empty target
// summarises all of them.
func CountAddonOperations(ctx context.Context, target string) (AddonOperationCounts, error) {
	const q = `
		SELECT
		  COUNT(*) FILTER (WHERE status = 'succeeded'),
		  COUNT(*) FILTER (WHERE status IN ('rejected', 'unreached')),
		  COUNT(*) FILTER (WHERE status IN ('dispatching', 'indeterminate'))
		FROM addon_operations
		WHERE $1 = '' OR target = $1`
	var c AddonOperationCounts
	if err := PG.QueryRow(ctx, q, target).Scan(&c.Succeeded, &c.Failed, &c.Unresolved); err != nil {
		return AddonOperationCounts{}, fmt.Errorf("count addon operations: %w", err)
	}
	return c, nil
}
