package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// 4.8–4.12 — the read path, and the one thing that must never come back with it.

// fakeRPC answers `user.query`, `group.query` and `system.version` from
// fixtures, so every behaviour worth testing here is testable without a NAS.
type fakeRPC struct {
	users   string
	groups  string
	version string
	err     error
	calls   []string
	params  []any
}

func (f *fakeRPC) Call(method string, _ int64, params any) (json.RawMessage, error) {
	f.calls = append(f.calls, method)
	f.params = append(f.params, params)
	if f.err != nil {
		return nil, f.err
	}
	switch method {
	case "user.query":
		return json.RawMessage(f.users), nil
	case "group.query":
		return json.RawMessage(f.groups), nil
	case "system.version":
		return json.RawMessage(`"` + f.version + `"`), nil
	}
	return json.RawMessage(`null`), nil
}
func (f *fakeRPC) Ping() (string, error) { return "pong", f.err }
func (f *fakeRPC) Close() error          { return nil }

func testServer(t *testing.T, fake *fakeRPC) *server {
	t.Helper()
	dir := t.TempDir()
	store, err := OpenStore(filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	mlog, err := OpenMutationLog(dir, 1<<20, 4)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { mlog.Close() })

	nas := newNAS(func() (rpc, error) { return fake, nil }, []string{"25.04"})
	nas.version = "25.04.2"
	return &server{
		auth: &authenticator{now: time.Now},
		nas:  nas, store: store, log: mlog,
		life:    newLifecycle(LifecycleActive),
		product: "truenas_scale",
	}
}

const (
	fixtureUsers = `[
		{"username":"ada","uid":3001,"locked":false,"smb":true,"groups":[545,900]},
		{"username":"leo","uid":3002,"locked":true,"smb":false,"groups":[900]}
	]`
	fixtureGroups = `[{"gid":545,"group":"builtin_users"},{"gid":900,"group":"lab_makers"}]`
)

// The hash fields are the reason the query names its columns. `user.query`
// returns `unixhash` and `smbhash`, and an NT hash is a pass-the-hash
// credential: possessing one is equivalent to possessing the password for SMB.
func TestNoHashFieldIsRequestedOrReturnedOnAnyPath(t *testing.T) {
	rpc := &fakeRPC{
		// The NAS offering them anyway, which is exactly what it does.
		users:  `[{"username":"ada","uid":3001,"locked":false,"smb":true,"groups":[900],"unixhash":"$6$deadbeef","smbhash":"AAD3B435B51404EE"}]`,
		groups: fixtureGroups,
	}
	s := testServer(t, rpc)

	rr := httptest.NewRecorder()
	s.handleSubjects(rr, httptest.NewRequest(http.MethodGet, "/subjects", nil), nil)

	body := rr.Body.String()
	for _, forbidden := range []string{"unixhash", "smbhash", "deadbeef", "AAD3B435B51404EE"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("the response carries %q — that is a credential", forbidden)
		}
	}

	// And the select asks for the fields by name, so they are absent by
	// construction rather than stripped afterwards. Stripping is a step
	// somebody can forget, and the forgetting would not be visible.
	sel := selectedFields(t, rpc, "user.query")
	for _, forbidden := range []string{"unixhash", "smbhash"} {
		for _, got := range sel {
			if got == forbidden {
				t.Errorf("the query must not select %q", forbidden)
			}
		}
	}
	if len(sel) == 0 {
		t.Fatal("the query must pass an explicit select, or the NAS returns everything it has")
	}

	// The snapshot is the other durable place one could land.
	snap, found, err := s.store.GetSnapshot()
	if err != nil || !found {
		t.Fatalf("snapshot: %v found=%t", err, found)
	}
	encoded, _ := json.Marshal(snap)
	for _, forbidden := range []string{"unixhash", "smbhash", "deadbeef"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Errorf("the snapshot carries %q", forbidden)
		}
	}
}

func selectedFields(t *testing.T, rpc *fakeRPC, method string) []string {
	t.Helper()
	for i, m := range rpc.calls {
		if m != method {
			continue
		}
		args, ok := rpc.params[i].([]any)
		if !ok || len(args) < 2 {
			return nil
		}
		opts, ok := args[1].(map[string]any)
		if !ok {
			return nil
		}
		sel, _ := opts["select"].([]string)
		return sel
	}
	return nil
}

// Group ids are resolved to names, because a mapping binds a role to a group
// NAME and comparing a name against a number would make every subject drift.
func TestGroupsAreResolvedToNames(t *testing.T) {
	s := testServer(t, &fakeRPC{users: fixtureUsers, groups: fixtureGroups})

	rr := httptest.NewRecorder()
	s.handleSubjects(rr, httptest.NewRequest(http.MethodGet, "/subjects", nil), nil)

	var got SubjectsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Subjects) != 2 {
		t.Fatalf("want 2 subjects, got %d", len(got.Subjects))
	}
	if strings.Join(got.Subjects[0].Groups, ",") != "builtin_users,lab_makers" {
		t.Errorf("groups must be names, got %v", got.Subjects[0].Groups)
	}
	// `locked` is the NAS's word and `enabled` is Syndra's; the translation
	// belongs here, because the backend does not know what TrueNAS calls things.
	if !got.Subjects[0].Enabled || got.Subjects[1].Enabled {
		t.Errorf("locked must invert into enabled: %+v", got.Subjects)
	}
	if !got.Current {
		t.Error("a live read must be marked current")
	}
}

// 4.12 — the snapshot serves a stale read WITH its age. The backend's drift
// sweep consumes only current reads; an ageing mirror served as current would
// make every outage manufacture findings.
func TestAStaleReadIsServedAndLabelledRatherThanFailed(t *testing.T) {
	rpc := &fakeRPC{users: fixtureUsers, groups: fixtureGroups}
	s := testServer(t, rpc)

	first := httptest.NewRecorder()
	s.handleSubjects(first, httptest.NewRequest(http.MethodGet, "/subjects", nil), nil)
	if first.Code != http.StatusOK {
		t.Fatalf("priming read failed: %s", first.Body.String())
	}

	rpc.err = errors.New("websocket: connection closed")
	s.nas.drop()
	// Past the reconnect cooldown check, which would otherwise be what refuses.
	s.nas.lastTry = time.Now().Add(-time.Hour)

	rr := httptest.NewRecorder()
	s.handleSubjects(rr, httptest.NewRequest(http.MethodGet, "/subjects", nil), nil)

	var got SubjectsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if rr.Code != http.StatusOK || len(got.Subjects) != 2 {
		t.Fatalf("the mirror must answer: %d %s", rr.Code, rr.Body.String())
	}
	if got.Current {
		t.Fatal("a mirrored read must NOT be marked current, or the sweep diffs against an ageing copy")
	}
	if got.TakenAt == "" {
		t.Error("and it must say how old it is")
	}
}

// Nothing current and nothing remembered is not an empty target. An empty list
// is a statement that the NAS holds no accounts, and the drift sweep would act
// on it.
func TestNoReadAndNoMirrorIsRefusedRatherThanAnsweredEmpty(t *testing.T) {
	s := testServer(t, &fakeRPC{err: errors.New("websocket: connection closed")})

	rr := httptest.NewRecorder()
	s.handleSubjects(rr, httptest.NewRequest(http.MethodGet, "/subjects", nil), nil)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d (%s)", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), `"subjects":[]`) {
		t.Error("an empty list would be read as a target with no accounts")
	}
}

// The capability set must still be servable when the NAS is unreachable: one
// that vanished during an outage would make the backend withdraw operations
// that are merely unobservable.
func TestCapabilitiesAnswerWhileTheTargetIsDown(t *testing.T) {
	s := testServer(t, &fakeRPC{err: errors.New("websocket: connection closed")})

	rr := httptest.NewRecorder()
	s.handleCapabilities(rr, httptest.NewRequest(http.MethodGet, "/capabilities", nil), nil)

	var m Manifest
	if err := json.Unmarshal(rr.Body.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m.ContractVersion != ContractVersion {
		t.Errorf("contract version = %d", m.ContractVersion)
	}
	if len(m.Operations) == 0 || len(m.EntitlementSchema) == 0 {
		t.Fatal("the manifest must be served whole")
	}
	// Lifecycle fields are declared and flagged, so the backend refuses a
	// role-to-target mapping naming one.
	var flagged int
	for _, f := range m.EntitlementSchema {
		if f.Lifecycle {
			flagged++
		}
	}
	if flagged != 2 {
		t.Errorf("enabled and smb_enabled must both be flagged lifecycle, got %d", flagged)
	}
}

// 4.5 — an untested major refuses mutations and says so through health, while
// reads continue. Refusing reads too would make the backend record the target
// as unreconciled: "we cannot see it" when the truth is "we will not write".
func TestAnUntestedMajorRefusesMutationsAndKeepsReading(t *testing.T) {
	s := testServer(t, &fakeRPC{users: fixtureUsers, groups: fixtureGroups})
	s.nas.version = "27.10.0"

	supported, why := s.nas.MajorSupported()
	if supported {
		t.Fatal("an untested major must not be supported")
	}
	if !strings.Contains(why, "27.10") {
		t.Errorf("the reason must name the version it saw, got %q", why)
	}
	for _, op := range operationSet(s) {
		if op.Available {
			t.Errorf("%s must be unavailable on an untested major", op.ID)
		}
		if op.UnavailableReason == "" {
			t.Errorf("%s must say why, or a disabled button explains nothing", op.ID)
		}
	}

	rr := httptest.NewRecorder()
	s.handleSubjects(rr, httptest.NewRequest(http.MethodGet, "/subjects", nil), nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("reads must continue on an untested major, got %d", rr.Code)
	}
}

// A capped read is CURRENT and still cannot support a conclusion about absence,
// which is half of what the drift diff does with it. So the flag travels
// separately.
func TestATruncatedReadSaysSoWithoutClaimingToBeStale(t *testing.T) {
	var b strings.Builder
	b.WriteString("[")
	for i := range subjectReadCap + 1 {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`{"username":"u`)
		b.WriteString(strconv.Itoa(i))
		b.WriteString(`","uid":`)
		b.WriteString(strconv.Itoa(4000 + i))
		b.WriteString(`,"locked":false,"smb":true,"groups":[900]}`)
	}
	b.WriteString("]")

	s := testServer(t, &fakeRPC{users: b.String(), groups: fixtureGroups})

	rr := httptest.NewRecorder()
	s.handleSubjects(rr, httptest.NewRequest(http.MethodGet, "/subjects", nil), nil)

	var got SubjectsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !got.Truncated {
		t.Fatal("a capped read must report itself capped")
	}
	if !got.Current {
		t.Error("and it is still current — the two say different things")
	}
	if len(got.Subjects) != subjectReadCap {
		t.Errorf("the cap must bound what is returned, got %d", len(got.Subjects))
	}
}

// The store is on a volume shared with whatever else the container mounts, and
// it names every account on the target.
func TestTheStoreIsNotWorldReadable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.db")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("want 0600, got %o", perm)
	}
}

// Replaying an operation id returns the recorded result rather than mutating
// again. §16 declines a nonce store on the grounds that the operation id is the
// nonce, and that only holds if the deduplication is universal.
func TestAReplayedOperationIdReturnsTheRecordedResult(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := store.Remember("c1", map[string]string{"created": "ada"}); err != nil {
		t.Fatal(err)
	}
	got, found, err := store.Recall("c1")
	if err != nil || !found {
		t.Fatalf("recall: %v found=%t", err, found)
	}
	if !strings.Contains(string(got), `"created":"ada"`) {
		t.Fatalf("the original outcome must come back, got %s", got)
	}

	if _, found, _ := store.Recall("never-seen"); found {
		t.Error("an unseen id must not be found")
	}

	// Past the window it is reported absent rather than deleted on a read path:
	// a read that writes turns a replay storm into a write storm.
	store.now = func() time.Time { return time.Now().Add(idempotencyTTL + time.Hour) }
	if _, found, _ := store.Recall("c1"); found {
		t.Error("an entry past its retention must not be recalled")
	}
	removed, err := store.SweepIdempotency()
	if err != nil || removed != 1 {
		t.Fatalf("the sweep must be what removes it: removed=%d err=%v", removed, err)
	}
}
