package main

import (
	"bufio"
	"bytes"
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

// What a record says became of a write. failed is the target answering no
// (or a store transaction that did not commit): nothing changed. unknown is
// every other error: the write may have landed, and a record must not claim
// otherwise.
const (
	outcomeIntent    = "intent"
	outcomeSucceeded = "succeeded"
	outcomeFailed    = "failed"
	outcomeNotSent   = "not_sent"
	outcomeUnknown   = "unknown"
)

// The mutation log (design §17).
//
// An append-only file with no stated guarantees is not a forensic record, so
// this one has a contract: `0600`, rotated by size with long retention, each
// record carrying the digest of the record before it, and written ahead of the
// change it describes, as a database journals (5.3b): an intent record,
// fsynced, before each write (server.mutate), and a result record after it. An
// intent with no result is a write in doubt: the add-on stopped between the
// two (InDoubt), and the next start settles it unknown (Recover).
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
	// Outcome is outcomeIntent on an intent, and on a result what became of
	// the write. A record from before intents is a result written after the
	// fact, always "succeeded".
	Outcome string `json:"outcome"`
	// Write is what the intent is about to change: the NAS method it sends, or
	// "store.…" for the add-on's own store (a binding, a setting). Intent is,
	// on a result, the Seq of the intent it settles. Both are absent from
	// every record written before intents.
	Write  string `json:"write,omitempty"`
	Intent uint64 `json:"intent,omitempty"`
	// AccountID is the account the write is for, by uid: on a create's writes
	// from the uid on (createAndConverge), and on every binding write (bind,
	// unbind, and a purge's delete). A `user.create` is the one write no other
	// field ties to an account, and design §5's one adoption without a person
	// matches on it (CreateInDoubt): by id, since a name is display, never
	// identity. A uid is not unique across the log on trust: an orphan
	// group's gid is reused by the next create (emptyGroupNamed), and a
	// restored store rolls ClaimID's high-water mark back, so CreateInDoubt
	// reads every subject's records of the id. The binding's own id, which
	// the store and Syndra hold already; never a value sent beside it.
	AccountID string `json:"account_id,omitempty"`
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
//
// Write and Intent join only when set: a record from before them digests
// exactly as it always did, so the chain Syndra anchors verifies across the
// upgrade and the head only moves forward. Absent and zero are one value, on
// the wire (omitempty) and here alike.
func (r Record) digestOf() string {
	fields := []string{
		fmt.Sprint(r.Seq), r.At, r.Operation, r.Subject, r.Actor, r.CallID, r.Outcome, r.Prev,
	}
	if r.Write != "" || r.Intent != 0 {
		fields = append(fields, r.Write, fmt.Sprint(r.Intent))
	}
	if r.AccountID != "" {
		// Likewise, so every record written before it digests as it did.
		// Length-prefixed fields decode one way, so a record with it never
		// digests as one without.
		fields = append(fields, r.AccountID)
	}
	h := sha256.New()
	for _, f := range fields {
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
	// syncDir makes the directory's entries durable (the package's syncDir),
	// injectable for the same reason. dirDirty says a rotation renamed or
	// created an entry that is not durable yet: the next Append syncs the
	// directory before it writes, and fails if it cannot, until one holds.
	syncDir  func() error
	dirDirty bool
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
	l.syncDir = func() error { return syncDir(dir) }
	// A file's fsync makes its bytes durable, not its name. On every open, not
	// only a create: the file may have been created, or a rotation renamed,
	// by a run that stopped before its own directory sync held.
	if err := l.syncDir(); err != nil {
		f.Close()
		return nil, err
	}

	// Before anything is appended: a crash mid-line leaves bytes that the next
	// Append would otherwise write straight after, merging two records into one
	// line nothing can parse.
	if err := truncateTornTail(f); err != nil {
		f.Close()
		return nil, err
	}
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

// truncateTornTail cuts the live file back to the end of its last complete
// record: the last newline-terminated line that parses. Anything after it was
// never fsynced as a whole, so nothing acknowledged is lost.
func truncateTornTail(f *os.File) error {
	data, err := os.ReadFile(f.Name())
	if err != nil {
		return fmt.Errorf("read mutation log: %w", err)
	}
	var keep, off int
	for off < len(data) {
		nl := bytes.IndexByte(data[off:], '\n')
		if nl < 0 {
			break
		}
		line := bytes.TrimSpace(data[off : off+nl])
		off += nl + 1
		var r Record
		if len(line) == 0 || json.Unmarshal(line, &r) == nil {
			keep = off
		}
	}
	if keep == len(data) {
		return nil
	}
	if err := f.Truncate(int64(keep)); err != nil {
		return fmt.Errorf("truncate torn mutation log tail: %w", err)
	}
	return f.Sync()
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
func lastRecordIn(path string) (last Record, found bool, err error) {
	err = eachRecord(path, func(r Record) { last, found = r, true })
	return last, found, err
}

// eachRecord calls fn for every complete record in one segment, in order. A
// missing segment has none; a line that does not parse is the torn tail a
// crash leaves, and skipped.
func eachRecord(path string, fn func(Record)) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read mutation log: %w", err)
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
			continue
		}
		fn(r)
	}
	if err := sc.Err(); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("scan mutation log: %w", err)
	}
	return nil
}

// Append writes one record and returns only once it is durable.
//
// fsync per record, not per batch. At makerspace volume the writes are rare
// enough that batching buys nothing measurable, and the guarantee it would cost
// is the only one that matters here: a completion reported before the record
// reached the disk is a completion with no evidence behind it, which is exactly
// the case the log exists for.
//
// The log sets Seq, At, Prev and Digest; the caller says the rest.
func (l *MutationLog) Append(r Record) (Record, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if err := l.rotateIfNeededLocked(); err != nil {
		return Record{}, err
	}
	if l.dirDirty {
		// The rotation's names first: a record is not written into a file
		// whose name a crash could still lose.
		if err := l.syncDir(); err != nil {
			return Record{}, err
		}
		l.dirDirty = false
	}

	r.Seq, r.At, r.Prev = l.seq+1, l.now().UTC().Format(time.RFC3339Nano), l.head
	r.Digest = r.digestOf()

	line, err := json.Marshal(r)
	if err != nil {
		return Record{}, fmt.Errorf("encode mutation record: %w", err)
	}
	line = append(line, '\n')

	n, err := l.file.Write(line)
	if err != nil {
		l.rollbackLocked()
		return Record{}, fmt.Errorf("append mutation record: %w", err)
	}
	if err := l.sync(); err != nil {
		// Written and not durable. Reported as a failure, because the caller is
		// about to send the write this record announces, and a write whose
		// intent may not survive a crash must not be sent (server.mutate).
		//
		// The head is deliberately NOT advanced. A record that may not have
		// survived must not become the link the next one chains onto, or one
		// failed fsync silently breaks every verification after it.
		//
		// And the bytes are cut back to the previous end. Left in place, the next
		// record would reuse this one's seq on a second line, or follow a
		// partial write onto the same line.
		l.rollbackLocked()
		return Record{}, fmt.Errorf("fsync mutation record: %w", err)
	}

	l.size += int64(n)
	l.seq, l.head = r.Seq, r.Digest
	return r, nil
}

// rollbackLocked drops whatever a failed append left past the last good end.
// Best-effort: if this fails too the disk is failing, and truncateTornTail
// repairs the tail on the next open.
func (l *MutationLog) rollbackLocked() {
	_ = l.file.Truncate(l.size)
}

// Head is what Syndra anchors: the current digest and the record count.
//
// The count is the half that detects truncation. A chain alone cannot — delete
// the last N records and the remainder verifies perfectly — so the anchor
// compares a count that must only ever grow.
//
// An empty log reports the digest of the empty log, never "": Syndra refuses
// an empty head at first sight, because recorded as the baseline it would
// compare equal to the next empty one and read as healthy for ever. The chain
// itself is unchanged: the first record's Prev is still "".
func (l *MutationLog) Head() (digest string, count uint64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.seq == 0 {
		return emptyLogDigest, 0
	}
	return l.head, l.seq
}

// emptyLogDigest is SHA-256 of no bytes: the head of a log with no record.
const emptyLogDigest = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

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
	// The rename, the new file and the pruned segments are directory
	// entries: durable only with the directory, which Append syncs next, and
	// retries on every append after until it holds.
	l.dirDirty = true
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

// InDoubt is every intent with no result, oldest first: a write the add-on
// may or may not have made, because it stopped between the two records.
//
// ponytail: reads every segment, up to keep × maxBytes (~192MB by default),
// once per start; a checkpoint of open intents is the upgrade if that is slow.
func (l *MutationLog) InDoubt() ([]Record, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	segments, err := l.segmentsNewestFirst()
	if err != nil {
		return nil, err
	}
	open := map[uint64]Record{}
	for i := len(segments) - 1; i >= 0; i-- {
		err := eachRecord(segments[i], func(r Record) {
			if r.Outcome == outcomeIntent {
				open[r.Seq] = r
			} else if r.Intent != 0 {
				delete(open, r.Intent)
			}
		})
		if err != nil {
			return nil, err
		}
	}
	out := make([]Record, 0, len(open))
	for _, r := range open {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Seq < out[j].Seq })
	return out, nil
}

// Recover settles every write in doubt, as a write-ahead log is recovered at
// start: each intent with no result gets one, unknown, because whether it
// landed is read on the NAS, not here. It returns the intents it settled, for
// main to say once. Settled, they are not in doubt again, so a second start
// appends nothing and the head and count only move forward. Syndra already
// holds the same calls as unconfirmed, since their replies never came, and
// re-plans before it re-sends.
func (l *MutationLog) Recover() ([]Record, error) {
	open, err := l.InDoubt()
	if err != nil {
		return nil, err
	}
	for i, intent := range open {
		result := intent
		result.Intent, result.Outcome = intent.Seq, outcomeUnknown
		if _, err := l.Append(result); err != nil {
			return open[:i], err
		}
	}
	return open, nil
}

// CreateInDoubt says whether the log holds design §5's one licence to adopt
// without a person, for this subject and this account id: one candidate,
// this subject's, and nothing since that says the account was anyone's.
//
//   - A candidate is a `user.create` intent asking for the id, kept with its
//     newest result. This subject's must have result `unknown` (settled so at
//     a start by Recover, or a write that got no answer).
//   - Any later binding record of the id, whichever subject wrote it (a
//     store.bind or store.unbind whose result is succeeded or unknown, since
//     an unknown one may have landed), cancels every candidate before it: the
//     account has been bound since, by a create, an adoption or this licence
//     itself, so the doubt is over and it is adopted at most once. So does a
//     later bind of this subject to any account, which covers a binding
//     written before records carried an id.
//   - Another subject's surviving candidate that did not plainly fail (any
//     result but failed or not_sent) is a second claim on the id: two
//     candidates are a conflict for a person, never an adoption.
//
// Every other case is no, an intent written before intents carried an
// account id, or one whose segment has been rotated away, included.
//
// ponytail: reads every segment, as InDoubt does, once per create that finds
// its derived name taken; an index of open creates is the upgrade if slow.
func (l *MutationLog) CreateInDoubt(subject, accountID string) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	segments, err := l.segmentsNewestFirst()
	if err != nil {
		return false, err
	}
	type candidate struct{ subject, result string }
	creates := map[uint64]candidate{} // intent seq -> who asked, its newest result
	for i := len(segments) - 1; i >= 0; i-- {
		err := eachRecord(segments[i], func(r Record) {
			mine := r.AccountID == accountID
			bound := r.Intent != 0 && (r.Outcome == outcomeSucceeded || r.Outcome == outcomeUnknown)
			c, create := creates[r.Intent]
			switch {
			case mine && r.Outcome == outcomeIntent && r.Write == "user.create":
				creates[r.Seq] = candidate{subject: r.Subject}
			case r.Intent != 0 && create:
				c.result = r.Outcome
				creates[r.Intent] = c
			case bound && mine && (r.Write == "store.bind" || r.Write == "store.unbind"),
				bound && r.Subject == subject && r.Write == "store.bind":
				clear(creates)
			}
		})
		if err != nil {
			return false, err
		}
	}
	licensed := false
	for _, c := range creates {
		switch {
		case c.subject == subject && c.result == outcomeUnknown:
			licensed = true
		case c.subject != subject && c.result != outcomeFailed && c.result != outcomeNotSent:
			return false, nil
		}
	}
	return licensed, nil
}

// syncDir makes a directory's entries durable: a created or renamed file's
// name, which the file's own fsync does not cover. A variable so a test can
// make it fail at open, before a log exists to inject into.
var syncDir = func(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("open log dir: %w", err)
	}
	defer d.Close()
	if err := d.Sync(); err != nil {
		return fmt.Errorf("fsync log dir: %w", err)
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

// CallState is what the log says became of the writes one call made, read
// back to since (constants.call_memory): open while any intent of it has no
// result or a result says unknown or succeeded (it acted, or may have);
// absent when the log holds nothing for it, or only writes that changed
// nothing (failed, not_sent). GET /calls answers in_doubt for open and
// unknown for absent, so "safe to try again" is said only of a call the log
// shows did nothing.
//
// ponytail: a linear scan of the segments back to since, newest first, run
// only for a call /calls was asked about and the store does not hold; an
// index by call_id is the upgrade if /calls gets hot.
func (l *MutationLog) CallState(callID string, since time.Time) (open bool, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	segments, err := l.segmentsNewestFirst()
	if err != nil {
		return false, err
	}
	intents := map[uint64]bool{} // intent seq -> settled
	acted := false
	for _, seg := range segments {
		older := false
		err := eachRecord(seg, func(r Record) {
			if at, err := time.Parse(time.RFC3339Nano, r.At); err == nil && at.Before(since) {
				older = true
			}
			if r.CallID != callID {
				return
			}
			switch {
			case r.Outcome == outcomeIntent:
				if _, seen := intents[r.Seq]; !seen {
					intents[r.Seq] = false
				}
			case r.Intent != 0:
				intents[r.Intent] = true
				fallthrough
			default:
				if r.Outcome == outcomeSucceeded || r.Outcome == outcomeUnknown {
					acted = true
				}
			}
		})
		if err != nil {
			return false, err
		}
		if older {
			break
		}
	}
	for _, settled := range intents {
		if !settled {
			return true, nil
		}
	}
	return acted, nil
}
