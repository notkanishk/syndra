package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
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
	// Legacy policy B (design 7t D6, pending owner Q2): halts, creates nothing.
	{"legacy out of range, number held by another name", Binding{Username: "bob", UID: 3051},
		`[{"username":"eve","id":12,"uid":3051,"locked":false,"smb":false,"groups":[]}]`, "", true},
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
						!strings.Contains(rr.Body.String(), "Release this subject") {
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
				case c.want == "":
					if rr.Code != http.StatusUnprocessableEntity || !strings.Contains(rr.Body.String(), "no longer exists") {
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
						if !strings.Contains(rr.Body.String(), `"standing"`) || len(writes) != 0 {
							t.Errorf("storage.status: %s, writes %v", rr.Body.String(), writes)
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
