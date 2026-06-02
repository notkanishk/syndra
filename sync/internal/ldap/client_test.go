package ldap

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"

	ldapv3 "github.com/go-ldap/ldap/v3"
)

func TestUserDN(t *testing.T) {
	p := &Pool{cfg: Config{BaseDN: "dc=example,dc=com"}}
	dn := p.UserDN("alice")
	expected := "uid=alice,ou=people,dc=example,dc=com"
	if dn != expected {
		t.Errorf("expected %s, got %s", expected, dn)
	}
}

func TestGroupDN(t *testing.T) {
	p := &Pool{cfg: Config{BaseDN: "dc=example,dc=com"}}
	dn := p.GroupDN("printing_member")
	expected := "cn=printing_member,ou=groups,dc=example,dc=com"
	if dn != expected {
		t.Errorf("expected %s, got %s", expected, dn)
	}
}

func TestUserDN_SpecialCharacters(t *testing.T) {
	p := &Pool{cfg: Config{BaseDN: "dc=example,dc=com"}}
	dn := p.UserDN("user+special,chars")
	// EscapeDN should escape special LDAP characters.
	if dn == "uid=user+special,chars,ou=people,dc=example,dc=com" {
		t.Error("special characters should be escaped in DN")
	}
	// The DN should still contain the base.
	if len(dn) < 30 {
		t.Errorf("DN seems too short: %s", dn)
	}
}

func TestIsConnectionError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "server down",
			err:      ldapv3.NewError(ldapv3.LDAPResultServerDown, fmt.Errorf("connection lost")),
			expected: true,
		},
		{
			name:     "busy",
			err:      ldapv3.NewError(ldapv3.LDAPResultBusy, fmt.Errorf("server busy")),
			expected: true,
		},
		{
			name:     "unavailable",
			err:      ldapv3.NewError(ldapv3.LDAPResultUnavailable, fmt.Errorf("unavailable")),
			expected: true,
		},
		{
			name:     "no such object (permanent)",
			err:      ldapv3.NewError(ldapv3.LDAPResultNoSuchObject, fmt.Errorf("not found")),
			expected: false,
		},
		{
			name:     "insufficient access (permanent)",
			err:      ldapv3.NewError(ldapv3.LDAPResultInsufficientAccessRights, fmt.Errorf("denied")),
			expected: false,
		},
		{
			name:     "network error",
			err:      &net.OpError{Op: "dial", Err: fmt.Errorf("connection refused")},
			expected: true,
		},
		{
			name:     "generic error",
			err:      fmt.Errorf("something else"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsConnectionError(tt.err); got != tt.expected {
				t.Errorf("IsConnectionError(%v) = %v, want %v", tt.err, got, tt.expected)
			}
		})
	}
}

func TestNewGroupAddRequest_UsesBindDNAsPlaceholderMember(t *testing.T) {
	p := &Pool{cfg: Config{
		BaseDN: "dc=example,dc=com",
		BindDN: "uid=admin,ou=people,dc=example,dc=com",
	}}
	req := p.newGroupAddRequest("samba_share_admin")

	var memberVals []string
	var objectClassVals []string
	for _, attr := range req.Attributes {
		switch attr.Type {
		case "member":
			memberVals = attr.Vals
		case "objectClass":
			objectClassVals = attr.Vals
		}
	}

	if len(memberVals) != 1 {
		t.Fatalf("expected exactly one member value, got %d: %v", len(memberVals), memberVals)
	}
	if memberVals[0] != "uid=admin,ou=people,dc=example,dc=com" {
		t.Errorf("expected member=[bindDN], got %q", memberVals[0])
	}
	if memberVals[0] == "" {
		t.Error("placeholder must not be an empty DN — strict OpenLDAP rejects it")
	}
	if len(objectClassVals) != 1 || objectClassVals[0] != "groupOfNames" {
		t.Errorf("expected objectClass=[groupOfNames], got %v", objectClassVals)
	}
}

func TestNewRemoveMemberRequest_DeletesBindDNPlaceholder(t *testing.T) {
	p := &Pool{cfg: Config{
		BaseDN: "dc=example,dc=com",
		BindDN: "uid=admin,ou=people,dc=example,dc=com",
	}}
	req := p.newRemoveMemberRequest("samba_share_admin", p.cfg.BindDN)

	if req.DN != "cn=samba_share_admin,ou=groups,dc=example,dc=com" {
		t.Errorf("unexpected group DN: %q", req.DN)
	}

	var found bool
	for _, change := range req.Changes {
		if change.Operation != ldapv3.DeleteAttribute {
			t.Errorf("expected a delete operation, got %d", change.Operation)
		}
		if change.Modification.Type == "member" {
			found = true
			if len(change.Modification.Vals) != 1 || change.Modification.Vals[0] != p.cfg.BindDN {
				t.Errorf("expected member delete=[bindDN], got %v", change.Modification.Vals)
			}
		}
	}
	if !found {
		t.Error("expected a delete on the member attribute")
	}
}

func TestWithConn_CancelledCtxFailsFast(t *testing.T) {
	p := &Pool{} // conn is nil; fn must not be invoked
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before calling withConn

	fnInvoked := false
	err := p.withConn(ctx, func(c *ldapv3.Conn) error {
		fnInvoked = true
		return nil
	})

	if fnInvoked {
		t.Error("fn must not be invoked when ctx is already cancelled")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}
