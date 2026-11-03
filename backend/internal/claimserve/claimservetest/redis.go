// Package claimservetest is an in-memory Redis for claimserve.Redis, with the
// faults the data plane must survive: an error on every command (Redis down)
// and a hang until the caller's deadline (Redis stalled).
package claimservetest

import (
	"context"
	"fmt"
	"sync"

	"github.com/redis/go-redis/v9"
)

// Redis is safe for concurrent use.
type Redis struct {
	mu     sync.Mutex
	kv     map[string]string
	hashes map[string]map[string]string
	sets   map[string]map[string]bool
	err    error
	hang   bool
	Calls  map[string]int // command name -> count
}

func New() *Redis {
	return &Redis{kv: map[string]string{}, hashes: map[string]map[string]string{}, sets: map[string]map[string]bool{}, Calls: map[string]int{}}
}

// Fail makes every command answer err (nil restores).
func (r *Redis) Fail(err error) { r.mu.Lock(); r.err, r.hang = err, false; r.mu.Unlock() }

// Hang makes every command wait for its context to end, as a Redis that
// accepts and never answers does under a client that honours deadlines.
func (r *Redis) Hang() { r.mu.Lock(); r.hang, r.err = true, nil; r.mu.Unlock() }

// HSet sets one hash field directly (test setup; not a counted command).
func (r *Redis) HSet(key, field, value string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.hashes[key] == nil {
		r.hashes[key] = map[string]string{}
	}
	r.hashes[key][field] = value
}

// HDel drops one hash field directly.
func (r *Redis) HDel(key, field string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.hashes[key], field)
}

// Set sets one string key directly.
func (r *Redis) Set(key, value string) { r.mu.Lock(); r.kv[key] = value; r.mu.Unlock() }

// Value reads one string key directly.
func (r *Redis) Value(key string) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.kv[key]
	return v, ok
}

// Count is how many times cmd ran.
func (r *Redis) Count(cmd string) int { r.mu.Lock(); defer r.mu.Unlock(); return r.Calls[cmd] }

// begin counts cmd and applies the fault, if any. Called without r.mu.
func (r *Redis) begin(ctx context.Context, cmd string) error {
	r.mu.Lock()
	r.Calls[cmd]++
	err, hang := r.err, r.hang
	r.mu.Unlock()
	if hang {
		<-ctx.Done()
		return fmt.Errorf("i/o timeout: %w", ctx.Err())
	}
	return err
}

func (r *Redis) Get(ctx context.Context, key string) *redis.StringCmd {
	if err := r.begin(ctx, "GET"); err != nil {
		return redis.NewStringResult("", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.kv[key]
	if !ok {
		return redis.NewStringResult("", redis.Nil)
	}
	return redis.NewStringResult(v, nil)
}

func (r *Redis) HGet(ctx context.Context, key, field string) *redis.StringCmd {
	if err := r.begin(ctx, "HGET"); err != nil {
		return redis.NewStringResult("", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.hashes[key][field]
	if !ok {
		return redis.NewStringResult("", redis.Nil)
	}
	return redis.NewStringResult(v, nil)
}

func (r *Redis) SMembers(ctx context.Context, key string) *redis.StringSliceCmd {
	if err := r.begin(ctx, "SMEMBERS"); err != nil {
		return redis.NewStringSliceResult(nil, err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []string{}
	for m := range r.sets[key] {
		out = append(out, m)
	}
	return redis.NewStringSliceResult(out, nil)
}

func (r *Redis) MGet(ctx context.Context, keys ...string) *redis.SliceCmd {
	if err := r.begin(ctx, "MGET"); err != nil {
		return redis.NewSliceResult(nil, err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]any, len(keys))
	for i, k := range keys {
		if v, ok := r.kv[k]; ok {
			out[i] = v
		}
	}
	return redis.NewSliceResult(out, nil)
}

func (r *Redis) MSet(ctx context.Context, values ...any) *redis.StatusCmd {
	if err := r.begin(ctx, "MSET"); err != nil {
		return redis.NewStatusResult("", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := 0; i+1 < len(values); i += 2 {
		r.kv[fmt.Sprint(values[i])] = fmt.Sprint(values[i+1])
	}
	return redis.NewStatusResult("OK", nil)
}

func (r *Redis) SAdd(ctx context.Context, key string, members ...any) *redis.IntCmd {
	if err := r.begin(ctx, "SADD"); err != nil {
		return redis.NewIntResult(0, err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.sets[key] == nil {
		r.sets[key] = map[string]bool{}
	}
	for _, m := range members {
		r.sets[key][fmt.Sprint(m)] = true
	}
	return redis.NewIntResult(int64(len(members)), nil)
}

func (r *Redis) Incr(ctx context.Context, key string) *redis.IntCmd {
	if err := r.begin(ctx, "INCR"); err != nil {
		return redis.NewIntResult(0, err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	var n int64
	fmt.Sscan(r.kv[key], &n)
	n++
	r.kv[key] = fmt.Sprint(n)
	return redis.NewIntResult(n, nil)
}
