package main

import (
	"encoding/json"
	"log"
	"time"

	bolt "go.etcd.io/bbolt"
)

// settingWishesDerived marks the one-time derivation of wishes for bindings that predate them.
const settingWishesDerived = "wishes_derived"

// Bindings is every binding, as stored (a legacy one with its account_id alone included).
func (s *Store) Bindings() ([]Binding, error) {
	var out []Binding
	err := s.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketBindings).ForEach(func(_, v []byte) error {
			var b Binding
			if json.Unmarshal(v, &b) == nil {
				out = append(out, b)
			}
			return nil
		})
	})
	return out, err
}

// deriveWishes is the one-time step for bindings from before wishes were kept (contract 1 bindings
// reach this add-on with none). A hand switch-off of such an account is a difference nothing converges,
// so no apply would ever write its wish and its standing could never say "outside Syndra". At the
// first complete read of the NAS (a request that read the NAS itself, handleSubjects), each wish still unset is set to what the NAS holds for that
// account, which is the last thing Syndra left it as: written through the same logged
// `store.bind` path as every binding change, and then a marker is recorded (`store.setting`) so it
// never runs again. A later hand switch-off is then a real difference from the wish.
//
// A truncated read decides nothing (an account missing from it proves nothing), and a failed write
// leaves the marker unset, so the next read finishes the job. An account no longer on the NAS keeps
// no wish (there is no standing to say).
func (s *server) deriveWishes(snap Snapshot) {
	if snap.Truncated {
		return
	}
	if _, done, err := s.store.GetSetting(settingWishesDerived); err != nil || done {
		return
	}
	bindings, err := s.store.Bindings()
	if err != nil {
		logStoreFailure("binding", "wishes", err)
		return
	}
	held := map[int64]Subject{}
	for _, sub := range snap.Subjects {
		held[sub.UID] = sub
	}
	for _, b := range bindings {
		uid := b.uid()
		sub, there := held[uid]
		if uid == 0 || !there || (b.WantsEnabled != nil && b.WantsSMB != nil) {
			continue
		}
		b.UID = uid // a binding that kept account_id alone keeps its account through the write
		if b.WantsEnabled == nil {
			on := sub.Enabled
			b.WantsEnabled = &on
		}
		if b.WantsSMB == nil {
			on := sub.SMBEnabled
			b.WantsSMB = &on
		}
		c := caller{operation: "wishes.derive", subject: b.SubjectID, actor: "syndra"}.on(uid)
		if err := s.mutate(c, "store.bind", func() error { return s.store.PutBinding(b) }); err != nil {
			logStoreFailure("binding", b.SubjectID, err)
			return
		}
	}
	mark := Setting{Value: "done", SetBy: "syndra", SetAt: time.Now().UTC()}
	c := caller{operation: "wishes.derive", actor: "syndra"}
	if err := s.mutate(c, "store.setting", func() error { return s.store.PutSetting(settingWishesDerived, mark) }); err != nil {
		log.Printf("[STORE] could not record the wish derivation: %v", err)
	}
}
