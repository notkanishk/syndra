package observe

import (
	"context"
	"testing"
	"time"

	"syndra/internal/db"
	"syndra/internal/zitadel"
)

// A read's time is when it began listing, from the database's clock, taken
// before the first page: a write Zitadel finished while the listing ran must
// never be read as one the read came after (services.unanswered compares the
// two). The observation returned says the same time the store does.
func TestOrgAndUserReadsAreDatedWhenTheyBegan(t *testing.T) {
	start := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	var order []string
	var recorded []time.Time
	origOK, origStart, origAll, origUser, origOrg, origRecUser := clientOK, readStart, listAllGrants, listUserGrants, recordOrg, recordUser
	t.Cleanup(func() {
		clientOK, readStart, listAllGrants, listUserGrants, recordOrg, recordUser = origOK, origStart, origAll, origUser, origOrg, origRecUser
	})
	clientOK = func() bool { return true }
	readStart = func(context.Context) (time.Time, error) { order = append(order, "start"); return start, nil }
	page := func() (*zitadel.SearchResult[zitadel.UserGrant], error) {
		order = append(order, "list")
		return &zitadel.SearchResult[zitadel.UserGrant]{}, nil
	}
	listAllGrants = func(context.Context, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.UserGrant], error) {
		return page()
	}
	listUserGrants = func(context.Context, string, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.UserGrant], error) {
		return page()
	}
	recordOrg = func(_ context.Context, at time.Time, _ []db.ObservedGrant, _ bool, _ string) error {
		recorded = append(recorded, at)
		return nil
	}
	recordUser = func(_ context.Context, _ string, at time.Time, _ []db.ObservedGrant, _ bool, _ string) error {
		recorded = append(recorded, at)
		return nil
	}

	org, err := Org(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	user, err := User(context.Background(), "u1")
	if err != nil {
		t.Fatal(err)
	}
	if len(order) != 4 || order[0] != "start" || order[1] != "list" || order[2] != "start" || order[3] != "list" {
		t.Fatalf("order %v, want each read's start taken before its listing", order)
	}
	for _, at := range append(recorded, org.ObservedAt, user.ObservedAt) {
		if !at.Equal(start) {
			t.Fatalf("a read dated %v, want its start %v (recorded %v)", at, start, recorded)
		}
	}
}
