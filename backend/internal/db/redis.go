package db

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

var Redis *redis.Client

func ConnectRedis() {
	url := os.Getenv("REDIS_URL")
	if url == "" {
		url = "localhost:6379" // fallback
	}

	client := NewRedisClient(url)

	_, err := client.Ping(context.Background()).Result()
	if err != nil {
		log.Fatalf("Unable to connect to Redis: %v", err)
	}

	Redis = client
	fmt.Println("Connected to Redis successfully.")
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
func NewRedisClient(addr string) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:                  addr,
		DB:                    0,
		ContextTimeoutEnabled: true,
		DialTimeout:           200 * time.Millisecond,
		ReadTimeout:           200 * time.Millisecond,
		WriteTimeout:          200 * time.Millisecond,
		PoolTimeout:           100 * time.Millisecond,
	})
}
