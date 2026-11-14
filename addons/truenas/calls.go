package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"unicode/utf16"
)

// Every write is answered at most once per call_id (design §5, contract 2).
//
// Four kinds of write share one store and one rule: apply, a mutating
// operation, lifecycle, and a setting. A call is identified by its call_id AND
// by its request identity, the SHA-256 of the RFC 8785 (JCS) canonical JSON of
// the part of the request that says what it does:
//
//	apply      {subject, desired}                 (plan_id, fingerprint, email excluded)
//	operation  {name, subject, params − secret}   (a secret never reaches the store, even hashed)
//	lifecycle  {state, reason}
//	setting    {name, value}
//
// The rules, in the order once applies them:
//
//  1. Same call_id, same identity, executing in this process now: 409
//     CALL_IN_PROGRESS with Retry-After. "In flight" is an in-memory lease and
//     nothing else: it does not survive a restart and is never inferred from a
//     log intent with no result, which would answer every post-crash re-send of
//     an indeterminate apply with 409 for ever.
//  2. Same call_id, other identity (or another kind of write): 422
//     CALL_ID_REUSED.
//  3. Same call_id, same identity, recorded: the recorded reply, byte for byte,
//     answered before anything else is checked — before the lifecycle gate and
//     before the fingerprint, because the backend re-plans before re-sending an
//     indeterminate row and the fingerprint will have moved.
//  4. Only a 2xx is recorded. A refusal changed nothing, so a re-send is
//     evaluated again rather than answered with an old refusal.
//  5. Writes for one subject run one at a time, whatever their call_ids: an
//     apply, and every operation on the subject's account. Two applies racing
//     to create one account each read "no account", and the loser's cleanup
//     (undoGroup) can delete the winner's primary group; the backend drains
//     its queue concurrently, so this is held here, where the writes happen.
//     Taken after rules 1-3, so a replay never waits behind a write and a
//     re-send of a waiting call is 409 under its lease. The lease never
//     blocks, so the subject lock is the only lock a write waits on and the
//     two cannot deadlock. The lifecycle gate comes after the lock: a write
//     whose turn comes in a drain or read_only is 503 LIFECYCLE_REFUSAL.
//
// An entry contract 1 recorded carries no identity digest and is treated as
// matching any identity, as an entry with no kind always was: outbox rows that
// were in doubt across the upgrade are re-driven under the same call_id, and
// without this each would be refused and become `target_refused`.

// errIdentityMismatch is a call_id recorded for a different request.
var errIdentityMismatch = errors.New("this call id was already used for a different request")

// callLeases is rule 1's lease: call_id → the identity executing under it.
type callLeases struct {
	mu   sync.Mutex
	held map[string]string
}

// take leases callID for one identity. It reports whether the lease was taken,
// and when not, whether the holder is a different request.
func (l *callLeases) take(callID, identity string) (release func(), held, other bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if holder, busy := l.held[callID]; busy {
		return nil, true, holder != identity
	}
	if l.held == nil {
		l.held = map[string]string{}
	}
	l.held[callID] = identity
	var once sync.Once
	return func() {
		once.Do(func() {
			l.mu.Lock()
			delete(l.held, callID)
			l.mu.Unlock()
		})
	}, false, false
}

// subjectLocks is rule 5: one mutex per subject.
//
// ponytail: a subject's mutex is never freed, so the map holds one per subject
// ever written to, bounded by the membership; a reference-counted map is the
// upgrade if that bound ever stops holding.
type subjectLocks struct{ m sync.Map }

// lock waits for subject's writes to be this caller's alone.
func (l *subjectLocks) lock(subject string) (unlock func()) {
	v, _ := l.m.LoadOrStore(subject, new(sync.Mutex))
	mu := v.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

// once runs one write under the rules above. subject is whose account the
// write touches (an apply, an operation), empty for a write that touches none
// (lifecycle, a setting); a subject's write is also the one the lifecycle
// gates. run writes its reply as any handler does; once records it when it is
// a 2xx and passes it on.
func (s *server) once(w http.ResponseWriter, callID, kind, subject string, identity any, run func(http.ResponseWriter)) {
	digest := identityDigest(identity)
	// The kind is part of what is leased: an apply and an operation under one
	// call_id are different requests whatever their identities hash to.
	release, held, other := s.calls.take(callID, kind+"\x00"+digest)
	switch {
	case held && other:
		writeError(w, http.StatusUnprocessableEntity, CodeCallIDReused,
			"This call_id is in use for a different request.", nil)
		return
	case held:
		writeError(w, http.StatusConflict, CodeCallInProgress,
			"A request with this call_id is being carried out now.", nil)
		return
	}
	defer release()

	entry, found, err := s.store.Recall(callID, kind, digest)
	switch {
	case errors.Is(err, errKindMismatch), errors.Is(err, errIdentityMismatch):
		writeError(w, http.StatusUnprocessableEntity, CodeCallIDReused,
			"This call_id was used before for a different request.", nil)
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, CodeStoreUnreadable,
			"The record of earlier calls could not be read, so this one was not run.", nil)
		return
	case found:
		reply := []byte(entry.Result)
		if entry.Digest == "" {
			if reply, err = s.legacyReply(kind, entry.Result); err != nil {
				writeError(w, http.StatusInternalServerError, CodeStoreUnreadable,
					"The recorded reply to this call could not be read.", nil)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(append(reply, '\n'))
		return
	}

	if subject != "" {
		// A mutation of a subject's account: run alone for its subject (rule
		// 5), then refused if the add-on is no longer taking mutations, and
		// counted in flight once admitted so a drain waits for it. Gated after
		// it waits, so a write whose turn comes in a drain or read_only is
		// refused with nothing done, and what a drain waits for is only ever
		// writes already running, never a queue behind them.
		defer s.subjects.lock(subject)()
		done, err := s.life.Begin()
		if err != nil {
			state, _ := s.life.State()
			writeLifecycleRefusal(w, state)
			return
		}
		defer done()
	}
	rec := &capture{header: http.Header{}}
	run(rec)
	// Recorded BEFORE the reply is returned, so a caller retrying after a lost
	// response gets the original outcome rather than a second mutation. Best
	// effort against the reply, though: the mutation already happened, and
	// refusing to report it because the record failed would lose the only
	// account of it the caller gets.
	if rec.status/100 == 2 {
		if err := s.store.Remember(callID, kind, digest, json.RawMessage(bytes.TrimSpace(rec.body.Bytes()))); err != nil {
			logStoreFailure("idempotency", callID, err)
		}
	}
	for k, v := range rec.header {
		w.Header()[k] = v
	}
	w.WriteHeader(rec.status)
	_, _ = w.Write(rec.body.Bytes())
}

// holding is whether a request with callID is executing in this process now.
func (l *callLeases) holding(callID string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	_, busy := l.held[callID]
	return busy
}

// handleCall is GET /calls/{call_id} (feature calls): what this add-on
// recorded for a write, from the three places that know, in order:
//
//   - the call store: a recorded reply is recorded, replayed as once replays
//     it (an entry contract 1 wrote, in contract 2's words);
//   - the lease: a request with the id executing now is in_doubt;
//   - the mutation log, back to constants.call_memory: an intent with no
//     result, or a write that acted or may have with no recorded reply (the
//     record failed), is in_doubt; anything else is unknown: no effect
//     recorded here, which is not "no effect" for a write older than
//     call_memory or sent before this log was written ahead (5.3b).
//
// Every write, apply and lifecycle included, is answered. Syndra settles a
// record whose answer it lost on recorded; this add-on has no physical
// operation, so its unknown settles nothing and the record stays for a
// person (AUTHORING, calls: what unknown settles). Never lifecycle-gated: a read of
// the add-on's own records changes nothing. No operation of this add-on has a
// secret reply (registry secret_result), so nothing is redacted.
//
// ponytail: a lifecycle change logs no intent; one whose reply was lost AND
// whose record failed reads unknown, though it ran. /health says the
// lifecycle as it is, and nobody re-sends one on this answer alone.
func (s *server) handleCall(w http.ResponseWriter, r *http.Request, _ []byte) {
	id := r.PathValue("call_id")
	answer := map[string]any{"call_id": id}
	entry, found, err := s.store.Recorded(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeStoreUnreadable,
			"The record of earlier calls could not be read.", nil)
		return
	}
	switch {
	case found:
		reply := []byte(entry.Result)
		if entry.Digest == "" {
			if reply, err = s.legacyReply(entry.Kind, entry.Result); err != nil {
				writeError(w, http.StatusInternalServerError, CodeStoreUnreadable,
					"The recorded reply to this call could not be read.", nil)
				return
			}
		}
		answer["state"], answer["reply"] = "recorded", json.RawMessage(reply)
	case s.calls.holding(id):
		answer["state"] = "in_doubt"
	default:
		open, err := s.log.CallState(id, s.store.now().Add(-idempotencyTTL))
		if err != nil {
			writeError(w, http.StatusInternalServerError, CodeStoreUnreadable,
				"The mutation log could not be read.", nil)
			return
		}
		answer["state"] = map[bool]string{true: "in_doubt", false: "unknown"}[open]
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(answer)
}

// capture holds a handler's reply until once has decided to record it.
type capture struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (c *capture) Header() http.Header { return c.header }

func (c *capture) WriteHeader(status int) {
	if c.status == 0 {
		c.status = status
	}
}

func (c *capture) Write(b []byte) (int, error) {
	c.WriteHeader(http.StatusOK)
	return c.body.Write(b)
}

// legacyReply is a reply contract 1 recorded, in contract 2's words: the uid
// becomes account_id. An apply's standing was never recorded, so it is read
// from the mirror; an account the mirror does not hold is replayed without
// account_id, since account_id travels only with standing.
func (s *server) legacyReply(kind string, raw json.RawMessage) ([]byte, error) {
	switch {
	case kind == kindApply:
		var out ApplyOutcome
		var old struct {
			UID int64 `json:"uid"`
		}
		if err := json.Unmarshal(raw, &out); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &old); err != nil {
			return nil, err
		}
		out.AccountID, out.Standing = "", nil
		if old.UID != 0 {
			if snap, found, err := s.store.GetSnapshot(); err == nil && found {
				for i := range snap.Subjects {
					if sub := snap.Subjects[i]; sub.UID == old.UID {
						sub.wantsSMB = s.wantedSMB()[old.UID]
						out.AccountID, out.Standing = accountID(old.UID), standingOf(&sub)
					}
				}
			}
		}
		return json.Marshal(out)
	case strings.HasPrefix(kind, kindOperation):
		var out OperationResult
		var old struct {
			AccountUID int64 `json:"account_uid"`
		}
		if err := json.Unmarshal(raw, &out); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &old); err != nil {
			return nil, err
		}
		if old.AccountUID != 0 {
			out.AccountID = accountID(old.AccountUID)
		}
		return json.Marshal(out)
	}
	return raw, nil
}

// identityDigest is the hex SHA-256 of identity in RFC 8785 canonical form.
func identityDigest(identity any) string {
	sum := sha256.Sum256(canonicalJSON(identity))
	return hex.EncodeToString(sum[:])
}

// plainJSON decodes each raw value, so two spellings of one JSON value ("a"
// and "a", [ "x" ] and ["x"]) are one identity.
func plainJSON(raw map[string]json.RawMessage) map[string]any {
	out := make(map[string]any, len(raw))
	for k, v := range raw {
		dec := json.NewDecoder(bytes.NewReader(v))
		dec.UseNumber()
		var x any
		_ = dec.Decode(&x)
		out[k] = x
	}
	return out
}

// canonicalJSON writes v, built of plain decoded values, as RFC 8785 (JCS) does:
// object keys sorted by UTF-16 code unit, no insignificant space, strings with
// only the escapes JSON requires, numbers as ECMAScript prints a double.
func canonicalJSON(v any) []byte {
	var b bytes.Buffer
	writeCanonical(&b, v)
	return b.Bytes()
}

func writeCanonical(b *bytes.Buffer, v any) {
	switch x := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		b.WriteString(strconv.FormatBool(x))
	case string:
		writeCanonicalString(b, x)
	case json.Number:
		f, err := x.Float64()
		if err != nil {
			f = math.NaN()
		}
		b.WriteString(canonicalNumber(f))
	case float64:
		b.WriteString(canonicalNumber(x))
	case int:
		b.WriteString(canonicalNumber(float64(x)))
	case []string:
		b.WriteByte('[')
		for i, e := range x {
			if i > 0 {
				b.WriteByte(',')
			}
			writeCanonicalString(b, e)
		}
		b.WriteByte(']')
	case []any:
		b.WriteByte('[')
		for i, e := range x {
			if i > 0 {
				b.WriteByte(',')
			}
			writeCanonical(b, e)
		}
		b.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		slices.SortFunc(keys, func(a, c string) int {
			return slices.Compare(utf16.Encode([]rune(a)), utf16.Encode([]rune(c)))
		})
		b.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				b.WriteByte(',')
			}
			writeCanonicalString(b, k)
			b.WriteByte(':')
			writeCanonical(b, x[k])
		}
		b.WriteByte('}')
	default:
		// Not a plain decoded value: normalise through JSON first.
		raw, _ := json.Marshal(x)
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.UseNumber()
		var plain any
		_ = dec.Decode(&plain)
		writeCanonical(b, plain)
	}
}

func writeCanonicalString(b *bytes.Buffer, s string) {
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
}

// canonicalNumber prints a double the way ECMAScript's Number.prototype.toString
// does, which is what RFC 8785 §3.2.2.3 requires. Identity objects carry
// strings, booleans and lists; this is here so a number in a parameter is
// still one identity, never a refusal.
func canonicalNumber(f float64) string {
	switch {
	case math.IsNaN(f) || math.IsInf(f, 0):
		return "null" // not JSON; never reaches here from a decoded request
	case f == 0:
		return "0"
	}
	if a := math.Abs(f); a >= 1e-6 && a < 1e21 {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	mant, exp, _ := strings.Cut(strconv.FormatFloat(f, 'e', -1, 64), "e")
	return mant + "e" + exp[:1] + strings.TrimLeft(exp[1:], "0")
}
