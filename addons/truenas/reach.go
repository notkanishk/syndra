package main

import (
	"errors"
	"fmt"
	"net/http"
	"path"
	"sort"
	"strings"
	"sync"
	"time"
)

// Where a person can actually connect, read from the NAS's own folder ACLs.
//
// A mapping says which group someone is in; only the ACL says what that group
// opens. Deriving addresses from group names (`smb://host/<group>`) assumed a
// share per group, and on the live NAS no share is named after any group. A
// second copy of "who opens what" kept in Syndra would be a second definition
// that drifts, so the NAS is read instead.
//
// ponytail: one level deep, each enabled share's root and its first-level
// folders. That matches a layout where each first-level folder is its own
// dataset; a grant deeper down shows as the folder above it. Read a level
// further if a share ever grants below its first level.

// reachMaxAge is how long one read of the ACLs is served before it is read
// again. A dozen ACL reads per page view would put the NAS's latency in front
// of every member, and ACLs change by hand, rarely.
const reachMaxAge = 5 * time.Minute

const (
	accessNone   = ""
	accessRead   = "read"
	accessChange = "change"
)

// Place is somewhere a person can connect, and what they can do there.
type Place struct {
	Share string `json:"share"`
	// Folder is the path inside the share, empty for the share's own root.
	Folder string `json:"folder,omitempty"`
	// Path is the absolute path on the NAS.
	Path string `json:"path"`
	// Access is "read" or "change". A place someone can only traverse is not a
	// place.
	Access string `json:"access"`
	// Dataset holds the place, the one whose user quota table says what someone
	// has saved there.
	Dataset string `json:"dataset,omitempty"`
	// Home marks the member's own home folder.
	Home bool `json:"home,omitempty"`
	// Usage is what the member has saved there. Only on a member's own places
	// they can change, and only when the quota table could be read; a
	// read-only place has nothing of theirs to count.
	Usage *PlaceUsage `json:"usage,omitempty"`
}

// PlaceUsage is one member's use of one place. UsedBytes 0 is "nothing saved
// here yet", a real answer.
type PlaceUsage struct {
	UsedBytes  int64 `json:"used_bytes"`
	QuotaBytes int64 `json:"quota_bytes,omitempty"`
}

// aclEntry is one NFS4 ACE, read unsimplified so every permission is explicit.
type aclEntry struct {
	Tag   string          `json:"tag"`
	ID    int64           `json:"id"`
	Type  string          `json:"type"`
	Perms map[string]bool `json:"perms"`
	Flags map[string]bool `json:"flags"`
}

// aclPlace is one folder as read: where it is, and its ACL.
type aclPlace struct {
	share, folder, path, dataset string
	ownerGID                     int64
	acl                          []aclEntry
}

// principals is who an evaluation is for: the groups an account is in, by gid.
// everyone@ applies to every account, so it is always counted.
type principals map[int64]bool

// allowed is NFS4's own rule: the entries are read in order, and the first one
// that applies to the reader and names the permission decides it.
func (p aclPlace) allowed(who principals, perm string) bool {
	for _, e := range p.acl {
		if e.Flags["INHERIT_ONLY"] || !e.Perms[perm] {
			continue
		}
		if e.Tag == "everyone@" || (e.Tag == "GROUP" && who[e.ID]) || (e.Tag == "group@" && who[p.ownerGID]) {
			return e.Type == "ALLOW"
		}
	}
	return false
}

func (p aclPlace) access(who principals) string {
	switch {
	case !p.allowed(who, "READ_DATA"):
		return accessNone
	case p.allowed(who, "WRITE_DATA"):
		return accessChange
	default:
		return accessRead
	}
}

func accessRank(a string) int {
	switch a {
	case accessChange:
		return 2
	case accessRead:
		return 1
	}
	return 0
}

// placesFor is everywhere `who` can connect, and what they can do there. A
// share's root is listed when it opens; a folder inside it only when it gives
// more than the root does, since what the root opens is reached through it.
// A folder behind a root `who` can't traverse is out of reach.
//
// beyond, when set, is a baseline to report above: an operator asking what a
// group opens wants what holding it ADDS, not what every account has anyway.
func placesFor(read []aclPlace, who, beyond principals) []Place {
	out := []Place{}
	roots := map[string]aclPlace{}
	for _, p := range read {
		if p.folder == "" {
			roots[p.share] = p
		}
	}
	for _, p := range read {
		got := p.access(who)
		floor := 0
		if beyond != nil {
			floor = accessRank(p.access(beyond))
		}
		if p.folder != "" {
			root := roots[p.share]
			if !root.allowed(who, "EXECUTE") {
				continue
			}
			if r := accessRank(root.access(who)); r > floor {
				floor = r
			}
		}
		if accessRank(got) <= floor {
			continue
		}
		out = append(out, Place{Share: p.share, Folder: p.folder, Path: p.path, Access: got, Dataset: p.dataset})
	}
	sortPlaces(out)
	return out
}

func sortPlaces(ps []Place) {
	sort.Slice(ps, func(i, j int) bool {
		if ps[i].Home != ps[j].Home {
			return ps[i].Home
		}
		if ps[i].Share != ps[j].Share {
			return ps[i].Share < ps[j].Share
		}
		return ps[i].Folder < ps[j].Folder
	})
}

// shareRead is one read of where people connect: the enabled shares, the
// datasets that hold them, and the ACLs at each share's root and first-level
// folders.
type shareRead struct {
	places []aclPlace
	shares []smbShare
	mounts map[string]string
}

// reachCache is the last shareRead.
type reachCache struct {
	mu     sync.Mutex
	readAt time.Time
	last   shareRead
}

// errNeverRead is a NAS that can't be read now and never has been: there is
// no last-known answer to fall back on.
var errNeverRead = errors.New("the folder permissions have not been read yet")

// readACLs serves the last read while it is fresh, and reads again when it
// isn't. A failed read falls back to the last one, reported as not current.
func (s *server) readACLs() (read shareRead, readAt time.Time, current bool, err error) {
	c := &s.reach
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.readAt.IsZero() && time.Since(c.readAt) < reachMaxAge {
		return c.last, c.readAt, true, nil
	}
	fresh, err := s.readSharesACLs()
	if err != nil {
		if c.readAt.IsZero() {
			return shareRead{}, time.Time{}, false, fmt.Errorf("%w: %w", errNeverRead, err)
		}
		return c.last, c.readAt, false, nil
	}
	c.last, c.readAt = fresh, time.Now().UTC()
	return c.last, c.readAt, true, nil
}

// readSharesACLs reads each enabled share's root and first-level folders.
func (s *server) readSharesACLs() (shareRead, error) {
	shares, err := s.enabledShares()
	if err != nil {
		return shareRead{}, err
	}
	datasets, err := s.datasetMounts()
	if err != nil {
		return shareRead{}, err
	}
	out := []aclPlace{}
	for _, sh := range shares {
		root, err := s.readACL(sh.Path)
		if err != nil {
			return shareRead{}, err
		}
		root.share, root.dataset = sh.Name, datasetAt(datasets, sh.Path)
		out = append(out, root)

		var dirs []struct {
			Name string `json:"name"`
			Path string `json:"path"`
		}
		if err := s.nas.call("filesystem.listdir", []any{sh.Path,
			[]any{[]any{"type", "=", "DIRECTORY"}}, map[string]any{"select": []string{"name", "path"}}}, &dirs); err != nil {
			return shareRead{}, fmt.Errorf("list %s: %w", sh.Name, err)
		}
		for _, d := range dirs {
			p, err := s.readACL(d.Path)
			if errors.Is(err, ErrNoSuchPath) {
				continue // removed between the listing and the read
			}
			if err != nil {
				return shareRead{}, err
			}
			p.share, p.folder, p.dataset = sh.Name, d.Name, datasetAt(datasets, d.Path)
			out = append(out, p)
		}
	}
	return shareRead{places: out, shares: shares, mounts: datasets}, nil
}

// readACL reads one folder's ACL. A POSIX ACL reads as opening nothing.
//
// ponytail: NFS4 only, which is what TrueNAS gives every SMB dataset by
// default. Evaluate POSIX1E entries here if a share ever carries one.
func (s *server) readACL(at string) (aclPlace, error) {
	var got struct {
		ACLType string     `json:"acltype"`
		GID     int64      `json:"gid"`
		ACL     []aclEntry `json:"acl"`
	}
	if err := s.nas.call("filesystem.getacl", []any{at, false, false}, &got); err != nil {
		return aclPlace{}, fmt.Errorf("read the permissions of %s: %w", at, err)
	}
	p := aclPlace{path: at, ownerGID: got.GID}
	if got.ACLType == "NFS4" {
		p.acl = got.ACL
	}
	return p, nil
}

type smbShare struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Enabled bool   `json:"enabled"`
}

// splitList reads a comma-separated list, dropping blanks.
func splitList(raw string) []string {
	var out []string
	for _, v := range strings.Split(raw, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// shareScope is the configured share list as a set, nil for "every share".
func shareScope(names []string) map[string]bool {
	if len(names) == 0 {
		return nil
	}
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	return set
}

// considers says whether Syndra looks at a share at all (TRUENAS_SHARES).
func (s *server) considers(share string) bool {
	return s.shares == nil || s.shares[share]
}

// enabledShares are the SMB shares a member can connect to, longest path
// first so shareAt finds the innermost one. Only shares Syndra considers.
func (s *server) enabledShares() ([]smbShare, error) {
	var all []smbShare
	if err := s.nas.call("sharing.smb.query",
		[]any{[]any{}, map[string]any{"select": []string{"name", "path", "enabled"}}}, &all); err != nil {
		return nil, fmt.Errorf("read shares: %w", err)
	}
	out := []smbShare{}
	for _, sh := range all {
		sh.Path = strings.TrimRight(sh.Path, "/")
		if sh.Enabled && strings.HasPrefix(sh.Path, "/mnt/") && s.considers(sh.Name) {
			out = append(out, sh)
		}
	}
	sort.Slice(out, func(i, j int) bool { return len(out[i].Path) > len(out[j].Path) })
	return out, nil
}

// shareAt is the share a path is reached through, and the path inside it.
func shareAt(shares []smbShare, at string) (share smbShare, inside string, ok bool) {
	for _, sh := range shares {
		if at == sh.Path {
			return sh, "", true
		}
		if strings.HasPrefix(at, sh.Path+"/") {
			return sh, strings.TrimPrefix(at, sh.Path+"/"), true
		}
	}
	return smbShare{}, "", false
}

// datasetMounts maps each dataset's mountpoint to its name.
func (s *server) datasetMounts() (map[string]string, error) {
	var rows []struct {
		Name       string `json:"name"`
		Mountpoint string `json:"mountpoint"`
	}
	if err := s.nas.call("pool.dataset.query", []any{[]any{}, map[string]any{"select": []string{"name", "mountpoint"}}}, &rows); err != nil {
		return nil, fmt.Errorf("read datasets: %w", err)
	}
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		if r.Mountpoint != "" {
			out[strings.TrimRight(r.Mountpoint, "/")] = r.Name
		}
	}
	return out, nil
}

// datasetAt is the innermost dataset holding a path.
func datasetAt(mounts map[string]string, at string) string {
	for p := at; p != "/" && p != "."; p = path.Dir(p) {
		if name, ok := mounts[p]; ok {
			return name
		}
	}
	return ""
}

// gidsOf maps group ids, as user.query lists them, to gids, which is what an
// ACL names.
func gidsOf(groups []nasGroup) map[string]int64 {
	byID := make(map[string]int64, len(groups))
	for _, g := range groups {
		byID[g.ID.String()] = g.GID
	}
	return byID
}

// ReachResponse is what each of the makerspace's groups opens.
type ReachResponse struct {
	// ReadAt is when the folder permissions were read. Current is false when
	// the NAS couldn't be read just now and this is the last read.
	ReadAt  time.Time `json:"read_at"`
	Current bool      `json:"current"`
	// Groups is every group of the makerspace's own, with what holding it opens
	// beyond what every account opens. An empty list is "opens nothing yet".
	Groups map[string][]Place `json:"groups"`
	// Everyone is what every account opens whatever its groups, through
	// everyone@. builtin_users is not everyone: Syndra never adds its accounts
	// to it, and only accounts made by hand on TrueNAS are in it.
	Everyone []Place `json:"everyone"`
}

// handleReach answers what each group opens, for the mappings page.
func (s *server) handleReach(w http.ResponseWriter, _ *http.Request, _ []byte) {
	groups, err := s.readGroups()
	if err != nil {
		writeJSON(w, statusFor(err), map[string]string{"error": "TARGET_UNREADABLE"})
		return
	}
	read, at, current, err := s.readACLs()
	if errors.Is(err, errNeverRead) {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "REACH_NEVER_READ"})
		return
	}
	if err != nil {
		writeJSON(w, statusFor(err), map[string]string{"error": "TARGET_UNREADABLE"})
		return
	}
	nobody := principals{} // an account in no group: what everyone@ alone opens
	out := ReachResponse{ReadAt: at, Current: current, Groups: map[string][]Place{},
		Everyone: placesFor(read.places, nobody, nil)}
	for _, g := range groups {
		if !g.Builtin {
			out.Groups[g.Name] = placesFor(read.places, principals{g.GID: true}, nobody)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// memberPlaces is everywhere one account can connect: what its own groups
// open, then its home folder first. Usage is filled in for the places it can
// change, one quota read per dataset.
func (s *server) memberPlaces(uid int64, groupIDs []apiID, home string) ([]Place, error) {
	groups, err := s.readGroups()
	if err != nil {
		return nil, err
	}
	read, _, _, err := s.readACLs()
	if err != nil {
		return nil, err
	}
	byID := gidsOf(groups)
	who := principals{}
	for _, id := range groupIDs {
		if gid, ok := byID[id.String()]; ok {
			who[gid] = true
		}
	}
	places := placesFor(read.places, who, nil)
	if sh, inside, ok := shareAt(read.shares, home); ok && inside != "" {
		places = append(places, Place{Share: sh.Name, Folder: inside, Path: home,
			Access: accessChange, Dataset: datasetAt(read.mounts, home), Home: true})
		sortPlaces(places)
	}

	quotas := map[string]map[int64]PlaceUsage{}
	for i, p := range places {
		if p.Access != accessChange || p.Dataset == "" {
			continue
		}
		table, read := quotas[p.Dataset]
		if !read {
			table = s.userQuotas(p.Dataset)
			quotas[p.Dataset] = table
		}
		if table == nil {
			continue // unreadable: say nothing rather than "nothing saved"
		}
		u := table[uid] // no row is nothing saved yet
		places[i].Usage = &u
	}
	return places, nil
}

// userQuotas reads one dataset's user quota table by uid, nil when it can't be
// read.
func (s *server) userQuotas(dataset string) map[int64]PlaceUsage {
	var rows []struct {
		ID        int64 `json:"id"`
		UsedBytes int64 `json:"used_bytes"`
		// shape-optional: TrueNAS omits `quota` for a uid with no limit set.
		QuotaBytes int64 `json:"quota"`
	}
	if err := s.nas.call("pool.dataset.get_quota", []any{dataset, "USER"}, &rows); err != nil {
		return nil
	}
	out := make(map[int64]PlaceUsage, len(rows))
	for _, r := range rows {
		out[r.ID] = PlaceUsage{UsedBytes: r.UsedBytes, QuotaBytes: r.QuotaBytes}
	}
	return out
}
