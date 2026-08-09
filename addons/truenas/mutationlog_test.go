package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// 4.13–4.15 — the durability contract.
//
// An append-only file with no stated guarantees is not a forensic record. Each
// case here is one clause of the contract, and the point of the chain is that
// altering or removing a record breaks it.

func openLog(t *testing.T) (*MutationLog, string) {
	t.Helper()
	dir := t.TempDir()
	l, err := OpenMutationLog(dir, 1<<20, 4)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	return l, filepath.Join(dir, logFileName)
}

func TestARecordIsDurableBeforeCompletionIsReported(t *testing.T) {
	l, path := openLog(t)

	if _, err := l.Append("password.set", "u1", "op_1", "c1", "succeeded"); err != nil {
		t.Fatal(err)
	}
	// Read from disk, not from the writer: the contract is about what survives
	// the process, and asserting against in-memory state would pass for a log
	// that was never flushed.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"call_id":"c1"`) {
		t.Fatal("the record must be on disk by the time Append returns")
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// The log names who did what to whom, on a volume shared with whatever else
	// the container mounts.
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("the log must be 0600, got %o", perm)
	}
}

// The whole point of the chain: an entry cannot be altered, and no entry can be
// removed from the middle, without breaking it.
func TestAlteringOrRemovingARecordBreaksTheChain(t *testing.T) {
	l, path := openLog(t)
	for _, id := range []string{"c1", "c2", "c3"} {
		if _, err := l.Append("password.set", "u1", "op_1", id, "succeeded"); err != nil {
			t.Fatal(err)
		}
	}
	if err := VerifyChain(path); err != nil {
		t.Fatalf("an untouched log must verify: %v", err)
	}

	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(original)), "\n")

	t.Run("a field edited in place", func(t *testing.T) {
		edited := append([]string(nil), lines...)
		edited[1] = strings.Replace(edited[1], `"subject":"u1"`, `"subject":"u9"`, 1)
		writeLines(t, path, edited)
		if err := VerifyChain(path); err == nil || !strings.Contains(err.Error(), "altered") {
			t.Fatalf("want an alteration report, got %v", err)
		}
	})

	t.Run("a record removed from the middle", func(t *testing.T) {
		writeLines(t, path, []string{lines[0], lines[2]})
		if err := VerifyChain(path); err == nil || !strings.Contains(err.Error(), "missing") {
			t.Fatalf("want a missing-record report, got %v", err)
		}
	})

	t.Run("a record re-signed after being edited", func(t *testing.T) {
		// The digest is recomputed honestly, so only the chain link fails —
		// which is exactly what the `Prev` field is for.
		var r Record
		if err := json.Unmarshal([]byte(lines[1]), &r); err != nil {
			t.Fatal(err)
		}
		r.Subject = "u9"
		r.Digest = r.digestOf()
		reforged, _ := json.Marshal(r)
		writeLines(t, path, []string{lines[0], string(reforged), lines[2]})
		if err := VerifyChain(path); err == nil {
			t.Fatal("re-signing an edited record must still break the chain")
		}
	})
}

// Tail truncation is the one attack the chain cannot see — delete the last N
// records and what remains verifies perfectly. The head digest and the record
// count are what Syndra anchors, somewhere the add-on cannot rewrite.
func TestTheHeadAndCountAreWhatDetectTruncation(t *testing.T) {
	l, path := openLog(t)
	for _, id := range []string{"c1", "c2", "c3"} {
		if _, err := l.Append("password.set", "u1", "op_1", id, "succeeded"); err != nil {
			t.Fatal(err)
		}
	}
	head, count := l.Head()
	if count != 3 || head == "" {
		t.Fatalf("want a head over 3 records, got %q/%d", head, count)
	}

	original, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(original)), "\n")
	writeLines(t, path, lines[:2])

	if err := VerifyChain(path); err != nil {
		t.Fatal("a truncated tail still verifies — which is precisely why the anchor exists")
	}
	reopened, err := OpenMutationLog(filepath.Dir(path), 1<<20, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	_, truncatedCount := reopened.Head()
	if truncatedCount >= count {
		t.Fatalf("the count must have gone backwards for the anchor to see it: %d then %d", count, truncatedCount)
	}
}

// A secret-bearing mutation records the event and not the value. There is no
// field for one, which is stronger than a rule that a writer must remember.
func TestASecretBearingMutationLogsTheEventAndNotTheValue(t *testing.T) {
	l, path := openLog(t)
	if _, err := l.Append("password.set", "u1", "u1", "c1", "succeeded"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)

	if !strings.Contains(string(data), `"operation":"password.set"`) {
		t.Fatal("the log must record that a password was set")
	}
	// Every key the record can carry, listed: a field added later without a
	// decision fails here rather than becoming the place a value lands.
	var raw map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &raw); err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{
		"seq": true, "at": true, "operation": true, "subject": true,
		"actor": true, "call_id": true, "outcome": true, "prev": true, "digest": true,
	}
	for k := range raw {
		if !allowed[k] {
			t.Errorf("the record carries an unaccounted field %q — a free field is where a submitted password lands", k)
		}
	}
}

// The chain continues across a rotation, because Prev and Seq live in the
// writer rather than in the file.
func TestRotationBoundsTheVolumeAndKeepsTheChain(t *testing.T) {
	dir := t.TempDir()
	// A tiny cap so a couple of records roll it over.
	l, err := OpenMutationLog(dir, 200, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	var last Record
	for i := range 12 {
		r, err := l.Append("password.set", "u1", "op_1", "c"+strconv.Itoa(i), "succeeded")
		if err != nil {
			t.Fatal(err)
		}
		if last.Digest != "" && r.Prev != last.Digest {
			t.Fatalf("record %d does not follow the one before it across a rotation", r.Seq)
		}
		last = r
	}

	rotated, _ := filepath.Glob(filepath.Join(dir, logFileName+".*"))
	if len(rotated) == 0 {
		t.Fatal("the log must have rotated")
	}
	if len(rotated) > 2 {
		t.Fatalf("retention must bound the volume, kept %d", len(rotated))
	}
	if _, count := l.Head(); count != 12 {
		t.Fatalf("the count must span rotations, got %d", count)
	}
}

// A crash mid-write leaves a partial line. The head is whatever the last
// COMPLETE record said, which is the honest reading: that record was fsynced
// and the partial one never was.
func TestAPartialTrailingLineDoesNotLoseTheLog(t *testing.T) {
	l, path := openLog(t)
	if _, err := l.Append("password.set", "u1", "op_1", "c1", "succeeded"); err != nil {
		t.Fatal(err)
	}
	want, _ := l.Head()
	l.Close()

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(`{"seq":2,"at":"2026-`)
	f.Close()

	reopened, err := OpenMutationLog(filepath.Dir(path), 1<<20, 4)
	if err != nil {
		t.Fatalf("a partial trailing line must not stop the log opening: %v", err)
	}
	defer reopened.Close()
	if got, count := reopened.Head(); got != want || count != 1 {
		t.Fatalf("the head must be the last complete record, got %q/%d", got, count)
	}
}

func writeLines(t *testing.T, path string, lines []string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// An fsync that fails is a record that may not have survived, and the caller is
// about to tell somebody their operation completed.
//
// Its SUCCESS is not observable from inside the process — an unsynced write is
// visible to a same-process read either way — so what is pinned here is the
// failure: Append refuses, and the head does not move, because a record that
// may not have landed must not become the link the next one chains onto.
func TestAFailedFsyncFailsTheAppendAndDoesNotAdvanceTheChain(t *testing.T) {
	l, _ := openLog(t)

	first, err := l.Append("password.set", "u1", "op_1", "c1", "succeeded")
	if err != nil {
		t.Fatal(err)
	}

	l.sync = func() error { return errors.New("disk gone") }
	if _, err := l.Append("password.set", "u1", "op_1", "c2", "succeeded"); err == nil {
		t.Fatal("an unsynced record must not be reported as written")
	}
	head, count := l.Head()
	if head != first.Digest || count != first.Seq {
		t.Fatalf("the head must not advance past a record that may not have landed: %q/%d", head, count)
	}

	// And the chain resumes from the last durable record rather than from the
	// one that failed, so nothing after it verifies against a phantom link.
	l.sync = func() error { return nil }
	next, err := l.Append("password.set", "u1", "op_1", "c3", "succeeded")
	if err != nil {
		t.Fatal(err)
	}
	if next.Prev != first.Digest {
		t.Fatalf("the next record must chain onto the last durable one, got prev=%q", next.Prev)
	}
}

// The sync happens before Append returns, not on some later flush. A source
// guard because the ordering is the contract and no in-process observation can
// see it.
func TestTheSyncPrecedesTheReturn(t *testing.T) {
	src, err := os.ReadFile("mutationlog.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)
	start := strings.Index(body, "func (l *MutationLog) Append(")
	if start < 0 {
		t.Fatal("Append not found")
	}
	body = body[start:]
	if end := strings.Index(body[1:], "\nfunc "); end > 0 {
		body = body[:end]
	}
	syncAt := strings.Index(body, "l.sync()")
	returnAt := strings.Index(body, "return r, nil")
	if syncAt < 0 || returnAt < 0 || syncAt > returnAt {
		t.Fatal("Append must fsync before it returns the record")
	}
}
