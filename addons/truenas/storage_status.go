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

// StorageStatus is a member's own account, as the target sees it.
type StorageStatus struct {
	Username string `json:"username"`
	// Usable is whether they can actually connect right now. NOT the same as
	// the account existing: an account with password authentication disabled is
	// present, correct, and refuses them.
	Usable bool `json:"usable"`
	// NeedsPassword is the reason, when there is one an action fixes. Setting a
	// password is what turns the account on, so this is an invitation rather
	// than an error.
	NeedsPassword bool `json:"needs_password"`
	// SMBEnabled says share access is on. A member entitled to a share whose
	// account has no password cannot have it yet — TrueNAS refuses SMB on an
	// account with password authentication disabled — so this follows the
	// password rather than the entitlement.
	SMBEnabled bool `json:"smb_enabled"`
	// Shares is what they can see, with usage. Empty is a real answer.
	Shares []ShareUsage `json:"shares"`
	// UsageReadable distinguishes "nothing used" from "could not look". Zero
	// bytes and an unread quota table are the same number otherwise.
	UsageReadable bool `json:"usage_readable"`
}

// ShareUsage is one share and what this member is holding in it.
type ShareUsage struct {
	Share string `json:"share"`
	// UsedBytes is what the target says this uid is using in the dataset behind
	// the share.
	UsedBytes int64 `json:"used_bytes"`
	// QuotaBytes is 0 when no quota is set, which is the common case and is not
	// the same as a quota of zero. A surface must say "no limit set" rather
	// than draw a full bar.
	QuotaBytes int64 `json:"quota_bytes,omitempty"`
}

// storageStatus reads the caller's own account state and usage.
func (s *server) storageStatus(req OperationRequest) (OperationResult, int, error) {
	binding, err := s.boundAccount(req.Subject)
	if err != nil {
		return OperationResult{}, http.StatusUnprocessableEntity, err
	}

	status := StorageStatus{Username: binding.Username, Shares: []ShareUsage{}}

	// Account state from a live read, not the mirror. This is the field a
	// member is about to act on, and a mirror could be six hours old — telling
	// somebody to set a password they set this morning.
	var accounts []struct {
		UID              int64 `json:"uid"`
		SMB              bool  `json:"smb"`
		Locked           bool  `json:"locked"`
		PasswordDisabled bool  `json:"password_disabled"`
	}
	if err := s.nas.call("user.query",
		[]any{[]any{[]any{"username", "=", binding.Username}},
			map[string]any{"select": []string{"uid", "smb", "locked", "password_disabled"}}},
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
	status.SMBEnabled = account.SMB
	status.NeedsPassword = account.PasswordDisabled
	status.Usable = !account.PasswordDisabled && !account.Locked

	// Usage is best-effort: a member who cannot be told their usage can still
	// be told whether their account works, and that is the more urgent half.
	if shares, err := s.shareUsage(account.UID); err == nil {
		status.Shares, status.UsageReadable = shares, true
	}

	return OperationResult{
		Operation: "storage.status", Subject: req.Subject, Outcome: "succeeded",
		Storage: &status,
	}, http.StatusOK, nil
}

// shareUsage reports what one uid holds in the dataset behind each enabled
// share.
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
func (s *server) shareUsage(uid int64) ([]ShareUsage, error) {
	var shares []struct {
		Name    string `json:"name"`
		Path    string `json:"path"`
		Enabled bool   `json:"enabled"`
	}
	if err := s.nas.call("sharing.smb.query",
		[]any{[]any{}, map[string]any{"select": []string{"name", "path", "enabled"}}}, &shares); err != nil {
		return nil, err
	}

	out := make([]ShareUsage, 0, len(shares))
	for _, sh := range shares {
		if !sh.Enabled {
			continue
		}
		dataset := datasetOf(sh.Path)
		if dataset == "" {
			continue
		}
		var rows []struct {
			ID         int64 `json:"id"`
			UsedBytes  int64 `json:"used_bytes"`
			QuotaBytes int64 `json:"quota"`
		}
		if err := s.nas.call("pool.dataset.get_quota", []any{dataset, "USER"}, &rows); err != nil {
			// One share's quota table being unreadable is not a reason to
			// withhold the others.
			continue
		}
		for _, row := range rows {
			if row.ID != uid {
				continue
			}
			out = append(out, ShareUsage{
				Share: sh.Name, UsedBytes: row.UsedBytes, QuotaBytes: row.QuotaBytes,
			})
			break
		}
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
