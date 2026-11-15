package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strings"
)

// One account identity (slice 7t): which NAS account a binding names, by boundAccount's rule and no
// other. Every caller asks it here: the apply (locate), the plan, the account operations (set, rotate,
// purge), storage.status, activity.get, adoption's repeat check, and the views over a read (the
// wishes behind a standing, the statistics, accountBound). Two adapters, one rule:
//
//   - resolveLive asks it of the target now, one account by an exact key per lookup (accountWhere:
//     the user table alone, since naming an account needs only its uid and name), and rewrites the
//     binding once when it should now read differently (a legacy binding's real uid and marker, an
//     in-range rename followed); operatedAccount is resolveLive with an operation's refusals: 422 no
//     binding or account gone, 409 a conflict or two accounts on one key, 502 an absence read from
//     a cut-short list, 500 STORE_UNREADABLE.
//   - resolveIn asks it of one read of the accounts (plan, a contract-1 reply's standing), and
//     attribute asks it of every binding over one read: /subjects says it on the wire (each row's
//     subject, and the conflicts), and the wishes, the statistics and accountBound are views of it.
//
// The name is asked first. A name that finds the recorded uid names that account, even when another
// account shares the uid; the uid is asked only when the name is gone, and two accounts on whichever
// key is asked are a refusal (errAmbiguousAccount), never the first one listed. An uid inside this
// add-on's id range names its account by number (those numbers are never reused, identity.go). Any
// other uid (an adopted hand-made account's, or a contract-1 record key) is only believed beside the
// recorded name, and a binding whose name is gone while its number is held by another name names no
// account at all: a conflict, and the holder is never its account.
//
// Invariant (TestEveryCallerResolvesOneIdentity): no operation ever sends a write to an account
// resolveLive did not return as its Account. The per-subject lock (calls.go) serialises
// resolve-then-act for applies and write operations; a read operation runs outside it and outside
// the lifecycle gate, so it resolves without rewriting anything.
//
// account_id is the uid and nothing else (AUTHORING, chapter 2): a key within one read, not an
// identity across reads. Which subject an account belongs to is said by attribute, never by a uid
// matched elsewhere; the backend's matching moves off account_id in slice 7t-3.

// accountIndex looks accounts up by exact key, as accountWhere does against the target: a key two
// accounts hold is marked, so a lookup of it refuses as the live one does.
type accountIndex struct {
	byName   map[string]*Subject
	byUID    map[int64]*Subject
	twoNames map[string]bool
	twoUIDs  map[int64]bool
}

func indexAccounts(subjects []Subject) accountIndex {
	ix := accountIndex{byName: map[string]*Subject{}, byUID: map[int64]*Subject{},
		twoNames: map[string]bool{}, twoUIDs: map[int64]bool{}}
	for i := range subjects {
		a := &subjects[i]
		if _, seen := ix.byName[a.Username]; seen {
			ix.twoNames[a.Username] = true
		}
		if _, seen := ix.byUID[a.UID]; seen {
			ix.twoUIDs[a.UID] = true
		}
		ix.byName[a.Username], ix.byUID[a.UID] = a, a
	}
	return ix
}

func (ix accountIndex) name(n string) (*Subject, error) {
	if ix.twoNames[n] {
		return nil, fmt.Errorf("%w: username=%s", errAmbiguousAccount, n)
	}
	return ix.byName[n], nil
}

func (ix accountIndex) uid(u int64) (*Subject, error) {
	if ix.twoUIDs[u] {
		return nil, fmt.Errorf("%w: uid=%d", errAmbiguousAccount, u)
	}
	return ix.byUID[u], nil
}

// accountWhere reads the one account whose field equals value, or nil when the target has none,
// from the user table alone (readAccountsWhere): its groups are record ids, unnamed (subjectWhere
// names them for the apply). Filtered at the target: the whole account list per lookup was a
// 100-person cascade's worth of full reads. The key comparison is kept on this side as well, so a
// target that ignored the filter cannot hand back the wrong account; the second return is that
// read's truncation, which is only ever set by such a target (an exact key matches one row) and
// stops an absence being concluded from a list that was cut short.
func (s *server) accountWhere(field string, value any) (*Subject, bool, error) {
	accounts, truncated, err := s.readAccountsWhere([]any{[]any{field, "=", value}})
	if err != nil {
		return nil, false, err
	}
	var found *Subject
	for i := range accounts {
		got := accounts[i]
		if (field == "username" && got.Username == value) || (field == "uid" && got.UID == value) {
			if found != nil {
				// Two accounts on one key is a question about which one, and the
				// answer is never "whichever the target listed first".
				return nil, truncated, fmt.Errorf("%w: %s=%v", errAmbiguousAccount, field, value)
			}
			found = &accounts[i]
		}
	}
	return found, truncated, nil
}

// inIDRange is whether n is a number this add-on hands out (identity.go). Those numbers are never
// handed out twice (ClaimID's high-water mark), so one names the same account for good; a number
// outside it can come back after a delete, so it never names an account by itself (boundAccount).
func inIDRange(n, floor, ceiling int64) bool { return n >= floor && n <= ceiling }

// legacyHaltsOnHeldNumber is legacy policy B (design 7t D6), pending the owner's answer to Q2: an
// unmarked binding (contract 1) outside the id range whose name is gone while its number is held by
// another name halts as a conflict and creates nothing, the guard a marked binding already gets.
// false is policy A: such a binding reads as missing and the apply re-creates it beside the holder.
const legacyHaltsOnHeldNumber = true

// boundAccount is the rule for which account a binding names. byName and byUID look up one account
// by an exact key. It returns the account (nil when there is none), the binding as it should now
// read, and a conflict when the name is held by a different account of ours, or, for a uid outside
// this add-on's range, when the recorded name is gone and the number is held by another name.
//
// What a binding's uid means decides the rest. In range, it is a real uid that is never reused: the
// account it names is that one, so a rename is followed. Outside the range (an adopted hand-made
// account, uid_kind "account"; or a contract-1 record key, which names nothing by number) a number
// can be handed to a new account after a delete, so it is only ever believed alongside the
// recorded name: name and number agree, or the binding halts for an operator (BindingConflict) and
// is never rebound to whoever holds the number.
func boundAccount(binding Binding, floor, ceiling int64,
	byName func(string) (*Subject, error), byUID func(int64) (*Subject, error),
) (*Subject, Binding, *BindingConflict, error) {
	found, err := byName(binding.Username)
	if err != nil {
		return nil, binding, nil, err
	}
	owned := binding.ownsUID(floor, ceiling)
	if found != nil {
		if owned && found.UID != binding.UID {
			// A name some other account now holds (the original was deleted
			// and the name reused). Converging it would hand this subject's
			// entitlements to a stranger, so it halts like any foreign name.
			return nil, binding, (&BindingConflict{Username: found.Username, AccountID: accountID(found.UID)}).worded(), nil
		}
		if !owned {
			// A legacy binding holding a record key (or a real uid it never said was one): the name
			// found the account, so the real uid, marked, and the next pass is guarded by the check above.
			binding.UID = found.UID
		}
		binding.UIDKind = uidKindAccount
		return found, binding, nil, nil
	}
	if binding.UID == 0 || (!owned && !legacyHaltsOnHeldNumber) {
		// A legacy uid of 0 is root, never a subject's account: with no name hit the account is
		// missing. (Policy A only: an unmarked out-of-range uid is not looked up either.)
		return nil, binding, nil, nil
	}
	// The name moved. A stable uid whose username changed out of band is a
	// RENAME, not a missing account: reporting it as missing would create a
	// replacement while the original kept the home data, or, for a subject
	// wanted off, leave the original unlocked.
	renamed, err := byUID(binding.UID)
	if err != nil {
		return nil, binding, nil, err
	}
	if renamed != nil {
		if !inIDRange(binding.UID, floor, ceiling) {
			// A number outside the range may have been handed to a new account since this one was
			// deleted: not a rename, whatever else is true, marked or not (policy B). An operator
			// decides; nothing is written to the account that holds the number and nothing is created.
			return nil, binding, (&BindingConflict{Username: renamed.Username, AccountID: accountID(renamed.UID),
				numberHeld: true, was: binding.Username}).worded(), nil
		}
		binding.Username = renamed.Username
		binding.UIDKind = uidKindAccount
		return renamed, binding, nil, nil
	}
	// Bound, and neither the name nor the uid is there. The account was
	// deleted out of band; convergence re-creates it under the recorded name
	// rather than re-deriving, because the recorded name is what every ACL and
	// share still refers to.
	return nil, binding, nil, nil
}

// resolved is what one binding names right now, by boundAccount's rule.
type resolved struct {
	// Binding is as it should now read: a real uid and its marker, an in-range rename followed.
	Binding Binding
	// Account is the account it names; nil when none this read could show.
	Account *Subject
	// Conflict is set when it names nothing because another account holds the name or the number.
	Conflict *BindingConflict
	// Cut is an absence concluded from a cut-short read (subjectWhere): not an absence at all.
	Cut bool
}

// resolveIn is boundAccount over one read of the accounts. Its only error is errAmbiguousAccount:
// two accounts in the read on the key it asked.
func (s *server) resolveIn(b Binding, ix accountIndex) (resolved, error) {
	floor, ceiling := s.idRange()
	found, now, conflict, err := boundAccount(b, floor, ceiling, ix.name, ix.uid)
	if err != nil {
		return resolved{Binding: b}, err
	}
	return resolved{Binding: now, Account: found, Conflict: conflict}, nil
}

// resolveLive is boundAccount against the target now, for the subject's binding; bound is false
// when the subject has none. A store that cannot be read is errStoreUnreadable.
//
// who is the write the binding's rewrite is logged under (store.bind), recorded once so the next
// pass finds the account by number and is guarded by its uid. nil is a read, which never writes:
// it runs outside the subject lock and the lifecycle gate, and resolves as well without it.
func (s *server) resolveLive(subject string, who *caller) (r resolved, bound bool, err error) {
	b, bound, err := s.store.GetBinding(subject)
	if err != nil {
		return resolved{}, false, fmt.Errorf("%w: %w", errStoreUnreadable, err)
	}
	if !bound {
		return resolved{}, false, nil
	}
	// Each lookup is one account by an exact key; a cut-short answer only
	// matters when the conclusion is that the account is not there.
	lookup := func(field string, value any) (*Subject, error) {
		found, cut, err := s.accountWhere(field, value)
		r.Cut = r.Cut || cut
		return found, err
	}
	floor, ceiling := s.idRange()
	found, now, conflict, err := boundAccount(b, floor, ceiling,
		func(name string) (*Subject, error) { return lookup("username", name) },
		func(uid int64) (*Subject, error) { return lookup("uid", uid) })
	if err != nil {
		return resolved{}, true, err
	}
	r.Binding, r.Account, r.Conflict = now, found, conflict
	r.Cut = r.Cut && found == nil && conflict == nil
	if who != nil && (now.Username != b.Username || now.UID != b.UID || now.UIDKind != b.UIDKind) {
		if err := s.mutate(who.on(now.UID), "store.bind", func() error { return s.store.PutBinding(now) }); err != nil {
			logStoreFailure("binding", subject, err)
		}
	}
	return r, true, nil
}

// operatedAccount resolves the subject's account for an operation, refusing rather than guessing:
// the account and its binding, or the status the operation answers and its sentence. who is as for
// resolveLive (nil for a read operation).
//
// An operation that could not find its subject must not fall back to derivation: derivation names an
// account that may not exist, and setting a password on the wrong one is the whole hazard.
func (s *server) operatedAccount(subject string, who *caller) (resolved, int, error) {
	r, bound, err := s.resolveLive(subject, who)
	switch {
	case err != nil:
		status, refusal := liveRefusal(err)
		return resolved{}, status, refusal
	case !bound:
		return resolved{}, http.StatusUnprocessableEntity, errors.New("This person has no account on TrueNAS yet.")
	case r.Conflict != nil:
		// The binding names nothing: another account holds its name or number. Refused whole, with
		// the apply's own words; the holder is never acted on.
		return resolved{}, http.StatusConflict, conflictRefusal(r.Conflict)
	case r.Account == nil && r.Cut:
		// A target that ignored the filter and cut its list short proves no absence.
		return resolved{}, http.StatusBadGateway, errAccountNotFound
	case r.Account == nil:
		// Gone: a deterministic 4xx, which Syndra records as rejected, never a
		// 5xx that parks it as "we do not know whether it happened".
		return resolved{}, http.StatusUnprocessableEntity, errors.New("This person's account is no longer on TrueNAS.")
	}
	return r, 0, nil
}

// liveRefusal is resolveLive's error as an operation answers it.
func liveRefusal(err error) (int, error) {
	if errors.Is(err, errStoreUnreadable) {
		// The add-on's own store, read before anything is sent to the NAS:
		// its fault, never the subject's, so 500 STORE_UNREADABLE (as /plan
		// answers it), never the 422 that means "no account". On a write,
		// Syndra reads any 5xx as possibly acted (indeterminate), which
		// overstates it and is safe: a refusal is never recorded, so GET
		// /calls answers unknown and a re-send runs it once.
		log.Printf("[STORE] binding for an operation could not be read: %v", err)
		return http.StatusInternalServerError, errStoreUnreadable
	}
	return statusForLookup(err), lookupRefusal(err)
}

// conflictRefusal is a conflict as an operation's refusal: the words the apply and plan say it in.
// It stays 409 OPERATION_FAILED, not BINDING_CONFLICT: the schema has BINDING_CONFLICT carry an
// apply outcome (subject, effect blocked, conflict: error.json), and an operation has no effect to
// carry. The same conflict is on /subjects (conflicts), with the holder, for Syndra to act on.
func conflictRefusal(c *BindingConflict) error {
	detail, next := c.words()
	return errors.New(detail + " " + next)
}

// attribution is every binding over one read of the accounts, by resolveIn and no other rule.
type attribution struct {
	// subjectOf is the subject of each account in the read that a binding names (resolveIn's
	// Account): what /subjects says as the row's subject. A binding names one account at most, and
	// one that names none is absent here, so a subject is on one row at most. Keyed by uid and name
	// together: a uid two rows share names neither alone (a name two rows share is never resolved).
	subjectOf map[rowKey]string
	// bindingOf is every uid a binding names by this read, with the binding: subjectOf's accounts,
	// and the own uid of an in-range binding that names none here (its account is not in the read,
	// or another account holds its name), since an in-range number is never reused. What usage and
	// the wishes behind a standing are keyed by; never said on the wire as a row's subject.
	bindingOf map[int64]Binding
	// claimedTwice is every uid two bindings name: given to neither (subjectOf, bindingOf), and
	// still bound for accountBound, so no third subject can adopt it.
	claimedTwice map[int64]bool
	// conflicts are the bindings that name no account, by subject (store order): another account
	// holds their name or number, two accounts hold their key, or two bindings name one account.
	// /subjects' conflicts; the holder is never a row's subject.
	conflicts []SubjectConflict
}

// rowKey is one row of a read: the uid and the name together.
type rowKey struct {
	uid      int64
	username string
}

func rowOf(a *Subject) rowKey { return rowKey{a.UID, a.Username} }

// SubjectConflict is one entry of /subjects' conflicts (subjects.json#/$defs/Conflict): the
// subject whose binding names nothing, and the holder with the apply's own words.
type SubjectConflict struct {
	Subject string `json:"subject"`
	*BindingConflict
}

// attribute is every binding over one read of the accounts. Its error is the store's.
//
// Every binding ends in one of three places, never none: its account's row (subjectOf), the
// conflicts, or neither because its account is gone. A binding whose lookup is ambiguous (two
// accounts in the read on the key boundAccount asked, errAmbiguousAccount) is a conflict listing
// every holder; two bindings naming one account are both conflicts and neither gets the row.
// Each fails closed: no rule picks a winner.
func (s *server) attribute(subjects []Subject) (attribution, error) {
	bindings, err := s.store.Bindings()
	if err != nil {
		return attribution{}, err
	}
	ix := indexAccounts(subjects)
	floor, ceiling := s.idRange()
	at := attribution{subjectOf: map[rowKey]string{}, bindingOf: map[int64]Binding{}, claimedTwice: map[int64]bool{},
		conflicts: []SubjectConflict{}}
	named := map[rowKey][]Binding{} // each row in the read, with every binding that names it
	rows := map[rowKey]*Subject{}
	for _, b := range bindings {
		r, err := s.resolveIn(b, ix)
		uid := int64(0)
		switch {
		case err != nil: // errAmbiguousAccount, resolveIn's only error
			at.conflicts = append(at.conflicts, SubjectConflict{Subject: b.SubjectID,
				BindingConflict: (&BindingConflict{Holders: holdersOf(subjects, b, ix)}).worded()})
		case r.Account != nil:
			uid = r.Account.UID
			named[rowOf(r.Account)] = append(named[rowOf(r.Account)], b)
			rows[rowOf(r.Account)] = r.Account
		case r.Conflict != nil:
			at.conflicts = append(at.conflicts, SubjectConflict{Subject: b.SubjectID, BindingConflict: r.Conflict})
		}
		if uid == 0 && inIDRange(b.UID, floor, ceiling) {
			uid = b.UID
		}
		if uid != 0 {
			at.bindingOf[uid] = b
		}
	}
	for row, bs := range named {
		if len(bs) == 1 {
			at.subjectOf[row] = bs[0].SubjectID
			continue
		}
		account := rows[row]
		at.claimedTwice[account.UID] = true
		delete(at.bindingOf, account.UID)
		for i, b := range bs {
			c := &BindingConflict{Username: account.Username, AccountID: accountID(account.UID), claimedTwice: true}
			if len(bs) == 2 {
				c.BoundTo = bs[1-i].SubjectID
			}
			at.conflicts = append(at.conflicts, SubjectConflict{Subject: b.SubjectID, BindingConflict: c.worded()})
		}
	}
	slices.SortStableFunc(at.conflicts, func(x, y SubjectConflict) int { return strings.Compare(x.Subject, y.Subject) })
	return at, nil
}

// holdersOf is every account in the read on the key a binding's lookup found ambiguous, as
// boundAccount asks them: the recorded name first, and the number only when the name is clear.
func holdersOf(subjects []Subject, b Binding, ix accountIndex) []Holder {
	on := func(a *Subject) bool { return a.UID == b.UID }
	if ix.twoNames[b.Username] {
		on = func(a *Subject) bool { return a.Username == b.Username }
	}
	var out []Holder
	for i := range subjects {
		if on(&subjects[i]) {
			out = append(out, Holder{Username: subjects[i].Username, AccountID: accountID(subjects[i].UID)})
		}
	}
	return out
}

// wishes is the bindings' wishes by account uid. An account absent from the map is bound to nobody.
func (at attribution) wishes() map[int64]Wish {
	out := make(map[int64]Wish, len(at.bindingOf))
	for uid, b := range at.bindingOf {
		out[uid] = Wish{SMB: b.WantsSMB != nil && *b.WantsSMB, Enabled: b.WantsEnabled}
	}
	return out
}

// wishesFor is attribute's wishes over a read of the accounts, or nil when the store cannot be
// read: the standing then says what the NAS shows, as it always could.
func (s *server) wishesFor(subjects []Subject) map[int64]Wish {
	at, err := s.attribute(subjects)
	if err != nil {
		logStoreFailure("binding", "subjects", err)
		return nil
	}
	return at.wishes()
}

// boundUIDs is every bound account's uid with its subject: how usage on the NAS is told to a person.
func (s *server) boundUIDs(subjects []Subject) (map[int64]string, error) {
	at, err := s.attribute(subjects)
	out := make(map[int64]string, len(at.bindingOf))
	for uid, b := range at.bindingOf {
		out[uid] = b.SubjectID
	}
	return out, err
}

// accountBound says whether any subject's binding names this account.
func (s *server) accountBound(account *Subject) (bool, error) {
	at, err := s.attribute([]Subject{*account})
	_, bound := at.bindingOf[account.UID]
	return bound || at.claimedTwice[account.UID], err
}
