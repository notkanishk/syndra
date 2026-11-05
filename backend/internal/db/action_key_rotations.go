package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Syndra's record of replacing the Actions v2 signing keys
// (action-key-autorotation, migration 000077). No key is ever written here.

// actionKeyLockKey is the session-level advisory lock that makes a
// replacement single flight across processes and clicks.
const actionKeyLockKey int64 = 771234502

// TryActionKeyRotationLock takes the replacement lock, or says another
// replacement holds it. Released by the closure, or by Postgres if the
// holder's session dies.
func TryActionKeyRotationLock(ctx context.Context) (func(), bool, error) {
	return tryAdvisoryLock(ctx, actionKeyLockKey)
}

// ActionKeyRotation is one attempt.
type ActionKeyRotation struct {
	ID         int64           `json:"id"`
	Trigger    string          `json:"trigger"`
	Actor      string          `json:"actor"`
	Targets    []string        `json:"targets"`
	Replaced   []string        `json:"replaced"`
	StartedAt  time.Time       `json:"started_at"`
	FinishedAt *time.Time      `json:"finished_at,omitempty"`
	Outcome    string          `json:"outcome"`
	Probe      json.RawMessage `json:"probe,omitempty"`
	Error      string          `json:"error,omitempty"`
}

// ActionKeyRotationState is what the schedule and the status read: the latest
// attempt, when Syndra last replaced the claims key, and its last automatic
// attempt.
type ActionKeyRotationState struct {
	Last           *ActionKeyRotation
	LastReplacedAt *time.Time
	LastAutoAt     *time.Time
}

// AbandonActionKeyRotations finishes every attempt left in flight. Called
// only while holding the lock, so none of them is still running.
func AbandonActionKeyRotations(ctx context.Context) error {
	_, err := querier(ctx).Exec(ctx, `
		UPDATE action_key_rotations
		SET outcome = 'abandoned', finished_at = NOW(), error = COALESCE(error, 'interrupted before it finished')
		WHERE outcome = 'in_flight'`)
	if err != nil {
		return fmt.Errorf("abandon action key rotations: %w", err)
	}
	return nil
}

// StartActionKeyRotation writes the trace before any Zitadel call.
func StartActionKeyRotation(ctx context.Context, trigger, actor string, targets []string) (int64, error) {
	var id int64
	err := querier(ctx).QueryRow(ctx, `
		INSERT INTO action_key_rotations (trigger, actor, targets) VALUES ($1, $2, $3) RETURNING id`,
		trigger, actor, targets).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("start action key rotation: %w", err)
	}
	return id, nil
}

// MarkActionKeyReplaced records that Zitadel replaced target's key, the
// moment its answer arrives.
func MarkActionKeyReplaced(ctx context.Context, id int64, target string) error {
	_, err := querier(ctx).Exec(ctx, `
		UPDATE action_key_rotations SET replaced = array_append(replaced, $2)
		WHERE id = $1 AND NOT ($2 = ANY(replaced))`, id, target)
	if err != nil {
		return fmt.Errorf("mark action key replaced: %w", err)
	}
	return nil
}

// FinishActionKeyRotation records the outcome (and later, an unverified one
// confirmed by the probes).
func FinishActionKeyRotation(ctx context.Context, id int64, outcome string, probe any, errText string) error {
	var raw []byte
	if probe != nil {
		var err error
		if raw, err = json.Marshal(probe); err != nil {
			return fmt.Errorf("finish action key rotation: %w", err)
		}
	}
	_, err := querier(ctx).Exec(ctx, `
		UPDATE action_key_rotations
		SET outcome = $2, finished_at = COALESCE(finished_at, NOW()), probe = COALESCE($3, probe), error = NULLIF($4, '')
		WHERE id = $1`, id, outcome, raw, errText)
	if err != nil {
		return fmt.Errorf("finish action key rotation: %w", err)
	}
	return nil
}

// ActionKeyRotationNow reads the state; claimTarget names the key whose age
// it is.
func ActionKeyRotationNow(ctx context.Context, claimTarget string) (ActionKeyRotationState, error) {
	var st ActionKeyRotationState
	var r ActionKeyRotation
	var errText *string
	err := querier(ctx).QueryRow(ctx, `
		SELECT id, trigger, actor, targets, replaced, started_at, finished_at, outcome, probe, error
		FROM action_key_rotations ORDER BY started_at DESC, id DESC LIMIT 1`).
		Scan(&r.ID, &r.Trigger, &r.Actor, &r.Targets, &r.Replaced, &r.StartedAt, &r.FinishedAt, &r.Outcome, &r.Probe, &errText)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return st, nil
	case err != nil:
		return st, fmt.Errorf("read action key rotations: %w", err)
	}
	if errText != nil {
		r.Error = *errText
	}
	st.Last = &r
	// The replacement time is when Zitadel answered: the attempt's finish, or
	// its start while the row was never finished.
	if err := querier(ctx).QueryRow(ctx, `
		SELECT
			(SELECT COALESCE(finished_at, started_at) FROM action_key_rotations
			 WHERE $1 = ANY(replaced) ORDER BY started_at DESC LIMIT 1),
			(SELECT MAX(started_at) FROM action_key_rotations WHERE trigger = 'auto')`, claimTarget).
		Scan(&st.LastReplacedAt, &st.LastAutoAt); err != nil {
		return st, fmt.Errorf("read action key rotations: %w", err)
	}
	return st, nil
}
