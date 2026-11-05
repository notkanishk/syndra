package propagation

import (
	"context"
	"testing"

	"syndra/internal/db"
	"syndra/internal/models"
	"syndra/internal/zitadel"
)

// Final verification N1: the plain cause of a refusal, from the status
// Zitadel answered with, as the drain records it.
func TestRefusalCause(t *testing.T) {
	cases := map[string]string{
		(&zitadel.StatusError{Code: 403, Message: "no"}).Error():                       db.RefusalNotAllowed,
		(&zitadel.StatusError{Code: 401}).Error():                                      db.RefusalNotAllowed,
		"add: could not read live grant: " + (&zitadel.StatusError{Code: 404}).Error(): db.RefusalNotFound,
		(&zitadel.StatusError{Code: 400}).Error():                                      db.RefusalInvalid,
		(&zitadel.StatusError{Code: 412}).Error():                                      db.RefusalInvalid,
		(&zitadel.StatusError{Code: 418}).Error():                                      db.RefusalOther,
		`unknown op_type "x"`:                                                          db.RefusalOther,
	}
	for msg, want := range cases {
		if got := RefusalCause(msg); got != want {
			t.Errorf("RefusalCause(%q) = %q, want %q", msg, got, want)
		}
	}
}

// The drain names which row Zitadel refused, so a caller can tell its own
// row from another's; a row it took is not named.
func TestDrain_NamesTheRefusedRow(t *testing.T) {
	stubDrainDeps(t)
	claimPending = func(context.Context, string, int) ([]models.PendingPropagation, error) {
		return []models.PendingPropagation{
			{ID: "bad", OpType: "add", UserID: "u", ProjectID: "p", RoleKeys: []string{"r1"}},
			{ID: "ok", OpType: "add", UserID: "u", ProjectID: "p", RoleKeys: []string{"r2"}},
		}, nil
	}
	zitadelAddUserGrant = func(_ context.Context, _, _ string, roles []string) error {
		if roles[0] == "r1" {
			return statusErr(403)
		}
		return nil
	}
	markFailed = func(context.Context, string, string) error { return nil }
	markApplied = func(context.Context, string) error { return nil }

	res, _ := Drain(context.Background())
	if res.Refused["bad"] != db.RefusalNotAllowed || len(res.Refused) != 1 {
		t.Fatalf("want only bad refused as not_allowed, got %+v", res.Refused)
	}
}
