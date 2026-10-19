package services

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"

	"syndra/internal/directory"
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

// --- what if ------------------------------------------------------------

var (
	// ErrInvalidWhatIf is a hypothetical that is not well formed: unknown kind,
	// a missing or malformed field. The handler answers 400.
	ErrInvalidWhatIf = errors.New("invalid what-if")
	// ErrWhatIfNotFound is a hypothetical about a bundle or rule that does not
	// exist. The handler answers 404.
	ErrWhatIfNotFound = errors.New("what-if subject not found")
)

func whatIfInvalid(field, why string) error {
	return fmt.Errorf("%w: %s %s", ErrInvalidWhatIf, field, why)
}

// validateWhatIf checks every field the kind needs, and every one it does not
// (a stray field is a request that does not mean what its sender thinks).
// Identifiers must be plain, so nothing odd reaches a query — a malformed uuid
// would otherwise surface as a 500.
func validateWhatIf(c models.InsightsWhatIfChange) error {
	need := map[string]string{}
	switch c.Kind {
	case "bundle_gives_also", "bundle_gives_not":
		need = map[string]string{"bundle_id": c.BundleID, "project_id": c.ProjectID, "role_key": c.RoleKey}
	case "bundle_gone":
		need = map[string]string{"bundle_id": c.BundleID}
	case "rule_gone":
		need = map[string]string{"rule_id": c.RuleID}
	default:
		return whatIfInvalid("kind", fmt.Sprintf("%q is not one of bundle_gives_also, bundle_gives_not, bundle_gone, rule_gone", c.Kind))
	}
	all := map[string]string{"bundle_id": c.BundleID, "project_id": c.ProjectID, "role_key": c.RoleKey, "rule_id": c.RuleID}
	for _, f := range []string{"bundle_id", "project_id", "role_key", "rule_id"} {
		v, wanted := need[f]
		switch {
		case wanted && v == "":
			return whatIfInvalid(f, "is required")
		case wanted && !insightRefPart.MatchString(v):
			return whatIfInvalid(f, "is malformed")
		case !wanted && all[f] != "":
			return whatIfInvalid(f, "does not apply to "+c.Kind)
		}
	}
	return nil
}

// InsightsWhatIf answers who would gain and who would lose which roles if the
// change were made, without making it: no plan, no outbox, no lock.
//
// Every number is computed the way the real cascade computes it — the same
// userBase* readers and the same effectiveClosure over the same rules — so the
// preview can't disagree with the change it previews.
//
// Reads run inside one read-only repeatable-read snapshot (svcInReadSnapshot),
// which does NOT take the access-mutation lock: a concurrent change can commit,
// the answer is simply of the world as it was when the snapshot opened.
//
// bundle_gives_also / bundle_gives_not are judged against each holder's OWN
// pinned version plus/minus the role, not against "the latest version", because
// that is what each of them would actually hold. People holding the bundle at a
// version that already has the role are therefore unchanged by gives_also.
//
// ponytail: per-person queries (a handful each) over the bundle's holders, or
// over every known user for rule_gone — the same cost as the cascade itself, at
// this deployment's ~200 users. A set-based read is the upgrade if it bites.
// gives_not keeps a role that an include of the holder's pinned version still
// gives (svcGetVersionIncludes, read once per distinct pinned version).
func InsightsWhatIf(ctx context.Context, change models.InsightsWhatIfChange) (models.InsightsWhatIf, error) {
	if err := validateWhatIf(change); err != nil {
		return models.InsightsWhatIf{}, err
	}
	// The directory is Zitadel over HTTP: read it before opening the snapshot
	// so a slow answer doesn't hold a transaction open.
	dirUsers, err := directory.Default.Users(ctx)
	if err != nil {
		return models.InsightsWhatIf{}, err
	}
	names := make(map[string]string, len(dirUsers))
	for _, u := range dirUsers {
		names[u.ID] = u.Name
	}

	out := models.InsightsWhatIf{Change: change, Gains: []models.InsightWhatIfPerson{}, Losses: []models.InsightWhatIfPerson{}}
	err = svcInReadSnapshot(ctx, func(ctx context.Context) error {
		basis, err := observationBasis(ctx)
		if err != nil {
			return err
		}
		out.ReadAt = basis.ReadAt

		rules, err := svcGetActiveMappingRules(ctx)
		if err != nil {
			return err
		}
		var people []string
		// holderChange returns a person's closure before and after the hypothetical.
		var holderChange func(ctx context.Context, userID string) (before, after map[roleKey]bool, err error)

		switch change.Kind {
		case "rule_gone":
			// A retired rule applies to nobody: there is nothing for it to go from.
			if rule, err := svcGetMappingRuleByID(ctx, change.RuleID); err != nil || rule.RetiredAt != nil {
				if err == nil {
					err = pgx.ErrNoRows
				}
				return whatIfNotFound(err, "rule")
			}
			without := make([]models.MappingRule, 0, len(rules))
			for _, r := range rules {
				if r.ID != change.RuleID {
					without = append(without, r)
				}
			}
			if people, err = svcGetAllKnownUserIDs(ctx); err != nil {
				return err
			}
			holderChange = func(ctx context.Context, u string) (map[roleKey]bool, map[roleKey]bool, error) {
				base, err := userBaseHoldings(ctx, u)
				if err != nil {
					return nil, nil, err
				}
				return effectiveClosure(base, rules), effectiveClosure(base, without), nil
			}
		default: // the three bundle kinds
			if bundle, err := svcGetBundleByID(ctx, change.BundleID); err != nil || bundle.RetiredAt != nil {
				if err == nil {
					err = pgx.ErrNoRows
				}
				return whatIfNotFound(err, "bundle")
			}
			holders, err := svcGetBundleHoldersByVersion(ctx, change.BundleID)
			if err != nil {
				return err
			}
			pin := make(map[string]string, len(holders)) // user -> pinned version id
			for _, h := range holders {
				people = append(people, h.UserID)
				pin[h.UserID] = h.VersionID
			}
			// Whether some include of a pinned version still gives the role,
			// read once per distinct version rather than once per holder.
			includeGives := map[string]bool{}
			target := roleKey{projectID: change.ProjectID, roleKey: change.RoleKey}
			holderChange = func(ctx context.Context, u string) (map[roleKey]bool, map[roleKey]bool, error) {
				before, err := userBaseHoldings(ctx, u)
				if err != nil {
					return nil, nil, err
				}
				var after map[roleKey]bool
				switch change.Kind {
				case "bundle_gone":
					after, err = userBaseHoldingsExcludingBundle(ctx, u, change.BundleID)
				default:
					add := change.Kind == "bundle_gives_also"
					var pinned []models.BundleRole
					if pinned, err = pinnedBundleRoles(ctx, u, change.BundleID); err != nil {
						break
					}
					keep := false
					if !add {
						if keep, err = anIncludeGives(ctx, includeGives, pin[u], target); err != nil {
							break
						}
					}
					after, err = userBaseHoldingsWithBundleAt(ctx, u, change.BundleID, withOrWithout(pinned, target, add, keep))
				}
				if err != nil {
					return nil, nil, err
				}
				return effectiveClosure(before, rules), effectiveClosure(after, rules), nil
			}
		}

		for _, u := range people {
			name, known := names[u]
			if !known {
				// Not in the directory: nobody to show, and nothing a change
				// would do to them that an operator could act on.
				continue
			}
			before, after, err := holderChange(ctx, u)
			if err != nil {
				return err
			}
			adds, revokes := closureDelta(before, after)
			if len(adds) == 0 && len(revokes) == 0 {
				out.Unchanged++
				continue
			}
			if len(adds) > 0 {
				out.Gains = append(out.Gains, whatIfPerson(u, name, adds))
			}
			if len(revokes) > 0 {
				out.Losses = append(out.Losses, whatIfPerson(u, name, revokes))
			}
		}
		return nil
	})
	if err != nil {
		return models.InsightsWhatIf{}, err
	}
	sortWhatIfPeople(out.Gains)
	sortWhatIfPeople(out.Losses)
	return out, nil
}

func whatIfNotFound(err error, what string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%w: no such %s", ErrWhatIfNotFound, what)
	}
	return err
}

// pinnedBundleRoles is what the bundle gives this person at the version they
// are pinned to (included bundles' roles too).
func pinnedBundleRoles(ctx context.Context, userID, bundleID string) ([]models.BundleRole, error) {
	grouped, err := svcGetUserBundleRolesGrouped(ctx, userID)
	if err != nil {
		return nil, err
	}
	return grouped[bundleID], nil
}

// anIncludeGives reports whether any bundle included by this pinned version
// gives the role, memoised in seen by version id. If one does, removing the role
// from the bundle itself takes nothing from the holder.
func anIncludeGives(ctx context.Context, seen map[string]bool, versionID string, k roleKey) (bool, error) {
	if v, ok := seen[versionID]; ok {
		return v, nil
	}
	includes, err := svcGetVersionIncludes(ctx, versionID)
	if err != nil {
		return false, err
	}
	gives := false
	for _, in := range includes {
		roles, err := svcGetRolesForVersion(ctx, in.ChildVersionID)
		if err != nil {
			return false, err
		}
		for _, r := range roles {
			if r.ProjectID == k.projectID && r.RoleKey == k.roleKey {
				gives = true
			}
		}
	}
	seen[versionID] = gives
	return gives, nil
}

// withOrWithout returns roles with the one role added (add), or with it removed
// unless keep says an include still gives it.
func withOrWithout(roles []models.BundleRole, k roleKey, add, keep bool) []models.BundleRole {
	out := make([]models.BundleRole, 0, len(roles)+1)
	for _, r := range roles {
		if !add && !keep && r.ProjectID == k.projectID && r.RoleKey == k.roleKey {
			continue
		}
		out = append(out, r)
	}
	if add {
		out = append(out, models.BundleRole{ProjectID: k.projectID, RoleKey: k.roleKey})
	}
	return out
}

func whatIfPerson(id, name string, keys []roleKey) models.InsightWhatIfPerson {
	p := models.InsightWhatIfPerson{UserID: id, Name: name, Roles: make([]models.RoleGrant, 0, len(keys))}
	for _, k := range keys { // closureDelta already sorted them
		p.Roles = append(p.Roles, models.RoleGrant{ProjectID: k.projectID, RoleKey: k.roleKey})
	}
	return p
}

func sortWhatIfPeople(ps []models.InsightWhatIfPerson) {
	sort.Slice(ps, func(i, j int) bool {
		if ps[i].Name != ps[j].Name {
			return ps[i].Name < ps[j].Name
		}
		return ps[i].UserID < ps[j].UserID
	})
}
