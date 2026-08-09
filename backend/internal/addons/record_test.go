package addons

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"syndra/internal/db"
)

// withClaimableRecord swaps the durable-record claim for a fake that enforces
// the same predicate the SQL does: the row must match the call, and it may be
// claimed once.
func withClaimableRecord(t *testing.T, row db.AddonOperation, err error) *int32 {
	t.Helper()
	var claims int32
	saved := dbClaimAddonOperation
	var mu sync.Mutex
	taken := false
	dbClaimAddonOperation = func(_ context.Context, id, target, operation, subject string) (db.AddonOperation, error) {
		atomic.AddInt32(&claims, 1)
		if err != nil {
			return db.AddonOperation{}, err
		}
		mu.Lock()
		defer mu.Unlock()
		if taken || row.ID != id || row.Target != target || row.Operation != operation || row.SubjectID != subject {
			return db.AddonOperation{}, db.ErrAddonOperationNotOpen
		}
		taken = true
		return row, nil
	}
	t.Cleanup(func() { dbClaimAddonOperation = saved })
	return &claims
}

func openRow() db.AddonOperation {
	return db.AddonOperation{
		ID: "rec-0001", Target: "truenas", Operation: "password.set",
		ActorID: "user-42", SubjectID: "user-42", Status: db.AddonOpDispatching,
	}
}

// P2 — a dispatch is authorised by a durable record that names it, not by a
// non-empty string. The token cannot be constructed outside this package, so
// the only way to obtain one is to have the record read back and checked.
func TestOperationRecordAuthorisesOnlyTheCallItDescribes(t *testing.T) {
	t.Run("a matching open record mints a token", func(t *testing.T) {
		claims := withClaimableRecord(t, openRow(), nil)
		rec, err := OperationRecord(context.Background(), "rec-0001", "truenas", "password.set", "user-42")
		if err != nil {
			t.Fatalf("OperationRecord: %v", err)
		}
		if !rec.valid() || rec.CallID() != "rec-0001" {
			t.Fatalf("token did not carry the record id: %+v", rec)
		}
		if *claims != 1 {
			t.Fatalf("the record was claimed %d times; the token must come from the claim, not from the argument", *claims)
		}
	})

	t.Run("no open record means no token", func(t *testing.T) {
		withClaimableRecord(t, db.AddonOperation{}, db.ErrAddonOperationNotOpen)
		if _, err := OperationRecord(context.Background(), "made-up", "truenas", "password.set", "user-42"); !errors.Is(err, ErrNoCallRecord) {
			t.Fatalf("err = %v, want ErrNoCallRecord", err)
		}
	})

	// The reason existence alone is not enough. A caller holding any real
	// record id could otherwise authorise a different call with it, leaving an
	// audit trail that describes something that did not happen — worse than no
	// trail, because it will be believed.
	mismatches := []struct{ name, target, operation, subject string }{
		{"another target", "unifi", "password.set", "user-42"},
		{"another operation", "truenas", "account.purge", "user-42"},
		{"another subject", "truenas", "password.set", "user-99"},
	}
	for _, tc := range mismatches {
		t.Run("a record for "+tc.name+" is refused", func(t *testing.T) {
			withClaimableRecord(t, openRow(), nil)
			_, err := OperationRecord(context.Background(), "rec-0001", tc.target, tc.operation, tc.subject)
			if !errors.Is(err, ErrNoCallRecord) {
				t.Fatalf("err = %v, want ErrNoCallRecord", err)
			}
		})
	}
}

// P2 — the token's zero value authorises nothing, so a CallRequest that simply
// omits it cannot dispatch. This is what makes the guarantee structural: there
// is no string a caller can supply instead.
func TestAZeroRecordCannotDispatch(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	signedAddon(t, srv.URL, []byte("k"))
	withBreaker(t, 1000, time.Minute)

	req := passwordSet(nil)
	req.Record = DispatchRecord{}
	resp := Call(context.Background(), req)

	if !errors.Is(resp.Err, ErrNoCallRecord) {
		t.Fatalf("err = %v, want ErrNoCallRecord", resp.Err)
	}
	if hits.Load() != 0 {
		t.Fatal("a call with no durable record reached the add-on")
	}
}

// P1 — the transport re-checks the schema, so no path to an add-on exists that
// skipped it, whoever adds that path later.
func TestTheTransportRefusesParametersPolicyDoesNotDeclare(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	signedAddon(t, srv.URL, []byte("k"))
	withBreaker(t, 1000, time.Minute)

	resp := Call(context.Background(), passwordSet(map[string]any{
		"password": "hunter2",
		"shell":    "/bin/sh",
	}))
	if !errors.Is(resp.Err, ErrInvalidParams) {
		t.Fatalf("err = %v, want ErrInvalidParams", resp.Err)
	}
	if resp.Outcome != OutcomeUnreached {
		t.Fatalf("outcome = %s, want unreached — nothing was sent", resp.Outcome)
	}
	if hits.Load() != 0 {
		t.Fatal("an undeclared parameter reached the add-on")
	}
}

// P1 — the binding travels in the token and is compared at dispatch. A check
// that only ran at mint time would have verified the call that was not made:
// mint against a health check, send a password under it.
func TestATokenCannotBeUsedForADifferentCall(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	signedAddon(t, srv.URL, []byte("k"))
	withBreaker(t, 1000, time.Minute)
	withClaimableRecord(t, openRow(), nil)

	// Legitimately minted, for password.set on user-42.
	rec, err := OperationRecord(context.Background(), "rec-0001", "truenas", "password.set", "user-42")
	if err != nil {
		t.Fatalf("OperationRecord: %v", err)
	}

	t.Run("the call it was minted for is allowed", func(t *testing.T) {
		req := passwordSet(nil)
		req.Record = rec
		if resp := Call(context.Background(), req); resp.Outcome != OutcomeSucceeded {
			t.Fatalf("outcome = %s (err %v)", resp.Outcome, resp.Err)
		}
	})

	for _, tc := range []struct{ name, target, operation, subject string }{
		{"another subject", "truenas", "password.set", "user-99"},
		{"another operation", "truenas", "health.get", "user-42"},
		{"another target", "unifi", "password.set", "user-42"},
	} {
		t.Run("reused for "+tc.name, func(t *testing.T) {
			before := hits.Load()
			req := passwordSet(nil)
			req.Record = rec
			req.Target, req.Operation, req.Subject = tc.target, tc.operation, tc.subject
			if tc.operation == "health.get" {
				req.Params = nil // health.get declares none
			}

			resp := Call(context.Background(), req)
			if resp.Outcome == OutcomeSucceeded {
				t.Fatal("a token authorised a call it was not minted for")
			}
			if hits.Load() != before {
				t.Fatal("the mismatched call reached the add-on")
			}
		})
	}
}

// P1 — the claim is taken once. A record cannot authorise a second dispatch:
// not concurrently with the first, and not after it settles. A lookup would
// permit both, because a lookup can be repeated.
func TestARecordCanBeClaimedOnlyOnce(t *testing.T) {
	claims := withClaimableRecord(t, openRow(), nil)

	first, err := OperationRecord(context.Background(), "rec-0001", "truenas", "password.set", "user-42")
	if err != nil {
		t.Fatalf("first claim: %v", err)
	}
	if !first.valid() {
		t.Fatal("the first claim produced no token")
	}

	if _, err := OperationRecord(context.Background(), "rec-0001", "truenas", "password.set", "user-42"); !errors.Is(err, ErrNoCallRecord) {
		t.Fatalf("a second claim on one record succeeded: %v", err)
	}
	if *claims != 2 {
		t.Fatalf("expected both attempts to reach the database, got %d", *claims)
	}
}

// P1 — the token is built from the row the database returned, not from the
// arguments the caller supplied.
//
// Built from the arguments, the binding would be an echo of the caller's own
// request, and the comparison in Call would compare a claim to itself — the
// same "verification that verifies nothing" as checking only at mint time, one
// level further down. The token has to be a fact about a committed record.
//
// The fake returns a row differing from what was asked for, which the real
// claim predicate cannot produce. That is the point: the test pins where the
// values come from, so the property survives a claim that later resolves an
// alias, canonicalises an id, or is replaced wholesale.
func TestTheTokenIsBuiltFromTheStoredRowNotTheRequest(t *testing.T) {
	stored := db.AddonOperation{
		ID: "rec-canonical", Target: "truenas-a", Operation: "password.set",
		ActorID: "user-42", SubjectID: "subject-canonical", Status: db.AddonOpDispatching,
	}
	saved := dbClaimAddonOperation
	dbClaimAddonOperation = func(context.Context, string, string, string, string) (db.AddonOperation, error) {
		return stored, nil
	}
	t.Cleanup(func() { dbClaimAddonOperation = saved })

	rec, err := OperationRecord(context.Background(), "rec-asked", "truenas-b", "password.set", "subject-asked")
	if err != nil {
		t.Fatalf("OperationRecord: %v", err)
	}
	if rec.CallID() != stored.ID {
		t.Fatalf("call id = %q, want the stored %q — the token echoed the request", rec.CallID(), stored.ID)
	}
	if !rec.authorises(stored.Target, stored.Operation, stored.SubjectID) {
		t.Fatalf("the token does not authorise the call the stored row describes: %+v", rec)
	}
	if rec.authorises("truenas-b", "password.set", "subject-asked") {
		t.Fatal("the token authorises the call as REQUESTED rather than as recorded")
	}
}
