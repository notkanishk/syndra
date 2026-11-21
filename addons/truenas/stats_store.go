package main

import (
	"encoding/json"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
)

// The statistics buffer: one bbolt bucket per metric under bucketStats, each
// holding its meta and a "days" bucket keyed by the day ("2006-01-02", which
// sorts as time does), one record per day. Additive: a binary that predates
// it never opens the bucket. Plain bbolt transactions, like PutSnapshot:
// the collector is a read path and never writes the mutation log.

var (
	bucketStats  = []byte("stats")
	statsDaysKey = []byte("days")
	statsMetaKey = []byte("meta")
)

// statsMaxBytes is the buffer's size ceiling per metric, past which the
// oldest days are dropped first. ponytail: bytes, not points (a value's
// length is free to read; a point count means decoding every day); about
// 300 members on 6 shares is under 200 KiB a day, so 64 MiB is the 90 days
// with room.
const statsMaxBytes = 64 << 20

// statDay is one metric's day: its points in keyset order and its gaps.
type statDay struct {
	Points []StatPoint `json:"points,omitempty"`
	Gaps   []StatGap   `json:"gaps,omitempty"`
}

// statsMeta is one metric's state.
type statsMeta struct {
	// Watermark is the day through whose start the metric is final: every
	// earlier day was read after it ended, or has its gap.
	Watermark string `json:"watermark"`
	// TakenAt is the last collection that read the NAS; Failing says the
	// collection since could not.
	TakenAt time.Time `json:"taken_at"`
	Failing bool      `json:"failing,omitempty"`
	// HeldFrom is the oldest day the buffer holds; before it, a gap.
	HeldFrom string `json:"held_from"`
	// Began is the day the first collection ran (set once, in PutStats). While HeldFrom is still Began, the days before
	// it were never collected; once the buffer has trimmed past it, they were collected and dropped.
	Began string `json:"began,omitempty"`
	// Version is the meta's schema: 0 (absent) is a meta written before Began existed, migrated once
	// by beganFor; 1 is the current.
	Version int `json:"version,omitempty"`
	// Behind is the day from which the audit log was too long to read in
	// one run: counted on the next.
	Behind string `json:"behind,omitempty"`
	// Unbound is who used the NAS without a binding, by day: a uid, or a
	// digest of a name the NAS no longer resolves. Never on the wire; only
	// its count is.
	Unbound map[string][]string `json:"unbound,omitempty"`
	// Recording is, per share, the instant its coverage began: the first
	// collection of an unbroken chain that each saw its auditing record
	// everyone (storage.sessions only). A day is counted only from its
	// first midnight at or after it: nothing before was observed recorded.
	Recording map[string]time.Time `json:"recording,omitempty"`
	// LastSeen is, per share, the last collection that saw it record
	// everyone, and Every the STATS_COLLECT_EVERY it ran under: a sighting
	// more than 3 x the larger of that and the current one after it breaks
	// the chain, so changing the setting never does.
	LastSeen map[string]time.Time     `json:"last_seen,omitempty"`
	Every    map[string]time.Duration `json:"every,omitempty"`
	// Before is, per share, the reason the days before Recording are gaps
	// when it is not reasonNotYet: reasonNAS or reasonUnwatched.
	Before map[string]string `json:"before,omitempty"`
}

func statsBucket(tx *bolt.Tx, metric string) *bolt.Bucket {
	if root := tx.Bucket(bucketStats); root != nil {
		return root.Bucket([]byte(metric))
	}
	return nil
}

// StatsMeta is one metric's state, and whether there is any.
func (s *Store) StatsMeta(metric string) (statsMeta, bool, error) {
	var m statsMeta
	var found bool
	err := s.db.View(func(tx *bolt.Tx) error {
		b := statsBucket(tx, metric)
		if b == nil || b.Get(statsMetaKey) == nil {
			return nil
		}
		found = true
		if err := json.Unmarshal(b.Get(statsMetaKey), &m); err != nil {
			return fmt.Errorf("stored statistics of %s are unreadable: %w", metric, err)
		}
		return nil
	})
	return m, found, err
}

// StatsDays calls each with every stored day from from on, in order, until
// it answers false.
func (s *Store) StatsDays(metric, from string, each func(day string, rec statDay) bool) error {
	return s.db.View(func(tx *bolt.Tx) error {
		b := statsBucket(tx, metric)
		if b == nil || b.Bucket(statsDaysKey) == nil {
			return nil
		}
		cur := b.Bucket(statsDaysKey).Cursor()
		for k, v := cur.Seek([]byte(from)); k != nil; k, v = cur.Next() {
			var rec statDay
			if err := json.Unmarshal(v, &rec); err != nil {
				return fmt.Errorf("stored day %s of %s is unreadable: %w", k, metric, err)
			}
			if !each(string(k), rec) {
				return nil
			}
		}
		return nil
	})
}

// PutStats writes one collection of a metric in one transaction: each day
// in days replaced whole (an empty record deletes it, so a provisional 3
// that is now 0 is gone), then the meta. Then the buffer's bounds: days
// before oldest go, and past statsMaxBytes the oldest go first; HeldFrom
// moves up past what went, and never back.
func (s *Store) PutStats(metric string, meta statsMeta, days map[string]statDay, oldest string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		root, err := tx.CreateBucketIfNotExists(bucketStats)
		if err != nil {
			return err
		}
		b, err := root.CreateBucketIfNotExists([]byte(metric))
		if err != nil {
			return err
		}
		db, err := b.CreateBucketIfNotExists(statsDaysKey)
		if err != nil {
			return err
		}
		for day, rec := range days {
			if len(rec.Points) == 0 && len(rec.Gaps) == 0 {
				if err := db.Delete([]byte(day)); err != nil {
					return err
				}
				continue
			}
			v, err := json.Marshal(rec)
			if err != nil {
				return err
			}
			if err := db.Put([]byte(day), v); err != nil {
				return err
			}
		}

		meta.Version, meta.Began = beganFor(b.Get(statsMetaKey), meta, oldest)
		if meta.HeldFrom < oldest {
			meta.HeldFrom = oldest
		}
		size := 0
		_ = db.ForEach(func(_, v []byte) error { size += len(v); return nil })
		var drop [][]byte
		cur := db.Cursor()
		for k, v := cur.First(); k != nil && (string(k) < meta.HeldFrom || size > statsMaxBytes); k, v = cur.Next() {
			drop, size = append(drop, append([]byte(nil), k...)), size-len(v)
			if after := nextDayKey(string(k)); after > meta.HeldFrom {
				meta.HeldFrom = after
			}
		}
		for _, k := range drop {
			if err := db.Delete(k); err != nil {
				return err
			}
		}
		for day := range meta.Unbound {
			if day < meta.HeldFrom {
				delete(meta.Unbound, day)
			}
		}
		v, err := json.Marshal(meta)
		if err != nil {
			return err
		}
		return b.Put(statsMetaKey, v)
	})
}

// statsMetaVersion is the current schema of statsMeta.
const statsMetaVersion = 1

// beganFor is the meta's Version and Began as they are to be written, from what is stored (stored,
// nil when there is none) and what this run brings (meta). Began is decided exactly once and then
// never changes, so nothing later (the clock, the zone, the buffer's size) can flip it.
//
//   - First collection: the stored Watermark was empty and this run's is not, so the day it held
//     from is the day counting began. A failed first run leaves Watermark empty and decides nothing.
//   - A meta from before Began existed (Version 0) that had collected is migrated here, once, by the one rule that is
//     left to it: a HeldFrom later than the oldest day the buffer could hold has never been trimmed,
//     so it is the first collected day; at or before it the buffer may have trimmed, and Began stays
//     empty (the gap before HeldFrom is then truthfully "no longer holds"). It is then Version 1 and
//     never judged again.
func beganFor(stored []byte, meta statsMeta, oldest string) (int, string) {
	var was statsMeta
	if stored == nil || json.Unmarshal(stored, &was) != nil {
		if meta.Watermark != "" {
			return statsMetaVersion, meta.HeldFrom
		}
		return statsMetaVersion, ""
	}
	if was.Began != "" {
		return statsMetaVersion, was.Began
	}
	if was.Version < statsMetaVersion && was.Watermark != "" {
		// Collected before Began existed: judged here, once. (A version-0 meta that never collected
		// has nothing to judge: it falls to the first-collection rule below.)
		if was.HeldFrom != "" && was.HeldFrom > oldest {
			return statsMetaVersion, was.HeldFrom
		}
		return statsMetaVersion, ""
	}
	if was.Watermark == "" && meta.Watermark != "" {
		return statsMetaVersion, meta.HeldFrom
	}
	return statsMetaVersion, ""
}

// nextDayKey is the calendar day after a stored one, by its date alone.
func nextDayKey(day string) string {
	t, _ := time.Parse(time.DateOnly, day)
	return t.AddDate(0, 0, 1).Format(time.DateOnly)
}
