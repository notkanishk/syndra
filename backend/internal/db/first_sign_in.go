package db

import (
	"context"
	"fmt"
	"time"
)

// RecordFirstSignIn writes a person's first sign-in to Syndra once. first is
// true only for the call that wrote it; every later call reads the original
// time back and changes nothing. The sign-in route turns that single true into
// the session's first-visit flag, so the welcome is a fact about one session.
func RecordFirstSignIn(ctx context.Context, userID string) (at time.Time, first bool, err error) {
	err = querier(ctx).QueryRow(ctx, `
		WITH ins AS (
			INSERT INTO person_first_sign_in (user_id, first_sign_in_at) VALUES ($1, now())
			ON CONFLICT (user_id) DO NOTHING
			RETURNING first_sign_in_at
		)
		SELECT first_sign_in_at, true FROM ins
		UNION ALL
		SELECT first_sign_in_at, false FROM person_first_sign_in WHERE user_id = $1
		LIMIT 1`, userID).Scan(&at, &first)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("record first sign-in: %w", err)
	}
	return at, first, nil
}
