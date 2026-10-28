package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// What the add-on does not record, it must not pretend it did: a refusal is not
// remembered as a result, a name is bound once, a torn log line is not carried
// forward, and an unknown is not read as a no.

const applyBody = `{"call_id":"c1","subject":"sub-1","email":"ada@example.edu",
	"desired":{"group":["lab_makers"],"enabled":true}}`

// A blocked outcome changed nothing, so a replay re-runs it. Remembered, it
// came back as a 200 — which the backend reads as applied.
func TestAReplayedRefusalIsRunAgainNotRememberedAsAnApply(t *testing.T) {
	s, m := applyServer(t, `[{"username":"ada","id":19,"uid":9001,"locked":false,"smb":false,"groups":[]}]`)

	first := postApply(t, s, applyBody)
	if first.Code != http.StatusConflict {
		t.Fatalf("want the first call blocked with 409, got %d", first.Code)
	}
	if _, found, _ := s.store.Recall("c1", kindApply); found {
		t.Fatal("a refusal must not be remembered")
	}
	// The target moves on: the account is gone, so the same call now creates.
	m.fakeRPC.users = `[]`
	second := postApply(t, s, applyBody)
	if second.Code != http.StatusOK || decodeOutcome(t, second).Effect != EffectApplied {
		t.Fatalf("a replay must re-run, got %d (%s)", second.Code, second.Body.String())
	}
	// And an applied one IS remembered: a replay returns it without a second create.
	postApply(t, s, applyBody)
	if len(m.creates) != 1 {
		t.Fatalf("a replay of an applied call must not create again, got %d creates", len(m.creates))
	}
}

func TestOneUsernameBindsToOneSubject(t *testing.T) {
	s, _ := applyServer(t, `[]`)
	if err := s.store.BindIfFree(Binding{SubjectID: "sub-1", Username: "ada", UID: 1}); err != nil {
		t.Fatal(err)
	}
	// The same subject again is fine; another subject is refused.
	if err := s.store.BindIfFree(Binding{SubjectID: "sub-1", Username: "ada", UID: 1}); err != nil {
		t.Fatalf("rebinding the same subject: %v", err)
	}
	err := s.store.BindIfFree(Binding{SubjectID: "sub-2", Username: "ada", UID: 1})
	var taken ErrUsernameBound
	if !errors.As(err, &taken) || taken.Owner != "sub-1" {
		t.Fatalf("want ErrUsernameBound naming sub-1, got %v", err)
	}
	if _, bound, _ := s.store.GetBinding("sub-2"); bound {
		t.Fatal("the refused binding must not be written")
	}
}

// A crash mid-line, then a restart and a write: the next record must start on
// its own line, so the chain still verifies.
func TestATornLogTailIsCutBeforeTheNextAppend(t *testing.T) {
	l, path := openLog(t)
	if _, err := l.Append("password.set", "u1", "op_1", "c1", "succeeded"); err != nil {
		t.Fatal(err)
	}
	l.Close()
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(`{"seq":2,"at":"2026-`)
	f.Close()

	reopened, err := OpenMutationLog(filepath.Dir(path), 1<<20, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if _, err := reopened.Append("password.set", "u1", "op_1", "c2", "succeeded"); err != nil {
		t.Fatal(err)
	}
	if err := VerifyLog(filepath.Dir(path)); err != nil {
		t.Fatalf("the log must verify after a torn tail and a new append: %v", err)
	}
}

// A failed write leaves nothing behind: the next record takes the next seq on a
// clean line instead of repeating one or following a partial.
func TestAFailedAppendIsCutBackSoTheLogStillVerifies(t *testing.T) {
	l, path := openLog(t)
	if _, err := l.Append("password.set", "u1", "op_1", "c1", "succeeded"); err != nil {
		t.Fatal(err)
	}
	l.sync = func() error { return errors.New("disk gone") }
	if _, err := l.Append("password.set", "u1", "op_1", "c2", "succeeded"); err == nil {
		t.Fatal("want the failed fsync reported")
	}
	l.sync = l.file.Sync
	if _, err := l.Append("password.set", "u1", "op_1", "c3", "succeeded"); err != nil {
		t.Fatal(err)
	}
	if err := VerifyLog(filepath.Dir(path)); err != nil {
		t.Fatalf("a record that failed to sync must not stay in the file: %v", err)
	}
}

// A purge that cannot tell whether it is deleting its own credential waits.
func TestAPurgeRefusesWhileTheAddonDoesNotKnowItsOwnAccount(t *testing.T) {
	s, _ := opServer(t)
	s.nas.selfName = ""
	called := false
	s.elevated = func(string) (rpc, error) {
		called = true
		return &recordingRPC{onCall: func(string) {}, onClose: func() {}}, nil
	}
	rr := postOperation(t, s, "account.purge",
		`{"call_id":"c1","subject":"sub-1","actor":"op_1","params":{"elevated_key":"k"}}`)
	if rr.Code != http.StatusUnprocessableEntity || called {
		t.Fatalf("want 422 and no deletion, got %d elevated=%t", rr.Code, called)
	}
}

// One failed `auth.me` must not blind the guard until a reconnect.
func TestAnUnknownOwnAccountIsAskedForAgain(t *testing.T) {
	s, m := applyServer(t, `[]`)
	m.fakeRPC.answers = map[string]string{"auth.me": `{"pw_name":"syndra","pw_uid":3001}`}
	s.nas.selfName = ""

	var rows []json.RawMessage
	for i := 0; i < 2 && func() bool { _, _, ok := s.nas.Self(); return !ok }(); i++ {
		_ = s.nas.call("user.query", []any{[]any{}, map[string]any{}}, &rows)
	}
	if name, uid, ok := s.nas.Self(); !ok || name != "syndra" || uid != 3001 {
		t.Fatalf("the own account must be read on a later call, got %q %d %t", name, uid, ok)
	}
}

// A group left empty by an earlier attempt, inside this add-on's id range, is
// taken over and its number becomes the account's. One with members, or one
// outside the range (somebody else's, however empty), stays refused.
func TestACreateAdoptsAnEmptyOrphanGroupAndRefusesOccupiedOnes(t *testing.T) {
	s, m := applyServer(t, `[]`)
	m.refuseGroupCreate = true
	m.fakeRPC.groups = `[{"id":42,"gid":900,"group":"lab_makers"},{"id":93,"gid":250000,"group":"ada","builtin":false,"users":[]}]`
	rr := postApply(t, s, applyBody)
	if rr.Code != http.StatusOK || len(m.creates) != 1 || fmt.Sprint(m.creates[0]["group"]) != "93" {
		t.Fatalf("want the empty orphan adopted as the primary group, got %d creates=%v (%s)",
			rr.Code, m.creates, rr.Body.String())
	}
	if fmt.Sprint(m.creates[0]["uid"]) != "250000" {
		t.Errorf("the account must take the adopted group's number, got uid %v", m.creates[0]["uid"])
	}

	for name, groups := range map[string]string{
		"occupied":     `[{"id":93,"gid":250000,"group":"ada","builtin":false,"users":[7]}]`,
		"out of range": `[{"id":93,"gid":5000,"group":"ada","builtin":false,"users":[]}]`,
	} {
		s2, m2 := applyServer(t, `[]`)
		m2.refuseGroupCreate = true
		m2.fakeRPC.groups = groups
		rr = postApply(t, s2, applyBody)
		if rr.Code == http.StatusOK || len(m2.creates) != 0 {
			t.Fatalf("%s group must not be taken, got %d creates=%d", name, rr.Code, len(m2.creates))
		}
	}
}

// An adopted group predates this attempt, so a refused account never deletes it.
func TestARefusedCreateLeavesAnAdoptedGroupAlone(t *testing.T) {
	s, m := applyServer(t, `[]`)
	m.refuseGroupCreate = true
	m.refuseUserCreate = true
	m.fakeRPC.groups = `[{"id":93,"gid":250000,"group":"ada","builtin":false,"users":[]}]`
	if rr := postApply(t, s, applyBody); rr.Code == http.StatusOK {
		t.Fatalf("a refused account must not read as success: %s", rr.Body.String())
	}
	if m.groupDeletes != 0 {
		t.Fatal("an adopted group was deleted")
	}
}

// The account exists even when it cannot be read back, so it is bound anyway.
func TestACreateThatCannotBeReadBackStillRecordsTheBinding(t *testing.T) {
	s, m := applyServer(t, `[]`)
	m.failReadAfterCreate = true
	rr := postApply(t, s, applyBody)
	if rr.Code == http.StatusOK {
		t.Fatalf("an unverified create must not read as a clean success: %s", rr.Body.String())
	}
	b, bound, err := s.store.GetBinding("sub-1")
	if err != nil || !bound || b.Username != "ada" || b.UID == 0 {
		t.Fatalf("the created account must be bound: %+v bound=%t err=%v", b, bound, err)
	}
}

// 200 are returned, and the 201st row is what says there were more.
func TestActivityBeyondTheLimitSaysItWasTruncated(t *testing.T) {
	rows := func(n int) string {
		parts := make([]string, n)
		for i := range parts {
			parts[i] = fmt.Sprintf(`{"message_timestamp":%d,"event":"AUTHENTICATION","success":true}`, 1700000000+i)
		}
		return "[" + strings.Join(parts, ",") + "]"
	}
	for _, tc := range []struct {
		n         int
		truncated bool
		returned  int
	}{{200, false, 200}, {201, true, 200}} {
		s, m := opServer(t)
		m.fakeRPC.audit, m.fakeRPC.shares = rows(tc.n), `[]`
		rr := postOperation(t, s, "activity.get", `{"call_id":"c1","subject":"sub-1"}`)
		var out OperationResult
		if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil || out.Activity == nil {
			t.Fatalf("%d rows: %v (%s)", tc.n, err, rr.Body.String())
		}
		if out.Activity.Truncated != tc.truncated || len(out.Activity.Events) != tc.returned {
			t.Errorf("%d rows: truncated=%t returned=%d", tc.n, out.Activity.Truncated, len(out.Activity.Events))
		}
	}
}

// Shutdown holds the NAS session until the last mutation settles.
func TestShutdownWaitsForTheDrainAndGivesUpAtTheDeadline(t *testing.T) {
	life := newLifecycle(LifecycleActive)
	done, err := life.Begin()
	if err != nil {
		t.Fatal(err)
	}
	_ = life.Set(LifecycleDraining, "shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if waitDrained(ctx, life) {
		t.Fatal("a mutation still in flight at the deadline is not a drain")
	}

	settled := make(chan bool, 1)
	go func() { settled <- waitDrained(context.Background(), life) }()
	time.Sleep(100 * time.Millisecond)
	done()
	select {
	case ok := <-settled:
		if !ok {
			t.Fatal("want drained once the mutation finished")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("waitDrained never returned")
	}
}
