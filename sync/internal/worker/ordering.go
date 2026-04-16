package worker

import "sync"

// UIDLocker provides per-UID mutual exclusion so intents for the same user
// are processed sequentially, while different users proceed in parallel.
type UIDLocker struct {
	mu    sync.Mutex
	locks map[string]*uidEntry
}

type uidEntry struct {
	mu      sync.Mutex
	waiters int
}

// NewUIDLocker creates a new per-UID locker.
func NewUIDLocker() *UIDLocker {
	return &UIDLocker{locks: make(map[string]*uidEntry)}
}

// Lock acquires the lock for the given UID.
func (l *UIDLocker) Lock(uid string) {
	l.mu.Lock()
	e, ok := l.locks[uid]
	if !ok {
		e = &uidEntry{}
		l.locks[uid] = e
	}
	e.waiters++
	l.mu.Unlock()

	e.mu.Lock()
}

// Unlock releases the lock for the given UID. If no other goroutines are
// waiting, the entry is removed from the map to prevent unbounded growth.
func (l *UIDLocker) Unlock(uid string) {
	l.mu.Lock()
	e := l.locks[uid]
	e.waiters--
	if e.waiters == 0 {
		delete(l.locks, uid)
	}
	l.mu.Unlock()

	e.mu.Unlock()
}

// Len returns the number of UIDs with active locks (for testing).
func (l *UIDLocker) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.locks)
}
