package main

import (
	"log"
	"time"
)

// settingWishesDerived marks the one-time derivation of wishes for bindings that predate them.
const settingWishesDerived = "wishes_derived"

// deriveWishes is the one-time step for bindings from before wishes were kept (contract 1 bindings
// reach this add-on with none). A hand switch-off of such an account is a difference nothing converges,
// so no apply would ever write its wish and its standing could never say "outside Syndra". At the
// first complete read of the NAS (a request that read it itself, handleSubjects, and that the fresher
// kept), each wish still unset is set to what the NAS holds for that account, which is the last
// thing Syndra left it as, and then a marker is recorded so it never runs again.
//
// It races the writers of a binding at cutover (the first drift pass's applies, a release, a purge),
// so every binding is handled alone, as they are: under its subject's lock, with the write a
// compare-and-set that first checks the binding is still the one read and each wish still unset
// (Store.DeriveWish), through the logged `store.bind` path. A binding released or purged since stays
// gone; a wish an apply has set is not overwritten; a renamed binding is not reverted. The lock is
// never held across NAS I/O: the read is done and nothing here calls the NAS.
//
// Only an active add-on derives, and it counts as in flight (life.Begin), so a drain waits for it
// and read_only or draining skips it (the marker stays unset; a later read does it). A truncated
// read decides nothing (an account missing from it proves nothing). The marker is set only when every
// binding is settled: a failed write, or a binding that changed under the read, leaves it unset so
// the next read finishes the job. Which account a binding names is resolveBinding's, the one rule;
// an account no longer on the NAS keeps no wish (there is no standing to say).
func (s *server) deriveWishes(snap Snapshot) {
	if snap.Truncated {
		return
	}
	if _, done, err := s.store.GetSetting(settingWishesDerived); err != nil || done {
		return
	}
	finished, err := s.life.Begin()
	if err != nil {
		return // not active: a drain or read_only is not interrupted by a bookkeeping write
	}
	defer finished()

	bindings, err := s.store.Bindings()
	if err != nil {
		logStoreFailure("binding", "wishes", err)
		return
	}
	ix := indexAccounts(snap.Subjects)
	settled := true
	for _, b := range bindings {
		if b.WantsEnabled != nil && b.WantsSMB != nil {
			continue
		}
		account := ix.byUID[s.resolveBinding(b, ix).uid] // the one rule: boundAccount's
		if account == nil {
			continue // no longer on the NAS
		}
		if !s.deriveOne(b, account) {
			settled = false
		}
	}
	if !settled {
		return
	}
	mark := Setting{Value: "done", SetBy: "syndra", SetAt: time.Now().UTC()}
	c := caller{operation: "wishes.derive", actor: "syndra"}
	if err := s.mutate(c, "store.setting", func() error { return s.store.PutSetting(settingWishesDerived, mark) }); err != nil {
		log.Printf("[STORE] could not record the wish derivation: %v", err)
	}
}

// deriveOne settles one binding under its subject's lock; false when it must be tried again.
func (s *server) deriveOne(b Binding, account *Subject) bool {
	defer s.subjects.lock(b.SubjectID)()
	c := caller{operation: "wishes.derive", subject: b.SubjectID, actor: "syndra"}.on(account.UID)
	var outcome wishOutcome
	err := s.mutate(c, "store.bind", func() (err error) {
		outcome, err = s.store.DeriveWish(b, account.Enabled, account.SMBEnabled)
		return err
	})
	if err != nil {
		logStoreFailure("binding", b.SubjectID, err)
		return false
	}
	return outcome != wishChanged
}
