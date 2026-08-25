package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Reading the target's state (design §10).
//
// The `select` is the security-relevant part. `user.query` returns `unixhash`
// and `smbhash`, and an NT hash is a pass-the-hash credential — possessing one
// is equivalent to possessing the password for SMB purposes. So the query asks
// for the fields this add-on needs by name, and `Subject` has nowhere for a
// hash to land even if that list were edited.

// subjectReadCap bounds one read.
//
// A cap is not a nicety on a full state read: without one, a NAS with a large
// directory would be pulled into memory in a single JSON document. What matters
// more is that hitting it is REPORTED — a capped read is current and still
// cannot support a conclusion about absence, which is half of what the drift
// diff does with it.
const subjectReadCap = 5000

// nasUser is the shape of one `user.query` row, narrowed to the selected
// fields. Deliberately not a map: a map would carry whatever the middleware
// returned, including the two fields this file exists to keep out.
// `id` and `uid` are different numbers and the difference is load-bearing:
// `id` is the middleware's record key, which is what `user.update` and
// `user.delete` take as their first argument, while `uid` is the unix identity
// that survives a rename and is what a binding recognises an account by. Root
// is id 1 and uid 0 — pass one where the other is meant and the call lands on
// somebody else.
type nasUser struct {
	ID       apiID  `json:"id"`
	Username string `json:"username"`
	UID      int64  `json:"uid"`
	Locked   bool   `json:"locked"`
	SMB      bool   `json:"smb"`
	// PasswordDisabled is the account having no usable credential yet.
	//
	// Read because TrueNAS couples it to SMB and refuses the pair: an account
	// with password authentication disabled CANNOT have `smb` turned on, and
	// the refusal arrives as a validation error on every convergence. Syndra
	// creates accounts before any password exists — the member sets their own
	// — so this is the normal state of a fresh account, not an edge case.
	PasswordDisabled bool `json:"password_disabled"`
	// Builtin is the target's own word for an account that came with the
	// operating system — root, daemon, nobody, and thirty more.
	//
	// Read so they can be left out. They are not unmanaged accounts awaiting an
	// operator's decision, they are the machine: reporting them buries the two
	// real findings under thirty that will never be actioned, and the inventory
	// is a surface whose credibility is set the first time somebody opens it.
	// Worse, every one of them was offered for adoption — including root.
	Builtin bool `json:"builtin"`
	// Groups is a list of group RECORD ids, not gids — the same key
	// `user.update({groups})` writes back, which is why both directions resolve
	// through one map.
	Groups []apiID `json:"groups"`
}

type nasGroup struct {
	ID   apiID  `json:"id"`
	GID  int64  `json:"gid"`
	Name string `json:"group"`
}

// apiID is a middleware record key as it appears on the wire.
//
// Kept as the token it arrived as rather than parsed into an int. The
// middleware has sent these as bare numbers on every release this add-on has
// been tested against and as quoted strings on some directory-service rows, and
// this add-on never does arithmetic on one — it reads an id and hands the same
// id back. Round-tripping the token means being wrong about which form a given
// release uses costs nothing.
type apiID struct{ raw json.RawMessage }

func (i *apiID) UnmarshalJSON(b []byte) error {
	i.raw = append(json.RawMessage(nil), b...)
	return nil
}

func (i apiID) MarshalJSON() ([]byte, error) {
	if len(i.raw) == 0 {
		return []byte("null"), nil
	}
	return i.raw, nil
}

// String is the comparison and map key form, quotes stripped so `3` and `"3"`
// are the same account whichever way each side sent it.
func (i apiID) String() string { return strings.Trim(string(i.raw), `"`) }

func (i apiID) known() bool { return i.String() != "" && i.String() != "null" }

// readSubjects performs the full state read.
//
// Groups are resolved to names, because a gid means nothing to the backend's
// resolver: mappings bind a role to a group NAME, and comparing a name against
// a number would make every subject look like drift.
func (s *server) readSubjects() (Snapshot, error) {
	var users []nasUser
	// `select` names every field, and only these. The two hash fields are
	// absent by construction rather than stripped afterwards — stripping is a
	// step somebody can forget, and the forgetting would not be visible.
	query := []any{
		[]any{},
		map[string]any{
			"select": []string{"id", "username", "uid", "locked", "smb", "groups", "builtin", "password_disabled"},
			"limit":  subjectReadCap + 1,
		},
	}
	if err := s.nas.call("user.query", query, &users); err != nil {
		return Snapshot{}, err
	}

	// Truncation is decided on what the TARGET sent, before anything is
	// dropped here. Deciding it after the filter would let a capped read of
	// mostly system accounts report itself as complete, and "complete" is the
	// word the drift diff needs to conclude that an account is absent.
	truncated := len(users) > subjectReadCap
	if truncated {
		users = users[:subjectReadCap]
	}

	// Filtered here rather than in the query. A server-side `builtin = false`
	// would be one filter expression away from a release that spells it
	// differently, and the failure mode of a rejected query is a read that
	// fails entirely — reporting a healthy target as unreachable. Filtering
	// what came back costs one pass over a list that is already in memory.
	users = withoutSystemAccounts(users)

	names, _, err := s.groupIndex()
	if err != nil {
		// A read that cannot name groups is a read whose group sets are
		// meaningless to the resolver. Reported as a failure rather than
		// returned with numbers in it, because the caller would diff those
		// numbers against names and find everything drifting.
		return Snapshot{}, err
	}

	selfName, _, _ := s.nas.Self()
	subjects := make([]Subject, 0, len(users))
	for _, u := range users {
		groups := make([]string, 0, len(u.Groups))
		for _, id := range u.Groups {
			if name, ok := names[id.String()]; ok {
				groups = append(groups, name)
			}
			// An id with no name is skipped rather than rendered as a number.
			// It is a group the resolver could never have asked for, so
			// including it would be reporting drift against a mapping that
			// cannot exist.
		}
		subjects = append(subjects, Subject{
			ID: u.ID, Username: u.Username, UID: u.UID, Groups: groups,
			// `locked` is the NAS's word for it; `enabled` is Syndra's, and the
			// translation belongs here rather than in the backend, which does
			// not know what TrueNAS calls things.
			Enabled:    !u.Locked,
			SMBEnabled: u.SMB,
			// Carried so the apply can tell "SMB is off because nobody asked
			// for it" from "SMB is off because the target will not accept it
			// yet". They converge differently.
			PasswordSet: !u.PasswordDisabled,
			// Whether this row is the add-on's own service account. Reported
			// rather than hidden: an operator who sees the account they know
			// exists simply vanish from the inventory learns nothing, while one
			// who sees it labelled learns why it is not on offer.
			Self: selfName != "" && strings.EqualFold(u.Username, selfName),
		})
	}
	return Snapshot{TakenAt: time.Now().UTC(), Subjects: subjects, Truncated: truncated}, nil
}

// systemUIDCeiling is where the operating system's accounts stop.
//
// TrueNAS allocates 0-999 to the system and its services and assigns member
// accounts from 1000 up, so this is a fact about the platform rather than a
// tunable. It exists as a SECOND check beside the `builtin` flag because the
// thing being prevented — Syndra taking ownership of root — is worth two
// independent guards: one reads what the target says about an account, and one
// reads what its identity is.
const systemUIDCeiling = 1000

// withoutSystemAccounts drops the accounts that belong to the machine.
func withoutSystemAccounts(users []nasUser) []nasUser {
	kept := make([]nasUser, 0, len(users))
	for _, u := range users {
		if u.Builtin || u.UID < systemUIDCeiling {
			continue
		}
		kept = append(kept, u)
	}
	return kept
}

// groupIndex reads the group table once and returns both directions of it.
//
// Both, from one read, deliberately. A read that resolved ids to names and a
// write that sent names would be two different vocabularies for one thing — and
// the write is the half nobody sees fail until an account is in the wrong
// groups, because the read it is later compared against would speak the other
// one and report the account as converged.
func (s *server) groupIndex() (byID map[string]string, byName map[string]apiID, err error) {
	var groups []nasGroup
	query := []any{
		[]any{},
		map[string]any{"select": []string{"id", "gid", "group"}, "limit": subjectReadCap + 1},
	}
	if err := s.nas.call("group.query", query, &groups); err != nil {
		return nil, nil, fmt.Errorf("read groups: %w", err)
	}
	byID = make(map[string]string, len(groups))
	byName = make(map[string]apiID, len(groups))
	for _, g := range groups {
		byID[g.ID.String()] = g.Name
		byName[g.Name] = g.ID
	}
	return byID, byName, nil
}

// groupsOf names the groups one account belongs to.
//
// Names, not ids, because the only caller compares them against an SMB audit
// watch list and TrueNAS writes group NAMES there. A number compared against a
// name is a member who belongs to nothing, which on that surface reads as a
// member nobody is auditing — the reassuring direction of a wrong answer is
// not available here, but the alarming one is still wrong.
//
// ponytail: two reads per activity call. `activity.get` is operator-initiated
// and one-at-a-time; cache the group index beside the method list if a caller
// ever makes this a loop.
func (s *server) groupsOf(username string) (map[string]bool, error) {
	var accounts []struct {
		// The primary group is its OWN field and is not repeated in `groups`.
		// Membership through it counts for both audit lists, so leaving it out
		// would miss an ignore_list naming somebody's primary group and report
		// them as watched.
		//
		// Raw for the same reason `event_data` is: the inner shape of this one
		// is not in the recorded contract, some releases answer the record and
		// some the bare id, and a surprise in it must cost this field rather
		// than the whole read.
		Group  json.RawMessage `json:"group"`
		Groups []apiID         `json:"groups"`
	}
	if err := s.nas.call("user.query",
		[]any{[]any{[]any{"username", "=", username}},
			map[string]any{"select": []string{"group", "groups"}}},
		&accounts); err != nil {
		return nil, err
	}
	if len(accounts) == 0 {
		return nil, fmt.Errorf("the account this binding names is not on the target")
	}

	names, _, err := s.groupIndex()
	if err != nil {
		return nil, err
	}
	memberOf := make(map[string]bool, len(accounts[0].Groups)+1)
	ids := accounts[0].Groups
	if primary, ok := primaryGroupID(accounts[0].Group); ok {
		ids = append(ids, primary)
	}
	for _, id := range ids {
		if name, ok := names[id.String()]; ok {
			memberOf[name] = true
		}
		// An id with no name is skipped rather than rendered as a number, for
		// the same reason readSubjects skips one: it is a group no list this is
		// compared against could ever have named.
	}
	return memberOf, nil
}

// primaryGroupID reads the id out of whichever form the release answers with.
//
// Not "ok" is not an error. It costs one group's worth of membership on a
// surface that already says what it could not see, where failing the whole
// read would cost all of it.
func primaryGroupID(raw json.RawMessage) (apiID, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return apiID{}, false
	}
	if trimmed[0] == '{' {
		var record struct {
			ID apiID `json:"id"`
		}
		if err := json.Unmarshal(trimmed, &record); err != nil {
			return apiID{}, false
		}
		return record.ID, record.ID.known()
	}
	// A bare token, and only a scalar one. `apiID` keeps whatever arrived and
	// never fails to unmarshal, so without this an array or any other surprise
	// becomes an "id" that matches no group — carried around and compared
	// against real ones rather than reported as the nothing it is.
	if trimmed[0] == '[' {
		return apiID{}, false
	}
	var bare apiID
	_ = json.Unmarshal(trimmed, &bare)
	return bare, bare.known()
}

// groupIDsFor resolves the names a mapping speaks into the ids a write takes.
//
// A name with no group is refused rather than dropped. Dropping it would apply
// a smaller set than the one that was approved and report it as converged,
// which is the silent under-grant version of the same failure the whole
// entitlement plane exists to prevent.
//
// ponytail: one extra group.query per apply that changes groups. Cache it
// alongside the method list if apply volume ever makes that measurable.
func (s *server) groupIDsFor(names []string) ([]apiID, error) {
	if len(names) == 0 {
		return []apiID{}, nil
	}
	_, byName, err := s.groupIndex()
	if err != nil {
		return nil, err
	}
	ids := make([]apiID, 0, len(names))
	for _, name := range names {
		id, ok := byName[name]
		if !ok {
			return nil, fmt.Errorf("this target has no group named %q", name)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// recordID resolves a bound account's record key.
//
// By UID first, because that is the number a binding records precisely so it
// can survive a rename. The apply path already follows a stable uid whose
// username moved out of band and treats it as a rename rather than a missing
// account (6.9); the one-shot operations went through the NAME instead, so an
// out-of-band rename made `password.set`, `password.rotate` and `account.purge`
// answer "the target has no account named X" until an apply happened to run and
// re-sync the binding. One rule for both paths, and it is the uid.
//
// The name is the fallback, for a binding recorded before a uid was, and it is
// also what a uid of zero would otherwise match: uid 0 is root.
//
// Read at the moment of use rather than stored, so there is no second copy of
// the record key to go stale.
func (s *server) recordID(b Binding) (apiID, error) {
	if b.UID != 0 {
		id, err := s.lookupOne("uid", b.UID)
		if err == nil {
			return id, nil
		}
		if !errors.Is(err, errNoSuchAccount) {
			// Ambiguity included, and deliberately. Two accounts sharing a uid
			// is a question about which one, and falling through to the name
			// would answer it by changing the subject — quietly resolving the
			// operation against whichever account happened to hold the recorded
			// name, on a path whose next call sets a credential.
			return apiID{}, err
		}
		// Gone by uid. Fall through: the account may have been recreated under
		// the recorded name, which the binding still points at.
	}
	id, err := s.lookupOne("username", b.Username)
	if err != nil {
		return apiID{}, err
	}
	return id, nil
}

// The two ways a lookup can fail to name one account, kept apart because the
// caller does different things with them. Absence is recoverable — try the
// other field — and ambiguity is not: two accounts matching is a question
// about which one, and the answer is never "whichever the next query returns".
var (
	errNoSuchAccount    = errors.New("the target has no such account")
	errAmbiguousAccount = errors.New("more than one account on the target matches")
)

// lookupOne resolves exactly one account by one field, refusing an ambiguous
// answer rather than picking from it.
func (s *server) lookupOne(field string, value any) (apiID, error) {
	var rows []nasUser
	// limit 2 rather than 1: one row is not evidence that only one matched, and
	// the second row is the whole reason this function can refuse.
	query := []any{
		[]any{[]any{field, "=", value}},
		map[string]any{"select": []string{"id", "username", "uid"}, "limit": 2},
	}
	if err := s.nas.call("user.query", query, &rows); err != nil {
		return apiID{}, err
	}
	switch {
	case len(rows) > 1:
		return apiID{}, fmt.Errorf("%w: %s=%v matched %d", errAmbiguousAccount, field, value, len(rows))
	case len(rows) == 0:
		return apiID{}, fmt.Errorf("%w: %s=%v", errNoSuchAccount, field, value)
	case !rows[0].ID.known():
		// Matched and unusable. Reported as absence, because the recoverable
		// reading is the true one: this field cannot name the record key, and
		// the other field is still worth asking.
		return apiID{}, fmt.Errorf("%w: %s=%v answered without a record id", errNoSuchAccount, field, value)
	}
	return rows[0].ID, nil
}

// Ping is the reachability probe `/health` uses.
func (n *NAS) Ping() (string, error) {
	c, err := n.session()
	if err != nil {
		return "", err
	}
	// Through the same lock as every other call: `Ping` writes a request frame
	// like any other, and a concurrent write is a panic rather than a race.
	n.callMu.Lock()
	res, err := c.Ping()
	n.callMu.Unlock()
	if err != nil {
		n.drop()
		return "", fmt.Errorf("ping: %w", classifyNASError(err))
	}
	n.mu.Lock()
	n.lastRead = n.now()
	n.mu.Unlock()
	return res, nil
}

// statusForLookup answers what a failed account lookup actually was.
//
// `statusFor` is about the SESSION — unreachable, rate-limited, or "the target
// said no" — and a lookup that resolved cleanly to "there is no such account"
// went through its default and came back 502. The backend reads a 5xx as
// indeterminate, so a revocation aimed at an account somebody had already
// deleted was recorded as "we do not know whether it happened" and parked on the
// unconfirmed-revocations surface for ever, when the truthful answer is that
// there was nothing there to revoke and no retry will change it.
//
// Both lookup failures are deterministic and neither is a target fault, so both
// are 4xx: the backend classifies those as rejected, which is what they are.
func statusForLookup(err error) int {
	switch {
	case errors.Is(err, errNoSuchAccount):
		return http.StatusUnprocessableEntity
	case errors.Is(err, errAmbiguousAccount):
		// A different refusal from absence. Two accounts matching is a question
		// about which one, and an operator has to answer it before anything
		// touches either.
		return http.StatusConflict
	default:
		return statusFor(err)
	}
}

// lookupRefusal is the sentence an operator reads.
//
// It distinguishes the two, because what to do next differs: an account that is
// gone needs the binding cleared, and two accounts matching needs one of them
// renamed before anything is safe to run.
func lookupRefusal(err error) error {
	switch {
	case errors.Is(err, errNoSuchAccount):
		return fmt.Errorf("the bound account no longer exists on the target")
	case errors.Is(err, errAmbiguousAccount):
		return fmt.Errorf("more than one account on the target matches the binding")
	default:
		return fmt.Errorf("the bound account could not be located on the target")
	}
}
