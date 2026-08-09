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
	ClaimedAt *time.Time `json:"claimed_at,omitempty"`
	SettledAt *time.Time `json:"settled_at,omitempty"`
}

// Dispatched reports whether this record was ever claimed for a call.
//
// The distinction matters on the unresolved surface. A claimed row that never
// settled may have applied to the target; an unclaimed one definitely did not,
// because nothing was sent. Both are unresolved — neither succeeded nor failed —
// but only one of them is a question about the target's state.
func (o AddonOperation) Dispatched() bool { return o.ClaimedAt != nil }

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

// ErrAddonOperationNotOpen means no unclaimed record matching this call is
// awaiting an outcome: it does not exist, it describes a different call, it has
// already been claimed, or it has already settled.
//
// The four are one error on purpose. Telling an unauthorised caller which of
// them applies is an oracle over records they have just demonstrated they should
// not be holding.
var ErrAddonOperationNotOpen = fmt.Errorf("no unclaimed addon operation matches this call")

// ClaimAddonOperation takes exclusive ownership of a record for dispatch and
// returns it, in one conditional UPDATE.
//
// This is a claim rather than a read, and that is the whole of its value. A read
// can be repeated: two callers could obtain the same record concurrently, and a
// caller could re-read a record after it settled and dispatch under it again.
// A conditional UPDATE has exactly one winner, so a record authorises exactly
// one call, once, ever.
//
// The call's identity is in the WHERE clause rather than compared afterwards.
// Compared afterwards, a mismatched attempt would still have consumed the
// record; in the predicate, a record that does not describe this call is simply
// not claimed, and the legitimate dispatch behind it is unharmed.
func ClaimAddonOperation(ctx context.Context, id, target, operation, subject string) (AddonOperation, error) {
	const q = `
		UPDATE addon_operations
		   SET claimed_at = NOW()
		 WHERE id = $1
		   AND status = 'dispatching'
		   AND claimed_at IS NULL
		   AND target = $2
		   AND operation = $3
		   AND subject_id = $4
		RETURNING id, target, operation, actor_id, subject_id, status, created_at, claimed_at, settled_at`
	var o AddonOperation
	err := PG.QueryRow(ctx, q, id, target, operation, subject).Scan(&o.ID, &o.Target, &o.Operation,
		&o.ActorID, &o.SubjectID, &o.Status, &o.CreatedAt, &o.ClaimedAt, &o.SettledAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return AddonOperation{}, fmt.Errorf("%w: %s", ErrAddonOperationNotOpen, id)
	}
	if err != nil {
		return AddonOperation{}, fmt.Errorf("claim addon operation: %w", err)
	}
	return o, nil
}

// ErrAddonOperationAlreadySettled means the row was not in `dispatching` when a
// terminal status was written.
var ErrAddonOperationAlreadySettled = fmt.Errorf("addon operation is not awaiting an outcome")

// SettleAddonOperation writes the terminal status, and only from `dispatching`,
// and only to an outcome the row's claim state can support.
//
// The status predicate is the first guard: an outcome may be recorded once.
// Without it a late or duplicated settle could overwrite `indeterminate` with
// `succeeded`, which is the one direction that must never happen — it would
// resolve, on no evidence, the exact question the unresolved surface exists to
// raise.
//
// The claim predicate is the second. A row that was never claimed was never
// sent, so `succeeded` and `rejected` would assert an answer from a target
// nobody asked, and `indeterminate` would raise a question about a call that
// never left. Only `unreached` describes an unclaimed row, and it describes it
// exactly. The database carries the same rule as a CHECK, because a predicate
// in one function is a rule for that function and a constraint is a rule for
// the table.
func SettleAddonOperation(ctx context.Context, id, status string) error {
	const q = `
		UPDATE addon_operations
		   SET status = $2, settled_at = NOW()
		 WHERE id = $1 AND status = 'dispatching'
		   AND (claimed_at IS NOT NULL OR $2 = 'unreached')`
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
// timeout. Age runs from the claim where there is one, since that is when the
// call actually started.
func ListUnresolvedAddonOperations(ctx context.Context, grace time.Duration, limit int) ([]AddonOperation, error) {
	const q = `
		SELECT id, target, operation, actor_id, subject_id, status, created_at, claimed_at, settled_at
		  FROM addon_operations
		 WHERE status = 'indeterminate'
		    OR (status = 'dispatching' AND COALESCE(claimed_at, created_at) < NOW() - $1::interval)
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
			&o.Status, &o.CreatedAt, &o.ClaimedAt, &o.SettledAt); err != nil {
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

// ErrSubjectRateLimited refuses a member driving a secret-bearing operation
// harder than any honest member would.
var ErrSubjectRateLimited = errors.New("db: too many recent operations for this subject")

// CountRecentAddonOperations counts a subject's records for one operation
// inside a window.
//
// The counter is the record table itself rather than a new store, and that is
// not only frugality: these rows already are the durable evidence that a
// secret-bearing call may have happened, so the thing being limited and the
// thing being counted cannot drift apart. A separate counter could be lost on a
// restart while the calls it was bounding stayed on the record.
//
// Only recorded attempts count. A call refused before the record — an unknown
// operation, a member naming somebody else, a parameter that failed validation
// — never reached the target and never cost it anything, so counting it would
// let one malformed client lock a member out of a path that was working.
func CountRecentAddonOperations(ctx context.Context, subjectID, operation string, window time.Duration) (int, error) {
	const q = `
		SELECT COUNT(*) FROM addon_operations
		 WHERE subject_id = $1 AND operation = $2
		   AND created_at > NOW() - ($3 || ' seconds')::interval`
	var n int
	if err := PG.QueryRow(ctx, q, subjectID, operation, int64(window/time.Second)).Scan(&n); err != nil {
		return 0, fmt.Errorf("count recent addon operations: %w", err)
	}
	return n, nil
}
