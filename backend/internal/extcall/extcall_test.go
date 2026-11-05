package extcall

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

var fast = Policy{Attempts: 3, Base: 100 * time.Millisecond, Max: 400 * time.Millisecond}

// recordSleeps swaps the sleep seam for one that records and returns at once.
func recordSleeps(t *testing.T) *[]time.Duration {
	t.Helper()
	var got []time.Duration
	orig := sleep
	sleep = func(ctx context.Context, d time.Duration) error { got = append(got, d); return ctx.Err() }
	t.Cleanup(func() { sleep = orig })
	return &got
}

func status(code int) *http.Response {
	return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}
}

// script returns an Op that answers from a list, one entry per try, and counts.
type answer struct {
	code int
	err  error
}

func script(n *int, answers ...answer) Op {
	return func(context.Context) (*http.Response, error) {
		a := answers[min(*n, len(answers)-1)]
		*n++
		if a.err != nil {
			return nil, a.err
		}
		return status(a.code), nil
	}
}

var timeout = &url.Error{Op: "Post", URL: "https://zitadel.example.org/x", Err: &net.DNSError{IsTimeout: true}}

func TestBackoff_DoublesWithJitterAndStopsAtTheCap(t *testing.T) {
	p := Policy{Base: time.Second, Max: 10 * time.Second}
	steps := []time.Duration{1, 2, 4, 8, 10, 10, 10}
	for n, step := range steps {
		step *= time.Second
		for i := 0; i < 200; i++ {
			d := Backoff(p, n)
			if d < step/2 || d > step {
				t.Fatalf("Backoff(%d) = %s, want within [%s, %s]", n, d, step/2, step)
			}
		}
	}
	// Equal jitter keeps spacing monotone: the smallest wait after n+1 tries is
	// the largest after n, so retries never bunch up.
	for n := 0; n < 3; n++ {
		if lo, hi := steps[n+1]*time.Second/2, steps[n]*time.Second; lo < hi {
			t.Fatalf("step %d: next floor %s below this ceiling %s", n, lo, hi)
		}
	}
}

func TestRetryableStatus(t *testing.T) {
	for code, want := range map[int]bool{
		200: false, 400: false, 401: false, 403: false, 404: false, 409: false,
		408: true, 429: true, 500: true, 502: true, 503: true, 504: true,
	} {
		if RetryableStatus(code) != want {
			t.Errorf("RetryableStatus(%d) = %v, want %v", code, !want, want)
		}
	}
}

func TestNoAnswer(t *testing.T) {
	if !NoAnswer(timeout) {
		t.Error("a timeout is no answer")
	}
	if !NoAnswer(&net.OpError{Op: "dial", Err: errors.New("connection refused")}) {
		t.Error("a refused connection is no answer")
	}
	if !NoAnswer(context.DeadlineExceeded) || !NoAnswer(io.ErrUnexpectedEOF) {
		t.Error("a per-try deadline and a cut connection are no answer")
	}
	if NoAnswer(context.Canceled) {
		t.Error("the caller cancelling is not a failure to retry")
	}
	if NoAnswer(errors.New("obtain access token: zitadel api 401")) {
		t.Error("an error that is not transport is final")
	}
}

func TestDo_ReadRetriesWithBackoffThenSucceeds(t *testing.T) {
	sleeps := recordSleeps(t)
	n := 0
	resp, err := Do(context.Background(), fast, Call{Name: "GET /x", Idempotent: true},
		script(&n, answer{code: 503}, answer{err: timeout}, answer{code: 200}))
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("got %v %v, want 200", resp, err)
	}
	if n != 3 || len(*sleeps) != 2 {
		t.Fatalf("tries=%d sleeps=%d, want 3 and 2", n, len(*sleeps))
	}
	for i, d := range *sleeps {
		step := fast.Base << i
		if d < step/2 || d > step {
			t.Errorf("sleep %d = %s, want within [%s, %s]", i, d, step/2, step)
		}
	}
}

func TestDo_ReadGivesUpAfterItsBudgetWithTheLastAnswer(t *testing.T) {
	recordSleeps(t)
	n := 0
	resp, err := Do(context.Background(), fast, Call{Name: "GET /x", Idempotent: true}, script(&n, answer{code: 503}))
	if err != nil || resp == nil || resp.StatusCode != 503 {
		t.Fatalf("the last answer must come back so its status survives; got %v %v", resp, err)
	}
	if n != fast.Attempts {
		t.Fatalf("tries = %d, want %d", n, fast.Attempts)
	}
}

func TestDo_TerminalAnswersAreNotRetried(t *testing.T) {
	recordSleeps(t)
	for _, code := range []int{400, 403, 404, 409} {
		n := 0
		resp, err := Do(context.Background(), fast, Call{Name: "GET /x", Idempotent: true}, script(&n, answer{code: code}))
		if err != nil || resp.StatusCode != code || n != 1 {
			t.Errorf("%d: tries=%d resp=%v err=%v, want one try and the answer back", code, n, resp, err)
		}
	}
}

// A write is repeated only after a 429: the other side said it did not
// process it.
func TestDo_WriteRetriesA429(t *testing.T) {
	recordSleeps(t)
	n := 0
	resp, err := Do(context.Background(), fast, Call{Name: "POST /x"}, script(&n, answer{code: 429}, answer{code: 200}))
	if err != nil || resp.StatusCode != 200 || n != 2 {
		t.Fatalf("tries=%d resp=%v err=%v, want a retry after 429", n, resp, err)
	}
}

// Any other failure of a write is ambiguous and, without a read-back, goes
// back to the caller unrepeated.
func TestDo_AmbiguousWriteIsNotRepeated(t *testing.T) {
	recordSleeps(t)
	for _, a := range []answer{{code: 503}, {code: 500}, {code: 408}, {err: timeout}} {
		n := 0
		resp, err := Do(context.Background(), fast, Call{Name: "POST /x"}, script(&n, a, answer{code: 200}))
		if n != 1 {
			t.Errorf("%+v: a write was sent %d times", a, n)
		}
		if a.err != nil && err == nil {
			t.Errorf("%+v: the transport error must come back", a)
		}
		if a.err == nil && (resp == nil || resp.StatusCode != a.code) {
			t.Errorf("%+v: the ambiguous answer must come back, got %v %v", a, resp, err)
		}
	}
}

func TestDo_ReadBackResolvesAnAmbiguousWrite(t *testing.T) {
	recordSleeps(t)
	reads := 0
	readBack := func(landed bool) func(context.Context) (bool, error) {
		return func(context.Context) (bool, error) { reads++; return landed, nil }
	}

	n := 0
	resp, err := Do(context.Background(), fast, Call{Name: "POST /x", ReadBack: readBack(true)}, script(&n, answer{err: timeout}))
	if err != nil || resp.StatusCode != 200 || n != 1 || reads != 1 {
		t.Fatalf("landed: tries=%d reads=%d resp=%v err=%v, want one try, one read, success", n, reads, resp, err)
	}

	n, reads = 0, 0
	resp, err = Do(context.Background(), fast, Call{Name: "POST /x", ReadBack: readBack(false)}, script(&n, answer{code: 503}, answer{code: 200}))
	if err != nil || resp.StatusCode != 503 || n != 1 || reads != 1 {
		t.Fatalf("not landed: tries=%d reads=%d resp=%v err=%v, want the 503 back and no second send", n, reads, resp, err)
	}

	// A read-back is never consulted on an answer that is not ambiguous.
	n, reads = 0, 0
	if _, err := Do(context.Background(), fast, Call{Name: "POST /x", ReadBack: readBack(true)}, script(&n, answer{code: 400})); err != nil || reads != 0 {
		t.Fatalf("a 400 is final; reads=%d err=%v", reads, err)
	}
}

func TestDo_StopsWhenTheCallerCancels(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	n := 0
	op := func(context.Context) (*http.Response, error) { n++; cancel(); return status(503), nil }
	if _, err := Do(ctx, fast, Call{Name: "GET /x", Idempotent: true}, op); !errors.Is(err, context.Canceled) || n != 1 {
		t.Fatalf("tries=%d err=%v, want one try and the cancellation", n, err)
	}
}

// The per-try timeout fires on a hung try, and its cancel waits for the body
// to be closed, so a caller can still read an answer after Do returns.
func TestDo_TimeoutPerTryAndTheBodyOutlivesDo(t *testing.T) {
	recordSleeps(t)
	p := fast
	p.Timeout = 20 * time.Millisecond

	n := 0
	var lastCtx context.Context
	op := func(ctx context.Context) (*http.Response, error) {
		n++
		lastCtx = ctx
		if n == 1 {
			<-ctx.Done() // hang until the per-try timeout
			return nil, &url.Error{Op: "Get", URL: "https://zitadel.example.org/x", Err: ctx.Err()}
		}
		return status(200), nil
	}
	resp, err := Do(context.Background(), p, Call{Name: "GET /x", Idempotent: true}, op)
	if err != nil || resp.StatusCode != 200 || n != 2 {
		t.Fatalf("tries=%d resp=%v err=%v, want the hung try timed out and retried", n, resp, err)
	}
	if lastCtx.Err() != nil {
		t.Fatal("the try's context was cancelled before the caller read the body")
	}
	resp.Body.Close()
	if lastCtx.Err() == nil {
		t.Fatal("closing the body must release the try's context")
	}
}

func TestDo_OneRequestIDAcrossTries(t *testing.T) {
	recordSleeps(t)
	var ids []string
	n := 0
	inner := script(&n, answer{code: 503}, answer{code: 200})
	op := func(ctx context.Context) (*http.Response, error) {
		ids = append(ids, RequestID(ctx))
		return inner(ctx)
	}
	if _, err := Do(context.Background(), fast, Call{Name: "GET /x", Idempotent: true}, op); err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] == "" || ids[0] != ids[1] {
		t.Fatalf("request ids %q: want one non-empty id for every try", ids)
	}
}
