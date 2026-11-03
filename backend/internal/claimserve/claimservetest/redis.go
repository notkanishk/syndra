// Package claimservetest is an in-memory Redis for claimserve.Redis, with the
// faults the data plane must survive: an error on every command (Redis down)
// and a hang until the caller's deadline (Redis stalled).
package claimservetest

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis is safe for concurrent use.
type Redis struct {
	mu     sync.Mutex
	kv     map[string]string
	hashes map[string]map[string]string
	sets   map[string]map[string]bool
	lists  map[string][]string
	err    error
	hang   bool
	Calls  map[string]int // command name -> count
}

func New() *Redis {
	return &Redis{kv: map[string]string{}, hashes: map[string]map[string]string{}, sets: map[string]map[string]bool{}, lists: map[string][]string{}, Calls: map[string]int{}}
}

// Fail makes every command answer err (nil restores).
func (r *Redis) Fail(err error) { r.mu.Lock(); r.err, r.hang = err, false; r.mu.Unlock() }

// Hang makes every command wait for its context to end, as a Redis that
// accepts and never answers does under a client that honours deadlines.
func (r *Redis) Hang() { r.mu.Lock(); r.hang, r.err = true, nil; r.mu.Unlock() }

// SetField sets one hash field directly (test setup; not a counted command).
func (r *Redis) SetField(key, field, value string) {
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

// The relay's commands (claimserve.RelayRedis). TTLs are not kept: a test
// that needs expiry deletes the key.

func (r *Redis) HIncrBy(ctx context.Context, key, field string, incr int64) *redis.IntCmd {
	if err := r.begin(ctx, "HINCRBY"); err != nil {
		return redis.NewIntResult(0, err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.hashes[key] == nil {
		r.hashes[key] = map[string]string{}
	}
	var n int64
	fmt.Sscan(r.hashes[key][field], &n)
	n += incr
	r.hashes[key][field] = fmt.Sprint(n)
	return redis.NewIntResult(n, nil)
}

func (r *Redis) HSet(ctx context.Context, key string, values ...any) *redis.IntCmd {
	if err := r.begin(ctx, "HSET"); err != nil {
		return redis.NewIntResult(0, err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.hashes[key] == nil {
		r.hashes[key] = map[string]string{}
	}
	for i := 0; i+1 < len(values); i += 2 {
		r.hashes[key][fmt.Sprint(values[i])] = fmt.Sprint(values[i+1])
	}
	return redis.NewIntResult(int64(len(values)/2), nil)
}

func (r *Redis) HGetAll(ctx context.Context, key string) *redis.MapStringStringCmd {
	cmd := redis.NewMapStringStringCmd(ctx)
	if err := r.begin(ctx, "HGETALL"); err != nil {
		cmd.SetErr(err)
		return cmd
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := map[string]string{}
	for f, v := range r.hashes[key] {
		out[f] = v
	}
	cmd.SetVal(out)
	return cmd
}

func (r *Redis) Expire(ctx context.Context, key string, _ time.Duration) *redis.BoolCmd {
	if err := r.begin(ctx, "EXPIRE"); err != nil {
		return redis.NewBoolResult(false, err)
	}
	return redis.NewBoolResult(true, nil)
}

func (r *Redis) Rename(ctx context.Context, key, newkey string) *redis.StatusCmd {
	if err := r.begin(ctx, "RENAME"); err != nil {
		return redis.NewStatusResult("", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	h, ok := r.hashes[key]
	if !ok {
		return redis.NewStatusResult("", errors.New("ERR no such key"))
	}
	delete(r.hashes, key)
	r.hashes[newkey] = h
	return redis.NewStatusResult("OK", nil)
}

func (r *Redis) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	if err := r.begin(ctx, "DEL"); err != nil {
		return redis.NewIntResult(0, err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	var n int64
	for _, k := range keys {
		if _, ok := r.hashes[k]; ok {
			n++
		}
		if _, ok := r.kv[k]; ok {
			n++
		}
		if _, ok := r.lists[k]; ok {
			n++
		}
		delete(r.hashes, k)
		delete(r.kv, k)
		delete(r.lists, k)
	}
	return redis.NewIntResult(n, nil)
}

func (r *Redis) SRem(ctx context.Context, key string, members ...any) *redis.IntCmd {
	if err := r.begin(ctx, "SREM"); err != nil {
		return redis.NewIntResult(0, err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, m := range members {
		delete(r.sets[key], fmt.Sprint(m))
	}
	return redis.NewIntResult(int64(len(members)), nil)
}

func (r *Redis) RPush(ctx context.Context, key string, values ...any) *redis.IntCmd {
	if err := r.begin(ctx, "RPUSH"); err != nil {
		return redis.NewIntResult(0, err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, v := range values {
		r.lists[key] = append(r.lists[key], fmt.Sprint(v))
	}
	return redis.NewIntResult(int64(len(r.lists[key])), nil)
}

// LTrim supports the relay's use: a negative start keeps the last -start.
func (r *Redis) LTrim(ctx context.Context, key string, start, stop int64) *redis.StatusCmd {
	if err := r.begin(ctx, "LTRIM"); err != nil {
		return redis.NewStatusResult("", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	l := r.lists[key]
	if start < 0 && int64(len(l)) > -start {
		r.lists[key] = append([]string(nil), l[int64(len(l))+start:]...)
	}
	return redis.NewStatusResult("OK", nil)
}

func (r *Redis) LRange(ctx context.Context, key string, _, _ int64) *redis.StringSliceCmd {
	if err := r.begin(ctx, "LRANGE"); err != nil {
		return redis.NewStringSliceResult(nil, err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return redis.NewStringSliceResult(append([]string{}, r.lists[key]...), nil)
}

func (r *Redis) LPopCount(ctx context.Context, key string, count int) *redis.StringSliceCmd {
	if err := r.begin(ctx, "LPOP"); err != nil {
		return redis.NewStringSliceResult(nil, err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	l := r.lists[key]
	if len(l) == 0 {
		return redis.NewStringSliceResult(nil, redis.Nil)
	}
	if count > len(l) {
		count = len(l)
	}
	out := append([]string{}, l[:count]...)
	r.lists[key] = l[count:]
	return redis.NewStringSliceResult(out, nil)
}

func (r *Redis) Ping(ctx context.Context) *redis.StatusCmd {
	if err := r.begin(ctx, "PING"); err != nil {
		return redis.NewStatusResult("", err)
	}
	return redis.NewStatusResult("PONG", nil)
}

// List reads one list directly.
func (r *Redis) List(key string) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string{}, r.lists[key]...)
}

// Hash reads one hash directly.
func (r *Redis) Hash(key string) map[string]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := map[string]string{}
	for f, v := range r.hashes[key] {
		out[f] = v
	}
	return out
}
