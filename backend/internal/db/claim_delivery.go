package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"syndra/internal/claimtel"
)

// WriteClaimDelivery folds one telemetry flush into Postgres, in one
// transaction: the 5-minute counter rows (added to what the bucket already
// holds), each app's last call, and payload shapes not seen before (the first
// example of a shape is never overwritten).
func WriteClaimDelivery(ctx context.Context, b claimtel.Batch) error {
	batch := &pgx.Batch{}
	for k, c := range b.Counts {
		batch.Queue(`
			INSERT INTO claim_delivery_bucket (bucket, project_id, client_id, function, key_slot, outcome, n, buckets)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (bucket, project_id, client_id, function, key_slot, outcome) DO UPDATE
			SET n = claim_delivery_bucket.n + EXCLUDED.n,
			    buckets = claim_hist_add(claim_delivery_bucket.buckets, EXCLUDED.buckets)`,
			k.Bucket, k.ProjectID, k.ClientID, k.Function, k.KeySlot, string(k.Outcome), c.N, c.Hist[:])
	}
	for k, v := range b.Clients {
		var served *time.Time
		if !v.ServedAt.IsZero() {
			served = &v.ServedAt
		}
		batch.Queue(`
			INSERT INTO claim_delivery_last (client_id, function, last_at, last_outcome, last_served_at, client_known)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (client_id, function) DO UPDATE
			SET last_at = GREATEST(claim_delivery_last.last_at, EXCLUDED.last_at),
			    last_outcome = CASE WHEN EXCLUDED.last_at >= claim_delivery_last.last_at
			                        THEN EXCLUDED.last_outcome ELSE claim_delivery_last.last_outcome END,
			    client_known = CASE WHEN EXCLUDED.last_at >= claim_delivery_last.last_at
			                        THEN EXCLUDED.client_known ELSE claim_delivery_last.client_known END,
			    last_served_at = GREATEST(claim_delivery_last.last_served_at, EXCLUDED.last_served_at)`,
			k.ClientID, k.Function, v.At, string(v.Outcome), served, v.ClientKnown)
	}
	for _, f := range b.Fingerprints {
		var example any
		if f.Example != nil {
			example = string(f.Example)
		}
		batch.Queue(`
			INSERT INTO claim_payload_fingerprints (fp, function, paths, example, first_seen)
			VALUES ($1, $2, $3, $4::jsonb, $5)
			ON CONFLICT (fp) DO NOTHING`,
			f.FP, f.Function, f.Paths, example, f.At)
	}

	tx, owned, err := beginOrJoin(ctx)
	if err != nil {
		return fmt.Errorf("claim delivery flush: %w", err)
	}
	if owned {
		defer func() { _ = tx.Rollback(ctx) }()
	}
	if err := tx.SendBatch(ctx, batch).Close(); err != nil {
		return fmt.Errorf("claim delivery flush: %w", err)
	}
	if owned {
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("claim delivery flush: %w", err)
		}
	}
	return nil
}

// ClaimDeliveryHourKeep is how long hourly rows are kept.
const ClaimDeliveryHourKeep = 90 * 24 * time.Hour

// claimDeliveryRollAfter is when a 5-minute row is rolled into its hour.
const claimDeliveryRollAfter = 48 * time.Hour

// RollClaimDelivery moves 5-minute rows older than 48h into hourly rows, in
// one statement so a row is never counted twice or lost, then drops hourly
// rows past 90 days. Returns the 5-minute rows rolled.
func RollClaimDelivery(ctx context.Context, now time.Time) (int64, error) {
	tag, err := querier(ctx).Exec(ctx, `
		WITH moved AS (
			DELETE FROM claim_delivery_bucket WHERE bucket < $1
			RETURNING bucket, project_id, client_id, function, key_slot, outcome, n, buckets
		)
		INSERT INTO claim_delivery_hour (hour, project_id, client_id, function, key_slot, outcome, n, buckets)
		SELECT date_trunc('hour', bucket, 'UTC'), project_id, client_id, function, key_slot, outcome,
		       sum(n), claim_hist_sum(buckets)
		FROM moved
		GROUP BY 1, 2, 3, 4, 5, 6
		ON CONFLICT (hour, project_id, client_id, function, key_slot, outcome) DO UPDATE
		SET n = claim_delivery_hour.n + EXCLUDED.n,
		    buckets = claim_hist_add(claim_delivery_hour.buckets, EXCLUDED.buckets)`,
		now.Add(-claimDeliveryRollAfter))
	if err != nil {
		return 0, fmt.Errorf("roll claim delivery: %w", err)
	}
	if _, err := querier(ctx).Exec(ctx, `DELETE FROM claim_delivery_hour WHERE hour < $1`, now.Add(-ClaimDeliveryHourKeep)); err != nil {
		return 0, fmt.Errorf("prune claim delivery: %w", err)
	}
	return tag.RowsAffected(), nil
}

// ClaimDeliveryRow is one dimension set's calls over a window.
type ClaimDeliveryRow struct {
	ProjectID string           `json:"project_id"`
	ClientID  string           `json:"client_id"`
	Function  string           `json:"function"`
	KeySlot   string           `json:"key_slot"`
	Outcome   claimtel.Outcome `json:"outcome"`
	N         int64            `json:"n"`
	Hist      []int64          `json:"latency_buckets"`
}

// ClaimDeliveryWindow is every dimension set's calls since `since`, from the
// 5-minute rows and, past 48h, the hourly ones (a row lives in exactly one).
func ClaimDeliveryWindow(ctx context.Context, since time.Time) ([]ClaimDeliveryRow, error) {
	rows, err := querier(ctx).Query(ctx, `
		SELECT project_id, client_id, function, key_slot, outcome, sum(n)::bigint, claim_hist_sum(buckets)
		FROM (
			SELECT project_id, client_id, function, key_slot, outcome, n, buckets
			FROM claim_delivery_bucket WHERE bucket >= $1
			UNION ALL
			SELECT project_id, client_id, function, key_slot, outcome, n, buckets
			FROM claim_delivery_hour WHERE hour >= date_trunc('hour', $1::timestamptz, 'UTC')
		) w
		GROUP BY 1, 2, 3, 4, 5
		ORDER BY 2, 3, 5`, since)
	if err != nil {
		return nil, fmt.Errorf("claim delivery window: %w", err)
	}
	defer rows.Close()
	out := []ClaimDeliveryRow{}
	for rows.Next() {
		var r ClaimDeliveryRow
		var outcome string
		if err := rows.Scan(&r.ProjectID, &r.ClientID, &r.Function, &r.KeySlot, &outcome, &r.N, &r.Hist); err != nil {
			return nil, fmt.Errorf("claim delivery window: %w", err)
		}
		r.Outcome = claimtel.Outcome(outcome)
		out = append(out, r)
	}
	return out, rows.Err()
}

// ClaimDeliveryLastRow is claim_delivery_last.
type ClaimDeliveryLastRow struct {
	ClientID     string     `json:"client_id"`
	Function     string     `json:"function"`
	LastAt       time.Time  `json:"last_at"`
	LastOutcome  string     `json:"last_outcome"`
	LastServedAt *time.Time `json:"last_served_at"`
	ClientKnown  bool       `json:"client_known"`
}

// ClaimDeliveryLast is every app's last call, per function.
func ClaimDeliveryLast(ctx context.Context) ([]ClaimDeliveryLastRow, error) {
	rows, err := querier(ctx).Query(ctx, `
		SELECT client_id, function, last_at, last_outcome, last_served_at, client_known
		FROM claim_delivery_last ORDER BY client_id, function`)
	if err != nil {
		return nil, fmt.Errorf("claim delivery last: %w", err)
	}
	defer rows.Close()
	out := []ClaimDeliveryLastRow{}
	for rows.Next() {
		var r ClaimDeliveryLastRow
		if err := rows.Scan(&r.ClientID, &r.Function, &r.LastAt, &r.LastOutcome, &r.LastServedAt, &r.ClientKnown); err != nil {
			return nil, fmt.Errorf("claim delivery last: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ClaimPayloadFingerprint is one payload shape Zitadel has sent. Changed is
// true for every shape after the first one seen for its function: Zitadel's
// payload changed shape.
type ClaimPayloadFingerprint struct {
	FP        string    `json:"fp"`
	Function  string    `json:"function"`
	Paths     []string  `json:"paths"`
	FirstSeen time.Time `json:"first_seen"`
	Changed   bool      `json:"changed"`
}

// ClaimPayloadFingerprints lists the shapes, oldest first. The example bodies
// stay in the table: they are fixtures, read by hand, not served.
func ClaimPayloadFingerprints(ctx context.Context) ([]ClaimPayloadFingerprint, error) {
	rows, err := querier(ctx).Query(ctx, `
		SELECT fp, function, paths, first_seen,
		       first_seen > min(first_seen) OVER (PARTITION BY function)
		FROM claim_payload_fingerprints ORDER BY first_seen, fp`)
	if err != nil {
		return nil, fmt.Errorf("claim payload fingerprints: %w", err)
	}
	defer rows.Close()
	out := []ClaimPayloadFingerprint{}
	for rows.Next() {
		var f ClaimPayloadFingerprint
		if err := rows.Scan(&f.FP, &f.Function, &f.Paths, &f.FirstSeen, &f.Changed); err != nil {
			return nil, fmt.Errorf("claim payload fingerprints: %w", err)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}
