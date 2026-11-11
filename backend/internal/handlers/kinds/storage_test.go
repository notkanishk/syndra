package kinds

import (
	"testing"

	"syndra/internal/addons"
)

// The address is built by protocol (design §4): smb gets both forms, escaped
// for the URL and literal for Windows; any other protocol, or no connection,
// gets none rather than a wrong one.
func TestAddressIsBuiltByProtocol(t *testing.T) {
	p := addons.Place{Share: "main", Folder: "projects/3d renders"}
	Address(&addons.Connection{Protocol: "smb", Host: "nas.example.org"}, &p)
	if p.SMBAddress != "smb://nas.example.org/main/projects/3d%20renders" || p.UNCAddress != `\\nas.example.org\main\projects\3d renders` {
		t.Fatalf("smb: %q %q", p.SMBAddress, p.UNCAddress)
	}
	for _, c := range []*addons.Connection{nil, {Protocol: "nfs", Host: "nas.example.org"}, {Protocol: "smb"}} {
		q := addons.Place{Share: "main"}
		Address(c, &q)
		if q.SMBAddress != "" || q.UNCAddress != "" {
			t.Errorf("%+v built %q %q", c, q.SMBAddress, q.UNCAddress)
		}
	}
}

// The storage view finds its fields by role, never by name, and refuses to
// guess when a role is declared twice.
func TestStorageFieldsAreFoundByRole(t *testing.T) {
	got := Storage(addons.Roles{"teams": addons.RoleMembership, "folder": addons.RoleFlag, "active": addons.RoleAccount})
	if got.Membership != "teams" || got.Home != "folder" {
		t.Fatalf("%+v", got)
	}
	if got := Storage(addons.Roles{"a": addons.RoleMembership, "b": addons.RoleMembership}); got.Membership != "" {
		t.Fatalf("two memberships must name none: %+v", got)
	}
}
