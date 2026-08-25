package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// The mutation log (design §17).
//
// An append-only file with no stated guarantees is not a forensic record, so
// this one has a contract: `0600`, fsynced before the operation it describes is
// reported complete, rotated by size with long retention, and each record
// carrying the digest of the record before it.
//
// The chain is worth a few lines because it buys real tamper evidence: an entry
// cannot be altered, and no entry can be removed from the middle, without
// breaking it. What it cannot see is TAIL truncation — delete the last N
// records and what remains verifies perfectly — which is why the head digest
// and the record count are reported through `/health` and anchored by Syndra,
// somewhere this add-on cannot rewrite.
//
// The limit of all of it, stated plainly: this add-on is the least trusted
// component and it is also the thing reporting its own head. A compromised one
// can truncate and report a consistent lie. What the anchor actually detects is
// log loss, volume corruption, and tampering by anything that is not the add-on
// itself — a rotation bug, a full disk, a bad volume mount. That is the failure
// that actually happens, and it is worth detecting.

// Record is one line of the log.
//
// There is no parameter field and no free-text field, for the reason the
// backend's operation record has neither: a `detail` or a `response_body` is
// precisely where a future maintainer puts the target's error payload, and that
// payload is the likeliest place for a submitted password to be echoed back.
// What is recorded is that a thing happened, to whom, by whom — never what was
// sent.
type Record struct {
	Seq       uint64 `json:"seq"`
	At        string `json:"at"`
	Operation string `json:"operation"`
	Subject   string `json:"subject"`
	Actor     string `json:"actor"`
	CallID    string `json:"call_id"`
	Outcome   string `json:"outcome"`
	// Prev is the digest of the record before this one, empty for the first.
	Prev string `json:"prev"`
	// Digest covers every field above it. Held on the record rather than
	// recomputed on read so a verifier compares two independently derived
	// values rather than one value against itself.
	Digest string `json:"digest"`
}

// digestOf hashes the record's content, deliberately excluding Digest itself.
//
// Field-tagged and length-prefixed rather than hashing the marshalled JSON:
// two encoders that order keys differently would produce two digests for one
// record, and a chain that breaks when the writer is upgraded is a chain nobody
// trusts.
func (r Record) digestOf() string {
	h := sha256.New()
	for _, f := range []string{
		fmt.Sprint(r.Seq), r.At, r.Operation, r.Subject, r.Actor, r.CallID, r.Outcome, r.Prev,
	} {
		fmt.Fprintf(h, "%d:", len(f))
		h.Write([]byte(f))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// MutationLog appends records durably and keeps the chain.
type MutationLog struct {
	mu       sync.Mutex
	dir      string
	maxBytes int64
	keep     int

	file *os.File
	size int64

	seq  uint64
	head string
	now  func() time.Time
	// sync is the durability step, injectable so its FAILURE is testable.
	// Its success is not observable from inside the process — an unsynced write
	// is visible to a read from the same process either way — so what a test
	// can pin is the property that matters: a record whose fsync failed is not
	// a record, and Append must say so rather than reporting a completion whose
	// evidence never landed.
	sync func() error
}

const (
	logFileName       = "mutations.log"
	defaultLogMaxSize = 8 << 20
	defaultLogKeep    = 24 // ~192MB ceiling; bounded so the volume cannot fill
)

// OpenMutationLog opens or creates the log and recovers the chain head.
//
// Recovery reads the existing file to the end rather than trusting a sidecar
// pointer. A pointer is a second thing to keep true, and the one moment it
// would be wrong is after the crash the log exists to survive.
func OpenMutationLog(dir string, maxBytes int64, keep int) (*MutationLog, error) {
	if maxBytes <= 0 {
		maxBytes = defaultLogMaxSize
	}
	if keep <= 0 {
		keep = defaultLogKeep
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}
	l := &MutationLog{dir: dir, maxBytes: maxBytes, keep: keep, now: time.Now}

	path := filepath.Join(dir, logFileName)
	// 0600: the log names who did what to whom, and the volume it sits on is
	// shared with whatever else the container mounts.
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open mutation log: %w", err)
	}
	l.file = f
	l.sync = f.Sync

	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("stat mutation log: %w", err)
	}
	l.size = info.Size()

	if err := l.recoverHead(); err != nil {
		f.Close()
		return nil, err
	}
	return l, nil
}

// recoverHead finds the last record written, wherever it lives.
//
// The live file first and then backwards through the rotated segments, because
// the live file is empty for exactly as long as it takes the first record after
// a rotation to arrive — and a restart in that window would otherwise reset the
// sequence to 0 and the head to "". That is precisely the tail-truncation
// signature the anchor exists to detect, manufactured by this add-on's own
// restart.
//
// A malformed trailing line — the shape a crash mid-write leaves — is not
// treated as corruption of the whole log. It is skipped, and the head is
// whatever the last COMPLETE record said, which is the honest reading: that
// record was fsynced and the partial one never was.
func (l *MutationLog) recoverHead() error {
	segments, err := l.segmentsNewestFirst()
	if err != nil {
		return err
	}
	for _, path := range segments {
		last, found, err := lastRecordIn(path)
		if err != nil {
			return err
		}
		if found {
			l.seq, l.head = last.Seq, last.Digest
			return nil
		}
	}
	return nil
}

// segmentsNewestFirst is the live file followed by the rotated ones, newest
// first. The rotated suffix is a zero-padded UTC timestamp, so lexicographic
// order is chronological — chosen for exactly that.
func (l *MutationLog) segmentsNewestFirst() ([]string, error) {
	rotated, err := filepath.Glob(filepath.Join(l.dir, logFileName+".*"))
	if err != nil {
		return nil, fmt.Errorf("list rotated logs: %w", err)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(rotated)))
	return append([]string{filepath.Join(l.dir, logFileName)}, rotated...), nil
}

// lastRecordIn returns the final complete record in one segment.
func lastRecordIn(path string) (Record, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Record{}, false, nil
		}
		return Record{}, false, fmt.Errorf("read mutation log: %w", err)
	}
	defer f.Close()

	var last Record
	var found bool
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var r Record
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			continue
		}
		last, found = r, true
	}
	if err := sc.Err(); err != nil && !errors.Is(err, io.EOF) {
		return Record{}, false, fmt.Errorf("scan mutation log: %w", err)
	}
	return last, found, nil
}

// Append writes one record and returns only once it is durable.
//
// fsync per record, not per batch. At makerspace volume the writes are rare
// enough that batching buys nothing measurable, and the guarantee it would cost
// is the only one that matters here: a completion reported before the record
// reached the disk is a completion with no evidence behind it, which is exactly
// the case the log exists for.
func (l *MutationLog) Append(operation, subject, actor, callID, outcome string) (Record, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if err := l.rotateIfNeededLocked(); err != nil {
		return Record{}, err
	}

	r := Record{
		Seq:       l.seq + 1,
		At:        l.now().UTC().Format(time.RFC3339Nano),
		Operation: operation, Subject: subject, Actor: actor,
		CallID: callID, Outcome: outcome,
		Prev: l.head,
	}
	r.Digest = r.digestOf()

	line, err := json.Marshal(r)
	if err != nil {
		return Record{}, fmt.Errorf("encode mutation record: %w", err)
	}
	line = append(line, '\n')

	n, err := l.file.Write(line)
	if err != nil {
		return Record{}, fmt.Errorf("append mutation record: %w", err)
	}
	if err := l.sync(); err != nil {
		// Written and not durable. Reported as a failure, because the caller is
		// about to tell somebody the operation completed and the evidence for
		// that claim is what just failed to land.
		//
		// The head is deliberately NOT advanced. A record that may not have
		// survived must not become the link the next one chains onto, or one
		// failed fsync silently breaks every verification after it.
		return Record{}, fmt.Errorf("fsync mutation record: %w", err)
	}

	l.size += int64(n)
	l.seq, l.head = r.Seq, r.Digest
	return r, nil
}

// Head is what Syndra anchors: the current digest and the record count.
//
// The count is the half that detects truncation. A chain alone cannot — delete
// the last N records and the remainder verifies perfectly — so the anchor
// compares a count that must only ever grow.
func (l *MutationLog) Head() (digest string, count uint64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.head, l.seq
}

// rotateIfNeededLocked rolls the file over and prunes the oldest.
//
// Rotation renames the live file and opens a fresh one; the chain continues
// across the boundary because `Prev` and `Seq` live in the writer, not in the
// file. Retention is long and bounded only so the volume cannot fill — the log
// is the record that survives losing everything else, so discarding it early
// costs the thing it was for.
func (l *MutationLog) rotateIfNeededLocked() error {
	if l.size < l.maxBytes {
		return nil
	}
	path := filepath.Join(l.dir, logFileName)
	rotated := fmt.Sprintf("%s.%s", path, l.now().UTC().Format("20060102T150405.000000000"))

	if err := l.sync(); err != nil {
		return fmt.Errorf("sync before rotate: %w", err)
	}
	if err := l.file.Close(); err != nil {
		return fmt.Errorf("close before rotate: %w", err)
	}
	if err := os.Rename(path, rotated); err != nil {
		// The old handle is already closed, so leaving it on the struct would
		// leave every later Append writing to a descriptor nothing can reach.
		// Reopened here so the failure costs a rotation rather than the log.
		if reopened, reopenErr := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o600); reopenErr == nil {
			l.file, l.sync = reopened, reopened.Sync
			if info, statErr := reopened.Stat(); statErr == nil {
				l.size = info.Size()
			}
		}
		return fmt.Errorf("rotate mutation log: %w", err)
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("reopen mutation log: %w", err)
	}
	l.file, l.size, l.sync = f, 0, f.Sync
	return l.pruneLocked()
}

func (l *MutationLog) pruneLocked() error {
	entries, err := filepath.Glob(filepath.Join(l.dir, logFileName+".*"))
	if err != nil {
		return fmt.Errorf("list rotated logs: %w", err)
	}
	if len(entries) <= l.keep {
		return nil
	}
	// Lexicographic order is chronological: the suffix is a zero-padded UTC
	// timestamp, chosen for exactly that.
	sort.Strings(entries)
	for _, old := range entries[:len(entries)-l.keep] {
		if err := os.Remove(old); err != nil {
			return fmt.Errorf("prune rotated log: %w", err)
		}
	}
	return nil
}

func (l *MutationLog) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return nil
	}
	return l.file.Close()
}

// VerifyChain reads a log file and reports the first record that breaks it.
//
// It recomputes each digest and checks that each record's `Prev` is the one
// before it, so altering a field, replacing a record, or removing one from the
// middle all fail — and fail with the sequence number, because "the log is
// broken" is not something an operator can act on.
// One segment is not the whole log. `VerifyChain` reads a file, and a rotated
// file opens at whatever sequence was live when it rolled over — so it adopts
// the opening record's sequence and `Prev` rather than demanding record 1.
// `VerifyLog` is the one that walks every segment in order and checks the joins
// between them, which is where a lost rotation actually shows up.
func VerifyChain(path string) error {
	_, _, err := verifySegment(path, "", 0)
	return err
}

// VerifyLog verifies the whole log, rotations included.
func VerifyLog(dir string) error {
	rotated, err := filepath.Glob(filepath.Join(dir, logFileName+".*"))
	if err != nil {
		return fmt.Errorf("list rotated logs: %w", err)
	}
	// Lexicographic is chronological: the suffix is a zero-padded UTC timestamp.
	sort.Strings(rotated)
	prev, expectSeq := "", uint64(0)
	for _, path := range append(rotated, filepath.Join(dir, logFileName)) {
		if prev, expectSeq, err = verifySegment(path, prev, expectSeq); err != nil {
			return err
		}
	}
	return nil
}

// verifySegment walks one file and reports the first record that breaks the
// chain, returning where it left off so the next segment continues from it.
//
// expectSeq 0 means "adopt whatever the first record says", which is what makes
// a single segment verifiable on its own.
func verifySegment(path, prev string, expectSeq uint64) (string, uint64, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return prev, expectSeq, nil
		}
		return prev, expectSeq, fmt.Errorf("open log: %w", err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var r Record
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			return prev, expectSeq, fmt.Errorf("record %d is not readable: %w", expectSeq, err)
		}
		if expectSeq == 0 {
			// The opening record of a segment read on its own: its `Prev` links
			// to a file this call was not given.
			expectSeq, prev = r.Seq, r.Prev
		}
		switch {
		case r.Seq != expectSeq:
			return prev, expectSeq, fmt.Errorf("record %d is missing: the next record calls itself %d", expectSeq, r.Seq)
		case r.Prev != prev:
			return prev, expectSeq, fmt.Errorf("record %d does not follow the one before it", r.Seq)
		case r.Digest != r.digestOf():
			return prev, expectSeq, fmt.Errorf("record %d has been altered", r.Seq)
		}
		prev = r.Digest
		expectSeq++
	}
	return prev, expectSeq, sc.Err()
}
