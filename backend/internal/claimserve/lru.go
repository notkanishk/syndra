package claimserve

import (
	"container/list"
	"sync"
	"time"

	"syndra/internal/claims"
)

// lru is the last-known-good copy: the facts last read from Redis for each
// (person, project), and when. Bounded; the least recently served goes first.
// ponytail: one mutex; ~50 calls/s at the design peak is nowhere near
// contention. Shard it if a profile ever says otherwise.
type lru struct {
	mu    sync.Mutex
	cap   int
	order *list.List // front = most recent
	items map[string]*list.Element
}

type lkgEntry struct {
	key   string
	facts claims.Facts
	at    time.Time
}

func newLRU(capacity int) *lru {
	return &lru{cap: capacity, order: list.New(), items: make(map[string]*list.Element, capacity)}
}

func (c *lru) put(key string, f claims.Facts, at time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.items[key]; ok {
		e.Value = lkgEntry{key, f, at}
		c.order.MoveToFront(e)
		return
	}
	c.items[key] = c.order.PushFront(lkgEntry{key, f, at})
	if c.order.Len() > c.cap {
		last := c.order.Back()
		c.order.Remove(last)
		delete(c.items, last.Value.(lkgEntry).key)
	}
}

func (c *lru) get(key string) (claims.Facts, time.Time, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.items[key]
	if !ok {
		return claims.Facts{}, time.Time{}, false
	}
	c.order.MoveToFront(e)
	v := e.Value.(lkgEntry)
	return v.facts, v.at, true
}

func (c *lru) drop(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.items[key]; ok {
		c.order.Remove(e)
		delete(c.items, key)
	}
}

func (c *lru) len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}
