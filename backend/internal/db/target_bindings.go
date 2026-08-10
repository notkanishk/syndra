package db

import (
	"context"
	"errors"
	"fmt"
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
type TargetBinding struct {
	Target     string    `json:"target"`
	SubjectID  string    `json:"subject_id"`
	Username   string    `json:"username"`
	AccountUID *int64    `json:"account_uid,omitempty"`
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

	const q = `
		INSERT INTO target_account_bindings (target, subject_id, username, account_uid, bound_by)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (target, subject_id) DO UPDATE
		   SET username     = EXCLUDED.username,
		       account_uid  = COALESCE(EXCLUDED.account_uid, target_account_bindings.account_uid),
		       last_seen_at = NOW()`

	tx, owned, err := beginOrJoin(ctx)
	if err != nil {
		return err
	}
	if owned {
		defer tx.Rollback(ctx)
	}
	if _, err := tx.Exec(ctx, q, b.Target, b.SubjectID, b.Username, b.AccountUID, b.BoundBy); err != nil {
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

// ErrInvalidTargetBinding refuses a binding that records nothing usable.
var ErrInvalidTargetBinding = errors.New("db: invalid target account binding")

// GetTargetBinding returns one subject's account on one target.
func GetTargetBinding(ctx context.Context, target, subjectID string) (TargetBinding, bool, error) {
	const q = `
		SELECT target, subject_id, username, account_uid, bound_by, bound_at, last_seen_at
		  FROM target_account_bindings
		 WHERE target = $1 AND subject_id = $2`
	var b TargetBinding
	err := PG.QueryRow(ctx, q, target, subjectID).Scan(
		&b.Target, &b.SubjectID, &b.Username, &b.AccountUID, &b.BoundBy, &b.BoundAt, &b.LastSeenAt)
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
		SELECT target, subject_id, username, account_uid, bound_by, bound_at, last_seen_at
		  FROM target_account_bindings
		 WHERE target = $1
		 ORDER BY username`
	rows, err := PG.Query(ctx, q, target)
	if err != nil {
		return nil, fmt.Errorf("list bindings on %s: %w", target, err)
	}
	defer rows.Close()

	var out []TargetBinding
	for rows.Next() {
		var b TargetBinding
		if err := rows.Scan(&b.Target, &b.SubjectID, &b.Username, &b.AccountUID,
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
