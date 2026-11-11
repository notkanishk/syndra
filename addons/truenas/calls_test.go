package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"
)

// Contract 2's call rules (design §5, task 1.4), on all four kinds of write.

// The add-on's own canonical form and digest reproduce the contract's four JCS
// vectors byte for byte, and two spellings of one value are one identity.
func TestRequestIdentityReproducesTheContractVectors(t *testing.T) {
	for _, kind := range []string{"apply", "operation", "lifecycle", "setting"} {
		var vec struct {
			Input     any    `json:"input"`
			Canonical string `json:"canonical"`
			SHA256    string `json:"sha256"`
		}
		dec := json.NewDecoder(bytes.NewReader(v2Read(t, "fixtures", "identity_"+kind+".json")))
		dec.UseNumber()
		if err := dec.Decode(&vec); err != nil {
			t.Fatal(err)
		}
		if got := string(canonicalJSON(vec.Input)); got != vec.Canonical {
			t.Errorf("%s: canonical\n got  %s\n want %s", kind, got, vec.Canonical)
		}
		if got := identityDigest(vec.Input); got != vec.SHA256 {
			t.Errorf("%s: digest %s, want %s", kind, got, vec.SHA256)
		}
	}
	a := map[string]json.RawMessage{"group": json.RawMessage(`[ "lab_makers" ]`), "enabled": json.RawMessage(`true`)}
	b := map[string]json.RawMessage{"enabled": json.RawMessage(`true`), "group": json.RawMessage(`["lab_makers"]`)}
	if identityDigest(plainJSON(a)) != identityDigest(plainJSON(b)) {
		t.Error("two spellings of one desired state are two identities")
	}
	// RFC 8785's corners: raw <, >, & and U+2028; keys by UTF-16 code unit
	// (U+1F600 sorts before U+FB33 there, after it by UTF-8); numbers as a
	// double prints in ECMAScript.
	got := string(canonicalJSON(map[string]any{"\U0001F600": 1, "דּ": 2, "r": "<a&b> \n",
		"n": []any{json.Number("1e21"), json.Number("1e-7"), json.Number("0.000001"), json.Number("-0"), json.Number("100")}}))
	want := "{\"n\":[1e+21,1e-7,0.000001,0,100],\"r\":\"<a&b> \\n\",\"\U0001F600\":1,\"דּ\":2}"
	if got != want {
		t.Errorf("canonical\n got  %s\n want %s", got, want)
	}
}

// Rule 3, the planner's case: an indeterminate row is re-planned before it is
// re-sent under the same call_id, so its fingerprint has moved. The same
// identity is answered from the record, byte for byte, BEFORE the fingerprint
// is checked, and nothing is written again.
func TestAReplayIsAnsweredBeforeTheFingerprint(t *testing.T) {
	s, m := applyServer(t, `[{"username":"charlotte","id":11,"uid":3001,"locked":true,"smb":false,"groups":[42]}]`)
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "charlotte", UID: 3001}); err != nil {
		t.Fatal(err)
	}
	body := `{"call_id":"row-1","subject":"sub-1","email":"charlotte@example.edu","desired":{"enabled":true,"group":["lab_makers"]}}`
	first := postApply(t, s, body)
	if first.Code != http.StatusOK || len(m.updates) != 1 {
		t.Fatalf("first apply: %d %s (%d updates)", first.Code, first.Body, len(m.updates))
	}
	// The re-plan sees the converged account, so the fingerprint is new; the
	// desired state is the same, spelled differently.
	replay := postApply(t, s, `{"call_id":"row-1","subject":"sub-1","email":"another@example.edu","plan_id":"p-2",
		"fingerprint":"a-new-fingerprint","desired":{ "group" : ["lab_makers"], "enabled" : true }}`)
	if replay.Code != http.StatusOK || replay.Body.String() != first.Body.String() || len(m.updates) != 1 {
		t.Fatalf("a replay must be the recorded reply, before the fingerprint, writing nothing:\n first  %s\n replay %d %s (%d updates)",
			first.Body, replay.Code, replay.Body, len(m.updates))
	}
	// Rule 2: the call_id for another desired state is another request.
	other := postApply(t, s, `{"call_id":"row-1","subject":"sub-1","fingerprint":"f","desired":{"enabled":false}}`)
	if other.Code != http.StatusUnprocessableEntity || problemCode(t, other) != CodeCallIDReused {
		t.Fatalf("another identity under the call_id: want 422 CALL_ID_REUSED, got %d %s", other.Code, other.Body)
	}
}

// The second test case task 1.4 names: a password.set re-sent with a different
// password is a replay of the first (the secret is outside the identity), and
// the store holds neither password nor a hash of either.
func TestAPasswordReplayWithAnotherPasswordIsTheFirstCall(t *testing.T) {
	s, m := opServer(t)
	canary, other := "canary-"+strconv.FormatInt(time.Now().UnixNano(), 36), "another-password-entirely"
	first := postOperation(t, s, "password.set", `{"call_id":"pw-1","operation":"password.set","subject":"sub-1","params":{"password":"`+canary+`"}}`)
	again := postOperation(t, s, "password.set", `{"call_id":"pw-1","operation":"password.set","subject":"sub-1","params":{"password":"`+other+`"}}`)
	if first.Code != http.StatusOK || again.Body.String() != first.Body.String() || len(m.updates) != 1 {
		t.Fatalf("want the recorded reply and one write:\n first %d %s\n again %d %s (%d updates)", first.Code, first.Body, again.Code, again.Body, len(m.updates))
	}
	path := s.store.db.Path()
	if err := s.store.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{canary, other} {
		sum := sha256.Sum256([]byte(secret))
		for _, form := range []string{secret, hex.EncodeToString(sum[:]), string(sum[:])} {
			if bytes.Contains(raw, []byte(form)) {
				t.Fatalf("the store file holds a password or its hash")
			}
		}
	}
	reopened, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	s.store = reopened
	t.Cleanup(func() { reopened.Close() })
}

// Rule 1: in flight is this process, now. A second request under a call_id
// whose first is still running is 409 CALL_IN_PROGRESS with Retry-After.
func TestACallStillRunningIsInProgress(t *testing.T) {
	users := `[{"username":"charlotte","id":11,"uid":3001,"locked":false,"smb":true,"groups":[42]}]`
	b := &blockingRPC{
		mutatingRPC: mutatingRPC{fakeRPC: fakeRPC{users: users, groups: fixtureGroups}},
		entered:     make(chan struct{}, 1),
		release:     make(chan struct{}),
	}
	s := testServer(t, &b.fakeRPC)
	s.nas = newNAS(func() (rpc, error) { return b, nil }, []string{"25.04"})
	s.nas.version, s.nas.probed = "25.04.2", true
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "charlotte", UID: 3001}); err != nil {
		t.Fatal(err)
	}
	body := `{"call_id":"pw-1","operation":"password.set","subject":"sub-1","params":{"password":"p"}}`
	done := make(chan *httptest.ResponseRecorder)
	go func() { done <- postOperation(t, s, "password.set", body) }()
	<-b.entered
	busy := postOperation(t, s, "password.set", body)
	if busy.Code != http.StatusConflict || problemCode(t, busy) != CodeCallInProgress || busy.Header().Get("Retry-After") == "" {
		t.Errorf("a call still running: want 409 CALL_IN_PROGRESS with Retry-After, got %d %s", busy.Code, busy.Body)
	}
	other := postOperation(t, s, "password.set", `{"call_id":"pw-1","operation":"password.set","subject":"sub-2","params":{"password":"p"}}`)
	if other.Code != http.StatusUnprocessableEntity || problemCode(t, other) != CodeCallIDReused {
		t.Errorf("another request under a running call_id: want 422 CALL_ID_REUSED, got %d %s", other.Code, other.Body)
	}
	close(b.release)
	if first := <-done; first.Code != http.StatusOK {
		t.Fatalf("the first call: %d %s", first.Code, first.Body)
	}
	if after := postOperation(t, s, "password.set", body); after.Code != http.StatusOK {
		t.Errorf("once it has answered, the lease is released and the call replays: %d %s", after.Code, after.Body)
	}
}

// After a crash between the write and its record, nothing is in flight: the
// re-send is executed (and finds the account by lookup before create), never
// answered 409 for ever because a log intent has no result.
func TestAReSendAfterACrashIsExecutedNotInProgress(t *testing.T) {
	s, m := applyServer(t, `[]`)
	body := `{"call_id":"row-9","subject":"sub-1","email":"charlotte@example.edu","desired":{"enabled":true,"group":["lab_makers"]}}`
	if first := postApply(t, s, body); first.Code != http.StatusOK {
		t.Fatalf("create: %d %s", first.Code, first.Body)
	}
	// The crash: the record of the reply is lost, the account and its binding
	// are not; a new process has no leases.
	if err := s.store.db.Update(func(tx *bolt.Tx) error { return tx.Bucket(bucketIdempotency).Delete([]byte("row-9")) }); err != nil {
		t.Fatal(err)
	}
	s.calls = callLeases{}
	creates := len(m.creates)
	again := postApply(t, s, body)
	out := decodeOutcome(t, again)
	if again.Code != http.StatusOK || out.Effect != EffectNoChange || len(m.creates) != creates {
		t.Fatalf("the re-send must run and find the account it made: %d %s (%d creates)", again.Code, again.Body, len(m.creates)-creates)
	}
}

// Rule 5: an entry contract 1 recorded has no digest and replays whatever the
// identity, as an entry with no kind always did, in contract 2's words.
func TestAContractOneEntryReplaysInContractTwoWords(t *testing.T) {
	s, m := applyServer(t, `[{"username":"charlotte","id":11,"uid":3001,"locked":false,"smb":true,"groups":[42],"password_disabled":true}]`)
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "charlotte", UID: 3001}); err != nil {
		t.Fatal(err)
	}
	snap, err := s.readSubjects()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.store.PutSnapshot(snap); err != nil {
		t.Fatal(err)
	}
	put := func(callID, kind, result string) {
		entry := fmt.Sprintf(`{"stored_at":%q,"kind":%q,"result":%s}`, time.Now().UTC().Format(time.RFC3339Nano), kind, result)
		if err := s.store.db.Update(func(tx *bolt.Tx) error {
			return tx.Bucket(bucketIdempotency).Put([]byte(callID), []byte(entry))
		}); err != nil {
			t.Fatal(err)
		}
	}
	put("v1-apply", kindApply, `{"subject":"sub-1","effect":"applied","detail":"Done.","username":"charlotte","uid":3001,"fingerprint":"f1"}`)
	put("v1-adopt", kindOperation+"account.adopt", `{"operation":"account.adopt","subject":"sub-1","outcome":"succeeded","account_uid":3001}`)

	rr := postApply(t, s, `{"call_id":"v1-apply","subject":"sub-1","fingerprint":"anything","desired":{"enabled":true}}`)
	out := decodeOutcome(t, rr)
	if rr.Code != http.StatusOK || out.AccountID != "3001" || out.Standing == nil || out.Standing.WaitingOn != "member" || len(m.updates) != 0 {
		t.Fatalf("a contract-1 apply entry must replay with account_id and standing, writing nothing: %d %s", rr.Code, rr.Body)
	}
	if strings.Contains(rr.Body.String(), `"uid"`) {
		t.Errorf("the replay still speaks contract 1: %s", rr.Body)
	}
	assertSchema(t, "apply_ok.json", rr.Body.Bytes(), "a contract-1 apply replayed")
	op := postOperation(t, s, "account.adopt", `{"call_id":"v1-adopt","operation":"account.adopt","subject":"sub-1","params":{"username":"charlotte"}}`)
	if op.Code != http.StatusOK || !strings.Contains(op.Body.String(), `"account_id":"3001"`) || strings.Contains(op.Body.String(), "account_uid") {
		t.Fatalf("a contract-1 adopt entry must replay with account_id: %d %s", op.Code, op.Body)
	}
}

// Lifecycle carries a call_id like every write, and a re-sent change is the
// recorded reply: a state set since is not undone by a stale re-send.
func TestALifecycleReSendIsTheRecordedReply(t *testing.T) {
	s, _ := applyServer(t, fixtureUsers)
	set := func(callID, state string) *httptest.ResponseRecorder {
		rr := httptest.NewRecorder()
		postTo(s.handleLifecycle, rr, "/lifecycle", fmt.Sprintf(`{"contract_version":2,"call_id":%q,"state":%q,"reason":"maintenance"}`, callID, state))
		return rr
	}
	first := set("lc-1", LifecycleReadOnly)
	if first.Code != http.StatusOK {
		t.Fatalf("%d %s", first.Code, first.Body)
	}
	if rr := set("lc-2", LifecycleActive); rr.Code != http.StatusOK {
		t.Fatalf("%d %s", rr.Code, rr.Body)
	}
	if again := set("lc-1", LifecycleReadOnly); again.Body.String() != first.Body.String() {
		t.Fatalf("a re-send must be the recorded reply: %s", again.Body)
	}
	if state, _ := s.life.State(); state != LifecycleActive {
		t.Errorf("a re-sent change must not run again; the add-on is %s", state)
	}
	if rr := set(" ", LifecycleActive); rr.Code != http.StatusBadRequest || problemCode(t, rr) != CodeBadRequest {
		t.Errorf("a lifecycle change without a call_id: %d %s", rr.Code, rr.Body)
	}
}

// The store keeps a call at least as long as the contract says (constants
// call_memory, P30D), and the sweep is what forgets it.
func TestCallMemoryIsAtLeastTheContractConstant(t *testing.T) {
	var c struct {
		Constants map[string]struct {
			Value any `json:"value"`
		} `json:"constants"`
	}
	if err := json.Unmarshal(v2Read(t, "registry", "constants.json"), &c); err != nil {
		t.Fatal(err)
	}
	memory, _ := c.Constants["call_memory"].Value.(string)
	days, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(memory, "P"), "D"))
	if err != nil {
		t.Fatalf("call_memory %q is not P<n>D", c.Constants["call_memory"].Value)
	}
	if idempotencyTTL < time.Duration(days)*24*time.Hour {
		t.Errorf("calls are remembered %v, the contract says at least %d days", idempotencyTTL, days)
	}
}

// Rollback restores nothing (decision 10, task 1.7): a contract-1 binary reads
// the store this one wrote — every binding by uid, every call by kind — with
// the types it decoded them with.
func TestAContractOneReaderOpensThisStore(t *testing.T) {
	s, _ := opServer(t)
	if rr := postOperation(t, s, "password.set", `{"call_id":"c1","operation":"password.set","subject":"sub-1","params":{"password":"x"}}`); rr.Code != http.StatusOK {
		t.Fatal(rr.Body.String())
	}
	path := s.store.db.Path()
	s.store.Close()
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: time.Second, ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// Contract 1's types, as its store.go declared them.
	type v1Binding struct {
		SubjectID string `json:"subject_id"`
		Username  string `json:"username"`
		UID       int64  `json:"uid"`
	}
	type v1Entry struct {
		Kind   string          `json:"kind"`
		Result json.RawMessage `json:"result"`
	}
	err = db.View(func(tx *bolt.Tx) error {
		var b v1Binding
		if err := json.Unmarshal(tx.Bucket([]byte("bindings")).Get([]byte("sub-1")), &b); err != nil || b.UID != 3001 || b.Username != "charlotte" {
			return fmt.Errorf("a contract-1 reader does not find the binding: %+v %v", b, err)
		}
		var e v1Entry
		if err := json.Unmarshal(tx.Bucket([]byte("idempotency")).Get([]byte("c1")), &e); err != nil || e.Kind != kindOperation+"password.set" || len(e.Result) == 0 {
			return fmt.Errorf("a contract-1 reader does not find the call: %+v %v", e, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if raw, _ := os.ReadFile(filepath.Join(filepath.Dir(path), "state.db")); !bytes.Contains(raw, []byte(`"account_id":"3001"`)) {
		t.Error("the binding is not written with account_id beside uid")
	}
}

// A binding a later binary wrote with account_id alone is read by it.
func TestABindingWithAccountIDAloneIsRead(t *testing.T) {
	s, _ := applyServer(t, fixtureUsers)
	if err := s.store.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketBindings).Put([]byte("sub-7"), []byte(`{"subject_id":"sub-7","username":"seth","account_id":"3002"}`))
	}); err != nil {
		t.Fatal(err)
	}
	b, found, err := s.store.GetBinding("sub-7")
	if err != nil || !found || b.UID != 3002 {
		t.Fatalf("got %+v %v %v", b, found, err)
	}
}

// Every route not served, authenticated like the rest, is a registered 404.
func TestAnUnknownRouteIsARegisteredRefusal(t *testing.T) {
	s, _ := applyServer(t, fixtureUsers)
	now := time.Now()
	s.auth = signedAuth(now)
	for _, target := range []string{"/stats?metric=x", "/calls", "/capabilities/extra"} {
		r := httptest.NewRequest(http.MethodGet, target, nil)
		r.Header.Set(SignatureHeader, fmt.Sprintf("t=%d,v1=%x", now.Unix(), computeMAC(now.Unix(), http.MethodGet, signedTarget(r.URL), nil, []byte(testKey))))
		rr := httptest.NewRecorder()
		s.routes().ServeHTTP(rr, r)
		if rr.Code != http.StatusNotFound || problemCode(t, rr) != CodeUnknownRoute {
			t.Errorf("%s: %d %s", target, rr.Code, rr.Body)
		}
	}
}

// An account in no group reports [] in state, never null (task 1.0a): a list
// field is a list.
func TestAnAccountInNoGroupReportsAnEmptyList(t *testing.T) {
	s := testServer(t, &fakeRPC{users: `[{"username":"bianca","id":13,"uid":3003,"locked":false,"smb":false,"groups":[41]}]`, groups: fixtureGroups})
	rr := httptest.NewRecorder()
	s.handleSubjects(rr, httptest.NewRequest(http.MethodGet, "/subjects", nil), nil)
	if !strings.Contains(rr.Body.String(), `"group":[]`) || strings.Contains(rr.Body.String(), "null") {
		t.Fatalf("an account in no group must report \"group\":[]: %s", rr.Body)
	}
}

// Two applies for one subject under different call_ids run one at a time
// (rule 5). Interleaved, A makes the personal group, B finds it as an empty
// orphan and creates the account in it, A's create is refused, and A's cleanup
// deletes the primary group of B's account. The fake stalls A's create with
// the socket free (as if A paused between its two calls) and refuses it, so
// that interleaving happens whenever the two are allowed to overlap.
func TestTwoAppliesForOneSubjectRunOneAtATime(t *testing.T) {
	s, m := applyServer(t, `[]`)
	body := func(callID string) string {
		return withContractVersion(t, `{"call_id":"`+callID+`","subject":"sub-1","email":"charlotte@example.org",`+
			`"fingerprint":"`+fingerprintSubject(nil)+`","desired":{"group":["lab_makers"]}}`)
	}
	aStalled, bCreating := make(chan struct{}), make(chan struct{}, 1)
	creates, overlapped := 0, false
	m.beforeUserCreate = func() bool {
		if creates++; creates > 1 {
			bCreating <- struct{}{}
			return false
		}
		s.nas.callMu.Unlock()
		close(aStalled)
		select {
		case <-bCreating:
			overlapped = true
		case <-time.After(300 * time.Millisecond):
		}
		s.nas.callMu.Lock()
		return true
	}

	var wg sync.WaitGroup
	replies := make([]*httptest.ResponseRecorder, 2)
	apply := func(i int, callID string) {
		defer wg.Done()
		rr := httptest.NewRecorder()
		b := body(callID)
		s.handleApply(rr, httptest.NewRequest(http.MethodPost, "/apply", strings.NewReader(b)), []byte(b))
		replies[i] = rr
	}
	wg.Add(2)
	go apply(0, "apply-a")
	<-aStalled
	go apply(1, "apply-b")
	wg.Wait()

	if overlapped {
		t.Error("two applies for one subject ran at the same time")
	}
	if replies[0].Code < 500 || replies[1].Code != http.StatusOK {
		t.Fatalf("want A failed and B applied, got %d %s / %d %s",
			replies[0].Code, replies[0].Body.String(), replies[1].Code, replies[1].Body.String())
	}
	// B's account and the primary group it was created in are both there.
	var rows []struct {
		ID    int64   `json:"id"`
		Group string  `json:"group"`
		Users []int64 `json:"users"`
	}
	if err := json.Unmarshal([]byte(m.fakeRPC.groups), &rows); err != nil {
		t.Fatal(err)
	}
	primary := fmt.Sprint(m.creates[len(m.creates)-1]["group"])
	for _, g := range rows {
		if fmt.Sprint(g.ID) == primary && g.Group == "charlotte" && len(g.Users) == 1 {
			return
		}
	}
	t.Fatalf("B's primary group %s is gone: %s", primary, m.fakeRPC.groups)
}

// Rule 5 holds for every write to a subject's account, not only apply: an
// operation waits while another write for the same subject runs.
func TestAnOperationWaitsForTheSubjectsWriteInProgress(t *testing.T) {
	for _, op := range []string{"password.set", "account.release"} {
		s, _ := opServer(t)
		unlock := s.subjects.lock("sub-1")
		done := make(chan int, 1)
		go func() {
			done <- postOperation(t, s, op, `{"call_id":"c1","operation":"`+op+`","subject":"sub-1","params":{"password":"x"}}`).Code
		}()
		select {
		case code := <-done:
			t.Fatalf("%s ran beside another write for its subject (%d)", op, code)
		case <-time.After(100 * time.Millisecond):
		}
		unlock()
		if code := <-done; code != http.StatusOK {
			t.Errorf("%s: want 200 once its turn came, got %d", op, code)
		}
	}
}

// The lifecycle gate comes after the subject lock: a write waiting its turn
// behind another for the same subject, whose turn comes after read_only was
// set, is 503 LIFECYCLE_REFUSAL and touches nothing, so what a drain waits for
// is only the write already running, never a queue behind it.
func TestAWriteWhoseTurnComesInReadOnlyIsRefused(t *testing.T) {
	s, m := opServer(t)
	unlock := s.subjects.lock("sub-1") // the first write, running
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		done <- postOperation(t, s, "password.set", `{"call_id":"c2","operation":"password.set","subject":"sub-1","params":{"password":"x"}}`)
	}()
	time.Sleep(100 * time.Millisecond) // the second is waiting its turn
	if err := s.life.Set(LifecycleReadOnly, "maintenance"); err != nil {
		t.Fatal(err)
	}
	calls := len(m.fakeRPC.calls)
	unlock()
	rr := <-done
	if rr.Code != http.StatusServiceUnavailable || problemCode(t, rr) != CodeLifecycleRefusal {
		t.Fatalf("want 503 LIFECYCLE_REFUSAL, got %d %s", rr.Code, rr.Body.String())
	}
	if len(m.updates) != 0 || len(m.fakeRPC.calls) != calls {
		t.Fatalf("a refused write touched the NAS: %v", m.fakeRPC.calls[calls:])
	}
	if s.life.InFlight() != 0 {
		t.Errorf("a refused write must not count in flight, got %d", s.life.InFlight())
	}
}
