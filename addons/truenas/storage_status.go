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
	binding, err := s.boundAccount(req.Subject)
	if err != nil {
		return OperationResult{}, http.StatusUnprocessableEntity, err
	}

	status := StorageStatus{Meters: []Meter{}, Places: []Place{}}

	// Account state from a read of the NAS, not the subjects mirror. This is
	// the field a member is about to act on, and that mirror could be six hours
	// old — telling somebody to set a password they set this morning. Its own
	// copy (fresh.go) is forgotten by every write, so a password set through
	// this add-on shows on the next read.
	var accounts []struct {
		UID              int64   `json:"uid"`
		SMB              bool    `json:"smb"`
		Locked           bool    `json:"locked"`
		PasswordDisabled bool    `json:"password_disabled"`
		Groups           []apiID `json:"groups"`
		Home             string  `json:"home"`
	}
	if err := s.nas.call("user.query",
		[]any{[]any{[]any{"username", "=", binding.Username}},
			map[string]any{"select": []string{"uid", "smb", "locked", "password_disabled", "groups", "home"}}},
		&accounts); err != nil {
		return OperationResult{}, statusFor(err), fmt.Errorf("your account could not be read")
	}
	if len(accounts) == 0 {
		// Bound to an account the target does not have. Said plainly rather
		// than as a usage report of nothing, because the fix is an operator's.
		return OperationResult{}, http.StatusUnprocessableEntity,
			fmt.Errorf("the account this binding names is not on the target")
	}
	account := accounts[0]
	status.NeedsPassword = account.PasswordDisabled
	status.NeedsNewPassword = !account.PasswordDisabled && !account.SMB &&
		binding.WantsSMB != nil && *binding.WantsSMB
	status.Usable = !account.PasswordDisabled && !account.Locked && !status.NeedsNewPassword
	status.Standing = standingOf(&Subject{Username: binding.Username, UID: account.UID,
		Enabled: !account.Locked, SMBEnabled: account.SMB, PasswordSet: !account.PasswordDisabled})

	// Best-effort like usage: whether the account works is the urgent half.
	if places, seen, err := s.memberPlaces(req.Fresh, account.UID, account.Groups, homeOf(account.Home)); err == nil {
		status.Places, status.PlacesReadable, status.placesSeen = places, true, seen
	}

	// Usage is best-effort: a member who cannot be told their usage can still
	// be told whether their account works, and that is the more urgent half.
	// What was read is sent; QuotaRead is true only when all of it was.
	meters, err := s.shareUsage(account.UID)
	if meters != nil {
		status.Meters = meters
	}
	status.QuotaRead = err == nil

	return OperationResult{
		Operation: "storage.status", Subject: req.Subject, Outcome: "succeeded",
		Result: &status,
	}, http.StatusOK, nil
}

// shareUsage reports what one uid holds in the dataset behind each enabled
// share. A quota table that cannot be read leaves that share out and makes the
// whole an error, with the meters that were read: partial, and said to be.
//
// The dataset is DERIVED from the share's own path rather than configured. A
// configured dataset would be a second definition of where a share lives, and
// the two would disagree the first time somebody moved one — which is exactly
// the failure this codebase keeps paying for. `/mnt/pool/dataset` is the
// mountpoint convention TrueNAS uses for every dataset it creates.
//
// Matched on UID and never on the name in the quota row: that name is null for
// a uid the target can no longer resolve, and matching on it would silently
// report nothing for the accounts most likely to matter.
func (s *server) shareUsage(uid int64) ([]Meter, error) {
	var shares []struct {
		Name    string `json:"name"`
		Path    string `json:"path"`
		Enabled bool   `json:"enabled"`
	}
	if err := s.nas.call("sharing.smb.query",
		[]any{[]any{}, map[string]any{"select": []string{"name", "path", "enabled"}}}, &shares); err != nil {
		return nil, err
	}

	out := make([]Meter, 0, len(shares))
	var unread []string
	for _, sh := range shares {
		if !sh.Enabled || !s.considers(sh.Name) {
			continue
		}
		dataset := datasetOf(sh.Path)
		if dataset == "" {
			continue
		}
		var rows []struct {
			ID        int64 `json:"id"`
			UsedBytes int64 `json:"used_bytes"`
			// shape-optional: TrueNAS omits `quota` entirely for a uid with no
			// quota set, which is the common case and the one every account on
			// the live target is in. Absent decodes to 0, and 0 is rendered as
			// "no limit set" rather than as a full bar — see MyStorage.
			QuotaBytes int64 `json:"quota"`
		}
		if err := s.nas.call("pool.dataset.get_quota", []any{dataset, "USER"}, &rows); err != nil {
			// One share's quota table being unreadable is not a reason to
			// withhold the others, and not one to pass the rest off as all.
			unread = append(unread, sh.Name)
			continue
		}
		for _, row := range rows {
			if row.ID != uid {
				continue
			}
			m := Meter{Label: sh.Name, Used: row.UsedBytes, Unit: "bytes"}
			if row.QuotaBytes > 0 {
				m.Limit = &row.QuotaBytes
			}
			out = append(out, m)
			break
		}
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
