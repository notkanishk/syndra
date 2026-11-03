package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"syndra/internal/claimtel"
)

// ApplyClaimFindings folds one detector pass into claim_findings, in one
// transaction. A failure upserts its row: an open row counts on, a settled one
// starts again (first seen now, count from this pass). Clean looks after the
// last failure add to the streak; at resolveAfter the row settles. A subject
// with only clean looks touches its row only while it is open.
func ApplyClaimFindings(ctx context.Context, ups []claimtel.FindingUpdate, resolveAfter int) error {
	if len(ups) == 0 {
		return nil
	}
	now := time.Now()
	batch := &pgx.Batch{}
	for _, u := range ups {
		if u.Fails == 0 {
			batch.Queue(`
				UPDATE claim_findings
				SET clean_streak = clean_streak + $3,
				    resolved_at = CASE WHEN clean_streak + $3 >= $4 THEN $5::timestamptz END
				WHERE kind = $1 AND subject = $2 AND resolved_at IS NULL`,
				u.Kind, u.Subject, u.TrailingClean, resolveAfter, now)
			continue
		}
		sample, err := json.Marshal(u.Sample)
		if err != nil || u.Sample == nil {
			sample = []byte(`{}`)
		}
		var resolved *time.Time
		if u.TrailingClean >= resolveAfter {
			resolved = &now
		}
		batch.Queue(`
			INSERT INTO claim_findings (kind, subject, first_seen, last_seen, count, sample, clean_streak, resolved_at)
			VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8)
			ON CONFLICT (kind, subject) DO UPDATE SET
			    first_seen = CASE WHEN claim_findings.resolved_at IS NULL THEN claim_findings.first_seen ELSE EXCLUDED.first_seen END,
			    count      = CASE WHEN claim_findings.resolved_at IS NULL THEN claim_findings.count + EXCLUDED.count ELSE EXCLUDED.count END,
			    last_seen  = GREATEST(claim_findings.last_seen, EXCLUDED.last_seen),
			    sample = EXCLUDED.sample, clean_streak = EXCLUDED.clean_streak, resolved_at = EXCLUDED.resolved_at`,
			u.Kind, u.Subject, u.FirstFail, u.LastFail, u.Fails, string(sample), u.TrailingClean, resolved)
	}
	tx, owned, err := beginOrJoin(ctx)
	if err != nil {
		return fmt.Errorf("claim findings: %w", err)
	}
	if owned {
		defer func() { _ = tx.Rollback(ctx) }()
	}
	if err := tx.SendBatch(ctx, batch).Close(); err != nil {
		return fmt.Errorf("claim findings: %w", err)
	}
	if owned {
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("claim findings: %w", err)
		}
	}
	return nil
}

// SettleUnseenClaimFindings settles the open findings of `kind` whose check
// ran (its subject's prefix before ':' is in checks) but which this run did
// not see at all: the thing they were about is gone (an app deleted, a shape
// removed), which is as clean as a read gets.
func SettleUnseenClaimFindings(ctx context.Context, kind string, checks, seen []string) error {
	_, err := querier(ctx).Exec(ctx, `
		UPDATE claim_findings SET resolved_at = now(), clean_streak = clean_streak + 1
		WHERE kind = $1 AND resolved_at IS NULL
		  AND split_part(subject, ':', 1) = ANY($2) AND NOT (subject = ANY($3))`,
		kind, checks, seen)
	if err != nil {
		return fmt.Errorf("settle claim findings: %w", err)
	}
	return nil
}

// ClaimFinding is one row of claim_findings.
type ClaimFinding struct {
	Kind        string         `json:"kind"`
	Subject     string         `json:"subject"`
	FirstSeen   time.Time      `json:"first_seen"`
	LastSeen    time.Time      `json:"last_seen"`
	Count       int64          `json:"count"`
	Sample      map[string]any `json:"sample"`
	CleanStreak int            `json:"clean_streak"`
	ResolvedAt  *time.Time     `json:"resolved_at"`
}

// ClaimFindings is every open finding and every one settled since
// `settledSince`, newest first.
func ClaimFindings(ctx context.Context, settledSince time.Time) ([]ClaimFinding, error) {
	rows, err := querier(ctx).Query(ctx, `
		SELECT kind, subject, first_seen, last_seen, count, sample, clean_streak, resolved_at
		FROM claim_findings
		WHERE resolved_at IS NULL OR resolved_at >= $1
		ORDER BY (resolved_at IS NULL) DESC, last_seen DESC, kind, subject`, settledSince)
	if err != nil {
		return nil, fmt.Errorf("claim findings: %w", err)
	}
	defer rows.Close()
	out := []ClaimFinding{}
	for rows.Next() {
		var f ClaimFinding
		var sample []byte
		if err := rows.Scan(&f.Kind, &f.Subject, &f.FirstSeen, &f.LastSeen, &f.Count, &sample, &f.CleanStreak, &f.ResolvedAt); err != nil {
			return nil, fmt.Errorf("claim findings: %w", err)
		}
		_ = json.Unmarshal(sample, &f.Sample)
		out = append(out, f)
	}
	return out, rows.Err()
}

// LastSignInPerApp is when each app (by application id) last had a sign-in.
func LastSignInPerApp(ctx context.Context) (map[string]time.Time, error) {
	rows, err := querier(ctx).Query(ctx, `
		SELECT app_id, max(last_at) FROM usage_last_seen
		WHERE kind = $1 AND app_id <> '' GROUP BY app_id`, UsageSignIn)
	if err != nil {
		return nil, fmt.Errorf("last sign-in per app: %w", err)
	}
	defer rows.Close()
	out := map[string]time.Time{}
	for rows.Next() {
		var id string
		var at time.Time
		if err := rows.Scan(&id, &at); err != nil {
			return nil, fmt.Errorf("last sign-in per app: %w", err)
		}
		out[id] = at
	}
	return out, rows.Err()
}
