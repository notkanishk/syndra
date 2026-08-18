package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// 5.1–5.14, 6.1–6.2, 6.7–6.12 — the entitlement plane.
//
// Level-triggered convergence, account creation folded into it, and the two
// things that must never happen quietly: adopting somebody else's account, and
// renaming one that already exists.

// errReadAfterWrite is the target going unreadable between the write and the
// read that verifies it — the one state the read-back introduces.
var errReadAfterWrite = errors.New("the target stopped answering")

// mutatingRPC records every write so a test can assert that none happened.
type mutatingRPC struct {
	fakeRPC
	updates []map[string]any
	creates []map[string]any
	nextUID int64
	// nextID is deliberately not nextUID. The record key and the unix uid are
	// different numbers on a real target, and a fake that used one for both
	// would agree with the bug where a write lands on the wrong account.
	nextID int64
	// divergeOn makes the target store something other than what was written,
	// keyed by the update field. Normalisation, coercion and silent refusal all
	// look like this from outside, and all of them are invisible to an answer
	// assembled from the request.
	divergeOn map[string]any
	// failReadAfterUpdate makes the account unreadable once a write has landed:
	// the mutation happened and its result cannot be seen, which is the one
	// state the read-back adds and the one that must not be reported as either
	// a clean success or a failure.
	failReadAfterUpdate bool
}

// remember adds a created account to the fixture the next read serves.
func (m *mutatingRPC) remember(params any) {
	args, _ := params.([]any)
	if len(args) != 1 {
		return
	}
	create, ok := args[0].(map[string]any)
	if !ok {
		return
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(orEmptyList(m.fakeRPC.users)), &rows); err != nil {
		return
	}
	locked, _ := create["locked"].(bool)
	smb, _ := create["smb"].(bool)
	row := map[string]any{
		"id": m.nextID, "username": create["username"], "uid": m.nextUID,
		"locked": locked, "smb": smb, "groups": create["groups"],
	}
	if row["groups"] == nil {
		row["groups"] = []apiID{}
	}
	encoded, err := json.Marshal(append(rows, row))
	if err != nil {
		return
	}
	m.fakeRPC.users = string(encoded)
}

func (m *mutatingRPC) Call(method string, timeout int64, params any) (json.RawMessage, error) {
	// The error flag applies to writes as well as reads. A fake whose failure
	// switch covers only half its methods is a fake that quietly passes the
	// tests about the other half.
	if m.fakeRPC.err != nil {
		m.fakeRPC.calls = append(m.fakeRPC.calls, method)
		return nil, m.fakeRPC.err
	}
	if m.failReadAfterUpdate && len(m.updates) > 0 && method == "user.query" {
		m.fakeRPC.calls = append(m.fakeRPC.calls, method)
		return nil, errReadAfterWrite
	}
	// Recorded like every other call, so an assertion about WHICH account a
	// write addressed has something to read.
	m.fakeRPC.calls = append(m.fakeRPC.calls, method)
	m.fakeRPC.params = append(m.fakeRPC.params, params)
	switch method {
	case "user.update":
		args, _ := params.([]any)
		if len(args) == 2 {
			if u, ok := args[1].(map[string]any); ok {
				m.updates = append(m.updates, u)
				m.applyUpdate(args[0], u)
			}
		}
		return envelope(`null`), nil
	case "user.create":
		args, _ := params.([]any)
		if len(args) == 1 {
			if c, ok := args[0].(map[string]any); ok {
				m.creates = append(m.creates, c)
			}
		}
		if m.nextUID == 0 {
			m.nextUID = 4001
		}
		if m.nextID == 0 {
			m.nextID = 71
		}
		// The created account joins the fixture, because the apply reads it back
		// — `user.create` answers with the record key and the binding needs the
		// unix uid, which are different numbers. A fake that forgot the account
		// it had just made would make that read-back untestable.
		m.remember(params)
		return envelope(strconvI(m.nextID)), nil
	}
	// Already recorded above; the embedded fake records the rest.
	m.fakeRPC.calls = m.fakeRPC.calls[:len(m.fakeRPC.calls)-1]
	m.fakeRPC.params = m.fakeRPC.params[:len(m.fakeRPC.params)-1]
	return m.fakeRPC.Call(method, timeout, params)
}

// applyUpdate makes the fixture agree with the write, the way a target does.
//
// It did not, and that was a fake agreeing with the defect: the apply built its
// answer from the values it had REQUESTED, so a fixture that never moved was
// indistinguishable from one that had. With the read-back in place the fixture
// has to move, and `divergeOn` is how a test makes it move to something else —
// which is the case the projection could never see.
func (m *mutatingRPC) applyUpdate(id any, update map[string]any) {
	var rows []map[string]any
	if err := json.Unmarshal([]byte(orEmptyList(m.fakeRPC.users)), &rows); err != nil {
		return
	}
	want := fmt.Sprintf("%v", id)
	for i := range rows {
		if fmt.Sprintf("%v", rows[i]["id"]) != want {
			continue
		}
		for k, v := range update {
			if m.divergeOn != nil {
				if got, ok := m.divergeOn[k]; ok {
					v = got
				}
			}
			switch k {
			case "locked", "smb", "groups":
				rows[i][k] = v
			}
		}
	}
	if encoded, err := json.Marshal(rows); err == nil {
		m.fakeRPC.users = string(encoded)
	}
}

func strconvI(n int64) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func applyServer(t *testing.T, users string) (*server, *mutatingRPC) {
	t.Helper()
	m := &mutatingRPC{fakeRPC: fakeRPC{users: users, groups: fixtureGroups}}
	s := testServer(t, &m.fakeRPC)
	// The NAS must route through the recording wrapper, not the bare fake.
	s.nas = newNAS(func() (rpc, error) { return m, nil }, []string{"25.04"})
	s.nas.version, s.nas.probed = "25.04.2", true
	return s, m
}

// planInto and applyInto drive a handler with the body a real caller would have
// sent: the wire version is declared on every body-carrying route, and a test
// about something else should not have to restate it.
func planInto(t *testing.T, s *server, rr *httptest.ResponseRecorder, body string) {
	t.Helper()
	body = withContractVersion(t, body)
	s.handlePlan(rr, httptest.NewRequest(http.MethodPost, "/plan", strings.NewReader(body)), []byte(body))
}

func applyInto(t *testing.T, s *server, rr *httptest.ResponseRecorder, body string) {
	t.Helper()
	body = withContractVersion(t, body)
	s.handleApply(rr, httptest.NewRequest(http.MethodPost, "/apply", strings.NewReader(body)), []byte(body))
}

func postApply(t *testing.T, s *server, body string) *httptest.ResponseRecorder {
	t.Helper()
	body = withContractVersion(t, withFingerprint(t, s, body))
	rr := httptest.NewRecorder()
	s.handleApply(rr, httptest.NewRequest(http.MethodPost, "/apply", strings.NewReader(body)), []byte(body))
	return rr
}

// withFingerprint fills in the one a real caller would have got from the plan.
//
// The apply refuses a request without one — an absent fingerprint verifies
// vacuously, which is not a guarantee — so a test about anything else has to
// carry a current one. A test about the fingerprint itself supplies its own and
// this leaves it alone.
func withFingerprint(t *testing.T, s *server, body string) string {
	t.Helper()
	var fields map[string]any
	if err := json.Unmarshal([]byte(body), &fields); err != nil {
		return body
	}
	if _, present := fields["fingerprint"]; present {
		return body
	}
	var req ApplyRequest
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		return body
	}
	current, _, _, _, err := s.locate(req)
	if err != nil {
		// The target is unreadable in this test; the request never gets as far
		// as the comparison, so any value will do.
		fields["fingerprint"] = "unreadable"
	} else {
		fields["fingerprint"] = fingerprintSubject(current)
	}
	encoded, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func decodeOutcome(t *testing.T, rr *httptest.ResponseRecorder) ApplyOutcome {
	t.Helper()
	var out ApplyOutcome
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode outcome: %v (%s)", err, rr.Body.String())
	}
	return out
}

// 5.2/6.1 — the account is created as part of convergence, and the derived name
// is reported. No separate creation operation has to be sequenced before this.
func TestAnAbsentAccountIsCreatedByTheApplyItself(t *testing.T) {
	s, m := applyServer(t, `[]`)

	rr := postApply(t, s, `{"call_id":"c1","subject":"sub-1","email":"ada@example.edu",
		"desired":{"group":["lab_makers"],"enabled":true,"smb_enabled":true}}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	out := decodeOutcome(t, rr)
	if out.Effect != EffectApplied || out.Username != "ada" {
		t.Fatalf("want the created name reported: %+v", out)
	}
	if len(m.creates) != 1 {
		t.Fatalf("want exactly one create, got %d", len(m.creates))
	}
	if got := m.creates[0]["username"]; got != "ada" {
		t.Errorf("created the wrong name: %v", got)
	}
	// The binding is recorded, or the next apply would find the account
	// unbound and halt as a conflict.
	b, bound, err := s.store.GetBinding("sub-1")
	if err != nil || !bound {
		t.Fatalf("the binding must be recorded: %v bound=%t", err, bound)
	}
	if b.Username != "ada" || b.UID == 0 {
		t.Errorf("the binding must carry the name and the uid: %+v", b)
	}
}

// 5.5 — re-applying an unchanged set is a no-op with no mutating call. Not an
// optimisation: the drain re-drives rows, so an apply that wrote every time
// would rewrite every account on every pass.
func TestReApplyingAnUnchangedSetIssuesNoMutatingCall(t *testing.T) {
	s, m := applyServer(t, `[{"username":"ada","id":11,"uid":3001,"locked":false,"smb":true,"groups":[42]}]`)
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "ada", UID: 3001}); err != nil {
		t.Fatal(err)
	}

	rr := postApply(t, s, `{"call_id":"c1","subject":"sub-1","email":"ada@example.edu",
		"desired":{"group":["lab_makers"],"enabled":true,"smb_enabled":true}}`)
	out := decodeOutcome(t, rr)
	if out.Effect != EffectNoChange {
		t.Fatalf("want no_change, got %+v", out)
	}
	if len(m.updates) != 0 || len(m.creates) != 0 {
		t.Fatalf("nothing may be written: %d updates %d creates", len(m.updates), len(m.creates))
	}
}

// 5.4/5.5 — a reduced set converges to exactly the remaining groups, and a set
// resolving both lifecycle fields to disabled locks and clears SMB. Then the
// same path restores it, with no second account.
func TestConvergenceIsLevelTriggeredInBothDirections(t *testing.T) {
	s, m := applyServer(t, `[{"username":"ada","id":11,"uid":3001,"locked":false,"smb":true,"groups":[41,42]}]`)
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "ada", UID: 3001}); err != nil {
		t.Fatal(err)
	}

	off := postApply(t, s, `{"call_id":"c1","subject":"sub-1","email":"ada@x.edu",
		"desired":{"group":[],"enabled":false,"smb_enabled":false}}`)
	if decodeOutcome(t, off).Effect != EffectApplied {
		t.Fatalf("want applied: %s", off.Body.String())
	}
	if len(m.updates) != 1 {
		t.Fatalf("want one update, got %d", len(m.updates))
	}
	u := m.updates[0]
	// An empty list is an instruction, not an omission: it means "in no managed
	// group", which is different from the field being absent.
	if groups, ok := u["groups"].([]apiID); !ok || len(groups) != 0 {
		t.Errorf("groups must be set to empty, got %v", u["groups"])
	}
	if u["locked"] != true || u["smb"] != false {
		t.Errorf("disabling must lock and clear SMB: %v", u)
	}

	// And back, through the same path, with no second account created.
	s2, m2 := applyServer(t, `[{"username":"ada","id":11,"uid":3001,"locked":true,"smb":false,"groups":[]}]`)
	if err := s2.store.PutBinding(Binding{SubjectID: "sub-1", Username: "ada", UID: 3001}); err != nil {
		t.Fatal(err)
	}
	on := postApply(t, s2, `{"call_id":"c2","subject":"sub-1","email":"ada@x.edu",
		"desired":{"group":["lab_makers"],"enabled":true,"smb_enabled":true}}`)
	if decodeOutcome(t, on).Effect != EffectApplied {
		t.Fatalf("restoration must go through the same apply: %s", on.Body.String())
	}
	if len(m2.creates) != 0 {
		t.Fatal("restoring must not create a second account")
	}
	if m2.updates[0]["locked"] != false || m2.updates[0]["smb"] != true {
		t.Errorf("want unlocked with SMB back: %v", m2.updates[0])
	}
}

// 6.7/6.8 — an unbound account holding the derived name halts the operation and
// surfaces the decision. Adopting silently would hand a subject's entitlements
// to whoever already owns that account.
func TestACollidingUnboundAccountHaltsAndSurfacesTheDecision(t *testing.T) {
	s, m := applyServer(t, `[{"username":"ada","id":19,"uid":9001,"locked":false,"smb":false,"groups":[]}]`)

	rr := postApply(t, s, `{"call_id":"c1","subject":"sub-1","email":"ada@example.edu",
		"desired":{"group":["lab_makers"],"enabled":true}}`)
	if rr.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d (%s)", rr.Code, rr.Body.String())
	}
	out := decodeOutcome(t, rr)
	if out.Effect != EffectBlocked || out.Conflict == nil {
		t.Fatalf("want a surfaced conflict: %+v", out)
	}
	if !out.Conflict.Adoptable || out.Conflict.UID != 9001 {
		t.Errorf("the conflict must carry what an operator needs to decide: %+v", out.Conflict)
	}
	// No create, no adopt, no modify.
	if len(m.creates) != 0 || len(m.updates) != 0 {
		t.Fatalf("nothing may be written: %d creates %d updates", len(m.creates), len(m.updates))
	}
	if _, bound, _ := s.store.GetBinding("sub-1"); bound {
		t.Fatal("and nothing may be bound")
	}
}

// 6.11/6.12 — a later email change must not rename an existing account.
// Renaming disturbs its home directory, its ACL entries and its SMB identity.
func TestAnEmailChangeDoesNotRenameAnExistingAccount(t *testing.T) {
	s, m := applyServer(t, `[{"username":"ada","id":11,"uid":3001,"locked":false,"smb":true,"groups":[42]}]`)
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "ada", UID: 3001}); err != nil {
		t.Fatal(err)
	}

	rr := postApply(t, s, `{"call_id":"c1","subject":"sub-1","email":"ada.lovelace@example.edu",
		"desired":{"group":["lab_makers"],"enabled":true,"smb_enabled":true}}`)
	out := decodeOutcome(t, rr)
	if out.Username != "ada" {
		t.Fatalf("the recorded binding is authoritative, got %q", out.Username)
	}
	if len(m.creates) != 0 {
		t.Fatal("a new email must not create a second account")
	}
	for _, u := range m.updates {
		if _, renaming := u["username"]; renaming {
			t.Fatal("an apply must never rename an account")
		}
	}
	// And the binding still resolves the subject.
	b, bound, _ := s.store.GetBinding("sub-1")
	if !bound || b.Username != "ada" {
		t.Fatalf("the binding must survive: %+v bound=%t", b, bound)
	}
}

// 6.9/6.10 — a stable uid whose username changed out of band is a RENAME, not a
// missing account. Reporting it as missing would create a replacement while the
// original kept the home data.
func TestAnOutOfBandRenameIsRecognisedRatherThanRecreated(t *testing.T) {
	s, m := applyServer(t, `[{"username":"ada_renamed","id":11,"uid":3001,"locked":false,"smb":true,"groups":[42]}]`)
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "ada", UID: 3001}); err != nil {
		t.Fatal(err)
	}

	rr := postApply(t, s, `{"call_id":"c1","subject":"sub-1","email":"ada@x.edu",
		"desired":{"group":["lab_makers"],"enabled":true,"smb_enabled":true}}`)
	out := decodeOutcome(t, rr)
	if len(m.creates) != 0 {
		t.Fatal("a renamed account must not be recreated")
	}
	if out.Username != "ada_renamed" {
		t.Fatalf("the binding must follow the uid, got %q", out.Username)
	}
	b, _, _ := s.store.GetBinding("sub-1")
	if b.Username != "ada_renamed" {
		t.Errorf("and the recorded name must be updated to match, got %q", b.Username)
	}
}

// 5.10/5.11 — a subject mutated out of band between plan and apply causes
// refusal with that subject named, and nothing is applied.
func TestAMovedSubjectRefusesTheApplyAndMutatesNothing(t *testing.T) {
	s, m := applyServer(t, `[{"username":"ada","id":11,"uid":3001,"locked":false,"smb":true,"groups":[42]}]`)
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "ada", UID: 3001}); err != nil {
		t.Fatal(err)
	}

	rr := postApply(t, s, `{"call_id":"c1","subject":"sub-1","email":"ada@x.edu","fingerprint":"stale",
		"desired":{"group":[],"enabled":false}}`)
	if rr.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d (%s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "sub-1") {
		t.Errorf("the refusal must name the subject that moved: %s", rr.Body.String())
	}
	if len(m.updates) != 0 || len(m.creates) != 0 {
		t.Fatal("nothing may be written")
	}
}

// A fingerprint from the plan matches and the apply proceeds — the other half,
// without which the test above would pass for an apply that refuses everything.
func TestAMatchingFingerprintProceeds(t *testing.T) {
	s, _ := applyServer(t, `[{"username":"ada","id":11,"uid":3001,"locked":false,"smb":true,"groups":[42]}]`)
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "ada", UID: 3001}); err != nil {
		t.Fatal(err)
	}
	current := Subject{Username: "ada", UID: 3001, Groups: []string{"lab_makers"}, Enabled: true, SMBEnabled: true}

	rr := postApply(t, s, `{"call_id":"c1","subject":"sub-1","email":"ada@x.edu","fingerprint":"`+
		fingerprintSubject(&current)+`","desired":{"group":[],"enabled":false}}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("a matching fingerprint must proceed: %d (%s)", rr.Code, rr.Body.String())
	}
}

// 5.12/5.13 — replaying an operation id returns the original outcome without a
// second mutating call.
func TestAReplayedCallIdDoesNotMutateTwice(t *testing.T) {
	s, m := applyServer(t, `[]`)
	const body = `{"call_id":"c1","subject":"sub-1","email":"ada@x.edu","desired":{"group":["lab_makers"]}}`

	first := postApply(t, s, body)
	if first.Code != http.StatusOK {
		t.Fatalf("first apply: %s", first.Body.String())
	}
	second := postApply(t, s, body)
	if second.Code != http.StatusOK {
		t.Fatalf("replay: %s", second.Body.String())
	}
	if len(m.creates) != 1 {
		t.Fatalf("a replay must not create twice, got %d", len(m.creates))
	}
	if first.Body.String() != second.Body.String() {
		t.Errorf("a replay must return the original outcome:\n%s\n%s", first.Body.String(), second.Body.String())
	}
}

// A lifecycle refusal reaches the apply path, and reaches it as queued rather
// than failed.
func TestAnApplyDuringAMaintenanceWindowIsRefusedAsRetryable(t *testing.T) {
	s, m := applyServer(t, `[]`)
	_ = s.life.Set(LifecycleReadOnly, "maintenance")

	rr := postApply(t, s, `{"call_id":"c1","subject":"sub-1","email":"ada@x.edu","desired":{"group":[]}}`)
	if rr.Code != http.StatusServiceUnavailable || rr.Header().Get("Retry-After") == "" {
		t.Fatalf("want a retryable refusal, got %d %v", rr.Code, rr.Header())
	}
	if len(m.creates) != 0 {
		t.Fatal("nothing may be written")
	}
}

// A field this add-on does not understand is refused rather than ignored.
// Ignoring it would let the backend believe it had converged something nothing
// acted on.
func TestAnUnknownDesiredFieldIsRefused(t *testing.T) {
	s, m := applyServer(t, `[]`)

	rr := postApply(t, s, `{"call_id":"c1","subject":"sub-1","email":"ada@x.edu","desired":{"quota_bytes":5}}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d (%s)", rr.Code, rr.Body.String())
	}
	if len(m.creates) != 0 || len(m.updates) != 0 {
		t.Fatal("nothing may be written")
	}
}

// 5.9 — planning issues no mutating call, returns the apply path's shape, and
// produces a fingerprint that changes when the subject's target state changes.
func TestPlanningMutatesNothingAndFingerprintsWhatItRead(t *testing.T) {
	s, m := applyServer(t, `[{"username":"ada","id":11,"uid":3001,"locked":false,"smb":true,"groups":[42]}]`)
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "ada", UID: 3001}); err != nil {
		t.Fatal(err)
	}
	const body = `{"subjects":[{"subject":"sub-1","email":"ada@x.edu","desired":{"group":["builtin_users"]}}]}`

	rr := httptest.NewRecorder()
	planInto(t, s, rr, body)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	if len(m.updates) != 0 || len(m.creates) != 0 {
		t.Fatal("planning must mutate nothing")
	}

	var plan PlanResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	if len(plan.Outcomes) != 1 || plan.Outcomes[0].Effect != EffectApply {
		t.Fatalf("want one actionable row: %+v", plan.Outcomes)
	}
	if plan.Outcomes[0].Consequence == "" {
		t.Error("a plan must say what the subject is left holding")
	}
	before := plan.Outcomes[0].Fingerprint

	// The subject moves on the target; the fingerprint must move with it.
	moved, _ := applyServer(t, `[{"username":"ada","id":11,"uid":3001,"locked":true,"smb":true,"groups":[42]}]`)
	if err := moved.store.PutBinding(Binding{SubjectID: "sub-1", Username: "ada", UID: 3001}); err != nil {
		t.Fatal(err)
	}
	rr2 := httptest.NewRecorder()
	planInto(t, moved, rr2, body)
	var after PlanResponse
	if err := json.Unmarshal(rr2.Body.Bytes(), &after); err != nil {
		t.Fatal(err)
	}
	if after.Outcomes[0].Fingerprint == before {
		t.Fatal("the fingerprint must change when the subject's target state does")
	}
}

// 5.6/5.7 — the per-request cap is defence in depth, and it reports the count it
// computed. The authoritative cohort guard is the backend's, where the cohort
// exists.
func TestAnOversizedRequestIsRefusedWithTheCountItComputed(t *testing.T) {
	s, m := applyServer(t, `[]`)

	var b strings.Builder
	b.WriteString(`{"subjects":[`)
	for i := range perRequestSubjectCap + 1 {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`{"subject":"s`)
		b.WriteString(strconvI(int64(i)))
		b.WriteString(`","email":"a@x.edu","desired":{}}`)
	}
	b.WriteString(`]}`)

	rr := httptest.NewRecorder()
	planInto(t, s, rr, b.String())
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d (%s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), strconvI(int64(perRequestSubjectCap+1))) {
		t.Errorf("the refusal must report the count: %s", rr.Body.String())
	}
	if len(m.updates)+len(m.creates) != 0 {
		t.Fatal("nothing may be written")
	}
}

// 5.14/5.15 — a subject missing from the expected set is reported and never
// deleted or locked. Deletion by absence would be catastrophic and the design
// forbids it outright: tombstones only.
func TestNothingInTheApplyPathDeletesAnAccount(t *testing.T) {
	for _, f := range []string{"apply.go", "plan.go", "subjects.go", "server.go"} {
		src, err := readSource(f)
		if err != nil {
			t.Fatal(err)
		}
		// Matched as an INVOCATION, not as a mention. The capability probe
		// declares which methods each operation depends on, and a guard that
		// fired on the declaration would be weakened until it fired on nothing.
		for _, forbidden := range []string{"user.delete", "group.delete"} {
			for _, form := range []string{`call("` + forbidden, `Call("` + forbidden} {
				if strings.Contains(src, form) {
					t.Errorf("%s calls %s — deletion by absence is the failure this design forbids outright", f, forbidden)
				}
			}
		}
	}
}

func readSource(name string) (string, error) {
	b, err := os.ReadFile(name)
	return string(b), err
}

// A field the request does not name is one this target does not manage for that
// subject. Converging it to a zero value would be inventing an instruction —
// and the instruction it invents is "remove them from every group".
func TestAnUnnamedFieldIsLeftAloneRatherThanZeroed(t *testing.T) {
	s, m := applyServer(t, `[{"username":"ada","id":11,"uid":3001,"locked":false,"smb":true,"groups":[41,42]}]`)
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "ada", UID: 3001}); err != nil {
		t.Fatal(err)
	}

	// Only the lifecycle half is named; `group` is absent.
	rr := postApply(t, s, `{"call_id":"c1","subject":"sub-1","email":"ada@x.edu","desired":{"smb_enabled":false}}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	if len(m.updates) != 1 {
		t.Fatalf("want one update, got %d", len(m.updates))
	}
	if _, touched := m.updates[0]["groups"]; touched {
		t.Fatalf("an unnamed field must not be written: %v", m.updates[0])
	}
	if m.updates[0]["smb"] != false {
		t.Errorf("the named field must be converged: %v", m.updates[0])
	}
}

// A mutation against an untested major is refused, not attempted. Reads
// continue — this is the half that could break something.
func TestAnUntestedMajorRefusesTheApplyWithoutWriting(t *testing.T) {
	s, m := applyServer(t, `[]`)
	// On the NAS as well as in the cache: every connection re-probes, so a
	// version set only here would be read back over on the first call.
	m.fakeRPC.version, s.nas.version = "27.10.0", "27.10.0"

	rr := postApply(t, s, `{"call_id":"c1","subject":"sub-1","email":"ada@x.edu","desired":{"group":["lab_makers"]}}`)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d (%s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "27.10") {
		t.Errorf("the refusal must name the version it saw: %s", rr.Body.String())
	}
	if len(m.creates) != 0 || len(m.updates) != 0 {
		t.Fatal("nothing may be written against an untested major")
	}
}

// The plan and the apply must derive the same name. A plan that predicts a name
// the apply does not produce is worse than no plan: an operator approves the
// creation of one account and a different one appears.
func TestThePlanAndTheApplyAgreeOnTheDerivedName(t *testing.T) {
	// A subject we manage already holds `ada`, so the derivation must suffix
	// past it — on both paths, identically.
	const users = `[{"username":"ada","id":11,"uid":3001,"locked":false,"smb":true,"groups":[42]}]`

	s, _ := applyServer(t, users)
	if err := s.store.PutBinding(Binding{SubjectID: "sub-other", Username: "ada", UID: 3001}); err != nil {
		t.Fatal(err)
	}
	const planBody = `{"subjects":[{"subject":"sub-1","email":"ada@example.edu","desired":{"group":["lab_makers"]}}]}`
	rr := httptest.NewRecorder()
	planInto(t, s, rr, planBody)
	var plan PlanResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	if len(plan.Outcomes) != 1 || plan.Outcomes[0].Username == "" {
		t.Fatalf("the plan must name what it would create: %+v", plan.Outcomes)
	}
	planned := plan.Outcomes[0].Username
	if planned == "ada" {
		t.Fatal("it must not plan to reuse another subject's account")
	}

	applied := postApply(t, s, `{"call_id":"c1","subject":"sub-1","email":"ada@example.edu","desired":{"group":["lab_makers"]}}`)
	out := decodeOutcome(t, applied)
	if out.Username != planned {
		t.Fatalf("the apply created %q where the plan said %q", out.Username, planned)
	}
}

// The plan reports the same conflict the apply halts on. An unbound account
// holding the derived name is not a collision to route around — suffixing past
// it would plan a second account for one person while the older one keeps the
// home directory every share points at.
func TestThePlanReportsAConflictRatherThanPlanningAroundIt(t *testing.T) {
	s, _ := applyServer(t, `[{"username":"ada","id":19,"uid":9001,"locked":false,"smb":false,"groups":[]}]`)

	const body = `{"subjects":[{"subject":"sub-1","email":"ada@example.edu","desired":{"group":["lab_makers"]}}]}`
	rr := httptest.NewRecorder()
	planInto(t, s, rr, body)

	var plan PlanResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	if len(plan.Outcomes) != 1 {
		t.Fatalf("want one row: %+v", plan.Outcomes)
	}
	got := plan.Outcomes[0]
	if got.Effect != EffectBlocked || got.Conflict == nil {
		t.Fatalf("want a blocked row carrying the conflict, got %+v", got)
	}
	if got.Username != "ada" || !got.Conflict.Adoptable {
		t.Errorf("the conflict must name the account an operator has to decide about: %+v", got.Conflict)
	}

	// And the apply reaches the same verdict, so the operator is not shown one
	// thing and given another.
	applied := postApply(t, s, `{"call_id":"c1","subject":"sub-1","email":"ada@example.edu","desired":{"group":["lab_makers"]}}`)
	if applied.Code != http.StatusConflict {
		t.Fatalf("the apply must halt on the same conflict, got %d (%s)", applied.Code, applied.Body.String())
	}
}

// An absent fingerprint verifies vacuously. §8's guarantee is that the diff an
// operator approved is the diff that lands, and a check a caller can opt out of
// by omitting a field is not that.
func TestAnApplyWithoutAFingerprintIsRefused(t *testing.T) {
	s, m := applyServer(t, `[{"username":"ada","id":11,"uid":3001,"locked":false,"smb":true,"groups":[42]}]`)
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "ada", UID: 3001}); err != nil {
		t.Fatal(err)
	}

	const body = `{"call_id":"c1","subject":"sub-1","email":"ada@x.edu","desired":{"enabled":false}}`
	rr := httptest.NewRecorder()
	applyInto(t, s, rr, body)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want a refusal, got %d (%s)", rr.Code, rr.Body.String())
	}
	if len(m.updates) != 0 {
		t.Fatal("nothing may be written for a call that verified nothing")
	}
}

// Groups are written as the ids the read resolved names FROM. A write in names
// against a read in ids leaves an account in the wrong groups and the next read
// calls it converged.
func TestGroupsAreWrittenAsTheIdsTheReadResolves(t *testing.T) {
	s, m := applyServer(t, `[{"username":"ada","id":11,"uid":3001,"locked":false,"smb":true,"groups":[41]}]`)
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "ada", UID: 3001}); err != nil {
		t.Fatal(err)
	}

	rr := postApply(t, s, `{"call_id":"c1","subject":"sub-1","email":"ada@x.edu","desired":{"group":["lab_makers"]}}`)
	if decodeOutcome(t, rr).Effect != EffectApplied {
		t.Fatalf("want applied: %s", rr.Body.String())
	}
	if len(m.updates) != 1 {
		t.Fatalf("want one update, got %d", len(m.updates))
	}
	groups, ok := m.updates[0]["groups"].([]apiID)
	if !ok || len(groups) != 1 || groups[0].String() != "42" {
		t.Fatalf("want the record id of lab_makers, got %#v", m.updates[0]["groups"])
	}

	// And the account is addressed by its record key, not by its unix uid. Root
	// is id 1 and uid 0; pass one where the other is meant and the write lands
	// on somebody else.
	args, _ := lastCallParams(m, "user.update").([]any)
	if len(args) != 2 {
		t.Fatalf("want two arguments to user.update, got %v", args)
	}
	if id, ok := args[0].(apiID); !ok || id.String() != "11" {
		t.Fatalf("want the record id, got %#v", args[0])
	}
}

// A group the target does not have is refused, not dropped. Dropping it applies
// a smaller set than the one that was approved and reports it as converged.
func TestAnUnknownGroupIsRefusedRatherThanDropped(t *testing.T) {
	s, m := applyServer(t, `[{"username":"ada","id":11,"uid":3001,"locked":false,"smb":true,"groups":[41]}]`)
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "ada", UID: 3001}); err != nil {
		t.Fatal(err)
	}

	rr := postApply(t, s, `{"call_id":"c1","subject":"sub-1","email":"ada@x.edu","desired":{"group":["no_such_group"]}}`)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want a refusal, got %d (%s)", rr.Code, rr.Body.String())
	}
	if len(m.updates) != 0 {
		t.Fatal("nothing may be written when a named group could not be resolved")
	}
}

// A field this add-on does not understand is refused at the boundary, not
// dropped. Two separately deployed binaries disagreeing about a field is the
// skew the contract version exists to surface.
func TestAnUnknownRequestFieldIsRefused(t *testing.T) {
	s, m := applyServer(t, `[]`)

	const body = `{"call_id":"c1","subject":"sub-1","fingerprint":"x","desired":{},"escalate":true}`
	rr := httptest.NewRecorder()
	applyInto(t, s, rr, body)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want a refusal, got %d (%s)", rr.Code, rr.Body.String())
	}
	if len(m.creates) != 0 {
		t.Fatal("nothing may be written for a body that was not fully understood")
	}
}

func lastCallParams(m *mutatingRPC, method string) any {
	for i := len(m.fakeRPC.calls) - 1; i >= 0; i-- {
		if m.fakeRPC.calls[i] == method {
			return m.fakeRPC.params[i]
		}
	}
	return nil
}

// cappedUsers is a user list one longer than a single read returns, so every
// read of it comes back truncated.
func cappedUsers() string {
	var b strings.Builder
	b.WriteString("[")
	for i := range subjectReadCap + 1 {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `{"username":"u%d","uid":%d,"locked":false,"smb":true,"groups":[42]}`, i, 4000+i)
	}
	b.WriteString("]")
	return b.String()
}

// §17 — the write path refuses what the plan already refuses.
//
// `plan` blocks a create from a capped read: an absence read out of a truncated
// list is not an absence, and the fingerprint cannot tell the two apart because
// "not in the read" and "not on the target" hash identically. `apply` did not,
// so a subject whose account sits past the cap got a SECOND one — while the
// first kept the home directory every share points at.
func TestAnApplyWillNotCreateFromATruncatedRead(t *testing.T) {
	s, rpc := applyServer(t, cappedUsers())

	rr := postApply(t, s, `{"call_id":"c1","subject":"sub-past-the-cap","email":"ada@example.edu",
		"desired":{"group":["lab_makers"],"enabled":true}}`)

	if rr.Code != http.StatusConflict {
		t.Fatalf("want the create refused, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(rpc.creates) != 0 {
		t.Fatalf("nothing may be created from a read that could not see the whole target: %v", rpc.creates)
	}
	if !strings.Contains(rr.Body.String(), "proves nothing") {
		t.Errorf("the refusal must say why, not merely refuse: %s", rr.Body.String())
	}
}

// And it stays a refusal about ABSENCE only. A subject the read did find is
// found whether or not the list was capped, and refusing that one would stall
// every ordinary convergence on a large target.
func TestATruncatedReadStillConvergesASubjectItFound(t *testing.T) {
	// Inside the cap, so the read does see them — the flag is set by the list's
	// length, not by where this row happens to sit.
	users := `[{"username":"ada","uid":5500,"locked":false,"smb":true,"groups":[42]},` +
		strings.TrimPrefix(cappedUsers(), "[")
	s, _ := applyServer(t, users)
	if err := s.store.PutBinding(Binding{SubjectID: "sub-ada", Username: "ada", UID: 5500}); err != nil {
		t.Fatal(err)
	}

	rr := postApply(t, s, `{"call_id":"c2","subject":"sub-ada","email":"ada@example.edu",
		"desired":{"group":["lab_makers"],"enabled":true}}`)

	if rr.Code != http.StatusOK {
		t.Fatalf("a subject the read found must still converge, got %d: %s", rr.Code, rr.Body.String())
	}
}

// The shipped defect, and the three cases that prove it is gone.
//
// `converge` used to answer with a projection: `applied := *current`, the
// REQUESTED values written over the managed fields, and a fingerprint of that.
// So the dominant path — every update to an account that already exists —
// reported intent in the shape of an observation, and the next plan's staleness
// check compared its fingerprint against a claim rather than against the target.
// The create path's own comment had stated the rule since the day it was
// written: a fingerprint computed from a state this add-on invented is a
// fingerprint the next plan verifies against nothing.

// A target that stores something other than what was asked for. Normalisation,
// coercion and silent refusal all look like this, and the projection reported
// every one of them as a clean, confirmed write.
func TestTheFingerprintComesFromTheReadAndNotFromTheRequest(t *testing.T) {
	s, m := applyServer(t, `[{"username":"ada","id":11,"uid":3001,"locked":true,"smb":false,"groups":[42]}]`)
	// The write says unlock; the target keeps it locked.
	m.divergeOn = map[string]any{"locked": true}
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "ada", UID: 3001}); err != nil {
		t.Fatal(err)
	}

	rr := postApply(t, s, `{"call_id":"c1","subject":"sub-1","email":"ada@x.edu",
		"desired":{"group":["lab_makers"],"enabled":true}}`)
	out := decodeOutcome(t, rr)
	if out.Effect != EffectApplied {
		t.Fatalf("want applied: %s", rr.Body.String())
	}

	// The invariant, stated as the next plan would: the fingerprint this apply
	// reports must equal the one a fresh read produces. Under the projection it
	// equalled a fingerprint of the request, and no read anywhere agreed with it.
	after, err := s.readBack("ada")
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if want := fingerprintSubject(after); out.Fingerprint != want {
		t.Fatalf("the fingerprint must digest what the target holds\n  got  %s\n  want %s",
			out.Fingerprint, want)
	}
	if enabled, ok := out.Observed[FieldEnabled].(bool); !ok || enabled {
		t.Fatalf("observed must report what the target stored, not what was asked: %v", out.Observed)
	}
}

// Managed fields only. An unmanaged one is not "unchanged", it is out of scope,
// and reporting it would invite a base claiming authority over something Syndra
// never set.
func TestObservedCarriesOnlyTheFieldsTheApplyManages(t *testing.T) {
	s, _ := applyServer(t, `[{"username":"ada","id":11,"uid":3001,"locked":false,"smb":true,"groups":[41,42]}]`)
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "ada", UID: 3001}); err != nil {
		t.Fatal(err)
	}

	rr := postApply(t, s, `{"call_id":"c1","subject":"sub-1","email":"ada@x.edu",
		"desired":{"enabled":false}}`)
	out := decodeOutcome(t, rr)
	if _, present := out.Observed[FieldEnabled]; !present {
		t.Fatalf("the managed field must be observed: %v", out.Observed)
	}
	for _, unmanaged := range []string{FieldGroup, FieldSMBEnabled} {
		if _, present := out.Observed[unmanaged]; present {
			t.Errorf("%s is not managed by this apply and must not be reported as observed: %v",
				unmanaged, out.Observed)
		}
	}
}

// The write landed and its result cannot be read. Both facts are reported: the
// effect is `applied`, because it was, and everything that would have to be an
// observation is absent. Calling it a failure invites a retry of a mutation
// already performed; calling it an ordinary success hands the next plan a
// fingerprint nobody read — which is the defect, arriving through the error path.
func TestAWriteThatCannotBeReadBackIsReportedAsUnverified(t *testing.T) {
	s, m := applyServer(t, `[{"username":"ada","id":11,"uid":3001,"locked":true,"smb":false,"groups":[42]}]`)
	m.failReadAfterUpdate = true
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "ada", UID: 3001}); err != nil {
		t.Fatal(err)
	}

	rr := postApply(t, s, `{"call_id":"c1","subject":"sub-1","email":"ada@x.edu",
		"desired":{"group":["lab_makers"],"enabled":true}}`)
	out := decodeOutcome(t, rr)
	if out.Effect != EffectApplied || !out.Unverified {
		t.Fatalf("want an applied-but-unverified outcome: %s", rr.Body.String())
	}
	if out.Fingerprint != "" {
		t.Errorf("an unverified apply must carry no fingerprint, got %q", out.Fingerprint)
	}
	if out.Observed != nil {
		t.Errorf("an unverified apply must carry no observed values, got %v", out.Observed)
	}
	if !strings.Contains(out.Consequence, "not been confirmed") {
		t.Errorf("the operator must be told the result is unconfirmed: %q", out.Consequence)
	}
	// And exactly one write. The failure is in the read that follows it, and a
	// retry inside the add-on would repeat a mutation nobody has verified.
	if len(m.updates) != 1 {
		t.Fatalf("want exactly one write, got %d", len(m.updates))
	}
}
