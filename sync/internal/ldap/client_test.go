package ldap

import (
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
