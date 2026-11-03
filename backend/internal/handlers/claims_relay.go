package handlers

import (
	"context"
	"log"
	"time"

	"syndra/internal/claimserve"
	"syndra/internal/claimtel"
	"syndra/internal/services/periodic"
)

// The backend's end of the relay (claims-delivery-integrity S7). The claims
// container has no Postgres: its counters, its calls and its requests reach
// this process through Redis (claimserve/relay.go), and this process folds,
// reads and acts on them exactly as on its own.

// FoldClaimRelay writes the claims container's closed minutes into Postgres.
// Run every minute beside FlushClaimDelivery; a failed write is retried.
func FoldClaimRelay(ctx context.Context) error {
	rr := claimRelayRedis()
	if rr == nil {
		return nil
	}
	_, err := claimserve.FoldTelemetry(ctx, rr, time.Now(), dbWriteClaimDelivery)
	return err
}

const signalsEvery = time.Second

// RunClaimSignals acts on what the claims container asks for, every second:
// a person to recompile after a facts miss, the keyring to read again after
// a mismatch, a debug capture to explain and keep.
func RunClaimSignals(ctx context.Context) {
	t := time.NewTicker(signalsEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			drainClaimSignals(ctx)
		}
	}
}

func drainClaimSignals(ctx context.Context) int {
	defer periodic.Recover("CLAIM-SIGNALS")
	rr := claimRelayRedis()
	if rr == nil {
		return 0
	}
	dctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	sigs, err := claimserve.DrainSignals(dctx, rr, 100)
	if err != nil {
		return 0
	}
	for _, s := range sigs {
		switch s.Kind {
		case "repair":
			claimRepair.enqueue(s.UserID)
		case "nudge":
			nudgeKeyring()
		case "debug":
			if s.Capture != nil {
				claimDebug.Record(debugCapture(*s.Capture))
			}
		}
	}
	return len(sigs)
}

// claimCalls is what the correlator (and L2) match against: this process's
// calls and the claims container's, the worst per function of either.
type claimCalls struct{}

func (claimCalls) Window(userID, clientID string, from, to time.Time) (map[string]claimtel.Outcome, bool) {
	worst, overflowed := claimDelivery.Window(userID, clientID, from, to)
	rr := claimRelayRedis()
	if rr == nil {
		return worst, overflowed
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	remote, rover := claimserve.RemoteWindow(ctx, rr, userID, clientID, from, to)
	cancel()
	if len(remote) > 0 && worst == nil {
		worst = map[string]claimtel.Outcome{}
	}
	for fn, o := range remote {
		if cur, ok := worst[fn]; !ok || claimtel.Worse(o, cur) {
			worst[fn] = o
		}
	}
	return worst, overflowed || rover
}

// claimLastCalled is when either door last had a call that named a person.
func claimLastCalled(ctx context.Context) time.Time {
	last := claimDelivery.LastCalled()
	if rr := claimRelayRedis(); rr != nil {
		rctx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
		_, called, err := claimserve.RemoteLast(rctx, rr)
		cancel()
		if err == nil && called.After(last) {
			last = called
		}
	}
	return last
}

// publishDebugWatch tells the claims container whose sign-ins debug mode is
// capturing; it reloads within a second. A failed publish is logged: debug
// mode then captures only what this process's door answers.
func publishDebugWatch(ctx context.Context) {
	rdb := claimConfigRedis()
	if rdb == nil {
		return
	}
	watch := map[string]claimserve.DebugWatch{}
	for client, s := range claimDebug.Watching() {
		watch[client] = claimserve.DebugWatch{UserID: s.UserID, Ends: s.EndsAt.Unix()}
	}
	if err := claimserve.PublishDebug(ctx, rdb, watch); err != nil {
		log.Printf("[CLAIM-DEBUG] the claims container has not got this debug session: %v", err)
	}
}
