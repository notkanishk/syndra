package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Two records disagreeing about who owns an account (§29; design §11).
//
// `RecordTargetBinding` refuses when the account the add-on just wrote to is
// one this backend attributes to a different subject. That refusal used to be a
// log line under a row marked `applied`; the accounting fix made the row settle
// terminally, and this is where the finding lives so it outlives the drain that
// produced it.
//
// It records a DISAGREEMENT, not a verdict. Syndra cannot tell which subject
// owns the account — that is the whole content of the finding — so both are
// named and the resolution is an operator's.

// ErrNoSuchConflict is a finding cited that does not exist or is already
// resolved. Refused rather than treated as done: resolving one twice would let
// a second operator believe they made a decision somebody else had already made
// differently.
var ErrNoSuchConflict = errors.New("db: no standing binding conflict")

// BindingConflict is one standing disagreement.
type BindingConflict struct {
	ID       string `json:"id"`
	Target   string `json:"target"`
	Username string `json:"username"`
	// AccountUID is the target's stable identity for the account when it
	// reported one.
	AccountUID *int64 `json:"account_uid,omitempty"`
	// ConvergedSubjectID's change landed on the account; BoundSubjectID is who
	// Syndra's own binding says owns it. Named apart rather than as
	// expected/actual, because neither is authoritative.
	ConvergedSubjectID string    `json:"converged_subject_id"`
	BoundSubjectID     string    `json:"bound_subject_id"`
	OutboxID           string    `json:"outbox_id"`
	DetectedAt         time.Time `json:"detected_at"`
}

// RecordBindingConflict persists a standing finding, or leaves the existing one
// alone.
//
// Idempotent on purpose. A re-drive of the same row re-detects the same
// disagreement, and stacking a second finding would turn one problem an
// operator has to decide about into a growing list of the same problem — which
// reads as it getting worse.
func RecordBindingConflict(ctx context.Context, c BindingConflict) error {
	switch {
	case strings.TrimSpace(c.Target) == "":
		return fmt.Errorf("%w: no target", ErrInvalidTargetBinding)
	case strings.TrimSpace(c.Username) == "":
		return fmt.Errorf("%w: no account name", ErrInvalidTargetBinding)
	case strings.TrimSpace(c.ConvergedSubjectID) == "" || strings.TrimSpace(c.BoundSubjectID) == "":
		return fmt.Errorf("%w: a conflict names two subjects", ErrInvalidTargetBinding)
	case c.ConvergedSubjectID == c.BoundSubjectID:
		// Refused rather than recorded. A subject cannot conflict with
		// themselves, so this is a bug in the detector, and rendering it as a
		// finding would put a person's name on a screen for no reason.
		return fmt.Errorf("%w: %s conflicts with itself", ErrInvalidTargetBinding, c.ConvergedSubjectID)
	}

	const q = `
		INSERT INTO target_binding_conflicts
			(target, username, account_uid, converged_subject_id, bound_subject_id, outbox_id)
		VALUES ($1, $2, $3, $4, $5, $6::uuid)
		ON CONFLICT (target, username) WHERE resolved_at IS NULL DO NOTHING`
	if _, err := querier(ctx).Exec(ctx, q,
		c.Target, c.Username, c.AccountUID, c.ConvergedSubjectID, c.BoundSubjectID, c.OutboxID); err != nil {
		return fmt.Errorf("record binding conflict on %s: %w", c.Target, err)
	}
	return nil
}

// StandingBindingConflicts lists what is still unresolved on a target.
func StandingBindingConflicts(ctx context.Context, target string) ([]BindingConflict, error) {
	const q = `
		SELECT id, target, username, account_uid, converged_subject_id, bound_subject_id,
		       outbox_id::text, detected_at
		  FROM target_binding_conflicts
		 WHERE target = $1 AND resolved_at IS NULL
		 ORDER BY detected_at DESC`
	rows, err := querier(ctx).Query(ctx, q, target)
	if err != nil {
		return nil, fmt.Errorf("read binding conflicts for %s: %w", target, err)
	}
	defer rows.Close()

	var out []BindingConflict
	for rows.Next() {
		var c BindingConflict
		if err := rows.Scan(&c.ID, &c.Target, &c.Username, &c.AccountUID,
			&c.ConvergedSubjectID, &c.BoundSubjectID, &c.OutboxID, &c.DetectedAt); err != nil {
			return nil, fmt.Errorf("scan binding conflict: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ResolveBindingConflict records who the account belongs to and rebinds it.
//
// One transaction, because the two halves are one decision: forgetting the
// losing binding without recording the winning one leaves the account owned by
// nobody — which puts it in the unmanaged inventory, where the offered action
// is adoption, which is the hazard this whole path exists to keep an operator
// away from.
//
// `owner` must be one of the two subjects the finding names. An operator
// assigning the account to a third party is not resolving this disagreement;
// they are making a different decision that has no rehearsal behind it.
func ResolveBindingConflict(ctx context.Context, id, owner, actor, note string) (BindingConflict, error) {
	switch {
	case strings.TrimSpace(actor) == "":
		return BindingConflict{}, fmt.Errorf("%w: no actor", ErrNoSuchConflict)
	case strings.TrimSpace(note) == "":
		return BindingConflict{}, fmt.Errorf("%w: resolving a conflict takes an explanation", ErrNoSuchConflict)
	}

	tx, owned, err := beginOrJoin(ctx)
	if err != nil {
		return BindingConflict{}, err
	}
	if owned {
		defer tx.Rollback(ctx)
	}

	const read = `
		SELECT id, target, username, account_uid, converged_subject_id, bound_subject_id,
		       outbox_id::text, detected_at
		  FROM target_binding_conflicts
		 WHERE id = $1::uuid AND resolved_at IS NULL
		 FOR UPDATE`
	var c BindingConflict
	if err := tx.QueryRow(ctx, read, id).Scan(&c.ID, &c.Target, &c.Username, &c.AccountUID,
		&c.ConvergedSubjectID, &c.BoundSubjectID, &c.OutboxID, &c.DetectedAt); err != nil {
		return BindingConflict{}, fmt.Errorf("%w: %s", ErrNoSuchConflict, id)
	}
	if owner != c.ConvergedSubjectID && owner != c.BoundSubjectID {
		return BindingConflict{}, fmt.Errorf(
			"%w: the account is claimed by %s and %s; %s is neither",
			ErrInvalidTargetBinding, c.ConvergedSubjectID, c.BoundSubjectID, owner)
	}

	loser := c.ConvergedSubjectID
	if owner == c.ConvergedSubjectID {
		loser = c.BoundSubjectID
	}

	// The losing binding goes first. Both orders leave the same end state, and
	// this one is the safe intermediate: the unique index on (target, username)
	// refuses the winning insert while the loser still holds it.
	if _, err := tx.Exec(ctx,
		`DELETE FROM target_account_bindings WHERE target = $1 AND subject_id = $2`,
		c.Target, loser); err != nil {
		return BindingConflict{}, fmt.Errorf("forget the losing binding: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO target_account_bindings (target, subject_id, username, account_uid, bound_by)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (target, subject_id) DO UPDATE
		   SET username = EXCLUDED.username,
		       account_uid = COALESCE(EXCLUDED.account_uid, target_account_bindings.account_uid),
		       last_seen_at = NOW()`,
		c.Target, owner, c.Username, c.AccountUID, actor); err != nil {
		return BindingConflict{}, fmt.Errorf("record the winning binding: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE target_binding_conflicts
		   SET resolved_at = NOW(), resolved_by = $2, resolution = $3
		 WHERE id = $1::uuid`, id, actor, note); err != nil {
		return BindingConflict{}, fmt.Errorf("close the conflict: %w", err)
	}

	const audit = `INSERT INTO audit_logs
		(actor_zitadel_user_id, target_zitadel_user_id, action, resource_id) VALUES ($1,$2,$3,$4)`
	if _, err := tx.Exec(ctx, audit, actor, owner,
		"target."+c.Target+".binding_conflict_resolved", c.Username); err != nil {
		return BindingConflict{}, fmt.Errorf("record the resolution: %w", err)
	}

	if owned {
		if err := tx.Commit(ctx); err != nil {
			return BindingConflict{}, fmt.Errorf("commit conflict resolution: %w", err)
		}
	}
	return c, nil
}

// BindingHolder answers "who does this backend say owns this account".
//
// Matched on the name OR the uid, because a conflict is raised by whichever of
// the two unique indexes fired and the answer has to cover both. Guessing from
// the name alone would name the wrong person exactly when the account was
// renamed out of band — which is the case the uid index exists for.
//
// The uid arm is guarded on non-zero: zero is "the target reported none", and a
// binding with a NULL uid must not match every account that also has none.
func BindingHolder(ctx context.Context, target, username string, uid int64) (string, bool, error) {
	const q = `
		SELECT subject_id FROM target_account_bindings
		 WHERE target = $1
		   AND (username = $2 OR ($3 <> 0 AND account_uid = $3))
		 LIMIT 1`
	var subject string
	err := querier(ctx).QueryRow(ctx, q, target, username, uid).Scan(&subject)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read the binding holding %s on %s: %w", username, target, err)
	}
	return subject, true, nil
}
