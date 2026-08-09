package services

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"

	"syndra/internal/models"
)

// Fingerprints of the object an operator reviewed (design §8).
//
// A plan is only worth citing if the world it describes is still the world. The
// fingerprint is how an apply asks that question, and it covers **the object
// that was reviewed**, not merely the subject's identity: for a bulk grant
// operation the grant set and the roles behind it, for drift triage the drift
// row's own status — because `rehearseOneDrift` already handles somebody
// resolving a row while the operator reads the list, and fingerprinting the
// grants alone would let exactly that case pass verification.
//
// Every fingerprint here is computed by ONE function used on both sides. A
// value the rehearsal computes one way and the apply computes another verifies
// nothing, in the same way a preview computed by different code from the token
// it previews is a preview of nothing.

// Fingerprint hashes an ordered list of fields into the digest the plan store
// records.
//
// Length-prefixed rather than joined on a separator: any separator can appear
// inside a role key or a project id, and `["ab","c"]` hashing to the same value
// as `["a","bc"]` is a collision an attacker picks rather than finds. The
// prefix makes the encoding injective, so two different field lists cannot
// produce one digest.
func Fingerprint(fields ...string) string {
	h := sha256.New()
	for _, f := range fields {
		_, _ = h.Write([]byte(strconv.Itoa(len(f))))
		_, _ = h.Write([]byte{':'})
		_, _ = h.Write([]byte(f))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// FingerprintUserAccess digests what a bulk grant operation was rehearsed
// against: the roles the subject effectively holds, and the direct grant rows
// the apply may act on.
//
// Both halves are needed and neither is sufficient. The grant rows alone miss a
// bundle assignment landing between rehearsal and apply, which changes whether
// a removal leaves the person holding the role. The effective roles alone miss
// a grant being renewed, re-sourced, or replaced by an identical-looking one
// with a different id — and `GrantIDs` on the plan row is what the apply
// mutates, so an id that moved is an id the operator did not review.
//
// Expiry is included because it is the reviewed fact for `extend`: the same
// grant with a later date is a different row to approve extending.
func FingerprintUserAccess(userID, status string, roles []string, grants []models.DirectGrant) string {
	// Directory status is part of the reviewed state, not context around it.
	// Granting to somebody who has left is the commonest way a bulk selection
	// goes wrong — it is why the rehearsal blocks it — so an account that
	// departs, or is reactivated, between the review and the apply must
	// invalidate the approval rather than slip through under it.
	fields := []string{"user", userID, "status", status, "roles"}

	sorted := append([]string(nil), roles...)
	sort.Strings(sorted)
	fields = append(fields, sorted...)

	// Sorted by id so a change in read order is not a change in state. The
	// grants themselves are sorted by the database only incidentally.
	rows := append([]models.DirectGrant(nil), grants...)
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })

	fields = append(fields, "grants")
	for _, g := range rows {
		expires := ""
		if g.ExpiresAt != nil {
			expires = g.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z")
		}
		fields = append(fields, g.ID, g.ProjectID, g.RoleKey, g.Source, expires)
	}
	return Fingerprint(fields...)
}

// effectiveRoleKeys flattens the rehearsal's role map into the stable list the
// fingerprint hashes. Project and role together, because a role key is only
// unique inside its project and two projects sharing one would otherwise be
// indistinguishable.
func effectiveRoleKeys(roleMap map[roleKey]*models.EffectiveRole) []string {
	out := make([]string, 0, len(roleMap))
	for k, r := range roleMap {
		if r == nil {
			continue
		}
		out = append(out, k.projectID+"/"+k.roleKey)
	}
	return out
}

// FingerprintDriftItem digests a triage row as it was reviewed.
//
// `Status` is the field this exists for. A row somebody else resolved while the
// operator was reading the list is the concrete case design §8 names, and it is
// invisible to any fingerprint taken over the access alone: the grants have not
// moved, the person has not moved, and the decision has already been made by
// somebody else.
func FingerprintDriftItem(item models.DriftItem) string {
	roles := append([]string(nil), item.RoleKeys...)
	sort.Strings(roles)
	return Fingerprint(append([]string{
		"drift", item.ID, item.Target, item.Status, item.UserID, item.ProjectID, "roles",
	}, roles...)...)
}

// FingerprintAccessRequest digests a pending request as it was reviewed.
//
// Status for the same reason drift carries it — a second reviewer deciding
// first is the ordinary case in a shared queue. Duration because approving
// mints a grant for it, so a request edited between rehearsal and apply would
// mint a window nobody approved.
func FingerprintAccessRequest(r models.AccessRequest) string {
	duration := ""
	if r.DurationDays != nil {
		duration = strconv.Itoa(*r.DurationDays)
	}
	return Fingerprint("request", r.ID, r.Status, r.RequesterID, r.ProjectID, r.RoleKey, duration)
}

// FingerprintBulkRequest digests the submitted request a rehearsal was computed
// for, so an apply cannot carry a different one under the same approval.
//
// Only what changes the effect. `Reason` is absent deliberately: it is an
// annotation recorded alongside the write, it changes nothing about who gets
// what, and binding it would make correcting a typo cost a re-plan — which
// trains operators to click through the stale-plan dialog, on the surface where
// that dialog matters most.
func FingerprintBulkRequest(req BulkRequest) string {
	users := dedupeIDs(req.UserIDs)
	sort.Strings(users)
	grants := append([]string(nil), req.GrantIDs...)
	sort.Strings(grants)

	fields := []string{
		"bulk", req.Op, req.ProjectID, req.RoleKey, req.BundleID,
		strconv.Itoa(req.DurationDays), "users",
	}
	fields = append(fields, users...)
	fields = append(fields, "grants")
	fields = append(fields, grants...)
	return Fingerprint(fields...)
}

// FingerprintIDCohort digests a surface whose request is a set of ids plus a
// handful of parameters — drift bulk resolution and bulk request decisions.
//
// The caller passes the parameters that change the effect and nothing else: an
// attribution source or a decision status, never the free-text reason or review
// note an operator writes at apply time.
func FingerprintIDCohort(op string, ids []string, params ...string) string {
	unique := map[string]struct{}{}
	var cohort []string
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, dup := unique[id]; dup {
			continue
		}
		unique[id] = struct{}{}
		cohort = append(cohort, id)
	}
	sort.Strings(cohort)

	fields := append([]string{"cohort", op}, params...)
	fields = append(fields, "ids")
	fields = append(fields, cohort...)
	return Fingerprint(fields...)
}
