package worker

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestUIDLocker_DifferentUIDs_Concurrent(t *testing.T) {
	locker := NewUIDLocker()
	var running atomic.Int32
	var maxConcurrent atomic.Int32
	var wg sync.WaitGroup

	for _, uid := range []string{"a", "b", "c"} {
		wg.Add(1)
		go func(uid string) {
			defer wg.Done()
			locker.Lock(uid)
			defer locker.Unlock(uid)

			cur := running.Add(1)
			if cur > maxConcurrent.Load() {
				maxConcurrent.Store(cur)
			}
			time.Sleep(10 * time.Millisecond)
			running.Add(-1)
		}(uid)
	}

	wg.Wait()
	if maxConcurrent.Load() < 2 {
		t.Errorf("expected concurrent execution for different UIDs, max concurrent was %d", maxConcurrent.Load())
	}
}

func TestUIDLocker_SameUID_Sequential(t *testing.T) {
	locker := NewUIDLocker()
	var running atomic.Int32
	var maxConcurrent atomic.Int32
	var wg sync.WaitGroup

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			locker.Lock("same-uid")
			defer locker.Unlock("same-uid")

			cur := running.Add(1)
			if cur > maxConcurrent.Load() {
				maxConcurrent.Store(cur)
			}
			time.Sleep(10 * time.Millisecond)
			running.Add(-1)
		}()
	}

	wg.Wait()
	if maxConcurrent.Load() != 1 {
		t.Errorf("expected sequential execution for same UID, max concurrent was %d", maxConcurrent.Load())
	}
}

func TestUIDLocker_MapCleanup(t *testing.T) {
	locker := NewUIDLocker()

	locker.Lock("temp-uid")
	if locker.Len() != 1 {
		t.Fatalf("expected 1 entry, got %d", locker.Len())
	}
	locker.Unlock("temp-uid")

	if locker.Len() != 0 {
		t.Errorf("expected 0 entries after unlock, got %d", locker.Len())
	}
}
