package handlers

import (
	"net/http"
	"sort"
	"strconv"
	"time"

	"syndra/internal/addons"
	"syndra/internal/db"
	"syndra/internal/services"
)

// Every managed account's home folder limit on a target (A-03 §3), for the
// system's account list: "Home folder up to 20 GB, from Fabrication lead", or
// the person's own limit, with what is saved there now.
//
// The roles' limit is the largest a person's roles give (the resolver's record
// of who gave what). Their own limit is a `set` allowance on `home_quota`,
// bounded by an end or a review date. Usage is the target's read; it can be
// missing, and then the page says less rather than guessing.

const fieldHomeQuota = "home_quota"

type homeOwnLimit struct {
	AllowanceID string     `json:"allowance_id"`
	Bytes       int64      `json:"bytes"` // 0 is no limit
	Reason      string     `json:"reason"`
	ActorID     string     `json:"actor_id"`
	SetAt       time.Time  `json:"set_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	ReviewDate  *time.Time `json:"review_date,omitempty"`
	ReviewDue   bool       `json:"review_due,omitempty"`
}

// homeEnded is an own limit that ended while the folder held more than the
// roles' limit: "She can open and delete files but can't save new ones".
type homeEnded struct {
	Bytes   int64     `json:"bytes"`
	EndedAt time.Time `json:"ended_at"`
}

type accountHome struct {
	SubjectID string `json:"subject_id"`
	Username  string `json:"username"`
	// GivesHome is whether any role they hold gives a home folder.
	GivesHome bool `json:"gives_home"`
	// RolesLimit is the largest limit their roles give, in bytes; absent when
	// no role sets one (no limit). RolesLimitFrom is the role that gives it.
	RolesLimit     *int64               `json:"roles_limit,omitempty"`
	RolesLimitFrom *services.RoleSource `json:"roles_limit_from,omitempty"`
	Own            *homeOwnLimit        `json:"own,omitempty"`
	OwnEnded       *homeEnded           `json:"own_ended,omitempty"`
	Home           string               `json:"home,omitempty"`
	UsedBytes      *int64               `json:"used_bytes,omitempty"`
	QuotaBytes     *int64               `json:"quota_bytes,omitempty"`
}

func handleTargetHomes(w http.ResponseWriter, r *http.Request) {
	target := r.PathValue("target")
	bindings, err := dbListTargetBindings(r.Context(), target)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	homes := addonsReadHomes(r.Context(), target)
	byName := map[string]addons.HomeUsage{}
	for _, h := range homes.Homes {
		byName[h.Username] = h
	}

	now := time.Now()
	out := make([]accountHome, 0, len(bindings))
	for _, b := range bindings {
		row := accountHome{SubjectID: b.SubjectID, Username: b.Username}
		if set, err := svcResolveEntitlementSet(r.Context(), b.SubjectID, target); err == nil {
			row.GivesHome = len(set.Fields["home"]) > 0
			row.RolesLimit, row.RolesLimitFrom = largestGiven(set, fieldHomeQuota)
		}
		if inForce, err := dbAllowancesInForce(r.Context(), b.SubjectID, target); err == nil {
			for _, a := range inForce {
				if a.Direction == db.AllowanceSet && a.Field == fieldHomeQuota {
					n, _ := strconv.ParseInt(a.Value, 10, 64)
					row.Own = &homeOwnLimit{AllowanceID: a.ID, Bytes: n, Reason: a.Reason, ActorID: a.ActorID,
						SetAt: a.CreatedAt, ExpiresAt: a.ExpiresAt, ReviewDate: a.ReviewDate, ReviewDue: a.ReviewDue(now)}
				}
			}
		}
		if h, ok := byName[b.Username]; ok {
			row.Home = h.Home
			if homes.UsageRead {
				used, quota := h.UsedBytes, h.QuotaBytes
				row.UsedBytes, row.QuotaBytes = &used, &quota
			}
		}
		if row.Own == nil && row.UsedBytes != nil && row.RolesLimit != nil && *row.UsedBytes > *row.RolesLimit {
			row.OwnEnded = lastOwnLimitEnded(r, b.SubjectID, target, now)
		}
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Username < out[j].Username })
	jsonResponse(w, http.StatusOK, map[string]any{"read": homes.Read, "usage_read": homes.UsageRead, "accounts": out})
}

// largestGiven is the largest limit a field's roles give, and the role that
// gives it. "0" is no limit, the largest of all, as the add-on applies it: then
// the limit is nil and the role is the one that lifts it. Both nil when no role
// gives one.
func largestGiven(set services.EntitlementSet, field string) (*int64, *services.RoleSource) {
	var best *int64
	var from, unlimited *services.RoleSource
	prefix := field + "\x00"
	for key, roles := range set.Given {
		if len(key) <= len(prefix) || key[:len(prefix)] != prefix || len(roles) == 0 {
			continue
		}
		n, err := strconv.ParseInt(key[len(prefix):], 10, 64)
		if err != nil {
			continue
		}
		if n == 0 {
			r := roles[0]
			unlimited = &r
			continue
		}
		if best == nil || n > *best {
			v, r := n, roles[0]
			best, from = &v, &r
		}
	}
	if unlimited != nil {
		return nil, unlimited
	}
	return best, from
}

// lastOwnLimitEnded is the most recent own limit that has ended, lifted or
// lapsed. Shown only while the folder is still over what the roles give.
func lastOwnLimitEnded(r *http.Request, subject, target string, now time.Time) *homeEnded {
	all, err := dbAllowancesForSubject(r.Context(), subject)
	if err != nil {
		return nil
	}
	var latest *homeEnded
	for _, a := range all {
		if a.Target != target || a.Direction != db.AllowanceSet || a.Field != fieldHomeQuota {
			continue
		}
		ended := a.LiftedAt
		if ended == nil && a.ExpiresAt != nil && !a.ExpiresAt.After(now) {
			ended = a.ExpiresAt
		}
		if ended == nil {
			continue
		}
		if latest == nil || ended.After(latest.EndedAt) {
			n, _ := strconv.ParseInt(a.Value, 10, 64)
			latest = &homeEnded{Bytes: n, EndedAt: *ended}
		}
	}
	return latest
}
