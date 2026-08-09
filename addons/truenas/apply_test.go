package main

import (
	"encoding/json"
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

// mutatingRPC records every write so a test can assert that none happened.
type mutatingRPC struct {
	fakeRPC
	updates []map[string]any
	creates []map[string]any
	nextUID int64
}

func (m *mutatingRPC) Call(method string, timeout int64, params any) (json.RawMessage, error) {
	// The error flag applies to writes as well as reads. A fake whose failure
	// switch covers only half its methods is a fake that quietly passes the
	// tests about the other half.
	if m.fakeRPC.err != nil {
		m.fakeRPC.calls = append(m.fakeRPC.calls, method)
		return nil, m.fakeRPC.err
	}
	switch method {
	case "user.update":
		args, _ := params.([]any)
		if len(args) == 2 {
			if u, ok := args[1].(map[string]any); ok {
				m.updates = append(m.updates, u)
			}
		}
		return json.RawMessage(`null`), nil
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
		return json.RawMessage(strconvI(m.nextUID)), nil
	}
	return m.fakeRPC.Call(method, timeout, params)
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
	s.nas.version = "25.04.2"
	return s, m
}

func postApply(t *testing.T, s *server, body string) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	s.handleApply(rr, httptest.NewRequest(http.MethodPost, "/apply", strings.NewReader(body)), []byte(body))
	return rr
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
	s, m := applyServer(t, `[{"username":"ada","uid":3001,"locked":false,"smb":true,"groups":[900]}]`)
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
	s, m := applyServer(t, `[{"username":"ada","uid":3001,"locked":false,"smb":true,"groups":[545,900]}]`)
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
	if groups, ok := u["groups"].([]string); !ok || len(groups) != 0 {
		t.Errorf("groups must be set to empty, got %v", u["groups"])
	}
	if u["locked"] != true || u["smb"] != false {
		t.Errorf("disabling must lock and clear SMB: %v", u)
	}

	// And back, through the same path, with no second account created.
	s2, m2 := applyServer(t, `[{"username":"ada","uid":3001,"locked":true,"smb":false,"groups":[]}]`)
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
	s, m := applyServer(t, `[{"username":"ada","uid":9001,"locked":false,"smb":false,"groups":[]}]`)

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
	s, m := applyServer(t, `[{"username":"ada","uid":3001,"locked":false,"smb":true,"groups":[900]}]`)
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
	s, m := applyServer(t, `[{"username":"ada_renamed","uid":3001,"locked":false,"smb":true,"groups":[900]}]`)
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
	s, m := applyServer(t, `[{"username":"ada","uid":3001,"locked":false,"smb":true,"groups":[900]}]`)
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
	s, _ := applyServer(t, `[{"username":"ada","uid":3001,"locked":false,"smb":true,"groups":[900]}]`)
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
	s, m := applyServer(t, `[{"username":"ada","uid":3001,"locked":false,"smb":true,"groups":[900]}]`)
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "ada", UID: 3001}); err != nil {
		t.Fatal(err)
	}
	const body = `{"subjects":[{"subject":"sub-1","email":"ada@x.edu","desired":{"group":["builtin_users"]}}]}`

	rr := httptest.NewRecorder()
	s.handlePlan(rr, httptest.NewRequest(http.MethodPost, "/plan", strings.NewReader(body)), []byte(body))
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
	moved, _ := applyServer(t, `[{"username":"ada","uid":3001,"locked":true,"smb":true,"groups":[900]}]`)
	if err := moved.store.PutBinding(Binding{SubjectID: "sub-1", Username: "ada", UID: 3001}); err != nil {
		t.Fatal(err)
	}
	rr2 := httptest.NewRecorder()
	moved.handlePlan(rr2, httptest.NewRequest(http.MethodPost, "/plan", strings.NewReader(body)), []byte(body))
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
	s.handlePlan(rr, httptest.NewRequest(http.MethodPost, "/plan", strings.NewReader(b.String())), []byte(b.String()))
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
	s, m := applyServer(t, `[{"username":"ada","uid":3001,"locked":false,"smb":true,"groups":[545,900]}]`)
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
	s.nas.version = "27.10.0"

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
	const users = `[{"username":"ada","uid":3001,"locked":false,"smb":true,"groups":[900]}]`

	s, _ := applyServer(t, users)
	if err := s.store.PutBinding(Binding{SubjectID: "sub-other", Username: "ada", UID: 3001}); err != nil {
		t.Fatal(err)
	}
	const planBody = `{"subjects":[{"subject":"sub-1","email":"ada@example.edu","desired":{"group":["lab_makers"]}}]}`
	rr := httptest.NewRecorder()
	s.handlePlan(rr, httptest.NewRequest(http.MethodPost, "/plan", strings.NewReader(planBody)), []byte(planBody))
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
	s, _ := applyServer(t, `[{"username":"ada","uid":9001,"locked":false,"smb":false,"groups":[]}]`)

	const body = `{"subjects":[{"subject":"sub-1","email":"ada@example.edu","desired":{"group":["lab_makers"]}}]}`
	rr := httptest.NewRecorder()
	s.handlePlan(rr, httptest.NewRequest(http.MethodPost, "/plan", strings.NewReader(body)), []byte(body))

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
