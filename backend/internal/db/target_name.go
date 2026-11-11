package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrTargetNameInvalid is a name the schema refuses (targets_name_check):
// blank, or longer than a rail row holds. The rule lives in the CHECK alone.
var ErrTargetNameInvalid = errors.New("A name is at most 60 characters.")

// TargetNameAudit is the audit action a name change writes.
const TargetNameAudit = "target.named"

// TargetName is what an operator named a target, "" when they have not.
func TargetName(ctx context.Context, target string) (string, error) {
	var name *string
	err := querier(ctx).QueryRow(ctx, `SELECT name FROM targets WHERE target = $1`, target).Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && name == nil) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read name of %s: %w", target, err)
	}
	return *name, nil
}

// SetTargetName names a target, or clears its name when name is "", and
// writes the audit row in the same transaction: a rename nobody can attribute
// is a member asking why their system changed name and nobody able to say.
func SetTargetName(ctx context.Context, target, name, actor string) error {
	var to *string
	if name != "" {
		to = &name
	}
	return InTx(ctx, func(tx pgx.Tx) error {
		var from *string
		err := tx.QueryRow(ctx, `SELECT name FROM targets WHERE target = $1 FOR UPDATE`, target).Scan(&from)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("no target %q", target)
		}
		if err != nil {
			return fmt.Errorf("read name of %s: %w", target, err)
		}
		_, err = tx.Exec(ctx, `UPDATE targets SET name = $2 WHERE target = $1`, target, to)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23514" && pgErr.ConstraintName == "targets_name_check" {
			return ErrTargetNameInvalid
		}
		if err != nil {
			return fmt.Errorf("set name of %s: %w", target, err)
		}
		return InsertAuditLogDetails(context.WithValue(ctx, txKey, tx), actor, "", TargetNameAudit, target,
			map[string]*string{"from": from, "to": to})
	})
}
