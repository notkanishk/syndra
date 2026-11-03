package db

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"syndra/internal/claimtel"
)

func truncateClaimDelivery(t *testing.T, ctx context.Context) {
	t.Helper()
	if _, err := PG.Exec(ctx, `TRUNCATE claim_delivery_bucket, claim_delivery_hour, claim_delivery_last, claim_payload_fingerprints`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

func hist(at int, n int64) [claimtel.HistBuckets]int64 {
	var h [claimtel.HistBuckets]int64
	h[at] = n
	return h
}

// Two flushes into one bucket add up, histogram included; the last call keeps
// the newest outcome and the newest served time; a shape's first example is
// never overwritten. Then the nightly roll moves rows older than 48h into
// their hour, once, and the window reads across both tables.
func TestClaimDelivery_FlushRollWindow(t *testing.T) {
	ctx := liveDB(t)
	truncateClaimDelivery(t, ctx)
	t.Cleanup(func() { truncateClaimDelivery(t, ctx) })

	now := time.Now().UTC()
	old := now.Add(-72 * time.Hour).Truncate(claimtel.BucketWidth)
	recent := now.Add(-time.Hour).Truncate(claimtel.BucketWidth)
	key := func(b time.Time, o claimtel.Outcome) claimtel.Key {
		return claimtel.Key{Bucket: b, ProjectID: "p1", ClientID: "c1", Function: "function/preaccesstoken", KeySlot: "current", Outcome: o}
	}
	first := claimtel.Batch{
		Counts: map[claimtel.Key]claimtel.Count{
			key(old, claimtel.Served):    {N: 2, Hist: hist(0, 2)},
			key(recent, claimtel.Served): {N: 3, Hist: hist(1, 3)},
		},
		Clients: map[claimtel.ClientKey]claimtel.ClientLast{
			{ClientID: "c1", Function: "function/preaccesstoken"}: {At: recent, Outcome: claimtel.Served, ServedAt: recent, ClientKnown: true},
		},
		Fingerprints: []claimtel.FingerprintSeen{{FP: "fp1", Function: "function/preaccesstoken", Paths: []string{"function"}, Example: json.RawMessage(`{"function":"x"}`), At: old}},
	}
	second := claimtel.Batch{
		Counts: map[claimtel.Key]claimtel.Count{
			key(recent, claimtel.Served):          {N: 1, Hist: hist(1, 1)},
			key(recent, claimtel.DegradedNoFacts): {N: 1, Hist: hist(11, 1)},
		},
		Clients: map[claimtel.ClientKey]claimtel.ClientLast{
			{ClientID: "c1", Function: "function/preaccesstoken"}: {At: recent.Add(time.Minute), Outcome: claimtel.DegradedNoFacts},
		},
		Fingerprints: []claimtel.FingerprintSeen{
			{FP: "fp1", Function: "function/preaccesstoken", Paths: []string{"function"}, Example: json.RawMessage(`{"overwritten":true}`), At: now},
			{FP: "fp2", Function: "function/preaccesstoken", Paths: []string{"function", "user"}, At: now},
		},
	}
	for _, b := range []claimtel.Batch{first, second} {
		if err := WriteClaimDelivery(ctx, b); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	var n int64
	var h []int64
	if err := PG.QueryRow(ctx, `SELECT n, buckets FROM claim_delivery_bucket WHERE bucket = $1 AND outcome = 'served'`, recent).Scan(&n, &h); err != nil {
		t.Fatal(err)
	}
	if n != 4 || h[1] != 4 || len(h) != claimtel.HistBuckets {
		t.Fatalf("merged bucket n=%d hist=%v", n, h)
	}
	last, err := ClaimDeliveryLast(ctx)
	if err != nil || len(last) != 1 || last[0].LastOutcome != "degraded_no_facts" || last[0].LastServedAt == nil || !last[0].LastServedAt.Equal(recent) {
		t.Fatalf("last = %+v, %v", last, err)
	}
	var example string
	if err := PG.QueryRow(ctx, `SELECT example::text FROM claim_payload_fingerprints WHERE fp = 'fp1'`).Scan(&example); err != nil || strings.Contains(example, "overwritten") {
		t.Fatalf("first example overwritten: %s %v", example, err)
	}
	fps, err := ClaimPayloadFingerprints(ctx)
	if err != nil || len(fps) != 2 || fps[0].Changed || !fps[1].Changed {
		t.Fatalf("fingerprints = %+v, %v", fps, err)
	}

	rolled, err := RollClaimDelivery(ctx, now)
	if err != nil || rolled != 1 {
		t.Fatalf("roll: %d rows, %v", rolled, err)
	}
	if again, err := RollClaimDelivery(ctx, now); err != nil || again != 0 {
		t.Fatalf("a second roll moved %d rows (%v): counted twice", again, err)
	}
	var hourN int64
	var hourHist []int64
	if err := PG.QueryRow(ctx, `SELECT n, buckets FROM claim_delivery_hour`).Scan(&hourN, &hourHist); err != nil || hourN != 2 || hourHist[0] != 2 {
		t.Fatalf("hour row n=%d hist=%v err=%v", hourN, hourHist, err)
	}

	rows, err := ClaimDeliveryWindow(ctx, now.Add(-7*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	got := map[claimtel.Outcome]int64{}
	for _, r := range rows {
		got[r.Outcome] += r.N
	}
	if got[claimtel.Served] != 6 || got[claimtel.DegradedNoFacts] != 1 {
		t.Fatalf("window = %v", got)
	}
	if rows, _ := ClaimDeliveryWindow(ctx, now.Add(-2*time.Hour)); len(rows) != 2 {
		t.Fatalf("a 2h window read %d rows, want only the recent bucket's two", len(rows))
	}

	// Hourly rows past 90 days go.
	if _, err := PG.Exec(ctx, `UPDATE claim_delivery_hour SET hour = hour - interval '100 days'`); err != nil {
		t.Fatal(err)
	}
	if _, err := RollClaimDelivery(ctx, now); err != nil {
		t.Fatal(err)
	}
	var left int
	_ = PG.QueryRow(ctx, `SELECT count(*) FROM claim_delivery_hour`).Scan(&left)
	if left != 0 {
		t.Fatalf("%d hourly rows past 90 days survived", left)
	}
}

// The migration pair is whole: the down file drops everything the up creates.
func TestMigration000072_DownUndoesUp(t *testing.T) {
	up, err := os.ReadFile("../../db/migrations/000072_claim_delivery.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile("../../db/migrations/000072_claim_delivery.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, obj := range []string{"claim_delivery_bucket", "claim_delivery_hour", "claim_delivery_last", "claim_payload_fingerprints", "claim_hist_sum", "claim_hist_add"} {
		if !strings.Contains(string(up), obj) || !strings.Contains(string(down), obj) {
			t.Errorf("%s is not in both halves of 000072", obj)
		}
	}
}
