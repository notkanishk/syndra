// Package kinds is the backend's view code for a kind profile (design §4):
// what one kind's pages read from its profile, and nothing the generic
// surfaces (mappings, drift, plans, pending changes) read. The core names no
// target field; a kind's view may name its profile's own words, and only here.
package kinds

import (
	"net/url"
	"strings"

	"syndra/internal/addons"
)

// StorageFields are the storage kind's fields, found by role (design §3): the
// membership whose values the storage profile's reach is keyed by
// (profiles/storage/reach.json `groups`), and the flag that gives a home
// folder. Each is "" unless the target declares exactly one field of that
// role, and the page then says less rather than guess which one is meant.
type StorageFields struct {
	Membership string
	Home       string
}

// Storage reads a target's storage fields from its role map.
func Storage(roles addons.Roles) StorageFields {
	return StorageFields{Membership: onlyOne(roles.Of(addons.RoleMembership)), Home: onlyOne(roles.Of(addons.RoleFlag))}
}

// HomeLimit is the home folder's limit: the target's one `limit` field
// counted in bytes, which the storage profile's usage (`used_bytes`) is
// compared against. Read from the accepted schema the caller holds (saved at
// acceptance, handlers.acceptedShape), because the role map carries no unit;
// false when there is no such field.
func HomeLimit(schema []addons.EntitlementField) (addons.EntitlementField, bool) {
	var found []addons.EntitlementField
	for _, f := range schema {
		if f.Role == addons.RoleLimit && f.Unit == "bytes" {
			found = append(found, f)
		}
	}
	if len(found) != 1 {
		return addons.EntitlementField{}, false
	}
	return found[0], true
}

func onlyOne(names []string) string {
	if len(names) == 1 {
		return names[0]
	}
	return ""
}

// Address fills a place's addresses for the protocol a member connects with,
// the one place an address is built (design §4: keyed on
// `connection.protocol`): smb://host/share/folder for macOS and Linux,
// \\host\share\folder for Windows. A protocol with no builder here leaves both
// empty, and the page then shows the path alone rather than a wrong address.
func Address(c *addons.Connection, p *addons.Place) {
	if c == nil || c.Protocol != "smb" || c.Host == "" {
		return
	}
	segments := []string{p.Share}
	if p.Folder != "" {
		segments = append(segments, strings.Split(p.Folder, "/")...)
	}
	escaped := make([]string, len(segments))
	for i, s := range segments {
		escaped[i] = url.PathEscape(s)
	}
	p.SMBAddress = "smb://" + c.Host + "/" + strings.Join(escaped, "/")
	p.UNCAddress = `\\` + c.Host + `\` + strings.Join(segments, `\`)
}
