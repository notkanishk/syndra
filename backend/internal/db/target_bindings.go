package db

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
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

// Version is the binding's strong validator (RFC 9110 §8.8.3, an ETag): it changes when the
// row is replaced (bound_at) or moved to another account in place (an adoption keeps bound_at but
// changes the name or id), and not on a sweep's last_seen_at. A page that read the row sends it
// back as If-Match would be sent, and a release refuses when it no longer matches.
func (b TargetBinding) Version() string {
	id := ""
	if b.AccountID != nil {
		id = *b.AccountID
	}
	sum := sha256.Sum256([]byte(b.BoundAt.UTC().Format(time.RFC3339Nano) + "\x00" + b.Username + "\x00" + id))
	return hex.EncodeToString(sum[:8])
}

// RecordTargetBinding writes what the target reported for a subject.
//
// Idempotent by design: an apply reports the account every time it converges,
// and the common case is a binding that has not changed.
//
// The add-on's answer is the authority (design §9.4b, ADR-007): its binding is
// the only link from a person to an account, and this table is the backend's
// mirror of it. So another subject's row on the same account is the stale one
// (TrueNAS handed a freed uid to a new account, and the add-on bound it to this
// subject), and it is cleared in the same transaction, logged and audited
// (binding.<target>.stale_cleared, naming who it was), never refused. "The same
// account" is the account id where both rows carry one; the name only where
// either has none. A row on the same name with a DIFFERENT id is another
// account whose mirrored name is out of date (renamed on the target, the
// add-on's binding following it by number), and it is kept: clearing it would
// drop a person the add-on still binds from the sweep until their next apply.
// Two LIVE claims on one account are the add-on's to report (/subjects
// `claimed_twice`, the binding_conflict finding); it never answers an apply or
// adoption for an account another binding names.
//
// No lock beyond the rows': two writes for two subjects on one account at once
// would need the add-on to give one account to both, which it refuses, and the
// next write would clear the loser anyway. (A per-target advisory lock here
// would invert the order ResolveBindingCall takes, row lock then this.)
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
	const upsert = `
		INSERT INTO target_account_bindings (target, subject_id, username, account_id, account_uid, bound_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (target, subject_id) DO UPDATE
		   SET username     = EXCLUDED.username,
		       account_id   = COALESCE(EXCLUDED.account_id, target_account_bindings.account_id),
		       account_uid  = CASE WHEN EXCLUDED.account_id IS NOT NULL THEN EXCLUDED.account_uid ELSE target_account_bindings.account_uid END,
		       last_seen_at = NOW()`
	// The same account: the same id when both carry one, the same name when
	// either does not. A NULL $4 never matches on the id, so a write that
	// reported none cannot match every row that also has none.
	const clearStale = `
		DELETE FROM target_account_bindings
		 WHERE target = $1 AND subject_id <> $2
		   AND (` + accountIDColumn + ` = $4::text
		        OR (username = $3 AND ($4::text IS NULL OR ` + accountIDColumn + ` IS NULL)))
		RETURNING subject_id, username, ` + accountIDColumn

	tx, owned, err := beginOrJoin(ctx)
	if err != nil {
		return err
	}
	if owned {
		defer tx.Rollback(ctx)
	}
	rows, err := tx.Query(ctx, clearStale, b.Target, b.SubjectID, b.Username, b.AccountID)
	if err != nil {
		return fmt.Errorf("clear stale bindings on %s: %w", b.Target, err)
	}
	stale, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (TargetBinding, error) {
		var s TargetBinding
		err := r.Scan(&s.SubjectID, &s.Username, &s.AccountID)
		return s, err
	})
	if err != nil {
		return fmt.Errorf("clear stale bindings on %s: %w", b.Target, err)
	}
	for _, s := range stale {
		log.Printf("[TARGETS] %s on %s now belongs to %s; cleared %s's stale record on the same name or account id (it named %s)",
			b.Username, b.Target, b.SubjectID, s.SubjectID, s.Username)
		details, _ := json.Marshal(map[string]any{
			"username": s.Username, "account_id": s.AccountID, "now_bound_to": b.SubjectID,
		})
		if _, err := tx.Exec(ctx, `INSERT INTO audit_logs
			(actor_zitadel_user_id, target_zitadel_user_id, action, resource_id, details) VALUES ($1, $2, $3, $4, $5)`,
			b.BoundBy, s.SubjectID, "binding."+b.Target+".stale_cleared", s.Username, details); err != nil {
			return fmt.Errorf("audit the stale binding cleared on %s: %w", b.Target, err)
		}
	}
	if _, err := tx.Exec(ctx, upsert, b.Target, b.SubjectID, b.Username, b.AccountID, legacyUID(b.AccountID), b.BoundBy); err != nil {
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
	if _, err := querier(ctx).Exec(ctx, `UPDATE target_account_bindings SET account_id = account_uid::text
		WHERE account_uid IS NOT NULL AND account_id IS DISTINCT FROM account_uid::text`); err != nil {
		return fmt.Errorf("backfill account_id in target_account_bindings: %w", err)
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
