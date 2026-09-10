package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// What Syndra asked Zitadel, and what Zitadel answered.
//
// The distinction this file exists to keep is between a store of OBSERVATIONS
// and a cache. A cache holds whatever it was last told and cannot say when, by
// whom, or whether the telling was complete. This holds answers, each dated,
// each knowing whether it covered everything it claimed to.

// ErrNoObservation is returned when nothing has been observed for a scope yet.
// Distinct from an empty answer: "nobody holds anything" and "nobody has
// looked" are different facts, and only one of them permits a conclusion.
var ErrNoObservation = errors.New("nothing has been observed for this scope yet")

// Observation is one completed read of Zitadel.
type Observation struct {
	Scope      string    `json:"scope"`
	SubjectID  string    `json:"subject_id,omitempty"`
	ObservedAt time.Time `json:"observed_at"`
	// Complete says the listing finished rather than stopping at its cap or
	// part-way through an error. Presence can be concluded from any
	// observation; ABSENCE only from a complete one.
	Complete   bool   `json:"complete"`
	GrantsSeen int    `json:"grants_seen"`
	Error      string `json:"error,omitempty"`
}

// ObservedGrant is one grant as Zitadel reported it.
type ObservedGrant struct {
	GrantID   string
	UserID    string
	ProjectID string
	RoleKeys  []string
}

// RecordOrgObservation replaces the whole store with what a sweep saw.
//
// THE SAFETY PROPERTY OF THIS FILE: only a COMPLETE listing may delete
// anything. A truncated or failed read is honest about what it saw and silent
// about the rest, so deleting the rows it did not cover would manufacture
// absences — and an absence is what makes Syndra revoke access, report drift,
// and tell an operator somebody has lost something. An incomplete pass
// therefore refreshes what it saw and removes nothing.
//
// The complete case replaces in ONE transaction, so no reader ever sees a
// half-swapped world in which somebody appears to hold nothing.
func RecordOrgObservation(ctx context.Context, grants []ObservedGrant, complete bool, readErr string) error {
	tx, err := beginTx(ctx)
	if err != nil {
		return fmt.Errorf("record observation: %w", err)
	}
	defer tx.Rollback(ctx)

	if complete {
		if _, err := tx.Exec(ctx, `DELETE FROM zitadel_grants_index`); err != nil {
			return fmt.Errorf("record observation: clear the store: %w", err)
		}
	}
	if err := upsertObserved(ctx, tx, grants); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO zitadel_observations (scope, subject_id, complete, grants_seen, error)
		VALUES ('org', NULL, $1, $2, NULLIF($3,''))`,
		complete, len(grants), readErr); err != nil {
		return fmt.Errorf("record observation: %w", err)
	}
	return tx.Commit(ctx)
}

// RecordUserObservation is the same act at one person's scope.
//
// A complete read of ONE person may delete only that person's rows — never
// anybody else's, whatever the listing happened to contain.
func RecordUserObservation(ctx context.Context, userID string, grants []ObservedGrant, complete bool, readErr string) error {
	tx, err := beginTx(ctx)
	if err != nil {
		return fmt.Errorf("record observation: %w", err)
	}
	defer tx.Rollback(ctx)

	if complete {
		if _, err := tx.Exec(ctx,
			`DELETE FROM zitadel_grants_index WHERE user_id = $1`, userID); err != nil {
			return fmt.Errorf("record observation: clear this person: %w", err)
		}
	}
	for _, g := range grants {
		if g.UserID != userID {
			// A read scoped to one person may not write rows about another.
			// Silently accepting them would let a mis-scoped call corrupt the
			// store for somebody nobody was asking about.
			return fmt.Errorf("record observation: a read of %s returned a grant for %s", userID, g.UserID)
		}
	}
	if err := upsertObserved(ctx, tx, grants); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO zitadel_observations (scope, subject_id, complete, grants_seen, error)
		VALUES ('user', $1, $2, $3, NULLIF($4,''))`,
		userID, complete, len(grants), readErr); err != nil {
		return fmt.Errorf("record observation: %w", err)
	}
	return tx.Commit(ctx)
}

func upsertObserved(ctx context.Context, tx pgx.Tx, grants []ObservedGrant) error {
	const q = `
		INSERT INTO zitadel_grants_index (grant_id, user_id, project_id, role_keys, observed_at)
		VALUES ($1,$2,$3,$4,NOW())
		ON CONFLICT (grant_id) DO UPDATE SET
			user_id = EXCLUDED.user_id, project_id = EXCLUDED.project_id,
			role_keys = EXCLUDED.role_keys, observed_at = NOW(), updated_at = NOW()`
	for _, g := range grants {
		if g.GrantID == "" || g.UserID == "" || g.ProjectID == "" {
			continue
		}
		if _, err := tx.Exec(ctx, q, g.GrantID, g.UserID, g.ProjectID, g.RoleKeys); err != nil {
			return fmt.Errorf("record observation: %s: %w", g.GrantID, err)
		}
	}
	return nil
}

// LatestObservation returns the most recent observation covering a person.
//
// An org sweep covers everybody, so a person's own read and the last complete
// sweep are both candidates and the NEWER wins. Returns ErrNoObservation when
// neither exists, because "nobody has looked" must never be readable as
// "nothing is there".
func LatestObservation(ctx context.Context, userID string) (Observation, error) {
	var o Observation
	var subject *string
	var readErr *string
	err := querier(ctx).QueryRow(ctx, `
		SELECT scope, subject_id, observed_at, complete, grants_seen, error
		  FROM zitadel_observations
		 WHERE scope = 'org' OR (scope = 'user' AND subject_id = $1)
		 ORDER BY observed_at DESC
		 LIMIT 1`, userID).Scan(&o.Scope, &subject, &o.ObservedAt, &o.Complete, &o.GrantsSeen, &readErr)
	if errors.Is(err, pgx.ErrNoRows) {
		return Observation{}, ErrNoObservation
	}
	if err != nil {
		return Observation{}, fmt.Errorf("read latest observation: %w", err)
	}
	if subject != nil {
		o.SubjectID = *subject
	}
	if readErr != nil {
		o.Error = *readErr
	}
	return o, nil
}

// LatestOrgObservation is the sweep's own record, for surfaces that state how
// current the whole picture is.
func LatestOrgObservation(ctx context.Context) (Observation, error) {
	var o Observation
	var readErr *string
	err := querier(ctx).QueryRow(ctx, `
		SELECT observed_at, complete, grants_seen, error
		  FROM zitadel_observations
		 WHERE scope = 'org'
		 ORDER BY observed_at DESC
		 LIMIT 1`).Scan(&o.ObservedAt, &o.Complete, &o.GrantsSeen, &readErr)
	if errors.Is(err, pgx.ErrNoRows) {
		return Observation{}, ErrNoObservation
	}
	if err != nil {
		return Observation{}, fmt.Errorf("read latest sweep: %w", err)
	}
	o.Scope = "org"
	if readErr != nil {
		o.Error = *readErr
	}
	return o, nil
}

// ObservedGrantsFor returns what the store holds for one person.
func ObservedGrantsFor(ctx context.Context, userID string) ([]ObservedGrant, error) {
	rows, err := querier(ctx).Query(ctx, `
		SELECT grant_id, user_id, project_id, role_keys
		  FROM zitadel_grants_index WHERE user_id = $1`, userID)
	if err != nil {
		return nil, fmt.Errorf("read observed grants: %w", err)
	}
	defer rows.Close()
	var out []ObservedGrant
	for rows.Next() {
		var g ObservedGrant
		if err := rows.Scan(&g.GrantID, &g.UserID, &g.ProjectID, &g.RoleKeys); err != nil {
			return nil, fmt.Errorf("read observed grants: %w", err)
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// ObservedGrantsPage returns one page of the whole store, ordered by grant ID
// for a stable page boundary, plus the total row count — for the org-wide
// surface that used to page a live Zitadel listing directly and now pages
// what the observer already recorded.
func ObservedGrantsPage(ctx context.Context, limit, offset int) ([]ObservedGrant, int, error) {
	var total int
	if err := querier(ctx).QueryRow(ctx, `SELECT COUNT(*) FROM zitadel_grants_index`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count observed grants: %w", err)
	}
	rows, err := querier(ctx).Query(ctx, `
		SELECT grant_id, user_id, project_id, role_keys
		  FROM zitadel_grants_index ORDER BY grant_id LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("read observed grants page: %w", err)
	}
	defer rows.Close()
	var out []ObservedGrant
	for rows.Next() {
		var g ObservedGrant
		if err := rows.Scan(&g.GrantID, &g.UserID, &g.ProjectID, &g.RoleKeys); err != nil {
			return nil, 0, fmt.Errorf("read observed grants page: %w", err)
		}
		out = append(out, g)
	}
	return out, total, rows.Err()
}
