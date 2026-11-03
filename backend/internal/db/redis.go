package db

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/redis/go-redis/v9"

	"syndra/internal/claimserve"
)

var Redis *redis.Client

// ConnectRedis sets Redis without waiting for it to answer: the claims data
// plane starts on it first and serves degraded (counted) until it does, so a
// Redis that is late at boot is not a crash loop (assessment S15).
func ConnectRedis() {
	url := os.Getenv("REDIS_URL")
	if url == "" {
		url = "localhost:6379" // fallback
	}
	Redis = NewRedisClient(url)
}

// AwaitRedis blocks until Redis answers a PING or ctx ends, saying so every
// few seconds. The admin side needs Redis; the data plane does not wait.
func AwaitRedis(ctx context.Context) error {
	for i := 0; ; i++ {
		pctx, cancel := context.WithTimeout(ctx, time.Second)
		err := Redis.Ping(pctx).Err()
		cancel()
		if err == nil {
			fmt.Println("Connected to Redis successfully.")
			return nil
		}
		if i%5 == 0 {
			log.Printf("[STARTUP] waiting for Redis: %v", err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

// NewRedisClient is the one way a Redis client is built, so every caller gets
// deadlines that hold.
//
// Without ContextTimeoutEnabled, go-redis v9 ignores a context's deadline on
// socket reads and writes and waits out its 3s ReadTimeout instead. The data
// plane's 50ms per-read budget was therefore a no-op against a Redis that
// accepts connections and never answers (stopped process, AOF fsync stall,
// half-open TCP), and one stalled GET spent Zitadel's whole budget for the
// call (claims-delivery-integrity F-1). The socket timeouts below are the
// ceiling for callers that pass no deadline at all; Redis on the internal
// network answers in well under 1ms.
//
// The options live in claimserve so the claims process (cmd/claims) builds
// its client the same way without importing this package.
func NewRedisClient(addr string) *redis.Client { return claimserve.NewRedisClient(addr) }
