package services

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"

	"syndra/internal/models"
)

// singleRouteShare is how much of a role's holders must arrive by one bundle or
// one rule, and nothing else, for the role to be reported as having a single
// route. minSingleRouteHolders keeps a role three people hold from being a
// finding.
//
// ponytail: unsettled — drawn at >= 90% with at least 5 holders. Tune against
// real data; no config until somebody actually wants two values.
const (
	singleRouteShare      = 0.9
	minSingleRouteHolders = 5
)

// minOverlapRoles is how many roles two bundles must share to be reported.
const minOverlapRoles = 2

// InsightsFindings reads the whole org once (newAccessSnapshot) and derives
// every finding from it. Read-only; never queries per person beyond the
// snapshot's own memoised For().
func InsightsFindings(ctx context.Context) (models.InsightsFindings, error) {
	snap, err := newAccessSnapshot(ctx)
	if err != nil {
		return models.InsightsFindings{}, err
	}
	rules, err := svcGetActiveMappingRules(ctx)
	if err != nil {
		return models.InsightsFindings{}, err
	}
	basis, err := snap.Basis()
	if err != nil {
		return models.InsightsFindings{}, err
	}
	catalogue, dirProjects, err := collectCatalogEntries(ctx)
	if err != nil {
		return models.InsightsFindings{}, err
	}
	bundles, err := svcGetAllBundles(ctx)
	if err != nil {
		return models.InsightsFindings{}, err
	}

	type ruleKey struct{ target, trigger roleKey }
	ruleByDerivation := make(map[ruleKey]models.MappingRule, len(rules))
	for _, r := range rules {
		k := ruleKey{
			target:  roleKey{r.TargetProject, r.TargetRole},
			trigger: roleKey{r.SourceProject, r.SourceRole},
		}
		if _, ok := ruleByDerivation[k]; !ok {
			ruleByDerivation[k] = r
		}
	}

	type roleTally struct {
		holders int
		onlyVia map[string]int // source key -> holders for whom it is the only source
		sources map[string]models.InsightSource
	}
	type bundleTally struct {
		holders  int
		gave     bool // gave at least one role to somebody
		gaveNew  bool // sole source of at least one role for somebody
		name     string
		bundleID string
	}
	roles := map[roleKey]*roleTally{}
	bundleStats := map[string]*bundleTally{}
	heldProjects := map[string]bool{}

	for _, u := range snap.Users() {
		roleMap, held, err := snap.For(u.ID)
		if err != nil {
			return models.InsightsFindings{}, err
		}
		for _, b := range held {
			bt := bundleStats[b.ID]
			if bt == nil {
				bt = &bundleTally{name: b.Name, bundleID: b.ID}
				bundleStats[b.ID] = bt
			}
			bt.holders++
		}
		for key, role := range roleMap {
			sources := roleSources(role.Reasons, key, func(trigger roleKey) (string, string) {
				if r, ok := ruleByDerivation[ruleKey{target: key, trigger: trigger}]; ok {
					return r.ID, fmt.Sprintf("%s:%s → %s:%s", r.SourceProject, r.SourceRole, r.TargetProject, r.TargetRole)
				}
				return trigger.projectID + ":" + trigger.roleKey, trigger.projectID + ":" + trigger.roleKey
			})
			rt := roles[key]
			if rt == nil {
				rt = &roleTally{onlyVia: map[string]int{}, sources: map[string]models.InsightSource{}}
				roles[key] = rt
			}
			rt.holders++
			heldProjects[key.projectID] = true
			for k, s := range sources {
				rt.sources[k] = s
				if s.Kind == "bundle" {
					if bt := bundleStats[s.ID]; bt != nil {
						bt.gave = true
						if len(sources) == 1 {
							bt.gaveNew = true
						}
					}
				}
			}
			if len(sources) == 1 {
				for k, s := range sources {
					if s.Kind != "direct" {
						rt.onlyVia[k]++
					}
				}
			}
		}
	}

	out := models.InsightsFindings{
		ReadAt:       basis.ReadAt,
		SingleRoutes: []models.InsightSingleRoute{},
		Unused: models.InsightUnused{
			Roles:    []models.RoleGrant{},
			Bundles:  []models.InsightUnusedBundle{},
			Projects: []models.InsightProjectRef{},
		},
		Overlaps: []models.InsightOverlap{},
	}

	for key, rt := range roles {
		if rt.holders < minSingleRouteHolders {
			continue
		}
		for k, n := range rt.onlyVia {
			if float64(n)/float64(rt.holders) >= singleRouteShare {
				out.SingleRoutes = append(out.SingleRoutes, models.InsightSingleRoute{
					ProjectID: key.projectID, RoleKey: key.roleKey,
					Source: rt.sources[k], OnlyThatWay: n, Holders: rt.holders,
				})
			}
		}
	}
	sort.Slice(out.SingleRoutes, func(i, j int) bool {
		a, b := out.SingleRoutes[i], out.SingleRoutes[j]
		if a.ProjectID != b.ProjectID {
			return a.ProjectID < b.ProjectID
		}
		return a.RoleKey < b.RoleKey
	})

	for _, e := range catalogue {
		if roles[roleKey{e.projectID, e.roleKey}] == nil {
			out.Unused.Roles = append(out.Unused.Roles, models.RoleGrant{ProjectID: e.projectID, RoleKey: e.roleKey})
		}
	}
	sortGrants(out.Unused.Roles)

	// ponytail: "gives nothing new" is judged per holder against every OTHER
	// source, so two bundles that cover each other are both reported — dropping
	// either is safe, dropping both is not. Say so in the UI copy, or resolve
	// pairs here if it bites. A bundle that gave no roles to anybody is empty,
	// not redundant, and is left out like one with no holders.
	for _, bt := range bundleStats {
		if bt.holders > 0 && bt.gave && !bt.gaveNew {
			out.Unused.Bundles = append(out.Unused.Bundles, models.InsightUnusedBundle{
				BundleID: bt.bundleID, Name: bt.name, Holders: bt.holders,
			})
		}
	}
	sort.Slice(out.Unused.Bundles, func(i, j int) bool {
		a, b := out.Unused.Bundles[i], out.Unused.Bundles[j]
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.BundleID < b.BundleID
	})

	for _, p := range dirProjects {
		if !heldProjects[p.ID] {
			out.Unused.Projects = append(out.Unused.Projects, models.InsightProjectRef{ProjectID: p.ID, Name: p.Name})
		}
	}
	sort.Slice(out.Unused.Projects, func(i, j int) bool {
		a, b := out.Unused.Projects[i], out.Unused.Projects[j]
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.ProjectID < b.ProjectID
	})

	overlaps, err := bundleOverlaps(ctx, bundles)
	if err != nil {
		return models.InsightsFindings{}, err
	}
	out.Overlaps = overlaps
	return out, nil
}

// roleSources collapses a role's reasons into its distinct routes: direct, one
// per bundle held (however many included bundles it came through), one per rule.
// Several reasons from one route are one route — a role held through the same
// bundle twice is not held two ways.
func roleSources(reasons []models.RoleReason, key roleKey, rule func(trigger roleKey) (id, name string)) map[string]models.InsightSource {
	out := map[string]models.InsightSource{}
	for _, r := range reasons {
		switch r.Kind {
		case "direct":
			out["direct"] = models.InsightSource{Kind: "direct"}
		case "bundle":
			out["bundle:"+r.BundleID] = models.InsightSource{Kind: "bundle", ID: r.BundleID, Name: r.BundleName}
		case "mapping":
			id, name := rule(roleKey{r.TriggerProject, r.TriggerRole})
			out["rule:"+id] = models.InsightSource{Kind: "rule", ID: id, Name: name}
		}
	}
	return out
}

func sortGrants(g []models.RoleGrant) {
	sort.Slice(g, func(i, j int) bool {
		if g[i].ProjectID != g[j].ProjectID {
			return g[i].ProjectID < g[j].ProjectID
		}
		return g[i].RoleKey < g[j].RoleKey
	})
}

// bundleOverlaps pairs bundles whose LATEST published versions share at least
// minOverlapRoles roles. The effective view (included roles too) is what
// db.LatestVersionRoles reads, so a bundle that includes another overlaps it.
// A bundle with nothing published has nothing to overlap with and is skipped.
//
// ponytail: O(n^2) over bundles, one query per bundle; fine at tens of bundles.
func bundleOverlaps(ctx context.Context, bundles []models.Bundle) ([]models.InsightOverlap, error) {
	type entry struct {
		ref   models.InsightBundleRef
		roles map[roleKey]bool
	}
	sort.Slice(bundles, func(i, j int) bool {
		if bundles[i].Name != bundles[j].Name {
			return bundles[i].Name < bundles[j].Name
		}
		return bundles[i].ID < bundles[j].ID
	})
	var entries []entry
	for _, b := range bundles {
		_, rs, err := svcLatestVersionRoles(ctx, b.ID)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		set := make(map[roleKey]bool, len(rs))
		for _, r := range rs {
			set[roleKey{r.ProjectID, r.RoleKey}] = true
		}
		entries = append(entries, entry{ref: models.InsightBundleRef{BundleID: b.ID, Name: b.Name}, roles: set})
	}

	grants := func(m map[roleKey]bool, in func(roleKey) bool) []models.RoleGrant {
		g := []models.RoleGrant{}
		for k := range m {
			if in(k) {
				g = append(g, models.RoleGrant{ProjectID: k.projectID, RoleKey: k.roleKey})
			}
		}
		sortGrants(g)
		return g
	}
	out := []models.InsightOverlap{}
	for i := range entries {
		for j := i + 1; j < len(entries); j++ {
			a, b := entries[i], entries[j]
			shared := grants(a.roles, func(k roleKey) bool { return b.roles[k] })
			if len(shared) < minOverlapRoles {
				continue
			}
			out = append(out, models.InsightOverlap{
				A: a.ref, B: b.ref, Shared: shared,
				AOnly: grants(a.roles, func(k roleKey) bool { return !b.roles[k] }),
				BOnly: grants(b.roles, func(k roleKey) bool { return !a.roles[k] }),
			})
		}
	}
	return out, nil
}

// --- who has X and not Y -------------------------------------------------

// ErrInvalidInsightRef is a malformed `role:<project>:<key>` / `bundle:<id>`.
var ErrInvalidInsightRef = errors.New("invalid reference")

// InsightRef is one side of a who question: a role or a bundle. The zero value
// (Kind "") means "no reference" and matches nobody.
type InsightRef struct {
	Kind      string // "role" | "bundle"
	ProjectID string
	RoleKey   string
	BundleID  string
}

var insightRefPart = regexp.MustCompile(`^[A-Za-z0-9._-]{1,255}$`)

// ParseInsightRef accepts exactly `role:<project>:<key>` or `bundle:<id>`; every
// part must be a plain identifier, so nothing odd reaches a lookup.
func ParseInsightRef(s string) (InsightRef, error) {
	kind, rest, _ := strings.Cut(s, ":")
	parts := strings.Split(rest, ":")
	switch {
	case kind == "role" && len(parts) == 2 && insightRefPart.MatchString(parts[0]) && insightRefPart.MatchString(parts[1]):
		return InsightRef{Kind: "role", ProjectID: parts[0], RoleKey: parts[1]}, nil
	case kind == "bundle" && len(parts) == 1 && insightRefPart.MatchString(parts[0]):
		return InsightRef{Kind: "bundle", BundleID: parts[0]}, nil
	}
	return InsightRef{}, fmt.Errorf("%w: %q (want role:<project>:<key> or bundle:<id>)", ErrInvalidInsightRef, s)
}

// holds reports whether this person holds ref, and how (the snapshot's own
// reasons). Holding a bundle means holding it at any version.
func (ref InsightRef) holds(roleMap map[roleKey]*models.EffectiveRole, bundles []models.Bundle) ([]models.RoleReason, bool) {
	switch ref.Kind {
	case "role":
		if role := roleMap[roleKey{ref.ProjectID, ref.RoleKey}]; role != nil {
			return role.Reasons, true
		}
	case "bundle":
		for _, b := range bundles {
			if b.ID == ref.BundleID {
				return []models.RoleReason{{
					Kind:        "bundle",
					Description: fmt.Sprintf("Holds bundle %s v%d", b.Name, b.PinnedVersion),
					BundleID:    b.ID,
					BundleName:  b.Name,
				}}, true
			}
		}
	}
	return nil, false
}

// InsightsWho lists the people who hold `has` and not `without` (zero value:
// no exclusion), each with how they hold `has`. One snapshot, no per-person
// queries beyond its memoised For().
func InsightsWho(ctx context.Context, has, without InsightRef) (models.InsightsWho, error) {
	snap, err := newAccessSnapshot(ctx)
	if err != nil {
		return models.InsightsWho{}, err
	}
	out := models.InsightsWho{People: []models.InsightPerson{}}
	for _, u := range snap.Users() {
		roleMap, bundles, err := snap.For(u.ID)
		if err != nil {
			return models.InsightsWho{}, err
		}
		how, ok := has.holds(roleMap, bundles)
		if !ok {
			continue
		}
		if _, no := without.holds(roleMap, bundles); no {
			continue
		}
		out.People = append(out.People, models.InsightPerson{UserID: u.ID, Name: u.Name, Email: u.Email, How: how})
	}
	sort.Slice(out.People, func(i, j int) bool {
		a, b := out.People[i], out.People[j]
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.UserID < b.UserID
	})
	return out, nil
}
