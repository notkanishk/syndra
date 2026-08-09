package main

import (
	"encoding/json"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
)

// Local state (design §9). A backstop, never a queue.
//
// There is deliberately no command queue here. Two durable queues would
// disagree about what is still pending, and Syndra's is the one that knows why
// an operation exists. What this holds is a result cache that makes a replayed
// call safe, and a mirror that lets a read answer while the target is down.

var (
	bucketIdempotency = []byte("idempotency")
	bucketSnapshot    = []byte("snapshot")
	bucketBindings    = []byte("bindings")
)

// idempotencyTTL is the actual replay window, and saying so is the point.
//
// §16 declines a separate nonce store on the grounds that the operation id
// already prevents replay, and that argument only holds if the deduplication is
// universal and outlives any plausible retry. Two things bound the risk beyond
// this number: an entitlement apply is level-triggered, so re-applying it is a
// no-op by construction, and in signed mode the signature timestamp rejects a
// stale request outright. So the retention is sized to comfortably exceed any
// outage rather than tuned to a threat.
const idempotencyTTL = 30 * 24 * time.Hour

// Store is the add-on's local durable state.
type Store struct {
	db  *bolt.DB
	now func() time.Time
}

func OpenStore(path string) (*Store, error) {
	// 0600: the snapshot names every account on the target.
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}
	err = db.Update(func(tx *bolt.Tx) error {
		for _, b := range [][]byte{bucketIdempotency, bucketSnapshot, bucketBindings} {
			if _, err := tx.CreateBucketIfNotExists(b); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("initialise store: %w", err)
	}
	return &Store{db: db, now: time.Now}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// idempotencyEntry is one recorded result.
type idempotencyEntry struct {
	StoredAt time.Time       `json:"stored_at"`
	Result   json.RawMessage `json:"result"`
}

// Remember records the outcome of a mutating call against its operation id.
//
// Every mutating call, not a chosen few. §16 declines a nonce store because the
// operation id already prevents replay, and that argument is only true if
// nothing is exempt from this.
func (s *Store) Remember(callID string, result any) error {
	encoded, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("encode idempotent result: %w", err)
	}
	entry, err := json.Marshal(idempotencyEntry{StoredAt: s.now().UTC(), Result: encoded})
	if err != nil {
		return fmt.Errorf("encode idempotency entry: %w", err)
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketIdempotency).Put([]byte(callID), entry)
	})
}

// Recall returns a previously recorded result, or found=false.
//
// An expired entry is reported as absent rather than deleted here: a read path
// that writes turns a replay storm into a write storm, and the sweep below is
// where removal belongs.
func (s *Store) Recall(callID string) (result json.RawMessage, found bool, err error) {
	err = s.db.View(func(tx *bolt.Tx) error {
		raw := tx.Bucket(bucketIdempotency).Get([]byte(callID))
		if raw == nil {
			return nil
		}
		var entry idempotencyEntry
		if err := json.Unmarshal(raw, &entry); err != nil {
			// Unreadable is not "never happened". Reporting absent here would
			// re-run a mutation; reporting an error makes the caller decide,
			// and the caller's safe answer is to refuse.
			return fmt.Errorf("stored result for %s is unreadable: %w", callID, err)
		}
		if s.now().Sub(entry.StoredAt) > idempotencyTTL {
			return nil
		}
		result, found = entry.Result, true
		return nil
	})
	return result, found, err
}

// SweepIdempotency removes entries past the retention window.
func (s *Store) SweepIdempotency() (int, error) {
	var removed int
	err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketIdempotency)
		var stale [][]byte
		if err := b.ForEach(func(k, v []byte) error {
			var entry idempotencyEntry
			if err := json.Unmarshal(v, &entry); err != nil {
				// Unreadable AND old enough to be past any window is rubbish;
				// unreadable and recent is left alone, because Recall refuses
				// on it and a refusal is safer than a silent re-run.
				return nil
			}
			if s.now().Sub(entry.StoredAt) > idempotencyTTL {
				stale = append(stale, append([]byte(nil), k...))
			}
			return nil
		}); err != nil {
			return err
		}
		for _, k := range stale {
			if err := b.Delete(k); err != nil {
				return err
			}
		}
		removed = len(stale)
		return nil
	})
	return removed, err
}

// Snapshot is the last good mirror of the target's state.
//
// `TakenAt` is not decoration. The backend's drift sweep consumes only reads
// marked CURRENT, and a stale mirror served as current would make every outage
// manufacture findings: the sweep would compare current desired state against
// an ageing mirror and report every intervening change as out-of-band. So the
// age travels with the data and the reader decides.
type Snapshot struct {
	TakenAt  time.Time `json:"taken_at"`
	Subjects []Subject `json:"subjects"`
	// Truncated says the read hit its cap and is incomplete. A capped read is
	// current and NOT usable for concluding an absence, which is what half the
	// drift diff does.
	Truncated bool `json:"truncated"`
}

// Subject is one account on the target, as the drift sweep sees it.
//
// There is no hash field, and there is no place to put one. `user.query`
// returns `unixhash` and `smbhash`, and an NT hash is a pass-the-hash
// credential — so the query passes an explicit `select` and this type has
// nowhere for one to land even if that select were edited.
type Subject struct {
	Username   string   `json:"username"`
	UID        int64    `json:"uid"`
	Groups     []string `json:"groups"`
	Enabled    bool     `json:"enabled"`
	SMBEnabled bool     `json:"smb_enabled"`
}

var snapshotKey = []byte("latest")

func (s *Store) PutSnapshot(snap Snapshot) error {
	encoded, err := json.Marshal(snap)
	if err != nil {
		return fmt.Errorf("encode snapshot: %w", err)
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketSnapshot).Put(snapshotKey, encoded)
	})
}

// GetSnapshot returns the mirror and whether there is one.
func (s *Store) GetSnapshot() (Snapshot, bool, error) {
	var snap Snapshot
	var found bool
	err := s.db.View(func(tx *bolt.Tx) error {
		raw := tx.Bucket(bucketSnapshot).Get(snapshotKey)
		if raw == nil {
			return nil
		}
		if err := json.Unmarshal(raw, &snap); err != nil {
			return fmt.Errorf("stored snapshot is unreadable: %w", err)
		}
		found = true
		return nil
	})
	return snap, found, err
}

// Binding is which account on the target belongs to which subject.
//
// Recorded at creation and authoritative thereafter. Derivation is a RECOVERY
// path, not the answer: renaming a TrueNAS account disturbs its home directory,
// its ACL entries and its SMB identity, so a later email change must not rename
// an existing account — and re-deriving from the new email would name an
// account that does not exist while the real one sat there unbound.
//
// The uid is recorded beside the name because the name can change out of band.
// A stable uid whose username moved is a RENAME, and reporting it as a missing
// account would create a replacement while the original kept the home data.
type Binding struct {
	SubjectID string    `json:"subject_id"`
	Username  string    `json:"username"`
	UID       int64     `json:"uid"`
	BoundAt   time.Time `json:"bound_at"`
	// BoundBy records whether an operator adopted an existing account or the
	// apply created one. Adoption hands somebody else's account to a subject if
	// it is wrong, so who decided it survives.
	BoundBy string `json:"bound_by"`
}

func (s *Store) PutBinding(b Binding) error {
	if b.BoundAt.IsZero() {
		b.BoundAt = s.now().UTC()
	}
	encoded, err := json.Marshal(b)
	if err != nil {
		return fmt.Errorf("encode binding: %w", err)
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketBindings).Put([]byte(b.SubjectID), encoded)
	})
}

func (s *Store) GetBinding(subjectID string) (Binding, bool, error) {
	var b Binding
	var found bool
	err := s.db.View(func(tx *bolt.Tx) error {
		raw := tx.Bucket(bucketBindings).Get([]byte(subjectID))
		if raw == nil {
			return nil
		}
		if err := json.Unmarshal(raw, &b); err != nil {
			return fmt.Errorf("stored binding for %s is unreadable: %w", subjectID, err)
		}
		found = true
		return nil
	})
	return b, found, err
}

// BoundUsernames is every name this add-on has claimed, so a collision check
// can tell "taken by somebody we manage" from "taken by an account we have
// never seen" — which are an ordinary rename and an operator decision
// respectively.
func (s *Store) BoundUsernames() (map[string]string, error) {
	out := map[string]string{}
	err := s.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketBindings).ForEach(func(_, v []byte) error {
			var b Binding
			if err := json.Unmarshal(v, &b); err != nil {
				return nil
			}
			out[b.Username] = b.SubjectID
			return nil
		})
	})
	return out, err
}
