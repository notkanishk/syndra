package main

// Which NAS account a binding names, asked of a read of the accounts: boundAccount's rule and no
// other, so locate, the plan, the wishes, the one-time derivation, the statistics and the adoption
// check can never disagree. An uid inside this add-on's id range names its account by number; a
// contract-1 record key, or 0, is outside it and names none by number, only the account the NAME
// finds (a stranger who happens to hold that number is never matched).

// accountIndex looks accounts up by exact key, as the apply's lookups do against the target.
type accountIndex struct {
	byName map[string]*Subject
	byUID  map[int64]*Subject
}

func indexAccounts(subjects []Subject) accountIndex {
	ix := accountIndex{byName: map[string]*Subject{}, byUID: map[int64]*Subject{}}
	for i := range subjects {
		ix.byName[subjects[i].Username] = &subjects[i]
		ix.byUID[subjects[i].UID] = &subjects[i]
	}
	return ix
}

// boundTo is one binding as resolved against a read: the binding as boundAccount says it now reads
// (a record key replaced by the account's real uid, a rename followed), and the uid of the account
// it names ("" subject never occurs; uid is 0 when it names no account that read could show).
type boundTo struct {
	binding Binding
	uid     int64
}

// resolveBinding is boundAccount over a read. A binding whose uid is in range and whose account is
// not in the read (or whose name another account holds) still names that uid by number: that is what
// boundAccount's `owned` means.
func (s *server) resolveBinding(b Binding, ix accountIndex) boundTo {
	floor, ceiling := s.idRange()
	found, resolved, _, _ := boundAccount(b, floor, ceiling,
		func(n string) (*Subject, error) { return ix.byName[n], nil },
		func(u int64) (*Subject, error) { return ix.byUID[u], nil })
	switch {
	case found != nil:
		return boundTo{binding: resolved, uid: found.UID}
	case inIDRange(b.UID, floor, ceiling):
		return boundTo{binding: resolved, uid: b.UID}
	}
	return boundTo{binding: resolved}
}

// boundAccounts is every account a binding names in this read, by uid, with its subject.
func (s *server) boundAccounts(subjects []Subject) (map[int64]string, map[int64]Binding, error) {
	bindings, err := s.store.Bindings()
	if err != nil {
		return nil, nil, err
	}
	ix := indexAccounts(subjects)
	subjectOf, bindingOf := map[int64]string{}, map[int64]Binding{}
	for _, b := range bindings {
		if r := s.resolveBinding(b, ix); r.uid != 0 {
			subjectOf[r.uid], bindingOf[r.uid] = b.SubjectID, b
		}
	}
	return subjectOf, bindingOf, nil
}

// wishesFor is the bindings' wishes by account uid, over a read of the accounts, or nil when the
// store cannot be read: the standing then says what the NAS shows, as it always could. An account
// absent from the map is bound to nobody.
func (s *server) wishesFor(subjects []Subject) map[int64]Wish {
	_, bindingOf, err := s.boundAccounts(subjects)
	if err != nil {
		logStoreFailure("binding", "subjects", err)
		return nil
	}
	out := make(map[int64]Wish, len(bindingOf))
	for uid, b := range bindingOf {
		out[uid] = Wish{SMB: b.WantsSMB != nil && *b.WantsSMB, Enabled: b.WantsEnabled}
	}
	return out
}

// boundUIDs is every bound account's uid with its subject: how usage on the NAS is told to a person.
func (s *server) boundUIDs(subjects []Subject) (map[int64]string, error) {
	subjectOf, _, err := s.boundAccounts(subjects)
	return subjectOf, err
}

// accountBound says whether any subject's binding names this account.
func (s *server) accountBound(account *Subject) (bool, error) {
	_, bindingOf, err := s.boundAccounts([]Subject{*account})
	_, bound := bindingOf[account.UID]
	return bound, err
}
