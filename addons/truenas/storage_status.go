package main

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"
)

// storage.status answers the two questions a member actually has: can I use
// this yet, and how much room is left.
//
// One operation because they are one question. Their account can exist and
// still refuse them — Syndra creates it before any password exists, and TrueNAS
// disables password authentication until the member sets one — so "you have an
// account" and "you can connect" are different facts, and a page that shows the
// first while the second is false reads as a broken system rather than an
// unfinished step.
//
// Member-scoped: it reports on the caller's own account and nothing else. It
// takes no parameters at all, so there is no shape in which one member asks
// about another.

// StorageStatus is a member's own account, as the target sees it: the storage
// profile's result (profiles/storage/status.json), the common status shape
// plus where the account connects and what its password waits on. Who it is
// (the username) is the backend's own binding, and stays off the wire.
type StorageStatus struct {
	// Usable is whether they can actually connect right now. NOT the same as
	// the account existing: an account with password authentication disabled is
	// present, correct, and refuses them.
	Usable bool `json:"usable"`
	// Standing is the account's standing, the one a subjects read reports for
	// it (standingOf, same rule), so the member's page says it without a full
	// subjects read.
	Standing *Standing `json:"standing"`
	// Meters is what they hold in each share they can see, against its quota.
	// Empty is a real answer when QuotaRead is true.
	Meters []Meter `json:"meters"`
	// Observed is when the account was read, set by readNow (fresh.go).
	Observed
	// Places is everywhere this account can connect, read from the folder
	// ACLs for the groups it is actually in, home folder first (reach.go).
	// PlacesReadable is false when the ACLs have never been read; an empty
	// list that was read is "no folder is open to you yet".
	Places         []Place `json:"places"`
	PlacesReadable bool    `json:"places_readable"`
	// placesSeen is how fresh the folder permissions behind Places were when
	// this account was read: a read just now is no fresher than it, and a copy
	// is dated no later than it (Observed.with).
	placesSeen Observed
	// quotaSeen is how fresh the quota tables behind Meters and the places'
	// usage were (quotaTable): a copy dates the card by its read, not now.
	quotaSeen Observed
	// NeedsPassword is the reason, when there is one an action fixes. Setting a
	// password is what turns the account on, so this is an invitation rather
	// than an error.
	NeedsPassword bool `json:"needs_password"`
	// NeedsNewPassword is an account that has had a password and is kept off
	// SMB until it gets a new one: access was taken away, which rotated the
	// password, and has since been given back. TrueNAS turns SMB on only in
	// the call that sets a password, so the member's next password is what
	// opens their folders again. Distinct from a first password
	// (NeedsPassword).
	NeedsNewPassword bool `json:"needs_new_password"`
	// QuotaRead distinguishes "nothing used" from "could not look" (homes.json's
	// name for it). Zero bytes and an unread quota table are the same number
	// otherwise. One share's table unread makes it false: a partial read is
	// not a whole one.
	QuotaRead bool `json:"quota_read"`
}

// Meter is one share and what this member is holding in it
// (common/status.json's Meter), in bytes.
type Meter struct {
	Label string `json:"label"`
	// Used is what the target says this uid is using in the dataset behind
	// the share.
	Used int64 `json:"used"`
	// Limit is absent when no quota is set, which is the common case and is
	// not the same as a quota of zero: a surface says "no limit set" rather
	// than draw a full bar.
	Limit *int64 `json:"limit,omitempty"`
	Unit  string `json:"unit"`
}

// storageStatus reads the caller's own account state and usage.
func (s *server) storageStatus(ctx context.Context, req OperationRequest) (OperationResult, int, error) {
	// A read: resolved, never rewritten (operatedAccount). The account comes
	// from a read of the NAS now, not the subjects mirror: this is the field a
	// member is about to act on, and that mirror could be six hours old,
	// telling somebody to set a password they set this morning. Its own copy
	// (fresh.go) is forgotten by every write, so a password set through this
	// add-on shows on the next read.
	r, refused, err := s.operatedAccount(ctx, req.Subject, nil)
	if err != nil {
		return OperationResult{}, refused, err
	}

	status := StorageStatus{Meters: []Meter{}, Places: []Place{}}
	// One account, one standing: usable and needs_new_password are read off
	// the same Subject the standing is, so the reply never says both.
	subject := *r.Account
	subject.wantsSMB = r.Binding.WantsSMB != nil && *r.Binding.WantsSMB
	subject.wantsEnabled = r.Binding.WantsEnabled
	status.Standing = standingOf(&subject)
	status.Usable = status.Standing.Usable
	status.NeedsPassword = !subject.PasswordSet
	status.NeedsNewPassword = subject.needsNewPassword()

	// One count for the places and the meters, so they never disagree.
	count, countErr := s.countFor(ctx, subject.UID, req.Fresh)
	counted := &count
	if countErr != nil {
		counted = nil
	}

	// Best-effort like usage: whether the account works is the urgent half.
	if places, seen, err := s.memberPlaces(ctx, req.Fresh, subject.groupIDs, subject.Home, counted); err == nil {
		status.Places, status.PlacesReadable, status.placesSeen = places, true, seen
	}

	// Usage is best-effort: a member who cannot be told their usage can still
	// be told whether their account works, and that is the more urgent half.
	// What was read is sent; QuotaRead is true only when all of it was.
	if countErr == nil {
		var meters []Meter
		meters, countErr = count.meters()
		status.Meters = meters
	}
	status.QuotaRead = countErr == nil
	if counted != nil {
		status.quotaSeen = *count.seen
	}

	return OperationResult{
		Operation: "storage.status", Subject: req.Subject, Outcome: "succeeded",
		Result: &status,
	}, http.StatusOK, nil
}

// memberCount is one member's count of what they hold, for their card's
// meters and places: usageUnder, the count collectUsage samples, over every
// dataset, other members' homes included (Rey's files in a home that is
// not his are his), each table read through the shared copy (quotaTable).
type memberCount struct {
	uid    int64
	shares []smbShare
	mounts datasetMap
	table  func(string) map[int64]PlaceUsage
	// seen is how fresh the tables behind the count are: the least fresh of
	// them, so the card is dated by its oldest copy, never by now.
	seen *Observed
}

// countFor makes uid's count. fresh asks for a read of each table now,
// within its cooldown (fresh.go).
func (s *server) countFor(ctx context.Context, uid int64, fresh bool) (memberCount, error) {
	shares, err := s.enabledShares(ctx)
	if err != nil {
		return memberCount{}, err
	}
	mounts, err := s.datasetMounts(ctx)
	if err != nil {
		return memberCount{}, err
	}
	c := memberCount{uid: uid, shares: shares, mounts: mounts, seen: &Observed{}}
	tables := map[string]map[int64]PlaceUsage{} // each at most once per load: a second fresh ask would meet its own cooldown
	c.table = func(dataset string) map[int64]PlaceUsage {
		if t, read := tables[dataset]; read {
			return t
		}
		t, obs := s.quotaTable(ctx, dataset, fresh)
		tables[dataset] = t
		if t != nil {
			if c.seen.TakenAt == "" {
				*c.seen = obs
			} else {
				*c.seen = c.seen.least(obs)
			}
		}
		return t
	}
	return c, nil
}

// quotaTable is one dataset's user quota table for members' cards, nil when
// it cannot be read, and when that read was taken. Every card shares one copy
// per dataset through the fresher (fresh.go): a copy answers while it is
// fresh (freshMaxAge), labelled as a copy and dated by its read, never
// current; a fresh ask reads again outside the cooldown; concurrent loads
// make one read. Counting every dataset on every load would otherwise be one
// NAS call per home per card. A write forgets it like every copy. The
// collector reads its own tables and never this copy.
func (s *server) quotaTable(ctx context.Context, dataset string, fresh bool) (map[int64]PlaceUsage, Observed) {
	val, obs, err := s.fresh.read("quota:"+dataset, fresh, func() (any, time.Time, error) {
		t := s.userQuotas(ctx, dataset)
		if t == nil {
			return nil, time.Time{}, fmt.Errorf("the quota table of %s could not be read", dataset)
		}
		return t, s.fresh.clock(), nil
	})
	if err != nil {
		return nil, Observed{}
	}
	return val.(map[int64]PlaceUsage), obs
}

// meters reports what the member holds behind each enabled share: every
// dataset the share reaches, nested ones included, against the limit on the
// share's own dataset. A share not wholly read (its dataset's table, or a
// nested one's, unreadable; or its own dataset locked) makes the whole an
// error, with the meters that were read: partial, and said to be. A locked
// dataset inside it is left out and is not a partial read: it affects only
// itself (owner, 21 Nov 2026), and is "not counted, locked" where it is a
// place. A share that is a plain
// folder has no table to read: no meter, and not a partial read (it is a
// place that is not counted on its own, on the card where they can open it).
//
// Where a share lives is DERIVED from the NAS's own read of its path and the
// datasets' mountpoints, never configured: a configured dataset would be a
// second definition of where a share lives.
//
// Matched on UID and never on the name in the quota row: that name is null for
// a uid the target can no longer resolve, and matching on it would silently
// report nothing for the accounts most likely to matter.
func (c memberCount) meters() ([]Meter, error) {
	out := make([]Meter, 0, len(c.shares))
	var unread []string
	for _, sh := range c.shares {
		u := usageUnder(c.mounts, sh.Path, c.table)
		if !u.Own {
			continue
		}
		if u.Top == nil || len(u.Unread) > 0 {
			// One share's quota table being unreadable is not a reason to
			// withhold the others, and not one to pass the rest off as all.
			unread = append(unread, sh.Name)
		}
		used, held := u.ByUID[c.uid]
		if !held {
			continue
		}
		m := Meter{Label: sh.Name, Used: used, Unit: "bytes"}
		// shape-optional: TrueNAS omits `quota` for a uid with no limit set,
		// the common case. 0 is rendered as "no limit set" — see MyStorage.
		if q := u.Top[c.uid].QuotaBytes; q > 0 {
			m.Limit = &q
		}
		out = append(out, m)
	}
	if len(unread) > 0 {
		return out, fmt.Errorf("the quota of %s could not be read", strings.Join(unread, ", "))
	}
	return out, nil
}

// place is the member's usage of one place they can change, the count its
// share's meter and the collector make; nil when not all of it was read, and
// then why when the NAS is why (Place.NotCounted). A plain folder is never
// counted as the dataset around it. A home is read as /homes reads it
// (homeUsage). A locked dataset inside a place leaves the rest of it counted:
// only the place that is that dataset says locked.
func (c memberCount) place(p Place) (*PlaceUsage, string) {
	if p.Home {
		u, locked := homeUsage(c.mounts, p.Path, c.uid, c.table)
		if locked {
			return nil, notCountedLocked
		}
		return u, ""
	}
	u := usageUnder(c.mounts, p.Path, c.table)
	switch {
	case !u.Own:
		return nil, notCountedFolder
	case u.Locked:
		return nil, notCountedLocked
	case u.Top == nil || len(u.Unread) > 0:
		return nil, "" // unreadable: say nothing rather than "nothing saved"
	}
	// no row is nothing saved yet
	return &PlaceUsage{UsedBytes: u.ByUID[c.uid], QuotaBytes: u.Top[c.uid].QuotaBytes}, ""
}

// homeUsage is what uid holds in the home folder at home, as /homes and the
// member's card read it: the bytes in the home's own dataset, the limit a
// user quota on the dataset holding its parent (homeDatasets). nil when
// either cannot be read, and locked when a locked dataset is why: never 0.
func homeUsage(m datasetMap, home string, uid int64, table func(string) map[int64]PlaceUsage) (u *PlaceUsage, locked bool) {
	own, parent, locked := homeDatasets(m, home)
	if locked {
		return nil, true
	}
	if own == "" || parent == "" {
		return nil, false
	}
	used, limits := table(own), table(parent)
	if used == nil || limits == nil {
		return nil, false
	}
	return &PlaceUsage{UsedBytes: used[uid].UsedBytes, QuotaBytes: limits[uid].QuotaBytes}, false
}

// homeDatasets is where a home folder is read, the one rule the card, /homes
// and storage.home_bytes share: its bytes in its own dataset (datasetAt: a
// home that is a dataset of its own holds them there, not in the parent), its
// limit on the dataset holding its parent, where convergeQuota writes it; and
// locked when either is, since then its files cannot be read.
func homeDatasets(m datasetMap, home string) (own, parent string, locked bool) {
	own, parent = datasetAt(m, home), datasetOf(path.Dir(home))
	return own, parent, m.locked[own] || m.locked[parent]
}

// datasetOf turns a share's mountpoint into the dataset name the quota call
// wants: `/mnt/pool0/main` -> `pool0/main`.
//
// Anything not under /mnt is not a dataset path, and returning "" for it is how
// a share on something else is skipped rather than guessed at.
func datasetOf(path string) string {
	const prefix = "/mnt/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	return strings.Trim(strings.TrimPrefix(path, prefix), "/")
}
