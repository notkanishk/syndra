package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"syndra/internal/models"
)

// InsightsSnapshot is one stored day (Insights "As of a date"). Day is a civil
// date, YYYY-MM-DD, and travels as text so no time zone can move it.
type InsightsSnapshot struct {
	Day        string
	TakenAt    time.Time
	Topology   models.TopologyGraph
	RolePeople map[string]int // "project_id:role_key" -> people holding it
}

// InsightsSnapshotExists says whether `day` already has its row.
func InsightsSnapshotExists(ctx context.Context, day string) (bool, error) {
	var ok bool
	err := querier(ctx).QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM insights_snapshots WHERE day = $1::date)`, day).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("insights snapshot exists: %w", err)
	}
	return ok, nil
}

// InsertInsightsSnapshot stores a day once. A day that already has a row is left
// exactly as it was (false, nil): the first picture of a day is the one kept.
func InsertInsightsSnapshot(ctx context.Context, s InsightsSnapshot) (bool, error) {
	tag, err := querier(ctx).Exec(ctx, `
		INSERT INTO insights_snapshots (day, topology, role_people)
		VALUES ($1::date, $2, $3)
		ON CONFLICT (day) DO NOTHING`, s.Day, s.Topology, s.RolePeople)
	if err != nil {
		return false, fmt.Errorf("insert insights snapshot: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// InsightsSnapshotOnOrBefore is the latest snapshot whose day is <= `day`;
// (nil, nil) when there is none.
func InsightsSnapshotOnOrBefore(ctx context.Context, day string) (*InsightsSnapshot, error) {
	var s InsightsSnapshot
	err := querier(ctx).QueryRow(ctx, `
		SELECT day::text, taken_at, topology, role_people
		  FROM insights_snapshots
		 WHERE day <= $1::date
		 ORDER BY day DESC LIMIT 1`, day).
		Scan(&s.Day, &s.TakenAt, &s.Topology, &s.RolePeople)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("insights snapshot on or before %s: %w", day, err)
	}
	return &s, nil
}

// InsightsSnapshotBounds is the first and last stored day, nil when none exist.
func InsightsSnapshotBounds(ctx context.Context) (first, last *string, err error) {
	err = querier(ctx).QueryRow(ctx,
		`SELECT MIN(day)::text, MAX(day)::text FROM insights_snapshots`).Scan(&first, &last)
	if err != nil {
		return nil, nil, fmt.Errorf("insights snapshot bounds: %w", err)
	}
	return first, last, nil
}
