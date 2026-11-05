package cache

import (
	"context"
	"errors"
	"fmt"
)

// refreshOne is the per-person recompile, a variable so the tests can count
// calls without a Redis.
var refreshOne = RefreshUser

// Refresh is the ONE call a write path makes after its commit: it recompiles
// the claims of every person the write touched, for the named projects (none
// named means every project they already have facts for, plus the ones they
// now hold roles in; see RefreshUser). The token path serves only what is
// stored, so a write that skips this leaves the old facts until the 12-hour
// warm. Callers pass the people the write affected and decide what to do with
// the error; one failed person never stops the rest.
//
// refresh_guard_test.go fails any listed write path that does not call it, and
// any new direct use of RefreshUser outside the cache package.
func Refresh(ctx context.Context, people []string, projects ...string) error {
	var errs []error
	seen := make(map[string]bool, len(people))
	for _, id := range people {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		if err := refreshOne(ctx, id, projects...); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", id, err))
		}
	}
	return errors.Join(errs...)
}
