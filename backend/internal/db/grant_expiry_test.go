package db

import (
	"errors"
	"regexp"
	"strings"
	"testing"
)

// 1.16 — the expiry re-check lives in the DELETE's own predicate. A renewal
// lands on the same row (ON CONFLICT DO UPDATE pushes expires_at forward), so
// anything decided from the sweep's snapshot is decided about a row that may
// already be alive again. Under READ COMMITTED the DELETE re-evaluates against
// the version it finds, and a renewed grant simply does not match.
func TestExpiryWriteCarriesItsOwnGuard(t *testing.T) {
	body := funcBody(t, readDBSource(t, "grants.go"), "DeleteExpiredDirectGrantAndEnqueue")

	del := regexp.MustCompile(`(?s)DELETE FROM direct_role_grants(.*?)RETURNING`).FindStringSubmatch(body)
	if del == nil {
		t.Fatal("could not isolate the expiry delete")
	}
	for _, want := range []string{
		"WHERE id = $1 AND user_id = $2",
		"expires_at IS NOT NULL",
		"expires_at <= NOW()",
	} {
		if !strings.Contains(del[1], want) {
			t.Errorf("the expiry delete must assert %q in its own predicate, not rely on the caller having checked", want)
		}
	}

	// The delta, the audit row and the ledger delete commit together or not at
	// all. Enqueuing outside this transaction would leave a revoke queued for a
	// grant that is still valid, and an audit row about a deletion that did not
	// happen.
	if !strings.Contains(body, "enqueueCascadeRows(ctx, tx,") {
		t.Error("the outbox and audit writes must run on this transaction")
	}
	if !strings.Contains(body, `Action: "direct_grant.revoked_by_expiry"`) {
		t.Error("the audit row must say a clock did this, not a person")
	}
	// Project and role are read back from the row that was actually removed, so
	// every downstream side effect names what went away rather than what the
	// snapshot said would.
	if !strings.Contains(body, "RETURNING zitadel_project_id, zitadel_role_key") {
		t.Error("the delete must return the identifiers the caller then uses")
	}
}

// A renewed grant is not a missing one. Nothing is wrong, nothing is owed, and
// the sweep must be able to tell the difference — one means leave it alone, the
// other means something is broken.
func TestRenewedIsNotMissing(t *testing.T) {
	if errors.Is(ErrGrantRenewed, ErrGrantNotFound) || errors.Is(ErrGrantNotFound, ErrGrantRenewed) {
		t.Fatal("a renewed grant and an absent one must be distinguishable")
	}
	body := funcBody(t, readDBSource(t, "grants.go"), "DeleteExpiredDirectGrantAndEnqueue")
	if !regexp.MustCompile(`(?s)errors\.Is\(err, pgx\.ErrNoRows\).*?ErrGrantRenewed`).MatchString(body) {
		t.Error("a no-match delete must be reported as a renewal, since the predicate that failed is the expiry one")
	}
}

// The batched delete this replaced took a user's whole candidate set in one
// statement, which cannot carry a per-grant delta: two grants deriving the same
// role would each see the other still covering it.
func TestTheBatchedExpiryDeleteIsGone(t *testing.T) {
	src := readDBSource(t, "grants.go")
	if strings.Contains(src, "func DeleteExpiredDirectGrantsByIDs") {
		t.Error("the batched delete has no caller and no delta; keeping it invites a second expiry path that revokes nothing")
	}
}
