package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// SignIn is what one Syndra sign-in leaves behind for the person.
type SignIn struct {
	FirstAt time.Time
	// Previous is when they were last here before this sign-in; nil on the first.
	Previous *time.Time
	// First is true only for the call that wrote the person's row.
	First bool
}

// A second call within this long of the last is the same sign-in (a retried
// callback), so it moves nothing.
const signInSettle = "1 minute"

// RecordSignIn writes a sign-in to Syndra. The first one creates the row; every
// later one moves the current time into previous and writes now into current
// (A-20 P18). The sign-in route turns First into the session's first-visit flag
// and Previous into "since Tuesday", so both are facts about one session.
func RecordSignIn(ctx context.Context, userID string) (SignIn, error) {
	q := querier(ctx)
	var out SignIn
	err := q.QueryRow(ctx, `
		INSERT INTO person_first_sign_in (user_id, first_sign_in_at, last_sign_in_at) VALUES ($1, now(), now())
		ON CONFLICT (user_id) DO NOTHING
		RETURNING first_sign_in_at`, userID).Scan(&out.FirstAt)
	if err == nil {
		out.First = true
		return out, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return SignIn{}, fmt.Errorf("record sign-in: %w", err)
	}
	err = q.QueryRow(ctx, `
		UPDATE person_first_sign_in SET
			previous_sign_in_at = CASE WHEN last_sign_in_at < now() - interval '`+signInSettle+`' THEN last_sign_in_at ELSE previous_sign_in_at END,
			last_sign_in_at     = CASE WHEN last_sign_in_at < now() - interval '`+signInSettle+`' THEN now() ELSE last_sign_in_at END
		WHERE user_id = $1
		RETURNING first_sign_in_at, previous_sign_in_at`, userID).Scan(&out.FirstAt, &out.Previous)
	if err != nil {
		return SignIn{}, fmt.Errorf("record sign-in: %w", err)
	}
	return out, nil
}
