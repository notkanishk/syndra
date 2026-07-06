package handlers

import (
	"context"
	"net/http"
	"sort"
	"time"

	"mkauth/internal/models"
	"mkauth/internal/services"
	"mkauth/internal/zitadel"
)

// reconciliationPageSize is the per-page limit on the Zitadel ListAllGrants
// pagination loop. 500 is below the documented 1000-row Zitadel cap and gives
// the loop a reasonable round-trip rhythm for makerspace-scale orgs.
//
// `var` (not const) so tests can override to a smaller number when exercising
// the multi-page path; production callers must not mutate this.
var reconciliationPageSize = 500

// reconciliationSafetyCap stops the pagination loop after this many grants so
// a pathologically large org cannot cause an unbounded fetch. Past this point
// the response sets Truncated=true and the diff is best-effort — only_in_mkauth
// and drift may contain false positives because un-fetched Zitadel pages were
// not compared. Sized for roughly 20× the largest expected makerspace tenant.
//
// `var` (not const) so tests can override to a smaller number; production
// callers must not mutate this.
var reconciliationSafetyCap = 2_000 // B2: right-sized for the single-LXC ~200-user makerspace (~10× headroom)

// ReconciliationGrant is one (user, project, role-set) pair on either the
// MkAuth or Zitadel side of the diff. Roles are sorted ascending so equality
// checks and rendering are stable.
type ReconciliationGrant struct {
	UserID    string   `json:"user_id"`
	ProjectID string   `json:"project_id"`
	RoleKeys  []string `json:"role_keys"`
	// GrantID is populated for Zitadel-side entries so the UI can drill into
	// the exact grant record. Empty on the MkAuth side (each row is keyed by
	// (user_id, project_id, role_key) without a corresponding Zitadel grant).
	GrantID string `json:"grant_id,omitempty"`
}

// ReconciliationDrift is a (user, project) pair that exists on both sides but
// with non-identical role sets. Operators see exactly which roles are missing
// from each side without a separate join.
type ReconciliationDrift struct {
	UserID        string   `json:"user_id"`
	ProjectID     string   `json:"project_id"`
	MkAuthRoles   []string `json:"mkauth_roles"`
	ZitadelRoles  []string `json:"zitadel_roles"`
	OnlyInMkAuth  []string `json:"only_in_mkauth"`
	OnlyInZitadel []string `json:"only_in_zitadel"`
	GrantID       string   `json:"grant_id,omitempty"`
}

// ReconciliationDiff is the full snapshot. Truncated is set only when the
// Zitadel side reported more grants than reconciliationSafetyCap can hold; in
// that case the diff is best-effort and the UI surfaces a warning. When
// Truncated is false, the diff is authoritative — both buckets and drift
// reflect the complete state at GeneratedAt.
type ReconciliationDiff struct {
	OnlyInMkAuth  []ReconciliationGrant `json:"only_in_mkauth"`
	OnlyInZitadel []ReconciliationGrant `json:"only_in_zitadel"`
	Drift         []ReconciliationDrift `json:"drift"`
	GeneratedAt   time.Time             `json:"generated_at"`
	Truncated     bool                  `json:"truncated"`
}

// handleGetReconciliationDiff is read-only: it surfaces the symmetric
// difference between MkAuth-direct grants and Zitadel-side user grants so
// operators can spot drift before it widens. No remediation is performed —
// the surface is visibility-only per the obsidian-clarity-redesign spec.
//
// Drift categories:
//   - only_in_mkauth: (user, project) pairs in the MkAuth direct-grants table
//     but absent from Zitadel. Usually a Zitadel-side revocation that wasn't
//     reflected back, or a pending sync.
//   - only_in_zitadel: (user, project) pairs Zitadel surfaces but MkAuth doesn't
//     directly track. Includes derived grants from mapping rules (expected) and
//     historical/manual grants (operator action needed).
//   - drift: same (user, project) on both sides but role sets diverge. The
//     response itemizes which keys are missing from each side.
func handleGetReconciliationDiff(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	mkauthGrants, err := svcAllDirectGrants(ctx)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	allZitadel, truncated, err := fetchAllZitadelGrants(ctx)
	if err != nil {
		jsonErrorResponse(w, http.StatusBadGateway, "ZITADEL_ERROR", err.Error())
		return
	}

	diff := computeReconciliationDiff(mkauthGrants, allZitadel)
	diff.Truncated = truncated
	diff.GeneratedAt = time.Now().UTC()

	// A lookup failure here MUST NOT silently become an empty set — that would
	// misclassify rule-derived / excluded grants as red drift. Fail the request.
	rules, err := svcGetActiveMappingRulesRecon(ctx)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	exclusions, err := svcGetExclusions(ctx)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	holder := services.BuildHolderSet(mkauthGrants, allZitadel)
	diff.OnlyInZitadel = filterExplained(diff.OnlyInZitadel, holder, rules, exclusions)

	jsonResponse(w, http.StatusOK, diff)
}

// filterExplained drops (user,project) entries whose every role is now explained
// by an active mapping rule or an external-grant exclusion — they are no longer
// pure Zitadel drift. A partially-explained entry keeps only its unexplained roles.
func filterExplained(in []ReconciliationGrant, holder map[services.HolderKey]bool,
	rules []models.MappingRule, exclusions []models.ExternalGrantExclusion) []ReconciliationGrant {
	out := make([]ReconciliationGrant, 0, len(in))
	for _, g := range in {
		var unexplained []string
		for _, rk := range g.RoleKeys {
			if services.ExpectedViaRule(holder, rules, g.UserID, g.ProjectID, rk) ||
				services.IsExcluded(exclusions, g.UserID, g.ProjectID, rk) {
				continue
			}
			unexplained = append(unexplained, rk)
		}
		if len(unexplained) > 0 {
			g.RoleKeys = unexplained
			out = append(out, g)
		}
	}
	return out
}

// fetchAllZitadelGrants paginates through Zitadel ListAllGrants until either
// every grant is fetched or reconciliationSafetyCap is hit. Returns the full
// slice, a truncated flag (true only when the cap halted iteration), and any
// transport error from a paged call. Iterating to completion is critical for
// reconciliation correctness — comparing MkAuth's full grant table against
// only the first Zitadel page produces false positives in only_in_mkauth and
// drift for any grant that lives on a later page.
func fetchAllZitadelGrants(ctx context.Context) ([]zitadel.UserGrant, bool, error) {
	var all []zitadel.UserGrant
	offset := 0
	for {
		page, err := zitadelListAllGrants(ctx, zitadel.SearchParams{
			Limit:  reconciliationPageSize,
			Offset: offset,
		})
		if err != nil {
			return nil, false, err
		}
		all = append(all, page.Items...)
		// Stop if we've drained the source. Total is the count Zitadel reports;
		// page.Items.length 0 is also a defensive stop in case a backend
		// regression returns Total=0 on a non-empty org.
		if len(all) >= page.Total || len(page.Items) == 0 {
			return all, false, nil
		}
		// Safety cap — bail before the slice grows unbounded on a pathological
		// directory size. Caller surfaces truncated=true so the UI can warn.
		if len(all) >= reconciliationSafetyCap {
			return all, true, nil
		}
		offset += len(page.Items)
	}
}

// computeReconciliationDiff is the pure comparison core. Extracted so tests
// can exercise it without spinning up an HTTP handler.
func computeReconciliationDiff(
	mkauthGrants []models.DirectGrant,
	zitadelGrants []zitadel.UserGrant,
) ReconciliationDiff {
	type pairKey struct{ userID, projectID string }

	// Build a (user, project) → role-set view of MkAuth direct grants. Multiple
	// rows for the same pair (one row per role) collapse into a single set.
	mkauthByPair := make(map[pairKey]map[string]struct{})
	for _, g := range mkauthGrants {
		k := pairKey{userID: g.UserID, projectID: g.ProjectID}
		if mkauthByPair[k] == nil {
			mkauthByPair[k] = make(map[string]struct{})
		}
		mkauthByPair[k][g.RoleKey] = struct{}{}
	}

	// Same for Zitadel side. UserGrant is already (user, project, []roleKeys),
	// but Zitadel can in principle emit two grants per pair (e.g. delegated
	// grants); merge defensively. Keep the first GrantID seen for drill-in.
	zitadelByPair := make(map[pairKey]map[string]struct{})
	zitadelGrantID := make(map[pairKey]string)
	for _, g := range zitadelGrants {
		k := pairKey{userID: g.UserID, projectID: g.ProjectID}
		if zitadelByPair[k] == nil {
			zitadelByPair[k] = make(map[string]struct{})
		}
		for _, role := range g.RoleKeys {
			zitadelByPair[k][role] = struct{}{}
		}
		if _, seen := zitadelGrantID[k]; !seen {
			zitadelGrantID[k] = g.ID
		}
	}

	out := ReconciliationDiff{
		OnlyInMkAuth:  []ReconciliationGrant{},
		OnlyInZitadel: []ReconciliationGrant{},
		Drift:         []ReconciliationDrift{},
	}

	// Walk MkAuth-side pairs first: emit only_in_mkauth or drift entries.
	for k, mkRoles := range mkauthByPair {
		zRoles, hasZ := zitadelByPair[k]
		if !hasZ {
			out.OnlyInMkAuth = append(out.OnlyInMkAuth, ReconciliationGrant{
				UserID:    k.userID,
				ProjectID: k.projectID,
				RoleKeys:  sortedKeys(mkRoles),
			})
			continue
		}

		// Same pair on both sides — diff the role sets.
		onlyInMkAuth := setDifference(mkRoles, zRoles)
		onlyInZitadel := setDifference(zRoles, mkRoles)
		if len(onlyInMkAuth) == 0 && len(onlyInZitadel) == 0 {
			continue // sets agree → no drift to report
		}
		out.Drift = append(out.Drift, ReconciliationDrift{
			UserID:        k.userID,
			ProjectID:     k.projectID,
			MkAuthRoles:   sortedKeys(mkRoles),
			ZitadelRoles:  sortedKeys(zRoles),
			OnlyInMkAuth:  onlyInMkAuth,
			OnlyInZitadel: onlyInZitadel,
			GrantID:       zitadelGrantID[k],
		})
	}

	// Pairs only in Zitadel (mapping-rule derivatives or pre-existing manual).
	for k, zRoles := range zitadelByPair {
		if _, hasMk := mkauthByPair[k]; hasMk {
			continue
		}
		out.OnlyInZitadel = append(out.OnlyInZitadel, ReconciliationGrant{
			UserID:    k.userID,
			ProjectID: k.projectID,
			RoleKeys:  sortedKeys(zRoles),
			GrantID:   zitadelGrantID[k],
		})
	}

	// Stable ordering for deterministic responses (eases UI rendering and tests).
	sort.Slice(out.OnlyInMkAuth, func(i, j int) bool { return reconciliationLess(out.OnlyInMkAuth[i], out.OnlyInMkAuth[j]) })
	sort.Slice(out.OnlyInZitadel, func(i, j int) bool { return reconciliationLess(out.OnlyInZitadel[i], out.OnlyInZitadel[j]) })
	sort.Slice(out.Drift, func(i, j int) bool {
		if out.Drift[i].UserID != out.Drift[j].UserID {
			return out.Drift[i].UserID < out.Drift[j].UserID
		}
		return out.Drift[i].ProjectID < out.Drift[j].ProjectID
	})

	return out
}

func reconciliationLess(a, b ReconciliationGrant) bool {
	if a.UserID != b.UserID {
		return a.UserID < b.UserID
	}
	return a.ProjectID < b.ProjectID
}

func sortedKeys(set map[string]struct{}) []string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// setDifference returns the elements of a that are not present in b, sorted.
func setDifference(a, b map[string]struct{}) []string {
	out := make([]string, 0)
	for k := range a {
		if _, present := b[k]; !present {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}
