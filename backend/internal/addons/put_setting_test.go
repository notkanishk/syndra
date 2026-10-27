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

// stubOperationRecord fakes the two ends of the record PutSetting writes: the
// row committed before the call and the status settled after it.
func stubOperationRecord(t *testing.T, beginErr error) (began *[]db.AddonOperationParams, settled *[]string) {
	t.Helper()
	b, s := []db.AddonOperationParams{}, []string{}
	savedBegin, savedSettle := dbBeginAddonOperation, dbSettleAddonOperation
	dbBeginAddonOperation = func(_ context.Context, p db.AddonOperationParams) (string, error) {
		if beginErr != nil {
			return "", beginErr
		}
		b = append(b, p)
		return "rec-put-1", nil
	}
	dbSettleAddonOperation = func(_ context.Context, id, status string) error {
		if id != "rec-put-1" {
			t.Errorf("settled %q, want the row that was begun", id)
		}
		s = append(s, status)
		return nil
	}
	t.Cleanup(func() { dbBeginAddonOperation, dbSettleAddonOperation = savedBegin, savedSettle })
	return &b, &s
}

// A setting write is a mutation, so it has the record and audit row every
// other one has, and the record says how it ended: a lost answer is
// indeterminate ("may have applied"), never unreached.
func TestPutSettingLeavesTheSameTraceAsOtherMutations(t *testing.T) {
	t.Run("an answer settles as the add-on judged it", func(t *testing.T) {
		var hits int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&hits, 1)
			if r.Method != http.MethodPut || r.URL.Path != "/settings/home_root" {
				t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()
		signedAddon(t, srv.URL, "a-test-secret")
		began, settled := stubOperationRecord(t, nil)

		resp, ok := PutSetting(context.Background(), "truenas", "home_root", "/mnt/p/home", "op-1")
		if !ok || resp.Outcome != OutcomeSucceeded {
			t.Fatalf("resp %+v ok %v", resp, ok)
		}
		want := db.AddonOperationParams{Target: "truenas", Operation: "setting.put", ActorID: "op-1", SubjectID: "home_root"}
		if len(*began) != 1 || (*began)[0] != want {
			t.Fatalf("record begun = %+v, want %+v", *began, want)
		}
		if len(*settled) != 1 || (*settled)[0] != db.AddonOpSucceeded {
			t.Fatalf("settled = %v", *settled)
		}
	})

	t.Run("a timeout may have applied", func(t *testing.T) {
		release := make(chan struct{})
		srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-release }))
		defer func() { close(release); srv.Close() }()
		signedAddon(t, srv.URL, "a-test-secret")
		withCallTimeout(t, 50*time.Millisecond)
		_, settled := stubOperationRecord(t, nil)

		resp, ok := PutSetting(context.Background(), "truenas", "home_root", "/mnt/p/home", "op-1")
		if ok || resp.Outcome != OutcomeIndeterminate {
			t.Fatalf("resp %+v ok %v, want an unanswered, indeterminate call", resp, ok)
		}
		if len(*settled) != 1 || (*settled)[0] != db.AddonOpIndeterminate {
			t.Fatalf("settled = %v, want indeterminate", *settled)
		}
	})

	t.Run("no record, no call", func(t *testing.T) {
		var hits int32
		srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { atomic.AddInt32(&hits, 1) }))
		defer srv.Close()
		signedAddon(t, srv.URL, "a-test-secret")
		stubOperationRecord(t, errors.New("db down"))

		resp, ok := PutSetting(context.Background(), "truenas", "home_root", "/mnt/p/home", "op-1")
		if ok || resp.Outcome != OutcomeUnreached || atomic.LoadInt32(&hits) != 0 {
			t.Fatalf("resp %+v ok %v hits %d: nothing may be sent without its record", resp, ok, hits)
		}
	})
}
