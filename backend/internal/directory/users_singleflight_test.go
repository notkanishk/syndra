package directory

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"syndra/internal/zitadel"
)

// A cold cache hit by many readers at once (several Home blocks, several tabs)
// costs one rebuild: one ListUsers and one metadata read per user, not N of each.
func TestZitadelSource_Users_ConcurrentMissesRebuildOnce(t *testing.T) {
	mc := newMockClient()
	release := make(chan struct{})
	var lists atomic.Int32
	mc.listUsersFn = func(context.Context, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ZitadelUser], error) {
		lists.Add(1)
		<-release // hold the rebuild open so every reader arrives mid-flight
		return &zitadel.SearchResult[zitadel.ZitadelUser]{
			Items: []zitadel.ZitadelUser{{ID: "u1", DisplayName: "Alice"}, {ID: "u2", DisplayName: "Bob"}},
			Total: 2,
		}, nil
	}
	src := newZitadelSourceForTest(mc)

	const readers = 10
	var wg sync.WaitGroup
	errs := make(chan error, readers)
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			users, err := src.Users(context.Background())
			if err == nil && len(users) != 2 {
				err = errors.New("wrong user count")
			}
			errs <- err
		}()
	}
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("Users error: %v", err)
		}
	}
	if n := lists.Load(); n != 1 {
		t.Fatalf("ListUsers ran %d times for %d concurrent misses, want 1", n, readers)
	}
	for id, n := range mc.listMetadataCalls {
		if n != 1 {
			t.Fatalf("metadata for %s read %d times, want 1", id, n)
		}
	}
}
