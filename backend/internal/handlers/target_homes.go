package handlers

import (
	"net/http"
	"strconv"
	"syndra/internal/textsort"
	"time"

	"syndra/internal/addons"
	"syndra/internal/db"
	"syndra/internal/handlers/kinds"
	"syndra/internal/services"
)

// Every managed account's home folder limit on a target (A-03 §3), for the
// system's account list: "Home folder up to 20 GB, from Fabrication lead", or
// the person's own limit, with what is saved there now.
//
// The roles' limit is the largest a person's roles give (the resolver's record
// of who gave what). Their own limit is a `set` allowance on the storage
// kind's limit field (kinds.HomeLimit, by role and unit, never by name),
// bounded by an end or a review date. Usage is the target's read; it can be
// missing, and then the page says less rather than guessing.

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
	// The home folder's limit field, by role: none known, no limit is shown.
	schema, _ := addonsEntitlementSchema(target)
	limit, _ := kinds.HomeLimit(schema)
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
			roleLimits(&row, set, limit)
		}
		row.Own = ownHomeLimit(r, b.SubjectID, target, limit.Name, now)
		if h, ok := byName[b.Username]; ok {
			row.Home = h.Home
			if homes.UsageRead {
				used, quota := h.UsedBytes, h.QuotaBytes
				row.UsedBytes, row.QuotaBytes = &used, &quota
			}
		}
		ownEndedIfOver(r, &row, target, limit.Name, now)
		out = append(out, row)
	}
	textsort.ByName(out, func(r accountHome) string { return r.Username }, nil)
	jsonResponse(w, http.StatusOK, map[string]any{"read": homes.Read, "usage_read": homes.UsageRead, "accounts": out})
}

// roleLimits is what a person's roles give: a home folder at all, and the
// largest limit, with the role that gives it.
func roleLimits(row *accountHome, set services.EntitlementSet, limit addons.EntitlementField) {
	if home := kinds.Storage(set.Roles).Home; home != "" {
		row.GivesHome = len(set.Fields[home]) > 0
	}
	if limit.Name != "" {
		row.RolesLimit, row.RolesLimitFrom = largestGiven(set, limit)
	}
}

// ownHomeLimit is the person's own limit in force, or nil.
func ownHomeLimit(r *http.Request, subject, target, field string, now time.Time) *homeOwnLimit {
	if field == "" {
		return nil
	}
	inForce, err := dbAllowancesInForce(r.Context(), subject, target)
	if err != nil {
		return nil
	}
	var own *homeOwnLimit
	for _, a := range inForce {
		if a.Direction == db.AllowanceSet && a.Field == field {
			n, _ := strconv.ParseInt(a.Value, 10, 64)
			own = &homeOwnLimit{AllowanceID: a.ID, Bytes: n, Reason: a.Reason, ActorID: a.ActorID,
				SetAt: a.CreatedAt, ExpiresAt: a.ExpiresAt, ReviewDate: a.ReviewDate, ReviewDue: a.ReviewDue(now)}
		}
	}
	return own
}

// ownEndedIfOver names the own limit that ended, only while the folder still
// holds more than the roles give.
func ownEndedIfOver(r *http.Request, row *accountHome, target, field string, now time.Time) {
	if row.Own == nil && row.UsedBytes != nil && row.RolesLimit != nil && *row.UsedBytes > *row.RolesLimit {
		row.OwnEnded = lastOwnLimitEnded(r, row.SubjectID, target, field, now)
	}
}

// largestGiven is the largest limit a field's roles give, and the role that
// gives it. The field's `unlimited` value is no limit, the largest of all, as
// the add-on applies it (combine: max, unlimited dominant): then the limit is
// nil and the role is the one that lifts it. Both nil when no role gives one.
func largestGiven(set services.EntitlementSet, field addons.EntitlementField) (*int64, *services.RoleSource) {
	var best *int64
	var from, unlimited *services.RoleSource
	prefix := field.Name + "\x00"
	for key, roles := range set.Given {
		if len(key) <= len(prefix) || key[:len(prefix)] != prefix || len(roles) == 0 {
			continue
		}
		n, err := strconv.ParseInt(key[len(prefix):], 10, 64)
		if err != nil {
			continue
		}
		if u, err := strconv.ParseInt(field.Unlimited, 10, 64); err == nil && n == u {
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
func lastOwnLimitEnded(r *http.Request, subject, target, field string, now time.Time) *homeEnded {
	all, err := dbAllowancesForSubject(r.Context(), subject)
	if err != nil {
		return nil
	}
	var latest *homeEnded
	for _, a := range all {
		if a.Target != target || a.Direction != db.AllowanceSet || a.Field != field {
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
