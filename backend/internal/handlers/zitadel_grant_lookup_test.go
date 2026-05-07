package handlers

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"mkauth/internal/zitadel"
)

// stubListUserGrants installs a deterministic listUserGrants seam for tests
// and restores the previous value via t.Cleanup. The stub serves the full
// items slice through pagination, honoring the Limit/Offset the caller
// passes — so tests can verify that pagination actually walks pages.
func stubListUserGrants(t *testing.T, items []zitadel.UserGrant, err error) {
	t.Helper()
	prev := zitadelListUserGrants
	t.Cleanup(func() { zitadelListUserGrants = prev })
	zitadelListUserGrants = func(_ context.Context, _ string, p zitadel.SearchParams) (*zitadel.SearchResult[zitadel.UserGrant], error) {
		if err != nil {
			return nil, err
		}
		start := p.Offset
		if start > len(items) {
			start = len(items)
		}
		end := start + p.Limit
		if end > len(items) {
			end = len(items)
		}
		return &zitadel.SearchResult[zitadel.UserGrant]{Items: items[start:end], Total: len(items)}, nil
	}
}

func TestListUserGrantsViaZitadel_FindsByGrantID(t *testing.T) {
	stubListUserGrants(t, []zitadel.UserGrant{
		{ID: "g-other", UserID: "u1", ProjectID: "p1", RoleKeys: []string{"a"}},
		{ID: "g-target", UserID: "u1", ProjectID: "p2", RoleKeys: []string{"b", "c"}},
	}, nil)

	got, err := listUserGrantsViaZitadel(context.Background(), "u1", "g-target")
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if got.ProjectID != "p2" {
		t.Errorf("ProjectID = %q, want p2", got.ProjectID)
	}
	if len(got.RoleKeys) != 2 {
		t.Errorf("RoleKeys = %v", got.RoleKeys)
	}
}

func TestListUserGrantsViaZitadel_GrantNotFound(t *testing.T) {
	stubListUserGrants(t, nil, nil)
	_, err := listUserGrantsViaZitadel(context.Background(), "u1", "g-missing")
	if err == nil {
		t.Fatal("err = nil, want grant-not-found error")
	}
}

func TestListUserGrantsViaZitadel_ZitadelError(t *testing.T) {
	stubListUserGrants(t, nil, errors.New("zitadel down"))
	_, err := listUserGrantsViaZitadel(context.Background(), "u1", "g-1")
	if err == nil {
		t.Fatal("err = nil, want propagated zitadel error")
	}
}

func TestListUserGrantsViaZitadel_RejectsBlankInputs(t *testing.T) {
	if _, err := listUserGrantsViaZitadel(context.Background(), "", "g"); err == nil {
		t.Error("expected error for empty user_id")
	}
	if _, err := listUserGrantsViaZitadel(context.Background(), "u", ""); err == nil {
		t.Error("expected error for empty grant_id")
	}
}

func TestListUserGrantsViaZitadel_PaginatesUntilFound(t *testing.T) {
	// Build a list larger than DefaultSearchLimit so finding a grant on a
	// later page exercises the pagination loop. Target lives well past the
	// first page.
	pageSize := zitadel.DefaultSearchLimit
	items := make([]zitadel.UserGrant, pageSize+5)
	for i := range items {
		items[i] = zitadel.UserGrant{ID: fmt.Sprintf("g-%d", i), UserID: "u1", ProjectID: "p1", RoleKeys: []string{"r"}}
	}
	target := items[pageSize+2]
	target.ID = "g-target"
	target.ProjectID = "p-deep"
	items[pageSize+2] = target

	pageCalls := 0
	prev := zitadelListUserGrants
	t.Cleanup(func() { zitadelListUserGrants = prev })
	zitadelListUserGrants = func(_ context.Context, _ string, p zitadel.SearchParams) (*zitadel.SearchResult[zitadel.UserGrant], error) {
		pageCalls++
		start := p.Offset
		if start > len(items) {
			start = len(items)
		}
		end := start + p.Limit
		if end > len(items) {
			end = len(items)
		}
		return &zitadel.SearchResult[zitadel.UserGrant]{Items: items[start:end], Total: len(items)}, nil
	}

	got, err := listUserGrantsViaZitadel(context.Background(), "u1", "g-target")
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if got.ProjectID != "p-deep" {
		t.Errorf("ProjectID = %q, want p-deep (target lives on page 2)", got.ProjectID)
	}
	if pageCalls < 2 {
		t.Errorf("pageCalls = %d, want >=2 (target is past the first page)", pageCalls)
	}
}

func TestListUserGrantsViaZitadel_ExhaustsPagesAndReturnsNotFound(t *testing.T) {
	pageSize := zitadel.DefaultSearchLimit
	items := make([]zitadel.UserGrant, pageSize+1)
	for i := range items {
		items[i] = zitadel.UserGrant{ID: fmt.Sprintf("g-%d", i), UserID: "u1", ProjectID: "p1"}
	}
	stubListUserGrants(t, items, nil)
	_, err := listUserGrantsViaZitadel(context.Background(), "u1", "g-missing")
	if err == nil {
		t.Fatal("err = nil, want not-found after walking every page")
	}
}
