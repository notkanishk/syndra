package main

import (
	"fmt"
	"net/http"
	"strings"
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
func (s *server) storageStatus(req OperationRequest) (OperationResult, int, error) {
	// A read: resolved, never rewritten (operatedAccount). The account comes
	// from a read of the NAS now, not the subjects mirror: this is the field a
	// member is about to act on, and that mirror could be six hours old,
	// telling somebody to set a password they set this morning. Its own copy
	// (fresh.go) is forgotten by every write, so a password set through this
	// add-on shows on the next read.
	r, refused, err := s.operatedAccount(req.Subject, nil)
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

	// Best-effort like usage: whether the account works is the urgent half.
	if places, seen, err := s.memberPlaces(req.Fresh, subject.UID, subject.groupIDs, subject.Home); err == nil {
		status.Places, status.PlacesReadable, status.placesSeen = places, true, seen
	}

	// Usage is best-effort: a member who cannot be told their usage can still
	// be told whether their account works, and that is the more urgent half.
	// What was read is sent; QuotaRead is true only when all of it was.
	meters, err := s.shareUsage(subject.UID)
	if meters != nil {
		status.Meters = meters
	}
	status.QuotaRead = err == nil

	return OperationResult{
		Operation: "storage.status", Subject: req.Subject, Outcome: "succeeded",
		Result: &status,
	}, http.StatusOK, nil
}

// shareUsage reports what one uid holds behind each enabled share: every
// dataset the share reaches, nested ones included (usageUnder, the count
// collectUsage samples), against the limit on the share's own dataset. A share
// not wholly read (its dataset's table, or a nested one's, unreadable, or the
// share a plain folder that cannot be counted by itself) makes the whole an
// error, with the meters that were read: partial, and said to be.
//
// Where a share lives is DERIVED from the NAS's own read of its path and the
// datasets' mountpoints, never configured: a configured dataset would be a
// second definition of where a share lives.
//
// Matched on UID and never on the name in the quota row: that name is null for
// a uid the target can no longer resolve, and matching on it would silently
// report nothing for the accounts most likely to matter.
func (s *server) shareUsage(uid int64) ([]Meter, error) {
	shares, err := s.enabledShares()
	if err != nil {
		return nil, err
	}
	mounts, err := s.datasetMounts()
	if err != nil {
		return nil, err
	}
	out := make([]Meter, 0, len(shares))
	var unread []string
	for _, sh := range shares {
		u := usageUnder(mounts, sh.Path, s.userQuotas)
		if u.Top == nil {
			// One share's quota table being unreadable is not a reason to
			// withhold the others, and not one to pass the rest off as all.
			unread = append(unread, sh.Name)
			continue
		}
		if len(u.Unread) > 0 {
			unread = append(unread, sh.Name)
		}
		used, held := u.ByUID[uid]
		if !held {
			continue
		}
		m := Meter{Label: sh.Name, Used: used, Unit: "bytes"}
		// shape-optional: TrueNAS omits `quota` for a uid with no limit set,
		// the common case. 0 is rendered as "no limit set" — see MyStorage.
		if q := u.Top[uid].QuotaBytes; q > 0 {
			m.Limit = &q
		}
		out = append(out, m)
	}
	if len(unread) > 0 {
		return out, fmt.Errorf("the quota of %s could not be read", strings.Join(unread, ", "))
	}
	return out, nil
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
