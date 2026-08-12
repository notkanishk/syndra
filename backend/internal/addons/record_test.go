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

// withClaimableRecord swaps the durable-record claim for a fake enforcing the
// same predicate the SQL does: the row must match the call, and it may be
// claimed once. The once is the point — it is the only thing standing between
// one minted token and any number of dispatches.
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

func recordFor(target, operation, subject string) DispatchRecord {
	return OperationRecord("rec-0001", target, operation, subject)
}

// P1 — a dispatch is authorised by a durable record naming it, and the
// authority is spent against the database rather than held in a Go value.
func TestOperationRecordAuthorisesOnlyTheCallItDescribes(t *testing.T) {
	t.Run("minting touches nothing", func(t *testing.T) {
		claims := withClaimableRecord(t, openRow(), nil)
		rec := recordFor("truenas", "password.set", "user-42")
		if !rec.valid() || rec.CallID() != "rec-0001" {
			t.Fatalf("token did not name the record: %+v", rec)
		}
		if *claims != 0 {
			t.Fatalf("minting claimed the record %d times; the claim belongs at the call, "+
				"or a token claimed early is still spendable twice", *claims)
		}
	})

	t.Run("the claim happens when the call does", func(t *testing.T) {
		claims := withClaimableRecord(t, openRow(), nil)
		if err := recordFor("truenas", "password.set", "user-42").consume(context.Background()); err != nil {
			t.Fatalf("consume: %v", err)
		}
		if *claims != 1 {
			t.Fatalf("the record was claimed %d times, want 1", *claims)
		}
	})

	t.Run("no claimable record means no dispatch", func(t *testing.T) {
		withClaimableRecord(t, db.AddonOperation{}, db.ErrAddonOperationNotOpen)
		rec := OperationRecord("made-up", "truenas", "password.set", "user-42")
		if err := rec.consume(context.Background()); !errors.Is(err, ErrNoCallRecord) {
			t.Fatalf("err = %v, want ErrNoCallRecord", err)
		}
	})

	// Existence alone is not enough. A caller holding any real record id could
	// otherwise authorise a different call with it, leaving an audit trail that
	// describes something that did not happen — worse than no trail, because it
	// will be believed.
	for _, tc := range []struct{ name, target, operation, subject string }{
		{"another target", "unifi", "password.set", "user-42"},
		{"another operation", "truenas", "account.purge", "user-42"},
		{"another subject", "truenas", "password.set", "user-99"},
	} {
		t.Run("a record for "+tc.name+" cannot be claimed", func(t *testing.T) {
			withClaimableRecord(t, openRow(), nil)
			err := recordFor(tc.target, tc.operation, tc.subject).consume(context.Background())
			if !errors.Is(err, ErrNoCallRecord) {
				t.Fatalf("err = %v, want ErrNoCallRecord", err)
			}
		})
	}
}

// P1 — one minted token dispatches once. A Go value copies freely, so nothing
// the token itself holds can stop a second Call; the durable claim can, and it
// is taken at the moment of the request rather than when the token was made.
//
// This is the case the two previous designs both allowed: a legitimately
// obtained capability replayed through the transport, producing a second
// network dispatch under one record id.
func TestOneTokenDispatchesOnce(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	signedAddon(t, srv.URL, "a-test-secret")
	withBreaker(t, 1000, time.Minute)
	claims := withClaimableRecord(t, openRow(), nil)

	req := passwordSet(nil)
	req.Record = recordFor("truenas", "password.set", "user-42")

	if resp := Call(context.Background(), req); resp.Outcome != OutcomeSucceeded {
		t.Fatalf("first call: outcome = %s (err %v)", resp.Outcome, resp.Err)
	}

	// The same token, copied by every argument pass since it was made.
	second := Call(context.Background(), req)
	if second.Outcome == OutcomeSucceeded {
		t.Fatal("one record authorised two dispatches")
	}
	if !errors.Is(second.Err, ErrNoCallRecord) {
		t.Fatalf("err = %v, want ErrNoCallRecord", second.Err)
	}
	if hits.Load() != 1 {
		t.Fatalf("the add-on was called %d times under one record", hits.Load())
	}
	if *claims != 2 {
		t.Fatalf("both attempts must reach the claim, got %d", *claims)
	}
}

// P1 — concurrent calls under one token: exactly one may reach the add-on. The
// in-process copy is not the thing being serialised; the durable row is.
func TestConcurrentCallsUnderOneTokenDispatchOnce(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	signedAddon(t, srv.URL, "a-test-secret")
	withBreaker(t, 1000, time.Minute)
	withClaimableRecord(t, openRow(), nil)

	req := passwordSet(nil)
	req.Record = recordFor("truenas", "password.set", "user-42")

	var wg sync.WaitGroup
	outcomes := make([]Outcome, 8)
	for i := range outcomes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			outcomes[i] = Call(context.Background(), req).Outcome
		}(i)
	}
	wg.Wait()

	succeeded := 0
	for _, o := range outcomes {
		if o == OutcomeSucceeded {
			succeeded++
		}
	}
	if succeeded != 1 {
		t.Fatalf("%d of %d concurrent calls succeeded under one record, want exactly 1", succeeded, len(outcomes))
	}
	if hits.Load() != 1 {
		t.Fatalf("the add-on was called %d times", hits.Load())
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

	signedAddon(t, srv.URL, "a-test-secret")
	withBreaker(t, 1000, time.Minute)

	for _, tc := range []struct{ name, target, operation, subject string }{
		{"another subject", "truenas", "password.set", "user-99"},
		{"another operation", "truenas", "health.get", "user-42"},
		{"another target", "unifi", "password.set", "user-42"},
	} {
		t.Run("reused for "+tc.name, func(t *testing.T) {
			claims := withClaimableRecord(t, openRow(), nil)
			before := hits.Load()

			req := passwordSet(nil)
			req.Record = recordFor("truenas", "password.set", "user-42")
			req.Target, req.Operation, req.Subject = tc.target, tc.operation, tc.subject
			if tc.operation == "health.get" {
				req.Params = nil // health.get declares no parameters
			}

			resp := Call(context.Background(), req)
			if resp.Outcome == OutcomeSucceeded {
				t.Fatal("a token authorised a call it was not minted for")
			}
			if hits.Load() != before {
				t.Fatal("the mismatched call reached the add-on")
			}
			if *claims != 0 {
				t.Fatal("a mismatched call consumed the record, spending a capability the " +
					"legitimate dispatch still needs")
			}
		})
	}
}

// P1 — a call refused before the network leaves the record unclaimed. Consuming
// it would record a dispatch that never happened, and would then make the row
// ineligible for the only outcome that describes it.
func TestARefusedCallDoesNotConsumeTheRecord(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	signedAddon(t, srv.URL, "a-test-secret")
	withBreaker(t, 1, time.Hour)
	claims := withClaimableRecord(t, openRow(), nil)

	a, _ := Get("truenas")
	a.br.record(timeNow(), CallResponse{Outcome: OutcomeIndeterminate})
	if !a.CircuitOpen() {
		t.Fatal("setup: the circuit should be open")
	}

	req := passwordSet(nil)
	req.Record = recordFor("truenas", "password.set", "user-42")
	resp := Call(context.Background(), req)

	if !errors.Is(resp.Err, ErrCircuitOpen) {
		t.Fatalf("err = %v, want ErrCircuitOpen", resp.Err)
	}
	if *claims != 0 {
		t.Fatalf("a call refused before the network claimed the record %d times", *claims)
	}
}

// P2 — the token's zero value authorises nothing, so a CallRequest that simply
// omits it cannot dispatch. There is no string a caller can supply instead.
func TestAZeroRecordCannotDispatch(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	signedAddon(t, srv.URL, "a-test-secret")
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

	signedAddon(t, srv.URL, "a-test-secret")
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

// P1 — a token with a binding but no way to spend it is not a token. Only code
// inside this package can build one, and only by mistake; refusing it here is
// the difference between a clear refusal and a nil-function panic on the
// dispatch path.
func TestATokenWithNoClaimCannotDispatch(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	signedAddon(t, srv.URL, "a-test-secret")
	withBreaker(t, 1000, time.Minute)

	req := passwordSet(nil)
	req.Record = DispatchRecord{
		callID: "rec-0001", target: "truenas", operation: "password.set", subject: "user-42",
	}

	resp := Call(context.Background(), req)
	if !errors.Is(resp.Err, ErrNoCallRecord) {
		t.Fatalf("err = %v, want ErrNoCallRecord", resp.Err)
	}
	if hits.Load() != 0 {
		t.Fatal("a token that can never be spent authorised a dispatch")
	}
}
