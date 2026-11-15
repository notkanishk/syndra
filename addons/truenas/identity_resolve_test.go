package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"
)

// One account identity (slice 7t-1): every caller names the account boundAccount names, through the
// module (identity_resolve.go), and a binding that names nothing is refused the same way everywhere.
// The table is every case of marked/unmarked × in/out of the id range × name found/gone × number
// free/held by another name; each caller is asserted against every case.

type idCase struct {
	name    string
	binding Binding // for sub-1; UIDKind empty is a contract-1 binding
	nas     string  // user.query rows
	want    string  // the account it names; "" with conflict false: none (gone)
	// conflict: it names nothing because another account holds the name or the number.
	conflict bool
}

// Record ids are not uids (11, 12): a write that lands on the wrong account shows.
var idCases = []idCase{
	{"in range, found", Binding{Username: "bob", UID: 300001, UIDKind: uidKindAccount},
		`[{"username":"bob","id":11,"uid":300001,"locked":false,"smb":false,"groups":[]}]`, "bob", false},
	{"in range, renamed", Binding{Username: "bob", UID: 300001, UIDKind: uidKindAccount},
		`[{"username":"bob_renamed","id":11,"uid":300001,"locked":false,"smb":false,"groups":[]}]`, "bob_renamed", false},
	{"in range, name held by another number", Binding{Username: "bob", UID: 300001, UIDKind: uidKindAccount},
		`[{"username":"bob","id":12,"uid":300002,"locked":false,"smb":false,"groups":[]}]`, "", true},
	{"in range, gone", Binding{Username: "bob", UID: 300001, UIDKind: uidKindAccount}, `[]`, "", false},
	{"out of range, found", Binding{Username: "bob", UID: 3051, UIDKind: uidKindAccount},
		`[{"username":"bob","id":11,"uid":3051,"locked":false,"smb":false,"groups":[]}]`, "bob", false},
	{"out of range, number held by another name", Binding{Username: "bob", UID: 3051, UIDKind: uidKindAccount},
		`[{"username":"eve","id":12,"uid":3051,"locked":false,"smb":false,"groups":[]}]`, "", true},
	{"out of range, name held by another number", Binding{Username: "bob", UID: 3051, UIDKind: uidKindAccount},
		`[{"username":"bob","id":12,"uid":3099,"locked":false,"smb":false,"groups":[]}]`, "", true},
	{"out of range, gone", Binding{Username: "bob", UID: 3051, UIDKind: uidKindAccount}, `[]`, "", false},
	{"legacy in range, found", Binding{Username: "bob", UID: 300001},
		`[{"username":"bob","id":11,"uid":300001,"locked":false,"smb":false,"groups":[]}]`, "bob", false},
	{"legacy in range, renamed", Binding{Username: "bob", UID: 300001},
		`[{"username":"bob_renamed","id":11,"uid":300001,"locked":false,"smb":false,"groups":[]}]`, "bob_renamed", false},
	{"legacy record key, found by name", Binding{Username: "bob", UID: 11},
		`[{"username":"bob","id":11,"uid":3051,"locked":false,"smb":false,"groups":[]}]`, "bob", false},
	// Legacy policy B (design 7t D6, pending owner Q2): halts, creates nothing. Policy A
	// (legacyHaltsOnHeldNumber false) reads it as gone: re-created beside the holder, who is
	// still never its account. Either answer is asserted by flipping the one constant.
	{"legacy out of range, number held by another name", Binding{Username: "bob", UID: 3051},
		`[{"username":"eve","id":12,"uid":3051,"locked":false,"smb":false,"groups":[]}]`, "", legacyHaltsOnHeldNumber},
	{"legacy out of range, gone", Binding{Username: "bob", UID: 3051}, `[]`, "", false},
	{"legacy uid 0, gone", Binding{Username: "bob"}, `[]`, "", false},
}

// row is the NAS row a case's account is, by username.
func (c idCase) row(t *testing.T, username string) (id string, uid int64) {
	t.Helper()
	var rows []struct {
		Username string `json:"username"`
		ID       int64  `json:"id"`
		UID      int64  `json:"uid"`
	}
	if err := json.Unmarshal([]byte(c.nas), &rows); err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.Username == username {
			return itoa(r.ID), r.UID
		}
	}
	t.Fatalf("%s: no row %s", c.name, username)
	return "", 0
}

// holderUID is the account that holds the name or number in a conflict.
func (c idCase) holderUID(t *testing.T) int64 {
	var rows []struct {
		UID int64 `json:"uid"`
	}
	_ = json.Unmarshal([]byte(c.nas), &rows)
	if len(rows) != 1 {
		t.Fatalf("%s: a conflict case has one holder", c.name)
	}
	return rows[0].UID
}

func (c idCase) server(t *testing.T) (*server, *mutatingRPC) {
	t.Helper()
	s, m := applyServer(t, c.nas)
	b := c.binding
	b.SubjectID = "sub-1"
	if err := s.store.PutBinding(b); err != nil {
		t.Fatal(err)
	}
	return s, m
}

// deleteRecorder is the elevated session a purge opens: what it was asked to delete.
type deleteRecorder struct{ deleted []any }

func (d *deleteRecorder) Call(method string, _ int64, params any) (json.RawMessage, error) {
	if method == "user.delete" {
		args, _ := params.([]any)
		d.deleted = append(d.deleted, args[0])
	}
	return envelope(`null`), nil
}
func (d *deleteRecorder) Ping() (string, error) { return "pong", nil }
func (d *deleteRecorder) Close() error          { return nil }

// writesTo is every account write the session sent, by record id, with the elevated deletes.
func writesTo(m *mutatingRPC, del *deleteRecorder) []string {
	var out []string
	for i, method := range m.fakeRPC.calls {
		if method == "user.update" || method == "user.delete" || method == "user.create" || strings.HasPrefix(method, "sharing.smb.") && method != "sharing.smb.query" {
			if args, ok := m.fakeRPC.params[i].([]any); ok && len(args) > 0 {
				raw, _ := json.Marshal(args[0])
				out = append(out, method+" "+strings.Trim(string(raw), `"`))
				continue
			}
			out = append(out, method)
		}
	}
	for _, id := range del.deleted {
		raw, _ := json.Marshal(id)
		out = append(out, "user.delete "+strings.Trim(string(raw), `"`))
	}
	return out
}

// loggedWrites is what the mutation log recorded.
func loggedWrites(t *testing.T, s *server) []Record {
	t.Helper()
	raw, err := os.ReadFile(mutationLogPath(t, s))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var out []Record
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if line == "" {
			continue
		}
		var r Record
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

func TestEveryCallerResolvesOneIdentity(t *testing.T) {
	type operation struct{ name, body string }
	ops := []operation{
		{"password.set", `{"call_id":"c1","subject":"sub-1","actor":"op_1","params":{"password":"` + theSecret + `"}}`},
		{"password.rotate", `{"call_id":"c1","subject":"sub-1","actor":"op_1"}`},
		{"account.purge", `{"call_id":"c1","subject":"sub-1","actor":"op_1","params":{"elevated_key":"k"}}`},
		{"storage.status", `{"call_id":"c1","subject":"sub-1"}`},
		{"activity.get", `{"call_id":"c1","subject":"sub-1"}`},
	}
	for _, c := range idCases {
		t.Run(c.name, func(t *testing.T) {
			var wantID string
			var wantUID int64
			if c.want != "" {
				wantID, wantUID = c.row(t, c.want)
			}

			for _, op := range ops {
				s, m := c.server(t)
				del := &deleteRecorder{}
				s.elevated = func(string) (rpc, error) { return del, nil }
				rr := postOperation(t, s, op.name, op.body)
				writes := writesTo(m, del)
				switch {
				case c.conflict:
					if rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), "now held by a different account") ||
						!strings.Contains(rr.Body.String(), "Stop tracking this person") {
						t.Fatalf("%s: want 409 in the add-on's words, got %d %s", op.name, rr.Code, rr.Body.String())
					}
					if strings.Contains(rr.Body.String(), itoa(c.holderUID(t))) {
						t.Errorf("%s: the holder's number is in the answer: %s", op.name, rr.Body.String())
					}
					if len(writes) != 0 {
						t.Errorf("%s: nothing is sent on a conflict, got %v", op.name, writes)
					}
					if logged := loggedWrites(t, s); len(logged) != 0 {
						t.Errorf("%s: nothing is logged on a conflict, got %+v", op.name, logged)
					}
					if op.name == "activity.get" && auditFilter(m) != "" {
						t.Errorf("activity.get: the audit log was read on a conflict (%q)", auditFilter(m))
					}
				case c.want == "":
					if rr.Code != http.StatusUnprocessableEntity || !strings.Contains(rr.Body.String(), "is no longer on TrueNAS") {
						t.Fatalf("%s: want 422 no account, got %d %s", op.name, rr.Code, rr.Body.String())
					}
					if len(writes) != 0 {
						t.Errorf("%s: nothing is sent for a gone account, got %v", op.name, writes)
					}
				default:
					if rr.Code != http.StatusOK {
						t.Fatalf("%s: want 200, got %d %s", op.name, rr.Code, rr.Body.String())
					}
					switch op.name {
					case "password.set", "password.rotate":
						if len(writes) != 1 || writes[0] != "user.update "+wantID {
							t.Errorf("%s: want one update of %s's record %s, got %v", op.name, c.want, wantID, writes)
						}
					case "account.purge":
						if len(writes) != 1 || writes[0] != "user.delete "+wantID {
							t.Errorf("%s: want %s's record %s deleted, got %v", op.name, c.want, wantID, writes)
						}
					case "activity.get":
						if filter := auditFilter(m); filter != c.want {
							t.Errorf("activity.get: want the audit log of %s, read %q", c.want, filter)
						}
					case "storage.status":
						// The account's own fields: every row here is unlocked with a password and
						// no SMB wanted, so usable, nothing to set, nothing to wait on.
						var res struct {
							Result StorageStatus `json:"result"`
						}
						if err := json.Unmarshal(rr.Body.Bytes(), &res); err != nil || !res.Result.Usable || res.Result.NeedsPassword ||
							res.Result.NeedsNewPassword || res.Result.Standing == nil || res.Result.Standing.WaitingOn != "none" || len(writes) != 0 {
							t.Errorf("storage.status: %v %s, writes %v", err, rr.Body.String(), writes)
						}
					}
				}
			}

			// Adoption's repeat check: the same account again is done, by the module.
			{
				s, m := c.server(t)
				rr := postOperation(t, s, "account.adopt",
					`{"call_id":"a1","subject":"sub-1","actor":"op_1","params":{"username":"`+c.binding.Username+`"}}`)
				switch {
				case c.conflict:
					if rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), "now held by a different account") {
						t.Fatalf("adopt: want 409, got %d %s", rr.Code, rr.Body.String())
					}
				case c.want == "":
					if rr.Code != http.StatusNotFound {
						t.Fatalf("adopt: want 404, got %d %s", rr.Code, rr.Body.String())
					}
				default:
					var res OperationResult
					_ = json.Unmarshal(rr.Body.Bytes(), &res)
					if rr.Code != http.StatusOK || res.AccountID != itoa(wantUID) {
						t.Fatalf("adopt: want %s's uid %d, got %d %s", c.want, wantUID, rr.Code, rr.Body.String())
					}
				}
				if w := writesTo(m, &deleteRecorder{}); len(w) != 0 {
					t.Errorf("adopt: touches no account, got %v", w)
				}
			}

			// The apply (locate).
			{
				s, m := c.server(t)
				rr := postApply(t, s, oneApply)
				out := decodeOutcome(t, rr)
				switch {
				case c.conflict:
					if rr.Code != http.StatusConflict || problemCode(t, rr) != CodeBindingConflict || len(m.updates)+len(m.creates) != 0 {
						t.Fatalf("apply: want BINDING_CONFLICT and nothing written, got %d %s", rr.Code, rr.Body.String())
					}
				case c.want == "":
					if len(m.creates) != 1 {
						t.Fatalf("apply: a gone account is re-created, got %d %s", rr.Code, rr.Body.String())
					}
				default:
					if rr.Code != http.StatusOK || len(m.creates) != 0 || out.AccountID != itoa(wantUID) {
						t.Fatalf("apply: want %s converged, got %d %s", c.want, rr.Code, rr.Body.String())
					}
					for _, w := range writesTo(m, &deleteRecorder{}) {
						if w != "user.update "+wantID {
							t.Errorf("apply: wrote %s, want only %s's record %s", w, c.want, wantID)
						}
					}
				}
			}

			// The plan (resolveIn), over a read of the same accounts.
			{
				s, _ := c.server(t)
				rr := httptest.NewRecorder()
				planInto(t, s, rr, `{"subjects":[{"subject":"sub-1","email":"bob@example.edu","desired":{"group":["lab_makers"],"enabled":true}}]}`)
				var plan PlanResponse
				if err := json.Unmarshal(rr.Body.Bytes(), &plan); err != nil || len(plan.Outcomes) != 1 {
					t.Fatalf("plan: %v %s", err, rr.Body.String())
				}
				out := plan.Outcomes[0]
				switch {
				case c.conflict:
					if out.Effect != EffectBlocked || out.Conflict == nil {
						t.Fatalf("plan: want blocked on the conflict, got %+v", out)
					}
				case c.want == "":
					if !strings.HasPrefix(out.Detail, "Creates ") {
						t.Fatalf("plan: want a create, got %+v", out)
					}
				default:
					if out.Effect == EffectBlocked || out.AccountID != itoa(wantUID) {
						t.Fatalf("plan: want %s, got %+v", c.want, out)
					}
				}

				// The views over a read (standing, statistics, adoption's check): exactly the account
				// named is bound, the holder of a conflict never.
				snap, err := s.readSubjects()
				if err != nil {
					t.Fatal(err)
				}
				bound, _ := s.boundUIDs(snap.Subjects)
				for i := range snap.Subjects {
					a := &snap.Subjects[i]
					isWant := a.Username == c.want
					if _, got := bound[a.UID]; got != isWant {
						t.Errorf("stats: %s bound=%t, want %t", a.Username, got, isWant)
					}
					if _, got := s.wishesFor(snap.Subjects)[a.UID]; got != isWant {
						t.Errorf("standing: %s bound=%t, want %t", a.Username, got, isWant)
					}
					if got, _ := s.accountBound(a); got != isWant {
						t.Errorf("accountBound: %s bound=%t, want %t", a.Username, got, isWant)
					}
				}

				// /subjects says it on the wire (slice 7t-2), from the same rule: the account named
				// carries the subject and no other row does; a conflict is listed with its holder and
				// the add-on's words, and the holder is never a row of the subject's.
				rr, got := getSubjects(t, s, true)
				if rr.Code != http.StatusOK {
					t.Fatalf("/subjects: %d %s", rr.Code, rr.Body.String())
				}
				for _, v := range got.Subjects {
					want := ""
					if v.Username == c.want {
						want = "sub-1"
					}
					if v.Subject != want {
						t.Errorf("/subjects: %s carries subject %q, want %q", v.Username, v.Subject, want)
					}
				}
				switch {
				case c.conflict:
					if len(got.Conflicts) != 1 {
						t.Fatalf("/subjects: want the one conflict, got %+v", got.Conflicts)
					}
					cf := got.Conflicts[0]
					if cf.Subject != "sub-1" || cf.AccountID != itoa(c.holderUID(t)) || cf.Adoptable || cf.Detail == "" || cf.Next == "" {
						t.Errorf("/subjects: conflict %+v, want sub-1's with holder uid %d and its words", cf, c.holderUID(t))
					}
				case len(got.Conflicts) != 0:
					t.Errorf("/subjects: no conflict here, got %+v", got.Conflicts)
				}
			}
		})
	}
}

// auditFilter is the username the last audit read asked for.
func auditFilter(m *mutatingRPC) string {
	args, _ := lastCallParams(m, "audit.query").([]any)
	if len(args) == 0 {
		return ""
	}
	q, _ := args[0].(map[string]any)
	filters, _ := q["query-filters"].([][]any)
	for _, f := range filters {
		if len(f) == 3 && f[0] == "username" {
			name, _ := f[2].(string)
			return name
		}
	}
	return ""
}

// A read resolves and never writes: it runs outside the subject lock and the lifecycle gate. A write
// operation records a legacy binding's real uid and marker once, logged.
func TestOnlyAWriteRewritesTheBinding(t *testing.T) {
	legacy := idCase{binding: Binding{Username: "bob", UID: 11},
		nas: `[{"username":"bob","id":11,"uid":3051,"locked":false,"smb":false,"groups":[]}]`}
	for _, read := range []string{"storage.status", "activity.get"} {
		s, _ := legacy.server(t)
		if rr := postOperation(t, s, read, `{"call_id":"c1","subject":"sub-1"}`); rr.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", read, rr.Code, rr.Body.String())
		}
		if b, _, _ := s.store.GetBinding("sub-1"); b.UID != 11 || b.UIDKind != "" {
			t.Errorf("%s rewrote the binding: %+v", read, b)
		}
		if logged := loggedWrites(t, s); len(logged) != 0 {
			t.Errorf("%s logged a write: %+v", read, logged)
		}
	}
	s, _ := legacy.server(t)
	if rr := postOperation(t, s, "password.rotate", `{"call_id":"c1","subject":"sub-1","actor":"op_1"}`); rr.Code != http.StatusOK {
		t.Fatalf("rotate: %d %s", rr.Code, rr.Body.String())
	}
	if b, _, _ := s.store.GetBinding("sub-1"); b.UID != 3051 || b.UIDKind != uidKindAccount {
		t.Errorf("a write records the real uid and marker, got %+v", b)
	}
	var binds int
	for _, r := range loggedWrites(t, s) {
		if r.Write == "store.bind" && r.Operation == "password.rotate" && r.AccountID == "3051" {
			binds++
		}
	}
	if binds != 2 { // intent and result
		t.Errorf("the rewrite is logged once under the operation, got %d records: %+v", binds, loggedWrites(t, s))
	}
}

// Naming an account needs only its uid and name: with the group table unreadable, every operation
// still names its account and answers (activity.get with its coverage caveat), and only the apply,
// which converges groups by name, reports the group read failing.
func TestOperationsNeedNoGroupTable(t *testing.T) {
	c := idCases[0] // in range, found: bob, record 11
	for _, op := range []struct{ name, body string }{
		{"password.set", `{"call_id":"c1","subject":"sub-1","actor":"op_1","params":{"password":"` + theSecret + `"}}`},
		{"password.rotate", `{"call_id":"c1","subject":"sub-1","actor":"op_1"}`},
		{"account.purge", `{"call_id":"c1","subject":"sub-1","actor":"op_1","params":{"elevated_key":"k"}}`},
		{"storage.status", `{"call_id":"c1","subject":"sub-1"}`},
		{"activity.get", `{"call_id":"c1","subject":"sub-1"}`},
	} {
		s, m := c.server(t)
		m.fakeRPC.refuse = map[string]string{"group.query": "permission denied"}
		m.fakeRPC.shares = `[{"name":"main","audit":{"enable":true,"watch_list":["staff"],"ignore_list":[]}}]`
		del := &deleteRecorder{}
		s.elevated = func(string) (rpc, error) { return del, nil }
		rr := postOperation(t, s, op.name, op.body)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s with no group table: %d %s", op.name, rr.Code, rr.Body.String())
		}
		if op.name == "activity.get" && !strings.Contains(rr.Body.String(), "coverage is unknown") {
			t.Errorf("activity.get: want the coverage caveat, got %s", rr.Body.String())
		}
	}
	s, m := c.server(t)
	m.fakeRPC.refuse = map[string]string{"group.query": "permission denied"}
	if rr := postApply(t, s, oneApply); rr.Code == http.StatusOK || len(m.updates)+len(m.creates) != 0 {
		t.Fatalf("the apply converges groups by name and says it could not read them: %d %s", rr.Code, rr.Body.String())
	}
}

// Two accounts on the uid a binding falls back to: the plan and the apply refuse it in the same
// words, and neither picks one.
func TestPlanAndApplyAgreeOnTwoAccountsOnOneUID(t *testing.T) {
	c := idCase{binding: Binding{Username: "bob", UID: 300001, UIDKind: uidKindAccount}, nas: `[
		{"username":"bob_a","id":11,"uid":300001,"locked":false,"smb":false,"groups":[]},
		{"username":"bob_b","id":12,"uid":300001,"locked":false,"smb":false,"groups":[]}]`}
	detail, next := ambiguityWords()

	s, _ := c.server(t)
	rr := httptest.NewRecorder()
	planInto(t, s, rr, `{"subjects":[{"subject":"sub-1","email":"bob@example.edu","desired":{"group":["lab_makers"],"enabled":true}}]}`)
	var plan PlanResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &plan); err != nil || len(plan.Outcomes) != 1 {
		t.Fatalf("plan: %v %s", err, rr.Body.String())
	}
	if out := plan.Outcomes[0]; out.Effect != EffectBlocked || out.Detail != detail || !strings.HasSuffix(out.Consequence, next) {
		t.Fatalf("plan: want blocked in the ambiguity's words, got %+v", out)
	}

	s, m := c.server(t)
	rr = postApply(t, s, oneApply)
	if rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), detail+" "+next) || len(m.updates)+len(m.creates) != 0 {
		t.Fatalf("apply: want 409 in the same words and nothing written, got %d %s", rr.Code, rr.Body.String())
	}

	// /subjects names neither row for the subject, whichever is listed first, and says so: an
	// ambiguous conflict listing both holders, in the same words. Silence would read as gone.
	rr, got := getSubjects(t, s, true)
	if rr.Code != http.StatusOK || len(got.Conflicts) != 1 {
		t.Fatalf("/subjects: %d, conflicts %+v", rr.Code, got.Conflicts)
	}
	cf := got.Conflicts[0]
	want := []Holder{{Username: "bob_a", AccountID: "300001"}, {Username: "bob_b", AccountID: "300001"}}
	if cf.Subject != "sub-1" || cf.Reason != reasonAmbiguous || !slices.Equal(cf.Holders, want) || cf.Username != "" ||
		cf.Adoptable || cf.Detail != detail || cf.Next != next {
		t.Errorf("/subjects: conflict %+v, want sub-1 ambiguous between %v in the ambiguity's words", *cf.BindingConflict, want)
	}
	for _, v := range got.Subjects {
		if v.Subject != "" {
			t.Errorf("/subjects: %s carries subject %q; two accounts on one key name neither", v.Username, v.Subject)
		}
	}
}

// Two bindings naming one account give it to neither: both are conflicts, claimed twice, each
// naming the other subject, and no write rule (stats, standing) picks a winner. The account stays
// bound for adoption, so no third subject can take it.
func TestTwoBindingsOnOneAccountGiveItToNeither(t *testing.T) {
	c := idCases[0] // in range, found: bob, uid 300001, bound to sub-1
	s, _ := c.server(t)
	if err := s.store.PutBinding(Binding{SubjectID: "sub-2", Username: "bob", UID: 300001, UIDKind: uidKindAccount}); err != nil {
		t.Fatal(err)
	}
	rr, got := getSubjects(t, s, true)
	if rr.Code != http.StatusOK || len(got.Subjects) != 1 || got.Subjects[0].Subject != "" {
		t.Fatalf("/subjects: %d %s; want bob listed, given to neither", rr.Code, rr.Body.String())
	}
	if len(got.Conflicts) != 2 {
		t.Fatalf("want both bindings as conflicts, got %+v", got.Conflicts)
	}
	for i, other := range []string{"sub-2", "sub-1"} {
		cf := got.Conflicts[i]
		if cf.Subject != []string{"sub-1", "sub-2"}[i] || cf.Reason != reasonClaimedTwice || cf.Username != "bob" ||
			cf.AccountID != "300001" || cf.BoundTo != other || cf.Adoptable || cf.Detail == "" || cf.Next == "" {
			t.Errorf("conflict %d: %+v", i, *cf.BindingConflict)
		}
	}
	snap, err := s.readSubjects()
	if err != nil {
		t.Fatal(err)
	}
	if u, _ := s.boundUIDs(snap.Subjects); len(u) != 0 {
		t.Errorf("statistics attribute bob to %v; claimed twice is nobody's", u)
	}
	if _, ok := s.wishesFor(snap.Subjects)[300001]; ok {
		t.Error("a wish applies to bob; claimed twice is nobody's")
	}
	if bound, _ := s.accountBound(&snap.Subjects[0]); !bound {
		t.Error("accountBound: bob is free, so a third subject could adopt it")
	}
}

// The name finds the account even where another row shares its uid, and only that row carries the
// subject: one subject on one row, never every row on the number.
func TestASharedUIDPutsTheSubjectOnTheNamedRowOnly(t *testing.T) {
	c := idCase{binding: Binding{Username: "bob_a", UID: 300001, UIDKind: uidKindAccount}, nas: `[
		{"username":"bob_a","id":11,"uid":300001,"locked":false,"smb":false,"groups":[]},
		{"username":"bob_b","id":12,"uid":300001,"locked":false,"smb":false,"groups":[]}]`}
	s, _ := c.server(t)
	rr, got := getSubjects(t, s, true)
	if rr.Code != http.StatusOK || len(got.Subjects) != 2 {
		t.Fatalf("/subjects: %d %s", rr.Code, rr.Body.String())
	}
	for _, v := range got.Subjects {
		if want := map[string]string{"bob_a": "sub-1"}[v.Username]; v.Subject != want {
			t.Errorf("%s carries subject %q, want %q", v.Username, v.Subject, want)
		}
	}
}

// A release forgets the binding, and with it the attribution: the account stays, bound to nobody,
// on the very next read (every write forgets the copy, so no earlier read's subject survives).
func TestAReleasedAccountCarriesNoSubject(t *testing.T) {
	c := idCases[0] // in range, found
	s, _ := c.server(t)
	if _, got := getSubjects(t, s, false); len(got.Subjects) != 1 || got.Subjects[0].Subject != "sub-1" {
		t.Fatalf("before the release: %+v", got.Subjects)
	}
	if rr := postOperation(t, s, "account.release", `{"call_id":"c-rel","subject":"sub-1"}`); rr.Code != http.StatusOK {
		t.Fatalf("release: %d %s", rr.Code, rr.Body.String())
	}
	_, got := getSubjects(t, s, false)
	if len(got.Subjects) != 1 || got.Subjects[0].Subject != "" || len(got.Conflicts) != 0 {
		t.Errorf("after the release: %+v, conflicts %+v; want bob listed, bound to nobody", got.Subjects, got.Conflicts)
	}
}

// The NAS answered and the store failed: the copy kept from earlier is not served labelled
// "unreachable", which would blame the NAS. The store's failure is the answer.
func TestAStoreFailureIsNeverServedAsAnUnreachableNAS(t *testing.T) {
	s, _ := idCases[0].server(t)
	now := at(s, time.Now())
	if rr, _ := getSubjects(t, s, true); rr.Code != http.StatusOK {
		t.Fatalf("first read: %d", rr.Code)
	}
	*now = now.Add(time.Hour) // past the cooldown and the copy's age
	corrupt(t, s, bucketBindings, "sub-2")
	rr, _ := getSubjects(t, s, true)
	if rr.Code != http.StatusInternalServerError || problemCode(t, rr) != CodeStoreUnreadable {
		t.Fatalf("want 500 %s, got %d %s", CodeStoreUnreadable, rr.Code, rr.Body.String())
	}
}

// Every reader of the binding table fails closed on a binding it cannot decode: its name is never
// read as free to claim (BindIfFree, BoundUsernames), nor its subject as bound to nothing.
func TestAnUndecodableBindingIsNeverReadAsFree(t *testing.T) {
	s, _ := idCases[0].server(t)
	corrupt(t, s, bucketBindings, "sub-2")
	if _, err := s.store.Bindings(); err == nil {
		t.Error("Bindings skipped it")
	}
	if _, err := s.store.BoundUsernames(); err == nil {
		t.Error("BoundUsernames skipped it, so its name reads as unclaimed")
	}
	err := s.store.BindIfFree(Binding{SubjectID: "sub-3", Username: "carol", UID: 300003, UIDKind: uidKindAccount})
	if err == nil || errors.As(err, new(ErrUsernameBound)) {
		t.Errorf("BindIfFree: %v; the name sub-2 holds is unknown, so no name is free", err)
	}
	if _, bound, _ := s.store.GetBinding("sub-3"); bound {
		t.Error("BindIfFree wrote the binding anyway")
	}
}

// A binding that cannot be read is no attribution at all: 500 STORE_UNREADABLE, never a read with
// that person's row unbound and no conflict, which says their account is gone. Another subject's
// unreadable binding counts the same: which row is whose cannot be said for the read.
func TestSubjectsWithAnUnreadableBindingAreRefused(t *testing.T) {
	s, _ := idCases[0].server(t)
	corrupt(t, s, bucketBindings, "sub-2")
	rr, _ := getSubjects(t, s, true)
	if rr.Code != http.StatusInternalServerError || problemCode(t, rr) != CodeStoreUnreadable {
		t.Fatalf("want 500 %s, got %d %s", CodeStoreUnreadable, rr.Code, rr.Body.String())
	}
}

// The apply says what its lookup's failure was: a rate-limited TrueNAS is a 429, which Syndra
// retries later, never a 502 it parks as possibly acted.
func TestARateLimitedLookupIsA429OnTheApply(t *testing.T) {
	s, m := idCases[0].server(t)
	m.rateLimitOn = "user.query"
	// A fingerprint of its own, so the first user.query is the apply's (postApply would read first).
	rr := postApply(t, s, `{"call_id":"c1","subject":"sub-1","email":"bob@example.edu","fingerprint":"f","desired":{"enabled":true}}`)
	if rr.Code != http.StatusTooManyRequests || problemCode(t, rr) != CodeApplyFailed || len(m.updates)+len(m.creates) != 0 {
		t.Fatalf("want 429 APPLY_FAILED and nothing written, got %d %s", rr.Code, rr.Body.String())
	}
}

// Adoption answers an unreadable binding as every operation does: 500 STORE_UNREADABLE, nothing sent.
func TestAdoptAnswersAnUnreadableBindingAsEveryOperationDoes(t *testing.T) {
	s, m := idCases[0].server(t)
	corrupt(t, s, bucketBindings, "sub-1")
	before := len(m.fakeRPC.calls)
	rr := postOperation(t, s, "account.adopt", `{"call_id":"a1","subject":"sub-1","actor":"op_1","params":{"username":"bob"}}`)
	if rr.Code != http.StatusInternalServerError || problemCode(t, rr) != CodeStoreUnreadable {
		t.Fatalf("want 500 %s, got %d %s", CodeStoreUnreadable, rr.Code, rr.Body.String())
	}
	if sent := m.fakeRPC.calls[before:]; len(sent) != 0 {
		t.Errorf("nothing is asked of the NAS: %v", sent)
	}
}

// A contract-1 apply reply replayed takes its standing from the account the subject's binding names
// in the mirror, never from whoever holds that uid now.
func TestALegacyReplyNeverShowsTheHolderOfAReusedNumber(t *testing.T) {
	c := idCases[5] // out of range, bob's number held by eve
	s, m := c.server(t)
	snap, err := s.readSubjects()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.store.PutSnapshot(snap); err != nil {
		t.Fatal(err)
	}
	entry := `{"stored_at":"` + time.Now().UTC().Format(time.RFC3339Nano) + `","kind":"` + kindApply +
		`","result":{"subject":"sub-1","effect":"applied","detail":"Done.","username":"bob","uid":3051,"fingerprint":"f1"}}`
	if err := s.store.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketIdempotency).Put([]byte("v1-apply"), []byte(entry))
	}); err != nil {
		t.Fatal(err)
	}
	rr := postApply(t, s, `{"call_id":"v1-apply","subject":"sub-1","fingerprint":"anything","desired":{"enabled":true}}`)
	out := decodeOutcome(t, rr)
	if rr.Code != http.StatusOK || out.AccountID != "" || out.Standing != nil || len(m.updates) != 0 {
		t.Fatalf("the replay must not carry eve's number or standing: %d %s", rr.Code, rr.Body.String())
	}
}

// storage.status reads the places of the account the binding names now: an in-range account
// renamed since it was bound shows its own groups' folders and its home.
func TestStorageStatusReadsTheResolvedAccountsPlaces(t *testing.T) {
	fake := reachFake()
	fake.users = `[{"username":"charlotte","id":11,"uid":200001,"locked":false,"smb":true,"password_disabled":false,
		"groups":[41,50],"home":"/mnt/tank/main/home_dir/charlotte"}]`
	fake.answers["pool.dataset.get_quota"] = `[]`
	s := testServer(t, fake)
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "charlotte_old", UID: 200001, UIDKind: uidKindAccount}); err != nil {
		t.Fatal(err)
	}
	res, status, err := s.storageStatus(OperationRequest{Subject: "sub-1"})
	if err != nil || status != 0 && status != http.StatusOK {
		t.Fatalf("%d %v", status, err)
	}
	got := res.Result.(*StorageStatus)
	if !got.PlacesReadable || describe(got.Places) != "main/home_dir/charlotte:change,main:read,main/waddle:change,open:read" {
		t.Fatalf("places = %q (readable %t)", describe(got.Places), got.PlacesReadable)
	}
	if !got.Usable || got.NeedsPassword {
		t.Errorf("the account's own fields: %+v", got)
	}
}
