package main

import (
	"fmt"
	"time"
)

// Reading the target's state (design §10).
//
// The `select` is the security-relevant part. `user.query` returns `unixhash`
// and `smbhash`, and an NT hash is a pass-the-hash credential — possessing one
// is equivalent to possessing the password for SMB purposes. So the query asks
// for the fields this add-on needs by name, and `Subject` has nowhere for a
// hash to land even if that list were edited.

// subjectReadCap bounds one read.
//
// A cap is not a nicety on a full state read: without one, a NAS with a large
// directory would be pulled into memory in a single JSON document. What matters
// more is that hitting it is REPORTED — a capped read is current and still
// cannot support a conclusion about absence, which is half of what the drift
// diff does with it.
const subjectReadCap = 5000

// nasUser is the shape of one `user.query` row, narrowed to the selected
// fields. Deliberately not a map: a map would carry whatever the middleware
// returned, including the two fields this file exists to keep out.
type nasUser struct {
	Username string  `json:"username"`
	UID      int64   `json:"uid"`
	Locked   bool    `json:"locked"`
	SMB      bool    `json:"smb"`
	Groups   []int64 `json:"groups"`
}

type nasGroup struct {
	GID  int64  `json:"gid"`
	Name string `json:"group"`
}

// readSubjects performs the full state read.
//
// Groups are resolved to names, because a gid means nothing to the backend's
// resolver: mappings bind a role to a group NAME, and comparing a name against
// a number would make every subject look like drift.
func (s *server) readSubjects() (Snapshot, error) {
	var users []nasUser
	// `select` names every field, and only these. The two hash fields are
	// absent by construction rather than stripped afterwards — stripping is a
	// step somebody can forget, and the forgetting would not be visible.
	query := []any{
		[]any{},
		map[string]any{
			"select": []string{"username", "uid", "locked", "smb", "groups"},
			"limit":  subjectReadCap + 1,
		},
	}
	if err := s.nas.call("user.query", query, &users); err != nil {
		return Snapshot{}, err
	}

	truncated := len(users) > subjectReadCap
	if truncated {
		users = users[:subjectReadCap]
	}

	names, err := s.groupNames()
	if err != nil {
		// A read that cannot name groups is a read whose group sets are
		// meaningless to the resolver. Reported as a failure rather than
		// returned with numbers in it, because the caller would diff those
		// numbers against names and find everything drifting.
		return Snapshot{}, err
	}

	subjects := make([]Subject, 0, len(users))
	for _, u := range users {
		groups := make([]string, 0, len(u.Groups))
		for _, gid := range u.Groups {
			if name, ok := names[gid]; ok {
				groups = append(groups, name)
			}
			// A gid with no name is skipped rather than rendered as a number.
			// It is a group the resolver could never have asked for, so
			// including it would be reporting drift against a mapping that
			// cannot exist.
		}
		subjects = append(subjects, Subject{
			Username: u.Username, UID: u.UID, Groups: groups,
			// `locked` is the NAS's word for it; `enabled` is Syndra's, and the
			// translation belongs here rather than in the backend, which does
			// not know what TrueNAS calls things.
			Enabled:    !u.Locked,
			SMBEnabled: u.SMB,
		})
	}
	return Snapshot{TakenAt: time.Now().UTC(), Subjects: subjects, Truncated: truncated}, nil
}

// groupNames maps gid to group name.
func (s *server) groupNames() (map[int64]string, error) {
	var groups []nasGroup
	query := []any{
		[]any{},
		map[string]any{"select": []string{"gid", "group"}, "limit": subjectReadCap + 1},
	}
	if err := s.nas.call("group.query", query, &groups); err != nil {
		return nil, fmt.Errorf("read groups: %w", err)
	}
	out := make(map[int64]string, len(groups))
	for _, g := range groups {
		out[g.GID] = g.Name
	}
	return out, nil
}

// Ping is the reachability probe `/health` uses.
func (n *NAS) Ping() (string, error) {
	c, err := n.session()
	if err != nil {
		return "", err
	}
	res, err := c.Ping()
	if err != nil {
		n.drop()
		return "", fmt.Errorf("ping: %w", classifyNASError(err))
	}
	n.mu.Lock()
	n.lastRead = n.now()
	n.mu.Unlock()
	return res, nil
}
