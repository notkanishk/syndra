package handlers

import (
	"net/http"
	"sort"
	"time"

	"syndra/internal/db"
	"syndra/internal/models"
	"syndra/internal/services"
	"syndra/internal/zitadel"
)

// ReconciliationGrant is one (user, project, role-set) pair on either the
// Syndra or Zitadel side of the diff. Roles are sorted ascending so equality
// checks and rendering are stable.
type ReconciliationGrant struct {
	UserID    string   `json:"user_id"`
	ProjectID string   `json:"project_id"`
	RoleKeys  []string `json:"role_keys"`
	// GrantID is populated for Zitadel-side entries so the UI can drill into
	// the exact grant record. Empty on the Syndra side (each row is keyed by
	// (user_id, project_id, role_key) without a corresponding Zitadel grant).
	GrantID string `json:"grant_id,omitempty"`
}

// ReconciliationDrift is a (user, project) pair that exists on both sides but
// with non-identical role sets. Operators see exactly which roles are missing
// from each side without a separate join.
type ReconciliationDrift struct {
	UserID        string   `json:"user_id"`
	ProjectID     string   `json:"project_id"`
	SyndraRoles   []string `json:"syndra_roles"`
	ZitadelRoles  []string `json:"zitadel_roles"`
	OnlyInSyndra  []string `json:"only_in_syndra"`
	OnlyInZitadel []string `json:"only_in_zitadel"`
	GrantID       string   `json:"grant_id,omitempty"`
}

// ReconciliationDiff is the full snapshot. Truncated is set whenever the
// observation behind it did not finish — the operator's own read hit its cap,
// or Zitadel answered only part of it; in that case the diff is best-effort
// and the UI surfaces a warning. When Truncated is false, the diff is
// authoritative — both buckets and drift reflect the complete state at
// GeneratedAt.
type ReconciliationDiff struct {
	OnlyInSyndra  []ReconciliationGrant `json:"only_in_syndra"`
	OnlyInZitadel []ReconciliationGrant `json:"only_in_zitadel"`
	Drift         []ReconciliationDrift `json:"drift"`
	GeneratedAt   time.Time             `json:"generated_at"`
	Truncated     bool                  `json:"truncated"`
}

// handleGetReconciliationDiff is read-only: it surfaces the symmetric
// difference between Syndra-direct grants and Zitadel-side user grants so
// operators can spot drift before it widens. No remediation is performed —
// the surface is visibility-only per the obsidian-clarity-redesign spec.
//
// one-truth-many-checks, "The last two readers": the operator pressed a
// button asking for now, so a FRESH observation is right — this calls
// observe.Org, which does the paginated Zitadel read and records it, exactly
// as the periodic sweep and discovery.go's grant-listing routes do. It then
// diffs whatever the store holds afterwards, so the read this button pays for
// is shared rather than discarded: the drift sweep's next pass, and anybody
// else who reads the store meanwhile, see it too.
//
// Drift categories:
//   - only_in_syndra: (user, project) pairs in the Syndra direct-grants table
//     but absent from Zitadel. Usually a Zitadel-side revocation that wasn't
//     reflected back, or a pending sync.
//   - only_in_zitadel: (user, project) pairs Zitadel surfaces but Syndra doesn't
//     directly track. Includes derived grants from mapping rules (expected) and
//     historical/manual grants (operator action needed).
//   - drift: same (user, project) on both sides but role sets diverge. The
//     response itemizes which keys are missing from each side.
func handleGetReconciliationDiff(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	syndraGrants, err := svcAllDirectGrants(ctx)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	obs, err := observeOrg(ctx)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	observed, err := dbAllObservedGrants(ctx)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	// Nothing came back at all, and the read that just ran is the reason —
	// not a real, empty org. Rendering this as an ordinary diff would put
	// every Syndra-side grant into only_in_syndra, which is the false
	// "somebody lost their access" this whole change exists to prevent.
	// Nothing was observed AT ALL this request — there is no client to ask.
	// `observe.Org` answers a zero Observation in local-policy-only mode, and
	// the store may still hold rows from when there was one. Diffing against
	// those would present a remembered world as a comparison made just now,
	// and stamp it `GeneratedAt` the Unix epoch.
	//
	// Checked before the emptiness test below, because it is a different fact
	// with a different answer: there is nothing to compare against, rather
	// than something that would not answer.
	if obs.ObservedAt.IsZero() {
		jsonErrorResponse(w, http.StatusBadGateway, "ZITADEL_NOT_CONFIGURED",
			"Syndra has no connection to Zitadel, so there is nothing to compare against.")
		return
	}
	if !obs.Complete && len(observed) == 0 {
		msg := obs.Error
		if msg == "" {
			msg = "Zitadel did not answer"
		}
		jsonErrorResponse(w, http.StatusBadGateway, "ZITADEL_ERROR", msg)
		return
	}
	allZitadel := services.ObservedToUserGrants(observed)

	diff := computeReconciliationDiff(syndraGrants, allZitadel)
	diff.Truncated = !obs.Complete
	diff.GeneratedAt = obs.ObservedAt

	// A lookup failure here MUST NOT silently become an empty set — that would
	// misclassify rule-derived / excluded grants as red drift. Fail the request.
	rules, err := svcGetActiveMappingRulesRecon(ctx)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	exclusions, err := svcGetExclusions(ctx, db.TargetZitadel)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	// Bundles, on the same terms as the rules and exclusions above — a failed
	// read fails the request rather than becoming an empty set.
	//
	// This detector is the THIRD to answer "does Syndra account for this
	// grant?", and the second to have been written without bundles in it. The
	// first cost a production incident: every projected bundle role reported as
	// unexplained drift, permanently. Here it is worse-tempered still, because
	// `syndraGrants` is direct grants only — so a bundle-projected role does not
	// merely go unexplained, it lands in "In Zitadel but not in Syndra" in
	// danger tone, on the screen an operator uses to decide what to adopt. And
	// adopting it writes the redundant direct grant that stops bundle removal
	// from revoking anything.
	bundled, err := svcAllBundleDerivedGrantsRecon(ctx)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	bundleSet := make(map[services.HolderKey]bool, len(bundled))
	for _, g := range bundled {
		bundleSet[services.HolderKey{UserID: g.UserID, ProjectID: g.ProjectID, RoleKey: g.RoleKey}] = true
	}

	holder := services.BuildHolderSet(syndraGrants, allZitadel)
	// What a bundle gives is held, not merely intended, so rule derivation has
	// to see it — the same hop the sweep needed.
	for k := range bundleSet {
		holder[k] = true
	}
	diff.OnlyInZitadel = filterExplained(diff.OnlyInZitadel, holder, bundleSet, rules, exclusions)

	jsonResponse(w, http.StatusOK, diff)
}

// filterExplained drops (user,project) entries whose every role Syndra accounts
// for — they are no longer pure Zitadel drift. A partially-explained entry keeps
// only its unexplained roles.
//
// It must cover every services.IntentSource. The direct-grant arm is implicit
// and worth naming: `OnlyInZitadel` is computed by diffing against Syndra's
// direct grants, so anything reaching here is already known not to be one.
func filterExplained(in []ReconciliationGrant, holder, bundleSet map[services.HolderKey]bool,
	rules []models.MappingRule, exclusions []models.ExternalGrantExclusion) []ReconciliationGrant {
	out := make([]ReconciliationGrant, 0, len(in))
	for _, g := range in {
		var unexplained []string
		for _, rk := range g.RoleKeys {
			if bundleSet[services.HolderKey{UserID: g.UserID, ProjectID: g.ProjectID, RoleKey: rk}] ||
				services.ExpectedViaRule(holder, rules, g.UserID, g.ProjectID, rk) ||
				services.IsExcluded(exclusions, db.TargetZitadel, g.UserID, g.ProjectID, rk) {
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

// computeReconciliationDiff is the pure comparison core. Extracted so tests
// can exercise it without spinning up an HTTP handler.
func computeReconciliationDiff(
	syndraGrants []models.DirectGrant,
	zitadelGrants []zitadel.UserGrant,
) ReconciliationDiff {
	type pairKey struct{ userID, projectID string }

	// Build a (user, project) → role-set view of Syndra direct grants. Multiple
	// rows for the same pair (one row per role) collapse into a single set.
	syndraByPair := make(map[pairKey]map[string]struct{})
	for _, g := range syndraGrants {
		k := pairKey{userID: g.UserID, projectID: g.ProjectID}
		if syndraByPair[k] == nil {
			syndraByPair[k] = make(map[string]struct{})
		}
		syndraByPair[k][g.RoleKey] = struct{}{}
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
		OnlyInSyndra:  []ReconciliationGrant{},
		OnlyInZitadel: []ReconciliationGrant{},
		Drift:         []ReconciliationDrift{},
	}

	// Walk Syndra-side pairs first: emit only_in_syndra or drift entries.
	for k, mkRoles := range syndraByPair {
		zRoles, hasZ := zitadelByPair[k]
		if !hasZ {
			out.OnlyInSyndra = append(out.OnlyInSyndra, ReconciliationGrant{
				UserID:    k.userID,
				ProjectID: k.projectID,
				RoleKeys:  sortedKeys(mkRoles),
			})
			continue
		}

		// Same pair on both sides — diff the role sets.
		onlyInSyndra := setDifference(mkRoles, zRoles)
		onlyInZitadel := setDifference(zRoles, mkRoles)
		if len(onlyInSyndra) == 0 && len(onlyInZitadel) == 0 {
			continue // sets agree → no drift to report
		}
		out.Drift = append(out.Drift, ReconciliationDrift{
			UserID:        k.userID,
			ProjectID:     k.projectID,
			SyndraRoles:   sortedKeys(mkRoles),
			ZitadelRoles:  sortedKeys(zRoles),
			OnlyInSyndra:  onlyInSyndra,
			OnlyInZitadel: onlyInZitadel,
			GrantID:       zitadelGrantID[k],
		})
	}

	// Pairs only in Zitadel (mapping-rule derivatives or pre-existing manual).
	for k, zRoles := range zitadelByPair {
		if _, hasMk := syndraByPair[k]; hasMk {
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
	sort.Slice(out.OnlyInSyndra, func(i, j int) bool { return reconciliationLess(out.OnlyInSyndra[i], out.OnlyInSyndra[j]) })
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
