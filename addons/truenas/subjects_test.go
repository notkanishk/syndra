package main

import (
	"encoding/json"
	"errors"
	"fmt"
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
	audit   string
	shares  string
	// health answers the four health reads by method name. A method absent
	// from the map fails, which is how a per-source degradation is exercised.
	health map[string]string
	err    error
	// refuse makes a method answer with a JSON-RPC error member — the target
	// saying no over a healthy socket, which is a different thing from the
	// transport failing and was for a while indistinguishable from success.
	refuse map[string]string
	// methods overrides what `core.get_methods` enumerates; empty means every
	// method this add-on uses is present.
	methods string
	calls   []string
	params  []any
}

const fixtureMethods = `{"user.query":{},"user.create":{},"user.update":{},"user.delete":{},` +
	`"group.query":{},"audit.query":{},"sharing.smb.query":{},"system.info":{},` +
	`"alert.list":{},"pool.query":{},"service.query":{}}`

// envelope wraps a result the way the middleware does.
//
// Every fixture goes through it, because the client hands back the WHOLE
// message and a fake that returned a bare result would agree with a bug rather
// than with the target. That agreement is what let a decode error reach
// production: 574 lines of tests passed against a shape TrueNAS never sends.
func envelope(result string) json.RawMessage {
	return json.RawMessage(`{"jsonrpc":"2.0","id":1,"result":` + result + `}`)
}

func errorEnvelope(message string) json.RawMessage {
	encoded, _ := json.Marshal(message)
	return json.RawMessage(`{"jsonrpc":"2.0","id":1,"error":{"code":-32001,"message":` + string(encoded) + `}}`)
}

func (f *fakeRPC) Call(method string, _ int64, params any) (json.RawMessage, error) {
	f.calls = append(f.calls, method)
	f.params = append(f.params, params)
	if f.err != nil {
		return nil, f.err
	}
	if msg, ok := f.refuse[method]; ok {
		return errorEnvelope(msg), nil
	}
	switch method {
	case "user.query":
		// Filters are APPLIED, not ignored. A fake that answered every query
		// with the whole fixture would agree with a lookup that resolved the
		// wrong account, which is the exact shape of agreement this package
		// already shipped one production bug through.
		return envelope(filterRows(orEmptyList(f.users), params)), nil
	case "group.query":
		return envelope(orEmptyList(f.groups)), nil
	case "system.version":
		// A supported release unless a fixture says otherwise. Every connection
		// re-probes, so a fake that answered with an empty string would blank
		// the version out from under a test that set one.
		if f.version == "" {
			return envelope(`"25.04.2"`), nil
		}
		return envelope(`"` + f.version + `"`), nil
	case "core.get_methods":
		if f.methods == "" {
			return envelope(fixtureMethods), nil
		}
		return envelope(f.methods), nil
	case "audit.query":
		return envelope(orEmptyList(f.audit)), nil
	case "sharing.smb.query":
		return envelope(orEmptyList(f.shares)), nil
	case "system.info", "alert.list", "pool.query", "service.query":
		if f.health == nil {
			return envelope(`{}`), nil
		}
		body, ok := f.health[method]
		if !ok {
			return nil, errors.New("method unavailable on this target")
		}
		return envelope(body), nil
	}
	return envelope(`null`), nil
}

// filterRows applies the equality filters of a `user.query`, and the row limit.
//
// Equality on one field is all this add-on ever sends. Anything richer would be
// a fake pretending to be middleware, which is how a fake starts being trusted
// about behaviour nobody verified.
//
// The query is validated BEFORE any row is looked at, and that ordering is the
// guard rather than a tidiness. Checking each filter inside the row loop meant
// an empty fixture never reached the check at all — the loop did not run — so a
// query this fake does not understand was answered with `[]` and nobody heard
// about it. Whether the fake happens to hold rows is not a fact about whether
// it understood what it was asked.
func filterRows(rows string, params any) string {
	args, ok := params.([]any)
	if !ok || len(args) == 0 {
		return rows
	}
	filters, ok := args[0].([]any)
	if !ok {
		// A filter list that is not a list. Returning the fixture whole here was
		// the second escape: the malformed-filter panic below could only ever
		// fire for a bad filter INSIDE a well-formed list.
		panic(fmt.Sprintf("fakeRPC: query filters are not a list: %#v", args[0]))
	}
	if len(filters) == 0 {
		// A genuine "no filter". `readSubjects` sends this, and everything is
		// the correct answer to it.
		return rows
	}

	type equality struct{ field, want string }
	checks := make([]equality, 0, len(filters))
	for _, raw := range filters {
		f, ok := raw.([]any)
		if !ok || len(f) != 3 {
			panic(fmt.Sprintf("fakeRPC: malformed query filter %#v", raw))
		}
		// Loudly, rather than by skipping it. A filter this fake does not
		// understand and quietly ignores is a filter it answers with the whole
		// fixture — the hole that let a lookup resolve the wrong account and the
		// test agree with it. The day somebody needs `~`, `in` or `nin`, this
		// says so instead of agreeing with them.
		if op, _ := f[1].(string); op != "=" {
			panic(fmt.Sprintf("fakeRPC: unhandled query operator %q — teach filterRows about it", op))
		}
		field, _ := f[0].(string)
		// Compared as text: the fixture decodes numbers to float64 and the
		// caller holds int64, and this fake has no business caring.
		checks = append(checks, equality{field: field, want: fmt.Sprint(f[2])})
	}

	var decoded []map[string]any
	if err := json.Unmarshal([]byte(rows), &decoded); err != nil {
		// A fixture that will not parse is a broken test, and answering it with
		// the unparsed string is the permissive default this whole function
		// exists to refuse.
		panic(fmt.Sprintf("fakeRPC: fixture is not a list of rows: %v", err))
	}
	kept := make([]map[string]any, 0, len(decoded))
	for _, row := range decoded {
		matches := true
		for _, check := range checks {
			if fmt.Sprint(row[check.field]) != check.want {
				matches = false
				break
			}
		}
		if matches {
			kept = append(kept, row)
		}
	}
	if opts, ok := args[len(args)-1].(map[string]any); ok {
		if limit, ok := opts["limit"].(int); ok && limit > 0 && len(kept) > limit {
			kept = kept[:limit]
		}
	}
	encoded, err := json.Marshal(kept)
	if err != nil {
		panic(fmt.Sprintf("fakeRPC: could not re-encode filtered rows: %v", err))
	}
	return string(encoded)
}

func orEmptyList(s string) string {
	if s == "" {
		return "[]"
	}
	return s
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
	// Pre-probed: the version is set here rather than read, so a test that says
	// nothing about versions gets a supported target. A test that wants the
	// probe itself clears `probed`.
	nas.version, nas.probed = "25.04.2", true
	return &server{
		auth: &authenticator{now: time.Now},
		nas:  nas, store: store, log: mlog,
		life:    newLifecycle(LifecycleActive),
		product: "truenas_scale",
	}
}

const (
	fixtureUsers = `[
		{"username":"ada","id":11,"uid":3001,"locked":false,"smb":true,"groups":[41,42]},
		{"username":"leo","id":12,"uid":3002,"locked":true,"smb":false,"groups":[42]}
	]`
	fixtureGroups = `[{"id":41,"gid":545,"group":"builtin_users"},{"id":42,"gid":900,"group":"lab_makers"}]`
)

// The hash fields are the reason the query names its columns. `user.query`
// returns `unixhash` and `smbhash`, and an NT hash is a pass-the-hash
// credential: possessing one is equivalent to possessing the password for SMB.
func TestNoHashFieldIsRequestedOrReturnedOnAnyPath(t *testing.T) {
	rpc := &fakeRPC{
		// The NAS offering them anyway, which is exactly what it does.
		users:  `[{"username":"ada","id":11,"uid":3001,"locked":false,"smb":true,"groups":[42],"unixhash":"$6$deadbeef","smbhash":"AAD3B435B51404EE"}]`,
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
		b.WriteString(`,"locked":false,"smb":true,"groups":[42]}`)
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

	if err := store.Remember("c1", kindApply, map[string]string{"created": "ada"}); err != nil {
		t.Fatal(err)
	}
	got, found, err := store.Recall("c1", kindApply)
	if err != nil || !found {
		t.Fatalf("recall: %v found=%t", err, found)
	}
	if !strings.Contains(string(got), `"created":"ada"`) {
		t.Fatalf("the original outcome must come back, got %s", got)
	}

	if _, found, _ := store.Recall("never-seen", kindApply); found {
		t.Error("an unseen id must not be found")
	}

	// Past the window it is reported absent rather than deleted on a read path:
	// a read that writes turns a replay storm into a write storm.
	store.now = func() time.Time { return time.Now().Add(idempotencyTTL + time.Hour) }
	if _, found, _ := store.Recall("c1", kindApply); found {
		t.Error("an entry past its retention must not be recalled")
	}
	removed, err := store.SweepIdempotency()
	if err != nil || removed != 1 {
		t.Fatalf("the sweep must be what removes it: removed=%d err=%v", removed, err)
	}
}

// The fake must fail loudly on a query it does not implement. Silently ignoring
// one means answering with the whole fixture — or with `[]` — which is how a
// lookup resolves the wrong account and its test agrees with it.
//
// Every case here was found by applying the rule to the guard itself: a guard is
// not real until it has been watched to fail, and two of these escaped a version
// of it that had already been reviewed. The empty-fixture case is the one that
// matters most, because it was reachable today — `orEmptyList` answers `[]`
// whenever a test sets no users, and `lookupOne` always sends a filter.
func TestTheFakeRefusesAQueryItDoesNotImplement(t *testing.T) {
	for _, tc := range []struct {
		name   string
		rows   string
		params any
	}{
		{
			"an unhandled operator",
			`[{"username":"ada","uid":3001}]`,
			[]any{[]any{[]any{"username", "~", "ad"}}, map[string]any{"limit": 2}},
		},
		{
			// The escape: the check lived inside the row loop, so no rows meant
			// no check. Whether the fake holds rows is not a fact about whether
			// it understood the question.
			"an unhandled operator against an empty fixture",
			`[]`,
			[]any{[]any{[]any{"username", "~", "ad"}}, map[string]any{"limit": 2}},
		},
		{
			// The other escape: this returned before any validation ran, so the
			// malformed-filter panic could only fire for a bad filter inside a
			// well-formed list.
			"a filter list that is not a list",
			`[{"username":"ada","uid":3001}]`,
			[]any{"username = ada", map[string]any{"limit": 2}},
		},
		{
			"a malformed filter",
			`[{"username":"ada","uid":3001}]`,
			[]any{[]any{[]any{"username", "="}}, map[string]any{"limit": 2}},
		},
		{
			// A non-string operator. `f[1].(string)` yields "" on the failed
			// assertion, which is not "=", so it lands on the same panic — worth
			// pinning rather than inferring, because the guard reads as a string
			// comparison and this is the path where there is no string.
			"an operator that is not a string, against an empty fixture",
			`[]`,
			[]any{[]any{[]any{"username", nil, "ada"}}, map[string]any{"limit": 2}},
		},
		{
			"a fixture that is not a list of rows",
			`{"username":"ada"}`,
			[]any{[]any{[]any{"username", "=", "ada"}}, map[string]any{"limit": 2}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("this must panic rather than answering anything at all")
				}
			}()
			filterRows(tc.rows, tc.params)
		})
	}
}

// And the two shapes that are genuinely answerable still are, so the guard has
// not been tightened into refusing the queries this add-on actually sends.
func TestTheFakeStillAnswersTheQueriesTheAddonSends(t *testing.T) {
	const rows = `[{"username":"ada","uid":3001},{"username":"leo","uid":3002}]`
	// `readSubjects`: no filters, everything.
	if got := filterRows(rows, []any{[]any{}, map[string]any{"limit": 5001}}); got != rows {
		t.Errorf("an unfiltered query must answer with everything, got %s", got)
	}
	// `lookupOne`: one equality, one row.
	got := filterRows(rows, []any{[]any{[]any{"uid", "=", int64(3002)}}, map[string]any{"limit": 2}})
	if !strings.Contains(got, "leo") || strings.Contains(got, "ada") {
		t.Errorf("want only leo, got %s", got)
	}
}
