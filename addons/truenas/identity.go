package main

import (
	"fmt"
)

// Identity numbers this add-on hands out (truenas-reconcile 11, found 14 Oct
// 2026). TrueNAS gives a new account the next free uid and its personal group
// the next free gid, and it cannot see numbers that survive only as the owners
// of old files. On the production NAS that handed three new accounts the home
// folders of three departed members. So this add-on picks the numbers itself,
// from a range nothing else uses, and never hands one out twice.
//
// 200000–999999 by default, read from the production NAS on 14 Oct:
//   - above every number in use or left on files (the highest was 9001),
//     and above 65534 (nobody) and 65535;
//   - clear of 100000–165535, where unprivileged Proxmox containers map
//     their users, and the NAS exports to containers over NFS;
//   - far below the API's limit (90,000,000) and every range TrueNAS reserves
//     for SMB and directory-service id mapping (90,000,001 and up);
//   - under 2,097,151, the largest uid the ustar tar format holds, so
//     backups keep owners intact.
//
// 800,000 numbers: ample for any makerspace, and the ceiling is configurable.
const (
	defaultIDFloor   = 200000
	defaultIDCeiling = 999999
)

func (s *server) idRange() (int64, int64) {
	floor, ceiling := s.idFloor, s.idCeiling
	if floor <= 0 {
		floor = defaultIDFloor
	}
	if ceiling <= 0 {
		ceiling = defaultIDCeiling
	}
	return floor, ceiling
}

// nextIdentity claims a number free as both a uid and a gid: one number for an
// account and its personal group, which is what makes "who owns this" readable
// from either.
func (s *server) nextIdentity() (int64, error) {
	floor, ceiling := s.idRange()
	var users []struct {
		UID int64 `json:"uid"`
	}
	if err := s.nas.call("user.query", []any{
		[]any{[]any{"uid", ">=", floor}, []any{"uid", "<=", ceiling}},
		map[string]any{"select": []string{"uid"}, "order_by": []string{"-uid"}, "limit": 1},
	}, &users); err != nil {
		return 0, fmt.Errorf("read the highest uid in range: %w", err)
	}
	var groups []struct {
		GID int64 `json:"gid"`
	}
	if err := s.nas.call("group.query", []any{
		[]any{[]any{"gid", ">=", floor}, []any{"gid", "<=", ceiling}},
		map[string]any{"select": []string{"gid"}, "order_by": []string{"-gid"}, "limit": 1},
	}, &groups); err != nil {
		return 0, fmt.Errorf("read the highest gid in range: %w", err)
	}
	observed := int64(0)
	if len(users) > 0 {
		observed = users[0].UID
	}
	if len(groups) > 0 && groups[0].GID > observed {
		observed = groups[0].GID
	}
	return s.store.ClaimID(floor, ceiling, observed)
}

// undoGroup removes a personal group whose account was never created, so a
// refused create leaves nothing behind. Best-effort: a group left over is an
// empty group with a number nobody will be given again.
func (s *server) undoGroup(id apiID) {
	if !id.known() {
		return
	}
	if err := s.nas.call("group.delete", []any{id}, nil); err != nil {
		logStoreFailure("personal group cleanup", id.String(), err)
	}
}
