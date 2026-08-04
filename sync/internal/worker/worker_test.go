package worker

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"syndra-sync/internal/backend"
	ldapclient "syndra-sync/internal/ldap"

	ldapv3 "github.com/go-ldap/ldap/v3"
)

// ---------------------------------------------------------------------------
// Mock implementations
// ---------------------------------------------------------------------------

type mockBackend struct {
	claimFn       func(ctx context.Context, limit int) ([]backend.ProvisioningIntent, error)
	completeFn    func(ctx context.Context, id string) error
	failFn        func(ctx context.Context, id, msg string) error
	shadowFn      func(ctx context.Context, uid string) (string, error)
	profileFn     func(ctx context.Context, uid string) (backend.UserProfile, error)
	completeCalls atomic.Int32
	failCalls     atomic.Int32
}

func (m *mockBackend) ClaimIntents(ctx context.Context, limit int) ([]backend.ProvisioningIntent, error) {
	if m.claimFn != nil {
		return m.claimFn(ctx, limit)
	}
	return nil, nil
}
func (m *mockBackend) CompleteIntent(ctx context.Context, id string) error {
	m.completeCalls.Add(1)
	if m.completeFn != nil {
		return m.completeFn(ctx, id)
	}
	return nil
}
func (m *mockBackend) FailIntent(ctx context.Context, id, msg string) error {
	m.failCalls.Add(1)
	if m.failFn != nil {
		return m.failFn(ctx, id, msg)
	}
	return nil
}
func (m *mockBackend) GetShadowCredentialHash(ctx context.Context, uid string) (string, error) {
	if m.shadowFn != nil {
		return m.shadowFn(ctx, uid)
	}
	return "", nil // No shadow credential by default.
}
func (m *mockBackend) GetUserProfile(ctx context.Context, uid string) (backend.UserProfile, error) {
	if m.profileFn != nil {
		return m.profileFn(ctx, uid)
	}
	return backend.UserProfile{UserID: uid, DisplayName: uid, Email: uid + "@test.com"}, nil
}

type mockLDAP struct {
	ensureUserFn  func(ctx context.Context, uid, dn, email string) error
	ensureGroupFn func(ctx context.Context, group string) error
	addFn         func(ctx context.Context, uid, group string) error
	removeFn      func(ctx context.Context, uid, group string) error
	setPassFn     func(ctx context.Context, uid, hash string) error
	addCalls      atomic.Int32
	removeCalls   atomic.Int32
	setPassCalls  atomic.Int32
}

func (m *mockLDAP) EnsureUser(ctx context.Context, uid, dn, email string) error {
	if m.ensureUserFn != nil {
		return m.ensureUserFn(ctx, uid, dn, email)
	}
	return nil
}
func (m *mockLDAP) EnsureGroup(ctx context.Context, group string) error {
	if m.ensureGroupFn != nil {
		return m.ensureGroupFn(ctx, group)
	}
	return nil
}
func (m *mockLDAP) AddUserToGroup(ctx context.Context, uid, group string) error {
	m.addCalls.Add(1)
	if m.addFn != nil {
		return m.addFn(ctx, uid, group)
	}
	return nil
}
func (m *mockLDAP) RemoveUserFromGroup(ctx context.Context, uid, group string) error {
	m.removeCalls.Add(1)
	if m.removeFn != nil {
		return m.removeFn(ctx, uid, group)
	}
	return nil
}
func (m *mockLDAP) SetUserPassword(ctx context.Context, uid, hash string) error {
	m.setPassCalls.Add(1)
	if m.setPassFn != nil {
		return m.setPassFn(ctx, uid, hash)
	}
	return nil
}

func testConfig() Config {
	return Config{
		PollInterval:  100 * time.Millisecond,
		WorkerCount:   2,
		IntentLimit:   10,
		RetryAttempts: 2,
		RetryBackoff:  1 * time.Millisecond,
	}
}

func testIntent(action string) backend.ProvisioningIntent {
	return backend.ProvisioningIntent{
		ID:         "int-1",
		TargetUID:  "u1",
		Action:     action,
		LLDAPGroup: "printing_member",
	}
}

// ---------------------------------------------------------------------------
// processIntent tests
// ---------------------------------------------------------------------------

func TestProcessIntent_AddSuccess(t *testing.T) {
	bc := &mockBackend{}
	lp := &mockLDAP{}
	locker := NewUIDLocker()

	processIntent(context.Background(), testIntent("add"), bc, lp, locker, testConfig())

	if lp.addCalls.Load() != 1 {
		t.Errorf("expected 1 add call, got %d", lp.addCalls.Load())
	}
	if bc.completeCalls.Load() != 1 {
		t.Errorf("expected 1 complete call, got %d", bc.completeCalls.Load())
	}
	if bc.failCalls.Load() != 0 {
		t.Errorf("expected 0 fail calls, got %d", bc.failCalls.Load())
	}
}

func TestProcessIntent_RemoveSuccess(t *testing.T) {
	bc := &mockBackend{}
	lp := &mockLDAP{}
	locker := NewUIDLocker()

	processIntent(context.Background(), testIntent("remove"), bc, lp, locker, testConfig())

	if lp.removeCalls.Load() != 1 {
		t.Errorf("expected 1 remove call, got %d", lp.removeCalls.Load())
	}
	if bc.completeCalls.Load() != 1 {
		t.Errorf("expected 1 complete call, got %d", bc.completeCalls.Load())
	}
}

func TestProcessIntent_LDAPPermanentFailure(t *testing.T) {
	bc := &mockBackend{}
	lp := &mockLDAP{
		addFn: func(_ context.Context, _, _ string) error {
			return ldapv3.NewError(ldapv3.LDAPResultNoSuchObject, fmt.Errorf("user not found in LLDAP"))
		},
	}
	locker := NewUIDLocker()

	processIntent(context.Background(), testIntent("add"), bc, lp, locker, testConfig())

	if bc.failCalls.Load() != 1 {
		t.Errorf("expected 1 fail call for permanent error, got %d", bc.failCalls.Load())
	}
	if bc.completeCalls.Load() != 0 {
		t.Errorf("expected 0 complete calls, got %d", bc.completeCalls.Load())
	}
}

func TestProcessIntent_LDAPTransientRetryThenSuccess(t *testing.T) {
	var attempts atomic.Int32
	bc := &mockBackend{}
	lp := &mockLDAP{
		addFn: func(_ context.Context, _, _ string) error {
			if attempts.Add(1) == 1 {
				return ldapv3.NewError(ldapv3.LDAPResultServerDown, fmt.Errorf("connection lost"))
			}
			return nil
		},
	}
	locker := NewUIDLocker()

	processIntent(context.Background(), testIntent("add"), bc, lp, locker, testConfig())

	if bc.completeCalls.Load() != 1 {
		t.Errorf("expected intent to complete after retry, got %d complete calls", bc.completeCalls.Load())
	}
}

func TestProcessIntent_ShadowPasswordSync(t *testing.T) {
	bc := &mockBackend{
		shadowFn: func(_ context.Context, _ string) (string, error) {
			return "$argon2id$hash", nil
		},
	}
	lp := &mockLDAP{}
	locker := NewUIDLocker()

	processIntent(context.Background(), testIntent("add"), bc, lp, locker, testConfig())

	if lp.setPassCalls.Load() != 1 {
		t.Errorf("expected 1 SetUserPassword call, got %d", lp.setPassCalls.Load())
	}
}

func TestProcessIntent_ShadowPasswordAbsent(t *testing.T) {
	bc := &mockBackend{} // Default: returns empty hash.
	lp := &mockLDAP{}
	locker := NewUIDLocker()

	processIntent(context.Background(), testIntent("add"), bc, lp, locker, testConfig())

	if lp.setPassCalls.Load() != 0 {
		t.Errorf("expected 0 SetUserPassword calls when no shadow cred, got %d", lp.setPassCalls.Load())
	}
}

func TestRetryTransient_PermanentError(t *testing.T) {
	calls := 0
	cfg := testConfig()
	err := retryTransient(context.Background(), cfg, func() error {
		calls++
		return ldapv3.NewError(ldapv3.LDAPResultNoSuchObject, fmt.Errorf("not found"))
	})
	if err == nil {
		t.Fatal("expected permanent error")
	}
	if calls != 1 {
		t.Errorf("expected 1 call (no retry for permanent), got %d", calls)
	}
}

func TestRetryTransient_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Already cancelled.

	cfg := testConfig()
	err := retryTransient(ctx, cfg, func() error {
		return ldapclient.NewConnectionError("server down")
	})
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestPoll_ClaimsAndDispatches(t *testing.T) {
	bc := &mockBackend{
		claimFn: func(_ context.Context, _ int) ([]backend.ProvisioningIntent, error) {
			return []backend.ProvisioningIntent{testIntent("add")}, nil
		},
	}
	ch := make(chan backend.ProvisioningIntent, 10)
	poll(context.Background(), bc, ch, 50)

	if len(ch) != 1 {
		t.Errorf("expected 1 intent in channel, got %d", len(ch))
	}
}
