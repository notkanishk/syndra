package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 6.13–6.24 — the one-shot operations.
//
// Most of what is asserted here is a NEGATIVE: the value a member submitted
// must not appear in a store, a snapshot, a log line, a response, or an error.
// Those are the paths a secret escapes through in practice, and each of them is
// code somebody adds later for debugging.

const theSecret = "correct-horse-battery-staple"

func opServer(t *testing.T) (*server, *mutatingRPC) {
	t.Helper()
	s, m := applyServer(t, `[{"username":"ada","id":11,"uid":3001,"locked":false,"smb":true,"groups":[42]}]`)
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "ada", UID: 3001}); err != nil {
		t.Fatal(err)
	}
	return s, m
}

func postOperation(t *testing.T, s *server, name, body string) *httptest.ResponseRecorder {
	t.Helper()
	body = withContractVersion(t, body)
	rr := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/operations/"+name, strings.NewReader(body))
	r.SetPathValue("name", name)
	s.handleOperation(rr, r, []byte(body))
	return rr
}

// withContractVersion fills in the field every real caller sends.
//
// Every body-carrying route refuses a request that does not declare the wire
// version, absent included — an omitted field reads as version 0, which is
// exactly what a caller from before the field existed looks like. A test about
// anything else should not have to restate it, and a test about the version
// itself supplies its own and this leaves it alone.
func withContractVersion(t *testing.T, body string) string {
	t.Helper()
	var fields map[string]any
	if err := json.Unmarshal([]byte(body), &fields); err != nil {
		// A body this test means to be malformed. Handed through untouched, or
		// the assertion would be about a document the test did not write.
		return body
	}
	if _, present := fields["contract_version"]; present {
		return body
	}
	fields["contract_version"] = ContractVersion
	out, err := json.Marshal(fields)
	if err != nil {
		t.Fatalf("re-encode body: %v", err)
	}
	return string(out)
}

// 6.13/6.14 — the plaintext reaches the target and appears nowhere durable.
func TestASubmittedCredentialReachesTheTargetAndIsKeptNowhere(t *testing.T) {
	s, m := opServer(t)
	logDir := filepath.Dir(mutationLogPath(t, s))

	rr := postOperation(t, s, "password.set",
		`{"call_id":"c1","subject":"sub-1","actor":"sub-1","params":{"password":"`+theSecret+`"}}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", rr.Code, rr.Body.String())
	}

	// It reached the target — a test that only checked the negatives would pass
	// for an operation that did nothing at all.
	if len(m.updates) != 1 || m.updates[0]["password"] != theSecret {
		t.Fatalf("the credential must reach the target: %v", m.updates)
	}
	// `user.update`, never `user.set_password`: the latter needs FULL_ADMIN
	// when the target is another user, which this add-on's identity is not.
	for _, method := range m.fakeRPC.calls {
		if method == "user.set_password" {
			t.Error("must use user.update({password}), which needs only ACCOUNT_WRITE")
		}
	}

	// And nowhere else. Each of these is a path a secret escapes through in
	// practice.
	if strings.Contains(rr.Body.String(), theSecret) {
		t.Error("the response carries the credential")
	}
	scanTreeForSecret(t, logDir, theSecret)
	snap, _, _ := s.store.GetSnapshot()
	encoded, _ := json.Marshal(snap)
	if strings.Contains(string(encoded), theSecret) {
		t.Error("the snapshot carries the credential")
	}
	cached, found, _ := s.store.Recall("c1", kindOperation+"password.set")
	if !found {
		t.Fatal("the outcome must be recorded for replay")
	}
	if strings.Contains(string(cached), theSecret) {
		t.Error("the idempotency entry carries the credential")
	}

	// The mutation log records that a password was set.
	data, err := os.ReadFile(mutationLogPath(t, s))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"operation":"password.set"`) {
		t.Error("the log must record the event even though it records no value")
	}
}

// A failure is where an echoed payload lands. The target's response body must
// not become this add-on's error text.
func TestAFailedCredentialChangeDoesNotEchoTheTarget(t *testing.T) {
	s, m := opServer(t)
	m.fakeRPC.err = errors.New("update failed for password=" + theSecret)

	rr := postOperation(t, s, "password.set",
		`{"call_id":"c1","subject":"sub-1","params":{"password":"`+theSecret+`"}}`)
	if rr.Code == http.StatusOK {
		t.Fatal("a refused change must not report success")
	}
	if strings.Contains(rr.Body.String(), theSecret) {
		t.Fatalf("the failure echoed the credential: %s", rr.Body.String())
	}
}

// The refusal names the parameter and says nothing about the value — not even
// about its shape. "Must be at least 8 characters" is a fact about a password
// somebody submitted, and an error string is logged, returned, and traced.
func TestAMissingSecretIsRefusedWithoutDescribingIt(t *testing.T) {
	s, m := opServer(t)

	rr := postOperation(t, s, "password.set", `{"call_id":"c1","subject":"sub-1","params":{}}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "password is required") {
		t.Errorf("the refusal must name the parameter: %s", rr.Body.String())
	}
	if len(m.updates) != 0 {
		t.Error("nothing may be sent")
	}
}

// 6.15/6.16 — rotation persists, caches and logs no value, and records that a
// rotation occurred.
func TestRotationMintsAValueNobodyEverSees(t *testing.T) {
	s, m := opServer(t)

	rr := postOperation(t, s, "password.rotate", `{"call_id":"c1","subject":"sub-1","actor":"op_1"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	if len(m.updates) != 1 {
		t.Fatalf("want one update, got %d", len(m.updates))
	}
	minted, ok := m.updates[0]["password"].(string)
	if !ok || len(minted) < 32 {
		t.Fatalf("a rotation must set a substantial value, got %q", minted)
	}

	// The whole point: it is applied and never handed back.
	if strings.Contains(rr.Body.String(), minted) {
		t.Fatal("the response carries the minted credential — rotation as a revocation depends on it not being handed out")
	}
	cached, _, _ := s.store.Recall("c1", kindOperation+"password.set")
	if strings.Contains(string(cached), minted) {
		t.Fatal("the idempotency entry carries the minted credential")
	}
	scanTreeForSecret(t, filepath.Dir(mutationLogPath(t, s)), minted)

	// It says a rotation happened, with the surface's copy about sessions.
	var out OperationResult
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if !out.Rotated {
		t.Error("the result must record that a rotation occurred")
	}
	if !strings.Contains(out.Detail, "reconnect") {
		t.Error("the copy must say sessions end on reconnect, not immediately — TrueNAS has no session-close method")
	}
}

// Two rotations mint two different values, or the "new" credential is the old
// one and the revocation revoked nothing.
func TestTwoRotationsMintDifferentCredentials(t *testing.T) {
	s, m := opServer(t)
	postOperation(t, s, "password.rotate", `{"call_id":"c1","subject":"sub-1"}`)
	postOperation(t, s, "password.rotate", `{"call_id":"c2","subject":"sub-1"}`)

	if len(m.updates) != 2 {
		t.Fatalf("want two rotations, got %d", len(m.updates))
	}
	if m.updates[0]["password"] == m.updates[1]["password"] {
		t.Fatal("a rotation that mints the same value revokes nothing")
	}
}

// 6.19/6.20 — purge runs on a credential injected for that call alone. The
// add-on's own session must never become delete-capable.
func TestPurgeUsesTheInjectedCredentialAndKeepsItNowhere(t *testing.T) {
	s, m := opServer(t)
	const elevated = "elevated-delete-key"

	var elevatedCalls []string
	var closed bool
	var usedKey string
	s.elevated = func(apiKey string) (rpc, error) {
		usedKey = apiKey
		return &recordingRPC{onCall: func(method string) { elevatedCalls = append(elevatedCalls, method) },
			onClose: func() { closed = true }}, nil
	}

	rr := postOperation(t, s, "account.purge",
		`{"call_id":"c1","subject":"sub-1","actor":"op_1","params":{"elevated_key":"`+elevated+`"}}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	if usedKey != elevated {
		t.Fatalf("the injected key must be the one used, got %q", usedKey)
	}
	if len(elevatedCalls) != 1 || elevatedCalls[0] != "user.delete" {
		t.Fatalf("the elevated session must be used for the delete and nothing else: %v", elevatedCalls)
	}
	if !closed {
		t.Fatal("the elevated session must be closed: it must not outlive the call it was injected for")
	}

	// The add-on's own long-lived session must never carry a delete.
	for _, method := range m.fakeRPC.calls {
		if method == "user.delete" {
			t.Fatal("the shared session must never be delete-capable")
		}
	}

	// And the key is nowhere durable.
	if strings.Contains(rr.Body.String(), elevated) {
		t.Error("the response carries the elevated key")
	}
	cached, _, _ := s.store.Recall("c1", kindOperation+"password.set")
	if strings.Contains(string(cached), elevated) {
		t.Error("the idempotency entry carries the elevated key")
	}
	scanTreeForSecret(t, filepath.Dir(mutationLogPath(t, s)), elevated)
}

// Purge without the injected credential is refused, not attempted on the
// add-on's own key — which the target would refuse for want of privilege, but
// only after a delete had been asked for.
func TestPurgeWithoutTheInjectedCredentialIsRefusedBeforeAnyCall(t *testing.T) {
	s, m := opServer(t)
	var elevatedOpened bool
	s.elevated = func(string) (rpc, error) { elevatedOpened = true; return nil, errors.New("should not be called") }

	rr := postOperation(t, s, "account.purge", `{"call_id":"c1","subject":"sub-1","params":{}}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d (%s)", rr.Code, rr.Body.String())
	}
	if elevatedOpened {
		t.Error("no session may be opened without the credential")
	}
	for _, method := range m.fakeRPC.calls {
		if method == "user.delete" {
			t.Fatal("nothing may be deleted")
		}
	}
}

// 6.21/6.22 — an empty result names the unaudited shares rather than implying
// no activity. SMB auditing is per share, so a quiet answer means either
// nothing happened or nobody was watching.
func TestAnEmptyActivityResultNamesWhatWasNotWatched(t *testing.T) {
	s, m := opServer(t)
	m.fakeRPC.audit = `[]`
	m.fakeRPC.shares = `[{"name":"lab","audit":{"enable":false}},{"name":"archive","audit":{"enable":true}}]`

	rr := postOperation(t, s, "activity.get", `{"call_id":"c1","subject":"sub-1"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	var out OperationResult
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Activity == nil {
		t.Fatal("want an activity report")
	}
	if len(out.Activity.Events) != 0 {
		t.Fatalf("the fixture has no events: %+v", out.Activity.Events)
	}
	if len(out.Activity.UnauditedShares) != 1 || out.Activity.UnauditedShares[0] != "lab" {
		t.Fatalf("an empty result must name the shares that were not being watched: %v", out.Activity.UnauditedShares)
	}
}

// 6.23/6.24 — health composes four sources and degrades per source rather than
// failing whole.
func TestHealthComposesFourSourcesAndDegradesPerSource(t *testing.T) {
	s, m := opServer(t)
	m.fakeRPC.health = map[string]string{
		"system.info":   `{"version":"25.04.2"}`,
		"pool.query":    `[{"name":"tank"}]`,
		"service.query": `[{"service":"cifs"}]`,
		// alert.list deliberately absent: one source failing must not take the
		// other three with it.
	}

	rr := postOperation(t, s, "health.get", `{"call_id":"c1","subject":"-"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	var out OperationResult
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Health == nil || out.Health.System == nil || out.Health.Pools == nil {
		t.Fatalf("the sources that answered must be present: %+v", out.Health)
	}
	if len(out.Health.Degraded) != 1 || out.Health.Degraded[0] != "alerts" {
		t.Fatalf("the source that failed must be named: %v", out.Health.Degraded)
	}
}

// All four failing is an unreachable target, not a health report with four
// holes in it.
func TestHealthWithNoSourceAnsweringIsAnOutageNotAReport(t *testing.T) {
	s, m := opServer(t)
	m.fakeRPC.health = map[string]string{}

	rr := postOperation(t, s, "health.get", `{"call_id":"c1","subject":"-"}`)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d (%s)", rr.Code, rr.Body.String())
	}
}

// An operation naming a subject with no account refuses rather than falling
// back to derivation — derivation names an account that may not exist, and
// setting a password on the wrong one is the whole hazard.
func TestAnOperationOnAnUnboundSubjectRefusesRatherThanGuessing(t *testing.T) {
	s, m := opServer(t)

	rr := postOperation(t, s, "password.set",
		`{"call_id":"c1","subject":"sub-unknown","params":{"password":"`+theSecret+`"}}`)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d (%s)", rr.Code, rr.Body.String())
	}
	if len(m.updates) != 0 {
		t.Fatal("nothing may be sent")
	}
}

// Replay returns the recorded outcome and mutates nothing a second time.
func TestAReplayedOperationDoesNotMutateTwice(t *testing.T) {
	s, m := opServer(t)
	const body = `{"call_id":"c1","subject":"sub-1","params":{"password":"` + theSecret + `"}}`

	first := postOperation(t, s, "password.set", body)
	second := postOperation(t, s, "password.set", body)
	if len(m.updates) != 1 {
		t.Fatalf("a replay must not set the credential twice, got %d", len(m.updates))
	}
	if first.Body.String() != second.Body.String() {
		t.Errorf("a replay must return the original outcome:\n%s\n%s", first.Body.String(), second.Body.String())
	}
}

// A read is never gated by lifecycle state; a mutation always is.
func TestMaintenanceGatesMutationsAndNotReads(t *testing.T) {
	s, m := opServer(t)
	m.fakeRPC.audit = `[]`
	m.fakeRPC.shares = `[]`
	_ = s.life.Set(LifecycleReadOnly, "maintenance")

	mutation := postOperation(t, s, "password.set",
		`{"call_id":"c1","subject":"sub-1","params":{"password":"`+theSecret+`"}}`)
	if mutation.Code != http.StatusServiceUnavailable || mutation.Header().Get("Retry-After") == "" {
		t.Fatalf("a mutation must be refused as retryable, got %d", mutation.Code)
	}
	if len(m.updates) != 0 {
		t.Fatal("nothing may be sent")
	}

	read := postOperation(t, s, "activity.get", `{"call_id":"c2","subject":"sub-1"}`)
	if read.Code != http.StatusOK {
		t.Fatalf("a read must still be served during maintenance, got %d (%s)", read.Code, read.Body.String())
	}
}

// Unknown ids fail closed. The backend's policy is the authority on what
// exists; this is the second gate.
func TestAnUnknownOperationFailsClosed(t *testing.T) {
	s, _ := opServer(t)
	rr := postOperation(t, s, "account.exfiltrate", `{"call_id":"c1","subject":"sub-1"}`)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d (%s)", rr.Code, rr.Body.String())
	}
}

// recordingRPC is the elevated session: it records what it was asked and
// whether it was closed.
type recordingRPC struct {
	onCall  func(string)
	onClose func()
}

func (r *recordingRPC) Call(method string, _ int64, _ any) (json.RawMessage, error) {
	r.onCall(method)
	return envelope(`null`), nil
}
func (r *recordingRPC) Ping() (string, error) { return "pong", nil }
func (r *recordingRPC) Close() error          { r.onClose(); return nil }

func mutationLogPath(t *testing.T, s *server) string {
	t.Helper()
	return filepath.Join(s.log.dir, logFileName)
}

// scanTreeForSecret walks every file under dir and fails if the value appears
// in any of them. Broader than checking the one file this code writes: the
// point is that the value is nowhere on the volume, including in whatever a
// future path starts writing there.
func scanTreeForSecret(t *testing.T, dir, secret string) {
	t.Helper()
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		if strings.Contains(string(data), secret) {
			t.Errorf("%s contains the secret", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// The target's own text never leaves the client wrapper, which is where the
// guarantee actually lives — the operation's own message above it is defence in
// depth over this, not the thing doing the work.
//
// TrueNAS builds its errors from the middleware's text, and a failed
// `user.update({password})` is a call whose parameters include the password.
// Classifying rather than wrapping is what stops that reaching a log line.
func TestTheClientWrapperNeverSurfacesTheTargetsOwnText(t *testing.T) {
	m := &mutatingRPC{fakeRPC: fakeRPC{err: errors.New("update failed for password=" + theSecret)}}
	n := newNAS(func() (rpc, error) { return m, nil }, []string{"25.04"})

	err := n.call("user.update", []any{1, map[string]any{"password": theSecret}}, nil)
	if err == nil {
		t.Fatal("the failure must be reported")
	}
	if strings.Contains(err.Error(), theSecret) {
		t.Fatalf("the target's text reached the caller: %v", err)
	}
	// It must still be actionable: the method, and a classification the caller
	// can branch on.
	if !strings.Contains(err.Error(), "user.update") {
		t.Errorf("the error must name the method: %v", err)
	}
	if !errors.Is(err, ErrTargetRefused) && !errors.Is(err, ErrTargetUnreachable) && !errors.Is(err, ErrRateLimited) {
		t.Errorf("the error must carry a classification: %v", err)
	}
}

// The injected delete key is declared secret, or every redaction rule that
// covers a member's password steps around the far more dangerous value.
func TestEveryValueThatIsASecretIsDeclaredOne(t *testing.T) {
	byID := map[string]Operation{}
	for _, op := range operationSet(alwaysAvailable{}) {
		byID[op.ID] = op
	}
	for _, want := range []struct{ id, param string }{
		{"password.set", "password"},
		{"account.purge", "elevated_key"},
	} {
		op, ok := byID[want.id]
		if !ok {
			t.Fatalf("%s is not in the manifest", want.id)
		}
		var declared bool
		for _, p := range op.SecretParams {
			if p == want.param {
				declared = true
			}
		}
		if !declared {
			t.Errorf("%s must declare %q as a secret parameter: %v", want.id, want.param, op.SecretParams)
		}
	}
	// And the manifest names a confirmation on the one irreversible operation.
	if !byID["account.purge"].Confirm {
		t.Error("account.purge is the one irreversible operation and must require confirmation")
	}
}

// alwaysAvailable is a probe that answers yes, so the manifest's declarations
// can be read without a target.
type alwaysAvailable struct{}

func (alwaysAvailable) availability(string) (bool, string) { return true, "" }

// A purge takes the binding with it. Leaving it behind leaves this add-on
// claiming an account that no longer exists, and the next apply reads
// bound-but-absent as an out-of-band deletion — which re-creates under the
// recorded name. That path is right for an account somebody else deleted and
// exactly wrong for one we deleted on purpose.
func TestAPurgeDoesNotLeaveTheSubjectBoundToADeletedAccount(t *testing.T) {
	s, _ := opServer(t)
	s.elevated = func(string) (rpc, error) {
		return &recordingRPC{onCall: func(string) {}, onClose: func() {}}, nil
	}

	rr := postOperation(t, s, "account.purge",
		`{"call_id":"c1","subject":"sub-1","actor":"op_1","params":{"elevated_key":"k"}}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	if _, bound, err := s.store.GetBinding("sub-1"); err != nil || bound {
		t.Fatalf("the binding must be gone: bound=%t err=%v", bound, err)
	}
}

// A refusal from the target is a failure, not a success. The elevated call
// wanted no result, and reading a refusal only when a result is wanted is how
// a delete that never happened gets reported as done.
func TestAPurgeTheTargetRefusesIsNotReportedAsDone(t *testing.T) {
	s, _ := opServer(t)
	s.elevated = func(string) (rpc, error) {
		return &refusingRPC{}, nil
	}

	rr := postOperation(t, s, "account.purge",
		`{"call_id":"c1","subject":"sub-1","actor":"op_1","params":{"elevated_key":"k"}}`)
	if rr.Code == http.StatusOK {
		t.Fatalf("a refused deletion must not answer 200: %s", rr.Body.String())
	}
	if _, bound, _ := s.store.GetBinding("sub-1"); !bound {
		t.Fatal("the binding must survive a deletion that did not happen")
	}
}

type refusingRPC struct{}

func (refusingRPC) Call(string, int64, any) (json.RawMessage, error) {
	return errorEnvelope("[EPERM] user.delete: not permitted"), nil
}
func (refusingRPC) Ping() (string, error) { return "pong", nil }
func (refusingRPC) Close() error          { return nil }

// One namespace so a reused call id is always caught, and a type tag so a
// cached result is never decoded as the wrong shape — which is an all-zero
// success, the worst answer available.
func TestACallIdReplayedAtAnotherEndpointIsRefused(t *testing.T) {
	s, _ := opServer(t)

	if rr := postOperation(t, s, "password.set",
		`{"call_id":"shared","subject":"sub-1","params":{"password":"`+theSecret+`"}}`); rr.Code != http.StatusOK {
		t.Fatalf("setup: %s", rr.Body.String())
	}
	rr := postOperation(t, s, "password.rotate", `{"call_id":"shared","subject":"sub-1"}`)
	if rr.Code != http.StatusConflict {
		t.Fatalf("want a conflict, got %d (%s)", rr.Code, rr.Body.String())
	}
}

// The log promises who did what to whom. Recording the subject as their own
// actor answers "who" with "whom".
func TestTheRecordedActorIsTheOneWhoAsked(t *testing.T) {
	s, _ := opServer(t)
	postOperation(t, s, "password.set",
		`{"call_id":"c1","subject":"sub-1","actor":"op_7","params":{"password":"`+theSecret+`"}}`)

	raw, err := os.ReadFile(mutationLogPath(t, s))
	if err != nil {
		t.Fatal(err)
	}
	var r Record
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(raw))), &r); err != nil {
		t.Fatal(err)
	}
	if r.Actor != "op_7" || r.Subject != "sub-1" {
		t.Fatalf("want actor op_7 on subject sub-1, got %+v", r)
	}
}

// 6.9 built the apply path to follow a stable uid whose username moved out of
// band — a rename, not a missing account. The one-shot operations resolved by
// NAME instead, so a rename made every one of them fail until an apply happened
// to run and re-sync the binding. One rule for both paths, and it is the uid.
func TestAnOutOfBandRenameDoesNotBreakTheOperationPath(t *testing.T) {
	s, m := applyServer(t, `[{"username":"ada_renamed","id":11,"uid":3001,"locked":false,"smb":true,"groups":[42]}]`)
	// The binding still names the account as it was.
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "ada", UID: 3001}); err != nil {
		t.Fatal(err)
	}

	rr := postOperation(t, s, "password.set",
		`{"call_id":"c1","subject":"sub-1","actor":"op_1","params":{"password":"`+theSecret+`"}}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("a rename must not break the credential path: %d (%s)", rr.Code, rr.Body.String())
	}
	if len(m.updates) != 1 || m.updates[0]["password"] != theSecret {
		t.Fatalf("the credential must reach the target: %v", m.updates)
	}
	// And it landed on the renamed account's record id, not on a guess.
	args, _ := lastCallParams(m, "user.update").([]any)
	if len(args) != 2 {
		t.Fatalf("want two arguments to user.update, got %v", args)
	}
	if id, ok := args[0].(apiID); !ok || id.String() != "11" {
		t.Fatalf("want the record id of the renamed account, got %#v", args[0])
	}
}

// A binding recorded before uids were is still resolvable, and uid 0 is root —
// never a subject's account, so it must not be matched on.
func TestABindingWithNoUidFallsBackToTheName(t *testing.T) {
	s, m := applyServer(t, `[{"username":"ada","id":11,"uid":3001,"locked":false,"smb":true,"groups":[42]}]`)
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "ada"}); err != nil {
		t.Fatal(err)
	}

	rr := postOperation(t, s, "password.set",
		`{"call_id":"c1","subject":"sub-1","actor":"op_1","params":{"password":"`+theSecret+`"}}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	for _, params := range m.fakeRPC.params {
		args, ok := params.([]any)
		if !ok || len(args) == 0 {
			continue
		}
		filters, ok := args[0].([]any)
		if !ok || len(filters) == 0 {
			continue
		}
		if one, ok := filters[0].([]any); ok && len(one) == 3 && one[0] == "uid" {
			t.Fatalf("a binding with no uid must not query uid 0 — that is root: %v", one)
		}
	}
}

// Two accounts sharing a uid is a question about which one, and the answer is
// never "whichever the next query returns". Falling through to the name would
// answer it by changing the subject — on the path whose next call sets a
// credential.
func TestAnAmbiguousUidRefusesRatherThanResolvingByName(t *testing.T) {
	s, m := applyServer(t, `[
		{"username":"ada","id":11,"uid":3001,"locked":false,"smb":true,"groups":[42]},
		{"username":"ada_clone","id":12,"uid":3001,"locked":false,"smb":true,"groups":[42]}
	]`)
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "ada", UID: 3001}); err != nil {
		t.Fatal(err)
	}

	rr := postOperation(t, s, "password.set",
		`{"call_id":"c1","subject":"sub-1","actor":"op_1","params":{"password":"`+theSecret+`"}}`)
	if rr.Code == http.StatusOK {
		t.Fatalf("an ambiguous uid must not resolve: %s", rr.Body.String())
	}
	if len(m.updates) != 0 {
		t.Fatalf("nothing may be written when the account could not be named: %v", m.updates)
	}
}

// And the rename test above is only meaningful because the fake applies the
// query's filters: with the binding naming `ada` and the target holding
// `ada_renamed`, a name-first lookup matches nothing at all.
func TestTheFakeAppliesTheFiltersTheLookupSends(t *testing.T) {
	s, _ := applyServer(t, `[
		{"username":"ada","id":11,"uid":3001,"locked":false,"smb":true,"groups":[42]},
		{"username":"leo","id":12,"uid":3002,"locked":false,"smb":true,"groups":[42]}
	]`)

	id, err := s.lookupOne("uid", int64(3002))
	if err != nil {
		t.Fatalf("a filtered lookup must find its row: %v", err)
	}
	if id.String() != "12" {
		t.Fatalf("want leo's record id, got %s", id.String())
	}
	if _, err := s.lookupOne("uid", int64(9999)); !errors.Is(err, errNoSuchAccount) {
		t.Fatalf("want errNoSuchAccount, got %v", err)
	}
}

// An account that is gone is a refusal, not a mystery.
//
// Found on the dev deployment: a revocation aimed at an account somebody had
// already deleted came back 502, which the backend reads as indeterminate — so
// it parked on the unconfirmed-revocations surface as "we do not know whether
// this happened" and stayed there. The truthful answer is that there was
// nothing to revoke and no retry will change it, and only a 4xx says that.
func TestAMissingBoundAccountIsRefusedRatherThanLeftUnknown(t *testing.T) {
	rpc := &fakeRPC{users: `[]`, groups: fixtureGroups}
	s := testServer(t, rpc)
	if err := s.store.PutBinding(Binding{SubjectID: "u1", Username: "ada", UID: 3001}); err != nil {
		t.Fatal(err)
	}

	// `account.purge` is deliberately absent: it refuses earlier still, for
	// want of the elevated key, and never reaches the lookup.
	for _, op := range []string{"password.rotate", "password.set"} {
		t.Run(op, func(t *testing.T) {
			req := OperationRequest{
				Operation: op, Subject: "u1", Actor: "op1", CallID: "c-" + op,
				Params: map[string]any{"password": "Correct-Horse-Battery-9!"},
			}
			_, status, err := s.runOperation(op, req)
			if err == nil {
				t.Fatal("want a refusal")
			}
			if status < 400 || status >= 500 {
				t.Fatalf("want a deterministic 4xx, got %d (%v)", status, err)
			}
			if !strings.Contains(err.Error(), "no longer exists") {
				t.Errorf("the operator needs to know the account is gone: %v", err)
			}
		})
	}
}

// And two accounts matching one binding is a different refusal, because what an
// operator does next differs: one of them has to be renamed before anything
// touches either.
func TestAnAmbiguousBindingIsItsOwnRefusal(t *testing.T) {
	rpc := &fakeRPC{
		users: `[
			{"username":"ada","id":11,"uid":3001,"locked":false,"smb":true,"groups":[42]},
			{"username":"ada-old","id":12,"uid":3001,"locked":false,"smb":false,"groups":[42]}
		]`,
		groups: fixtureGroups,
	}
	s := testServer(t, rpc)
	if err := s.store.PutBinding(Binding{SubjectID: "u1", Username: "ada", UID: 3001}); err != nil {
		t.Fatal(err)
	}

	_, status, err := s.rotatePassword(OperationRequest{
		Operation: "password.rotate", Subject: "u1", Actor: "op1", CallID: "c1",
	})
	if err == nil {
		t.Fatal("want a refusal")
	}
	if status != http.StatusConflict {
		t.Fatalf("want 409, got %d (%v)", status, err)
	}
}
