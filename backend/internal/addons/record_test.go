package addons

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"syndra/internal/db"
)

// withOpenRecord swaps the durable-record lookup for a fixed row.
func withOpenRecord(t *testing.T, row db.AddonOperation, err error) *int32 {
	t.Helper()
	var reads int32
	saved := dbOpenAddonOperation
	dbOpenAddonOperation = func(context.Context, string) (db.AddonOperation, error) {
		atomic.AddInt32(&reads, 1)
		return row, err
	}
	t.Cleanup(func() { dbOpenAddonOperation = saved })
	return &reads
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
		reads := withOpenRecord(t, openRow(), nil)
		rec, err := OperationRecord(context.Background(), "rec-0001", "truenas", "password.set", "user-42")
		if err != nil {
			t.Fatalf("OperationRecord: %v", err)
		}
		if !rec.valid() || rec.CallID() != "rec-0001" {
			t.Fatalf("token did not carry the record id: %+v", rec)
		}
		if *reads != 1 {
			t.Fatalf("the record was read %d times; the token must come from a read, not from the argument", *reads)
		}
	})

	t.Run("no open record means no token", func(t *testing.T) {
		withOpenRecord(t, db.AddonOperation{}, db.ErrAddonOperationNotOpen)
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
			withOpenRecord(t, openRow(), nil)
			_, err := OperationRecord(context.Background(), "rec-0001", tc.target, tc.operation, tc.subject)
			if !errors.Is(err, ErrRecordMismatch) {
				t.Fatalf("err = %v, want ErrRecordMismatch", err)
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
