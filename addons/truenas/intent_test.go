package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The mutation log is written ahead of every write (5.3b): an intent, fsynced,
// before the NAS sees it, and a result after.

// records reads the log as it is on disk, every line.
func records(t *testing.T, s *server) []Record {
	t.Helper()
	raw, err := os.ReadFile(mutationLogPath(t, s))
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
			t.Fatalf("a log line does not decode: %v", err)
		}
		out = append(out, r)
	}
	return out
}

// intentWatcher is a NAS that, as each write arrives, reads the log from disk
// and requires that write's intent to be the last record there.
type intentWatcher struct {
	rpc
	t      *testing.T
	s      *server
	writes []string
}

func (w *intentWatcher) Call(method string, timeout int64, params any) (json.RawMessage, error) {
	if !readMethods[method] && method != "core.ping" {
		w.writes = append(w.writes, method)
		all := records(w.t, w.s)
		if len(all) == 0 {
			w.t.Errorf("%s reached the NAS with nothing in the log", method)
		} else if last := all[len(all)-1]; last.Outcome != outcomeIntent || last.Write != method {
			w.t.Errorf("%s reached the NAS and the last record on disk is %+v, not its intent", method, last)
		}
	}
	return w.rpc.Call(method, timeout, params)
}

// watched puts the watcher between the add-on and the fake NAS.
func watched(t *testing.T, s *server, inner rpc) *intentWatcher {
	w := &intentWatcher{rpc: inner, t: t, s: s}
	s.nas = newNAS(func() (rpc, error) { return w, nil }, []string{"25.04"})
	s.nas.version, s.nas.probed = "25.04.2", true
	s.nas.selfName, s.nas.selfUID = "syndra", 3001
	return w
}

// Every write site, each reached the way a caller reaches it: the create
// (group, account, and the group undone when the account is refused), an
// update, a limit, both credential writes and the purge on its own session.
func TestEveryWriteIsAnIntentOnDiskBeforeTheNASSeesIt(t *testing.T) {
	cases := []struct {
		name string
		run  func(t *testing.T) (*intentWatcher, *server)
		want []string
	}{
		{"create", func(t *testing.T) (*intentWatcher, *server) {
			s, m := applyServer(t, `[]`)
			w := watched(t, s, m)
			postApply(t, s, `{"call_id":"c1","subject":"sub-9","email":"zoe@example.org","desired":{"group":["lab_makers"],"enabled":true}}`)
			return w, s
		}, []string{"group.create", "user.create"}},
		{"refused create", func(t *testing.T) (*intentWatcher, *server) {
			s, m := applyServer(t, `[]`)
			m.refuseUserCreate = true
			w := watched(t, s, m)
			postApply(t, s, `{"call_id":"c1","subject":"sub-9","email":"zoe@example.org","desired":{"group":["lab_makers"],"enabled":true}}`)
			return w, s
		}, []string{"group.create", "user.create", "group.delete"}},
		{"update", func(t *testing.T) (*intentWatcher, *server) {
			s, m := opServer(t)
			w := watched(t, s, m)
			postApply(t, s, `{"call_id":"c1","subject":"sub-1","email":"charlotte@example.org","desired":{"enabled":false}}`)
			return w, s
		}, []string{"user.update"}},
		{"limit", func(t *testing.T) (*intentWatcher, *server) {
			s, m := homeServer(t, `[{"username":"charlotte","id":11,"uid":3001,"locked":false,"smb":true,"groups":[42],"home":"`+homePlace+`/charlotte"}]`,
				map[string]string{"pool.dataset.get_quota": `[]`}, nil)
			w := watched(t, s, m)
			applyHome(t, s, `{"home":["true"],"home_quota":["10737418240"]}`)
			return w, s
		}, []string{"pool.dataset.set_quota"}},
		{"password.set", func(t *testing.T) (*intentWatcher, *server) {
			s, m := opServer(t)
			w := watched(t, s, m)
			postOperation(t, s, "password.set", `{"call_id":"c1","subject":"sub-1","params":{"password":"`+theSecret+`"}}`)
			return w, s
		}, []string{"user.update"}},
		{"password.rotate", func(t *testing.T) (*intentWatcher, *server) {
			s, m := opServer(t)
			w := watched(t, s, m)
			postOperation(t, s, "password.rotate", `{"call_id":"c1","subject":"sub-1"}`)
			return w, s
		}, []string{"user.update"}},
		{"account.purge", func(t *testing.T) (*intentWatcher, *server) {
			s, _ := opServer(t)
			w := &intentWatcher{rpc: &recordingRPC{onCall: func(string) {}, onClose: func() {}}, t: t, s: s}
			s.elevated = func(string) (rpc, error) { return w, nil }
			postOperation(t, s, "account.purge", `{"call_id":"c1","subject":"sub-1","params":{"elevated_key":"k"}}`)
			return w, s
		}, []string{"user.delete"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, s := tc.run(t)
			if strings.Join(w.writes, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("want writes %v, the NAS saw %v", tc.want, w.writes)
			}
			// And each intent was settled by a result naming it.
			intents := map[uint64]string{}
			for _, r := range records(t, s) {
				if r.Outcome == outcomeIntent {
					intents[r.Seq] = r.Write
				} else if r.Intent != 0 {
					if intents[r.Intent] != r.Write {
						t.Errorf("result %d settles intent %d, which is not its write %s", r.Seq, r.Intent, r.Write)
					}
					delete(intents, r.Intent)
				}
			}
			if len(intents) != 0 {
				t.Errorf("intents with no result: %v", intents)
			}
		})
	}
}

// An intent whose fsync fails is not an intent: nothing is sent, nothing is
// stored, nothing is recorded, and the answer is a 500 a person sees.
func TestAnIntentThatCannotBeSyncedSendsNothing(t *testing.T) {
	failSync := func(s *server) { s.log.sync = func() error { return errors.New("disk gone") } }

	t.Run("apply", func(t *testing.T) {
		s, m := opServer(t)
		w := watched(t, s, m)
		failSync(s)
		rr := postApply(t, s, `{"call_id":"c1","subject":"sub-1","email":"charlotte@example.org","desired":{"enabled":false}}`)
		if rr.Code != http.StatusInternalServerError || problemCode(t, rr) != CodeApplyFailed {
			t.Fatalf("want 500 APPLY_FAILED, got %d %s", rr.Code, rr.Body.String())
		}
		if len(w.writes) != 0 || len(m.updates) != 0 {
			t.Fatalf("a write with no durable intent reached the NAS: %v", w.writes)
		}
		if _, count := s.log.Head(); count != 0 {
			t.Fatalf("the log moved: %d records", count)
		}
	})
	t.Run("operation", func(t *testing.T) {
		s, m := opServer(t)
		w := watched(t, s, m)
		failSync(s)
		rr := postOperation(t, s, "password.set", `{"call_id":"c1","subject":"sub-1","params":{"password":"`+theSecret+`"}}`)
		if rr.Code != http.StatusInternalServerError || problemCode(t, rr) != CodeOperationFailed {
			t.Fatalf("want 500 OPERATION_FAILED, got %d %s", rr.Code, rr.Body.String())
		}
		if len(w.writes) != 0 {
			t.Fatalf("a write with no durable intent reached the NAS: %v", w.writes)
		}
	})
	t.Run("store write", func(t *testing.T) {
		s, _ := opServer(t)
		failSync(s)
		rr := postOperation(t, s, "account.release", `{"call_id":"c1","subject":"sub-1"}`)
		if rr.Code != http.StatusInternalServerError || problemCode(t, rr) != CodeOperationFailed {
			t.Fatalf("want 500 OPERATION_FAILED, got %d %s", rr.Code, rr.Body.String())
		}
		if _, bound, _ := s.store.GetBinding("sub-1"); !bound {
			t.Fatal("the binding was released with no durable intent")
		}
	})
	t.Run("setting", func(t *testing.T) {
		s := testServer(t, &fakeRPC{})
		failSync(s)
		rr := putHomeRoot(t, s, "")
		if rr.Code != http.StatusInternalServerError || problemCode(t, rr) != CodeSettingNotSaved {
			t.Fatalf("want 500 SETTING_NOT_SAVED, got %d %s", rr.Code, rr.Body.String())
		}
		if _, found, _ := s.store.GetSetting(settingHomeRoot); found {
			t.Fatal("the setting was stored with no durable intent")
		}
	})
}

// The result says what became of the write, and only what is known: failed
// only when nothing changed, unknown when it may have.
func TestEveryWriteHasAResultThatSaysWhatBecameOfIt(t *testing.T) {
	s := testServer(t, &fakeRPC{})
	who := caller{"apply", "sub-1", "op_lita", "c1"}
	for _, tc := range []struct {
		write string
		err   error
		want  string
	}{
		{"user.update", nil, outcomeSucceeded},
		{"user.update", fmt.Errorf("user.update: %w", ErrTargetRefused), outcomeFailed},
		{"user.update", fmt.Errorf("user.update: %w", ErrRateLimited), outcomeFailed},
		{"user.update", fmt.Errorf("%w: dial", ErrNotSent), outcomeNotSent},
		{"user.update", fmt.Errorf("user.update: %w", ErrTargetUnreachable), outcomeUnknown},
		{"user.create", errors.New("user.create: decode result: bad"), outcomeUnknown},
		{"store.bind", errors.New("bolt: tx failed"), outcomeFailed},
	} {
		ran := false
		err := s.mutate(who, tc.write, func() error {
			all := records(t, s)
			if last := all[len(all)-1]; last.Outcome != outcomeIntent {
				t.Errorf("%s ran before its intent was recorded", tc.write)
			}
			ran = true
			return tc.err
		})
		if !ran || !errors.Is(err, tc.err) {
			t.Fatalf("%s: the write must run and its error come back, ran=%t err=%v", tc.write, ran, err)
		}
		all := records(t, s)
		intent, result := all[len(all)-2], all[len(all)-1]
		if result.Intent != intent.Seq || result.Write != tc.write || result.Outcome != tc.want {
			t.Errorf("%v: want a result %q settling intent %d, got %+v", tc.err, tc.want, intent.Seq, result)
		}
	}
	if err := VerifyLog(s.log.dir); err != nil {
		t.Fatal(err)
	}
}

// Production holds a log written before intents, anchored by Syndra. The new
// records append to that chain: no old byte rewritten, the old records still
// verify, and the head moves forward (backend ClassifyLogHead: more records
// and another head is "extended", anything else is tampering).
func TestALogWrittenBeforeIntentsCarriesOnUnderThem(t *testing.T) {
	before, err := os.ReadFile(filepath.Join("testdata", "mutations-before-intents.log"))
	if err != nil {
		t.Fatal(err)
	}
	s, m := opServer(t)
	dir := s.log.dir
	s.log.Close()
	if err := os.WriteFile(filepath.Join(dir, logFileName), before, 0o600); err != nil {
		t.Fatal(err)
	}
	if s.log, err = OpenMutationLog(dir, 1<<20, 4); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.log.Close() })
	oldHead, oldCount := s.log.Head()
	if oldCount != 3 || oldHead != "e212aa98f0168026e307ab98d82ce8b2479c88b950b7b2864785e01c8ca3c1d4" {
		t.Fatalf("the old log's head must be recovered as Syndra anchored it, got %s %d", oldHead, oldCount)
	}

	watched(t, s, m)
	if rr := postOperation(t, s, "password.set", `{"call_id":"c9","subject":"sub-1","params":{"password":"`+theSecret+`"}}`); rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d %s", rr.Code, rr.Body.String())
	}

	after, err := os.ReadFile(filepath.Join(dir, logFileName))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(after), string(before)) {
		t.Fatal("an old record was rewritten")
	}
	if err := VerifyLog(dir); err != nil {
		t.Fatalf("the chain must verify across the upgrade: %v", err)
	}
	head, count := s.log.Head()
	if count != oldCount+2 || head == oldHead {
		t.Fatalf("want the count grown by an intent and a result and the head moved, got %d %s (was %d %s)", count, head, oldCount, oldHead)
	}
	all := records(t, s)
	if all[3].Prev != oldHead || all[3].Outcome != outcomeIntent || all[4].Intent != all[3].Seq {
		t.Fatalf("the intent must chain onto the old head and its result settle it: %+v %+v", all[3], all[4])
	}
	// The old records decode with the new fields absent, and digest as before.
	for _, r := range all[:3] {
		if r.Write != "" || r.Intent != 0 || r.digestOf() != r.Digest {
			t.Fatalf("an old record reads differently under the new code: %+v", r)
		}
	}
}

// No secret, in any record: the password set, the one minted, the elevated key.
func TestNoSecretIsInAnyIntentOrResult(t *testing.T) {
	s, m := opServer(t)
	s.elevated = func(string) (rpc, error) { return &recordingRPC{onCall: func(string) {}, onClose: func() {}}, nil }
	postOperation(t, s, "password.set", `{"call_id":"c1","subject":"sub-1","params":{"password":"`+theSecret+`"}}`)
	postOperation(t, s, "password.rotate", `{"call_id":"c2","subject":"sub-1"}`)
	minted, _ := m.updates[len(m.updates)-1]["password"].(string)
	const elevated = "elevated-delete-key-canary"
	postOperation(t, s, "account.purge", `{"call_id":"c3","subject":"sub-1","params":{"elevated_key":"`+elevated+`"}}`)

	all := records(t, s)
	if len(all) != 6 || minted == "" {
		t.Fatalf("want an intent and a result for each of three writes, got %d records (minted %t)", len(all), minted != "")
	}
	raw, _ := os.ReadFile(mutationLogPath(t, s))
	for _, secret := range []string{theSecret, minted, elevated} {
		if strings.Contains(string(raw), secret) {
			t.Errorf("a record carries a secret:\n%s", raw)
		}
	}
}

// The NAS session sends no write of its own accord: call serves reads, and a
// write method given to it is refused before anything is sent.
func TestTheReadPathRefusesAWrite(t *testing.T) {
	s, m := opServer(t)
	w := watched(t, s, m)
	if err := s.nas.call("user.update", []any{11, map[string]any{"locked": true}}, nil); err == nil {
		t.Fatal("call must refuse a write")
	}
	if len(w.writes) != 0 || len(m.updates) != 0 {
		t.Fatalf("the refused write reached the NAS: %v", w.writes)
	}
}

// The seam cannot be walked around. Every route to the wire is named here
// with the one function allowed to take it: a NAS write only from send, the
// purge's own session only inside a mutate, and the raw client only from the
// three places that already hold to the rule (invoke under call and sendWrite,
// the ping, and callOnce).
func TestEveryNASWriteGoesThroughTheSeam(t *testing.T) {
	allowed := map[string][]string{
		"sendWrite": {"send"},
		"roundTrip": {"call", "sendWrite"},
		"invoke":    {"roundTrip"},
		"Call":      {"invoke", "provenLocked", "callOnce"},
		"callOnce":  {"purgeAccount"},
	}
	fset := token.NewFileSet()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			// The function literals passed to mutate: the only place callOnce
			// may run.
			inMutate := map[*ast.FuncLit]bool{}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok && calleeName(call) == "mutate" {
					for _, arg := range call.Args {
						if lit, ok := arg.(*ast.FuncLit); ok {
							inMutate[lit] = true
						}
					}
				}
				return true
			})
			var stack []ast.Node
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if n == nil {
					stack = stack[:len(stack)-1]
					return false
				}
				stack = append(stack, n)
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				callee := calleeName(call)
				places, guarded := allowed[callee]
				if !guarded {
					return true
				}
				if !slices.Contains(places, fn.Name.Name) {
					t.Errorf("%s: %s calls %s; only %v may", fset.Position(call.Pos()), fn.Name.Name, callee, places)
				}
				if callee == "callOnce" && !enclosedBy(stack, inMutate) {
					t.Errorf("%s: callOnce outside a mutate sends a write with no intent", fset.Position(call.Pos()))
				}
				return true
			})
		}
	}
}

func calleeName(call *ast.CallExpr) string {
	switch f := call.Fun.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		return f.Sel.Name
	}
	return ""
}

func enclosedBy(stack []ast.Node, lits map[*ast.FuncLit]bool) bool {
	for _, n := range stack {
		if lit, ok := n.(*ast.FuncLit); ok && lits[lit] {
			return true
		}
	}
	return false
}

// An intent with no result is in doubt; one with a result is not, wherever a
// rotation put the two.
func TestAnIntentWithNoResultIsInDoubt(t *testing.T) {
	dir := t.TempDir()
	l, err := OpenMutationLog(dir, 1, 4) // rotates before every append
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	settled, _ := l.Append(Record{Operation: "apply", Subject: "sub-rey", CallID: "c1", Write: "user.update", Outcome: outcomeIntent})
	if _, err := l.Append(Record{Operation: "apply", Subject: "sub-rey", CallID: "c1", Write: "user.update", Intent: settled.Seq, Outcome: outcomeSucceeded}); err != nil {
		t.Fatal(err)
	}
	open, _ := l.Append(Record{Operation: "apply", Subject: "sub-becky", CallID: "c2", Write: "user.create", Outcome: outcomeIntent})

	reopened, err := OpenMutationLog(dir, 1, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	doubt, err := reopened.InDoubt()
	if err != nil {
		t.Fatal(err)
	}
	if len(doubt) != 1 || doubt[0].Seq != open.Seq || doubt[0].Write != "user.create" {
		t.Fatalf("want only the unsettled create in doubt, got %+v", doubt)
	}
}
