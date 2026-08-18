package main

import (
	"testing"
)

// A member's account can exist and still refuse them.
//
// Syndra creates the account before any password exists — it has none to set —
// and TrueNAS disables password authentication until the member sets one. So
// "you have an account" and "you can connect" are different facts, and the page
// that showed only the first read as a broken system rather than an unfinished
// step. This is the operation that tells them apart.
func TestAnAccountWithNoPasswordIsReportedAsNotYetUsable(t *testing.T) {
	s, _ := applyServer(t, `[{"username":"ada","id":11,"uid":3001,"locked":false,"smb":false,"password_disabled":true,"groups":[]}]`)
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "ada", UID: 3001}); err != nil {
		t.Fatal(err)
	}

	res, status, err := s.storageStatus(OperationRequest{Subject: "sub-1"})
	if err != nil || status != 200 {
		t.Fatalf("status=%d err=%v", status, err)
	}
	got := res.Storage
	if got == nil {
		t.Fatal("no storage status returned")
	}
	if got.Usable {
		t.Error("an account with password authentication disabled was reported as usable")
	}
	if !got.NeedsPassword {
		t.Error("the member is not told the one action that fixes it")
	}
	if got.Username != "ada" {
		t.Errorf("username = %q", got.Username)
	}
}

// A locked account is not usable either, and locking is an operator's decision
// rather than something the member can act on — so it must not be reported as
// "set a password".
func TestALockedAccountIsNotUsableAndNotTheMembersToFix(t *testing.T) {
	s, _ := applyServer(t, `[{"username":"ada","id":11,"uid":3001,"locked":true,"smb":true,"password_disabled":false,"groups":[]}]`)
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "ada", UID: 3001}); err != nil {
		t.Fatal(err)
	}

	res, _, err := s.storageStatus(OperationRequest{Subject: "sub-1"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Storage.Usable {
		t.Error("a locked account was reported as usable")
	}
	if res.Storage.NeedsPassword {
		t.Error("a locked account told the member to set a password, which would not help")
	}
}

// A binding naming an account the target does not have is said plainly, not
// reported as a usage report of nothing.
func TestAMissingAccountIsRefusedRatherThanReportedEmpty(t *testing.T) {
	s, _ := applyServer(t, `[]`)
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "ghost", UID: 4999}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.storageStatus(OperationRequest{Subject: "sub-1"}); err == nil {
		t.Fatal("a binding onto a missing account produced a status instead of a refusal")
	}
}

// The dataset behind a share is derived from the share's own path, never
// configured. A configured one is a second definition of where a share lives,
// and the two disagree the first time somebody moves one.
func TestTheDatasetIsDerivedFromTheSharePath(t *testing.T) {
	for path, want := range map[string]string{
		"/mnt/pool0/main":                         "pool0/main",
		"/mnt/pool0/application_data/gitlab_data": "pool0/application_data/gitlab_data",
		"/mnt/tank/": "tank",
		// Not a dataset path: skipped rather than guessed at.
		"/some/other/place": "",
		"":                  "",
	} {
		if got := datasetOf(path); got != want {
			t.Errorf("datasetOf(%q) = %q, want %q", path, got, want)
		}
	}
}
