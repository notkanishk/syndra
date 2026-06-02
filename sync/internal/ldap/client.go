package ldap

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"sync"

	ldapv3 "github.com/go-ldap/ldap/v3"
)

// Config holds LLDAP connection parameters.
type Config struct {
	URL                string
	BindDN             string
	BindPassword       string
	BaseDN             string
	InsecureSkipVerify bool
}

// Pool manages a single LLDAP connection with auto-reconnect.
// A mutex serializes all LDAP operations and reconnection attempts.
type Pool struct {
	cfg  Config
	conn *ldapv3.Conn
	mu   sync.Mutex
}

// Connect dials the LLDAP server and binds with the service account.
func Connect(cfg Config) (*Pool, error) {
	p := &Pool{cfg: cfg}
	if err := p.connect(); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *Pool) connect() error {
	conn, err := ldapv3.DialURL(p.cfg.URL, ldapv3.DialWithTLSConfig(&tls.Config{
		InsecureSkipVerify: p.cfg.InsecureSkipVerify,
	}))
	if err != nil {
		return fmt.Errorf("dial LLDAP %s: %w", p.cfg.URL, err)
	}
	if err := conn.Bind(p.cfg.BindDN, p.cfg.BindPassword); err != nil {
		conn.Close()
		return fmt.Errorf("bind as %s: %w", p.cfg.BindDN, err)
	}
	p.conn = conn
	return nil
}

func (p *Pool) reconnect() error {
	if p.conn != nil {
		p.conn.Close()
	}
	log.Println("[LDAP] Reconnecting...")
	return p.connect()
}

// Close unbinds and closes the connection.
func (p *Pool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.conn != nil {
		p.conn.Close()
		p.conn = nil
	}
}

// withConn executes fn with the current connection. Context cancellation
// is honored at LDAP-operation boundaries: ctx.Err() is checked before
// acquiring the pool mutex, after acquiring it, and before any reconnect
// retry. Once fn is running, it owns the LDAP call to completion (or the
// underlying conn's timeout). If fn returns a connection error, reconnect
// is attempted once and fn is retried.
func (p *Pool) withConn(ctx context.Context, fn func(*ldapv3.Conn) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}

	err := fn(p.conn)
	if err != nil && IsConnectionError(err) {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if reconnErr := p.reconnect(); reconnErr != nil {
			return fmt.Errorf("reconnect failed: %w (original: %v)", reconnErr, err)
		}
		return fn(p.conn) // retry once
	}
	return err
}

// UserDN builds the distinguished name for a user.
func (p *Pool) UserDN(uid string) string {
	return fmt.Sprintf("uid=%s,ou=people,%s", ldapv3.EscapeDN(uid), p.cfg.BaseDN)
}

// GroupDN builds the distinguished name for a group.
func (p *Pool) GroupDN(group string) string {
	return fmt.Sprintf("cn=%s,ou=groups,%s", ldapv3.EscapeDN(group), p.cfg.BaseDN)
}

// EnsureUser searches for a user by uid. If absent, creates with displayName and mail.
// If present, updates displayName and mail if they differ.
func (p *Pool) EnsureUser(ctx context.Context, uid, displayName, email string) error {
	return p.withConn(ctx, func(conn *ldapv3.Conn) error {
		userDN := p.UserDN(uid)

		// Search for existing user.
		sr, err := conn.Search(ldapv3.NewSearchRequest(
			userDN, ldapv3.ScopeBaseObject, ldapv3.NeverDerefAliases, 1, 0, false,
			"(objectClass=*)", []string{"displayName", "mail"}, nil,
		))
		if err != nil {
			if ldapv3.IsErrorWithCode(err, ldapv3.LDAPResultNoSuchObject) {
				// User does not exist — create.
				addReq := ldapv3.NewAddRequest(userDN, nil)
				addReq.Attribute("objectClass", []string{"inetOrgPerson"})
				addReq.Attribute("uid", []string{uid})
				addReq.Attribute("cn", []string{displayName})
				addReq.Attribute("sn", []string{uid}) // sn is required for inetOrgPerson
				if displayName != "" {
					addReq.Attribute("displayName", []string{displayName})
				}
				if email != "" {
					addReq.Attribute("mail", []string{email})
				}
				if err := conn.Add(addReq); err != nil {
					return fmt.Errorf("create LLDAP user %s: %w", uid, err)
				}
				log.Printf("[LDAP] Created user: uid=%s displayName=%s email=%s", uid, displayName, email)
				return nil
			}
			return fmt.Errorf("search user %s: %w", uid, err)
		}

		// User exists — update displayName and mail if needed.
		if len(sr.Entries) > 0 {
			entry := sr.Entries[0]
			currentDisplay := entry.GetAttributeValue("displayName")
			currentMail := entry.GetAttributeValue("mail")
			if currentDisplay != displayName || currentMail != email {
				modReq := ldapv3.NewModifyRequest(userDN, nil)
				if displayName != "" {
					modReq.Replace("displayName", []string{displayName})
				}
				if email != "" {
					modReq.Replace("mail", []string{email})
				}
				if err := conn.Modify(modReq); err != nil {
					log.Printf("[LDAP] Warning: failed to update user attrs for %s: %v", uid, err)
				}
			}
		}
		return nil
	})
}

// newGroupAddRequest builds the AddRequest used by EnsureGroup. Extracted
// so the request shape is unit-testable without a fake LDAP connection.
// `groupOfNames` requires at least one member; we use the bind DN (a real
// principal in the directory) as the placeholder so strict OpenLDAP
// accepts it. AddUserToGroup prunes this placeholder once a real member
// exists (see newRemoveMemberRequest) so the service account does not
// linger as a member of every group.
func (p *Pool) newGroupAddRequest(group string) *ldapv3.AddRequest {
	addReq := ldapv3.NewAddRequest(p.GroupDN(group), nil)
	addReq.Attribute("objectClass", []string{"groupOfNames"})
	addReq.Attribute("cn", []string{group})
	addReq.Attribute("member", []string{p.cfg.BindDN})
	return addReq
}

// newRemoveMemberRequest builds the ModifyRequest that deletes a single member
// DN from a group. Extracted so the request shape is unit-testable without a
// fake LDAP connection. Used to prune the bind-DN placeholder seeded by
// newGroupAddRequest.
func (p *Pool) newRemoveMemberRequest(group, memberDN string) *ldapv3.ModifyRequest {
	modReq := ldapv3.NewModifyRequest(p.GroupDN(group), nil)
	modReq.Delete("member", []string{memberDN})
	return modReq
}

// EnsureGroup searches for a group by cn. If absent, creates it.
func (p *Pool) EnsureGroup(ctx context.Context, group string) error {
	return p.withConn(ctx, func(conn *ldapv3.Conn) error {
		groupDN := p.GroupDN(group)

		_, err := conn.Search(ldapv3.NewSearchRequest(
			groupDN, ldapv3.ScopeBaseObject, ldapv3.NeverDerefAliases, 1, 0, false,
			"(objectClass=*)", []string{"cn"}, nil,
		))
		if err != nil {
			if ldapv3.IsErrorWithCode(err, ldapv3.LDAPResultNoSuchObject) {
				if err := conn.Add(p.newGroupAddRequest(group)); err != nil {
					return fmt.Errorf("create LLDAP group %s: %w", group, err)
				}
				log.Printf("[LDAP] Created group: cn=%s", group)
				return nil
			}
			return fmt.Errorf("search group %s: %w", group, err)
		}
		return nil
	})
}

// AddUserToGroup adds a user's DN to the group's member attribute.
// Idempotent: ignores AttributeOrValueExists (code 20). Once a real member is
// present, it prunes the bind-DN placeholder that EnsureGroup seeds to satisfy
// groupOfNames' "at least one member" rule — otherwise the service account
// would linger as a member of every group it creates.
func (p *Pool) AddUserToGroup(ctx context.Context, targetUID, lldapGroup string) error {
	return p.withConn(ctx, func(conn *ldapv3.Conn) error {
		groupDN := p.GroupDN(lldapGroup)
		userDN := p.UserDN(targetUID)

		modReq := ldapv3.NewModifyRequest(groupDN, nil)
		modReq.Add("member", []string{userDN})
		if err := conn.Modify(modReq); err != nil &&
			!ldapv3.IsErrorWithCode(err, ldapv3.LDAPResultAttributeOrValueExists) {
			// AttributeOrValueExists means already a member — fall through to
			// placeholder pruning, which may still be needed.
			return fmt.Errorf("add %s to group %s: %w", targetUID, lldapGroup, err)
		}

		// Prune the bind-DN placeholder now that a real member exists. The user
		// we just added keeps the group non-empty, so removing the bind DN is
		// safe. Best-effort and idempotent: NoSuchAttribute means it was already
		// pruned (or the group predates this logic). A real user DN never equals
		// the bind DN; the guard prevents removing the last member regardless.
		if userDN != p.cfg.BindDN {
			if err := conn.Modify(p.newRemoveMemberRequest(lldapGroup, p.cfg.BindDN)); err != nil &&
				!ldapv3.IsErrorWithCode(err, ldapv3.LDAPResultNoSuchAttribute) {
				// The real member was added; a lingering placeholder is cosmetic.
				// Log and continue rather than failing the whole sync op.
				log.Printf("[LDAP] Warning: could not prune placeholder member from group %s: %v", lldapGroup, err)
			}
		}
		return nil
	})
}

// RemoveUserFromGroup removes a user's DN from the group's member attribute.
// Idempotent: ignores NoSuchAttribute (code 16).
func (p *Pool) RemoveUserFromGroup(ctx context.Context, targetUID, lldapGroup string) error {
	return p.withConn(ctx, func(conn *ldapv3.Conn) error {
		groupDN := p.GroupDN(lldapGroup)
		userDN := p.UserDN(targetUID)

		modReq := ldapv3.NewModifyRequest(groupDN, nil)
		modReq.Delete("member", []string{userDN})
		if err := conn.Modify(modReq); err != nil {
			if ldapv3.IsErrorWithCode(err, ldapv3.LDAPResultNoSuchAttribute) {
				return nil // Not a member — idempotent success.
			}
			return fmt.Errorf("remove %s from group %s: %w", targetUID, lldapGroup, err)
		}
		return nil
	})
}

// SetUserPassword sets the userPassword attribute to a pre-hashed value.
func (p *Pool) SetUserPassword(ctx context.Context, targetUID, passwordHash string) error {
	return p.withConn(ctx, func(conn *ldapv3.Conn) error {
		userDN := p.UserDN(targetUID)
		modReq := ldapv3.NewModifyRequest(userDN, nil)
		modReq.Replace("userPassword", []string{passwordHash})
		if err := conn.Modify(modReq); err != nil {
			return fmt.Errorf("set password for %s: %w", targetUID, err)
		}
		return nil
	})
}

// NewConnectionError creates a transient LDAP server-down error (for testing).
func NewConnectionError(msg string) error {
	return ldapv3.NewError(ldapv3.LDAPResultServerDown, fmt.Errorf("%s", msg))
}

// IsConnectionError returns true for transient LDAP errors worth reconnecting for.
func IsConnectionError(err error) bool {
	if ldapv3.IsErrorWithCode(err, ldapv3.LDAPResultServerDown) ||
		ldapv3.IsErrorWithCode(err, ldapv3.LDAPResultBusy) ||
		ldapv3.IsErrorWithCode(err, ldapv3.LDAPResultUnavailable) {
		return true
	}
	// Check for underlying network errors.
	if _, ok := err.(*net.OpError); ok {
		return true
	}
	return false
}
