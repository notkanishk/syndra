package main

import (
	"errors"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"
)

// Home folders by role (truenas-reconcile D10, owner 14 Oct 2026).
//
// A role can put its holders' home folders under a place on the NAS. Each
// person's folder is <place>/<account name>, created by TrueNAS itself
// (`home_create`) with mode 700, which is how the one hand-made home on the
// production NAS is private. The rules are about data, so they are strict:
//
//   - A home is never moved. An account whose home is somewhere else keeps it,
//     and the outcome says where the role would have put it.
//   - A home is never removed. Losing the role leaves the folder, and the files.
//   - Two roles naming different places: the home is left as it is.
//   - A folder that already exists and belongs to another uid is refused: a new
//     account must never inherit an old member's files.

// noHome is TrueNAS's home for an account without one.
const noHome = "/var/empty"

func homeOf(raw string) string {
	if raw == "" || raw == noHome || raw == "/nonexistent" {
		return ""
	}
	return raw
}

func uniqueSorted(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimRight(strings.TrimSpace(v), "/")
		if v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}

// homeDecision is what a convergence does about one account's home folder.
type homeDecision struct {
	// path is set when the home should be created there.
	path string
	// note is what the outcome says when it is not (moved, disputed, taken).
	note string
}

// statOwner returns the owner uid of a path, or ErrNoSuchPath. A stat that
// fails for any other reason is an error, never "free".
func (s *server) statOwner(p string) (int64, error) {
	var st struct {
		UID int64 `json:"uid"`
	}
	if err := s.nas.call("filesystem.stat", []any{p}, &st); err != nil {
		if errors.Is(err, ErrNoSuchPath) {
			return 0, ErrNoSuchPath
		}
		return 0, fmt.Errorf("read %s: %w", p, err)
	}
	return st.UID, nil
}

// decideHome applies the rules above. uid is the account's, or -1 for one that
// does not exist yet.
func (s *server) decideHome(desired desiredState, username, current string, uid int64) (homeDecision, error) {
	if !desired.managed[FieldHome] || len(desired.homes) == 0 {
		return homeDecision{}, nil
	}
	if len(desired.homes) > 1 {
		return homeDecision{note: fmt.Sprintf("Two of %s's roles give different home folders (%s), so the home was left as it is.",
			username, strings.Join(desired.homes, " and "))}, nil
	}
	want := path.Join(desired.homes[0], username)
	switch current {
	case want:
		return homeDecision{}, nil
	case "":
	default:
		return homeDecision{note: fmt.Sprintf("%s's home is %s. The role puts homes in %s, and Syndra never moves a home folder; move it on TrueNAS if it should be there.",
			username, current, desired.homes[0])}, nil
	}
	owner, err := s.statOwner(want)
	switch {
	case errors.Is(err, ErrNoSuchPath):
		return homeDecision{path: want}, nil
	case err != nil:
		return homeDecision{}, err
	case uid >= 0 && owner == uid:
		return homeDecision{path: want}, nil
	default:
		return homeDecision{note: fmt.Sprintf("A folder already exists at %s and belongs to someone else (uid %d), so it was not made %s's home.",
			want, owner, username)}, nil
	}
}

// homeFields is the part of a user.create or user.update that makes the home.
func homeFields(p string) map[string]any {
	return map[string]any{"home": p, "home_create": true, "home_mode": "700"}
}

// convergeQuota sets the account's limit on the dataset holding its home, when
// one is wanted and it differs. A limit nobody gives any more is left as it
// is: removing it would make the folder unlimited, which no one decided.
func (s *server) convergeQuota(desired desiredState, home string, uid int64) (string, error) {
	if !desired.managed[FieldHomeQuota] || desired.homeQuota == 0 || home == "" {
		return "", nil
	}
	dataset := datasetOf(path.Dir(home))
	if dataset == "" {
		return "", nil
	}
	var rows []struct {
		ID int64 `json:"id"`
		// shape-optional: TrueNAS 25.10's get_quota schema says "absent if no
		// quota is set", and the production NAS had none set when recorded.
		Quota int64 `json:"quota"`
	}
	if err := s.nas.call("pool.dataset.get_quota", []any{dataset, "USER", []any{[]any{"id", "=", uid}}}, &rows); err != nil {
		return "", fmt.Errorf("read the home folder limit: %w", err)
	}
	for _, r := range rows {
		if r.ID == uid && r.Quota == desired.homeQuota {
			return "", nil
		}
	}
	if err := s.nas.call("pool.dataset.set_quota", []any{dataset, []any{map[string]any{
		"quota_type": "USER", "id": strconv.FormatInt(uid, 10), "quota_value": desired.homeQuota,
	}}}, nil); err != nil {
		return "", fmt.Errorf("set the home folder limit: %w", err)
	}
	return fmt.Sprintf("Home folder limit set to %s.", humanBytes(desired.homeQuota)), nil
}

// humanBytes says a limit the way a person reads one.
func humanBytes(n int64) string {
	const gib = 1 << 30
	if n%gib == 0 {
		return fmt.Sprintf("%d GiB", n/gib)
	}
	return fmt.Sprintf("%.1f GiB", float64(n)/gib)
}

// homePlaces are where a role may put home folders: every dataset inside an
// enabled SMB share, so a home is always somewhere a member can reach. The
// share's own root is left out: homes belong in a folder of their own.
func (s *server) homePlaces() ([]string, error) {
	var shares []struct {
		Path    string `json:"path"`
		Enabled bool   `json:"enabled"`
	}
	if err := s.nas.call("sharing.smb.query", []any{[]any{}, map[string]any{"select": []string{"path", "enabled"}}}, &shares); err != nil {
		return nil, fmt.Errorf("read shares: %w", err)
	}
	var datasets []struct {
		Mountpoint string `json:"mountpoint"`
	}
	if err := s.nas.call("pool.dataset.query", []any{[]any{}, map[string]any{"select": []string{"name", "mountpoint"}}}, &datasets); err != nil {
		return nil, fmt.Errorf("read datasets: %w", err)
	}
	places := []string{}
	for _, d := range datasets {
		for _, sh := range shares {
			root := strings.TrimRight(sh.Path, "/")
			if sh.Enabled && root != "" && strings.HasPrefix(d.Mountpoint, root+"/") {
				places = append(places, d.Mountpoint)
				break
			}
		}
	}
	return uniqueSorted(places), nil
}
