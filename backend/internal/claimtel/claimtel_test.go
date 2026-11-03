package claimtel

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 3, 10, 2, 0, 0, time.UTC)

func fixed(at time.Time) func() time.Time { return func() time.Time { return at } }

// capture is a flush target that keeps what it was given.
type capture struct {
	mu      sync.Mutex
	batches []Batch
	fail    error
}

func (c *capture) write(_ context.Context, b Batch) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fail != nil {
		return c.fail
	}
	c.batches = append(c.batches, b)
	return nil
}

func (c *capture) total() map[Key]Count {
	out := map[Key]Count{}
	for _, b := range c.batches {
		for k, v := range b.Counts {
			cur := out[k]
			cur.add(v)
			out[k] = cur
		}
	}
	return out
}

func TestRecord_CountsByDimensionAndLatency(t *testing.T) {
	r := New(fixed(t0))
	for _, ms := range []int{0, 3, 3, 40, 4000} {
		r.Record(Call{Outcome: Served, ProjectID: "p1", ClientID: "c1", Function: "function/preaccesstoken",
			KeySlot: KeySlotCurrent, UserID: "u1", At: t0, Elapsed: time.Duration(ms) * time.Millisecond})
	}
	r.Record(Call{Outcome: SigMismatch, At: t0}) // no dimensions: a refused signature names nothing

	c := &capture{}
	if err := r.Flush(context.Background(), c.write); err != nil {
		t.Fatal(err)
	}
	got := c.total()
	bucket := t0.Truncate(BucketWidth)
	served := got[Key{bucket, "p1", "c1", "function/preaccesstoken", KeySlotCurrent, Served}]
	if served.N != 5 {
		t.Fatalf("served n = %d, want 5 (%v)", served.N, got)
	}
	want := [HistBuckets]int64{1, 0, 2, 0, 0, 1, 0, 0, 0, 0, 0, 1}
	if served.Hist != want {
		t.Errorf("histogram = %v, want %v", served.Hist, want)
	}
	if sig := got[Key{bucket, None, None, None, None, SigMismatch}]; sig.N != 1 {
		t.Errorf("a refused call must count under '-' dimensions: %v", got)
	}
	// Only the read call names an app's last delivery.
	cl := c.batches[0].Clients
	if len(cl) != 1 || cl[ClientKey{"c1", "function/preaccesstoken"}].ServedAt != t0 {
		t.Errorf("clients = %+v", cl)
	}
	// Drained: a second flush writes nothing new.
	_ = r.Flush(context.Background(), c.write)
	if len(c.batches) != 1 {
		t.Errorf("an empty flush wrote a batch")
	}
}

// An outcome nobody enumerated still lands in a class.
func TestRecord_UnknownOutcomeIsBadRequest(t *testing.T) {
	r := New(fixed(t0))
	r.Record(Call{Outcome: "made_up", At: t0})
	if got := r.Recent(1); got[BadRequest] != 1 || len(got) != 1 {
		t.Fatalf("recent = %v", got)
	}
}

// A failed flush keeps every count for the next one, then the next success
// writes them once. Nothing is double-counted, nothing lost.
func TestFlush_FailureKeepsCountsAndSaysSo(t *testing.T) {
	r := New(fixed(t0))
	r.Record(Call{Outcome: Served, ProjectID: "p", ClientID: "c", Function: "f", At: t0})
	c := &capture{fail: errors.New("postgres down")}
	if err := r.Flush(context.Background(), c.write); err == nil {
		t.Fatal("want the write error back")
	}
	if st := r.Status(); st.Failures != 1 || st.Retained != 1 || !strings.Contains(st.LastErr, "postgres down") {
		t.Fatalf("status = %+v", st)
	}
	r.Record(Call{Outcome: Served, ProjectID: "p", ClientID: "c", Function: "f", At: t0})
	c.fail = nil
	if err := r.Flush(context.Background(), c.write); err != nil {
		t.Fatal(err)
	}
	if n := c.total()[Key{t0.Truncate(BucketWidth), "p", "c", "f", None, Served}].N; n != 2 {
		t.Fatalf("after recovery n = %d, want 2", n)
	}
	if st := r.Status(); st.Failures != 0 || st.Retained != 0 {
		t.Fatalf("status after recovery = %+v", st)
	}
}

// A long outage costs detail, never unbounded memory.
func TestFlush_RetainedCountsAreCapped(t *testing.T) {
	r := New(fixed(t0))
	for i := 0; i < maxRetained+50; i++ {
		r.Record(Call{Outcome: Served, ProjectID: fmt.Sprint("p", i), Function: "f", At: t0})
	}
	c := &capture{fail: errors.New("down")}
	_ = r.Flush(context.Background(), c.write)
	if st := r.Status(); st.Retained != maxRetained || st.Dropped != 50 {
		t.Fatalf("status = %+v, want %d retained and 50 dropped", st, maxRetained)
	}
}

// The (person, client) map is bounded and pruned.
func TestPairs_PrunedAndBounded(t *testing.T) {
	now := t0
	r := New(func() time.Time { return now })
	r.Record(Call{Outcome: Served, UserID: "old", ClientID: "c", Function: "f", At: t0})
	now = t0.Add(25 * time.Minute)
	r.Record(Call{Outcome: Served, UserID: "new", ClientID: "c", Function: "f", At: now})
	_ = r.Flush(context.Background(), (&capture{}).write)
	if _, ok := r.LastCall("old", "c"); ok {
		t.Error("a pair older than 20 minutes survived the prune")
	}
	if lc, ok := r.LastCall("new", "c"); !ok || lc.Outcome != Served {
		t.Errorf("the fresh pair is gone: %+v", lc)
	}
	for i := 0; i < shards*maxPairsPerShard+500; i++ {
		r.Record(Call{Outcome: Served, UserID: fmt.Sprint("u", i), ClientID: "c", Function: "f", At: now})
	}
	if n := r.ActivePairs(); n > shards*maxPairsPerShard {
		t.Fatalf("pairs = %d, cap %d", n, shards*maxPairsPerShard)
	}
}

func TestUpperBoundMS(t *testing.T) {
	hist := make([]int64, HistBuckets)
	hist[0], hist[4], hist[11] = 98, 1, 1
	if p := UpperBoundMS(hist, 0.5); p == nil || *p != 1 {
		t.Errorf("p50 = %v", p)
	}
	if p := UpperBoundMS(hist, 0.99); p == nil || *p != 25 {
		t.Errorf("p99 = %v", p)
	}
	if p := UpperBoundMS(hist, 1); p != nil {
		t.Errorf("p100 in +Inf must be nil, got %v", *p)
	}
	if p := UpperBoundMS(make([]int64, HistBuckets), 0.5); p != nil {
		t.Errorf("no calls must be nil")
	}
}

const payloadA = `{"function":"function/preaccesstoken","user":{"id":"111","human":{"email":"ada@example.edu"}},
 "userinfo":{"email":"ada@example.edu","name":"Ada"},"user_grants":[{"project_id":"pNet","roles":["staff"],"user_id":"111"}],
 "application":{"client_id":"c1"},"org":{"id":"o1","name":"Lab"}}`

// The fingerprint follows the payload's contract, not the person: another
// person with different userinfo and metadata has the same shape; a renamed
// grant key does not.
func TestFingerprint_FollowsTheContractNotThePerson(t *testing.T) {
	other := `{"function":"function/preaccesstoken","user":{"id":"222","human":{"email":"b@example.edu"}},
 "userinfo":{"email":"b@example.edu","given_name":"B","locale":"en"},"user_metadata":[{"key":"team","value":"eA=="}],
 "user_grants":[{"project_id":"pDoor","roles":["a","b"],"user_id":"222"}],"application":{"client_id":"c2"},"org":{"id":"o1","name":"Lab"}}`
	renamed := strings.Replace(payloadA, `"project_id"`, `"projectId"`, 1)
	fa, fo := Fingerprint(Paths([]byte(payloadA))), Fingerprint(Paths([]byte(other)))
	if fa != Fingerprint(Paths([]byte(strings.Replace(payloadA, "staff", "lead", 1)))) {
		t.Error("a value change moved the fingerprint")
	}
	if fo == fa {
		// user_metadata is a new top-level key in `other`; that is a shape
		// change. Without it the two must match.
		t.Error("user_metadata present vs absent should differ at the top level")
	}
	otherNoMeta := strings.Replace(other, `"user_metadata":[{"key":"team","value":"eA=="}],`, "", 1)
	if Fingerprint(Paths([]byte(otherNoMeta))) != fa {
		t.Errorf("a different person, same payload contract, got another fingerprint:\n%v\n%v", Paths([]byte(otherNoMeta)), Paths([]byte(payloadA)))
	}
	if Fingerprint(Paths([]byte(renamed))) == fa {
		t.Error("renaming user_grants[].project_id must change the fingerprint")
	}
	if Paths([]byte(`[1,2]`)) != nil || Paths([]byte(`nope`)) != nil {
		t.Error("a non-object body has no fingerprint")
	}
}

// The stored example carries the contract and nothing about the person.
func TestRedacted_KeepsTheContractDropsThePerson(t *testing.T) {
	ex := string(Redacted([]byte(payloadA)))
	for _, pii := range []string{"ada@example.edu", "Ada", "111", "Lab"} {
		if strings.Contains(ex, pii) {
			t.Errorf("example still holds %q: %s", pii, ex)
		}
	}
	for _, keep := range []string{`"function/preaccesstoken"`, `"pNet"`, `"staff"`, `"c1"`, `"user_grants"`, `"email"`} {
		if !strings.Contains(ex, keep) {
			t.Errorf("example lost %s: %s", keep, ex)
		}
	}
}

// A new shape is reported once per process, with its redacted example.
func TestRecord_NewShapeQueuedOnce(t *testing.T) {
	r := New(fixed(t0))
	for i := 0; i < 3; i++ {
		r.Record(Call{Outcome: Served, Function: "function/preaccesstoken", Body: []byte(payloadA), At: t0})
	}
	c := &capture{}
	_ = r.Flush(context.Background(), c.write)
	fps := c.batches[0].Fingerprints
	if len(fps) != 1 || fps[0].Example == nil || strings.Contains(string(fps[0].Example), "ada@") {
		t.Fatalf("fingerprints = %+v", fps)
	}
}

// Concurrent calls and flushes: run with -race. Every call is counted once.
func TestRecorder_ConcurrentRecordAndFlush(t *testing.T) {
	r := New(time.Now)
	c := &capture{}
	const workers, per = 8, 500
	var wg sync.WaitGroup
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
				_ = r.Flush(context.Background(), c.write)
			}
		}
	}()
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < per; i++ {
				r.Record(Call{Outcome: Served, ProjectID: "p", ClientID: fmt.Sprint("c", w%3), Function: "f",
					UserID: fmt.Sprint("u", i), Body: []byte(payloadA)})
				_ = r.Recent(5)
				_ = r.ActivePairs()
			}
		}(w)
	}
	wg.Wait()
	close(stop)
	_ = r.Flush(context.Background(), c.write)
	var n int64
	for _, v := range c.total() {
		n += v.N
	}
	if n != workers*per {
		t.Fatalf("counted %d calls, want %d", n, workers*per)
	}
}

func TestOutcomes_SeverityCoversEveryClass(t *testing.T) {
	seen := map[Outcome]bool{}
	for _, o := range Outcomes {
		if seen[o] {
			t.Fatalf("%s listed twice", o)
		}
		seen[o] = true
	}
	if len(seen) != 17 {
		t.Fatalf("%d classes, want the 16 of assessment §4a and served_welcome (SC4)", len(seen))
	}
	if ServedWelcome.Severity() != Informational || Rank(ServedWelcome) > Rank(ServedLKG) {
		t.Error("a welcome answer is delivered, not a fault")
	}
	if SigMissing.Severity() != Informational || SigMismatch.Severity() != Hard || DegradedNoFacts.Severity() != Soft {
		t.Error("severity table changed")
	}
}
