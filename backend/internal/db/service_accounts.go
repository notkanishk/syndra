package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// ServiceAccountRow is Syndra's own record of a service account (migration
// 000067). Whether a user IS a machine is Zitadel's to say; this holds what
// Zitadel cannot: the operator's mark on a human-type account, and the kind,
// purpose and owner of any account.
type ServiceAccountRow struct {
	UserID    string
	Kind      string
	Purpose   string
	OwnerID   string // "" when nobody answers for it
	MarkedBy  string
	MarkedAt  time.Time
	UpdatedAt time.Time
}

const serviceAccountCols = `user_id, kind, purpose, COALESCE(owner_id, ''), marked_by, marked_at, updated_at`

func scanServiceAccount(row pgx.Row) (ServiceAccountRow, error) {
	var r ServiceAccountRow
	err := row.Scan(&r.UserID, &r.Kind, &r.Purpose, &r.OwnerID, &r.MarkedBy, &r.MarkedAt, &r.UpdatedAt)
	return r, err
}

// ListServiceAccounts returns every stored row, in no particular order. The
// table is a handful of rows, so "is this one marked" is a lookup in this.
func ListServiceAccounts(ctx context.Context) ([]ServiceAccountRow, error) {
	rows, err := querier(ctx).Query(ctx, `SELECT `+serviceAccountCols+` FROM service_accounts`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ServiceAccountRow{}
	for rows.Next() {
		r, err := scanServiceAccount(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// UpsertServiceAccount marks a user or changes an existing row's kind, purpose
// and owner. A change keeps who marked it and when; only updated_at moves.
func UpsertServiceAccount(ctx context.Context, userID, kind, purpose, ownerID, actor string) (ServiceAccountRow, error) {
	return scanServiceAccount(querier(ctx).QueryRow(ctx, `
		INSERT INTO service_accounts (user_id, kind, purpose, owner_id, marked_by)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5)
		ON CONFLICT (user_id) DO UPDATE SET kind = EXCLUDED.kind,
			purpose = EXCLUDED.purpose, owner_id = EXCLUDED.owner_id, updated_at = now()
		RETURNING `+serviceAccountCols, userID, kind, purpose, ownerID, actor))
}

// DeleteServiceAccount removes the mark; false when there was none.
func DeleteServiceAccount(ctx context.Context, userID string) (bool, error) {
	tag, err := querier(ctx).Exec(ctx, `DELETE FROM service_accounts WHERE user_id = $1`, userID)
	return tag.RowsAffected() > 0, err
}
