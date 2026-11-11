package db

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// The backend's record of which account on a target belongs to which subject
// (migration 000032, change `addon-platform` 1.18).
//
// Written from what the add-on REPORTS, never from what the backend guesses. The
// apply resolves a subject to an account through the add-on's own store; this
// follows that decision rather than competing with it, which is why every writer
// here is downstream of an add-on answer.

// ErrBindingConflict is an account already recorded against somebody else.
//
// Its own error because its own operator action: adopting an account that is
// already somebody's is not a retry, it is a question about which of the two
// people is meant to have it.
var ErrBindingConflict = errors.New("db: that account on that target is already bound to another subject")

// TargetBinding is one subject's account on one target.
//
// AccountID is the target's own stable id for the account (contract 2's
// account_id, a string). It is stored in account_id and, while it is a
// number, in account_uid too, for one release (migration 000088, decision 10),
// so a rolled-back contract-1 backend still reads it.
type TargetBinding struct {
	Target     string    `json:"target"`
	SubjectID  string    `json:"subject_id"`
	Username   string    `json:"username"`
	AccountID  *string   `json:"account_id,omitempty"`
	BoundBy    string    `json:"bound_by"`
	BoundAt    time.Time `json:"bound_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
}

// RecordTargetBinding writes what the target reported for a subject.
//
// Idempotent by design: an apply reports the account every time it converges,
// and the common case is a binding that has not changed. What it must NOT do is
// move an account from one subject to another silently — that is the conflict
// above, and it is raised rather than resolved.
//
// `bound_by` and `bound_at` survive an update. They record who first attached
// this account to this person, which is the question asked after an adoption
// turns out to have been wrong; overwriting them on every convergence would
// replace that answer with "the last drain".
func RecordTargetBinding(ctx context.Context, b TargetBinding) error {
	switch {
	case strings.TrimSpace(b.Target) == "" || b.Target == TargetZitadel:
		return fmt.Errorf("%w: %q holds no accounts of its own", ErrInvalidTargetBinding, b.Target)
	case strings.TrimSpace(b.SubjectID) == "":
		return fmt.Errorf("%w: no subject", ErrInvalidTargetBinding)
	case strings.TrimSpace(b.Username) == "":
		return fmt.Errorf("%w: no account name", ErrInvalidTargetBinding)
	case strings.TrimSpace(b.BoundBy) == "":
		return fmt.Errorf("%w: no actor", ErrInvalidTargetBinding)
	}

	// account_uid follows account_id whenever the write carries one (NULL for
	// a non-numeric id), and stays as it was otherwise: so the two differ only
	// where a contract-1 backend wrote account_uid, which is exactly what
	// BackfillAccountIDs corrects. An id, once recorded, is never erased by a
	// write that reported none.
	const q = `
		INSERT INTO target_account_bindings (target, subject_id, username, account_id, account_uid, bound_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (target, subject_id) DO UPDATE
		   SET username     = EXCLUDED.username,
		       account_id   = COALESCE(EXCLUDED.account_id, target_account_bindings.account_id),
		       account_uid  = CASE WHEN EXCLUDED.account_id IS NOT NULL THEN EXCLUDED.account_uid ELSE target_account_bindings.account_uid END,
		       last_seen_at = NOW()`

	tx, owned, err := beginOrJoin(ctx)
	if err != nil {
		return err
	}
	if owned {
		defer tx.Rollback(ctx)
	}
	if _, err := tx.Exec(ctx, q, b.Target, b.SubjectID, b.Username, b.AccountID, legacyUID(b.AccountID), b.BoundBy); err != nil {
		// The unique indexes on (target, username) and (target, uid) are what
		// this can violate, and both mean the same thing: the account belongs to
		// somebody else already. Reported as that rather than as a constraint
		// name, which describes the schema to whoever asked.
		if IsUniqueViolation(err) {
			return fmt.Errorf("%w: %s on %s", ErrBindingConflict, b.Username, b.Target)
		}
		return fmt.Errorf("record binding for %s on %s: %w", b.SubjectID, b.Target, err)
	}
	if owned {
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit binding: %w", err)
		}
	}
	return nil
}

// accountIDColumn reads account_id, or account_uid where only that was written
// (a row a contract-1 backend wrote since the last backfill).
const accountIDColumn = `COALESCE(account_id, account_uid::text)`

// legacyUID is account_id as contract 1's account_uid, written beside it for
// one release (decision 10): the number when it is one, NULL when it is not
// (a UUID has no account_uid, and contract 1 never bound one).
func legacyUID(accountID *string) *int64 {
	if accountID == nil {
		return nil
	}
	n, err := strconv.ParseInt(*accountID, 10, 64)
	if err != nil || strconv.FormatInt(n, 10) != *accountID {
		return nil
	}
	return &n
}

// BackfillAccountIDs makes account_id say what account_uid says wherever a
// contract-1 backend wrote or updated only the second: before 000088, or
// during a rollback (a rename it recorded, an account recreated with a new
// uid). Re-runnable, and run at every start, so a rollback and a forward
// deploy again lose nothing (task 1.7).
func BackfillAccountIDs(ctx context.Context) error {
	for _, table := range []string{"target_account_bindings", "target_binding_conflicts"} {
		if _, err := querier(ctx).Exec(ctx, `UPDATE `+table+` SET account_id = account_uid::text
			WHERE account_uid IS NOT NULL AND account_id IS DISTINCT FROM account_uid::text`); err != nil {
			return fmt.Errorf("backfill account_id in %s: %w", table, err)
		}
	}
	return nil
}

// ErrInvalidTargetBinding refuses a binding that records nothing usable.
var ErrInvalidTargetBinding = errors.New("db: invalid target account binding")

// GetTargetBinding returns one subject's account on one target.
func GetTargetBinding(ctx context.Context, target, subjectID string) (TargetBinding, bool, error) {
	const q = `
		SELECT target, subject_id, username, ` + accountIDColumn + `, bound_by, bound_at, last_seen_at
		  FROM target_account_bindings
		 WHERE target = $1 AND subject_id = $2`
	var b TargetBinding
	err := querier(ctx).QueryRow(ctx, q, target, subjectID).Scan(
		&b.Target, &b.SubjectID, &b.Username, &b.AccountID, &b.BoundBy, &b.BoundAt, &b.LastSeenAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return TargetBinding{}, false, nil
	}
	if err != nil {
		return TargetBinding{}, false, fmt.Errorf("read binding for %s on %s: %w", subjectID, target, err)
	}
	return b, true, nil
}

// ListTargetBindings returns every binding on a target, in a stable order.
//
// The whole set rather than a lookup per account, because its one consumer is
// the inventory: a listing that asked per account would be one query per row of
// a full state read.
func ListTargetBindings(ctx context.Context, target string) ([]TargetBinding, error) {
	const q = `
		SELECT target, subject_id, username, ` + accountIDColumn + `, bound_by, bound_at, last_seen_at
		  FROM target_account_bindings
		 WHERE target = $1
		 ORDER BY username`
	rows, err := querier(ctx).Query(ctx, q, target)
	if err != nil {
		return nil, fmt.Errorf("list bindings on %s: %w", target, err)
	}
	defer rows.Close()

	var out []TargetBinding
	for rows.Next() {
		var b TargetBinding
		if err := rows.Scan(&b.Target, &b.SubjectID, &b.Username, &b.AccountID,
			&b.BoundBy, &b.BoundAt, &b.LastSeenAt); err != nil {
			return nil, fmt.Errorf("scan binding: %w", err)
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// ForgetTargetBinding removes a binding whose account is gone.
//
// Called after a purge, and only then. A binding that outlives its account is
// not a harmless leftover: the apply path reads bound-but-absent as an
// out-of-band deletion and recreates the account under the recorded name, which
// is right for one somebody else deleted and exactly wrong for one we purged.
func ForgetTargetBinding(ctx context.Context, target, subjectID string) error {
	tx, owned, err := beginOrJoin(ctx)
	if err != nil {
		return err
	}
	if owned {
		defer tx.Rollback(ctx)
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM target_account_bindings WHERE target = $1 AND subject_id = $2`,
		target, subjectID); err != nil {
		return fmt.Errorf("forget binding for %s on %s: %w", subjectID, target, err)
	}
	if owned {
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit binding removal: %w", err)
		}
	}
	return nil
}
