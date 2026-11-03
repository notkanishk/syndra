package db

import (
	"context"
	"encoding/json"
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
	// About is the non-secret thing the change was about (an account name, a
	// setting's value). The caller decides which operations may carry one;
	// password operations never do.
	About string
	// RetryOf names the first send when this is the page's one automatic
	// resend of the same value. The insert checks it describes the same call.
	RetryOf string
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
	About     string     `json:"about,omitempty"`
}

// SentAt is when the call left: the claim, or the record when it never was.
func (o AddonOperation) SentAt() time.Time {
	if o.ClaimedAt != nil {
		return *o.ClaimedAt
	}
	return o.CreatedAt
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
	var id string
	var details []byte
	if p.RetryOf == "" {
		const insertOp = `
			INSERT INTO addon_operations (target, operation, actor_id, subject_id, status, about)
			VALUES ($1, $2, $3, $4, 'dispatching', NULLIF($5, ''))
			RETURNING id`
		if err := tx.QueryRow(ctx, insertOp, p.Target, p.Operation, p.ActorID, p.SubjectID, p.About).Scan(&id); err != nil {
			return "", fmt.Errorf("insert addon operation: %w", err)
		}
	} else {
		// The resend inherits nothing from the request but the link, and the
		// link is checked here: it must name a FIRST send of this same call.
		// The unique index on retry_of is what makes it once.
		const insertRetry = `
			INSERT INTO addon_operations (target, operation, actor_id, subject_id, status, attempt, retry_of)
			SELECT $1, $2, $3, $4, 'dispatching', 2, first.id
			  FROM addon_operations first
			 WHERE first.id = $5::uuid AND first.attempt = 1
			   AND first.target = $1 AND first.operation = $2 AND first.subject_id = $4
			RETURNING id`
		err := tx.QueryRow(ctx, insertRetry, p.Target, p.Operation, p.ActorID, p.SubjectID, p.RetryOf).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) || IsUniqueViolation(err) {
			return "", fmt.Errorf("%w: %s", ErrNotResendable, p.RetryOf)
		}
		if err != nil {
			return "", fmt.Errorf("insert addon operation resend: %w", err)
		}
		details, _ = json.Marshal(map[string]any{"attempt": 2, "retry_of": p.RetryOf, "retried_by": "automatic"})
	}

	const insertAudit = `INSERT INTO audit_logs
		(actor_zitadel_user_id, target_zitadel_user_id, action, resource_id, details) VALUES ($1,$2,$3,$4,$5)`
	action := "addon." + p.Target + "." + p.Operation + ".dispatched"
	if _, err := tx.Exec(ctx, insertAudit, p.ActorID, p.SubjectID, action, id, details); err != nil {
		return "", fmt.Errorf("insert addon operation audit: %w", err)
	}
	return id, nil
}

// ErrNotResendable means a resend named a send that is not a first send of the
// same call, or that has already been resent once.
var ErrNotResendable = errors.New("db: that send cannot be resent")

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
	err := querier(ctx).QueryRow(ctx, q, id, target, operation, subject).Scan(&o.ID, &o.Target, &o.Operation,
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
	tag, err := querier(ctx).Exec(ctx, q, id, status)
	if err != nil {
		return fmt.Errorf("settle addon operation: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: %s", ErrAddonOperationAlreadySettled, id)
	}
	return nil
}

// AddonUnresolvedGrace is how long a claimed call may go unanswered before it
// is a question rather than a call in flight. Comfortably past the add-on
// call's 30-second timeout.
const AddonUnresolvedGrace = 2 * time.Minute

// SelfSettledOperations are the kinds nobody is asked about. A member settles
// their own password by setting it again (A-13 §4), and a rotate belongs to a
// revoke, which already waits in Unfinished revocations (§5). An operator never
// sees either on Waiting on you, its counts, or Home.
var SelfSettledOperations = []string{"password.set", "password.rotate"}

// ListUnresolvedAddonOperations returns one target's operations nobody has an
// answer for, newest first (A-09 §3), excluding the kinds that settle
// themselves and `dispatching` rows young enough to still be in flight.
//
// Claimed rows only. A record never claimed was never sent, so it is not a
// question: SettleUnclaimedAddonOperations closes those as `unreached`.
//
// The grace period is what stops the surface flickering: a call in progress is
// indistinguishable by status alone from one whose backend died, and only time
// separates them. Callers pass something comfortably longer than the dispatch
// timeout. Age runs from the claim, since that is when the call actually started.
func ListUnresolvedAddonOperations(ctx context.Context, target string, grace time.Duration, limit int) ([]AddonOperation, error) {
	const q = `
		SELECT id, target, operation, actor_id, subject_id, status, created_at, claimed_at, settled_at, COALESCE(about, '')
		  FROM addon_operations
		 WHERE target = $1 AND resolution IS NULL AND claimed_at IS NOT NULL
		   AND operation <> ALL($4::text[])
		   AND (status = 'indeterminate'
		        OR (status = 'dispatching' AND claimed_at < NOW() - $2::interval))
		 ORDER BY created_at DESC
		 LIMIT $3`
	rows, err := querier(ctx).Query(ctx, q, target, grace.String(), limit, SelfSettledOperations)
	if err != nil {
		return nil, fmt.Errorf("list unresolved addon operations: %w", err)
	}
	defer rows.Close()

	out := []AddonOperation{}
	for rows.Next() {
		var o AddonOperation
		if err := rows.Scan(&o.ID, &o.Target, &o.Operation, &o.ActorID, &o.SubjectID,
			&o.Status, &o.CreatedAt, &o.ClaimedAt, &o.SettledAt, &o.About); err != nil {
			return nil, fmt.Errorf("scan unresolved addon operation: %w", err)
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// CountUnresolvedAddonOperations is ListUnresolvedAddonOperations' predicate as
// one number per target, for the governance indicators. Targets with none are
// absent.
func CountUnresolvedAddonOperations(ctx context.Context, grace time.Duration) (map[string]int, error) {
	const q = `
		SELECT target, COUNT(*)
		  FROM addon_operations
		 WHERE resolution IS NULL AND claimed_at IS NOT NULL
		   AND operation <> ALL($2::text[])
		   AND (status = 'indeterminate'
		        OR (status = 'dispatching' AND claimed_at < NOW() - $1::interval))
		 GROUP BY target`
	rows, err := querier(ctx).Query(ctx, q, grace.String(), SelfSettledOperations)
	if err != nil {
		return nil, fmt.Errorf("count unresolved addon operations: %w", err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var target string
		var n int
		if err := rows.Scan(&target, &n); err != nil {
			return nil, fmt.Errorf("scan unresolved addon operation count: %w", err)
		}
		out[target] = n
	}
	return out, rows.Err()
}

// SettleUnclaimedAddonOperations closes, as `unreached`, every record on a
// target that was written and never claimed, past the grace period. Nothing was
// sent, so this is certain, and it is the one settle the claim CHECK allows an
// unclaimed row. Returns how many it closed.
func SettleUnclaimedAddonOperations(ctx context.Context, target string, grace time.Duration) (int64, error) {
	const q = `
		UPDATE addon_operations
		   SET status = 'unreached', settled_at = NOW()
		 WHERE target = $1 AND status = 'dispatching' AND claimed_at IS NULL
		   AND created_at < NOW() - $2::interval`
	tag, err := querier(ctx).Exec(ctx, q, target, grace.String())
	if err != nil {
		return 0, fmt.Errorf("settle unclaimed addon operations: %w", err)
	}
	return tag.RowsAffected(), nil
}

// Resolutions an open operation may be closed with.
const (
	AddonResolutionHappened    = "happened"
	AddonResolutionNotHappened = "not_happened"
	AddonResolutionSuperseded  = "superseded"
)

// SyndraReader is the actor recorded when Syndra settles an operation from a
// read of the target rather than from a person's answer.
const SyndraReader = "system:syndra"

// ErrAddonOperationNotUnresolved means the record is not one a person may
// answer: not on this target, never sent, already answered, still in flight,
// or a kind that settles itself.
var ErrAddonOperationNotUnresolved = errors.New("db: that change is not waiting for an answer")

// ResolveAddonOperation records what happened to an unconfirmed operation,
// with who said so, and its audit row, in one transaction.
//
// Its own path, apart from SettleAddonOperation, on purpose. The dispatch path
// may never move `indeterminate` anywhere, because a late answer there would
// be evidence of nothing. This path moves `status` one step at most: a claimed
// `dispatching` row past its grace becomes `indeterminate` (the honest reading
// of a call whose answer never came), so a very late dispatch settle is then
// refused rather than writing over the answer. The answer itself goes in
// `resolution`; nothing here can write `succeeded`.
//
// by is the operator's id, or SyndraReader for a read of the target; the audit
// action says which (`.answered` or `.read`), with the answer in its details.
func ResolveAddonOperation(ctx context.Context, target, id string, happened bool, by string, grace time.Duration) (AddonOperation, error) {
	resolution := AddonResolutionNotHappened
	if happened {
		resolution = AddonResolutionHappened
	}
	tx, err := PG.Begin(ctx)
	if err != nil {
		return AddonOperation{}, fmt.Errorf("begin resolve addon operation tx: %w", err)
	}
	defer tx.Rollback(ctx) // no-op after a successful Commit

	const q = `
		UPDATE addon_operations
		   SET status = 'indeterminate', settled_at = COALESCE(settled_at, NOW()),
		       resolution = $3, resolved_by = $4, resolved_at = NOW()
		 WHERE id = $1::uuid AND target = $2 AND resolution IS NULL AND claimed_at IS NOT NULL
		   AND operation <> ALL($6::text[])
		   AND (status = 'indeterminate'
		        OR (status = 'dispatching' AND claimed_at < NOW() - $5::interval))
		RETURNING id, target, operation, actor_id, subject_id, status, created_at, claimed_at, settled_at, COALESCE(about, '')`
	var o AddonOperation
	err = tx.QueryRow(ctx, q, id, target, resolution, by, grace.String(), SelfSettledOperations).Scan(
		&o.ID, &o.Target, &o.Operation, &o.ActorID, &o.SubjectID, &o.Status, &o.CreatedAt, &o.ClaimedAt, &o.SettledAt, &o.About)
	if errors.Is(err, pgx.ErrNoRows) {
		return AddonOperation{}, ErrAddonOperationNotUnresolved
	}
	if err != nil {
		return AddonOperation{}, fmt.Errorf("resolve addon operation: %w", err)
	}

	how := ".answered"
	if by == SyndraReader {
		how = ".read"
	}
	details, _ := json.Marshal(map[string]any{"happened": happened})
	const insertAudit = `INSERT INTO audit_logs
		(actor_zitadel_user_id, target_zitadel_user_id, action, resource_id, details) VALUES ($1,$2,$3,$4,$5)`
	if _, err := tx.Exec(ctx, insertAudit, by, o.SubjectID, "addon."+o.Target+"."+o.Operation+how, o.ID, details); err != nil {
		return AddonOperation{}, fmt.Errorf("insert addon operation answer audit: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return AddonOperation{}, fmt.Errorf("commit resolve addon operation tx: %w", err)
	}
	return o, nil
}

// SupersedeAddonOperations closes every earlier open operation of the same
// kind, for the same person and system, as made moot by `by`, which the target
// confirmed (A-13 §4: a confirmed password.set settles every earlier
// unconfirmed one). Their own status is untouched: each still says its answer
// was lost.
func SupersedeAddonOperations(ctx context.Context, by string) (int64, error) {
	const q = `
		UPDATE addon_operations o
		   SET resolution = 'superseded', superseded_by = b.id,
		       resolved_by = b.subject_id, resolved_at = NOW()
		  FROM addon_operations b
		 WHERE b.id = $1::uuid AND b.status = 'succeeded'
		   AND o.target = b.target AND o.subject_id = b.subject_id AND o.operation = b.operation
		   AND o.id <> b.id AND o.created_at <= b.created_at
		   AND o.resolution IS NULL AND o.claimed_at IS NOT NULL
		   AND o.status IN ('dispatching', 'indeterminate')`
	tag, err := querier(ctx).Exec(ctx, q, by)
	if err != nil {
		return 0, fmt.Errorf("supersede addon operations: %w", err)
	}
	return tag.RowsAffected(), nil
}

// ResendableAddonOperation finds the first send the page may resend once: this
// subject's latest attempt-1 record of this operation on this target, inside
// the window, not settled as refused or unreached (the page had a clear answer
// for those and does not resend), and not resent already. found is false when
// there is none, and the caller then treats the send as a fresh change.
func ResendableAddonOperation(ctx context.Context, target, subject, operation string, window time.Duration) (id string, found bool, err error) {
	const q = `
		SELECT o.id FROM addon_operations o
		 WHERE o.target = $1 AND o.subject_id = $2 AND o.operation = $3 AND o.attempt = 1
		   AND o.created_at > NOW() - $4::interval
		   AND o.status NOT IN ('rejected', 'unreached')
		   AND NOT EXISTS (SELECT 1 FROM addon_operations r WHERE r.retry_of = o.id)
		 ORDER BY o.created_at DESC
		 LIMIT 1`
	err = querier(ctx).QueryRow(ctx, q, target, subject, operation, window.String()).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("find resendable addon operation: %w", err)
	}
	return id, true, nil
}

// UnconfirmedChangeAt is when a subject's latest change of this operation on
// this target was sent, while that change is still unconfirmed: no answer on
// either of its sends, and nothing later has settled it. found is false
// otherwise (A-13 For code: `password_change`).
//
// The latest FIRST send stands for the change, with its resend folded in. A
// resend TrueNAS confirmed supersedes the first, so the change reads as settled.
func UnconfirmedChangeAt(ctx context.Context, target, subject, operation string, grace time.Duration) (at time.Time, found bool, err error) {
	const q = `
		SELECT COALESCE(o.claimed_at, o.created_at),
		       o.resolution IS NULL AND o.claimed_at IS NOT NULL
		       AND (o.status = 'indeterminate'
		            OR (o.status = 'dispatching' AND o.claimed_at < NOW() - $4::interval))
		  FROM addon_operations o
		 WHERE o.target = $1 AND o.subject_id = $2 AND o.operation = $3 AND o.attempt = 1
		 ORDER BY o.created_at DESC
		 LIMIT 1`
	var open bool
	err = querier(ctx).QueryRow(ctx, q, target, subject, operation, grace.String()).Scan(&at, &open)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, fmt.Errorf("read unconfirmed change: %w", err)
	}
	return at, open, nil
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
	if err := querier(ctx).QueryRow(ctx, q, target).Scan(&c.Succeeded, &c.Failed, &c.Unresolved); err != nil {
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
func CountRecentAddonOperations(ctx context.Context, target, subjectID, operation string, window time.Duration) (int, error) {
	const q = `
		SELECT COUNT(*) FROM addon_operations
		 WHERE target = $1 AND subject_id = $2 AND operation = $3
		   -- The page's one automatic resend is not a second change, and
		   -- counting it would let the limiter answer the resend (A-13).
		   AND retry_of IS NULL
		   -- make_interval, not a parameter concatenated with a unit word and
		   -- cast to interval. That concatenation
		   -- makes $4 TEXT and pgx cannot encode a Go integer for it, so this
		   -- count returned an ERROR rather than a number — and a rate limiter
		   -- that cannot count is one that is not limiting.
		   AND created_at > NOW() - make_interval(secs => $4::double precision)`
	var n int
	if err := querier(ctx).QueryRow(ctx, q, target, subjectID, operation, int64(window/time.Second)).Scan(&n); err != nil {
		return 0, fmt.Errorf("count recent addon operations: %w", err)
	}
	return n, nil
}
