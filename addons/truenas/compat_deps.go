package main

import "syndra/addons/truenas/compat"

// deps is what this add-on relies on in each TrueNAS method it calls: the one
// table the version check compares (design-version-check.md §3, §4.2). Reads
// are in readMethods; whether a method is a read is said there only. The
// check's own reads (system.version, auth.me, core.get_methods) are not here:
// they run before there is a report.
//
// Replies are the Go types the call sites decode into, so the relied-on paths
// are derived from their json tags, never listed. ponytail: a read whose call
// sites decode only into inline anonymous structs lists none yet; slice 9e
// names those types and registers them here (design §5).
var deps = map[string]compat.Dep{
	"alert.list":             {},
	"audit.query":            {Replies: []any{[]auditRow{}}},
	"filesystem.getacl":      {},
	"filesystem.listdir":     {},
	"filesystem.stat":        {},
	"group.query":            {Replies: []any{[]nasGroup{}}},
	"pool.dataset.get_quota": {},
	"pool.dataset.query":     {},
	"pool.query":             {},
	"service.query":          {},
	"sharing.smb.query":      {Replies: []any{[]smbShare{}}},
	"system.info":            {},
	"user.query":             {Replies: []any{[]nasUser{}}},

	// Writes. Sends is every field any call site sends (apply.go, home.go,
	// identity.go, operations.go).
	"group.create": {Replies: []any{apiID{}}, Sends: []string{"/0/name", "/0/gid", "/0/smb"}},
	"group.delete": {Sends: []string{"/0"}},
	"pool.dataset.set_quota": {Sends: []string{
		"/0", "/1/0/quota_type", "/1/0/id", "/1/0/quota_value",
	}},
	"user.create": {Replies: []any{apiID{}}, Sends: []string{
		"/0/username", "/0/full_name", "/0/shell", "/0/password_disabled", "/0/smb", "/0/locked",
		"/0/groups", "/0/uid", "/0/group", "/0/group_create", "/0/home", "/0/home_create", "/0/home_mode",
	}},
	"user.update": {Sends: []string{
		"/0", "/1/groups", "/1/locked", "/1/smb", "/1/home", "/1/home_create", "/1/home_mode",
		"/1/password", "/1/password_disabled",
	}},
}

// elevatedDeps is checked only on the purge's elevated session: the add-on's
// own key may not delete, so the NAS never lists user.delete to it, and in
// deps it would read missing on every version (design §3.1).
var elevatedDeps = map[string]compat.Dep{
	"user.delete": {Sends: []string{"/0"}},
}
